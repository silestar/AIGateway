package proxy

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"go.uber.org/zap"

	"github.com/bokelife/aigateway/internal/account"
	"github.com/bokelife/aigateway/internal/channel"
	"github.com/bokelife/aigateway/internal/config"
	adapterregistry "github.com/bokelife/aigateway/pkg/adapter/registry"
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

// Forward 转发请求到上游（非流式）
func (e *Engine) Forward(ctx context.Context, ch *channel.Channel, acc *account.Account, originalReq *http.Request) (*http.Response, error) {
	// 1. 获取解密后的 API Key
	plainKey, err := e.accountMgr.GetDecryptedAPIKey(ctx, acc.ID)
	if err != nil {
		return nil, fmt.Errorf("get decrypted api key: %w", err)
	}

	// 2. 获取适配器
	adp, err := adapterregistry.GetAdapter(ch.Type)
	if err != nil {
		return nil, fmt.Errorf("get adapter for type %s: %w", ch.Type, err)
	}

	// 3. 转换请求
	_, err = adp.ConvertRequest(ctx, originalReq, "")
	if err != nil {
		return nil, fmt.Errorf("convert request: %w", err)
	}

	// 4. 构建上游请求
	upstreamURL := ch.BaseURL + originalReq.URL.Path
	if originalReq.URL.RawQuery != "" {
		upstreamURL += "?" + originalReq.URL.RawQuery
	}

	upstreamReq, err := http.NewRequestWithContext(ctx, originalReq.Method, upstreamURL, originalReq.Body)
	if err != nil {
		return nil, fmt.Errorf("create upstream request: %w", err)
	}

	// 复制请求头（排除 Host 和 Authorization）
	for k, vv := range originalReq.Header {
		if k == "Host" || k == "Authorization" {
			continue
		}
		for _, v := range vv {
			upstreamReq.Header.Add(k, v)
		}
	}

	// 注入上游认证
	upstreamReq.Header.Set("Authorization", "Bearer "+plainKey)
	upstreamReq.Header.Set("Content-Type", "application/json")

	// 5. 发送请求
	resp, err := e.client.Do(upstreamReq)
	if err != nil {
		return nil, fmt.Errorf("upstream request: %w", err)
	}

	// 6. 转换响应
	convertedResp, err := adp.ConvertResponse(ctx, resp)
	if err != nil {
		return nil, fmt.Errorf("convert response: %w", err)
	}

	return convertedResp, nil
}

// ForwardStream 流式转发请求
func (e *Engine) ForwardStream(ctx context.Context, ch *channel.Channel, acc *account.Account, originalReq *http.Request, flusher http.Flusher, w io.Writer) error {
	// 1. 获取解密后的 API Key
	plainKey, err := e.accountMgr.GetDecryptedAPIKey(ctx, acc.ID)
	if err != nil {
		return fmt.Errorf("get decrypted api key: %w", err)
	}

	// 2. 获取适配器
	adp, err := adapterregistry.GetAdapter(ch.Type)
	if err != nil {
		return fmt.Errorf("get adapter for type %s: %w", ch.Type, err)
	}

	// 3. 构建上游请求
	upstreamURL := ch.BaseURL + originalReq.URL.Path
	if originalReq.URL.RawQuery != "" {
		upstreamURL += "?" + originalReq.URL.RawQuery
	}

	upstreamReq, err := http.NewRequestWithContext(ctx, originalReq.Method, upstreamURL, originalReq.Body)
	if err != nil {
		return fmt.Errorf("create upstream request: %w", err)
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

	// 4. 发送请求（不设超时，流式可能很长）
	streamClient := &http.Client{
		Transport: e.client.Transport,
	}
	resp, err := streamClient.Do(upstreamReq)
	if err != nil {
		return fmt.Errorf("upstream stream request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("upstream returned %d: %s", resp.StatusCode, string(body))
	}

	// 5. 逐 chunk 转发
	buf := make([]byte, 4096)
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			chunk := buf[:n]

			// 适配器转换
			converted, convErr := adp.ConvertStreamChunk(ctx, chunk)
			if convErr != nil {
				e.logger.Warn("convert stream chunk error", zap.Error(convErr))
			} else if converted != nil {
				if _, writeErr := w.Write(converted); writeErr != nil {
					return writeErr
				}
				flusher.Flush()
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return fmt.Errorf("read stream: %w", readErr)
		}
	}

	return nil
}
