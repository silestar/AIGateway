package stats

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// Manager 统计管理器
type Manager struct {
	db        *gorm.DB
	logger    *zap.Logger
	counters  *TodayCounters
	mu        sync.RWMutex
	// 密钥级实时计数
	keysCounters map[uint]*TodayCounters
	// 渠道级实时计数
	channelCounters  map[uint]*TodayCounters
	// 聚合调度器
	aggregatorCancel context.CancelFunc
}

// NewManager 创建统计管理器
func NewManager(db *gorm.DB, logger *zap.Logger) *Manager {
	return &Manager{
		db:               db,
		logger:           logger,
		counters:         NewTodayCounters(),
		keysCounters: make(map[uint]*TodayCounters),
		channelCounters:  make(map[uint]*TodayCounters),
	}
}

// DB 返回数据库实例（供 handler 直接查询）
func (m *Manager) DB() *gorm.DB {
	return m.db
}

// StartAggregator 启动日聚合调度器（每 5 分钟一次）
func (m *Manager) StartAggregator() {
	ctx, cancel := context.WithCancel(context.Background())
	m.aggregatorCancel = cancel

	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()

		// 启动时立即执行一次
		m.runAggregation(ctx)

		for {
			select {
			case <-ticker.C:
				m.runAggregation(ctx)
			case <-ctx.Done():
				m.logger.Info("aggregator stopped")
				return
			}
		}
	}()

	m.logger.Info("stats aggregator started (interval: 5m)")
}

// StopAggregator 停止聚合调度器
func (m *Manager) StopAggregator() {
	if m.aggregatorCancel != nil {
		m.aggregatorCancel()
	}
}

// IncrementCounters 递增实时计数器（仅消费类日志）
func (m *Manager) IncrementCounters(log *RequestLog) {
	// 只统计消费类日志，排除探测和健康检查
	if log.LogType != "" && log.LogType != "consumption" {
		return
	}
	success := log.StatusCode >= 200 && log.StatusCode < 300
	tokens := log.PromptTokens + log.CompletionTokens
	m.counters.Increment(success, log.LatencyMs, tokens)

	// 密钥级计数
	m.mu.Lock()
	cc, ok := m.keysCounters[log.KeysID]
	if !ok {
		cc = NewTodayCounters()
		m.keysCounters[log.KeysID] = cc
	}
	m.mu.Unlock()
	cc.Increment(success, log.LatencyMs, tokens)

	// 渠道级计数
	if log.ChannelID != nil {
		m.mu.Lock()
		chc, ok := m.channelCounters[*log.ChannelID]
		if !ok {
			chc = NewTodayCounters()
			m.channelCounters[*log.ChannelID] = chc
		}
		m.mu.Unlock()
		chc.Increment(success, log.LatencyMs, tokens)
	}
}

// GetRealtime 获取今日实时统计
func (m *Manager) GetRealtime(ctx context.Context) (*RealtimeStats, error) {
	date, total, success, fail, avgLatency, tokens := m.counters.Snapshot()

	// 活跃密钥和渠道数量
	m.mu.RLock()
	activeKeys := int64(len(m.keysCounters))
	activeChannels := int64(len(m.channelCounters))
	m.mu.RUnlock()

	return &RealtimeStats{
		TotalRequests:   total,
		SuccessRequests: success,
		FailRequests:    fail,
		AvgLatencyMs:    avgLatency,
		TotalTokens:     tokens,
		ActiveKeys: activeKeys,
		ActiveChannels:  activeChannels,
		Date:            date,
	}, nil
}

// GetOverview 获取概览统计（今日实时 + 最近 N 天历史）
func (m *Manager) GetOverview(ctx context.Context, days int) (*OverviewStats, error) {
	if days <= 0 {
		days = 7
	}

	// 今日实时
	realtime, _ := m.GetRealtime(ctx)
	today := RealtimeStats{}
	if realtime != nil {
		today = *realtime
	}

	// 历史数据
	var history []SystemDailyStats
	cutoff := time.Now().AddDate(0, 0, -(days)).Format("2006-01-02")
	if err := m.db.WithContext(ctx).
		Where("date >= ?", cutoff).
		Order("date ASC").
		Find(&history).Error; err != nil {
		m.logger.Error("query system daily stats failed", zap.Error(err))
	}

	last7Days := make([]DailyStatEntry, 0, len(history))
	for _, s := range history {
		last7Days = append(last7Days, DailyStatEntry{
			Date:        s.Date,
			TotalReqs:   int64(s.TotalRequests),
			SuccessReqs: int64(s.SuccessRequests),
			FailReqs:    int64(s.FailRequests),
		})
	}

	return &OverviewStats{
		Today:     today,
		Last7Days: last7Days,
	}, nil
}

