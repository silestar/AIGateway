package storage

import "gorm.io/gorm"

// Storage 存储抽象接口
type Storage interface {
	GetDB() *gorm.DB
	Close() error
}
