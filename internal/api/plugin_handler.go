package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	adapterregistry "github.com/silestar/AIGateway/pkg/adapter/registry"
	"github.com/gin-gonic/gin"
	"github.com/silestar/AIGateway/internal/config"
	"github.com/silestar/AIGateway/internal/plugin"
)

// RegistryEntry 注册中心插件条目
type RegistryEntry struct {
	Name         string `json:"name"`
	Version      string `json:"version"`
	Description  string `json:"description"`
	Author       string `json:"author"`
	DownloadURL  string `json:"download_url"`
	HomePage     string `json:"homepage,omitempty"`
	Tags         string `json:"tags,omitempty"`          // JSON array string
	MinAGWVersion string `json:"min_agw_version,omitempty"` // 最低 AGW 版本要求
}

// registryCache 注册中心缓存
type registryCache struct {
	mu       sync.RWMutex
	entries  []RegistryEntry
	fetchedAt time.Time
	ttl      time.Duration
}

// PluginHandler 插件 API
type PluginHandler struct {
	pluginMgr *plugin.Manager
	cfg       *config.Config
	registry  *registryCache
}

// NewPluginHandler 创建插件 Handler
func NewPluginHandler(pluginMgr *plugin.Manager, cfg *config.Config) *PluginHandler {
	return &PluginHandler{
		pluginMgr: pluginMgr,
		cfg:       cfg,
		registry: &registryCache{
			ttl: 5 * time.Minute,
		},
	}
}

// RegisterRoutes 注册插件路由
func (h *PluginHandler) RegisterRoutes(rg *gin.RouterGroup) {
	p := rg.Group("/plugins")
	p.GET("", h.List)
	p.POST("/upload", h.Upload)        // 上传 ZIP → 只解析返回预览
	p.POST("/install", h.Install)      // 根据 upload_id 执行安装
	p.GET("/:id", h.GetById)
	p.PUT("/:id/status", h.UpdateStatus)
	p.DELETE("/:id", h.Delete)
	p.PUT("/:id/config", h.UpdateConfig)
	// 渠道级插件配置
	p.GET("/:id/channel-configs", h.ListChannelConfigs)
	p.PUT("/:id/channel-configs/:channelId", h.SetChannelConfig)
	p.DELETE("/:id/channel-configs/:channelId", h.DeleteChannelConfig)
	// 权限管理
	p.GET("/:id/permissions", h.GetPermissions)
	p.PUT("/:id/permissions/:permName/grant", h.GrantPermission)
	p.PUT("/:id/permissions/:permName/deny", h.DenyPermission)
	p.POST("/:id/permissions/grant-all", h.GrantAllPermissions)
	p.POST("/:id/permissions/deny-all", h.DenyAllPermissions)
	// 注册中心
	p.GET("/registry/list", h.RegistryList)
	p.POST("/registry/install", h.RegistryInstall)
	// 渠道类型
	p.GET("/channel-types", h.ListChannelTypes)
}

// List 插件列表
func (h *PluginHandler) List(c *gin.Context) {
	plugins, err := h.pluginMgr.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("internal_error", err.Error()))
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": plugins, "total": len(plugins)})
}

// Upload 上传插件 ZIP（只解析返回预览，不安装）
func (h *PluginHandler) Upload(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_file", "请上传 ZIP 文件"))
		return
	}

	// 保存到随机临时文件
	tmpFile, err := os.CreateTemp("", "plugin_upload_*.zip")
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("temp_failed", err.Error()))
		return
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	if err := c.SaveUploadedFile(file, tmpPath); err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("save_failed", err.Error()))
		return
	}

	// 解析 manifest，保存到待安装目录
	manifest, uploadID, err := h.pluginMgr.Upload(c.Request.Context(), tmpPath)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("upload_failed", err.Error()))
		return
	}

	// 返回预览信息 + upload_id
	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"upload_id":    uploadID,
			"name":         manifest.Name,
			"version":      manifest.Version,
			"description":  manifest.Description,
			"author":       manifest.Author,
			"type":         manifest.Type,
			"hooks":        manifest.Hooks,
			"port":         manifest.Port,
			"config_schema": manifest.ConfigSchema,
		},
	})
}

