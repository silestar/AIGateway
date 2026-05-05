package channel

import "time"

// ConsumerGroupChannelGroup 消费者分组↔渠道分组关联
type ConsumerGroupChannelGroup struct {
	ID               uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	ConsumerGroupID  uint      `gorm:"not null;index" json:"consumer_group_id"`
	ChannelGroupID   uint      `gorm:"not null;index" json:"channel_group_id"`
	CreatedAt        time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (ConsumerGroupChannelGroup) TableName() string { return "consumer_group_channel_groups" }
