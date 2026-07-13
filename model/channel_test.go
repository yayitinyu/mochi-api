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
