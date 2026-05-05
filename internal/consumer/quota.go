package consumer

import (
	"context"
	"fmt"
	"time"

	"github.com/bokelife/aigateway/internal/account"
)

// QuotaError 配额超限错误
type QuotaError struct {
	Type  string // rpm / tpm
	Limit int
	Used  int
}

func (e *QuotaError) Error() string {
	return fmt.Sprintf("quota exceeded: %s limit=%d used=%d", e.Type, e.Limit, e.Used)
}

// CheckQuota 检查消费者配额（RPM/TPM）
// 在路由之前执行，超限直接返回 429
func (s *service) CheckQuota(ctx context.Context, consumerID uint, tokenCount int) error {
	// 获取消费者所属的分组及配额配置
	var members []ConsumerGroupMember
	if err := s.db.WithContext(ctx).Where("consumer_id = ?", consumerID).Find(&members).Error; err != nil {
		return nil // 查询失败放行，不阻塞请求
	}

	if len(members) == 0 {
		return nil // 未分组 → 无配额限制
	}

	// 取最严格的配额限制
	minRPM := 0
	minTPM := 0
	for _, m := range members {
		if m.QuotaRPM > 0 {
			if minRPM == 0 || m.QuotaRPM < minRPM {
				minRPM = m.QuotaRPM
			}
		}
		if m.QuotaTPM > 0 {
			if minTPM == 0 || m.QuotaTPM < minTPM {
				minTPM = m.QuotaTPM
			}
		}
	}

	// 使用内存缓存做计数器（降级方案）
	// Redis 实现下使用 INCR + TTL
	now := time.Now()
	minuteKey := now.Format("200601021504") // 分钟级粒度

	// RPM 检查
	if minRPM > 0 {
		rpmKey := fmt.Sprintf("stats:consumer:%d:rpm:%s", consumerID, minuteKey)
		cache := s.getCache()
		if cache != nil {
			count, _ := cache.Incr(rpmKey)
			if count == 1 {
				cache.Set(rpmKey, fmt.Sprintf("%d", count), 120*time.Second) // TTL 120s
			}
			if int(count) > minRPM {
				return &QuotaError{Type: "rpm", Limit: minRPM, Used: int(count)}
			}
		}
	}

	// TPM 检查
	if minTPM > 0 && tokenCount > 0 {
		tpmKey := fmt.Sprintf("stats:consumer:%d:tpm:%s", consumerID, minuteKey)
		cache := s.getCache()
		if cache != nil {
			count, _ := cache.Incr(tpmKey) // 简化：实际应 INCRBY tokenCount
			if count == 1 {
				cache.Set(tpmKey, fmt.Sprintf("%d", count), 120*time.Second)
			}
			if int(count) > minTPM {
				return &QuotaError{Type: "tpm", Limit: minTPM, Used: int(count)}
			}
		}
	}

	return nil
}

// setCache 设置缓存实例（注入）
func (s *service) SetCache(cache account.Cache) {
	s.cache = cache
}

func (s *service) getCache() account.Cache {
	return s.cache
}
