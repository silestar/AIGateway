package log

import (
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/bokelife/aigateway/internal/config"
)

// dailyWriter 实现跨日滚动的日志写入器
type dailyWriter struct {
	mu       sync.Mutex
	cfg      config.LogConfig
	current  *os.File
	curDate  string // 当前日期 YYYY-MM-DD
	curSize  int64  // 当前文件大小
}

func newDailyWriter(cfg config.LogConfig) (*dailyWriter, error) {
	w := &dailyWriter{cfg: cfg}
	if err := w.rotate(time.Now()); err != nil {
		return nil, err
	}
	return w, nil
}

// rotate 切换到指定日期的日志文件
func (w *dailyWriter) rotate(t time.Time) error {
	dateStr := t.Format("2006-01-02")
	dateDir := filepath.Join(w.cfg.Dir, t.Format("2006"), t.Format("01"))
	if err := os.MkdirAll(dateDir, 0755); err != nil {
		return err
	}
	logFile := filepath.Join(dateDir, t.Format("02")+".log")

	// 检查已有文件大小
	var initialSize int64
	if info, err := os.Stat(logFile); err == nil {
		initialSize = info.Size()
	}

	file, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	if w.current != nil {
		w.current.Close()
	}
	w.current = file
	w.curDate = dateStr
	w.curSize = initialSize
	return nil
}

// Write 实现 io.Writer 接口
func (w *dailyWriter) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// 检查是否需要跨日滚动
	today := time.Now().Format("2006-01-02")
	if today != w.curDate {
		if err := w.rotate(time.Now()); err != nil {
			return 0, err
		}
	}

	// 检查文件大小限制（配置为 0 表示不限制）
	if w.cfg.MaxSizeMB > 0 && w.curSize >= int64(w.cfg.MaxSizeMB)*1024*1024 {
		// 文件已超限，切换到带后缀的新文件
		dateDir := filepath.Join(w.cfg.Dir, time.Now().Format("2006"), time.Now().Format("01"))
		ts := time.Now().Format("150405")
		logFile := filepath.Join(dateDir, time.Now().Format("02")+"-"+ts+".log")
		file, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return 0, err
		}
		w.current.Close()
		w.current = file
		w.curSize = 0
	}

	n, err = w.current.Write(p)
	w.curSize += int64(n)
	return n, err
}

// Sync 实现 zapcore.WriteSyncer 接口
func (w *dailyWriter) Sync() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.current != nil {
		return w.current.Sync()
	}
	return nil
}

// Close 关闭当前文件
func (w *dailyWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.current != nil {
		return w.current.Close()
	}
	return nil
}

// NewLogger 创建 zap 日志实例
// 按日滚动：logs/2026/05/04.log
// 支持跨日自动切换和文件大小限制
func NewLogger(cfg config.LogConfig) (*zap.Logger, error) {
	// 确保日志目录存在
	if err := os.MkdirAll(cfg.Dir, 0755); err != nil {
		return nil, err
	}

	// 创建跨日滚动写入器
	writer, err := newDailyWriter(cfg)
	if err != nil {
		return nil, err
	}

	// 日志级别
	level := zapcore.InfoLevel
	switch cfg.Level {
	case "debug":
		level = zapcore.DebugLevel
	case "info":
		level = zapcore.InfoLevel
	case "warn":
		level = zapcore.WarnLevel
	case "error":
		level = zapcore.ErrorLevel
	}

	// 编码器配置
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "ts",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	// 文件输出：JSON 格式
	fileCore := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderConfig),
		zapcore.AddSync(writer),
		level,
	)

	// 控制台输出：彩色可读格式
	consoleEncoder := zapcore.NewConsoleEncoder(zapcore.EncoderConfig{
		TimeKey:        "ts",
		LevelKey:       "level",
		NameKey:        "logger",
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalColorLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
	})
	consoleCore := zapcore.NewCore(
		consoleEncoder,
		zapcore.AddSync(os.Stdout),
		level,
	)

	// 合并输出
	core := zapcore.NewTee(fileCore, consoleCore)
	logger := zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))

	return logger, nil
}

// StartLogCleaner 启动日志自动清理定时任务
func StartLogCleaner(cfg config.LogConfig, logger *zap.Logger) {
	if cfg.MaxAgeDays <= 0 {
		return // 0 表示不清理
	}
	go func() {
		ticker := time.NewTicker(1 * time.Hour) // 每小时检查一次
		defer ticker.Stop()
		for range ticker.C {
			cleanOldLogs(cfg.Dir, cfg.MaxAgeDays, logger)
		}
	}()
	logger.Info("log cleaner started", zap.Int("max_age_days", cfg.MaxAgeDays))
}

// cleanOldLogs 删除超过保留天数的旧日志文件
func cleanOldLogs(logDir string, maxAgeDays int, logger *zap.Logger) {
	cutoff := time.Now().AddDate(0, 0, -maxAgeDays)

	// 遍历年份目录
	yearDirs, err := os.ReadDir(logDir)
	if err != nil {
		return
	}
	for _, yearDir := range yearDirs {
		if !yearDir.IsDir() {
			continue
		}
		yearPath := filepath.Join(logDir, yearDir.Name())

		// 遍历月份目录
		monthDirs, err := os.ReadDir(yearPath)
		if err != nil {
			continue
		}
		for _, monthDir := range monthDirs {
			if !monthDir.IsDir() {
				continue
			}
			monthPath := filepath.Join(yearPath, monthDir.Name())

			// 遍历日志文件
			logFiles, err := os.ReadDir(monthPath)
			if err != nil {
				continue
			}
			for _, logFile := range logFiles {
				if logFile.IsDir() {
					continue
				}
				info, err := logFile.Info()
				if err != nil {
					continue
				}
				if info.ModTime().Before(cutoff) {
					filePath := filepath.Join(monthPath, logFile.Name())
					if err := os.Remove(filePath); err == nil {
						logger.Info("cleaned old log file", zap.String("file", filePath))
					}
				}
			}

			// 清理空月份目录
			entries, _ := os.ReadDir(monthPath)
			if len(entries) == 0 {
				os.Remove(monthPath)
			}
		}

		// 清理空年份目录
		entries, _ := os.ReadDir(yearPath)
		if len(entries) == 0 {
			os.Remove(yearPath)
		}
	}
}
