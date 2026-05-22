package postgres

import (
	"fmt"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/silestar/AIGateway/internal/account"
	"github.com/silestar/AIGateway/internal/channel"
	"github.com/silestar/AIGateway/internal/config"
	"github.com/silestar/AIGateway/internal/keys"
	"github.com/silestar/AIGateway/internal/plugin"
	"github.com/silestar/AIGateway/internal/stats"
)

type PostgresStorage struct {
	db *gorm.DB
}

// New 创建 PostgreSQL 存储
func New(cfg config.DBConfig) (*PostgresStorage, error) {
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d sslmode=disable TimeZone=Asia/Shanghai",
		cfg.Host, cfg.User, cfg.Password, cfg.Name, cfg.Port)

	gormConfig := &gorm.Config{}
	if cfg.Host == "" {
		cfg.Host = "127.0.0.1"
	}
	if cfg.Port == 0 {
		cfg.Port = 5432
	}

	db, err := gorm.Open(postgres.Open(dsn), gormConfig)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}

	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)

	if err := autoMigrate(db); err != nil {
		return nil, fmt.Errorf("auto migrate: %w", err)
	}

	return &PostgresStorage{db: db}, nil
}

// NewWithLogger 创建带日志级别的 PostgreSQL 存储
func NewWithLogger(cfg config.DBConfig, logLevel logger.LogLevel) (*PostgresStorage, error) {
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d sslmode=disable TimeZone=Asia/Shanghai",
		cfg.Host, cfg.User, cfg.Password, cfg.Name, cfg.Port)

	gormConfig := &gorm.Config{
		Logger: logger.Default.LogMode(logLevel),
	}

	db, err := gorm.Open(postgres.Open(dsn), gormConfig)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}

	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)

	if err := autoMigrate(db); err != nil {
		return nil, fmt.Errorf("auto migrate: %w", err)
	}

	return &PostgresStorage{db: db}, nil
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
		&plugin.Hook{},
		&plugin.PluginPermission{},
	)
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