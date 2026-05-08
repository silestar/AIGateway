package log

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gorm.io/gorm"
)

// DetailEntry 请求详细内容文件结构
type DetailEntry struct {
	TraceID  string        `json:"trace_id"`
	Request  DetailSection `json:"request"`
	Response DetailSection `json:"response"`
}

// DetailSection 请求/响应内容
type DetailSection struct {
	Method     string            `json:"method,omitempty"`
	Path       string            `json:"path,omitempty"`
	Headers    map[string]string `json:"headers,omitempty"`
	Body       interface{}       `json:"body,omitempty"`
	StatusCode int               `json:"status_code,omitempty"`
}

// DetailWriter 详细内容异步写入器
type DetailWriter struct {
	cfg *DetailWriterConfig
	db  *gorm.DB
}

// DetailWriterConfig 配置
type DetailWriterConfig struct {
	Enabled    bool   // 全局开关
	LogDir     string // 日志根目录（如 "logs"）
	MaxAgeDays int    // 保留天数
}

// NewDetailWriter 创建详细内容写入器
func NewDetailWriter(cfg *DetailWriterConfig, db *gorm.DB) *DetailWriter {
	return &DetailWriter{cfg: cfg, db: db}
}

// WriteDetail 异步写入 trace_id.json 文件，完成后标记 has_detail=1
func (w *DetailWriter) WriteDetail(traceID string, timestamp time.Time, reqSection DetailSection, respSection DetailSection) {
	if !w.cfg.Enabled || traceID == "" {
		return
	}

	entry := &DetailEntry{
		TraceID:  traceID,
		Request:  reqSection,
		Response: respSection,
	}

	// goroutine 异步写入，不阻塞主请求
	go func() {
		if err := w.writeFile(traceID, timestamp, entry); err != nil {
			// 写入失败静默忽略，不影响主流程
			return
		}

		// 标记 DB has_detail = 1
		w.db.Model(&struct {
			HasDetail int
		}{}).Table("request_logs").
			Where("trace_id = ?", traceID).
			Update("has_detail", 1)
	}()
}

// writeFile 将详情写入文件
func (w *DetailWriter) writeFile(traceID string, timestamp time.Time, entry *DetailEntry) error {
	dir := filepath.Join(w.cfg.LogDir, "requests",
		fmt.Sprintf("%04d", timestamp.Year()),
		fmt.Sprintf("%02d", timestamp.Month()),
		fmt.Sprintf("%02d", timestamp.Day()),
	)

	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	filePath := filepath.Join(dir, traceID+".json")
	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	return os.WriteFile(filePath, data, 0644)
}

// ReadDetail 读取指定 trace_id 的详细内容
func ReadDetail(logDir, traceID string, timestamp time.Time) (*DetailEntry, error) {
	filePath := filepath.Join(logDir, "requests",
		fmt.Sprintf("%04d", timestamp.Year()),
		fmt.Sprintf("%02d", timestamp.Month()),
		fmt.Sprintf("%02d", timestamp.Day()),
		traceID+".json",
	)

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var entry DetailEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, err
	}
	return &entry, nil
}

// CleanRequestDetails 清理过期的详细内容文件夹
func CleanRequestDetails(logDir string, maxAgeDays int) {
	if maxAgeDays <= 0 {
		return
	}

	requestsDir := filepath.Join(logDir, "requests")
	if _, err := os.Stat(requestsDir); os.IsNotExist(err) {
		return
	}

	cutoff := time.Now().AddDate(0, 0, -maxAgeDays)

	// 遍历年目录
	yearDirs, _ := os.ReadDir(requestsDir)
	for _, yearEntry := range yearDirs {
		if !yearEntry.IsDir() {
			continue
		}
		yearPath := filepath.Join(requestsDir, yearEntry.Name())

		monthDirs, _ := os.ReadDir(yearPath)
		for _, monthEntry := range monthDirs {
			if !monthEntry.IsDir() {
				continue
			}
			monthPath := filepath.Join(yearPath, monthEntry.Name())

			dayDirs, _ := os.ReadDir(monthPath)
			for _, dayEntry := range dayDirs {
				if !dayEntry.IsDir() {
					continue
				}
				dayPath := filepath.Join(monthPath, dayEntry.Name())

				// 解析日期
				dateStr := fmt.Sprintf("%s-%s-%s", yearEntry.Name(), monthEntry.Name(), dayEntry.Name())
				date, err := time.Parse("2006-01-02", dateStr)
				if err != nil {
					continue
				}

				if date.Before(cutoff) {
					os.RemoveAll(dayPath)
				}
			}

			// 清理空月份文件夹
			remaining, _ := os.ReadDir(monthPath)
			if len(remaining) == 0 {
				os.Remove(monthPath)
			}
		}

		// 清理空年份文件夹
		remaining, _ := os.ReadDir(yearPath)
		if len(remaining) == 0 {
			os.Remove(yearPath)
		}
	}
}