package api
import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/bokelife/aigateway/internal/account"
)

// AccountHandler 账号管理 API
type AccountHandler struct {
	svc account.AccountManager
}

func NewAccountHandler(svc account.AccountManager) *AccountHandler {
	return &AccountHandler{svc: svc}
}

// RegisterRoutes 注册账号路由
func (h *AccountHandler) RegisterRoutes(rg *gin.RouterGroup) {
	accounts := rg.Group("/accounts")
	accounts.POST("", h.Create)
	accounts.GET("/:id", h.GetById)
	accounts.GET("/channel/:channel_id", h.ListByChannel)
	accounts.PUT("/:id/priority", h.UpdatePriority)
	accounts.PUT("/:id/status", h.UpdateStatus)
	accounts.POST("/:id/reveal-key", h.RevealKey)
	accounts.DELETE("/:id", h.Delete)
}

// Create 创建账号
func (h *AccountHandler) Create(c *gin.Context) {
	var req struct {
		ChannelID uint   `json:"channel_id" binding:"required"`
		APIKey    string `json:"api_key" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_request", err.Error()))
		return
	}

	acc, err := h.svc.Create(c.Request.Context(), req.ChannelID, req.APIKey)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("create_failed", err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"id":        acc.ID,
		"channel_id": acc.ChannelID,
		"status":    acc.Status,
		"priority":  acc.Priority,
	}})
}

// GetById 获取账号详情
func (h *AccountHandler) GetById(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_id", err.Error()))
		return
	}

	acc, err := h.svc.GetById(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, errorResponse("not_found", "account not found"))
		return
	}

	// 使用 APIKeyPrefix 脱敏展示
	maskedKey := maskKey(acc.APIKeyPrefix)

	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"id":            acc.ID,
		"channel_id":     acc.ChannelID,
		"status":        acc.Status,
		"priority":      acc.Priority,
		"api_key_mask":  maskedKey,
		"created_at":    acc.CreatedAt,
		"updated_at":    acc.UpdatedAt,
	}})
}

// ListByChannel 查询渠道下所有账号
func (h *AccountHandler) ListByChannel(c *gin.Context) {
	channelID, err := parseIDFromParam(c, "channel_id")
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_channel_id", err.Error()))
		return
	}

	accounts, err := h.svc.ListByChannel(c.Request.Context(), channelID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("internal_error", err.Error()))
		return
	}

	// 批量脱敏
	result := make([]gin.H, len(accounts))
	for i, acc := range accounts {
		result[i] = gin.H{
			"id":           acc.ID,
			"channel_id":    acc.ChannelID,
			"status":       acc.Status,
			"priority":     acc.Priority,
			"api_key_mask": maskKey(acc.APIKeyPrefix),
		}
	}

	c.JSON(http.StatusOK, gin.H{"data": result})
}

// UpdatePriority 更新账号优先级
func (h *AccountHandler) UpdatePriority(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_id", err.Error()))
		return
	}

	var req struct {
		Priority int `json:"priority" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_request", err.Error()))
		return
	}

	if err := h.svc.UpdatePriority(c.Request.Context(), id, req.Priority); err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("internal_error", err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{"id": id, "priority": req.Priority}})
}

// UpdateStatus 更新账号状态
func (h *AccountHandler) UpdateStatus(c *gin.Context) {
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
		c.JSON(http.StatusBadRequest, errorResponse("update_failed", err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{"id": id, "status": req.Status}})
}

// RevealKey 查看账号密钥（审计）
func (h *AccountHandler) RevealKey(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_id", err.Error()))
		return
	}

	apiKey, err := h.svc.RevealKey(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("reveal_failed", err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{"id": id, "api_key": apiKey}})
}

// Delete 删除账号
func (h *AccountHandler) Delete(c *gin.Context) {
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

