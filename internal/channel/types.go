package channel

import (
	"context"
	"time"
)

// Channel 渠道模型
type Channel struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	Name      string    `gorm:"size:100;not null" json:"name"`
	Type      string    `gorm:"size:30;not null" json:"type"`  // openai / openai-response / anthropic / gemini
	BaseURL   string    `gorm:"size:500;not null" json:"base_url"`
	Status    string    `gorm:"size:20;not null;default:'active'" json:"status"` // active / disabled
	Weight    int       `gorm:"not null;default:0" json:"weight"` // 越大越优先
	MaxRPM             int       `gorm:"not null;default:0" json:"max_rpm"`             // 每分钟最大请求数，0 不限制
	MaxTPM             int       `gorm:"not null;default:0" json:"max_tpm"`             // 每分钟最大 Token 数，0 不限制
	MaxDailyRequests   int       `gorm:"not null;default:0" json:"max_daily_requests"`  // 每日最大请求数（每个账号），0 不限制
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (Channel) TableName() string { return "channels" }

// ChannelModel 渠道模型映射
type ChannelModel struct {
	ID                uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	ChannelID         uint   `gorm:"not null;index" json:"channel_id"`
	DisplayModelName  string `gorm:"size:100;not null;index" json:"display_model_name"` // 对外展示的名称
	ActualModelName   string `gorm:"size:100;not null" json:"actual_model_name"`        // 实际请求上游的名称
	Status            string `gorm:"size:20;not null;default:'enabled'" json:"status"`   // enabled / disabled
}

func (ChannelModel) TableName() string { return "channel_models" }

// ChannelGroup 渠道分组
type ChannelGroup struct {
	ID          uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	Name        string    `gorm:"size:100;not null" json:"name"`
	Description string    `gorm:"size:500" json:"description"`
	Weight      int       `gorm:"not null;default:0" json:"weight"` // 越大越优先
	CreatedAt   time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (ChannelGroup) TableName() string { return "channel_groups" }

// ChannelGroupMember 渠道-分组关联
type ChannelGroupMember struct {
	ID        uint `gorm:"primaryKey;autoIncrement" json:"id"`
	GroupID   uint `gorm:"not null;index" json:"group_id"`
	ChannelID uint `gorm:"not null;index" json:"channel_id"`
	Weight    int  `gorm:"not null;default:0" json:"weight"` // 组内权重
}

func (ChannelGroupMember) TableName() string { return "channel_group_members" }

// ModelInfo 模型信息（FetchModels 返回）
type ModelInfo struct {
	ID      string `json:"id"`
	OwnedBy string `json:"owned_by"`
}

// ListFilter 渠道列表筛选
type ListFilter struct {
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
	Status   string `form:"status"`
	Type     string `form:"type"`
}

// ChannelService 渠道服务接口
type ChannelService interface {
	Create(ctx context.Context, name, channelType, baseURL string) (*Channel, error)
	GetById(ctx context.Context, id uint) (*Channel, error)
	List(ctx context.Context, filter ListFilter) ([]Channel, int64, error)
	Update(ctx context.Context, id uint, name, baseURL string, weight, maxRPM, maxTPM, maxDailyRequests int) error
	Delete(ctx context.Context, id uint) error
	UpdateStatus(ctx context.Context, id uint, status string) error
	UpdateWeight(ctx context.Context, id uint, weight int) error
	TestConnection(ctx context.Context, channelType, baseURL, apiKey string) error
	FetchModels(ctx context.Context, id uint, testKey string) ([]ModelInfo, error)
	GetModelsByChannel(ctx context.Context, id uint) ([]ChannelModel, error)
	SaveModels(ctx context.Context, id uint, models []ChannelModel) error
}
