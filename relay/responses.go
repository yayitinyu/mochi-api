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

// responsesHasHostedTool reports whether a Responses request contains a
// provider-hosted tool rather than a portable function declaration.
func responsesHasHostedTool(body []byte) bool {
	for _, tool := range gjson.GetBytes(body, "tools").Array() {
		if tool.Get("type").String() != "function" {
			return true
		}
	}
	return false
}

// convertRequestResponsesToOpenAIChat converts the portable subset of a
// Responses request to the Chat Completions intermediate used by the Claude
// and Gemini bridges. OpenAI-compatible channels receive the original body.
func convertRequestResponsesToOpenAIChat(body []byte) ([]byte, error) {
	root := gjson.ParseBytes(body)
	out := map[string]any{"model": root.Get("model").String()}
	for _, field := range []string{"temperature", "top_p", "stream", "parallel_tool_calls", "user"} {
		if v := root.Get(field); v.Exists() {
			out[field] = v.Value()
		}
	}
	if v := root.Get("max_output_tokens"); v.Exists() {
		out["max_completion_tokens"] = v.Int()
	}
	if v := root.Get("reasoning"); v.Exists() {
		out["reasoning"] = v.Value()
	}

	messages := make([]map[string]any, 0)
	if instructions := root.Get("instructions"); instructions.Type == gjson.String && instructions.String() != "" {
		messages = append(messages, map[string]any{"role": "system", "content": instructions.String()})
	}
	input := root.Get("input")
	if input.Type == gjson.String {
		messages = append(messages, map[string]any{"role": "user", "content": input.String()})
	} else {
		for _, item := range input.Array() {
			switch item.Get("type").String() {
			case "function_call":
				call := map[string]any{
					"id":   item.Get("call_id").String(),
					"type": "function",
					"function": map[string]any{
						"name": item.Get("name").String(), "arguments": item.Get("arguments").String(),
					},
				}
				if extra := item.Get("extra_content"); extra.Exists() {
					call["extra_content"] = extra.Value()
				}
				if len(messages) > 0 && messages[len(messages)-1]["role"] == "assistant" {
					if calls, ok := messages[len(messages)-1]["tool_calls"].([]any); ok {
						messages[len(messages)-1]["tool_calls"] = append(calls, call)
						continue
					}
				}
				messages = append(messages, map[string]any{
					"role": "assistant", "content": nil, "tool_calls": []any{call},
				})
			case "function_call_output":
				messages = append(messages, map[string]any{
					"role": "tool", "tool_call_id": item.Get("call_id").String(),
					"content": responseOutputText(item.Get("output")),
				})
			case "reasoning":
				// Reasoning items carry provider state, not a user-visible message.
				// Direct Responses upstreams receive them unchanged.
			default: // message items may omit type
				role := item.Get("role").String()
				if role == "" {
					role = "user"
				}
				messages = append(messages, map[string]any{
					"role": role, "content": responseContentToOpenAI(item.Get("content")),
				})
			}
		}
	}
	out["messages"] = messages

	if tools := root.Get("tools").Array(); len(tools) > 0 {
		converted := make([]any, 0, len(tools))
		for _, tool := range tools {
			if tool.Get("type").String() != "function" {
				converted = append(converted, tool.Value())
				continue
			}
			fn := map[string]any{
				"name": tool.Get("name").String(), "description": tool.Get("description").String(),
				"parameters": tool.Get("parameters").Value(),
			}
			if strict := tool.Get("strict"); strict.Exists() {
				fn["strict"] = strict.Bool()
			}
			converted = append(converted, map[string]any{"type": "function", "function": fn})
		}
		out["tools"] = converted
	}
	if choice := root.Get("tool_choice"); choice.Exists() {
		if choice.Get("type").String() == "function" {
			out["tool_choice"] = map[string]any{
				"type": "function", "function": map[string]any{"name": choice.Get("name").String()},
			}
		} else {
			out["tool_choice"] = choice.Value()
		}
	}
	if format := root.Get("text.format"); format.Exists() {
		switch format.Get("type").String() {
		case "json_schema":
			out["response_format"] = map[string]any{
				"type": "json_schema",
				"json_schema": map[string]any{
					"name": format.Get("name").String(), "schema": format.Get("schema").Value(),
					"strict": format.Get("strict").Bool(),
				},
			}
		case "json_object":
			out["response_format"] = map[string]any{"type": "json_object"}
		}
	}
	return json.Marshal(out)
}

