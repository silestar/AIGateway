package group

import (
	"context"

	"github.com/silestar/AIGateway/internal/account"
	"github.com/silestar/AIGateway/internal/channel"
	"github.com/silestar/AIGateway/internal/keys"
)

// KeysGroupWithCount 密钥分组 + 渠道分组绑定计数
type KeysGroupWithCount struct {
	keys.KeysGroup
	ChannelCount int64 `json:"channel_count"`
}

// KeysGroupDetail 密钥分组详情（含配额+已绑渠道分组+可选渠道分组+密钥列表）
type KeysGroupDetail struct {
	keys.KeysGroup
	BoundChannelGroups    []channel.ChannelGroup `json:"bound_channel_groups"`
	AvailableChannelGroups []channel.ChannelGroup `json:"available_channel_groups"`
	BoundKeys             []keysInfo             `json:"bound_keys"`
	AvailableKeys         []keysInfo             `json:"available_keys"`
}

// ChannelGroupWithCount 渠道分组 + 成员计数
type ChannelGroupWithCount struct {
	channel.ChannelGroup
	ChannelCount int64 `json:"channel_count"`
}

// ChannelGroupDetail 渠道分组详情（含关联渠道列表）
type ChannelGroupDetail struct {
	channel.ChannelGroup
	Channels []channelInfo `json:"channels"`
}

// keysInfo 关联密钥的简要信息
type keysInfo struct {
	ID     uint   `json:"id"`
	Name   string `json:"name"`
	Prefix string `json:"prefix"`
	Status string `json:"status"`
}

// channelInfo 关联渠道的简要信息
type channelInfo struct {
	ID     uint   `json:"id"`
	Name   string `json:"name"`
	Type   string `json:"type"`
	Status string `json:"status"`
	Weight int    `json:"weight"`
}

// GroupRouter 分组路由接口
type GroupRouter interface {
	// 核心路由：密钥+模型名 → 渠道+账号
	Route(ctx context.Context, keysID uint, modelName string) (*channel.Channel, *account.Account, error)

	// 渠道分组 CRUD
	CreateChannelGroup(ctx context.Context, name, description string, weight int) (*channel.ChannelGroup, error)
	ListChannelGroups(ctx context.Context) ([]ChannelGroupWithCount, error)
	GetChannelGroup(ctx context.Context, id uint) (*ChannelGroupDetail, error)
	UpdateChannelGroup(ctx context.Context, id uint, name, description string, weight int) error
	DeleteChannelGroup(ctx context.Context, id uint) error
	AddChannelToGroup(ctx context.Context, groupID, channelID uint, weight int) error
	RemoveChannelFromGroup(ctx context.Context, groupID, channelID uint) error
	SetChannelGroupChannels(ctx context.Context, groupID uint, channelIDs []uint) error

	// 密钥分组 CRUD
	CreateKeysGroup(ctx context.Context, name, description string, quotaRPM, quotaTPM int) (*keys.KeysGroup, error)
	ListKeysGroups(ctx context.Context) ([]KeysGroupWithCount, error)
	GetKeysGroup(ctx context.Context, id uint) (*KeysGroupDetail, error)
	UpdateKeysGroup(ctx context.Context, id uint, name, description string, quotaRPM, quotaTPM int) error
	DeleteKeysGroup(ctx context.Context, id uint) error
	AddKeysToGroup(ctx context.Context, groupID, keysID uint) error
	RemoveKeysFromGroup(ctx context.Context, groupID, keysID uint) error
	SetKeysGroupChannelGroups(ctx context.Context, groupID uint, channelGroupIDs []uint) error
	BindChannelGroup(ctx context.Context, keysGroupID, channelGroupID uint) error
	UnbindChannelGroup(ctx context.Context, keysGroupID, channelGroupID uint) error
}