package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/bokelife/aigateway/internal/account"
	"github.com/bokelife/aigateway/internal/channel"
	"github.com/bokelife/aigateway/internal/config"
	adapterregistry "github.com/bokelife/aigateway/pkg/adapter/registry"
	"github.com/bokelife/aigateway/pkg/usage"
)

// Engine HTTP 代理引擎
type Engine struct {
	logger     *zap.Logger
	cfg        config.ProxyConfig
	accountMgr account.AccountManager
	client     *http.Client
}

// NewEngine 创建代理引擎
func NewEngine(cfg config.ProxyConfig, accountMgr account.AccountManager, logger *zap.Logger) *Engine {
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout: time.Duration(cfg.ConnectTimeout) * time.Second,
		}).DialContext,
		MaxIdleConns:        cfg.MaxIdleConns,
		MaxIdleConnsPerHost: cfg.MaxIdleConns,
		IdleConnTimeout:     time.Duration(cfg.IdleConnTimeout) * time.Second,
	}

	return &Engine{
		logger:     logger,
		cfg:        cfg,
		accountMgr: accountMgr,
		client: &http.Client{
			Transport: transport,
			Timeout:   time.Duration(cfg.ReadTimeout) * time.Second,
		},
	}
}

// Forward 转发请求到上游（非流式），返回 ProxyResult 含响应体和 token usage
func (e *Engine) Forward(ctx context.Context, ch *channel.Channel, acc *account.Account, originalReq *http.Request) (*ProxyResult, error) {
	plainKey, err := e.accountMgr.GetDecryptedAPIKey(ctx, acc.ID)
	if err != nil {
		return nil, fmt.Errorf("get decrypted api key: %w", err)
	}

	adp, err := adapterregistry.GetAdapter(ch.Type)
	if err != nil {
		return nil, fmt.Errorf("get adapter for type %s: %w", ch.Type, err)
	}

	_, err = adp.ConvertRequest(ctx, originalReq, "")
	if err != nil {
		return nil, fmt.Errorf("convert request: %w", err)
	}

	upstreamURL := ch.BaseURL + originalReq.URL.Path
	if originalReq.URL.RawQuery != "" {
		upstreamURL += "?" + originalReq.URL.RawQuery
	}

	upstreamReq, err := http.NewRequestWithContext(ctx, originalReq.Method, upstreamURL, originalReq.Body)
	if err != nil {
		return nil, fmt.Errorf("create upstream request: %w", err)
	}

	for k, vv := range originalReq.Header {
		if k == "Host" || k == "Authorization" {
			continue
		}
		for _, v := range vv {
			upstreamReq.Header.Add(k, v)
		}
	}
	upstreamReq.Header.Set("Authorization", "Bearer "+plainKey)
	upstreamReq.Header.Set("Content-Type", "application/json")

	resp, err := e.client.Do(upstreamReq)
	if err != nil {
		return nil, fmt.Errorf("upstream request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	tokenUsage := usage.ExtractFromResponse(body)

	// 提取响应摘要（model/finish_reason/system_fingerprint）
	var respModel, finishReason, sysFP string
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		var respObj map[string]json.RawMessage
		if json.Unmarshal(body, &respObj) == nil {
			if m, ok := respObj["model"]; ok {
				var s string
				if json.Unmarshal(m, &s) == nil {
					respModel = s
				}
			}
			// OpenAI: choices[0].finish_reason
			if choicesRaw, ok := respObj["choices"]; ok {
				var choices []map[string]json.RawMessage
				if json.Unmarshal(choicesRaw, &choices) == nil && len(choices) > 0 {
					if fr, ok := choices[0]["finish_reason"]; ok {
						var s string
						if json.Unmarshal(fr, &s) == nil {
							finishReason = s
						}
					}
				}
			}
			// Anthropic: stop_reason
			if sr, ok := respObj["stop_reason"]; ok {
				var s string
				if json.Unmarshal(sr, &s) == nil && finishReason == "" {
					finishReason = s
				}
			}
			if fp, ok := respObj["system_fingerprint"]; ok {
				var s string
				if json.Unmarshal(fp, &s) == nil {
					sysFP = s
				}
			}
		}
	}

	headers := make(map[string][]string)
	for k, vv := range resp.Header {
		headers[k] = vv
	}

	// 从上游响应头提取处理耗时
	upstreamLatencyMs := extractUpstreamLatency(resp.Header)

	return &ProxyResult{
		StatusCode:        resp.StatusCode,
		Body:              body,
		Headers:           headers,
		Usage:             tokenUsage,
		ResponseModel:     respModel,
		FinishReason:      finishReason,
		SystemFingerprint: sysFP,
		UpstreamLatencyMs: upstreamLatencyMs,
	}, nil
}

// StreamResult 流式转发结果
type StreamResult struct {
	Usage *usage.TokenUsage
	// 响应摘要（成功时填充）
	ResponseModel     string `json:"response_model,omitempty"`
	FinishReason      string `json:"finish_reason,omitempty"`
	SystemFingerprint string `json:"system_fingerprint,omitempty"`
	UpstreamLatencyMs int    `json:"upstream_latency_ms,omitempty"` // 上游处理耗时(ms)
	// Body 流式响应的完整内容（上限 5MB），供 detail writer 写入文件
	Body []byte `json:"-"`
}

// ForwardStream 流式转发请求，边读边转发，结束后返回 StreamResult
func (e *Engine) ForwardStream(ctx context.Context, ch *channel.Channel, acc *account.Account, originalReq *http.Request, flusher http.Flusher, w io.Writer) (*StreamResult, error) {
	plainKey, err := e.accountMgr.GetDecryptedAPIKey(ctx, acc.ID)
	if err != nil {
		return nil, fmt.Errorf("get decrypted api key: %w", err)
	}

	adp, err := adapterregistry.GetAdapter(ch.Type)
	if err != nil {
		return nil, fmt.Errorf("get adapter for type %s: %w", ch.Type, err)
	}

	upstreamURL := ch.BaseURL + originalReq.URL.Path
	if originalReq.URL.RawQuery != "" {
		upstreamURL += "?" + originalReq.URL.RawQuery
	}

	upstreamReq, err := http.NewRequestWithContext(ctx, originalReq.Method, upstreamURL, originalReq.Body)
	if err != nil {
		return nil, fmt.Errorf("create upstream request: %w", err)
	}

	for k, vv := range originalReq.Header {
		if k == "Host" || k == "Authorization" {
			continue
		}
		for _, v := range vv {
			upstreamReq.Header.Add(k, v)
		}
	}
	upstreamReq.Header.Set("Authorization", "Bearer "+plainKey)
	upstreamReq.Header.Set("Content-Type", "application/json")

	streamClient := &http.Client{Transport: e.client.Transport}
	resp, err := streamClient.Do(upstreamReq)
	if err != nil {
		return nil, fmt.Errorf("upstream stream request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("upstream returned %d: %s", resp.StatusCode, string(body))
	}

	var lastChunks strings.Builder
	var fullBody bytes.Buffer // 累积完整流式响应体（上限 5MB）
	buf := make([]byte, 4096)
	const maxBodySize = 5 * 1024 * 1024 // 5MB
	var bodyOverflow bool
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			chunk := buf[:n]
			// 累积最后 64KB 的 chunk 用于提取 usage（不存完整响应体，避免数据库暴增）
			lastChunks.Write(chunk)
			if lastChunks.Len() > 65536 {
				// 只保留最后 64KB
				excess := lastChunks.Len() - 65536
				remaining := lastChunks.String()[excess:]
				lastChunks.Reset()
				lastChunks.WriteString(remaining)
			}

			// 累积完整 body 供 detail writer（上限 5MB）
			if !bodyOverflow {
				if fullBody.Len()+n > maxBodySize {
					bodyOverflow = true
					fullBody.Reset()
					fullBody.WriteString(`{"overflow": true}`)
				} else {
					fullBody.Write(chunk)
				}
			}

			converted, convErr := adp.ConvertStreamChunk(ctx, chunk)
			if convErr != nil {
				e.logger.Warn("convert stream chunk error", zap.Error(convErr))
			} else if converted != nil {
				if _, writeErr := w.Write(converted); writeErr != nil {
					return nil, writeErr
				}
				flusher.Flush()
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return nil, fmt.Errorf("read stream: %w", readErr)
		}
	}

	lastChunkStr := lastChunks.String()
	streamUsage := usage.ExtractFromStream(lastChunkStr)

	// 从流式最后 chunk 提取响应摘要
	var respModel, finishReason, sysFP string
	// 从累积的尾部 chunks 的 SSE data 行中提取
	for _, line := range strings.Split(lastChunkStr, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data: ") || line == "data: [DONE]" {
			continue
		}
		var chunk map[string]json.RawMessage
		if json.Unmarshal([]byte(line[6:]), &chunk) != nil {
			continue
		}
		if m, ok := chunk["model"]; ok {
			var s string
			if json.Unmarshal(m, &s) == nil {
				respModel = s
			}
		}
		// OpenAI stream: choices[0].finish_reason
		if choicesRaw, ok := chunk["choices"]; ok {
			var choices []map[string]json.RawMessage
			if json.Unmarshal(choicesRaw, &choices) == nil && len(choices) > 0 {
				if fr, ok := choices[0]["finish_reason"]; ok {
					var s string
					if json.Unmarshal(fr, &s) == nil && s != "" && s != "null" {
						finishReason = s
					}
				}
			}
		}
		if fp, ok := chunk["system_fingerprint"]; ok {
			var s string
			if json.Unmarshal(fp, &s) == nil {
				sysFP = s
			}
		}
	}

	return &StreamResult{
		Usage:             streamUsage,
		ResponseModel:     respModel,
		FinishReason:      finishReason,
		SystemFingerprint: sysFP,
		UpstreamLatencyMs: extractUpstreamLatency(resp.Header),
		Body:              fullBody.Bytes(),
	}, nil
}

