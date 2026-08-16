// Command fgctl is the ForgeGuardian command-line interface.
//
// Sub-commands:
//
//	fgctl sign     --recipe=npm --package=lodash --version=4.17.21 [--rekor-url=...] [--out=att.json]
//	fgctl verify   --attestation=att.json --sha256=<hex>
//	fgctl scan     --recipe=npm --package=lodash --version=4.17.21 [--json] [--fail-on=high]
//	fgctl advisory --recipe=npm --package=lodash --version=4.17.21 [--api-key=...] [--json]
//	fgctl sbom     --recipe=npm --package=lodash --version=4.17.21 [--format=cyclonedx-json] [--out=sbom.json]
//	fgctl provenance --recipe=npm --package=lodash --version=4.17.21 [--out=prov.json]
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/mah3sec/forgeguardian/internal/agent/triage"
	"github.com/mah3sec/forgeguardian/internal/build/recipes"
	_ "github.com/mah3sec/forgeguardian/internal/build/recipes/all"
	"github.com/mah3sec/forgeguardian/internal/core"
	"github.com/mah3sec/forgeguardian/internal/intelligence"
	"github.com/mah3sec/forgeguardian/internal/license"
	"github.com/mah3sec/forgeguardian/internal/localscanner"
	"github.com/mah3sec/forgeguardian/internal/sbom"
	"github.com/mah3sec/forgeguardian/internal/scanner"
	"github.com/mah3sec/forgeguardian/internal/signer"
	"github.com/mah3sec/forgeguardian/internal/ui"
)

// Set via -ldflags at build time; fall back to Go module info for `go install`.
var (
	version   = ""
	commit    = ""
	buildTime = ""
)

func init() {
	if version != "" {
		return
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		version, commit, buildTime = "dev", "none", "unknown"
		return
	}
	version = info.Main.Version
	if version == "" || version == "(devel)" {
		version = "dev"
	}
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			if len(s.Value) > 7 {
				commit = s.Value[:7]
			} else {
				commit = s.Value
			}
		case "vcs.time":
			buildTime = s.Value
		}
	}
	if commit == "" {
		commit = "none"
	}
	if buildTime == "" {
		buildTime = "unknown"
	}
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	logLevel := slog.LevelWarn
	if len(os.Args) > 1 {
		for _, a := range os.Args[1:] {
			if a == "--debug" || a == "-debug" {
				logLevel = slog.LevelInfo
				break
			}
		}
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel}))

	cmd := os.Args[1]
	rawArgs := os.Args[2:]

	globalFlags := map[string]bool{"--no-banner": true, "--no-color": true, "-no-banner": true, "-no-color": true}
	args := make([]string, 0, len(rawArgs))
	for _, a := range rawArgs {
		if !globalFlags[a] {
			args = append(args, a)
		}
	}

	// Pre-parse to detect machine-output modes before flag.Parse runs in subcommands.
	suppressHuman, noBanner := preParseOutputFlags(os.Args)
	noColor := os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb"
	p := ui.New(noColor, suppressHuman, version)
	if noBanner {
		p.NoBanner = true
	}

	// Load signature count for banner display (best-effort, silent on error).
	if !suppressHuman {
		if storePath, err := signaturesStorePath(); err == nil {
			if store, err := intelligence.LoadStoreWithLocalFallback(storePath); err == nil {
				p.SetSigCount(len(store.Signatures))
				heuristicCount := 0
				for _, s := range store.Signatures {
					if s.Type == intelligence.SigMalwarePattern {
						heuristicCount++
					}
				}
				p.SetHeuristicCount(heuristicCount)
				p.SetEngineCount(8) // matches orchestrator.New() registered engines
			}
		}
	}

	if !suppressHuman && cmd != "help" && cmd != "--help" && cmd != "-h" && cmd != "version" {
		p.Banner()
	}

	// First-run check: skip in machine-output/CI modes to avoid stdin prompts.
	if !suppressHuman {
		firstRunCheck(logger)
	}

	var err error
	switch cmd {
	case "sign":
		err = runSign(args, logger)
	case "verify":
		err = runVerify(args, logger)
	case "scan":
		err = runScanCmd(args, logger, p)
	case "advisory":
		if err = license.RequirePro("fgctl advisory"); err != nil {
			break
		}
		err = runAdvisory(args, logger)
	case "sbom":
		err = runSBOM(args, logger)
	case "provenance":
		err = runProvenance(args, logger)
	case "update":
		err = runUpdate(args, logger)
	case "patch":
		if err = license.RequirePro("fgctl patch"); err != nil {
			break
		}
		err = runPatch(args, logger)
	case "monitor":
		if err = license.RequirePro("fgctl monitor"); err != nil {
			break
		}
		err = runMonitor(args, logger, p)
	case "audit":
		err = runAudit(args, logger, p)
	case "debug":
		err = runDebug(args, logger, p)
	case "config":
		err = runConfig(args, logger, p)
	case "intel":
		err = runIntel(args)
	case "sig":
		err = runSig(args, logger)
	case "policy":
		err = runPolicy(args, logger, p)
	case "setup":
		err = runSetup(args, logger)
	case "doctor":
		err = runDoctor(args, logger)
	case "stats":
		err = runStats(args, logger, p)
	case "license":
		fmt.Println(license.Info())
	case "help", "--help", "-h":
		printUsage()
	case "version", "--version":
		fmt.Printf("fgctl %s (%s) built %s\n", version, commit, buildTime)
	default:
		fmt.Fprintf(os.Stderr, "fgctl: unknown command %q\n\n", cmd)
		printUsage()
		os.Exit(1)
	}

	if err != nil {
		p.Error("%v", err)
		os.Exit(1)
	}
}

