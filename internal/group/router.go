package group

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/bokelife/aigateway/internal/account"
	"github.com/bokelife/aigateway/internal/channel"
	"github.com/bokelife/aigateway/internal/keys"
	"github.com/bokelife/aigateway/internal/proxy"
)

// Router 分组路由实现
type Router struct {
	db         *gorm.DB
	keysSvc    keys.KeysService
	accountMgr account.AccountManager
	logger     *zap.Logger
}

// NewRouter 创建路由引擎
func NewRouter(db *gorm.DB, keysSvc keys.KeysService, accountMgr account.AccountManager, logger *zap.Logger) *Router {
	return &Router{
		db:         db,
		keysSvc:    keysSvc,
		accountMgr: accountMgr,
		logger:     logger,
	}
}

// RouteResult 路由结果
type RouteResult struct {
	Channel         *channel.Channel
	Account         *account.Account
	RetryChain      *proxy.RetryChain
	ActualModelName string // 映射后的实际上游模型名（与请求中的 model 不同时表示有别名映射）
}

// Route 核心路由：密钥+模型名 → 渠道+账号
func (r *Router) Route(ctx context.Context, keysID uint, modelName string) (*RouteResult, error) {
	retryChain := proxy.NewRetryChain()

	// 1. 获取密钥有权访问的渠道分组
	var memberRows []keys.KeysGroupMember
	if err := r.db.WithContext(ctx).Where("keys_id = ?", keysID).Find(&memberRows).Error; err != nil {
		return nil, fmt.Errorf("query keys groups: %w", err)
	}

	if len(memberRows) == 0 {
		return nil, fmt.Errorf("keys %d has no group assignment", keysID)
	}

	keysGroupIDs := make([]uint, 0, len(memberRows))
	for _, m := range memberRows {
		keysGroupIDs = append(keysGroupIDs, m.GroupID)
	}

	// 2. 查询密钥分组关联的渠道分组
	type channelGroupBinding struct {
		KeysGroupID    uint
		ChannelGroupID uint
	}
	var bindings []channelGroupBinding
	if err := r.db.WithContext(ctx).Table("keys_group_channel_groups").
		Where("keys_group_id IN ?", keysGroupIDs).
		Find(&bindings).Error; err != nil {
		return nil, fmt.Errorf("query channel group bindings: %w", err)
	}

	if len(bindings) == 0 {
		return nil, fmt.Errorf("no channel group bound to keys groups")
	}

	channelGroupIDSet := make(map[uint]bool)
	for _, b := range bindings {
		channelGroupIDSet[b.ChannelGroupID] = true
	}
	channelGroupIDs := make([]uint, 0, len(channelGroupIDSet))
	for id := range channelGroupIDSet {
		channelGroupIDs = append(channelGroupIDs, id)
	}

	var channelGroups []channel.ChannelGroup
	if err := r.db.WithContext(ctx).Where("id IN ?", channelGroupIDs).
		Order("weight DESC, id ASC").Find(&channelGroups).Error; err != nil {
		return nil, fmt.Errorf("query channel groups: %w", err)
	}

	for _, cg := range channelGroups {
		var groupMembers []channel.ChannelGroupMember
		if err := r.db.WithContext(ctx).Where("group_id = ?", cg.ID).
			Order("weight DESC, channel_id ASC").Find(&groupMembers).Error; err != nil {
			continue
		}

		channelIDs := make([]uint, 0, len(groupMembers))
		for _, gm := range groupMembers {
			channelIDs = append(channelIDs, gm.ChannelID)
		}

		// 查询匹配模型的渠道及对应的 actual_model_name（用于模型映射替换）
		type modelMatch struct {
			ChannelID       uint
			ActualModelName string
		}
		var matches []modelMatch
		if err := r.db.WithContext(ctx).Model(&channel.ChannelModel{}).
			Select("channel_id, actual_model_name").
			Where("channel_id IN ? AND display_model_name = ? AND status = 'enabled'", channelIDs, modelName).
			Scan(&matches).Error; err != nil {
			continue
		}

		if len(matches) == 0 {
			continue
		}

		// 构建 channel_id -> actual_model_name 映射
		actualModelMap := make(map[uint]string)
		chIDs := make([]uint, 0, len(matches))
		for _, m := range matches {
			actualModelMap[m.ChannelID] = m.ActualModelName
			chIDs = append(chIDs, m.ChannelID)
		}

		var channels []channel.Channel
		if err := r.db.WithContext(ctx).Where("id IN ? AND status = 'active'", chIDs).
			Order("weight DESC, id ASC").Find(&channels).Error; err != nil {
			continue
		}

		for _, ch := range channels {
			acc, err := r.accountMgr.SelectAccount(ctx, keysID, ch.ID)
			if err != nil {
				retryChain.AddAttempt(ch.ID, 0)
				retryChain.MarkError("no available account")
				continue
			}

			_ = retryChain.AddAttempt(ch.ID, acc.ID)
			retryChain.MarkSuccess()

			actualName := actualModelMap[ch.ID]
			if actualName == "" {
				actualName = modelName
			}

			return &RouteResult{
				Channel:         &ch,
				Account:         acc,
				RetryChain:      retryChain,
				ActualModelName: actualName,
			}, nil
		}
	}

	return nil, fmt.Errorf("no available channel for model %s", modelName)
}

