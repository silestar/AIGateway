package group

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/bokelife/aigateway/internal/account"
	"github.com/bokelife/aigateway/internal/channel"
	"github.com/bokelife/aigateway/internal/consumer"
	"github.com/bokelife/aigateway/internal/proxy"
)

// Router 分组路由实现
type Router struct {
	db          *gorm.DB
	consumerSvc consumer.ConsumerService
	accountMgr  account.AccountManager
	logger      *zap.Logger
}

// NewRouter 创建路由引擎
func NewRouter(db *gorm.DB, consumerSvc consumer.ConsumerService, accountMgr account.AccountManager, logger *zap.Logger) *Router {
	return &Router{
		db:          db,
		consumerSvc: consumerSvc,
		accountMgr:  accountMgr,
		logger:      logger,
	}
}

// RouteResult 路由结果
type RouteResult struct {
	Channel    *channel.Channel
	Account    *account.Account
	RetryChain *proxy.RetryChain
}

// Route 核心路由：消费者+模型名 → 渠道+账号
func (r *Router) Route(ctx context.Context, consumerID uint, modelName string) (*RouteResult, error) {
	retryChain := proxy.NewRetryChain()

	// 1. 获取消费者有权访问的渠道分组
	var memberRows []consumer.ConsumerGroupMember
	if err := r.db.WithContext(ctx).Where("consumer_id = ?", consumerID).Find(&memberRows).Error; err != nil {
		return nil, fmt.Errorf("query consumer groups: %w", err)
	}

	if len(memberRows) == 0 {
		return nil, fmt.Errorf("consumer %d has no group assignment", consumerID)
	}

	// 收集所有消费者分组ID
	consumerGroupIDs := make([]uint, 0, len(memberRows))
	for _, m := range memberRows {
		consumerGroupIDs = append(consumerGroupIDs, m.GroupID)
	}

	// 2. 查询消费者分组关联的渠道分组
	type channelGroupBinding struct {
		ConsumerGroupID uint
		ChannelGroupID  uint
	}
	var bindings []channelGroupBinding
	if err := r.db.WithContext(ctx).Table("consumer_group_channel_groups").
		Where("consumer_group_id IN ?", consumerGroupIDs).
		Find(&bindings).Error; err != nil {
		return nil, fmt.Errorf("query channel group bindings: %w", err)
	}

	if len(bindings) == 0 {
		return nil, fmt.Errorf("no channel group bound to consumer groups")
	}

	// 收集渠道分组ID（去重）
	channelGroupIDSet := make(map[uint]bool)
	for _, b := range bindings {
		channelGroupIDSet[b.ChannelGroupID] = true
	}
	channelGroupIDs := make([]uint, 0, len(channelGroupIDSet))
	for id := range channelGroupIDSet {
		channelGroupIDs = append(channelGroupIDs, id)
	}

	// 3. 查询渠道分组，按权重降序排序
	var channelGroups []channel.ChannelGroup
	if err := r.db.WithContext(ctx).Where("id IN ?", channelGroupIDs).
		Order("weight DESC, id ASC").Find(&channelGroups).Error; err != nil {
		return nil, fmt.Errorf("query channel groups: %w", err)
	}

	// 4. 遍历每个渠道分组
	for _, cg := range channelGroups {
		// 获取分组内的渠道（含权重）
		var groupMembers []channel.ChannelGroupMember
		if err := r.db.WithContext(ctx).Where("group_id = ?", cg.ID).
			Order("weight DESC, channel_id ASC").Find(&groupMembers).Error; err != nil {
			continue
		}

		channelIDs := make([]uint, 0, len(groupMembers))
		for _, gm := range groupMembers {
			channelIDs = append(channelIDs, gm.ChannelID)
		}

		// 5. 模型存在性过滤：查询有该模型的渠道
		var channelsWithModel []uint
		if err := r.db.WithContext(ctx).Model(&channel.ChannelModel{}).
			Where("channel_id IN ? AND display_model_name = ? AND status = 'enabled'", channelIDs, modelName).
			Distinct("channel_id").Pluck("channel_id", &channelsWithModel).Error; err != nil {
			continue
		}

		if len(channelsWithModel) == 0 {
			continue // 该分组无此模型
		}

		// 查询渠道详情
		var channels []channel.Channel
		if err := r.db.WithContext(ctx).Where("id IN ? AND status = 'active'", channelsWithModel).
			Order("weight DESC, id ASC").Find(&channels).Error; err != nil {
			continue
		}

		// 6. 遍历渠道
		for _, ch := range channels {
			// 6a. 选择账号
			acc, err := r.accountMgr.SelectAccount(ctx, consumerID, ch.ID)
			if err != nil {
				retryChain.AddAttempt(ch.ID, 0)
				retryChain.MarkError("no available account")
				continue
			}

			entry := retryChain.AddAttempt(ch.ID, acc.ID)

			// 6b. 尝试转发（实际转发在 handler 层执行，这里只做选择）
			_ = entry
			retryChain.MarkSuccess()

			return &RouteResult{
				Channel:    &ch,
				Account:    acc,
				RetryChain: retryChain,
			}, nil
		}
	}

	return nil, fmt.Errorf("no available channel for model %s", modelName)
}

