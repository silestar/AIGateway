package main

import (
	"context"
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

	"github.com/bokelife/aigateway/internal/account"
	"github.com/bokelife/aigateway/internal/channel"
	"github.com/bokelife/aigateway/internal/config"
	"github.com/bokelife/aigateway/internal/consumer"
	"github.com/bokelife/aigateway/internal/crypto"
	"github.com/bokelife/aigateway/internal/group"
	agwlog "github.com/bokelife/aigateway/internal/log"
	"github.com/bokelife/aigateway/internal/plugin"
	"github.com/bokelife/aigateway/internal/proxy"
	"github.com/bokelife/aigateway/internal/stats"
	"github.com/bokelife/aigateway/internal/storage/sqlite"
	agwapi "github.com/bokelife/aigateway/internal/api"
	"github.com/bokelife/aigateway/pkg/middleware"
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
	consumerSvc := consumer.NewService(db)
	consumerSvc.SetCache(cache)
	accountMgr := account.NewManager(db, cache, cryptoService, cfg.AccountManager, logger)
	channelSvc := channel.NewService(db)
	groupRouter := group.NewRouter(db, consumerSvc, accountMgr, logger)
	proxyEngine := proxy.NewEngine(cfg.Proxy, accountMgr, logger)

	// 统计管理器 + 异步日志写入器
	statsMgr := stats.NewManager(db, logger)
	asyncWriter := stats.NewAsyncWriter(db, logger, statsMgr, 10000, 50, 100)
	asyncWriter.Start()
	statsMgr.StartAggregator()

	// 插件管理器
	pluginMgr := plugin.NewManager(db, logger, "plugins")

	// 启动账号池后台任务
	accountMgr.StartProbeScheduler()
	accountMgr.StartGlobalHealthCheck()

	// 7. 创建 Gin 引擎
	if cfg.Server.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}
	router := gin.New()
	router.Use(middleware.Recovery())
	router.Use(middleware.CORS())
	router.Use(middleware.Logger(logger))

	// 注入 db 到上下文
	router.Use(func(c *gin.Context) {
		c.Set("db", db)
		c.Set("consumerSvc", consumerSvc)
		c.Set("proxyEngine", proxyEngine)
		c.Set("accountMgr", accountMgr)
		c.Set("channelSvc", channelSvc)
		c.Set("groupRouter", groupRouter)
		c.Set("statsMgr", statsMgr)
		c.Set("asyncWriter", asyncWriter)
		c.Set("logger", logger)
		c.Next()
	})

	// 8. 注册路由（代理 + 健康检查）
	registerRoutes(router, cfg, logger)

	// 9. 认证 + 管理API
	authHandler := agwapi.NewAuthHandler(cfg.Server.APIToken)
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
	agwapi.NewConsumerHandler(consumerSvc).RegisterRoutes(protected)
	agwapi.NewChannelHandler(channelSvc).RegisterRoutes(protected)
	agwapi.NewAccountHandler(accountMgr).RegisterRoutes(protected)
	agwapi.NewGroupHandler(groupRouter).RegisterRoutes(protected)
	agwapi.NewStatsHandler(statsMgr).RegisterRoutes(protected)
	agwapi.NewLogHandler(statsMgr).RegisterRoutes(protected)
	agwapi.NewPluginHandler(pluginMgr).RegisterRoutes(protected)
	agwapi.NewSystemHandler(cfg).RegisterRoutes(protected)

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
func registerRoutes(r *gin.Engine, cfg *config.Config, logger *zap.Logger) {
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
	v1.GET("/models", notImplemented)
}

// handleChatCompletions 处理 Chat Completions 请求
func handleChatCompletions(c *gin.Context) {
	consumerSvc := c.MustGet("consumerSvc").(consumer.ConsumerService)
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
	cons, err := consumerSvc.Authenticate(c.Request.Context(), apiKey)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": gin.H{"code": "unauthorized", "message": "Invalid API key"},
		})
		return
	}

	// 3. 配额检查
	if err := consumerSvc.CheckQuota(c.Request.Context(), cons.ID, 0); err != nil {
		if qe, ok := err.(*consumer.QuotaError); ok {
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
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": gin.H{"code": "no_available_channel", "message": err.Error()},
		})
		return
	}

	// 6. 判断是否流式
	isStream := c.GetHeader("Accept") == "text/event-stream"

	// 7. 转发请求 + 记录日志
	var statusCode int
	var latencyMs int

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

		if err := proxyEngine.ForwardStream(c.Request.Context(), result.Channel, result.Account, c.Request, flusher, c.Writer); err != nil {
			result.RetryChain.MarkError(err.Error())
			statusCode = http.StatusBadGateway
			latencyMs = int(time.Since(startTime).Milliseconds())
			logger.Error("stream forward error", zap.Error(err))

			// 记录失败日志
			asyncWriter.Record(buildRequestLog(cons.ID, modelName, result, isStream, statusCode, latencyMs, err.Error()))

			c.JSON(http.StatusBadGateway, gin.H{
				"error": gin.H{"code": "upstream_error", "message": err.Error()},
			})
			return
		}
		result.RetryChain.MarkSuccess()
		statusCode = http.StatusOK
		latencyMs = int(time.Since(startTime).Milliseconds())
	} else {
		resp, err := proxyEngine.Forward(c.Request.Context(), result.Channel, result.Account, c.Request)
		latencyMs = int(time.Since(startTime).Milliseconds())

		if err != nil {
			result.RetryChain.MarkError(err.Error())
			// 同渠道内尝试下一个账号
			accountMgr.ReportResult(c.Request.Context(), result.Account.ID, false, 0)
			statusCode = http.StatusBadGateway

			// 记录失败日志
			asyncWriter.Record(buildRequestLog(cons.ID, modelName, result, isStream, statusCode, latencyMs, err.Error()))

			c.JSON(http.StatusBadGateway, gin.H{
				"error": gin.H{"code": "upstream_error", "message": err.Error()},
			})
			return
		}
		defer resp.Body.Close()
		result.RetryChain.MarkSuccess()
		statusCode = resp.StatusCode
		accountMgr.ReportResult(c.Request.Context(), result.Account.ID, true, resp.StatusCode)

		// 复制响应头
		for k, vv := range resp.Header {
			for _, v := range vv {
				c.Writer.Header().Add(k, v)
			}
		}
		c.Writer.WriteHeader(resp.StatusCode)
		io.Copy(c.Writer, resp.Body)
	}

	// 记录成功日志
	asyncWriter.Record(buildRequestLog(cons.ID, modelName, result, isStream, statusCode, latencyMs, ""))
}

// buildRequestLog 构造请求日志
func buildRequestLog(consumerID uint, modelName string, result *group.RouteResult, isStream bool, statusCode, latencyMs int, errMsg string) *stats.RequestLog {
	log := &stats.RequestLog{
		Timestamp:        time.Now(),
		ConsumerID:       consumerID,
		ModelName:        modelName,
		ChannelID:        &result.Channel.ID,
		AccountID:        &result.Account.ID,
		RetryChain:       result.RetryChain.ToJSON(),
		IsStream:         isStream,
		StatusCode:       statusCode,
		LatencyMs:        latencyMs,
	}
	if errMsg != "" {
		log.ErrorMsg = &errMsg
	}
	return log
}

// extractModelName 从请求体提取 model 名称
func extractModelName(c *gin.Context) string {
	// 尝试从 URL query 读取
	if m := c.Query("model"); m != "" {
		return m
	}
	// TODO: 从请求体 JSON 解析 model 字段
	// 目前用 query 参数或默认值
	return ""
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
