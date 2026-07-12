package relay

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/tidwall/gjson"

	"mochi-api/model"
)

// ListUpstreamModels calls GET {baseURL}/v1/models on an upstream channel.
// Both OpenAI and Anthropic expose this endpoint with a data[].id shape.
// It doubles as the connectivity test: a 200 response means the base URL
// and API key are valid.
func ListUpstreamModels(chType, baseURL, apiKey string) ([]string, time.Duration, error) {
	req, err := http.NewRequest(http.MethodGet, baseURL+"/v1/models", nil)
	if err != nil {
		return nil, 0, err
	}
	if chType == model.ChannelTypeAnthropic {
		req.Header.Set("x-api-key", apiKey)
		req.Header.Set("anthropic-version", defaultAnthropicVersion)
	} else {
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
	for _, item := range gjson.GetBytes(body, "data").Array() {
		if id := item.Get("id").String(); id != "" {
			models = append(models, id)
		}
	}
	return models, latency, nil
}
