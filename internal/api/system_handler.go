package api

import (
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/silestar/AIGateway/internal/account"
	"github.com/silestar/AIGateway/internal/config"
	"gorm.io/gorm"
)

// SystemHandler 系统配置 API
type SystemHandler struct {
	cfg        *config.Config
	accountMgr account.AccountManager
	db         *gorm.DB
	version    string
}

func NewSystemHandler(cfg *config.Config, accountMgr account.AccountManager, db *gorm.DB) *SystemHandler {
	return &SystemHandler{
		cfg:        cfg,
		accountMgr: accountMgr,
		db:         db,
		version:    loadVersion("docs/VERSION"),
	}
}

// loadVersion 从 VERSION 文件读取版本号
func loadVersion(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return "0.1.0" // fallback
	}
	v := strings.TrimSpace(string(data))
	if v == "" {
		return "0.1.0"
	}
	return v
}

// RegisterRoutes 注册系统路由
func (h *SystemHandler) RegisterRoutes(rg *gin.RouterGroup) {
	system := rg.Group("/system")
	system.GET("/config", h.GetConfig)
	system.PUT("/config", h.UpdateConfig)
	system.POST("/cache/flush", h.FlushCache)
	system.GET("/monitor/channel-health", h.GetChannelHealth)
}

// Info 系统信息
func (h *SystemHandler) Info(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"version":    h.version,
			"go_version": "1.25.0",
			"port":       h.cfg.Server.Port,
			"db_type":    h.cfg.DB.Type,
		},
	})
}

// GetConfig 获取所有可热加载的配置项
func (h *SystemHandler) GetConfig(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"data": h.cfg.GetHotReloadableConfig(),
	})
}

// UpdateConfig 热更新配置（修改内存 + 写回 config.yaml）
func (h *SystemHandler) UpdateConfig(c *gin.Context) {
	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_request", err.Error()))
		return
	}

	if err := h.cfg.UpdateHotReloadableConfig(updates); err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("update_failed", err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{"message": "config updated successfully"},
	})
}

// FlushCache 清空所有系统缓存（粘性绑定、账号状态缓存、速率计数等）
func (h *SystemHandler) FlushCache(c *gin.Context) {
	h.accountMgr.FlushAllCache()
	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{"message": "cache flushed successfully"},
	})
}

// ChannelHealthItem 渠道健康快照
type ChannelHealthItem struct {
	ID               uint    `json:"id"`
	Name             string  `json:"name"`
	ActiveAccounts   int     `json:"active_accounts"`
	DisabledAccounts int     `json:"disabled_accounts"`
	CoolingAccounts  int     `json:"cooling_accounts"`
	TotalAccounts    int     `json:"total_accounts"`
	ActiveRatio      float64 `json:"active_ratio"`
	CooldownCycles   int     `json:"cooldown_cycles"`
	CooldownLevel    string  `json:"cooldown_level"` // "normal" / "L1" / "L2"
	LastProbedAt     string  `json:"last_probed_at"`
}

// ChannelHealthResponse 渠道健康监控响应
type ChannelHealthResponse struct {
	Channels        []ChannelHealthItem  `json:"channels"`
	CoolingAccounts []CoolingAccountItem `json:"cooling_accounts"`
}

// CoolingAccountItem 冷却中账号
type CoolingAccountItem struct {
	AccountID           uint   `json:"account_id"`
	ChannelID           uint   `json:"channel_id"`
	ChannelName         string `json:"channel_name"`
	CooldownCycles      int    `json:"cooldown_cycles"`
	ConsecutiveFailures int    `json:"consecutive_failures"`
	CooldownUntil       string `json:"cooldown_until"`
}

