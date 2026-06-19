// Package notify delivers scan finding alerts to external webhook endpoints.
// Supported targets: Slack, Discord, and generic HTTP webhooks.
package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/mah3sec/forgeguardian/internal/core"
)

// Config holds webhook delivery configuration.
type Config struct {
	SlackWebhookURL   string `yaml:"slack_webhook_url"`
	DiscordWebhookURL string `yaml:"discord_webhook_url"`
	GenericWebhookURL string `yaml:"webhook_url"`
	OnSeverity        string `yaml:"on_severity"` // "critical"|"high"|"medium"|"low"; "" = disabled
}

// Redirects disabled deliberately: a real Slack/Discord/generic webhook URL
// never legitimately 3xx-redirects a POST. A bad/expired/typo'd webhook URL
// (e.g. a stale https://hooks.slack.com/... path) gets redirected to a real
// 200 OK marketing/docs page by the target host, which the default
// redirect-following client would silently report as a successful delivery.
var httpClient = &http.Client{
	Timeout: 10 * time.Second,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

// MaybeNotify fires webhook notifications if any finding meets the OnSeverity
// threshold. It is a best-effort operation — errors are returned but do not
// fail the scan.
func MaybeNotify(cfg Config, pkgName string, findings []core.Finding) error {
	if cfg.OnSeverity == "" {
		return nil
	}
	if cfg.SlackWebhookURL == "" && cfg.DiscordWebhookURL == "" && cfg.GenericWebhookURL == "" {
		return nil
	}

	threshold := severityOrd(core.Severity(strings.ToUpper(cfg.OnSeverity)))
	var triggered []core.Finding
	for _, f := range findings {
		if severityOrd(f.Severity) >= threshold {
			triggered = append(triggered, f)
		}
	}
	if len(triggered) == 0 {
		return nil
	}

	top := triggered[0]
	msg := fmt.Sprintf("[%s] %s — %s (%s)", top.Severity, pkgName, top.Title, top.ID)

	var errs []string
	if cfg.SlackWebhookURL != "" {
		if err := SendSlack(cfg.SlackWebhookURL, msg); err != nil {
			errs = append(errs, "slack: "+err.Error())
		}
	}
	if cfg.DiscordWebhookURL != "" {
		if err := SendDiscord(cfg.DiscordWebhookURL, msg); err != nil {
			errs = append(errs, "discord: "+err.Error())
		}
	}
	if cfg.GenericWebhookURL != "" {
		payload := map[string]any{
			"package":   pkgName,
			"severity":  string(top.Severity),
			"finding":   top.ID,
			"title":     top.Title,
			"count":     len(triggered),
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		}
		if err := SendGeneric(cfg.GenericWebhookURL, payload); err != nil {
			errs = append(errs, "generic: "+err.Error())
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("notify: %s", strings.Join(errs, "; "))
	}
	return nil
}

// SendSlack posts a plain-text message to a Slack Incoming Webhook URL.
func SendSlack(webhookURL, message string) error {
	payload, _ := json.Marshal(map[string]string{"text": message})
	return postJSON(webhookURL, payload)
}

// SendDiscord posts a plain-text message to a Discord webhook URL.
func SendDiscord(webhookURL, message string) error {
	payload, _ := json.Marshal(map[string]string{"content": message})
	return postJSON(webhookURL, payload)
}

// SendGeneric posts a JSON payload to an arbitrary webhook URL.
func SendGeneric(webhookURL string, payload map[string]any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return postJSON(webhookURL, data)
}

func postJSON(url string, body []byte) error {
	resp, err := httpClient.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return nil
}

func severityOrd(s core.Severity) int {
	switch s {
	case core.SeverityCritical:
		return 4
	case core.SeverityHigh:
		return 3
	case core.SeverityMedium:
		return 2
	case core.SeverityLow:
		return 1
	default:
		return 0
	}
}
