package relay

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/tidwall/gjson"

	"mochi-api/model"
)

// ListUpstreamModels lists the models an upstream channel serves. It doubles
// as the connectivity test: a 200 response means the base URL and API key
// are valid. OpenAI/Anthropic use GET /v1/models (data[].id); Gemini uses
// GET /v1beta/models (models[].name, prefixed with "models/").
func ListUpstreamModels(chType, baseURL, apiKey string) ([]string, time.Duration, error) {
	url := baseURL + "/v1/models"
	if chType == model.ChannelTypeGemini {
		url = baseURL + "/v1beta/models"
	}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, err
	}
	switch chType {
	case model.ChannelTypeAnthropic:
		req.Header.Set("x-api-key", apiKey)
		req.Header.Set("anthropic-version", defaultAnthropicVersion)
	case model.ChannelTypeGemini:
		req.Header.Set("x-goog-api-key", apiKey)
	default:
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	client := &http.Client{Timeout: 15 * time.Second}
	start := time.Now()
	resp, err := client.Do(req)
	latency := time.Since(start)
	if err != nil {
		return nil, latency, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, latency, err
	}

	if resp.StatusCode != http.StatusOK {
		message := gjson.GetBytes(body, "error.message").String()
		if message == "" {
			message = http.StatusText(resp.StatusCode)
		}
		return nil, latency, fmt.Errorf("上游返回 %d: %s", resp.StatusCode, message)
	}

	var models []string
	if chType == model.ChannelTypeGemini {
		for _, item := range gjson.GetBytes(body, "models").Array() {
			// Gemini returns names like "models/gemini-1.5-pro".
			name := strings.TrimPrefix(item.Get("name").String(), "models/")
			// Keep only models that support text generation.
			supported := false
			for _, m := range item.Get("supportedGenerationMethods").Array() {
				if m.String() == "generateContent" {
					supported = true
					break
				}
			}
			if name != "" && supported {
				models = append(models, name)
			}
		}
		return models, latency, nil
	}
	for _, item := range gjson.GetBytes(body, "data").Array() {
		if id := item.Get("id").String(); id != "" {
			models = append(models, id)
		}
	}
	return models, latency, nil
}
