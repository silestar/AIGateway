package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/silestar/AIGateway/internal/account"
	"github.com/silestar/AIGateway/internal/channel"
	"github.com/silestar/AIGateway/internal/config"
	"github.com/silestar/AIGateway/internal/keys"
	"github.com/silestar/AIGateway/internal/crypto"
	"github.com/silestar/AIGateway/internal/group"
	agwlog "github.com/silestar/AIGateway/internal/log"
	"github.com/silestar/AIGateway/internal/models"
	"github.com/silestar/AIGateway/internal/plugin"

	// System plugins — blank imports trigger init() registration
	_ "github.com/silestar/AIGateway/plugins/tls-fingerprint-masquerade"
	"github.com/silestar/AIGateway/internal/proxy"
	"github.com/silestar/AIGateway/internal/stats"
	"github.com/silestar/AIGateway/internal/storage/sqlite"
	agwapi "github.com/silestar/AIGateway/internal/api"
	"github.com/silestar/AIGateway/pkg/middleware"
	"github.com/silestar/AIGateway/pkg/usage"
)

func main() {
	// 1. 加载配置
	cfg, err := config.Load("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}

	// 2. 初始化日志
	logger, err := agwlog.NewLogger(cfg.Log)
	if err != nil {
		fmt.Fprintf(os.Stderr, "init logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	// 启动日志自动清理
	agwlog.StartLogCleaner(cfg.Log, logger)

	// 3. 确保加密密钥
	secretKey, err := crypto.EnsureSecretKey("./config/.env")
	if err != nil {
		logger.Fatal("ensure secret key", zap.Error(err))
	}

	// 4. 初始化加密服务
	cryptoService, err := crypto.NewCrypto(secretKey)
	if err != nil {
		logger.Fatal("init crypto", zap.Error(err))
	}

	// 5. 初始化存储
	store, err := sqlite.New(cfg.DB)
	if err != nil {
		logger.Fatal("init storage", zap.Error(err))
	}
	defer store.Close()
	db := store.GetDB()

	// 6. 初始化服务
	cache := account.NewMemoryCache() // 内存缓存降级
	keysSvc := keys.NewService(db)
	keysSvc.SetCache(cache)
	keysSvc.SetCrypto(cryptoService)
	channelSvc := channel.NewService(db)
	pluginMgr := plugin.NewManager(db, logger, cfg.Plugin.PluginDir, cfg.Plugin.SidecarTimeout)
	accountMgr := account.NewManager(db, cache, cryptoService, channelSvc, cfg.AccountManager, logger)
	groupRouter := group.NewRouter(db, keysSvc, accountMgr, logger, cache)
	proxyEngine := proxy.NewEngine(cfg.Proxy, accountMgr, pluginMgr, logger)

	// 模型目录服务 + 渠道模型变更回调
	catalogSvc := models.NewCatalogService(db, logger)
	channelSvc.SetOnModelsChange(func() {
		if err := catalogSvc.SyncFromChannelModels(context.Background()); err != nil {
			logger.Warn("failed to sync model catalog", zap.Error(err))
		}
	})

	// 启动时初始同步一次
	if err := catalogSvc.SyncFromChannelModels(context.Background()); err != nil {
		logger.Warn("initial model catalog sync failed", zap.Error(err))
	}

	// 统计管理器 + 异步日志写入器
	statsMgr := stats.NewManager(db, logger)
	asyncWriter := stats.NewAsyncWriter(db, logger, statsMgr, 10000, 50, 100)
	// 注册 on_log 钩子：日志入队前触发插件的 on_log 钩子
	asyncWriter.SetOnLogHook(func(log *stats.RequestLog) {
		if pluginMgr != nil {
			hookReq := &plugin.HookRequest{
				KeysID: log.KeysID,
				Model:  log.ModelName,
			}
			if log.StatusCode > 0 {
				hookReq.Response = &plugin.HookResponseBody{
					StatusCode: log.StatusCode,
				}
			}
			_, _ = pluginMgr.TriggerHook(context.Background(), plugin.HookOnLog, hookReq)
		}
	})
	asyncWriter.Start()
	statsMgr.StartAggregator()

	// 详细内容写入器
	detailWriter := agwlog.NewDetailWriter(&agwlog.DetailWriterConfig{
		Enabled:    cfg.Log.DetailLogEnabled,
		LogDir:     cfg.Log.Dir,
		MaxAgeDays: cfg.Log.MaxAgeDays,
	}, db)

	// 启动账号池后台任务
	accountMgr.SetOnProbeDone(func(channelID, accountID uint, success bool, logType string, elapsedMs int, statusCode int, errMsg string, promptTokens int, completionTokens int) {
		chID := channelID
		accID := accountID
		if success {
			statusCode = 200
		} else if statusCode == 0 {
			statusCode = 0
		}
		if errMsg == "" && !success {
			errMsg = logType + " failed"
		}
		log := &stats.RequestLog{
			Timestamp:       time.Now(),
			ChannelID:       &chID,
			AccountID:       &accID,
			ModelName:       logType,
			StatusCode:      statusCode,
			LatencyMs:       elapsedMs,
			LogType:         logType,
			TraceID:         middleware.GenerateTraceID(logType),
			PromptTokens:    promptTokens,
			CompletionTokens: completionTokens,
		}
		if errMsg != "" {
			log.ErrorMsg = &errMsg
		}
		asyncWriter.Record(log)
	})
	accountMgr.StartProbeScheduler()
	accountMgr.StartGlobalHealthCheck()

	// 7. 创建 Gin 引擎
	if cfg.Server.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}
	router := gin.New()
	router.Use(middleware.Recovery())
	router.Use(middleware.CORS())
	router.Use(middleware.TraceID())
	router.Use(middleware.Logger(logger))

	// 注入 db 到上下文
	router.Use(func(c *gin.Context) {
		c.Set("db", db)
		c.Set("keysSvc", keysSvc)
		c.Set("proxyEngine", proxyEngine)
		c.Set("accountMgr", accountMgr)
		c.Set("channelSvc", channelSvc)
		c.Set("groupRouter", groupRouter)
		c.Set("statsMgr", statsMgr)
		c.Set("asyncWriter", asyncWriter)
		c.Set("detailWriter", detailWriter)
		c.Set("logger", logger)
		c.Set("cache", cache)
		c.Set("pluginMgr", pluginMgr)
		c.Next()
	})

	// 8. 注册路由（代理 + 健康检查）
	registerRoutes(router, cfg, catalogSvc, logger)

	// 9. 认证 + 管理API
	authHandler := agwapi.NewAuthHandler(cfg.Server.APIToken, cfg)
	// 调试：打印 apiToken 长度
	if cfg.Server.APIToken != "" {
		logger.Info("api_token loaded", zap.Int("length", len(cfg.Server.APIToken)))
	} else {
		logger.Warn("api_token is empty, auth will be skipped")
	}
	apiGroup := router.Group("/api")
	apiGroup.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	// 登录接口不需要鉴权
	authHandler.RegisterPublicRoutes(apiGroup)
	// 以下接口需要鉴权
	protected := apiGroup.Group("")
	protected.Use(authHandler.AuthMiddleware())
	agwapi.NewKeysHandler(keysSvc).RegisterRoutes(protected)
	agwapi.NewChannelHandler(channelSvc, accountMgr, asyncWriter).RegisterRoutes(protected)
	agwapi.NewAccountHandler(accountMgr).RegisterRoutes(protected)
	agwapi.NewGroupHandler(groupRouter).RegisterRoutes(protected)
	agwapi.NewStatsHandler(statsMgr).RegisterRoutes(protected)
	agwapi.NewLogHandler(statsMgr, &cfg.Log).RegisterRoutes(protected)
	agwapi.NewPluginHandler(pluginMgr, cfg).RegisterRoutes(protected)
	agwapi.NewModelHandler(catalogSvc).RegisterRoutes(protected)
	agwapi.NewSystemHandler(cfg).RegisterRoutes(protected)
	agwapi.NewSystemLogHandler(cfg).RegisterRoutes(protected)

	// 10. 静态文件服务（前端 SPA）
	router.Static("/assets", "./web/dist/assets")
	router.StaticFile("/favicon.ico", "./web/dist/favicon.ico")
	router.NoRoute(func(c *gin.Context) {
		// SPA fallback：所有未匹配路由返回 index.html，交给 Vue Router 处理
		c.File("./web/dist/index.html")
	})

	// 11. 启动服务
	srv := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
	}

	go func() {
		logger.Info("server starting",
			zap.String("host", cfg.Server.Host),
			zap.Int("port", cfg.Server.Port),
			zap.String("mode", cfg.Server.Mode),
		)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("listen", zap.Error(err))
		}
	}()

	// 10. 优雅关闭
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info("shutting down...")

	// 停止统计和日志
	statsMgr.StopAggregator()
	asyncWriter.Close(5 * time.Second)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Fatal("forced shutdown", zap.Error(err))
	}
	logger.Info("server exited")
}