func responseContentToOpenAI(content gjson.Result) any {
	if content.Type == gjson.String {
		return content.String()
	}
	parts := make([]any, 0, len(content.Array()))
	for _, part := range content.Array() {
		switch part.Get("type").String() {
		case "input_image":
			if url := part.Get("image_url").String(); url != "" {
				parts = append(parts, map[string]any{"type": "image_url", "image_url": map[string]any{"url": url}})
			}
		default: // input_text and output_text
			if text := part.Get("text").String(); text != "" {
				parts = append(parts, map[string]any{"type": "text", "text": text})
			}
		}
	}
	return parts
}

func responseOutputText(output gjson.Result) string {
	if output.Type == gjson.String {
		return output.String()
	}
	var text strings.Builder
	for _, part := range output.Array() {
		text.WriteString(part.Get("text").String())
	}
	if text.Len() > 0 {
		return text.String()
	}
	data, _ := json.Marshal(output.Value())
	return string(data)
}

// convertResponseOpenAIToResponses maps a Chat Completions response to the
// Responses output-item model for non-Responses upstreams.
func convertResponseOpenAIToResponses(body []byte) ([]byte, error) {
	root := gjson.ParseBytes(body)
	choice := root.Get("choices.0")
	message := choice.Get("message")
	output := make([]any, 0)
	if reasoning := message.Get("reasoning_content").String(); reasoning != "" {
		item := map[string]any{
			"id": "rs_" + common.GenerateKey(24), "type": "reasoning",
			"summary": []any{map[string]any{"type": "summary_text", "text": reasoning}},
		}
		if extra := message.Get("extra_content"); extra.Exists() {
			item["extra_content"] = extra.Value()
		}
		output = append(output, item)
	}
	if text := message.Get("content").String(); text != "" {
		output = append(output, map[string]any{
			"id": "msg_" + common.GenerateKey(24), "type": "message", "status": "completed",
			"role": "assistant", "content": []any{map[string]any{
				"type": "output_text", "text": text, "annotations": []any{},
			}},
		})
	}
	for _, call := range message.Get("tool_calls").Array() {
		item := map[string]any{
			"id": "fc_" + common.GenerateKey(24), "type": "function_call", "status": "completed",
			"call_id": call.Get("id").String(), "name": call.Get("function.name").String(),
			"arguments": call.Get("function.arguments").String(),
		}
		if extra := call.Get("extra_content"); extra.Exists() {
			item["extra_content"] = extra.Value()
		}
		output = append(output, item)
	}

	status := "completed"
	var incomplete any
	if choice.Get("finish_reason").String() == "length" {
		status = "incomplete"
		incomplete = map[string]any{"reason": "max_output_tokens"}
	}
	u := root.Get("usage")
	reasoningTokens := u.Get("completion_tokens_details.reasoning_tokens").Int()
	response := responseObject(
		"resp_"+common.GenerateKey(24), root.Get("created").Int(), root.Get("model").String(),
		status, output, map[string]any{
			"input_tokens":          u.Get("prompt_tokens").Int(),
			"input_tokens_details":  map[string]any{"cached_tokens": u.Get("prompt_tokens_details.cached_tokens").Int()},
			"output_tokens":         u.Get("completion_tokens").Int(),
			"output_tokens_details": map[string]any{"reasoning_tokens": reasoningTokens},
			"total_tokens":          u.Get("total_tokens").Int(),
		},
	)
	response["incomplete_details"] = incomplete
	return json.Marshal(response)
}

