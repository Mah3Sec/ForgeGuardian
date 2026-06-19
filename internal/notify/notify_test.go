package notify

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mah3sec/forgeguardian/internal/core"
)

// captureServer returns a test server that records the last request body.
func captureServer(t *testing.T) (*httptest.Server, *[]byte) {
	t.Helper()
	var captured []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		captured = body
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	return srv, &captured
}

// ── MaybeNotify ───────────────────────────────────────────────────────────────

func TestMaybeNotify_NothingConfigured_NoOp(t *testing.T) {
	cfg := Config{OnSeverity: "critical"}
	err := MaybeNotify(cfg, "pkg", []core.Finding{{Severity: core.SeverityCritical}})
	if err != nil {
		t.Errorf("no URLs configured: want nil, got %v", err)
	}
}

func TestMaybeNotify_OnSeverityEmpty_NoOp(t *testing.T) {
	srv, captured := captureServer(t)
	cfg := Config{OnSeverity: "", SlackWebhookURL: srv.URL}
	_ = MaybeNotify(cfg, "pkg", []core.Finding{{Severity: core.SeverityCritical}})
	if *captured != nil {
		t.Error("empty OnSeverity: webhook must not be called")
	}
}

func TestMaybeNotify_BelowThreshold_NoOp(t *testing.T) {
	srv, captured := captureServer(t)
	cfg := Config{OnSeverity: "critical", SlackWebhookURL: srv.URL}
	_ = MaybeNotify(cfg, "pkg", []core.Finding{{Severity: core.SeverityHigh}})
	if *captured != nil {
		t.Error("below threshold: webhook must not be called")
	}
}

func TestMaybeNotify_AtThreshold_Fires(t *testing.T) {
	srv, captured := captureServer(t)
	cfg := Config{OnSeverity: "critical", SlackWebhookURL: srv.URL}
	err := MaybeNotify(cfg, "evil-pkg", []core.Finding{{
		ID:       "CVE-2024-1234",
		Severity: core.SeverityCritical,
		Title:    "Remote code execution",
	}})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if *captured == nil {
		t.Fatal("webhook not called")
	}
	var payload map[string]string
	if err := json.Unmarshal(*captured, &payload); err != nil {
		t.Fatalf("invalid JSON payload: %v", err)
	}
	if payload["text"] == "" {
		t.Error("Slack payload missing 'text' field")
	}
}

func TestMaybeNotify_AboveThreshold_Fires(t *testing.T) {
	// critical finding with threshold=high — should fire
	srv, captured := captureServer(t)
	cfg := Config{OnSeverity: "high", SlackWebhookURL: srv.URL}
	_ = MaybeNotify(cfg, "pkg", []core.Finding{{Severity: core.SeverityCritical, Title: "crit"}})
	if *captured == nil {
		t.Error("critical finding above high threshold: webhook must fire")
	}
}

// ── SendSlack ─────────────────────────────────────────────────────────────────

func TestSendSlack_PostsJSON(t *testing.T) {
	srv, captured := captureServer(t)
	if err := SendSlack(srv.URL, "hello slack"); err != nil {
		t.Fatalf("SendSlack: %v", err)
	}
	var p map[string]string
	if err := json.Unmarshal(*captured, &p); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if p["text"] != "hello slack" {
		t.Errorf("want text='hello slack', got %q", p["text"])
	}
}

// ── SendDiscord ───────────────────────────────────────────────────────────────

func TestSendDiscord_PostsJSON(t *testing.T) {
	srv, captured := captureServer(t)
	if err := SendDiscord(srv.URL, "hello discord"); err != nil {
		t.Fatalf("SendDiscord: %v", err)
	}
	var p map[string]string
	if err := json.Unmarshal(*captured, &p); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if p["content"] != "hello discord" {
		t.Errorf("want content='hello discord', got %q", p["content"])
	}
}

// ── SendGeneric ───────────────────────────────────────────────────────────────

func TestSendGeneric_PostsJSON(t *testing.T) {
	srv, captured := captureServer(t)
	payload := map[string]any{"severity": "CRITICAL", "package": "lodash"}
	if err := SendGeneric(srv.URL, payload); err != nil {
		t.Fatalf("SendGeneric: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(*captured, &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if got["severity"] != "CRITICAL" {
		t.Errorf("want severity=CRITICAL, got %v", got["severity"])
	}
}

func TestSendGeneric_HTTP4xx_ReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()
	err := SendGeneric(srv.URL, map[string]any{"x": 1})
	if err == nil {
		t.Error("HTTP 401: want error, got nil")
	}
}

// ── severityOrd ───────────────────────────────────────────────────────────────

func TestSeverityOrd_Ordering(t *testing.T) {
	cases := []struct{ a, b core.Severity }{
		{core.SeverityCritical, core.SeverityHigh},
		{core.SeverityHigh, core.SeverityMedium},
		{core.SeverityMedium, core.SeverityLow},
	}
	for _, tc := range cases {
		if severityOrd(tc.a) <= severityOrd(tc.b) {
			t.Errorf("%s must rank higher than %s", tc.a, tc.b)
		}
	}
}
