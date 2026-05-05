package channel

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	adapterregistry "github.com/bokelife/aigateway/pkg/adapter/registry"
)

type service struct {
	db *gorm.DB
}

// NewService 创建渠道服务
func NewService(db *gorm.DB) ChannelService {
	return &service{db: db}
}

func (s *service) Create(ctx context.Context, name, channelType, baseURL string) (*Channel, error) {
	// 校验渠道类型
	if _, err := adapterregistry.GetAdapter(channelType); err != nil {
		return nil, fmt.Errorf("unsupported channel type: %s", channelType)
	}

	ch := &Channel{
		Name:    name,
		Type:    channelType,
		BaseURL: baseURL,
		Status:  "active",
		Weight:  0,
	}

	if err := s.db.WithContext(ctx).Create(ch).Error; err != nil {
		return nil, fmt.Errorf("create channel: %w", err)
	}
	return ch, nil
}

func (s *service) GetById(ctx context.Context, id uint) (*Channel, error) {
	var ch Channel
	if err := s.db.WithContext(ctx).First(&ch, id).Error; err != nil {
		return nil, err
	}
	return &ch, nil
}

func (s *service) List(ctx context.Context, filter ListFilter) ([]Channel, int64, error) {
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize < 1 || filter.PageSize > 100 {
		filter.PageSize = 20
	}

	var channels []Channel
	var total int64

	query := s.db.WithContext(ctx).Model(&Channel{})
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	if filter.Type != "" {
		query = query.Where("type = ?", filter.Type)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (filter.Page - 1) * filter.PageSize
	if err := query.Order("id DESC").Offset(offset).Limit(filter.PageSize).Find(&channels).Error; err != nil {
		return nil, 0, err
	}

	return channels, total, nil
}

func (s *service) Update(ctx context.Context, id uint, name, baseURL string, weight int) error {
	return s.db.WithContext(ctx).Model(&Channel{}).Where("id = ?", id).
		Updates(map[string]interface{}{"name": name, "base_url": baseURL, "weight": weight}).Error
}

func (s *service) Delete(ctx context.Context, id uint) error {
	tx := s.db.WithContext(ctx).Begin()
	// 删除关联的模型映射
	if err := tx.Where("channel_id = ?", id).Delete(&ChannelModel{}).Error; err != nil {
		tx.Rollback()
		return err
	}
	// 删除渠道
	if err := tx.Delete(&Channel{}, id).Error; err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit().Error
}

func (s *service) FetchModels(ctx context.Context, id uint, testKey string) ([]ModelInfo, error) {
	var ch Channel
	if err := s.db.WithContext(ctx).First(&ch, id).Error; err != nil {
		return nil, fmt.Errorf("channel not found: %w", err)
	}

	// 获取适配器
	adp, err := adapterregistry.GetAdapter(ch.Type)
	if err != nil {
		return nil, err
	}

	// 使用测试 Key 或渠道账号的 Key
	apiKey := testKey
	if apiKey == "" {
		// 从第一个 active 账号获取
		var acc struct {
			APIKeyEncrypted string
		}
		if err := s.db.WithContext(ctx).Table("channel_accounts").
			Where("channel_id = ? AND status = ?", id, "active").
			Select("api_key_encrypted").Order("priority ASC").First(&acc).Error; err != nil {
			return nil, fmt.Errorf("no active account for channel %d: %w", id, err)
		}
		// 需要解密，但 channel service 不应直接依赖 crypto
		// 这里通过 testKey 参数由 handler 层解密后传入
		return nil, fmt.Errorf("no test key provided, please provide via test_key parameter")
	}

	adapterModels, err := adp.FetchModels(ctx, ch.BaseURL, apiKey)
	if err != nil {
		return nil, err
	}
	// 转换 adapter.ModelInfo → channel.ModelInfo
	result := make([]ModelInfo, len(adapterModels))
	for i, m := range adapterModels {
		result[i] = ModelInfo{ID: m.ID, OwnedBy: m.OwnedBy}
	}
	return result, nil
}

func (s *service) SaveModels(ctx context.Context, id uint, models []ChannelModel) error {
	tx := s.db.WithContext(ctx).Begin()

	// 删除旧的模型映射
	if err := tx.Where("channel_id = ?", id).Delete(&ChannelModel{}).Error; err != nil {
		tx.Rollback()
		return err
	}

	// 批量插入新的模型映射
	for i := range models {
		models[i].ChannelID = id
		if err := tx.Create(&models[i]).Error; err != nil {
			tx.Rollback()
			return err
		}
	}

	return tx.Commit().Error
}
