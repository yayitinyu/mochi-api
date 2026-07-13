package controller

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"mochi-api/model"
)

func ListInvites(c *gin.Context) {
	codes, err := model.ListInviteCodes()
	if err != nil {
		respondError(c, http.StatusInternalServerError, "数据库错误")
		return
	}
	respondData(c, codes)
}

type createInvitesRequest struct {
	Count int `json:"count"`
}

func CreateInvites(c *gin.Context) {
	var req createInvitesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "请求格式错误")
		return
	}
	if req.Count < 1 {
		req.Count = 1
	}
	if req.Count > 100 {
		req.Count = 100
	}
	codes, err := model.CreateInviteCodes(c.GetInt("id"), req.Count)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "生成邀请码失败")
		return
	}
	respondData(c, codes)
}

func DeleteInvite(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		respondError(c, http.StatusBadRequest, "无效的邀请码 ID")
		return
	}
	if err := model.DeleteInviteCode(id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			respondError(c, http.StatusBadRequest, "仅可删除未使用的邀请码")
			return
		}
		respondError(c, http.StatusInternalServerError, "数据库错误")
		return
	}
	respondData(c, nil)
}
