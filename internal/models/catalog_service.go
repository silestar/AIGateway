package models

import (
	"context"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// CatalogService 模型目录服务（基于 channel_models 表）
type CatalogService interface {
	// GetUpstreamModels 获取上游模型去重列表（按 actual_model_name 分组）
	GetUpstreamModels(ctx context.Context) ([]UpstreamModelItem, error)
	// GetDisplayModels 获取映射模型去重列表（按 display_model_name 分组，排除 display==actual 的行）
	GetDisplayModels(ctx context.Context) ([]DisplayModelItem, error)
	// BatchSetUpstreamVisible 按 actual_model_name 批量设置 upstream_visible
	BatchSetUpstreamVisible(ctx context.Context, modelName string, visible bool) error
	// BatchSetDisplayVisible 按 display_model_name 批量设置 display_visible
	BatchSetDisplayVisible(ctx context.Context, modelName string, visible bool) error
	// GetVisibleModels 获取 /v1/models 应暴露的模型列表（去重）
	GetVisibleModels(ctx context.Context) ([]string, error)
}

// UpstreamModelItem 上游模型列表项（左列）
type UpstreamModelItem struct {
	ActualModelName string `json:"actual_model_name"`
	Visible         bool   `json:"visible"`
	RefCount        int    `json:"ref_count"`
}

// DisplayModelItem 映射模型列表项（右列）
type DisplayModelItem struct {
	DisplayModelName string `json:"display_model_name"`
	Visible          bool   `json:"visible"`
	RefCount         int    `json:"ref_count"`
}

type catalogService struct {
	db     *gorm.DB
	logger *zap.Logger
}

func NewCatalogService(db *gorm.DB, logger *zap.Logger) CatalogService {
	return &catalogService{db: db, logger: logger}
}

// GetUpstreamModels 获取上游模型去重列表
func (s *catalogService) GetUpstreamModels(ctx context.Context) ([]UpstreamModelItem, error) {
	type row struct {
		ActualModelName string
		AllVisible      bool
		RefCount        int
	}
	var rows []row
	err := s.db.WithContext(ctx).Raw(`
		SELECT
			cm.actual_model_name,
			MIN(CASE WHEN cm.upstream_visible THEN 1 ELSE 0 END) = 1 AS all_visible,
			COUNT(*) AS ref_count
		FROM channel_models cm
		JOIN channels c ON c.id = cm.channel_id
		WHERE c.status != 'disabled' AND cm.status = 'enabled'
		GROUP BY cm.actual_model_name
		ORDER BY LOWER(cm.actual_model_name)
	`).Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	items := make([]UpstreamModelItem, len(rows))
	for i, r := range rows {
		items[i] = UpstreamModelItem{
			ActualModelName: r.ActualModelName,
			Visible:         r.AllVisible,
			RefCount:        r.RefCount,
		}
	}
	return items, nil
}

// GetDisplayModels 获取映射模型去重列表（排除 display==actual 的透传模型）
func (s *catalogService) GetDisplayModels(ctx context.Context) ([]DisplayModelItem, error) {
	type row struct {
		DisplayModelName string
		AllVisible       bool
		RefCount         int
	}
	var rows []row
	err := s.db.WithContext(ctx).Raw(`
		SELECT
			cm.display_model_name,
			MIN(CASE WHEN cm.display_visible THEN 1 ELSE 0 END) = 1 AS all_visible,
			COUNT(*) AS ref_count
		FROM channel_models cm
		JOIN channels c ON c.id = cm.channel_id
		WHERE c.status != 'disabled' AND cm.status = 'enabled'
		  AND cm.display_model_name != cm.actual_model_name
		GROUP BY cm.display_model_name
		ORDER BY LOWER(cm.display_model_name)
	`).Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	items := make([]DisplayModelItem, len(rows))
	for i, r := range rows {
		items[i] = DisplayModelItem{
			DisplayModelName: r.DisplayModelName,
			Visible:          r.AllVisible,
			RefCount:         r.RefCount,
		}
	}
	return items, nil
}

// BatchSetUpstreamVisible 按 actual_model_name 批量设置 upstream_visible
func (s *catalogService) BatchSetUpstreamVisible(ctx context.Context, modelName string, visible bool) error {
	return s.db.WithContext(ctx).
		Table("channel_models").
		Where("actual_model_name = ?", modelName).
		Update("upstream_visible", visible).Error
}

// BatchSetDisplayVisible 按 display_model_name 批量设置 display_visible
func (s *catalogService) BatchSetDisplayVisible(ctx context.Context, modelName string, visible bool) error {
	return s.db.WithContext(ctx).
		Table("channel_models").
		Where("display_model_name = ?", modelName).
		Update("display_visible", visible).Error
}

// GetVisibleModels 获取 /v1/models 应暴露的模型列表（去重）
func (s *catalogService) GetVisibleModels(ctx context.Context) ([]string, error) {
	var models []string

	// 上游可见模型
	err := s.db.WithContext(ctx).Raw(`
		SELECT DISTINCT cm.actual_model_name
		FROM channel_models cm
		JOIN channels c ON c.id = cm.channel_id
		WHERE c.status != 'disabled' AND cm.status = 'enabled' AND cm.upstream_visible = true
	`).Scan(&models).Error
	if err != nil {
		return nil, err
	}

	// 映射可见模型
	var displayModels []string
	err = s.db.WithContext(ctx).Raw(`
		SELECT DISTINCT cm.display_model_name
		FROM channel_models cm
		JOIN channels c ON c.id = cm.channel_id
		WHERE c.status != 'disabled' AND cm.status = 'enabled' AND cm.display_visible = true
	`).Scan(&displayModels).Error
	if err != nil {
		return nil, err
	}

	// 合并去重
	seen := make(map[string]bool)
	var result []string
	for _, m := range models {
		if !seen[m] {
			seen[m] = true
			result = append(result, m)
		}
	}
	for _, m := range displayModels {
		if !seen[m] {
			seen[m] = true
			result = append(result, m)
		}
	}
	return result, nil
}