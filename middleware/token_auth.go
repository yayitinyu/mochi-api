package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"mochi-api/model"
)

// TokenAuth authenticates relay requests via API key. It accepts
// "Authorization: Bearer sk-xxx" (OpenAI SDKs) and "x-api-key: sk-xxx"
// (Anthropic SDKs). claudeStyle controls the error body format.
func TokenAuth(claudeStyle bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.GetHeader("Authorization")
		key = strings.TrimPrefix(key, "Bearer ")
		if key == "" {
			key = c.GetHeader("x-api-key")
		}
		key = strings.TrimPrefix(strings.TrimSpace(key), "sk-")
		if key == "" {
			abortRelay(c, claudeStyle, http.StatusUnauthorized, "未提供 API 密钥")
			return
		}
		token, err := model.GetTokenByKey(key)
		if err != nil {
			abortRelay(c, claudeStyle, http.StatusInternalServerError, "数据库错误")
			return
		}
		if token == nil {
			abortRelay(c, claudeStyle, http.StatusUnauthorized, "无效的 API 密钥")
			return
		}
		if token.Status != model.StatusEnabled {
			abortRelay(c, claudeStyle, http.StatusUnauthorized, "该 API 密钥已被禁用")
			return
		}
		enabled, err := model.IsUserEnabled(token.UserId)
		if err != nil {
			abortRelay(c, claudeStyle, http.StatusInternalServerError, "数据库错误")
			return
		}
		if !enabled {
			abortRelay(c, claudeStyle, http.StatusUnauthorized, "该账号已被禁用")
			return
		}
		c.Set("user_id", token.UserId)
		c.Set("token_name", token.Name)
		c.Next()
	}
}

func abortRelay(c *gin.Context, claudeStyle bool, status int, message string) {
	if claudeStyle {
		c.AbortWithStatusJSON(status, gin.H{
			"type":  "error",
			"error": gin.H{"type": "authentication_error", "message": message},
		})
		return
	}
	c.AbortWithStatusJSON(status, gin.H{
		"error": gin.H{"message": message, "type": "invalid_request_error"},
	})
}
