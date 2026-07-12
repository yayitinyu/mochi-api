package model

import (
	"testing"

	"github.com/stretchr/testify/require"

	"mochi-api/common"
)

// Regression: on an empty database the Scan-based stats aggregations must
// return empty (non-nil) slices. A nil slice serializes as JSON null and
// crashed the dashboard on fresh deployments.
func TestStatsReturnEmptySlicesOnEmptyDatabase(t *testing.T) {
	common.DataDir = t.TempDir()
	require.NoError(t, InitDB())
	t.Cleanup(func() {
		// Close the pooled connection so TempDir cleanup works on Windows.
		if sqlDB, err := DB.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})

	daily, err := GetDailyStats(0, 365)
	require.NoError(t, err)
	require.NotNil(t, daily)
	require.Empty(t, daily)

	models, err := GetModelStats(0, 30)
	require.NoError(t, err)
	require.NotNil(t, models)
	require.Empty(t, models)
}
