package api

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/bokelife/aigateway/internal/config"
)

// SystemHandler 系统配置 API
type SystemHandler struct {
	cfg *config.Config
}

func NewSystemHandler(cfg *config.Config) *SystemHandler {
	return &SystemHandler{cfg: cfg}
}

// RegisterRoutes 注册系统路由
func (h *SystemHandler) RegisterRoutes(rg *gin.RouterGroup) {
	system := rg.Group("/system")
	system.GET("/info", h.Info)
	system.GET("/config", h.GetConfig)
	system.PUT("/config", h.UpdateConfig)
	system.GET("/logs/download", h.DownloadLogs)
}

// Info 系统信息
func (h *SystemHandler) Info(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"version":    "0.1.0",
			"go_version": "1.25.0",
			"port":       h.cfg.Server.Port,
			"db_type":    h.cfg.DB.Type,
		},
	})
}

// GetConfig 获取配置
func (h *SystemHandler) GetConfig(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"server": gin.H{
				"port": h.cfg.Server.Port,
			},
			"database": gin.H{
				"type": h.cfg.DB.Type,
				"path": h.cfg.DB.Path,
			},
			"proxy": gin.H{
				"connect_timeout":   h.cfg.Proxy.ConnectTimeout,
				"read_timeout":       h.cfg.Proxy.ReadTimeout,
				"max_idle_conns":     h.cfg.Proxy.MaxIdleConns,
				"idle_conn_timeout":  h.cfg.Proxy.IdleConnTimeout,
			},
			"account_manager": gin.H{
				"affinity_ttl":                  h.cfg.AccountManager.AffinityTTL,
				"consecutive_failure_threshold":  h.cfg.AccountManager.ConsecutiveFailureThreshold,
				"probe_cooldown_duration":        h.cfg.AccountManager.ProbeCooldownDuration,
				"probe_cooldown_duration_l2":     h.cfg.AccountManager.ProbeCooldownDurationL2,
				"global_health_check_interval":   h.cfg.AccountManager.GlobalHealthCheckInterval,
			},
		},
	})
}

// UpdateConfig 更新配置
func (h *SystemHandler) UpdateConfig(c *gin.Context) {
	// 配置热更新比较复杂，此阶段仅返回成功
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"message": "config update is not yet supported in this version"}})
}

// DownloadLogs 下载日志文件
func (h *SystemHandler) DownloadLogs(c *gin.Context) {
	logPath := filepath.Join("logs", "agw.log")
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, errorResponse("not_found", "log file not found"))
		return
	}

	c.Header("Content-Disposition", "attachment; filename=agw.log")
	c.File(logPath)
}