package api

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/silestar/AIGateway/internal/account"
	"github.com/silestar/AIGateway/internal/channel"
	"github.com/silestar/AIGateway/internal/stats"
)

// ChannelHandler 渠道管理 API
type ChannelHandler struct {
	svc         channel.ChannelService
	accountMgr  account.AccountManager
	asyncWriter *stats.AsyncWriter
}

func NewChannelHandler(svc channel.ChannelService, accountMgr account.AccountManager, asyncWriter *stats.AsyncWriter) *ChannelHandler {
	return &ChannelHandler{svc: svc, accountMgr: accountMgr, asyncWriter: asyncWriter}
}

// RegisterRoutes 注册渠道路由
func (h *ChannelHandler) RegisterRoutes(rg *gin.RouterGroup) {
	channels := rg.Group("/channels")
	channels.GET("", h.List)
	channels.POST("", h.Create)
	channels.GET("/custom-model-names", h.GetCustomModelNames) // 必须在 /:id 之前
	channels.GET("/:id", h.GetById)
	channels.PUT("/:id", h.Update)
	channels.DELETE("/:id", h.Delete)
	channels.PATCH("/:id/status", h.UpdateStatus)
	channels.PATCH("/:id/weight", h.UpdateWeight)
	channels.POST("/test-connection", h.TestConnection)
	channels.POST("/:id/fetch-models", h.FetchModels)
	channels.GET("/:id/models", h.GetModelsByChannel)
	channels.PUT("/:id/models", h.SaveModels)
	// 新增端点
	channels.POST("/:id/test", h.TestChannel)
	channels.POST("/:id/test-models", h.BatchTestModels)
	channels.PUT("/:id/test-model", h.UpdateTestModel)
	channels.POST("/:id/copy", h.CopyChannel)
	// 账号测试与恢复（渠道维度）
	channels.POST("/:id/accounts/:accountId/test", h.TestAccount)
	channels.POST("/:id/accounts/batch-recover", h.BatchRecoverAccounts)
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
		Page:      intQuery(c, "page", 1),
		PageSize:  intQuery(c, "page_size", 20),
		Status:   c.Query("status"),
		Type:      c.Query("type"),
		Search:    c.Query("search"),
		SortBy:    c.Query("sort_by"),
		SortOrder: c.Query("sort_order"),
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
		Name             string `json:"name"`
		BaseURL          string `json:"base_url"`
		Weight           int    `json:"weight"`
		MaxRPM           int    `json:"max_rpm"`
		MaxTPM           int    `json:"max_tpm"`
		MaxDailyRequests int    `json:"max_daily_requests"`
		TestModel        string `json:"test_model"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_request", err.Error()))
		return
	}

	if err := h.svc.Update(c.Request.Context(), id, req.Name, req.BaseURL, req.Weight, req.MaxRPM, req.MaxTPM, req.MaxDailyRequests); err != nil {
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

// TestChannel 测试渠道可用性
func (h *ChannelHandler) TestChannel(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_id", err.Error()))
		return
	}

	// 获取活跃账号的 API Key
	apiKey, err := h.getActiveAPIKey(c, id)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("no_active_account", err.Error()))
		return
	}

	result, err := h.svc.TestChannel(c.Request.Context(), id, apiKey)
	if err != nil {
		// 记录 health_check 失败日志
		errMsg := err.Error()
		traceID, _ := c.Get("trace_id")
		traceIDStr, _ := traceID.(string)
		h.asyncWriter.Record(&stats.RequestLog{
			Timestamp:  time.Now(),
			ChannelID:  &id,
			ModelName:  "test",
			StatusCode: 0,
			LatencyMs:  0,
			LogType:    "health_check",
			TraceID:    traceIDStr,
			ClientIP:   c.ClientIP(),
			ErrorMsg:   &errMsg,
		})
		c.JSON(http.StatusBadRequest, errorResponse("test_failed", err.Error()))
		return
	}

	// 记录 health_check 成功日志
	chID := id
	traceID, _ := c.Get("trace_id")
	traceIDStr, _ := traceID.(string)
	h.asyncWriter.Record(&stats.RequestLog{
		Timestamp:       time.Now(),
		ChannelID:       &chID,
		ModelName:       result.Model,
		StatusCode:      result.Status,
		LatencyMs:       result.Latency,
		LogType:         "health_check",
		TraceID:         traceIDStr,
		ClientIP:        c.ClientIP(),
		PromptTokens:    result.PromptTokens,
		CompletionTokens: result.CompletionTokens,
	})

	c.JSON(http.StatusOK, gin.H{"data": result})
}

// BatchTestModels 批量测试模型
func (h *ChannelHandler) BatchTestModels(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_id", err.Error()))
		return
	}

	var req struct {
		Models []string `json:"models" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_request", err.Error()))
		return
	}

	apiKey, err := h.getActiveAPIKey(c, id)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("no_active_account", err.Error()))
		return
	}

	results, err := h.svc.BatchTestModels(c.Request.Context(), id, req.Models, apiKey)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("test_failed", err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": results})
}