// preParseOutputFlags scans os.Args before flag.Parse to determine whether
// the current invocation will produce machine-readable output, so the Printer
// and banner are configured correctly before any subcommand runs.
func preParseOutputFlags(args []string) (suppressHuman, noBanner bool) {
	// sbom always writes structured data to stdout (unless --out redirects it).
	// Suppress the banner unless the user explicitly redirected output to a file.
	cmd := ""
	if len(args) > 1 {
		cmd = args[1]
	}
	if cmd == "sbom" {
		hasOut := false
		for _, a := range args[2:] {
			if strings.HasPrefix(a, "--out=") || a == "--out" {
				hasOut = true
				break
			}
		}
		if !hasOut {
			suppressHuman = true
		}
	}

	for i, a := range args {
		switch a {
		case "--json", "-json",
			"--format=json", "--format=sarif",
			"--format=cyclonedx-json", "--format=spdx-json",
			"--quiet", "-quiet", "--ci":
			suppressHuman = true
		case "--no-banner":
			noBanner = true
		case "--format":
			if i+1 < len(args) {
				switch args[i+1] {
				case "json", "sarif", "cyclonedx-json", "spdx-json":
					suppressHuman = true
				}
			}
		}
	}
	return
}

// ── sign ─────────────────────────────────────────────────────────────────────

