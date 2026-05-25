package api

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/silestar/AIGateway/internal/config"
	"github.com/silestar/AIGateway/internal/plugin"
)

// SystemLogHandler 系统日志 API（读取 zap JSON 日志文件 + 插件纯文本日志）
type SystemLogHandler struct {
	cfg       *config.Config
	pluginMgr plugin.PluginManager
}

// NewSystemLogHandler 创建系统日志 Handler
func NewSystemLogHandler(cfg *config.Config, pluginMgr plugin.PluginManager) *SystemLogHandler {
	return &SystemLogHandler{cfg: cfg, pluginMgr: pluginMgr}
}

// RegisterRoutes 注册系统日志路由
func (h *SystemLogHandler) RegisterRoutes(rg *gin.RouterGroup) {
	s := rg.Group("/system/logs")
	s.GET("", h.List)
	s.GET("/dates", h.Dates)
	s.GET("/download", h.Download)
	s.GET("/plugins", h.ListPlugins) // 插件日志源列表
}

// logDir 返回日志根目录
func (h *SystemLogHandler) logDir() string {
	if h.cfg.Log.Dir != "" {
		return h.cfg.Log.Dir
	}
	return "logs"
}

// ListPlugins 返回已安装插件的日志源列表
// GET /api/system/logs/plugins
func (h *SystemLogHandler) ListPlugins(c *gin.Context) {
	plugins, err := h.pluginMgr.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("list_plugins_failed", err.Error()))
		return
	}

	var result []map[string]interface{}
	result = []map[string]interface{}{} // 确保空数组而非 null

	for _, p := range plugins {
		pluginLogDir := filepath.Join(h.logDir(), "plugins", p.Name)
		hasLogs := false
		if entries, err := os.ReadDir(pluginLogDir); err == nil && len(entries) > 0 {
			hasLogs = true
		}
		result = append(result, map[string]interface{}{
			"name":      p.Name,
			"has_logs":  hasLogs,
			"log_dir":   filepath.Join("logs/plugins", p.Name),
		})
	}

	c.JSON(http.StatusOK, gin.H{"data": result})
}

// List 读取日志文件，解析并返回
// GET /api/system/logs?date=2026-05-07&level=info,warn&keyword=xxx&trace_id=xxx&page=1&page_size=100&since=...&source=system|agp-proxy
func (h *SystemLogHandler) List(c *gin.Context) {
	dateStr := c.Query("date")
	if dateStr == "" {
		c.JSON(http.StatusBadRequest, errorResponse("missing_date", "date 参数必填，格式 YYYY-MM-DD"))
		return
	}
	parsedDate, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_date", "date 格式错误，需 YYYY-MM-DD"))
		return
	}

	// source 参数："" / "system" / 插件名
	source := c.Query("source")

	var logFilePath string
	var isPluginLog bool
	if source != "" && source != "system" {
		// 插件日志：logs/plugins/<source>/YYYY-MM-DD.log
		logFilePath = filepath.Join(h.logDir(), "plugins", source, parsedDate.Format("2006-01-02")+".log")
		isPluginLog = true
	} else {
		// 系统日志：logs/年/月/日.log
		logFilePath = filepath.Join(h.logDir(), parsedDate.Format("2006"), parsedDate.Format("01"), parsedDate.Format("02")+".log")
		isPluginLog = false
	}

	file, err := os.Open(logFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			c.JSON(http.StatusOK, gin.H{
				"data":      []interface{}{},
				"total":     0,
				"page":      1,
				"page_size": 100,
			})
			return
		}
		c.JSON(http.StatusInternalServerError, errorResponse("open_failed", "无法打开日志文件: "+err.Error()))
		return
	}
	defer file.Close()

	// 解析查询参数
	levelFilter := c.Query("level")
	keyword := c.Query("keyword")
	traceID := c.Query("trace_id")
