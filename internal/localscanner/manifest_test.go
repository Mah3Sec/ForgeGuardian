package localscanner

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return path
}

// ── package.json ─────────────────────────────────────────────────────────────

func TestParsePackageJSON_BasicDeps(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "package.json", `{
		"dependencies": {
			"lodash": "4.17.21",
			"express": "^4.18.2"
		},
		"devDependencies": {
			"jest": "29.0.0"
		}
	}`)
	entries, err := ParseManifest(path)
	if err != nil {
		t.Fatalf("ParseManifest: %v", err)
	}
	prod := 0
	dev := 0
	for _, e := range entries {
		if e.IsDevDependency {
			dev++
		} else {
			prod++
		}
	}
	if prod != 2 {
		t.Errorf("want 2 prod deps, got %d", prod)
	}
	if dev != 1 {
		t.Errorf("want 1 dev dep, got %d", dev)
	}
}

func TestParsePackageJSON_PinnedVersion_Extracted(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "package.json", `{
		"dependencies": { "lodash": "4.17.21" }
	}`)
	entries, _ := ParseManifest(path)
	if len(entries) == 0 {
		t.Fatal("expected entries")
	}
	found := false
	for _, e := range entries {
		if e.Name == "lodash" && e.Version == "4.17.21" {
			found = true
		}
	}
	if !found {
		t.Error("lodash@4.17.21 not found in entries")
	}
}

func TestParsePackageJSON_CaretVersion_Stripped(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "package.json", `{
		"dependencies": { "express": "^4.18.2" }
	}`)
	entries, _ := ParseManifest(path)
	for _, e := range entries {
		if e.Name == "express" && e.Version != "4.18.2" {
			t.Errorf("caret not stripped: got version %q", e.Version)
		}
	}
}

func TestParsePackageJSON_MalformedJSON_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "package.json", `{not valid json`)
	_, err := ParseManifest(path)
	if err == nil {
		t.Error("want error for malformed JSON")
	}
}

// ── requirements.txt ─────────────────────────────────────────────────────────

func TestParseRequirements_Basic(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "requirements.txt", `
requests==2.28.0
flask>=2.0.0
# this is a comment
pytest==7.1.0
`)
	entries, err := ParseManifest(path)
	if err != nil {
		t.Fatalf("ParseManifest: %v", err)
	}
	names := map[string]string{}
	for _, e := range entries {
		names[e.Name] = e.Version
	}
	if names["requests"] != "2.28.0" {
		t.Errorf("requests: want 2.28.0, got %q", names["requests"])
	}
	if _, ok := names["pytest"]; !ok {
		t.Error("pytest missing from entries")
	}
}

func TestParseRequirements_Comments_Skipped(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "requirements.txt", `
# comment line
requests==2.28.0
`)
	entries, _ := ParseManifest(path)
	for _, e := range entries {
		if e.Name == "" || e.Name[0] == '#' {
			t.Errorf("comment line parsed as entry: %v", e)
		}
	}
}

// ── go.mod ───────────────────────────────────────────────────────────────────

func TestParseGoMod_Require(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "go.mod", `module example.com/app

go 1.21

require (
	github.com/gin-gonic/gin v1.9.1
	github.com/stretchr/testify v1.8.4
)
`)
	entries, err := ParseManifest(path)
	if err != nil {
		t.Fatalf("ParseManifest: %v", err)
	}
	if len(entries) < 2 {
		t.Fatalf("want >=2 entries, got %d", len(entries))
	}
	names := map[string]string{}
	for _, e := range entries {
		names[e.Name] = e.Version
	}
	if _, ok := names["github.com/gin-gonic/gin"]; !ok {
		t.Error("gin not found in entries")
	}
	// Parser may strip the "v" prefix — accept either form
	ver := names["github.com/gin-gonic/gin"]
	if ver != "v1.9.1" && ver != "1.9.1" {
		t.Errorf("gin: want v1.9.1 or 1.9.1, got %q", ver)
	}
}

// ── Cargo.toml ───────────────────────────────────────────────────────────────

func TestParseCargoToml_Dependencies(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "Cargo.toml", `[package]
name = "myapp"
version = "0.1.0"

[dependencies]
serde = "1.0"
tokio = "1.28"

[dev-dependencies]
criterion = "0.5"
`)
	entries, err := ParseManifest(path)
	if err != nil {
		t.Fatalf("ParseManifest: %v", err)
	}
	if len(entries) < 2 {
		t.Fatalf("want >=2 entries total, got %d", len(entries))
	}
	names := map[string]bool{}
	for _, e := range entries {
		names[e.Name] = true
	}
	if !names["serde"] {
		t.Error("serde not found in entries")
	}
	if !names["tokio"] {
		t.Error("tokio not found in entries")
	}
}

// ── Unknown file ─────────────────────────────────────────────────────────────

func TestParseManifest_UnknownFile_ReturnsNil(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "random.txt", "foo=bar")
	entries, err := ParseManifest(path)
	if err != nil {
		t.Errorf("want nil error for unknown file, got %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("want 0 entries for unknown file, got %d", len(entries))
	}
}

// ── normalizeVersion ────────────────────────────────────────────────────────

func TestNormalizeVersion(t *testing.T) {
	cases := []struct{ input, want string }{
		{"4.17.21", "4.17.21"},
		{"^4.17.21", "4.17.21"},
		{"~4.17.21", "4.17.21"},
		{"==2.28.0", "2.28.0"},
		{"~=2.28.0", "2.28.0"},
		{">=1.0.0", "1.0.0"},
		{"*", ""},
		{"", ""},
	}
	for _, tc := range cases {
		got := normalizeVersion(tc.input)
		if got != tc.want {
			t.Errorf("normalizeVersion(%q): want %q, got %q", tc.input, tc.want, got)
		}
	}
}
