package relay

import (
	"net/http"
	"strings"

	"github.com/tidwall/gjson"
)

// streamClaudeToClaude forwards a Claude SSE stream verbatim (event lines,
// data lines and blank separators) while capturing usage from
// message_start / message_delta events.
func streamClaudeToClaude(rc *relayContext, resp *http.Response) usage {
	setSSEHeaders(rc.c)
	var u usage

	scanSSE(resp.Body, func(line string) {
		_, _ = rc.c.Writer.WriteString(line + "\n")
		if line == "" {
			rc.c.Writer.Flush()
			return
		}
		if payload, ok := strings.CutPrefix(line, "data:"); ok {
			captureClaudeUsage(strings.TrimSpace(payload), &u)
		}
	})
	rc.c.Writer.Flush()

	if u.completion == 0 {
		u.estimated = true
	}
	return u
}

// captureClaudeUsage updates u from a Claude stream event payload.
// Cache tokens are folded into prompt tokens as an approximation.
func captureClaudeUsage(payload string, u *usage) {
	switch gjson.Get(payload, "type").String() {
	case "message_start":
		msgUsage := gjson.Get(payload, "message.usage")
		u.prompt = int(msgUsage.Get("input_tokens").Int() +
			msgUsage.Get("cache_creation_input_tokens").Int() +
			msgUsage.Get("cache_read_input_tokens").Int())
	case "message_delta":
		if v := gjson.Get(payload, "usage.output_tokens"); v.Exists() {
			u.completion = int(v.Int())
		}
		if v := gjson.Get(payload, "usage.input_tokens"); v.Exists() && v.Int() > 0 {
			u.prompt = int(v.Int())
		}
	}
}
