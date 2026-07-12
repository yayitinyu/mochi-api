package middleware

import (
	"net/http"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"

	"mochi-api/model"
)

// UserAuth requires a logged-in session and stores id/role in the context.
func UserAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		session := sessions.Default(c)
		id, ok := session.Get("id").(int)
		if !ok || id == 0 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"message": "未登录或登录已过期",
			})
			return
		}
		role, _ := session.Get("role").(int)
		c.Set("id", id)
		c.Set("role", role)
		c.Next()
	}
}

// AdminAuth requires the session user to be an admin. Must run after UserAuth.
func AdminAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.GetInt("role") < model.RoleAdmin {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"success": false,
				"message": "需要管理员权限",
			})
			return
		}
		c.Next()
	}
}
