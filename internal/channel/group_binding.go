package channel

import "time"

// KeysGroupChannelGroup 密钥分组↔渠道分组关联
type KeysGroupChannelGroup struct {
	ID               uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	KeysGroupID  uint      `gorm:"not null;index" json:"keys_group_id"`
	ChannelGroupID   uint      `gorm:"not null;index" json:"channel_group_id"`
	CreatedAt        time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (KeysGroupChannelGroup) TableName() string { return "keys_group_channel_groups" }
