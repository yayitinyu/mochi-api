package controller

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"mochi-api/model"
	"mochi-api/relay"
)

// TestChannel verifies connectivity of a saved channel by listing its
// upstream models. POST /api/channels/:id/test
func TestChannel(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	channel, err := model.GetChannelById(id)
	if err != nil {
		respondError(c, http.StatusNotFound, "渠道不存在")
		return
	}
	models, latency, err := relay.ListUpstreamModels(channel.Type, channel.BaseURL, channel.ApiKey)
	if err != nil {
		respondError(c, http.StatusBadGateway, "连接失败: "+err.Error())
		return
	}
	respondData(c, gin.H{
		"latency_ms":  latency.Milliseconds(),
		"model_count": len(models),
	})
}

// FetchChannelModels lists upstream models for the channel form (works
// before the channel is saved). POST /api/channels/fetch_models
// An empty api_key with a channel_id falls back to the stored key,
// matching the edit form's "leave blank to keep" behavior.
func FetchChannelModels(c *gin.Context) {
	var req struct {
		Type      string `json:"type"`
		BaseURL   string `json:"base_url"`
		ApiKey    string `json:"api_key"`
		ChannelId int    `json:"channel_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "请求格式错误")
		return
	}
	req.BaseURL = strings.TrimRight(strings.TrimSpace(req.BaseURL), "/")
	if !strings.HasPrefix(req.BaseURL, "http://") && !strings.HasPrefix(req.BaseURL, "https://") {
		respondError(c, http.StatusBadRequest, "Base URL 必须以 http:// 或 https:// 开头")
		return
	}
	if req.ApiKey == "" && req.ChannelId > 0 {
		channel, err := model.GetChannelById(req.ChannelId)
		if err != nil {
			respondError(c, http.StatusNotFound, "渠道不存在")
			return
		}
		req.ApiKey = channel.ApiKey
	}
	models, _, err := relay.ListUpstreamModels(req.Type, req.BaseURL, req.ApiKey)
	if err != nil {
		respondError(c, http.StatusBadGateway, "获取失败: "+err.Error())
		return
	}
	if models == nil {
		models = []string{}
	}
	respondData(c, gin.H{"models": models})
}