method := c.Query("method")
	page := intQuery(c, "page", 1)
	pageSize := intQuery(c, "page_size", 100)
	sinceStr := c.Query("since")

	if pageSize > 500 {
		pageSize = 500
	}
	if pageSize <= 0 {
		pageSize = 100
	}
	if page <= 0 {
		page = 1
	}

	// 构建 level 筛选集合
	levelSet := make(map[string]bool)
	if levelFilter != "" {
		for _, l := range strings.Split(levelFilter, ",") {
			trimmed := strings.TrimSpace(l)
			if trimmed != "" {
				levelSet[trimmed] = true
			}
		}
	}

	// 解析 since 时间戳
	var sinceTime *time.Time
	if sinceStr != "" {
		t, err := time.Parse(time.RFC3339, sinceStr)
		if err == nil {
			sinceTime = &t
		}
	}

	// 逐行读取并筛选
	var allLogs []map[string]interface{}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if len(line) == 0 {
			continue
		}

		var entry map[string]interface{}

		if isPluginLog {
			// 插件日志：纯文本格式 YYYY/MM/DD HH:MM:SS [LEVEL] message
			entry = parsePluginLogLine(line, source)
		} else {
			// 系统日志：zap JSON 格式
			if err := json.Unmarshal([]byte(line), &entry); err != nil {
				continue
			}
		}

		if entry == nil {
			continue
		}

		// 按 level 筛选
		if len(levelSet) > 0 {
			levelVal, _ := entry["level"].(string)
			if !levelSet[levelVal] {
				continue
			}
		}

		// 按 keyword（msg + path 字段模糊匹配）筛选
		if keyword != "" {
			msgVal, _ := entry["msg"].(string)
			pathVal, _ := entry["path"].(string)
			kw := strings.ToLower(keyword)
			if !strings.Contains(strings.ToLower(msgVal), kw) && !strings.Contains(strings.ToLower(pathVal), kw) {
				continue
			}
		}

		// 按 method 筛选
		if method != "" {
			methodVal, _ := entry["method"].(string)
			if !strings.EqualFold(methodVal, method) {
				continue
			}
		}

		// 按 trace_id 精确匹配筛选
		if traceID != "" {
			tidVal, _ := entry["trace_id"].(string)
			if tidVal != traceID {
				continue
			}
		}

		// 按 since 时间戳筛选
		if sinceTime != nil {
			tsVal, _ := entry["ts"].(string)
			if tsVal != "" {
				logTime, err := time.Parse("2006-01-02T15:04:05.000-0700", tsVal)
				if err == nil && !logTime.After(*sinceTime) {
					continue
				}
			}
		}

		allLogs = append(allLogs, entry)
		// 添加可读字段（模块中文名 + 消息可读描述）
		enrichLogEntry(entry)
	}

	if err := scanner.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("read_failed", "读取日志文件失败: "+err.Error()))
		return
	}

	// 默认按时间戳倒序排列（最新在前）
	sort.Slice(allLogs, func(i, j int) bool {
		tsI, _ := allLogs[i]["ts"].(string)
		tsJ, _ := allLogs[j]["ts"].(string)
		return tsI > tsJ
	})

	// 分页
	total := len(allLogs)
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}

	var pageData []map[string]interface{}
	if start < end {
		pageData = allLogs[start:end]
	} else {
		pageData = []map[string]interface{}{}
	}

	c.JSON(http.StatusOK, gin.H{
		"data":      pageData,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// Dates 扫描日志目录，返回所有有 .log 文件的日期列表
// GET /api/system/logs/dates?source=system|agp-proxy
func (h *SystemLogHandler) Dates(c *gin.Context) {
	source := c.Query("source")
	var scanDir string
	if source != "" && source != "system" {
		// 插件日志目录
		scanDir = filepath.Join(h.logDir(), "plugins", source)
	} else {
		scanDir = h.logDir()
	}

	var dates []string
	dates = []string{}

	err := filepath.WalkDir(scanDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".log") {
			return nil
		}

		// 从文件名提取日期
		// 插件日志文件名：YYYY-MM-DD.log
		// 系统日志路径：年/月/日.log
		rel, err := filepath.Rel(scanDir, path)
		if err != nil {
			return nil
		}

		if source != "" && source != "system" {
			// 插件日志：文件名本身就是日期
			dateStr := strings.TrimSuffix(d.Name(), ".log")
			if _, err := time.Parse("2006-01-02", dateStr); err == nil {
				dates = append(dates, dateStr)
			}
		} else {
			// 系统日志：rel 格式如 2026/05/07.log 或 plugins/xxx/2026-05-21.log
			relSlash := filepath.ToSlash(rel)
			// 排除 plugins/ 子目录
			if strings.HasPrefix(relSlash, "plugins/") {
				return nil
			}
			parts := strings.Split(relSlash, "/")
			if len(parts) >= 3 {
				year := parts[0]
				month := parts[1]
				dayFile := parts[2]
				dayFile = strings.TrimSuffix(dayFile, ".log")
				day := strings.SplitN(dayFile, "-", 2)[0]
				dateStr := fmt.Sprintf("%s-%s-%s", year, month, day)
				if _, err := time.Parse("2006-01-02", dateStr); err == nil {
					dates = append(dates, dateStr)
				}
			}
		}
		return nil
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("scan_failed", "扫描日志目录失败: "+err.Error()))
		return
	}

	// 去重
	seen := make(map[string]bool)
	var uniqueDates []string
	for _, d := range dates {
		if !seen[d] {
			seen[d] = true
			uniqueDates = append(uniqueDates, d)
		}
	}

	sort.Slice(uniqueDates, func(i, j int) bool {
		return uniqueDates[i] > uniqueDates[j]
	})

	c.JSON(http.StatusOK, gin.H{"data": uniqueDates})
}

