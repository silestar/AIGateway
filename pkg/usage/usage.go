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
	CachedTokens     int `json:"cached_tokens"`     // 缓存命中Token数（来自 prompt_tokens_details.cached_tokens）
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
			extractCachedTokens(usageRaw, &usage)
			return &usage
		}

		// Anthropic 格式: { "usage": { "input_tokens": N, "output_tokens": N } }
		var anthUsage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		}
		if err := json.Unmarshal(usageRaw, &anthUsage); err == nil && anthUsage.InputTokens > 0 {
			u := &TokenUsage{
				PromptTokens:     anthUsage.InputTokens,
				CompletionTokens: anthUsage.OutputTokens,
				TotalTokens:      anthUsage.InputTokens + anthUsage.OutputTokens,
			}
			extractCachedTokens(usageRaw, u)
			return u
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

		usage := extractUsageFromSSEData(data)
		if usage != nil {
			return usage
		}
	}
	return nil
}

// extractUsageFromSSEData 从单行 SSE data 提取 usage
func extractUsageFromSSEData(data string) *TokenUsage {
	var chunk map[string]json.RawMessage
	if err := json.Unmarshal([]byte(data), &chunk); err != nil {
		return nil
	}

	if usageRaw, ok := chunk["usage"]; ok {
		var u TokenUsage
		if err := json.Unmarshal(usageRaw, &u); err == nil && (u.PromptTokens > 0 || u.CompletionTokens > 0) {
			// 提取 prompt_tokens_details.cached_tokens
			extractCachedTokens(usageRaw, &u)
			return &u
		}
		// Anthropic 格式
		var anthUsage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		}
		if err := json.Unmarshal(usageRaw, &anthUsage); err == nil && (anthUsage.InputTokens > 0 || anthUsage.OutputTokens > 0) {
			u := &TokenUsage{
				PromptTokens:     anthUsage.InputTokens,
				CompletionTokens: anthUsage.OutputTokens,
				TotalTokens:      anthUsage.InputTokens + anthUsage.OutputTokens,
			}
			extractCachedTokens(usageRaw, u)
			return u
		}
	}
	return nil
}

// extractCachedTokens 从 usage JSON 中提取 prompt_tokens_details.cached_tokens 或 cache_creation_input_tokens/cache_read_input_tokens
func extractCachedTokens(usageRaw json.RawMessage, u *TokenUsage) {
	var details struct {
		CachedTokens int `json:"cached_tokens"`
	}
	// OpenAI 格式: usage.prompt_tokens_details.cached_tokens
	var usageMap map[string]json.RawMessage
	if json.Unmarshal(usageRaw, &usageMap) == nil {
		if ptd, ok := usageMap["prompt_tokens_details"]; ok {
			if json.Unmarshal(ptd, &details) == nil && details.CachedTokens > 0 {
				u.CachedTokens = details.CachedTokens
				return
			}
		}
		// Anthropic 格式: usage.cache_read_input_tokens
		var cacheRead struct {
			CacheReadInputTokens int `json:"cache_read_input_tokens"`
		}
		if json.Unmarshal(usageRaw, &cacheRead) == nil && cacheRead.CacheReadInputTokens > 0 {
			u.CachedTokens = cacheRead.CacheReadInputTokens
		}
	}
}
