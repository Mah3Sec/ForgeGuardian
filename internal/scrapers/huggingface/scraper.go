// Package huggingface implements a registry scraper for HuggingFace AI models.
// This is a first-of-its-kind supply chain scanner for AI model weights.
package huggingface

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/mah3sec/forgeguardian/internal/core"
)

const (
	hfAPIBase  = "https://huggingface.co/api"
	hfBaseURL  = "https://huggingface.co"
	userAgent  = "forgeguardian-scraper/0.1 (supply chain security; https://github.com/forgeguardian)"
)

// Scraper polls HuggingFace for recently modified AI models.
type Scraper struct {
	client *http.Client
	token  string // optional HF_TOKEN for private models
	log    *slog.Logger
}

// New creates a new HuggingFace Scraper.
// token is an optional HuggingFace API token (HF_TOKEN env var).
func New(token string) *Scraper {
	return &Scraper{
		client: &http.Client{Timeout: 60 * time.Second},
		token:  token,
		log:    slog.New(slog.NewJSONHandler(os.Stdout, nil)).With("scraper", "huggingface"),
	}
}

// Name implements core.RegistryScraper.
func (s *Scraper) Name() string { return "huggingface" }

// hfModel is a single model entry from the HuggingFace API.
type hfModel struct {
	ID             string      `json:"id"`           // "author/model-name"
	ModelID        string      `json:"modelId"`
	Author         string      `json:"author"`
	LastModified   time.Time   `json:"lastModified"`
	Tags           []string    `json:"tags"`
	PipelineTag    string      `json:"pipeline_tag"`
	SafeTensors    *hfSafeTensors `json:"safetensors"`
	CardData       *hfCardData `json:"cardData"`
	Downloads      int         `json:"downloads"`
	Likes          int         `json:"likes"`
	// Private field: whether the model is private (filtered out by default)
	Private        bool        `json:"private"`
}

type hfSafeTensors struct {
	Total      int64             `json:"total"`
	Parameters map[string]int64  `json:"parameters"`
}

type hfCardData struct {
	License  string   `json:"license"`
	Language []string `json:"language"`
	Tags     []string `json:"tags"`
	// Datasets can be a string or []string in the wild — use json.RawMessage.
	Datasets json.RawMessage `json:"datasets"`
}

// hfFile is a file entry in a HuggingFace model repository.
type hfFile struct {
	RFilename string `json:"rfilename"`
	Size      int64  `json:"size"`
	SHA256    string `json:"sha256"`
}

// hfModelInfo is the detailed model information including files.
type hfModelInfo struct {
	hfModel
	Siblings []hfFile `json:"siblings"`
}

