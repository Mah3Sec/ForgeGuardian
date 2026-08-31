// Package license provides tier detection and feature gating.
package license

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var publicKeyHex = "0000000000000000000000000000000000000000000000000000000000000000"

var activationURL = "https://api.forgeguardian.dev/v1/license"

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

type LicensePayload struct {
	ID       string `json:"id"`
	Tier     string `json:"t"`
	Email    string `json:"e"`
	Expiry   int64  `json:"x"`
	IssuedAt int64  `json:"i"`
	MaxSeats int    `json:"s,omitempty"`
}

var (
	cachedTier     Tier
	cachedPayload  *LicensePayload
	cacheOnce      sync.Once
	revokedKeys    map[string]bool
	revokeMu       sync.RWMutex
)

func init() {
	loadRevocationList()
}

func Current() Tier {
	cacheOnce.Do(func() {
		cachedTier, cachedPayload = resolveCurrentTier()
	})
	return cachedTier
}

func CurrentPayload() *LicensePayload {
	Current() // ensure cache is populated
	return cachedPayload
}

func resolveCurrentTier() (Tier, *LicensePayload) {
	key := strings.TrimSpace(os.Getenv("FG_LICENSE_KEY"))
	if key == "" {
		return TierCommunity, nil
	}

	payload, err := Validate(key)
	if err != nil {
		return TierCommunity, nil
	}

	if payload.Tier == "enterprise" {
		if !checkActivation(payload) {
			return TierCommunity, nil
		}
		return TierEnterprise, payload
	}

	return TierPro, payload
}

func Validate(key string) (*LicensePayload, error) {
	parts := strings.SplitN(key, ".", 3)
	if len(parts) != 3 || parts[0] != "fg2" {
		return nil, fmt.Errorf("invalid key format")
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("invalid payload encoding")
	}

	sigBytes, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, fmt.Errorf("invalid signature encoding")
	}

	pubKeyBytes, err := hex.DecodeString(publicKeyHex)
	if err != nil || len(pubKeyBytes) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("invalid public key configuration")
	}

	if !ed25519.Verify(ed25519.PublicKey(pubKeyBytes), payloadBytes, sigBytes) {
		return nil, fmt.Errorf("invalid signature")
	}

	var payload LicensePayload
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return nil, fmt.Errorf("invalid payload")
	}

	if payload.Expiry > 0 && time.Now().Unix() > payload.Expiry {
		return nil, fmt.Errorf("license expired on %s", time.Unix(payload.Expiry, 0).Format("2006-01-02"))
	}

	if isRevoked(payload.ID) {
		return nil, fmt.Errorf("license has been revoked")
	}

	if payload.Tier != "pro" && payload.Tier != "enterprise" {
		return nil, fmt.Errorf("unknown tier %q", payload.Tier)
	}

	return &payload, nil
}

func IsPro() bool { return Current() >= TierPro }

func IsEnterprise() bool { return Current() >= TierEnterprise }

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

func Info() string {
	key := strings.TrimSpace(os.Getenv("FG_LICENSE_KEY"))
	if key == "" {
		return "Community (free) — upgrade at forgeguardian.dev/pro"
	}
	payload, err := Validate(key)
	if err != nil {
		return fmt.Sprintf("Invalid license: %s — using Community tier", err)
	}

	tier := TierPro
	if payload.Tier == "enterprise" {
		tier = TierEnterprise
	}

	expiry := time.Unix(payload.Expiry, 0)
	parts := []string{fmt.Sprintf("%s license", tier)}
	if payload.Email != "" {
		parts = append(parts, fmt.Sprintf("(%s)", payload.Email))
	}
	if payload.ID != "" {
		parts = append(parts, fmt.Sprintf("[%s]", payload.ID))
	}
	parts = append(parts, fmt.Sprintf("— expires %s", expiry.Format("2006-01-02")))
	return strings.Join(parts, " ")
}



