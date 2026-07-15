package relay

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"math/rand/v2"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"

	"mochi-api/model"
)

// Format identifies the wire format of a request/response.
type Format string

const (
	FormatOpenAI    Format = "openai"
	FormatResponses Format = "responses"
	FormatClaude    Format = "claude"
	FormatGemini    Format = "gemini"
)

const defaultAnthropicVersion = "2023-06-01"

// httpClient has no total timeout so long SSE streams are never killed;
// only connection setup and header wait are bounded.
var httpClient = &http.Client{
	Transport: &http.Transport{
		DialContext:           (&net.Dialer{Timeout: 15 * time.Second}).DialContext,
		TLSHandshakeTimeout:   15 * time.Second,
		ResponseHeaderTimeout: 10 * time.Minute,
	},
}

type usage struct {
	prompt     int
	completion int
	estimated  bool
}

type relayContext struct {
	c                *gin.Context
	clientFormat     Format
	upstreamFormat   Format
	channel          *model.Channel
	modelName        string
	upstreamModel    string // resolved upstream name (equals modelName when no alias)
	stream           bool
	clientWantsUsage bool   // OpenAI client explicitly set stream_options.include_usage
	promptText       string // rough concatenation of input text, for fallback estimation
	start            time.Time
}

// Handle is the shared relay pipeline for OpenAI Chat Completions, OpenAI
// Responses, and Anthropic Messages compatible endpoints.
func Handle(c *gin.Context, clientFormat Format) {
	rc := &relayContext{c: c, clientFormat: clientFormat, start: time.Now()}

	body, err := io.ReadAll(c.Request.Body)
	if err != nil || len(body) == 0 {
		writeError(c, clientFormat, http.StatusBadRequest, "invalid_request_error", "无法读取请求体")
		return
	}
	rc.modelName = gjson.GetBytes(body, "model").String()
	if rc.modelName == "" {
		writeError(c, clientFormat, http.StatusBadRequest, "invalid_request_error", "缺少 model 字段")
		return
	}
	var targetModels []string
	if upstream, ok := model.ResolveAlias(rc.modelName); ok {
		targetModels = model.ParseModelList(upstream)
	} else {
		targetModels = []string{rc.modelName}
	}
	rc.stream = gjson.GetBytes(body, "stream").Bool()
	rc.clientWantsUsage = clientFormat == FormatOpenAI &&
		gjson.GetBytes(body, "stream_options.include_usage").Bool()
	rc.promptText = collectPromptText(body)

	channels, err := model.GetEnabledChannelsForModelList(targetModels)
	if err != nil {
		writeError(c, clientFormat, http.StatusInternalServerError, "api_error", "数据库错误")
		return
	}
	if len(channels) == 0 {
		writeError(c, clientFormat, http.StatusNotFound, "invalid_request_error",
			"没有可用渠道支持模型 "+rc.modelName)
		return
	}

	// Try channels in failover order until one accepts the request. The
	// request body is fully buffered, so it can be replayed against the
	// next channel; nothing is written to the client until a channel is
	// chosen, so switching is invisible to the caller.
	candidates := orderChannels(channels)
	var resp *http.Response
	var lastErr error
	for i := range candidates {
		rc.channel = &candidates[i]
		actualModel, ok := rc.channel.FirstSupportedModel(targetModels)
		if !ok {
			continue
		}
		rc.upstreamModel = actualModel
		rc.upstreamFormat = upstreamFormatFor(rc.channel, clientFormat)
		last := i == len(candidates)-1

		channelBody := body
		if actualModel != rc.modelName {
			channelBody, _ = sjson.SetBytes(body, "model", actualModel)
		}

		upstreamBody, err := prepareUpstreamBody(rc, channelBody)
		if err != nil {
			// Conversion errors depend on the channel type; another channel
			// may accept the same request in its native format.
			lastErr = fmt.Errorf("请求转换失败: %w", err)
			if last {
				writeError(c, clientFormat, http.StatusBadRequest, "invalid_request_error", lastErr.Error())
				return
			}
			continue
		}

		resp, err = sendUpstreamWithResponsesFallback(rc, body, upstreamBody)
		if err != nil {
			lastErr = fmt.Errorf("上游请求失败: %w", err)
			markChannelFailure(rc.channel.Id)
			recordRelayLog(rc, usage{}, http.StatusBadGateway)
			if last {
				writeError(c, clientFormat, http.StatusBadGateway, "api_error", lastErr.Error())
				return
			}
			continue
		}
		if resp.StatusCode != http.StatusOK && !last && retriableStatus(resp.StatusCode) {
			markChannelFailure(rc.channel.Id)
			recordRelayLog(rc, usage{}, resp.StatusCode)
			_ = resp.Body.Close()
			resp = nil
			continue
		}
		break
	}
	if resp == nil {
		// Every candidate failed with a network error or was skipped.
		writeError(c, clientFormat, http.StatusBadGateway, "api_error", lastErr.Error())
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if retriableStatus(resp.StatusCode) {
			markChannelFailure(rc.channel.Id)
		}
		relayUpstreamError(rc, resp)
		return
	}
	markChannelSuccess(rc.channel.Id)

	isEventStream := strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream")
	var u usage
	if isEventStream {
		u = dispatchStream(rc, resp)
	} else {
		u = dispatchNonStream(rc, resp)
	}
	if u.prompt == 0 {
		u.prompt = estimateTokens(rc.promptText)
		u.estimated = true
	}
	recordRelayLog(rc, u, http.StatusOK)
}

