// intel.go — fgctl intel subcommands:
//
//	fgctl intel new                    interactive signature wizard
//	fgctl intel validate <file>        schema + regex validation
//	fgctl intel test <file> [flags]    run signature against a live package
//	fgctl intel update                 pull latest community signatures
//	fgctl intel list                   list loaded signatures
package main

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/mah3sec/forgeguardian/internal/core"
	"github.com/mah3sec/forgeguardian/internal/intelligence"
	"github.com/mah3sec/forgeguardian/internal/scanner"
	"github.com/mah3sec/forgeguardian/internal/scanner/behavioral"
	"github.com/mah3sec/forgeguardian/internal/scanner/malware"
	"github.com/mah3sec/forgeguardian/internal/scanner/mcp"
	"github.com/mah3sec/forgeguardian/internal/scanner/osv"
)

const (
	communitySignaturesURL = "https://raw.githubusercontent.com/mah3sec/forgeguardian-signatures/main/dist/signatures.json"
	communityRepoURL       = "https://github.com/mah3sec/forgeguardian-signatures"
)

// runIntel dispatches fgctl intel subcommands.
func runIntel(args []string) error {
	if len(args) == 0 {
		printIntelUsage()
		return nil
	}
	switch args[0] {
	case "new":
		return runIntelNew(args[1:])
	case "validate":
		return runIntelValidate(args[1:])
	case "test":
		return runIntelTest(args[1:])
	case "update":
		return runIntelUpdate(args[1:])
	case "list":
		return runIntelList(args[1:])
	case "help", "--help", "-h":
		printIntelUsage()
		return nil
	default:
		return fmt.Errorf("unknown subcommand %q\n\nRun: fgctl intel help", args[0])
	}
}

func printIntelUsage() {
	fmt.Println(`fgctl intel — community signature toolkit

Usage:
  fgctl intel new                  Interactive wizard to create a signature
  fgctl intel validate <file>      Validate a signature YAML file
  fgctl intel test <file> [flags]  Test signature against a live package
  fgctl intel update               Pull latest community signatures
  fgctl intel list                 List all loaded signatures

Examples:
  fgctl intel new
  fgctl intel validate ./signatures/blocklisted/npm/FG-npm-evil-pkg.yaml
  fgctl intel test ./FG-npm-evil-pkg.yaml --ecosystem=npm --package=evil-pkg --version=1.0.0
  fgctl intel update
  fgctl intel list --type=malware_pattern

Community repo: ` + communityRepoURL)
}

// ─── new ─────────────────────────────────────────────────────────────────────

