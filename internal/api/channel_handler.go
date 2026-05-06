package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/bokelife/aigateway/internal/account"
	"github.com/bokelife/aigateway/internal/channel"
)

// ChannelHandler 渠道管理 API
type ChannelHandler struct {
	svc        channel.ChannelService
	accountMgr account.AccountManager
}

func NewChannelHandler(svc channel.ChannelService, accountMgr account.AccountManager) *ChannelHandler {
	return &ChannelHandler{svc: svc, accountMgr: accountMgr}
}

// RegisterRoutes 注册渠道路由
func (h *ChannelHandler) RegisterRoutes(rg *gin.RouterGroup) {
	channels := rg.Group("/channels")
	channels.GET("", h.List)
	channels.POST("", h.Create)
	channels.GET("/:id", h.GetById)
	channels.PUT("/:id", h.Update)
	channels.DELETE("/:id", h.Delete)
	channels.PATCH("/:id/status", h.UpdateStatus)
	channels.PATCH("/:id/weight", h.UpdateWeight)
	channels.POST("/test-connection", h.TestConnection)
	channels.POST("/:id/fetch-models", h.FetchModels)
	channels.GET("/:id/models", h.GetModelsByChannel)
	channels.PUT("/:id/models", h.SaveModels)
}

// Create 创建渠道
func (h *ChannelHandler) Create(c *gin.Context) {
	var req struct {
		Name    string `json:"name" binding:"required"`
		Type    string `json:"type" binding:"required"`
		BaseURL string `json:"base_url" binding:"required"`
		APIKey  string `json:"api_key" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_request", err.Error()))
		return
	}

	ch, err := h.svc.Create(c.Request.Context(), req.Name, req.Type, req.BaseURL)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("create_failed", err.Error()))
		return
	}

	// 自动创建第一个账号
	acc, err := h.accountMgr.Create(c.Request.Context(), ch.ID, req.APIKey)
	if err != nil {
		// 账号创建失败不影响渠道创建，但返回警告
		c.JSON(http.StatusOK, gin.H{"data": ch, "warning": "渠道已创建但账号添加失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": ch, "account_id": acc.ID})
}

// TestConnection 测试渠道连接
func (h *ChannelHandler) TestConnection(c *gin.Context) {
	var req struct {
		Type    string `json:"type" binding:"required"`
		BaseURL string `json:"base_url" binding:"required"`
		APIKey  string `json:"api_key" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_request", err.Error()))
		return
	}

	if err := h.svc.TestConnection(c.Request.Context(), req.Type, req.BaseURL, req.APIKey); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// List 渠道列表
func (h *ChannelHandler) List(c *gin.Context) {
	filter := channel.ListFilter{
		Page:     intQuery(c, "page", 1),
		PageSize: intQuery(c, "page_size", 20),
		Status:   c.Query("status"),
		Type:     c.Query("type"),
	}

	items, total, err := h.svc.List(c.Request.Context(), filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("internal_error", err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":      items,
		"total":     total,
		"page":      filter.Page,
		"page_size": filter.PageSize,
	})
}

// GetById 获取渠道详情
func (h *ChannelHandler) GetById(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_id", err.Error()))
		return
	}

	ch, err := h.svc.GetById(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, errorResponse("not_found", "channel not found"))
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": ch})
}

// Update 更新渠道
func (h *ChannelHandler) Update(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_id", err.Error()))
		return
	}

	var req struct {
		Name    string `json:"name"`
		BaseURL string `json:"base_url"`
		Weight  int    `json:"weight"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_request", err.Error()))
		return
	}

	if err := h.svc.Update(c.Request.Context(), id, req.Name, req.BaseURL, req.Weight); err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("internal_error", err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{"id": id}})
}

// UpdateStatus 更新渠道状态
func (h *ChannelHandler) UpdateStatus(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_id", err.Error()))
		return
	}

	var req struct {
		Status string `json:"status" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_request", err.Error()))
		return
	}

	if err := h.svc.UpdateStatus(c.Request.Context(), id, req.Status); err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("internal_error", err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{"id": id, "status": req.Status}})
}

// UpdateWeight 更新渠道权重
func (h *ChannelHandler) UpdateWeight(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_id", err.Error()))
		return
	}

	var req struct {
		Weight int `json:"weight"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_request", err.Error()))
		return
	}

	if err := h.svc.UpdateWeight(c.Request.Context(), id, req.Weight); err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("internal_error", err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{"id": id, "weight": req.Weight}})
}

// Delete 删除渠道
func (h *ChannelHandler) Delete(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_id", err.Error()))
		return
	}

	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("internal_error", err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{"id": id}})
}

// GetModelsByChannel 获取渠道已配置的模型列表
func (h *ChannelHandler) GetModelsByChannel(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_id", err.Error()))
		return
	}
	models, err := h.svc.GetModelsByChannel(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("get_models_failed", err.Error()))
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": models})
}

// FetchModels 获取渠道可用模型
func (h *ChannelHandler) FetchModels(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_id", err.Error()))
		return
	}

	var req struct {
		TestKey string `json:"test_key"`
	}
	c.ShouldBindJSON(&req) // 可选参数

	apiKey := req.TestKey
	if apiKey == "" {
		// testKey 为空时，自动从渠道的活跃账号解密获取
		accounts, err := h.accountMgr.ListByChannel(c.Request.Context(), id)
		if err != nil || len(accounts) == 0 {
			c.JSON(http.StatusBadRequest, errorResponse("no_account", "该渠道没有可用账号，请先添加账号或提供测试密钥"))
			return
		}
		// 找第一个 active 的账号
		for _, acc := range accounts {
			if acc.Status == "active" {
				plainKey, err := h.accountMgr.GetDecryptedAPIKey(c.Request.Context(), acc.ID)
				if err != nil {
					continue
				}
				apiKey = plainKey
				break
			}
		}
		if apiKey == "" {
			c.JSON(http.StatusBadRequest, errorResponse("no_active_account", "该渠道没有活跃账号，请先启用账号或提供测试密钥"))
			return
		}
	}

	models, err := h.svc.FetchModels(c.Request.Context(), id, apiKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("fetch_models_failed", err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": models})
}

// SaveModels 保存渠道模型映射
func (h *ChannelHandler) SaveModels(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_id", err.Error()))
		return
	}

	var req struct {
		Models []channel.ChannelModel `json:"models" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_request", err.Error()))
		return
	}

	if err := h.svc.SaveModels(c.Request.Context(), id, req.Models); err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("save_models_failed", err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{"id": id, "count": len(req.Models)}})
}