type activationCache struct {
	LicenseID  string `json:"license_id"`
	ActivatedAt int64 `json:"activated_at"`
	ValidUntil  int64 `json:"valid_until"`
	MachineID   string `json:"machine_id"`
}

func activationCachePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".forgeguardian", "activation.json")
}

func machineFingerprint() string {
	hostname, _ := os.Hostname()
	home, _ := os.UserHomeDir()
	raw := hostname + "|" + home + "|" + os.Getenv("USER") + os.Getenv("USERNAME")
	h := make([]byte, 0, 16)
	for i, b := range []byte(raw) {
		if i < 16 {
			h = append(h, b)
		} else {
			h[i%16] ^= b
		}
	}
	return hex.EncodeToString(h)
}

func checkActivation(payload *LicensePayload) bool {
	cache := loadActivationCache()
	if cache != nil && cache.LicenseID == payload.ID && cache.MachineID == machineFingerprint() {
		if time.Now().Unix() < cache.ValidUntil {
			return true
		}
	}

	if activateOnline(payload) {
		return true
	}

	if cache != nil && cache.LicenseID == payload.ID {
		gracePeriod := cache.ValidUntil + (30 * 24 * 60 * 60)
		if time.Now().Unix() < gracePeriod {
			return true
		}
	}

	return false
}

func activateOnline(payload *LicensePayload) bool {
	url := activationURL + "/activate"
	body := fmt.Sprintf(`{"license_id":%q,"machine_id":%q}`, payload.ID, machineFingerprint())

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(url, "application/json", strings.NewReader(body))
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return false
	}

	var result struct {
		Activated  bool  `json:"activated"`
		ValidUntil int64 `json:"valid_until"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil || !result.Activated {
		return false
	}

	cache := &activationCache{
		LicenseID:   payload.ID,
		ActivatedAt: time.Now().Unix(),
		ValidUntil:  result.ValidUntil,
		MachineID:   machineFingerprint(),
	}
	saveActivationCache(cache)
	return true
}

func loadActivationCache() *activationCache {
	data, err := os.ReadFile(activationCachePath())
	if err != nil {
		return nil
	}
	var cache activationCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil
	}
	return &cache
}

func saveActivationCache(cache *activationCache) {
	dir := filepath.Dir(activationCachePath())
	os.MkdirAll(dir, 0700)
	data, _ := json.Marshal(cache)
	os.WriteFile(activationCachePath(), data, 0600)
}


func revocationListPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".forgeguardian", "revoked-licenses.json")
}

func loadRevocationList() {
	revokeMu.Lock()
	defer revokeMu.Unlock()
	revokedKeys = make(map[string]bool)

	data, err := os.ReadFile(revocationListPath())
	if err != nil {
		return
	}
	var ids []string
	if err := json.Unmarshal(data, &ids); err != nil {
		return
	}
	for _, id := range ids {
		revokedKeys[id] = true
	}
}

func isRevoked(licenseID string) bool {
	revokeMu.RLock()
	defer revokeMu.RUnlock()
	return revokedKeys[licenseID]
}

func UpdateRevocationList() error {
	url := activationURL + "/revoked"
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("fetch revocation list: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("revocation list: HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	var ids []string
	if err := json.Unmarshal(body, &ids); err != nil {
		return fmt.Errorf("invalid revocation list format")
	}

	dir := filepath.Dir(revocationListPath())
	os.MkdirAll(dir, 0700)
	if err := os.WriteFile(revocationListPath(), body, 0600); err != nil {
		return err
	}

	loadRevocationList()
	return nil
}

func Revoke(licenseID string) {
	revokeMu.Lock()
	defer revokeMu.Unlock()
	if revokedKeys == nil {
		revokedKeys = make(map[string]bool)
	}
	revokedKeys[licenseID] = true

	var ids []string
	for id := range revokedKeys {
		ids = append(ids, id)
	}
	data, _ := json.Marshal(ids)
	dir := filepath.Dir(revocationListPath())
	os.MkdirAll(dir, 0700)
	os.WriteFile(revocationListPath(), data, 0600)
}
