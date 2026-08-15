package intelligence

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const defaultStoreName = ".forgeguardian/signatures.json"

// DefaultStorePath returns ~/.forgeguardian/signatures.json.
func DefaultStorePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("store: home dir: %w", err)
	}
	return filepath.Join(home, defaultStoreName), nil
}

// LoadStore reads the JSON signature store from path.
// If the file does not exist, an empty store is returned (not an error).
func LoadStore(path string) (*SignatureStore, error) {
	path = expandTilde(path)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &SignatureStore{Version: 1}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("store: read %s: %w", path, err)
	}
	var store SignatureStore
	if err := json.Unmarshal(data, &store); err != nil {
		return nil, fmt.Errorf("store: unmarshal: %w", err)
	}
	return &store, nil
}

// LoadStoreWithLocalFallback loads the JSON signature store at path, then —
// if a local `signatures/` directory is discoverable by walking up from the
// current working directory — merges in those YAML signatures too (deduped
// against whatever's already in the JSON store).
//
// This exists so every command that reports on or uses signatures agrees
// with each other: without it, `fgctl scan .` would use 24 real signatures
// (via the fallback) while `fgctl stats` reported 0 (reading signatures.json
// directly), which is exactly what happened before this function existed —
// confusing and just plain wrong for anyone running from a fresh clone with
// no signatures.json yet. Use this instead of a bare LoadStore call anywhere
// the result is shown to a user or used to drive scanning.
func LoadStoreWithLocalFallback(storePath string) (*SignatureStore, error) {
	store, err := LoadStore(storePath)
	if err != nil {
		return nil, err
	}
	wd, err := os.Getwd()
	if err != nil {
		return store, nil
	}
	sigDir := FindLocalSignaturesDir(wd, 6)
	if sigDir == "" {
		return store, nil
	}
	yamlSigs, err := LoadYAMLSignatures(sigDir)
	if err != nil {
		return store, nil
	}
	MergeSignatures(store, yamlSigs)
	return store, nil
}

// SaveStore atomically writes the store to path (temp file + rename).
func SaveStore(path string, store *SignatureStore) error {
	path = expandTilde(path)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("store: mkdir: %w", err)
	}
	store.UpdatedAt = time.Now().UTC()

	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return fmt.Errorf("store: marshal: %w", err)
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("store: write tmp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("store: rename: %w", err)
	}
	return nil
}

// MergeSignatures appends new signatures to the store, skipping duplicates.
// Deduplication key: type + ecosystem + (package | pattern | rule | target).
// Returns the count of signatures actually added.
func MergeSignatures(store *SignatureStore, incoming []DetectionSignature) int {
	seen := make(map[string]bool, len(store.Signatures))
	for _, s := range store.Signatures {
		seen[dedupeKey(s)] = true
	}

	added := 0
	for _, s := range incoming {
		k := dedupeKey(s)
		if seen[k] {
			continue
		}
		seen[k] = true
		store.Signatures = append(store.Signatures, s)
		added++
	}
	return added
}

func dedupeKey(s DetectionSignature) string {
	return string(s.Type) + "\x00" + s.Ecosystem + "\x00" + s.Package + s.Pattern + s.Rule + s.Target
}

// LoadYAMLSignatures walks dir for *.yaml/*.yml community signature files and
// converts each into its runtime DetectionSignature form. Files that fail to
// parse or fail ValidateSignature are skipped (not fatal) so one malformed
// signature in a local checkout can't break every scan — callers that want
// strict validation should use `fgctl intel validate` instead, which is fatal
// on purpose.
func LoadYAMLSignatures(dir string) ([]DetectionSignature, error) {
	var out []DetectionSignature
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".yaml" && ext != ".yml" {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		var sig SignatureYAML
		if yamlErr := yaml.Unmarshal(data, &sig); yamlErr != nil {
			return nil
		}
		if problems := ValidateSignature(sig); len(problems) > 0 {
			return nil
		}
		out = append(out, ToDetectionSignature(sig))
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("store: walk %s: %w", dir, err)
	}
	return out, nil
}

// FindLocalSignaturesDir walks upward from startDir looking for a
// "signatures" directory that looks like ForgeGuardian's own community
// signature tree (contains at least one of the known type subdirectories).
// Returns "" if none is found within maxUp levels — this is a best-effort
// convenience for running straight out of a git clone, not a general-purpose
// directory search.
func FindLocalSignaturesDir(startDir string, maxUp int) string {
	dir := startDir
	for i := 0; i <= maxUp; i++ {
		candidate := filepath.Join(dir, "signatures")
		if looksLikeSignaturesDir(candidate) {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

func looksLikeSignaturesDir(path string) bool {
	for _, sub := range []string{"blocklisted", "typosquatting", "behavioral", "malware", "mcp", "ai-model"} {
		if info, err := os.Stat(filepath.Join(path, sub)); err == nil && info.IsDir() {
			return true
		}
	}
	return false
}

func expandTilde(p string) string {
	if !strings.HasPrefix(p, "~/") {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return p
	}
	return filepath.Join(home, p[2:])
}
