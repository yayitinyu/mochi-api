package model

import (
	"testing"

	"github.com/stretchr/testify/require"

	"mochi-api/common"
)

// setupTestDB points the package-level DB at a fresh temp SQLite file.
func setupTestDB(t *testing.T) {
	t.Helper()
	common.DataDir = t.TempDir()
	require.NoError(t, InitDB())
	t.Cleanup(func() {
		// Close the pooled connection so TempDir cleanup works on Windows.
		if sqlDB, err := DB.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
}