func runIntelNew(args []string) error {
	fs := flag.NewFlagSet("intel new", flag.ExitOnError)
	outFlag := fs.String("out", "", "output file path (default: auto-named in current dir)")
	fs.Parse(args)

	r := bufio.NewReader(os.Stdin)

	fmt.Println()
	fmt.Println("┌─────────────────────────────────────────────────────────┐")
	fmt.Println("│  ForgeGuardian — New Signature Wizard                   │")
	fmt.Println("│  Answer the questions below. Press Enter for defaults.  │")
	fmt.Println("└─────────────────────────────────────────────────────────┘")
	fmt.Println()

	sigTypes := []string{
		"blocklisted_package",
		"typosquatting_target",
		"behavioral_rule",
		"malware_pattern",
		"mcp_injection_pattern",
		"pickle_rule",
	}
	sigTypeDesc := map[string]string{
		"blocklisted_package":   "A specific package version confirmed malicious",
		"typosquatting_target":  "A popular package that attackers misspell to hijack",
		"behavioral_rule":       "A dangerous install script pattern (regex)",
		"malware_pattern":       "A byte/regex pattern found in malware (regex)",
		"mcp_injection_pattern": "A prompt injection pattern in MCP tool descriptions (regex)",
		"pickle_rule":           "An unsafe AI model configuration pattern",
	}

	fmt.Println("Signature types:")
	for i, t := range sigTypes {
		fmt.Printf("  %d. %-28s %s\n", i+1, t, sigTypeDesc[t])
	}
	sigType := promptChoice(r, "Type (1-6)", sigTypes)

	ecosystems := []string{"npm", "pypi", "go", "rubygems", "crates", "maven", "huggingface", "mcp", "*"}
	ecosystem := promptChoice(r, "Ecosystem", ecosystems)

	severity := promptChoice(r, "Severity", []string{"CRITICAL", "HIGH", "MEDIUM", "LOW", "INFORMATIONAL"})

	name := prompt(r, "Short name (e.g. 'Backdoor in evil-pkg@1.0.0')", "")
	for name == "" {
		fmt.Println("  Name is required.")
		name = prompt(r, "Short name", "")
	}

	description := prompt(r, "Description (what does it do / why is it dangerous)", "")
	for description == "" {
		fmt.Println("  Description is required.")
		description = prompt(r, "Description", "")
	}

	author := prompt(r, "Your GitHub username", "anonymous")
	cve := prompt(r, "CVE ID (leave blank if none)", "")
	references := prompt(r, "Reference URL (leave blank to skip)", "")

	// Type-specific fields
	var pkgName, target, pattern, rule string
	switch intelligence.SignatureType(sigType) {
	case intelligence.SigBlocklisted:
		pkgName = prompt(r, "Package name (exact)", "")
	case intelligence.SigTypoTarget:
		target = prompt(r, "Legitimate package name to protect", "")
	case intelligence.SigMalwarePattern, intelligence.SigMCPInjection:
		pattern = prompt(r, "Regex pattern", "")
		for pattern != "" {
			if _, err := regexp.Compile(pattern); err != nil {
				fmt.Printf("  Invalid regex: %v — try again.\n", err)
				pattern = prompt(r, "Regex pattern", "")
			} else {
				break
			}
		}
	case intelligence.SigBehavioral, intelligence.SigPickleRule:
		rule = prompt(r, "Rule description or regex", "")
	}

	// Build ID
	sigID := intelligence.BuildSignatureID(intelligence.SignatureType(sigType), ecosystem, name)

	tags := intelligence.DefaultTags(intelligence.SignatureType(sigType))

	sig := sigFileYAML{
		ID:          sigID,
		Name:        name,
		Type:        sigType,
		Ecosystem:   ecosystem,
		Severity:    severity,
		Package:     pkgName,
		Target:      target,
		Pattern:     pattern,
		Rule:        rule,
		Description: description,
		Author:      author,
		Tags:        tags,
	}
	if cve != "" {
		sig.CVE = cve
	}
	if references != "" {
		sig.References = []string{references}
	}

	data, err := yaml.Marshal(sig)
	if err != nil {
		return fmt.Errorf("marshal yaml: %w", err)
	}

	outPath := *outFlag
	if outPath == "" {
		outPath = sigID + ".yaml"
	}

	if err := os.WriteFile(outPath, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", outPath, err)
	}

	fmt.Println()
	fmt.Printf("✓  Signature written to: %s\n", outPath)
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Printf("  1. Review: cat %s\n", outPath)
	fmt.Printf("  2. Validate: fgctl intel validate %s\n", outPath)
	fmt.Printf("  3. Test:     fgctl intel test %s --ecosystem=%s --package=<pkg> --version=<ver>\n", outPath, ecosystem)
	fmt.Printf("  4. Submit:   %s/fork\n", communityRepoURL)
	fmt.Printf("     Place in: signatures/%s/%s/%s\n", intelligence.SigTypeDir(intelligence.SignatureType(sigType)), ecosystem, outPath)
	fmt.Printf("     PR title: [sig] %s — %s\n", sigID, name)
	return nil
}

// ─── validate ────────────────────────────────────────────────────────────────

