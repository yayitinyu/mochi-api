// Command mockupstream is a tiny fake OpenAI/Anthropic upstream used for
// local end-to-end testing of the relay. Run: go run ./devtools/mockupstream
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
)

func main() {
	http.HandleFunc("/v1/chat/completions", handleOpenAI)
	http.HandleFunc("/v1/responses", handleResponses)
	http.HandleFunc("/v1/messages", handleClaude)
	http.HandleFunc("/v1/models", handleModels)
	http.HandleFunc("/v1beta/models", handleGeminiModels)
	http.HandleFunc("/v1beta/models/", handleGemini) // :generateContent / :streamGenerateContent
	log.Println("mock upstream listening on :9100")
	log.Fatal(http.ListenAndServe(":9100", nil))
}

func handleResponses(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	model, _ := body["model"].(string)
	stream, _ := body["stream"].(bool)
	response := map[string]any{
		"id": "resp_mock123", "object": "response", "created_at": 1700000000,
		"status": "completed", "model": model,
		"output": []any{map[string]any{
			"id": "msg_mock123", "type": "message", "status": "completed", "role": "assistant",
			"content": []any{map[string]any{"type": "output_text", "text": "你好，我是 Responses mock！", "annotations": []any{}}},
		}},
		"usage": map[string]any{"input_tokens": 12, "output_tokens": 8, "total_tokens": 20},
	}
	if !stream {
		writeJSON(w, response)
		return
	}
	response["status"] = "in_progress"
	response["output"] = []any{}
	response["usage"] = nil
	sequence := 0
	event := func(eventType string, fields map[string]any) string {
		fields["type"] = eventType
		fields["sequence_number"] = sequence
		sequence++
		data, _ := json.Marshal(fields)
		return "event: " + eventType + "\ndata: " + string(data) + "\n\n"
	}
	created := event("response.created", map[string]any{"response": response})
	inProgress := event("response.in_progress", map[string]any{"response": response})
	messageInProgress := map[string]any{
		"id": "msg_mock123", "type": "message", "status": "in_progress",
		"role": "assistant", "content": []any{},
	}
	outputAdded := event("response.output_item.added", map[string]any{
		"output_index": 0, "item": messageInProgress,
	})
	emptyPart := map[string]any{"type": "output_text", "text": "", "annotations": []any{}}
	contentAdded := event("response.content_part.added", map[string]any{
		"item_id": "msg_mock123", "output_index": 0, "content_index": 0, "part": emptyPart,
	})
	delta := event("response.output_text.delta", map[string]any{
		"item_id": "msg_mock123", "output_index": 0, "content_index": 0,
		"delta": "你好，我是 Responses mock！",
	})
	completedPart := map[string]any{
		"type": "output_text", "text": "你好，我是 Responses mock！", "annotations": []any{},
	}
	textDone := event("response.output_text.done", map[string]any{
		"item_id": "msg_mock123", "output_index": 0, "content_index": 0,
		"text": "你好，我是 Responses mock！",
	})
	contentDone := event("response.content_part.done", map[string]any{
		"item_id": "msg_mock123", "output_index": 0, "content_index": 0, "part": completedPart,
	})
	completedMessage := map[string]any{
		"id": "msg_mock123", "type": "message", "status": "completed", "role": "assistant",
		"content": []any{completedPart},
	}
	outputDone := event("response.output_item.done", map[string]any{
		"output_index": 0, "item": completedMessage,
	})
	response["status"] = "completed"
	response["output"] = []any{completedMessage}
	response["usage"] = map[string]any{"input_tokens": 12, "output_tokens": 8, "total_tokens": 20}
	completed := event("response.completed", map[string]any{"response": response})
	sse(w, created, inProgress, outputAdded, contentAdded, delta, textDone, contentDone, outputDone, completed)
}

func handleGeminiModels(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("x-goog-api-key") == "" {
		w.WriteHeader(http.StatusUnauthorized)
		writeJSON(w, map[string]any{"error": map[string]any{"message": "missing key"}})
		return
	}
	writeJSON(w, map[string]any{
		"models": []any{
			map[string]any{"name": "models/gemini-2.0-flash", "supportedGenerationMethods": []any{"generateContent"}},
			map[string]any{"name": "models/gemini-1.5-pro", "supportedGenerationMethods": []any{"generateContent"}},
			map[string]any{"name": "models/embedding-001", "supportedGenerationMethods": []any{"embedContent"}},
		},
	})
}

