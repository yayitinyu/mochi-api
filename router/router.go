package router

import (
	"github.com/gin-gonic/gin"

	"mochi-api/controller"
	"mochi-api/middleware"
	"mochi-api/relay"
)

// SetRelayRouter wires up the OpenAI/Claude-compatible relay under /v1.
func SetRelayRouter(r *gin.Engine) {
	v1 := r.Group("/v1")
	v1.GET("/models", middleware.TokenAuth(false), controller.ListRelayModels)
	v1.POST("/chat/completions", middleware.TokenAuth(false), func(c *gin.Context) {
		relay.Handle(c, relay.FormatOpenAI)
	})
	v1.POST("/messages", middleware.TokenAuth(true), func(c *gin.Context) {
		relay.Handle(c, relay.FormatClaude)
	})
}

// SetApiRouter wires up the dashboard API under /api.
func SetApiRouter(r *gin.Engine) {
	api := r.Group("/api")

	auth := api.Group("/auth")
	{
		auth.POST("/register", controller.Register)
		auth.POST("/login", controller.Login)
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
		admin.POST("/channels", controller.CreateChannel)
		admin.PUT("/channels/:id", controller.UpdateChannel)
		admin.DELETE("/channels/:id", controller.DeleteChannel)
		admin.POST("/channels/:id/test", controller.TestChannel)
		admin.POST("/channels/fetch_models", controller.FetchChannelModels)

		admin.GET("/prices", controller.ListPrices)
		admin.POST("/prices", controller.CreatePrice)
		admin.PUT("/prices/:id", controller.UpdatePrice)
		admin.DELETE("/prices/:id", controller.DeletePrice)
	}
}