// sigFileYAML is a local alias for the community YAML format, kept so the
// rest of this file (and its callers) don't need to change. The struct
// itself — and the validation/authoring logic — now live in
// internal/intelligence/authoring.go so the REST API can share them.
type sigFileYAML = intelligence.SignatureYAML

func runIntelValidate(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: fgctl intel validate <signature-file.yaml>")
	}

	// Support glob: fgctl intel validate signatures/**/*.yaml
	files := expandGlob(args)
	if len(files) == 0 {
		return fmt.Errorf("no files matched: %v", args)
	}

	failed := 0
	for _, path := range files {
		if err := validateSigFile(path); err != nil {
			fmt.Fprintf(os.Stderr, "✗  %s\n   %v\n\n", path, err)
			failed++
		}
	}

	if failed > 0 {
		return fmt.Errorf("%d/%d signature(s) failed validation", failed, len(files))
	}
	if len(files) > 1 {
		fmt.Printf("✓  All %d signatures valid\n", len(files))
	}
	return nil
}

func validateSigFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("cannot read file: %w", err)
	}

	// Support both YAML and JSON
	var sig sigFileYAML
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".json" {
		if err := json.Unmarshal(data, &sig); err != nil {
			return fmt.Errorf("invalid JSON: %w", err)
		}
	} else {
		if err := yaml.Unmarshal(data, &sig); err != nil {
			return fmt.Errorf("invalid YAML: %w", err)
		}
	}

	problems := intelligence.ValidateSignature(sig)

	if len(problems) > 0 {
		return fmt.Errorf("%s\n%s", path, "   "+strings.Join(problems, "\n   "))
	}

	fmt.Printf("✓  %-55s  %s/%s  %s\n", sig.ID, sig.Type, sig.Ecosystem, strings.ToUpper(sig.Severity))
	return nil
}

func expandGlob(patterns []string) []string {
	var files []string
	for _, p := range patterns {
		matches, err := filepath.Glob(p)
		if err != nil || len(matches) == 0 {
			// Not a glob — treat as literal path
			files = append(files, p)
		} else {
			files = append(files, matches...)
		}
	}
	return files
}

// ─── test ────────────────────────────────────────────────────────────────────

