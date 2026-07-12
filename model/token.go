package model

import (
	"errors"

	"gorm.io/gorm"
)

type Token struct {
	Id        int    `gorm:"primaryKey" json:"id"`
	UserId    int    `gorm:"index" json:"user_id"`
	Key       string `gorm:"uniqueIndex;size:64" json:"-"` // stored without the "sk-" prefix
	Name      string `json:"name"`
	Status    int    `json:"status"`
	CreatedAt int64  `json:"created_at"`
}

// KeyPreview returns a masked form like "sk-abcd****wxyz" for list views.
func (t *Token) KeyPreview() string {
	if len(t.Key) < 8 {
		return "sk-****"
	}
	return "sk-" + t.Key[:4] + "****" + t.Key[len(t.Key)-4:]
}

func GetTokensByUserId(userId int) ([]Token, error) {
	var tokens []Token
	err := DB.Where("user_id = ?", userId).Order("id desc").Find(&tokens).Error
	return tokens, err
}

func GetTokenById(id, userId int) (*Token, error) {
	var token Token
	err := DB.Where("id = ? AND user_id = ?", id, userId).First(&token).Error
	if err != nil {
		return nil, err
	}
	return &token, nil
}

// GetTokenByKey returns nil, nil when no token matches the key.
func GetTokenByKey(key string) (*Token, error) {
	var token Token
	err := DB.Where("key = ?", key).First(&token).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &token, nil
}

func CreateToken(token *Token) error {
	return DB.Create(token).Error
}

func UpdateToken(token *Token) error {
	return DB.Model(token).Select("name", "status").Updates(token).Error
}

func DeleteToken(id, userId int) error {
	return DB.Where("id = ? AND user_id = ?", id, userId).Delete(&Token{}).Error
}
