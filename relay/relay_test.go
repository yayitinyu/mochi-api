package relay

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"mochi-api/model"
)

func TestUpstreamTargetBaseURLConventions(t *testing.T) {
	tests := []struct {
		name    string
		chType  string
		baseURL string
		format  Format
		stream  bool
		want    string
	}{
		{
			name:   "openai standard",
			chType: model.ChannelTypeOpenAI, baseURL: "https://api.openai.com",
			format: FormatOpenAI,
			want:   "https://api.openai.com/v1/chat/completions",
		},
		{
			name:   "openai full prefix keeps custom version segment",
			chType: model.ChannelTypeOpenAI, baseURL: "https://open.bigmodel.cn/api/paas/v4/",
			format: FormatOpenAI,
			want:   "https://open.bigmodel.cn/api/paas/v4/chat/completions",
		},
		{
			name:   "openai full prefix without version segment",
			chType: model.ChannelTypeOpenAI, baseURL: "https://api.perplexity.ai/",
			format: FormatOpenAI,
			want:   "https://api.perplexity.ai/chat/completions",
		},
		{
			name:   "openai exact endpoint",
			chType: model.ChannelTypeOpenAI, baseURL: "https://example.com/weird/path/chat#",
			format: FormatOpenAI,
			want:   "https://example.com/weird/path/chat",
		},
		{
			name:   "claude standard",
			chType: model.ChannelTypeAnthropic, baseURL: "https://api.anthropic.com",
			format: FormatClaude,
			want:   "https://api.anthropic.com/v1/messages",
		},
		{
			name:   "claude subpath standard still appends v1",
			chType: model.ChannelTypeAnthropic, baseURL: "https://api.deepseek.com/anthropic",
			format: FormatClaude,
			want:   "https://api.deepseek.com/anthropic/v1/messages",
		},
		{
			name:   "responses full prefix",
			chType: model.ChannelTypeOpenAI, baseURL: "https://proxy.example.com/api/",
			format: FormatResponses,
			want:   "https://proxy.example.com/api/responses",
		},
		{
			name:   "image generation full prefix",
			chType: model.ChannelTypeOpenAI, baseURL: "https://ark.example.com/api/v3/",
			format: FormatImage,
			want:   "https://ark.example.com/api/v3/images/generations",
		},
		{
			name:   "gemini standard",
			chType: model.ChannelTypeGemini, baseURL: "https://generativelanguage.googleapis.com",
			format: FormatGemini,
			want:   "https://generativelanguage.googleapis.com/v1beta/models/gemini-pro:generateContent",
		},
		{
			name:   "gemini full prefix skips v1beta",
			chType: model.ChannelTypeGemini, baseURL: "https://gw.example.com/gemini/",
			format: FormatGemini,
			want:   "https://gw.example.com/gemini/models/gemini-pro:generateContent",
		},
		{
			name:   "gemini stream action",
			chType: model.ChannelTypeGemini, baseURL: "https://generativelanguage.googleapis.com",
			format: FormatGemini, stream: true,
			want: "https://generativelanguage.googleapis.com/v1beta/models/gemini-pro:streamGenerateContent?alt=sse",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rc := &relayContext{
				channel:        &model.Channel{Type: tt.chType, BaseURL: tt.baseURL, ApiKey: "k"},
				upstreamFormat: tt.format,
				modelName:      "gemini-pro",
				upstreamModel:  "gemini-pro",
				stream:         tt.stream,
			}
			url, _, _ := upstreamTarget(rc)
			require.Equal(t, tt.want, url)
		})
	}
}

func TestUpstreamFormatForResponsesCapability(t *testing.T) {
	tests := []struct {
		name         string
		channel      model.Channel
		clientFormat Format
		want         Format
	}{
		{
			name:         "legacy OpenAI channel defaults to Chat conversion",
			channel:      model.Channel{Type: model.ChannelTypeOpenAI},
			clientFormat: FormatResponses,
			want:         FormatOpenAI,
		},
		{
			name: "explicit Chat mode converts Responses",
			channel: model.Channel{
				Type: model.ChannelTypeOpenAI, ResponsesMode: model.ChannelResponsesModeChat,
			},
			clientFormat: FormatResponses,
			want:         FormatOpenAI,
		},
		{
			name: "explicit native mode forwards Responses",
			channel: model.Channel{
				Type: model.ChannelTypeOpenAI, ResponsesMode: model.ChannelResponsesModeNative,
			},
			clientFormat: FormatResponses,
			want:         FormatResponses,
		},
		{
			name: "native Responses capability does not change Chat requests",
			channel: model.Channel{
				Type: model.ChannelTypeOpenAI, ResponsesMode: model.ChannelResponsesModeNative,
			},
			clientFormat: FormatOpenAI,
			want:         FormatOpenAI,
		},
		{
			name:         "Anthropic keeps native wire format",
			channel:      model.Channel{Type: model.ChannelTypeAnthropic},
			clientFormat: FormatResponses,
			want:         FormatClaude,
		},
		{
			name:         "Gemini keeps native wire format",
			channel:      model.Channel{Type: model.ChannelTypeGemini},
			clientFormat: FormatResponses,
			want:         FormatGemini,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, upstreamFormatFor(&tt.channel, tt.clientFormat))
		})
	}
}

func TestModelsURLFromExact(t *testing.T) {
	require.Equal(t, "https://x.com/api/models",
		modelsURLFromExact("https://x.com/api/chat/completions"))
	require.Equal(t, "https://x.com/api/models",
		modelsURLFromExact("https://x.com/api/messages"))
	require.Equal(t, "https://x.com/api/models",
		modelsURLFromExact("https://x.com/api"))
	require.Equal(t, "https://x.com/v1/models",
		modelsURLFromExact("https://x.com/v1/images/generations"))
}

func TestXAIImageModelsURL(t *testing.T) {
	require.Equal(t, "https://api.x.ai/v1/image-generation-models",
		xAIImageModelsURL("https://api.x.ai"))
	require.Equal(t, "", xAIImageModelsURL("https://proxy.example.com"))
}

func TestRetriableStatus(t *testing.T) {
	for _, code := range []int{401, 403, 404, 408, 429, 500, 502, 503, 529} {
		require.True(t, retriableStatus(code), "code %d should be retriable", code)
	}
	for _, code := range []int{400, 402, 409, 422} {
		require.False(t, retriableStatus(code), "code %d should not be retriable", code)
	}
}

func TestOrderChannelsMovesCoolingChannelsBack(t *testing.T) {
	channels := []model.Channel{
		{Id: 1, Priority: 10},
		{Id: 2, Priority: 10},
		{Id: 3, Priority: 0},
	}
	markChannelFailure(1)
	t.Cleanup(func() { markChannelSuccess(1) })

	ordered := orderChannels(channels)
	require.Len(t, ordered, 3)
	// Channel 1 is cooling down and must come last regardless of priority.
	require.Equal(t, 1, ordered[2].Id)

	markChannelSuccess(1)
	require.False(t, channelCoolingDown(1))
}

func TestChannelCooldownExpires(t *testing.T) {
	channelCooldowns.Store(99, time.Now().Add(-time.Second))
	require.False(t, channelCoolingDown(99))
	// The expired entry is cleaned up on read.
	_, ok := channelCooldowns.Load(99)
	require.False(t, ok)
}
