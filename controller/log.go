package controller

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"mochi-api/model"
)

// statsUserId returns the user filter for log/stats queries:
// admins see everything (0), regular users see only their own data.
func statsUserId(c *gin.Context) int {
	if c.GetInt("role") >= model.RoleAdmin {
		return 0
	}
	return c.GetInt("id")
}

func ListLogs(c *gin.Context) {
	page, _ := strconv.Atoi(c.Query("page"))
	pageSize, _ := strconv.Atoi(c.Query("page_size"))
	code, _ := strconv.Atoi(c.Query("code"))
	start, _ := strconv.ParseInt(c.Query("start"), 10, 64)
	end, _ := strconv.ParseInt(c.Query("end"), 10, 64)
	logs, total, err := model.GetLogs(model.LogQuery{
		UserId:    statsUserId(c),
		Model:     c.Query("model"),
		TokenName: c.Query("token_name"),
		Code:      code,
		Start:     start,
		End:       end,
		Page:      page,
		PageSize:  pageSize,
	})
	if err != nil {
		respondError(c, http.StatusInternalServerError, "数据库错误")
		return
	}
	respondData(c, gin.H{"items": logs, "total": total})
}
