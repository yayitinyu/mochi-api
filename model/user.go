package model

import (
	"errors"

	"gorm.io/gorm"
)

const (
	RoleUser  = 1
	RoleAdmin = 10

	StatusEnabled  = 1
	StatusDisabled = 2
)

type User struct {
	Id        int    `gorm:"primaryKey" json:"id"`
	Username  string `gorm:"uniqueIndex;size:64" json:"username"`
	Password  string `json:"-"` // bcrypt hash
	Role      int    `json:"role"`
	CreatedAt int64  `json:"created_at"`
}

// GetUserByUsername returns nil, nil when the user does not exist.
func GetUserByUsername(username string) (*User, error) {
	var user User
	err := DB.Where("username = ?", username).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func GetUserById(id int) (*User, error) {
	var user User
	err := DB.First(&user, id).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func CountUsers() (int64, error) {
	var count int64
	err := DB.Model(&User{}).Count(&count).Error
	return count, err
}

func CreateUser(user *User) error {
	return DB.Create(user).Error
}
