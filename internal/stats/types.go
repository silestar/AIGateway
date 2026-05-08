package stats

import (
	"context"
	"encoding/json"
	"time"
)

// RequestLog 请求日志模型
type RequestLog struct {
	ID               uint            `gorm:"primaryKey;autoIncrement" json:"id"`
	Timestamp        time.Time       `gorm:"index;not null" json:"timestamp"`
	KeysID           uint            `gorm:"index" json:"keys_id"`
	ModelName        string          `gorm:"size:100;index" json:"model_name"`
	ChannelID        *uint           `gorm:"index" json:"channel_id"`
	AccountID        *uint           `json:"account_id"`
	RetryChain       json.RawMessage `gorm:"type:json" json:"retry_chain"`
	IsStream         bool            `json:"is_stream"`
	PromptTokens     int             `json:"prompt_tokens"`
	CompletionTokens int             `json:"completion_tokens"`
	StatusCode       int             `json:"status_code"`
	ErrorMsg         *string         `gorm:"type:text" json:"error_msg"`
	LatencyMs        int             `json:"latency_ms"`
	UpstreamLatencyMs int            `gorm:"default:0" json:"upstream_latency_ms"` // 上游处理耗时(ms)
	Cost             float64         `json:"cost"`
	CacheTokens      int             `gorm:"default:0" json:"cache_tokens"`       // 缓存命中Token数
	MappedModel      string          `gorm:"size:100;default:''" json:"mapped_model"` // 映射后请求模型
	UpstreamModel    string          `gorm:"size:100;default:''" json:"upstream_model"` // 上游实际响应模型
	RequestMeta      json.RawMessage `gorm:"type:json" json:"request_meta"`
	ResponseMeta     json.RawMessage `gorm:"type:json" json:"response_meta"`
	LogType          string          `gorm:"size:20;index;default:consumption" json:"log_type"` // consumption/probe/health_check
	TraceID          string          `gorm:"size:64;index" json:"trace_id"`                    // 请求追踪ID
	ClientIP         string          `gorm:"size:45" json:"client_ip"`                         // 客户端IP
}

func (RequestLog) TableName() string { return "request_logs" }

// SystemDailyStats 系统日统计
type SystemDailyStats struct {
	ID              uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	Date            string    `gorm:"size:10;uniqueIndex;not null" json:"date"` // 2026-05-05
	TotalRequests   int       `json:"total_requests"`
	SuccessRequests int       `json:"success_requests"`
	FailRequests    int       `json:"fail_requests"`
	AvgLatencyMs    float64   `json:"avg_latency_ms"`
	TotalTokens     int64     `json:"total_tokens"`
	TotalCost       float64   `json:"total_cost"`
	CreatedAt       time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (SystemDailyStats) TableName() string { return "system_daily_stats" }

// KeysDailyStats 密钥日统计
type KeysDailyStats struct {
	ID              uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	Date            string `gorm:"size:10;not null" json:"date"`
	KeysID      uint   `gorm:"not null;index" json:"keys_id"`
	ModelName       string `gorm:"size:100;not null" json:"model_name"`
	TotalRequests   int    `json:"total_requests"`
	SuccessRequests int    `json:"success_requests"`
	FailRequests    int    `json:"fail_requests"`
	AvgLatencyMs    float64 `json:"avg_latency_ms"`
	TotalTokens     int64  `json:"total_tokens"`
	TotalCost       float64 `json:"total_cost"`
}

func (KeysDailyStats) TableName() string { return "keys_daily_stats" }

// ChannelDailyStats 渠道日统计
type ChannelDailyStats struct {
	ID              uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	Date            string `gorm:"size:10;not null" json:"date"`
	ChannelID       uint   `gorm:"not null;index" json:"channel_id"`
	ModelName       string `gorm:"size:100;not null" json:"model_name"`
	TotalRequests   int    `json:"total_requests"`
	SuccessRequests int    `json:"success_requests"`
	FailRequests    int    `json:"fail_requests"`
	AvgLatencyMs    float64 `json:"avg_latency_ms"`
	TotalTokens     int64  `json:"total_tokens"`
	TotalCost       float64 `json:"total_cost"`
	ActiveAccounts  int    `json:"active_accounts"`
}

func (ChannelDailyStats) TableName() string { return "channel_daily_stats" }

// RealtimeStats 实时统计
type RealtimeStats struct {
	Date            string `json:"date"`
	TotalRequests   int64  `json:"total_requests"`
	SuccessRequests int64  `json:"success_requests"`
	FailRequests    int64  `json:"fail_requests"`
	AvgLatencyMs    float64 `json:"avg_latency_ms"`
	TotalTokens     int64  `json:"total_tokens"`
	ActiveKeys int64  `json:"active_keys"`
	ActiveChannels  int64  `json:"active_channels"`
}

// OverviewStats 概览统计
type OverviewStats struct {
	Today     RealtimeStats    `json:"today"`
	Last7Days []DailyStatEntry `json:"last_7_days"`
}

// DailyStatEntry 日统计条目
type DailyStatEntry struct {
	Date       string `json:"date"`
	TotalReqs  int64  `json:"total_requests"`
	SuccessReqs int64 `json:"success_requests"`
	FailReqs   int64  `json:"fail_requests"`
}

// StatsManager 统计管理接口
type StatsManager interface {
	RecordRequest(ctx context.Context, log *RequestLog) error
	GetRealtime(ctx context.Context) (*RealtimeStats, error)
	GetOverview(ctx context.Context, days int) (*OverviewStats, error)
	StartAggregator()
}
