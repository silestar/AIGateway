package keys

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

// CheckQuota 检查密钥配额（RPM/TPM）
func (s *service) CheckQuota(ctx context.Context, keysID uint, tokenCount int) error {
	var members []KeysGroupMember
	if err := s.db.WithContext(ctx).Where("keys_id = ?", keysID).Find(&members).Error; err != nil {
		return nil
	}

	if len(members) == 0 {
		return nil
	}

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

	now := time.Now()
	minuteKey := now.Format("200601021504")

	if minRPM > 0 {
		rpmKey := fmt.Sprintf("stats:keys:%d:rpm:%s", keysID, minuteKey)
		cache := s.getCache()
		if cache != nil {
			count, _ := cache.Incr(rpmKey)
			if count == 1 {
				cache.Set(rpmKey, fmt.Sprintf("%d", count), 120*time.Second)
			}
			if int(count) > minRPM {
				return &QuotaError{Type: "rpm", Limit: minRPM, Used: int(count)}
			}
		}
	}

	if minTPM > 0 && tokenCount > 0 {
		tpmKey := fmt.Sprintf("stats:keys:%d:tpm:%s", keysID, minuteKey)
		cache := s.getCache()
		if cache != nil {
			count, _ := cache.Incr(tpmKey)
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

func (s *service) SetCache(cache account.Cache) {
	s.cache = cache
}

func (s *service) getCache() account.Cache {
	return s.cache
}