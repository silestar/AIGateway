package log

import (
	"os"
	"path/filepath"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/bokelife/aigateway/internal/config"
)

// NewLogger 创建 zap 日志实例
// 按日滚动：logs/2026/05/04.log
func NewLogger(cfg config.LogConfig) (*zap.Logger, error) {
	// 确保日志目录存在
	if err := os.MkdirAll(cfg.Dir, 0755); err != nil {
		return nil, err
	}

	// 按日期构建日志文件路径
	now := time.Now()
	dateDir := filepath.Join(cfg.Dir, now.Format("2006"), now.Format("01"))
	if err := os.MkdirAll(dateDir, 0755); err != nil {
		return nil, err
	}
	logFile := filepath.Join(dateDir, now.Format("02")+".log")

	// 打开日志文件（追加模式）
	file, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
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
		zapcore.AddSync(file),
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
