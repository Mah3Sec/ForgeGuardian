// Package license provides tier detection and feature gating for ForgeGuardian.
//
// Tiers:
//
//	Community — free forever, full CLI scan + signatures + SBOM + signing
//	Pro        — AI advisory, autonomous patch, continuous monitoring, full dashboard
//	Enterprise — Pro + RBAC, SSO, team management, SLA
//
// License key format: fg{tier}-{expiry-unix}-{hmac16}
//
//	fgp-1893456000-abc123def456  (Pro, expires 2030-01-01)
//	fge-1893456000-abc123def456  (Enterprise)
//
// Set via: export FG_LICENSE_KEY=fgp-...
// Dev mode: FG_LICENSE_KEY=dev skips validation (local dev only)
package license

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// secret is overridden at build time by .goreleaser.yaml's ldflags, sourced
// from the FG_LICENSE_SECRET repo secret in .github/workflows/release.yml
// (which refuses to run if that secret isn't set — see that workflow's
// "Verify license secret is configured" step). For a manual local build:
//
//	go build -ldflags "-X github.com/mah3sec/forgeguardian/internal/license.secret=YOUR_SECRET"
//
// Never commit the real secret. Use a random 32-byte hex string.
//
// This value is PUBLIC in every build that doesn't override it (including
// this source file). Anyone who reads it can pass FG_LICENSE_KEY=dev for
// full Enterprise tier, or call the exported GenerateKey() themselves to
// mint arbitrary Pro/Enterprise keys that will validate. Official releases
// override this at build time; local/dev builds intentionally do not, so
// that local development never requires a real license key.
var secret = "forgeguardian-community-build-do-not-use-in-prod"

// Tier represents the license level.
type Tier int

const (
	TierCommunity Tier = iota
	TierPro
	TierEnterprise
)

func (t Tier) String() string {
	switch t {
	case TierPro:
		return "Pro"
	case TierEnterprise:
		return "Enterprise"
	default:
		return "Community"
	}
}

// Current returns the active license tier based on FG_LICENSE_KEY env var.
func Current() Tier {
	key := strings.TrimSpace(os.Getenv("FG_LICENSE_KEY"))
	if key == "" {
		return TierCommunity
	}
	// Dev bypass — local development only, never valid in prod builds
	if key == "dev" && secret == "forgeguardian-community-build-do-not-use-in-prod" {
		return TierEnterprise
	}
	tier, _, ok := parseKey(key)
	if !ok {
		return TierCommunity
	}
	return tier
}

// IsPro returns true for Pro and Enterprise tiers.
func IsPro() bool { return Current() >= TierPro }

// IsEnterprise returns true only for Enterprise tier.
func IsEnterprise() bool { return Current() >= TierEnterprise }

// RequirePro returns an error with upgrade instructions if not Pro.
func RequirePro(featureName string) error {
	if IsPro() {
		return nil
	}
	return fmt.Errorf(`%s requires ForgeGuardian Pro

  Get a license:  https://forgeguardian.dev/pro
  Join waitlist:  https://forgeguardian.dev/waitlist

  Free forever:   fgctl scan .
                  fgctl intel new/validate/test
                  fgctl sbom .
                  fgctl sign / verify
                  fgctl doctor`, featureName)
}

// Info returns a human-readable license summary.
func Info() string {
	key := strings.TrimSpace(os.Getenv("FG_LICENSE_KEY"))
	if key == "" {
		return "Community (free) — upgrade at forgeguardian.dev/pro"
	}
	tier, expiry, ok := parseKey(key)
	if !ok {
		return "Invalid license key — using Community tier"
	}
	if !expiry.IsZero() && time.Now().After(expiry) {
		return fmt.Sprintf("%s license EXPIRED on %s — renew at forgeguardian.dev/pro",
			tier, expiry.Format("2006-01-02"))
	}
	if expiry.IsZero() {
		return fmt.Sprintf("%s license (no expiry)", tier)
	}
	return fmt.Sprintf("%s license — expires %s", tier, expiry.Format("2006-01-02"))
}

// GenerateKey generates a license key for the given tier and expiry.
// Used internally by the license server — not exposed in the CLI.
func GenerateKey(tier Tier, expiry time.Time) string {
	prefix := "fgp"
	if tier == TierEnterprise {
		prefix = "fge"
	}
	expiryStr := strconv.FormatInt(expiry.Unix(), 10)
	payload := prefix + "-" + expiryStr
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	sig := hex.EncodeToString(mac.Sum(nil))[:16]
	return payload + "-" + sig
}

// parseKey parses and validates a license key.
// Returns tier, expiry, and whether the key is valid.
func parseKey(key string) (Tier, time.Time, bool) {
	parts := strings.Split(key, "-")
	if len(parts) < 3 {
		return TierCommunity, time.Time{}, false
	}

	prefix := parts[0]
	expiryStr := parts[1]
	sig := parts[2]

	var tier Tier
	switch prefix {
	case "fgp":
		tier = TierPro
	case "fge":
		tier = TierEnterprise
	default:
		return TierCommunity, time.Time{}, false
	}

	payload := prefix + "-" + expiryStr
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	expected := hex.EncodeToString(mac.Sum(nil))[:16]

	if !hmac.Equal([]byte(sig), []byte(expected)) {
		return TierCommunity, time.Time{}, false
	}

	expiryUnix, err := strconv.ParseInt(expiryStr, 10, 64)
	if err != nil {
		return TierCommunity, time.Time{}, false
	}

	expiry := time.Unix(expiryUnix, 0)
	if !expiry.IsZero() && time.Now().After(expiry) {
		return TierCommunity, expiry, false // expired
	}

	return tier, expiry, true
}
