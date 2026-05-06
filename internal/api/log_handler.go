package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/bokelife/aigateway/internal/stats"
)

// LogHandler 日志 API
type LogHandler struct {
	statsMgr *stats.Manager
}

// NewLogHandler 创建日志 Handler
func NewLogHandler(statsMgr *stats.Manager) *LogHandler {
	return &LogHandler{statsMgr: statsMgr}
}

// RegisterRoutes 注册日志路由
func (h *LogHandler) RegisterRoutes(rg *gin.RouterGroup) {
	logs := rg.Group("/logs")
	logs.GET("", h.List)
	logs.GET("/:id", h.GetById)
}

// List 请求日志列表
func (h *LogHandler) List(c *gin.Context) {
	filter := stats.LogFilter{
		KeysID: uint(intQuery(c, "keys_id", 0)),
		ChannelID:  uint(intQuery(c, "channel_id", 0)),
		ModelName:  c.Query("model_name"),
		Status:     c.Query("status"),
		Start:      c.Query("start"),
		End:        c.Query("end"),
		Page:       intQuery(c, "page", 1),
		PageSize:   intQuery(c, "page_size", 20),
	}

	logs, total, err := h.statsMgr.QueryRequestLogs(c.Request.Context(), filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("internal_error", err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":      logs,
		"total":     total,
		"page":      filter.Page,
		"page_size": filter.PageSize,
	})
}

// GetById 日志详情
func (h *LogHandler) GetById(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_id", "invalid log id"))
		return
	}

	log, err := h.statsMgr.GetRequestLogByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, errorResponse("not_found", "log not found"))
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": log})
}