func runSign(args []string, log *slog.Logger) error {
	fs := flag.NewFlagSet("sign", flag.ExitOnError)
	recipeFlag := fs.String("recipe", "", "ecosystem recipe (npm|pypi|maven|go|...)")
	pkgFlag := fs.String("package", "", "package name")
	verFlag := fs.String("version", "", "package version")
	rekorFlag := fs.String("rekor-url", "", "Rekor URL (default: public sigstore.dev)")
	outFlag := fs.String("out", "", "output attestation JSON file (default: stdout)")
	timeoutFlag := fs.Duration("timeout", 5*time.Minute, "total timeout")
	fs.Parse(args)

	if *recipeFlag == "" || *pkgFlag == "" || *verFlag == "" {
		return fmt.Errorf("--recipe, --package, and --version are required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeoutFlag)
	defer cancel()

	artifact, err := buildArtifact(ctx, *recipeFlag, *pkgFlag, *verFlag, log)
	if err != nil {
		return err
	}
	defer os.Remove(artifact.LocalPath)

	prov := signer.NewProvenance(artifact)
	s := signer.New(*rekorFlag)

	log.Info("signing artifact", "sha256", artifact.SHA256)
	att, err := s.Sign(ctx, artifact, prov)
	if err != nil {
		return fmt.Errorf("sign: %w", err)
	}

	if att.RekorLogID != "" {
		log.Info("uploaded to Rekor", "log_id", att.RekorLogID, "index", att.RekorIndex)
	}

	data, err := signer.SaveAttestation(att)
	if err != nil {
		return err
	}

	return writeOutput(*outFlag, data)
}

// ── verify ────────────────────────────────────────────────────────────────────

func runVerify(args []string, log *slog.Logger) error {
	fs := flag.NewFlagSet("verify", flag.ExitOnError)
	attFlag := fs.String("attestation", "", "path to attestation JSON file")
	sha256Flag := fs.String("sha256", "", "expected SHA256 of the artifact")
	timeoutFlag := fs.Duration("timeout", 2*time.Minute, "total timeout")
	fs.Parse(args)

	if *attFlag == "" || *sha256Flag == "" {
		return fmt.Errorf("--attestation and --sha256 are required")
	}

	data, err := os.ReadFile(*attFlag)
	if err != nil {
		return fmt.Errorf("read attestation: %w", err)
	}
	att, err := signer.LoadAttestation(data)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeoutFlag)
	defer cancel()

	s := signer.New("")
	result := s.Verify(ctx, att, *sha256Flag)

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(result)

	if !result.Valid {
		return fmt.Errorf("verification failed: %s", result.Error)
	}

	log.Info("verification passed",
		"sha256_match", result.SHA256Match,
		"rekor_verified", result.RekorVerified)
	return nil
}

// ── scan ──────────────────────────────────────────────────────────────────────

func runScanCmd(args []string, log *slog.Logger, p *ui.Printer) error {
	fs := flag.NewFlagSet("scan", flag.ExitOnError)
	recipeFlag := fs.String("recipe", "", "ecosystem recipe")
	pkgFlag := fs.String("package", "", "package name")
	verFlag := fs.String("version", "", "package version")
	jsonFlag := fs.Bool("json", false, "output JSON")
	failOnFlag := fs.String("fail-on", "", "exit 2 if findings at this severity or above (critical|high|medium|low)")
	timeoutFlag := fs.Duration("timeout", 10*time.Minute, "total timeout")
	workersFlag := fs.Int("workers", 4, "concurrent workers for local project scan")
	compactFlag := fs.Bool("compact", false, "one line per finding, no descriptions")
	summaryFlag := fs.Bool("summary", false, "print severity table only, no per-finding details")
	quietFlag := fs.Bool("quiet", false, "suppress all output except exit code (for CI)")
	severityFlag := fs.String("severity", "", "filter: only show findings at this severity or above (critical|high|medium|low)")
	ecosystemFlag := fs.String("ecosystem", "", "filter: only show packages from this ecosystem (npm|pypi|go|...)")
	onlyFixable := fs.Bool("only-fixable", false, "only show findings that have a known fix version")
	formatFlag := fs.String("format", "text", "output format: text|json|sarif")
	verboseFlag := fs.Bool("verbose", false, "show informational findings and expand grouped output")
	debugFlag := fs.Bool("debug", false, "show engine errors and raw metadata")
	prodOnlyFlag := fs.Bool("prod-only", false, "exclude dev dependencies from scan")
	excludeDevFlag := fs.Bool("exclude-dev", false, "alias for --prod-only")
	ciFlag := fs.Bool("ci", false, "CI mode: quiet + SARIF output + fail-on=high")
	executiveFlag := fs.Bool("executive", false, "executive summary: severity table only")
	noBannerFlag := fs.Bool("no-banner", false, "suppress the ASCII banner")
	noColorFlag := fs.Bool("no-color", false, "disable ANSI colors")
	remoteFlag := fs.String("remote", "", "SSH target user@host for a remote dependency scan")
	remotePortFlag := fs.Int("remote-port", 22, "SSH port for --remote")
	identityFlag := fs.String("identity", "", "SSH private key path (default: ~/.ssh/id_ed25519, ~/.ssh/id_rsa)")
	remotePathFlag := fs.String("remote-path", "", "remote search root for --remote (default: remote $HOME)")
	acceptNewHostKeyFlag := fs.Bool("accept-new-host-key", false, "trust a host key absent from known_hosts for this connection only (never written to known_hosts)")
	remoteMaxDepthFlag := fs.Int("remote-max-depth", 0, "limit remote find depth for --remote (0 = unlimited)")
	keepTempFlag := fs.Bool("keep-temp", false, "do not delete the local temp dir after a --remote scan (debugging)")
	// Go's flag package stops at the first non-flag positional argument, so
	// `scan . --format json` would leave "--format" and "json" unprocessed.
	// Move any positional args (path or dot-notation) to the end so all flags
	// are parsed regardless of where the user places the path.
	var flagArgs, posArgs []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if strings.HasPrefix(a, "-") {
			flagArgs = append(flagArgs, a)
			// Consume the next token if this flag takes a value (not --foo=bar style).
			if !strings.Contains(a, "=") {
				// Peek: if the flag name (without leading dashes) is a known value-flag,
				// grab the next arg as its value.
				name := strings.TrimLeft(a, "-")
				knownValueFlags := map[string]bool{
					"recipe": true, "package": true, "version": true,
					"fail-on": true, "timeout": true, "workers": true,
					"severity": true, "ecosystem": true, "format": true,
					"remote": true, "remote-port": true, "identity": true,
					"remote-path": true, "remote-max-depth": true,
				}
				if knownValueFlags[name] && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
					i++
					flagArgs = append(flagArgs, args[i])
				}
			}
		} else {
			posArgs = append(posArgs, a)
		}
	}
	fs.Parse(append(flagArgs, posArgs...))

	// Apply composite modes before flag normalization.
	if *ciFlag {
		if *formatFlag == "text" {
			*formatFlag = "sarif"
		}
		*quietFlag = true
		if *failOnFlag == "" {
			*failOnFlag = "high"
		}
	}
	if *executiveFlag {
		*summaryFlag = true
	}
	if *noBannerFlag {
		p.NoBanner = true
	}
	if *noColorFlag {
		p.NoColor = true
		color.NoColor = true
	}
	p.Verbose = *verboseFlag
	p.Debug = *debugFlag

	// --json is a legacy alias for --format=json.
	if *jsonFlag && *formatFlag == "text" {
		*formatFlag = "json"
	}
	// Apply quiet mode and any non-text format to printer — suppresses all decoration.
	if *quietFlag || *formatFlag != "text" {
		p.JSON = true
	}

	localOpts := localScanOpts{
		jsonMode:    *jsonFlag,
		failOn:      *failOnFlag,
		workers:     *workersFlag,
		timeout:     *timeoutFlag,
		compact:     *compactFlag,
		summaryOnly: *summaryFlag,
		quiet:       *quietFlag,
		severity:    *severityFlag,
		ecosystem:   *ecosystemFlag,
		onlyFixable: *onlyFixable,
		prodOnly:    *prodOnlyFlag || *excludeDevFlag,
		verbose:     *verboseFlag,
		debug:       *debugFlag,
		format:      *formatFlag,
	}

	// Remote SSH scan: `fgctl scan --remote user@host`
	if *remoteFlag != "" {
		return runRemoteScan(*remoteFlag, remoteScanOpts{
			port:             *remotePortFlag,
			identity:         *identityFlag,
			remotePath:       *remotePathFlag,
			acceptNewHostKey: *acceptNewHostKeyFlag,
			maxDepth:         *remoteMaxDepthFlag,
			keepTemp:         *keepTempFlag,
			local:            localOpts,
		}, log, p)
	}

	// Local project scan: `fgctl scan .` or `fgctl scan <path>`
	remaining := fs.Args()
	if len(remaining) > 0 && isLocalPath(remaining[0]) {
		return runLocalScan(remaining[0], localOpts, log, p)
	}

	// Dot-notation shortcut: `fgctl scan npm/lodash@4.17.20`
	if len(remaining) > 0 && isDotNotation(remaining[0]) {
		eco, pkg, ver := parseDotNotation(remaining[0])
		recipeFlag, pkgFlag, verFlag = &eco, &pkg, &ver
	}

	if *recipeFlag == "" || *pkgFlag == "" || *verFlag == "" {
		return fmt.Errorf("--recipe, --package, and --version are required\n  Or use: fgctl scan .  (local project)  |  fgctl scan npm/lodash@4.17.20")
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeoutFlag)
	defer cancel()

	sp := p.Spinner("Fetching artifact...")
	artifact, err := buildArtifact(ctx, *recipeFlag, *pkgFlag, *verFlag, log)
	ui.StopSpinner(sp)
	if err != nil {
		return err
	}
	defer os.Remove(artifact.LocalPath)

	sp2 := p.Spinner("Running scan engines...")
	orch := scanner.New()
	results := orch.Scan(ctx, artifact)
	allFindings := scanner.MergeFindings(results)
	ui.StopSpinner(sp2)

	// Separate tool-not-installed informational noise from real findings.
	var findings []core.Finding
	var missingTools []string
	seen := map[string]bool{}
	for _, f := range allFindings {
		if f.ID == "TOOL-NOT-INSTALLED" {
			if !seen[f.Source] {
				seen[f.Source] = true
				missingTools = append(missingTools, f.Source)
			}
			continue
		}
		findings = append(findings, f)
	}
	// Apply severity filter for registry scans (same as local scans).
	if *severityFlag != "" {
		threshold := core.Severity(strings.ToUpper(*severityFlag))
		var filtered []core.Finding
		for _, f := range findings {
			if severityAtLeast(f.Severity, threshold) {
				filtered = append(filtered, f)
			}
		}
		findings = filtered
	}

	summary := scanner.Summarize(findings)
	p.PrintMissingToolsWarning(missingTools)

	switch *formatFlag {
	case "sarif":
		return ui.WriteSARIF(os.Stdout, fmt.Sprintf("%s@%s", *pkgFlag, *verFlag), "", findings, summary, version)
	case "json":
		output := map[string]any{
			"package":  fmt.Sprintf("%s@%s", *pkgFlag, *verFlag),
			"sha256":   artifact.SHA256,
			"summary":  summary,
			"findings": findings,
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(output)
	}

	p.PrintFindings(fmt.Sprintf("%s@%s", *pkgFlag, *verFlag), findings, summary)

	if *failOnFlag != "" {
		threshold := core.Severity(strings.ToUpper(*failOnFlag))
		for _, f := range findings {
			if severityAtLeast(f.Severity, threshold) {
				os.Exit(2)
			}
		}
	}
	return nil
}

