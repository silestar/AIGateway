package gemini

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

// Adapter Google Gemini API 适配器
// 负责将 OpenAI Chat Completions 格式 ↔ Gemini generateContent 格式互相转换
type Adapter struct{}

func (a *Adapter) Type() string {
	return "gemini"
}

// ConvertRequest 将 OpenAI Chat 请求转换为 Gemini generateContent 请求
func (a *Adapter) ConvertRequest(ctx context.Context, originalReq *http.Request, targetModel string) (*http.Request, error) {
	body, err := io.ReadAll(originalReq.Body)
	if err != nil {
		return nil, fmt.Errorf("read request body: %w", err)
	}
	originalReq.Body.Close()

	var openaiReq openAIChatRequest
	if err := json.Unmarshal(body, &openaiReq); err != nil {
		return nil, fmt.Errorf("parse openai request: %w", err)
	}

	geminiReq := convertToGeminiRequest(&openaiReq)
	geminiBody, err := json.Marshal(geminiReq)
	if err != nil {
		return nil, fmt.Errorf("marshal gemini request: %w", err)
	}

	// 构建上游 URL：/v1beta/models/{model}:generateContent
	model := openaiReq.Model
	if targetModel != "" {
		model = targetModel
	}
	method := "generateContent"
	if openaiReq.Stream {
		method = "streamGenerateContent"
	}

	upstreamURL := fmt.Sprintf("/v1beta/models/%s:%s", model, method)
	if originalReq.URL.RawQuery != "" {
		upstreamURL += "?" + originalReq.URL.RawQuery
	}

	newReq, err := http.NewRequestWithContext(ctx, originalReq.Method, upstreamURL, bytes.NewReader(geminiBody))
	if err != nil {
		return nil, fmt.Errorf("create gemini request: %w", err)
	}

	for k, vv := range originalReq.Header {
		if k == "Host" || k == "Authorization" {
			continue
		}
		for _, v := range vv {
			newReq.Header.Add(k, v)
		}
	}
	newReq.Header.Set("Content-Type", "application/json")

	return newReq, nil
}

// ConvertResponse 将 Gemini 响应转换为 OpenAI Chat 响应
func (a *Adapter) ConvertResponse(ctx context.Context, upstreamResp *http.Response) (*http.Response, error) {
	body, err := io.ReadAll(upstreamResp.Body)
	if err != nil {
		return nil, fmt.Errorf("read gemini response: %w", err)
	}
	upstreamResp.Body.Close()

	var geminiResp geminiGenerateContentResponse
	if err := json.Unmarshal(body, &geminiResp); err != nil {
		return nil, fmt.Errorf("parse gemini response: %w", err)
	}

	openaiResp := convertToOpenAIResponse(&geminiResp)
	respBody, err := json.Marshal(openaiResp)
	if err != nil {
		return nil, fmt.Errorf("marshal openai response: %w", err)
	}

	upstreamResp.Body = io.NopCloser(bytes.NewReader(respBody))
	upstreamResp.ContentLength = int64(len(respBody))
	return upstreamResp, nil
}

// ConvertStreamChunk 将 Gemini 流式 chunk 转换为 OpenAI SSE chunk
func (a *Adapter) ConvertStreamChunk(ctx context.Context, chunk []byte) ([]byte, error) {
	line := strings.TrimSpace(string(chunk))
	if !strings.HasPrefix(line, "data: ") {
		return nil, nil
	}
	data := strings.TrimPrefix(line, "data: ")
	if data == "[DONE]" {
		return []byte("data: [DONE]\n\n"), nil
	}

	var geminiChunk geminiGenerateContentResponse
	if err := json.Unmarshal([]byte(data), &geminiChunk); err != nil {
		return chunk, nil
	}

	// 提取文本
	content := ""
	if len(geminiChunk.Candidates) > 0 && len(geminiChunk.Candidates[0].Content.Parts) > 0 {
		content = geminiChunk.Candidates[0].Content.Parts[0].Text
	}

	finishReason := ""
	if len(geminiChunk.Candidates) > 0 {
		fr := geminiChunk.Candidates[0].FinishReason
		if fr == "STOP" {
			finishReason = "stop"
		} else if fr == "MAX_TOKENS" {
			finishReason = "length"
		}
	}

	openaiChunk := map[string]interface{}{
		"id":      "chatcmpl-gemini",
		"object":  "chat.completion.chunk",
		"created": 0,
		"model":   "gemini",
		"choices": []map[string]interface{}{
			{
				"index": 0,
				"delta": map[string]string{
					"content": content,
				},
			},
		},
	}
	if finishReason != "" {
		openaiChunk["choices"] = []map[string]interface{}{
			{
				"index":         0,
				"delta":         map[string]string{},
				"finish_reason": finishReason,
			},
		}
	}

	b, _ := json.Marshal(openaiChunk)
	return []byte("data: " + string(b) + "\n\n"), nil
}

