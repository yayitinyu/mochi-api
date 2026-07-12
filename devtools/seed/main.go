// Command seed inserts backdated demo logs so the dashboard heatmap and
// trend charts have data to render during local verification.
// Run: go run ./devtools/seed
package main

import (
	"math/rand"
	"time"

	"mochi-api/model"
)

func main() {
	if err := model.InitDB(); err != nil {
		panic(err)
	}
	models := []struct {
		name              string
		inPrice, outPrice float64
	}{
		{"gpt-4o", 2.5, 10},
		{"gpt-4o-mini", 0.15, 0.6},
		{"claude-3-5-sonnet", 3, 15},
	}
	rng := rand.New(rand.NewSource(42))
	now := time.Now()
	inserted := 0
	for daysAgo := 0; daysAgo < 300; daysAgo++ {
		day := now.AddDate(0, 0, -daysAgo)
		// Skip ~40% of days to create a natural, uneven heatmap.
		if rng.Float64() < 0.4 {
			continue
		}
		count := rng.Intn(30) + 1
		for i := 0; i < count; i++ {
			m := models[rng.Intn(len(models))]
			prompt := rng.Intn(3000) + 100
			completion := rng.Intn(1500) + 50
			ts := time.Date(day.Year(), day.Month(), day.Day(),
				rng.Intn(24), rng.Intn(60), 0, 0, day.Location())
			cost := int64(float64(prompt)*m.inPrice + float64(completion)*m.outPrice)
			log := &model.Log{
				UserId:           1,
				CreatedAt:        ts.Unix(),
				Day:              ts.Format("2006-01-02"),
				TokenName:        "demo-key",
				ChannelId:        1,
				ModelName:        m.name,
				PromptTokens:     prompt,
				CompletionTokens: completion,
				CostMicros:       cost,
				UseTimeMs:        rng.Intn(4000) + 200,
				IsStream:         rng.Float64() < 0.6,
				Code:             200,
			}
			if err := model.DB.Create(log).Error; err != nil {
				panic(err)
			}
			inserted++
		}
	}
	println("seeded", inserted, "logs")
}