func runIntelTest(args []string) error {
	fs := flag.NewFlagSet("intel test", flag.ExitOnError)
	ecosystemFlag := fs.String("ecosystem", "", "ecosystem to test against (npm, pypi, go, ...)")
	packageFlag := fs.String("package", "", "package name")
	versionFlag := fs.String("version", "", "package version")
	fs.Parse(args)

	remaining := fs.Args()
	if len(remaining) == 0 {
		return fmt.Errorf("usage: fgctl intel test <signature.yaml> --ecosystem=npm --package=lodash --version=4.17.20")
	}
	sigPath := remaining[0]

	if *packageFlag == "" || *versionFlag == "" || *ecosystemFlag == "" {
		return fmt.Errorf("--ecosystem, --package, and --version are required")
	}

	// Load and validate the signature first
	data, err := os.ReadFile(sigPath)
	if err != nil {
		return fmt.Errorf("cannot read %s: %w", sigPath, err)
	}
	var sigYAML sigFileYAML
	if err := yaml.Unmarshal(data, &sigYAML); err != nil {
		return fmt.Errorf("invalid YAML: %w", err)
	}

	// Convert to DetectionSignature
	sig := intelligence.DetectionSignature{
		ID:          sigYAML.ID,
		Type:        intelligence.SignatureType(sigYAML.Type),
		Ecosystem:   sigYAML.Ecosystem,
		Target:      sigYAML.Target,
		Pattern:     sigYAML.Pattern,
		Package:     sigYAML.Package,
		Rule:        sigYAML.Rule,
		Severity:    strings.ToUpper(sigYAML.Severity),
		Title:       sigYAML.Name,
		Description: sigYAML.Description,
		Source:      "test",
		CVE:         sigYAML.CVE,
		CreatedAt:   time.Now(),
	}

	store := &intelligence.SignatureStore{
		Version:    1,
		Signatures: []intelligence.DetectionSignature{sig},
	}

	fmt.Printf("Testing %s against %s/%s@%s...\n\n", sig.ID, *ecosystemFlag, *packageFlag, *versionFlag)

	// Download package to temp dir
	fmt.Println("  → Downloading package from registry...")
	artifact, cleanup, dlErr := downloadForTest(*ecosystemFlag, *packageFlag, *versionFlag)
	if dlErr != nil {
		fmt.Printf("  ⚠  Download failed (%v) — running metadata-only checks\n\n", dlErr)
		artifact = core.BuiltArtifact{
			Source: core.SourceArtifact{
				Package: core.PackageVersion{
					Ecosystem: *ecosystemFlag,
					Name:      *packageFlag,
					Version:   *versionFlag,
				},
			},
		}
		cleanup = func() {}
	} else {
		fmt.Printf("  ✓  Downloaded to %s\n\n", artifact.LocalPath)
	}
	defer cleanup()

	// Run only the engines relevant to this signature type
	var scanners []core.Scanner
	switch sig.Type {
	case intelligence.SigBlocklisted, intelligence.SigTypoTarget:
		scanners = []core.Scanner{
			osv.New(),
			behavioral.NewWithStore(store),
		}
	case intelligence.SigMalwarePattern:
		scanners = []core.Scanner{malware.NewWithStore(store)}
	case intelligence.SigBehavioral:
		scanners = []core.Scanner{behavioral.NewWithStore(store)}
	case intelligence.SigMCPInjection:
		scanners = []core.Scanner{mcp.NewWithStore(store)}
	default:
		// Run all community-signature-aware engines
		scanners = []core.Scanner{
			behavioral.NewWithStore(store),
			malware.NewWithStore(store),
			mcp.NewWithStore(store),
		}
	}

	orch := scanner.NewWith(scanners...)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	results := orch.Scan(ctx, artifact)
	findings := scanner.MergeFindings(results)

	// Filter to only findings that came from our test signature
	var matched []core.Finding
	for _, f := range findings {
		if f.ID == sig.ID ||
			strings.Contains(strings.ToUpper(f.Title), strings.ToUpper(sigYAML.Name)) ||
			(sig.Package != "" && strings.Contains(f.Description, sig.Package)) {
			matched = append(matched, f)
		}
	}
	// Also show all findings so contributor sees full picture
	other := findings

	if len(matched) > 0 {
		fmt.Printf("✅  MATCH — signature triggered (%d finding(s))\n\n", len(matched))
		for _, f := range matched {
			fmt.Printf("   [%s] %s\n", f.Severity, f.Title)
			fmt.Printf("          %s\n", f.Description)
			fmt.Println()
		}
	} else {
		fmt.Printf("⚠️   NO MATCH — signature did not trigger on %s/%s@%s\n\n", *ecosystemFlag, *packageFlag, *versionFlag)
		fmt.Println("   This could mean:")
		fmt.Println("   • Package is not actually malicious (test against a known-bad version)")
		fmt.Println("   • Pattern needs adjustment (check regex against package contents)")
		fmt.Println("   • Package name / ecosystem mismatch in signature")
		fmt.Println()
	}

	if len(other) > 0 {
		fmt.Printf("Other findings from all engines (%d total):\n", len(other))
		for _, f := range other {
			if f.Severity == core.SeverityInformational {
				continue
			}
			fmt.Printf("   [%-12s] [%-12s] %s\n", f.Severity, f.Source, f.Title)
		}
		fmt.Println()
	}

	return nil
}

