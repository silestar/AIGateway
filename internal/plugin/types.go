package plugin

import (
	"context"
	"encoding/json"
	"time"
)

// PluginStatus 插件状态常量
const (
	StatusInstalled  = "installed"
	StatusRunning    = "running"
	StatusStopped    = "stopped"
	StatusUnhealthy  = "unhealthy"
	StatusError      = "error"
	StatusUninstalled = "uninstalled"
)

// PermissionName 权限名称常量
type PermissionName string

const (
	PermAccountID          PermissionName = "account_id"
	PermChannelID          PermissionName = "channel_id"
	PermKeysID             PermissionName = "keys_id"
	PermModelName          PermissionName = "model_name"
	PermRequestHeaders     PermissionName = "request_headers"
	PermRequestBodySummary PermissionName = "request_body_summary"
	PermResponseStatus     PermissionName = "response_status"
	PermResponseBodySummary PermissionName = "response_body_summary"
	PermServerInfo         PermissionName = "server_info"
	PermChannelInfo        PermissionName = "channel_info"
	PermChannelConfig      PermissionName = "channel_config"
)

// HighSensitivePermissions 高敏感权限列表（授予时需二次确认）
var HighSensitivePermissions = map[PermissionName]bool{
	PermRequestHeaders: true,
	PermChannelConfig:  true,
}

// PermissionStatus 权限状态常量
const (
	PermPending  = "pending"
	PermGranted  = "granted"
	PermDenied   = "denied"
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

// PluginPermission 插件权限授权记录
type PluginPermission struct {
	ID              uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	PluginName      string    `gorm:"size:100;not null;uniqueIndex:uk_plugin_perm" json:"plugin_name"`
	PluginVersion   string    `gorm:"size:20;not null;default:''" json:"plugin_version"`
	PermissionName  string    `gorm:"size:100;not null;uniqueIndex:uk_plugin_perm" json:"permission_name"`
	Status          string    `gorm:"size:20;not null;default:'pending'" json:"status"` // pending/granted/denied
	Description     string    `gorm:"type:text" json:"description"`                      // 插件声明的权限描述
	Required        bool      `gorm:"not null;default:false" json:"required"`           // 插件声明的 required
	GrantedBy       string    `gorm:"size:100" json:"granted_by"`                        // 操作者
	GrantedAt       *time.Time `gorm:"" json:"granted_at,omitempty"`
	RevokedAt       *time.Time `gorm:"" json:"revoked_at,omitempty"`
	CreatedAt       time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt       time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (PluginPermission) TableName() string { return "plugin_permissions" }

// Manifest 插件描述文件
type Manifest struct {
	Name         string            `json:"name"`
	Version      string            `json:"version"`
	Description  string            `json:"description"`
	Author       string            `json:"author"`
	Type         string            `json:"type,omitempty"`
	Binary       string            `json:"binary"`
	Binaries     map[string]string `json:"binaries,omitempty"`
	Port         int               `json:"port"`
	Hooks        []string          `json:"hooks"`
	ConfigSchema json.RawMessage   `json:"config_schema"`
	Permissions  []PermissionDecl  `json:"permissions,omitempty"`
}

// PermissionDecl 插件权限声明
type PermissionDecl struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
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
	// 获取渠道启用的 connection_decorator 插件地址
	GetConnectionDecoratorAddr(channelID uint) string
	// 获取单个插件
	GetByID(ctx context.Context, id uint) (*Plugin, error)
	// 更新插件配置
	UpdateConfig(ctx context.Context, id uint, config string) error
	// 健康检查（定时调用所有运行中插件的 /health）
	HealthCheck(ctx context.Context)
	// 获取插件权限列表
	GetPermissions(ctx context.Context, pluginName string) ([]PluginPermission, error)
	// 授予插件权限
	GrantPermission(ctx context.Context, pluginName, permissionName, grantedBy string) error
	// 撤销插件权限
	DenyPermission(ctx context.Context, pluginName, permissionName, grantedBy string) error
	// 全部授予
	GrantAllPermissions(ctx context.Context, pluginName, grantedBy string) error
	// 全部撤销
	DenyAllPermissions(ctx context.Context, pluginName, grantedBy string) error
	// 获取已授予的权限列表（从缓存读取）
	GetGrantedPermissions(pluginName string) []string
	// 检查插件是否有未满足的必需权限
	CheckRequiredPermissions(pluginName string) (missing []string, err error)
	// 同步插件权限声明（安装/升级时调用）
	SyncPermissions(ctx context.Context, pluginName, pluginVersion string, declarations []PermissionDecl) error
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
