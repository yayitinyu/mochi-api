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
	req.Header.Set("Access-Control-Request-Headers", "authorization,content-type,x-stainless-lang")
	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusNoContent, recorder.Code)
	require.Equal(t, "*", recorder.Header().Get("Access-Control-Allow-Origin"))
	// The requested headers are reflected verbatim, so vendor SDK headers
	// (x-stainless-*, anthropic-beta, ...) are allowed without a static list.
	require.Equal(t, "authorization,content-type,x-stainless-lang",
		recorder.Header().Get("Access-Control-Allow-Headers"))
	require.Contains(t, recorder.Header().Get("Access-Control-Allow-Methods"), "POST")
}

func TestRelayPreflightFallsBackWhenNoRequestHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	server := gin.New()
	SetRelayRouter(server)

	req := httptest.NewRequest(http.MethodOptions, "/v1/messages", nil)
	req.Header.Set("Origin", "app://chatbox")
	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusNoContent, recorder.Code)
	allow := recorder.Header().Get("Access-Control-Allow-Headers")
	require.Contains(t, allow, "Authorization")
	require.Contains(t, allow, "x-api-key")
}
