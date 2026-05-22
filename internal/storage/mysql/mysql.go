package mysql

import (
	"fmt"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/silestar/AIGateway/internal/account"
	"github.com/silestar/AIGateway/internal/channel"
	"github.com/silestar/AIGateway/internal/config"
	"github.com/silestar/AIGateway/internal/keys"
	"github.com/silestar/AIGateway/internal/plugin"
	"github.com/silestar/AIGateway/internal/stats"
)

type MySQLStorage struct {
	db *gorm.DB
}

// New 创建 MySQL 存储
func New(cfg config.DBConfig) (*MySQLStorage, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.Name)

	gormConfig := &gorm.Config{}
	if cfg.Host == "" {
		cfg.Host = "127.0.0.1"
	}
	if cfg.Port == 0 {
		cfg.Port = 3306
	}

	db, err := gorm.Open(mysql.Open(dsn), gormConfig)
	if err != nil {
		return nil, fmt.Errorf("open mysql: %w", err)
	}

	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)

	if err := autoMigrate(db); err != nil {
		return nil, fmt.Errorf("auto migrate: %w", err)
	}

	return &MySQLStorage{db: db}, nil
}

// NewWithLogger 创建带日志级别的 MySQL 存储
func NewWithLogger(cfg config.DBConfig, logLevel logger.LogLevel) (*MySQLStorage, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.Name)

	gormConfig := &gorm.Config{
		Logger: logger.Default.LogMode(logLevel),
	}

	db, err := gorm.Open(mysql.Open(dsn), gormConfig)
	if err != nil {
		return nil, fmt.Errorf("open mysql: %w", err)
	}

	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)

	if err := autoMigrate(db); err != nil {
		return nil, fmt.Errorf("auto migrate: %w", err)
	}

	return &MySQLStorage{db: db}, nil
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