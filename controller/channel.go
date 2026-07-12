package controller

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"mochi-api/model"
)

type channelRequest struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	BaseURL  string `json:"base_url"`
	ApiKey   string `json:"api_key"`
	Models   string `json:"models"`
	Priority int    `json:"priority"`
	Status   int    `json:"status"`
}

func (req *channelRequest) validate() string {
	req.Name = strings.TrimSpace(req.Name)
	req.BaseURL = strings.TrimRight(strings.TrimSpace(req.BaseURL), "/")
	req.Models = strings.TrimSpace(req.Models)
	if req.Name == "" {
		return "名称不能为空"
	}
	if req.Type != model.ChannelTypeOpenAI && req.Type != model.ChannelTypeAnthropic &&
		req.Type != model.ChannelTypeGemini {
		return "类型必须是 openai、anthropic 或 gemini"
	}
	if !strings.HasPrefix(req.BaseURL, "http://") && !strings.HasPrefix(req.BaseURL, "https://") {
		return "Base URL 必须以 http:// 或 https:// 开头"
	}
	if req.Models == "" {
		return "模型列表不能为空"
	}
	if req.Status == 0 {
		req.Status = model.StatusEnabled
	}
	if req.Status != model.StatusEnabled && req.Status != model.StatusDisabled {
		return "无效的状态值"
	}
	return ""
}

func ListChannels(c *gin.Context) {
	channels, err := model.GetAllChannels()
	if err != nil {
		respondError(c, http.StatusInternalServerError, "数据库错误")
		return
	}
	respondData(c, channels)
}

func CreateChannel(c *gin.Context) {
	var req channelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "请求格式错误")
		return
	}
	if msg := req.validate(); msg != "" {
		respondError(c, http.StatusBadRequest, msg)
		return
	}
	channel := &model.Channel{
		Name: req.Name, Type: req.Type, BaseURL: req.BaseURL, ApiKey: req.ApiKey,
		Models: req.Models, Priority: req.Priority, Status: req.Status,
		CreatedAt: time.Now().Unix(),
	}
	if err := model.CreateChannel(channel); err != nil {
		respondError(c, http.StatusInternalServerError, "创建失败")
		return
	}
	respondData(c, channel)
}

func UpdateChannel(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	channel, err := model.GetChannelById(id)
	if err != nil {
		respondError(c, http.StatusNotFound, "渠道不存在")
		return
	}
	var req channelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "请求格式错误")
		return
	}
	// An empty api_key keeps the stored key so edits don't require re-entering it.
	if req.ApiKey == "" {
		req.ApiKey = channel.ApiKey
	}
	if msg := req.validate(); msg != "" {
		respondError(c, http.StatusBadRequest, msg)
		return
	}
	channel.Name, channel.Type, channel.BaseURL = req.Name, req.Type, req.BaseURL
	channel.ApiKey, channel.Models = req.ApiKey, req.Models
	channel.Priority, channel.Status = req.Priority, req.Status
	if err := model.UpdateChannel(channel); err != nil {
		respondError(c, http.StatusInternalServerError, "更新失败")
		return
	}
	respondData(c, channel)
}

func DeleteChannel(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := model.DeleteChannel(id); err != nil {
		respondError(c, http.StatusInternalServerError, "删除失败")
		return
	}
	respondData(c, nil)
}
