package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"mochi-api/model"
)

func GetSettings(c *gin.Context) {
	respondData(c, gin.H{
		"register_mode": model.GetRegisterMode(),
	})
}

type updateSettingsRequest struct {
	RegisterMode string `json:"register_mode"`
}

func UpdateSettings(c *gin.Context) {
	var req updateSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "请求格式错误")
		return
	}
	switch req.RegisterMode {
	case model.RegisterModeOpen, model.RegisterModeInvite, model.RegisterModeClosed:
	default:
		respondError(c, http.StatusBadRequest, "无效的注册模式")
		return
	}
	if err := model.SetOption(model.OptionRegisterMode, req.RegisterMode); err != nil {
		respondError(c, http.StatusInternalServerError, "保存设置失败")
		return
	}
	respondData(c, nil)
}
