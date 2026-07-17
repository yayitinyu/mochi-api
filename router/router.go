package router

import (
	"time"

	"github.com/gin-gonic/gin"

	"mochi-api/controller"
	"mochi-api/middleware"
	"mochi-api/relay"
)

// SetRelayRouter wires up the OpenAI/Claude-compatible relay under /v1.
func SetRelayRouter(r *gin.Engine) {
	v1 := r.Group("/v1")
	v1.Use(relayCORS())
	v1.OPTIONS("/*path", func(c *gin.Context) {
		c.Status(204)
	})
	v1.GET("/models", middleware.TokenAuth(false), controller.ListRelayModels)
	v1.POST("/chat/completions", middleware.TokenAuth(false), func(c *gin.Context) {
		relay.Handle(c, relay.FormatOpenAI)
	})
	v1.POST("/responses", middleware.TokenAuth(false), func(c *gin.Context) {
		relay.Handle(c, relay.FormatResponses)
	})
	v1.POST("/messages", middleware.TokenAuth(true), func(c *gin.Context) {
		relay.Handle(c, relay.FormatClaude)
	})
}

// relayCORS allows browser/WebView-based API clients (including Chatbox's
// direct mode) to call the token-authenticated /v1 endpoints. Dashboard
// session endpoints deliberately remain outside this policy.
func relayCORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		// Echo whatever headers the browser announced: relay auth is
		// token-based (no cookies), so reflecting request headers is safe and
		// keeps up with SDKs that add vendor headers (anthropic-beta,
		// x-stainless-*, ...) faster than a static list could.
		allowHeaders := c.GetHeader("Access-Control-Request-Headers")
		if allowHeaders == "" {
			allowHeaders = "Authorization, Content-Type, Accept, x-api-key, anthropic-version, anthropic-beta, OpenAI-Beta"
		}
		c.Header("Access-Control-Allow-Headers", allowHeaders)
		// The allowed-headers value echoes a request header, so caches must key
		// on it to avoid serving one client's preflight answer to another.
		c.Header("Vary", "Access-Control-Request-Headers")
		c.Header("Access-Control-Expose-Headers", "Content-Type, X-Request-Id")
		c.Header("Access-Control-Max-Age", "86400")
		c.Next()
	}
}

// SetApiRouter wires up the dashboard API under /api.
func SetApiRouter(r *gin.Engine) {
	api := r.Group("/api")
	api.Use(middleware.BodyLimit(1 << 20))

	api.GET("/status", controller.PublicStatus)

	// Session auth relies on bcrypt being slow; the limiter stops an
	// attacker from simply parallelizing guesses.
	authLimit := middleware.RateLimit(15, time.Minute)
	auth := api.Group("/auth")
	{
		auth.POST("/register", authLimit, controller.Register)
		auth.POST("/login", authLimit, controller.Login)
		auth.POST("/logout", controller.Logout)
		auth.GET("/me", middleware.UserAuth(), controller.Me)
	}

	user := api.Group("", middleware.UserAuth())
	{
		user.GET("/tokens", controller.ListTokens)
		user.POST("/tokens", controller.CreateToken)
		user.PUT("/tokens/:id", controller.UpdateToken)
		user.DELETE("/tokens/:id", controller.DeleteToken)

		user.GET("/logs", controller.ListLogs)
		user.GET("/stats/summary", controller.StatsSummary)
		user.GET("/stats/daily", controller.StatsDaily)
		user.GET("/stats/models", controller.StatsModels)
	}

	admin := api.Group("", middleware.UserAuth(), middleware.AdminAuth())
	{
		admin.GET("/channels", controller.ListChannels)
		admin.GET("/channels/models", controller.ListConfiguredModels)
		admin.POST("/channels", controller.CreateChannel)
		admin.PUT("/channels/:id", controller.UpdateChannel)
		admin.DELETE("/channels/:id", controller.DeleteChannel)
		admin.POST("/channels/:id/test", controller.TestChannel)
		admin.POST("/channels/fetch_models", controller.FetchChannelModels)

		admin.GET("/prices", controller.ListPrices)
		admin.POST("/prices", controller.CreatePrice)
		admin.PUT("/prices/:id", controller.UpdatePrice)
		admin.DELETE("/prices/:id", controller.DeletePrice)

		admin.GET("/users", controller.ListUsers)
		admin.PUT("/users/:id", controller.UpdateUser)
		admin.DELETE("/users/:id", controller.DeleteUser)

		admin.GET("/invites", controller.ListInvites)
		admin.POST("/invites", controller.CreateInvites)
		admin.DELETE("/invites/:id", controller.DeleteInvite)

		admin.GET("/settings", controller.GetSettings)
		admin.PUT("/settings", controller.UpdateSettings)

		admin.GET("/stats/users", controller.StatsUsers)

		admin.GET("/model_mappings", controller.ListModelMappings)
		admin.POST("/model_mappings", controller.CreateModelMapping)
		admin.PUT("/model_mappings/:id", controller.UpdateModelMapping)
		admin.DELETE("/model_mappings/:id", controller.DeleteModelMapping)
	}
}
