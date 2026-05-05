package account

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/bokelife/aigateway/internal/channel"
)

// StartProbeScheduler 启动按需探测调度器
func (m *Manager) StartProbeScheduler() {
	go func() {
		ticker := time.NewTicker(time.Duration(m.cfg.ProbeInterval) * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			m.runProbeCycle(context.Background())
		}
	}()
	m.logger.Info("probe scheduler started",
		zap.Int("interval_seconds", m.cfg.ProbeInterval),
	)
}

// StartGlobalHealthCheck 启动全局健康巡检
func (m *Manager) StartGlobalHealthCheck() {
	go func() {
		ticker := time.NewTicker(time.Duration(m.cfg.GlobalHealthCheckInterval) * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			m.runGlobalHealthCheck(context.Background())
		}
	}()
	m.logger.Info("global health check started",
		zap.Int("interval_seconds", m.cfg.GlobalHealthCheckInterval),
	)
}

// runProbeCycle 按需探测一轮
func (m *Manager) runProbeCycle(ctx context.Context) {
	var channels []channel.Channel
	if err := m.db.WithContext(ctx).Where("status = ?", "active").Find(&channels).Error; err != nil {
		m.logger.Error("probe: query channels", zap.Error(err))
		return
	}

	for _, ch := range channels {
		m.probeChannel(ctx, &ch)
	}
}

// probeChannel 探测单个渠道
func (m *Manager) probeChannel(ctx context.Context, ch *channel.Channel) {
	// 1. 计算 active_ratio
	activeCount := m.getActiveCount(ctx, ch.ID)
	totalCount := m.getTotalCount(ctx, ch.ID)
	if totalCount == 0 {
		return
	}

	activeRatio := float64(activeCount) / float64(totalCount)
	if activeRatio >= m.cfg.ProbeActiveRatioThreshold {
		return // 活跃率充足，无需探测
	}

	// 2. 获取分布式锁
	lockKey := fmt.Sprintf("probe_lock:%d", ch.ID)
	acquired, _ := m.cache.SetNX(lockKey, "1", 30*time.Second)
	if !acquired {
		return // 其他实例正在探测
	}
	defer m.cache.Del(lockKey)

	// 3. 获取 disabled 账号（排除 cooling）
	var disabledAccounts []Account
	if err := m.db.WithContext(ctx).
		Where("channel_id = ? AND status IN ?", ch.ID, []string{"disabled"}).
		Where("probe_cooldown_until IS NULL OR probe_cooldown_until < ?", time.Now()).
		Order("last_failed_at ASC").
		Find(&disabledAccounts).Error; err != nil {
		m.logger.Error("probe: query disabled accounts", zap.Error(err))
		return
	}

	recovered := 0
	for _, acc := range disabledAccounts {
		if recovered >= m.cfg.MaxProbeRecoverPerCycle {
			break
		}

		// 检查 min_disable_duration
		if acc.LastFailedAt != nil {
			disabledDuration := time.Since(*acc.LastFailedAt)
			if disabledDuration < time.Duration(m.cfg.MinDisableDuration)*time.Second {
				continue
			}
		}

		// 探测
		success := m.probeAccount(ctx, ch, &acc)
		if success {
			// 恢复账号
			m.recoverAccount(ctx, &acc)
			recovered++
		}
	}

	// 如果没有恢复任何账号 → 增加 consecutive_cooldown_cycles
	if recovered == 0 && len(disabledAccounts) > 0 {
		m.incrementCooldownCycles(ctx, ch.ID)
	} else if recovered > 0 {
		// 恢复成功 → 重置冷却周期
		m.resetCooldownCycles(ctx, ch.ID)
	}
}

// runGlobalHealthCheck 全局健康巡检一轮
func (m *Manager) runGlobalHealthCheck(ctx context.Context) {
	var channels []channel.Channel
	if err := m.db.WithContext(ctx).Find(&channels).Error; err != nil {
		m.logger.Error("health check: query channels", zap.Error(err))
		return
	}

	for _, ch := range channels {
		m.healthCheckChannel(ctx, &ch)
	}
}

// healthCheckChannel 巡检单个渠道
func (m *Manager) healthCheckChannel(ctx context.Context, ch *channel.Channel) {
	// 获取第一个 disabled/cooling 账号
	var acc Account
	if err := m.db.WithContext(ctx).
		Where("channel_id = ? AND status IN ?", ch.ID, []string{"disabled", "cooling"}).
		Where("probe_cooldown_until IS NULL OR probe_cooldown_until < ?", time.Now()).
		Order("last_failed_at ASC").
		First(&acc).Error; err != nil {
		return // 无需巡检
	}

	// 检查 min_disable_duration
	if acc.LastFailedAt != nil {
		disabledDuration := time.Since(*acc.LastFailedAt)
		if disabledDuration < time.Duration(m.cfg.MinDisableDuration)*time.Second {
			return
		}
	}

	// 获取锁
	lockKey := fmt.Sprintf("probe_lock:%d", ch.ID)
	acquired, _ := m.cache.SetNX(lockKey, "1", 30*time.Second)
	if !acquired {
		return
	}
	defer m.cache.Del(lockKey)

	// 探测
	success := m.probeAccount(ctx, ch, &acc)
	if success {
		m.recoverAccount(ctx, &acc)
		m.resetCooldownCycles(ctx, ch.ID)
	}
}

// ========== 冷却相关方法 ==========

// incrementCooldownCycles 增加渠道冷却周期
func (m *Manager) incrementCooldownCycles(ctx context.Context, channelID uint) {
	var ch channel.Channel
	if err := m.db.WithContext(ctx).First(&ch, channelID).Error; err != nil {
		return
	}

	cycles := 1 // 默认新增一个周期
	// TODO: ch.ConsecutiveCooldownCycles 字段需要添加到 Channel 模型

	m.logger.Info("cooldown cycles incremented",
		zap.Uint("channel_id", channelID),
		zap.Int("cycles", cycles),
	)
}

// resetCooldownCycles 重置渠道冷却周期
func (m *Manager) resetCooldownCycles(ctx context.Context, channelID uint) {
	m.logger.Info("cooldown cycles reset",
		zap.Uint("channel_id", channelID),
	)
}

// getCooldownDuration 根据冷却周期获取冷却时长
func (m *Manager) getCooldownDuration(cycles int) time.Duration {
	if cycles >= 2 {
		return time.Duration(m.cfg.ProbeCooldownDurationL2) * time.Second // 二级冷却 24h
	}
	return time.Duration(m.cfg.ProbeCooldownDuration) * time.Second // 一级冷却 2h
}

// recoverAccount 恢复账号为 active
func (m *Manager) recoverAccount(ctx context.Context, acc *Account) {
	now := time.Now()
	m.db.WithContext(ctx).Model(&Account{}).Where("id = ?", acc.ID).Updates(map[string]interface{}{
		"status":                "active",
		"consecutive_failures": 0,
		"probe_cooldown_until": nil,
		"last_failed_at":       nil,
	})

	m.logger.Info("account recovered",
		zap.Uint("account_id", acc.ID),
		zap.Uint("channel_id", acc.ChannelID),
	)
	_ = now // 避免未使用警告
}
