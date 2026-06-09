package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func init() { gin.SetMode(gin.TestMode) }

func newRouter(mw gin.HandlerFunc) *gin.Engine {
	r := gin.New()
	r.Use(mw)
	r.GET("/api/v1/test", func(c *gin.Context) { c.Status(http.StatusOK) })
	r.GET("/healthz", func(c *gin.Context) { c.Status(http.StatusOK) })
	return r
}

// ── APIKeyAuth ────────────────────────────────────────────────────────────────

func TestAPIKeyAuth_NoKeyConfigured_AllowsAll(t *testing.T) {
	r := newRouter(APIKeyAuth(""))
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/test", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("no key configured: want 200, got %d", w.Code)
	}
}

func TestAPIKeyAuth_ValidXApiKey(t *testing.T) {
	r := newRouter(APIKeyAuth("secret"))
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/test", nil)
	req.Header.Set("X-Api-Key", "secret")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("valid X-Api-Key: want 200, got %d", w.Code)
	}
}

func TestAPIKeyAuth_ValidBearerToken(t *testing.T) {
	r := newRouter(APIKeyAuth("secret"))
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/test", nil)
	req.Header.Set("Authorization", "Bearer secret")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("valid Bearer: want 200, got %d", w.Code)
	}
}

func TestAPIKeyAuth_WrongKey_Returns401(t *testing.T) {
	r := newRouter(APIKeyAuth("secret"))
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/test", nil)
	req.Header.Set("X-Api-Key", "wrong")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("wrong key: want 401, got %d", w.Code)
	}
}

func TestAPIKeyAuth_MissingKey_Returns401(t *testing.T) {
	r := newRouter(APIKeyAuth("secret"))
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/test", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("missing key: want 401, got %d", w.Code)
	}
}

func TestAPIKeyAuth_HealthzExempt(t *testing.T) {
	r := newRouter(APIKeyAuth("secret"))
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/healthz", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("/healthz should bypass auth, got %d", w.Code)
	}
}

// ── RateLimiter ───────────────────────────────────────────────────────────────

func TestRateLimiter_AllowsWithinBurst(t *testing.T) {
	r := newRouter(RateLimiter(100, 5))
	for i := 0; i < 5; i++ {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/test", nil)
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("request %d within burst: want 200, got %d", i+1, w.Code)
		}
	}
}

func TestRateLimiter_BlocksAfterBurst(t *testing.T) {
	r := newRouter(RateLimiter(1, 3)) // 1 rps, burst 3
	// Drain the burst.
	for i := 0; i < 3; i++ {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/test", nil)
		r.ServeHTTP(w, req)
	}
	// Next request should be rate-limited.
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/test", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("after burst exhausted: want 429, got %d", w.Code)
	}
}

func TestRateLimiter_RefillsTokensOverTime(t *testing.T) {
	r := newRouter(RateLimiter(100, 1)) // 100 rps, burst 1
	// Drain.
	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest("GET", "/api/v1/test", nil)
	r.ServeHTTP(w1, req1)
	// Wait for refill.
	time.Sleep(20 * time.Millisecond)
	// Should be allowed again.
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", "/api/v1/test", nil)
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Errorf("after refill: want 200, got %d", w2.Code)
	}
}
