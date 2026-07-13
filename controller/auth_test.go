package controller_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"mochi-api/common"
	"mochi-api/model"
	"mochi-api/router"
)

// newTestServer boots a fresh SQLite database and a gin engine wired
// like main.go (session cookies + API/relay routers).
func newTestServer(t *testing.T) *gin.Engine {
	t.Helper()
	common.DataDir = t.TempDir()
	require.NoError(t, model.InitDB())
	t.Cleanup(func() {
		// Close the pooled connection so TempDir cleanup works on Windows.
		if sqlDB, err := model.DB.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})

	gin.SetMode(gin.TestMode)
	server := gin.New()
	store := cookie.NewStore([]byte("test-secret"))
	server.Use(sessions.Sessions("mochi_session", store))
	router.SetApiRouter(server)
	router.SetRelayRouter(server)
	return server
}

func doJSON(t *testing.T, server *gin.Engine, method, path string, body any, cookies []*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		require.NoError(t, err)
		reader = bytes.NewReader(raw)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	for _, c := range cookies {
		req.AddCookie(c)
	}
	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, req)
	return recorder
}

func register(t *testing.T, server *gin.Engine, username, password, inviteCode string) *httptest.ResponseRecorder {
	t.Helper()
	return doJSON(t, server, http.MethodPost, "/api/auth/register", gin.H{
		"username": username, "password": password, "invite_code": inviteCode,
	}, nil)
}

func login(t *testing.T, server *gin.Engine, username, password string) *httptest.ResponseRecorder {
	t.Helper()
	return doJSON(t, server, http.MethodPost, "/api/auth/login", gin.H{
		"username": username, "password": password,
	}, nil)
}

func responseData(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var envelope struct {
		Success bool           `json:"success"`
		Data    map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
	return envelope.Data
}

func TestFirstUserBypassesClosedModeAndBecomesAdmin(t *testing.T) {
	server := newTestServer(t)
	require.NoError(t, model.SetOption(model.OptionRegisterMode, model.RegisterModeClosed))

	rec := register(t, server, "admin", "password123", "")
	require.Equal(t, http.StatusOK, rec.Code)
	data := responseData(t, rec)
	require.EqualValues(t, model.RoleAdmin, data["role"])

	rec = register(t, server, "second", "password123", "")
	require.Equal(t, http.StatusForbidden, rec.Code)
}

func TestRegisterInviteFlow(t *testing.T) {
	server := newTestServer(t)
	rec := register(t, server, "admin", "password123", "")
	require.Equal(t, http.StatusOK, rec.Code)
	require.NoError(t, model.SetOption(model.OptionRegisterMode, model.RegisterModeInvite))

	// Missing code.
	rec = register(t, server, "alice", "password123", "")
	require.Equal(t, http.StatusBadRequest, rec.Code)

	// Wrong code.
	rec = register(t, server, "alice", "password123", "bogus-code")
	require.Equal(t, http.StatusBadRequest, rec.Code)

	// Valid code.
	codes, err := model.CreateInviteCodes(1, 1)
	require.NoError(t, err)
	rec = register(t, server, "alice", "password123", codes[0].Code)
	require.Equal(t, http.StatusOK, rec.Code)

	// The code is one-shot.
	rec = register(t, server, "bob", "password123", codes[0].Code)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestDisabledUserSessionAndLoginRejected(t *testing.T) {
	server := newTestServer(t)
	require.Equal(t, http.StatusOK, register(t, server, "admin", "password123", "").Code)
	rec := register(t, server, "alice", "password123", "")
	require.Equal(t, http.StatusOK, rec.Code)
	aliceId := int(responseData(t, rec)["id"].(float64))
	cookies := rec.Result().Cookies()

	rec = doJSON(t, server, http.MethodGet, "/api/auth/me", nil, cookies)
	require.Equal(t, http.StatusOK, rec.Code)

	require.NoError(t, model.UpdateUserStatus(aliceId, model.StatusDisabled))

	// Existing session is rejected immediately.
	rec = doJSON(t, server, http.MethodGet, "/api/auth/me", nil, cookies)
	require.Equal(t, http.StatusUnauthorized, rec.Code)

	// Fresh logins are rejected too.
	require.Equal(t, http.StatusForbidden, login(t, server, "alice", "password123").Code)
}

func TestDisabledUserTokenRejected(t *testing.T) {
	server := newTestServer(t)
	rec := register(t, server, "alice", "password123", "")
	require.Equal(t, http.StatusOK, rec.Code)
	aliceId := int(responseData(t, rec)["id"].(float64))
	require.NoError(t, model.CreateToken(&model.Token{
		UserId: aliceId, Key: "testkey12345", Name: "t", Status: model.StatusEnabled,
	}))

	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	req.Header.Set("Authorization", "Bearer sk-testkey12345")
	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusOK, recorder.Code)

	require.NoError(t, model.UpdateUserStatus(aliceId, model.StatusDisabled))

	recorder = httptest.NewRecorder()
	server.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusUnauthorized, recorder.Code)
}

