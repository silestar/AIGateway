package consumer

import (
	"context"
	"time"

	"github.com/bokelife/aigateway/internal/account"
)

// Consumer 消费者模型
type Consumer struct {
	ID          uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	Name        string    `gorm:"size:100;not null" json:"name"`
	APIKeyHash  string    `gorm:"size:64;uniqueIndex;not null;column:api_key_hash" json:"-"`
	Status      string    `gorm:"size:20;not null;default:'active'" json:"status"` // active / disabled
	CreatedAt   time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (Consumer) TableName() string { return "consumers" }

// ConsumerGroup 消费者分组
type ConsumerGroup struct {
	ID          uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	Name        string    `gorm:"size:100;not null" json:"name"`
	Description string    `gorm:"size:500" json:"description"`
	CreatedAt   time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (ConsumerGroup) TableName() string { return "consumer_groups" }

// ConsumerGroupMember 消费者-分组关联
type ConsumerGroupMember struct {
	ID          uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	ConsumerID  uint   `gorm:"not null;index" json:"consumer_id"`
	GroupID     uint   `gorm:"not null;index" json:"group_id"`
	QuotaRPM    int    `gorm:"not null;default:0" json:"quota_rpm"`  // 0=不限制
	QuotaTPM    int    `gorm:"not null;default:0" json:"quota_tpm"`  // 0=不限制
}

func (ConsumerGroupMember) TableName() string { return "consumer_group_members" }

// ListFilter 消费者列表筛选
type ListFilter struct {
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
	Status   string `form:"status"`
	Name     string `form:"name"`
}

// ConsumerService 消费者服务接口
type ConsumerService interface {
	Create(ctx context.Context, name string) (*Consumer, string, error) // 返回对象+明文key
	Authenticate(ctx context.Context, apiKey string) (*Consumer, error)
	CheckQuota(ctx context.Context, consumerID uint, tokenCount int) error
	SetCache(cache account.Cache)
	GetById(ctx context.Context, id uint) (*Consumer, error)
	List(ctx context.Context, filter ListFilter) ([]Consumer, int64, error)
	Update(ctx context.Context, id uint, name string) error
	Delete(ctx context.Context, id uint) error
	ResetKey(ctx context.Context, id uint) (string, error)
	RevealKey(ctx context.Context, id uint) (string, error) // 审计日志
	UpdateStatus(ctx context.Context, id uint, status string) error
}
