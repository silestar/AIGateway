package stats

import (
	"encoding/json"
	"testing"
	"time"

	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := db.AutoMigrate(&RequestLog{}, &SystemDailyStats{}, &KeysDailyStats{}, &ChannelDailyStats{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestAsyncWriter_Record(t *testing.T) {
	db := setupTestDB(t)
	logger := zap.NewNop()
	statsMgr := NewManager(db, logger)

	writer := NewAsyncWriter(db, logger, statsMgr, 100, 10, 50)
	writer.Start()

	// 写入一条日志
	log := &RequestLog{
		Timestamp:   time.Now(),
		KeysID:  1,
		ModelName:   "gpt-4",
		ChannelID:   uintPtr(1),
		AccountID:   uintPtr(1),
		RetryChain:  json.RawMessage(`[]`),
		IsStream:    false,
		StatusCode:  200,
		LatencyMs:   100,
	}
	if !writer.Record(log) {
		t.Fatal("failed to record log")
	}

	// 等待刷入
	time.Sleep(200 * time.Millisecond)
	writer.Close(2 * time.Second)

	// 验证数据库中有记录
	var count int64
	db.Model(&RequestLog{}).Count(&count)
	if count != 1 {
		t.Errorf("expected 1 log, got %d", count)
	}

	// 验证实时计数器
	_, total, success, fail, _, _ := statsMgr.counters.Snapshot()
	if total != 1 {
		t.Errorf("expected total=1, got %d", total)
	}
	if success != 1 {
		t.Errorf("expected success=1, got %d", success)
	}
	if fail != 0 {
		t.Errorf("expected fail=0, got %d", fail)
	}
}

func TestManager_Realtime(t *testing.T) {
	db := setupTestDB(t)
	logger := zap.NewNop()
	mgr := NewManager(db, logger)

	// 模拟递增
	log := &RequestLog{
		StatusCode:      200,
		LatencyMs:       50,
		PromptTokens:    100,
		CompletionTokens: 50,
	}
	mgr.IncrementCounters(log)

	stats, err := mgr.GetRealtime(nil)
	if err != nil {
		t.Fatalf("GetRealtime: %v", err)
	}
	if stats.TotalRequests != 1 {
		t.Errorf("expected 1 total, got %d", stats.TotalRequests)
	}
	if stats.SuccessRequests != 1 {
		t.Errorf("expected 1 success, got %d", stats.SuccessRequests)
	}
	if stats.AvgLatencyMs != 50 {
		t.Errorf("expected avg latency 50, got %d", stats.AvgLatencyMs)
	}
}

func TestManager_QueryRequestLogs(t *testing.T) {
	db := setupTestDB(t)
	logger := zap.NewNop()
	mgr := NewManager(db, logger)

	// 插入测试数据
	db.Create(&RequestLog{
		Timestamp:  time.Now(),
		KeysID: 1,
		ModelName:  "gpt-4",
		ChannelID:  uintPtr(1),
		StatusCode: 200,
		LatencyMs:  50,
	})
	db.Create(&RequestLog{
		Timestamp:  time.Now(),
		KeysID: 2,
		ModelName:  "gpt-3.5",
		ChannelID:  uintPtr(2),
		StatusCode: 500,
		LatencyMs:  200,
	})

	// 查所有
	logs, total, err := mgr.QueryRequestLogs(nil, LogFilter{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("QueryRequestLogs: %v", err)
	}
	if total != 2 {
		t.Errorf("expected 2 logs, got %d", total)
	}
	if len(logs) != 2 {
		t.Errorf("expected 2 logs, got %d", len(logs))
	}

	// 按模型筛选
	logs, total, err = mgr.QueryRequestLogs(nil, LogFilter{ModelName: "gpt-4", Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("QueryRequestLogs filter: %v", err)
	}
	if total != 1 {
		t.Errorf("expected 1 log for gpt-4, got %d", total)
	}

	// 按状态筛选
	logs, total, err = mgr.QueryRequestLogs(nil, LogFilter{Status: "failed", Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("QueryRequestLogs status: %v", err)
	}
	if total != 1 {
		t.Errorf("expected 1 failed log, got %d", total)
	}
}

func TestManager_Aggregation(t *testing.T) {
	db := setupTestDB(t)
	logger := zap.NewNop()
	mgr := NewManager(db, logger)

	// 插入测试数据
	now := time.Now()
	db.Create(&RequestLog{
		Timestamp:        now,
		KeysID:       1,
		ModelName:        "gpt-4",
		ChannelID:        uintPtr(1),
		AccountID:        uintPtr(1),
		StatusCode:       200,
		LatencyMs:        100,
		PromptTokens:     50,
		CompletionTokens: 50,
	})
	db.Create(&RequestLog{
		Timestamp:        now,
		KeysID:       1,
		ModelName:        "gpt-4",
		ChannelID:        uintPtr(1),
		AccountID:        uintPtr(1),
		StatusCode:       500,
		LatencyMs:        200,
		PromptTokens:     30,
		CompletionTokens: 20,
	})

	// 执行聚合
	mgr.runAggregation(nil)

	// 验证系统日统计
	var sysStats SystemDailyStats
	db.Where("date = ?", now.Format("2006-01-02")).First(&sysStats)
	if sysStats.TotalRequests != 2 {
		t.Errorf("expected 2 total requests, got %d", sysStats.TotalRequests)
	}
	if sysStats.SuccessRequests != 1 {
		t.Errorf("expected 1 success, got %d", sysStats.SuccessRequests)
	}
	if sysStats.FailRequests != 1 {
		t.Errorf("expected 1 fail, got %d", sysStats.FailRequests)
	}
}

func uintPtr(v uint) *uint { return &v }
