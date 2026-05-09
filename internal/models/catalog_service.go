package models

import (
	"context"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// CatalogService 模型目录服务
type CatalogService interface {
	// SyncFromChannelModels 从 channel_models 全量聚合同步到 model_catalog
	SyncFromChannelModels(ctx context.Context) error
	// ListCatalog 返回完整目录
	ListCatalog(ctx context.Context) ([]ModelCatalog, error)
	// UpdateVisibility 切换模型可见性
	UpdateVisibility(ctx context.Context, id uint, visible bool) error
	// GetVisibleModels 获取可见模型列表（/v1/models 使用）
	GetVisibleModels(ctx context.Context) ([]ModelCatalog, error)
}

type catalogService struct {
	db     *gorm.DB
	logger *zap.Logger
}

// NewCatalogService 创建模型目录服务
func NewCatalogService(db *gorm.DB, logger *zap.Logger) CatalogService {
	return &catalogService{db: db, logger: logger}
}

// SyncFromChannelModels 从 channel_models 全量聚合同步到 model_catalog
//
// 同步逻辑（全量刷新）：
//  1. 从所有活跃渠道的 channel_models 聚合当前有效的 display_model_name 集合
//  2. 与 model_catalog 对比：新增 INSERT、已存在则更新 ref_count / is_mapped、
//     不再被任何渠道引用的标记 visible=false（不删记录，保留管理员手动配置）
//  3. 不覆盖已有的 is_mapped 标记（管理员可能手动调整过）
func (s *catalogService) SyncFromChannelModels(ctx context.Context) error {
	// 1. 聚合查询：从活跃渠道的 enabled 模型中按 display_model_name 分组
	type aggRow struct {
		DisplayName string
		HasMapping  bool // 是否存在 display != actual 的记录
		RefCount    int
	}
	var rows []aggRow
	err := s.db.WithContext(ctx).Raw(`
		SELECT
			cm.display_model_name AS display_name,
			MAX(CASE WHEN cm.display_model_name != cm.actual_model_name THEN 1 ELSE 0 END) = 1 AS has_mapping,
			COUNT(*) AS ref_count
		FROM channel_models cm
		JOIN channels c ON c.id = cm.channel_id
		WHERE c.status != 'disabled' AND cm.status = 'enabled'
		GROUP BY cm.display_model_name
	`).Scan(&rows).Error
	if err != nil {
		return err
	}

	// 2. 构建当前有效模型名集合
	currentSet := make(map[string]aggRow, len(rows))
	for _, r := range rows {
		currentSet[r.DisplayName] = r
	}

	// 3. 获取现有 catalog
	var existing []ModelCatalog
	if err := s.db.WithContext(ctx).Find(&existing).Error; err != nil {
		return err
	}
	existingMap := make(map[string]*ModelCatalog, len(existing))
	for i := range existing {
		existingMap[existing[i].ModelName] = &existing[i]
	}

	// 4. 对比：新增 / 更新
	for _, r := range rows {
		if cat, ok := existingMap[r.DisplayName]; ok {
			// 已存在：更新 ref_count，但保留已有的 is_mapped（不覆盖）
			updates := map[string]interface{}{
				"ref_count": r.RefCount,
			}
			// 只有在 catalog 中 is_mapped=false 且当前也没有映射时，保持不变
			// 如果当前聚合结果显示有映射，且 catalog 里还没标记，才更新
			if r.HasMapping && !cat.IsMapped {
				updates["is_mapped"] = true
			}
			// 如果之前被标记为不可见（因为从所有渠道移除过），现在又有了引用，自动恢复可见
			if !cat.Visible && r.RefCount > 0 {
				updates["visible"] = true
			}
			s.db.WithContext(ctx).Model(cat).Updates(updates)
		} else {
			// 新条目：INSERT
			s.db.WithContext(ctx).Create(&ModelCatalog{
				ModelName: r.DisplayName,
				IsMapped:  r.HasMapping,
				Visible:   true,
				RefCount:  r.RefCount,
			})
		}
	}

	// 5. 标记不再被任何渠道引用的模型：ref_count=0, visible=false
	// 不删除记录——保留管理员的可见性配置，以便模型重新出现时恢复
	var activeNames []string
	for _, r := range rows {
		activeNames = append(activeNames, r.DisplayName)
	}
	updateQuery := s.db.WithContext(ctx).Model(&ModelCatalog{})
	if len(activeNames) > 0 {
		updateQuery = updateQuery.Where("model_name NOT IN ?", activeNames)
	}
	result := updateQuery.Updates(map[string]interface{}{
		"ref_count": 0,
		"visible":   false,
	})
	if result.RowsAffected > 0 {
		s.logger.Info("model catalog: some models no longer referenced by any channel, marked invisible",
			zap.Int64("count", result.RowsAffected),
		)
	}

	return nil
}

// ListCatalog 返回完整目录
func (s *catalogService) ListCatalog(ctx context.Context) ([]ModelCatalog, error) {
	var list []ModelCatalog
	err := s.db.WithContext(ctx).Order("model_name").Find(&list).Error
	return list, err
}

// UpdateVisibility 切换模型可见性
func (s *catalogService) UpdateVisibility(ctx context.Context, id uint, visible bool) error {
	return s.db.WithContext(ctx).Model(&ModelCatalog{}).Where("id = ?", id).
		Update("visible", visible).Error
}

// GetVisibleModels 获取可见模型列表（/v1/models 使用）
func (s *catalogService) GetVisibleModels(ctx context.Context) ([]ModelCatalog, error) {
	var list []ModelCatalog
	err := s.db.WithContext(ctx).Where("visible = ? AND ref_count > 0", true).Order("model_name").Find(&list).Error
	return list, err
}
