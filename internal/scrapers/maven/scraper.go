// Package maven implements a registry scraper for the Maven Central ecosystem.
package maven

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/mah3sec/forgeguardian/internal/core"
)

const (
	searchBase  = "https://search.maven.org/solrsearch/select"
	centralBase = "https://repo1.maven.org/maven2"
	userAgent   = "forgeguardian-scraper/0.1 (supply chain security)"
)

// Scraper polls Maven Central for new artifact versions.
type Scraper struct {
	client *http.Client
	log    *slog.Logger
}

// New creates a new Maven Scraper.
func New() *Scraper {
	return &Scraper{
		client: &http.Client{Timeout: 30 * time.Second},
		log:    slog.New(slog.NewJSONHandler(os.Stdout, nil)).With("scraper", "maven"),
	}
}

// Name implements core.RegistryScraper.
func (s *Scraper) Name() string { return "maven" }

// mavenSearchResponse is the Solr response from Maven Central search.
type mavenSearchResponse struct {
	Response struct {
		Docs []mavenDoc `json:"docs"`
	} `json:"response"`
}

type mavenDoc struct {
	ID            string `json:"id"` // "group:artifact"
	GroupID       string `json:"g"`
	ArtifactID    string `json:"a"`
	LatestVersion string `json:"latestVersion"`
	VersionCount  int    `json:"versionCount"`
	Timestamp     int64  `json:"timestamp"` // milliseconds since epoch
	// For version-specific queries
	Version string `json:"v,omitempty"`
}

// Poll returns Maven artifacts updated since lastRun.
func (s *Scraper) Poll(ctx context.Context, lastRun time.Time) ([]core.PackageVersion, error) {
	sinceMs := lastRun.UnixMilli()

	params := url.Values{}
	params.Set("q", fmt.Sprintf("timestamp:[%d TO *]", sinceMs))
	params.Set("rows", "100")
	params.Set("wt", "json")
	params.Set("core", "gav") // group:artifact:version level

	reqURL := searchBase + "?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("maven: build search request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("maven: search request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("maven: search returned %d", resp.StatusCode)
	}

	var result mavenSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("maven: decode search: %w", err)
	}

	var versions []core.PackageVersion
	for _, doc := range result.Response.Docs {
		published := time.UnixMilli(doc.Timestamp)

		// Construct the JAR download URL from groupId/artifactId/version
		groupPath := strings.ReplaceAll(doc.GroupID, ".", "/")
		jarName := fmt.Sprintf("%s-%s.jar", doc.ArtifactID, doc.Version)
		sourceURL := fmt.Sprintf("%s/%s/%s/%s/%s",
			centralBase, groupPath, doc.ArtifactID, doc.Version, jarName)

		// Package name: "groupId:artifactId"
		name := fmt.Sprintf("%s:%s", doc.GroupID, doc.ArtifactID)

		// Fetch the SHA256 checksum file (.sha256 is available since Maven 3.0.4)
		checksum, _ := s.fetchChecksum(ctx, sourceURL+".sha256")

		meta := map[string]any{
			"group_id":    doc.GroupID,
			"artifact_id": doc.ArtifactID,
			"pom_url":     fmt.Sprintf("%s/%s/%s/%s/%s-%s.pom", centralBase, groupPath, doc.ArtifactID, doc.Version, doc.ArtifactID, doc.Version),
		}

		versions = append(versions, core.PackageVersion{
			Ecosystem:   "maven",
			Name:        name,
			Version:     doc.Version,
			SourceURL:   sourceURL,
			Checksum:    checksum,
			PublishedAt: published,
			Metadata:    meta,
		})
	}
	return versions, nil
}

// fetchChecksum downloads a .sha256 or .md5 checksum file from Maven Central.
func (s *Scraper) fetchChecksum(ctx context.Context, checksumURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, checksumURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", nil
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 128))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

// FetchSource downloads the JAR for a Maven artifact.
func (s *Scraper) FetchSource(ctx context.Context, pkg core.PackageVersion) (core.SourceArtifact, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pkg.SourceURL, nil)
	if err != nil {
		return core.SourceArtifact{}, fmt.Errorf("maven: build download request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := s.client.Do(req)
	if err != nil {
		return core.SourceArtifact{}, fmt.Errorf("maven: download jar: %w", err)
	}
	defer resp.Body.Close()

	f, err := os.CreateTemp("", fmt.Sprintf("fg-maven-%s-*.jar", pkg.Version))
	if err != nil {
		return core.SourceArtifact{}, fmt.Errorf("maven: create temp file: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	size, err := io.Copy(io.MultiWriter(f, h), resp.Body)
	if err != nil {
		return core.SourceArtifact{}, fmt.Errorf("maven: write jar: %w", err)
	}

	return core.SourceArtifact{
		Package:   pkg,
		LocalPath: f.Name(),
		Size:      size,
		SHA256:    hex.EncodeToString(h.Sum(nil)),
	}, nil
}

// VerifyIntegrity validates SHA256 against the Maven Central checksum.
func (s *Scraper) VerifyIntegrity(_ context.Context, src core.SourceArtifact) error {
	expected := src.Package.Checksum
	if expected == "" {
		return nil
	}
	if src.SHA256 != expected {
		return fmt.Errorf("maven: integrity mismatch for %s@%s: got %s want %s",
			src.Package.Name, src.Package.Version, src.SHA256, expected)
	}
	return nil
}
