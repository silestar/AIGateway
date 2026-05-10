package api

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/silestar/AIGateway/internal/config"
)

// SystemLogHandler 系统日志 API（读取 zap JSON 日志文件）
type SystemLogHandler struct {
	cfg *config.Config
}

// NewSystemLogHandler 创建系统日志 Handler
func NewSystemLogHandler(cfg *config.Config) *SystemLogHandler {
	return &SystemLogHandler{cfg: cfg}
}

// RegisterRoutes 注册系统日志路由
func (h *SystemLogHandler) RegisterRoutes(rg *gin.RouterGroup) {
	s := rg.Group("/system/logs")
	s.GET("", h.List)
	s.GET("/dates", h.Dates)
	s.GET("/download", h.Download)
}

// logDir 返回日志根目录
func (h *SystemLogHandler) logDir() string {
	if h.cfg.Log.Dir != "" {
		return h.cfg.Log.Dir
	}
	return "logs"
}

// List 读取 zap JSON 日志文件，解析并返回
// GET /api/system/logs?date=2026-05-07&level=info,warn&keyword=xxx&trace_id=xxx&page=1&page_size=100&since=...
func (h *SystemLogHandler) List(c *gin.Context) {
	// 解析必填参数 date
	dateStr := c.Query("date")
	if dateStr == "" {
		c.JSON(http.StatusBadRequest, errorResponse("missing_date", "date 参数必填，格式 YYYY-MM-DD"))
		return
	}
	// 校验日期格式
	parsedDate, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_date", "date 格式错误，需 YYYY-MM-DD"))
		return
	}

	// 构建日志文件路径：logs/年份/月份/日.log
	logFilePath := filepath.Join(h.logDir(), parsedDate.Format("2006"), parsedDate.Format("01"), parsedDate.Format("02")+".log")

	// 打开日志文件
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
	levelFilter := c.Query("level")           // 逗号分隔，如 info,warn
	keyword := c.Query("keyword")             // msg 字段模糊匹配
	traceID := c.Query("trace_id")            // trace_id 精确匹配
	page := intQuery(c, "page", 1)
	pageSize := intQuery(c, "page_size", 100)
	sinceStr := c.Query("since")              // RFC3339 时间戳

	// 限制 page_size 最大 500
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
	// 增大缓冲区以支持超长行（如含 stacktrace）
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		// 解析 JSON 行
		var entry map[string]interface{}
		if err := json.Unmarshal(line, &entry); err != nil {
			// 非 JSON 行跳过
			continue
		}

		// 按 level 筛选
		if len(levelSet) > 0 {
			levelVal, _ := entry["level"].(string)
			if !levelSet[levelVal] {
				continue
			}
		}

		// 按 keyword（msg 字段模糊匹配）筛选
		if keyword != "" {
			msgVal, _ := entry["msg"].(string)
			if !strings.Contains(strings.ToLower(msgVal), strings.ToLower(keyword)) {
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

		// 按 since 时间戳筛选（只返回该时间戳之后的新日志）
		if sinceTime != nil {
			tsVal, _ := entry["ts"].(string)
			if tsVal != "" {
				// zap 输出的 ts 格式为 ISO8601（如 2026-05-07T19:39:09.086+0800）
				logTime, err := time.Parse("2006-01-02T15:04:05.000-0700", tsVal)
				if err == nil && !logTime.After(*sinceTime) {
					continue
				}
			}
		}

		allLogs = append(allLogs, entry)
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
// GET /api/system/logs/dates
func (h *SystemLogHandler) Dates(c *gin.Context) {
	logDir := h.logDir()
	var dates []string
	dates = []string{} // 确保返回空数组而非 null

	// 遍历 logs/年/月/ 目录下的 .log 文件
	err := filepath.WalkDir(logDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // 忽略无法访问的目录
		}
		if d.IsDir() {
			return nil
		}
		// 只处理 .log 文件
		if !strings.HasSuffix(d.Name(), ".log") {
			return nil
		}

		// 从路径中提取日期：logs/2026/05/07.log → 2026-05-07
		rel, err := filepath.Rel(logDir, path)
		if err != nil {
			return nil
		}
		// rel 格式如 2026/05/07.log
		parts := strings.Split(filepath.ToSlash(rel), "/")
		if len(parts) >= 3 {
			year := parts[0]
			month := parts[1]
			dayFile := parts[2]
			// 先去掉 .log 后缀
			dayFile = strings.TrimSuffix(dayFile, ".log")
			// 再去掉可能的额外后缀（如 07-150405）
			day := strings.SplitN(dayFile, "-", 2)[0]
			// 校验格式
			dateStr := fmt.Sprintf("%s-%s-%s", year, month, day)
			if _, err := time.Parse("2006-01-02", dateStr); err == nil {
				dates = append(dates, dateStr)
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

	// 降序排列（最新在前）
	sort.Slice(uniqueDates, func(i, j int) bool {
		return uniqueDates[i] > uniqueDates[j]
	})

	c.JSON(http.StatusOK, gin.H{
		"data": uniqueDates,
	})
}

// Download 返回指定日期的原始 .log 文件流
// GET /api/system/logs/download?date=2026-05-07
func (h *SystemLogHandler) Download(c *gin.Context) {
	dateStr := c.Query("date")
	if dateStr == "" {
		c.JSON(http.StatusBadRequest, errorResponse("missing_date", "date 参数必填，格式 YYYY-MM-DD"))
		return
	}

	// 校验日期格式
	parsedDate, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_date", "date 格式错误，需 YYYY-MM-DD"))
		return
	}

	// 构建日志文件路径
	logFilePath := filepath.Join(h.logDir(), parsedDate.Format("2006"), parsedDate.Format("01"), parsedDate.Format("02")+".log")

	// 检查文件是否存在
	if _, err := os.Stat(logFilePath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, errorResponse("not_found", "日志文件不存在"))
		return
	}

	// 设置下载响应头
	fileName := parsedDate.Format("02") + ".log"
	c.Header("Content-Disposition", "attachment; filename="+strconv.Quote(fileName))
	c.Header("Content-Type", "application/octet-stream")
	c.File(logFilePath)
}
