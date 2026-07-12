package controller

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"mochi-api/model"
)

type priceRequest struct {
	Model       string  `json:"model"`
	InputPrice  float64 `json:"input_price"`
	OutputPrice float64 `json:"output_price"`
}

func (req *priceRequest) validate() string {
	req.Model = strings.TrimSpace(req.Model)
	if req.Model == "" || len(req.Model) > 128 {
		return "模型名不能为空且不超过 128 个字符"
	}
	if req.InputPrice < 0 || req.OutputPrice < 0 {
		return "价格不能为负数"
	}
	return ""
}

func ListPrices(c *gin.Context) {
	prices, err := model.GetAllPrices()
	if err != nil {
		respondError(c, http.StatusInternalServerError, "数据库错误")
		return
	}
	respondData(c, prices)
}

func CreatePrice(c *gin.Context) {
	var req priceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "请求格式错误")
		return
	}
	if msg := req.validate(); msg != "" {
		respondError(c, http.StatusBadRequest, msg)
		return
	}
	price := &model.ModelPrice{
		Model: req.Model, InputPrice: req.InputPrice, OutputPrice: req.OutputPrice,
	}
	if err := model.CreatePrice(price); err != nil {
		respondError(c, http.StatusBadRequest, "创建失败，模型名可能已存在")
		return
	}
	respondData(c, price)
}

func UpdatePrice(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var req priceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "请求格式错误")
		return
	}
	if msg := req.validate(); msg != "" {
		respondError(c, http.StatusBadRequest, msg)
		return
	}
	price := &model.ModelPrice{
		Id: id, Model: req.Model, InputPrice: req.InputPrice, OutputPrice: req.OutputPrice,
	}
	if err := model.UpdatePrice(price); err != nil {
		respondError(c, http.StatusBadRequest, "更新失败，模型名可能已存在")
		return
	}
	respondData(c, price)
}

func DeletePrice(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := model.DeletePrice(id); err != nil {
		respondError(c, http.StatusInternalServerError, "删除失败")
		return
	}
	respondData(c, nil)
}
