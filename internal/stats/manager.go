package stats

import (
	"context"
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
	// 消费者级实时计数
	consumerCounters map[uint]*TodayCounters
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
		consumerCounters: make(map[uint]*TodayCounters),
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

// IncrementCounters 递增实时计数器
func (m *Manager) IncrementCounters(log *RequestLog) {
	success := log.StatusCode >= 200 && log.StatusCode < 300
	tokens := log.PromptTokens + log.CompletionTokens
	m.counters.Increment(success, log.LatencyMs, tokens)

	// 消费者级计数
	m.mu.Lock()
	cc, ok := m.consumerCounters[log.ConsumerID]
	if !ok {
		cc = NewTodayCounters()
		m.consumerCounters[log.ConsumerID] = cc
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

	// 活跃消费者和渠道数量
	m.mu.RLock()
	activeConsumers := int64(len(m.consumerCounters))
	activeChannels := int64(len(m.channelCounters))
	m.mu.RUnlock()

	return &RealtimeStats{
		TotalRequests:   total,
		SuccessRequests: success,
		FailRequests:    fail,
		AvgLatencyMs:    avgLatency,
		TotalTokens:     tokens,
		ActiveConsumers: activeConsumers,
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

// GetConsumerStats 获取消费者统计
func (m *Manager) GetConsumerStats(ctx context.Context, consumerID uint, start, end string) ([]ConsumerDailyStats, error) {
	query := m.db.WithContext(ctx).Model(&ConsumerDailyStats{}).Where("consumer_id = ?", consumerID)
	if start != "" {
		query = query.Where("date >= ?", start)
	}
	if end != "" {
		query = query.Where("date <= ?", end)
	}

	var stats []ConsumerDailyStats
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

	if filter.ConsumerID > 0 {
		query = query.Where("consumer_id = ?", filter.ConsumerID)
	}
	if filter.ChannelID > 0 {
		query = query.Where("channel_id = ?", filter.ChannelID)
	}
	if filter.ModelName != "" {
		query = query.Where("model_name = ?", filter.ModelName)
	}
	if filter.Status == "success" {
		query = query.Where("status_code >= 200 AND status_code < 300")
	} else if filter.Status == "failed" {
		query = query.Where("status_code < 200 OR status_code >= 300")
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
	ConsumerID uint   `form:"consumer_id"`
	ChannelID  uint   `form:"channel_id"`
	ModelName  string `form:"model_name"`
	Status     string `form:"status"` // success / failed
	Start      string `form:"start"`
	End        string `form:"end"`
	Page       int    `form:"page"`
	PageSize   int    `form:"page_size"`
}

// ========== 聚合任务 ==========

// runAggregation 执行一次聚合
func (m *Manager) runAggregation(ctx context.Context) {
	today := time.Now().Format("2006-01-02")

	// 1. 系统日统计
	m.aggregateSystemDaily(ctx, today)

	// 2. 消费者日统计
	m.aggregateConsumerDaily(ctx, today)

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
		AvgMs   int
	}

	m.db.WithContext(ctx).Model(&RequestLog{}).
		Where("timestamp >= ? AND timestamp < ?", date+" 00:00:00", date+" 23:59:59").
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

// aggregateConsumerDaily 聚合消费者日统计
func (m *Manager) aggregateConsumerDaily(ctx context.Context, date string) {
	type row struct {
		ConsumerID uint
		ModelName  string
		Total      int
		Success    int
		Fail       int
		Tokens     int64
		AvgMs      int
	}

	var rows []row
	m.db.WithContext(ctx).Model(&RequestLog{}).
		Where("timestamp >= ? AND timestamp < ?", date+" 00:00:00", date+" 23:59:59").
		Select("consumer_id, model_name, COUNT(*) as total, SUM(CASE WHEN status_code >= 200 AND status_code < 300 THEN 1 ELSE 0 END) as success, SUM(CASE WHEN status_code < 200 OR status_code >= 300 THEN 1 ELSE 0 END) as fail, COALESCE(SUM(prompt_tokens + completion_tokens), 0) as tokens, COALESCE(AVG(latency_ms), 0) as avg_ms").
		Group("consumer_id, model_name").
		Scan(&rows)

	for _, r := range rows {
		m.db.WithContext(ctx).
			Where("date = ? AND consumer_id = ? AND model_name = ?", date, r.ConsumerID, r.ModelName).
			Assign(map[string]interface{}{
				"total_requests":   r.Total,
				"success_requests": r.Success,
				"fail_requests":     r.Fail,
				"total_tokens":      r.Tokens,
				"avg_latency_ms":    r.AvgMs,
			}).
			FirstOrCreate(&ConsumerDailyStats{Date: date, ConsumerID: r.ConsumerID, ModelName: r.ModelName})
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
		AvgMs     int
	}

	var rows []row
	m.db.WithContext(ctx).Model(&RequestLog{}).
		Where("timestamp >= ? AND timestamp < ?", date+" 00:00:00", date+" 23:59:59").
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
