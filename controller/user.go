package controller

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"mochi-api/common"
	"mochi-api/model"
)

func ListUsers(c *gin.Context) {
	users, err := model.ListUsers()
	if err != nil {
		respondError(c, http.StatusInternalServerError, "数据库错误")
		return
	}
	respondData(c, users)
}

type updateUserRequest struct {
	Status   *int    `json:"status"`
	Role     *int    `json:"role"`
	Password *string `json:"password"`
}

// UpdateUser applies partial updates (status, role, password) to a
// user. Admins may reset their own password but cannot change their
// own status or role, so an instance can never lock out its last admin.
func UpdateUser(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		respondError(c, http.StatusBadRequest, "无效的用户 ID")
		return
	}
	var req updateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "请求格式错误")
		return
	}
	self := id == c.GetInt("id")
	if req.Status != nil {
		if self {
			respondError(c, http.StatusBadRequest, "不能修改自己的状态")
			return
		}
		if *req.Status != model.StatusEnabled && *req.Status != model.StatusDisabled {
			respondError(c, http.StatusBadRequest, "无效的状态值")
			return
		}
		if err := model.UpdateUserStatus(id, *req.Status); err != nil {
			respondUpdateError(c, err)
			return
		}
	}
	if req.Role != nil {
		if self {
			respondError(c, http.StatusBadRequest, "不能修改自己的角色")
			return
		}
		if *req.Role != model.RoleUser && *req.Role != model.RoleAdmin {
			respondError(c, http.StatusBadRequest, "无效的角色值")
			return
		}
		if err := model.UpdateUserRole(id, *req.Role); err != nil {
			respondUpdateError(c, err)
			return
		}
	}
	if req.Password != nil {
		if len(*req.Password) < 8 || len(*req.Password) > 64 {
			respondError(c, http.StatusBadRequest, "密码长度需在 8-64 个字符之间")
			return
		}
		hash, err := common.HashPassword(*req.Password)
		if err != nil {
			respondError(c, http.StatusInternalServerError, "密码处理失败")
			return
		}
		if err := model.UpdateUserPassword(id, hash); err != nil {
			respondUpdateError(c, err)
			return
		}
	}
	respondData(c, nil)
}

func DeleteUser(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		respondError(c, http.StatusBadRequest, "无效的用户 ID")
		return
	}
	if id == c.GetInt("id") {
		respondError(c, http.StatusBadRequest, "不能删除自己")
		return
	}
	if err := model.DeleteUserCascade(id); err != nil {
		respondUpdateError(c, err)
		return
	}
	respondData(c, nil)
}

func respondUpdateError(c *gin.Context, err error) {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		respondError(c, http.StatusNotFound, "用户不存在")
		return
	}
	respondError(c, http.StatusInternalServerError, "数据库错误")
}
