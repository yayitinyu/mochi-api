package relay

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"

	"mochi-api/common"
)

// Gemini uses a distinct wire format (contents/parts, generationConfig,
// usageMetadata). We always bridge through the OpenAI intermediate format:
// the request arrives as OpenAI JSON, and responses are emitted as OpenAI
// (then optionally converted to Claude by the caller).

// --- request: OpenAI -> Gemini ---

func convertRequestOpenAIToGemini(body []byte) ([]byte, error) {
	root := gjson.ParseBytes(body)
	out := map[string]any{}

	var contents []map[string]any
	var systemParts []map[string]any

	for _, msg := range root.Get("messages").Array() {
		role := msg.Get("role").String()
		switch role {
		case "system", "developer":
			if text := contentText(msg.Get("content")); text != "" {
				systemParts = append(systemParts, map[string]any{"text": text})
			}
		case "tool":
			contents = append(contents, map[string]any{
				"role": "user",
				"parts": []any{map[string]any{
					"functionResponse": map[string]any{
						"name":     msg.Get("name").String(),
						"response": map[string]any{"content": contentText(msg.Get("content"))},
					},
				}},
			})
		case "assistant":
			var parts []any
			if text := contentText(msg.Get("content")); text != "" {
				parts = append(parts, map[string]any{"text": text})
			}
			for _, tc := range msg.Get("tool_calls").Array() {
				var args any = map[string]any{}
				if s := tc.Get("function.arguments").String(); s != "" {
					_ = json.Unmarshal([]byte(s), &args)
				}
				parts = append(parts, map[string]any{
					"functionCall": map[string]any{
						"name": tc.Get("function.name").String(),
						"args": args,
					},
				})
			}
			if len(parts) > 0 {
				contents = append(contents, map[string]any{"role": "model", "parts": parts})
			}
		default: // user
			contents = append(contents, map[string]any{
				"role":  "user",
				"parts": openAIContentToGeminiParts(msg.Get("content")),
			})
		}
	}
	out["contents"] = contents
	if len(systemParts) > 0 {
		out["systemInstruction"] = map[string]any{"parts": systemParts}
	}

	genConfig := map[string]any{}
	if v := root.Get("temperature"); v.Exists() {
		genConfig["temperature"] = v.Float()
	}
	if v := root.Get("top_p"); v.Exists() {
		genConfig["topP"] = v.Float()
	}
	if v := root.Get("max_tokens"); v.Exists() {
		genConfig["maxOutputTokens"] = v.Int()
	} else if v := root.Get("max_completion_tokens"); v.Exists() {
		genConfig["maxOutputTokens"] = v.Int()
	}
	if v := root.Get("stop"); v.Exists() {
		if v.Type == gjson.String {
			genConfig["stopSequences"] = []string{v.String()}
		} else {
			var stops []string
			for _, s := range v.Array() {
				stops = append(stops, s.String())
			}
			genConfig["stopSequences"] = stops
		}
	}
	if len(genConfig) > 0 {
		out["generationConfig"] = genConfig
	}

	if tools := root.Get("tools").Array(); len(tools) > 0 {
		var decls []map[string]any
		for _, t := range tools {
			fn := t.Get("function")
			decl := map[string]any{
				"name":        fn.Get("name").String(),
				"description": fn.Get("description").String(),
			}
			if params := fn.Get("parameters"); params.Exists() {
				decl["parameters"] = params.Value()
			}
			decls = append(decls, decl)
		}
		out["tools"] = []any{map[string]any{"functionDeclarations": decls}}
	}

	return json.Marshal(out)
}

