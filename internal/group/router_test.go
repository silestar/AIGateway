package group

import (
	"context"
	"fmt"
	"testing"

	"github.com/bokelife/aigateway/internal/account"
	"github.com/bokelife/aigateway/internal/channel"
	"github.com/bokelife/aigateway/internal/consumer"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// ========== Mock AccountManager ==========

type mockAccountManager struct {
	selectAccountFn func(ctx context.Context, consumerID, channelID uint) (*account.Account, error)
}

func (m *mockAccountManager) SelectAccount(ctx context.Context, consumerID, channelID uint) (*account.Account, error) {
	if m.selectAccountFn != nil {
		return m.selectAccountFn(ctx, consumerID, channelID)
	}
	return &account.Account{ID: 1, ChannelID: channelID, Status: "active", Priority: 0}, nil
}
func (m *mockAccountManager) GetDecryptedAPIKey(ctx context.Context, id uint) (string, error) { return "sk-test", nil }
func (m *mockAccountManager) ReportResult(ctx context.Context, id uint, success bool, statusCode int) error {
	return nil
}
func (m *mockAccountManager) Create(ctx context.Context, channelID uint, apiKey string) (*account.Account, error) {
	return nil, nil
}
func (m *mockAccountManager) GetById(ctx context.Context, id uint) (*account.Account, error) { return nil, nil }
func (m *mockAccountManager) ListByChannel(ctx context.Context, channelID uint) ([]account.Account, error) {
	return nil, nil
}
func (m *mockAccountManager) UpdatePriority(ctx context.Context, id uint, priority int) error  { return nil }
func (m *mockAccountManager) UpdateStatus(ctx context.Context, id uint, status string) error    { return nil }
func (m *mockAccountManager) RevealKey(ctx context.Context, id uint) (string, error)           { return "", nil }
func (m *mockAccountManager) Delete(ctx context.Context, id uint) error                        { return nil }
func (m *mockAccountManager) StartProbeScheduler()                                            {}
func (m *mockAccountManager) StartGlobalHealthCheck()                                          {}

// ========== Helper ==========

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	// 迁移所有表
	db.AutoMigrate(
		&consumer.Consumer{},
		&consumer.ConsumerGroup{},
		&consumer.ConsumerGroupMember{},
		&channel.Channel{},
		&channel.ChannelGroup{},
		&channel.ChannelGroupMember{},
		&channel.ChannelModel{},
		&channel.ConsumerGroupChannelGroup{},
		&account.Account{},
	)
	return db
}

func setupRouter(t *testing.T) (*Router, *gorm.DB) {
	t.Helper()
	db := setupTestDB(t)
	logger := zap.NewNop()
	am := &mockAccountManager{}
	r := NewRouter(db, nil, am, logger)
	return r, db
}

// ========== Tests ==========

func TestCreateChannelGroup(t *testing.T) {
	r, _ := setupRouter(t)
	ctx := context.Background()

	cg, err := r.CreateChannelGroup(ctx, "premium", "高优先级渠道组", 100)
	if err != nil {
		t.Fatalf("CreateChannelGroup: %v", err)
	}
	if cg.ID == 0 || cg.Name != "premium" || cg.Weight != 100 {
		t.Errorf("unexpected channel group: %+v", cg)
	}
}

func TestCreateConsumerGroup(t *testing.T) {
	r, _ := setupRouter(t)
	ctx := context.Background()

	cg, err := r.CreateConsumerGroup(ctx, "vip", "VIP用户组")
	if err != nil {
		t.Fatalf("CreateConsumerGroup: %v", err)
	}
	if cg.ID == 0 || cg.Name != "vip" {
		t.Errorf("unexpected consumer group: %+v", cg)
	}
}

func TestAddRemoveChannelFromGroup(t *testing.T) {
	r, db := setupRouter(t)
	ctx := context.Background()

	// 创建渠道分组
	cg, _ := r.CreateChannelGroup(ctx, "test-group", "", 10)

	// 创建渠道
	ch := &channel.Channel{Name: "openai-1", Type: "openai", BaseURL: "https://api.openai.com", Status: "active", Weight: 50}
	db.Create(ch)

	// 添加渠道到分组
	if err := r.AddChannelToGroup(ctx, cg.ID, ch.ID, 80); err != nil {
		t.Fatalf("AddChannelToGroup: %v", err)
	}

	// 验证成员存在
	var count int64
	db.Model(&channel.ChannelGroupMember{}).Where("group_id = ? AND channel_id = ?", cg.ID, ch.ID).Count(&count)
	if count != 1 {
		t.Errorf("expected 1 member, got %d", count)
	}

	// 移除渠道
	if err := r.RemoveChannelFromGroup(ctx, cg.ID, ch.ID); err != nil {
		t.Fatalf("RemoveChannelFromGroup: %v", err)
	}

	db.Model(&channel.ChannelGroupMember{}).Where("group_id = ? AND channel_id = ?", cg.ID, ch.ID).Count(&count)
	if count != 0 {
		t.Errorf("expected 0 members after remove, got %d", count)
	}
}

