package account

import (
	"fmt"
	"sync"
	"time"
)

// Cache 缓存抽象接口（Redis 或内存降级）
type Cache interface {
	Get(key string) (string, error)
	Set(key string, value string, ttl time.Duration) error
	Del(key string) error
	Incr(key string) (int64, error)
	Decr(key string) (int64, error)
	SetNX(key string, value string, ttl time.Duration) (bool, error)
}

// ========== 内存缓存实现（Redis 降级方案） ==========

type memoryCache struct {
	mu    sync.RWMutex
	items map[string]*cacheItem
}

type cacheItem struct {
	value   string
	expiry  time.Time
}

// NewMemoryCache 创建内存缓存实例
func NewMemoryCache() Cache {
	c := &memoryCache{
		items: make(map[string]*cacheItem),
	}
	go c.cleanup()
	return c
}

func (c *memoryCache) Get(key string) (string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	item, ok := c.items[key]
	if !ok || time.Now().After(item.expiry) {
		return "", fmt.Errorf("key not found: %s", key)
	}
	return item.value, nil
}

func (c *memoryCache) Set(key string, value string, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[key] = &cacheItem{value: value, expiry: time.Now().Add(ttl)}
	return nil
}

func (c *memoryCache) Del(key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.items, key)
	return nil
}

func (c *memoryCache) Incr(key string) (int64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	item, ok := c.items[key]
	if !ok {
		c.items[key] = &cacheItem{value: "1", expiry: time.Now().Add(48 * time.Hour)}
		return 1, nil
	}
	val := parseInt(item.value) + 1
	item.value = fmt.Sprintf("%d", val)
	return int64(val), nil
}

func (c *memoryCache) Decr(key string) (int64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	item, ok := c.items[key]
	if !ok {
		c.items[key] = &cacheItem{value: "-1", expiry: time.Now().Add(48 * time.Hour)}
		return -1, nil
	}
	val := parseInt(item.value) - 1
	item.value = fmt.Sprintf("%d", val)
	return int64(val), nil
}

func (c *memoryCache) SetNX(key string, value string, ttl time.Duration) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	item, ok := c.items[key]
	if ok && time.Now().Before(item.expiry) {
		return false, nil
	}
	c.items[key] = &cacheItem{value: value, expiry: time.Now().Add(ttl)}
	return true, nil
}

func (c *memoryCache) cleanup() {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		c.mu.Lock()
		now := time.Now()
		for k, v := range c.items {
			if now.After(v.expiry) {
				delete(c.items, k)
			}
		}
		c.mu.Unlock()
	}
}

func parseInt(s string) int {
	var n int
	fmt.Sscanf(s, "%d", &n)
	return n
}
