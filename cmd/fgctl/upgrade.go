package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	upgradeRepo   = "Mah3Sec/ForgeGuardian"
	upgradeAPIURL = "https://api.github.com/repos/" + upgradeRepo + "/releases/latest"
)

type githubRelease struct {
	TagName string `json:"tag_name"`
}

func runUpgrade() error {
	fmt.Println("Checking for updates...")

	currentVer := version
	if currentVer == "" || currentVer == "dev" {
		fmt.Println("Running a dev build — cannot determine current version.")
		fmt.Println("Re-run the install script to get the latest release:")
		fmt.Println("  curl -sSfL https://raw.githubusercontent.com/Mah3Sec/ForgeGuardian/main/install.sh | bash")
		return nil
	}

	resp, err := http.Get(upgradeAPIURL)
	if err != nil {
		return fmt.Errorf("failed to check for updates: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GitHub API returned HTTP %d", resp.StatusCode)
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return fmt.Errorf("failed to parse release info: %w", err)
	}

	latestVer := strings.TrimPrefix(release.TagName, "v")
	currentClean := strings.TrimPrefix(currentVer, "v")

	if latestVer == currentClean {
		fmt.Printf("Already on the latest version (%s).\n", release.TagName)
		return nil
	}

	fmt.Printf("Current: v%s → Latest: %s\n", currentClean, release.TagName)
	fmt.Println("Downloading...")

	goos := runtime.GOOS
	goarch := runtime.GOARCH

	ext := "tar.gz"
	if goos == "windows" {
		ext = "zip"
	}

	archiveName := fmt.Sprintf("forgeguardian_%s_%s_%s.%s", latestVer, goos, goarch, ext)
	downloadURL := fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", upgradeRepo, release.TagName, archiveName)

	dlResp, err := http.Get(downloadURL)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer dlResp.Body.Close()

	if dlResp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("no binary available for %s/%s at %s — use the install script instead", goos, goarch, release.TagName)
	}
	if dlResp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: HTTP %d", dlResp.StatusCode)
	}

	tmpDir, err := os.MkdirTemp("", "fgctl-upgrade-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	archivePath := filepath.Join(tmpDir, archiveName)
	f, err := os.Create(archivePath)
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	if _, err := io.Copy(f, dlResp.Body); err != nil {
		f.Close()
		return fmt.Errorf("download write: %w", err)
	}
	f.Close()

	binaryName := "fgctl"
	if goos == "windows" {
		binaryName = "fgctl.exe"
	}

	var extractedPath string
	if ext == "zip" {
		extractedPath, err = extractFromZip(archivePath, binaryName, tmpDir)
	} else {
		extractedPath, err = extractFromTarGz(archivePath, binaryName, tmpDir)
	}
	if err != nil {
		return fmt.Errorf("extract binary: %w", err)
	}

	currentExe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate current binary: %w", err)
	}
	currentExe, err = filepath.EvalSymlinks(currentExe)
	if err != nil {
		return fmt.Errorf("resolve symlink: %w", err)
	}

	oldPath := currentExe + ".old"
	if err := os.Rename(currentExe, oldPath); err != nil {
		return fmt.Errorf("backup current binary: %w — try running with elevated permissions", err)
	}

	src, err := os.Open(extractedPath)
	if err != nil {
		os.Rename(oldPath, currentExe)
		return fmt.Errorf("open new binary: %w", err)
	}
	defer src.Close()

	dst, err := os.OpenFile(currentExe, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		os.Rename(oldPath, currentExe)
		return fmt.Errorf("write new binary: %w — try running with elevated permissions", err)
	}
	if _, err := io.Copy(dst, src); err != nil {
		dst.Close()
		os.Rename(oldPath, currentExe)
		return fmt.Errorf("copy new binary: %w", err)
	}
	dst.Close()

	os.Remove(oldPath)

	fmt.Printf("Upgraded to %s successfully.\n", release.TagName)
	fmt.Println("Run 'fgctl version' to confirm.")
	return nil
}

func extractFromTarGz(archivePath, target, destDir string) (string, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return "", err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}

		if filepath.Base(hdr.Name) == target && hdr.Typeflag == tar.TypeReg {
			outPath := filepath.Join(destDir, target)
			out, err := os.OpenFile(outPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
			if err != nil {
				return "", err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return "", err
			}
			out.Close()
			return outPath, nil
		}
	}
	return "", fmt.Errorf("%s not found in archive", target)
}

func extractFromZip(archivePath, target, destDir string) (string, error) {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return "", err
	}
	defer r.Close()

	for _, f := range r.File {
		if filepath.Base(f.Name) == target {
			rc, err := f.Open()
			if err != nil {
				return "", err
			}
			outPath := filepath.Join(destDir, target)
			out, err := os.OpenFile(outPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
			if err != nil {
				rc.Close()
				return "", err
			}
			if _, err := io.Copy(out, rc); err != nil {
				out.Close()
				rc.Close()
				return "", err
			}
			out.Close()
			rc.Close()
			return outPath, nil
		}
	}
	return "", fmt.Errorf("%s not found in archive", target)
}