// prepareUpstreamBody converts the request body to the upstream's format.
// OpenAI is used as the intermediate format for Gemini conversions.
func prepareUpstreamBody(rc *relayContext, body []byte) ([]byte, error) {
	var err error
	sourceFormat := rc.clientFormat
	if sourceFormat == FormatResponses && rc.upstreamFormat != FormatResponses {
		if gjson.GetBytes(body, "previous_response_id").String() != "" {
			return nil, fmt.Errorf("当前渠道不支持 previous_response_id，请传入完整 input 历史")
		}
		if rc.upstreamFormat == FormatClaude && responsesHasHostedTool(body) {
			return nil, fmt.Errorf("Anthropic 渠道暂不支持 Responses 内置工具")
		}
		body, err = convertRequestResponsesToOpenAIChat(body)
		if err != nil {
			return nil, err
		}
		sourceFormat = FormatOpenAI
	}
	if rc.upstreamFormat == FormatGemini {
		// Normalize to OpenAI first, then OpenAI -> Gemini.
		if sourceFormat == FormatClaude {
			if body, err = convertRequestClaudeToOpenAI(body); err != nil {
				return nil, err
			}
		}
		return convertRequestOpenAIToGemini(body)
	}
	if sourceFormat != rc.upstreamFormat {
		if rc.upstreamFormat == FormatClaude {
			body, err = convertRequestOpenAIToClaude(body)
		} else {
			body, err = convertRequestClaudeToOpenAI(body)
		}
		if err != nil {
			return nil, err
		}
	}
	if rc.upstreamFormat == FormatOpenAI && rc.stream {
		body, err = sjson.SetBytes(body, "stream_options.include_usage", true)
		if err != nil {
			return nil, err
		}
	}
	return body, nil
}

