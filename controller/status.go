package controller

import (
	"github.com/gin-gonic/gin"

	"mochi-api/model"
)

// PublicStatus exposes unauthenticated site metadata, currently just
// the registration mode so the login page can adapt its form.
func PublicStatus(c *gin.Context) {
	respondData(c, gin.H{
		"register_mode": model.GetRegisterMode(),
	})
}
