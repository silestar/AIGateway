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

func (s *service) Update(ctx context.Context, id uint, name, baseURL string, weight, maxRPM, maxTPM, maxDailyRequests int) error {
	return s.db.WithContext(ctx).Model(&Channel{}).Where("id = ?", id).
		Updates(map[string]interface{}{"name": name, "base_url": baseURL, "weight": weight, "max_rpm": maxRPM, "max_tpm": maxTPM, "max_daily_requests": maxDailyRequests}).Error
}

func (s *service) UpdateStatus(ctx context.Context, id uint, status string) error {
	return s.db.WithContext(ctx).Model(&Channel{}).Where("id = ?", id).
		Update("status", status).Error
}

func (s *service) UpdateWeight(ctx context.Context, id uint, weight int) error {
	return s.db.WithContext(ctx).Model(&Channel{}).Where("id = ?", id).
		Update("weight", weight).Error
}

func (s *service) TestConnection(ctx context.Context, channelType, baseURL, apiKey string) error {
	adp, err := adapterregistry.GetAdapter(channelType)
	if err != nil {
		return fmt.Errorf("unsupported channel type: %s", channelType)
	}
	_, err = adp.FetchModels(ctx, baseURL, apiKey)
	return err
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

	adapterModels, err := adp.FetchModels(ctx, ch.BaseURL, testKey)
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

func (s *service) GetModelsByChannel(ctx context.Context, id uint) ([]ChannelModel, error) {
	var models []ChannelModel
	if err := s.db.WithContext(ctx).Where("channel_id = ?", id).Order("display_model_name").Find(&models).Error; err != nil {
		return nil, err
	}
	return models, nil
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
