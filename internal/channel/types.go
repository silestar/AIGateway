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
	TestModel          string    `gorm:"size:100;not null;default:''" json:"test_model"`           // 指定测试模型，为空时取第一个已配置模型
	LastTestLatency    int       `gorm:"not null;default:0" json:"last_test_latency"`              // 最近测试响应延迟（毫秒），0=未测试
	LastTestedAt       *time.Time `json:"last_tested_at"`                                          // 最近测试时间
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
	UpstreamVisible   bool   `gorm:"not null;default:true" json:"upstream_visible"`     // 是否作为上游模型暴露
	DisplayVisible    bool   `gorm:"not null;default:true" json:"display_visible"`       // 是否作为映射别名暴露
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
	Search   string `form:"search"` // 按名称/ID/类型/模型名模糊搜索
	SortBy   string `form:"sort_by"` // weight / id / latency
	SortOrder string `form:"sort_order"` // asc / desc
}

// ChannelListItem 渠道列表项（含聚合信息）
type ChannelListItem struct {
	Channel
	ActiveAccountCount int      `json:"active_account_count"`
	TotalAccountCount  int      `json:"total_account_count"`
	Groups             []GroupInfo `json:"groups"`
}

// GroupInfo 渠道分组简要信息
type GroupInfo struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

// TestResult 单次测试结果
type TestResult struct {
	Success           bool   `json:"success"`
	Latency           int    `json:"latency"`  // 毫秒
	Status            int    `json:"status,omitempty"`
	Error             string `json:"error,omitempty"`
	Model             string `json:"model"`
	PromptTokens      int    `json:"prompt_tokens,omitempty"`
	CompletionTokens  int    `json:"completion_tokens,omitempty"`
}

// BatchTestResult 批量测试结果项
type BatchTestResultItem struct {
	Model   string `json:"model"`
	Success bool   `json:"success"`
	Latency int    `json:"latency"` // 毫秒
	Error   string `json:"error,omitempty"`
	Status  int    `json:"status,omitempty"` // HTTP 状态码（失败时）
}

// ChannelService 渠道服务接口
type ChannelService interface {
	Create(ctx context.Context, name, channelType, baseURL string) (*Channel, error)
	GetById(ctx context.Context, id uint) (*Channel, error)
	List(ctx context.Context, filter ListFilter) ([]ChannelListItem, int64, error)
	Update(ctx context.Context, id uint, name, baseURL string, weight, maxRPM, maxTPM, maxDailyRequests int) error
	Delete(ctx context.Context, id uint) error
	UpdateStatus(ctx context.Context, id uint, status string) error
	UpdateWeight(ctx context.Context, id uint, weight int) error
	TestConnection(ctx context.Context, channelType, baseURL, apiKey string) error
	FetchModels(ctx context.Context, id uint, testKey string) ([]ModelInfo, error)
	GetModelsByChannel(ctx context.Context, id uint) ([]ChannelModel, error)
	SaveModels(ctx context.Context, id uint, models []ChannelModel) error
	// 新增方法
	TestChannel(ctx context.Context, id uint, apiKey string) (*TestResult, error)
	// TestAccount 测试指定账号（不限状态）
	TestAccount(ctx context.Context, channelID, accountID uint, apiKey string) (*TestResult, error)
	BatchTestModels(ctx context.Context, id uint, modelNames []string, apiKey string) ([]BatchTestResultItem, error)
	UpdateTestModel(ctx context.Context, id uint, testModel string) error
	CopyChannel(ctx context.Context, id uint) (*Channel, error)
	// SetOnModelsChange 设置模型变更回调（用于同步 model_catalog）
	SetOnModelsChange(fn func())
	// GetCustomModelNames 获取所有渠道已配置的自定义模型名（display != actual）
	GetCustomModelNames(ctx context.Context) ([]string, error)
}
