package relay

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
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
	base, mode := splitBaseURL(baseURL)
	var url string
	switch {
	case chType == model.ChannelTypeGemini:
		// Gemini has no bare "models" leaf under a custom prefix; the version
		// segment is always required.
		if mode == standardBase {
			url = base + "/v1beta/models"
		} else {
			url = base + "/models"
		}
	case mode == exactEndpoint:
		// The exact-endpoint marker targets a chat endpoint, not a model list;
		// derive the models path from the host root instead.
		url = modelsURLFromExact(base)
	case mode == fullPrefix:
		url = base + "/models"
	default:
		url = base + "/v1/models"
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
	// xAI exposes richer image-model discovery on a dedicated endpoint that
	// may include models omitted from the generic /v1/models response.
	if imageModelsURL := xAIImageModelsURL(base); imageModelsURL != "" {
		if imageModels := fetchXAIImageModels(client, imageModelsURL, apiKey); len(imageModels) > 0 {
			seen := make(map[string]bool, len(models)+len(imageModels))
			for _, name := range models {
				seen[name] = true
			}
			for _, name := range imageModels {
				if !seen[name] {
					models = append(models, name)
					seen[name] = true
				}
			}
		}
	}
	return models, latency, nil
}

func xAIImageModelsURL(base string) string {
	parsed, err := url.Parse(base)
	if err != nil || !strings.EqualFold(parsed.Hostname(), "api.x.ai") {
		return ""
	}
	return parsed.Scheme + "://" + parsed.Host + "/v1/image-generation-models"
}

func fetchXAIImageModels(client *http.Client, endpoint, apiKey string) []string {
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil
	}
	var models []string
	for _, item := range gjson.GetBytes(body, "models").Array() {
		if id := item.Get("id").String(); id != "" {
			models = append(models, id)
		}
	}
	return models
}

// modelsURLFromExact guesses the model-list URL for an exact-endpoint ("#")
// base by stripping the known chat endpoint leaf and appending "models".
func modelsURLFromExact(exactURL string) string {
	for _, leaf := range []string{"/chat/completions", "/messages", "/responses", "/images/generations"} {
		if prefix, ok := strings.CutSuffix(exactURL, leaf); ok {
			return prefix + "/models"
		}
	}
	return strings.TrimSuffix(exactURL, "/") + "/models"
}