// FetchModels Gemini 返回预设模型列表
func (a *Adapter) FetchModels(ctx context.Context, baseURL, apiKey string) ([]adapter.ModelInfo, error) {
	models := []adapter.ModelInfo{
		{ID: "gemini-2.0-flash", OwnedBy: "google"},
		{ID: "gemini-2.0-flash-lite", OwnedBy: "google"},
		{ID: "gemini-1.5-pro", OwnedBy: "google"},
		{ID: "gemini-1.5-flash", OwnedBy: "google"},
		{ID: "gemini-1.0-pro", OwnedBy: "google"},
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
	MaxTokens   int     `json:"max_tokens,omitempty"`
	Temperature float64 `json:"temperature,omitempty"`
	Stream      bool    `json:"stream,omitempty"`
}

type geminiGenerateContentRequest struct {
	Contents         []geminiContent   `json:"contents"`
	GenerationConfig *geminiGenConfig  `json:"generationConfig,omitempty"`
}

type geminiContent struct {
	Role  string        `json:"role"`
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiGenConfig struct {
	Temperature     float64 `json:"temperature,omitempty"`
	MaxOutputTokens int     `json:"maxOutputTokens,omitempty"`
}

type geminiGenerateContentResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
			Role string `json:"role"`
		} `json:"content"`
		FinishReason  string `json:"finishReason"`
	} `json:"candidates"`
	UsageMetadata *struct {
		PromptTokenCount     int `json:"promptTokenCount"`
		CandidatesTokenCount int `json:"candidatesTokenCount"`
		TotalTokenCount      int `json:"totalTokenCount"`
	} `json:"usageMetadata,omitempty"`
	ModelVersion string `json:"modelVersion,omitempty"`
}

// ========== 转换函数 ==========

func convertToGeminiRequest(openaiReq *openAIChatRequest) *geminiGenerateContentRequest {
	contents := make([]geminiContent, 0, len(openaiReq.Messages))

	for _, msg := range openaiReq.Messages {
		role := "user"
		if msg.Role == "assistant" {
			role = "model"
		}
		// 跳过 system 消息（Gemini 用 systemInstruction，简化处理）
		if msg.Role == "system" {
			// 可扩展：放入 systemInstruction 字段
			role = "user"
		}
		contents = append(contents, geminiContent{
			Role:  role,
			Parts: []geminiPart{{Text: msg.Content}},
		})
	}

	req := &geminiGenerateContentRequest{
		Contents: contents,
	}

	if openaiReq.Temperature > 0 || openaiReq.MaxTokens > 0 {
		req.GenerationConfig = &geminiGenConfig{}
		if openaiReq.Temperature > 0 {
			req.GenerationConfig.Temperature = openaiReq.Temperature
		}
		if openaiReq.MaxTokens > 0 {
			req.GenerationConfig.MaxOutputTokens = openaiReq.MaxTokens
		}
	}

	return req
}

func convertToOpenAIResponse(geminiResp *geminiGenerateContentResponse) *map[string]interface{} {
	content := ""
	if len(geminiResp.Candidates) > 0 && len(geminiResp.Candidates[0].Content.Parts) > 0 {
		content = geminiResp.Candidates[0].Content.Parts[0].Text
	}

	finishReason := "stop"
	if len(geminiResp.Candidates) > 0 {
		switch geminiResp.Candidates[0].FinishReason {
		case "MAX_TOKENS":
			finishReason = "length"
		case "SAFETY":
			finishReason = "content_filter"
		}
	}

	promptTokens := 0
	completionTokens := 0
	if geminiResp.UsageMetadata != nil {
		promptTokens = geminiResp.UsageMetadata.PromptTokenCount
		completionTokens = geminiResp.UsageMetadata.CandidatesTokenCount
	}

	return &map[string]interface{}{
		"id":      "chatcmpl-gemini",
		"object":  "chat.completion",
		"created": 0,
		"model":   "gemini",
		"choices": []map[string]interface{}{
			{
				"index": 0,
				"message": map[string]string{
					"role":    "assistant",
					"content": content,
				},
				"finish_reason": finishReason,
			},
		},
		"usage": map[string]int{
			"prompt_tokens":     promptTokens,
			"completion_tokens": completionTokens,
			"total_tokens":      promptTokens + completionTokens,
		},
	}
}
