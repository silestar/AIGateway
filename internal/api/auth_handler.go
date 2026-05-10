package api

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/silestar/AIGateway/internal/config"
)

// AuthHandler 管理端认证 Handler
type AuthHandler struct {
	apiToken string
	sessions map[string]time.Time // token -> expireAt
	mu       sync.RWMutex
	cfg      *config.Config
}

func NewAuthHandler(apiToken string, cfg *config.Config) *AuthHandler {
	h := &AuthHandler{
		apiToken: apiToken,
		sessions: make(map[string]time.Time),
		cfg:      cfg,
	}
	// 启动清理协程
	go h.cleanupSessions()
	return h
}

// RegisterRoutes 注册认证路由（不需要鉴权）
func (h *AuthHandler) RegisterPublicRoutes(rg *gin.RouterGroup) {
	auth := rg.Group("/auth")
	auth.POST("/login", h.Login)

	// 公开的系统信息端点（登录页也需要获取版本号）
	sys := rg.Group("/system")
	sys.GET("/info", h.SystemInfo)
}

// SystemInfo 返回系统基本信息（无需认证）
func (h *AuthHandler) SystemInfo(c *gin.Context) {
	info := gin.H{
		"version": "0.1.0",
	}
	if h.cfg != nil {
		info["go_version"] = "1.25.0"
		info["port"] = h.cfg.Server.Port
		info["db_type"] = h.cfg.DB.Type
	}
	c.JSON(http.StatusOK, gin.H{"data": info})
}

// Login 管理端登录
func (h *AuthHandler) Login(c *gin.Context) {
	var req struct {
		Token string `json:"token" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("bad_request", "token is required"))
		return
	}

	// 调试：确认 apiToken 是否加载
	if h.apiToken == "" {
		c.JSON(http.StatusUnauthorized, errorResponse("unauthorized", "server api_token not configured, set AGW_SERVER_API_TOKEN in .env"))
		return
	}

	if req.Token != h.apiToken {
		c.JSON(http.StatusUnauthorized, errorResponse("unauthorized", "invalid token"))
		return
	}

	// 生成 session token
	sessionToken, err := h.generateToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("internal_error", "failed to generate token"))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"token":   sessionToken,
			"expires": 86400, // 24h
		},
	})
}

// AuthMiddleware 管理端鉴权中间件
func (h *AuthHandler) AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 如果未配置 api_token，跳过鉴权
		if h.apiToken == "" {
			c.Next()
			return
		}

		auth := c.GetHeader("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			c.JSON(http.StatusUnauthorized, errorResponse("unauthorized", "missing or invalid authorization header"))
			c.Abort()
			return
		}

		token := strings.TrimPrefix(auth, "Bearer ")
		if !h.validateToken(token) {
			c.JSON(http.StatusUnauthorized, errorResponse("unauthorized", "invalid or expired token"))
			c.Abort()
			return
		}

		c.Next()
	}
}

func (h *AuthHandler) generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	token := hex.EncodeToString(b)

	h.mu.Lock()
	h.sessions[token] = time.Now().Add(24 * time.Hour)
	h.mu.Unlock()

	return token, nil
}

func (h *AuthHandler) validateToken(token string) bool {
	h.mu.RLock()
	expireAt, ok := h.sessions[token]
	h.mu.RUnlock()

	if !ok {
		return false
	}

	if time.Now().After(expireAt) {
		h.mu.Lock()
		delete(h.sessions, token)
		h.mu.Unlock()
		return false
	}

	return true
}

func (h *AuthHandler) cleanupSessions() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		h.mu.Lock()
		now := time.Now()
		for token, expireAt := range h.sessions {
			if now.After(expireAt) {
				delete(h.sessions, token)
			}
		}
		h.mu.Unlock()
	}
}
