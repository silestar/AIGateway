package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/bokelife/aigateway/internal/consumer"
)

// ConsumerHandler 消费者管理 API
type ConsumerHandler struct {
	svc consumer.ConsumerService
}

func NewConsumerHandler(svc consumer.ConsumerService) *ConsumerHandler {
	return &ConsumerHandler{svc: svc}
}

// RegisterRoutes 注册消费者路由
func (h *ConsumerHandler) RegisterRoutes(rg *gin.RouterGroup) {
	consumers := rg.Group("/consumers")
	consumers.GET("", h.List)
	consumers.POST("", h.Create)
	consumers.GET("/:id", h.GetById)
	consumers.PUT("/:id", h.Update)
	consumers.DELETE("/:id", h.Delete)
	consumers.PUT("/:id/status", h.UpdateStatus)
	consumers.POST("/:id/reset-key", h.ResetKey)
	consumers.POST("/:id/reveal-key", h.RevealKey)
}

// Create 创建消费者
func (h *ConsumerHandler) Create(c *gin.Context) {
	var req struct {
		Name string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_request", err.Error()))
		return
	}

	cons, apiKey, err := h.svc.Create(c.Request.Context(), req.Name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("internal_error", err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"id":       cons.ID,
			"name":     cons.Name,
			"status":   cons.Status,
			"api_key":  apiKey, // 仅创建时返回明文
			"created_at": cons.CreatedAt,
		},
	})
}

// List 消费者列表
func (h *ConsumerHandler) List(c *gin.Context) {
	filter := consumer.ListFilter{
		Page:     intQuery(c, "page", 1),
		PageSize: intQuery(c, "page_size", 20),
		Status:   c.Query("status"),
		Name:     c.Query("name"),
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

// GetById 获取消费者详情
func (h *ConsumerHandler) GetById(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_id", err.Error()))
		return
	}

	cons, err := h.svc.GetById(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, errorResponse("not_found", "consumer not found"))
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": cons})
}

// Update 更新消费者
func (h *ConsumerHandler) Update(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_id", err.Error()))
		return
	}

	var req struct {
		Name string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_request", err.Error()))
		return
	}

	if err := h.svc.Update(c.Request.Context(), id, req.Name); err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("internal_error", err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{"id": id, "name": req.Name}})
}

// Delete 删除消费者
func (h *ConsumerHandler) Delete(c *gin.Context) {
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

// UpdateStatus 更新消费者状态
func (h *ConsumerHandler) UpdateStatus(c *gin.Context) {
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
		c.JSON(http.StatusBadRequest, errorResponse("invalid_status", err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{"id": id, "status": req.Status}})
}

// ResetKey 重置密钥
func (h *ConsumerHandler) ResetKey(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_id", err.Error()))
		return
	}

	apiKey, err := h.svc.ResetKey(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("internal_error", err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{"id": id, "api_key": apiKey}})
}

// RevealKey 查看密钥（审计）
func (h *ConsumerHandler) RevealKey(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_id", err.Error()))
		return
	}

	apiKey, err := h.svc.RevealKey(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusForbidden, errorResponse("reveal_forbidden", err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{"id": id, "api_key": apiKey}})
}

