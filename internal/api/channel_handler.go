package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/bokelife/aigateway/internal/channel"
)

// ChannelHandler 渠道管理 API
type ChannelHandler struct {
	svc channel.ChannelService
}

func NewChannelHandler(svc channel.ChannelService) *ChannelHandler {
	return &ChannelHandler{svc: svc}
}

// RegisterRoutes 注册渠道路由
func (h *ChannelHandler) RegisterRoutes(rg *gin.RouterGroup) {
	channels := rg.Group("/channels")
	channels.GET("", h.List)
	channels.POST("", h.Create)
	channels.GET("/:id", h.GetById)
	channels.PUT("/:id", h.Update)
	channels.DELETE("/:id", h.Delete)
	channels.POST("/:id/fetch-models", h.FetchModels)
	channels.PUT("/:id/models", h.SaveModels)
}

// Create 创建渠道
func (h *ChannelHandler) Create(c *gin.Context) {
	var req struct {
		Name     string `json:"name" binding:"required"`
		Type     string `json:"type" binding:"required"`
		BaseURL  string `json:"base_url" binding:"required"`
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

	c.JSON(http.StatusOK, gin.H{"data": ch})
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

	models, err := h.svc.FetchModels(c.Request.Context(), id, req.TestKey)
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