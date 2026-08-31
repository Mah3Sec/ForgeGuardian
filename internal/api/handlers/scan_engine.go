package handlers

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mah3sec/forgeguardian/internal/core"
)

const maxDownloadSize = 512 * 1024 * 1024 // 512 MB

var dlClient = &http.Client{Timeout: 2 * time.Minute}

// downloadArtifact fetches the package tarball from its registry, extracts it
// to a temp directory, and returns a BuiltArtifact with a real LocalPath.
// Caller must call cleanup() when done.
func downloadArtifact(ctx context.Context, ecosystem, name, version string) (core.BuiltArtifact, func(), error) {
	url, err := resolveDownloadURL(ctx, ecosystem, name, version)
	if err != nil {
		return core.BuiltArtifact{}, nil, fmt.Errorf("resolve url: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return core.BuiltArtifact{}, nil, err
	}
	req.Header.Set("User-Agent", "forgeguardian-api/1.0")

	resp, err := dlClient.Do(req)
	if err != nil {
		return core.BuiltArtifact{}, nil, fmt.Errorf("download %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return core.BuiltArtifact{}, nil, fmt.Errorf("registry HTTP %d for %s/%s@%s", resp.StatusCode, ecosystem, name, version)
	}

	// Write to temp file
	tmp, err := os.CreateTemp("", fmt.Sprintf("fg-%s-%s-*", safeFilename(name), version))
	if err != nil {
		return core.BuiltArtifact{}, nil, err
	}
	h := sha256.New()
	size, err := io.Copy(io.MultiWriter(tmp, h), io.LimitReader(resp.Body, maxDownloadSize))
	tmp.Close()
	if err != nil {
		os.Remove(tmp.Name())
		return core.BuiltArtifact{}, nil, fmt.Errorf("write artifact: %w", err)
	}
	sha := hex.EncodeToString(h.Sum(nil))

	// Extract to directory so all scanners can walk the contents
	dir, err := os.MkdirTemp("", fmt.Sprintf("fg-ext-%s-*", safeFilename(name)))
	if err != nil {
		os.Remove(tmp.Name())
		return core.BuiltArtifact{}, nil, err
	}

	_ = extractArchive(tmp.Name(), dir) // best-effort; scanners handle empty dir
	os.Remove(tmp.Name())

	cleanup := func() { os.RemoveAll(dir) }

	art := core.BuiltArtifact{
		Source: core.SourceArtifact{
			Package: core.PackageVersion{
				Ecosystem: ecosystem,
				Name:      name,
				Version:   version,
			},
			LocalPath: dir,
			Size:      size,
			SHA256:    sha,
		},
		LocalPath: dir,
		SHA256:    sha,
		BuildTime: time.Now().UTC(),
	}
	return art, cleanup, nil
}

// resolveDownloadURL returns the direct tarball/archive URL for a package version.
func resolveDownloadURL(ctx context.Context, ecosystem, name, version string) (string, error) {
	switch ecosystem {
	case "npm":
		return npmTarballURL(ctx, name, version)
	case "pypi":
		return pypiTarballURL(ctx, name, version)
	case "go":
		return fmt.Sprintf("https://proxy.golang.org/%s/@v/%s.zip", name, version), nil
	case "rubygems":
		return fmt.Sprintf("https://rubygems.org/downloads/%s-%s.gem", name, version), nil
	case "crates":
		return fmt.Sprintf("https://static.crates.io/crates/%s/%s/download", name, version), nil
	case "maven":
		// name format: "group.id:artifact-id"
		parts := strings.SplitN(name, ":", 2)
		if len(parts) != 2 {
			return "", fmt.Errorf("maven name must be groupId:artifactId, got %q", name)
		}
		group := strings.ReplaceAll(parts[0], ".", "/")
		artifact := parts[1]
		return fmt.Sprintf("https://repo1.maven.org/maven2/%s/%s/%s/%s-%s.jar",
			group, artifact, version, artifact, version), nil
	case "huggingface":
		// Fetch config.json as a minimal artifact for behavioral analysis
		return fmt.Sprintf("https://huggingface.co/%s/resolve/main/config.json", name), nil
	default:
		return "", fmt.Errorf("ecosystem %q does not support registry download", ecosystem)
	}
}

func npmTarballURL(ctx context.Context, name, version string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf("https://registry.npmjs.org/%s/%s", name, version), nil)
	if err != nil {
		return "", err
	}
	resp, err := dlClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var meta struct {
		Dist struct {
			Tarball string `json:"tarball"`
		} `json:"dist"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&meta); err != nil {
		return "", fmt.Errorf("npm metadata decode: %w", err)
	}
	if meta.Dist.Tarball == "" {
		return "", fmt.Errorf("npm: no tarball for %s@%s", name, version)
	}
	return meta.Dist.Tarball, nil
}

func pypiTarballURL(ctx context.Context, name, version string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf("https://pypi.org/pypi/%s/%s/json", name, version), nil)
	if err != nil {
		return "", err
	}
	resp, err := dlClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var meta struct {
		URLs []struct {
			URL      string `json:"url"`
			Filename string `json:"filename"`
		} `json:"urls"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&meta); err != nil {
		return "", fmt.Errorf("pypi metadata decode: %w", err)
	}
	// prefer source dist
	for _, u := range meta.URLs {
		if strings.HasSuffix(u.Filename, ".tar.gz") {
			return u.URL, nil
		}
	}
	for _, u := range meta.URLs {
		if strings.HasSuffix(u.Filename, ".whl") {
			return u.URL, nil
		}
	}
	if len(meta.URLs) > 0 {
		return meta.URLs[0].URL, nil
	}
	return "", fmt.Errorf("pypi: no download URL for %s@%s", name, version)
}

// extractArchive unpacks .tgz / .tar.gz / .zip into dest directory.
func extractArchive(src, dest string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()

	// Try gzip+tar
	gr, err := gzip.NewReader(f)
	if err == nil {
		defer gr.Close()
		return untar(tar.NewReader(gr), dest)
	}

	// Try zip
	f.Seek(0, io.SeekStart)
	fi, _ := f.Stat()
	if zr, err := zip.NewReader(f, fi.Size()); err == nil {
		return unzip(zr, dest)
	}

	// Raw file — copy into dest as-is
	f.Seek(0, io.SeekStart)
	out, err := os.Create(filepath.Join(dest, filepath.Base(src)))
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, f)
	return err
}

