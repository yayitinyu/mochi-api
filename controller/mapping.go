package controller

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"mochi-api/model"
)

func ListModelMappings(c *gin.Context) {
	mappings, err := model.GetAllModelMappings()
	if err != nil {
		respondError(c, http.StatusInternalServerError, "数据库错误")
		return
	}
	respondData(c, mappings)
}

func CreateModelMapping(c *gin.Context) {
	var m model.ModelMapping
	if err := c.ShouldBindJSON(&m); err != nil {
		respondError(c, http.StatusBadRequest, "请求格式错误")
		return
	}
	if msg := validateMapping(&m); msg != "" {
		respondError(c, http.StatusBadRequest, msg)
		return
	}
	m.CreatedAt = time.Now().Unix()
	if err := model.CreateModelMapping(&m); err != nil {
		respondError(c, http.StatusInternalServerError, "创建失败")
		return
	}
	respondData(c, m)
}

func UpdateModelMapping(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var m model.ModelMapping
	if err := c.ShouldBindJSON(&m); err != nil {
		respondError(c, http.StatusBadRequest, "请求格式错误")
		return
	}
	if msg := validateMapping(&m); msg != "" {
		respondError(c, http.StatusBadRequest, msg)
		return
	}
	m.Id = id
	if err := model.UpdateModelMapping(&m); err != nil {
		respondError(c, http.StatusInternalServerError, "更新失败")
		return
	}
	respondData(c, m)
}

func DeleteModelMapping(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := model.DeleteModelMapping(id); err != nil {
		respondError(c, http.StatusInternalServerError, "删除失败")
		return
	}
	respondData(c, nil)
}

func validateMapping(m *model.ModelMapping) string {
	m.Alias = strings.TrimSpace(m.Alias)
	m.UpstreamName = strings.TrimSpace(m.UpstreamName)
	if m.Alias == "" {
		return "别名不能为空"
	}
	if m.UpstreamName == "" {
		return "上游模型名称不能为空"
	}
	if strings.Contains(m.Alias, ",") {
		return "别名不能包含逗号"
	}

	upstreams := model.ParseModelList(m.UpstreamName)
	if len(upstreams) == 0 {
		return "上游模型名称不能为空"
	}

	// Normalize the comma-separated string formatting (e.g. "modelA, modelB")
	m.UpstreamName = strings.Join(upstreams, ", ")

	for _, name := range upstreams {
		if name == m.Alias {
			return "别名不能与任一上游模型名称相同"
		}
	}
	return ""
}
