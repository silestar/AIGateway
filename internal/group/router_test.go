package group

import (
	"context"
	"fmt"
	"testing"

	"github.com/silestar/AIGateway/internal/account"
	"github.com/silestar/AIGateway/internal/apikey"
	"github.com/silestar/AIGateway/internal/channel"
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
func (m *mockAccountManager) UpdatePriority(ctx context.Context, id uint, priority int) error { return nil }
func (m *mockAccountManager) UpdateStatus(ctx context.Context, id uint, status string) error   { return nil }
func (m *mockAccountManager) RevealKey(ctx context.Context, id uint) (string, error)           { return "", nil }
func (m *mockAccountManager) Delete(ctx context.Context, id uint) error                        { return nil }
func (m *mockAccountManager) StartProbeScheduler()                                             {}
func (m *mockAccountManager) StartGlobalHealthCheck()                                           {}

// ========== Helper ==========

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	db.AutoMigrate(
		&apikey.ApiKey{},
		&apikey.KeyGroup{},
		&apikey.KeyGroupMember{},
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

func TestCreateKeyGroup(t *testing.T) {
	r, _ := setupRouter(t)
	ctx := context.Background()

	cg, err := r.CreateKeyGroup(ctx, "vip", "VIP用户组")
	if err != nil {
		t.Fatalf("CreateKeyGroup: %v", err)
	}
	if cg.ID == 0 || cg.Name != "vip" {
		t.Errorf("unexpected key group: %+v", cg)
	}
}

func TestAddRemoveChannelFromGroup(t *testing.T) {
	r, db := setupRouter(t)
	ctx := context.Background()

	cg, _ := r.CreateChannelGroup(ctx, "test-group", "", 10)
	ch := &channel.Channel{Name: "openai-1", Type: "openai", BaseURL: "https://api.openai.com", Status: "active", Weight: 50}
	db.Create(ch)

	if err := r.AddChannelToGroup(ctx, cg.ID, ch.ID, 80); err != nil {
		t.Fatalf("AddChannelToGroup: %v", err)
	}

	var count int64
	db.Model(&channel.ChannelGroupMember{}).Where("group_id = ? AND channel_id = ?", cg.ID, ch.ID).Count(&count)
	if count != 1 {
		t.Errorf("expected 1 member, got %d", count)
	}

	if err := r.RemoveChannelFromGroup(ctx, cg.ID, ch.ID); err != nil {
		t.Fatalf("RemoveChannelFromGroup: %v", err)
	}

	db.Model(&channel.ChannelGroupMember{}).Where("group_id = ? AND channel_id = ?", cg.ID, ch.ID).Count(&count)
	if count != 0 {
		t.Errorf("expected 0 members after remove, got %d", count)
	}
}

func TestAddRemoveKeyFromGroup(t *testing.T) {
	r, db := setupRouter(t)
	ctx := context.Background()

	cg, _ := r.CreateKeyGroup(ctx, "free", "免费用户")
	k := &apikey.ApiKey{Name: "user1", APIKeyHash: "fakehash1234567890123456789012345678", Status: "active"}
	db.Create(k)

	if err := r.AddKeyToGroup(ctx, cg.ID, k.ID, 10, 1000); err != nil {
		t.Fatalf("AddKeyToGroup: %v", err)
	}

	var member apikey.KeyGroupMember
	db.Where("group_id = ? AND key_id = ?", cg.ID, k.ID).First(&member)
	if member.QuotaRPM != 10 || member.QuotaTPM != 1000 {
		t.Errorf("unexpected quota: rpm=%d tpm=%d", member.QuotaRPM, member.QuotaTPM)
	}

	if err := r.RemoveKeyFromGroup(ctx, cg.ID, k.ID); err != nil {
		t.Fatalf("RemoveKeyFromGroup: %v", err)
	}
}

func TestBindUnbindChannelGroup(t *testing.T) {
	r, _ := setupRouter(t)
	ctx := context.Background()

	keyGrp, _ := r.CreateKeyGroup(ctx, "team-a", "团队A")
	channelGrp, _ := r.CreateChannelGroup(ctx, "prod-channels", "生产渠道", 50)

	if err := r.BindChannelGroup(ctx, keyGrp.ID, channelGrp.ID); err != nil {
		t.Fatalf("BindChannelGroup: %v", err)
	}

	if err := r.UnbindChannelGroup(ctx, keyGrp.ID, channelGrp.ID); err != nil {
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

func TestDeleteKeyGroup(t *testing.T) {
	r, db := setupRouter(t)
	ctx := context.Background()

	cg, _ := r.CreateKeyGroup(ctx, "to-delete", "")
	k := &apikey.ApiKey{Name: "u", APIKeyHash: "fakehash0000111122223333444455556666", Status: "active"}
	db.Create(k)
	r.AddKeyToGroup(ctx, cg.ID, k.ID, 0, 0)

	if err := r.DeleteKeyGroup(ctx, cg.ID); err != nil {
		t.Fatalf("DeleteKeyGroup: %v", err)
	}

	var count int64
	db.Model(&apikey.KeyGroup{}).Where("id = ?", cg.ID).Count(&count)
	if count != 0 {
		t.Errorf("key group should be deleted")
	}
}

func TestRoute_NoGroupAssignment(t *testing.T) {
	r, _ := setupRouter(t)
	ctx := context.Background()

	_, err := r.Route(ctx, 999, "gpt-4")
	if err == nil {
		t.Fatal("expected error for key with no group")
	}
}

func TestRoute_FullChain(t *testing.T) {
	r, db := setupRouter(t)
	ctx := context.Background()

	ch := &channel.Channel{Name: "openai-main", Type: "openai", BaseURL: "https://api.openai.com", Status: "active", Weight: 100}
	db.Create(ch)

	cm := &channel.ChannelModel{ChannelID: ch.ID, ActualModelName: "gpt-4", DisplayModelName: "gpt-4", Status: "enabled"}
	db.Create(cm)

	acc := &account.Account{ChannelID: ch.ID, APIKeyEncrypted: "enc123", Status: "active", Priority: 0}
	db.Create(acc)

	k := &apikey.ApiKey{Name: "test-user", APIKeyHash: "hashfullchaintest123456789012345", Status: "active"}
	db.Create(k)

	keyGrp, _ := r.CreateKeyGroup(ctx, "team", "")
	r.AddKeyToGroup(ctx, keyGrp.ID, k.ID, 0, 0)

	channelGrp, _ := r.CreateChannelGroup(ctx, "main-channels", "", 50)
	r.AddChannelToGroup(ctx, channelGrp.ID, ch.ID, 100)

	r.BindChannelGroup(ctx, keyGrp.ID, channelGrp.ID)

	result, err := r.Route(ctx, k.ID, "gpt-4")
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

	k := &apikey.ApiKey{Name: "u", APIKeyHash: "hashmodelnotfound1234567890123456", Status: "active"}
	db.Create(k)

	keyGrp, _ := r.CreateKeyGroup(ctx, "g", "")
	r.AddKeyToGroup(ctx, keyGrp.ID, k.ID, 0, 0)

	channelGrp, _ := r.CreateChannelGroup(ctx, "cg", "", 10)
	r.AddChannelToGroup(ctx, channelGrp.ID, ch.ID, 10)
	r.BindChannelGroup(ctx, keyGrp.ID, channelGrp.ID)

	_, err := r.Route(ctx, k.ID, "gpt-4")
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

	k := &apikey.ApiKey{Name: "u2", APIKeyHash: "hashnoaccount123456789012345678", Status: "active"}
	db.Create(k)

	keyGrp, _ := r.CreateKeyGroup(ctx, "g2", "")
	r.AddKeyToGroup(ctx, keyGrp.ID, k.ID, 0, 0)

	channelGrp, _ := r.CreateChannelGroup(ctx, "cg2", "", 10)
	r.AddChannelToGroup(ctx, channelGrp.ID, ch.ID, 10)
	r.BindChannelGroup(ctx, keyGrp.ID, channelGrp.ID)

	r.accountMgr = &mockAccountManager{
		selectAccountFn: func(ctx context.Context, consumerID, channelID uint) (*account.Account, error) {
			return nil, fmt.Errorf("no available account")
		},
	}

	_, err := r.Route(ctx, k.ID, "gpt-4")
	if err == nil {
		t.Fatal("expected error when no account available")
	}
}