func TestAddRemoveConsumerFromGroup(t *testing.T) {
	r, db := setupRouter(t)
	ctx := context.Background()

	// 创建消费者分组
	cg, _ := r.CreateConsumerGroup(ctx, "free", "免费用户")
	// 创建消费者
	c := &consumer.Consumer{Name: "user1", APIKeyHash: "fakehash1234567890123456789012345678", Status: "active"}
	db.Create(c)

	// 添加消费者到分组
	if err := r.AddConsumerToGroup(ctx, cg.ID, c.ID, 10, 1000); err != nil {
		t.Fatalf("AddConsumerToGroup: %v", err)
	}

	var member consumer.ConsumerGroupMember
	db.Where("group_id = ? AND consumer_id = ?", cg.ID, c.ID).First(&member)
	if member.QuotaRPM != 10 || member.QuotaTPM != 1000 {
		t.Errorf("unexpected quota: rpm=%d tpm=%d", member.QuotaRPM, member.QuotaTPM)
	}

	// 移除
	if err := r.RemoveConsumerFromGroup(ctx, cg.ID, c.ID); err != nil {
		t.Fatalf("RemoveConsumerFromGroup: %v", err)
	}
}

func TestBindUnbindChannelGroup(t *testing.T) {
	r, _ := setupRouter(t)
	ctx := context.Background()

	// 创建消费者分组 + 渠道分组
	consumerGrp, _ := r.CreateConsumerGroup(ctx, "team-a", "团队A")
	channelGrp, _ := r.CreateChannelGroup(ctx, "prod-channels", "生产渠道", 50)

	// 绑定
	if err := r.BindChannelGroup(ctx, consumerGrp.ID, channelGrp.ID); err != nil {
		t.Fatalf("BindChannelGroup: %v", err)
	}

	// 解绑
	if err := r.UnbindChannelGroup(ctx, consumerGrp.ID, channelGrp.ID); err != nil {
		t.Fatalf("UnbindChannelGroup: %v", err)
	}
}

func TestDeleteChannelGroup(t *testing.T) {
	r, _ := setupRouter(t)
	ctx := context.Background()

	cg, _ := r.CreateChannelGroup(ctx, "to-delete", "", 10)
	if err := r.DeleteChannelGroup(ctx, cg.ID); err != nil {
		t.Fatalf("DeleteChannelGroup: %v", err)
	}

	var count int64
	db := r.db
	db.Model(&channel.ChannelGroup{}).Where("id = ?", cg.ID).Count(&count)
	if count != 0 {
		t.Errorf("channel group should be deleted")
	}
}

func TestDeleteConsumerGroup(t *testing.T) {
	r, db := setupRouter(t)
	ctx := context.Background()

	cg, _ := r.CreateConsumerGroup(ctx, "to-delete", "")
	c := &consumer.Consumer{Name: "u", APIKeyHash: "fakehash0000111122223333444455556666", Status: "active"}
	db.Create(c)
	r.AddConsumerToGroup(ctx, cg.ID, c.ID, 0, 0)

	if err := r.DeleteConsumerGroup(ctx, cg.ID); err != nil {
		t.Fatalf("DeleteConsumerGroup: %v", err)
	}

	var count int64
	db.Model(&consumer.ConsumerGroup{}).Where("id = ?", cg.ID).Count(&count)
	if count != 0 {
		t.Errorf("consumer group should be deleted")
	}
}

func TestRoute_NoGroupAssignment(t *testing.T) {
	r, _ := setupRouter(t)
	ctx := context.Background()

	_, err := r.Route(ctx, 999, "gpt-4")
	if err == nil {
		t.Fatal("expected error for consumer with no group")
	}
}