// GetChannelHealth 各渠道健康快照 + 账号分布
func (h *SystemHandler) GetChannelHealth(c *gin.Context) {
	// 渠道+账号聚合查询
	type row struct {
		ID             uint   `json:"id"`
		Name           string `json:"name"`
		CooldownCycles int    `json:"cooldown_cycles"`
		Status         string `json:"status"`
		Count          int    `json:"count"`
	}
	var rows []row

	// raw SQL for cross-db compatibility
	h.db.Raw(`
		SELECT c.id, c.name, c.consecutive_cooldown_cycles,
			COALESCE(ca.status, 'no_accounts') as status,
			COUNT(ca.id) as count
		FROM channels c
		LEFT JOIN channel_accounts ca ON ca.channel_id = c.id
		WHERE c.status = 'active'
		GROUP BY c.id, ca.status
		ORDER BY c.weight DESC, c.id ASC
	`).Scan(&rows)

	// pivot: channel_id -> {active, disabled, cooling, cycles}
	type accAgg struct {
		Name           string
		Active         int
		Disabled       int
		Cooling        int
		CooldownCycles int
	}
	chMap := make(map[uint]*accAgg)
	for _, r := range rows {
		ch, ok := chMap[r.ID]
		if !ok {
			ch = &accAgg{Name: r.Name, CooldownCycles: r.CooldownCycles}
			chMap[r.ID] = ch
		}
		switch r.Status {
		case "active":
			ch.Active = r.Count
		case "disabled":
			ch.Disabled = r.Count
		case "cooling":
			ch.Cooling = r.Count
		}
	}

	// Get last probe time per channel
	type lastProbe struct {
		ChannelID uint   `json:"channel_id"`
		Timestamp string `json:"timestamp"`
	}
	var probes []lastProbe
	h.db.Raw(`
		SELECT channel_id, MAX(timestamp) as timestamp
		FROM request_logs
		WHERE log_type IN ('probe', 'health_check')
		GROUP BY channel_id
	`).Scan(&probes)
	probeMap := make(map[uint]string)
	for _, p := range probes {
		probeMap[p.ChannelID] = p.Timestamp
	}

	// Build channel health items
	items := make([]ChannelHealthItem, 0, len(chMap))
	for id, ch := range chMap {
		total := ch.Active + ch.Disabled + ch.Cooling
		ratio := 0.0
		if total > 0 {
			ratio = float64(ch.Active) / float64(total)
		}
		level := "normal"
		if ch.CooldownCycles == 1 {
			level = "L1"
		} else if ch.CooldownCycles >= 2 {
			level = "L2"
		}
		items = append(items, ChannelHealthItem{
			ID:               id,
			Name:             ch.Name,
			ActiveAccounts:   ch.Active,
			DisabledAccounts: ch.Disabled,
			CoolingAccounts:  ch.Cooling,
			TotalAccounts:    total,
			ActiveRatio:      ratio,
			CooldownCycles:   ch.CooldownCycles,
			CooldownLevel:    level,
			LastProbedAt:     probeMap[id],
		})
	}

	// Query cooling accounts
	type coolingRow struct {
		AccountID           uint       `json:"account_id"`
		ChannelID           uint       `json:"channel_id"`
		ChannelName         string     `json:"channel_name"`
		CooldownCycles      int        `json:"cooldown_cycles"`
		ConsecutiveFailures int        `json:"consecutive_failures"`
		CooldownUntil       *time.Time `json:"cooldown_until"`
	}
	var coolRows []coolingRow
	h.db.Raw(`
		SELECT ca.id as account_id, ca.channel_id, c.name as channel_name,
			c.consecutive_cooldown_cycles as cooldown_cycles, ca.consecutive_failures, ca.probe_cooldown_until as cooldown_until
		FROM channel_accounts ca
		JOIN channels c ON c.id = ca.channel_id
		WHERE ca.status = 'disabled' AND ca.probe_cooldown_until IS NOT NULL
		ORDER BY ca.probe_cooldown_until ASC
	`).Scan(&coolRows)

	coolingItems := make([]CoolingAccountItem, 0, len(coolRows))
	for _, cr := range coolRows {
		cu := ""
		if cr.CooldownUntil != nil {
			cu = cr.CooldownUntil.Format("2006-01-02 15:04:05")
		}
		coolingItems = append(coolingItems, CoolingAccountItem{
			AccountID:           cr.AccountID,
			ChannelID:           cr.ChannelID,
			ChannelName:         cr.ChannelName,
			CooldownCycles:      cr.CooldownCycles,
			ConsecutiveFailures: cr.ConsecutiveFailures,
			CooldownUntil:       cu,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"data": ChannelHealthResponse{
			Channels:        items,
			CoolingAccounts: coolingItems,
		},
	})
}