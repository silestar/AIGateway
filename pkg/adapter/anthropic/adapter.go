package anthropic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/silestar/AIGateway/pkg/adapter"
)

// Adapter Anthropic Messages API 适配器
// 负责将 OpenAI Chat Completions 格式 ↔ Anthropic Messages 格式互相转换
type Adapter struct{}

func (a *Adapter) Type() string {
	return "anthropic"
}

// ========== 请求转换：OpenAI Chat → Anthropic Messages ==========

// ConvertRequest 将 OpenAI Chat Completions 请求转换为 Anthropic Messages 请求
func (a *Adapter) ConvertRequest(ctx context.Context, originalReq *http.Request, targetModel string) (*http.Request, error) {
	// 1. 读取原始请求体
	body, err := io.ReadAll(originalReq.Body)
	if err != nil {
		return nil, fmt.Errorf("read request body: %w", err)
	}
	originalReq.Body.Close()

	// 2. 解析 OpenAI 格式
	var openaiReq openAIChatRequest
	if err := json.Unmarshal(body, &openaiReq); err != nil {
		return nil, fmt.Errorf("parse openai request: %w", err)
	}

	// 3. 转换为 Anthropic 格式
	anthropicReq := convertToAnthropicRequest(&openaiReq, targetModel)

	// 4. 序列化
	anthropicBody, err := json.Marshal(anthropicReq)
	if err != nil {
		return nil, fmt.Errorf("marshal anthropic request: %w", err)
	}

	// 5. 构建新请求（路径替换为 /v1/messages）
	upstreamURL := strings.Replace(originalReq.URL.Path, "/v1/chat/completions", "/v1/messages", 1)
	if originalReq.URL.RawQuery != "" {
		upstreamURL += "?" + originalReq.URL.RawQuery
	}

	newReq, err := http.NewRequestWithContext(ctx, originalReq.Method, upstreamURL, bytes.NewReader(anthropicBody))
	if err != nil {
		return nil, fmt.Errorf("create anthropic request: %w", err)
	}

	// 复制 headers，替换 Content-Type
	for k, vv := range originalReq.Header {
		if k == "Host" || k == "Authorization" {
			continue
		}
		for _, v := range vv {
			newReq.Header.Add(k, v)
		}
	}
	newReq.Header.Set("Content-Type", "application/json")
	// Anthropic 需要 anthropic-version header
	newReq.Header.Set("anthropic-version", "2023-06-01")

	return newReq, nil
}

// ConvertResponse 将 Anthropic Messages 响应转换为 OpenAI Chat Completions 响应
func (a *Adapter) ConvertResponse(ctx context.Context, upstreamResp *http.Response) (*http.Response, error) {
	body, err := io.ReadAll(upstreamResp.Body)
	if err != nil {
		return nil, fmt.Errorf("read anthropic response: %w", err)
	}
	upstreamResp.Body.Close()

	var anthropicResp anthropicMessagesResponse
	if err := json.Unmarshal(body, &anthropicResp); err != nil {
		return nil, fmt.Errorf("parse anthropic response: %w", err)
	}

	openaiResp := convertToOpenAIResponse(&anthropicResp)
	respBody, err := json.Marshal(openaiResp)
	if err != nil {
		return nil, fmt.Errorf("marshal openai response: %w", err)
	}

	upstreamResp.Body = io.NopCloser(bytes.NewReader(respBody))
	upstreamResp.ContentLength = int64(len(respBody))
	return upstreamResp, nil
}

// ConvertStreamChunk 将 Anthropic SSE chunk 转换为 OpenAI SSE chunk
func (a *Adapter) ConvertStreamChunk(ctx context.Context, chunk []byte) ([]byte, error) {
	// Anthropic 事件格式：
	// event: content_block_delta
	// data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"..."}}
	// event: message_stop
	// data: {"type":"message_stop"}

	line := strings.TrimSpace(string(chunk))
	if !strings.HasPrefix(line, "data: ") {
		return nil, nil // 跳过非 data 行
	}
	data := strings.TrimPrefix(line, "data: ")
	if data == "[DONE]" {
		return []byte("data: [DONE]\n\n"), nil
	}

	var event map[string]json.RawMessage
	if err := json.Unmarshal([]byte(data), &event); err != nil {
		return chunk, nil // 无法解析时透传
	}

	var eventType string
	if t, ok := event["type"]; ok {
		json.Unmarshal(t, &eventType)
	}

	switch eventType {
	case "content_block_delta":
		var delta struct {
			Type  string `json:"type"`
			Index int    `json:"index"`
			Delta struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"delta"`
		}
		if err := json.Unmarshal([]byte(data), &delta); err != nil {
			return nil, nil
		}
		openaiChunk := openAIStreamChunk{
			ID:      "chatcmpl-anthropic",
			Object:  "chat.completion.chunk",
			Created: 0,
			Model:   "claude",
			Choices: []struct {
				Index        int    `json:"index"`
				Delta        *struct {
					Content string `json:"content,omitempty"`
					Role    string `json:"role,omitempty"`
				} `json:"delta"`
				FinishReason *string `json:"finish_reason"`
			}{
				{
					Index: delta.Index,
					Delta: &struct {
						Content string `json:"content,omitempty"`
						Role    string `json:"role,omitempty"`
					}{Content: delta.Delta.Text},
				},
			},
		}
		b, _ := json.Marshal(openaiChunk)
		return []byte("data: " + string(b) + "\n\n"), nil

	case "message_start":
		// 提取 model 信息
		var msgStart struct {
			Message struct {
				ID      string `json:"id"`
				Model   string `json:"model"`
				Role    string `json:"role"`
				Content []struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"content"`
			} `json:"message"`
		}
		if err := json.Unmarshal([]byte(data), &msgStart); err != nil {
			return nil, nil
		}
		openaiChunk := openAIStreamChunk{
			ID:      msgStart.Message.ID,
			Object:  "chat.completion.chunk",
			Created: 0,
			Model:   msgStart.Message.Model,
			Choices: []struct {
				Index        int    `json:"index"`
				Delta        *struct {
					Content string `json:"content,omitempty"`
					Role    string `json:"role,omitempty"`
				} `json:"delta"`
				FinishReason *string `json:"finish_reason"`
			}{
				{
					Index: 0,
					Delta: &struct {
						Content string `json:"content,omitempty"`
						Role    string `json:"role,omitempty"`
					}{Role: "assistant"},
				},
			},
		}
		b, _ := json.Marshal(openaiChunk)
		return []byte("data: " + string(b) + "\n\n"), nil

	case "message_stop":
		openaiChunk := openAIStreamChunk{
			ID:      "chatcmpl-anthropic",
			Object:  "chat.completion.chunk",
			Choices: []struct {
				Index        int    `json:"index"`
				Delta        *struct {
					Content string `json:"content,omitempty"`
					Role    string `json:"role,omitempty"`
				} `json:"delta"`
				FinishReason *string `json:"finish_reason"`
			}{
				{
					Index:        0,
					Delta:        nil,
					FinishReason: strPtr("stop"),
				},
			},
		}
		b, _ := json.Marshal(openaiChunk)
		return []byte("data: " + string(b) + "\n\ndata: [DONE]\n\n"), nil

	default:
		return nil, nil // 跳过其他事件类型
	}
}