func sendUpstream(rc *relayContext, body []byte) (*http.Response, error) {
	url, headerKey, headerVal := upstreamTarget(rc)
	req, err := http.NewRequestWithContext(rc.c.Request.Context(), http.MethodPost,
		url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", rc.c.Request.Header.Get("Accept"))
	req.Header.Set(headerKey, headerVal)
	if rc.upstreamFormat == FormatClaude {
		version := rc.c.Request.Header.Get("anthropic-version")
		if version == "" {
			version = defaultAnthropicVersion
		}
		req.Header.Set("anthropic-version", version)
	}
	return httpClient.Do(req)
}

// sendUpstreamWithResponsesFallback retries a portable Responses request via
// Chat Completions only when an OpenAI-compatible upstream explicitly reports
// that its Responses method is unsupported.
func sendUpstreamWithResponsesFallback(
	rc *relayContext,
	clientBody, upstreamBody []byte,
) (*http.Response, error) {
	resp, err := sendUpstream(rc, upstreamBody)
	if err != nil || resp.StatusCode == http.StatusOK ||
		rc.clientFormat != FormatResponses || rc.upstreamFormat != FormatResponses ||
		rc.channel.Type != model.ChannelTypeOpenAI {
		return resp, err
	}

	originalBody := resp.Body
	errorBody, readErr := io.ReadAll(originalBody)
	_ = originalBody.Close()
	resp.Body = io.NopCloser(bytes.NewReader(errorBody))
	if readErr != nil {
		return resp, nil
	}
	if !isUnsupportedResponsesError(resp.StatusCode, errorBody) {
		return resp, nil
	}

	rc.upstreamFormat = FormatOpenAI
	fallbackBody, prepareErr := prepareUpstreamBody(rc, clientBody)
	if prepareErr != nil {
		rc.upstreamFormat = FormatResponses
		return resp, nil
	}
	_ = resp.Body.Close()
	return sendUpstream(rc, fallbackBody)
}

func isUnsupportedResponsesError(status int, body []byte) bool {
	switch status {
	case http.StatusBadRequest, http.StatusNotFound, http.StatusMethodNotAllowed,
		http.StatusUnprocessableEntity, http.StatusNotImplemented:
	default:
		return false
	}
	message := gjson.GetBytes(body, "error.message").String()
	if message == "" {
		message = string(body)
	}
	message = strings.ToLower(message)
	mentionsResponses := strings.Contains(message, "responses") || strings.Contains(message, "/v1/responses")
	unsupported := strings.Contains(message, "does not support") ||
		strings.Contains(message, "not supported") ||
		strings.Contains(message, "unsupported") ||
		strings.Contains(message, "not implemented") ||
		strings.Contains(message, "unknown endpoint")
	return mentionsResponses && unsupported
}

// upstreamTarget returns the request URL and auth header for the channel.
//
// Base URL conventions for non-standard upstream paths:
//   - no trailing marker: the standard version prefix is appended
//     (/v1/chat/completions, /v1/messages, /v1beta/models/...)
//   - trailing "/": the base is a complete API prefix; only the endpoint
//     leaf is appended (chat/completions, messages, models/{model}:...)
//   - trailing "#": the base is the exact endpoint URL, used as-is.
//     Gemini interpolates the model into the path, so "#" falls back to
//     the trailing-"/" behavior there.
func upstreamTarget(rc *relayContext) (url, headerKey, headerVal string) {
	base, exact := splitBaseURL(rc.channel.BaseURL)
	switch rc.upstreamFormat {
	case FormatClaude:
		return joinUpstreamPath(base, exact, "/v1/messages"), "x-api-key", rc.channel.ApiKey
	case FormatGemini:
		action := "generateContent"
		if rc.stream {
			action = "streamGenerateContent?alt=sse"
		}
		leaf := fmt.Sprintf("/models/%s:%s", rc.upstreamModel, action)
		if exact == exactEndpoint || exact == fullPrefix {
			return strings.TrimSuffix(base, "/") + leaf, "x-goog-api-key", rc.channel.ApiKey
		}
		return base + "/v1beta" + leaf, "x-goog-api-key", rc.channel.ApiKey
	case FormatResponses:
		return joinUpstreamPath(base, exact, "/v1/responses"), "Authorization", "Bearer " + rc.channel.ApiKey
	default:
		return joinUpstreamPath(base, exact, "/v1/chat/completions"), "Authorization", "Bearer " + rc.channel.ApiKey
	}
}

type baseURLMode int

const (
	standardBase  baseURLMode = iota // append the full version path
	fullPrefix                       // trailing "/": append only the endpoint leaf
	exactEndpoint                    // trailing "#": use the URL as-is
)

// splitBaseURL strips the trailing path marker and reports which mode applies.
func splitBaseURL(baseURL string) (string, baseURLMode) {
	if strings.HasSuffix(baseURL, "#") {
		return strings.TrimSuffix(baseURL, "#"), exactEndpoint
	}
	if strings.HasSuffix(baseURL, "/") {
		return strings.TrimSuffix(baseURL, "/"), fullPrefix
	}
	return baseURL, standardBase
}

// joinUpstreamPath combines the cleaned base URL with the standard endpoint
// path according to the mode. versionedPath must start with "/v1/".
func joinUpstreamPath(base string, mode baseURLMode, versionedPath string) string {
	switch mode {
	case exactEndpoint:
		return base
	case fullPrefix:
		return base + strings.TrimPrefix(versionedPath, "/v1")
	default:
		return base + versionedPath
	}
}

func dispatchStream(rc *relayContext, resp *http.Response) usage {
	switch rc.upstreamFormat {
	case FormatGemini:
		if rc.clientFormat == FormatResponses {
			return streamGeminiToResponses(rc, resp)
		}
		if rc.clientFormat == FormatClaude {
			return streamGeminiToClaude(rc, resp)
		}
		return streamGeminiToOpenAI(rc, resp)
	case FormatClaude:
		if rc.clientFormat == FormatResponses {
			return streamClaudeToResponses(rc, resp)
		}
		if rc.clientFormat == FormatClaude {
			return streamClaudeToClaude(rc, resp)
		}
		return streamClaudeToOpenAI(rc, resp)
	case FormatResponses:
		return streamResponsesToResponses(rc, resp)
	default: // OpenAI Chat Completions upstream
		if rc.clientFormat == FormatClaude {
			return streamOpenAIToClaude(rc, resp)
		}
		if rc.clientFormat == FormatResponses {
			return streamOpenAIToResponses(rc, resp)
		}
		return streamOpenAIToOpenAI(rc, resp)
	}
}

func dispatchNonStream(rc *relayContext, resp *http.Response) usage {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		writeError(rc.c, rc.clientFormat, http.StatusBadGateway, "api_error", "读取上游响应失败")
		return usage{}
	}
	u := extractUsage(body, rc.upstreamFormat)

	if rc.upstreamFormat == FormatResponses {
		rc.c.Data(http.StatusOK, "application/json", body)
		return u
	}

	// Convert Gemini upstream to the OpenAI Chat Completions intermediate first.
	if rc.upstreamFormat == FormatGemini {
		body, err = convertResponseGeminiToOpenAI(body, rc.modelName)
		if err != nil {
			writeError(rc.c, rc.clientFormat, http.StatusBadGateway, "api_error", "响应转换失败")
			return u
		}
		if rc.clientFormat == FormatClaude {
			if body, err = convertResponseOpenAIToClaude(body); err != nil {
				writeError(rc.c, rc.clientFormat, http.StatusBadGateway, "api_error", "响应转换失败")
				return u
			}
		} else if rc.clientFormat == FormatResponses {
			if body, err = convertResponseOpenAIToResponses(body); err != nil {
				writeError(rc.c, rc.clientFormat, http.StatusBadGateway, "api_error", "响应转换失败")
				return u
			}
		}
		rc.c.Data(http.StatusOK, "application/json", body)
		return u
	}

	if rc.clientFormat == FormatResponses {
		if rc.upstreamFormat == FormatClaude {
			body, err = convertResponseClaudeToOpenAI(body)
		}
		if err == nil {
			body, err = convertResponseOpenAIToResponses(body)
		}
		if err != nil {
			writeError(rc.c, rc.clientFormat, http.StatusBadGateway, "api_error", "响应转换失败")
			return u
		}
	} else if rc.clientFormat != rc.upstreamFormat {
		var converted []byte
		if rc.clientFormat == FormatOpenAI {
			converted, err = convertResponseClaudeToOpenAI(body)
		} else {
			converted, err = convertResponseOpenAIToClaude(body)
		}
		if err != nil {
			writeError(rc.c, rc.clientFormat, http.StatusBadGateway, "api_error", "响应转换失败")
			return u
		}
		body = converted
	}
	rc.c.Data(http.StatusOK, "application/json", body)
	return u
}