func responseObject(id string, created int64, modelName, status string, output []any, usageValue any) map[string]any {
	if created == 0 {
		created = time.Now().Unix()
	}
	if output == nil {
		output = []any{}
	}
	return map[string]any{
		"id": id, "object": "response", "created_at": created, "status": status,
		"error": nil, "incomplete_details": nil, "instructions": nil, "max_output_tokens": nil,
		"model": modelName, "output": output, "parallel_tool_calls": true,
		"previous_response_id": nil, "reasoning": map[string]any{"effort": nil, "summary": "auto"},
		"store": false, "temperature": 1.0, "text": map[string]any{"format": map[string]any{"type": "text"}},
		"tool_choice": "auto", "tools": []any{}, "top_p": 1.0, "truncation": "disabled",
		"usage": usageValue, "user": nil, "metadata": map[string]any{},
	}
}

// streamResponsesToResponses forwards a native Responses stream byte-for-byte
// while capturing its completed usage for dashboard statistics.
func streamResponsesToResponses(rc *relayContext, resp *http.Response) usage {
	setSSEHeaders(rc.c)
	var u usage
	var outputText strings.Builder
	scanSSE(resp.Body, func(line string) {
		_, _ = rc.c.Writer.WriteString(line + "\n")
		if line == "" {
			rc.c.Writer.Flush()
			return
		}
		payload, ok := strings.CutPrefix(line, "data:")
		if !ok {
			return
		}
		event := gjson.Parse(strings.TrimSpace(payload))
		switch event.Get("type").String() {
		case "response.output_text.delta":
			outputText.WriteString(event.Get("delta").String())
		case "response.completed":
			usageField := event.Get("response.usage")
			u.prompt = int(usageField.Get("input_tokens").Int())
			u.completion = int(usageField.Get("output_tokens").Int())
		}
	})
	rc.c.Writer.Flush()
	if u.completion == 0 && outputText.Len() > 0 {
		u.completion = estimateTokens(outputText.String())
		u.estimated = true
	}
	return u
}

type responseStream struct {
	rc               *relayContext
	id               string
	created          int64
	sequence         int
	output           []any
	reasoningID      string
	reasoningIndex   int
	reasoningText    strings.Builder
	reasoningExtra   any
	messageID        string
	messageIndex     int
	messageText      strings.Builder
	finishIncomplete bool
}

func newResponseStream(rc *relayContext) *responseStream {
	s := &responseStream{rc: rc, id: "resp_" + common.GenerateKey(24), created: time.Now().Unix()}
	s.emit(gin.H{"type": "response.created", "response": responseObject(s.id, s.created, rc.modelName, "in_progress", nil, nil)})
	s.emit(gin.H{"type": "response.in_progress", "response": responseObject(s.id, s.created, rc.modelName, "in_progress", nil, nil)})
	return s
}

func (s *responseStream) emit(event gin.H) {
	event["sequence_number"] = s.sequence
	s.sequence++
	data, _ := json.Marshal(event)
	eventType, _ := event["type"].(string)
	_, _ = s.rc.c.Writer.WriteString("event: " + eventType + "\ndata: " + string(data) + "\n\n")
	s.rc.c.Writer.Flush()
}

func (s *responseStream) addReasoning(text, signature string) {
	if s.reasoningID == "" {
		s.reasoningID = "rs_" + common.GenerateKey(24)
		s.reasoningIndex = len(s.output)
		s.output = append(s.output, nil)
		s.emit(gin.H{
			"type": "response.output_item.added", "output_index": s.reasoningIndex,
			"item": gin.H{"id": s.reasoningID, "type": "reasoning", "summary": []any{}},
		})
		s.emit(gin.H{
			"type": "response.reasoning_summary_part.added", "item_id": s.reasoningID,
			"output_index": s.reasoningIndex, "summary_index": 0,
			"part": gin.H{"type": "summary_text", "text": ""},
		})
	}
	if signature != "" {
		s.reasoningExtra = map[string]any{"google": map[string]any{"thought_signature": signature}}
	}
	if text == "" {
		return
	}
	s.reasoningText.WriteString(text)
	s.emit(gin.H{
		"type": "response.reasoning_summary_text.delta", "item_id": s.reasoningID,
		"output_index": s.reasoningIndex, "summary_index": 0, "delta": text,
	})
}

