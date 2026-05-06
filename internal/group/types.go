package group

import (
	"context"

	"github.com/bokelife/aigateway/internal/account"
	"github.com/bokelife/aigateway/internal/channel"
	"github.com/bokelife/aigateway/internal/keys"
)

// GroupRouter 分组路由接口
type GroupRouter interface {
	// 核心路由：密钥+模型名 → 渠道+账号
	Route(ctx context.Context, keysID uint, modelName string) (*channel.Channel, *account.Account, error)

	// 渠道分组 CRUD
	CreateChannelGroup(ctx context.Context, name, description string, weight int) (*channel.ChannelGroup, error)
	UpdateChannelGroup(ctx context.Context, id uint, name, description string, weight int) error
	DeleteChannelGroup(ctx context.Context, id uint) error
	AddChannelToGroup(ctx context.Context, groupID, channelID uint, weight int) error
	RemoveChannelFromGroup(ctx context.Context, groupID, channelID uint) error

	// 密钥分组 CRUD
	CreateKeysGroup(ctx context.Context, name, description string) (*keys.KeysGroup, error)
	UpdateKeysGroup(ctx context.Context, id uint, name, description string) error
	DeleteKeysGroup(ctx context.Context, id uint) error
	AddKeysToGroup(ctx context.Context, groupID, keysID uint, quotaRPM, quotaTPM int) error
	RemoveKeysFromGroup(ctx context.Context, groupID, keysID uint) error
	BindChannelGroup(ctx context.Context, keysGroupID, channelGroupID uint) error
	UnbindChannelGroup(ctx context.Context, keysGroupID, channelGroupID uint) error
}