func TestRoute_FullChain(t *testing.T) {
	r, db := setupRouter(t)
	ctx := context.Background()

	// 创建渠道
	ch := &channel.Channel{Name: "openai-main", Type: "openai", BaseURL: "https://api.openai.com", Status: "active", Weight: 100}
	db.Create(ch)

	// 创建渠道模型映射
	cm := &channel.ChannelModel{ChannelID: ch.ID, ActualModelName: "gpt-4", DisplayModelName: "gpt-4", Status: "enabled"}
	db.Create(cm)

	// 创建账号
	acc := &account.Account{ChannelID: ch.ID, APIKeyEncrypted: "enc123", Status: "active", Priority: 0}
	db.Create(acc)

	// 创建消费者
	cons := &consumer.Consumer{Name: "test-user", APIKeyHash: "hashfullchaintest123456789012345", Status: "active"}
	db.Create(cons)

	// 创建消费者分组
	consumerGrp, _ := r.CreateConsumerGroup(ctx, "team", "")
	r.AddConsumerToGroup(ctx, consumerGrp.ID, cons.ID, 0, 0)

	// 创建渠道分组
	channelGrp, _ := r.CreateChannelGroup(ctx, "main-channels", "", 50)
	r.AddChannelToGroup(ctx, channelGrp.ID, ch.ID, 100)

	// 绑定消费者分组 → 渠道分组
	r.BindChannelGroup(ctx, consumerGrp.ID, channelGrp.ID)

	// 路由
	result, err := r.Route(ctx, cons.ID, "gpt-4")
	if err != nil {
		t.Fatalf("Route: %v", err)
	}
	if result.Channel.ID != ch.ID {
		t.Errorf("expected channel ID %d, got %d", ch.ID, result.Channel.ID)
	}
	if result.Account.ID != acc.ID {
		t.Errorf("expected account ID %d, got %d", acc.ID, result.Account.ID)
	}
}

func TestRoute_ModelNotFound(t *testing.T) {
	r, db := setupRouter(t)
	ctx := context.Background()

	ch := &channel.Channel{Name: "ch", Type: "openai", BaseURL: "https://api.openai.com", Status: "active", Weight: 100}
	db.Create(ch)

	cons := &consumer.Consumer{Name: "u", APIKeyHash: "hashmodelnotfound1234567890123456", Status: "active"}
	db.Create(cons)

	consumerGrp, _ := r.CreateConsumerGroup(ctx, "g", "")
	r.AddConsumerToGroup(ctx, consumerGrp.ID, cons.ID, 0, 0)

	channelGrp, _ := r.CreateChannelGroup(ctx, "cg", "", 10)
	r.AddChannelToGroup(ctx, channelGrp.ID, ch.ID, 10)
	r.BindChannelGroup(ctx, consumerGrp.ID, channelGrp.ID)

	// 没有 gpt-4 模型映射
	_, err := r.Route(ctx, cons.ID, "gpt-4")
	if err == nil {
		t.Fatal("expected error for missing model")
	}
}

func TestRoute_NoAvailableAccount(t *testing.T) {
	r, db := setupRouter(t)
	ctx := context.Background()

	ch := &channel.Channel{Name: "ch2", Type: "openai", BaseURL: "https://api.openai.com", Status: "active", Weight: 100}
	db.Create(ch)

	cm := &channel.ChannelModel{ChannelID: ch.ID, ActualModelName: "gpt-4", DisplayModelName: "gpt-4", Status: "enabled"}
	db.Create(cm)

	cons := &consumer.Consumer{Name: "u2", APIKeyHash: "hashnoaccount123456789012345678", Status: "active"}
	db.Create(cons)

	consumerGrp, _ := r.CreateConsumerGroup(ctx, "g2", "")
	r.AddConsumerToGroup(ctx, consumerGrp.ID, cons.ID, 0, 0)

	channelGrp, _ := r.CreateChannelGroup(ctx, "cg2", "", 10)
	r.AddChannelToGroup(ctx, channelGrp.ID, ch.ID, 10)
	r.BindChannelGroup(ctx, consumerGrp.ID, channelGrp.ID)

	// Mock SelectAccount 返回错误
	r.accountMgr = &mockAccountManager{
		selectAccountFn: func(ctx context.Context, consumerID, channelID uint) (*account.Account, error) {
			return nil, fmt.Errorf("no available account")
		},
	}

	_, err := r.Route(ctx, cons.ID, "gpt-4")
	if err == nil {
		t.Fatal("expected error when no account available")
	}
}
