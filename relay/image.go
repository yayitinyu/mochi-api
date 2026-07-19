package relay

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"

	"mochi-api/model"
)

// HandleImageGeneration serves the OpenAI-compatible Images generations API.
// OpenAI-compatible channels are forwarded verbatim; Gemini channels use the
// native generateContent endpoint and are converted back to the Images shape.
func HandleImageGeneration(c *gin.Context) {
	rc := &relayContext{c: c, clientFormat: FormatImage, start: time.Now()}

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxRequestBody)
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeError(c, FormatImage, http.StatusRequestEntityTooLarge,
				"invalid_request_error", "请求体过大")
			return
		}
		writeError(c, FormatImage, http.StatusBadRequest, "invalid_request_error", "无法读取请求体")
		return
	}
	if !gjson.ValidBytes(body) {
		writeError(c, FormatImage, http.StatusBadRequest, "invalid_request_error", "请求格式错误")
		return
	}

	rc.modelName = gjson.GetBytes(body, "model").String()
	prompt := gjson.GetBytes(body, "prompt")
	if rc.modelName == "" {
		writeError(c, FormatImage, http.StatusBadRequest, "invalid_request_error", "缺少 model 字段")
		return
	}
	if prompt.Type != gjson.String || strings.TrimSpace(prompt.String()) == "" {
		writeError(c, FormatImage, http.StatusBadRequest, "invalid_request_error", "缺少 prompt 字段")
		return
	}
	rc.stream = gjson.GetBytes(body, "stream").Bool()

	targetModels := model.ResolveModelTargets(rc.modelName)
	channels, err := model.GetEnabledChannelsForModelList(targetModels)
	if err != nil {
		writeError(c, FormatImage, http.StatusInternalServerError, "api_error", "数据库错误")
		return
	}
	if len(channels) == 0 {
		writeError(c, FormatImage, http.StatusNotFound, "invalid_request_error",
			"没有可用渠道支持模型 "+rc.modelName)
		return
	}

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
		rc.upstreamFormat = upstreamFormatFor(rc.channel, FormatImage)
		last := i == len(candidates)-1

		channelBody := body
		if actualModel != rc.modelName {
			channelBody, _ = sjson.SetBytes(body, "model", actualModel)
		}
		upstreamBody := channelBody
		if rc.upstreamFormat == FormatClaude {
			lastErr = errors.New("Anthropic 渠道不支持 Images 生成接口")
			if last {
				writeError(c, FormatImage, http.StatusBadRequest, "invalid_request_error", lastErr.Error())
				return
			}
			continue
		}
		if rc.upstreamFormat == FormatGemini {
			upstreamBody, err = convertImageRequestToGemini(channelBody, actualModel)
			if err != nil {
				lastErr = fmt.Errorf("请求转换失败: %w", err)
				if last {
					writeError(c, FormatImage, http.StatusBadRequest, "invalid_request_error", lastErr.Error())
					return
				}
				continue
			}
		}

		resp, err = sendUpstream(rc, upstreamBody)
		if err != nil {
			if c.Request.Context().Err() != nil {
				recordRelayLog(rc, usage{}, statusClientClosedRequest)
				return
			}
			lastErr = fmt.Errorf("上游请求失败: %w", err)
			markChannelFailure(rc.channel.Id)
			recordRelayLog(rc, usage{}, http.StatusBadGateway)
			if last {
				writeError(c, FormatImage, http.StatusBadGateway, "api_error", lastErr.Error())
				return
			}
			continue
		}
		if resp.StatusCode != http.StatusOK && !last && retriableStatus(resp.StatusCode) {
			lastErr = fmt.Errorf("上游返回 %d", resp.StatusCode)
			markChannelFailure(rc.channel.Id)
			recordRelayLog(rc, usage{}, resp.StatusCode)
			_ = resp.Body.Close()
			resp = nil
			continue
		}
		break
	}
	if resp == nil {
		if lastErr == nil {
			lastErr = errors.New("没有可用渠道")
		}
		writeError(c, FormatImage, http.StatusBadGateway, "api_error", lastErr.Error())
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

	var u usage
	isEventStream := strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream")
	switch {
	case rc.upstreamFormat == FormatGemini && isEventStream:
		var converted bool
		u, converted = streamGeminiImagesToOpenAI(rc, resp)
		if !converted {
			recordRelayLog(rc, u, http.StatusBadGateway)
			return
		}
	case rc.upstreamFormat == FormatGemini:
		upstreamResponse, readErr := readBounded(resp.Body, maxUpstreamBody)
		if readErr != nil {
			recordRelayLog(rc, usage{}, http.StatusBadGateway)
			writeError(c, FormatImage, http.StatusBadGateway, "api_error", "读取上游响应失败")
			return
		}
		converted, convertedUsage, convertErr := convertResponseGeminiToImages(
			upstreamResponse, gjson.GetBytes(body, "response_format").String(),
		)
		if convertErr != nil {
			recordRelayLog(rc, convertedUsage, http.StatusBadGateway)
			writeError(c, FormatImage, http.StatusBadGateway, "api_error", convertErr.Error())
			return
		}
		u = convertedUsage
		c.Data(http.StatusOK, "application/json", converted)
	case isEventStream:
		u = streamImageToImage(rc, resp)
	default:
		upstreamResponse, readErr := readBounded(resp.Body, maxUpstreamBody)
		if readErr != nil {
			recordRelayLog(rc, usage{}, http.StatusBadGateway)
			writeError(c, FormatImage, http.StatusBadGateway, "api_error", "读取上游响应失败")
			return
		}
		u = extractImageUsage(upstreamResponse)
		contentType := resp.Header.Get("Content-Type")
		if contentType == "" {
			contentType = "application/json"
		}
		c.Data(http.StatusOK, contentType, upstreamResponse)
	}
	if u.prompt == 0 {
		u.prompt = estimateTokens(prompt.String())
		u.estimated = true
	}
	recordRelayLog(rc, u, http.StatusOK)
}

