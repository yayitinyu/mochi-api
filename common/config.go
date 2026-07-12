package common

import (
	"os"
	"path/filepath"
)

var (
	// Port is the HTTP listen port.
	Port = getEnv("PORT", "3000")
	// DataDir holds the SQLite database file.
	DataDir = getEnv("MOCHI_DATA", ".")
)

// SQLitePath returns the full path of the SQLite database file.
func SQLitePath() string {
	return filepath.Join(DataDir, "mochi.db")
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
