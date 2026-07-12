package relay

import (
	"bufio"
	"bytes"
	"io"
	"math/rand/v2"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"

	"mochi-api/model"
)

// Format identifies the wire format of a request/response.
type Format string

const (
	FormatOpenAI Format = "openai"
	FormatClaude Format = "claude"
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
	stream           bool
	clientWantsUsage bool   // OpenAI client explicitly set stream_options.include_usage
	promptText       string // rough concatenation of input text, for fallback estimation
	start            time.Time
}

// Handle is the shared relay pipeline for /v1/chat/completions (FormatOpenAI)
// and /v1/messages (FormatClaude).
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
	rc.stream = gjson.GetBytes(body, "stream").Bool()
	rc.clientWantsUsage = clientFormat == FormatOpenAI &&
		gjson.GetBytes(body, "stream_options.include_usage").Bool()
	rc.promptText = collectPromptText(body)

	channels, err := model.GetEnabledChannelsForModel(rc.modelName)
	if err != nil {
		writeError(c, clientFormat, http.StatusInternalServerError, "api_error", "数据库错误")
		return
	}
	if len(channels) == 0 {
		writeError(c, clientFormat, http.StatusNotFound, "invalid_request_error",
			"没有可用渠道支持模型 "+rc.modelName)
		return
	}
	rc.channel = pickChannel(channels)
	rc.upstreamFormat = FormatOpenAI
	if rc.channel.Type == model.ChannelTypeAnthropic {
		rc.upstreamFormat = FormatClaude
	}

	upstreamBody, err := prepareUpstreamBody(rc, body)
	if err != nil {
		writeError(c, clientFormat, http.StatusBadRequest, "invalid_request_error",
			"请求转换失败: "+err.Error())
		return
	}

	resp, err := sendUpstream(rc, upstreamBody)
	if err != nil {
		recordRelayLog(rc, usage{}, http.StatusBadGateway)
		writeError(c, clientFormat, http.StatusBadGateway, "api_error", "上游请求失败: "+err.Error())
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		relayUpstreamError(rc, resp)
		return
	}

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

// prepareUpstreamBody converts the body across formats when needed and
// injects stream_options.include_usage for streaming OpenAI upstreams.
func prepareUpstreamBody(rc *relayContext, body []byte) ([]byte, error) {
	var err error
	if rc.clientFormat != rc.upstreamFormat {
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
	path := "/v1/chat/completions"
	if rc.upstreamFormat == FormatClaude {
		path = "/v1/messages"
	}
	req, err := http.NewRequestWithContext(rc.c.Request.Context(), http.MethodPost,
		rc.channel.BaseURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", rc.c.Request.Header.Get("Accept"))
	if rc.upstreamFormat == FormatClaude {
		req.Header.Set("x-api-key", rc.channel.ApiKey)
		version := rc.c.Request.Header.Get("anthropic-version")
		if version == "" {
			version = defaultAnthropicVersion
		}
		req.Header.Set("anthropic-version", version)
	} else {
		req.Header.Set("Authorization", "Bearer "+rc.channel.ApiKey)
	}
	return httpClient.Do(req)
}

func dispatchStream(rc *relayContext, resp *http.Response) usage {
	switch {
	case rc.clientFormat == FormatOpenAI && rc.upstreamFormat == FormatOpenAI:
		return streamOpenAIToOpenAI(rc, resp)
	case rc.clientFormat == FormatClaude && rc.upstreamFormat == FormatClaude:
		return streamClaudeToClaude(rc, resp)
	case rc.clientFormat == FormatOpenAI && rc.upstreamFormat == FormatClaude:
		return streamClaudeToOpenAI(rc, resp)
	default:
		return streamOpenAIToClaude(rc, resp)
	}
}

func dispatchNonStream(rc *relayContext, resp *http.Response) usage {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		writeError(rc.c, rc.clientFormat, http.StatusBadGateway, "api_error", "读取上游响应失败")
		return usage{}
	}
	u := extractUsage(body, rc.upstreamFormat)
	if rc.clientFormat != rc.upstreamFormat {
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
	if format == FormatClaude {
		u := gjson.GetBytes(body, "usage")
		return usage{
			prompt: int(u.Get("input_tokens").Int() +
				u.Get("cache_creation_input_tokens").Int() +
				u.Get("cache_read_input_tokens").Int()),
			completion: int(u.Get("output_tokens").Int()),
		}
	}
	u := gjson.GetBytes(body, "usage")
	return usage{
		prompt:     int(u.Get("prompt_tokens").Int()),
		completion: int(u.Get("completion_tokens").Int()),
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

func pickChannel(channels []model.Channel) *model.Channel {
	// channels are sorted by priority desc; choose randomly among the top tier.
	top := channels[0].Priority
	n := 1
	for n < len(channels) && channels[n].Priority == top {
		n++
	}
	return &channels[rand.IntN(n)]
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
	c.Writer.Header().Set("Cache-Control", "no-cache")
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
