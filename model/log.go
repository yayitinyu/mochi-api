package model

import (
	"time"
)

type Log struct {
	Id               int    `gorm:"primaryKey" json:"id"`
	UserId           int    `gorm:"index:idx_user_time,priority:1" json:"user_id"`
	CreatedAt        int64  `gorm:"index:idx_user_time,priority:2" json:"created_at"`
	Day              string `gorm:"index;size:10" json:"day"` // "2026-07-12", server-local
	TokenName        string `json:"token_name"`
	ChannelId        int    `json:"channel_id"`
	ModelName        string `gorm:"index;size:128" json:"model_name"`
	PromptTokens     int    `json:"prompt_tokens"`
	CompletionTokens int    `json:"completion_tokens"`
	CostMicros       int64  `json:"cost_micros"` // micro-dollars
	UseTimeMs        int    `json:"use_time_ms"`
	IsStream         bool   `json:"is_stream"`
	Code             int    `json:"code"` // upstream HTTP status
}

// RecordLog stamps CreatedAt/Day and inserts the row.
func RecordLog(log *Log) error {
	now := time.Now()
	log.CreatedAt = now.Unix()
	log.Day = now.Format("2006-01-02")
	return DB.Create(log).Error
}

type LogQuery struct {
	UserId    int // 0 = all users (admin)
	Model     string
	TokenName string
	Code      int   // 0 = all
	Start     int64 // unix seconds, 0 = unbounded
	End       int64
	Page      int
	PageSize  int
}

func GetLogs(q LogQuery) ([]Log, int64, error) {
	tx := DB.Model(&Log{})
	if q.UserId > 0 {
		tx = tx.Where("user_id = ?", q.UserId)
	}
	if q.Model != "" {
		tx = tx.Where("model_name = ?", q.Model)
	}
	if q.TokenName != "" {
		tx = tx.Where("token_name = ?", q.TokenName)
	}
	if q.Code > 0 {
		tx = tx.Where("code = ?", q.Code)
	}
	if q.Start > 0 {
		tx = tx.Where("created_at >= ?", q.Start)
	}
	if q.End > 0 {
		tx = tx.Where("created_at <= ?", q.End)
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if q.Page < 1 {
		q.Page = 1
	}
	if q.PageSize < 1 || q.PageSize > 100 {
		q.PageSize = 20
	}
	var logs []Log
	err := tx.Order("id desc").Offset((q.Page - 1) * q.PageSize).Limit(q.PageSize).Find(&logs).Error
	return logs, total, err
}
