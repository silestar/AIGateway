package api

import (
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/silestar/AIGateway/internal/config"
)

// SystemHandler 系统配置 API
type SystemHandler struct {
	cfg     *config.Config
	version string
}

func NewSystemHandler(cfg *config.Config) *SystemHandler {
	return &SystemHandler{
		cfg:     cfg,
		version: loadVersion("docs/VERSION"),
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