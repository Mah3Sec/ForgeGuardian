package intelligence

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func tmpStore(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	return filepath.Join(dir, "signatures.json")
}

func makeSig(id, typ, eco, pkg, pattern string) DetectionSignature {
	return DetectionSignature{
		ID:          id,
		Type:        SignatureType(typ),
		Ecosystem:   eco,
		Package:     pkg,
		Pattern:     pattern,
		Severity:    "HIGH",
		Title:       "Test " + id,
		Description: "desc",
		Source:      "test",
		CreatedAt:   time.Now(),
	}
}

// ── LoadStore ────────────────────────────────────────────────────────────────

func TestLoadStore_MissingFile_ReturnsEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonexistent.json")
	store, err := LoadStore(path)
	if err != nil {
		t.Fatalf("want nil err, got %v", err)
	}
	if store == nil {
		t.Fatal("want non-nil store")
	}
	if len(store.Signatures) != 0 {
		t.Errorf("want 0 sigs, got %d", len(store.Signatures))
	}
}

func TestLoadStore_InvalidJSON_ReturnsError(t *testing.T) {
	path := tmpStore(t)
	os.WriteFile(path, []byte("not json"), 0o600)
	_, err := LoadStore(path)
	if err == nil {
		t.Error("want error for invalid JSON, got nil")
	}
}

// ── SaveStore + LoadStore round-trip ─────────────────────────────────────────

func TestSaveLoad_RoundTrip(t *testing.T) {
	path := tmpStore(t)
	store := &SignatureStore{
		Version: 1,
		Signatures: []DetectionSignature{
			makeSig("FG-test-1", "blocklisted_package", "npm", "evil-pkg", ""),
			makeSig("FG-test-2", "malware_pattern", "*", "", "eval\\("),
		},
	}
	if err := SaveStore(path, store); err != nil {
		t.Fatalf("SaveStore: %v", err)
	}
	loaded, err := LoadStore(path)
	if err != nil {
		t.Fatalf("LoadStore: %v", err)
	}
	if len(loaded.Signatures) != 2 {
		t.Errorf("want 2 sigs, got %d", len(loaded.Signatures))
	}
	if loaded.Signatures[0].ID != "FG-test-1" {
		t.Errorf("want FG-test-1, got %s", loaded.Signatures[0].ID)
	}
	if loaded.UpdatedAt.IsZero() {
		t.Error("UpdatedAt must be set by SaveStore")
	}
}

func TestSaveStore_AtomicWrite(t *testing.T) {
	path := tmpStore(t)
	store := &SignatureStore{Version: 1}
	if err := SaveStore(path, store); err != nil {
		t.Fatalf("SaveStore: %v", err)
	}
	// Temp file must be cleaned up
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Error(".tmp file not cleaned up after save")
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("final file not present: %v", err)
	}
}

// ── MergeSignatures ──────────────────────────────────────────────────────────

func TestMergeSignatures_AddsNew(t *testing.T) {
	store := &SignatureStore{Version: 1}
	sigs := []DetectionSignature{
		makeSig("FG-1", "blocklisted_package", "npm", "evil", ""),
	}
	added := MergeSignatures(store, sigs)
	if added != 1 {
		t.Errorf("want added=1, got %d", added)
	}
	if len(store.Signatures) != 1 {
		t.Errorf("want 1 sig in store, got %d", len(store.Signatures))
	}
}

func TestMergeSignatures_DeduplicatesSameKey(t *testing.T) {
	sig := makeSig("FG-1", "blocklisted_package", "npm", "evil", "")
	store := &SignatureStore{Signatures: []DetectionSignature{sig}}
	added := MergeSignatures(store, []DetectionSignature{sig})
	if added != 0 {
		t.Errorf("duplicate: want added=0, got %d", added)
	}
	if len(store.Signatures) != 1 {
		t.Errorf("duplicate: want 1 sig, got %d", len(store.Signatures))
	}
}

func TestMergeSignatures_AddsDistinctPackage(t *testing.T) {
	store := &SignatureStore{Signatures: []DetectionSignature{
		makeSig("FG-1", "blocklisted_package", "npm", "evil-a", ""),
	}}
	added := MergeSignatures(store, []DetectionSignature{
		makeSig("FG-2", "blocklisted_package", "npm", "evil-b", ""),
	})
	if added != 1 {
		t.Errorf("distinct package: want added=1, got %d", added)
	}
	if len(store.Signatures) != 2 {
		t.Errorf("want 2 sigs, got %d", len(store.Signatures))
	}
}

func TestMergeSignatures_SamePackageDifferentEco_BothAdded(t *testing.T) {
	store := &SignatureStore{Signatures: []DetectionSignature{
		makeSig("FG-1", "blocklisted_package", "npm", "evil", ""),
	}}
	added := MergeSignatures(store, []DetectionSignature{
		makeSig("FG-2", "blocklisted_package", "pypi", "evil", ""),
	})
	if added != 1 {
		t.Errorf("different eco: want added=1, got %d", added)
	}
}

func TestMergeSignatures_Empty(t *testing.T) {
	store := &SignatureStore{}
	added := MergeSignatures(store, nil)
	if added != 0 {
		t.Errorf("empty: want 0, got %d", added)
	}
}

// ── expandTilde ──────────────────────────────────────────────────────────────

func TestExpandTilde_NoTilde_Unchanged(t *testing.T) {
	got := expandTilde("/absolute/path")
	if got != "/absolute/path" {
		t.Errorf("want /absolute/path, got %s", got)
	}
}

func TestExpandTilde_TildePrefix_Expanded(t *testing.T) {
	got := expandTilde("~/some/path")
	if got == "~/some/path" {
		t.Error("tilde not expanded")
	}
	if len(got) < 5 {
		t.Error("expanded path too short")
	}
}
