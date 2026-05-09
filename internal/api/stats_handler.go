package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/bokelife/aigateway/internal/stats"
)

// StatsHandler 统计 API
type StatsHandler struct {
	statsMgr *stats.Manager
}

// NewStatsHandler 创建统计 Handler
func NewStatsHandler(statsMgr *stats.Manager) *StatsHandler {
	return &StatsHandler{statsMgr: statsMgr}
}

// RegisterRoutes 注册统计路由
func (h *StatsHandler) RegisterRoutes(rg *gin.RouterGroup) {
	s := rg.Group("/stats")
	s.GET("/dashboard", h.Dashboard)
	s.GET("/realtime", h.Realtime)
	s.GET("/requests", h.Requests)
	s.GET("/models", h.Models)
	s.GET("/channels", h.Channels)
	s.GET("/keys/:id", h.KeysStats)
	s.GET("/channel/:id", h.ChannelStats)
	// 实时聚合（从 request_logs 直接查询）
	s.GET("/keys-realtime/:id", h.KeysRealtime)
	s.GET("/channel-realtime/:id", h.ChannelRealtime)
}

// Dashboard 仪表盘概览
func (h *StatsHandler) Dashboard(c *gin.Context) {
	days := intQuery(c, "days", 7)
	hours := days * 24 // 趋势图按天折算小时数

	// 1. 今日概览
	overview, err := h.statsMgr.GetOverview(c.Request.Context(), days)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("internal_error", err.Error()))
		return
	}

	today := overview.Today
	successRate := 0.0
	if today.TotalRequests > 0 {
		successRate = float64(today.SuccessRequests) / float64(today.TotalRequests) * 100
	}

	// 2. 小时趋势
	hourlyTrend, _ := h.statsMgr.GetHourlyTrend(c.Request.Context(), hours)

	// 3. Top 模型
	topModels, _ := h.statsMgr.GetTopModels(c.Request.Context(), 5)

	// 4. Top 渠道
	topChannels, _ := h.statsMgr.GetTopChannels(c.Request.Context(), 10)

	// 5. 最近异常
	recentErrors, _ := h.statsMgr.GetRecentErrors(c.Request.Context(), 5)

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"date":                today.Date,
			"total_requests_today": today.TotalRequests,
			"success_rate":         successRate,
			"avg_latency_ms":      today.AvgLatencyMs,
			"total_tokens":        today.TotalTokens,
			"active_keys":         today.ActiveKeys,
			"active_channels":     today.ActiveChannels,
			"last_7_days":         overview.Last7Days,
			"hourly_trend":        hourlyTrend,
			"top_models":          topModels,
			"top_channels":        topChannels,
			"recent_errors":       recentErrors,
		},
	})
}

// Realtime 实时统计
func (h *StatsHandler) Realtime(c *gin.Context) {
	realtime, err := h.statsMgr.GetRealtime(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("internal_error", err.Error()))
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": realtime})
}

// Requests 请求统计
func (h *StatsHandler) Requests(c *gin.Context) {
	start := c.Query("start")
	end := c.Query("end")
	granularity := c.DefaultQuery("granularity", "daily")

	// 从 system_daily_stats 查询历史数据
	overview, err := h.statsMgr.GetOverview(c.Request.Context(), 30)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("internal_error", err.Error()))
		return
	}

	_ = start
	_ = end
	_ = granularity

	c.JSON(http.StatusOK, gin.H{
		"data":  overview.Last7Days,
		"total": len(overview.Last7Days),
	})
}

// Models 模型请求分布
func (h *StatsHandler) Models(c *gin.Context) {
	// 从消费者日统计中聚合模型分布
	var modelStats []struct {
		ModelName     string `json:"model_name"`
		TotalRequests int64  `json:"total_requests"`
	}
	h.statsMgr.DB().WithContext(c.Request.Context()).
		Model(&stats.KeysDailyStats{}).
		Select("model_name, SUM(total_requests) as total_requests").
		Group("model_name").
		Order("total_requests DESC").
		Scan(&modelStats)

	c.JSON(http.StatusOK, gin.H{"data": modelStats})
}

// Channels 渠道负载排行
func (h *StatsHandler) Channels(c *gin.Context) {
	var channelStats []struct {
		ChannelID     uint   `json:"channel_id"`
		TotalRequests int64  `json:"total_requests"`
		SuccessRate   float64 `json:"success_rate"`
		AvgLatencyMs  int    `json:"avg_latency_ms"`
	}
	h.statsMgr.DB().WithContext(c.Request.Context()).
		Model(&stats.ChannelDailyStats{}).
		Select("channel_id, SUM(total_requests) as total_requests, CASE WHEN SUM(total_requests) > 0 THEN CAST(SUM(success_requests) AS REAL) / SUM(total_requests) * 100 ELSE 0 END as success_rate, CASE WHEN SUM(total_requests) > 0 THEN SUM(avg_latency_ms * total_requests) / SUM(total_requests) ELSE 0 END as avg_latency_ms").
		Group("channel_id").
		Order("total_requests DESC").
		Scan(&channelStats)

	c.JSON(http.StatusOK, gin.H{"data": channelStats})
}

// KeysStats 消费者统计详情
func (h *StatsHandler) KeysStats(c *gin.Context) {
	keysID, err := parseID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_keys_id", "invalid keys id"))
		return
	}

	start := c.DefaultQuery("start", "")
	end := c.DefaultQuery("end", "")

	statsData, err := h.statsMgr.GetKeysStats(c.Request.Context(), keysID, start, end)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("internal_error", err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": statsData})
}

// ChannelStats 渠道统计详情
func (h *StatsHandler) ChannelStats(c *gin.Context) {
	channelID, err := parseID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_channel_id", "invalid channel id"))
		return
	}

	start := c.DefaultQuery("start", "")
	end := c.DefaultQuery("end", "")

	statsData, err := h.statsMgr.GetChannelStats(c.Request.Context(), channelID, start, end)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("internal_error", err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": statsData})
}

// KeysRealtime 密钥实时聚合统计（从 request_logs 直接查询）
func (h *StatsHandler) KeysRealtime(c *gin.Context) {
	keysID, err := parseID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_keys_id", "invalid keys id"))
		return
	}
	stats, err := h.statsMgr.GetConsumerRealtime(c.Request.Context(), keysID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("internal_error", err.Error()))
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": stats})
}

// ChannelRealtime 渠道实时聚合统计（从 request_logs 直接查询）
func (h *StatsHandler) ChannelRealtime(c *gin.Context) {
	channelID, err := parseID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_channel_id", "invalid channel id"))
		return
	}
	stats, err := h.statsMgr.GetChannelRealtime(c.Request.Context(), channelID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("internal_error", err.Error()))
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": stats})
}
