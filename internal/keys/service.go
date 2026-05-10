package keys

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/silestar/AIGateway/internal/account"
	"github.com/silestar/AIGateway/internal/crypto"
	"gorm.io/gorm"
)

type service struct {
	db     *gorm.DB
	cache  account.Cache
	crypto *crypto.CryptoService
}

// NewService 创建密钥服务
func NewService(db *gorm.DB) KeysService {
	return &service{db: db}
}

// SetCrypto 注入加密服务（在 main.go 中调用）
func (s *service) SetCrypto(c *crypto.CryptoService) {
	s.crypto = c
}

// Create 创建密钥，返回对象+明文密钥
func (s *service) Create(ctx context.Context, name string) (*Keys, string, error) {
	apiKey, hash, err := generateAPIKey()
	if err != nil {
		return nil, "", fmt.Errorf("generate api key: %w", err)
	}

	// 提取前缀 sk-agw-xxxx（前10位）
	prefix := apiKey
	if len(prefix) > 10 {
		prefix = prefix[:10]
	}

	k := &Keys{
		Name:         name,
		APIKeyHash:   hash,
		APIKeyPrefix: prefix,
		Status:       "active",
	}

	// 加密存储原文
	if s.crypto != nil {
		encrypted, err := s.crypto.Encrypt(apiKey)
		if err != nil {
			return nil, "", fmt.Errorf("encrypt api key: %w", err)
		}
		k.APIKeyEncrypted = encrypted
	}

	if err := s.db.WithContext(ctx).Create(k).Error; err != nil {
		return nil, "", fmt.Errorf("create keys: %w", err)
	}

	return k, apiKey, nil
}

// Authenticate 认证密钥
func (s *service) Authenticate(ctx context.Context, apiKey string) (*Keys, error) {
	hash := sha256Hash(apiKey)

	var k Keys
	if err := s.db.WithContext(ctx).Where("api_key_hash = ? AND status = ?", hash, "active").First(&k).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("invalid api key")
		}
		return nil, fmt.Errorf("authenticate: %w", err)
	}

	return &k, nil
}

// GetById 根据 ID 获取密钥
func (s *service) GetById(ctx context.Context, id uint) (*Keys, error) {
	var k Keys
	if err := s.db.WithContext(ctx).First(&k, id).Error; err != nil {
		return nil, err
	}
	return &k, nil
}

// List 获取密钥列表
func (s *service) List(ctx context.Context, filter ListFilter) ([]Keys, int64, error) {
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize < 1 || filter.PageSize > 100 {
		filter.PageSize = 20
	}

	var keysList []Keys
	var total int64

	query := s.db.WithContext(ctx).Model(&Keys{})
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
	if err := query.Order("id DESC").Offset(offset).Limit(filter.PageSize).Find(&keysList).Error; err != nil {
		return nil, 0, err
	}

	return keysList, total, nil
}

// Update 更新密钥信息
func (s *service) Update(ctx context.Context, id uint, name string) error {
	return s.db.WithContext(ctx).Model(&Keys{}).Where("id = ?", id).Update("name", name).Error
}

// Delete 删除密钥
func (s *service) Delete(ctx context.Context, id uint) error {
	return s.db.WithContext(ctx).Delete(&Keys{}, id).Error
}

// ResetKey 重置密钥
func (s *service) ResetKey(ctx context.Context, id uint) (string, error) {
	apiKey, hash, err := generateAPIKey()
	if err != nil {
		return "", err
	}

	prefix := apiKey
	if len(prefix) > 10 {
		prefix = prefix[:10]
	}

	updates := map[string]interface{}{
		"api_key_hash":   hash,
		"api_key_prefix": prefix,
	}

	if s.crypto != nil {
		encrypted, err := s.crypto.Encrypt(apiKey)
		if err != nil {
			return "", fmt.Errorf("encrypt api key: %w", err)
		}
		updates["api_key_encrypted"] = encrypted
	}

	if err := s.db.WithContext(ctx).Model(&Keys{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return "", err
	}

	return apiKey, nil
}

// RevealKey 查看密钥（解密返回明文）
func (s *service) RevealKey(ctx context.Context, id uint) (string, error) {
	var k Keys
	if err := s.db.WithContext(ctx).Select("id", "api_key_encrypted").First(&k, id).Error; err != nil {
		return "", fmt.Errorf("keys not found")
	}

	if k.APIKeyEncrypted == "" || s.crypto == nil {
		return "", fmt.Errorf("api key cannot be revealed, encrypted data not available. Use reset-key to generate a new one")
	}

	plaintext, err := s.crypto.Decrypt(k.APIKeyEncrypted)
	if err != nil {
		return "", fmt.Errorf("decrypt api key: %w", err)
	}

	return plaintext, nil
}

// UpdateStatus 更新密钥状态
func (s *service) UpdateStatus(ctx context.Context, id uint, status string) error {
	if status != "active" && status != "disabled" {
		return fmt.Errorf("invalid status: %s", status)
	}
	return s.db.WithContext(ctx).Model(&Keys{}).Where("id = ?", id).Update("status", status).Error
}

// generateAPIKey 生成 API Key
func generateAPIKey() (string, string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", "", err
	}
	apiKey := "sk-agw-" + hex.EncodeToString(bytes)
	hash := sha256Hash(apiKey)
	return apiKey, hash, nil
}

func sha256Hash(s string) string {
	h := sha256.New()
	h.Write([]byte(s))
	return hex.EncodeToString(h.Sum(nil))
}