func convertImageRequestToGemini(body []byte, modelName string) ([]byte, error) {
	root := gjson.ParseBytes(body)
	n := root.Get("n").Int()
	if n > 1 {
		return nil, fmt.Errorf("Gemini 图片渠道暂不支持 n 大于 1")
	}

	imageOptions, err := geminiImageOptions(root, modelName)
	if err != nil {
		return nil, err
	}
	generationConfig := map[string]any{
		"responseModalities": []string{"IMAGE"},
	}
	if len(imageOptions) > 0 {
		generationConfig["responseFormat"] = map[string]any{"image": imageOptions}
	}
	if seed := root.Get("seed"); seed.Exists() && seed.Type == gjson.Number {
		generationConfig["seed"] = seed.Int()
	}

	request := map[string]any{
		"contents": []any{map[string]any{
			"role":  "user",
			"parts": []any{map[string]any{"text": root.Get("prompt").String()}},
		}},
		"generationConfig": generationConfig,
	}
	return json.Marshal(request)
}

func geminiImageOptions(root gjson.Result, modelName string) (map[string]any, error) {
	options := map[string]any{}
	aspectRatio := strings.TrimSpace(root.Get("aspect_ratio").String())
	size := strings.TrimSpace(root.Get("size").String())
	imageSize := ""

	if aspectRatio == "" && size != "" && !strings.EqualFold(size, "auto") {
		upperSize := strings.ToUpper(size)
		switch upperSize {
		case "512", "1K", "2K", "4K":
			imageSize = upperSize
		default:
			width, height, ok := parseImageDimensions(size)
			if !ok {
				return nil, fmt.Errorf("Gemini 图片渠道无法转换 size %q", size)
			}
			aspectRatio = reducedRatio(width, height)
			switch longest := max(width, height); {
			case longest <= 512:
				imageSize = "512"
			case longest <= 1024:
				imageSize = "1K"
			case longest <= 2048:
				imageSize = "2K"
			case longest <= 4096:
				imageSize = "4K"
			default:
				return nil, fmt.Errorf("Gemini 图片渠道不支持超过 4K 的 size")
			}
		}
	}

	if aspectRatio != "" {
		if !supportedGeminiAspectRatios[aspectRatio] {
			return nil, fmt.Errorf("Gemini 图片渠道不支持宽高比 %s", aspectRatio)
		}
		options["aspectRatio"] = aspectRatio
	}
	// Gemini 2.5 image models always render at their fixed native size and
	// reject the newer imageSize option.
	if imageSize != "" && !strings.Contains(strings.ToLower(modelName), "2.5") {
		options["imageSize"] = imageSize
	}
	return options, nil
}

var supportedGeminiAspectRatios = map[string]bool{
	"1:1": true, "1:4": true, "4:1": true, "1:8": true, "8:1": true,
	"2:3": true, "3:2": true, "3:4": true, "4:3": true, "4:5": true,
	"5:4": true, "9:16": true, "16:9": true, "21:9": true,
}

func parseImageDimensions(size string) (int, int, bool) {
	parts := strings.Split(strings.ToLower(size), "x")
	if len(parts) != 2 {
		return 0, 0, false
	}
	width, errWidth := strconv.Atoi(strings.TrimSpace(parts[0]))
	height, errHeight := strconv.Atoi(strings.TrimSpace(parts[1]))
	return width, height, errWidth == nil && errHeight == nil && width > 0 && height > 0
}

func reducedRatio(width, height int) string {
	divisor := gcd(width, height)
	return fmt.Sprintf("%d:%d", width/divisor, height/divisor)
}

func gcd(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}

