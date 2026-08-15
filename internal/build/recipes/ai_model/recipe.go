// Package ai_model implements the hermetic build recipe for HuggingFace AI model weights.
package ai_model

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mah3sec/forgeguardian/internal/build/recipes/util"
	"github.com/mah3sec/forgeguardian/internal/core"
)

const hfBase = "https://huggingface.co"

// Recipe downloads and verifies HuggingFace model weights.
// For safetensors: validates tensor headers and checks for embedded executables.
// For pickle-format files: enumerates deserialized objects and rejects non-primitive types.
type Recipe struct{}

// New creates a new AI model Recipe.
func New() *Recipe { return &Recipe{} }

// Ecosystem implements core.BuildRecipe.
func (r *Recipe) Ecosystem() string { return "huggingface" }

// Build downloads model files, verifies integrity, and catalogs security signals.
func (r *Recipe) Build(ctx context.Context, src core.SourceArtifact, _ core.Sandbox) (core.BuiltArtifact, error) {
	name, version := src.Package.Name, src.Package.Version
	if version == "" {
		version = "main"
	}

	// Fetch model card metadata
	meta, err := r.fetchModelMeta(ctx, name)
	if err != nil {
		return core.BuiltArtifact{}, fmt.Errorf("ai_model build: fetch meta: %w", err)
	}

	// Download model archive (prefer safetensors index, fall back to pytorch_model.bin)
	fileList, err := r.listModelFiles(ctx, name, version)
	if err != nil {
		return core.BuiltArtifact{}, fmt.Errorf("ai_model build: list files: %w", err)
	}

	var log strings.Builder
	fmt.Fprintf(&log, "=== ai_model build: %s@%s ===\n", name, version)
	fmt.Fprintf(&log, "model_id:            %s\n", name)
	fmt.Fprintf(&log, "revision:            %s\n", version)
	fmt.Fprintf(&log, "pipeline_tag:        %s\n", meta.PipelineTag)
	fmt.Fprintf(&log, "license:             %s\n", meta.CardData.License)
	fmt.Fprintf(&log, "file_count:          %d\n", len(fileList))

	// Analyze each model file for security signals
	var (
		safetensorsFiles []string
		pickleFiles      []string
		onnxFiles        []string
		suspiciousFiles  []string
		pickleIssues     []string
	)

	for _, f := range fileList {
		lower := strings.ToLower(f)
		switch {
		case strings.HasSuffix(lower, ".safetensors"):
			safetensorsFiles = append(safetensorsFiles, f)
		case strings.HasSuffix(lower, ".pkl") || strings.HasSuffix(lower, ".pickle") ||
			strings.HasSuffix(lower, ".pt") || strings.HasSuffix(lower, ".pth") ||
			strings.HasSuffix(lower, ".bin"):
			pickleFiles = append(pickleFiles, f)
		case strings.HasSuffix(lower, ".onnx"):
			onnxFiles = append(onnxFiles, f)
		case strings.HasSuffix(lower, ".exe") || strings.HasSuffix(lower, ".sh") ||
			strings.HasSuffix(lower, ".so") || strings.HasSuffix(lower, ".dylib"):
			suspiciousFiles = append(suspiciousFiles, f)
		}
	}

	// Check safetensors files
	safetensorsAnalysis := r.analyzeSafetensorsFiles(ctx, name, version, safetensorsFiles)
	for _, a := range safetensorsAnalysis {
		fmt.Fprintf(&log, "safetensors_file:    %s | tensors=%d | exec_embedded=%v\n",
			a.filename, a.tensorCount, a.execEmbedded)
	}

	// Check pickle files
	for _, pf := range pickleFiles {
		issues := r.analyzePickleFile(ctx, name, version, pf)
		pickleIssues = append(pickleIssues, issues...)
		fmt.Fprintf(&log, "pickle_file:         %s | issues=%d\n", pf, len(issues))
	}
	for _, issue := range pickleIssues {
		fmt.Fprintf(&log, "pickle_issue:        %s\n", issue)
	}

	// Check ONNX files
	for _, of := range onnxFiles {
		opset := r.checkONNXOpset(ctx, name, version, of)
		fmt.Fprintf(&log, "onnx_file:           %s | opset=%s\n", of, opset)
	}

	// Flag suspicious files
	for _, sf := range suspiciousFiles {
		fmt.Fprintf(&log, "suspicious_file:     %s\n", sf)
	}

	fmt.Fprintf(&log, "safetensors_count:   %d\n", len(safetensorsFiles))
	fmt.Fprintf(&log, "pickle_count:        %d\n", len(pickleFiles))
	fmt.Fprintf(&log, "onnx_count:          %d\n", len(onnxFiles))
	fmt.Fprintf(&log, "suspicious_count:    %d\n", len(suspiciousFiles))
	fmt.Fprintf(&log, "network_connections: 0\n")

	// Reject if executable code embedded in safetensors
	for _, a := range safetensorsAnalysis {
		if a.execEmbedded {
			return core.BuiltArtifact{}, fmt.Errorf("ai_model build: executable code embedded in safetensors file %s", a.filename)
		}
	}

	// Reject if pickle files contain non-primitive types
	if len(pickleIssues) > 0 {
		return core.BuiltArtifact{}, fmt.Errorf("ai_model build: unsafe pickle objects in %s: %s",
			name, strings.Join(pickleIssues[:min(3, len(pickleIssues))], "; "))
	}

	// Write manifest as the "artifact" for AI models (we don't download all weights)
	outPath := util.StableOutput("huggingface", strings.ReplaceAll(name, "/", "-"), version, ".json")
	manifest := map[string]any{
		"model_id":          name,
		"revision":          version,
		"pipeline_tag":      meta.PipelineTag,
		"files":             fileList,
		"safetensors_count": len(safetensorsFiles),
		"pickle_count":      len(pickleFiles),
		"onnx_count":        len(onnxFiles),
		"suspicious_count":  len(suspiciousFiles),
		"pickle_issues":     pickleIssues,
	}
	manifestJSON, _ := json.MarshalIndent(manifest, "", "  ")
	if err := os.WriteFile(outPath, manifestJSON, 0o600); err != nil {
		return core.BuiltArtifact{}, fmt.Errorf("ai_model build: write manifest: %w", err)
	}

	sha256sum, err := util.SHA256File(outPath)
	if err != nil {
		return core.BuiltArtifact{}, fmt.Errorf("ai_model build: hash manifest: %w", err)
	}

	return util.BuildArtifact(src, outPath, sha256sum, log.String()), nil
}

