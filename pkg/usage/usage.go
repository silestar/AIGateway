package usage

import (
	"encoding/json"
	"strings"
)

// TokenUsage Token 用量信息
type TokenUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ExtractFromResponse 从非流式响应体中提取 token usage
// 支持 OpenAI / Anthropic / Gemini 三种格式
func ExtractFromResponse(body []byte) *TokenUsage {
	var resp map[string]json.RawMessage
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil
	}

	// OpenAI 格式: { "usage": { "prompt_tokens": N, "completion_tokens": N, "total_tokens": N } }
	if usageRaw, ok := resp["usage"]; ok {
		var usage TokenUsage
		if err := json.Unmarshal(usageRaw, &usage); err == nil && usage.TotalTokens > 0 {
			return &usage
		}

		// Anthropic 格式: { "usage": { "input_tokens": N, "output_tokens": N } }
		var anthUsage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		}
		if err := json.Unmarshal(usageRaw, &anthUsage); err == nil && anthUsage.InputTokens > 0 {
			return &TokenUsage{
				PromptTokens:     anthUsage.InputTokens,
				CompletionTokens: anthUsage.OutputTokens,
				TotalTokens:      anthUsage.InputTokens + anthUsage.OutputTokens,
			}
		}
	}

	// Gemini 格式: { "usageMetadata": { "promptTokenCount": N, "candidatesTokenCount": N, "totalTokenCount": N } }
	if metaRaw, ok := resp["usageMetadata"]; ok {
		var gemUsage struct {
			PromptTokenCount      int `json:"promptTokenCount"`
			CandidatesTokenCount  int `json:"candidatesTokenCount"`
			TotalTokenCount       int `json:"totalTokenCount"`
		}
		if err := json.Unmarshal(metaRaw, &gemUsage); err == nil && gemUsage.TotalTokenCount > 0 {
			return &TokenUsage{
				PromptTokens:     gemUsage.PromptTokenCount,
				CompletionTokens: gemUsage.CandidatesTokenCount,
				TotalTokens:      gemUsage.TotalTokenCount,
			}
		}
	}

	return nil
}

// ExtractFromStream 从流式最后一个 chunk 提取 usage
func ExtractFromStream(lastChunk string) *TokenUsage {
	for _, line := range strings.Split(lastChunk, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			continue
		}

		var chunk map[string]json.RawMessage
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		if usageRaw, ok := chunk["usage"]; ok {
			var u TokenUsage
			if err := json.Unmarshal(usageRaw, &u); err == nil && u.TotalTokens > 0 {
				return &u
			}
			var anthUsage struct {
				InputTokens  int `json:"input_tokens"`
				OutputTokens int `json:"output_tokens"`
			}
			if err := json.Unmarshal(usageRaw, &anthUsage); err == nil && anthUsage.InputTokens > 0 {
				return &TokenUsage{
					PromptTokens:     anthUsage.InputTokens,
					CompletionTokens: anthUsage.OutputTokens,
					TotalTokens:      anthUsage.InputTokens + anthUsage.OutputTokens,
				}
			}
		}
	}
	return nil
}