// FetchModels Anthropic 不提供公开模型列表 API，返回预设列表
func (a *Adapter) FetchModels(ctx context.Context, baseURL, apiKey string) ([]adapter.ModelInfo, error) {
	// Anthropic 没有类似 /v1/models 的接口，返回已知模型列表
	models := []adapter.ModelInfo{
		{ID: "claude-3-5-sonnet-20241022", OwnedBy: "anthropic"},
		{ID: "claude-3-5-haiku-20241022", OwnedBy: "anthropic"},
		{ID: "claude-3-opus-20240229", OwnedBy: "anthropic"},
		{ID: "claude-3-sonnet-20240229", OwnedBy: "anthropic"},
		{ID: "claude-3-haiku-20240307", OwnedBy: "anthropic"},
	}
	return models, nil
}

// ========== 内部类型定义 ==========

type openAIChatRequest struct {
	Model       string `json:"model"`
	Messages    []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"messages"`
	MaxTokens  int     `json:"max_tokens,omitempty"`
	Temperature float64 `json:"temperature,omitempty"`
	Stream     bool    `json:"stream,omitempty"`
}

type anthropicMessagesRequest struct {
	Model     string `json:"model"`
	Messages  []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"messages"`
	MaxTokens int     `json:"max_tokens"`
	System    string  `json:"system,omitempty"`
	Temperature *float64 `json:"temperature,omitempty"`
	Stream    bool    `json:"stream,omitempty"`
}

type anthropicMessagesResponse struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Role    string `json:"role"`
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Model        string  `json:"model"`
	StopReason   string  `json:"stop_reason"`
	Usage        struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

type openAIChatResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index        int    `json:"index"`
		Message      *struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

type openAIStreamChunk struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index        int    `json:"index"`
		Delta        *struct {
			Content string `json:"content,omitempty"`
			Role    string `json:"role,omitempty"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
}

// ========== 转换函数 ==========

func convertToAnthropicRequest(openaiReq *openAIChatRequest, targetModel string) *anthropicMessagesRequest {
	model := openaiReq.Model
	if targetModel != "" {
		model = targetModel
	}

	maxTokens := openaiReq.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 4096
	}

	// 分离 system 消息
	var system string
	messages := make([]struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}, 0, len(openaiReq.Messages))

	for _, msg := range openaiReq.Messages {
		if msg.Role == "system" {
			system += msg.Content + "\n"
		} else {
			messages = append(messages, msg)
		}
	}

	req := &anthropicMessagesRequest{
		Model:       model,
		Messages:    messages,
		MaxTokens:   maxTokens,
		Stream:      openaiReq.Stream,
	}
	if system != "" {
		req.System = strings.TrimSpace(system)
	}
	if openaiReq.Temperature > 0 {
		req.Temperature = &openaiReq.Temperature
	}

	return req
}

func convertToOpenAIResponse(anthropicResp *anthropicMessagesResponse) *openAIChatResponse {
	content := ""
	if len(anthropicResp.Content) > 0 {
		content = anthropicResp.Content[0].Text
	}

	finishReason := "stop"
	if anthropicResp.StopReason == "max_tokens" {
		finishReason = "length"
	}

	return &openAIChatResponse{
		ID:      anthropicResp.ID,
		Object:  "chat.completion",
		Created: 0,
		Model:   anthropicResp.Model,
		Choices: []struct {
			Index        int    `json:"index"`
			Message      *struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		}{
			{
				Index: 0,
				Message: &struct {
					Role    string `json:"role"`
					Content string `json:"content"`
				}{Role: "assistant", Content: content},
				FinishReason: finishReason,
			},
		},
		Usage: struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		}{
			PromptTokens:     anthropicResp.Usage.InputTokens,
			CompletionTokens: anthropicResp.Usage.OutputTokens,
			TotalTokens:      anthropicResp.Usage.InputTokens + anthropicResp.Usage.OutputTokens,
		},
	}
}

func strPtr(s string) *string { return &s }
