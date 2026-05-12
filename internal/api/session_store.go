package api

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// Session 管理端会话记录 — 持久化到 SQLite sessions 表
// 容器重启后登录态不丢失
type Session struct {
	Token     string    `gorm:"primaryKey;size:64" json:"token"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

func (Session) TableName() string {
	return "sessions"
}

// SessionStore session 存储接口 — 支持 SQLite 和 Redis
type SessionStore interface {
	Save(token string, expireAt time.Time) error
	Validate(token string) (bool, error)
	Cleanup() error
}

// ==================== SQLite 实现 ====================

// SQLiteSessionStore 基于 GORM 的 SQLite session 存储
type SQLiteSessionStore struct {
	db *gorm.DB
}

// NewSQLiteSessionStore 创建 SQLite session 存储并自动建表
func NewSQLiteSessionStore(db *gorm.DB) *SQLiteSessionStore {
	db.AutoMigrate(&Session{})
	return &SQLiteSessionStore{db: db}
}

func (s *SQLiteSessionStore) Save(token string, expireAt time.Time) error {
	session := &Session{
		Token:     token,
		CreatedAt: time.Now(),
		ExpiresAt: expireAt,
	}
	return s.db.Save(session).Error
}

func (s *SQLiteSessionStore) Validate(token string) (bool, error) {
	var session Session
	if err := s.db.Where("token = ?", token).First(&session).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return false, nil
		}
		return false, err
	}
	if time.Now().After(session.ExpiresAt) {
		s.db.Delete(&session)
		return false, nil
	}
	return true, nil
}

func (s *SQLiteSessionStore) Cleanup() error {
	return s.db.Where("expires_at < ?", time.Now()).Delete(&Session{}).Error
}

// ==================== Redis 实现 ====================

const (
	sessionKeyPrefix = "agw_session:"
	sessionTTL       = 24 * time.Hour
)

// RedisSessionStore 基于 Redis 的 session 存储
// TTL 自动过期，无需清理协程
type RedisSessionStore struct {
	client *redis.Client
}

// NewRedisSessionStore 创建 Redis session 存储
func NewRedisSessionStore(addr, password string, db int) (*RedisSessionStore, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	// 连接测试
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		client.Close()
		return nil, fmt.Errorf("redis ping failed: %w", err)
	}

	return &RedisSessionStore{client: client}, nil
}

func (r *RedisSessionStore) Save(token string, expireAt time.Time) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return r.client.Set(ctx, sessionKeyPrefix+token, "1", sessionTTL).Err()
}

func (r *RedisSessionStore) Validate(token string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := r.client.Get(ctx, sessionKeyPrefix+token).Result()
	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (r *RedisSessionStore) Cleanup() error {
	// Redis TTL 自动过期，无需手动清理
	return nil
}

func (r *RedisSessionStore) Close() error {
	return r.client.Close()
}

// ==================== 降级：内存 map（兜底） ====================

// MemSessionStore 内存 map 兜底存储（极端情况）
type MemSessionStore struct {
	sessions map[string]time.Time
}

func NewMemSessionStore() *MemSessionStore {
	return &MemSessionStore{sessions: make(map[string]time.Time)}
}

func (m *MemSessionStore) Save(token string, expireAt time.Time) error {
	m.sessions[token] = expireAt
	return nil
}

func (m *MemSessionStore) Validate(token string) (bool, error) {
	expireAt, ok := m.sessions[token]
	if !ok {
		return false, nil
	}
	if time.Now().After(expireAt) {
		delete(m.sessions, token)
		return false, nil
	}
	return true, nil
}

func (m *MemSessionStore) Cleanup() error {
	now := time.Now()
	for token, expireAt := range m.sessions {
		if now.After(expireAt) {
			delete(m.sessions, token)
		}
	}
	return nil
}

// ==================== SessionStore 工厂 ====================

// NewSessionStore 按优先级创建 SessionStore：Redis → SQLite → 内存
// 这是"普罗大众优先"原则的体现：SQLite 为默认方案，Redis 可选
func NewSessionStore(db *gorm.DB, redisCfg RedisSessionConfig, logger *zap.SugaredLogger) SessionStore {
	// 1. 尝试 Redis
	if redisCfg.Enabled {
		addr := fmt.Sprintf("%s:%d", redisCfg.Host, redisCfg.Port)
		store, err := NewRedisSessionStore(addr, redisCfg.Password, redisCfg.DB)
		if err == nil {
			return store
		}
		if logger != nil {
			logger.Warnf("Redis unavailable, falling back to SQLite: %v", err)
		}
	}

	// 2. 降级到 SQLite（默认方案）
	if db != nil {
		return NewSQLiteSessionStore(db)
	}

	// 3. 极端兜底：内存 map
	if logger != nil {
		logger.Warn("no database available, using in-memory session store (sessions lost on restart)")
	}
	return NewMemSessionStore()
}

// RedisSessionConfig Redis 连接配置（避免 import cycle）
type RedisSessionConfig struct {
	Enabled  bool
	Host     string
	Port     int
	Password string
	DB       int
}