// registerRoutes 注册代理和健康检查路由
func registerRoutes(r *gin.Engine, cfg *config.Config, catalogSvc models.CatalogService, logger *zap.Logger) {
	// 健康检查
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	r.GET("/ready", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	})

	// ========== 代理端点 ==========
	v1 := r.Group("/v1")
	v1.POST("/chat/completions", handleChatCompletions)
	v1.POST("/completions", notImplemented)
	v1.GET("/models", func(c *gin.Context) {
		handleModelsList(c, catalogSvc)
	})
}

// handleChatCompletions 处理 Chat Completions 请求
func handleChatCompletions(c *gin.Context) {
	keysSvc := c.MustGet("keysSvc").(keys.KeysService)
	proxyEngine := c.MustGet("proxyEngine").(*proxy.Engine)
	groupRouter := c.MustGet("groupRouter").(*group.Router)
	accountMgr := c.MustGet("accountMgr").(account.AccountManager)
	asyncWriter := c.MustGet("asyncWriter").(*stats.AsyncWriter)
	logger := c.MustGet("logger").(*zap.Logger)

	startTime := time.Now()

	// 1. 提取 API Key
	apiKey := extractAPIKey(c)
	if apiKey == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": gin.H{"code": "unauthorized", "message": "Missing API key"},
		})
		return
	}

	// 2. 认证
	cons, err := keysSvc.Authenticate(c.Request.Context(), apiKey)
	if err != nil {
		traceID, _ := c.Get("trace_id")
		traceIDStr, _ := traceID.(string)
		modelName := extractModelName(c)
		errMsg := err.Error()
		asyncWriter.Record(&stats.RequestLog{
			Timestamp:  time.Now(),
			ModelName:  modelName,
			StatusCode: http.StatusUnauthorized,
			LatencyMs:  int(time.Since(startTime).Milliseconds()),
			LogType:    "consumption",
			TraceID:    traceIDStr,
			ClientIP:   c.ClientIP(),
			ErrorMsg:   &errMsg,
		})
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": gin.H{"code": "unauthorized", "message": "Invalid API key"},
		})
		return
	}

	// 3. 配额检查
	if err := keysSvc.CheckQuota(c.Request.Context(), cons.ID, 0); err != nil {
		if qe, ok := err.(*keys.QuotaError); ok {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": gin.H{"code": "quota_exceeded", "message": qe.Error()},
			})
			return
		}
	}

	// 4. 解析请求体中的 model 字段
	modelName := extractModelName(c)

	// 5. 路由选择
	result, err := groupRouter.Route(c.Request.Context(), cons.ID, modelName)
	if err != nil {
		errMsg := err.Error()
		traceID, _ := c.Get("trace_id")
		traceIDStr, _ := traceID.(string)
		asyncWriter.Record(&stats.RequestLog{
			Timestamp:  time.Now(),
			KeysID:     cons.ID,
			ModelName:  modelName,
			StatusCode: http.StatusServiceUnavailable,
			LatencyMs:  int(time.Since(startTime).Milliseconds()),
			LogType:    "consumption",
			TraceID:    traceIDStr,
			ClientIP:   c.ClientIP(),
			ErrorMsg:   &errMsg,
		})
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": gin.H{"code": "no_available_channel", "message": err.Error()},
		})
		return
	}

	// 5.1 插件钩子：account_select — 插件可过滤排除特定账号
	if pMgr, ok := c.Get("pluginMgr"); ok {
		if pluginMgr, ok := pMgr.(plugin.PluginManager); ok && pluginMgr != nil {
			hookReq := &plugin.HookRequest{
				KeysID:  cons.ID,
				Model:   modelName,
			}
			hookResp, hookErr := pluginMgr.TriggerHook(c.Request.Context(), plugin.HookAccountSelect, hookReq)
			if hookErr == nil && hookResp != nil && hookResp.Action == plugin.ActionFilter && len(hookResp.ExcludeIDs) > 0 {
				// 插件要求排除某些账号，检查当前选中的账号是否在排除列表中
				excluded := false
				for _, id := range hookResp.ExcludeIDs {
					if id == result.Account.ID {
						excluded = true
						break
					}
				}
				if excluded {
					// 当前账号被排除，用 SelectAccountWithExclude 重新选择
					newAcc, selectErr := accountMgr.SelectAccountWithExclude(c.Request.Context(), cons.ID, result.Channel.ID, hookResp.ExcludeIDs)
					if selectErr != nil {
						c.JSON(http.StatusServiceUnavailable, gin.H{
							"error": gin.H{"code": "no_available_account_after_plugin_filter", "message": "no available account after plugin filter"},
						})
						return
					}
					result.Account = newAcc
				}
			}
		}
	}

	// 5.5 模型映射替换：如果路由返回的 ActualModelName 与请求不同，替换请求体中的 model
	if result.ActualModelName != "" && result.ActualModelName != modelName {
		if parsedBody, exists := c.Get("parsedBody"); exists {
			reqBody := parsedBody.(map[string]interface{})
			reqBody["model"] = result.ActualModelName
			if newBody, err := json.Marshal(reqBody); err == nil {
				c.Request.Body = io.NopCloser(bytes.NewReader(newBody))
				c.Request.ContentLength = int64(len(newBody))
			}
		}
	}

	// 6. 判断是否流式（兼容 Header Accept 和 body stream 字段）
	isStream := c.GetHeader("Accept") == "text/event-stream"
	if !isStream {
		if parsedBody, exists := c.Get("parsedBody"); exists {
			if b, ok := parsedBody.(map[string]interface{}); ok {
				if s, ok := b["stream"]; ok {
					if v, ok := s.(bool); ok && v {
						isStream = true
					}
				}
			}
		}
	}
	clientIP := c.ClientIP()
	traceID, _ := c.Get("trace_id")
	traceIDStr, _ := traceID.(string)

	// 7. 转发请求 + 记录日志
	var statusCode int
	var latencyMs int
	var usage *usage.TokenUsage
	var respSummary *ResponseSummary

	if isStream {
		flusher, ok := c.Writer.(http.Flusher)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": gin.H{"code": "internal_error", "message": "Streaming not supported"},
			})
			return
		}

		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")

		// 流式请求：在未向客户端发送任何数据前失败时，允许重试一次
		currentStreamResult := result
		const maxStreamRetries = 1
		var streamResult *proxy.StreamResult

		for attempt := 0; attempt <= maxStreamRetries; attempt++ {
			streamResult, err = proxyEngine.ForwardStream(c.Request.Context(), currentStreamResult.Channel, currentStreamResult.Account, c.Request, flusher, c.Writer)
			latencyMs = int(time.Since(startTime).Milliseconds())

			if err != nil {
				// 只有在未向客户端发送任何数据时才可安全重试
				if !c.Writer.Written() && attempt < maxStreamRetries {
					accountMgr.ReportResult(c.Request.Context(), currentStreamResult.Account.ID, false, 0)
					currentStreamResult.RetryChain.MarkError(shortenError(err.Error()), latencyMs, http.StatusBadGateway)
					logger.Warn("stream forward failed (pre-write), retrying",
						zap.Int("attempt", attempt+1),
						zap.Uint("channel", currentStreamResult.Channel.ID),
						zap.Uint("account", currentStreamResult.Account.ID),
						zap.Error(err))

					retryResult, retryErr := groupRouter.RerouteAfterFailure(c.Request.Context(), currentStreamResult)
					if retryErr != nil {
						// 重试路由也失败，放弃
						logger.Warn("stream retry route failed", zap.Error(retryErr))
						break
					}
					currentStreamResult = retryResult
					continue
				}
				// 已写数据 或 重试也失败 → 不可恢复，直接返回错误
				break
			}
			// 成功
			break
		}

		if err != nil {
			result.RetryChain = currentStreamResult.RetryChain
			result.RetryChain.MarkError(shortenError(err.Error()), latencyMs, http.StatusBadGateway)
			statusCode = http.StatusBadGateway
			logger.Error("stream forward error", zap.Error(err))
			asyncWriter.Record(buildRequestLog(cons.ID, modelName, result.ActualModelName, result, isStream, statusCode, latencyMs, clientIP, nil, shortenError(err.Error()), traceIDStr, nil))

			c.JSON(http.StatusBadGateway, gin.H{
				"error": gin.H{"code": "upstream_error", "message": shortenError(err.Error())},
			})
			return
		}

		result.RetryChain = currentStreamResult.RetryChain
		result.RetryChain.MarkSuccess(latencyMs, http.StatusOK)
		statusCode = http.StatusOK
		if streamResult != nil {
			usage = streamResult.Usage
			respSummary = &ResponseSummary{
				ResponseModel:     streamResult.ResponseModel,
				FinishReason:      streamResult.FinishReason,
				SystemFingerprint: streamResult.SystemFingerprint,
				UpstreamLatencyMs: streamResult.UpstreamLatencyMs,
			}
			// 缓存流式完整响应体供 detail writer 使用
			c.Set("streamResultBody", streamResult.Body)
		}
	} else {
		// 非流式请求 — 带完整重试循环
		var proxyResult *proxy.ProxyResult
		maxRetries := 3 // TODO: 从配置读取
		retryCount := 0
		currentResult := result

		for {
			// 检查客户端是否断开
			select {
				case <-c.Request.Context().Done():
					_ = c.Request.Context().Err()
					statusCode = http.StatusGatewayTimeout
				result.RetryChain.MarkError("client disconnected", latencyMs, statusCode)
				errMsg := "client disconnected"
				asyncWriter.Record(buildRequestLog(cons.ID, modelName, result.ActualModelName, result, isStream, statusCode, latencyMs, clientIP, nil, errMsg, traceIDStr, nil))
				c.JSON(http.StatusGatewayTimeout, gin.H{"error": gin.H{"code": "client_disconnected", "message": errMsg}})
				return
			default:
			}

			// 检查重试上限
			if retryCount > 0 && retryCount >= maxRetries {
				errMsg := "max retry attempts exceeded"
				statusCode = http.StatusBadGateway
				result.RetryChain.MarkError(errMsg, latencyMs, statusCode)
				asyncWriter.Record(buildRequestLog(cons.ID, modelName, result.ActualModelName, result, isStream, statusCode, latencyMs, clientIP, nil, errMsg, traceIDStr, nil))
				c.JSON(http.StatusBadGateway, gin.H{"error": gin.H{"code": "max_retries_exceeded", "message": errMsg}})
				return
			}

			if retryCount > 0 {
				// 重试：重新路由获取下一个账号/渠道
				currentResult, err = groupRouter.RerouteAfterFailure(c.Request.Context(), currentResult)
				if err != nil {
					errMsg := err.Error()
					statusCode = http.StatusBadGateway
					asyncWriter.Record(buildRequestLog(cons.ID, modelName, result.ActualModelName, result, isStream, statusCode, latencyMs, clientIP, nil, errMsg, traceIDStr, nil))
					c.JSON(http.StatusBadGateway, gin.H{"error": gin.H{"code": "no_available_account", "message": errMsg}})
					return
				}
			}

			proxyResult, err = proxyEngine.Forward(c.Request.Context(), currentResult.Channel, currentResult.Account, c.Request)
			latencyMs = int(time.Since(startTime).Milliseconds())

			if err != nil {
				// 记录失败 + 进入重试循环
				currentResult.RetryChain.MarkError(shortenError(err.Error()), latencyMs, http.StatusBadGateway)
				accountMgr.ReportResult(c.Request.Context(), currentResult.Account.ID, false, 0)
				logger.Warn("forward attempt failed, retrying",
					zap.Int("retry", retryCount),
					zap.Uint("channel", currentResult.Channel.ID),
					zap.Uint("account", currentResult.Account.ID),
					zap.Error(err))
				retryCount++
				continue
			}

			// 成功！
			result.RetryChain = currentResult.RetryChain // 同步 retry_chain
			result.RetryChain.MarkSuccess(latencyMs, proxyResult.StatusCode)
			statusCode = proxyResult.StatusCode
			usage = proxyResult.Usage
			accountMgr.ReportResult(c.Request.Context(), currentResult.Account.ID, true, proxyResult.StatusCode)
			respSummary = &ResponseSummary{
				ResponseModel:     proxyResult.ResponseModel,
				FinishReason:      proxyResult.FinishReason,
				SystemFingerprint: proxyResult.SystemFingerprint,
				UpstreamLatencyMs: proxyResult.UpstreamLatencyMs,
			}

			// 复制响应头
			for k, vv := range proxyResult.Headers {
				for _, v := range vv {
					c.Writer.Header().Add(k, v)
				}
			}
			c.Writer.WriteHeader(proxyResult.StatusCode)
			c.Writer.Write(proxyResult.Body)
			c.Set("proxyResultBody", proxyResult.Body)
			break
		}
	}

	// 更新速率/配额计数器
	cache := c.MustGet("cache").(account.Cache)
	updateRateLimitCounters(cache, result.Channel.ID, result.Account.ID)

	// 记录成功日志（含响应摘要）
	asyncWriter.Record(buildRequestLog(cons.ID, modelName, result.ActualModelName, result, isStream, statusCode, latencyMs, clientIP, usage, "", traceIDStr, respSummary))

	// 写入详细请求/响应内容文件（异步）
	if detailWriter, ok := c.Get("detailWriter"); ok {
		if dw, ok := detailWriter.(*agwlog.DetailWriter); ok {
			var respBodyBytes []byte
			if statusCode >= 200 && statusCode < 300 {
				if !isStream {
					// 非流式：从 proxyResult 取缓存 body
					if pb, ok := c.Get("proxyResultBody"); ok {
						if b, ok := pb.([]byte); ok {
							respBodyBytes = b
						}
					}
				} else {
					// 流式：从 streamResult 取累积的完整 body
					if sb, ok := c.Get("streamResultBody"); ok {
						if b, ok := sb.([]byte); ok {
							respBodyBytes = b
						}
					}
				}
			}
			captureAndWriteDetail(c, dw, traceIDStr, respBodyBytes)
		}
	}
}

