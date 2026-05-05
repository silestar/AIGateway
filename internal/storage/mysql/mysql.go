package mysql

// MySQL 存储实现桩
// 阶段一仅定义接口，后续阶段按需实现

import (
	"fmt"

	"gorm.io/gorm"

	"github.com/bokelife/aigateway/internal/config"
)

type MySQLStorage struct {
	db *gorm.DB
}

func New(cfg config.DBConfig) (*MySQLStorage, error) {
	return nil, fmt.Errorf("mysql storage not implemented yet")
}

func (s *MySQLStorage) GetDB() *gorm.DB {
	return s.db
}

func (s *MySQLStorage) Close() error {
	if s.db == nil {
		return nil
	}
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
