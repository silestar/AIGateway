package account

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/bokelife/aigateway/internal/channel"
	"github.com/bokelife/aigateway/internal/config"
	"github.com/bokelife/aigateway/internal/crypto"
)

// Manager 账号管理器实现
type Manager struct {
	db        *gorm.DB
	cache     Cache
	cryptoSvc *crypto.CryptoService
	cfg       config.AccountManagerConfig
	logger    *zap.Logger
}

// NewManager 创建账号管理器
func NewManager(db *gorm.DB, cache Cache, cryptoSvc *crypto.CryptoService, cfg config.AccountManagerConfig, logger *zap.Logger) *Manager {
	return &Manager{
		db:        db,
		cache:     cache,
		cryptoSvc: cryptoSvc,
		cfg:       cfg,
		logger:    logger,
	}
}

// ========== 核心路由方法 ==========

// SelectAccount 选择账号（粘性→优先级→降级）
func (m *Manager) SelectAccount(ctx context.Context, keysID, channelID uint) (*Account, error) {
	// 1. 检查粘性绑定
	affinityKey := fmt.Sprintf("keys_account_affinity:%d:%d", keysID, channelID)
	if accountIDStr, err := m.cache.Get(affinityKey); err == nil && accountIDStr != "" {
		var acc Account
		if err := m.db.WithContext(ctx).Where("id = ? AND status = ?", accountIDStr, "active").First(&acc).Error; err == nil {
			return &acc, nil
		}
		// 绑定的账号不可用，清除绑定
		_ = m.cache.Del(affinityKey)
	}

	// 2. 按优先级选择 active 账号
	var accounts []Account
	if err := m.db.WithContext(ctx).Where("channel_id = ? AND status = ?", channelID, "active").Order("priority ASC").Find(&accounts).Error; err != nil {
		return nil, fmt.Errorf("query accounts: %w", err)
	}
	if len(accounts) == 0 {
		return nil, fmt.Errorf("no active account for channel %d", channelID)
	}

	// 3. 选择第一个，设置粘性绑定
	acc := accounts[0]
	ttl := time.Duration(m.cfg.AffinityTTL) * time.Second
	_ = m.cache.Set(affinityKey, fmt.Sprintf("%d", acc.ID), ttl)

	// 4. 更新活跃计数
	countKey := fmt.Sprintf("channel_active_count:%d", channelID)
	m.cache.Incr(countKey)

	return &acc, nil
}

// GetDecryptedAPIKey 获取解密后的 API Key（带缓存）
func (m *Manager) GetDecryptedAPIKey(ctx context.Context, accountID uint) (string, error) {
	// 检查缓存
	cacheKey := fmt.Sprintf("account_key_cache:%d", accountID)
	if cached, err := m.cache.Get(cacheKey); err == nil {
		return cached, nil
	}

	// 从 DB 读取并解密
	var acc Account
	if err := m.db.WithContext(ctx).First(&acc, accountID).Error; err != nil {
		return "", fmt.Errorf("get account: %w", err)
	}

	plainKey, err := m.cryptoSvc.Decrypt(acc.APIKeyEncrypted)
	if err != nil {
		return "", fmt.Errorf("decrypt api key: %w", err)
	}

	// 写入缓存
	ttl := time.Duration(m.cfg.AccountKeyCacheTTL) * time.Second
	_ = m.cache.Set(cacheKey, plainKey, ttl)

	return plainKey, nil
}

// ReportResult 报告请求结果（故障降级逻辑）
func (m *Manager) ReportResult(ctx context.Context, accountID uint, success bool, statusCode int) error {
	var acc Account
	if err := m.db.WithContext(ctx).First(&acc, accountID).Error; err != nil {
		return err
	}

	if success {
		// 成功：重置连续失败计数
		if acc.ConsecutiveFailures > 0 {
			return m.db.WithContext(ctx).Model(&Account{}).Where("id = ?", accountID).
				Updates(map[string]interface{}{"consecutive_failures": 0}).Error
		}
		return nil
	}

	// 判断是否计入失败
	if !isFailureCountable(statusCode) {
		m.logger.Debug("failure not countable",
			zap.Uint("account_id", accountID),
			zap.Int("status_code", statusCode),
		)
		return nil
	}

	// 增加连续失败计数
	newFailures := acc.ConsecutiveFailures + 1
	updates := map[string]interface{}{
		"consecutive_failures": newFailures,
	}

	// 达到阈值 → 禁用
	if newFailures >= m.cfg.ConsecutiveFailureThreshold {
		updates["status"] = "disabled"
		now := time.Now()
		updates["last_failed_at"] = now

		m.logger.Warn("account disabled due to consecutive failures",
			zap.Uint("account_id", accountID),
			zap.Uint("channel_id", acc.ChannelID),
			zap.Int("consecutive_failures", newFailures),
		)

		// 清除粘性绑定和 key 缓存
		m.clearAccountBindings(ctx, &acc)
	}

	return m.db.WithContext(ctx).Model(&Account{}).Where("id = ?", accountID).Updates(updates).Error
}

// ========== CRUD 方法 ==========