// UpdateTestModel 更新渠道指定测试模型
func (h *ChannelHandler) UpdateTestModel(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_id", err.Error()))
		return
	}

	var req struct {
		TestModel string `json:"test_model"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_request", err.Error()))
		return
	}

	if err := h.svc.UpdateTestModel(c.Request.Context(), id, req.TestModel); err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("internal_error", err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{"id": id, "test_model": req.TestModel}})
}

// GetCustomModelNames 获取所有渠道已配置的自定义模型名（用于前端自动补全）
func (h *ChannelHandler) GetCustomModelNames(c *gin.Context) {
	names, err := h.svc.GetCustomModelNames(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("internal_error", err.Error()))
		return
	}
	if names == nil {
		names = []string{}
	}
	c.JSON(http.StatusOK, gin.H{"data": names})
}

// CopyChannel 复制渠道
func (h *ChannelHandler) CopyChannel(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_id", err.Error()))
		return
	}

	newCh, err := h.svc.CopyChannel(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("copy_failed", err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":    newCh,
		"message": "渠道已复制，请前往新渠道配置账号",
	})
}

// getActiveAPIKey 获取渠道下优先级最高的 active 账号的 API Key
func (h *ChannelHandler) getActiveAPIKey(c *gin.Context, channelID uint) (string, error) {
	accounts, err := h.accountMgr.ListByChannel(c.Request.Context(), channelID)
	if err != nil || len(accounts) == 0 {
		return "", fmt.Errorf("该渠道没有可用账号，请先添加账号")
	}

	// 找优先级最高的 active 账号
	var activeAccount *account.Account
	for i := range accounts {
		if accounts[i].Status == "active" {
			if activeAccount == nil || accounts[i].Priority > activeAccount.Priority {
				activeAccount = &accounts[i]
			}
		}
	}
	if activeAccount == nil {
		return "", fmt.Errorf("该渠道没有活跃账号，请先启用账号")
	}

	plainKey, err := h.accountMgr.GetDecryptedAPIKey(c.Request.Context(), activeAccount.ID)
	if err != nil {
		return "", fmt.Errorf("获取 API Key 失败: %w", err)
	}

	return plainKey, nil
}

// TestAccount 手动测试单个账号
func (h *ChannelHandler) TestAccount(c *gin.Context) {
	channelID, err := parseID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	accountID, err := parseIDFromParam(c, "accountId")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result, err := h.accountMgr.TestAccount(c.Request.Context(), channelID, accountID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

// BatchRecoverAccounts 批量恢复渠道下所有 disabled 账号
func (h *ChannelHandler) BatchRecoverAccounts(c *gin.Context) {
	channelID, err := parseID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	results, err := h.accountMgr.BatchRecover(c.Request.Context(), channelID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"results": results})
}