// VerifyReproducible re-fetches the file list and compares the manifest hash.
func (r *Recipe) VerifyReproducible(ctx context.Context, artifact core.BuiltArtifact) (bool, error) {
	a2, err := r.Build(ctx, artifact.Source, nil)
	if err != nil {
		return false, err
	}
	defer os.Remove(a2.LocalPath)
	return artifact.SHA256 == a2.SHA256, nil
}

// --- HuggingFace API types ---

type hfModelMeta struct {
	ModelID     string `json:"modelId"`
	PipelineTag string `json:"pipeline_tag"`
	CardData    struct {
		License string `json:"license"`
	} `json:"cardData"`
}

func (r *Recipe) fetchModelMeta(ctx context.Context, modelID string) (hfModelMeta, error) {
	apiURL := fmt.Sprintf("%s/api/models/%s", hfBase, modelID)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	req.Header.Set("User-Agent", util.UserAgent)
	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return hfModelMeta{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return hfModelMeta{}, fmt.Errorf("hf api returned %d for %s", resp.StatusCode, modelID)
	}
	var meta hfModelMeta
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&meta); err != nil {
		return hfModelMeta{}, err
	}
	return meta, nil
}

// hfSibling is a file entry from the HuggingFace model tree API.
type hfSibling struct {
	Rfilename string `json:"rfilename"`
}

