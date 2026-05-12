package api

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/silestar/AIGateway/internal/config"
)

// AuthHandler 管理端认证 Handler
type AuthHandler struct {
	adminUser    string
	adminPass    string
	sessionStore SessionStore
	cfg          *config.Config
}

func NewAuthHandler(cfg *config.Config, sessionStore SessionStore) *AuthHandler {
	h := &AuthHandler{
		adminUser:    cfg.Server.AdminUser,
		adminPass:    cfg.Server.AdminPass,
		sessionStore: sessionStore,
		cfg:          cfg,
	}
	// 启动清理协程（SQLite 模式需要）
	go h.cleanupSessions()
	return h
}

// RegisterPublicRoutes 注册认证路由（不需要鉴权）
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
		"version": "0.2.0",
	}
	if h.cfg != nil {
		info["go_version"] = "1.25.0"
		info["port"] = h.cfg.Server.Port
		info["db_type"] = h.cfg.DB.Type
	}
	c.JSON(http.StatusOK, gin.H{"data": info})
}

// Login 管理端登录（账户+密码方式）
func (h *AuthHandler) Login(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("bad_request", "username and password are required"))
		return
	}

	// 检查管理端是否配置
	if h.adminUser == "" || h.adminPass == "" {
		c.JSON(http.StatusUnauthorized, errorResponse("unauthorized",
			"admin credentials not configured, set AGW_ADMIN_USER and AGW_ADMIN_PASS in .env"))
		return
	}

	if req.Username != h.adminUser || req.Password != h.adminPass {
		c.JSON(http.StatusUnauthorized, errorResponse("unauthorized", "invalid username or password"))
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
		// 如果未配置管理员账号，跳过鉴权（开发/自用场景）
		if h.adminUser == "" {
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
		valid, err := h.sessionStore.Validate(token)
		if err != nil || !valid {
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

	if err := h.sessionStore.Save(token, time.Now().Add(24*time.Hour)); err != nil {
		return "", err
	}

	return token, nil
}

// cleanupSessions 定期清理过期 session
func (h *AuthHandler) cleanupSessions() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		_ = h.sessionStore.Cleanup()
	}
}