// extractUsage reads token usage from a non-stream response body.
// Claude cache tokens are folded into prompt tokens as an approximation.
func extractUsage(body []byte, format Format) usage {
	switch format {
	case FormatClaude:
		u := gjson.GetBytes(body, "usage")
		return usage{
			prompt: int(u.Get("input_tokens").Int() +
				u.Get("cache_creation_input_tokens").Int() +
				u.Get("cache_read_input_tokens").Int()),
			completion: int(u.Get("output_tokens").Int()),
		}
	case FormatGemini:
		u := gjson.GetBytes(body, "usageMetadata")
		return usage{
			prompt:     int(u.Get("promptTokenCount").Int()),
			completion: int(u.Get("candidatesTokenCount").Int() + u.Get("thoughtsTokenCount").Int()),
		}
	case FormatResponses:
		u := gjson.GetBytes(body, "usage")
		return usage{prompt: int(u.Get("input_tokens").Int()), completion: int(u.Get("output_tokens").Int())}
	default:
		u := gjson.GetBytes(body, "usage")
		return usage{
			prompt:     int(u.Get("prompt_tokens").Int()),
			completion: int(u.Get("completion_tokens").Int()),
		}
	}
}

// relayUpstreamError forwards a non-200 upstream response. Cross-format
// error bodies are rewrapped so the client always sees its own dialect.
func relayUpstreamError(rc *relayContext, resp *http.Response) {
	body, _ := io.ReadAll(resp.Body)
	recordRelayLog(rc, usage{}, resp.StatusCode)
	if rc.clientFormat == rc.upstreamFormat {
		contentType := resp.Header.Get("Content-Type")
		if contentType == "" {
			contentType = "application/json"
		}
		rc.c.Data(resp.StatusCode, contentType, body)
		return
	}
	message := gjson.GetBytes(body, "error.message").String()
	if message == "" {
		message = strings.TrimSpace(string(body))
	}
	if len(message) > 500 {
		message = message[:500]
	}
	writeError(rc.c, rc.clientFormat, resp.StatusCode, "api_error", message)
}

