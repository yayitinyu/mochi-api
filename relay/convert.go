package relay

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"

	"mochi-api/common"
)

// This file implements OpenAI <-> Claude conversion for requests,
// non-stream responses and SSE streams. Coverage is intentionally
// limited to text chat, images and tool calls; unknown fields are dropped.

const defaultClaudeMaxTokens = 4096

// --- stop/finish reason mapping ---

func claudeStopToOpenAI(stopReason string) string {
	switch stopReason {
	case "max_tokens":
		return "length"
	case "tool_use":
		return "tool_calls"
	default: // end_turn, stop_sequence, ""
		return "stop"
	}
}

func openAIFinishToClaude(finishReason string) string {
	switch finishReason {
	case "length":
		return "max_tokens"
	case "tool_calls", "function_call":
		return "tool_use"
	default: // stop, content_filter, ""
		return "end_turn"
	}
}

// contentText flattens an OpenAI content value (string or part array) to text.
func contentText(content gjson.Result) string {
	if content.Type == gjson.String {
		return content.String()
	}
	var sb strings.Builder
	for _, part := range content.Array() {
		sb.WriteString(part.Get("text").String())
	}
	return sb.String()
}

// isEmptyOpenAIContent reports whether a message content value has no
// user-visible payload. Strict OpenAI-compatible providers (Moonshot/Kimi)
// reject user messages whose content is null, "", whitespace-only, or an
// array of empty parts with HTTP 400 "role 'user' must not be empty".
func isEmptyOpenAIContent(content gjson.Result) bool {
	if !content.Exists() || content.Type == gjson.Null {
		return true
	}
	if content.Type == gjson.String {
		return strings.TrimSpace(content.String()) == ""
	}
	if !content.IsArray() {
		// Unexpected object/number content: treat as non-empty so we do not
		// silently drop messages we cannot interpret.
		return false
	}
	for _, part := range content.Array() {
		if strings.TrimSpace(part.Get("text").String()) != "" {
			return false
		}
		// image_url may be a string URL or an object {url: "..."}.
		if img := part.Get("image_url"); img.Exists() {
			if img.Type == gjson.String && img.String() != "" {
				return false
			}
			if img.Get("url").String() != "" {
				return false
			}
		}
		if part.Get("url").String() != "" {
			return false
		}
	}
	return true
}