func (m *Manager) Create(ctx context.Context, channelID uint, apiKey string) (*Account, error) {
	encrypted, err := m.cryptoSvc.Encrypt(apiKey)
	if err != nil {
		return nil, fmt.Errorf("encrypt api key: %w", err)
	}

	var maxPriority int
	m.db.WithContext(ctx).Model(&Account{}).Where("channel_id = ?", channelID).
		Select("COALESCE(MAX(priority), -1)").Scan(&maxPriority)

	// 提取前缀用于脱敏展示，如 sk-abc... → sk-abc****
	prefix := apiKey
	if len(prefix) > 8 {
		prefix = prefix[:8]
	}

	acc := &Account{
		ChannelID:       channelID,
		APIKeyEncrypted: encrypted,
		APIKeyPrefix:    prefix,
		Priority:        maxPriority + 1,
		Status:          "active",
	}

	if err := m.db.WithContext(ctx).Create(acc).Error; err != nil {
		return nil, fmt.Errorf("create account: %w", err)
	}
	return acc, nil
}

func (m *Manager) GetById(ctx context.Context, id uint) (*Account, error) {
	var acc Account
	if err := m.db.WithContext(ctx).First(&acc, id).Error; err != nil {
		return nil, err
	}
	return &acc, nil
}

func (m *Manager) ListByChannel(ctx context.Context, channelID uint) ([]Account, error) {
	var accounts []Account
	if err := m.db.WithContext(ctx).Where("channel_id = ?", channelID).Order("priority ASC").Find(&accounts).Error; err != nil {
		return nil, err
	}
	return accounts, nil
}

func (m *Manager) UpdatePriority(ctx context.Context, id uint, priority int) error {
	// 获取账号信息用于清除粘性绑定
	var acc Account
	if err := m.db.WithContext(ctx).First(&acc, id).Error; err != nil {
		return err
	}

	err := m.db.WithContext(ctx).Model(&Account{}).Where("id = ?", id).Update("priority", priority).Error
	if err != nil {
		return err
	}

	// 清除该渠道所有消费者的粘性绑定
	m.clearChannelAffinities(acc.ChannelID)
	return nil
}

func (m *Manager) UpdateStatus(ctx context.Context, id uint, status string) error {
	updates := map[string]interface{}{"status": status}
	if status == "active" {
		// 手动恢复时重置失败计数
		updates["consecutive_failures"] = 0
		updates["probe_cooldown_until"] = nil
	}
	return m.db.WithContext(ctx).Model(&Account{}).Where("id = ?", id).Updates(updates).Error
}

func (m *Manager) RevealKey(ctx context.Context, id uint) (string, error) {
	var acc Account
	if err := m.db.WithContext(ctx).First(&acc, id).Error; err != nil {
		return "", err
	}
	plainKey, err := m.cryptoSvc.Decrypt(acc.APIKeyEncrypted)
	if err != nil {
		return "", err
	}
	// TODO: 审计日志记录
	return plainKey, nil
}

func (m *Manager) Delete(ctx context.Context, id uint) error {
	return m.db.WithContext(ctx).Delete(&Account{}, id).Error
}

// ========== 辅助方法 ==========

// clearAccountBindings 清除账号相关的缓存绑定
func (m *Manager) clearAccountBindings(ctx context.Context, acc *Account) {
	// 清除 key 缓存
	keyCacheKey := fmt.Sprintf("account_key_cache:%d", acc.ID)
	_ = m.cache.Del(keyCacheKey)

	// 减少活跃计数
	countKey := fmt.Sprintf("channel_active_count:%d", acc.ChannelID)
	m.cache.Decr(countKey)
}

// clearChannelAffinities 清除渠道所有粘性绑定
// 简化实现：由于内存缓存没有 scan 能力，这里只能清除已知的绑定
func (m *Manager) clearChannelAffinities(channelID uint) {
	// 内存缓存下无法遍历所有 key，跳过
	// Redis 实现下可用 SCAN keys_account_affinity:*:channelID
	m.logger.Debug("clear channel affinities", zap.Uint("channel_id", channelID))
}

// isFailureCountable 判断 HTTP 状态码是否计入失败
func isFailureCountable(statusCode int) bool {
	// 5xx: 服务端错误 → 计入
	// 429: 限流 → 计入
	// 其他 4xx: 客户端错误 → 不计入
	return statusCode >= 500 || statusCode == 429
}

// generateAPIKey 生成随机 API Key
func generateAPIKey() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return "sk-agw-" + hex.EncodeToString(bytes), nil
}

// getActiveCount 获取渠道活跃账号数
func (m *Manager) getActiveCount(ctx context.Context, channelID uint) int64 {
	var count int64
	m.db.WithContext(ctx).Model(&Account{}).Where("channel_id = ? AND status = ?", channelID, "active").Count(&count)
	return count
}

// getTotalCount 获取渠道总账号数
func (m *Manager) getTotalCount(ctx context.Context, channelID uint) int64 {
	var count int64
	m.db.WithContext(ctx).Model(&Account{}).Where("channel_id = ?", channelID).Count(&count)
	return count
}

// probeAccount 探测账号可用性
func (m *Manager) probeAccount(ctx context.Context, ch *channel.Channel, acc *Account) bool {
	plainKey, err := m.GetDecryptedAPIKey(ctx, acc.ID)
	if err != nil {
		return false
	}

	// 发送最小探测请求
	// TODO: 实际通过适配器发送探测请求
	_ = plainKey
	_ = ch
	return false
}
