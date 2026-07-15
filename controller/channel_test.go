package controller

import (
	"testing"

	"github.com/stretchr/testify/require"

	"mochi-api/model"
)

func TestChannelRequestResponsesModeValidation(t *testing.T) {
	validRequest := func() channelRequest {
		return channelRequest{
			Name: "test", Type: model.ChannelTypeOpenAI,
			BaseURL: "https://example.com", Models: "model-a",
		}
	}

	req := validRequest()
	require.Empty(t, req.validate())
	require.Equal(t, model.ChannelResponsesModeChat, req.ResponsesMode)

	req = validRequest()
	req.ResponsesMode = model.ChannelResponsesModeNative
	require.Empty(t, req.validate())
	require.Equal(t, model.ChannelResponsesModeNative, req.ResponsesMode)

	req = validRequest()
	req.ResponsesMode = "partial"
	require.Equal(t, "Responses 模式必须是 chat 或 native", req.validate())

	req = validRequest()
	req.Type = model.ChannelTypeAnthropic
	req.ResponsesMode = model.ChannelResponsesModeNative
	require.Empty(t, req.validate())
	require.Equal(t, model.ChannelResponsesModeChat, req.ResponsesMode)
}
