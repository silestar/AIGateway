package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/bokelife/aigateway/internal/keys"
)

// KeysHandler 密钥管理 API
type KeysHandler struct {
	svc keys.KeysService
}

func NewKeysHandler(svc keys.KeysService) *KeysHandler {
	return &KeysHandler{svc: svc}
}

// RegisterRoutes 注册密钥路由
func (h *KeysHandler) RegisterRoutes(rg *gin.RouterGroup) {
	keysGroup := rg.Group("/keys")
	keysGroup.GET("", h.List)
	keysGroup.POST("", h.Create)
	keysGroup.GET("/:id", h.GetById)
	keysGroup.PUT("/:id", h.Update)
	keysGroup.DELETE("/:id", h.Delete)
	keysGroup.PUT("/:id/status", h.UpdateStatus)
	keysGroup.POST("/:id/reset-key", h.ResetKey)
	keysGroup.POST("/:id/reveal-key", h.RevealKey)
}

// List 获取密钥列表
func (h *KeysHandler) List(c *gin.Context) {
	var filter keys.ListFilter
	if err := c.ShouldBindQuery(&filter); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_request", err.Error()))
		return
	}

	keysList, total, err := h.svc.List(c.Request.Context(), filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("internal_error", err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":      keysList,
		"total":     total,
		"page":      filter.Page,
		"page_size": filter.PageSize,
	})
}

// Create 创建密钥
func (h *KeysHandler) Create(c *gin.Context) {
	var req struct {
		Name string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_request", err.Error()))
		return
	}

	k, apiKey, err := h.svc.Create(c.Request.Context(), req.Name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("internal_error", err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"id":         k.ID,
			"name":       k.Name,
			"status":     k.Status,
			"api_key":    apiKey,
			"created_at": k.CreatedAt,
		},
	})
}

// GetById 获取密钥详情
func (h *KeysHandler) GetById(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_id", err.Error()))
		return
	}

	k, err := h.svc.GetById(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, errorResponse("not_found", "keys not found"))
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": k})
}

// Update 更新密钥
func (h *KeysHandler) Update(c *gin.Context) {
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

	c.JSON(http.StatusOK, gin.H{"data": gin.H{"id": id}})
}

// Delete 删除密钥
func (h *KeysHandler) Delete(c *gin.Context) {
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

// UpdateStatus 更新密钥状态
func (h *KeysHandler) UpdateStatus(c *gin.Context) {
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

// ResetKey 重置密钥
func (h *KeysHandler) ResetKey(c *gin.Context) {
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

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{"id": id, "api_key": apiKey},
	})
}

// RevealKey 查看密钥
func (h *KeysHandler) RevealKey(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_id", err.Error()))
		return
	}

	apiKey, err := h.svc.RevealKey(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, errorResponse("not_available", err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{"id": id, "api_key": apiKey},
	})
}