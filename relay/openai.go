package relay

import (
	"net/http"
	"strings"

	"github.com/tidwall/gjson"

	"github.com/gin-gonic/gin"
)

// streamOpenAIToOpenAI forwards an OpenAI SSE stream verbatim while capturing
// usage. The stream_options.include_usage chunk we injected upstream is
// suppressed when the client did not ask for it (usage-only chunk: empty choices).
func streamOpenAIToOpenAI(rc *relayContext, resp *http.Response) usage {
	setSSEHeaders(rc.c)
	var u usage
	gotUsage := false
	var outputText strings.Builder
	doneSent := false

	scanSSE(resp.Body, func(line string) {
		if !strings.HasPrefix(line, "data:") {
			return
		}
		payload := strings.TrimSpace(line[len("data:"):])
		if payload == "[DONE]" {
			writeSSELine(rc.c, "data: [DONE]")
			doneSent = true
			return
		}
		usageField := gjson.Get(payload, "usage")
		if usageField.IsObject() {
			u.prompt = int(usageField.Get("prompt_tokens").Int())
			u.completion = int(usageField.Get("completion_tokens").Int())
			gotUsage = true
			if !rc.clientWantsUsage && len(gjson.Get(payload, "choices").Array()) == 0 {
				return // injected usage-only chunk: keep it out of the client stream
			}
		}
		for _, choice := range gjson.Get(payload, "choices").Array() {
			outputText.WriteString(choice.Get("delta.content").String())
		}
		writeSSELine(rc.c, "data: "+payload)
	})

	if !doneSent {
		writeSSELine(rc.c, "data: [DONE]")
	}
	if !gotUsage {
		u.completion = estimateTokens(outputText.String())
		u.estimated = true
	}
	return u
}

// nonStream OpenAI responses are handled by the shared dispatchNonStream.

// openAIChunk builds a chat.completion.chunk SSE payload.
func openAIChunk(id string, created int64, modelName string, delta gin.H, finishReason any) gin.H {
	return gin.H{
		"id":      id,
		"object":  "chat.completion.chunk",
		"created": created,
		"model":   modelName,
		"choices": []gin.H{{
			"index":         0,
			"delta":         delta,
			"finish_reason": finishReason,
		}},
	}
}
