package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/bokelife/aigateway/internal/config"
	agwlog "github.com/bokelife/aigateway/internal/log"
	"github.com/bokelife/aigateway/internal/stats"
)

// LogHandler 日志 API
type LogHandler struct {
	statsMgr *stats.Manager
	cfg      *config.LogConfig
}

// NewLogHandler 创建日志 Handler
func NewLogHandler(statsMgr *stats.Manager, cfg *config.LogConfig) *LogHandler {
	return &LogHandler{statsMgr: statsMgr, cfg: cfg}
}

// RegisterRoutes 注册日志路由
func (h *LogHandler) RegisterRoutes(rg *gin.RouterGroup) {
	logs := rg.Group("/logs")
	logs.GET("", h.List)
	logs.GET("/stats", h.Stats)
	logs.GET("/channels", h.ListChannels)
	logs.GET("/keys", h.ListKeys)
	logs.GET("/:id", h.GetById)
	logs.GET("/:id/detail", h.GetDetail)
}

// List 请求日志列表
func (h *LogHandler) List(c *gin.Context) {
	filter := stats.LogFilter{
		KeysID:      uint(intQuery(c, "keys_id", 0)),
		KeysName:    c.Query("keys_name"),
		ChannelID:   uint(intQuery(c, "channel_id", 0)),
		ChannelName: c.Query("channel_name"),
		ModelName:   c.Query("model_name"),
		Status:      c.Query("status"),
		LogTypes:    c.Query("log_types"),
		Keyword:     c.Query("keyword"),
		TraceID:     c.Query("trace_id"),
		Start:       c.Query("start"),
		End:         c.Query("end"),
		Page:        intQuery(c, "page", 1),
		PageSize:    intQuery(c, "page_size", 20),
	}

	logs, total, err := h.statsMgr.QueryRequestLogs(c.Request.Context(), filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("internal_error", err.Error()))
		return
	}

	// 填充关联名称
	type LogWithNames struct {
		stats.RequestLog
		ChannelName string `json:"channel_name,omitempty"`
		KeysName    string `json:"keys_name,omitempty"`
		AccountNote string `json:"account_note,omitempty"`
		GroupName   string `json:"group_name,omitempty"`
	}

	result := make([]LogWithNames, 0, len(logs))
	for _, log := range logs {
		item := LogWithNames{RequestLog: log}

		// 查渠道名
		if log.ChannelID != nil && *log.ChannelID > 0 {
			var chName string
			h.statsMgr.DB().Table("channels").Where("id = ?", *log.ChannelID).Select("name").Scan(&chName)
			item.ChannelName = chName
		}

		// 查密钥名
		if log.KeysID > 0 {
			var kName string
			h.statsMgr.DB().Table("keys").Where("id = ?", log.KeysID).Select("name").Scan(&kName)
			item.KeysName = kName
		}

		// 查账号备注
		if log.AccountID != nil && *log.AccountID > 0 {
			var remark string
			h.statsMgr.DB().Table("channel_accounts").Where("id = ?", *log.AccountID).Select("remark").Scan(&remark)
			item.AccountNote = remark
		}

		// 查分组名
		if log.KeysID > 0 {
			var gName string
			h.statsMgr.DB().Table("keys_groups").Select("keys_groups.name").
				Joins("JOIN keys_group_members ON keys_groups.id = keys_group_members.group_id").
				Where("keys_group_members.keys_id = ?", log.KeysID).
				Limit(1).Scan(&gName)
			item.GroupName = gName
		}

		result = append(result, item)
	}

	c.JSON(http.StatusOK, gin.H{
		"data":      result,
		"total":     total,
		"page":      filter.Page,
		"page_size": filter.PageSize,
	})
}

// Stats 请求日志统计
func (h *LogHandler) Stats(c *gin.Context) {
	start := c.Query("start")
	end := c.Query("end")
	logTypes := c.Query("log_types")

	query := h.statsMgr.DB().Table("request_logs")

	if start != "" {
		query = query.Where("timestamp >= ?", start)
	}
	if end != "" {
		query = query.Where("timestamp < ?", end)
	}
	if logTypes != "" {
		query = query.Where("log_type IN ?", splitComma(logTypes))
	}

	type StatsResult struct {
		TotalRequests   int64 `json:"total_requests"`
		SuccessRequests int64 `json:"success_requests"`
		FailedRequests  int64 `json:"failed_requests"`
		AvgLatencyMs    float64 `json:"avg_latency_ms"`
		TotalTokens     int64 `json:"total_tokens"`
	}

	var result StatsResult
	query.Select(
		"COUNT(*) as total_requests, "+
			"SUM(CASE WHEN status_code >= 200 AND status_code < 300 THEN 1 ELSE 0 END) as success_requests, "+
			"SUM(CASE WHEN status_code < 200 OR status_code >= 300 THEN 1 ELSE 0 END) as failed_requests, "+
			"COALESCE(AVG(latency_ms), 0) as avg_latency_ms, "+
			"COALESCE(SUM(prompt_tokens + completion_tokens), 0) as total_tokens",
	).Scan(&result)

	c.JSON(http.StatusOK, gin.H{"data": result})
}

// ListChannels 列出所有渠道（供筛选下拉）
func (h *LogHandler) ListChannels(c *gin.Context) {
	type ChannelOption struct {
		ID   uint   `json:"value"`
		Name string `json:"label"`
	}
	var options []ChannelOption
	h.statsMgr.DB().Table("channels").Where("status = ?", "active").
		Select("id, name").Order("id ASC").Scan(&options)
	c.JSON(http.StatusOK, gin.H{"data": options})
}

// ListKeys 列出所有密钥（供筛选下拉）
func (h *LogHandler) ListKeys(c *gin.Context) {
	type KeysOption struct {
		ID   uint   `json:"value"`
		Name string `json:"label"`
	}
	var options []KeysOption
	h.statsMgr.DB().Table("keys").Where("status = ?", "active").
		Select("id, name").Order("id ASC").Scan(&options)
	c.JSON(http.StatusOK, gin.H{"data": options})
}

// GetById 日志详情
func (h *LogHandler) GetById(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_id", "invalid log id"))
		return
	}

	log, err := h.statsMgr.GetRequestLogByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, errorResponse("not_found", "log not found"))
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": log})
}

// splitComma 逗号分隔字符串
func splitComma(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, ",")
}

// GetDetail 获取请求日志的详细内容文件
func (h *LogHandler) GetDetail(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_id", "invalid log id"))
		return
	}

	log, err := h.statsMgr.GetRequestLogByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, errorResponse("not_found", "log not found"))
		return
	}

	if log.TraceID == "" {
		c.JSON(http.StatusNotFound, errorResponse("not_found", "no trace_id for this log"))
		return
	}

	entry, err := agwlog.ReadDetail(h.cfg.Dir, log.TraceID, log.Timestamp)
	if err != nil {
		c.JSON(http.StatusNotFound, errorResponse("not_found", "detail file not found: "+err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": entry})
}
