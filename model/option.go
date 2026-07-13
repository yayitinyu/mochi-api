package model

import (
	"errors"

	"gorm.io/gorm"
)

// Option is a simple key-value store for instance-level settings,
// e.g. the persisted session secret.
type Option struct {
	Key   string `gorm:"primaryKey;size:64" json:"key"`
	Value string `json:"value"`
}

const (
	OptionRegisterMode = "register_mode"

	RegisterModeOpen   = "open"
	RegisterModeInvite = "invite"
	RegisterModeClosed = "closed"
)

// GetRegisterMode returns the current registration mode. Missing,
// invalid, or unreadable values fall back to open so that existing
// deployments keep their pre-option behavior.
func GetRegisterMode() string {
	value, err := GetOption(OptionRegisterMode)
	if err != nil {
		return RegisterModeOpen
	}
	switch value {
	case RegisterModeInvite, RegisterModeClosed:
		return value
	default:
		return RegisterModeOpen
	}
}

// GetOption returns the stored value for key, or "" when absent.
func GetOption(key string) (string, error) {
	var opt Option
	err := DB.Where("key = ?", key).First(&opt).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return opt.Value, nil
}

// SetOption inserts or updates a key-value pair.
func SetOption(key, value string) error {
	return DB.Save(&Option{Key: key, Value: value}).Error
}
