package models

import "time"

// ModelCatalog 全局模型目录（从 channel_models 聚合去重）
type ModelCatalog struct {
	ID         uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	ModelName  string    `gorm:"size:100;not null;uniqueIndex" json:"model_name"` // display_model_name 去重后的值
	IsMapped   bool      `gorm:"not null;default:false" json:"is_mapped"`        // 是否为自定义映射
	Visible    bool      `gorm:"not null;default:true" json:"visible"`           // 是否在 /v1/models 返回
	RefCount   int       `gorm:"not null;default:1" json:"ref_count"`            // 被多少条 channel_model 引用
	CreatedAt  time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt  time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (ModelCatalog) TableName() string { return "model_catalog" }
