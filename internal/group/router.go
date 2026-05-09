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
	cache      account.Cache
}

// NewRouter 创建路由引擎
func NewRouter(db *gorm.DB, keysSvc keys.KeysService, accountMgr account.AccountManager, logger *zap.Logger, cache account.Cache) *Router {
	return &Router{
		db:         db,
		keysSvc:    keysSvc,
		accountMgr: accountMgr,
		logger:     logger,
		cache:      cache,
	}
}

// RouteResult 路由结果
type RouteResult struct {
	Channel         *channel.Channel
	Account         *account.Account
	RetryChain      *proxy.RetryChain
	ActualModelName string // 映射后的实际上游模型名

	// 内部状态（用于重试循环，不序列化给外部）
	excludedAccountIDs map[uint][]uint // channelID → 已排除的 accountIDs
	modelName          string
	keysID             uint
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
				retryChain.MarkError("no available account", 0, 0)
				continue
			}

			// 账号级速率检查：限制值来自渠道配置，计数器按账号维度统计
			if limitType, limited := r.accountMgr.IsAccountRateLimited(acc.ID, ch.MaxRPM, ch.MaxTPM, ch.MaxDailyRequests); limited {
				r.logger.Debug("account rate limit exceeded, skipping",
					zap.Uint("channel_id", ch.ID),
					zap.Uint("account_id", acc.ID),
					zap.String("limit_type", limitType))
				retryChain.AddAttempt(ch.ID, acc.ID)
				retryChain.MarkError("account rate limit exceeded: "+limitType, 0, 0)
				continue
			}

			_ = retryChain.AddAttempt(ch.ID, acc.ID)
			retryChain.MarkSuccess(0, 0)

			actualName := actualModelMap[ch.ID]
			if actualName == "" {
				actualName = modelName
			}

			return &RouteResult{
				Channel:         &ch,
				Account:         acc,
				RetryChain:      retryChain,
				ActualModelName: actualName,
				excludedAccountIDs: make(map[uint][]uint),
				modelName:          modelName,
				keysID:             keysID,
			}, nil
		}
	}

	return nil, fmt.Errorf("no available channel for model %s", modelName)
}

// RerouteAfterFailure 获取下一个可用的账号/渠道（同渠道优先→跨渠道降级）
// 调用前需先执行 accountMgr.ClearAccountAffinity + ReportResult
func (r *Router) RerouteAfterFailure(ctx context.Context, failedResult *RouteResult) (*RouteResult, error) {
	// 1. 清除失败账号的粘性绑定 + 加入排除列表
	r.accountMgr.ClearAccountAffinity(failedResult.keysID, failedResult.Channel.ID)
	failedResult.excludedAccountIDs[failedResult.Channel.ID] = append(
		failedResult.excludedAccountIDs[failedResult.Channel.ID],
		failedResult.Account.ID,
	)

	// 2. 尝试当前渠道内的下一个账号
	acc, err := r.accountMgr.SelectAccountWithExclude(ctx, failedResult.keysID, failedResult.Channel.ID,
		failedResult.excludedAccountIDs[failedResult.Channel.ID])
	if err == nil {
		// 同渠道有下一个账号
		r.accountMgr.GetDecryptedAPIKey(ctx, acc.ID) // 预热缓存
		failedResult.RetryChain.AddAttempt(failedResult.Channel.ID, acc.ID)
		failedResult.Account = acc
		return failedResult, nil
	}

	// 3. 当前渠道账号已耗尽 → 降级：重新运行完整 Route（但排除已失败渠道？不，Route 本身会 fallthrough）
	// 简化：直接重新调用 Route（Route 内部会从第一个渠道分组重新遍历，但由于 ReportResult 已禁用失败账号，
	// SelectAccount 不会再选中它）
	return r.Route(ctx, failedResult.keysID, failedResult.modelName)
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
	// 检查是否被密钥分组引用
	var count int64
	if err := r.db.WithContext(ctx).Table("keys_group_channel_groups").
		Where("channel_group_id = ?", id).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("该分组被 %d 个密钥分组引用，无法删除", count)
	}

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

