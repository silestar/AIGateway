package api

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/bokelife/aigateway/internal/plugin"
)

// PluginHandler 插件 API
type PluginHandler struct {
	pluginMgr *plugin.Manager
}

// NewPluginHandler 创建插件 Handler
func NewPluginHandler(pluginMgr *plugin.Manager) *PluginHandler {
	return &PluginHandler{pluginMgr: pluginMgr}
}

// RegisterRoutes 注册插件路由
func (h *PluginHandler) RegisterRoutes(rg *gin.RouterGroup) {
	p := rg.Group("/plugins")
	p.GET("", h.List)
	p.POST("/upload", h.Create)
	p.GET("/:id", h.GetById)
	p.PUT("/:id/status", h.UpdateStatus)
	p.DELETE("/:id", h.Delete)
	p.PUT("/:id/config", h.UpdateConfig)
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

// Create 安装插件（上传 ZIP）
func (h *PluginHandler) Create(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_file", "请上传 ZIP 文件"))
		return
	}

	// 保存到随机临时文件，避免并发覆盖
	tmpFile, err := os.CreateTemp("", "plugin_*.zip")
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("temp_failed", err.Error()))
		return
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath) // 处理完成后清理

	if err := c.SaveUploadedFile(file, tmpPath); err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("save_failed", err.Error()))
		return
	}

	p, err := h.pluginMgr.Install(c.Request.Context(), tmpPath)
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
