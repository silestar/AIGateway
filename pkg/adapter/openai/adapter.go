package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/bokelife/aigateway/pkg/adapter"
)

// Adapter OpenAI Chat Completions 透传适配器
// 入站和出站都是 OpenAI 格式，零转换开销
type Adapter struct{}

func (a *Adapter) Type() string {
	return "openai"
}

// ConvertRequest 透传请求（OpenAI → OpenAI，零转换）
func (a *Adapter) ConvertRequest(ctx context.Context, originalReq *http.Request, targetModel string) (*http.Request, error) {
	// 如果指定了 targetModel 且与请求中的 model 不同，替换 model 字段
	// 大多数情况下直接透传
	return originalReq, nil
}

// ConvertResponse 透传响应（OpenAI → OpenAI，零转换）
func (a *Adapter) ConvertResponse(ctx context.Context, upstreamResp *http.Response) (*http.Response, error) {
	return upstreamResp, nil
}

// ConvertStreamChunk 透传 SSE chunk
func (a *Adapter) ConvertStreamChunk(ctx context.Context, chunk []byte) ([]byte, error) {
	return chunk, nil
}

// FetchModels 获取上游可用模型列表
func (a *Adapter) FetchModels(ctx context.Context, baseURL, apiKey string) ([]adapter.ModelInfo, error) {
	url := fmt.Sprintf("%s/v1/models", baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("fetch models status %d: %s", resp.StatusCode, string(body))
	}

	// 解析 OpenAI 模型列表响应
	var result struct {
		Data []struct {
			ID      string `json:"id"`
			OwnedBy string `json:"owned_by"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode models: %w", err)
	}

	models := make([]adapter.ModelInfo, 0, len(result.Data))
	for _, m := range result.Data {
		models = append(models, adapter.ModelInfo{
			ID:      m.ID,
			OwnedBy: m.OwnedBy,
		})
	}

	return models, nil
}