func openAIContentToGeminiParts(content gjson.Result) []any {
	if content.Type == gjson.String {
		return []any{map[string]any{"text": content.String()}}
	}
	var parts []any
	for _, part := range content.Array() {
		switch part.Get("type").String() {
		case "image_url":
			url := part.Get("image_url.url").String()
			if data, ok := strings.CutPrefix(url, "data:"); ok {
				if mediaType, b64, found := strings.Cut(data, ";base64,"); found {
					parts = append(parts, map[string]any{
						"inlineData": map[string]any{"mimeType": mediaType, "data": b64},
					})
				}
			}
		default:
			if text := part.Get("text").String(); text != "" {
				parts = append(parts, map[string]any{"text": text})
			}
		}
	}
	if len(parts) == 0 {
		parts = append(parts, map[string]any{"text": ""})
	}
	return parts
}

// geminiFinishToOpenAI maps a Gemini finishReason to an OpenAI finish_reason.
func geminiFinishToOpenAI(reason string) string {
	switch reason {
	case "MAX_TOKENS":
		return "length"
	case "SAFETY", "RECITATION", "BLOCKLIST", "PROHIBITED_CONTENT":
		return "content_filter"
	case "":
		return ""
	default: // STOP
		return "stop"
	}
}

// geminiCandidateToOpenAI extracts text and tool calls from a Gemini
// candidate's content.parts array.
func geminiCandidateToOpenAI(candidate gjson.Result) (string, []map[string]any) {
	var text strings.Builder
	var toolCalls []map[string]any
	for _, part := range candidate.Get("content.parts").Array() {
		if t := part.Get("text"); t.Exists() {
			text.WriteString(t.String())
		}
		if fc := part.Get("functionCall"); fc.Exists() {
			args, _ := json.Marshal(fc.Get("args").Value())
			toolCalls = append(toolCalls, map[string]any{
				"id":   "call_" + common.GenerateKey(16),
				"type": "function",
				"function": map[string]any{
					"name":      fc.Get("name").String(),
					"arguments": string(args),
				},
			})
		}
	}
	return text.String(), toolCalls
}

// --- non-stream response: Gemini -> OpenAI ---

func convertResponseGeminiToOpenAI(body []byte, modelName string) ([]byte, error) {
	root := gjson.ParseBytes(body)
	candidate := root.Get("candidates.0")
	text, toolCalls := geminiCandidateToOpenAI(candidate)

	message := map[string]any{"role": "assistant", "content": text}
	if len(toolCalls) > 0 {
		message["tool_calls"] = toolCalls
	}
	u := root.Get("usageMetadata")
	prompt := u.Get("promptTokenCount").Int()
	completion := u.Get("candidatesTokenCount").Int()
	out := map[string]any{
		"id":      "chatcmpl-" + common.GenerateKey(24),
		"object":  "chat.completion",
		"created": time.Now().Unix(),
		"model":   modelName,
		"choices": []map[string]any{{
			"index":         0,
			"message":       message,
			"finish_reason": geminiFinishToOpenAI(candidate.Get("finishReason").String()),
		}},
		"usage": map[string]any{
			"prompt_tokens":     prompt,
			"completion_tokens": completion,
			"total_tokens":      prompt + completion,
		},
	}
	return json.Marshal(out)
}

// --- stream: Gemini -> OpenAI client ---

