package model

import (
	"sort"
	"strings"
)

const (
	ChannelTypeOpenAI    = "openai"
	ChannelTypeAnthropic = "anthropic"
	ChannelTypeGemini    = "gemini"
)

type Channel struct {
	Id      int    `gorm:"primaryKey" json:"id"`
	Name    string `json:"name"`
	Type    string `gorm:"size:32" json:"type"` // "openai" | "anthropic"
	BaseURL string `json:"base_url"`            // e.g. https://api.openai.com; a trailing "/" marks a full API prefix, a trailing "#" marks an exact endpoint URL
	ApiKey  string `json:"api_key"`
	Models  string `json:"models"` // comma-joined model names
	// Icon is either a preset icon key (e.g. "deepseek") or an image URL
	// for custom channels; the frontend resolves it.
	Icon      string `gorm:"size:512" json:"icon"`
	Priority  int    `json:"priority"`
	Status    int    `json:"status"`
	CreatedAt int64  `json:"created_at"`
}

// ModelList splits the comma-joined Models field into trimmed names.
func (ch *Channel) ModelList() []string {
	parts := strings.Split(ch.Models, ",")
	models := make([]string, 0, len(parts))
	for _, p := range parts {
		if name := strings.TrimSpace(p); name != "" {
			models = append(models, name)
		}
	}
	return models
}

// SupportsModel reports whether the channel serves the given model.
func (ch *Channel) SupportsModel(model string) bool {
	for _, m := range ch.ModelList() {
		if m == model {
			return true
		}
	}
	return false
}

func GetAllChannels() ([]Channel, error) {
	var channels []Channel
	err := DB.Order("priority desc, id asc").Find(&channels).Error
	return channels, err
}

// GetEnabledChannelsForModel returns enabled channels serving the model,
// ordered by priority descending.
func GetEnabledChannelsForModel(model string) ([]Channel, error) {
	var channels []Channel
	err := DB.Where("status = ?", StatusEnabled).Order("priority desc").Find(&channels).Error
	if err != nil {
		return nil, err
	}
	matched := make([]Channel, 0, len(channels))
	for _, ch := range channels {
		if ch.SupportsModel(model) {
			matched = append(matched, ch)
		}
	}
	return matched, nil
}

// GetEnabledModels returns the deduplicated union of models across enabled channels.
func GetEnabledModels() ([]string, error) {
	var channels []Channel
	err := DB.Where("status = ?", StatusEnabled).Find(&channels).Error
	if err != nil {
		return nil, err
	}
	return channelModelNames(channels), nil
}

// GetConfiguredModels returns models from all channels, including disabled
// channels, for dashboard configuration such as model pricing.
func GetConfiguredModels() ([]string, error) {
	channels, err := GetAllChannels()
	if err != nil {
		return nil, err
	}
	return channelModelNames(channels), nil
}

func channelModelNames(channels []Channel) []string {
	seen := make(map[string]struct{})
	models := make([]string, 0)
	for i := range channels {
		for _, name := range channels[i].ModelList() {
			if _, exists := seen[name]; exists {
				continue
			}
			seen[name] = struct{}{}
			models = append(models, name)
		}
	}
	sort.Strings(models)
	return models
}

func GetChannelById(id int) (*Channel, error) {
	var channel Channel
	err := DB.First(&channel, id).Error
	if err != nil {
		return nil, err
	}
	return &channel, nil
}

func CreateChannel(channel *Channel) error {
	return DB.Create(channel).Error
}

func UpdateChannel(channel *Channel) error {
	return DB.Model(channel).
		Select("name", "type", "base_url", "api_key", "models", "icon", "priority", "status").
		Updates(channel).Error
}

func DeleteChannel(id int) error {
	return DB.Delete(&Channel{}, id).Error
}