func (s *responseStream) addText(text string) {
	if text == "" {
		return
	}
	if s.messageID == "" {
		s.messageID = "msg_" + common.GenerateKey(24)
		s.messageIndex = len(s.output)
		s.output = append(s.output, nil)
		s.emit(gin.H{
			"type": "response.output_item.added", "output_index": s.messageIndex,
			"item": gin.H{"id": s.messageID, "type": "message", "status": "in_progress", "role": "assistant", "content": []any{}},
		})
		s.emit(gin.H{
			"type": "response.content_part.added", "item_id": s.messageID,
			"output_index": s.messageIndex, "content_index": 0,
			"part": gin.H{"type": "output_text", "text": "", "annotations": []any{}},
		})
	}
	s.messageText.WriteString(text)
	s.emit(gin.H{
		"type": "response.output_text.delta", "item_id": s.messageID,
		"output_index": s.messageIndex, "content_index": 0, "delta": text,
	})
}

func (s *responseStream) addFunctionCall(callID, name, arguments, signature string) {
	if callID == "" {
		callID = "call_" + common.GenerateKey(16)
	}
	index := len(s.output)
	itemID := "fc_" + common.GenerateKey(24)
	item := map[string]any{
		"id": itemID, "type": "function_call", "status": "in_progress",
		"call_id": callID, "name": name, "arguments": "",
	}
	if signature != "" {
		item["extra_content"] = map[string]any{"google": map[string]any{"thought_signature": signature}}
	}
	s.emit(gin.H{"type": "response.output_item.added", "output_index": index, "item": item})
	if arguments != "" {
		s.emit(gin.H{
			"type": "response.function_call_arguments.delta", "item_id": itemID,
			"output_index": index, "delta": arguments,
		})
	}
	s.emit(gin.H{
		"type": "response.function_call_arguments.done", "item_id": itemID,
		"output_index": index, "arguments": arguments,
	})
	item["status"] = "completed"
	item["arguments"] = arguments
	s.output = append(s.output, item)
	s.emit(gin.H{"type": "response.output_item.done", "output_index": index, "item": item})
}

func (s *responseStream) finish(u usage, reasoningTokens int) {
	if s.reasoningID != "" {
		text := s.reasoningText.String()
		s.emit(gin.H{
			"type": "response.reasoning_summary_text.done", "item_id": s.reasoningID,
			"output_index": s.reasoningIndex, "summary_index": 0, "text": text,
		})
		s.emit(gin.H{
			"type": "response.reasoning_summary_part.done", "item_id": s.reasoningID,
			"output_index": s.reasoningIndex, "summary_index": 0,
			"part": gin.H{"type": "summary_text", "text": text},
		})
		item := map[string]any{
			"id": s.reasoningID, "type": "reasoning",
			"summary": []any{map[string]any{"type": "summary_text", "text": text}},
		}
		if s.reasoningExtra != nil {
			item["extra_content"] = s.reasoningExtra
		}
		s.output[s.reasoningIndex] = item
		s.emit(gin.H{"type": "response.output_item.done", "output_index": s.reasoningIndex, "item": item})
	}
	if s.messageID != "" {
		text := s.messageText.String()
		part := map[string]any{"type": "output_text", "text": text, "annotations": []any{}}
		s.emit(gin.H{
			"type": "response.output_text.done", "item_id": s.messageID,
			"output_index": s.messageIndex, "content_index": 0, "text": text,
		})
		s.emit(gin.H{
			"type": "response.content_part.done", "item_id": s.messageID,
			"output_index": s.messageIndex, "content_index": 0, "part": part,
		})
		item := map[string]any{
			"id": s.messageID, "type": "message", "status": "completed", "role": "assistant",
			"content": []any{part},
		}
		s.output[s.messageIndex] = item
		s.emit(gin.H{"type": "response.output_item.done", "output_index": s.messageIndex, "item": item})
	}
	status := "completed"
	var incomplete any
	if s.finishIncomplete {
		status = "incomplete"
		incomplete = map[string]any{"reason": "max_output_tokens"}
	}
	uValue := map[string]any{
		"input_tokens": u.prompt, "input_tokens_details": map[string]any{"cached_tokens": 0},
		"output_tokens": u.completion, "output_tokens_details": map[string]any{"reasoning_tokens": reasoningTokens},
		"total_tokens": u.prompt + u.completion,
	}
	response := responseObject(s.id, s.created, s.rc.modelName, status, s.output, uValue)
	response["incomplete_details"] = incomplete
	eventType := "response.completed"
	if status == "incomplete" {
		eventType = "response.incomplete"
	}
	s.emit(gin.H{"type": eventType, "response": response})
}

