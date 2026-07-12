package controller

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"mochi-api/common"
	"mochi-api/model"
)

type tokenView struct {
	Id         int    `json:"id"`
	Name       string `json:"name"`
	KeyPreview string `json:"key_preview"`
	Status     int    `json:"status"`
	CreatedAt  int64  `json:"created_at"`
}

func ListTokens(c *gin.Context) {
	tokens, err := model.GetTokensByUserId(c.GetInt("id"))
	if err != nil {
		respondError(c, http.StatusInternalServerError, "数据库错误")
		return
	}
	views := make([]tokenView, 0, len(tokens))
	for _, t := range tokens {
		views = append(views, tokenView{
			Id: t.Id, Name: t.Name, KeyPreview: t.KeyPreview(),
			Status: t.Status, CreatedAt: t.CreatedAt,
		})
	}
	respondData(c, views)
}

func CreateToken(c *gin.Context) {
	var req struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "请求格式错误")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || len(req.Name) > 50 {
		respondError(c, http.StatusBadRequest, "名称不能为空且不超过 50 个字符")
		return
	}
	token := &model.Token{
		UserId:    c.GetInt("id"),
		Key:       common.GenerateKey(48),
		Name:      req.Name,
		Status:    model.StatusEnabled,
		CreatedAt: time.Now().Unix(),
	}
	if err := model.CreateToken(token); err != nil {
		respondError(c, http.StatusInternalServerError, "创建失败")
		return
	}
	// The full key is returned only once, at creation time.
	respondData(c, gin.H{"id": token.Id, "name": token.Name, "key": "sk-" + token.Key})
}

func UpdateToken(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	token, err := model.GetTokenById(id, c.GetInt("id"))
	if err != nil {
		respondError(c, http.StatusNotFound, "密钥不存在")
		return
	}
	var req struct {
		Name   *string `json:"name"`
		Status *int    `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "请求格式错误")
		return
	}
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" || len(name) > 50 {
			respondError(c, http.StatusBadRequest, "名称不能为空且不超过 50 个字符")
			return
		}
		token.Name = name
	}
	if req.Status != nil {
		if *req.Status != model.StatusEnabled && *req.Status != model.StatusDisabled {
			respondError(c, http.StatusBadRequest, "无效的状态值")
			return
		}
		token.Status = *req.Status
	}
	if err := model.UpdateToken(token); err != nil {
		respondError(c, http.StatusInternalServerError, "更新失败")
		return
	}
	respondData(c, nil)
}

func DeleteToken(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := model.DeleteToken(id, c.GetInt("id")); err != nil {
		respondError(c, http.StatusInternalServerError, "删除失败")
		return
	}
	respondData(c, nil)
}
