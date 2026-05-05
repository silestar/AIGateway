package group

import (
	"context"

	"github.com/bokelife/aigateway/internal/account"
	"github.com/bokelife/aigateway/internal/channel"
	"github.com/bokelife/aigateway/internal/consumer"
)

// GroupRouter 分组路由接口
type GroupRouter interface {
	// 核心路由：消费者+模型名 → 渠道+账号
	Route(ctx context.Context, consumerID uint, modelName string) (*channel.Channel, *account.Account, error)

	// 渠道分组 CRUD
	CreateChannelGroup(ctx context.Context, name, description string, weight int) (*channel.ChannelGroup, error)
	UpdateChannelGroup(ctx context.Context, id uint, name, description string, weight int) error
	DeleteChannelGroup(ctx context.Context, id uint) error
	AddChannelToGroup(ctx context.Context, groupID, channelID uint, weight int) error
	RemoveChannelFromGroup(ctx context.Context, groupID, channelID uint) error

	// 消费者分组 CRUD
	CreateConsumerGroup(ctx context.Context, name, description string) (*consumer.ConsumerGroup, error)
	UpdateConsumerGroup(ctx context.Context, id uint, name, description string) error
	DeleteConsumerGroup(ctx context.Context, id uint) error
	AddConsumerToGroup(ctx context.Context, groupID, consumerID uint, quotaRPM, quotaTPM int) error
	RemoveConsumerFromGroup(ctx context.Context, groupID, consumerID uint) error
	BindChannelGroup(ctx context.Context, consumerGroupID, channelGroupID uint) error
	UnbindChannelGroup(ctx context.Context, consumerGroupID, channelGroupID uint) error
}
