package model

import (
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"mochi-api/common"
)

var DB *gorm.DB

// InitDB opens the SQLite database and migrates all tables.
func InitDB() error {
	db, err := gorm.Open(sqlite.Open(common.SQLitePath()), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return err
	}
	DB = db
	if err := DB.AutoMigrate(
		&User{},
		&Token{},
		&Channel{},
		&ModelPrice{},
		&Log{},
		&Option{},
		&InviteCode{},
	); err != nil {
		return err
	}
	// Rows that existed before the status column was added may hold the
	// zero value instead of the column default; normalize them once.
	if err := DB.Model(&User{}).
		Where("status IS NULL OR status = 0").
		Update("status", StatusEnabled).Error; err != nil {
		return err
	}
	// Existing OpenAI-compatible channels predate explicit Responses
	// capabilities. Default them to the safer Chat conversion path.
	return DB.Model(&Channel{}).
		Where("responses_mode IS NULL OR responses_mode = ''").
		Update("responses_mode", ChannelResponsesModeChat).Error
}
