package relay

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"os"
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
	f, _ := os.OpenFile("debug.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if f != nil {
		_, _ = f.WriteString("\n=== REQUEST START ===\n" + string(body) + "\n")
		defer f.Close()
	}
	root := gjson.ParseBytes(body)
	out := map[string]any{}

	var contents []map[string]any
	var systemParts []map[string]any
	toolNames := map[string]string{}

	for _, msg := range root.Get("messages").Array() {
		role := msg.Get("role").String()
		switch role {
		case "system", "developer":
			if text := contentText(msg.Get("content")); text != "" {
				systemParts = append(systemParts, map[string]any{"text": text})
			}
		case "tool":
			toolCallID := msg.Get("tool_call_id").String()
			name := msg.Get("name").String()
			if name == "" {
				name = toolNames[toolCallID]
			}
			if name == "" {
				name, _ = decodeToolCallID(toolCallID)
			}
			contents = append(contents, map[string]any{
				"role": "user",
				"parts": []any{map[string]any{
					"functionResponse": map[string]any{
						"name":     name,
						"id":       toolCallID,
						"response": map[string]any{"content": contentText(msg.Get("content"))},
					},
				}},
			})
		case "assistant":
			var parts []any
			if reasoning := msg.Get("reasoning_content").String(); reasoning != "" {
				part := map[string]any{"text": reasoning, "thought": true}
				if signature := msg.Get("extra_content.google.thought_signature").String(); signature != "" {
					part["thoughtSignature"] = signature
				}
				parts = append(parts, part)
			}
			if text := contentText(msg.Get("content")); text != "" {
				part := map[string]any{"text": text}
				if signature := msg.Get("extra_content.google.thought_signature").String(); signature != "" {
					part["thoughtSignature"] = signature
				}
				parts = append(parts, part)
			}
			for _, tc := range msg.Get("tool_calls").Array() {
				var args any = map[string]any{}
				if s := tc.Get("function.arguments").String(); s != "" {
					_ = json.Unmarshal([]byte(s), &args)
				}
				tcID := tc.Get("id").String()
				name, signature := decodeToolCallID(tcID)
				if name == "" {
					name = tc.Get("function.name").String()
				}
				functionCall := map[string]any{
					"name": name,
					"args": args,
					"id":   tcID,
				}
				part := map[string]any{"functionCall": functionCall}
				if signature == "" {
					signature = tc.Get("extra_content.google.thought_signature").String()
				}
				if signature != "" {
					part["thoughtSignature"] = signature
				}
				parts = append(parts, part)
				toolNames[tcID] = name
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
	modelName := root.Get("model").String()
	if strings.Contains(modelName, "gemini-2.5") || strings.HasPrefix(modelName, "gemini-3") || root.Get("reasoning").Exists() {
		thinking := map[string]any{"includeThoughts": true}
		if effort := root.Get("reasoning.effort").String(); effort != "" {
			applyGeminiReasoningEffort(thinking, modelName, effort)
		}
		genConfig["thinkingConfig"] = thinking
	}
	if len(genConfig) > 0 {
		out["generationConfig"] = genConfig
	}

	if tools := root.Get("tools").Array(); len(tools) > 0 {
		var decls []map[string]any
		var geminiTools []any
		for _, t := range tools {
			typeName := t.Get("type").String()
			if typeName == "web_search" || typeName == "web_search_preview" {
				geminiTools = append(geminiTools, map[string]any{"googleSearch": map[string]any{}})
				continue
			}
			fn := t.Get("function")
			if !fn.Exists() {
				continue
			}
			decl := map[string]any{
				"name":        fn.Get("name").String(),
				"description": fn.Get("description").String(),
			}
			if params := fn.Get("parameters"); params.Exists() {
				decl["parameters"] = sanitizeGeminiSchema(params.Value())
			}
			decls = append(decls, decl)
		}
		if len(decls) > 0 {
			geminiTools = append(geminiTools, map[string]any{"functionDeclarations": decls})
		}
		if len(geminiTools) > 0 {
			out["tools"] = geminiTools
		}
	}
	if choice := root.Get("tool_choice"); choice.Exists() {
		config := map[string]any{}
		switch {
		case choice.Type == gjson.String && choice.String() == "required":
			config["mode"] = "ANY"
		case choice.Type == gjson.String && choice.String() == "none":
			config["mode"] = "NONE"
		case choice.Get("function.name").String() != "":
			config["mode"] = "ANY"
			config["allowedFunctionNames"] = []string{choice.Get("function.name").String()}
		default:
			config["mode"] = "AUTO"
		}
		out["toolConfig"] = map[string]any{"functionCallingConfig": config}
	}

	converted, err := json.Marshal(out)
	if f != nil && err == nil {
		_, _ = f.WriteString("=== GEMINI REQ ===\n" + string(converted) + "\n=== END ===\n")
	}
	return converted, err
}


// sanitizeGeminiSchema removes JSON Schema keywords that Gemini's
// FunctionDeclaration.parameters field does not accept. OpenAI-compatible
// clients commonly attach these keywords at every nested object level.
func sanitizeGeminiSchema(value any) any {
	switch schema := value.(type) {
	case []any:
		cleaned := make([]any, len(schema))
		for i, item := range schema {
			cleaned[i] = sanitizeGeminiSchema(item)
		}
		return cleaned
	case map[string]any:
		cleaned := make(map[string]any, len(schema))
		for key, item := range schema {
			switch key {
			case "$schema", "additionalProperties":
				continue
			default:
				cleaned[key] = sanitizeGeminiSchema(item)
			}
		}
		return cleaned
	default:
		return value
	}
}

func applyGeminiReasoningEffort(config map[string]any, modelName, effort string) {
	if strings.HasPrefix(modelName, "gemini-3") {
		level := effort
		switch effort {
		case "none", "minimal":
			level = "minimal"
		case "xhigh":
			level = "high"
		}
		config["thinkingLevel"] = level
		return
	}
	if strings.Contains(modelName, "gemini-2.5") {
		budget := -1
		switch effort {
		case "none", "minimal":
			budget = 0
			if strings.Contains(modelName, "pro") {
				budget = 128
			}
		case "low":
			budget = 1024
		case "medium":
			budget = 8192
		case "high":
			budget = 24576
		case "xhigh":
			budget = 24576
			if strings.Contains(modelName, "pro") {
				budget = 32768
			}
		}
		config["thinkingBudget"] = budget
	}
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
func geminiCandidateToOpenAI(candidate gjson.Result) (string, string, []map[string]any, map[string]any) {
	var text strings.Builder
	var reasoning strings.Builder
	var toolCalls []map[string]any
	var messageExtra map[string]any

	lastThoughtSignature := ""
	for _, part := range candidate.Get("content.parts").Array() {
		if sig := part.Get("thoughtSignature").String(); sig != "" {
			lastThoughtSignature = sig
		}
	}

	for _, part := range candidate.Get("content.parts").Array() {
		if t := part.Get("text"); t.Exists() {
			if part.Get("thought").Bool() {
				reasoning.WriteString(t.String())
			} else {
				text.WriteString(t.String())
			}
			if signature := part.Get("thoughtSignature").String(); signature != "" {
				messageExtra = map[string]any{"google": map[string]any{"thought_signature": signature}}
			}
		}
		if fc := part.Get("functionCall"); fc.Exists() {
			args, _ := json.Marshal(fc.Get("args").Value())
			name := fc.Get("name").String()
			signature := lastThoughtSignature
			callID := fc.Get("id").String()
			if callID == "" {
				callID = encodeToolCallID(name, signature)
			}
			call := map[string]any{
				"id":   callID,
				"type": "function",
				"function": map[string]any{
					"name":      name,
					"arguments": string(args),
				},
			}
			if signature != "" {
				call["extra_content"] = map[string]any{"google": map[string]any{"thought_signature": signature}}
			}
			toolCalls = append(toolCalls, call)
		}
	}
	return text.String(), reasoning.String(), toolCalls, messageExtra
}


// --- non-stream response: Gemini -> OpenAI ---

func convertResponseGeminiToOpenAI(body []byte, modelName string) ([]byte, error) {
	root := gjson.ParseBytes(body)
	candidate := root.Get("candidates.0")
	text, reasoning, toolCalls, messageExtra := geminiCandidateToOpenAI(candidate)

	message := map[string]any{"role": "assistant", "content": text}
	if reasoning != "" {
		message["reasoning_content"] = reasoning
	}
	if messageExtra != nil {
		message["extra_content"] = messageExtra
	}
	if len(toolCalls) > 0 {
		message["tool_calls"] = toolCalls
	}
	u := root.Get("usageMetadata")
	prompt := u.Get("promptTokenCount").Int()
	reasoningTokens := u.Get("thoughtsTokenCount").Int()
	completion := u.Get("candidatesTokenCount").Int() + reasoningTokens
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
			"prompt_tokens":             prompt,
			"completion_tokens":         completion,
			"total_tokens":              prompt + completion,
			"completion_tokens_details": map[string]any{"reasoning_tokens": reasoningTokens},
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
	reasoningTokens := 0
	lastThoughtSignature := ""

	scanSSE(resp.Body, func(line string) {
		payload, ok := strings.CutPrefix(line, "data:")
		if !ok {
			return
		}
		event := gjson.Parse(strings.TrimSpace(payload))
		if um := event.Get("usageMetadata"); um.Exists() {
			u.prompt = int(um.Get("promptTokenCount").Int())
			reasoningTokens = int(um.Get("thoughtsTokenCount").Int())
			u.completion = int(um.Get("candidatesTokenCount").Int()) + reasoningTokens
		}
		candidate := event.Get("candidates.0")
		if firstChunk {
			writeOpenAIChunkData(rc, openAIChunk(id, created, rc.modelName,
				gin.H{"role": "assistant", "content": ""}, nil))
			firstChunk = false
		}
		for _, part := range candidate.Get("content.parts").Array() {
			if sig := part.Get("thoughtSignature").String(); sig != "" {
				lastThoughtSignature = sig
			}
		}
		for _, part := range candidate.Get("content.parts").Array() {
			if t := part.Get("text"); t.Exists() && t.String() != "" {
				field := "content"
				if part.Get("thought").Bool() {
					field = "reasoning_content"
				}
				delta := gin.H{field: t.String()}
				if signature := part.Get("thoughtSignature").String(); signature != "" {
					delta["extra_content"] = gin.H{"google": gin.H{"thought_signature": signature}}
				}
				writeOpenAIChunkData(rc, openAIChunk(id, created, rc.modelName, delta, nil))
			}
			if fc := part.Get("functionCall"); fc.Exists() {
				args, _ := json.Marshal(fc.Get("args").Value())
				name := fc.Get("name").String()
				signature := lastThoughtSignature
				callID := fc.Get("id").String()
				if callID == "" {
					callID = encodeToolCallID(name, signature)
				}
				call := gin.H{
					"index": toolIndex,
					"id":    callID,
					"type":  "function",
					"function": gin.H{
						"name": name, "arguments": string(args),
					},
				}
				if signature != "" {
					call["extra_content"] = gin.H{"google": gin.H{"thought_signature": signature}}
				}
				writeOpenAIChunkData(rc, openAIChunk(id, created, rc.modelName, gin.H{
					"tool_calls": []gin.H{call},
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
			"prompt_tokens":             u.prompt,
			"completion_tokens":         u.completion,
			"total_tokens":              u.prompt + u.completion,
			"completion_tokens_details": gin.H{"reasoning_tokens": reasoningTokens},
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
	blockIndex := -1
	blockType := ""
	thinkingSignature := ""
	hadToolCall := false

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
		if blockType == "" {
			return
		}
		if blockType == "thinking" && thinkingSignature != "" {
			writeClaudeEvent(rc, "content_block_delta", gin.H{
				"type": "content_block_delta", "index": blockIndex,
				"delta": gin.H{"type": "signature_delta", "signature": thinkingSignature},
			})
		}
		writeClaudeEvent(rc, "content_block_stop", gin.H{"type": "content_block_stop", "index": blockIndex})
		blockType = ""
		thinkingSignature = ""
	}
	openBlock := func(kind string) {
		if blockType == kind {
			return
		}
		closeBlock()
		blockIndex++
		blockType = kind
		content := gin.H{"type": "text", "text": ""}
		if kind == "thinking" {
			content = gin.H{"type": "thinking", "thinking": "", "signature": ""}
		}
		writeClaudeEvent(rc, "content_block_start", gin.H{
			"type": "content_block_start", "index": blockIndex, "content_block": content,
		})
	}

	scanSSE(resp.Body, func(line string) {
		payload, ok := strings.CutPrefix(line, "data:")
		if !ok {
			return
		}
		event := gjson.Parse(strings.TrimSpace(payload))
		if um := event.Get("usageMetadata"); um.Exists() {
			u.prompt = int(um.Get("promptTokenCount").Int())
			u.completion = int(um.Get("candidatesTokenCount").Int() + um.Get("thoughtsTokenCount").Int())
		}
		candidate := event.Get("candidates.0")
		for _, part := range candidate.Get("content.parts").Array() {
			text := part.Get("text").String()
			if part.Get("thought").Bool() {
				openBlock("thinking")
				if signature := part.Get("thoughtSignature").String(); signature != "" {
					thinkingSignature = signature
				}
				if text != "" {
					writeClaudeEvent(rc, "content_block_delta", gin.H{
						"type": "content_block_delta", "index": blockIndex,
						"delta": gin.H{"type": "thinking_delta", "thinking": text},
					})
				}
				continue
			}
			if text != "" {
				openBlock("text")
				writeClaudeEvent(rc, "content_block_delta", gin.H{
					"type": "content_block_delta", "index": blockIndex,
					"delta": gin.H{"type": "text_delta", "text": text},
				})
			}
			if call := part.Get("functionCall"); call.Exists() {
				closeBlock()
				blockIndex++
				hadToolCall = true
				callID := call.Get("id").String()
				if callID == "" {
					callID = "toolu_" + common.GenerateKey(16)
				}
				writeClaudeEvent(rc, "content_block_start", gin.H{
					"type": "content_block_start", "index": blockIndex,
					"content_block": gin.H{"type": "tool_use", "id": callID, "name": call.Get("name").String(), "input": gin.H{}},
				})
				args, _ := json.Marshal(call.Get("args").Value())
				writeClaudeEvent(rc, "content_block_delta", gin.H{
					"type": "content_block_delta", "index": blockIndex,
					"delta": gin.H{"type": "input_json_delta", "partial_json": string(args)},
				})
				writeClaudeEvent(rc, "content_block_stop", gin.H{"type": "content_block_stop", "index": blockIndex})
			}
		}
		if fr := candidate.Get("finishReason"); fr.Exists() {
			finishReason = fr.String()
		}
	})

	closeBlock()
	stopReason := "end_turn"
	if geminiFinishToOpenAI(finishReason) == "length" {
		stopReason = "max_tokens"
	} else if hadToolCall {
		stopReason = "tool_use"
	}
	writeClaudeEvent(rc, "message_delta", gin.H{
		"type":  "message_delta",
		"delta": gin.H{"stop_reason": stopReason, "stop_sequence": nil},
		"usage": gin.H{"input_tokens": u.prompt, "output_tokens": u.completion},
	})
	writeClaudeEvent(rc, "message_stop", gin.H{"type": "message_stop"})
	return u
}

// encodeToolCallID packages the function name and thought signature into a single
// OpenAI-compatible tool call ID. Standard clients will echo this ID back to us.
func encodeToolCallID(name, signature string) string {
	data := name + "|" + signature
	encoded := base64.RawURLEncoding.EncodeToString([]byte(data))
	return "call_" + encoded
}

// decodeToolCallID extracts the function name and thought signature from an encoded ID.
func decodeToolCallID(id string) (string, string) {
	if !strings.HasPrefix(id, "call_") {
		return "", ""
	}
	encoded := strings.TrimPrefix(id, "call_")
	decodedBytes, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return "", ""
	}
	parts := strings.SplitN(string(decodedBytes), "|", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "", ""
}