// downloadForTest downloads a package tarball and extracts to temp dir.
// Reuses logic from handlers/scan_engine.go patterns — pure stdlib.
func downloadForTest(ecosystem, name, version string) (core.BuiltArtifact, func(), error) {
	var dlURL string
	switch ecosystem {
	case "npm":
		meta := struct {
			Dist struct {
				Tarball string `json:"tarball"`
			} `json:"dist"`
		}{}
		resp, err := http.Get(fmt.Sprintf("https://registry.npmjs.org/%s/%s", name, version))
		if err != nil {
			return core.BuiltArtifact{}, nil, err
		}
		json.NewDecoder(resp.Body).Decode(&meta)
		resp.Body.Close()
		dlURL = meta.Dist.Tarball
	case "pypi":
		meta := struct {
			URLs []struct {
				URL      string `json:"url"`
				Filename string `json:"filename"`
			} `json:"urls"`
		}{}
		resp, err := http.Get(fmt.Sprintf("https://pypi.org/pypi/%s/%s/json", name, version))
		if err != nil {
			return core.BuiltArtifact{}, nil, err
		}
		json.NewDecoder(resp.Body).Decode(&meta)
		resp.Body.Close()
		for _, u := range meta.URLs {
			if strings.HasSuffix(u.Filename, ".tar.gz") {
				dlURL = u.URL
				break
			}
		}
		if dlURL == "" && len(meta.URLs) > 0 {
			dlURL = meta.URLs[0].URL
		}
	case "go":
		dlURL = fmt.Sprintf("https://proxy.golang.org/%s/@v/%s.zip", name, version)
	case "crates":
		dlURL = fmt.Sprintf("https://static.crates.io/crates/%s/%s/download", name, version)
	default:
		return core.BuiltArtifact{}, nil, fmt.Errorf("unsupported ecosystem %q for test download", ecosystem)
	}

	if dlURL == "" {
		return core.BuiltArtifact{}, nil, fmt.Errorf("could not resolve download URL for %s/%s@%s", ecosystem, name, version)
	}

	resp, err := http.Get(dlURL)
	if err != nil {
		return core.BuiltArtifact{}, nil, err
	}
	defer resp.Body.Close()

	tmp, err := os.CreateTemp("", "fg-test-*.tgz")
	if err != nil {
		return core.BuiltArtifact{}, nil, err
	}
	io.Copy(tmp, io.LimitReader(resp.Body, 128*1024*1024))
	tmp.Close()

	dir, err := os.MkdirTemp("", "fg-test-ext-*")
	if err != nil {
		os.Remove(tmp.Name())
		return core.BuiltArtifact{}, nil, err
	}

	// Extract
	f, _ := os.Open(tmp.Name())
	if gr, err2 := gzip.NewReader(f); err2 == nil {
		tr := tar.NewReader(gr)
		for {
			hdr, err3 := tr.Next()
			if err3 == io.EOF {
				break
			}
			if err3 != nil {
				break
			}
			if hdr.Typeflag != tar.TypeReg {
				continue
			}
			target := filepath.Join(dir, filepath.Clean(hdr.Name))
			os.MkdirAll(filepath.Dir(target), 0o755)
			out, _ := os.Create(target)
			if out != nil {
				io.Copy(out, io.LimitReader(tr, 10*1024*1024))
				out.Close()
			}
		}
		gr.Close()
	}
	f.Close()
	os.Remove(tmp.Name())

	cleanup := func() { os.RemoveAll(dir) }
	return core.BuiltArtifact{
		Source: core.SourceArtifact{
			Package:   core.PackageVersion{Ecosystem: ecosystem, Name: name, Version: version},
			LocalPath: dir,
		},
		LocalPath: dir,
		BuildTime: time.Now(),
	}, cleanup, nil
}

// ─── update ──────────────────────────────────────────────────────────────────