// ── advisory ──────────────────────────────────────────────────────────────────

func runAdvisory(args []string, log *slog.Logger) error {
	fs := flag.NewFlagSet("advisory", flag.ExitOnError)
	recipeFlag := fs.String("recipe", "", "ecosystem recipe")
	pkgFlag := fs.String("package", "", "package name")
	verFlag := fs.String("version", "", "package version")
	apiKeyFlag := fs.String("api-key", "", "Anthropic API key (default: ANTHROPIC_API_KEY env)")
	skipScanFlag := fs.Bool("skip-scan", false, "skip scan phase")
	jsonFlag := fs.Bool("json", false, "output JSON")
	timeoutFlag := fs.Duration("timeout", 10*time.Minute, "total timeout")
	fs.Parse(args)

	if *recipeFlag == "" || *pkgFlag == "" || *verFlag == "" {
		return fmt.Errorf("--recipe, --package, and --version are required")
	}

	apiKey := *apiKeyFlag
	if apiKey == "" {
		apiKey = os.Getenv("ANTHROPIC_API_KEY")
	}
	if apiKey == "" {
		return fmt.Errorf("ANTHROPIC_API_KEY not set — pass --api-key or set the env var")
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeoutFlag)
	defer cancel()

	artifact, err := buildArtifact(ctx, *recipeFlag, *pkgFlag, *verFlag, log)
	if err != nil {
		return err
	}
	defer os.Remove(artifact.LocalPath)

	var findings []core.Finding
	if !*skipScanFlag {
		log.Info("scanning artifact")
		orch := scanner.New()
		results := orch.Scan(ctx, artifact)
		findings = scanner.MergeFindings(results)
	}

	log.Info("generating AI advisory")
	eng := triage.New(apiKey)
	advisory, err := eng.Triage(ctx, artifact, findings)
	if err != nil {
		return fmt.Errorf("triage: %w", err)
	}

	if *jsonFlag {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(advisory)
	}

	fmt.Printf("Advisory for %s@%s (%s)\n", advisory.Package.Name, advisory.Package.Version, advisory.Package.Ecosystem)
	fmt.Printf("  Severity:   %s (confidence %.0f%%)\n", advisory.Severity, advisory.Confidence*100)
	fmt.Printf("  Advisory:   %s\n", advisory.Advisory)
	fmt.Printf("  Action:     %s\n", advisory.RecommendedAction)
	return nil
}

// ── sbom ──────────────────────────────────────────────────────────────────────

