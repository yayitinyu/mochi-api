package main

import (
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"

	"mochi-api/common"
	"mochi-api/model"
	"mochi-api/router"
)

func main() {
	if err := model.InitDB(); err != nil {
		log.Fatalf("failed to init database: %v", err)
	}

	secret, err := sessionSecret()
	if err != nil {
		log.Fatalf("failed to load session secret: %v", err)
	}

	gin.SetMode(gin.ReleaseMode)
	server := gin.New()
	server.Use(gin.Logger(), gin.Recovery())

	// Gin trusts all proxies by default, which lets any client spoof its
	// address via X-Forwarded-For. Trust none unless explicitly configured
	// (comma-separated IPs/CIDRs of the reverse proxy in front of mochi).
	if trusted := os.Getenv("TRUSTED_PROXIES"); trusted != "" {
		if err := server.SetTrustedProxies(strings.Split(trusted, ",")); err != nil {
			log.Fatalf("invalid TRUSTED_PROXIES: %v", err)
		}
	} else if err := server.SetTrustedProxies(nil); err != nil {
		log.Fatalf("failed to configure trusted proxies: %v", err)
	}

	store := cookie.NewStore([]byte(secret))
	store.Options(sessions.Options{
		Path:     "/",
		MaxAge:   30 * 24 * 3600,
		HttpOnly: true,
		// Lax blocks the session cookie on cross-site POSTs, which is the
		// CSRF surface of the JSON dashboard API.
		SameSite: http.SameSiteLaxMode,
	})
	server.Use(sessions.Sessions("mochi_session", store))

	router.SetApiRouter(server)
	router.SetRelayRouter(server)
	router.SetWebRouter(server)

	log.Printf("mochi-api listening on :%s", common.Port)
	if err := server.Run(":" + common.Port); err != nil {
		log.Fatalf("server exited: %v", err)
	}
}

// sessionSecret returns the persisted session secret, generating and
// storing one on first launch so logins survive restarts.
func sessionSecret() (string, error) {
	secret, err := model.GetOption("session_secret")
	if err != nil {
		return "", err
	}
	if secret == "" {
		secret = common.GenerateKey(32)
		if err := model.SetOption("session_secret", secret); err != nil {
			return "", err
		}
	}
	return secret, nil
}
