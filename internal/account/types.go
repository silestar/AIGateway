package account

import (
	"context"
	"time"
)

// Account 渠道账号模型
type Account struct {
	ID                  uint       `gorm:"primaryKey;autoIncrement" json:"id"`
	ChannelID           uint       `gorm:"not null;index" json:"channel_id"`
	APIKeyEncrypted     string     `gorm:"type:text;not null;column:api_key_encrypted" json:"-"`
	APIKeyPrefix        string     `gorm:"size:12;not null;default:'';column:api_key_prefix" json:"api_key_prefix"` // sk-...前几位，用于脱敏展示
	Priority            int        `gorm:"not null;default:0" json:"priority"` // 越小越优先
	Status              string     `gorm:"size:20;not null;default:'active'" json:"status"` // active / disabled / cooling
	ConsecutiveFailures int        `gorm:"not null;default:0" json:"consecutive_failures"`
	LastFailedAt        *time.Time `json:"last_failed_at"`
	ProbeCooldownUntil  *time.Time `json:"probe_cooldown_until"`
	CreatedAt           time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt           time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
}

func (Account) TableName() string { return "channel_accounts" }

// AccountManager 账号管理接口
type AccountManager interface {
	// 核心路由用
	SelectAccount(ctx context.Context, keysID, channelID uint) (*Account, error)
	GetDecryptedAPIKey(ctx context.Context, accountID uint) (string, error)
	ReportResult(ctx context.Context, accountID uint, success bool, statusCode int) error

	// CRUD
	Create(ctx context.Context, channelID uint, apiKey string) (*Account, error)
	GetById(ctx context.Context, id uint) (*Account, error)
	ListByChannel(ctx context.Context, channelID uint) ([]Account, error)
	UpdatePriority(ctx context.Context, id uint, priority int) error
	UpdateStatus(ctx context.Context, id uint, status string) error
	RevealKey(ctx context.Context, id uint) (string, error) // 审计日志
	Delete(ctx context.Context, id uint) error

	// 后台调度
	StartProbeScheduler()
	StartGlobalHealthCheck()
}
