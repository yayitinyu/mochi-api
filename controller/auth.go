package controller

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"

	"mochi-api/common"
	"mochi-api/model"
)

type authRequest struct {
	Username   string `json:"username"`
	Password   string `json:"password"`
	InviteCode string `json:"invite_code"`
}

func respondError(c *gin.Context, status int, message string) {
	c.JSON(status, gin.H{"success": false, "message": message})
}

func respondData(c *gin.Context, data any) {
	c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
}

// Register creates a user; the first registered user becomes admin.
func Register(c *gin.Context) {
	var req authRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "请求格式错误")
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	if len(req.Username) < 2 || len(req.Username) > 30 {
		respondError(c, http.StatusBadRequest, "用户名长度需在 2-30 个字符之间")
		return
	}
	if len(req.Password) < 8 || len(req.Password) > 64 {
		respondError(c, http.StatusBadRequest, "密码长度需在 8-64 个字符之间")
		return
	}
	existing, err := model.GetUserByUsername(req.Username)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "数据库错误")
		return
	}
	if existing != nil {
		respondError(c, http.StatusBadRequest, "用户名已被占用")
		return
	}
	count, err := model.CountUsers()
	if err != nil {
		respondError(c, http.StatusInternalServerError, "数据库错误")
		return
	}
	// The first user bootstraps the instance: they become admin and
	// bypass the register mode so a pre-configured "closed" setting
	// cannot lock a fresh deployment out.
	registerMode := model.RegisterModeOpen
	if count > 0 {
		registerMode = model.GetRegisterMode()
	}
	req.InviteCode = strings.TrimSpace(req.InviteCode)
	switch registerMode {
	case model.RegisterModeClosed:
		respondError(c, http.StatusForbidden, "注册已关闭")
		return
	case model.RegisterModeInvite:
		if req.InviteCode == "" {
			respondError(c, http.StatusBadRequest, "需要邀请码")
			return
		}
	}
	hash, err := common.HashPassword(req.Password)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "密码处理失败")
		return
	}
	role := model.RoleUser
	if count == 0 {
		role = model.RoleAdmin
	}
	user := &model.User{
		Username:  req.Username,
		Password:  hash,
		Role:      role,
		Status:    model.StatusEnabled,
		CreatedAt: time.Now().Unix(),
	}
	if registerMode == model.RegisterModeInvite {
		err = model.CreateUserWithInvite(user, req.InviteCode)
	} else {
		err = model.CreateUser(user)
	}
	if err != nil {
		if errors.Is(err, model.ErrInviteInvalid) {
			respondError(c, http.StatusBadRequest, "邀请码无效或已被使用")
			return
		}
		respondError(c, http.StatusInternalServerError, "创建用户失败")
		return
	}
	setLoginSession(c, user)
	respondData(c, user)
}

func Login(c *gin.Context) {
	var req authRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "请求格式错误")
		return
	}
	user, err := model.GetUserByUsername(strings.TrimSpace(req.Username))
	if err != nil {
		respondError(c, http.StatusInternalServerError, "数据库错误")
		return
	}
	if user == nil || !common.CheckPassword(req.Password, user.Password) {
		respondError(c, http.StatusUnauthorized, "用户名或密码错误")
		return
	}
	if user.Status != model.StatusEnabled {
		respondError(c, http.StatusForbidden, "账号已被禁用")
		return
	}
	setLoginSession(c, user)
	respondData(c, user)
}

func Logout(c *gin.Context) {
	session := sessions.Default(c)
	session.Clear()
	_ = session.Save()
	respondData(c, nil)
}

func Me(c *gin.Context) {
	user, err := model.GetUserById(c.GetInt("id"))
	if err != nil {
		respondError(c, http.StatusUnauthorized, "用户不存在")
		return
	}
	respondData(c, user)
}

func setLoginSession(c *gin.Context, user *model.User) {
	session := sessions.Default(c)
	session.Set("id", user.Id)
	session.Set("role", user.Role)
	_ = session.Save()
}
