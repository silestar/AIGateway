package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Recovery panic 恢复中间件
func Recovery() gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, err interface{}) {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "internal_error",
				"message": "Internal Server Error",
			},
		})
	})
}