// Install 根据 upload_id 安装插件
func (h *PluginHandler) Install(c *gin.Context) {
	var req struct {
		UploadID string `json:"upload_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_request", err.Error()))
		return
	}

	p, err := h.pluginMgr.InstallFromUpload(c.Request.Context(), req.UploadID)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("install_failed", err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": p})
}

// GetById 获取插件详情
func (h *PluginHandler) GetById(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_id", "invalid plugin id"))
		return
	}

	p, err := h.pluginMgr.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, errorResponse("not_found", "plugin not found"))
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": p})
}

// UpdateStatus 启用/禁用插件
func (h *PluginHandler) UpdateStatus(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_id", "invalid plugin id"))
		return
	}

	var req struct {
		Action string `json:"action" binding:"required"` // start / stop
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_request", err.Error()))
		return
	}

	switch req.Action {
	case "start":
		if err := h.pluginMgr.Start(c.Request.Context(), id); err != nil {
			c.JSON(http.StatusInternalServerError, errorResponse("start_failed", err.Error()))
			return
		}
	case "stop":
		if err := h.pluginMgr.Stop(c.Request.Context(), id); err != nil {
			c.JSON(http.StatusInternalServerError, errorResponse("stop_failed", err.Error()))
			return
		}
	default:
		c.JSON(http.StatusBadRequest, errorResponse("invalid_action", "action must be start or stop"))
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{"id": id, "action": req.Action}})
}

// UpdateConfig 更新插件配置
func (h *PluginHandler) UpdateConfig(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_id", "invalid plugin id"))
		return
	}

	var req struct {
		Config string `json:"config" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_request", err.Error()))
		return
	}

	if err := h.pluginMgr.UpdateConfig(c.Request.Context(), id, req.Config); err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("update_failed", err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{"id": id}})
}

// Delete 卸载插件
func (h *PluginHandler) Delete(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_id", "invalid plugin id"))
		return
	}

	if err := h.pluginMgr.Uninstall(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("uninstall_failed", err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{"id": id}})
}

// ListChannelConfigs 获取某插件的所有渠道级配置
func (h *PluginHandler) ListChannelConfigs(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_id", "invalid plugin id"))
		return
	}
	settings, err := h.pluginMgr.ListChannelPluginConfigs(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("internal_error", err.Error()))
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": settings})
}

// SetChannelConfig 设置某插件在某渠道的配置
func (h *PluginHandler) SetChannelConfig(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_id", "invalid plugin id"))
		return
	}
	channelID, err := parseIDFromParam(c, "channelId")
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_channel_id", "invalid channel id"))
		return
	}
	var body struct {
		Config string `json:"config"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_body", err.Error()))
		return
	}
	if err := h.pluginMgr.SetChannelPluginConfig(c.Request.Context(), channelID, id, body.Config); err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("save_failed", err.Error()))
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"plugin_id": id, "channel_id": channelID}})
}

// RegistryList 获取注册中心插件列表（带缓存）
func (h *PluginHandler) RegistryList(c *gin.Context) {
	registryURL := h.cfg.Plugin.PluginRegistryURL
	if registryURL == "" {
		c.JSON(http.StatusBadRequest, errorResponse("not_configured", "plugin registry URL is not configured"))
		return
	}

	// 检查缓存
	h.registry.mu.RLock()
	if h.registry.entries != nil && time.Since(h.registry.fetchedAt) < h.registry.ttl {
		entries := h.registry.entries
		h.registry.mu.RUnlock()
		c.JSON(http.StatusOK, gin.H{"data": entries})
		return
	}
	h.registry.mu.RUnlock()

	// 缓存过期或为空，从远程拉取
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, registryURL, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("request_failed", err.Error()))
		return
	}

	// 如果需要认证，附加 session token
	if h.cfg.Plugin.UseRegistryAuth {
		token := c.GetHeader("Authorization")
		if token != "" {
			req.Header.Set("Authorization", token)
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, errorResponse("registry_unreachable", err.Error()))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		c.JSON(http.StatusBadGateway, errorResponse("registry_error", fmt.Sprintf("registry returned %d: %s", resp.StatusCode, string(body))))
		return
	}

	var entries []RegistryEntry
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("parse_failed", err.Error()))
		return
	}

	// 更新缓存
	h.registry.mu.Lock()
	h.registry.entries = entries
	h.registry.fetchedAt = time.Now()
	h.registry.mu.Unlock()

	c.JSON(http.StatusOK, gin.H{"data": entries})
}

// RegistryInstall 从注册中心安装插件
func (h *PluginHandler) RegistryInstall(c *gin.Context) {
	registryURL := h.cfg.Plugin.PluginRegistryURL
	if registryURL == "" {
		c.JSON(http.StatusBadRequest, errorResponse("not_configured", "plugin registry URL is not configured"))
		return
	}

	var req struct {
		Name        string `json:"name" binding:"required"`
		DownloadURL string `json:"download_url" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_request", err.Error()))
		return
	}

	// 1. 下载 ZIP 到临时文件
	client := &http.Client{Timeout: 60 * time.Second}
	httpReq, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, req.DownloadURL, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("download_request_failed", err.Error()))
		return
	}

	if h.cfg.Plugin.UseRegistryAuth {
		token := c.GetHeader("Authorization")
		if token != "" {
			httpReq.Header.Set("Authorization", token)
		}
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		c.JSON(http.StatusBadGateway, errorResponse("download_failed", err.Error()))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.JSON(http.StatusBadGateway, errorResponse("download_error", fmt.Sprintf("download returned %d", resp.StatusCode)))
		return
	}

	// 2. 保存到临时文件
	tmpFile, err := os.CreateTemp("", "registry_plugin_*.zip")
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("temp_failed", err.Error()))
		return
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		tmpFile.Close()
		c.JSON(http.StatusInternalServerError, errorResponse("save_failed", err.Error()))
		return
	}
	tmpFile.Close()

	// 3. 调用已有的 Install 流程
	p, err := h.pluginMgr.Install(c.Request.Context(), tmpPath)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("install_failed", err.Error()))
		return
	}

	// 4. 安装成功后清除缓存，下次拉取时刷新
	h.registry.mu.Lock()
	h.registry.entries = nil
	h.registry.mu.Unlock()

	c.JSON(http.StatusOK, gin.H{"data": p})
}