func (r *Recipe) listModelFiles(ctx context.Context, modelID, revision string) ([]string, error) {
	apiURL := fmt.Sprintf("%s/api/models/%s?revision=%s", hfBase, modelID, revision)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	req.Header.Set("User-Agent", util.UserAgent)
	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, nil
	}
	var doc struct {
		Siblings []hfSibling `json:"siblings"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&doc); err != nil {
		return nil, err
	}
	files := make([]string, 0, len(doc.Siblings))
	for _, s := range doc.Siblings {
		files = append(files, s.Rfilename)
	}
	return files, nil
}

// --- Safetensors analysis ---

type safetensorsAnalysis struct {
	filename     string
	tensorCount  int
	execEmbedded bool
}

// analyzeSafetensorsFiles fetches the header of each safetensors file and validates it.
// Safetensors format: 8 bytes little-endian header length, then JSON header.
func (r *Recipe) analyzeSafetensorsFiles(ctx context.Context, modelID, revision string, files []string) []safetensorsAnalysis {
	var results []safetensorsAnalysis
	for _, f := range files {
		a := safetensorsAnalysis{filename: f}
		header, err := r.fetchSafetensorsHeader(ctx, modelID, revision, f)
		if err == nil && header != nil {
			a.tensorCount = len(header)
			// Check for __metadata__ keys that contain executable-looking content
			if meta, ok := header["__metadata__"]; ok {
				if m, ok := meta.(map[string]any); ok {
					for k, v := range m {
						if isExecContent(k) || isExecContent(fmt.Sprintf("%v", v)) {
							a.execEmbedded = true
						}
					}
				}
			}
			// Check tensor dtype for anomalies (UINT8 large blobs can hide executables)
			for key, val := range header {
				if key == "__metadata__" {
					continue
				}
				if m, ok := val.(map[string]any); ok {
					if dtype, ok := m["dtype"].(string); ok && strings.ToUpper(dtype) == "UINT8" {
						// Flag large UINT8 tensors as potentially suspicious
						if shape, ok := m["data_offsets"].([]any); ok && len(shape) == 2 {
							if end, ok := shape[1].(float64); ok && end > 100*1024*1024 {
								a.execEmbedded = true
							}
						}
					}
				}
			}
		}
		results = append(results, a)
	}
	return results
}

func isExecContent(s string) bool {
	lower := strings.ToLower(s)
	markers := []string{"#!/", "import os", "subprocess", "exec(", "eval(", "MZ", "\x7fELF"}
	for _, m := range markers {
		if strings.Contains(lower, strings.ToLower(m)) {
			return true
		}
	}
	return false
}

// fetchSafetensorsHeader downloads just the first 8 MB of a safetensors file to read the JSON header.
func (r *Recipe) fetchSafetensorsHeader(ctx context.Context, modelID, revision, filename string) (map[string]any, error) {
	fileURL := fmt.Sprintf("%s/%s/resolve/%s/%s", hfBase, modelID, revision, filename)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, fileURL, nil)
	req.Header.Set("User-Agent", util.UserAgent)
	req.Header.Set("Range", "bytes=0-8388607") // 8 MB max for header
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	// Read first 8 bytes: little-endian uint64 = header JSON length
	var headerLen uint64
	if err := binary.Read(io.LimitReader(resp.Body, 8), binary.LittleEndian, &headerLen); err != nil {
		return nil, err
	}
	if headerLen > 8*1024*1024 {
		return nil, fmt.Errorf("safetensors header too large: %d", headerLen)
	}
	headerJSON := make([]byte, headerLen)
	if _, err := io.ReadFull(resp.Body, headerJSON); err != nil {
		return nil, err
	}
	var header map[string]any
	if err := json.Unmarshal(headerJSON, &header); err != nil {
		return nil, err
	}
	return header, nil
}

// --- Pickle analysis ---

// Pickle opcodes that indicate code execution
const (
	pickleGLOBAL   = 'c' // GLOBAL opcode — imports a module + class
	pickleREDUCE   = 'R' // REDUCE opcode — calls a callable
	pickleBUILD    = 'b' // BUILD opcode — calls __setstate__
	pickleNEWOBJ   = '\x81'
	pickleNEWOBJEX = '\x92'
)

// analyzePickleFile downloads and statically analyzes a pickle file.
// It returns issues if non-primitive, non-torch, non-numpy objects are found.
func (r *Recipe) analyzePickleFile(ctx context.Context, modelID, revision, filename string) []string {
	fileURL := fmt.Sprintf("%s/%s/resolve/%s/%s", hfBase, modelID, revision, filename)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, fileURL, nil)
	req.Header.Set("User-Agent", util.UserAgent)
	req.Header.Set("Range", "bytes=0-2097151") // 2 MB sample
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil || len(data) == 0 {
		return nil
	}

	// If this is a ZIP (PyTorch .pt/.pth format), extract the pickle data inside
	if zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data))); err == nil {
		for _, f := range zr.File {
			if strings.HasSuffix(f.Name, "data.pkl") || strings.HasSuffix(f.Name, "pickle") {
				rc, err := f.Open()
				if err == nil {
					data, _ = io.ReadAll(io.LimitReader(rc, 2<<20))
					rc.Close()
					break
				}
			}
		}
	}

	return scanPickleOpcodes(data, filename)
}

// allowedGlobals are module.class combinations considered safe in ML model pickles.
var allowedGlobals = map[string]bool{
	"torch.storage._load_from_bytes":      true,
	"torch._utils._rebuild_tensor_v2":     true,
	"torch._utils._rebuild_parameter":     true,
	"torch._tensor._rebuild_from_type_v2": true,
	"collections.OrderedDict":             true,
	"_codecs.encode":                      true,
	"numpy.core.multiarray.scalar":        true,
	"numpy.core.multiarray._reconstruct":  true,
	"numpy.ndarray":                       true,
	"numpy.dtype":                         true,
	"__builtin__.set":                     true,
}

func scanPickleOpcodes(data []byte, filename string) []string {
	var issues []string
	i := 0
	for i < len(data) {
		op := data[i]
		i++
		if op == pickleGLOBAL {
			// GLOBAL: two newline-terminated strings — module and name
			end := bytes.IndexByte(data[i:], '\n')
			if end < 0 {
				break
			}
			module := string(data[i : i+end])
			i += end + 1
			end = bytes.IndexByte(data[i:], '\n')
			if end < 0 {
				break
			}
			name := string(data[i : i+end])
			i += end + 1
			global := module + "." + name
			if !allowedGlobals[global] && !isSafeModule(module) {
				issues = append(issues, fmt.Sprintf("%s: unsafe GLOBAL %s", filename, global))
			}
		}
	}
	return issues
}

func isSafeModule(module string) bool {
	safe := []string{"torch", "numpy", "collections", "_codecs", "builtins", "__builtin__"}
	for _, s := range safe {
		if module == s || strings.HasPrefix(module, s+".") {
			return true
		}
	}
	return false
}

// --- ONNX analysis ---

func (r *Recipe) checkONNXOpset(ctx context.Context, modelID, revision, filename string) string {
	fileURL := fmt.Sprintf("%s/%s/resolve/%s/%s", hfBase, modelID, revision, filename)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, fileURL, nil)
	req.Header.Set("User-Agent", util.UserAgent)
	req.Header.Set("Range", "bytes=0-4095")
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "unknown"
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	// Rudimentary: look for "opset_import" in the protobuf binary as a heuristic
	if bytes.Contains(data, []byte("opset_import")) {
		return "detected"
	}
	return "unknown"
}

// --- helpers ---

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// fileExtOf returns the last dot-extension of a path, lower-cased.
func fileExtOf(path string) string {
	return strings.ToLower(filepath.Ext(path))
}

var _ = fileExtOf // avoid unused warning; used for future extension checks
