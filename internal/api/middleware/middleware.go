// Package middleware provides Gin middleware for logging, recovery, CORS, rate limiting, and auth.
package middleware

import (
	"crypto/subtle"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mah3sec/forgeguardian/internal/auth"
)

// perIP holds a simple token-bucket state for one IP address.
type perIP struct {
	tokens   float64
	lastSeen time.Time
}

// RateLimiter returns a per-IP token-bucket rate limiter middleware.
// rps is the sustained requests-per-second allowed; burst is the maximum burst size.
func RateLimiter(rps float64, burst int) gin.HandlerFunc {
	var mu sync.Mutex
	clients := make(map[string]*perIP)

	// Evict stale entries every 5 minutes to avoid unbounded map growth.
	go func() {
		for range time.Tick(5 * time.Minute) {
			mu.Lock()
			cutoff := time.Now().Add(-10 * time.Minute)
			for ip, s := range clients {
				if s.lastSeen.Before(cutoff) {
					delete(clients, ip)
				}
			}
			mu.Unlock()
		}
	}()

	return func(c *gin.Context) {
		ip := c.ClientIP()
		now := time.Now()

		mu.Lock()
		s, ok := clients[ip]
		if !ok {
			s = &perIP{tokens: float64(burst), lastSeen: now}
			clients[ip] = s
		}
		// Refill tokens based on elapsed time.
		elapsed := now.Sub(s.lastSeen).Seconds()
		s.tokens += elapsed * rps
		if s.tokens > float64(burst) {
			s.tokens = float64(burst)
		}
		s.lastSeen = now
		allowed := s.tokens >= 1
		if allowed {
			s.tokens--
		}
		mu.Unlock()

		if !allowed {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "rate limit exceeded — slow down requests",
			})
			return
		}
		c.Next()
	}
}

// Logger returns a Gin middleware that logs requests via slog.
func Logger(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		log.Info("request",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"latency_ms", time.Since(start).Milliseconds(),
			"client_ip", c.ClientIP(),
		)
	}
}

// Recovery returns a Gin middleware that recovers from panics.
func Recovery(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				log.Error("panic recovered", "error", r, "path", c.Request.URL.Path)
				c.AbortWithStatusJSON(http.StatusInternalServerError,
					gin.H{"error": "internal server error"})
			}
		}()
		c.Next()
	}
}

// APIKeyAuth returns middleware that enforces a static API key.
// Reads the key from the Authorization header ("Bearer <key>") or X-Api-Key header.
// Skips auth for /healthz, /metrics, and the dashboard SPA (anything not under /api/).
// Set FG_API_KEY env var; if empty, auth is disabled (dev mode).
func APIKeyAuth(validKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// No key configured → dev mode, allow everything.
		if validKey == "" {
			c.Next()
			return
		}
		// Exempt non-API paths (health probe, metrics, dashboard assets).
		if !strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.Next()
			return
		}

		// Extract key from Authorization: Bearer <key>  OR  X-Api-Key: <key>
		key := c.GetHeader("X-Api-Key")
		if key == "" {
			if auth := c.GetHeader("Authorization"); strings.HasPrefix(auth, "Bearer ") {
				key = strings.TrimPrefix(auth, "Bearer ")
			}
		}

		// Constant-time compare prevents timing attacks.
		if subtle.ConstantTimeCompare([]byte(key), []byte(validKey)) != 1 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "invalid or missing API key — set X-Api-Key or Authorization: Bearer <key>",
			})
			return
		}
		c.Next()
	}
}

// SecurityHeaders sets standard security headers on every response.
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-XSS-Protection", "0")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Header("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
			c.Header("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
		}
		c.Next()
	}
}