// ========== 渠道分组 CRUD ==========

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

// ========== 密钥分组 CRUD ==========

func (r *Router) CreateKeysGroup(ctx context.Context, name, description string) (*keys.KeysGroup, error) {
	cg := &keys.KeysGroup{Name: name, Description: description}
	if err := r.db.WithContext(ctx).Create(cg).Error; err != nil {
		return nil, err
	}
	return cg, nil
}

func (r *Router) UpdateKeysGroup(ctx context.Context, id uint, name, description string) error {
	return r.db.WithContext(ctx).Model(&keys.KeysGroup{}).Where("id = ?", id).
		Updates(map[string]interface{}{"name": name, "description": description}).Error
}

func (r *Router) DeleteKeysGroup(ctx context.Context, id uint) error {
	tx := r.db.WithContext(ctx).Begin()
	if err := tx.Where("group_id = ?", id).Delete(&keys.KeysGroupMember{}).Error; err != nil {
		tx.Rollback()
		return err
	}
	if err := tx.Delete(&keys.KeysGroup{}, id).Error; err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit().Error
}

func (r *Router) AddKeysToGroup(ctx context.Context, groupID, keysID uint, quotaRPM, quotaTPM int) error {
	member := &keys.KeysGroupMember{
		GroupID: groupID, KeysID: keysID, QuotaRPM: quotaRPM, QuotaTPM: quotaTPM,
	}
	return r.db.WithContext(ctx).Create(member).Error
}

func (r *Router) RemoveKeysFromGroup(ctx context.Context, groupID, keysID uint) error {
	return r.db.WithContext(ctx).Where("group_id = ? AND keys_id = ?", groupID, keysID).
		Delete(&keys.KeysGroupMember{}).Error
}

func (r *Router) BindChannelGroup(ctx context.Context, keysGroupID, channelGroupID uint) error {
	return r.db.WithContext(ctx).Table("keys_group_channel_groups").
		Create(map[string]interface{}{"keys_group_id": keysGroupID, "channel_group_id": channelGroupID}).Error
}

func (r *Router) UnbindChannelGroup(ctx context.Context, keysGroupID, channelGroupID uint) error {
	return r.db.WithContext(ctx).Table("keys_group_channel_groups").
		Where("keys_group_id = ? AND channel_group_id = ?", keysGroupID, channelGroupID).
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

// ListKeysGroups 查询所有密钥分组
func (r *Router) ListKeysGroups(ctx context.Context) ([]keys.KeysGroup, error) {
	var groups []keys.KeysGroup
	if err := r.db.WithContext(ctx).Order("id ASC").Find(&groups).Error; err != nil {
		return nil, err
	}
	return groups, nil
}