func TestAdminCannotDisableDemoteOrDeleteSelf(t *testing.T) {
	server := newTestServer(t)
	rec := register(t, server, "admin", "password123", "")
	require.Equal(t, http.StatusOK, rec.Code)
	adminId := int(responseData(t, rec)["id"].(float64))
	cookies := rec.Result().Cookies()

	path := fmt.Sprintf("/api/users/%d", adminId)
	rec = doJSON(t, server, http.MethodPut, path, gin.H{"status": model.StatusDisabled}, cookies)
	require.Equal(t, http.StatusBadRequest, rec.Code)

	rec = doJSON(t, server, http.MethodPut, path, gin.H{"role": model.RoleUser}, cookies)
	require.Equal(t, http.StatusBadRequest, rec.Code)

	rec = doJSON(t, server, http.MethodDelete, path, nil, cookies)
	require.Equal(t, http.StatusBadRequest, rec.Code)

	// Resetting one's own password is allowed.
	rec = doJSON(t, server, http.MethodPut, path, gin.H{"password": "newpassword123"}, cookies)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, http.StatusOK, login(t, server, "admin", "newpassword123").Code)
}

func TestUserManagementByAdmin(t *testing.T) {
	server := newTestServer(t)
	rec := register(t, server, "admin", "password123", "")
	require.Equal(t, http.StatusOK, rec.Code)
	adminCookies := rec.Result().Cookies()

	rec = register(t, server, "alice", "password123", "")
	require.Equal(t, http.StatusOK, rec.Code)
	aliceId := int(responseData(t, rec)["id"].(float64))
	aliceCookies := rec.Result().Cookies()

	// Non-admin is rejected from admin endpoints.
	rec = doJSON(t, server, http.MethodGet, "/api/users", nil, aliceCookies)
	require.Equal(t, http.StatusForbidden, rec.Code)

	rec = doJSON(t, server, http.MethodGet, "/api/users", nil, adminCookies)
	require.Equal(t, http.StatusOK, rec.Code)

	// Promote alice; role change takes effect without re-login because
	// UserAuth reads the role from the database.
	path := fmt.Sprintf("/api/users/%d", aliceId)
	rec = doJSON(t, server, http.MethodPut, path, gin.H{"role": model.RoleAdmin}, adminCookies)
	require.Equal(t, http.StatusOK, rec.Code)
	rec = doJSON(t, server, http.MethodGet, "/api/users", nil, aliceCookies)
	require.Equal(t, http.StatusOK, rec.Code)

	// Delete alice.
	rec = doJSON(t, server, http.MethodDelete, path, nil, adminCookies)
	require.Equal(t, http.StatusOK, rec.Code)
	user, err := model.GetUserByUsername("alice")
	require.NoError(t, err)
	require.Nil(t, user)
}

func TestPublicStatusAndSettings(t *testing.T) {
	server := newTestServer(t)
	rec := register(t, server, "admin", "password123", "")
	require.Equal(t, http.StatusOK, rec.Code)
	cookies := rec.Result().Cookies()

	rec = doJSON(t, server, http.MethodGet, "/api/status", nil, nil)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "open", responseData(t, rec)["register_mode"])

	rec = doJSON(t, server, http.MethodPut, "/api/settings", gin.H{"register_mode": "invite"}, cookies)
	require.Equal(t, http.StatusOK, rec.Code)

	rec = doJSON(t, server, http.MethodPut, "/api/settings", gin.H{"register_mode": "bogus"}, cookies)
	require.Equal(t, http.StatusBadRequest, rec.Code)

	rec = doJSON(t, server, http.MethodGet, "/api/status", nil, nil)
	require.Equal(t, "invite", responseData(t, rec)["register_mode"])
}
