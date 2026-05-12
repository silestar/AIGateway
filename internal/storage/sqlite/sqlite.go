package sqlite

import (
	"fmt"
	"os"
	"path/filepath"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/silestar/AIGateway/internal/config"
	"github.com/silestar/AIGateway/internal/keys"
	"github.com/silestar/AIGateway/internal/channel"
	"github.com/silestar/AIGateway/internal/account"
	"github.com/silestar/AIGateway/internal/stats"
	"github.com/silestar/AIGateway/internal/plugin"
)

type SQLiteStorage struct {
	db *gorm.DB
}

// New 创建 SQLite 存储
func New(cfg config.DBConfig) (*SQLiteStorage, error) {
	dir := filepath.Dir(cfg.Path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create db directory: %w", err)
	}

	gormConfig := &gorm.Config{}
	if cfg.Path == "" {
		cfg.Path = "data/agw.db"
	}

	db, err := gorm.Open(sqlite.Open(cfg.Path+"?_journal_mode=WAL"), gormConfig)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)

	if err := autoMigrate(db); err != nil {
		return nil, fmt.Errorf("auto migrate: %w", err)
	}

	return &SQLiteStorage{db: db}, nil
}

func NewWithLogger(cfg config.DBConfig, logLevel logger.LogLevel) (*SQLiteStorage, error) {
	dir := filepath.Dir(cfg.Path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create db directory: %w", err)
	}

	gormConfig := &gorm.Config{
		Logger: logger.Default.LogMode(logLevel),
	}

	db, err := gorm.Open(sqlite.Open(cfg.Path+"?_journal_mode=WAL"), gormConfig)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)

	if err := autoMigrate(db); err != nil {
		return nil, fmt.Errorf("auto migrate: %w", err)
	}

	return &SQLiteStorage{db: db}, nil
}

func autoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&keys.Keys{},
		&channel.Channel{},
		&channel.ChannelModel{},
		&account.Account{},
		&channel.ChannelGroup{},
		&channel.ChannelGroupMember{},
		&keys.KeysGroup{},
		&keys.KeysGroupMember{},
		&channel.KeysGroupChannelGroup{},
		&stats.RequestLog{},
		&stats.SystemDailyStats{},
		&stats.KeysDailyStats{},
		&stats.ChannelDailyStats{},
		&plugin.Plugin{},
		&plugin.PluginPermission{},
	)
}

func (s *SQLiteStorage) GetDB() *gorm.DB {
	return s.db
}

func (s *SQLiteStorage) Close() error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}