package api

import (
	"strconv"

	"github.com/gin-gonic/gin"
)

// parseID 从 URL 参数 :id 解析 ID
func parseID(c *gin.Context) (uint, error) {
	s := c.Param("id")
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return 0, err
	}
	return uint(n), nil
}

// parseIDFromParam 从指定参数名解析 ID
func parseIDFromParam(c *gin.Context, param string) (uint, error) {
	s := c.Param(param)
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return 0, err
	}
	return uint(n), nil
}

// intQuery 从 query 参数解析整数
func intQuery(c *gin.Context, key string, defaultVal int) int {
	s := c.Query(key)
	if s == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return defaultVal
	}
	return n
}

// errorResponse 统一错误响应格式
func errorResponse(code, message string) gin.H {
	return gin.H{"error": gin.H{"code": code, "message": message}}
}

// maskKey 密钥脱敏显示
func maskKey(prefix string) string {
	if prefix == "" {
		return "****"
	}
	return prefix + "****"
}