func handleGemini(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("x-goog-api-key") == "" {
		w.WriteHeader(http.StatusUnauthorized)
		writeJSON(w, map[string]any{"error": map[string]any{"message": "missing key"}})
		return
	}
	stream := strings.Contains(r.URL.Path, "streamGenerateContent") || r.URL.RawQuery == "alt=sse"
	if !stream {
		writeJSON(w, map[string]any{
			"candidates": []any{map[string]any{
				"content": map[string]any{"role": "model", "parts": []any{
					map[string]any{"thought": true, "text": "先确认语言和回答目标。", "thoughtSignature": "mock-thought-signature"},
					map[string]any{"text": "안녕하세요，我是 Gemini mock！"},
				}},
				"finishReason": "STOP",
			}},
			"usageMetadata": map[string]any{"promptTokenCount": 15, "candidatesTokenCount": 9, "thoughtsTokenCount": 4, "totalTokenCount": 28},
		})
		return
	}
	sse(w,
		`data: {"candidates":[{"content":{"role":"model","parts":[{"thought":true,"text":"先确认语言和回答目标。","thoughtSignature":"mock-thought-signature"}]}}],"usageMetadata":{"promptTokenCount":15,"candidatesTokenCount":0,"thoughtsTokenCount":4}}`+"\n\n",
		`data: {"candidates":[{"content":{"role":"model","parts":[{"text":"안녕하세요，Gemini mock 입니다！"}]}}],"usageMetadata":{"promptTokenCount":15,"candidatesTokenCount":9,"thoughtsTokenCount":4}}`+"\n\n",
		`data: {"candidates":[{"content":{"role":"model","parts":[{"text":""}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":15,"candidatesTokenCount":9,"thoughtsTokenCount":4,"totalTokenCount":28}}`+"\n\n",
	)
}

func handleModels(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Authorization") == "" && r.Header.Get("x-api-key") == "" {
		w.WriteHeader(http.StatusUnauthorized)
		writeJSON(w, map[string]any{"error": map[string]any{"message": "missing key"}})
		return
	}
	writeJSON(w, map[string]any{
		"object": "list",
		"data": []any{
			map[string]any{"id": "gpt-4o", "object": "model"},
			map[string]any{"id": "gpt-4o-mini", "object": "model"},
			map[string]any{"id": "claude-3-5-sonnet", "object": "model"},
			map[string]any{"id": "deepseek-chat", "object": "model"},
		},
	})
}

func readBody(r *http.Request) map[string]any {
	var body map[string]any
	_ = json.NewDecoder(r.Body).Decode(&body)
	return body
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func sse(w http.ResponseWriter, chunks ...string) {
	w.Header().Set("Content-Type", "text/event-stream")
	flusher := w.(http.Flusher)
	for _, chunk := range chunks {
		fmt.Fprint(w, chunk)
		flusher.Flush()
	}
}

func handleOpenAI(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	model, _ := body["model"].(string)
	stream, _ := body["stream"].(bool)
	if !stream {
		writeJSON(w, map[string]any{
			"id": "chatcmpl-mock123", "object": "chat.completion", "created": 1700000000,
			"model": model,
			"choices": []any{map[string]any{
				"index":         0,
				"message":       map[string]any{"role": "assistant", "content": "你好，我是 mock 助手！"},
				"finish_reason": "stop",
			}},
			"usage": map[string]any{"prompt_tokens": 19, "completion_tokens": 7, "total_tokens": 26},
		})
		return
	}
	includeUsage := false
	if opts, ok := body["stream_options"].(map[string]any); ok {
		includeUsage, _ = opts["include_usage"].(bool)
	}
	chunkFmt := `data: {"id":"chatcmpl-mock123","object":"chat.completion.chunk","created":1700000000,"model":"` + model + `","choices":[{"index":0,"delta":%s,"finish_reason":%s}]}` + "\n\n"
	chunks := []string{
		fmt.Sprintf(chunkFmt, `{"role":"assistant","content":""}`, "null"),
		fmt.Sprintf(chunkFmt, `{"content":"你好，"}`, "null"),
		fmt.Sprintf(chunkFmt, `{"content":"我是 mock！"}`, "null"),
		fmt.Sprintf(chunkFmt, `{}`, `"stop"`),
	}
	if includeUsage {
		chunks = append(chunks, `data: {"id":"chatcmpl-mock123","object":"chat.completion.chunk","created":1700000000,"model":"`+model+`","choices":[],"usage":{"prompt_tokens":19,"completion_tokens":7,"total_tokens":26}}`+"\n\n")
	}
	chunks = append(chunks, "data: [DONE]\n\n")
	sse(w, chunks...)
}

func handleClaude(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	model, _ := body["model"].(string)
	stream, _ := body["stream"].(bool)
	if !stream {
		writeJSON(w, map[string]any{
			"id": "msg_mock456", "type": "message", "role": "assistant", "model": model,
			"content":     []any{map[string]any{"type": "text", "text": "こんにちは，我是 Claude mock！"}},
			"stop_reason": "end_turn", "stop_sequence": nil,
			"usage": map[string]any{"input_tokens": 17, "output_tokens": 8},
		})
		return
	}
	sse(w,
		`event: message_start`+"\n"+`data: {"type":"message_start","message":{"id":"msg_mock456","type":"message","role":"assistant","model":"`+model+`","content":[],"stop_reason":null,"stop_sequence":null,"usage":{"input_tokens":17,"output_tokens":1}}}`+"\n\n",
		`event: content_block_start`+"\n"+`data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`+"\n\n",
		`event: content_block_delta`+"\n"+`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"こんにちは，"}}`+"\n\n",
		`event: content_block_delta`+"\n"+`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"mock です！"}}`+"\n\n",
		`event: content_block_stop`+"\n"+`data: {"type":"content_block_stop","index":0}`+"\n\n",
		`event: message_delta`+"\n"+`data: {"type":"message_delta","delta":{"stop_reason":"end_turn","stop_sequence":null},"usage":{"output_tokens":8}}`+"\n\n",
		`event: message_stop`+"\n"+`data: {"type":"message_stop"}`+"\n\n",
	)
}