// upstreamFormatFor maps a channel and client protocol to the wire format sent
// upstream. OpenAI-compatible channels use Chat conversion unless native
// Responses support was explicitly enabled.
func upstreamFormatFor(channel *model.Channel, clientFormat Format) Format {
	switch channel.Type {
	case model.ChannelTypeAnthropic:
		return FormatClaude
	case model.ChannelTypeGemini:
		return FormatGemini
	default:
		if clientFormat == FormatResponses && channel.UsesNativeResponses() {
			return FormatResponses
		}
		return FormatOpenAI
	}
}

// channelCooldowns tracks channels that recently failed so orderChannels can
// deprioritize them for a short window instead of hammering a dead upstream.
var channelCooldowns sync.Map // channel id (int) -> cooldown expiry (time.Time)

const channelCooldown = 60 * time.Second

func markChannelFailure(id int) {
	channelCooldowns.Store(id, time.Now().Add(channelCooldown))
}

func markChannelSuccess(id int) {
	channelCooldowns.Delete(id)
}

func channelCoolingDown(id int) bool {
	v, ok := channelCooldowns.Load(id)
	if !ok {
		return false
	}
	if time.Now().After(v.(time.Time)) {
		channelCooldowns.Delete(id)
		return false
	}
	return true
}

// retriableStatus reports whether an upstream status suggests another channel
// could serve the request: auth/permission/quota problems and server errors
// are channel-specific, while other 4xx statuses indict the request itself.
func retriableStatus(code int) bool {
	switch code {
	case http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound,
		http.StatusRequestTimeout, http.StatusTooManyRequests:
		return true
	}
	return code >= 500
}