func runIntelUpdate(args []string) error {
	fs := flag.NewFlagSet("intel update", flag.ExitOnError)
	storeFlag := fs.String("store", "", "signature store path (default: ~/.forgeguardian/signatures.json)")
	urlFlag := fs.String("url", communitySignaturesURL, "signatures bundle URL")
	fs.Parse(args)

	storePath := *storeFlag
	if storePath == "" {
		var err error
		storePath, err = intelligence.DefaultStorePath()
		if err != nil {
			return err
		}
	}

	fmt.Printf("Fetching community signatures from:\n  %s\n\n", *urlFlag)

	result, err := intelligence.UpdateFromCommunity(storePath, *urlFlag)
	if err != nil {
		return fmt.Errorf("%w\n\nRepo: %s\n\nThe community repo may not have published a bundle yet.\nYou can load individual YAML files with: fgctl intel validate ./signatures/**/*.yaml", err, communityRepoURL)
	}

	fmt.Printf("Community signatures: %d total", result.Total)
	if result.Added > 0 {
		fmt.Printf(" (↑ %d new since last update)", result.Added)
	} else {
		fmt.Printf(" (already up to date)")
	}
	fmt.Printf("\nPrevious: %d  →  Now: %d\n", result.Before, result.Total)
	fmt.Printf("Stored at: %s\n\n", storePath)
	fmt.Printf("Contribute a signature: %s\n", communityRepoURL)
	return nil
}

// ─── list ────────────────────────────────────────────────────────────────────

func runIntelList(args []string) error {
	fs := flag.NewFlagSet("intel list", flag.ExitOnError)
	typeFlag := fs.String("type", "", "filter by signature type")
	ecoFlag := fs.String("ecosystem", "", "filter by ecosystem")
	storeFlag := fs.String("store", "", "signature store path")
	fs.Parse(args)

	storePath := *storeFlag
	if storePath == "" {
		var err error
		storePath, err = intelligence.DefaultStorePath()
		if err != nil {
			return err
		}
	}

	store, err := intelligence.LoadStoreWithLocalFallback(storePath)
	if err != nil {
		return err
	}

	if len(store.Signatures) == 0 {
		fmt.Println("No signatures loaded. Run: fgctl intel update")
		return nil
	}

	var sigs []intelligence.DetectionSignature
	for _, s := range store.Signatures {
		if *typeFlag != "" && string(s.Type) != *typeFlag {
			continue
		}
		if *ecoFlag != "" && s.Ecosystem != *ecoFlag && s.Ecosystem != "*" {
			continue
		}
		sigs = append(sigs, s)
	}

	fmt.Printf("%-50s  %-24s  %-12s  %-8s\n", "ID", "TYPE", "ECOSYSTEM", "SEVERITY")
	fmt.Println(strings.Repeat("─", 100))
	for _, s := range sigs {
		fmt.Printf("%-50s  %-24s  %-12s  %-8s\n",
			truncate(s.ID, 50), truncate(string(s.Type), 24), s.Ecosystem, s.Severity)
	}
	fmt.Printf("\n%d signature(s)", len(sigs))
	if *typeFlag != "" || *ecoFlag != "" {
		fmt.Printf(" (filtered from %d total)", len(store.Signatures))
	}
	fmt.Println()
	return nil
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func prompt(r *bufio.Reader, question, defaultVal string) string {
	if defaultVal != "" {
		fmt.Printf("  %s [%s]: ", question, defaultVal)
	} else {
		fmt.Printf("  %s: ", question)
	}
	answer, _ := r.ReadString('\n')
	answer = strings.TrimSpace(answer)
	if answer == "" {
		return defaultVal
	}
	return answer
}

func promptChoice(r *bufio.Reader, question string, choices []string) string {
	fmt.Printf("  %s\n", question)
	for i, c := range choices {
		fmt.Printf("    %d. %s\n", i+1, c)
	}
	fmt.Printf("  Choice [1-%d, or type value]: ", len(choices))
	answer, _ := r.ReadString('\n')
	answer = strings.TrimSpace(answer)
	// Try numeric
	for i, c := range choices {
		if answer == fmt.Sprintf("%d", i+1) {
			return c
		}
	}
	// Try exact match
	for _, c := range choices {
		if strings.EqualFold(answer, c) {
			return c
		}
	}
	// Default to first
	if answer == "" {
		return choices[0]
	}
	return answer
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
