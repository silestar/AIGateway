package proxy

import (
	"encoding/json"
	"time"

	"github.com/bokelife/aigateway/pkg/usage"
)

// RetryChainEntry 重试链条目
type RetryChainEntry struct {
	ChannelID uint   `json:"channel_id"`
	AccountID uint   `json:"account_id"`
	Error     string `json:"error,omitempty"`
	Result    string `json:"result,omitempty"`
	StartedAt string `json:"started_at,omitempty"`  // ISO8601 时间戳
	LatencyMs int    `json:"latency_ms,omitempty"` // 本次尝试耗时
	ErrorCode int    `json:"status_code,omitempty"` // HTTP 状态码
}

// RetryChain 重试链
type RetryChain struct {
	Entries []RetryChainEntry `json:"entries"`
}

// NewRetryChain 创建重试链
func NewRetryChain() *RetryChain {
	return &RetryChain{Entries: make([]RetryChainEntry, 0)}
}

// AddAttempt 添加一次尝试记录
func (rc *RetryChain) AddAttempt(channelID, accountID uint) *RetryChainEntry {
	entry := RetryChainEntry{
		ChannelID: channelID,
		AccountID: accountID,
		StartedAt: time.Now().Format(time.RFC3339),
	}
	rc.Entries = append(rc.Entries, entry)
	return &rc.Entries[len(rc.Entries)-1]
}

// MarkSuccess 标记最后一次尝试成功
func (rc *RetryChain) MarkSuccess(latencyMs int, statusCode int) {
	if len(rc.Entries) > 0 {
		rc.Entries[len(rc.Entries)-1].Result = "success"
		rc.Entries[len(rc.Entries)-1].LatencyMs = latencyMs
		rc.Entries[len(rc.Entries)-1].ErrorCode = statusCode
	}
}

// MarkError 标记最后一次尝试失败
func (rc *RetryChain) MarkError(err string, latencyMs int, statusCode int) {
	if len(rc.Entries) > 0 {
		rc.Entries[len(rc.Entries)-1].Error = err
		rc.Entries[len(rc.Entries)-1].Result = "failed"
		rc.Entries[len(rc.Entries)-1].LatencyMs = latencyMs
		rc.Entries[len(rc.Entries)-1].ErrorCode = statusCode
	}
}

// ToJSON 序列化为 JSON
func (rc *RetryChain) ToJSON() json.RawMessage {
	data, _ := json.Marshal(rc.Entries)
	return json.RawMessage(data)
}

// Len 返回尝试次数
func (rc *RetryChain) Len() int {
	return len(rc.Entries)
}

// ProxyResult 代理结果（包含响应体和 token 用量）
type ProxyResult struct {
	StatusCode int
	Body       []byte
	Headers    map[string][]string
	Usage      *usage.TokenUsage
	// 响应摘要（成功时填充）
	ResponseModel         string `json:"response_model,omitempty"`
	FinishReason          string `json:"finish_reason,omitempty"`
	SystemFingerprint     string `json:"system_fingerprint,omitempty"`
	UpstreamLatencyMs     int    `json:"upstream_latency_ms,omitempty"` // 上游处理耗时(ms)
}

