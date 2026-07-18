package relay

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

	"mochi-api/model"
)

func imageRequest(t *testing.T, body string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	HandleImageGeneration(c)
	return recorder
}

func TestHandleImageGenerationOpenAICompatiblePassthrough(t *testing.T) {
	setupRelayDB(t)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/images/generations", r.URL.Path)
		require.Equal(t, "Bearer upstream-key", r.Header.Get("Authorization"))
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.Equal(t, "gpt-image-test", gjson.GetBytes(body, "model").String())
		require.Equal(t, "draw a sakura", gjson.GetBytes(body, "prompt").String())
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"created":1,"data":[{"b64_json":"aW1hZ2U="}],` +
			`"usage":{"input_tokens":7,"output_tokens":11,"total_tokens":18}}`))
	}))
	defer upstream.Close()

	channel := &model.Channel{
		Name: "OpenAI image", Type: model.ChannelTypeOpenAI, BaseURL: upstream.URL,
		ApiKey: "upstream-key", Models: "gpt-image-test", Status: model.StatusEnabled,
	}
	require.NoError(t, model.CreateChannel(channel))
	t.Cleanup(func() { markChannelSuccess(channel.Id) })

	recorder := imageRequest(t, `{"model":"gpt-image-test","prompt":"draw a sakura"}`)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, "aW1hZ2U=", gjson.Get(recorder.Body.String(), "data.0.b64_json").String())
	logs, total, err := model.GetLogs(model.LogQuery{})
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Equal(t, 7, logs[0].PromptTokens)
	require.Equal(t, 11, logs[0].CompletionTokens)
}

func TestHandleImageGenerationGeminiConversion(t *testing.T) {
	setupRelayDB(t)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1beta/models/gemini-image-test:generateContent", r.URL.Path)
		require.Equal(t, "gemini-key", r.Header.Get("x-goog-api-key"))
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.Equal(t, "draw a mochi", gjson.GetBytes(body, "contents.0.parts.0.text").String())
		require.Equal(t, "IMAGE", gjson.GetBytes(body,
			"generationConfig.responseModalities.0").String())
		require.Equal(t, "3:2", gjson.GetBytes(body,
			"generationConfig.responseFormat.image.aspectRatio").String())
		require.Equal(t, "2K", gjson.GetBytes(body,
			"generationConfig.responseFormat.image.imageSize").String())
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"candidates":[{"content":{"parts":[
				{"thought":true,"inlineData":{"mimeType":"image/jpeg","data":"dGhvdWdodA=="}},
				{"inlineData":{"mimeType":"image/jpeg","data":"aW1hZ2U="}}
			]}}],
			"usageMetadata":{"promptTokenCount":5,"candidatesTokenCount":9,"thoughtsTokenCount":2}
		}`))
	}))
	defer upstream.Close()

	channel := &model.Channel{
		Name: "Nano Banana", Type: model.ChannelTypeGemini, BaseURL: upstream.URL,
		ApiKey: "gemini-key", Models: "gemini-image-test", Status: model.StatusEnabled,
	}
	require.NoError(t, model.CreateChannel(channel))
	t.Cleanup(func() { markChannelSuccess(channel.Id) })

	recorder := imageRequest(t, `{
		"model":"gemini-image-test",
		"prompt":"draw a mochi",
		"size":"1536x1024",
		"response_format":"url"
	}`)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, "data:image/jpeg;base64,aW1hZ2U=",
		gjson.Get(recorder.Body.String(), "data.0.url").String())
	require.Equal(t, int64(5), gjson.Get(recorder.Body.String(), "usage.input_tokens").Int())
	require.Equal(t, int64(11), gjson.Get(recorder.Body.String(), "usage.output_tokens").Int())
}

func TestHandleImageGenerationGeminiStream(t *testing.T) {
	setupRelayDB(t)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1beta/models/gemini-image-test:streamGenerateContent", r.URL.Path)
		require.Equal(t, "sse", r.URL.Query().Get("alt"))
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"candidates\":[{\"content\":{\"parts\":[{\"inlineData\":{\"mimeType\":\"image/png\",\"data\":\"aW1hZ2U=\"}}]}}],\"usageMetadata\":{\"promptTokenCount\":3,\"candidatesTokenCount\":8}}\n\n"))
	}))
	defer upstream.Close()

	channel := &model.Channel{
		Name: "Nano Banana", Type: model.ChannelTypeGemini, BaseURL: upstream.URL,
		ApiKey: "gemini-key", Models: "gemini-image-test", Status: model.StatusEnabled,
	}
	require.NoError(t, model.CreateChannel(channel))
	t.Cleanup(func() { markChannelSuccess(channel.Id) })

	recorder := imageRequest(t, `{"model":"gemini-image-test","prompt":"draw","stream":true}`)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Contains(t, recorder.Header().Get("Content-Type"), "text/event-stream")
	require.Contains(t, recorder.Body.String(), "event: image_generation.completed")
	require.Contains(t, recorder.Body.String(), `"b64_json":"aW1hZ2U="`)
}

func TestConvertImageRequestToGeminiRejectsMultipleImages(t *testing.T) {
	_, err := convertImageRequestToGemini(
		[]byte(`{"model":"gemini-image","prompt":"draw","n":2}`),
		"gemini-image",
	)
	require.EqualError(t, err, "Gemini 图片渠道暂不支持 n 大于 1")
}

func TestGemini25ImageSizeUsesFixedNativeResolution(t *testing.T) {
	body, err := convertImageRequestToGemini(
		[]byte(`{"model":"gemini-2.5-flash-image","prompt":"draw","size":"1536x1024"}`),
		"gemini-2.5-flash-image",
	)
	require.NoError(t, err)
	require.Equal(t, "3:2", gjson.GetBytes(body,
		"generationConfig.responseFormat.image.aspectRatio").String())
	require.False(t, gjson.GetBytes(body,
		"generationConfig.responseFormat.image.imageSize").Exists())
}
