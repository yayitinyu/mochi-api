package middleware

import (
	"net/http"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"

	"mochi-api/model"
)

// UserAuth requires a logged-in session and stores id/role in the
// context. The user is re-read from the database on every request so
// that disabling a user or changing their role takes effect
// immediately; the role cached in the session is not trusted.
func UserAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		session := sessions.Default(c)
		id, ok := session.Get("id").(int)
		if !ok || id == 0 {
			abortAuth(c, session, "未登录或登录已过期")
			return
		}
		user, err := model.GetUserById(id)
		if err != nil {
			abortAuth(c, session, "未登录或登录已过期")
			return
		}
		if user.Status != model.StatusEnabled {
			abortAuth(c, session, "账号已被禁用")
			return
		}
		c.Set("id", id)
		c.Set("role", user.Role)
		c.Next()
	}
}

func abortAuth(c *gin.Context, session sessions.Session, message string) {
	session.Clear()
	_ = session.Save()
	c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
		"success": false,
		"message": message,
	})
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
