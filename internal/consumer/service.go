package consumer

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/bokelife/aigateway/internal/account"
	"gorm.io/gorm"
)

type service struct {
	db    *gorm.DB
	cache account.Cache
}

// NewService 创建消费者服务
func NewService(db *gorm.DB) ConsumerService {
	return &service{db: db}
}

// Create 创建消费者，返回对象+明文密钥
func (s *service) Create(ctx context.Context, name string) (*Consumer, string, error) {
	apiKey, hash, err := generateAPIKey()
	if err != nil {
		return nil, "", fmt.Errorf("generate api key: %w", err)
	}

	c := &Consumer{
		Name:       name,
		APIKeyHash: hash,
		Status:     "active",
	}

	if err := s.db.WithContext(ctx).Create(c).Error; err != nil {
		return nil, "", fmt.Errorf("create consumer: %w", err)
	}

	return c, apiKey, nil
}

// Authenticate 认证消费者
func (s *service) Authenticate(ctx context.Context, apiKey string) (*Consumer, error) {
	hash := sha256Hash(apiKey)

	var c Consumer
	if err := s.db.WithContext(ctx).Where("api_key_hash = ? AND status = ?", hash, "active").First(&c).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("invalid api key")
		}
		return nil, fmt.Errorf("authenticate: %w", err)
	}

	return &c, nil
}

// GetById 根据 ID 获取消费者
func (s *service) GetById(ctx context.Context, id uint) (*Consumer, error) {
	var c Consumer
	if err := s.db.WithContext(ctx).First(&c, id).Error; err != nil {
		return nil, err
	}
	return &c, nil
}

// List 获取消费者列表
func (s *service) List(ctx context.Context, filter ListFilter) ([]Consumer, int64, error) {
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize < 1 || filter.PageSize > 100 {
		filter.PageSize = 20
	}

	var consumers []Consumer
	var total int64

	query := s.db.WithContext(ctx).Model(&Consumer{})
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	if filter.Name != "" {
		query = query.Where("name LIKE ?", "%"+filter.Name+"%")
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (filter.Page - 1) * filter.PageSize
	if err := query.Order("id DESC").Offset(offset).Limit(filter.PageSize).Find(&consumers).Error; err != nil {
		return nil, 0, err
	}

	return consumers, total, nil
}

// Update 更新消费者信息
func (s *service) Update(ctx context.Context, id uint, name string) error {
	return s.db.WithContext(ctx).Model(&Consumer{}).Where("id = ?", id).Update("name", name).Error
}

// Delete 删除消费者
func (s *service) Delete(ctx context.Context, id uint) error {
	return s.db.WithContext(ctx).Delete(&Consumer{}, id).Error
}

// ResetKey 重置密钥
func (s *service) ResetKey(ctx context.Context, id uint) (string, error) {
	apiKey, hash, err := generateAPIKey()
	if err != nil {
		return "", err
	}

	if err := s.db.WithContext(ctx).Model(&Consumer{}).Where("id = ?", id).Update("api_key_hash", hash).Error; err != nil {
		return "", err
	}

	return apiKey, nil
}

// RevealKey 查看密钥（审计日志）
func (s *service) RevealKey(ctx context.Context, id uint) (string, error) {
	// 注意：由于只存储哈希，无法反查明文密钥
	// 此方法仅记录审计日志，实际密钥需要 ResetKey 重新生成
	return "", fmt.Errorf("api key cannot be revealed, only hash is stored. Use reset-key to generate a new one")
}

// UpdateStatus 更新消费者状态
func (s *service) UpdateStatus(ctx context.Context, id uint, status string) error {
	if status != "active" && status != "disabled" {
		return fmt.Errorf("invalid status: %s", status)
	}
	return s.db.WithContext(ctx).Model(&Consumer{}).Where("id = ?", id).Update("status", status).Error
}

// generateAPIKey 生成 API Key
// 格式: sk-agw-{32字节随机hex}
func generateAPIKey() (string, string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", "", err
	}
	apiKey := "sk-agw-" + hex.EncodeToString(bytes)
	hash := sha256Hash(apiKey)
	return apiKey, hash, nil
}

// sha256Hash 计算 SHA256 哈希
func sha256Hash(s string) string {
	h := sha256.New()
	h.Write([]byte(s))
	return hex.EncodeToString(h.Sum(nil))
}