// buildRequestLog 构造请求日志
func buildRequestLog(keysID uint, modelName string, mappedModel string, result *group.RouteResult, isStream bool, statusCode, latencyMs int, clientIP string, usage *usage.TokenUsage, errMsg string, traceID string, respMeta *ResponseSummary) *stats.RequestLog {
	log := &stats.RequestLog{
		Timestamp:  time.Now(),
		KeysID:     keysID,
		ModelName:  modelName,
		MappedModel: mappedModel,
		ChannelID:  &result.Channel.ID,
		AccountID:  &result.Account.ID,
		RetryChain: result.RetryChain.ToJSON(),
		IsStream:   isStream,
		StatusCode: statusCode,
		LatencyMs:  latencyMs,
		LogType:    "consumption",
		TraceID:    traceID,
		ClientIP:   clientIP,
	}

	// Token 用量
	if usage != nil {
		log.PromptTokens = usage.PromptTokens
		log.CompletionTokens = usage.CompletionTokens
	}

	// 简易费用计算（基于模型名估算单价，后续可接入配置）
	log.Cost = estimateCost(modelName, log.PromptTokens, log.CompletionTokens)

	// 请求元数据（始终填充渠道+模型上下文，方便排查）
	reqMetaMap := map[string]interface{}{
		"model":        modelName,
		"stream":       isStream,
		"channel_id":   result.Channel.ID,
		"channel_name": result.Channel.Name,
		"account_id":   result.Account.ID,
	}

	// 错误上下文回填：失败时额外注入关键信息
	if errMsg != "" {
		log.ErrorMsg = &errMsg
		reqMetaMap["error_context"] = map[string]interface{}{
			"channel_id":   result.Channel.ID,
			"channel_name": result.Channel.Name,
			"account_id":   result.Account.ID,
			"model":        modelName,
		}
	}
	reqMeta, _ := json.Marshal(reqMetaMap)
	log.RequestMeta = reqMeta

	// 成功响应摘要填充
	if statusCode >= 200 && statusCode < 300 && respMeta != nil {
		respMetaMap := map[string]interface{}{}
		if respMeta.ResponseModel != "" {
			respMetaMap["model"] = respMeta.ResponseModel
			log.UpstreamModel = respMeta.ResponseModel
		}
		if respMeta.FinishReason != "" {
			respMetaMap["finish_reason"] = respMeta.FinishReason
		}
		if respMeta.SystemFingerprint != "" {
			respMetaMap["system_fingerprint"] = respMeta.SystemFingerprint
		}
		if respMeta.UpstreamLatencyMs > 0 {
			log.UpstreamLatencyMs = respMeta.UpstreamLatencyMs
		}
		if len(respMetaMap) > 0 {
			rm, _ := json.Marshal(respMetaMap)
			log.ResponseMeta = rm
		}
	}

	return log
}

