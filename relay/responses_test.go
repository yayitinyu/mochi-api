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

	"mochi-api/model"
)

func TestResponsesRequestConvertsReasoningAndWebSearchForGemini(t *testing.T) {
	responsesBody := []byte(`{
		"model":"gemini-3-flash-preview",
		"input":"What happened today?",
		"reasoning":{"effort":"high","summary":"auto"},
		"tools":[
			{"type":"web_search"},
			{"type":"function","name":"lookup","description":"Lookup an ID","parameters":{"type":"object"}}
		]
	}`)

	chatBody, err := convertRequestResponsesToOpenAIChat(responsesBody)
	require.NoError(t, err)
	require.Equal(t, "What happened today?", gjson.GetBytes(chatBody, "messages.0.content").String())
	require.Equal(t, "web_search", gjson.GetBytes(chatBody, "tools.0.type").String())

	geminiBody, err := convertRequestOpenAIToGemini(chatBody)
	require.NoError(t, err)
	require.True(t, gjson.GetBytes(geminiBody, "generationConfig.thinkingConfig.includeThoughts").Bool())
	require.Equal(t, "high", gjson.GetBytes(geminiBody, "generationConfig.thinkingConfig.thinkingLevel").String())
	require.True(t, gjson.GetBytes(geminiBody, "tools.0.googleSearch").IsObject())
	require.Equal(t, "lookup", gjson.GetBytes(geminiBody, "tools.1.functionDeclarations.0.name").String())
}

func TestGeminiFunctionSchemaDropsUnsupportedOpenAIKeywords(t *testing.T) {
	body := []byte(`{
		"model":"gemini-2.5-flash",
		"messages":[{"role":"user","content":"Search for mochi"}],
		"tools":[{"type":"function","function":{
			"name":"search",
			"description":"Search the web",
			"parameters":{
				"$schema":"https://json-schema.org/draft/2020-12/schema",
				"type":"object",
				"additionalProperties":false,
				"properties":{
					"filters":{
						"type":"object",
						"additionalProperties":false,
						"properties":{"query":{"type":"string"}},
						"oneOf":[{
							"type":"object",
							"$schema":"https://json-schema.org/draft/2020-12/schema",
							"additionalProperties":false
						}]
					}
				},
				"required":["filters"]
			}
		}}]
	}`)

	converted, err := convertRequestOpenAIToGemini(body)
	require.NoError(t, err)
	require.NotContains(t, string(converted), `"$schema"`)
	require.NotContains(t, string(converted), `"additionalProperties"`)
	require.Equal(t, "string", gjson.GetBytes(converted,
		"tools.0.functionDeclarations.0.parameters.properties.filters.properties.query.type").String())
}

func TestOpenAIChatNonStreamConvertsToResponses(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	rc := &relayContext{
		c: context, clientFormat: FormatResponses, upstreamFormat: FormatOpenAI, modelName: "chat-only-model",
	}
	upstream := &http.Response{Body: io.NopCloser(strings.NewReader(`{
		"id":"chatcmpl-fallback","created":1700000000,"model":"chat-only-model",
		"choices":[{"message":{"role":"assistant","content":"Fallback answer"},"finish_reason":"stop"}],
		"usage":{"prompt_tokens":4,"completion_tokens":2,"total_tokens":6}
	}`))}

	gotUsage := dispatchNonStream(rc, upstream)
	require.Equal(t, "response", gjson.GetBytes(recorder.Body.Bytes(), "object").String())
	require.Equal(t, "Fallback answer", gjson.GetBytes(recorder.Body.Bytes(), "output.0.content.0.text").String())
	require.Equal(t, 4, gotUsage.prompt)
	require.Equal(t, 2, gotUsage.completion)
}

func TestOpenAIChatStreamConvertsToResponsesEvents(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	rc := &relayContext{
		c: context, clientFormat: FormatResponses, upstreamFormat: FormatOpenAI, modelName: "chat-only-model",
	}
	upstream := &http.Response{Body: io.NopCloser(strings.NewReader(
		`data: {"choices":[{"delta":{"role":"assistant"},"finish_reason":null}]}` + "\n\n" +
			`data: {"choices":[{"delta":{"content":"Fallback answer"},"finish_reason":"stop"}]}` + "\n\n" +
			`data: {"choices":[],"usage":{"prompt_tokens":4,"completion_tokens":2,"total_tokens":6}}` + "\n\n" +
			`data: [DONE]` + "\n\n",
	))}

	gotUsage := dispatchStream(rc, upstream)
	body := recorder.Body.String()
	require.Contains(t, body, "event: response.created")
	require.Contains(t, body, "event: response.output_text.delta")
	require.Contains(t, body, "event: response.completed")
	require.NotContains(t, body, "chat.completion.chunk")
	require.NotContains(t, body, `"type":"reasoning"`)
	require.Equal(t, 4, gotUsage.prompt)
	require.Equal(t, 2, gotUsage.completion)
}