// Download 返回指定日期的原始 .log 文件流
// GET /api/system/logs/download?date=2026-05-07&source=system|agp-proxy
func (h *SystemLogHandler) Download(c *gin.Context) {
	dateStr := c.Query("date")
	if dateStr == "" {
		c.JSON(http.StatusBadRequest, errorResponse("missing_date", "date 参数必填，格式 YYYY-MM-DD"))
		return
	}

	parsedDate, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_date", "date 格式错误，需 YYYY-MM-DD"))
		return
	}

	source := c.Query("source")

	var logFilePath string
	if source != "" && source != "system" {
		logFilePath = filepath.Join(h.logDir(), "plugins", source, parsedDate.Format("2006-01-02")+".log")
	} else {
		logFilePath = filepath.Join(h.logDir(), parsedDate.Format("2006"), parsedDate.Format("01"), parsedDate.Format("02")+".log")
	}

	if _, err := os.Stat(logFilePath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, errorResponse("not_found", "日志文件不存在"))
		return
	}

	fileName := parsedDate.Format("02") + ".log"
	c.Header("Content-Disposition", "attachment; filename="+strconv.Quote(fileName))
	c.Header("Content-Type", "application/octet-stream")
	c.File(logFilePath)
}

// ============ 日志可读性转换 ============

// callerToModule 将 zap caller 转换为可读的中文模块名
func callerToModule(caller string) string {
	switch {
	case strings.Contains(caller, "storage/"):
		return "存储层"
	case strings.Contains(caller, "proxy/"):
		return "代理引擎"
	case strings.Contains(caller, "internal/auth/"), strings.Contains(caller, "api/auth/"):
		return "认证模块"
	case strings.Contains(caller, "internal/api/"):
		return "API 层"
	case strings.Contains(caller, "internal/stats/"):
		return "统计模块"
	case strings.Contains(caller, "internal/plugin/"):
		return "插件系统"
	case strings.Contains(caller, "internal/channel/"):
		return "渠道管理"
	case strings.Contains(caller, "internal/keys/"):
		return "密钥管理"
	case strings.Contains(caller, "internal/account/"):
		return "账号池"
	case strings.Contains(caller, "internal/adapter/"):
		return "适配器"
	case strings.Contains(caller, "internal/middleware/"):
		return "中间件"
	case strings.Contains(caller, "internal/config/"):
		return "配置模块"
	case strings.Contains(caller, "cmd/"):
		return "系统启动"
	case strings.Contains(caller, "pkg/"):
		return "工具库"
	default:
		return "系统"
	}
}