// ParseModelName 从请求体解析 model 字段
func ParseModelName(body []byte) string {
	var reqBody map[string]interface{}
	if err := json.Unmarshal(body, &reqBody); err != nil {
		return ""
	}
	modelName, _ := reqBody["model"].(string)
	return modelName
}

// IsStreamRequest 判断是否为流式请求
func IsStreamRequest(body []byte) bool {
	var reqBody map[string]interface{}
	if err := json.Unmarshal(body, &reqBody); err != nil {
		return false
	}
	stream, _ := reqBody["stream"].(bool)
	return stream
}

// ExtractRequestMeta 提取请求元数据摘要
func ExtractRequestMeta(body []byte) json.RawMessage {
	var reqBody map[string]interface{}
	if err := json.Unmarshal(body, &reqBody); err != nil {
		return nil
	}

	meta := make(map[string]interface{})
	if model, ok := reqBody["model"]; ok {
		meta["model"] = model
	}
	if stream, ok := reqBody["stream"]; ok {
		meta["stream"] = stream
	}
	if temperature, ok := reqBody["temperature"]; ok {
		meta["temperature"] = temperature
	}
	if maxTokens, ok := reqBody["max_tokens"]; ok {
		meta["max_tokens"] = maxTokens
	}
	if messages, ok := reqBody["messages"]; ok {
		if msgArr, ok := messages.([]interface{}); ok {
			meta["message_count"] = len(msgArr)
		}
	}

	result, _ := json.Marshal(meta)
	return json.RawMessage(result)
}

