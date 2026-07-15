package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConfiguredModelsIncludeDisabledChannelsAndDeduplicate(t *testing.T) {
	setupTestDB(t)
	require.NoError(t, CreateChannel(&Channel{
		Name: "enabled", Type: ChannelTypeOpenAI, BaseURL: "https://enabled.example",
		Models: "model-b, model-a", Status: StatusEnabled,
	}))
	require.NoError(t, CreateChannel(&Channel{
		Name: "disabled", Type: ChannelTypeOpenAI, BaseURL: "https://disabled.example",
		Models: "model-c, model-a", Status: StatusDisabled,
	}))

	configured, err := GetConfiguredModels()
	require.NoError(t, err)
	require.Equal(t, []string{"model-a", "model-b", "model-c"}, configured)

	enabled, err := GetEnabledModels()
	require.NoError(t, err)
	require.Equal(t, []string{"model-a", "model-b"}, enabled)
}

func TestChannelResponsesModeDefaultsAndPersists(t *testing.T) {
	setupTestDB(t)
	channel := &Channel{
		Name: "chat-only", Type: ChannelTypeOpenAI, BaseURL: "https://example.com",
		Models: "model-a", Status: StatusEnabled,
	}
	require.NoError(t, CreateChannel(channel))

	stored, err := GetChannelById(channel.Id)
	require.NoError(t, err)
	require.Equal(t, ChannelResponsesModeChat, stored.ResponsesMode)
	require.False(t, stored.UsesNativeResponses())

	stored.ResponsesMode = ChannelResponsesModeNative
	require.NoError(t, UpdateChannel(stored))
	updated, err := GetChannelById(channel.Id)
	require.NoError(t, err)
	require.Equal(t, ChannelResponsesModeNative, updated.ResponsesMode)
	require.True(t, updated.UsesNativeResponses())
}

func TestChannelResponsesModeBackfillOnMigrate(t *testing.T) {
	setupTestDB(t)
	channel := &Channel{
		Name: "legacy", Type: ChannelTypeOpenAI, BaseURL: "https://example.com",
		Models: "model-a", Status: StatusEnabled,
	}
	require.NoError(t, CreateChannel(channel))
	require.NoError(t, DB.Model(&Channel{}).
		Where("id = ?", channel.Id).
		Update("responses_mode", "").Error)

	if sqlDB, err := DB.DB(); err == nil {
		_ = sqlDB.Close()
	}
	require.NoError(t, InitDB())

	stored, err := GetChannelById(channel.Id)
	require.NoError(t, err)
	require.Equal(t, ChannelResponsesModeChat, stored.ResponsesMode)
}