// msgToReadable 将常见的英文消息转为中文可读描述
func msgToReadable(msg string) string {
	// 精确匹配
	switch msg {
	case "request":
		return "请求处理"
	case "context canceled":
		return "客户端连接中断（请求方主动断开）"
	case "connection reset by peer":
		return "上游服务器主动关闭连接"
	case "i/o timeout":
		return "请求超时（上游 API 无响应）"
	case "no route found":
		return "路由匹配失败"
	case "unauthorized":
		return "认证失败"
	case "forbidden":
		return "权限不足"
	case "not found":
		return "资源不存在"
	case "internal server error":
		return "内部服务器错误"
	case "bad gateway":
		return "上游网关错误"
	case "service unavailable":
		return "服务不可用"
	case "gateway timeout":
		return "网关超时"
	case "too many requests":
		return "请求频率过高（被限流）"
	}

	// 前缀匹配
	switch {
	case strings.HasPrefix(msg, "request started"):
		return "请求开始处理"
	case strings.HasPrefix(msg, "request completed"):
		return "请求处理完成"
	case strings.HasPrefix(msg, "plugin started"):
		return "插件启动"
	case strings.HasPrefix(msg, "plugin stopped"):
		return "插件停止"
	case strings.HasPrefix(msg, "aggregation completed"):
		return "日聚合完成"
	case strings.HasPrefix(msg, "channel test"):
		return "渠道可用性检测"
	case strings.HasPrefix(msg, "model test"):
		return "渠道模型测试"
	case strings.HasPrefix(msg, "account probe"):
		return "账号探测"
	case strings.HasPrefix(msg, "proxy dial"):
		return "代理连接建立"
	case strings.HasPrefix(msg, "rate limit"):
		return "触发速率限制"
	case strings.HasPrefix(msg, "config"):
		return "配置变更"
	case strings.HasPrefix(msg, "migration"):
		return "数据库迁移"
	case strings.HasPrefix(msg, "health check"):
		return "健康检查"
	case strings.HasPrefix(msg, "cleanup"):
		return "资源清理"
	}

	return ""
}

// enrichLogEntry 为日志条目添加可读字段（模块中文名 + 消息可读描述）
func enrichLogEntry(entry map[string]interface{}) {
	// 添加模块可读名
	if caller, ok := entry["caller"].(string); ok && caller != "" {
		entry["module"] = callerToModule(caller)
	}
	// 添加消息可读描述
	if msg, ok := entry["msg"].(string); ok && msg != "" {
		if readable := msgToReadable(msg); readable != "" {
			entry["message"] = readable
		}
	}
}

// 插件日志行正则：YYYY/MM/DD HH:MM:SS [LEVEL] message
var pluginLogRe = regexp.MustCompile(`^(\d{4}/\d{2}/\d{2}\s+\d{2}:\d{2}:\d{2})\s+\[(\w+)\]\s+(.*)$`)

// parsePluginLogLine 解析插件纯文本日志行，转换为与系统日志统一的 map 格式
func parsePluginLogLine(line string, pluginName string) map[string]interface{} {
	matches := pluginLogRe.FindStringSubmatch(line)
	if matches == nil {
		// 无法解析的行：fallback 为 INFO 级别，整行作为消息
		return map[string]interface{}{
			"ts":     "",
			"level":  "info",
			"msg":    line,
			"source": pluginName,
		}
	}

	// 将 YYYY/MM/DD HH:MM:SS 转换为 zap 兼容的 ts 格式（ISO8601）
	rawTime := matches[1]
	parsedTime, err := time.Parse("2006/01/02 15:04:05", rawTime)
	var tsStr string
	if err == nil {
		tsStr = parsedTime.Format("2006-01-02T15:04:05.000+0800")
	} else {
		tsStr = rawTime
	}

	level := strings.ToLower(matches[2])
	msg := matches[3]

	return map[string]interface{}{
		"ts":     tsStr,
		"level":  level,
		"msg":    msg,
		"source": pluginName,
	}
}