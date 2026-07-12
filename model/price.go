package model

import (
	"math"
	"strings"
)

// ModelPrice holds USD prices per 1M tokens for a model.
// Model may end with "*" for prefix matching, e.g. "claude-3-5*".
type ModelPrice struct {
	Id          int     `gorm:"primaryKey" json:"id"`
	Model       string  `gorm:"uniqueIndex;size:128" json:"model"`
	InputPrice  float64 `json:"input_price"`  // USD per 1M prompt tokens
	OutputPrice float64 `json:"output_price"` // USD per 1M completion tokens
}

func GetAllPrices() ([]ModelPrice, error) {
	var prices []ModelPrice
	err := DB.Order("model asc").Find(&prices).Error
	return prices, err
}

func CreatePrice(price *ModelPrice) error {
	return DB.Create(price).Error
}

func UpdatePrice(price *ModelPrice) error {
	return DB.Model(price).Select("model", "input_price", "output_price").Updates(price).Error
}

func DeletePrice(id int) error {
	return DB.Delete(&ModelPrice{}, id).Error
}

// MatchPrice finds the price entry for a model: exact match first, then
// the longest matching "prefix*" wildcard. Returns nil when nothing matches.
func MatchPrice(model string) (*ModelPrice, error) {
	prices, err := GetAllPrices()
	if err != nil {
		return nil, err
	}
	var best *ModelPrice
	bestLen := -1
	for i := range prices {
		p := &prices[i]
		if p.Model == model {
			return p, nil
		}
		if prefix, ok := strings.CutSuffix(p.Model, "*"); ok &&
			strings.HasPrefix(model, prefix) && len(prefix) > bestLen {
			best = p
			bestLen = len(prefix)
		}
	}
	return best, nil
}

// ComputeCostMicros converts token counts into micro-dollars.
// Price in USD per 1M tokens equals micro-dollars per token, so the
// product needs no further unit conversion.
func ComputeCostMicros(price *ModelPrice, promptTokens, completionTokens int) int64 {
	if price == nil {
		return 0
	}
	cost := float64(promptTokens)*price.InputPrice + float64(completionTokens)*price.OutputPrice
	if math.IsNaN(cost) || cost < 0 {
		return 0
	}
	return int64(math.Round(cost))
}
