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
	return DB.AutoMigrate(
		&User{},
		&Token{},
		&Channel{},
		&ModelPrice{},
		&Log{},
		&Option{},
	)
}