func runSBOM(args []string, log *slog.Logger) error {
	fs := flag.NewFlagSet("sbom", flag.ExitOnError)
	recipeFlag := fs.String("recipe", "", "ecosystem recipe")
	pkgFlag := fs.String("package", "", "package name")
	verFlag := fs.String("version", "", "package version")
	formatFlag := fs.String("format", "cyclonedx-json", "sbom format: cyclonedx-json|cyclonedx-xml|spdx-json|spdx-tv")
	outFlag := fs.String("out", "", "output file (default: stdout)")
	timeoutFlag := fs.Duration("timeout", 10*time.Minute, "total timeout")

	var flagArgs, posArgs []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if strings.HasPrefix(a, "-") {
			flagArgs = append(flagArgs, a)
			if !strings.Contains(a, "=") {
				name := strings.TrimLeft(a, "-")
				knownValueFlags := map[string]bool{
					"recipe": true, "package": true, "version": true,
					"format": true, "out": true, "timeout": true,
				}
				if knownValueFlags[name] && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
					i++
					flagArgs = append(flagArgs, args[i])
				}
			}
		} else {
			posArgs = append(posArgs, a)
		}
	}
	fs.Parse(append(flagArgs, posArgs...))

	remaining := fs.Args()
	if len(remaining) > 0 && isLocalPath(remaining[0]) {
		return runLocalSBOM(remaining[0], *formatFlag, *outFlag, log)
	}
	if len(remaining) > 0 && isDotNotation(remaining[0]) {
		eco, pkg, ver := parseDotNotation(remaining[0])
		recipeFlag, pkgFlag, verFlag = &eco, &pkg, &ver
	}

	if *recipeFlag == "" || *pkgFlag == "" || *verFlag == "" {
		return fmt.Errorf("--recipe, --package, and --version are required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeoutFlag)
	defer cancel()

	artifact, err := buildArtifact(ctx, *recipeFlag, *pkgFlag, *verFlag, log)
	if err != nil {
		return err
	}
	defer os.Remove(artifact.LocalPath)

	format := sbom.Format(*formatFlag)
	var w *os.File
	if *outFlag == "" {
		w = os.Stdout
	} else {
		w, err = os.Create(*outFlag)
		if err != nil {
			return fmt.Errorf("create output: %w", err)
		}
		defer w.Close()
	}

	log.Info("generating SBOM", "format", format)
	if err := sbom.Generate(artifact, format, w); err != nil {
		return fmt.Errorf("sbom: %w", err)
	}
	if *outFlag != "" {
		log.Info("SBOM written", "file", *outFlag)
	}
	return nil
}

// ── provenance ────────────────────────────────────────────────────────────────

func runProvenance(args []string, log *slog.Logger) error {
	fs := flag.NewFlagSet("provenance", flag.ExitOnError)
	recipeFlag := fs.String("recipe", "", "ecosystem recipe")
	pkgFlag := fs.String("package", "", "package name")
	verFlag := fs.String("version", "", "package version")
	outFlag := fs.String("out", "", "output file (default: stdout)")
	timeoutFlag := fs.Duration("timeout", 5*time.Minute, "total timeout")
	fs.Parse(args)

	if *recipeFlag == "" || *pkgFlag == "" || *verFlag == "" {
		return fmt.Errorf("--recipe, --package, and --version are required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeoutFlag)
	defer cancel()

	artifact, err := buildArtifact(ctx, *recipeFlag, *pkgFlag, *verFlag, log)
	if err != nil {
		return err
	}
	defer os.Remove(artifact.LocalPath)

	prov := signer.NewProvenance(artifact)
	data, err := json.MarshalIndent(prov, "", "  ")
	if err != nil {
		return err
	}
	return writeOutput(*outFlag, data)
}

// ── helpers ───────────────────────────────────────────────────────────────────

func buildArtifact(ctx context.Context, recipeName, pkg, version string, log *slog.Logger) (core.BuiltArtifact, error) {
	recipe, err := recipes.Get(recipeName)
	if err != nil {
		return core.BuiltArtifact{}, fmt.Errorf("recipe %q not found", recipeName)
	}
	src := core.SourceArtifact{
		Package: core.PackageVersion{
			Ecosystem: recipeName,
			Name:      pkg,
			Version:   version,
		},
	}
	log.Info("building artifact", "recipe", recipeName, "package", pkg, "version", version)
	artifact, err := recipe.Build(ctx, src, nil)
	if err != nil {
		return core.BuiltArtifact{}, fmt.Errorf("build: %w", err)
	}
	return artifact, nil
}

func writeOutput(path string, data []byte) error {
	if path == "" {
		os.Stdout.Write(data)
		fmt.Println()
		return nil
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func severityAtLeast(s, threshold core.Severity) bool {
	return severityOrd(s) >= severityOrd(threshold)
}

func severityOrd(s core.Severity) int {
	switch s {
	case core.SeverityCritical:
		return 4
	case core.SeverityHigh:
		return 3
	case core.SeverityMedium:
		return 2
	case core.SeverityLow:
		return 1
	case core.SeverityInformational:
		return 0
	default:
		return -1 // empty/unknown severity never passes a threshold filter
	}
}

// ── update ────────────────────────────────────────────────────────────────────

const (
	// signaturesReleaseURL is the community signatures bundle — now served from
	// the forgeguardian-signatures repo. `fgctl update` is an alias for `fgctl intel update`.
	signaturesReleaseURL = communitySignaturesURL
	firstRunSentinel     = ".forgeguardian/.initialized"
)

// runUpdate downloads the latest community signatures from the GitHub release.
func runUpdate(args []string, log *slog.Logger) error {
	fs := flag.NewFlagSet("update", flag.ExitOnError)
	storeFlag := fs.String("store", "", "signature store path (default: ~/.forgeguardian/signatures.json)")
	_ = fs.Parse(args)

	storePath := *storeFlag
	if storePath == "" {
		var err error
		storePath, err = intelligence.DefaultStorePath()
		if err != nil {
			return err
		}
	}

	fmt.Println("Downloading latest community signatures...")

	resp, err := http.Get(signaturesReleaseURL)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("download failed: HTTP 404 — %s doesn't have a signatures.json release yet (or the repo isn't public)", communityRepoURL)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: HTTP %d fetching %s", resp.StatusCode, signaturesReleaseURL)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	var incoming intelligence.SignatureStore
	if err := json.Unmarshal(body, &incoming); err != nil {
		return fmt.Errorf("parse signatures: %w", err)
	}

	existing, err := intelligence.LoadStore(storePath)
	if err != nil {
		return err
	}

	before := len(existing.Signatures)
	added := intelligence.MergeSignatures(existing, incoming.Signatures)

	if err := intelligence.SaveStore(storePath, existing); err != nil {
		return err
	}

	total := len(existing.Signatures)
	if added == 0 {
		fmt.Printf("Already up to date. %d signatures total.\n", total)
	} else {
		fmt.Printf("Added %d new signatures (%d → %d total).\n", added, before, total)
	}
	fmt.Printf("Stored at: %s\n", storePath)

	// Mark initialized so first-run prompt doesn't re-appear.
	markInitialized()
	return nil
}

// ── sig ───────────────────────────────────────────────────────────────────────

// runSig handles: fgctl sig validate <file>
func runSig(args []string, log *slog.Logger) error {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: fgctl sig <subcommand> [flags]")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Subcommands:")
		fmt.Fprintln(os.Stderr, "  validate <file>   Validate a signature YAML/JSON file before submitting")
		return nil
	}
	switch args[0] {
	case "validate":
		return runSigValidate(args[1:], log)
	default:
		return fmt.Errorf("unknown subcommand %q — try: fgctl sig validate <file>", args[0])
	}
}

func runSigValidate(args []string, log *slog.Logger) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: fgctl sig validate <signature-file.json>")
	}

	path := args[0]
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("cannot read %s: %w", path, err)
	}

	var sig intelligence.DetectionSignature
	if err := json.Unmarshal(data, &sig); err != nil {
		printValidationFail(path, "file is not valid JSON: "+err.Error())
		os.Exit(2)
	}

	var problems []string

	if sig.ID == "" {
		problems = append(problems, "missing required field: id")
	}
	if sig.Type == "" {
		problems = append(problems, "missing required field: type")
	} else {
		valid := map[intelligence.SignatureType]bool{
			intelligence.SigTypoTarget:     true,
			intelligence.SigMalwarePattern: true,
			intelligence.SigBlocklisted:    true,
			intelligence.SigBehavioral:     true,
			intelligence.SigMCPInjection:   true,
			intelligence.SigPickleRule:     true,
		}
		if !valid[sig.Type] {
			problems = append(problems, fmt.Sprintf(
				"invalid type %q — must be one of: typosquat_target, malware_pattern, blocklisted_package, behavioral_rule, mcp_injection_pattern, pickle_rule",
				sig.Type,
			))
		}
	}
	if sig.Ecosystem == "" {
		problems = append(problems, "missing required field: ecosystem (use * for all ecosystems)")
	}
	if sig.Severity == "" {
		problems = append(problems, "missing required field: severity (critical|high|medium|low|informational)")
	} else {
		validSev := map[string]bool{"critical": true, "high": true, "medium": true, "low": true, "informational": true}
		if !validSev[strings.ToLower(sig.Severity)] {
			problems = append(problems, fmt.Sprintf("invalid severity %q — must be: critical, high, medium, low, or informational", sig.Severity))
		}
	}
	if sig.Title == "" {
		problems = append(problems, "missing required field: title")
	}
	if sig.Description == "" {
		problems = append(problems, "missing required field: description")
	}

	// Type-specific field checks
	switch sig.Type {
	case intelligence.SigBlocklisted:
		if sig.Package == "" {
			problems = append(problems, "blocklisted_package signature requires field: package")
		}
	case intelligence.SigTypoTarget:
		if sig.Target == "" {
			problems = append(problems, "typosquat_target signature requires field: target (the legitimate package name)")
		}
	case intelligence.SigMalwarePattern:
		if sig.Pattern == "" {
			problems = append(problems, "malware_pattern signature requires field: pattern")
		}
	case intelligence.SigBehavioral, intelligence.SigMCPInjection, intelligence.SigPickleRule:
		if sig.Rule == "" {
			problems = append(problems, fmt.Sprintf("%s signature requires field: rule (a regex pattern)", sig.Type))
		}
	}

	if len(problems) > 0 {
		printValidationFail(path, "")
		for _, p := range problems {
			fmt.Fprintf(os.Stderr, "  ✗  %s\n", p)
		}
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Fix the issues above and run fgctl sig validate again.")
		fmt.Fprintln(os.Stderr, "See SIGNATURES.md for the full field reference.")
		os.Exit(2)
	}

	fmt.Printf("✓  %s — valid %s signature\n", path, sig.Type)
	fmt.Printf("   ID:        %s\n", sig.ID)
	fmt.Printf("   Ecosystem: %s\n", sig.Ecosystem)
	fmt.Printf("   Severity:  %s\n", sig.Severity)
	fmt.Printf("   Title:     %s\n", sig.Title)
	fmt.Println()
	fmt.Println("Ready to submit. Open a pull request with:")
	fmt.Printf("  git checkout -b sig/%s\n", sig.ID)
	fmt.Printf("  git add %s\n", path)
	fmt.Printf("  git commit -m \"[sig] %s — %s\"\n", sig.ID, sig.Title)
	fmt.Println("  git push origin sig/" + sig.ID)
	return nil
}