// CORS adds CORS headers for the dashboard. If dashboardOrigin is non-empty,
// that specific origin is echoed back with Access-Control-Allow-Credentials
// set (required for cookies to work cross-origin). Otherwise falls back to
// the permissive wildcard behavior (backward compat when
// FG_DASHBOARD_ORIGIN isn't customized).
func CORS(dashboardOrigin string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if dashboardOrigin != "" {
			c.Header("Access-Control-Allow-Origin", dashboardOrigin)
			c.Header("Access-Control-Allow-Credentials", "true")
		} else {
			origin := c.GetHeader("Origin")
			if origin != "" && isLocalhostOrigin(origin) {
				c.Header("Access-Control-Allow-Origin", origin)
				c.Header("Access-Control-Allow-Credentials", "true")
			}
		}
		c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type,Authorization,X-Api-Key")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func isLocalhostOrigin(origin string) bool {
	lower := strings.ToLower(origin)
	for _, prefix := range []string{
		"http://localhost", "https://localhost",
		"http://127.0.0.1", "https://127.0.0.1",
		"http://[::1]", "https://[::1]",
		"http://0.0.0.0", "https://0.0.0.0",
	} {
		if lower == prefix {
			return true
		}
		// Allow origin with a port suffix (e.g. http://localhost:8080)
		if strings.HasPrefix(lower, prefix+":") {
			return true
		}
		// Allow origin with a path (e.g. http://localhost/)
		if strings.HasPrefix(lower, prefix+"/") {
			return true
		}
	}
	return false
}

// LoginRateLimiter returns a per-IP token-bucket rate limiter tuned tightly
// for the login route: 5 attempts per minute, burst of 5. Register this on
// the login route only (not globally) to slow down credential-guessing
// without affecting other endpoints.
func LoginRateLimiter() gin.HandlerFunc {
	return RateLimiter(5.0/60.0, 5)
}

// authExemptPaths are the exact routes that must remain reachable
// regardless of auth state (login/me need to work while logged out;
// logout needs to work to actually clear a session).
var authExemptPaths = map[string]bool{
	"/api/v1/auth/login":    true,
	"/api/v1/auth/logout":   true,
	"/api/v1/auth/me":       true,
	"/api/v1/auth/password": true,
}

// DualAuth returns middleware that authorizes a request if EITHER the
// static API key OR a valid session cookie is present. It replaces a bare
// APIKeyAuth() call so that dashboard session-cookie auth and the legacy
// API-key auth can coexist without one clobbering the other.
//
// Behavior:
//   - Skips non-/api/ paths (health probe, metrics, dashboard assets) — same
//     exemption APIKeyAuth already had.
//   - Skips /api/v1/auth/* (login/logout/me must be reachable unauthenticated).
//   - If apiKey == "" and !authEnabled, auth is fully disabled (today's dev
//     mode) — allow everything.
//   - If apiKey != "", check X-Api-Key / Authorization: Bearer via
//     constant-time compare. Success authorizes the request.
//   - Otherwise, if authEnabled, check the fg_session cookie via
//     auth.ParseToken. Success authorizes the request.
//   - If neither path authorized the request and at least one auth
//     mechanism is actually configured, return 401.
func DualAuth(apiKey string, sessionSecret []byte, authEnabled bool, adminIdentity ...*auth.AdminIdentity) gin.HandlerFunc {
	var admin *auth.AdminIdentity
	if len(adminIdentity) > 0 {
		admin = adminIdentity[0]
	}
	return func(c *gin.Context) {
		path := c.Request.URL.Path

		// Exempt non-API paths (health probe, metrics, dashboard assets).
		if !strings.HasPrefix(path, "/api/") {
			c.Next()
			return
		}
		// Exempt the auth endpoints themselves — must be reachable while
		// logged out (login/me) or to clear a session (logout).
		if authExemptPaths[path] {
			c.Next()
			return
		}

		// No auth mechanism configured at all → today's fully-open dev mode.
		if apiKey == "" && !authEnabled {
			c.Next()
			return
		}

		// Try the API key first.
		if apiKey != "" {
			key := c.GetHeader("X-Api-Key")
			if key == "" {
				if hdr := c.GetHeader("Authorization"); strings.HasPrefix(hdr, "Bearer ") {
					key = strings.TrimPrefix(hdr, "Bearer ")
				}
			}
			if subtle.ConstantTimeCompare([]byte(key), []byte(apiKey)) == 1 {
				c.Next()
				return
			}
		}

		// Fall through to session-cookie auth.
		if authEnabled {
			if tok, err := c.Cookie("fg_session"); err == nil {
				if _, err := auth.ParseToken(tok, sessionSecret); err == nil {
					// Enforce server-side password change requirement
					if admin != nil && admin.PasswordMustChange && path != "/api/v1/auth/password" {
						c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
							"error":                "password change required before accessing other endpoints",
							"password_must_change": true,
						})
						return
					}
					c.Next()
					return
				}
			}
		}

		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"error": "unauthenticated — set X-Api-Key/Authorization: Bearer <key>, or log in via /api/v1/auth/login",
		})
	}
}
