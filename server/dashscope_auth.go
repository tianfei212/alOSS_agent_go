package server

import (
	"log"
	"net/http"
	"strings"

	"github.com/derekt/oss-cli/config"
	"github.com/gin-gonic/gin"
)

// DashScopeAuthMiddleware F5 百炼专用鉴权中间件。
// 校验 Authorization Bearer 与进程内 AL_KEY 一致，不复用 F1-F4 的 AuthMiddleware。
func DashScopeAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := config.ValidateDashScopeKey(); err != nil {
			log.Printf("[ERROR] dashscope 鉴权: AL_KEY 未配置: %v", err)
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
				"error": "AL_KEY 未配置，请在 .env.local 中设置客户百炼 API Key",
			})
			return
		}

		expectedKey := config.AppConfig.DashScope.APIKey
		authHeader := c.GetHeader("Authorization")
		var token string
		if strings.HasPrefix(authHeader, "Bearer ") {
			token = strings.TrimPrefix(authHeader, "Bearer ")
		} else {
			token = authHeader
		}

		if token != expectedKey {
			log.Printf("[WARN] dashscope 鉴权失败，IP: %s", c.ClientIP())
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid API Key，F5 路由须使用 AL_KEY 作为 Bearer Token",
			})
			return
		}

		log.Printf("[DEBUG] dashscope 鉴权通过，IP: %s", c.ClientIP())
		c.Next()
	}
}
