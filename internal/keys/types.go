package keys

import (
	"context"
	"time"

	"github.com/silestar/AIGateway/internal/account"
	"github.com/silestar/AIGateway/internal/crypto"
)

// Keys 密钥模型
type Keys struct {
	ID               uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	Name             string    `gorm:"size:100;not null" json:"name"`
	APIKeyHash       string    `gorm:"size:64;uniqueIndex;not null;column:api_key_hash" json:"-"`
	APIKeyEncrypted  string    `gorm:"size:256;column:api_key_encrypted" json:"-"`
	APIKeyPrefix     string    `gorm:"size:20;column:api_key_prefix" json:"api_key_prefix"`
	Status           string    `gorm:"size:20;not null;default:'active'" json:"status"` // active / disabled
	CreatedAt        time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt        time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (Keys) TableName() string { return "keys" }

// KeysGroup 密钥分组
type KeysGroup struct {
	ID          uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	Name        string    `gorm:"size:100;not null" json:"name"`
	Description string    `gorm:"size:500" json:"description"`
	QuotaRPM    int       `gorm:"not null;default:0" json:"quota_rpm"`  // 每个密钥各自的 RPM 限额，0=不限制
	QuotaTPM    int       `gorm:"not null;default:0" json:"quota_tpm"`  // 每个密钥各自的 TPM 限额，0=不限制
	CreatedAt   time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (KeysGroup) TableName() string { return "keys_groups" }

// KeysGroupMember 密钥-分组关联
type KeysGroupMember struct {
	ID      uint `gorm:"primaryKey;autoIncrement" json:"id"`
	KeysID  uint `gorm:"not null;index;column:keys_id" json:"keys_id"`
	GroupID uint `gorm:"not null;index" json:"group_id"`
}

func (KeysGroupMember) TableName() string { return "keys_group_members" }

// ListFilter 密钥列表筛选
type ListFilter struct {
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
	Status   string `form:"status"`
	Name     string `form:"name"`
}

// KeysService 密钥服务接口
type KeysService interface {
	Create(ctx context.Context, name string) (*Keys, string, error)
	Authenticate(ctx context.Context, apiKey string) (*Keys, error)
	CheckQuota(ctx context.Context, keysID uint, tokenCount int) error
	SetCache(cache account.Cache)
	SetCrypto(c *crypto.CryptoService)
	GetById(ctx context.Context, id uint) (*Keys, error)
	List(ctx context.Context, filter ListFilter) ([]Keys, int64, error)
	Update(ctx context.Context, id uint, name string) error
	Delete(ctx context.Context, id uint) error
	ResetKey(ctx context.Context, id uint) (string, error)
	RevealKey(ctx context.Context, id uint) (string, error)
	UpdateStatus(ctx context.Context, id uint, status string) error
}