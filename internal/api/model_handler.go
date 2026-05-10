package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/silestar/AIGateway/internal/models"
)

// ModelHandler 模型管理 API
type ModelHandler struct {
	catalogSvc models.CatalogService
}

// NewModelHandler 创建模型管理 Handler
func NewModelHandler(catalogSvc models.CatalogService) *ModelHandler {
	return &ModelHandler{catalogSvc: catalogSvc}
}

// RegisterRoutes 注册模型管理路由
func (h *ModelHandler) RegisterRoutes(rg *gin.RouterGroup) {
	m := rg.Group("/models")
	m.GET("/list", h.ListModels)
	m.PUT("/upstream/visible", h.SetUpstreamVisible)
	m.PUT("/display/visible", h.SetDisplayVisible)
}

// ListModels 获取模型列表（双列）
func (h *ModelHandler) ListModels(c *gin.Context) {
	upstream, err := h.catalogSvc.GetUpstreamModels(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("list_upstream_failed", err.Error()))
		return
	}
	display, err := h.catalogSvc.GetDisplayModels(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("list_display_failed", err.Error()))
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"upstream": upstream,
			"display":  display,
		},
	})
}

// SetUpstreamVisible 设置上游模型可见性
func (h *ModelHandler) SetUpstreamVisible(c *gin.Context) {
	var req struct {
		ModelName string `json:"model_name" binding:"required"`
		Visible   bool   `json:"visible"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_body", err.Error()))
		return
	}
	if err := h.catalogSvc.BatchSetUpstreamVisible(c.Request.Context(), req.ModelName, req.Visible); err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("update_failed", err.Error()))
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"model_name": req.ModelName, "upstream_visible": req.Visible}})
}

// SetDisplayVisible 设置映射模型可见性
func (h *ModelHandler) SetDisplayVisible(c *gin.Context) {
	var req struct {
		ModelName string `json:"model_name" binding:"required"`
		Visible   bool   `json:"visible"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_body", err.Error()))
		return
	}
	if err := h.catalogSvc.BatchSetDisplayVisible(c.Request.Context(), req.ModelName, req.Visible); err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("update_failed", err.Error()))
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"model_name": req.ModelName, "display_visible": req.Visible}})
}