// ========== 分组 CRUD ==========

func (r *Router) CreateChannelGroup(ctx context.Context, name, description string, weight int) (*channel.ChannelGroup, error) {
	cg := &channel.ChannelGroup{Name: name, Description: description, Weight: weight}
	if err := r.db.WithContext(ctx).Create(cg).Error; err != nil {
		return nil, err
	}
	return cg, nil
}

func (r *Router) UpdateChannelGroup(ctx context.Context, id uint, name, description string, weight int) error {
	return r.db.WithContext(ctx).Model(&channel.ChannelGroup{}).Where("id = ?", id).
		Updates(map[string]interface{}{"name": name, "description": description, "weight": weight}).Error
}

func (r *Router) DeleteChannelGroup(ctx context.Context, id uint) error {
	tx := r.db.WithContext(ctx).Begin()
	if err := tx.Where("group_id = ?", id).Delete(&channel.ChannelGroupMember{}).Error; err != nil {
		tx.Rollback()
		return err
	}
	if err := tx.Delete(&channel.ChannelGroup{}, id).Error; err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit().Error
}

func (r *Router) AddChannelToGroup(ctx context.Context, groupID, channelID uint, weight int) error {
	member := &channel.ChannelGroupMember{GroupID: groupID, ChannelID: channelID, Weight: weight}
	return r.db.WithContext(ctx).Create(member).Error
}

func (r *Router) RemoveChannelFromGroup(ctx context.Context, groupID, channelID uint) error {
	return r.db.WithContext(ctx).Where("group_id = ? AND channel_id = ?", groupID, channelID).
		Delete(&channel.ChannelGroupMember{}).Error
}

func (r *Router) CreateConsumerGroup(ctx context.Context, name, description string) (*consumer.ConsumerGroup, error) {
	cg := &consumer.ConsumerGroup{Name: name, Description: description}
	if err := r.db.WithContext(ctx).Create(cg).Error; err != nil {
		return nil, err
	}
	return cg, nil
}

func (r *Router) UpdateConsumerGroup(ctx context.Context, id uint, name, description string) error {
	return r.db.WithContext(ctx).Model(&consumer.ConsumerGroup{}).Where("id = ?", id).
		Updates(map[string]interface{}{"name": name, "description": description}).Error
}

func (r *Router) DeleteConsumerGroup(ctx context.Context, id uint) error {
	tx := r.db.WithContext(ctx).Begin()
	if err := tx.Where("group_id = ?", id).Delete(&consumer.ConsumerGroupMember{}).Error; err != nil {
		tx.Rollback()
		return err
	}
	if err := tx.Delete(&consumer.ConsumerGroup{}, id).Error; err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit().Error
}

func (r *Router) AddConsumerToGroup(ctx context.Context, groupID, consumerID uint, quotaRPM, quotaTPM int) error {
	member := &consumer.ConsumerGroupMember{
		GroupID: groupID, ConsumerID: consumerID, QuotaRPM: quotaRPM, QuotaTPM: quotaTPM,
	}
	return r.db.WithContext(ctx).Create(member).Error
}

func (r *Router) RemoveConsumerFromGroup(ctx context.Context, groupID, consumerID uint) error {
	return r.db.WithContext(ctx).Where("group_id = ? AND consumer_id = ?", groupID, consumerID).
		Delete(&consumer.ConsumerGroupMember{}).Error
}

func (r *Router) BindChannelGroup(ctx context.Context, consumerGroupID, channelGroupID uint) error {
	// 使用关联表 consumer_group_channel_groups
	return r.db.WithContext(ctx).Table("consumer_group_channel_groups").
		Create(map[string]interface{}{"consumer_group_id": consumerGroupID, "channel_group_id": channelGroupID}).Error
}

func (r *Router) UnbindChannelGroup(ctx context.Context, consumerGroupID, channelGroupID uint) error {
	return r.db.WithContext(ctx).Table("consumer_group_channel_groups").
		Where("consumer_group_id = ? AND channel_group_id = ?", consumerGroupID, channelGroupID).
		Delete(nil).Error
}

// ListChannelGroups 查询所有渠道分组
func (r *Router) ListChannelGroups(ctx context.Context) ([]channel.ChannelGroup, error) {
	var groups []channel.ChannelGroup
	if err := r.db.WithContext(ctx).Order("weight DESC, id ASC").Find(&groups).Error; err != nil {
		return nil, err
	}
	return groups, nil
}

// ListConsumerGroups 查询所有消费者分组
func (r *Router) ListConsumerGroups(ctx context.Context) ([]consumer.ConsumerGroup, error) {
	var groups []consumer.ConsumerGroup
	if err := r.db.WithContext(ctx).Order("id ASC").Find(&groups).Error; err != nil {
		return nil, err
	}
	return groups, nil
}
