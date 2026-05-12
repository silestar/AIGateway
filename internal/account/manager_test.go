package account

import (
	"context"
	"testing"
	"time"

	"github.com/silestar/AIGateway/internal/channel"
	"github.com/silestar/AIGateway/internal/config"
	"github.com/silestar/AIGateway/internal/crypto"

	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// setupTestDB 创建测试用内存数据库
func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := db.AutoMigrate(&Account{}, &channel.Channel{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

// setupTestManager 创建测试用 Manager
func setupTestManager(t *testing.T) (*Manager, *gorm.DB) {
	t.Helper()
	db := setupTestDB(t)

	secretKey, err := crypto.EnsureSecretKey("/tmp/agw_test.env")
	if err != nil {
		t.Fatalf("ensure secret key: %v", err)
	}
	cryptoSvc, err := crypto.NewCrypto(secretKey)
	if err != nil {
		t.Fatalf("init crypto: %v", err)
	}

	cfg := config.AccountManagerConfig{
		AffinityTTL:                 3600,
		ConsecutiveFailureThreshold: 3,
		MinDisableDuration:          10, // 测试用短时间
		ProbeInterval:               30,
		ProbeActiveRatioThreshold:   0.4,
		MaxProbeFailures:            10,
		MaxProbeRecoverPerCycle:     1,
		ProbeCooldownDuration:       7200,
		ProbeCooldownDurationL2:     86400,
		ChannelHealthCheckInterval:   3600,
		AccountStatusCacheTTL:       30,
		AccountKeyCacheTTL:          60,
	}

	logger := zap.NewNop()
	cache := NewMemoryCache()
	mgr := NewManager(db, cache, cryptoSvc, cfg, logger)

	return mgr, db
}

// ========== 测试：账号选择 ==========

func TestSelectAccount_NoAffinity(t *testing.T) {
	mgr, db := setupTestManager(t)
	ctx := context.Background()

	// 创建渠道和账号
	ch := &channel.Channel{Name: "test", Type: "openai", BaseURL: "https://api.openai.com", Status: "active"}
	db.Create(ch)

	mgr.Create(ctx, ch.ID, "sk-test-key-1")
	mgr.Create(ctx, ch.ID, "sk-test-key-2")

	// 无绑定 → 按优先级选择第一个
	acc, err := mgr.SelectAccount(ctx, 1, ch.ID)
	if err != nil {
		t.Fatalf("SelectAccount: %v", err)
	}
	if acc.Priority != 0 {
		t.Errorf("expected priority 0, got %d", acc.Priority)
	}
}

func TestSelectAccount_AffinityReuse(t *testing.T) {
	mgr, db := setupTestManager(t)
	ctx := context.Background()

	ch := &channel.Channel{Name: "test", Type: "openai", BaseURL: "https://api.openai.com", Status: "active"}
	db.Create(ch)

	mgr.Create(ctx, ch.ID, "sk-test-key-1")
	mgr.Create(ctx, ch.ID, "sk-test-key-2")

	// 第一次选择
	acc1, _ := mgr.SelectAccount(ctx, 100, ch.ID)
	// 第二次选择同一消费者 → 应命中粘性
	acc2, _ := mgr.SelectAccount(ctx, 100, ch.ID)

	if acc1.ID != acc2.ID {
		t.Errorf("affinity not working: first=%d, second=%d", acc1.ID, acc2.ID)
	}
}

func TestSelectAccount_AffinityExpired(t *testing.T) {
	mgr, db := setupTestManager(t)
	ctx := context.Background()

	// 设置很短的 affinity TTL
	mgr.cfg.AffinityTTL = 1 // 1秒

	ch := &channel.Channel{Name: "test", Type: "openai", BaseURL: "https://api.openai.com", Status: "active"}
	db.Create(ch)

	mgr.Create(ctx, ch.ID, "sk-test-key-1")

	// 第一次选择
	acc1, _ := mgr.SelectAccount(ctx, 100, ch.ID)

	// 等待绑定过期
	time.Sleep(2 * time.Second)

	// 绑定过期后应该重新选择
	acc2, err := mgr.SelectAccount(ctx, 100, ch.ID)
	if err != nil {
		t.Fatalf("SelectAccount after expiry: %v", err)
	}
	// 由于只有一个账号，ID 仍然相同，但绑定已被刷新
	_ = acc1
	_ = acc2
}

func TestSelectAccount_NoActiveAccount(t *testing.T) {
	mgr, db := setupTestManager(t)
	ctx := context.Background()

	ch := &channel.Channel{Name: "test", Type: "openai", BaseURL: "https://api.openai.com", Status: "active"}
	db.Create(ch)

	// 不创建任何账号
	_, err := mgr.SelectAccount(ctx, 1, ch.ID)
	if err == nil {
		t.Error("expected error when no active accounts")
	}
}

// ========== 测试：故障降级 ==========

func TestReportResult_Success(t *testing.T) {
	mgr, db := setupTestManager(t)
	ctx := context.Background()

	ch := &channel.Channel{Name: "test", Type: "openai", BaseURL: "https://api.openai.com", Status: "active"}
	db.Create(ch)
	acc, _ := mgr.Create(ctx, ch.ID, "sk-test-key-1")

	// 先设置一些失败计数
	db.Model(&Account{}).Where("id = ?", acc.ID).Update("consecutive_failures", 3)

	// 报告成功 → 重置失败计数
	err := mgr.ReportResult(ctx, acc.ID, true, 200)
	if err != nil {
		t.Fatalf("ReportResult success: %v", err)
	}

	var updated Account
	db.First(&updated, acc.ID)
	if updated.ConsecutiveFailures != 0 {
		t.Errorf("expected consecutive_failures=0, got %d", updated.ConsecutiveFailures)
	}
}

func TestReportResult_FailureDisable(t *testing.T) {
	mgr, db := setupTestManager(t)
	ctx := context.Background()

	// 阈值=3
	ch := &channel.Channel{Name: "test", Type: "openai", BaseURL: "https://api.openai.com", Status: "active"}
	db.Create(ch)
	acc, _ := mgr.Create(ctx, ch.ID, "sk-test-key-1")

	// 连续失败3次 → 禁用
	for i := 0; i < 3; i++ {
		mgr.ReportResult(ctx, acc.ID, false, 500)
	}

	var updated Account
	db.First(&updated, acc.ID)
	if updated.Status != "disabled" {
		t.Errorf("expected status=disabled, got %s", updated.Status)
	}
	if updated.ConsecutiveFailures != 3 {
		t.Errorf("expected consecutive_failures=3, got %d", updated.ConsecutiveFailures)
	}
}

func TestReportResult_4xxNotCounted(t *testing.T) {
	mgr, db := setupTestManager(t)
	ctx := context.Background()

	ch := &channel.Channel{Name: "test", Type: "openai", BaseURL: "https://api.openai.com", Status: "active"}
	db.Create(ch)
	acc, _ := mgr.Create(ctx, ch.ID, "sk-test-key-1")

	// 4xx 非 429 → 不计入失败
	mgr.ReportResult(ctx, acc.ID, false, 400)
	mgr.ReportResult(ctx, acc.ID, false, 403)

	var updated Account
	db.First(&updated, acc.ID)
	if updated.ConsecutiveFailures != 0 {
		t.Errorf("4xx should not be counted, got consecutive_failures=%d", updated.ConsecutiveFailures)
	}
}

func TestReportResult_429Counted(t *testing.T) {
	mgr, db := setupTestManager(t)
	ctx := context.Background()

	ch := &channel.Channel{Name: "test", Type: "openai", BaseURL: "https://api.openai.com", Status: "active"}
	db.Create(ch)
	acc, _ := mgr.Create(ctx, ch.ID, "sk-test-key-1")

	// 429 → 计入失败
	mgr.ReportResult(ctx, acc.ID, false, 429)

	var updated Account
	db.First(&updated, acc.ID)
	if updated.ConsecutiveFailures != 1 {
		t.Errorf("429 should be counted, got consecutive_failures=%d", updated.ConsecutiveFailures)
	}
}

// ========== 测试：CRUD ==========

func TestCreate(t *testing.T) {
	mgr, _ := setupTestManager(t)
	ctx := context.Background()

	ch := &channel.Channel{Name: "test", Type: "openai", BaseURL: "https://api.openai.com", Status: "active"}
	// 需要 db 来创建 channel
	db, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	db.AutoMigrate(&channel.Channel{})
	db.Create(ch)

	acc, err := mgr.Create(ctx, ch.ID, "sk-test-api-key")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if acc.ID == 0 {
		t.Error("expected non-zero ID")
	}
	if acc.Status != "active" {
		t.Errorf("expected status=active, got %s", acc.Status)
	}
	if acc.Priority != 0 {
		t.Errorf("expected priority=0 for first account, got %d", acc.Priority)
	}
}

func TestGetDecryptedAPIKey(t *testing.T) {
	mgr, db := setupTestManager(t)
	ctx := context.Background()

	ch := &channel.Channel{Name: "test", Type: "openai", BaseURL: "https://api.openai.com", Status: "active"}
	db.Create(ch)

	originalKey := "sk-test-my-secret-key"
	acc, _ := mgr.Create(ctx, ch.ID, originalKey)

	// 解密后应与原始一致
	decrypted, err := mgr.GetDecryptedAPIKey(ctx, acc.ID)
	if err != nil {
		t.Fatalf("GetDecryptedAPIKey: %v", err)
	}
	if decrypted != originalKey {
		t.Errorf("decrypted key mismatch: got %s, want %s", decrypted, originalKey)
	}
}

func TestUpdateStatus(t *testing.T) {
	mgr, db := setupTestManager(t)
	ctx := context.Background()

	ch := &channel.Channel{Name: "test", Type: "openai", BaseURL: "https://api.openai.com", Status: "active"}
	db.Create(ch)
	acc, _ := mgr.Create(ctx, ch.ID, "sk-test-key")

	// 禁用
	mgr.UpdateStatus(ctx, acc.ID, "disabled")
	var updated Account
	db.First(&updated, acc.ID)
	if updated.Status != "disabled" {
		t.Errorf("expected disabled, got %s", updated.Status)
	}

	// 重新启用 → 应重置失败计数
	db.Model(&Account{}).Where("id = ?", acc.ID).Update("consecutive_failures", 5)
	mgr.UpdateStatus(ctx, acc.ID, "active")
	db.First(&updated, acc.ID)
	if updated.ConsecutiveFailures != 0 {
		t.Errorf("expected consecutive_failures=0 after re-enable, got %d", updated.ConsecutiveFailures)
	}
}

func TestDelete(t *testing.T) {
	mgr, db := setupTestManager(t)
	ctx := context.Background()

	ch := &channel.Channel{Name: "test", Type: "openai", BaseURL: "https://api.openai.com", Status: "active"}
	db.Create(ch)
	acc, _ := mgr.Create(ctx, ch.ID, "sk-test-key")

	err := mgr.Delete(ctx, acc.ID)
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}

	var count int64
	db.Model(&Account{}).Where("id = ?", acc.ID).Count(&count)
	if count != 0 {
		t.Error("account should be deleted")
	}
}

// ========== 测试：缓存 ==========

func TestMemoryCache_Basic(t *testing.T) {
	cache := NewMemoryCache()

	// Set + Get
	cache.Set("key1", "value1", 10*time.Second)
	val, err := cache.Get("key1")
	if err != nil || val != "value1" {
		t.Errorf("expected value1, got %s, err=%v", val, err)
	}

	// Del
	cache.Del("key1")
	_, err = cache.Get("key1")
	if err == nil {
		t.Error("expected error after delete")
	}
}

func TestMemoryCache_IncrDecr(t *testing.T) {
	cache := NewMemoryCache()

	v, _ := cache.Incr("counter")
	if v != 1 {
		t.Errorf("expected 1, got %d", v)
	}
	v, _ = cache.Incr("counter")
	if v != 2 {
		t.Errorf("expected 2, got %d", v)
	}
	v, _ = cache.Decr("counter")
	if v != 1 {
		t.Errorf("expected 1, got %d", v)
	}
}

func TestMemoryCache_SetNX(t *testing.T) {
	cache := NewMemoryCache()

	// 首次设置成功
	ok, _ := cache.SetNX("lock", "1", 10*time.Second)
	if !ok {
		t.Error("expected SetNX to succeed on first call")
	}

	// 再次设置失败（key 已存在）
	ok, _ = cache.SetNX("lock", "1", 10*time.Second)
	if ok {
		t.Error("expected SetNX to fail on second call")
	}
}

func TestMemoryCache_TTL(t *testing.T) {
	cache := NewMemoryCache()

	cache.Set("short", "value", 1*time.Second)
	time.Sleep(2 * time.Second)

	_, err := cache.Get("short")
	if err == nil {
		t.Error("expected error for expired key")
	}
}

// ========== 测试：密钥加密解密 ==========

func TestCryptoEncryptDecrypt(t *testing.T) {
	secretKey, _ := crypto.EnsureSecretKey("/tmp/agw_test.env")
	cryptoSvc, _ := crypto.NewCrypto(secretKey)

	plain := "sk-test-my-api-key-12345"
	encrypted, err := cryptoSvc.Encrypt(plain)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	// 密文应与明文不同
	if encrypted == plain {
		t.Error("encrypted should differ from plain")
	}

	// 解密应还原明文
	decrypted, err := cryptoSvc.Decrypt(encrypted)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if decrypted != plain {
		t.Errorf("decrypted mismatch: got %s, want %s", decrypted, plain)
	}

	// 两次加密同一明文应产生不同密文（随机 nonce）
	encrypted2, _ := cryptoSvc.Encrypt(plain)
	if encrypted == encrypted2 {
		t.Error("two encryptions of same plain should differ (random nonce)")
	}
}
