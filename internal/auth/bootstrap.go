package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// AdminIdentityPath returns the canonical path to the persisted admin
// identity file: ~/.forgeguardian/admin.json.
func AdminIdentityPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".forgeguardian", "admin.json")
}

// AdminIdentity is the persisted bootstrapped dashboard admin — email plus
// bcrypt password hash. The plaintext password is never stored.
type AdminIdentity struct {
	Email        string `json:"email"`
	PasswordHash string `json:"password_hash"`
}

// LoadOrBootstrapAdmin checks for an existing persisted admin identity at
// AdminIdentityPath(). If one exists, it is loaded and returned, ignoring
// the supplied email/plaintextPassword — the server is already bootstrapped.
//
// If no identity file exists and both email and plaintextPassword are
// non-empty, the password is hashed and {email, hash} is persisted to
// AdminIdentityPath() (parent dir created as needed, file mode 0600), then
// returned.
//
// If no identity file exists and either email or plaintextPassword is
// empty, (nil, nil) is returned — no error, just means dashboard login is
// disabled. Callers should check for a nil identity to detect this case.
func LoadOrBootstrapAdmin(email, plaintextPassword string) (*AdminIdentity, error) {
	path := AdminIdentityPath()

	data, err := os.ReadFile(path)
	if err == nil {
		var existing AdminIdentity
		if err := json.Unmarshal(data, &existing); err != nil {
			return nil, fmt.Errorf("auth: parse admin identity: %w", err)
		}
		return &existing, nil
	}
	if !os.IsNotExist(err) {
		return nil, fmt.Errorf("auth: read admin identity: %w", err)
	}

	// No persisted identity yet.
	if email == "" || plaintextPassword == "" {
		return nil, nil
	}

	hash, err := HashPassword(plaintextPassword)
	if err != nil {
		return nil, err
	}
	identity := &AdminIdentity{Email: email, PasswordHash: hash}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("auth: mkdir: %w", err)
	}
	out, err := json.Marshal(identity)
	if err != nil {
		return nil, fmt.Errorf("auth: marshal admin identity: %w", err)
	}
	if err := os.WriteFile(path, out, 0o600); err != nil {
		return nil, fmt.Errorf("auth: write admin identity: %w", err)
	}

	return identity, nil
}
