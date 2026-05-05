package proxy

import "encoding/json"

// RetryChainEntry 重试链条目
type RetryChainEntry struct {
	ChannelID uint   `json:"channel_id"`
	AccountID uint   `json:"account_id"`
	Error      string `json:"error,omitempty"`
	Result     string `json:"result,omitempty"`
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
	}
	rc.Entries = append(rc.Entries, entry)
	return &rc.Entries[len(rc.Entries)-1]
}

// MarkSuccess 标记最后一次尝试成功
func (rc *RetryChain) MarkSuccess() {
	if len(rc.Entries) > 0 {
		rc.Entries[len(rc.Entries)-1].Result = "success"
	}
}

// MarkError 标记最后一次尝试失败
func (rc *RetryChain) MarkError(err string) {
	if len(rc.Entries) > 0 {
		rc.Entries[len(rc.Entries)-1].Error = err
		rc.Entries[len(rc.Entries)-1].Result = "failed"
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