func (r *Router) CreateKeysGroup(ctx context.Context, name, description string, quotaRPM, quotaTPM int) (*keys.KeysGroup, error) {
	cg := &keys.KeysGroup{Name: name, Description: description, QuotaRPM: quotaRPM, QuotaTPM: quotaTPM}
	if err := r.db.WithContext(ctx).Create(cg).Error; err != nil {
		return nil, err
	}
	return cg, nil
}

func (r *Router) UpdateKeysGroup(ctx context.Context, id uint, name, description string, quotaRPM, quotaTPM int) error {
	return r.db.WithContext(ctx).Model(&keys.KeysGroup{}).Where("id = ?", id).
		Updates(map[string]interface{}{"name": name, "description": description, "quota_rpm": quotaRPM, "quota_tpm": quotaTPM}).Error
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

func (r *Router) AddKeysToGroup(ctx context.Context, groupID, keysID uint) error {
	member := &keys.KeysGroupMember{
		GroupID: groupID, KeysID: keysID,
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

// ListChannelGroups 查询所有渠道分组（含成员计数）
func (r *Router) ListChannelGroups(ctx context.Context) ([]ChannelGroupWithCount, error) {
	var groups []ChannelGroupWithCount
	if err := r.db.WithContext(ctx).Model(&channel.ChannelGroup{}).
		Select("channel_groups.*, COUNT(cgm.channel_id) AS channel_count").
		Joins("LEFT JOIN channel_group_members cgm ON cgm.group_id = channel_groups.id").
		Group("channel_groups.id").
		Order("weight DESC, channel_groups.id ASC").
		Find(&groups).Error; err != nil {
		return nil, err
	}
	return groups, nil
}

// ListKeysGroups 查询所有密钥分组（含渠道分组绑定计数）
func (r *Router) ListKeysGroups(ctx context.Context) ([]KeysGroupWithCount, error) {
	var groups []KeysGroupWithCount
	if err := r.db.WithContext(ctx).Model(&keys.KeysGroup{}).
		Select("keys_groups.*, COUNT(kgcg.channel_group_id) AS channel_count").
		Joins("LEFT JOIN keys_group_channel_groups kgcg ON kgcg.keys_group_id = keys_groups.id").
		Group("keys_groups.id").
		Order("keys_groups.id ASC").
		Find(&groups).Error; err != nil {
		return nil, err
	}
	return groups, nil
}

// ========== 详情接口 ==========

// GetChannelGroup 获取渠道分组详情（含关联渠道）
func (r *Router) GetChannelGroup(ctx context.Context, id uint) (*ChannelGroupDetail, error) {
	var cg channel.ChannelGroup
	if err := r.db.WithContext(ctx).First(&cg, id).Error; err != nil {
		return nil, err
	}

	var members []channel.ChannelGroupMember
	r.db.WithContext(ctx).Where("group_id = ?", id).
		Order("weight DESC, channel_id ASC").Find(&members)

	channelIDs := make([]uint, len(members))
	for i, m := range members {
		channelIDs[i] = m.ChannelID
	}

	var chs []channel.Channel
	if len(channelIDs) > 0 {
		r.db.WithContext(ctx).Where("id IN ?", channelIDs).Find(&chs)
	}
	// 保持 channelIDs 的顺序
	chMap := make(map[uint]channel.Channel, len(chs))
	for _, ch := range chs {
		chMap[ch.ID] = ch
	}

	infos := make([]channelInfo, 0, len(members))
	for _, m := range members {
		if ch, ok := chMap[m.ChannelID]; ok {
			infos = append(infos, channelInfo{ID: ch.ID, Name: ch.Name, Type: ch.Type, Status: ch.Status, Weight: ch.Weight})
		}
	}

	return &ChannelGroupDetail{ChannelGroup: cg, Channels: infos}, nil
}

// GetKeysGroup 获取密钥分组详情（含密钥+渠道分组绑定）
func (r *Router) GetKeysGroup(ctx context.Context, id uint) (*KeysGroupDetail, error) {
	var kg keys.KeysGroup
	if err := r.db.WithContext(ctx).First(&kg, id).Error; err != nil {
		return nil, err
	}

	// 已绑定的密钥
	var boundKeysIDs []uint
	r.db.WithContext(ctx).Model(&keys.KeysGroupMember{}).
		Where("group_id = ?", id).Pluck("keys_id", &boundKeysIDs)
	var boundKeys []keys.Keys
	if len(boundKeysIDs) > 0 {
		r.db.WithContext(ctx).Where("id IN ?", boundKeysIDs).Order("name ASC").Find(&boundKeys)
	} else {
		boundKeys = []keys.Keys{}
	}

	// 全部密钥（用于判断可用）
	var allKeys []keys.Keys
	r.db.WithContext(ctx).Order("name ASC").Find(&allKeys)
	boundKeySet := make(map[uint]bool, len(boundKeysIDs))
	for _, kid := range boundKeysIDs {
		boundKeySet[kid] = true
	}
	availableKeys := make([]keys.Keys, 0)
	for _, k := range allKeys {
		if !boundKeySet[k.ID] {
			availableKeys = append(availableKeys, k)
		}
	}
	if availableKeys == nil {
		availableKeys = []keys.Keys{}
	}

	ki := func(ks []keys.Keys) []keysInfo {
		res := make([]keysInfo, len(ks))
		for i, k := range ks {
			res[i] = keysInfo{ID: k.ID, Name: k.Name, Prefix: k.APIKeyPrefix, Status: k.Status}
		}
		return res
	}

	// 已绑定的渠道分组
	var boundIDs []uint
	r.db.WithContext(ctx).Table("keys_group_channel_groups").
		Where("keys_group_id = ?", id).Pluck("channel_group_id", &boundIDs)

	var bound []channel.ChannelGroup
	if len(boundIDs) > 0 {
		r.db.WithContext(ctx).Where("id IN ?", boundIDs).Order("weight DESC, id ASC").Find(&bound)
	} else {
		bound = []channel.ChannelGroup{}
	}

	// 全部渠道分组
	var all []channel.ChannelGroup
	r.db.WithContext(ctx).Order("weight DESC, id ASC").Find(&all)

	// 可选 = 全部 - 已绑
	boundSet := make(map[uint]bool, len(boundIDs))
	for _, bid := range boundIDs {
		boundSet[bid] = true
	}
	available := make([]channel.ChannelGroup, 0)
	for _, g := range all {
		if !boundSet[g.ID] {
			available = append(available, g)
		}
	}
	if available == nil {
		available = []channel.ChannelGroup{}
	}

	return &KeysGroupDetail{
		KeysGroup:               kg,
		BoundKeys:              ki(boundKeys),
		AvailableKeys:           ki(availableKeys),
		BoundChannelGroups:    bound,
		AvailableChannelGroups: available,
	}, nil
}

// ========== 批量设置接口 ==========

// SetChannelGroupChannels 全量替换渠道分组内的渠道
func (r *Router) SetChannelGroupChannels(ctx context.Context, groupID uint, channelIDs []uint) error {
	tx := r.db.WithContext(ctx).Begin()
	if err := tx.Where("group_id = ?", groupID).Delete(&channel.ChannelGroupMember{}).Error; err != nil {
		tx.Rollback()
		return err
	}
	for _, chID := range channelIDs {
		if err := tx.Create(&channel.ChannelGroupMember{GroupID: groupID, ChannelID: chID}).Error; err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit().Error
}

// SetKeysGroupChannelGroups 全量替换密钥分组可访问的渠道分组
func (r *Router) SetKeysGroupChannelGroups(ctx context.Context, groupID uint, channelGroupIDs []uint) error {
	tx := r.db.WithContext(ctx).Begin()
	if err := tx.Table("keys_group_channel_groups").
		Where("keys_group_id = ?", groupID).Delete(nil).Error; err != nil {
		tx.Rollback()
		return err
	}
	for _, cgID := range channelGroupIDs {
		if err := tx.Table("keys_group_channel_groups").Create(map[string]interface{}{
			"keys_group_id":    groupID,
			"channel_group_id": cgID,
		}).Error; err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit().Error
}