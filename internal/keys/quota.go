package keys

import (
	"context"
	"fmt"
	"time"

	"github.com/silestar/AIGateway/internal/account"
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

// CheckQuota 检查密钥所在分组的配额（RPM/TPM）
// 配额从 KeysGroup 取值，语义：该分组下每个密钥各自的限额（非分组共享总量）
func (s *service) CheckQuota(ctx context.Context, keysID uint, tokenCount int) error {
	// 1. 查密钥所在的分组
	var member KeysGroupMember
	if err := s.db.WithContext(ctx).Where("keys_id = ?", keysID).First(&member).Error; err != nil {
		return nil // 未加入任何分组，不限制
	}

	// 2. 查分组的配额
	var group KeysGroup
	if err := s.db.WithContext(ctx).First(&group, member.GroupID).Error; err != nil {
		return nil
	}

	if group.QuotaRPM == 0 && group.QuotaTPM == 0 {
		return nil
	}

	now := time.Now()
	minuteKey := now.Format("200601021504")

	cache := s.getCache()
	if cache == nil {
		return nil
	}

	if group.QuotaRPM > 0 {
		rpmKey := fmt.Sprintf("stats:keys:%d:rpm:%s", keysID, minuteKey)
		count, _ := cache.Incr(rpmKey)
		if count == 1 {
			cache.Set(rpmKey, fmt.Sprintf("%d", count), 120*time.Second)
		}
		if int(count) > group.QuotaRPM {
			return &QuotaError{Type: "rpm", Limit: group.QuotaRPM, Used: int(count)}
		}
	}

	if group.QuotaTPM > 0 && tokenCount > 0 {
		tpmKey := fmt.Sprintf("stats:keys:%d:tpm:%s", keysID, minuteKey)
		count, _ := cache.Incr(tpmKey)
		if count == 1 {
			cache.Set(tpmKey, fmt.Sprintf("%d", count), 120*time.Second)
		}
		if int(count) > group.QuotaTPM {
			return &QuotaError{Type: "tpm", Limit: group.QuotaTPM, Used: int(count)}
		}
	}

	return nil
}

func (s *service) SetCache(cache account.Cache) {
	s.cache = cache
}

func (s *service) getCache() account.Cache {
	return s.cache
}