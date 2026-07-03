// Command sigcompile compiles the community signature YAML tree
// (signatures/**/*.yaml in the main ForgeGuardian repo, or the same tree
// mirrored into forgeguardian-signatures/) into the single signatures.json
// file that `fgctl update` downloads and internal/intelligence.LoadStore
// reads at scan time.
//
// Usage:
//
//	go run ./cmd/sigcompile -in signatures -out forgeguardian-signatures/dist/signatures.json
//
// This is the missing link between "someone merged a signature YAML PR" and
// "fgctl update actually serves it" — CI in forgeguardian-signatures should
// run this on every push to main and commit the result (or upload it as a
// release asset), otherwise merging a signature PR has no visible effect.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mah3sec/forgeguardian/internal/intelligence"
)

func main() {
	in := flag.String("in", "signatures", "root directory to walk for signature YAML files")
	out := flag.String("out", "dist/signatures.json", "output path for the compiled signatures.json")
	flag.Parse()

	sigs, err := intelligence.LoadYAMLSignatures(*in)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sigcompile: %v\n", err)
		os.Exit(1)
	}
	if len(sigs) == 0 {
		fmt.Fprintf(os.Stderr, "sigcompile: no valid signatures found under %s — refusing to write an empty signatures.json\n", *in)
		os.Exit(1)
	}

	store := intelligence.SignatureStore{
		Version:    1,
		Signatures: sigs,
	}

	if err := os.MkdirAll(filepath.Dir(*out), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "sigcompile: mkdir %s: %v\n", filepath.Dir(*out), err)
		os.Exit(1)
	}

	if err := intelligence.SaveStore(*out, &store); err != nil {
		fmt.Fprintf(os.Stderr, "sigcompile: write %s: %v\n", *out, err)
		os.Exit(1)
	}

	data, _ := json.Marshal(store.Signatures)
	fmt.Printf("sigcompile: wrote %d signature(s) (%d bytes) → %s\n", len(sigs), len(data), *out)
}
