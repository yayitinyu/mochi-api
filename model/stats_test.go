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

	users, err := GetUserStats(30)
	require.NoError(t, err)
	require.NotNil(t, users)
	require.Empty(t, users)
}

func TestGetUserStatsAggregatesAndOrders(t *testing.T) {
	setupTestDB(t)

	alice := &User{Username: "alice", Password: "hash", Role: RoleUser, Status: StatusEnabled}
	require.NoError(t, CreateUser(alice))
	bob := &User{Username: "bob", Password: "hash", Role: RoleUser, Status: StatusEnabled}
	require.NoError(t, CreateUser(bob))

	require.NoError(t, RecordLog(&Log{UserId: alice.Id, ModelName: "m", PromptTokens: 10, CompletionTokens: 5, CostMicros: 100}))
	require.NoError(t, RecordLog(&Log{UserId: alice.Id, ModelName: "m", PromptTokens: 20, CompletionTokens: 15, CostMicros: 300}))
	require.NoError(t, RecordLog(&Log{UserId: bob.Id, ModelName: "m", PromptTokens: 1, CompletionTokens: 1, CostMicros: 900}))
	// Logs of a deleted user must survive with an empty username.
	require.NoError(t, RecordLog(&Log{UserId: 999, ModelName: "m", PromptTokens: 2, CompletionTokens: 2, CostMicros: 50}))

	stats, err := GetUserStats(30)
	require.NoError(t, err)
	require.Len(t, stats, 3)

	// Ordered by cost desc: bob (900), alice (400), deleted user (50).
	require.Equal(t, bob.Id, stats[0].UserId)
	require.Equal(t, "bob", stats[0].Username)
	require.EqualValues(t, 900, stats[0].CostMicros)

	require.Equal(t, alice.Id, stats[1].UserId)
	require.EqualValues(t, 2, stats[1].Requests)
	require.EqualValues(t, 30, stats[1].PromptTokens)
	require.EqualValues(t, 20, stats[1].CompletionTokens)
	require.EqualValues(t, 400, stats[1].CostMicros)

	require.Equal(t, 999, stats[2].UserId)
	require.Equal(t, "", stats[2].Username)
}