// GetKeysStats 获取密钥统计
func (m *Manager) GetKeysStats(ctx context.Context, keysID uint, start, end string) ([]KeysDailyStats, error) {
	query := m.db.WithContext(ctx).Model(&KeysDailyStats{}).Where("keys_id = ?", keysID)
	if start != "" {
		query = query.Where("date >= ?", start)
	}
	if end != "" {
		query = query.Where("date <= ?", end)
	}

	var stats []KeysDailyStats
	err := query.Order("date ASC").Find(&stats).Error
	return stats, err
}

// GetChannelStats 获取渠道统计
func (m *Manager) GetChannelStats(ctx context.Context, channelID uint, start, end string) ([]ChannelDailyStats, error) {
	query := m.db.WithContext(ctx).Model(&ChannelDailyStats{}).Where("channel_id = ?", channelID)
	if start != "" {
		query = query.Where("date >= ?", start)
	}
	if end != "" {
		query = query.Where("date <= ?", end)
	}

	var stats []ChannelDailyStats
	err := query.Order("date ASC").Find(&stats).Error
	return stats, err
}

// QueryRequestLogs 查询请求日志（分页 + 筛选）
func (m *Manager) QueryRequestLogs(ctx context.Context, filter LogFilter) ([]RequestLog, int64, error) {
	query := m.db.WithContext(ctx).Model(&RequestLog{})

	if filter.KeysID > 0 {
		query = query.Where("keys_id = ?", filter.KeysID)
	}
	if filter.KeysName != "" {
		query = query.Where("keys_id IN (SELECT id FROM keys WHERE name LIKE ?)", "%"+filter.KeysName+"%")
	}
	if filter.ChannelID > 0 {
		query = query.Where("channel_id = ?", filter.ChannelID)
	}
	if filter.ChannelName != "" {
		query = query.Where("channel_id IN (SELECT id FROM channels WHERE name LIKE ?)", "%"+filter.ChannelName+"%")
	}
	if filter.ModelName != "" {
		query = query.Where("model_name LIKE ?", "%"+filter.ModelName+"%")
	}
	if filter.Status == "success" {
		query = query.Where("status_code >= 200 AND status_code < 300")
	} else if filter.Status == "failed" {
		query = query.Where("status_code < 200 OR status_code >= 300")
	}
	if filter.LogTypes != "" {
		types := strings.Split(filter.LogTypes, ",")
		if len(types) > 0 {
			query = query.Where("log_type IN ?", types)
		}
	}
	if filter.TraceID != "" {
		query = query.Where("trace_id = ?", filter.TraceID)
	}
	if filter.Keyword != "" {
		kw := "%" + filter.Keyword + "%"
		query = query.Where("trace_id LIKE ? OR error_msg LIKE ?", kw, kw)
	}
	if filter.Start != "" {
		query = query.Where("timestamp >= ?", filter.Start)
	}
	if filter.End != "" {
		query = query.Where("timestamp < ?", filter.End)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var logs []RequestLog
	offset := (filter.Page - 1) * filter.PageSize
	err := query.Order("timestamp DESC").
		Offset(offset).
		Limit(filter.PageSize).
		Find(&logs).Error

	return logs, total, err
}

// GetRequestLogByID 获取单条日志详情
func (m *Manager) GetRequestLogByID(ctx context.Context, id uint) (*RequestLog, error) {
	var log RequestLog
	err := m.db.WithContext(ctx).First(&log, id).Error
	return &log, err
}

// LogFilter 日志查询筛选条件
type LogFilter struct {
	KeysID       uint   `form:"keys_id"`
	KeysName     string `form:"keys_name"`     // 密钥名模糊搜索
	ChannelID    uint   `form:"channel_id"`
	ChannelName  string `form:"channel_name"`  // 渠道名模糊搜索
	ModelName    string `form:"model_name"`
	Status       string `form:"status"`        // success / failed
	LogTypes     string `form:"log_types"`     // 逗号分隔多选
	Keyword      string `form:"keyword"`       // 搜索 trace_id、error_msg
	TraceID      string `form:"trace_id"`      // 精确 trace_id 搜索
	Start        string `form:"start"`
	End          string `form:"end"`
	Page         int    `form:"page"`
	PageSize     int    `form:"page_size"`
}

// ========== 聚合任务 ==========

// runAggregation 执行一次聚合
func (m *Manager) runAggregation(ctx context.Context) {
	today := time.Now().Format("2006-01-02")

	// 1. 系统日统计
	m.aggregateSystemDaily(ctx, today)

	// 2. 密钥日统计
	m.aggregateKeysDaily(ctx, today)

	// 3. 渠道日统计
	m.aggregateChannelDaily(ctx, today)

	m.logger.Info("aggregation completed", zap.String("date", today))
}

// aggregateSystemDaily 聚合系统日统计
func (m *Manager) aggregateSystemDaily(ctx context.Context, date string) {
	var result struct {
		Total   int
		Success int
		Fail    int
		Tokens  int64
		AvgMs   float64
	}

	m.db.WithContext(ctx).Model(&RequestLog{}).
		Where("timestamp >= ? AND timestamp < ?", date+" 00:00:00", date+" 23:59:59").
		Where("log_type = ?", "consumption").
		Select("COUNT(*) as total, SUM(CASE WHEN status_code >= 200 AND status_code < 300 THEN 1 ELSE 0 END) as success, SUM(CASE WHEN status_code < 200 OR status_code >= 300 THEN 1 ELSE 0 END) as fail, COALESCE(SUM(prompt_tokens + completion_tokens), 0) as tokens, COALESCE(AVG(latency_ms), 0) as avg_ms").
		Scan(&result)

	// UPSERT
	m.db.WithContext(ctx).Where("date = ?", date).
		Assign(map[string]interface{}{
			"total_requests":   result.Total,
			"success_requests": result.Success,
			"fail_requests":     result.Fail,
			"total_tokens":      result.Tokens,
			"avg_latency_ms":    result.AvgMs,
		}).
		FirstOrCreate(&SystemDailyStats{Date: date})
}

// aggregateKeysDaily 聚合密钥日统计
func (m *Manager) aggregateKeysDaily(ctx context.Context, date string) {
	type row struct {
		KeysID uint
		ModelName  string
		Total      int
		Success    int
		Fail       int
		Tokens     int64
		AvgMs      float64
	}

	var rows []row
	m.db.WithContext(ctx).Model(&RequestLog{}).
		Where("timestamp >= ? AND timestamp < ?", date+" 00:00:00", date+" 23:59:59").
		Where("log_type = ?", "consumption").
		Select("keys_id, model_name, COUNT(*) as total, SUM(CASE WHEN status_code >= 200 AND status_code < 300 THEN 1 ELSE 0 END) as success, SUM(CASE WHEN status_code < 200 OR status_code >= 300 THEN 1 ELSE 0 END) as fail, COALESCE(SUM(prompt_tokens + completion_tokens), 0) as tokens, COALESCE(AVG(latency_ms), 0) as avg_ms").
		Group("keys_id, model_name").
		Scan(&rows)

	for _, r := range rows {
		m.db.WithContext(ctx).
			Where("date = ? AND keys_id = ? AND model_name = ?", date, r.KeysID, r.ModelName).
			Assign(map[string]interface{}{
				"total_requests":   r.Total,
				"success_requests": r.Success,
				"fail_requests":     r.Fail,
				"total_tokens":      r.Tokens,
				"avg_latency_ms":    r.AvgMs,
			}).
			FirstOrCreate(&KeysDailyStats{Date: date, KeysID: r.KeysID, ModelName: r.ModelName})
	}
}

// aggregateChannelDaily 聚合渠道日统计
func (m *Manager) aggregateChannelDaily(ctx context.Context, date string) {
	type row struct {
		ChannelID uint
		ModelName string
		Total     int
		Success   int
		Fail      int
		Tokens    int64
		AvgMs     float64
	}

	var rows []row
	m.db.WithContext(ctx).Model(&RequestLog{}).
		Where("timestamp >= ? AND timestamp < ?", date+" 00:00:00", date+" 23:59:59").
		Where("log_type = ?", "consumption").
		Select("channel_id, model_name, COUNT(*) as total, SUM(CASE WHEN status_code >= 200 AND status_code < 300 THEN 1 ELSE 0 END) as success, SUM(CASE WHEN status_code < 200 OR status_code >= 300 THEN 1 ELSE 0 END) as fail, COALESCE(SUM(prompt_tokens + completion_tokens), 0) as tokens, COALESCE(AVG(latency_ms), 0) as avg_ms").
		Where("channel_id IS NOT NULL").
		Group("channel_id, model_name").
		Scan(&rows)

	for _, r := range rows {
		m.db.WithContext(ctx).
			Where("date = ? AND channel_id = ? AND model_name = ?", date, r.ChannelID, r.ModelName).
			Assign(map[string]interface{}{
				"total_requests":   r.Total,
				"success_requests": r.Success,
				"fail_requests":     r.Fail,
				"total_tokens":      r.Tokens,
				"avg_latency_ms":    r.AvgMs,
			}).
			FirstOrCreate(&ChannelDailyStats{Date: date, ChannelID: r.ChannelID, ModelName: r.ModelName})
	}
}

// CleanOldLogs 清理过期日志
func (m *Manager) CleanOldLogs(ctx context.Context, retentionDays int) error {
	if retentionDays <= 0 {
		retentionDays = 30
	}
	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	result := m.db.WithContext(ctx).
		Where("timestamp < ?", cutoff).
		Delete(&RequestLog{})

	if result.Error != nil {
		return result.Error
	}

	m.logger.Info("cleaned old request logs",
		zap.Int64("deleted", result.RowsAffected),
		zap.Int("retention_days", retentionDays),
	)
	return nil
}

// ConsumerRealtimeStats 单个消费者的实时聚合统计
type ConsumerRealtimeStats struct {
	KeysID         uint    `json:"keys_id"`
	TotalRequests  int64   `json:"total_requests"`
	SuccessCount   int64   `json:"success_count"`
	FailCount      int64   `json:"fail_count"`
	TotalTokens    int64   `json:"total_tokens"`
	TotalCost      float64 `json:"total_cost"`
	AvgLatencyMs   float64 `json:"avg_latency_ms"`
	TopModels      []ModelCount `json:"top_models"`
}

// ModelCount 模型请求计数
type ModelCount struct {
	ModelName     string `json:"model_name"`
	TotalRequests int64  `json:"total_requests"`
}

// GetConsumerRealtime 从 request_logs 实时聚合消费者统计
func (m *Manager) GetConsumerRealtime(ctx context.Context, keysID uint) (*ConsumerRealtimeStats, error) {
	var stats ConsumerRealtimeStats
	stats.KeysID = keysID

	today := time.Now().Format("2006-01-02")
	query := m.db.WithContext(ctx).
		Where("keys_id = ? AND log_type = 'consumption' AND DATE(timestamp) = ?", keysID, today)

	// 基础聚合
	var result struct {
		TotalRequests int64
		SuccessCount  int64
		TotalTokens   int64
		TotalCost     float64
		AvgLatency    float64
	}
	err := query.Model(&RequestLog{}).
		Select("COUNT(*) as total_requests, SUM(CASE WHEN status_code >= 200 AND status_code < 300 THEN 1 ELSE 0 END) as success_count, SUM(prompt_tokens + completion_tokens) as total_tokens, COALESCE(SUM(cost), 0) as total_cost, CASE WHEN COUNT(*) > 0 THEN AVG(latency_ms) ELSE 0 END as avg_latency").
		Scan(&result).Error
	if err != nil {
		return nil, err
	}

	stats.TotalRequests = result.TotalRequests
	stats.SuccessCount = result.SuccessCount
	stats.FailCount = result.TotalRequests - result.SuccessCount
	stats.TotalTokens = result.TotalTokens
	stats.TotalCost = result.TotalCost
	stats.AvgLatencyMs = result.AvgLatency

	// Top 模型
	var topModels []ModelCount
	m.db.WithContext(ctx).
		Model(&RequestLog{}).
		Select("model_name, COUNT(*) as total_requests").
		Where("keys_id = ? AND log_type = 'consumption' AND DATE(timestamp) = ?", keysID, today).
		Group("model_name").
		Order("total_requests DESC").
		Limit(10).
		Scan(&topModels)
	stats.TopModels = topModels

	return &stats, nil
}

// ChannelRealtimeStats 单个渠道的实时聚合统计
type ChannelRealtimeStats struct {
	ChannelID     uint    `json:"channel_id"`
	TotalRequests int64   `json:"total_requests"`
	SuccessCount  int64   `json:"success_count"`
	FailCount     int64   `json:"fail_count"`
	TotalTokens   int64   `json:"total_tokens"`
	TotalCost     float64 `json:"total_cost"`
	AvgLatencyMs  float64 `json:"avg_latency_ms"`
	TopModels     []ModelCount `json:"top_models"`
}

// GetChannelRealtime 从 request_logs 实时聚合渠道统计
func (m *Manager) GetChannelRealtime(ctx context.Context, channelID uint) (*ChannelRealtimeStats, error) {
	var stats ChannelRealtimeStats
	stats.ChannelID = channelID

	today := time.Now().Format("2006-01-02")
	query := m.db.WithContext(ctx).
		Where("channel_id = ? AND log_type = 'consumption' AND DATE(timestamp) = ?", channelID, today)

	var result struct {
		TotalRequests int64
		SuccessCount  int64
		TotalTokens   int64
		TotalCost     float64
		AvgLatency    float64
	}
	err := query.Model(&RequestLog{}).
		Select("COUNT(*) as total_requests, SUM(CASE WHEN status_code >= 200 AND status_code < 300 THEN 1 ELSE 0 END) as success_count, SUM(prompt_tokens + completion_tokens) as total_tokens, COALESCE(SUM(cost), 0) as total_cost, CASE WHEN COUNT(*) > 0 THEN AVG(latency_ms) ELSE 0 END as avg_latency").
		Scan(&result).Error
	if err != nil {
		return nil, err
	}

	stats.TotalRequests = result.TotalRequests
	stats.SuccessCount = result.SuccessCount
	stats.FailCount = result.TotalRequests - result.SuccessCount
	stats.TotalTokens = result.TotalTokens
	stats.TotalCost = result.TotalCost
	stats.AvgLatencyMs = result.AvgLatency

	// Top 模型
	var topModels []ModelCount
	m.db.WithContext(ctx).
		Model(&RequestLog{}).
		Select("model_name, COUNT(*) as total_requests").
		Where("channel_id = ? AND log_type = 'consumption' AND DATE(timestamp) = ?", channelID, today).
		Group("model_name").
		Order("total_requests DESC").
		Limit(10).
		Scan(&topModels)
	stats.TopModels = topModels

	return &stats, nil
}

// ========== 仪表盘聚合查询 ==========

// HourlyTrendEntry 小时趋势条目
type HourlyTrendEntry struct {
	Hour    string `json:"hour"`
	Success int64  `json:"success"`
	Fail    int64  `json:"fail"`
}

// GetHourlyTrend 获取近 N 小时请求趋势（按小时聚合）
func (m *Manager) GetHourlyTrend(ctx context.Context, hours int) ([]HourlyTrendEntry, error) {
	if hours <= 0 {
		hours = 24
	}
	cutoff := time.Now().Add(-time.Duration(hours) * time.Hour).Format("2006-01-02 15:04:00")

	type row struct {
		Hour    string
		Success int64
		Fail    int64
	}
	var rows []row
	err := m.db.WithContext(ctx).Model(&RequestLog{}).
		Select("strftime('%Y-%m-%d %H:00', timestamp) as hour, SUM(CASE WHEN status_code >= 200 AND status_code < 300 THEN 1 ELSE 0 END) as success, SUM(CASE WHEN status_code < 200 OR status_code >= 300 THEN 1 ELSE 0 END) as fail").
		Where("timestamp >= ? AND log_type = ?", cutoff, "consumption").
		Group("hour").
		Order("hour ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	result := make([]HourlyTrendEntry, 0, len(rows))
	for _, r := range rows {
		result = append(result, HourlyTrendEntry{
			Hour:    r.Hour,
			Success: r.Success,
			Fail:    r.Fail,
		})
	}
	return result, nil
}

// TopModelEntry 模型排行条目
type TopModelEntry struct {
	ModelName     string `json:"model_name"`
	TotalRequests int64  `json:"total_requests"`
}

// GetTopModels 获取 Top N 模型排行
func (m *Manager) GetTopModels(ctx context.Context, limit int) ([]TopModelEntry, error) {
	if limit <= 0 {
		limit = 5
	}
	today := time.Now().Format("2006-01-02")

	var models []TopModelEntry
	err := m.db.WithContext(ctx).Model(&RequestLog{}).
		Select("model_name, COUNT(*) as total_requests").
		Where("log_type = ? AND DATE(timestamp) = ?", "consumption", today).
		Group("model_name").
		Order("total_requests DESC").
		Limit(limit).
		Scan(&models).Error
	return models, err
}

// TopChannelEntry 渠道排行条目
type TopChannelEntry struct {
	ChannelID     uint    `json:"channel_id"`
	ChannelName   string  `json:"channel_name"`
	TotalRequests int64   `json:"total_requests"`
	SuccessRate   float64 `json:"success_rate"`
	AvgLatencyMs  float64 `json:"avg_latency_ms"`
}

// GetTopChannels 获取 Top N 渠道负载排行
func (m *Manager) GetTopChannels(ctx context.Context, limit int) ([]TopChannelEntry, error) {
	if limit <= 0 {
		limit = 10
	}
	today := time.Now().Format("2006-01-02")

	type row struct {
		ChannelID     uint
		TotalRequests int64
		SuccessCount  int64
		AvgLatencyMs  float64
	}
	var rows []row
	err := m.db.WithContext(ctx).Model(&RequestLog{}).
		Select("channel_id, COUNT(*) as total_requests, SUM(CASE WHEN status_code >= 200 AND status_code < 300 THEN 1 ELSE 0 END) as success_count, CASE WHEN COUNT(*) > 0 THEN AVG(latency_ms) ELSE 0 END as avg_latency_ms").
		Where("log_type = ? AND DATE(timestamp) = ? AND channel_id IS NOT NULL", "consumption", today).
		Group("channel_id").
		Order("total_requests DESC").
		Limit(limit).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	// 批量查询渠道名称
	channelIDs := make([]uint, 0, len(rows))
	for _, r := range rows {
		channelIDs = append(channelIDs, r.ChannelID)
	}

	type channelNameRow struct {
		ID   uint
		Name string
	}
	var channelNames []channelNameRow
	if len(channelIDs) > 0 {
		m.db.WithContext(ctx).Table("channels").
			Select("id, name").
			Where("id IN ?", channelIDs).
			Scan(&channelNames)
	}

	nameMap := make(map[uint]string, len(channelNames))
	for _, c := range channelNames {
		nameMap[c.ID] = c.Name
	}

	result := make([]TopChannelEntry, 0, len(rows))
	for _, r := range rows {
		sr := 0.0
		if r.TotalRequests > 0 {
			sr = float64(r.SuccessCount) / float64(r.TotalRequests) * 100
		}
		name := nameMap[r.ChannelID]
		if name == "" {
			name = fmt.Sprintf("Channel #%d", r.ChannelID)
		}
		result = append(result, TopChannelEntry{
			ChannelID:     r.ChannelID,
			ChannelName:   name,
			TotalRequests: r.TotalRequests,
			SuccessRate:   sr,
			AvgLatencyMs:  r.AvgLatencyMs,
		})
	}
	return result, nil
}

// RecentErrorEntry 最近异常请求条目
type RecentErrorEntry struct {
	ID         uint    `json:"id"`
	Timestamp  string  `json:"timestamp"`
	ModelName  string  `json:"model_name"`
	ChannelID  *uint   `json:"channel_id"`
	StatusCode int     `json:"status_code"`
	LatencyMs  int     `json:"latency_ms"`
	ErrorMsg   *string `json:"error_msg"`
	TraceID    string  `json:"trace_id"`
}

// GetRecentErrors 获取最近 N 条异常请求（status_code 非 2xx 或延迟超阈值）
func (m *Manager) GetRecentErrors(ctx context.Context, limit int) ([]RecentErrorEntry, error) {
	if limit <= 0 {
		limit = 5
	}
	latencyThreshold := 10000 // 10s 视为异常

	var logs []RequestLog
	err := m.db.WithContext(ctx).
		Where("log_type = ? AND (status_code < 200 OR status_code >= 300 OR latency_ms > ?)", "consumption", latencyThreshold).
		Order("timestamp DESC").
		Limit(limit).
		Find(&logs).Error
	if err != nil {
		return nil, err
	}

	result := make([]RecentErrorEntry, 0, len(logs))
	for _, l := range logs {
		result = append(result, RecentErrorEntry{
			ID:         l.ID,
			Timestamp:  l.Timestamp.Format("2006-01-02 15:04:05"),
			ModelName:  l.ModelName,
			ChannelID:  l.ChannelID,
			StatusCode: l.StatusCode,
			LatencyMs:  l.LatencyMs,
			ErrorMsg:   l.ErrorMsg,
			TraceID:    l.TraceID,
		})
	}
	return result, nil
}