// ResponseSummary 响应摘要（从 ProxyResult/StreamResult 提取）
type ResponseSummary struct {
	ResponseModel     string
	FinishReason      string
	SystemFingerprint string
	UpstreamLatencyMs int
}

// shortenError 精简错误信息，移除冗长 URL 和堆栈，保留核心错误类型
func shortenError(errMsg string) string {
	// 移除 "upstream request: Post \"https://...\": " 前缀模式
	if idx := strings.Index(errMsg, ": "); idx != -1 {
		suffix := errMsg[idx+2:]
		// 如果后缀仍含 URL，继续精简
		if strings.HasPrefix(suffix, "Post ") || strings.HasPrefix(suffix, "Get ") {
			// 提取 URL 后面的真正错误
			if urlEnd := strings.Index(suffix, "\": "); urlEnd != -1 {
				return suffix[urlEnd+3:]
			}
			if urlEnd := strings.Index(suffix, "\": "); urlEnd != -1 {
				return suffix[urlEnd+3:]
			}
		}
		// context deadline exceeded / connection refused 等标准错误
		return suffix
	}
	// 截断超长错误
	if len(errMsg) > 200 {
		return errMsg[:200] + "..."
	}
	return errMsg
}

// estimateCost 简易费用估算（美元），后续可接入模型价格配置
func estimateCost(modelName string, promptTokens, completionTokens int) float64 {
	// 默认费率（按每1K token计价）
	promptPrice := 0.001    // $0.001/1K input
	completionPrice := 0.003 // $0.003/1K output

	// 常见模型费率映射
	switch {
	case strings.Contains(modelName, "gpt-4o"), strings.Contains(modelName, "gpt-4-turbo"):
		promptPrice = 0.01
		completionPrice = 0.03
	case strings.Contains(modelName, "gpt-4"):
		promptPrice = 0.03
		completionPrice = 0.06
	case strings.Contains(modelName, "gpt-3.5"), strings.Contains(modelName, "gpt-4o-mini"):
		promptPrice = 0.0005
		completionPrice = 0.0015
	case strings.Contains(modelName, "claude-3.5-sonnet"), strings.Contains(modelName, "claude-3-5-sonnet"):
		promptPrice = 0.003
		completionPrice = 0.015
	case strings.Contains(modelName, "claude-3-opus"):
		promptPrice = 0.015
		completionPrice = 0.075
	case strings.Contains(modelName, "claude-3-haiku"), strings.Contains(modelName, "claude-3.5-haiku"):
		promptPrice = 0.00025
		completionPrice = 0.00125
	case strings.Contains(modelName, "gemini-1.5-pro"), strings.Contains(modelName, "gemini-2.5-pro"):
		promptPrice = 0.00125
		completionPrice = 0.005
	case strings.Contains(modelName, "gemini-1.5-flash"), strings.Contains(modelName, "gemini-2.0-flash"):
		promptPrice = 0.000075
		completionPrice = 0.0003
	}

	return float64(promptTokens)*promptPrice/1000 + float64(completionTokens)*completionPrice/1000
}

