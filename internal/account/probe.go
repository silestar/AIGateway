package account

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/silestar/AIGateway/internal/channel"
	"github.com/silestar/AIGateway/pkg/middleware"
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
		ticker := time.NewTicker(time.Duration(m.cfg.ChannelHealthCheckInterval) * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			m.runGlobalHealthCheck(context.Background())
		}
	}()
	m.logger.Info("global health check started",
		zap.Int("interval_seconds", m.cfg.ChannelHealthCheckInterval),
	)
}

// runProbeCycle 按需探测一轮
func (m *Manager) runProbeCycle(ctx context.Context) {
	traceID := middleware.GenerateTraceID("probe")
	var channels []channel.Channel
	if err := m.db.WithContext(ctx).Where("status = ?", "active").Find(&channels).Error; err != nil {
		m.logger.Error("probe: query channels", zap.Error(err), zap.String("trace_id", traceID))
		return
	}

	for _, ch := range channels {
		m.probeChannel(ctx, &ch, traceID)
	}
}

// probeChannel 探测单个渠道
func (m *Manager) probeChannel(ctx context.Context, ch *channel.Channel, traceID string) {
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

	// 3. 获取 disabled 账号（排除冷却中的）
	var disabledAccounts []Account
	if err := m.db.WithContext(ctx).
		Where("channel_id = ? AND status IN ?", ch.ID, []string{"disabled"}).
		Where("probe_cooldown_until IS NULL OR probe_cooldown_until < ?", time.Now()).
		Where("probe_failures < ?", m.cfg.MaxProbeFailures). // 未达探测失败上限
		Order("last_failed_at ASC").
		Find(&disabledAccounts).Error; err != nil {
		m.logger.Error("probe: query disabled accounts", zap.Error(err), zap.String("trace_id", traceID))
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

		// 探测（复用渠道可用性检查）
		startTime := time.Now()
		plainKey, keyErr := m.GetDecryptedAPIKey(ctx, acc.ID)
		if keyErr != nil {
			elapsedMs := int(time.Since(startTime).Milliseconds())
			m.recordProbeLog(ctx, ch.ID, acc.ID, false, "probe", elapsedMs, 0, keyErr.Error(), 0, 0)
			continue
		}
		testResult, testErr := m.channelSvc.TestAccount(ctx, ch.ID, acc.ID, plainKey)
		elapsedMs := int(time.Since(startTime).Milliseconds())

		if testErr != nil || !testResult.Success {
			statusCode := 0
			errMsg := "probe failed"
			if testResult != nil {
				statusCode = testResult.Status
				if testResult.Error != "" {
					errMsg = testResult.Error
				}
			} else if testErr != nil {
				errMsg = testErr.Error()
			}
			m.recordProbeLog(ctx, ch.ID, acc.ID, false, "probe", elapsedMs, statusCode, errMsg, 0, 0)
			// 探测失败 → 增加探测失败计数
			newProbeFailures := acc.ProbeFailures + 1
			updates := map[string]interface{}{
				"probe_failures": newProbeFailures,
			}
			// 达到探测失败上限 → 设置冷却，停止后续探测
			if newProbeFailures >= m.cfg.MaxProbeFailures {
				cooldownDuration := m.getCooldownDuration(1) // 使用一级冷却时长
				cooldownUntil := time.Now().Add(cooldownDuration)
				updates["probe_cooldown_until"] = cooldownUntil
				m.logger.Warn("account probe failures reached limit, entering cooldown",
					zap.Uint("account_id", acc.ID),
					zap.Uint("channel_id", ch.ID),
					zap.Int("probe_failures", newProbeFailures),
					zap.Time("cooldown_until", cooldownUntil),
					zap.String("trace_id", traceID),
				)
			}
			m.db.WithContext(ctx).Model(&Account{}).Where("id = ?", acc.ID).Updates(updates)
		} else {
			// 探测成功 → 恢复账号
			m.recordProbeLog(ctx, ch.ID, acc.ID, true, "probe", elapsedMs, testResult.Status, "", testResult.PromptTokens, testResult.CompletionTokens)
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
	traceID := middleware.GenerateTraceID("health-check")
	var channels []channel.Channel
	if err := m.db.WithContext(ctx).Find(&channels).Error; err != nil {
		m.logger.Error("health check: query channels", zap.Error(err), zap.String("trace_id", traceID))
		return
	}

	for _, ch := range channels {
		m.healthCheckChannel(ctx, &ch, traceID)
	}
}

// healthCheckChannel 巡检单个渠道
// 1. 尝试恢复 disabled/cooling 账号（原有逻辑）
// 2. 对 active 渠道进行主动健康探测（新增逻辑）
func (m *Manager) healthCheckChannel(ctx context.Context, ch *channel.Channel, traceID string) {
	// ===== 阶段1：恢复 disabled/cooling 账号 =====
	var disabledAcc Account
	hasDisabled := true
	if err := m.db.WithContext(ctx).
		Where("channel_id = ? AND status IN ?", ch.ID, []string{"disabled", "cooling"}).
		Where("probe_cooldown_until IS NULL OR probe_cooldown_until < ?", time.Now()).
		Order("last_failed_at ASC").
		First(&disabledAcc).Error; err != nil {
		hasDisabled = false
	}

	if hasDisabled {
		// 检查 min_disable_duration
		if disabledAcc.LastFailedAt != nil {
			disabledDuration := time.Since(*disabledAcc.LastFailedAt)
			if disabledDuration < time.Duration(m.cfg.MinDisableDuration)*time.Second {
				hasDisabled = false
			}
		}
	}

	if hasDisabled {
		// 获取锁
		lockKey := fmt.Sprintf("probe_lock:%d", ch.ID)
		acquired, _ := m.cache.SetNX(lockKey, "1", 30*time.Second)
		if acquired {
			defer m.cache.Del(lockKey)

			startTime := time.Now()
			plainKey, keyErr := m.GetDecryptedAPIKey(ctx, disabledAcc.ID)
			if keyErr != nil {
				elapsedMs := int(time.Since(startTime).Milliseconds())
				m.recordProbeLog(ctx, ch.ID, disabledAcc.ID, false, "health_check", elapsedMs, 0, keyErr.Error(), 0, 0)
			} else {
				testResult, testErr := m.channelSvc.TestAccount(ctx, ch.ID, disabledAcc.ID, plainKey)
				elapsedMs := int(time.Since(startTime).Milliseconds())

				if testErr != nil || !testResult.Success {
					statusCode := 0
					errMsg := "health check failed"
					if testResult != nil {
						statusCode = testResult.Status
						if testResult.Error != "" {
							errMsg = testResult.Error
						}
					} else if testErr != nil {
						errMsg = testErr.Error()
					}
					m.recordProbeLog(ctx, ch.ID, disabledAcc.ID, false, "health_check", elapsedMs, statusCode, errMsg, 0, 0)
				} else {
					m.recordProbeLog(ctx, ch.ID, disabledAcc.ID, true, "health_check", elapsedMs, testResult.Status, "", testResult.PromptTokens, testResult.CompletionTokens)
					m.recoverAccount(ctx, &disabledAcc)
					m.resetCooldownCycles(ctx, ch.ID)
				}
			}
		}
	}

	// ===== 阶段2：对 active 渠道做主动健康探测 =====
	// 只对启用中的渠道，取优先级最高的 active 账号做一次轻量测试
	if !m.cfg.ChannelDisableOnFailure {
		return
	}

	var activeAcc Account
	if err := m.db.WithContext(ctx).
		Where("channel_id = ? AND status = ?", ch.ID, "active").
		Order("priority DESC, id ASC").
		First(&activeAcc).Error; err != nil {
		return // 无 active 账号
	}

	// 获取锁（使用不同 key 避免与阶段1冲突）
	lockKey := fmt.Sprintf("active_probe_lock:%d", ch.ID)
	acquired, _ := m.cache.SetNX(lockKey, "1", 30*time.Second)
	if !acquired {
		return
	}
	defer m.cache.Del(lockKey)

	startTime := time.Now()
	plainKey, keyErr := m.GetDecryptedAPIKey(ctx, activeAcc.ID)
	if keyErr != nil {
		return
	}

	testResult, testErr := m.channelSvc.TestAccount(ctx, ch.ID, activeAcc.ID, plainKey)
	elapsedMs := int(time.Since(startTime).Milliseconds())

	if testErr != nil || !testResult.Success {
		statusCode := 0
		errMsg := "active health check failed"
		if testResult != nil {
			statusCode = testResult.Status
			if testResult.Error != "" {
				errMsg = testResult.Error
			}
		} else if testErr != nil {
			errMsg = testErr.Error()
		}
		m.recordProbeLog(ctx, ch.ID, activeAcc.ID, false, "active_health_check", elapsedMs, statusCode, errMsg, 0, 0)
		// 通过 ReportResult 累积失败次数，由已有阈值机制决定是否禁用
		m.ReportResult(ctx, activeAcc.ID, false, statusCode)
	} else {
		m.recordProbeLog(ctx, ch.ID, activeAcc.ID, true, "active_health_check", elapsedMs, testResult.Status, "", testResult.PromptTokens, testResult.CompletionTokens)
		// 成功：重置连续失败计数
		m.ReportResult(ctx, activeAcc.ID, true, testResult.Status)

		// 如果 channel_enable_on_success 且有 disabled 账号，尝试恢复
		if m.cfg.ChannelEnableOnSuccess {
			var disabledAccounts []Account
			m.db.WithContext(ctx).
				Where("channel_id = ? AND status IN ?", ch.ID, []string{"disabled", "cooling"}).
				Where("probe_cooldown_until IS NULL OR probe_cooldown_until < ?", time.Now()).
				Find(&disabledAccounts)
			for _, da := range disabledAccounts {
				if da.LastFailedAt != nil {
					disabledDuration := time.Since(*da.LastFailedAt)
					if disabledDuration >= time.Duration(m.cfg.MinDisableDuration)*time.Second {
						m.recoverAccount(ctx, &da)
					}
				}
			}
			m.resetCooldownCycles(ctx, ch.ID)
		}
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
		"probe_failures":       0,
		"probe_cooldown_until": nil,
		"last_failed_at":       nil,
	})

	m.logger.Info("account recovered",
		zap.Uint("account_id", acc.ID),
		zap.Uint("channel_id", acc.ChannelID),
	)
	_ = now // 避免未使用警告
}

// recordProbeLog 记录探测日志（通过回调通知外部写入）
func (m *Manager) recordProbeLog(ctx context.Context, channelID, accountID uint, success bool, logType string, elapsedMs int, statusCode int, errMsg string, promptTokens int, completionTokens int) {
	if m.onProbeDone != nil {
		m.onProbeDone(channelID, accountID, success, logType, elapsedMs, statusCode, errMsg, promptTokens, completionTokens)
	}
}
