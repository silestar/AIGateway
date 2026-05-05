package postgres

// PostgreSQL 存储实现桩
// 阶段一仅定义接口，后续阶段按需实现

import (
	"fmt"

	"gorm.io/gorm"

	"github.com/bokelife/aigateway/internal/config"
)

type PostgresStorage struct {
	db *gorm.DB
}

func New(cfg config.DBConfig) (*PostgresStorage, error) {
	return nil, fmt.Errorf("postgres storage not implemented yet")
}

func (s *PostgresStorage) GetDB() *gorm.DB {
	return s.db
}

func (s *PostgresStorage) Close() error {
	if s.db == nil {
		return nil
	}
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