func printValidationFail(path, msg string) {
	fmt.Fprintf(os.Stderr, "✗  %s — validation failed\n", path)
	if msg != "" {
		fmt.Fprintf(os.Stderr, "   %s\n", msg)
	}
	fmt.Fprintln(os.Stderr, "")
}

// ── first-run onboarding ──────────────────────────────────────────────────────

func firstRunCheck(log *slog.Logger) {
	// Skip for non-interactive commands and if already initialized.
	if isInitialized() {
		return
	}
	// Only show prompt for scan/advisory/sbom commands (not update/sig/help).
	cmd := ""
	if len(os.Args) >= 2 {
		cmd = os.Args[1]
	}
	interactive := map[string]bool{"scan": true, "advisory": true, "sbom": true, "sign": true}
	if !interactive[cmd] {
		return
	}

	storePath, err := intelligence.DefaultStorePath()
	if err != nil {
		return
	}
	store, err := intelligence.LoadStoreWithLocalFallback(storePath)
	if err != nil || len(store.Signatures) > 0 {
		// Either error reading (skip silently) or signatures already exist
		// (including via the local signatures/ dir fallback — a git clone
		// already has real signatures, no need to nag about fgctl update).
		if err == nil {
			markInitialized()
		}
		return
	}

	// No signatures found — prompt.
	fmt.Println()
	fmt.Println("┌─────────────────────────────────────────────────────┐")
	fmt.Println("│  Welcome to ForgeGuardian!                          │")
	fmt.Println("│                                                     │")
	fmt.Println("│  Download community detection signatures now?       │")
	fmt.Println("│  These improve scan accuracy for known malicious    │")
	fmt.Println("│  packages, typosquatting attacks, and more.         │")
	fmt.Println("│                                                     │")
	fmt.Println("│  You can always run: fgctl update                   │")
	fmt.Println("└─────────────────────────────────────────────────────┘")
	fmt.Print("Download now? [Y/n] ")

	reader := bufio.NewReader(os.Stdin)
	answer, _ := reader.ReadString('\n')
	answer = strings.TrimSpace(strings.ToLower(answer))

	if answer == "" || answer == "y" || answer == "yes" {
		if err := runUpdate(nil, log); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not download signatures: %v\n", err)
			fmt.Fprintln(os.Stderr, "Continuing without community signatures. Run 'fgctl update' later.")
		}
	} else {
		fmt.Println("Skipping. Run 'fgctl update' at any time to download community signatures.")
		markInitialized()
	}
	fmt.Println()
}

// ── local scan helpers ────────────────────────────────────────────────────────

// localScanOpts holds all options for a local project scan.
type localScanOpts struct {
	jsonMode    bool
	failOn      string
	workers     int
	timeout     time.Duration
	compact     bool
	summaryOnly bool
	quiet       bool
	severity    string // min severity filter
	ecosystem   string // ecosystem filter
	onlyFixable bool
	prodOnly    bool
	verbose     bool
	debug       bool
	format      string // "text" | "json" | "sarif"
}

// isLocalPath returns true for ".", "..", paths starting with "./" "../" "/" or "~".
func isLocalPath(s string) bool {
	return s == "." || s == ".." ||
		strings.HasPrefix(s, "./") || strings.HasPrefix(s, "../") ||
		strings.HasPrefix(s, "/") || strings.HasPrefix(s, "~")
}

// isDotNotation returns true for "npm/lodash@4.17.20" style arguments.
func isDotNotation(s string) bool {
	return strings.Contains(s, "/") && strings.Contains(s, "@")
}

// parseDotNotation parses "npm/lodash@4.17.20" → ("npm", "lodash", "4.17.20").
func parseDotNotation(s string) (eco, pkg, ver string) {
	slash := strings.Index(s, "/")
	at := strings.LastIndex(s, "@")
	if slash < 0 || at < 0 || at <= slash {
		return "", s, ""
	}
	return s[:slash], s[slash+1 : at], s[at+1:]
}

// runLocalScan scans a local project directory and prints results.
func runLocalScan(rootDir string, opts localScanOpts, log *slog.Logger, p *ui.Printer) error {
	ctx, cancel := context.WithTimeout(context.Background(), opts.timeout)
	defer cancel()

	storePath, _ := signaturesStorePath()
	ls := localscanner.New(storePath)
	ls.Workers = opts.workers
	ls.ProdOnly = opts.prodOnly

	scanStart := time.Now()
	sp := p.Spinner("Scanning project...")
	result, err := ls.Scan(ctx, rootDir, func(done, total int) {
		if sp != nil {
			sp.Suffix = fmt.Sprintf("  Scanning %d/%d packages...", done, total)
		}
	})
	ui.StopSpinner(sp)
	if err != nil {
		return fmt.Errorf("local scan: %w", err)
	}

	return printLocalScanResult(result, opts, p, scanStart, result.RootDir)
}