func streamGeminiToOpenAI(rc *relayContext, resp *http.Response) usage {
	setSSEHeaders(rc.c)
	id := "chatcmpl-" + common.GenerateKey(24)
	created := time.Now().Unix()
	var u usage
	firstChunk := true
	toolIndex := 0

	scanSSE(resp.Body, func(line string) {
		payload, ok := strings.CutPrefix(line, "data:")
		if !ok {
			return
		}
		event := gjson.Parse(strings.TrimSpace(payload))
		if um := event.Get("usageMetadata"); um.Exists() {
			u.prompt = int(um.Get("promptTokenCount").Int())
			u.completion = int(um.Get("candidatesTokenCount").Int())
		}
		candidate := event.Get("candidates.0")
		if firstChunk {
			writeOpenAIChunkData(rc, openAIChunk(id, created, rc.modelName,
				gin.H{"role": "assistant", "content": ""}, nil))
			firstChunk = false
		}
		for _, part := range candidate.Get("content.parts").Array() {
			if t := part.Get("text"); t.Exists() && t.String() != "" {
				writeOpenAIChunkData(rc, openAIChunk(id, created, rc.modelName,
					gin.H{"content": t.String()}, nil))
			}
			if fc := part.Get("functionCall"); fc.Exists() {
				args, _ := json.Marshal(fc.Get("args").Value())
				writeOpenAIChunkData(rc, openAIChunk(id, created, rc.modelName, gin.H{
					"tool_calls": []gin.H{{
						"index": toolIndex,
						"id":    "call_" + common.GenerateKey(16),
						"type":  "function",
						"function": gin.H{
							"name":      fc.Get("name").String(),
							"arguments": string(args),
						},
					}},
				}, nil))
				toolIndex++
			}
		}
		if fr := candidate.Get("finishReason"); fr.Exists() {
			writeOpenAIChunkData(rc, openAIChunk(id, created, rc.modelName,
				gin.H{}, geminiFinishToOpenAI(fr.String())))
		}
	})

	if firstChunk {
		// Empty stream: still emit a role chunk + stop so clients terminate cleanly.
		writeOpenAIChunkData(rc, openAIChunk(id, created, rc.modelName,
			gin.H{"role": "assistant", "content": ""}, "stop"))
	}
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

// --- stream: Gemini -> Claude client ---

func streamGeminiToClaude(rc *relayContext, resp *http.Response) usage {
	setSSEHeaders(rc.c)
	msgId := "msg_" + common.GenerateKey(24)
	var u usage
	finishReason := ""
	blockOpen := false

	writeClaudeEvent(rc, "message_start", gin.H{
		"type": "message_start",
		"message": gin.H{
			"id": msgId, "type": "message", "role": "assistant",
			"model": rc.modelName, "content": []gin.H{},
			"stop_reason": nil, "stop_sequence": nil,
			"usage": gin.H{"input_tokens": 0, "output_tokens": 0},
		},
	})

	scanSSE(resp.Body, func(line string) {
		payload, ok := strings.CutPrefix(line, "data:")
		if !ok {
			return
		}
		event := gjson.Parse(strings.TrimSpace(payload))
		if um := event.Get("usageMetadata"); um.Exists() {
			u.prompt = int(um.Get("promptTokenCount").Int())
			u.completion = int(um.Get("candidatesTokenCount").Int())
		}
		candidate := event.Get("candidates.0")
		for _, part := range candidate.Get("content.parts").Array() {
			text := part.Get("text").String()
			if text == "" {
				continue
			}
			if !blockOpen {
				writeClaudeEvent(rc, "content_block_start", gin.H{
					"type": "content_block_start", "index": 0,
					"content_block": gin.H{"type": "text", "text": ""},
				})
				blockOpen = true
			}
			writeClaudeEvent(rc, "content_block_delta", gin.H{
				"type": "content_block_delta", "index": 0,
				"delta": gin.H{"type": "text_delta", "text": text},
			})
		}
		if fr := candidate.Get("finishReason"); fr.Exists() {
			finishReason = fr.String()
		}
	})

	if blockOpen {
		writeClaudeEvent(rc, "content_block_stop", gin.H{"type": "content_block_stop", "index": 0})
	}
	stopReason := "end_turn"
	if geminiFinishToOpenAI(finishReason) == "length" {
		stopReason = "max_tokens"
	}
	writeClaudeEvent(rc, "message_delta", gin.H{
		"type":  "message_delta",
		"delta": gin.H{"stop_reason": stopReason, "stop_sequence": nil},
		"usage": gin.H{"input_tokens": u.prompt, "output_tokens": u.completion},
	})
	writeClaudeEvent(rc, "message_stop", gin.H{"type": "message_stop"})
	return u
}
