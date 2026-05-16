package adapter

import (
	"context"
	"io"
	"net/http"
)

// ChannelAdapter 渠道适配器接口
// 负责入站 OpenAI 格式 ↔ 出站上游格式的转换
type ChannelAdapter interface {
	// Type 返回适配器类型标识
	Type() string

	// ConvertRequest 转换请求：OpenAI Chat → 上游格式
	ConvertRequest(ctx context.Context, originalReq *http.Request, targetModel string) (*http.Request, error)

	// ConvertResponse 转换响应：上游格式 → OpenAI Chat
	ConvertResponse(ctx context.Context, upstreamResp *http.Response) (*http.Response, error)

	// ConvertStreamChunk 转换流式 chunk：上游 SSE → OpenAI Chat SSE
	// 返回 nil 表示此 chunk 应被跳过
	ConvertStreamChunk(ctx context.Context, chunk []byte) ([]byte, error)

	// FetchModels 获取上游可用模型列表
	FetchModels(ctx context.Context, baseURL, apiKey string) ([]ModelInfo, error)
}

// TestEndpointInfo 测试端点信息
type TestEndpointInfo struct {
	ID     string `json:"id"`      // 唯一标识，如 "auto", "openai-chat", "anthropic-messages"
	Label  string `json:"label"`  // 展示名称，如 "自动检测（默认）"
	Path   string `json:"path"`   // 端点路径模板，如 "/v1/chat/completions"，含 {model} 占位符
	IsAuto bool   `json:"is_auto"` // 是否为自动检测
}

// TestEndpointProvider 测试端点提供者接口（可选扩展）
// 适配器可实现此接口以注册该渠道类型支持的测试端点
type TestEndpointProvider interface {
	// TestEndpoints 返回该渠道类型支持的所有测试端点
	TestEndpoints() []TestEndpointInfo
}

// ModelInfo 模型信息
type ModelInfo struct {
	ID      string `json:"id"`
	OwnedBy string `json:"owned_by"`
}

// StreamReader 流式读取器接口（供 proxy 使用）
type StreamReader interface {
	ReadChunk() ([]byte, error)
	Close() error
}

// StreamWriter 流式写入器接口
type StreamWriter interface {
	WriteChunk(data []byte) error
	WriteDone() error
	Flush() error
	Close() error
}

// SSEChunk SSE 数据块
type SSEChunk struct {
	Event string
	Data  string
	ID    string
	Retry int
}

// ParseSSE 从 reader 解析 SSE 事件
func ParseSSE(r io.Reader) <-chan SSEChunk {
	ch := make(chan SSEChunk, 100)
	go func() {
		defer close(ch)
		// SSE 解析逻辑将在 proxy/stream.go 中实现
	}()
	return ch
}