func TestUnsupportedResponsesMethodRetriesChatCompletions(t *testing.T) {
	var paths []string
	var chatBody []byte
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		if r.URL.Path == "/v1/responses" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":{"message":"Model does not support responses method.","type":"invalid_request_error"}}`))
			return
		}
		chatBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id":"chatcmpl-fallback","created":1700000000,"model":"chat-only-model",
			"choices":[{"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],
			"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}
		}`))
	}))
	defer upstream.Close()

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	rc := &relayContext{
		c: context, clientFormat: FormatResponses, upstreamFormat: FormatResponses,
		channel:   &model.Channel{Type: model.ChannelTypeOpenAI, BaseURL: upstream.URL, ApiKey: "test"},
		modelName: "chat-only-model",
	}
	clientBody := []byte(`{"model":"chat-only-model","input":"hello"}`)

	resp, err := sendUpstreamWithResponsesFallback(rc, clientBody, clientBody)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, FormatOpenAI, rc.upstreamFormat)
	require.Equal(t, []string{"/v1/responses", "/v1/chat/completions"}, paths)
	require.Equal(t, "hello", gjson.GetBytes(chatBody, "messages.0.content").String())
}

func TestResponsesFallbackOnlyMatchesUnsupportedMethodErrors(t *testing.T) {
	require.True(t, isUnsupportedResponsesError(http.StatusBadRequest, []byte(
		`{"error":{"message":"Model does not support responses method."}}`,
	)))
	for _, testCase := range []struct {
		status int
		body   string
	}{
		{http.StatusUnauthorized, `{"error":{"message":"Responses authentication failed"}}`},
		{http.StatusTooManyRequests, `{"error":{"message":"Responses rate limit exceeded"}}`},
		{http.StatusNotFound, `{"error":{"message":"Model was not found"}}`},
		{http.StatusBadRequest, `{"error":{"message":"Invalid Responses input"}}`},
	} {
		require.False(t, isUnsupportedResponsesError(testCase.status, []byte(testCase.body)))
	}
}

func TestGeminiThoughtsAndSignaturesBecomeOpenAIExtensions(t *testing.T) {
	body := []byte(`{
		"candidates":[{"content":{"parts":[
			{"thought":true,"text":"I should verify this.","thoughtSignature":"reason-sig"},
			{"text":"Final answer."},
			{"functionCall":{"id":"call-1","name":"lookup","args":{"id":7}},"thoughtSignature":"tool-sig"}
		]},"finishReason":"STOP"}],
		"usageMetadata":{"promptTokenCount":11,"candidatesTokenCount":7,"thoughtsTokenCount":5,"totalTokenCount":23}
	}`)

	converted, err := convertResponseGeminiToOpenAI(body, "gemini-3-flash-preview")
	require.NoError(t, err)
	require.Equal(t, "I should verify this.", gjson.GetBytes(converted, "choices.0.message.reasoning_content").String())
	require.Equal(t, "Final answer.", gjson.GetBytes(converted, "choices.0.message.content").String())
	require.Equal(t, "reason-sig", gjson.GetBytes(converted, "choices.0.message.extra_content.google.thought_signature").String())
	// Native Gemini ids are replaced with an encoded id that preserves name+signature
	// for clients that strip extra_content on the next turn.
	expectedID := encodeToolCallID("lookup", "tool-sig")
	require.Equal(t, expectedID, gjson.GetBytes(converted, "choices.0.message.tool_calls.0.id").String())
	require.Equal(t, "tool-sig", gjson.GetBytes(converted, "choices.0.message.tool_calls.0.extra_content.google.thought_signature").String())
	require.Equal(t, int64(12), gjson.GetBytes(converted, "usage.completion_tokens").Int())
	require.Equal(t, int64(5), gjson.GetBytes(converted, "usage.completion_tokens_details.reasoning_tokens").Int())
}

func TestGeminiRequestReturnsToolThoughtSignatureToOriginalPart(t *testing.T) {
	body := []byte(`{
		"model":"gemini-3-flash-preview",
		"messages":[
			{"role":"assistant","tool_calls":[{"id":"call-1","type":"function","function":{"name":"lookup","arguments":"{\"id\":7}"},"extra_content":{"google":{"thought_signature":"tool-sig"}}}]},
			{"role":"tool","tool_call_id":"call-1","content":"done"}
		]
	}`)

	converted, err := convertRequestOpenAIToGemini(body)
	require.NoError(t, err)
	require.Equal(t, "call-1", gjson.GetBytes(converted, "contents.0.parts.0.functionCall.id").String())
	require.Equal(t, "tool-sig", gjson.GetBytes(converted, "contents.0.parts.0.thoughtSignature").String())
	require.Equal(t, "lookup", gjson.GetBytes(converted, "contents.1.parts.0.functionResponse.name").String())
	require.Equal(t, "call-1", gjson.GetBytes(converted, "contents.1.parts.0.functionResponse.id").String())
}

func TestChatCompletionConvertsToResponsesItems(t *testing.T) {
	body := []byte(`{
		"id":"chatcmpl-test","created":1700000000,"model":"gemini-test",
		"choices":[{"message":{"role":"assistant","content":"Answer","reasoning_content":"Reason","tool_calls":[{"id":"call-1","type":"function","function":{"name":"lookup","arguments":"{}"}}]},"finish_reason":"stop"}],
		"usage":{"prompt_tokens":10,"completion_tokens":8,"total_tokens":18,"completion_tokens_details":{"reasoning_tokens":3}}
	}`)

	converted, err := convertResponseOpenAIToResponses(body)
	require.NoError(t, err)
	require.Equal(t, "response", gjson.GetBytes(converted, "object").String())
	require.Equal(t, "reasoning", gjson.GetBytes(converted, "output.0.type").String())
	require.Equal(t, "Reason", gjson.GetBytes(converted, "output.0.summary.0.text").String())
	require.Equal(t, "message", gjson.GetBytes(converted, "output.1.type").String())
	require.Equal(t, "Answer", gjson.GetBytes(converted, "output.1.content.0.text").String())
	require.Equal(t, "function_call", gjson.GetBytes(converted, "output.2.type").String())
	require.Equal(t, int64(3), gjson.GetBytes(converted, "usage.output_tokens_details.reasoning_tokens").Int())
}

func TestGeminiThoughtsStreamAsResponsesReasoningEvents(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	rc := &relayContext{c: context, clientFormat: FormatResponses, modelName: "gemini-3-flash-preview"}
	upstream := &http.Response{
		Body: io.NopCloser(strings.NewReader(
			`data: {"candidates":[{"content":{"parts":[{"thought":true,"text":"Checking","thoughtSignature":"sig"}]}}]}` + "\n\n" +
				`data: {"candidates":[{"content":{"parts":[{"text":"Answer"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":4,"candidatesTokenCount":5,"thoughtsTokenCount":3}}` + "\n\n",
		)),
	}

	gotUsage := streamGeminiToResponses(rc, upstream)
	body := recorder.Body.String()
	require.Contains(t, body, "event: response.reasoning_summary_text.delta")
	require.Contains(t, body, "event: response.output_text.delta")
	require.Contains(t, body, "event: response.completed")
	require.NotContains(t, body, `"output":[null`)
	require.Equal(t, 4, gotUsage.prompt)
	require.Equal(t, 8, gotUsage.completion)
}

func TestGeminiThoughtsStreamAsChatReasoningContent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	rc := &relayContext{c: context, clientFormat: FormatOpenAI, modelName: "gemini-3-flash-preview", clientWantsUsage: true}
	upstream := &http.Response{
		Body: io.NopCloser(strings.NewReader(
			`data: {"candidates":[{"content":{"parts":[{"thought":true,"text":"Checking","thoughtSignature":"sig"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":4,"candidatesTokenCount":2,"thoughtsTokenCount":3}}` + "\n\n",
		)),
	}

	gotUsage := streamGeminiToOpenAI(rc, upstream)
	body := recorder.Body.String()
	require.Contains(t, body, `"reasoning_content":"Checking"`)
	require.Contains(t, body, `"thought_signature":"sig"`)
	require.Contains(t, body, `"reasoning_tokens":3`)
	require.Equal(t, 5, gotUsage.completion)
}

func TestGeminiToolCallEncodingAndSignaturePreservation(t *testing.T) {
	// 1. Simulate a Gemini response containing a tool call and a thought signature
	geminiResponse := []byte(`{
		"candidates":[{"content":{"parts":[
			{"functionCall":{"name":"default_api:web_search","args":{"query":"weather in Tokyo"}},"thoughtSignature":"my-thought-sig-123"}
		]},"finishReason":"STOP"}],
		"usageMetadata":{"promptTokenCount":10,"candidatesTokenCount":5}
	}`)

	openaiResponseBytes, err := convertResponseGeminiToOpenAI(geminiResponse, "gemini-3-flash-preview")
	require.NoError(t, err)

	toolCallID := gjson.GetBytes(openaiResponseBytes, "choices.0.message.tool_calls.0.id").String()
	require.True(t, strings.HasPrefix(toolCallID, "call_"))

	// 2. Simulate the client sending this tool call and the tool execution result back in the next turn
	clientRequest := []byte(`{
		"model": "gemini-3-flash-preview",
		"messages": [
			{
				"role": "assistant",
				"tool_calls": [
					{
						"id": "` + toolCallID + `",
						"type": "function",
						"function": {
							"name": "default_api:web_search",
							"arguments": "{\"query\":\"weather in Tokyo\"}"
						}
					}
				]
			},
			{
				"role": "tool",
				"tool_call_id": "` + toolCallID + `",
				"content": "Tokyo weather is sunny."
			}
		]
	}`)

	convertedRequestBytes, err := convertRequestOpenAIToGemini(clientRequest)
	require.NoError(t, err)

	// Verify that the assistant message part successfully restored the thoughtSignature
	require.Equal(t, "my-thought-sig-123", gjson.GetBytes(convertedRequestBytes, "contents.0.parts.0.thoughtSignature").String())
	// Verify that the tool response part successfully resolved the name even if "name" is missing in OpenAI tool message
	require.Equal(t, "default_api:web_search", gjson.GetBytes(convertedRequestBytes, "contents.1.parts.0.functionResponse.name").String())
}

func TestGeminiToolCallStreamAccumulatesThoughtSignature(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	rc := &relayContext{c: context, clientFormat: FormatOpenAI, modelName: "gemini-3-flash-preview"}
	upstream := &http.Response{
		Body: io.NopCloser(strings.NewReader(
			`data: {"candidates":[{"content":{"parts":[{"thought":true,"text":"Checking","thoughtSignature":"sig-123"}]}}]}` + "\n\n" +
				`data: {"candidates":[{"content":{"parts":[{"functionCall":{"name":"default_api:web_search","args":{"query":"weather"}}]}}],"finishReason":"STOP","usageMetadata":{"promptTokenCount":4,"candidatesTokenCount":5}}` + "\n\n",
		)),
	}

	_ = streamGeminiToOpenAI(rc, upstream)
	body := recorder.Body.String()

	// The tool call ID should be encoded from "default_api:web_search" and "sig-123"
	expectedID := encodeToolCallID("default_api:web_search", "sig-123")
	require.Contains(t, body, `"id":"`+expectedID+`"`)
	require.Contains(t, body, `"name":"default_api:web_search"`)
	require.Contains(t, body, `"thought_signature":"sig-123"`)
}

func TestGeminiMessageGrouping(t *testing.T) {
	body := []byte(`{
		"model": "gemini-3-flash-preview",
		"messages": [
			{"role": "user", "content": "hello"},
			{"role": "user", "content": "world"},
			{
				"role": "assistant",
				"tool_calls": [
					{"id": "call-1", "type": "function", "function": {"name": "search", "arguments": "{}"}}
				]
			},
			{
				"role": "assistant",
				"content": "thinking..."
			},
			{"role": "tool", "tool_call_id": "call-1", "content": "result 1"},
			{"role": "tool", "tool_call_id": "call-1", "content": "result 2"}
		]
	}`)

	converted, err := convertRequestOpenAIToGemini(body)
	require.NoError(t, err)

	// Verify that there are only 3 contents in the converted request (User -> Model -> User)
	contentsResult := gjson.GetBytes(converted, "contents")
	require.Equal(t, int64(3), contentsResult.Get("#").Int())

	// contents[0] (User): has 2 parts ("hello" and "world")
	require.Equal(t, "user", contentsResult.Get("0.role").String())
	require.Equal(t, int64(2), contentsResult.Get("0.parts.#").Int())
	require.Equal(t, "hello", contentsResult.Get("0.parts.0.text").String())
	require.Equal(t, "world", contentsResult.Get("0.parts.1.text").String())

	// contents[1] (Model): has 2 parts (functionCall, text)
	require.Equal(t, "model", contentsResult.Get("1.role").String())
	require.Equal(t, int64(2), contentsResult.Get("1.parts.#").Int())

	// contents[2] (User - tool responses): has 2 functionResponse parts
	require.Equal(t, "user", contentsResult.Get("2.role").String())
	require.Equal(t, int64(2), contentsResult.Get("2.parts.#").Int())
	require.Equal(t, "search", contentsResult.Get("2.parts.0.functionResponse.name").String())
	require.Equal(t, "search", contentsResult.Get("2.parts.1.functionResponse.name").String())
}

func TestGeminiNativeFunctionCallIDStillEncodesSignature(t *testing.T) {
	// Gemini often returns both a native functionCall.id and thoughtSignature on the
	// same part. Using the native id as-is would drop the signature once clients strip
	// extra_content, which triggers Gemini's thought_signature requirement error.
	geminiResponse := []byte(`{
		"candidates":[{"content":{"parts":[
			{"functionCall":{"id":"native-fc-42","name":"default_api:web_search","args":{"query":"weather"}},"thoughtSignature":"sig-native-42"}
		]},"finishReason":"STOP"}],
		"usageMetadata":{"promptTokenCount":3,"candidatesTokenCount":2}
	}`)

	openaiBytes, err := convertResponseGeminiToOpenAI(geminiResponse, "gemini-3-flash-preview")
	require.NoError(t, err)

	toolCallID := gjson.GetBytes(openaiBytes, "choices.0.message.tool_calls.0.id").String()
	require.Equal(t, encodeToolCallID("default_api:web_search", "sig-native-42"), toolCallID)
	require.NotEqual(t, "native-fc-42", toolCallID)

	// Client replays history without extra_content (common OpenAI-compatible behavior).
	clientRequest := []byte(`{
		"model":"gemini-3-flash-preview",
		"messages":[
			{"role":"assistant","tool_calls":[{"id":"` + toolCallID + `","type":"function","function":{"name":"default_api:web_search","arguments":"{\"query\":\"weather\"}"}}]},
			{"role":"tool","tool_call_id":"` + toolCallID + `","content":"sunny"}
		]
	}`)

	converted, err := convertRequestOpenAIToGemini(clientRequest)
	require.NoError(t, err)
	require.Equal(t, "sig-native-42", gjson.GetBytes(converted, "contents.0.parts.0.thoughtSignature").String())
	require.Equal(t, "default_api:web_search", gjson.GetBytes(converted, "contents.0.parts.0.functionCall.name").String())
	require.Equal(t, "default_api:web_search", gjson.GetBytes(converted, "contents.1.parts.0.functionResponse.name").String())
	require.NotEmpty(t, gjson.GetBytes(converted, "contents.1.parts.0.functionResponse.name").String())
}

func TestGeminiToolResponseNameFromEncodedIDWithoutAssistantName(t *testing.T) {
	// Some clients only send tool results with tool_call_id, without a "name" field
	// and without replaying the assistant tool_calls that would populate toolNames.
	toolCallID := encodeToolCallID("default_api:web_search", "sig-only-in-id")
	body := []byte(`{
		"model":"gemini-3-flash-preview",
		"messages":[
			{"role":"tool","tool_call_id":"` + toolCallID + `","content":"result payload"}
		]
	}`)

	converted, err := convertRequestOpenAIToGemini(body)
	require.NoError(t, err)
	require.Equal(t, "default_api:web_search", gjson.GetBytes(converted, "contents.0.parts.0.functionResponse.name").String())
	require.Equal(t, toolCallID, gjson.GetBytes(converted, "contents.0.parts.0.functionResponse.id").String())
}

func TestGeminiStreamNativeFunctionCallIDEncodesSignature(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	rc := &relayContext{c: context, clientFormat: FormatOpenAI, modelName: "gemini-3-flash-preview"}
	upstream := &http.Response{
		Body: io.NopCloser(strings.NewReader(
			`data: {"candidates":[{"content":{"parts":[{"functionCall":{"id":"native-stream-1","name":"lookup","args":{"q":1}},"thoughtSignature":"stream-sig-9"}]},"finishReason":"STOP"}]}` + "\n\n",
		)),
	}

	_ = streamGeminiToOpenAI(rc, upstream)
	body := recorder.Body.String()
	expectedID := encodeToolCallID("lookup", "stream-sig-9")
	require.Contains(t, body, `"id":"`+expectedID+`"`)
	require.Contains(t, body, `"thought_signature":"stream-sig-9"`)
	require.NotContains(t, body, `"id":"native-stream-1"`)
}

func TestDecodeToolCallIDIgnoresPlainOpenAIIds(t *testing.T) {
	name, sig := decodeToolCallID("call_AbCdEfGhIjKlMnOp")
	require.Equal(t, "", name)
	require.Equal(t, "", sig)

	name, sig = decodeToolCallID(encodeToolCallID("web_search", "abc|def"))
	require.Equal(t, "web_search", name)
	require.Equal(t, "abc|def", sig)
}


