package relay

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

	"mochi-api/common"
	"mochi-api/model"
)

// setupRelayDB points the model package at a fresh temp SQLite file.
func setupRelayDB(t *testing.T) {
	t.Helper()
	common.DataDir = t.TempDir()
	require.NoError(t, model.InitDB())
	t.Cleanup(func() {
		// Close the pooled connection so TempDir cleanup works on Windows.
		if sqlDB, err := model.DB.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
}

func relayRequest(t *testing.T, body string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	Handle(c, FormatOpenAI)
	return recorder
}

func TestHandleFailsOverToHealthyChannel(t *testing.T) {
	setupRelayDB(t)

	var badHits, goodHits int
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		badHits++
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":{"message":"boom"}}`))
	}))
	defer bad.Close()
	good := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		goodHits++
		require.Equal(t, "/v1/chat/completions", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"1","object":"chat.completion","model":"test-model",` +
			`"choices":[{"index":0,"message":{"role":"assistant","content":"hi"},"finish_reason":"stop"}],` +
			`"usage":{"prompt_tokens":3,"completion_tokens":2}}`))
	}))
	defer good.Close()

	badCh := &model.Channel{Name: "坏渠道", Type: model.ChannelTypeOpenAI, BaseURL: bad.URL,
		Models: "test-model", Priority: 10, Status: model.StatusEnabled}
	goodCh := &model.Channel{Name: "好渠道", Type: model.ChannelTypeOpenAI, BaseURL: good.URL,
		Models: "test-model", Priority: 0, Status: model.StatusEnabled}
	require.NoError(t, model.CreateChannel(badCh))
	require.NoError(t, model.CreateChannel(goodCh))
	t.Cleanup(func() {
		markChannelSuccess(badCh.Id)
		markChannelSuccess(goodCh.Id)
	})

	recorder := relayRequest(t, `{"model":"test-model","messages":[{"role":"user","content":"hi"}]}`)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, "hi", gjson.Get(recorder.Body.String(), "choices.0.message.content").String())
	require.Equal(t, 1, badHits, "high-priority channel should be tried first")
	require.Equal(t, 1, goodHits, "request should fail over to the healthy channel")
	require.True(t, channelCoolingDown(badCh.Id), "failed channel should enter cooldown")
	require.False(t, channelCoolingDown(goodCh.Id))

	// Both the failed attempt and the final success are logged with the
	// channel name for observability.
	logs, total, err := model.GetLogs(model.LogQuery{})
	require.NoError(t, err)
	require.EqualValues(t, 2, total)
	byName := map[string]int{}
	for _, l := range logs {
		byName[l.ChannelName] = l.Code
	}
	require.Equal(t, http.StatusInternalServerError, byName["坏渠道"])
	require.Equal(t, http.StatusOK, byName["好渠道"])

	// While the bad channel cools down, the next request goes straight to
	// the healthy one.
	recorder = relayRequest(t, `{"model":"test-model","messages":[{"role":"user","content":"hi"}]}`)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, 1, badHits, "cooling channel should be skipped")
	require.Equal(t, 2, goodHits)
}

func TestHandleNonRetriableErrorIsForwarded(t *testing.T) {
	setupRelayDB(t)

	var hits int
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits++
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"max_tokens is too large","type":"invalid_request_error"}}`))
	}))
	defer upstream.Close()

	ch := &model.Channel{Name: "ch", Type: model.ChannelTypeOpenAI, BaseURL: upstream.URL,
		Models: "test-model", Priority: 0, Status: model.StatusEnabled}
	require.NoError(t, model.CreateChannel(ch))
	t.Cleanup(func() { markChannelSuccess(ch.Id) })

	recorder := relayRequest(t, `{"model":"test-model","messages":[{"role":"user","content":"hi"}]}`)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Equal(t, 1, hits)
	require.False(t, channelCoolingDown(ch.Id), "request-level errors must not cool the channel down")
}

func TestHandleFullPrefixBaseURL(t *testing.T) {
	setupRelayDB(t)

	var gotPath string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"1","object":"chat.completion","model":"test-model",` +
			`"choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],` +
			`"usage":{"prompt_tokens":1,"completion_tokens":1}}`))
	}))
	defer upstream.Close()

	ch := &model.Channel{Name: "prefix", Type: model.ChannelTypeOpenAI,
		BaseURL: upstream.URL + "/api/v4/", Models: "test-model", Status: model.StatusEnabled}
	require.NoError(t, model.CreateChannel(ch))
	t.Cleanup(func() { markChannelSuccess(ch.Id) })

	recorder := relayRequest(t, `{"model":"test-model","messages":[{"role":"user","content":"hi"}]}`)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, "/api/v4/chat/completions", gotPath,
		"trailing-slash base URL must skip the /v1 version segment")
}