// Poll returns AI model versions modified since lastRun.
func (s *Scraper) Poll(ctx context.Context, lastRun time.Time) ([]core.PackageVersion, error) {
	url := fmt.Sprintf("%s/models?sort=lastModified&direction=-1&limit=100", hfAPIBase)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("huggingface: build request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)
	if s.token != "" {
		req.Header.Set("Authorization", "Bearer "+s.token)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("huggingface: fetch models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("huggingface: models API returned %d", resp.StatusCode)
	}

	var models []hfModel
	if err := json.NewDecoder(resp.Body).Decode(&models); err != nil {
		return nil, fmt.Errorf("huggingface: decode models: %w", err)
	}

	var versions []core.PackageVersion
	for _, m := range models {
		if m.Private || m.LastModified.Before(lastRun) {
			continue
		}

		// Fetch detailed model info including file list
		info, err := s.fetchModelInfo(ctx, m.ID)
		if err != nil {
			s.log.Warn("failed to fetch model info", "model", m.ID, "err", err)
			// Use what we have from the listing
			info = &hfModelInfo{hfModel: m}
		}

		analysis := s.analyzeModel(info)

		// The "version" for a HuggingFace model is the git commit SHA (main branch).
		// We use the LastModified timestamp as the version string when no tag is present.
		version := m.LastModified.Format("2006-01-02T150405Z")

		// Source URL: the model's git repository on HuggingFace
		sourceURL := fmt.Sprintf("%s/%s", hfBaseURL, m.ID)

		meta := map[string]any{
			"author":              m.Author,
			"pipeline_tag":        m.PipelineTag,
			"tags":               m.Tags,
			"downloads":           m.Downloads,
			"likes":               m.Likes,
			"has_safetensors":     m.SafeTensors != nil,
			"file_formats":        analysis.fileFormats,
			"has_pickle":          analysis.hasPickle,
			"has_custom_code":     analysis.hasCustomCode,
			"has_onnx":            analysis.hasONNX,
			"has_gguf":            analysis.hasGGUF,
			"parameter_count":     analysis.parameterCount,
			"network_access_risk": analysis.networkAccessRisk,
			"shadow_model_risk":   isShadowModel(m.ID),
			"license":             getLicense(m),
			"files":               analysis.fileSummary,
		}

		versions = append(versions, core.PackageVersion{
			Ecosystem:   "huggingface",
			Name:        m.ID,
			Version:     version,
			SourceURL:   sourceURL,
			Checksum:    "", // computed per-file during FetchSource
			PublishedAt: m.LastModified,
			Metadata:    meta,
		})
	}
	return versions, nil
}

// modelAnalysis holds the security-relevant analysis of a model's files.
type modelAnalysis struct {
	fileFormats       []string
	hasPickle         bool
	hasCustomCode     bool
	hasONNX           bool
	hasGGUF           bool
	parameterCount    int64
	networkAccessRisk bool
	fileSummary       []string
}

// analyzeModel inspects model files for supply chain risks.
func (s *Scraper) analyzeModel(info *hfModelInfo) modelAnalysis {
	var a modelAnalysis
	formatSet := map[string]bool{}

	for _, f := range info.Siblings {
		lower := strings.ToLower(f.RFilename)
		a.fileSummary = append(a.fileSummary, f.RFilename)

		switch {
		case strings.HasSuffix(lower, ".safetensors"):
			formatSet["safetensors"] = true
		case strings.HasSuffix(lower, ".bin") || strings.HasSuffix(lower, ".pt") ||
			strings.HasSuffix(lower, ".pth"):
			formatSet["pytorch_pickle"] = true
			a.hasPickle = true
		case strings.HasSuffix(lower, ".onnx"):
			a.hasONNX = true
			formatSet["onnx"] = true
		case strings.HasSuffix(lower, ".gguf") || strings.HasSuffix(lower, ".ggml"):
			a.hasGGUF = true
			formatSet["gguf"] = true
		case lower == "config.json":
			// Fetched separately; custom_code flag lives here
		case strings.HasSuffix(lower, ".py"):
			a.hasCustomCode = true
		}

		// Heuristic: any .py file that is NOT adapter_config or tokenizer is suspicious
		if strings.HasSuffix(lower, ".py") &&
			!strings.Contains(lower, "tokenizer") &&
			!strings.Contains(lower, "adapter_config") {
			a.networkAccessRisk = true
		}
	}

	if info.SafeTensors != nil {
		for _, count := range info.SafeTensors.Parameters {
			a.parameterCount += count
		}
	}

	for f := range formatSet {
		a.fileFormats = append(a.fileFormats, f)
	}
	return a
}

// isShadowModel heuristically detects models that shadow popular ones.
func isShadowModel(modelID string) bool {
	popular := []string{
		"llama", "mistral", "falcon", "gpt", "bert", "roberta",
		"stable-diffusion", "whisper", "clip", "bloom",
	}
	lower := strings.ToLower(modelID)
	for _, name := range popular {
		if strings.Contains(lower, name) {
			// Only flag if the author is not the canonical publisher
			canonical := map[string]string{
				"llama":             "meta-llama",
				"mistral":           "mistralai",
				"falcon":            "tiiuae",
				"stable-diffusion":  "stabilityai",
				"whisper":           "openai",
				"clip":              "openai",
			}
			if canonAuthor, ok := canonical[name]; ok {
				author := strings.Split(modelID, "/")[0]
				if !strings.EqualFold(author, canonAuthor) {
					return true
				}
			}
		}
	}
	return false
}

// getLicense extracts the license from model card data.
func getLicense(m hfModel) string {
	if m.CardData != nil && m.CardData.License != "" {
		return m.CardData.License
	}
	for _, tag := range m.Tags {
		if strings.HasPrefix(tag, "license:") {
			return strings.TrimPrefix(tag, "license:")
		}
	}
	return "unknown"
}

// fetchModelInfo fetches the detailed model information from the HuggingFace API.
func (s *Scraper) fetchModelInfo(ctx context.Context, modelID string) (*hfModelInfo, error) {
	url := fmt.Sprintf("%s/models/%s", hfAPIBase, modelID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	if s.token != "" {
		req.Header.Set("Authorization", "Bearer "+s.token)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("model info returned %d", resp.StatusCode)
	}

	var info hfModelInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, err
	}
	return &info, nil
}

// FetchSource downloads the model's config and representative weight file for analysis.
// A full model may be hundreds of GB; we download only config.json and tokenizer files.
func (s *Scraper) FetchSource(ctx context.Context, pkg core.PackageVersion) (core.SourceArtifact, error) {
	// Download config.json to inspect custom_code, trust_remote_code flags.
	configURL := fmt.Sprintf("%s/%s/resolve/main/config.json", hfBaseURL, pkg.Name)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, configURL, nil)
	if err != nil {
		return core.SourceArtifact{}, fmt.Errorf("huggingface: build config request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)
	if s.token != "" {
		req.Header.Set("Authorization", "Bearer "+s.token)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return core.SourceArtifact{}, fmt.Errorf("huggingface: fetch config: %w", err)
	}
	defer resp.Body.Close()

	f, err := os.CreateTemp("", fmt.Sprintf("fg-hf-%s-config-*.json",
		strings.ReplaceAll(pkg.Name, "/", "-")))
	if err != nil {
		return core.SourceArtifact{}, fmt.Errorf("huggingface: create temp file: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	size, err := io.Copy(io.MultiWriter(f, h), resp.Body)
	if err != nil {
		return core.SourceArtifact{}, fmt.Errorf("huggingface: write config: %w", err)
	}

	return core.SourceArtifact{
		Package:   pkg,
		LocalPath: f.Name(),
		Size:      size,
		SHA256:    hex.EncodeToString(h.Sum(nil)),
	}, nil
}

// VerifyIntegrity for HuggingFace models checks that config.json was fetched cleanly.
// Per-file hash verification against the model's git LFS pointers happens in Phase 3.
func (s *Scraper) VerifyIntegrity(_ context.Context, src core.SourceArtifact) error {
	if src.Size == 0 {
		return fmt.Errorf("huggingface: empty config for %s", src.Package.Name)
	}
	return nil
}
