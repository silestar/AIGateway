package plugin

import (
	"context"
	"encoding/json"
	"time"
)

// PluginStatus 插件状态常量
const (
	StatusInstalled = "installed"
	StatusRunning   = "running"
	StatusStopped   = "stopped"
	StatusUnhealthy = "unhealthy"
	StatusError     = "error"
)

// HookName 钩子名称（对齐设计文档）
type HookName string

const (
	HookPreRequest         HookName = "pre_request"
	HookPostResponse       HookName = "post_response"
	HookOnLog              HookName = "on_log"
	HookAccountSelect      HookName = "account_select"
	HookConnectionDecorator HookName = "connection_decorator"
)

// HookAction 钩子响应动作
type HookAction string

const (
	ActionContinue    HookAction = "continue"
	ActionReject     HookAction = "reject"
	ActionUseDefault HookAction = "use_default"
	ActionFilter     HookAction = "filter"
)

// Plugin 插件数据库模型
type Plugin struct {
	ID          uint       `gorm:"primaryKey;autoIncrement" json:"id"`
	Name        string     `gorm:"size:100;uniqueIndex;not null" json:"name"`
	Version     string     `gorm:"size:20;not null" json:"version"`
	Description string     `gorm:"type:text" json:"description"`
	Author      string     `gorm:"size:200" json:"author"`
	PluginType  string     `gorm:"size:20;not null;default:'sidecar'" json:"plugin_type"` // sidecar / system
	Binary      string     `gorm:"size:200;not null" json:"binary"`
	Port        int        `gorm:"not null" json:"port"`
	Hooks       string     `gorm:"type:text" json:"hooks"` // JSON array: ["pre_request","post_response"]
	ConfigSchema string   `gorm:"type:text" json:"config_schema"` // JSON Schema
	Manifest     string   `gorm:"type:text" json:"manifest"`     // 完整的 manifest.json 内容
	AuthToken   string     `gorm:"type:text" json:"-"` // AES 加密存储
	Status      string     `gorm:"size:20;not null;default:'installed'" json:"status"` // installed/running/stopped/unhealthy
	Config      string     `gorm:"type:text" json:"config"` // 当前配置 JSON
	Pid         int        `gorm:"default:0" json:"pid"`
	CreatedAt   time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
}

func (Plugin) TableName() string { return "plugins" }

// ChannelPluginSetting 渠道级插件配置（同一插件在不同渠道可有不同配置）
type ChannelPluginSetting struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	ChannelID uint      `gorm:"not null;index:idx_channel_plugin" json:"channel_id"`
	PluginID  uint      `gorm:"not null;index:idx_channel_plugin" json:"plugin_id"`
	Config    string    `gorm:"type:text" json:"config"` // 渠道级配置 JSON（覆盖插件全局 config）
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (ChannelPluginSetting) TableName() string { return "channel_plugin_settings" }

// HookRequest 钩子请求（主系统 → 插件）
type HookRequest struct {
	KeysID        uint                   `json:"keys_id"`
	KeysName      string                 `json:"keys_name,omitempty"`
	Model             string                 `json:"model"`
	Request           *HookRequestBody       `json:"request,omitempty"`
	Response          *HookResponseBody      `json:"response,omitempty"`
	ChannelID         uint                   `json:"channel_id,omitempty"`
	AccountID         uint                   `json:"account_id,omitempty"`
	CandidateAccounts []CandidateAccount     `json:"candidate_accounts,omitempty"`
	Config            map[string]interface{} `json:"config,omitempty"`
}

type HookRequestBody struct {
	Headers map[string]string      `json:"headers,omitempty"`
	Body    map[string]interface{} `json:"body,omitempty"`
}

type HookResponseBody struct {
	StatusCode int                    `json:"status_code,omitempty"`
	Headers    map[string]string      `json:"headers,omitempty"`
	Body       map[string]interface{} `json:"body,omitempty"`
}

type CandidateAccount struct {
	ID       uint   `json:"id"`
	Priority int    `json:"priority"`
	Status   string `json:"status"`
}

// HookResponse 钩子响应（插件 → 主系统）
type HookResponse struct {
	Action           HookAction             `json:"action"`
	StatusCode       int                    `json:"status_code,omitempty"`
	Message          string                 `json:"message,omitempty"`
	ModifiedRequest  *HookRequestBody       `json:"modified_request,omitempty"`
	ModifiedResponse *HookResponseBody      `json:"modified_response,omitempty"`
	ExcludeIDs       []uint                 `json:"exclude_ids,omitempty"` // account_select 用
}

// Manifest 插件描述文件
type Manifest struct {
	Name         string            `json:"name"`
	Version      string            `json:"version"`
	Description  string            `json:"description"`
	Author       string            `json:"author"`
	Type         string            `json:"type,omitempty"`        // 插件类型：sidecar（默认）/ system
	Binary       string            `json:"binary"`               // 单架构二进制文件名（向后兼容）
	Binaries     map[string]string `json:"binaries,omitempty"`    // 多架构映射：GOOS/GOARCH → 二进制文件名
	Port         int             `json:"port"`
	Hooks        []string        `json:"hooks"`
	ConfigSchema json.RawMessage `json:"config_schema"`
}

// PluginManager 插件管理器接口
type PluginManager interface {
	// 安装插件（解压 ZIP，读取 manifest，入库）
	Install(ctx context.Context, zipPath string) (*Plugin, error)
	// 启动插件（fork 子进程，注入环境变量）
	Start(ctx context.Context, pluginID uint) error
	// 停止插件（优雅关闭进程）
	Stop(ctx context.Context, pluginID uint) error
	// 卸载插件（停止 + 删除目录 + 删除记录）
	Uninstall(ctx context.Context, pluginID uint) error
	// 触发钩子（遍历订阅该钩子的运行中插件，HTTP 调用）
	TriggerHook(ctx context.Context, hook HookName, req *HookRequest) (*HookResponse, error)
	// 列出所有插件
	List(ctx context.Context) ([]Plugin, error)
	// 获取单个插件
	GetByID(ctx context.Context, id uint) (*Plugin, error)
	// 更新插件配置
	UpdateConfig(ctx context.Context, id uint, config string) error
	// 健康检查（定时调用所有运行中插件的 /health）
	HealthCheck(ctx context.Context)
}

// ContinueHook 快速构造 continue 响应
func ContinueHook() *HookResponse {
	return &HookResponse{Action: ActionContinue}
}

// RejectHook 快速构造 reject 响应
func RejectHook(statusCode int, message string) *HookResponse {
	return &HookResponse{Action: ActionReject, StatusCode: statusCode, Message: message}
}

// UseDefaultHook 快速构造 use_default 响应
func UseDefaultHook() *HookResponse {
	return &HookResponse{Action: ActionUseDefault}
}

// FilterHook 快速构造 filter 响应
func FilterHook(excludeIDs []uint) *HookResponse {
	return &HookResponse{Action: ActionFilter, ExcludeIDs: excludeIDs}
}