// BuildErrorMessage 从上游响应构建错误信息
func BuildErrorMessage(statusCode int, body []byte) string {
	errMsg := string(body)
	if len(errMsg) > 500 {
		errMsg = errMsg[:500] + "..."
	}

	// 尝试提取结构化错误
	var errResp map[string]json.RawMessage
	if json.Unmarshal(body, &errResp) == nil {
		if errorObj, ok := errResp["error"]; ok {
			var errStruct struct {
				Message string `json:"message"`
				Code    string `json:"code"`
			}
			if json.Unmarshal(errorObj, &errStruct) == nil && errStruct.Message != "" {
				if errStruct.Code != "" {
					return fmt.Sprintf("%s: %s", errStruct.Code, errStruct.Message)
				}
				return errStruct.Message
			}
		}
	}

	return fmt.Sprintf("HTTP %d: %s", statusCode, errMsg)
}

// SplitSSEData 分割 SSE 数据流中的 data 行
func SplitSSEData(data string) []string {
	var results []string
	for _, line := range strings.Split(data, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "data: ") {
			results = append(results, strings.TrimPrefix(line, "data: "))
		}
	}
	return results
}

// extractUpstreamLatency 从上游响应头中提取处理耗时(ms)
// 支持: openai-processing-ms, x-processing-time, x-request-duration
func extractUpstreamLatency(headers http.Header) int {
	for _, key := range []string{"Openai-Processing-Ms", "X-Processing-Time", "X-Request-Duration"} {
		if v := headers.Get(key); v != "" {
			if ms, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && ms > 0 {
				return ms
			}
		}
	}
	return 0
}
