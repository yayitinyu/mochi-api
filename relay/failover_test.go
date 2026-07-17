package relay

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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

// A client that gives up mid-request must not poison channel health: the
// canceled context fails every channel identically, so failing over (and
// cooling each one down) would take the whole fleet offline for 60s.
func TestHandleClientCancelDoesNotCoolChannels(t *testing.T) {
	setupRelayDB(t)

	release := make(chan struct{})
	var hits int
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits++
		<-release
		w.WriteHeader(http.StatusOK)
	}))
	defer slow.Close()
	defer close(release)

	chA := &model.Channel{Name: "a", Type: model.ChannelTypeOpenAI, BaseURL: slow.URL,
		Models: "test-model", Priority: 10, Status: model.StatusEnabled}
	chB := &model.Channel{Name: "b", Type: model.ChannelTypeOpenAI, BaseURL: slow.URL,
		Models: "test-model", Priority: 0, Status: model.StatusEnabled}
	require.NoError(t, model.CreateChannel(chA))
	require.NoError(t, model.CreateChannel(chB))
	t.Cleanup(func() {
		markChannelSuccess(chA.Id)
		markChannelSuccess(chB.Id)
	})

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	ctx, cancel := context.WithCancel(context.Background())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions",
		strings.NewReader(`{"model":"test-model","messages":[{"role":"user","content":"hi"}]}`)).
		WithContext(ctx)
	c.Request.Header.Set("Content-Type", "application/json")

	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()
	Handle(c, FormatOpenAI)

	require.Equal(t, 1, hits, "a canceled request must not fail over to further channels")
	require.False(t, channelCoolingDown(chA.Id), "client cancellation is not a channel failure")
	require.False(t, channelCoolingDown(chB.Id))

	logs, total, err := model.GetLogs(model.LogQuery{})
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Equal(t, statusClientClosedRequest, logs[0].Code)
}

// When a native-Responses channel rejects the Responses method and mochi falls
// back to Chat Completions, the retried request must carry the alias-resolved
// upstream model name, not the alias the client used.
func TestResponsesFallbackKeepsResolvedModel(t *testing.T) {
	setupRelayDB(t)

	var chatModel string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/responses" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":{"message":"Model does not support responses method.","type":"invalid_request_error"}}`))
			return
		}
		body, _ := io.ReadAll(r.Body)
		chatModel = gjson.GetBytes(body, "model").String()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id":"chatcmpl-1","created":1700000000,"model":"upstream-model",
			"choices":[{"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],
			"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}
		}`))
	}))
	defer upstream.Close()

	channel := &model.Channel{
		Name: "native", Type: model.ChannelTypeOpenAI, BaseURL: upstream.URL,
		Models: "upstream-model", ResponsesMode: model.ChannelResponsesModeNative,
		Status: model.StatusEnabled,
	}
	require.NoError(t, model.CreateChannel(channel))
	require.NoError(t, model.CreateModelMapping(&model.ModelMapping{
		Alias: "friendly-name", UpstreamName: "upstream-model",
	}))
	t.Cleanup(func() { markChannelSuccess(channel.Id) })

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses",
		strings.NewReader(`{"model":"friendly-name","input":"hello"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	Handle(c, FormatResponses)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, "upstream-model", chatModel)
}

func TestHandleRejectsOversizedBody(t *testing.T) {
	setupRelayDB(t)

	huge := `{"model":"test-model","messages":[{"role":"user","content":"` +
		strings.Repeat("x", maxRequestBody) + `"}]}`
	recorder := relayRequest(t, huge)

	require.Equal(t, http.StatusRequestEntityTooLarge, recorder.Code)
}
