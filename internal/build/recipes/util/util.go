// Package util provides shared helpers for build recipes.
package util

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mah3sec/forgeguardian/internal/core"
)

const UserAgent = "forgeguardian-builder/0.1"

// Download fetches url into a temp file, returning the path and SHA256.
func Download(url, suffix string) (path, sha256sum string, err error) {
	if suffix == "" {
		suffix = "-dl"
	}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", "", fmt.Errorf("download: build request: %w", err)
	}
	req.Header.Set("User-Agent", UserAgent)

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("download: server returned %d for %s", resp.StatusCode, url)
	}

	f, err := os.CreateTemp("", "fg-dl-*"+suffix)
	if err != nil {
		return "", "", fmt.Errorf("download: create temp: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(io.MultiWriter(f, h), resp.Body); err != nil {
		_ = os.Remove(f.Name())
		return "", "", fmt.Errorf("download: write: %w", err)
	}
	return f.Name(), hex.EncodeToString(h.Sum(nil)), nil
}

// SHA256File computes the SHA256 of a local file.
func SHA256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// CopyFile copies src to dst, returning an error on failure.
func CopyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

// Sanitize replaces characters unsafe for filenames.
func Sanitize(s string) string {
	return strings.NewReplacer("/", "-", "@", "", ":", "-").Replace(s)
}

// StableOutput returns a deterministic output path in os.TempDir().
func StableOutput(ecosystem, name, version, ext string) string {
	return filepath.Join(os.TempDir(),
		fmt.Sprintf("fg-built-%s-%s-%s%s", ecosystem, Sanitize(name), version, ext))
}

// BuildArtifact constructs a BuiltArtifact from common fields.
func BuildArtifact(src core.SourceArtifact, outPath, sha256sum, log string) core.BuiltArtifact {
	return core.BuiltArtifact{
		Source:    src,
		LocalPath: outPath,
		SHA256:    sha256sum,
		BuildLog:  log,
		BuildTime: time.Now().UTC(),
	}
}
