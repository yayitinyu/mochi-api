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
)

func TestOpenAIToolWithoutParametersGetsObjectSchema(t *testing.T) {
	body := []byte(`{
		"model":"claude-x",
		"messages":[{"role":"user","content":"hi"}],
		"tools":[{"type":"function","function":{"name":"ping","description":"No-arg tool"}}]
	}`)

	converted, err := convertRequestOpenAIToClaude(body)
	require.NoError(t, err)
	require.Equal(t, "object",
		gjson.GetBytes(converted, "tools.0.input_schema.type").String(),
		"Claude requires an object input_schema even when OpenAI omits parameters")
}

func TestOpenAIToClaudeThinkingPassthrough(t *testing.T) {
	// The thinking config passes through verbatim, and the invented
	// max_tokens default is raised above the budget.
	body := []byte(`{
		"model":"claude-x",
		"thinking":{"type":"enabled","budget_tokens":8192},
		"messages":[{"role":"user","content":"hi"}]
	}`)
	converted, err := convertRequestOpenAIToClaude(body)
	require.NoError(t, err)
	require.Equal(t, int64(8192), gjson.GetBytes(converted, "thinking.budget_tokens").Int())
	require.Greater(t, gjson.GetBytes(converted, "max_tokens").Int(), int64(8192))

	// An explicit client max_tokens is never patched.
	body = []byte(`{
		"model":"claude-x",
		"max_tokens":2000,
		"thinking":{"type":"enabled","budget_tokens":8192},
		"messages":[{"role":"user","content":"hi"}]
	}`)
	converted, err = convertRequestOpenAIToClaude(body)
	require.NoError(t, err)
	require.Equal(t, int64(2000), gjson.GetBytes(converted, "max_tokens").Int())
}

func TestOpenAIToClaudeThinkingReplayIsGated(t *testing.T) {
	history := `[
		{"role":"user","content":"hi"},
		{"role":"assistant",
		 "content":"the answer",
		 "reasoning_content":"let me think",
		 "extra_content":{"anthropic":{"thinking_signature":"sig-1"}},
		 "tool_calls":[{"id":"toolu_1","type":"function","function":{"name":"lookup","arguments":"{}"}}]},
		{"role":"tool","tool_call_id":"toolu_1","content":"result"}
	]`

	// With thinking enabled, the signed reasoning replays as a thinking block
	// placed before other content, as Anthropic requires.
	withThinking := []byte(`{"model":"claude-x","thinking":{"type":"enabled","budget_tokens":2048},"messages":` + history + `}`)
	converted, err := convertRequestOpenAIToClaude(withThinking)
	require.NoError(t, err)
	assistant := gjson.GetBytes(converted, "messages.1.content")
	require.Equal(t, "thinking", assistant.Get("0.type").String())
	require.Equal(t, "let me think", assistant.Get("0.thinking").String())
	require.Equal(t, "sig-1", assistant.Get("0.signature").String())
	require.Equal(t, "text", assistant.Get("1.type").String())
	require.Equal(t, "tool_use", assistant.Get("2.type").String())

	// Without a thinking config the block is dropped, matching the previous
	// behavior: Claude rejects thinking blocks when thinking is disabled.
	withoutThinking := []byte(`{"model":"claude-x","messages":` + history + `}`)
	converted, err = convertRequestOpenAIToClaude(withoutThinking)
	require.NoError(t, err)
	require.Equal(t, "text", gjson.GetBytes(converted, "messages.1.content.0.type").String())

	// Unsigned reasoning (from non-Anthropic upstreams) can never validate
	// and is dropped even when thinking is on.
	unsigned := []byte(`{"model":"claude-x","thinking":{"type":"enabled","budget_tokens":2048},"messages":[
		{"role":"user","content":"hi"},
		{"role":"assistant","content":"answer","reasoning_content":"raw thoughts"}
	]}`)
	converted, err = convertRequestOpenAIToClaude(unsigned)
	require.NoError(t, err)
	require.Equal(t, "text", gjson.GetBytes(converted, "messages.1.content.0.type").String())
}

// claudeStreamEvents parses "event:"/"data:" SSE pairs into data payloads.
func claudeStreamEvents(body string) []gjson.Result {
	var events []gjson.Result
	for _, line := range strings.Split(body, "\n") {
		if payload, ok := strings.CutPrefix(line, "data: "); ok {
			events = append(events, gjson.Parse(payload))
		}
	}
	return events
}

func TestStreamOpenAIToClaudeEmitsThinkingBlocks(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	rc := &relayContext{c: c, clientFormat: FormatClaude, upstreamFormat: FormatOpenAI, modelName: "r1"}
	upstream := &http.Response{Body: io.NopCloser(strings.NewReader(
		`data: {"choices":[{"delta":{"role":"assistant"}}]}` + "\n\n" +
			`data: {"choices":[{"delta":{"reasoning_content":"step 1"}}]}` + "\n\n" +
			`data: {"choices":[{"delta":{"extra_content":{"anthropic":{"thinking_signature":"sig-9"}}}}]}` + "\n\n" +
			`data: {"choices":[{"delta":{"content":"final"},"finish_reason":"stop"}]}` + "\n\n" +
			`data: [DONE]` + "\n\n",
	))}

	u := streamOpenAIToClaude(rc, upstream)

	var thinkingStart, thinkingDelta, signatureDelta, textDelta bool
	for _, ev := range claudeStreamEvents(recorder.Body.String()) {
		switch ev.Get("type").String() {
		case "content_block_start":
			if ev.Get("content_block.type").String() == "thinking" {
				thinkingStart = true
			}
		case "content_block_delta":
			switch ev.Get("delta.type").String() {
			case "thinking_delta":
				require.Equal(t, "step 1", ev.Get("delta.thinking").String())
				thinkingDelta = true
			case "signature_delta":
				require.Equal(t, "sig-9", ev.Get("delta.signature").String())
				signatureDelta = true
			case "text_delta":
				require.Equal(t, "final", ev.Get("delta.text").String())
				textDelta = true
			}
		}
	}
	require.True(t, thinkingStart, "reasoning_content must open a thinking block")
	require.True(t, thinkingDelta)
	require.True(t, signatureDelta, "signature chunks must become signature_delta events")
	require.True(t, textDelta)
	// Reasoning counts toward the estimated completion tokens.
	require.True(t, u.estimated)
	require.Greater(t, u.completion, 0)
}