// printLocalScanResult filters, formats, and prints a completed
// ProjectScanResult. Shared by the local-path scan (runLocalScan) and the
// --remote scan (runRemoteScan) so both produce identical output — the only
// difference between them is displayLabel, since a remote scan's temp-dir
// path on the local filesystem is meaningless to show the user in place of
// the actual "user@host:/remote/path" they scanned.
func printLocalScanResult(result *localscanner.ProjectScanResult, opts localScanOpts, p *ui.Printer, scanStart time.Time, displayLabel string) error {
	// Apply filters to results before display.
	filteredResults := filterResults(result.Results, opts)
	filteredSummary := buildSummary(filteredResults)

	if opts.format == "json" || opts.jsonMode {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}
	if opts.format == "sarif" {
		var allFindings []core.Finding
		for _, r := range filteredResults {
			allFindings = append(allFindings, r.Findings...)
		}
		return ui.WriteSARIF(os.Stdout, displayLabel, "", allFindings, filteredSummary, version)
	}

	if opts.quiet {
		return checkFailOn(filteredResults, opts.failOn)
	}

	if opts.summaryOnly {
		p.PrintSummaryTable(countScanned(result.Results), filteredSummary)
		p.PrintMissingToolsWarning(result.MissingTools)
		return checkFailOn(filteredResults, opts.failOn)
	}

	p.PrintLocalScanHeader(displayLabel)
	p.PrintMissingToolsWarning(result.MissingTools)

	// Group results by manifest file.
	if opts.compact {
		p.PrintCompactGrouped(filteredResults)
		fmt.Fprintln(os.Stdout)
	} else {
		p.PrintGroupedFindings(filteredResults)
	}

	p.PrintScanSummary(filteredSummary)
	if !opts.quiet && opts.format == "text" {
		color.New(color.FgHiBlack).Fprintf(os.Stdout, "  Completed in %.1fs\n", time.Since(scanStart).Seconds())
	}
	if filteredSummary.Total > 0 {
		eco, name := guessFirstVulnEcoAndName(filteredResults)
		fmt.Fprintf(os.Stdout, "  Run 'fgctl advisory %s/%s@<version>' for AI remediation.\n\n", eco, name)
	} else {
		p.Success("No findings across %d package(s).", countScanned(result.Results))
	}

	return checkFailOn(filteredResults, opts.failOn)
}

// filterResults applies severity, ecosystem, and fixability filters to scan results.
func filterResults(results []localscanner.LocalScanResult, opts localScanOpts) []localscanner.LocalScanResult {
	threshold := core.Severity(strings.ToUpper(opts.severity))
	var out []localscanner.LocalScanResult
	for _, r := range results {
		if opts.ecosystem != "" && r.Entry.Ecosystem != opts.ecosystem {
			continue
		}
		var kept []core.Finding
		for _, f := range r.Findings {
			// Hide informational findings by default; show with --verbose or --severity=informational.
			if f.Severity == core.SeverityInformational &&
				!opts.verbose &&
				strings.ToUpper(opts.severity) != "INFORMATIONAL" {
				continue
			}
			// Always exclude informational findings when a severity filter is active,
			// unless the user explicitly requested informational level.
			if opts.severity != "" {
				if string(f.Severity) == "INFORMATIONAL" &&
					strings.ToUpper(opts.severity) != "INFORMATIONAL" {
					continue
				}
				if !severityAtLeast(f.Severity, threshold) {
					continue
				}
			}
			// --only-fixable: informational findings never have a fix version.
			if opts.onlyFixable && (f.FixedVersion == "" || string(f.Severity) == "INFORMATIONAL") {
				continue
			}
			kept = append(kept, f)
		}
		if len(kept) > 0 || r.Skipped {
			nr := r
			nr.Findings = kept
			out = append(out, nr)
		}
	}
	return out
}

func buildSummary(results []localscanner.LocalScanResult) scanner.ScanSummary {
	var all []core.Finding
	for _, r := range results {
		all = append(all, r.Findings...)
	}
	return scanner.Summarize(all)
}

func countScanned(results []localscanner.LocalScanResult) int {
	n := 0
	for _, r := range results {
		if !r.Skipped {
			n++
		}
	}
	return n
}

func checkFailOn(results []localscanner.LocalScanResult, failOn string) error {
	if failOn == "" {
		return nil
	}
	threshold := core.Severity(strings.ToUpper(failOn))
	for _, r := range results {
		for _, f := range r.Findings {
			if severityAtLeast(f.Severity, threshold) {
				os.Exit(2)
			}
		}
	}
	return nil
}

// guessFirstVulnEcoAndName returns the ecosystem and name of the first
// finding for use in the follow-up advisory hint.
func guessFirstVulnEcoAndName(results []localscanner.LocalScanResult) (eco, name string) {
	for _, r := range results {
		if len(r.Findings) > 0 {
			return r.Entry.Ecosystem, r.Entry.Name
		}
	}
	return "npm", "package"
}

// signaturesStorePath returns the default path to the local signatures store.
func signaturesStorePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return home + "/.forgeguardian/signatures.json", nil
}

func isInitialized() bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return true // don't prompt if we can't determine home
	}
	_, err = os.Stat(home + "/" + firstRunSentinel)
	return err == nil
}

