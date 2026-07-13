package model

import (
	"errors"
	"time"

	"gorm.io/gorm"

	"mochi-api/common"
)

// ErrInviteInvalid is returned when an invite code does not exist or
// has already been redeemed.
var ErrInviteInvalid = errors.New("邀请码无效或已被使用")

type InviteCode struct {
	Id        int    `gorm:"primaryKey" json:"id"`
	Code      string `gorm:"uniqueIndex;size:32" json:"code"`
	CreatedBy int    `json:"created_by"`
	CreatedAt int64  `json:"created_at"`
	// UsedByUserId 0 means the code has not been redeemed yet.
	UsedByUserId int   `gorm:"index;default:0" json:"used_by_user_id"`
	UsedAt       int64 `json:"used_at"`
}

// InviteCodeView augments InviteCode with the redeemer's username for
// list views; empty when unused or the user was deleted.
type InviteCodeView struct {
	InviteCode
	UsedByUsername string `json:"used_by_username"`
}

func ListInviteCodes() ([]InviteCodeView, error) {
	codes := make([]InviteCodeView, 0)
	err := DB.Table("invite_codes").
		Select("invite_codes.*, COALESCE(users.username, '') as used_by_username").
		Joins("LEFT JOIN users ON users.id = invite_codes.used_by_user_id").
		Order("invite_codes.id desc").
		Scan(&codes).Error
	return codes, err
}

func CreateInviteCodes(creatorId, count int) ([]InviteCode, error) {
	now := time.Now().Unix()
	codes := make([]InviteCode, count)
	for i := range codes {
		codes[i] = InviteCode{
			Code:      common.GenerateKey(16),
			CreatedBy: creatorId,
			CreatedAt: now,
		}
	}
	if err := DB.Create(&codes).Error; err != nil {
		return nil, err
	}
	return codes, nil
}

// DeleteInviteCode removes an unused code; deleting a redeemed code is
// refused to preserve the registration audit trail.
func DeleteInviteCode(id int) error {
	res := DB.Where("id = ? AND used_by_user_id = 0", id).Delete(&InviteCode{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// CreateUserWithInvite creates the user and redeems the invite code in
// one transaction. The conditional UPDATE makes redemption atomic: if a
// concurrent request already claimed the code, RowsAffected is 0 and
// the whole transaction (including the new user) rolls back.
func CreateUserWithInvite(user *User, code string) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(user).Error; err != nil {
			return err
		}
		res := tx.Model(&InviteCode{}).
			Where("code = ? AND used_by_user_id = 0", code).
			Updates(map[string]any{
				"used_by_user_id": user.Id,
				"used_at":         time.Now().Unix(),
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return ErrInviteInvalid
		}
		return nil
	})
}
