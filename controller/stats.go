package controller

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"mochi-api/model"
)

// StatsSummary returns today/this-week/this-month request, token and cost totals.
func StatsSummary(c *gin.Context) {
	userId := statsUserId(c)
	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	// Week starts on Monday.
	weekday := int(now.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	weekStart := todayStart.AddDate(0, 0, -(weekday - 1))
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())

	result := gin.H{}
	for name, since := range map[string]time.Time{
		"today": todayStart, "week": weekStart, "month": monthStart,
	} {
		stat, err := model.GetPeriodStat(userId, since.Unix())
		if err != nil {
			respondError(c, http.StatusInternalServerError, "数据库错误")
			return
		}
		result[name] = stat
	}
	respondData(c, result)
}

func StatsDaily(c *gin.Context) {
	days, _ := strconv.Atoi(c.DefaultQuery("days", "365"))
	if days < 1 || days > 731 {
		days = 365
	}
	stats, err := model.GetDailyStats(statsUserId(c), days)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "数据库错误")
		return
	}
	respondData(c, stats)
}

func StatsModels(c *gin.Context) {
	days, _ := strconv.Atoi(c.DefaultQuery("days", "30"))
	if days < 1 || days > 731 {
		days = 30
	}
	stats, err := model.GetModelStats(statsUserId(c), days)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "数据库错误")
		return
	}
	respondData(c, stats)
}