func convertResponseGeminiToImages(body []byte, responseFormat string) ([]byte, usage, error) {
	root := gjson.ParseBytes(body)
	u := extractUsage(body, FormatGemini)
	data := make([]map[string]any, 0, 1)
	var revisedPrompt string

	for _, candidate := range root.Get("candidates").Array() {
		for _, part := range candidate.Get("content.parts").Array() {
			if part.Get("thought").Bool() {
				continue
			}
			if text := strings.TrimSpace(part.Get("text").String()); text != "" && revisedPrompt == "" {
				revisedPrompt = text
			}
			inline := part.Get("inlineData")
			encoded := inline.Get("data").String()
			if encoded == "" {
				continue
			}
			entry := map[string]any{}
			if responseFormat == "url" {
				mimeType := inline.Get("mimeType").String()
				if mimeType == "" {
					mimeType = "image/png"
				}
				entry["url"] = "data:" + mimeType + ";base64," + encoded
			} else {
				entry["b64_json"] = encoded
			}
			if revisedPrompt != "" {
				entry["revised_prompt"] = revisedPrompt
			}
			data = append(data, entry)
		}
	}
	if len(data) == 0 {
		message := root.Get("promptFeedback.blockReasonMessage").String()
		if message == "" {
			message = "Gemini 上游未返回图片"
		}
		return nil, u, errors.New(message)
	}

	result := map[string]any{
		"created": time.Now().Unix(),
		"data":    data,
		"usage": map[string]any{
			"input_tokens":  u.prompt,
			"output_tokens": u.completion,
			"total_tokens":  u.prompt + u.completion,
		},
	}
	converted, err := json.Marshal(result)
	return converted, u, err
}

func extractImageUsage(body []byte) usage {
	u := gjson.GetBytes(body, "usage")
	prompt := int(u.Get("input_tokens").Int())
	if prompt == 0 {
		prompt = int(u.Get("prompt_tokens").Int())
	}
	completion := int(u.Get("output_tokens").Int())
	if completion == 0 {
		completion = int(u.Get("completion_tokens").Int())
	}
	if completion == 0 {
		total := int(u.Get("total_tokens").Int())
		if total > prompt {
			completion = total - prompt
		}
	}
	return usage{prompt: prompt, completion: completion}
}

func streamImageToImage(rc *relayContext, resp *http.Response) usage {
	setSSEHeaders(rc.c)
	reader := bufio.NewReaderSize(resp.Body, 64*1024)
	var u usage
	for {
		line, err := reader.ReadString('\n')
		if len(line) > 0 {
			_, _ = rc.c.Writer.WriteString(line)
			if payload, ok := strings.CutPrefix(strings.TrimSpace(line), "data:"); ok {
				candidate := extractImageUsage([]byte(strings.TrimSpace(payload)))
				if candidate.prompt != 0 || candidate.completion != 0 {
					u = candidate
				}
			}
			if strings.TrimSpace(line) == "" {
				rc.c.Writer.Flush()
			}
		}
		if err != nil {
			break
		}
	}
	rc.c.Writer.Flush()
	return u
}

func streamGeminiImagesToOpenAI(rc *relayContext, resp *http.Response) (usage, bool) {
	setSSEHeaders(rc.c)
	var u usage
	var imageData, mimeType, revisedPrompt string

	scanSSE(resp.Body, func(line string) {
		payload, ok := strings.CutPrefix(line, "data:")
		if !ok {
			return
		}
		event := gjson.Parse(strings.TrimSpace(payload))
		if metadata := event.Get("usageMetadata"); metadata.Exists() {
			u.prompt = int(metadata.Get("promptTokenCount").Int())
			u.completion = int(metadata.Get("candidatesTokenCount").Int() +
				metadata.Get("thoughtsTokenCount").Int())
		}
		for _, candidate := range event.Get("candidates").Array() {
			for _, part := range candidate.Get("content.parts").Array() {
				if part.Get("thought").Bool() {
					continue
				}
				if text := strings.TrimSpace(part.Get("text").String()); text != "" {
					revisedPrompt = text
				}
				if encoded := part.Get("inlineData.data").String(); encoded != "" {
					imageData = encoded
					mimeType = part.Get("inlineData.mimeType").String()
				}
			}
		}
	})

	if imageData == "" {
		writeImageSSEEvent(rc.c, "error", map[string]any{
			"error": map[string]any{"type": "api_error", "message": "Gemini 上游未返回图片"},
		})
		return u, false
	}
	payload := map[string]any{
		"type":     "image_generation.completed",
		"b64_json": imageData,
		"usage": map[string]any{
			"input_tokens":  u.prompt,
			"output_tokens": u.completion,
			"total_tokens":  u.prompt + u.completion,
		},
	}
	if mimeType != "" {
		payload["mime_type"] = mimeType
	}
	if revisedPrompt != "" {
		payload["revised_prompt"] = revisedPrompt
	}
	writeImageSSEEvent(rc.c, "image_generation.completed", payload)
	return u, true
}

func writeImageSSEEvent(c *gin.Context, event string, payload any) {
	data, _ := json.Marshal(payload)
	_, _ = c.Writer.WriteString("event: " + event + "\ndata: " + string(data) + "\n\n")
	c.Writer.Flush()
}
