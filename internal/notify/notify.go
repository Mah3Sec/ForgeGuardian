// Package notify delivers scan event alerts to external webhook endpoints.
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

// ScanEvent carries full context about a scan for webhook delivery.
type ScanEvent struct {
	Type      string // "start", "complete", "error"
	Package   string
	ScanType  string // "registry", "local", "upload", "remote"
	Target    string // path, URL, or "user@host" for remote
	Engines   []string
	Duration  time.Duration
	Findings  []core.Finding
	Summary   *ScanSummaryInfo
	Error     string
}

// ScanSummaryInfo is a simplified summary for webhook delivery.
type ScanSummaryInfo struct {
	Total         int
	Critical      int
	High          int
	Medium        int
	Low           int
	HighestSev    string
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

func hasWebhook(cfg Config) bool {
	return cfg.SlackWebhookURL != "" || cfg.DiscordWebhookURL != "" || cfg.GenericWebhookURL != ""
}

// NotifyScanStart sends a scan-started notification to all configured webhooks.
func NotifyScanStart(cfg Config, evt ScanEvent) error {
	if !hasWebhook(cfg) {
		return nil
	}

	label := evt.Package
	if evt.Target != "" && evt.Target != evt.Package {
		label = evt.Target
	}

	msg := fmt.Sprintf("🔍 *Scan Started* — %s\n• Type: %s\n• Target: %s",
		label, evt.ScanType, label)
	if len(evt.Engines) > 0 {
		msg += fmt.Sprintf("\n• Engines: %s", strings.Join(evt.Engines, ", "))
	}

	genericPayload := map[string]any{
		"event":     "scan_start",
		"package":   evt.Package,
		"scan_type": evt.ScanType,
		"target":    evt.Target,
		"engines":   evt.Engines,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	return broadcast(cfg, msg, genericPayload)
}

// NotifyScanComplete sends a scan-completed notification with full results.
func NotifyScanComplete(cfg Config, evt ScanEvent) error {
	if !hasWebhook(cfg) {
		return nil
	}

	label := evt.Package
	if evt.Target != "" && evt.Target != evt.Package {
		label = evt.Target
	}

	// Build severity breakdown
	var sevParts []string
	if evt.Summary != nil {
		if evt.Summary.Critical > 0 {
			sevParts = append(sevParts, fmt.Sprintf("%d CRITICAL", evt.Summary.Critical))
		}
		if evt.Summary.High > 0 {
			sevParts = append(sevParts, fmt.Sprintf("%d HIGH", evt.Summary.High))
		}
		if evt.Summary.Medium > 0 {
			sevParts = append(sevParts, fmt.Sprintf("%d MEDIUM", evt.Summary.Medium))
		}
		if evt.Summary.Low > 0 {
			sevParts = append(sevParts, fmt.Sprintf("%d LOW", evt.Summary.Low))
		}
	}

	totalFindings := 0
	if evt.Summary != nil {
		totalFindings = evt.Summary.Total
	}

	var sb strings.Builder

	if totalFindings == 0 {
		sb.WriteString(fmt.Sprintf("✅ *Scan Complete* — %s\n", label))
		sb.WriteString(fmt.Sprintf("• Type: %s\n", evt.ScanType))
		sb.WriteString("• Result: No vulnerabilities found")
	} else {
		sb.WriteString(fmt.Sprintf("🛡️ *Scan Complete* — %s\n", label))
		sb.WriteString(fmt.Sprintf("• Type: %s\n", evt.ScanType))
		sb.WriteString(fmt.Sprintf("• Findings: %d total", totalFindings))
		if len(sevParts) > 0 {
			sb.WriteString(fmt.Sprintf(" (%s)", strings.Join(sevParts, " · ")))
		}

		// List findings that meet severity threshold
		threshold := severityOrd(core.Severity(strings.ToUpper(cfg.OnSeverity)))
		var triggered []core.Finding
		for _, f := range evt.Findings {
			if severityOrd(f.Severity) >= threshold {
				triggered = append(triggered, f)
			}
		}

		if len(triggered) > 0 {
			sb.WriteString("\n\n*Findings:*")
			limit := 20
			for i, f := range triggered {
				if i >= limit {
					sb.WriteString(fmt.Sprintf("\n  … and %d more", len(triggered)-limit))
					break
				}
				fixInfo := ""
				if f.FixedVersion != "" {
					fixInfo = fmt.Sprintf(" → fix: %s", f.FixedVersion)
				}
				sb.WriteString(fmt.Sprintf("\n  • [%s] %s (%s)%s", f.Severity, f.Title, f.ID, fixInfo))
			}
		}
	}

	if evt.Duration > 0 {
		sb.WriteString(fmt.Sprintf("\n• Duration: %.1fs", evt.Duration.Seconds()))
	}

	msg := sb.String()

	// Generic webhook payload with structured data
	type findingEntry struct {
		ID         string `json:"id"`
		Title      string `json:"title"`
		Severity   string `json:"severity"`
		Source     string `json:"source,omitempty"`
		FixedVersion string `json:"fixed_version,omitempty"`
	}
	findingsList := make([]findingEntry, 0, len(evt.Findings))
	for _, f := range evt.Findings {
		if severityOrd(f.Severity) >= 1 { // skip INFORMATIONAL for generic webhook
			findingsList = append(findingsList, findingEntry{
				ID:         f.ID,
				Title:      f.Title,
				Severity:   string(f.Severity),
				Source:     f.Source,
				FixedVersion: f.FixedVersion,
			})
		}
	}

	genericPayload := map[string]any{
		"event":     "scan_complete",
		"package":   evt.Package,
		"scan_type": evt.ScanType,
		"target":    evt.Target,
		"summary": map[string]any{
			"total":       totalFindings,
			"critical":    evt.Summary.Critical,
			"high":        evt.Summary.High,
			"medium":      evt.Summary.Medium,
			"low":         evt.Summary.Low,
			"highest_sev": evt.Summary.HighestSev,
		},
		"findings":  findingsList,
		"duration":  evt.Duration.Seconds(),
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	return broadcast(cfg, msg, genericPayload)
}

// NotifyScanError sends a scan-error notification.
func NotifyScanError(cfg Config, evt ScanEvent) error {
	if !hasWebhook(cfg) {
		return nil
	}

	label := evt.Package
	if evt.Target != "" {
		label = evt.Target
	}

	msg := fmt.Sprintf("❌ *Scan Failed* — %s\n• Type: %s\n• Error: %s",
		label, evt.ScanType, evt.Error)

	genericPayload := map[string]any{
		"event":     "scan_error",
		"package":   evt.Package,
		"scan_type": evt.ScanType,
		"target":    evt.Target,
		"error":     evt.Error,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	return broadcast(cfg, msg, genericPayload)
}

// MonitorEvent carries information about a finding change during continuous monitoring.
type MonitorEvent struct {
	Target   string
	Action   string // "notify", "quarantine", "block"
	Added    []MonitorFinding
	Resolved []MonitorFinding
}

// MonitorFinding is a finding with its associated package identifier.
type MonitorFinding struct {
	Package string
	Finding core.Finding
}

// NotifyMonitorAlert sends a real-time monitor alert when new threats are detected.
func NotifyMonitorAlert(cfg Config, evt MonitorEvent) error {
	if !hasWebhook(cfg) {
		return nil
	}
	if len(evt.Added) == 0 && len(evt.Resolved) == 0 {
		return nil
	}

	var sb strings.Builder

	if len(evt.Added) > 0 {
		sb.WriteString(fmt.Sprintf("🚨 *Live Monitor — %d new threat(s) detected*\n", len(evt.Added)))
		sb.WriteString(fmt.Sprintf("• Target: %s\n", evt.Target))
		sb.WriteString(fmt.Sprintf("• Action: %s\n", evt.Action))
		sb.WriteString("\n*New findings:*")
		for i, mf := range evt.Added {
			if i >= 15 {
				sb.WriteString(fmt.Sprintf("\n  … and %d more", len(evt.Added)-15))
				break
			}
			sb.WriteString(fmt.Sprintf("\n  • [%s] %s — %s (%s)",
				mf.Finding.Severity, mf.Package, mf.Finding.Title, mf.Finding.ID))
		}
	}

	if len(evt.Resolved) > 0 {
		if sb.Len() > 0 {
			sb.WriteString("\n\n")
		}
		sb.WriteString(fmt.Sprintf("✅ *%d finding(s) resolved*", len(evt.Resolved)))
		for i, mf := range evt.Resolved {
			if i >= 10 {
				sb.WriteString(fmt.Sprintf("\n  … and %d more", len(evt.Resolved)-10))
				break
			}
			sb.WriteString(fmt.Sprintf("\n  • %s — %s", mf.Package, mf.Finding.Title))
		}
	}

	if evt.Action == "quarantine" {
		sb.WriteString("\n\n⚠️ *Quarantined* — packages added to policy deny list")
	} else if evt.Action == "block" {
		sb.WriteString("\n\n🛑 *Blocked* — packages added to policy deny list, scan will fail")
	}

	msg := sb.String()

	type findingDetail struct {
		Package  string `json:"package"`
		ID       string `json:"id"`
		Title    string `json:"title"`
		Severity string `json:"severity"`
	}

	addedList := make([]findingDetail, len(evt.Added))
	for i, mf := range evt.Added {
		addedList[i] = findingDetail{
			Package:  mf.Package,
			ID:       mf.Finding.ID,
			Title:    mf.Finding.Title,
			Severity: string(mf.Finding.Severity),
		}
	}
	resolvedList := make([]findingDetail, len(evt.Resolved))
	for i, mf := range evt.Resolved {
		resolvedList[i] = findingDetail{
			Package:  mf.Package,
			ID:       mf.Finding.ID,
			Title:    mf.Finding.Title,
			Severity: string(mf.Finding.Severity),
		}
	}

	genericPayload := map[string]any{
		"event":     "monitor_alert",
		"target":    evt.Target,
		"action":    evt.Action,
		"added":     addedList,
		"resolved":  resolvedList,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	return broadcast(cfg, msg, genericPayload)
}

// NotifyMonitorStarted sends a notification that continuous monitoring has begun.
func NotifyMonitorStarted(cfg Config, target string, interval time.Duration) error {
	if !hasWebhook(cfg) {
		return nil
	}
	msg := fmt.Sprintf("👁️ *Continuous Monitoring Started*\n• Target: %s\n• Interval: %s\n• Watching for dependency changes…",
		target, interval)
	payload := map[string]any{
		"event":     "monitor_started",
		"target":    target,
		"interval":  interval.Seconds(),
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}
	return broadcast(cfg, msg, payload)
}

// MaybeNotify fires webhook notifications if any finding meets the OnSeverity
// threshold. Kept for backward compat — prefer NotifyScanComplete for richer messages.
func MaybeNotify(cfg Config, pkgName string, findings []core.Finding) error {
	if cfg.OnSeverity == "" {
		return nil
	}
	if !hasWebhook(cfg) {
		return nil
	}

	threshold := severityOrd(core.Severity(strings.ToUpper(cfg.OnSeverity)))
	hasTriggered := false
	for _, f := range findings {
		if severityOrd(f.Severity) >= threshold {
			hasTriggered = true
			break
		}
	}
	if !hasTriggered {
		return nil
	}

	summary := &ScanSummaryInfo{}
	for _, f := range findings {
		switch f.Severity {
		case core.SeverityCritical:
			summary.Critical++
		case core.SeverityHigh:
			summary.High++
		case core.SeverityMedium:
			summary.Medium++
		case core.SeverityLow:
			summary.Low++
		}
	}
	summary.Total = summary.Critical + summary.High + summary.Medium + summary.Low

	return NotifyScanComplete(cfg, ScanEvent{
		Type:     "complete",
		Package:  pkgName,
		ScanType: "registry",
		Target:   pkgName,
		Findings: findings,
		Summary:  summary,
	})
}

// broadcast sends a message to all configured webhook targets.
func broadcast(cfg Config, slackMsg string, genericPayload map[string]any) error {
	var errs []string
	if cfg.SlackWebhookURL != "" {
		if err := SendSlack(cfg.SlackWebhookURL, slackMsg); err != nil {
			errs = append(errs, "slack: "+err.Error())
		}
	}
	if cfg.DiscordWebhookURL != "" {
		discordMsg := strings.ReplaceAll(slackMsg, "*", "**")
		if err := SendDiscord(cfg.DiscordWebhookURL, discordMsg); err != nil {
			errs = append(errs, "discord: "+err.Error())
		}
	}
	if cfg.GenericWebhookURL != "" {
		if err := SendGeneric(cfg.GenericWebhookURL, genericPayload); err != nil {
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
