package stats

import (
	"context"
	"encoding/json"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// OnLogHook 日志钩子回调类型（用于插件 on_log 钩子，解耦 stats 包和 plugin 包）
type OnLogHook func(log *RequestLog)

// AsyncWriter 异步批量日志写入器
type AsyncWriter struct {
	db        *gorm.DB
	logger    *zap.Logger
	ch        chan *RequestLog
	done      chan struct{}
	wg        sync.WaitGroup
	batchSize int
	flushMs   int
	statsMgr  *Manager   // 用于实时计数
	onLogHook OnLogHook  // 日志写入前的钩子回调
}

// NewAsyncWriter 创建异步写入器
func NewAsyncWriter(db *gorm.DB, logger *zap.Logger, statsMgr *Manager, bufferSize, batchSize, flushMs int) *AsyncWriter {
	if bufferSize <= 0 {
		bufferSize = 10000
	}
	if batchSize <= 0 {
		batchSize = 50
	}
	if flushMs <= 0 {
		flushMs = 100
	}
	return &AsyncWriter{
		db:        db,
		logger:    logger,
		ch:        make(chan *RequestLog, bufferSize),
		done:      make(chan struct{}),
		batchSize: batchSize,
		flushMs:   flushMs,
		statsMgr:  statsMgr,
	}
}

// Start 启动写入协程
func (w *AsyncWriter) Start() {
	w.wg.Add(1)
	go w.run()
}

// SetOnLogHook 设置日志钩子回调（日志入队前触发）
func (w *AsyncWriter) SetOnLogHook(hook OnLogHook) {
	w.onLogHook = hook
}

// Record 记录一条请求日志（非阻塞，满则丢弃）
func (w *AsyncWriter) Record(log *RequestLog) bool {
	// 触发 on_log 钩子（异步，不阻塞主流程）
	if w.onLogHook != nil {
		func() {
			defer func() {
				if r := recover(); r != nil {
					w.logger.Warn("on_log hook panic recovered", zap.Any("recover", r))
				}
			}()
			w.onLogHook(log)
		}()
	}

	select {
	case w.ch <- log:
		return true
	default:
		// channel 满了，丢弃并记录警告
		w.logger.Warn("async writer buffer full, dropping log",
			zap.Uint("keys_id", log.KeysID),
			zap.String("model", log.ModelName),
		)
		return false
	}
}

// RecordWait 记录一条请求日志（阻塞等待，直到写入或超时）
func (w *AsyncWriter) RecordWait(ctx context.Context, log *RequestLog) error {
	select {
	case w.ch <- log:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Close 优雅关闭：停止接收，等待已入队的日志全部刷入
func (w *AsyncWriter) Close(timeout time.Duration) {
	close(w.done)
	// 等待写入协程结束，带超时
	done := make(chan struct{})
	go func() {
		w.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		w.logger.Info("async writer closed gracefully")
	case <-time.After(timeout):
		w.logger.Warn("async writer close timeout, some logs may be lost")
	}
}

// run 写入协程主循环
func (w *AsyncWriter) run() {
	defer w.wg.Done()

	batch := make([]*RequestLog, 0, w.batchSize)
	ticker := time.NewTicker(time.Duration(w.flushMs) * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case log, ok := <-w.ch:
			if ok {
				batch = append(batch, log)
				if len(batch) >= w.batchSize {
					w.flush(batch)
					batch = batch[:0]
				}
			}
		case <-ticker.C:
			if len(batch) > 0 {
				w.flush(batch)
				batch = batch[:0]
			}
		case <-w.done:
			// 收到关闭信号，排空 channel
			for len(w.ch) > 0 {
				log := <-w.ch
				batch = append(batch, log)
			}
			if len(batch) > 0 {
				w.flush(batch)
			}
			return
		}
	}
}

// flush 批量写入数据库
func (w *AsyncWriter) flush(batch []*RequestLog) {
	if len(batch) == 0 {
		return
	}

	// 重试逻辑：最多 3 次
	for attempt := 0; attempt < 3; attempt++ {
		err := w.db.WithContext(context.Background()).CreateInBatches(batch, len(batch)).Error
		if err == nil {
			// 写入成功，更新实时计数器
			if w.statsMgr != nil {
				for _, log := range batch {
					w.statsMgr.IncrementCounters(log)
				}
			}
			return
		}

		w.logger.Error("flush request logs failed",
			zap.Int("attempt", attempt+1),
			zap.Int("batch_size", len(batch)),
			zap.Error(err),
		)

		if attempt < 2 {
			time.Sleep(time.Duration(attempt+1) * 2 * time.Second) // 指数退避
		}
	}

	// 所有重试失败，写入本地文件兜底
	w.fallbackToFile(batch)
}

// fallbackToFile 写入本地文件作为兜底
func (w *AsyncWriter) fallbackToFile(batch []*RequestLog) {
	w.logger.Error("all flush retries failed, writing to fallback file")
	// 简单实现：记录错误日志，后续可扩展为写文件
	for _, log := range batch {
		data, _ := json.Marshal(log)
		w.logger.Error("dropped request log", zap.String("log", string(data)))
	}
}

// ========== 内存实时计数器 ==========

// TodayCounters 今日实时计数器
type TodayCounters struct {
	date            atomic.Value // string
	totalRequests   atomic.Int64
	successRequests atomic.Int64
	failRequests    atomic.Int64
	totalTokens     atomic.Int64
	totalLatencyMs  atomic.Int64 // 累计延迟，用于计算平均
	requestCount    atomic.Int64 // 用于计算平均延迟的计数
}

// NewTodayCounters 创建计数器
func NewTodayCounters() *TodayCounters {
	c := &TodayCounters{}
	c.date.Store(time.Now().Format("2006-01-02"))
	return c
}

// Increment 递增计数器
func (c *TodayCounters) Increment(success bool, latencyMs int, tokens int) {
	today := time.Now().Format("2006-01-02")
	if stored := c.date.Load().(string); stored != today {
		// 日期变了，重置
		c.date.Store(today)
		c.totalRequests.Store(0)
		c.successRequests.Store(0)
		c.failRequests.Store(0)
		c.totalTokens.Store(0)
		c.totalLatencyMs.Store(0)
		c.requestCount.Store(0)
	}

	c.totalRequests.Add(1)
	if success {
		c.successRequests.Add(1)
	} else {
		c.failRequests.Add(1)
	}
	c.totalTokens.Add(int64(tokens))
	c.totalLatencyMs.Add(int64(latencyMs))
	c.requestCount.Add(1)
}

// Snapshot 获取当前快照
func (c *TodayCounters) Snapshot() (date string, total, success, fail int64, avgLatencyMs float64, tokens int64) {
	date = c.date.Load().(string)
	total = c.totalRequests.Load()
	success = c.successRequests.Load()
	fail = c.failRequests.Load()
	tokens = c.totalTokens.Load()
	count := c.requestCount.Load()
	if count > 0 {
		avgLatencyMs = float64(c.totalLatencyMs.Load()) / float64(count)
	}
	return
}
