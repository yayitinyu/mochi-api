package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"mochi-api/model"
)

// PublicStatus exposes unauthenticated metadata used to adapt the login page.
func PublicStatus(c *gin.Context) {
	userCount, err := model.CountUsers()
	if err != nil {
		respondError(c, http.StatusInternalServerError, "数据库错误")
		return
	}
	respondData(c, gin.H{
		"register_mode":     model.GetRegisterMode(),
		"bootstrap_pending": userCount == 0,
	})
}