const maxExtractFiles = 10000
const maxExtractTotalBytes int64 = 2 << 30 // 2 GB

func untar(tr *tar.Reader, dest string) error {
	fileCount := 0
	var totalBytes int64
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		clean := filepath.Clean(hdr.Name)
		target := filepath.Join(dest, clean)
		if !strings.HasPrefix(filepath.Clean(target), filepath.Clean(dest)+string(os.PathSeparator)) {
			continue
		}
		if hdr.Typeflag == tar.TypeSymlink || hdr.Typeflag == tar.TypeLink {
			continue
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			os.MkdirAll(target, 0o755)
		case tar.TypeReg:
			fileCount++
			if fileCount > maxExtractFiles {
				return fmt.Errorf("archive exceeds maximum file count (%d)", maxExtractFiles)
			}
			os.MkdirAll(filepath.Dir(target), 0o755)
			out, err := os.Create(target)
			if err != nil {
				continue
			}
			n, _ := io.Copy(out, io.LimitReader(tr, 50<<20))
			out.Close()
			totalBytes += n
			if totalBytes > maxExtractTotalBytes {
				return fmt.Errorf("archive exceeds maximum total extracted size")
			}
		}
	}
	return nil
}

func unzip(zr *zip.Reader, dest string) error {
	if len(zr.File) > maxExtractFiles {
		return fmt.Errorf("archive exceeds maximum file count (%d)", maxExtractFiles)
	}
	var totalBytes int64
	for _, f := range zr.File {
		clean := filepath.Clean(f.Name)
		target := filepath.Join(dest, clean)
		if !strings.HasPrefix(filepath.Clean(target), filepath.Clean(dest)+string(os.PathSeparator)) {
			continue
		}
		if f.FileInfo().IsDir() {
			os.MkdirAll(target, 0o755)
			continue
		}
		os.MkdirAll(filepath.Dir(target), 0o755)
		rc, err := f.Open()
		if err != nil {
			continue
		}
		out, err := os.Create(target)
		if err != nil {
			rc.Close()
			continue
		}
		n, _ := io.Copy(out, io.LimitReader(rc, 50<<20))
		out.Close()
		rc.Close()
		totalBytes += n
		if totalBytes > maxExtractTotalBytes {
			return fmt.Errorf("archive exceeds maximum total extracted size")
		}
	}
	return nil
}

func safeFilename(s string) string {
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteRune('-')
		}
	}
	res := b.String()
	if len(res) > 40 {
		res = res[:40]
	}
	return res
}
