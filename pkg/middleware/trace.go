package middleware

import (
	"crypto/rand"
	"fmt"

	"github.com/gin-gonic/gin"
)

// TraceID 为每个请求生成 trace_id 并注入 gin.Context
// 格式：xxxxxxxx-xxxxxxxx（16位hex）
func TraceID() gin.HandlerFunc {
	return func(c *gin.Context) {
		traceID := GenerateTraceID("")
		c.Set("trace_id", traceID)
		c.Header("X-Trace-ID", traceID)
		c.Next()
	}
}

// GenerateTraceID 生成 trace_id
// prefix 为空时格式：xxxxxxxx-xxxxxxxx
// prefix 非空时格式：prefix-xxxxxxxx-xxxxxxxx（如 probe-xxx、stats-xxx）
func GenerateTraceID(prefix string) string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	id := fmt.Sprintf("%x-%x", b[:4], b[4:])
	if prefix != "" {
		return prefix + "-" + id
	}
	return id
}
