package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/bokelife/aigateway/internal/models"
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
	m.GET("/catalog", h.ListCatalog)
	m.PUT("/catalog/:id/visibility", h.UpdateVisibility)
}

// ListCatalog 获取模型目录（管理端）
func (h *ModelHandler) ListCatalog(c *gin.Context) {
	list, err := h.catalogSvc.ListCatalog(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("list_failed", err.Error()))
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": list})
}

// UpdateVisibility 切换模型可见性
func (h *ModelHandler) UpdateVisibility(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_id", "invalid model id"))
		return
	}
	var req struct {
		Visible bool `json:"visible"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_body", err.Error()))
		return
	}
	if err := h.catalogSvc.UpdateVisibility(c.Request.Context(), id, req.Visible); err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("update_failed", err.Error()))
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"id": id, "visible": req.Visible}})
}
