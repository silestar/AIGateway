package proxy

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/silestar/AIGateway/internal/account"
	"github.com/silestar/AIGateway/internal/channel"
	"github.com/silestar/AIGateway/internal/config"
	"github.com/silestar/AIGateway/internal/plugin"
	adapterregistry "github.com/silestar/AIGateway/pkg/adapter/registry"
	"github.com/silestar/AIGateway/pkg/usage"
)

// context keys for passing channel/account info to DialTLSContext
type ctxKey int

const (
	ctxKeyChannelID ctxKey = iota
	ctxKeyAccountID
)

// Engine HTTP 代理引擎
type Engine struct {
	logger             *zap.Logger
	cfg                config.ProxyConfig
	accountMgr         account.AccountManager
	pluginMgr          plugin.PluginManager
	client             *http.Client
	latencyThresholdMs int64 // 响应时间阈值（毫秒），0=不限制，仅非流式生效
}

// NewEngine 创建代理引擎
func NewEngine(cfg config.ProxyConfig, accountMgr account.AccountManager, pluginMgr plugin.PluginManager, logger *zap.Logger) *Engine {
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout: time.Duration(cfg.ConnectTimeout) * time.Second,
		}).DialContext,
		DialTLSContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			// 1. 从 context 获取 channel 信息（由上游调用者设置）
			channelID, _ := ctx.Value(ctxKeyChannelID).(uint)

			// 2. 检查该渠道是否有 running 的 connection_decorator 插件
			if pluginMgr != nil {
				if pluginAddr := pluginMgr.GetConnectionDecoratorAddr(channelID); pluginAddr != "" {
					// 构建权限头部（根据 context 中可用的信息）
					permHeaders := map[string]string{}
					accountID, _ := ctx.Value(ctxKeyAccountID).(uint)
					if accountID > 0 {
						permHeaders["X-AGW-Account-ID"] = fmt.Sprintf("%d", accountID)
					}
					if channelID > 0 {
						permHeaders["X-AGW-Channel-ID"] = fmt.Sprintf("%d", channelID)
					}
					// 尝试通过插件代理连接
					conn, err := dialViaDecorator(ctx, pluginAddr, addr, permHeaders)
					if err == nil {
						return conn, nil
					}
					// 插件不可用 → 回退标准 TLS
					logger.Warn("connection decorator unavailable, fallback to standard TLS",
						zap.String("addr", addr), zap.String("plugin_addr", pluginAddr), zap.Error(err))
				}
			}

			// 3. 标准路径：建立原始 TCP 连接
			dialer := &net.Dialer{}
			rawConn, err := dialer.DialContext(ctx, network, addr)
			if err != nil {
				return nil, err
			}

// 4. 标准 TLS 握手
		tlsCfg := &tls.Config{
			MinVersion: tls.VersionTLS12,
		}
		// 从 addr (host:port) 提取 hostname 作为 ServerName
		host, _, err := net.SplitHostPort(addr)
		if err != nil {
			host = addr
		}
		// 如果是 IP 地址则跳过证书域名校验，否则设 ServerName
		if ip := net.ParseIP(host); ip != nil {
			tlsCfg.InsecureSkipVerify = true
		} else {
			tlsCfg.ServerName = host
		}
			tlsConn := tls.Client(rawConn, tlsCfg)
			if err := tlsConn.HandshakeContext(ctx); err != nil {
				rawConn.Close()
				return nil, fmt.Errorf("tls handshake: %w", err)
			}
			return tlsConn, nil
		},
		MaxIdleConns:        cfg.MaxIdleConns,
		MaxIdleConnsPerHost: cfg.MaxIdleConns,
		IdleConnTimeout:     time.Duration(cfg.IdleConnTimeout) * time.Second,
	}

	return &Engine{
		logger:     logger,
		cfg:        cfg,
		accountMgr: accountMgr,
		pluginMgr:  pluginMgr,
		client: &http.Client{
			Transport: transport,
			Timeout:   time.Duration(cfg.ReadTimeout) * time.Second,
		},
	}
}

