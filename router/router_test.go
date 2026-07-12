package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRelayPreflightAllowsBrowserAPIClients(t *testing.T) {
	gin.SetMode(gin.TestMode)
	server := gin.New()
	SetRelayRouter(server)

	req := httptest.NewRequest(http.MethodOptions, "/v1/responses", nil)
	req.Header.Set("Origin", "app://chatbox")
	req.Header.Set("Access-Control-Request-Headers", "authorization,content-type")
	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusNoContent, recorder.Code)
	require.Equal(t, "*", recorder.Header().Get("Access-Control-Allow-Origin"))
	require.Contains(t, recorder.Header().Get("Access-Control-Allow-Headers"), "Authorization")
	require.Contains(t, recorder.Header().Get("Access-Control-Allow-Methods"), "POST")
}
