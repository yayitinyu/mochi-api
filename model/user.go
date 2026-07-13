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
	Status    int    `gorm:"default:1" json:"status"`
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

func ListUsers() ([]User, error) {
	users := make([]User, 0)
	err := DB.Order("id asc").Find(&users).Error
	return users, err
}

func updateUserColumn(id int, column string, value any) error {
	res := DB.Model(&User{}).Where("id = ?", id).Update(column, value)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func UpdateUserStatus(id, status int) error {
	return updateUserColumn(id, "status", status)
}

func UpdateUserRole(id, role int) error {
	return updateUserColumn(id, "role", role)
}

func UpdateUserPassword(id int, passwordHash string) error {
	return updateUserColumn(id, "password", passwordHash)
}

// DeleteUserCascade removes the user and their tokens in one transaction.
// Logs are kept so historical usage stats stay complete.
func DeleteUserCascade(id int) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ?", id).Delete(&Token{}).Error; err != nil {
			return err
		}
		res := tx.Delete(&User{}, id)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
}

// IsUserEnabled reports whether the user exists and is not disabled.
func IsUserEnabled(id int) (bool, error) {
	var status int
	err := DB.Model(&User{}).Where("id = ?", id).Select("status").Scan(&status).Error
	if err != nil {
		return false, err
	}
	return status == StatusEnabled, nil
}