// SetLatencyThresholdMs 设置响应时间阈值（毫秒），0=不限制
func (e *Engine) SetLatencyThresholdMs(ms int64) {
	e.latencyThresholdMs = ms
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
	// 预先缓存请求 body，后续插件 pre_request 会读空 originalReq.Body，需独立备份给 upstreamReq
	if originalReq.Body != nil {
		cachedReqBody, _ := io.ReadAll(originalReq.Body)
		originalReq.Body = io.NopCloser(bytes.NewReader(cachedReqBody))
		upstreamReq.Body = io.NopCloser(bytes.NewReader(cachedReqBody))
		upstreamReq.ContentLength = int64(len(cachedReqBody))
	}

	for k, vv := range originalReq.Header {
		// 不转发 Accept-Encoding：让 Go Transport 自动管理 gzip（自动发送 + 自动解压），
		// 避免显式设置后 Go 不自动解压导致乱码
		if k == "Host" || k == "Authorization" || k == "Accept-Encoding" {
			continue
		}
		for _, v := range vv {
			upstreamReq.Header.Add(k, v)
		}
	}
	upstreamReq.Header.Set("Authorization", "Bearer "+plainKey)
	upstreamReq.Header.Set("Content-Type", "application/json")

	// === 插件钩子：pre_request ===
	if e.pluginMgr != nil {
		hookReq := &plugin.HookRequest{
			ChannelID: ch.ID,
			AccountID: acc.ID,
		}
		// 从请求体提取 model、headers 和 body
		if originalReq.Body != nil {
			if bodyBytes, readErr := io.ReadAll(originalReq.Body); readErr == nil {
				originalReq.Body = io.NopCloser(bytes.NewReader(bodyBytes))
				var bodyMap map[string]interface{}
				if json.Unmarshal(bodyBytes, &bodyMap) == nil {
					if m, ok := bodyMap["model"].(string); ok {
						hookReq.Model = m
					}
					hookReq.Request = &plugin.HookRequestBody{
						Body: bodyMap,
					}
				}
				// 触发 pre_request 钩子
				hookResp, hookErr := e.pluginMgr.TriggerHook(ctx, plugin.HookPreRequest, hookReq)
				if hookErr != nil {
					e.logger.Warn("pre_request hook error", zap.Error(hookErr))
				} else if hookResp != nil && hookResp.Action == plugin.ActionReject {
					return &ProxyResult{
						StatusCode:      hookResp.StatusCode,
						Body:            []byte(fmt.Sprintf(`{"error":{"code":"plugin_reject","message":"%s"}}`, hookResp.Message)),
						DisconnectType:  "plugin_reject",
					}, nil
				} else if hookResp != nil && hookResp.ModifiedRequest != nil && hookResp.ModifiedRequest.Body != nil {
					if modifiedBody, marshalErr := json.Marshal(hookResp.ModifiedRequest.Body); marshalErr == nil {
						upstreamReq.Body = io.NopCloser(bytes.NewReader(modifiedBody))
						upstreamReq.ContentLength = int64(len(modifiedBody))
					}
				}
			}
		}
	}

	resp, err := e.client.Do(upstreamReq)
	if err != nil {
		return nil, fmt.Errorf("upstream request: %w", err)
	}
	defer resp.Body.Close()

	// 记录请求耗时（用于 latency_threshold 判断）
	forwardStart := time.Now()

	// 处理 gzip 压缩的响应体（防御性：即使已过滤 Accept-Encoding，上游仍可能返回 gzip）
	var reader io.Reader = resp.Body
	if resp.Header.Get("Content-Encoding") == "gzip" {
		if gzReader, gzErr := gzip.NewReader(resp.Body); gzErr == nil {
			reader = gzReader
			defer gzReader.Close()
		}
		// gzip 解压失败则回退读原始 body
	}

	body, err := io.ReadAll(reader)
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

	// === 插件钩子：post_response ===
	if e.pluginMgr != nil {
		hookReq := &plugin.HookRequest{
			ChannelID: ch.ID,
			AccountID: acc.ID,
		}
		var respBodyMap map[string]interface{}
		if json.Unmarshal(body, &respBodyMap) == nil {
			if m, ok := respBodyMap["model"].(string); ok {
				hookReq.Model = m
			}
		}
		hookReq.Response = &plugin.HookResponseBody{
			StatusCode: resp.StatusCode,
			Body:       respBodyMap,
		}
		hookResp, hookErr := e.pluginMgr.TriggerHook(ctx, plugin.HookPostResponse, hookReq)
		if hookErr != nil {
			e.logger.Warn("post_response hook error", zap.Error(hookErr))
		} else if hookResp != nil && hookResp.ModifiedResponse != nil {
			// 插件修改了响应
			if hookResp.ModifiedResponse.Body != nil {
				if modifiedBody, marshalErr := json.Marshal(hookResp.ModifiedResponse.Body); marshalErr == nil {
					body = modifiedBody
				}
			}
			if hookResp.ModifiedResponse.StatusCode != 0 {
				resp.StatusCode = hookResp.ModifiedResponse.StatusCode
			}
		}
	}

	// 非成功响应：检查关键词匹配
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if kw := e.accountMgr.CheckDisableKeywords(string(body)); kw != "" {
			go e.accountMgr.DisableAccountByKeyword(ctx, acc.ID, kw)
		}
	}

	// 响应时间阈值检查（只对非流式请求，通过 ReportResult 累积失败）
	if e.latencyThresholdMs > 0 {
		elapsedMs := time.Since(forwardStart).Milliseconds()
		if elapsedMs > e.latencyThresholdMs {
			e.logger.Warn("forward latency exceeded threshold",
				zap.Uint("account_id", acc.ID),
				zap.Int64("elapsed_ms", elapsedMs),
				zap.Int64("threshold_ms", e.latencyThresholdMs),
			)
			go e.accountMgr.ReportResult(ctx, acc.ID, false, 0, nil)
		}
	}

	return &ProxyResult{
		StatusCode:        resp.StatusCode,
		Body:              body,
		Headers:           headers,
		Usage:             tokenUsage,
		ResponseModel:     respModel,
		FinishReason:      finishReason,
		SystemFingerprint: sysFP,
		UpstreamLatencyMs: upstreamLatencyMs,
		DisconnectType:   "normal",
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
    FirstTokenMs      int    `json:"first_token_ms,omitempty"`      // 首Token时间(ms)，仅流式
	// Body 流式响应的完整内容（上限 5MB），供 detail writer 写入文件
	Body []byte `json:"-"`
	// DisconnectType 请求终止原因（仅内部排查，不返回客户端）
	DisconnectType string `json:"disconnect_type,omitempty"`
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

	// 注入 stream_options.include_usage 让上游在流式最后一个 chunk 返回 token usage
	var reqBodyForUpstream io.Reader = originalReq.Body
	if bodyBytes, readErr := io.ReadAll(originalReq.Body); readErr == nil {
		var bodyMap map[string]interface{}
		if json.Unmarshal(bodyBytes, &bodyMap) == nil {
			if _, exists := bodyMap["stream_options"]; !exists {
				bodyMap["stream_options"] = map[string]interface{}{
					"include_usage": true,
				}
				if modified, marshalErr := json.Marshal(bodyMap); marshalErr == nil {
					reqBodyForUpstream = bytes.NewReader(modified)
				} else {
					reqBodyForUpstream = bytes.NewReader(bodyBytes)
				}
			} else {
				reqBodyForUpstream = bytes.NewReader(bodyBytes)
			}
		} else {
			reqBodyForUpstream = bytes.NewReader(bodyBytes)
		}
	}

	upstreamReq, err := http.NewRequestWithContext(ctx, originalReq.Method, upstreamURL, reqBodyForUpstream)
	if err != nil {
		return nil, fmt.Errorf("create upstream request: %w", err)
	}

	for k, vv := range originalReq.Header {
		// 不转发 Accept-Encoding：让 Go Transport 自动管理 gzip（自动发送 + 自动解压），
		// 避免显式设置后 Go 不自动解压导致乱码
		if k == "Host" || k == "Authorization" || k == "Accept-Encoding" {
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
		// 非流式错误响应：尝试 gzip 解压后再读取
		var reader io.Reader = resp.Body
		if resp.Header.Get("Content-Encoding") == "gzip" {
			if gzReader, gzErr := gzip.NewReader(resp.Body); gzErr == nil {
				reader = gzReader
				defer gzReader.Close()
			}
		}
		body, _ := io.ReadAll(reader)
		// 关键词匹配：检查上游错误响应体中是否包含禁用关键词
		bodyStr := string(body)
		if kw := e.accountMgr.CheckDisableKeywords(bodyStr); kw != "" {
			go e.accountMgr.DisableAccountByKeyword(ctx, acc.ID, kw)
		}
		return nil, fmt.Errorf("upstream returned %d: %s", resp.StatusCode, bodyStr)
	}

	// 流式读取超时：每次读到 chunk 后重置 deadline
	var streamReadTimeout time.Duration
	if e.cfg.StreamReadTimeout > 0 {
		streamReadTimeout = time.Duration(e.cfg.StreamReadTimeout) * time.Second
	}

	// 尝试获取底层连接以设置 read deadline
	var rawConn net.Conn
	if tcpConn, ok := resp.Body.(interface{ NetConn() net.Conn }); ok {
		rawConn = tcpConn.NetConn()
	} else {
		// resp.Body 可能被 http 透明 gzip 包装，尝试 unwrap
		if unwrap, ok := resp.Body.(interface{ Unwrap() io.ReadCloser }); ok {
			if inner := unwrap.Unwrap(); inner != nil {
				if tcpConn2, ok2 := inner.(interface{ NetConn() net.Conn }); ok2 {
					rawConn = tcpConn2.NetConn()
				}
			}
		}
	}

	if rawConn != nil && streamReadTimeout > 0 {
		_ = rawConn.SetReadDeadline(time.Now().Add(streamReadTimeout))
	}

	var firstTokenMs int                          // FRT：首Token时间
    upstreamStart := time.Now()                   // 流式请求发出时间
    var lastChunks strings.Builder
	var fullBody bytes.Buffer // 累积完整流式响应体（上限 5MB）
	buf := make([]byte, 4096)
	const maxBodySize = 5 * 1024 * 1024 // 5MB
	var bodyOverflow bool
	var streamUsage *usage.TokenUsage // 流式过程中逐步提取 usage
	for {
		// 检查客户端是否断开
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("client disconnected: %w", ctx.Err())
		default:
		}

		n, readErr := resp.Body.Read(buf)
if n > 0 {
            // 记录首Token时间（仅第一次读到数据时记录）
            if firstTokenMs == 0 {
                firstTokenMs = int(time.Since(upstreamStart).Milliseconds())
            }
            // 每次成功读到数据后重置流式读取 deadline
			if rawConn != nil && streamReadTimeout > 0 {
				_ = rawConn.SetReadDeadline(time.Now().Add(streamReadTimeout))
			}

			chunk := buf[:n]
			// 累积最后 64KB 的 chunk 用于提取摘要（不存完整响应体）
			lastChunks.Write(chunk)
			if lastChunks.Len() > 65536 {
				excess := lastChunks.Len() - 65536
				remaining := lastChunks.String()[excess:]
				lastChunks.Reset()
				lastChunks.WriteString(remaining)
			}

			// 逐个 chunk 尝试提取 usage（不在尾部也能抓到）
			if streamUsage == nil {
				streamUsage = usage.ExtractFromStream(string(chunk))
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
	// 优先用流式过程中提取的 usage，兜底从尾部 64KB 再搜一次
	if streamUsage == nil {
		streamUsage = usage.ExtractFromStream(lastChunkStr)
	}

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
        FirstTokenMs:      firstTokenMs,
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

// dialViaDecorator 通过 connection_decorator 插件代理连接
// 使用简化的 CONNECT 协议：发送目标地址 + 权限头部，插件完成 TLS 握手后返回已建立的连接
func dialViaDecorator(ctx context.Context, pluginAddr, targetAddr string, permHeaders map[string]string) (net.Conn, error) {
	// 连接插件进程
	conn, err := net.DialTimeout("tcp", pluginAddr, 3*time.Second)
	if err != nil {
		return nil, fmt.Errorf("connect to decorator plugin: %w", err)
	}

	// 构建 CONNECT 请求（含权限头部）
	var req strings.Builder
	req.WriteString(fmt.Sprintf("CONNECT %s\r\n", targetAddr))
	for k, v := range permHeaders {
		req.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
	}
	req.WriteString("\r\n")

	if _, err := conn.Write([]byte(req.String())); err != nil {
		conn.Close()
		return nil, fmt.Errorf("send CONNECT to decorator: %w", err)
	}

	// 读取插件响应（预期：200 OK\r\n\r\n）
	buf := make([]byte, 256)
	if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		conn.Close()
		return nil, fmt.Errorf("set deadline: %w", err)
	}
	n, err := conn.Read(buf)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("read decorator response: %w", err)
	}
	conn.SetReadDeadline(time.Time{}) // 清除 deadline

	resp := string(buf[:n])
	if !strings.HasPrefix(resp, "200") {
		conn.Close()
		return nil, fmt.Errorf("decorator rejected: %s", strings.TrimSpace(resp))
	}

	// 连接已建立，后续由插件双向转发
	return conn, nil
}
