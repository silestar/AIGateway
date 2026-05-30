package api
import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/silestar/AIGateway/internal/account"
)

// AccountHandler 账号管理 API
type AccountHandler struct {
	svc   account.AccountManager
	cache account.Cache
}

func NewAccountHandler(svc account.AccountManager, cache account.Cache) *AccountHandler {
	return &AccountHandler{svc: svc, cache: cache}
}

// RegisterRoutes 注册账号路由
func (h *AccountHandler) RegisterRoutes(rg *gin.RouterGroup) {
	accounts := rg.Group("/accounts")
	accounts.POST("", h.Create)
	accounts.GET("/:id", h.GetById)
	accounts.GET("/channel/:channel_id", h.ListByChannel)
	accounts.PUT("/:id/priority", h.UpdatePriority)
	accounts.PUT("/:id/status", h.UpdateStatus)
	accounts.PATCH("/:id/remark", h.UpdateRemark)
	accounts.POST("/:id/reveal-key", h.RevealKey)
	accounts.DELETE("/:id", h.Delete)
}

// Create 创建账号
func (h *AccountHandler) Create(c *gin.Context) {
	var req struct {
		ChannelID uint   `json:"channel_id" binding:"required"`
		APIKey    string `json:"api_key" binding:"required"`
		Remark    string `json:"remark"`
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

	// 更新备注
	if req.Remark != "" {
		_ = h.svc.UpdateRemark(c.Request.Context(), acc.ID, req.Remark)
		acc.Remark = req.Remark
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"id":         acc.ID,
		"channel_id": acc.ChannelID,
		"status":     acc.Status,
		"priority":   acc.Priority,
		"remark":     acc.Remark,
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

	// 批量脱敏 + 速率限制用量
	minuteKey := time.Now().Format("2006-01-02-15:04")
	todayKey := time.Now().Format("2006-01-02")
	result := make([]gin.H, len(accounts))
	for i, acc := range accounts {
		rpmUsed, tpmUsed, dailyUsed := 0, 0, 0
		if rpmStr, err := h.cache.Get(fmt.Sprintf("stats:account:%d:rpm:%s", acc.ID, minuteKey)); err == nil {
			fmt.Sscanf(rpmStr, "%d", &rpmUsed)
		}
		if tpmStr, err := h.cache.Get(fmt.Sprintf("stats:account:%d:tpm:%s", acc.ID, minuteKey)); err == nil {
			fmt.Sscanf(tpmStr, "%d", &tpmUsed)
		}
		if dailyStr, err := h.cache.Get(fmt.Sprintf("stats:account:%d:daily_requests:%s", acc.ID, todayKey)); err == nil {
			fmt.Sscanf(dailyStr, "%d", &dailyUsed)
		}
result[i] = gin.H{
			"id":                    acc.ID,
			"channel_id":            acc.ChannelID,
			"status":                acc.Status,
			"priority":              acc.Priority,
			"api_key_mask":          maskKey(acc.APIKeyPrefix),
			"remark":                acc.Remark,
			"disabled_reason":       acc.DisabledReason,
			"probe_cooldown_until":  acc.ProbeCooldownUntil,
			"consecutive_failures":  acc.ConsecutiveFailures,
			"probe_failures":        acc.ProbeFailures,
			"last_failed_at":        acc.LastFailedAt,
			"rate_limit": gin.H{
				"rpm_used":   rpmUsed,
				"rpm_limit":  0, // 占位，前端从渠道配置取
				"tpm_used":   tpmUsed,
				"tpm_limit":  0,
				"daily_used": dailyUsed,
				"daily_limit": 0,
			},
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

// UpdateRemark 更新账号备注
func (h *AccountHandler) UpdateRemark(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_id", err.Error()))
		return
	}

	var req struct {
		Remark string `json:"remark"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_request", err.Error()))
		return
	}

	if err := h.svc.UpdateRemark(c.Request.Context(), id, req.Remark); err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("internal_error", err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{"id": id, "remark": req.Remark}})
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