// extractModelName 从请求体 JSON 提取 model 字段，同时缓存 body 供后续转发使用
func extractModelName(c *gin.Context) string {
	// 读取请求体
	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return ""
	}
	c.Request.Body.Close()

	// 解析 JSON 提取 model
	var reqBody map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &reqBody); err != nil {
		// 非 JSON 格式，恢复 body 并尝试从 query 读取
		c.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		return c.Query("model")
	}

	modelName, _ := reqBody["model"].(string)

	// 将解析后的 body 缓存到 gin context，后续模型替换时复用
	c.Set("cachedBody", bodyBytes)
	c.Set("parsedBody", reqBody)

	// 恢复 body 供后续读取
	c.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))

	return modelName
}

func extractAPIKey(c *gin.Context) string {
	auth := c.GetHeader("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	return ""
}

func notImplemented(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{
		"error": gin.H{
			"code":    "not_implemented",
			"message": "This endpoint is not implemented yet",
		},
	})
}

// updateRateLimitCounters 更新账号级 RPM/TPM/每日请求计数器
func updateRateLimitCounters(cache account.Cache, channelID, accountID uint) {
	now := time.Now()
	minuteKey := now.Format("2006-01-02-15:04")
	todayKey := now.Format("2006-01-02")

	// 账号 RPM 计数器
	rpmKey := fmt.Sprintf("stats:account:%d:rpm:%s", accountID, minuteKey)
	cache.Incr(rpmKey)

	// 账号 TPM 计数器（暂时只计数请求，Token 计数需解析响应体）
	tpmKey := fmt.Sprintf("stats:account:%d:tpm:%s", accountID, minuteKey)
	cache.Incr(tpmKey)

	// 账号每日请求计数器
	dailyKey := fmt.Sprintf("stats:account:%d:daily_requests:%s", accountID, todayKey)
	cache.Incr(dailyKey)
}