// DeleteChannelConfig 删除某插件在某渠道的配置
func (h *PluginHandler) DeleteChannelConfig(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_id", "invalid plugin id"))
		return
	}
	channelID, err := parseIDFromParam(c, "channelId")
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_channel_id", "invalid channel id"))
		return
	}
	if err := h.pluginMgr.DeleteChannelPluginConfig(c.Request.Context(), channelID, id); err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("delete_failed", err.Error()))
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"plugin_id": id, "channel_id": channelID}})
}

// ListChannelTypes 获取所有渠道类型（内置 + 插件注册的）
func (h *PluginHandler) ListChannelTypes(c *gin.Context) {
	types := adapterregistry.ListChannelTypes()
	c.JSON(http.StatusOK, gin.H{"data": types})
}

// ========== 权限管理 API ==========

// GetPermissions 获取插件权限列表
func (h *PluginHandler) GetPermissions(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_id", "invalid plugin id"))
		return
	}
	p, err := h.pluginMgr.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, errorResponse("not_found", "plugin not found"))
		return
	}
	perms, err := h.pluginMgr.GetPermissions(c.Request.Context(), p.Name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("query_failed", err.Error()))
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": perms})
}

// GrantPermission 授予插件权限
func (h *PluginHandler) GrantPermission(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_id", "invalid plugin id"))
		return
	}
	permName := c.Param("permName")
	p, err := h.pluginMgr.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, errorResponse("not_found", "plugin not found"))
		return
	}
	grantedBy := c.GetString("username")
	if grantedBy == "" {
		grantedBy = "admin"
	}
	if err := h.pluginMgr.GrantPermission(c.Request.Context(), p.Name, permName, grantedBy); err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("grant_failed", err.Error()))
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"plugin_name": p.Name, "permission": permName, "status": "granted"}})
}

// DenyPermission 撤销插件权限
func (h *PluginHandler) DenyPermission(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_id", "invalid plugin id"))
		return
	}
	permName := c.Param("permName")
	p, err := h.pluginMgr.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, errorResponse("not_found", "plugin not found"))
		return
	}
	grantedBy := c.GetString("username")
	if grantedBy == "" {
		grantedBy = "admin"
	}
	if err := h.pluginMgr.DenyPermission(c.Request.Context(), p.Name, permName, grantedBy); err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("deny_failed", err.Error()))
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"plugin_name": p.Name, "permission": permName, "status": "denied"}})
}

// GrantAllPermissions 全部授予
func (h *PluginHandler) GrantAllPermissions(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_id", "invalid plugin id"))
		return
	}
	p, err := h.pluginMgr.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, errorResponse("not_found", "plugin not found"))
		return
	}
	grantedBy := c.GetString("username")
	if grantedBy == "" {
		grantedBy = "admin"
	}
	if err := h.pluginMgr.GrantAllPermissions(c.Request.Context(), p.Name, grantedBy); err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("grant_all_failed", err.Error()))
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"plugin_name": p.Name, "status": "all_granted"}})
}

// DenyAllPermissions 全部撤销
func (h *PluginHandler) DenyAllPermissions(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_id", "invalid plugin id"))
		return
	}
	p, err := h.pluginMgr.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, errorResponse("not_found", "plugin not found"))
		return
	}
	grantedBy := c.GetString("username")
	if grantedBy == "" {
		grantedBy = "admin"
	}
	if err := h.pluginMgr.DenyAllPermissions(c.Request.Context(), p.Name, grantedBy); err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("deny_all_failed", err.Error()))
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"plugin_name": p.Name, "status": "all_denied"}})
}
