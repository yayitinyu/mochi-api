package controller

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"mochi-api/model"
)

// ListRelayModels serves GET /v1/models in OpenAI format:
// the union of models across enabled channels.
func ListRelayModels(c *gin.Context) {
	models, err := model.GetEnabledModels()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{"message": "数据库错误", "type": "api_error"},
		})
		return
	}
	now := time.Now().Unix()
	data := make([]gin.H, 0, len(models))
	for _, m := range models {
		data = append(data, gin.H{
			"id": m, "object": "model", "created": now, "owned_by": "mochi-api",
		})
	}
	c.JSON(http.StatusOK, gin.H{"object": "list", "data": data})
}