func markInitialized() {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	path := home + "/" + firstRunSentinel
	_ = os.MkdirAll(strings.TrimSuffix(path, "/.initialized"), 0o700)
	_ = os.WriteFile(path, []byte("1"), 0o600)
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `ForgeGuardian — Local-first AI-native Software Supply Chain Security Platform  (%s)

  Protect your software supply chain: scan dependencies, detect malicious packages,
  generate SBOMs, sign artifacts, and autonomously patch vulnerabilities — all
  running locally, with no mandatory cloud, no telemetry, no data exfiltration.

Usage: fgctl <command> [flags]

DETECTION & ANALYSIS
  scan        Multi-engine vulnerability + supply chain risk scan
  advisory    AI-powered security advisory  (requires ANTHROPIC_API_KEY)
  audit       Audit globally installed packages across all package managers

SUPPLY CHAIN INTEGRITY
  sbom        Generate CycloneDX / SPDX Software Bill of Materials
  sign        Sigstore keyless artifact signing (SLSA provenance)
  verify      Verify artifact attestation against Rekor log
  provenance  Generate SLSA v1.0 provenance statement

REMEDIATION & MONITORING
  patch       Autonomous AI-powered dependency patching  (requires ANTHROPIC_API_KEY)
  monitor     Watch manifest files for new vulnerabilities

PLATFORM MANAGEMENT
  setup       Interactive setup — configure dashboard credentials + .env
  policy      Manage security policy  (~/.forgeguardian/policy.yaml)
  config      Manage platform configuration  (~/.forgeguardian/config.yaml)
  update      Fetch latest community detection signatures
  intel       Author, validate, test, and update detection signatures  (new|validate|test|update|list)
  license     Print current license tier  (set via FG_LICENSE_KEY)
  stats       Runtime statistics — loaded signatures, ecosystems, engine state  (--json)
  doctor      Self-diagnostic + environment repair  (--fix)
  debug       Collect diagnostic info for bug reports
  sig         Community signature utilities  (sig validate <file>)
  version     Print version information

Common flags  (scan / advisory / sbom / sign / provenance):
  --recipe=<npm|pypi|maven|go|rubygems|crates|huggingface|mcp>
  --package=<name>
  --version=<version>
  --timeout=<duration>   (default varies per command)

Scan flags:
  --fail-on=<critical|high|medium|low>  exit 2 if findings meet threshold
  --format=<text|json|sarif>            output format (default: text)
  --compact                             one line per finding
  --summary                             severity table only
  --quiet                               suppress output, exit code only
  --severity=<level>                    filter to this severity and above
  --only-fixable                        show only findings with a known fix
  --verbose                             expand all grouped findings
  --debug                               show engine errors and raw scan metadata
  --prod-only / --exclude-dev           skip dev dependencies
  --workers=<n>                         concurrent workers for local project scan (default: 4)
  --ecosystem=<npm|pypi|go|...>         filter local scan to one ecosystem
  --ci                                  CI mode: quiet + SARIF + fail-on=high
  --executive                           severity table summary only
  --no-banner                           suppress ASCII banner
  --no-color                            disable ANSI colors

Remote scan flags  (scan --remote user@host):
  --remote=<user@host[:port]>           scan manifests on a remote host over SSH
  --remote-port=<port>                  SSH port (default: 22)
  --identity=<path>                     SSH private key file (else ssh-agent / ~/.ssh defaults)
  --remote-path=<path>                  remote directory to search (default: remote $HOME)
  --remote-max-depth=<n>                directory recursion depth for manifest discovery
  --accept-new-host-key                 trust a host not yet in ~/.ssh/known_hosts (key mismatches always hard-fail regardless)
  --keep-temp                           keep the local temp dir the pulled manifests were scanned in

Examples:
  fgctl setup                                           # configure credentials + .env
  fgctl doctor --fix                                    # validate + auto-repair
  fgctl scan .                                          # scan current project
  fgctl scan npm/lodash@4.17.20                         # scan specific package
  fgctl scan . --fail-on=high --format=sarif            # CI policy gate
  fgctl scan --remote deploy@10.0.4.12 --accept-new-host-key  # scan a remote host over SSH
  fgctl advisory npm/lodash@4.17.20                     # AI advisory
  fgctl patch .                                         # autonomous patching
  fgctl monitor --watch .                               # continuous monitoring
  fgctl policy init && fgctl policy set fail_on=high    # configure policy
  fgctl audit system                                    # audit global packages
  fgctl sbom .                                          # project SBOM (all manifests)
  fgctl sbom npm/chalk@5.3.0 --format=cyclonedx-json   # generate SBOM
  fgctl sign --recipe=npm --package=lodash --version=4.17.21 --out=att.json
  fgctl verify --attestation=att.json --sha256=<hex>
  fgctl update                                          # fetch signatures
  fgctl intel new                                       # author a detection signature
  fgctl sig validate ./my-signature.json
  fgctl license                                         # show current license tier
`, version)
}

// ── local SBOM ────────────────────────────────────────────────────────────────

// runLocalSBOM generates a project SBOM from all manifest files under rootDir.
func runLocalSBOM(rootDir, format, outPath string, log *slog.Logger) error {
	abs, err := filepath.Abs(rootDir)
	if err != nil {
		return fmt.Errorf("sbom: resolve path: %w", err)
	}
	manifests, err := localscanner.Walk(abs)
	if err != nil {
		return fmt.Errorf("sbom: walk: %w", err)
	}
	if len(manifests) == 0 {
		return fmt.Errorf("sbom: no manifest files found under %s", abs)
	}

	var totalEntries int
	for _, mf := range manifests {
		entries, err := localscanner.ParseManifest(mf.Path)
		if err != nil {
			log.Warn("sbom: skipping manifest", "path", mf.Path, "err", err)
			continue
		}
		totalEntries += len(entries)
	}

	projectName := filepath.Base(abs)
	rootArtifact := core.BuiltArtifact{
		Source: core.SourceArtifact{
			Package: core.PackageVersion{
				Ecosystem: "project",
				Name:      projectName,
				Version:   "0.0.0",
			},
		},
		BuildLog: fmt.Sprintf("components: %d\nmanifests: %d", totalEntries, len(manifests)),
	}

	var w io.Writer = os.Stdout
	if outPath != "" {
		f, err := os.Create(outPath)
		if err != nil {
			return fmt.Errorf("sbom: create output: %w", err)
		}
		defer f.Close()
		w = f
	}

	if err := sbom.Generate(rootArtifact, sbom.Format(format), w); err != nil {
		return fmt.Errorf("sbom: %w", err)
	}
	log.Info("project SBOM generated", "root", abs, "components", totalEntries,
		"manifests", len(manifests), "format", format)
	return nil
}
