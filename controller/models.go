package controller

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"mochi-api/model"
)

// ListRelayModels serves GET /v1/models in OpenAI format:
// the union of models across enabled channels. Upstream names that have
// aliases are hidden; the aliases are shown instead.
func ListRelayModels(c *gin.Context) {
	models, err := model.GetEnabledModels()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{"message": "数据库错误", "type": "api_error"},
		})
		return
	}

	// Replace upstream names that have aliases with their aliases.
	hidden := model.GetUpstreamNamesWithAliases()
	aliases := model.GetAllAliases()

	// Also hide the alias names themselves from the first loop to prevent
	// duplicates if a channel natively lists a model name that matches an alias.
	for _, alias := range aliases {
		hidden[alias] = true
	}

	now := time.Now().Unix()
	data := make([]gin.H, 0, len(models)+len(aliases))
	for _, m := range models {
		if hidden[m] {
			continue // hide original name if it has an alias
		}
		data = append(data, gin.H{
			"id": m, "object": "model", "created": now, "owned_by": "mochi-api",
		})
	}
	// Add aliases if either a mapped target or a literal same-name upstream
	// exists in enabled channels. Routing keeps both kinds eligible.
	enabled := make(map[string]bool, len(models))
	for _, m := range models {
		enabled[m] = true
	}
	for _, alias := range aliases {
		hasEnabledTarget := false
		for _, m := range model.ResolveModelTargets(alias) {
			if enabled[m] {
				hasEnabledTarget = true
				break
			}
		}
		if hasEnabledTarget {
			data = append(data, gin.H{
				"id": alias, "object": "model", "created": now, "owned_by": "mochi-api",
			})
		}
	}
	c.JSON(http.StatusOK, gin.H{"object": "list", "data": data})
}
