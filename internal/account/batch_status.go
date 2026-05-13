package account

import "sync"

// BatchResult 单个账号的批量操作结果
type BatchResult struct {
	AccountID uint   `json:"account_id"`
	Success   bool   `json:"success"`
	Error     string `json:"error,omitempty"`
}

// BatchStatus 批量操作进度
type BatchStatus struct {
	Total   int           `json:"total"`
	Done    int           `json:"done"`
	Running bool          `json:"running"`
	Results []BatchResult `json:"results"`
}

// BatchProgress 批量操作进度管理器（内存，不持久化）
type BatchProgress struct {
	mu     sync.Mutex
	status map[uint]*BatchStatus
}

var batchProgress = &BatchProgress{
	status: make(map[uint]*BatchStatus),
}

// Start 开始一个批量操作
func (bp *BatchProgress) Start(channelID uint, total int) {
	bp.mu.Lock()
	defer bp.mu.Unlock()
	bp.status[channelID] = &BatchStatus{
		Total:   total,
		Done:    0,
		Running: true,
		Results: make([]BatchResult, 0, total),
	}
}

// AddResult 记录一个账号的结果
func (bp *BatchProgress) AddResult(channelID uint, result BatchResult) {
	bp.mu.Lock()
	defer bp.mu.Unlock()
	if s, ok := bp.status[channelID]; ok && s.Running {
		s.Done++
		s.Results = append(s.Results, result)
	}
}

// Finish 标记批量操作完成
func (bp *BatchProgress) Finish(channelID uint) {
	bp.mu.Lock()
	defer bp.mu.Unlock()
	if s, ok := bp.status[channelID]; ok {
		s.Running = false
	}
}

// GetStatus 查询批量操作进度
func (bp *BatchProgress) GetStatus(channelID uint) *BatchStatus {
	bp.mu.Lock()
	defer bp.mu.Unlock()
	s, ok := bp.status[channelID]
	if !ok {
		return nil
	}
	// 返回副本避免并发读写
	cp := *s
	cp.Results = make([]BatchResult, len(s.Results))
	copy(cp.Results, s.Results)
	return &cp
}

// GetGlobalProgress 获取全局进度实例
func GetBatchProgress() *BatchProgress {
	return batchProgress
}