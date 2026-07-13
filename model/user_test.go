package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDeleteUserCascadeRemovesTokensKeepsLogs(t *testing.T) {
	setupTestDB(t)

	user := &User{Username: "alice", Password: "hash", Role: RoleUser, Status: StatusEnabled}
	require.NoError(t, CreateUser(user))
	require.NoError(t, CreateToken(&Token{UserId: user.Id, Key: "k1", Name: "t1", Status: StatusEnabled}))
	require.NoError(t, RecordLog(&Log{UserId: user.Id, ModelName: "gpt-test", CostMicros: 100}))

	require.NoError(t, DeleteUserCascade(user.Id))

	tokens, err := GetTokensByUserId(user.Id)
	require.NoError(t, err)
	require.Empty(t, tokens)

	_, total, err := GetLogs(LogQuery{UserId: user.Id, Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.EqualValues(t, 1, total)

	count, err := CountUsers()
	require.NoError(t, err)
	require.EqualValues(t, 0, count)
}

func TestStatusBackfillOnMigrate(t *testing.T) {
	setupTestDB(t)

	user := &User{Username: "legacy", Password: "hash", Role: RoleUser}
	require.NoError(t, CreateUser(user))
	// Simulate a row created before the status column existed.
	require.NoError(t, DB.Exec("UPDATE users SET status = 0 WHERE id = ?", user.Id).Error)

	// Re-running InitDB (same data dir) must normalize the zero value.
	// Close the old pool first so the file handle is released on Windows.
	if sqlDB, err := DB.DB(); err == nil {
		_ = sqlDB.Close()
	}
	require.NoError(t, InitDB())

	fresh, err := GetUserById(user.Id)
	require.NoError(t, err)
	require.Equal(t, StatusEnabled, fresh.Status)
}

func TestIsUserEnabled(t *testing.T) {
	setupTestDB(t)

	enabled := &User{Username: "alice", Password: "hash", Role: RoleUser, Status: StatusEnabled}
	require.NoError(t, CreateUser(enabled))
	disabled := &User{Username: "bob", Password: "hash", Role: RoleUser, Status: StatusDisabled}
	require.NoError(t, CreateUser(disabled))

	ok, err := IsUserEnabled(enabled.Id)
	require.NoError(t, err)
	require.True(t, ok)

	ok, err = IsUserEnabled(disabled.Id)
	require.NoError(t, err)
	require.False(t, ok)

	ok, err = IsUserEnabled(99999)
	require.NoError(t, err)
	require.False(t, ok)
}