// sanitizeOpenAIChatMessages drops empty user/assistant/system turns that
// tool-calling agents often inject mid-loop. Assistant turns that only carry
// tool_calls (content null/"") are kept; tool turns are kept even when their
// content is empty. Returns an error when nothing usable remains.
func sanitizeOpenAIChatMessages(body []byte) ([]byte, error) {
	msgs := gjson.GetBytes(body, "messages")
	if !msgs.IsArray() {
		return body, nil
	}
	out := make([]any, 0, len(msgs.Array()))
	for _, msg := range msgs.Array() {
		role := msg.Get("role").String()
		empty := isEmptyOpenAIContent(msg.Get("content"))
		hasToolCalls := len(msg.Get("tool_calls").Array()) > 0
		hasReasoning := strings.TrimSpace(msg.Get("reasoning_content").String()) != ""

		switch role {
		case "user", "system", "developer":
			if empty {
				continue
			}
		case "assistant":
			// Empty assistant text is valid when the model is only calling tools
			// or replaying reasoning; pure empty turns are dropped.
			if empty && !hasToolCalls && !hasReasoning {
				continue
			}
		case "tool":
			// Keep tool results; some clients send empty strings on success.
		default:
			// Unknown roles pass through unchanged.
		}
		if v := msg.Value(); v != nil {
			out = append(out, v)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("messages 中没有非空内容")
	}
	return sjson.SetBytes(body, "messages", out)
}

// --- request conversion: OpenAI -> Claude ---

func convertRequestOpenAIToClaude(body []byte) ([]byte, error) {
	root := gjson.ParseBytes(body)
	out := map[string]any{
		"model":      root.Get("model").String(),
		"max_tokens": defaultClaudeMaxTokens,
	}
	explicitMaxTokens := false
	if v := root.Get("max_tokens"); v.Exists() {
		out["max_tokens"] = v.Int()
		explicitMaxTokens = true
	} else if v := root.Get("max_completion_tokens"); v.Exists() {
		out["max_tokens"] = v.Int()
		explicitMaxTokens = true
	}
	if v := root.Get("temperature"); v.Exists() {
		out["temperature"] = v.Float()
	}
	if v := root.Get("top_p"); v.Exists() {
		out["top_p"] = v.Float()
	}
	if v := root.Get("stream"); v.Exists() {
		out["stream"] = v.Bool()
	}
	thinkingEnabled := false
	if v := root.Get("thinking"); v.IsObject() {
		// Vendor extension: clients targeting Claude through the OpenAI format
		// can pass Anthropic's thinking config verbatim.
		out["thinking"] = v.Value()
		thinkingEnabled = v.Get("type").String() != "disabled"
		// Anthropic requires max_tokens > thinking.budget_tokens; only patch
		// the value we invented, never one the client chose explicitly.
		if !explicitMaxTokens {
			if budget := v.Get("budget_tokens").Int(); budget >= defaultClaudeMaxTokens {
				out["max_tokens"] = budget + defaultClaudeMaxTokens
			}
		}
	}
	if v := root.Get("stop"); v.Exists() {
		if v.Type == gjson.String {
			out["stop_sequences"] = []string{v.String()}
		} else {
			var stops []string
			for _, s := range v.Array() {
				stops = append(stops, s.String())
			}
			out["stop_sequences"] = stops
		}
	}

	var systemParts []string
	var messages []map[string]any
	appendBlocks := func(role string, blocks []any) {
		if len(blocks) == 0 {
			return
		}
		if len(messages) > 0 && messages[len(messages)-1]["role"] == role {
			prev := messages[len(messages)-1]["content"].([]any)
			messages[len(messages)-1]["content"] = append(prev, blocks...)
			return
		}
		messages = append(messages, map[string]any{"role": role, "content": blocks})
	}

	for _, msg := range root.Get("messages").Array() {
		role := msg.Get("role").String()
		content := msg.Get("content")
		switch role {
		case "system", "developer":
			if text := contentText(content); text != "" {
				systemParts = append(systemParts, text)
			}
		case "tool":
			appendBlocks("user", []any{map[string]any{
				"type":        "tool_result",
				"tool_use_id": msg.Get("tool_call_id").String(),
				"content":     contentText(content),
			}})
		case "assistant":
			var blocks []any
			// Replay prior thinking turns only when this request runs with
			// thinking enabled and the block is signed: Claude rejects thinking
			// blocks otherwise, and unsigned reasoning (from non-Anthropic
			// upstreams) can never validate.
			if thinkingEnabled {
				if reasoning := msg.Get("reasoning_content").String(); reasoning != "" {
					if signature := msg.Get("extra_content.anthropic.thinking_signature").String(); signature != "" {
						blocks = append(blocks, map[string]any{
							"type": "thinking", "thinking": reasoning, "signature": signature,
						})
					}
				}
			}
			if text := contentText(content); text != "" {
				blocks = append(blocks, map[string]any{"type": "text", "text": text})
			}
			for _, tc := range msg.Get("tool_calls").Array() {
				var input any = map[string]any{}
				if args := tc.Get("function.arguments").String(); args != "" {
					_ = json.Unmarshal([]byte(args), &input)
				}
				blocks = append(blocks, map[string]any{
					"type":  "tool_use",
					"id":    tc.Get("id").String(),
					"name":  tc.Get("function.name").String(),
					"input": input,
				})
			}
			appendBlocks("assistant", blocks)
		default: // user
			appendBlocks("user", openAIUserContentToClaudeBlocks(content))
		}
	}
	if len(systemParts) > 0 {
		out["system"] = strings.Join(systemParts, "\n")
	}
	out["messages"] = messages

	if tools := root.Get("tools").Array(); len(tools) > 0 {
		var claudeTools []map[string]any
		for _, t := range tools {
			fn := t.Get("function")
			schema := fn.Get("parameters").Value()
			if schema == nil {
				// OpenAI allows omitting parameters for no-arg functions;
				// Claude requires an object schema.
				schema = map[string]any{"type": "object"}
			}
			claudeTools = append(claudeTools, map[string]any{
				"name":         fn.Get("name").String(),
				"description":  fn.Get("description").String(),
				"input_schema": schema,
			})
		}
		out["tools"] = claudeTools
	}
	if tc := root.Get("tool_choice"); tc.Exists() {
		switch {
		case tc.Type == gjson.String && tc.String() == "required":
			out["tool_choice"] = map[string]any{"type": "any"}
		case tc.Type == gjson.String && tc.String() == "none":
			out["tool_choice"] = map[string]any{"type": "none"}
		case tc.Type == gjson.String:
			out["tool_choice"] = map[string]any{"type": "auto"}
		case tc.Get("function.name").Exists():
			out["tool_choice"] = map[string]any{
				"type": "tool", "name": tc.Get("function.name").String(),
			}
		}
	}
	return json.Marshal(out)
}

// openAIUserContentToClaudeBlocks maps user message content (string or parts)
// to Claude content blocks, converting image_url parts to image blocks.
func openAIUserContentToClaudeBlocks(content gjson.Result) []any {
	if content.Type == gjson.String {
		if content.String() == "" {
			return nil
		}
		return []any{map[string]any{"type": "text", "text": content.String()}}
	}
	var blocks []any
	for _, part := range content.Array() {
		switch part.Get("type").String() {
		case "image_url":
			url := part.Get("image_url.url").String()
			if source := imageURLToClaudeSource(url); source != nil {
				blocks = append(blocks, map[string]any{"type": "image", "source": source})
			}
		default: // "text"
			if text := part.Get("text").String(); text != "" {
				blocks = append(blocks, map[string]any{"type": "text", "text": text})
			}
		}
	}
	return blocks
}

// imageURLToClaudeSource converts an OpenAI image URL (http(s) or data URI)
// to a Claude image source object.
func imageURLToClaudeSource(url string) map[string]any {
	if data, ok := strings.CutPrefix(url, "data:"); ok {
		mediaType, b64, found := strings.Cut(data, ";base64,")
		if !found {
			return nil
		}
		return map[string]any{"type": "base64", "media_type": mediaType, "data": b64}
	}
	if url == "" {
		return nil
	}
	return map[string]any{"type": "url", "url": url}
}

// --- request conversion: Claude -> OpenAI ---

func convertRequestClaudeToOpenAI(body []byte) ([]byte, error) {
	root := gjson.ParseBytes(body)
	out := map[string]any{"model": root.Get("model").String()}
	if v := root.Get("max_tokens"); v.Exists() {
		out["max_tokens"] = v.Int()
	}
	if v := root.Get("temperature"); v.Exists() {
		out["temperature"] = v.Float()
	}
	if v := root.Get("top_p"); v.Exists() {
		out["top_p"] = v.Float()
	}
	if v := root.Get("stream"); v.Exists() {
		out["stream"] = v.Bool()
	}
	if v := root.Get("stop_sequences"); v.Exists() {
		var stops []string
		for _, s := range v.Array() {
			stops = append(stops, s.String())
		}
		out["stop"] = stops
	}

	var messages []map[string]any
	if system := root.Get("system"); system.Exists() {
		text := system.String()
		if system.IsArray() {
			var sb strings.Builder
			for _, block := range system.Array() {
				sb.WriteString(block.Get("text").String())
			}
			text = sb.String()
		}
		if text != "" {
			messages = append(messages, map[string]any{"role": "system", "content": text})
		}
	}

	for _, msg := range root.Get("messages").Array() {
		role := msg.Get("role").String()
		content := msg.Get("content")
		if role == "assistant" {
			var textParts []string
			var toolCalls []map[string]any
			if content.Type == gjson.String {
				textParts = append(textParts, content.String())
			}
			for _, block := range content.Array() {
				switch block.Get("type").String() {
				case "text":
					textParts = append(textParts, block.Get("text").String())
				case "tool_use":
					args, _ := json.Marshal(block.Get("input").Value())
					toolCalls = append(toolCalls, map[string]any{
						"id":   block.Get("id").String(),
						"type": "function",
						"function": map[string]any{
							"name":      block.Get("name").String(),
							"arguments": string(args),
						},
					})
				}
			}
			m := map[string]any{"role": "assistant", "content": strings.Join(textParts, "")}
			if len(toolCalls) > 0 {
				m["tool_calls"] = toolCalls
			}
			messages = append(messages, m)
			continue
		}
		// user message: split tool_result blocks into OpenAI "tool" messages
		if content.Type == gjson.String {
			// Skip empty user turns — Moonshot/Kimi reject them with 400001.
			if text := content.String(); strings.TrimSpace(text) != "" {
				messages = append(messages, map[string]any{"role": "user", "content": text})
			}
			continue
		}
		var parts []any
		flushParts := func() {
			if len(parts) > 0 {
				messages = append(messages, map[string]any{"role": "user", "content": parts})
				parts = nil
			}
		}
		for _, block := range content.Array() {
			switch block.Get("type").String() {
			case "text":
				if text := block.Get("text").String(); strings.TrimSpace(text) != "" {
					parts = append(parts, map[string]any{"type": "text", "text": text})
				}
			case "image":
				if url := claudeSourceToImageURL(block.Get("source")); url != "" {
					parts = append(parts, map[string]any{
						"type": "image_url", "image_url": map[string]any{"url": url},
					})
				}
			case "tool_result":
				flushParts()
				resultText := block.Get("content").String()
				if block.Get("content").IsArray() {
					var sb strings.Builder
					for _, rb := range block.Get("content").Array() {
						sb.WriteString(rb.Get("text").String())
					}
					resultText = sb.String()
				}
				messages = append(messages, map[string]any{
					"role":         "tool",
					"tool_call_id": block.Get("tool_use_id").String(),
					"content":      resultText,
				})
			}
		}
		flushParts()
	}
	out["messages"] = messages

	if tools := root.Get("tools").Array(); len(tools) > 0 {
		var openAITools []map[string]any
		for _, t := range tools {
			openAITools = append(openAITools, map[string]any{
				"type": "function",
				"function": map[string]any{
					"name":        t.Get("name").String(),
					"description": t.Get("description").String(),
					"parameters":  t.Get("input_schema").Value(),
				},
			})
		}
		out["tools"] = openAITools
	}
	if tc := root.Get("tool_choice"); tc.Exists() {
		switch tc.Get("type").String() {
		case "any":
			out["tool_choice"] = "required"
		case "none":
			out["tool_choice"] = "none"
		case "tool":
			out["tool_choice"] = map[string]any{
				"type":     "function",
				"function": map[string]any{"name": tc.Get("name").String()},
			}
		default:
			out["tool_choice"] = "auto"
		}
	}
	return json.Marshal(out)
}

// claudeSourceToImageURL converts a Claude image source to an OpenAI image URL.
func claudeSourceToImageURL(source gjson.Result) string {
	switch source.Get("type").String() {
	case "base64":
		return "data:" + source.Get("media_type").String() + ";base64," + source.Get("data").String()
	case "url":
		return source.Get("url").String()
	}
	return ""
}

// --- non-stream response conversion ---

func convertResponseClaudeToOpenAI(body []byte) ([]byte, error) {
	root := gjson.ParseBytes(body)
	var textParts []string
	var reasoningParts []string
	var reasoningSignature string
	var toolCalls []map[string]any
	for _, block := range root.Get("content").Array() {
		switch block.Get("type").String() {
		case "text":
			textParts = append(textParts, block.Get("text").String())
		case "thinking":
			reasoningParts = append(reasoningParts, block.Get("thinking").String())
			if signature := block.Get("signature").String(); signature != "" {
				reasoningSignature = signature
			}
		case "tool_use":
			args, _ := json.Marshal(block.Get("input").Value())
			toolCalls = append(toolCalls, map[string]any{
				"id":   block.Get("id").String(),
				"type": "function",
				"function": map[string]any{
					"name":      block.Get("name").String(),
					"arguments": string(args),
				},
			})
		}
	}
	message := map[string]any{"role": "assistant", "content": strings.Join(textParts, "")}
	if len(reasoningParts) > 0 {
		message["reasoning_content"] = strings.Join(reasoningParts, "")
	}
	if reasoningSignature != "" {
		message["extra_content"] = map[string]any{
			"anthropic": map[string]any{"thinking_signature": reasoningSignature},
		}
	}
	if len(toolCalls) > 0 {
		message["tool_calls"] = toolCalls
	}
	u := root.Get("usage")
	prompt := u.Get("input_tokens").Int() +
		u.Get("cache_creation_input_tokens").Int() + u.Get("cache_read_input_tokens").Int()
	completion := u.Get("output_tokens").Int()
	out := map[string]any{
		"id":      "chatcmpl-" + strings.TrimPrefix(root.Get("id").String(), "msg_"),
		"object":  "chat.completion",
		"created": time.Now().Unix(),
		"model":   root.Get("model").String(),
		"choices": []map[string]any{{
			"index":         0,
			"message":       message,
			"finish_reason": claudeStopToOpenAI(root.Get("stop_reason").String()),
		}},
		"usage": map[string]any{
			"prompt_tokens":     prompt,
			"completion_tokens": completion,
			"total_tokens":      prompt + completion,
		},
	}
	return json.Marshal(out)
}

func convertResponseOpenAIToClaude(body []byte) ([]byte, error) {
	root := gjson.ParseBytes(body)
	choice := root.Get("choices.0")
	var blocks []map[string]any
	if reasoning := choice.Get("message.reasoning_content").String(); reasoning != "" {
		block := map[string]any{"type": "thinking", "thinking": reasoning}
		signature := choice.Get("message.extra_content.google.thought_signature").String()
		if signature == "" {
			signature = choice.Get("message.extra_content.anthropic.thinking_signature").String()
		}
		block["signature"] = signature
		blocks = append(blocks, block)
	}
	if text := choice.Get("message.content").String(); text != "" {
		blocks = append(blocks, map[string]any{"type": "text", "text": text})
	}
	for _, tc := range choice.Get("message.tool_calls").Array() {
		var input any = map[string]any{}
		if args := tc.Get("function.arguments").String(); args != "" {
			_ = json.Unmarshal([]byte(args), &input)
		}
		blocks = append(blocks, map[string]any{
			"type":  "tool_use",
			"id":    tc.Get("id").String(),
			"name":  tc.Get("function.name").String(),
			"input": input,
		})
	}
	if blocks == nil {
		blocks = []map[string]any{}
	}
	out := map[string]any{
		"id":            "msg_" + strings.TrimPrefix(root.Get("id").String(), "chatcmpl-"),
		"type":          "message",
		"role":          "assistant",
		"model":         root.Get("model").String(),
		"content":       blocks,
		"stop_reason":   openAIFinishToClaude(choice.Get("finish_reason").String()),
		"stop_sequence": nil,
		"usage": map[string]any{
			"input_tokens":  root.Get("usage.prompt_tokens").Int(),
			"output_tokens": root.Get("usage.completion_tokens").Int(),
		},
	}
	return json.Marshal(out)
}

// --- stream conversion: Claude upstream -> OpenAI client ---

func writeOpenAIChunkData(rc *relayContext, payload gin.H) {
	data, _ := json.Marshal(payload)
	writeSSELine(rc.c, "data: "+string(data))
}

func streamClaudeToOpenAI(rc *relayContext, resp *http.Response) usage {
	setSSEHeaders(rc.c)
	id := "chatcmpl-" + common.GenerateKey(24)
	created := time.Now().Unix()
	var u usage
	stopReason := ""
	toolIndex := map[int64]int{} // claude content block index -> openai tool_calls index
	nextToolIndex := 0
	thinkingBlocks := map[int64]bool{}

	scanSSE(resp.Body, func(line string) {
		payload, ok := strings.CutPrefix(line, "data:")
		if !ok {
			return
		}
		payload = strings.TrimSpace(payload)
		captureClaudeUsage(payload, &u)
		event := gjson.Parse(payload)
		switch event.Get("type").String() {
		case "message_start":
			writeOpenAIChunkData(rc, openAIChunk(id, created, rc.modelName,
				gin.H{"role": "assistant", "content": ""}, nil))
		case "content_block_start":
			block := event.Get("content_block")
			if block.Get("type").String() == "thinking" {
				thinkingBlocks[event.Get("index").Int()] = true
			} else if block.Get("type").String() == "tool_use" {
				idx := nextToolIndex
				toolIndex[event.Get("index").Int()] = idx
				nextToolIndex++
				writeOpenAIChunkData(rc, openAIChunk(id, created, rc.modelName, gin.H{
					"tool_calls": []gin.H{{
						"index": idx,
						"id":    block.Get("id").String(),
						"type":  "function",
						"function": gin.H{
							"name":      block.Get("name").String(),
							"arguments": "",
						},
					}},
				}, nil))
			}
		case "content_block_delta":
			delta := event.Get("delta")
			switch delta.Get("type").String() {
			case "text_delta":
				writeOpenAIChunkData(rc, openAIChunk(id, created, rc.modelName,
					gin.H{"content": delta.Get("text").String()}, nil))
			case "thinking_delta":
				writeOpenAIChunkData(rc, openAIChunk(id, created, rc.modelName,
					gin.H{"reasoning_content": delta.Get("thinking").String()}, nil))
			case "signature_delta":
				if thinkingBlocks[event.Get("index").Int()] {
					writeOpenAIChunkData(rc, openAIChunk(id, created, rc.modelName, gin.H{
						"extra_content": gin.H{"anthropic": gin.H{"thinking_signature": delta.Get("signature").String()}},
					}, nil))
				}
			case "input_json_delta":
				if idx, ok := toolIndex[event.Get("index").Int()]; ok {
					writeOpenAIChunkData(rc, openAIChunk(id, created, rc.modelName, gin.H{
						"tool_calls": []gin.H{{
							"index":    idx,
							"function": gin.H{"arguments": delta.Get("partial_json").String()},
						}},
					}, nil))
				}
			}
		case "message_delta":
			if v := event.Get("delta.stop_reason"); v.Exists() {
				stopReason = v.String()
			}
		}
	})

	writeOpenAIChunkData(rc, openAIChunk(id, created, rc.modelName,
		gin.H{}, claudeStopToOpenAI(stopReason)))
	if rc.clientWantsUsage {
		chunk := openAIChunk(id, created, rc.modelName, gin.H{}, nil)
		chunk["choices"] = []gin.H{}
		chunk["usage"] = gin.H{
			"prompt_tokens":     u.prompt,
			"completion_tokens": u.completion,
			"total_tokens":      u.prompt + u.completion,
		}
		writeOpenAIChunkData(rc, chunk)
	}
	writeSSELine(rc.c, "data: [DONE]")
	return u
}

// --- stream conversion: OpenAI upstream -> Claude client ---

func writeClaudeEvent(rc *relayContext, event string, payload gin.H) {
	data, _ := json.Marshal(payload)
	_, _ = rc.c.Writer.WriteString("event: " + event + "\ndata: " + string(data) + "\n\n")
	rc.c.Writer.Flush()
}

func streamOpenAIToClaude(rc *relayContext, resp *http.Response) usage {
	setSSEHeaders(rc.c)
	msgId := "msg_" + common.GenerateKey(24)
	var u usage
	gotUsage := false
	finishReason := ""
	blockIndex := -1 // current claude content block index
	blockType := ""  // "text" | "tool_use" | ""
	curToolIndex := int64(-1)
	var outputText strings.Builder

	writeClaudeEvent(rc, "message_start", gin.H{
		"type": "message_start",
		"message": gin.H{
			"id": msgId, "type": "message", "role": "assistant",
			"model": rc.modelName, "content": []gin.H{},
			"stop_reason": nil, "stop_sequence": nil,
			"usage": gin.H{"input_tokens": 0, "output_tokens": 0},
		},
	})

	closeBlock := func() {
		if blockType != "" {
			writeClaudeEvent(rc, "content_block_stop", gin.H{
				"type": "content_block_stop", "index": blockIndex,
			})
			blockType = ""
		}
	}

	scanSSE(resp.Body, func(line string) {
		payload, ok := strings.CutPrefix(line, "data:")
		if !ok {
			return
		}
		payload = strings.TrimSpace(payload)
		if payload == "[DONE]" {
			return
		}
		if usageField := gjson.Get(payload, "usage"); usageField.IsObject() {
			u.prompt = int(usageField.Get("prompt_tokens").Int())
			u.completion = int(usageField.Get("completion_tokens").Int())
			gotUsage = true
		}
		choice := gjson.Get(payload, "choices.0")
		if v := choice.Get("finish_reason"); v.Exists() && v.Type == gjson.String {
			finishReason = v.String()
		}
		delta := choice.Get("delta")
		if reasoning := delta.Get("reasoning_content").String(); reasoning != "" {
			if blockType != "thinking" {
				closeBlock()
				blockIndex++
				blockType = "thinking"
				writeClaudeEvent(rc, "content_block_start", gin.H{
					"type": "content_block_start", "index": blockIndex,
					"content_block": gin.H{"type": "thinking", "thinking": ""},
				})
			}
			outputText.WriteString(reasoning)
			writeClaudeEvent(rc, "content_block_delta", gin.H{
				"type": "content_block_delta", "index": blockIndex,
				"delta": gin.H{"type": "thinking_delta", "thinking": reasoning},
			})
		}
		// Signatures may ride along a reasoning delta or arrive in their own
		// chunk right after the last one; both land while the block is open.
		if signature := thinkingSignatureFromDelta(delta); signature != "" && blockType == "thinking" {
			writeClaudeEvent(rc, "content_block_delta", gin.H{
				"type": "content_block_delta", "index": blockIndex,
				"delta": gin.H{"type": "signature_delta", "signature": signature},
			})
		}
		if text := delta.Get("content").String(); text != "" {
			if blockType != "text" {
				closeBlock()
				blockIndex++
				blockType = "text"
				writeClaudeEvent(rc, "content_block_start", gin.H{
					"type": "content_block_start", "index": blockIndex,
					"content_block": gin.H{"type": "text", "text": ""},
				})
			}
			outputText.WriteString(text)
			writeClaudeEvent(rc, "content_block_delta", gin.H{
				"type": "content_block_delta", "index": blockIndex,
				"delta": gin.H{"type": "text_delta", "text": text},
			})
		}
		for _, tc := range delta.Get("tool_calls").Array() {
			tcIndex := tc.Get("index").Int()
			if blockType != "tool_use" || tcIndex != curToolIndex {
				closeBlock()
				blockIndex++
				blockType = "tool_use"
				curToolIndex = tcIndex
				writeClaudeEvent(rc, "content_block_start", gin.H{
					"type": "content_block_start", "index": blockIndex,
					"content_block": gin.H{
						"type": "tool_use",
						"id":   tc.Get("id").String(),
						"name": tc.Get("function.name").String(),
					},
				})
			}
			if args := tc.Get("function.arguments").String(); args != "" {
				writeClaudeEvent(rc, "content_block_delta", gin.H{
					"type": "content_block_delta", "index": blockIndex,
					"delta": gin.H{"type": "input_json_delta", "partial_json": args},
				})
			}
		}
	})

	closeBlock()
	if !gotUsage {
		u.completion = estimateTokens(outputText.String())
		u.estimated = true
	}
	writeClaudeEvent(rc, "message_delta", gin.H{
		"type":  "message_delta",
		"delta": gin.H{"stop_reason": openAIFinishToClaude(finishReason), "stop_sequence": nil},
		"usage": gin.H{"input_tokens": u.prompt, "output_tokens": u.completion},
	})
	writeClaudeEvent(rc, "message_stop", gin.H{"type": "message_stop"})
	return u
}

// thinkingSignatureFromDelta pulls a thinking signature from the vendor
// extension fields our own bridges emit (Anthropic first, then Gemini).
func thinkingSignatureFromDelta(delta gjson.Result) string {
	if signature := delta.Get("extra_content.anthropic.thinking_signature").String(); signature != "" {
		return signature
	}
	return delta.Get("extra_content.google.thought_signature").String()
}
