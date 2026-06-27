package remotescan

import (
	"fmt"
	"log/slog"
	"os"
)

// NewTempDir creates a 0700 local temp directory for one remote-scan run's
// pulled-down manifest files.
func NewTempDir() (string, error) {
	dir, err := os.MkdirTemp("", "fg-remotescan-*")
	if err != nil {
		return "", fmt.Errorf("remotescan: create temp dir: %w", err)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		os.RemoveAll(dir)
		return "", fmt.Errorf("remotescan: chmod temp dir: %w", err)
	}
	return dir, nil
}

// Cleanup removes dir unless keep is true (the --keep-temp debug escape
// hatch). Intended to be called via defer immediately after NewTempDir so
// cleanup runs on every exit path, including panic unwind.
func Cleanup(dir string, keep bool, log *slog.Logger) {
	if keep {
		if log != nil {
			log.Info("remote scan: temp dir retained for debugging", "path", dir)
		}
		return
	}
	if err := os.RemoveAll(dir); err != nil && log != nil {
		log.Warn("remote scan: failed to remove temp dir", "path", dir, "err", err)
	}
}
