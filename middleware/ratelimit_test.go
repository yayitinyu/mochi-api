package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRateLimitBlocksAfterMax(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/login", RateLimit(3, time.Minute), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	do := func(ip string) int {
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/login", nil)
		req.RemoteAddr = ip + ":12345"
		router.ServeHTTP(recorder, req)
		return recorder.Code
	}

	for i := 0; i < 3; i++ {
		require.Equal(t, http.StatusOK, do("10.0.0.1"), "request %d within the limit", i+1)
	}
	require.Equal(t, http.StatusTooManyRequests, do("10.0.0.1"))
	// Another client is unaffected.
	require.Equal(t, http.StatusOK, do("10.0.0.2"))
}

func TestBodyLimitRejectsOversizedBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/api", BodyLimit(16), func(c *gin.Context) {
		var payload struct {
			Name string `json:"name"`
		}
		if err := c.ShouldBindJSON(&payload); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}
		c.Status(http.StatusOK)
	})

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api", strings.NewReader(`{"name":"ok"}`))
	router.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusOK, recorder.Code)

	recorder = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api",
		strings.NewReader(`{"name":"`+strings.Repeat("x", 64)+`"}`))
	router.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
}