// orderChannels returns the failover order: channels grouped by priority
// (already sorted descending), shuffled within each tier to spread load,
// with channels in failure cooldown moved to the back as a last resort.
func orderChannels(channels []model.Channel) []model.Channel {
	ordered := make([]model.Channel, len(channels))
	copy(ordered, channels)
	for lo := 0; lo < len(ordered); {
		hi := lo + 1
		for hi < len(ordered) && ordered[hi].Priority == ordered[lo].Priority {
			hi++
		}
		rand.Shuffle(hi-lo, func(i, j int) {
			ordered[lo+i], ordered[lo+j] = ordered[lo+j], ordered[lo+i]
		})
		lo = hi
	}
	healthy := make([]model.Channel, 0, len(ordered))
	var cooling []model.Channel
	for _, ch := range ordered {
		if channelCoolingDown(ch.Id) {
			cooling = append(cooling, ch)
		} else {
			healthy = append(healthy, ch)
		}
	}
	return append(healthy, cooling...)
}

func recordRelayLog(rc *relayContext, u usage, code int) {
	price, err := model.MatchPrice(rc.modelName)
	if err != nil {
		price = nil
	}
	_ = model.RecordLog(&model.Log{
		UserId:           rc.c.GetInt("user_id"),
		TokenName:        rc.c.GetString("token_name"),
		ChannelId:        rc.channel.Id,
		ChannelName:      rc.channel.Name,
		ModelName:        rc.modelName,
		PromptTokens:     u.prompt,
		CompletionTokens: u.completion,
		CostMicros:       model.ComputeCostMicros(price, u.prompt, u.completion),
		UseTimeMs:        int(time.Since(rc.start).Milliseconds()),
		IsStream:         rc.stream,
		Code:             code,
	})
}

// estimateTokens is the rough fallback when the upstream reports no usage:
// ~4 characters per token.
func estimateTokens(text string) int {
	n := len([]rune(text))
	if n == 0 {
		return 0
	}
	return n/4 + 1
}

// collectPromptText concatenates input text for fallback token estimation.
func collectPromptText(body []byte) string {
	var sb strings.Builder
	instructions := gjson.GetBytes(body, "instructions")
	if instructions.Type == gjson.String {
		sb.WriteString(instructions.String())
	}
	input := gjson.GetBytes(body, "input")
	if input.Type == gjson.String {
		sb.WriteString(input.String())
	} else {
		for _, item := range input.Array() {
			content := item.Get("content")
			if content.Type == gjson.String {
				sb.WriteString(content.String())
			}
			for _, part := range content.Array() {
				sb.WriteString(part.Get("text").String())
			}
		}
	}
	system := gjson.GetBytes(body, "system")
	if system.Type == gjson.String {
		sb.WriteString(system.String())
	}
	for _, msg := range gjson.GetBytes(body, "messages").Array() {
		content := msg.Get("content")
		if content.Type == gjson.String {
			sb.WriteString(content.String())
			continue
		}
		for _, part := range content.Array() {
			sb.WriteString(part.Get("text").String())
		}
	}
	return sb.String()
}

// writeError emits an error body in the client's dialect.
func writeError(c *gin.Context, format Format, status int, errType, message string) {
	if format == FormatClaude {
		c.JSON(status, gin.H{
			"type":  "error",
			"error": gin.H{"type": errType, "message": message},
		})
		return
	}
	c.JSON(status, gin.H{
		"error": gin.H{"message": message, "type": errType},
	})
}

// --- SSE plumbing shared by all stream handlers ---

func setSSEHeaders(c *gin.Context) {
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache, no-transform")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Writer.WriteHeader(http.StatusOK)
}

// scanSSE invokes fn for every line of the upstream SSE body
// (line endings stripped, blank lines included).
func scanSSE(body io.Reader, fn func(line string)) {
	reader := bufio.NewReaderSize(body, 64*1024)
	for {
		line, err := reader.ReadString('\n')
		if len(line) > 0 {
			fn(strings.TrimRight(line, "\r\n"))
		}
		if err != nil {
			return
		}
	}
}

// writeSSELine writes one already-formatted SSE line plus the event-ending
// blank line, then flushes so the client sees it immediately.
func writeSSELine(c *gin.Context, line string) {
	_, _ = c.Writer.WriteString(line + "\n\n")
	c.Writer.Flush()
}