// captureAndWriteDetail 捕获请求/响应内容并异步写入文件
func captureAndWriteDetail(c *gin.Context, dw *agwlog.DetailWriter, traceID string, respBodyBytes []byte) {
	// 捕获请求信息
	reqSection := agwlog.DetailSection{
		Method:  c.Request.Method,
		Path:    c.Request.URL.Path,
		Headers: captureHeaders(c.Request.Header),
	}

	// 捕获请求体
	if cachedBody, exists := c.Get("cachedBody"); exists {
		if bodyBytes, ok := cachedBody.([]byte); ok {
			var bodyJSON interface{}
			if json.Unmarshal(bodyBytes, &bodyJSON) == nil {
				reqSection.Body = bodyJSON
			}
		}
	}

	// 捕获响应信息
	respSection := agwlog.DetailSection{
		StatusCode: c.Writer.Status(),
		Headers:    captureHeaders(c.Writer.Header()),
	}

	// 尝试解析响应 body 为 JSON
	if len(respBodyBytes) > 0 {
		var respJSON interface{}
		if json.Unmarshal(respBodyBytes, &respJSON) == nil {
			respSection.Body = respJSON
		} else {
			// 非 JSON 就截断存字符串
			respSection.Body = string(respBodyBytes[:min(len(respBodyBytes), 4096)])
		}
	}

	dw.WriteDetail(traceID, time.Now(), reqSection, respSection)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// captureHeaders 捕获 HTTP headers（脱敏 key）
func captureHeaders(h map[string][]string) map[string]string {
	result := make(map[string]string)
	for k, vv := range h {
		if len(vv) > 0 {
			val := vv[0]
			// 脱敏 Authorization
			if k == "Authorization" && len(val) > 15 {
				val = val[:15] + "..." + val[len(val)-6:]
			}
			result[k] = val
		}
	}
	return result
}

// handleModelsList 处理 /v1/models 请求（OpenAI 兼容格式）
func handleModelsList(c *gin.Context, catalogSvc models.CatalogService) {
	list, err := catalogSvc.GetVisibleModels(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{"code": "internal_error", "message": err.Error()},
		})
		return
	}

	type modelItem struct {
		ID      string `json:"id"`
		Object  string `json:"object"`
		OwnedBy string `json:"owned_by"`
	}
	items := make([]modelItem, len(list))
	for i, m := range list {
		items[i] = modelItem{
			ID:      m.ModelName,
			Object:  "model",
			OwnedBy: "aigateway",
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"object": "list",
		"data":   items,
	})
}