func streamGeminiToResponses(rc *relayContext, resp *http.Response) usage {
	setSSEHeaders(rc.c)
	stream := newResponseStream(rc)
	var u usage
	reasoningTokens := 0
	scanSSE(resp.Body, func(line string) {
		payload, ok := strings.CutPrefix(line, "data:")
		if !ok {
			return
		}
		event := gjson.Parse(strings.TrimSpace(payload))
		if metadata := event.Get("usageMetadata"); metadata.Exists() {
			u.prompt = int(metadata.Get("promptTokenCount").Int())
			reasoningTokens = int(metadata.Get("thoughtsTokenCount").Int())
			u.completion = int(metadata.Get("candidatesTokenCount").Int()) + reasoningTokens
		}
		candidate := event.Get("candidates.0")
		for _, part := range candidate.Get("content.parts").Array() {
			signature := part.Get("thoughtSignature").String()
			if part.Get("thought").Bool() {
				stream.addReasoning(part.Get("text").String(), signature)
				continue
			}
			if text := part.Get("text").String(); text != "" {
				stream.addText(text)
			}
			if call := part.Get("functionCall"); call.Exists() {
				args, _ := json.Marshal(call.Get("args").Value())
				stream.addFunctionCall(call.Get("id").String(), call.Get("name").String(), string(args), signature)
			}
		}
		if candidate.Get("finishReason").String() == "MAX_TOKENS" {
			stream.finishIncomplete = true
		}
	})
	if u.completion == 0 {
		u.completion = estimateTokens(stream.messageText.String()) + estimateTokens(stream.reasoningText.String())
		u.estimated = true
	}
	stream.finish(u, reasoningTokens)
	return u
}

func streamClaudeToResponses(rc *relayContext, resp *http.Response) usage {
	setSSEHeaders(rc.c)
	stream := newResponseStream(rc)
	var u usage
	type blockState struct {
		kind, id, name, signature string
		arguments                 strings.Builder
	}
	blocks := map[int64]*blockState{}
	scanSSE(resp.Body, func(line string) {
		payload, ok := strings.CutPrefix(line, "data:")
		if !ok {
			return
		}
		payload = strings.TrimSpace(payload)
		captureClaudeUsage(payload, &u)
		event := gjson.Parse(payload)
		index := event.Get("index").Int()
		switch event.Get("type").String() {
		case "content_block_start":
			block := event.Get("content_block")
			blocks[index] = &blockState{
				kind: block.Get("type").String(), id: block.Get("id").String(), name: block.Get("name").String(),
			}
		case "content_block_delta":
			delta := event.Get("delta")
			switch delta.Get("type").String() {
			case "text_delta":
				stream.addText(delta.Get("text").String())
			case "thinking_delta":
				stream.addReasoning(delta.Get("thinking").String(), "")
			case "signature_delta":
				if state := blocks[index]; state != nil {
					state.signature += delta.Get("signature").String()
					stream.addReasoning("", state.signature)
				}
			case "input_json_delta":
				if state := blocks[index]; state != nil {
					state.arguments.WriteString(delta.Get("partial_json").String())
				}
			}
		case "content_block_stop":
			if state := blocks[index]; state != nil && state.kind == "tool_use" {
				stream.addFunctionCall(state.id, state.name, state.arguments.String(), "")
			}
		case "message_delta":
			if event.Get("delta.stop_reason").String() == "max_tokens" {
				stream.finishIncomplete = true
			}
		}
	})
	if u.completion == 0 {
		u.completion = estimateTokens(stream.messageText.String()) + estimateTokens(stream.reasoningText.String())
		u.estimated = true
	}
	stream.finish(u, 0)
	return u
}
