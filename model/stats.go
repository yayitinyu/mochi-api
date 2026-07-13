package model

import "time"

type DailyStat struct {
	Day              string `json:"day"`
	Requests         int64  `json:"requests"`
	PromptTokens     int64  `json:"prompt_tokens"`
	CompletionTokens int64  `json:"completion_tokens"`
	CostMicros       int64  `json:"cost_micros"`
}

type ModelStat struct {
	ModelName        string `json:"model_name"`
	Requests         int64  `json:"requests"`
	PromptTokens     int64  `json:"prompt_tokens"`
	CompletionTokens int64  `json:"completion_tokens"`
	CostMicros       int64  `json:"cost_micros"`
}

type PeriodStat struct {
	Requests   int64 `json:"requests"`
	Tokens     int64 `json:"tokens"`
	CostMicros int64 `json:"cost_micros"`
}

type UserStat struct {
	UserId           int    `json:"user_id"`
	Username         string `json:"username"`
	Requests         int64  `json:"requests"`
	PromptTokens     int64  `json:"prompt_tokens"`
	CompletionTokens int64  `json:"completion_tokens"`
	CostMicros       int64  `json:"cost_micros"`
}

// GetDailyStats aggregates logs per day for the last `days` days.
// userId 0 means all users.
func GetDailyStats(userId, days int) ([]DailyStat, error) {
	since := time.Now().AddDate(0, 0, -days+1).Format("2006-01-02")
	tx := DB.Model(&Log{}).
		Select("day, COUNT(*) as requests, SUM(prompt_tokens) as prompt_tokens, "+
			"SUM(completion_tokens) as completion_tokens, SUM(cost_micros) as cost_micros").
		Where("day >= ?", since)
	if userId > 0 {
		tx = tx.Where("user_id = ?", userId)
	}
	// Scan (unlike Find) leaves a nil slice when no rows match, which
	// would serialize as JSON null and break list handling in the UI.
	stats := make([]DailyStat, 0)
	err := tx.Group("day").Order("day asc").Scan(&stats).Error
	return stats, err
}

// GetModelStats aggregates logs per model for the last `days` days.
func GetModelStats(userId, days int) ([]ModelStat, error) {
	since := time.Now().AddDate(0, 0, -days+1).Format("2006-01-02")
	tx := DB.Model(&Log{}).
		Select("model_name, COUNT(*) as requests, SUM(prompt_tokens) as prompt_tokens, "+
			"SUM(completion_tokens) as completion_tokens, SUM(cost_micros) as cost_micros").
		Where("day >= ?", since)
	if userId > 0 {
		tx = tx.Where("user_id = ?", userId)
	}
	stats := make([]ModelStat, 0)
	err := tx.Group("model_name").Order("cost_micros desc").Scan(&stats).Error
	return stats, err
}

// GetPeriodStat aggregates logs since the given unix timestamp.
func GetPeriodStat(userId int, since int64) (PeriodStat, error) {
	tx := DB.Model(&Log{}).
		Select("COUNT(*) as requests, "+
			"COALESCE(SUM(prompt_tokens + completion_tokens), 0) as tokens, "+
			"COALESCE(SUM(cost_micros), 0) as cost_micros").
		Where("created_at >= ?", since)
	if userId > 0 {
		tx = tx.Where("user_id = ?", userId)
	}
	var stat PeriodStat
	err := tx.Scan(&stat).Error
	return stat, err
}

// GetUserStats aggregates logs per user for the last `days` days,
// ordered by cost. Logs of deleted users are kept and surface with an
// empty username.
func GetUserStats(days int) ([]UserStat, error) {
	since := time.Now().AddDate(0, 0, -days+1).Format("2006-01-02")
	stats := make([]UserStat, 0)
	err := DB.Table("logs").
		Select("logs.user_id, COALESCE(users.username, '') as username, "+
			"COUNT(*) as requests, SUM(logs.prompt_tokens) as prompt_tokens, "+
			"SUM(logs.completion_tokens) as completion_tokens, SUM(logs.cost_micros) as cost_micros").
		Joins("LEFT JOIN users ON users.id = logs.user_id").
		Where("logs.day >= ?", since).
		Group("logs.user_id, users.username").
		Order("cost_micros desc").
		Scan(&stats).Error
	return stats, err
}
