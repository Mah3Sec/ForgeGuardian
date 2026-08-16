// commands.go contains implementations for: patch, monitor, audit, debug, config.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/fatih/color"
	"gopkg.in/yaml.v3"

	"github.com/mah3sec/forgeguardian/internal/core"
	"github.com/mah3sec/forgeguardian/internal/intelligence"
	"github.com/mah3sec/forgeguardian/internal/localscanner"
	"github.com/mah3sec/forgeguardian/internal/policy"
	"github.com/mah3sec/forgeguardian/internal/scanner"
	"github.com/mah3sec/forgeguardian/internal/ui"
)

// ── patch ─────────────────────────────────────────────────────────────────────

// runPatch delegates to the fg-agent binary with --apply.
// Usage: fgctl patch [--project-dir=.] [ecosystem/pkg@ver]
func runPatch(args []string, log *slog.Logger) error {
	fs := flag.NewFlagSet("patch", flag.ExitOnError)
	dirFlag := fs.String("project-dir", ".", "directory containing manifest files")
	fs.Parse(args)

	agentBin := findAgentBinary()
	if agentBin == "" {
		return fmt.Errorf("fg-agent not found in PATH or bin/ — run: go install github.com/mah3sec/forgeguardian/cmd/fg-agent@latest")
	}

	agentArgs := []string{"--apply", "--project-dir=" + *dirFlag}

	// If a dot-notation arg was given, forward recipe/package/version.
	remaining := fs.Args()
	if len(remaining) > 0 && isDotNotation(remaining[0]) {
		eco, pkg, ver := parseDotNotation(remaining[0])
		agentArgs = append(agentArgs,
			"--recipe="+eco,
			"--package="+pkg,
			"--version="+ver,
		)
	}

	log.Info("delegating to fg-agent", "args", agentArgs)
	cmd := exec.Command(agentBin, agentArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func findAgentBinary() string {
	// Check PATH first.
	if p, err := exec.LookPath("fg-agent"); err == nil {
		return p
	}
	// Fall back to bin/ relative to the executable.
	exe, _ := os.Executable()
	local := filepath.Join(filepath.Dir(exe), "fg-agent")
	if _, err := os.Stat(local); err == nil {
		return local
	}
	// Check ./bin/ in working directory (common during development).
	if _, err := os.Stat("./bin/fg-agent"); err == nil {
		return "./bin/fg-agent"
	}
	return ""
}

// ── monitor ───────────────────────────────────────────────────────────────────

// runMonitor watches manifest files for changes and re-scans on each change.
// Uses polling (fsnotify-free) with a 3s interval — avoids adding a new dep.
func runMonitor(args []string, log *slog.Logger, p *ui.Printer) error {
	fs := flag.NewFlagSet("monitor", flag.ExitOnError)
	watchFlag := fs.Bool("watch", false, "watch for manifest changes and re-scan")
	dirFlag := fs.String("dir", ".", "directory to monitor")
	intervalFlag := fs.Duration("interval", 3*time.Second, "polling interval for --watch mode")
	workersFlag := fs.Int("workers", 4, "scan workers")
	timeoutFlag := fs.Duration("timeout", 5*time.Minute, "scan timeout per cycle")
	fs.Parse(args)

	dir := *dirFlag
	if len(fs.Args()) > 0 {
		dir = fs.Args()[0]
	}

	if !*watchFlag {
		// One-shot: just scan and exit.
		opts := localScanOpts{workers: *workersFlag, timeout: *timeoutFlag}
		return runLocalScan(dir, opts, log, p)
	}

	p.Banner()
	fmt.Printf("  Watching %s for manifest changes (poll interval: %s)...\n", dir, *intervalFlag)
	fmt.Printf("  Press Ctrl+C to stop.\n\n")

	storePath, _ := signaturesStorePath()
	ls := localscanner.New(storePath)
	ls.Workers = *workersFlag

	// Collect baseline manifest mtimes.
	mtimes := manifestMtimes(dir)

	// Run initial scan.
	prevFindings := map[string]core.Finding{}
	ctx, cancel := context.WithTimeout(context.Background(), *timeoutFlag)
	result, err := ls.Scan(ctx, dir, nil)
	cancel()
	if err == nil {
		for _, r := range result.Results {
			for _, f := range r.Findings {
				prevFindings[r.Entry.Ecosystem+"/"+r.Entry.Name+"@"+r.Entry.Version+":"+f.ID] = f
			}
		}
		ts := time.Now().Format("15:04:05")
		crit := result.Summary.Critical
		high := result.Summary.High
		line := fmt.Sprintf("  [%s] Initial scan: %d finding(s)", ts, result.Summary.Total)
		if crit > 0 || high > 0 {
			line += fmt.Sprintf(" (%d CRIT, %d HIGH)", crit, high)
		}
		fmt.Fprintln(p.Out, line)
	}

	for {
		time.Sleep(*intervalFlag)

		newMtimes := manifestMtimes(dir)
		changed := ""
		for path, mtime := range newMtimes {
			if prev, ok := mtimes[path]; !ok || !mtime.Equal(prev) {
				changed = filepath.Base(path)
				break
			}
		}
		mtimes = newMtimes

		if changed == "" {
			continue
		}

		fmt.Printf("  [%s] %s changed — rescanning...\n", time.Now().Format("15:04:05"), changed)

		ctx2, cancel2 := context.WithTimeout(context.Background(), *timeoutFlag)
		newResult, err := ls.Scan(ctx2, dir, nil)
		cancel2()
		if err != nil {
			fmt.Printf("  [%s] scan error: %v\n", time.Now().Format("15:04:05"), err)
			continue
		}

		// Diff: new vs previous findings.
		newFindings := map[string]core.Finding{}
		for _, r := range newResult.Results {
			for _, f := range r.Findings {
				newFindings[r.Entry.Ecosystem+"/"+r.Entry.Name+"@"+r.Entry.Version+":"+f.ID] = f
			}
		}

		added := 0
		for key, f := range newFindings {
			if _, existed := prevFindings[key]; !existed {
				// Parse package identifier from key "eco/name@ver:findingID".
				pkgLabel := ""
				if parts := strings.SplitN(key, ":", 2); len(parts) == 2 {
					pkgLabel = " (" + parts[0] + ")"
				}
				sev := ui.SeverityColor(f.Severity)
				fmt.Printf("  [%s] NEW %s%s  %s\n",
					time.Now().Format("15:04:05"),
					sev.Sprintf("[%s]", f.Severity),
					pkgLabel,
					f.Title)
				added++
			}
		}
		resolved := 0
		for key, f := range prevFindings {
			if _, still := newFindings[key]; !still {
				fmt.Printf("  [%s] RESOLVED  %s\n", time.Now().Format("15:04:05"), f.Title)
				resolved++
			}
		}

		ts := time.Now().Format("15:04:05")
		if added == 0 && resolved == 0 {
			color.New(color.FgWhite).Fprintf(p.Out, "  [%s] No changes.\n", ts)
		} else {
			var deltaParts []string
			if added > 0 {
				deltaParts = append(deltaParts, color.New(color.FgRed).Sprintf("%d new", added))
			}
			if resolved > 0 {
				deltaParts = append(deltaParts, color.New(color.FgGreen).Sprintf("%d resolved", resolved))
			}
			fmt.Fprintf(p.Out, "  [%s] %s\n", ts, strings.Join(deltaParts, "  "))
		}

		prevFindings = newFindings
	}
}

// manifestMtimes returns a map of manifest file path → mtime.
func manifestMtimes(rootDir string) map[string]time.Time {
	manifests, _ := localscanner.Walk(rootDir)
	m := make(map[string]time.Time, len(manifests))
	for _, mf := range manifests {
		if info, err := os.Stat(mf.Path); err == nil {
			m[mf.Path] = info.ModTime()
		}
	}
	return m
}

// ── audit ─────────────────────────────────────────────────────────────────────

type auditSource struct {
	name      string
	ecosystem string
	parse     func() []localscanner.ManifestEntry
}

// ecoAuditResult holds per-ecosystem scan results for deferred display.
type ecoAuditResult struct {
	label       string
	scanned     int
	pkgFindings []struct {
		name     string
		version  string
		findings []core.Finding
	}
}

// runAudit enumerates globally installed packages and scans them.
// Usage: fgctl audit system
func runAudit(args []string, log *slog.Logger, p *ui.Printer) error {
	if len(args) == 0 || args[0] != "system" {
		fmt.Fprintln(os.Stderr, "Usage: fgctl audit system")
		return nil
	}

	p.Banner()
	fmt.Println("  Auditing globally installed packages...")

	sources := []auditSource{
		{"npm (global)", "npm", auditNPMGlobals},
		{"Python (pip)", "pypi", auditPipGlobals},
		{"Cargo bins", "crates", auditCargoBins},
		{"Go binaries", "go", auditGoBins},
		{"Homebrew", "brew", auditBrewPackages},
		{"Ruby gems", "rubygems", auditGemPackages},
		{"Docker images", "docker", auditDockerImages},
	}

	ctx := context.Background()
	storePath, _ := signaturesStorePath()
	orch := scanner.NewWithIntelligence(storePath)

	totalScanned := 0
	var allFindings []core.Finding
	var ecoResults []*ecoAuditResult

	for _, src := range sources {
		entries := src.parse()
		if len(entries) == 0 {
			continue
		}

		er := &ecoAuditResult{label: src.name}

		for _, e := range entries {
			if e.Version == "" {
				// Count skipped packages as scanned for accurate totals.
				er.scanned++
				continue
			}
			art := core.BuiltArtifact{
				Source: core.SourceArtifact{
					Package: core.PackageVersion{
						Ecosystem: e.Ecosystem,
						Name:      e.Name,
						Version:   e.Version,
					},
				},
				BuildLog: "install_scripts: []\nnetwork_connections: 0\nfiles: []",
			}
			results := orch.Scan(ctx, art)
			findings := scanner.MergeFindings(results)

			// Strip INFORMATIONAL and TOOL-NOT-INSTALLED noise.
			var real []core.Finding
			for _, f := range findings {
				if f.ID != "TOOL-NOT-INSTALLED" && f.Severity != core.SeverityInformational {
					real = append(real, f)
				}
			}

			er.scanned++
			totalScanned++
			allFindings = append(allFindings, real...)

			er.pkgFindings = append(er.pkgFindings, struct {
				name     string
				version  string
				findings []core.Finding
			}{e.Name, e.Version, real})
		}

		ecoResults = append(ecoResults, er)
	}

	// Print one summary line per ecosystem.
	fmt.Println()
	for _, er := range ecoResults {
		total := 0
		crit, high, med, low := 0, 0, 0, 0
		for _, pf := range er.pkgFindings {
			for _, f := range pf.findings {
				total++
				switch f.Severity {
				case core.SeverityCritical:
					crit++
				case core.SeverityHigh:
					high++
				case core.SeverityMedium:
					med++
				case core.SeverityLow:
					low++
				}
			}
		}

		line := fmt.Sprintf("  %-22s %4d scanned", er.label, er.scanned)
		if total == 0 {
			line += color.New(color.FgGreen).Sprint("   ✓ clean")
		} else {
			line += fmt.Sprintf("   %d finding(s)", total)
			if crit > 0 {
				line += color.New(color.FgRed, color.Bold).Sprintf(" [%d CRIT]", crit)
			}
			if high > 0 {
				line += color.New(color.FgHiRed).Sprintf(" [%d HIGH]", high)
			}
			if med > 0 {
				line += color.New(color.FgYellow).Sprintf(" [%d MED]", med)
			}
			if low > 0 {
				line += color.New(color.FgCyan).Sprintf(" [%d LOW]", low)
			}
		}
		fmt.Fprintln(p.Out, line)

		// Verbose: show per-package details for packages with findings only.
		if p.Verbose {
			for _, pf := range er.pkgFindings {
				if len(pf.findings) == 0 {
					continue
				}
				fmt.Fprintf(p.Out, "    %s@%s\n", pf.name, pf.version)
				for _, f := range pf.findings {
					sev := ui.SeverityColor(f.Severity)
					fmt.Fprintf(p.Out, "      %s  %s\n", sev.Sprintf("[%-8s]", string(f.Severity)), f.Title)
				}
			}
		}
	}

	// PATH hijack check
	if hijackDirs := checkPATHHijack(); len(hijackDirs) > 0 {
		color.New(color.FgRed, color.Bold).Fprintf(p.Out, "\n  [WARN] PATH hijack risk — world-writable directories:\n")
		for _, d := range hijackDirs {
			fmt.Fprintf(p.Out, "    %s\n", d)
		}
	} else {
		color.New(color.FgGreen).Fprintf(p.Out, "\n  ✓  PATH: no world-writable directories\n")
	}

	fmt.Println()
	sum := scanner.Summarize(allFindings)
	fmt.Printf("  System Audit Summary: %d packages audited · %d finding(s)", totalScanned, sum.Total)
	if sum.Critical > 0 {
		fmt.Printf(" (%d CRITICAL)", sum.Critical)
	}
	if sum.High > 0 {
		fmt.Printf(" (%d HIGH)", sum.High)
	}
	fmt.Println()
	return nil
}

func auditNPMGlobals() []localscanner.ManifestEntry {
	out, err := exec.Command("npm", "-g", "list", "--json", "--depth=0").Output()
	if err != nil || len(out) == 0 {
		return nil
	}
	var report struct {
		Dependencies map[string]struct{ Version string } `json:"dependencies"`
	}
	if err := json.Unmarshal(out, &report); err != nil {
		return nil
	}
	var entries []localscanner.ManifestEntry
	for name, info := range report.Dependencies {
		entries = append(entries, localscanner.ManifestEntry{
			Ecosystem: "npm",
			Name:      name,
			Version:   strings.TrimPrefix(info.Version, "v"),
		})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
	return entries
}

func auditPipGlobals() []localscanner.ManifestEntry {
	out, err := exec.Command("pip", "list", "--format=json").Output()
	if err == nil && len(out) == 0 {
		out, err = exec.Command("pip3", "list", "--format=json").Output()
	}
	if err != nil || len(out) == 0 {
		return nil
	}
	var pkgs []struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}
	if err := json.Unmarshal(out, &pkgs); err != nil {
		return nil
	}
	entries := make([]localscanner.ManifestEntry, 0, len(pkgs))
	for _, p := range pkgs {
		entries = append(entries, localscanner.ManifestEntry{
			Ecosystem: "pypi",
			Name:      p.Name,
			Version:   p.Version,
		})
	}
	return entries
}

func auditCargoBins() []localscanner.ManifestEntry {
	out, err := exec.Command("cargo", "install", "--list").Output()
	if err != nil || len(out) == 0 {
		return nil
	}
	var entries []localscanner.ManifestEntry
	for _, line := range strings.Split(string(out), "\n") {
		// Lines look like: "ripgrep v14.1.0:"
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, " ") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		name := parts[0]
		ver := strings.TrimSuffix(strings.TrimPrefix(parts[1], "v"), ":")
		entries = append(entries, localscanner.ManifestEntry{
			Ecosystem: "crates",
			Name:      name,
			Version:   ver,
		})
	}
	return entries
}

func auditGoBins() []localscanner.ManifestEntry {
	gopath := os.Getenv("GOPATH")
	if gopath == "" {
		out, err := exec.Command("go", "env", "GOPATH").Output()
		if err != nil {
			return nil
		}
		gopath = strings.TrimSpace(string(out))
	}
	binDir := filepath.Join(gopath, "bin")
	entries, err := os.ReadDir(binDir)
	if err != nil {
		return nil
	}
	var result []localscanner.ManifestEntry
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		// We can't reliably get the version of Go binaries without running them,
		// so we record them with an empty version (will show as [SKIP]).
		result = append(result, localscanner.ManifestEntry{
			Ecosystem: "go",
			Name:      e.Name(),
			Version:   "", // unknown
		})
	}
	return result
}

// auditBrewPackages discovers Homebrew-installed packages via `brew list --versions`.
// NOTE: CVE coverage for brew packages is limited; results reflect blocklist/typosquat matches only.
func auditBrewPackages() []localscanner.ManifestEntry {
	out, err := exec.Command("brew", "list", "--versions").Output()
	if err != nil || len(out) == 0 {
		return nil
	}
	var entries []localscanner.ManifestEntry
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		name := parts[0]
		ver := parts[len(parts)-1] // last token is most recent version
		entries = append(entries, localscanner.ManifestEntry{
			Ecosystem: "brew",
			Name:      name,
			Version:   ver,
		})
	}
	return entries
}

// auditGemPackages discovers system-installed Ruby gems via `gem list --local`.
func auditGemPackages() []localscanner.ManifestEntry {
	out, err := exec.Command("gem", "list", "--local").Output()
	if err != nil || len(out) == 0 {
		return nil
	}
	var entries []localscanner.ManifestEntry
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "*") || strings.HasPrefix(line, "LOCAL GEMS") {
			continue
		}
		// Format: "name (version1, version2, ...)"
		paren := strings.Index(line, " (")
		if paren < 0 {
			continue
		}
		name := strings.TrimSpace(line[:paren])
		verStr := strings.Trim(line[paren+2:], "()")
		versions := strings.Split(verStr, ", ")
		ver := strings.TrimSpace(versions[0])
		entries = append(entries, localscanner.ManifestEntry{
			Ecosystem: "rubygems",
			Name:      name,
			Version:   ver,
		})
	}
	return entries
}

// auditDockerImages discovers locally cached Docker images via `docker images`.
// NOTE: Docker image names are matched against blocklist/malware signatures only.
func auditDockerImages() []localscanner.ManifestEntry {
	out, err := exec.Command("docker", "images",
		"--format", "{{.Repository}}:{{.Tag}}",
	).Output()
	if err != nil || len(out) == 0 {
		return nil
	}
	var entries []localscanner.ManifestEntry
	seen := map[string]bool{}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.Contains(line, "<none>") || seen[line] {
			continue
		}
		seen[line] = true
		lastColon := strings.LastIndex(line, ":")
		name, tag := line, "latest"
		if lastColon > 0 {
			name = line[:lastColon]
			tag = line[lastColon+1:]
		}
		entries = append(entries, localscanner.ManifestEntry{
			Ecosystem: "docker",
			Name:      name,
			Version:   tag,
		})
	}
	return entries
}

// checkPATHHijack scans PATH directories for world-writable entries that could enable
// PATH hijacking attacks. Returns a list of vulnerable directory paths.
func checkPATHHijack() []string {
	pathEnv := os.Getenv("PATH")
	if pathEnv == "" {
		return nil
	}
	var writable []string
	for _, dir := range strings.Split(pathEnv, ":") {
		if dir == "" {
			continue
		}
		info, err := os.Stat(dir)
		if err != nil {
			continue
		}
		if info.Mode()&0o002 != 0 {
			writable = append(writable, dir)
		}
	}
	return writable
}

// ── debug ─────────────────────────────────────────────────────────────────────

// runDebug collects diagnostic info for bug reports.
func runDebug(args []string, log *slog.Logger, p *ui.Printer) error {
	fs := flag.NewFlagSet("debug", flag.ExitOnError)
	jsonFlag := fs.Bool("json", false, "output JSON")
	fs.Parse(args)

	home, _ := os.UserHomeDir()
	apiURL := os.Getenv("FORGEGUARDIAN_API_URL")
	if apiURL == "" {
		apiURL = "http://localhost:8080"
	}

	// Check API reachability.
	apiStatus := "NOT REACHABLE"
	client := &http.Client{Timeout: 2 * time.Second}
	if resp, err := client.Get(apiURL + "/healthz"); err == nil {
		resp.Body.Close()
		apiStatus = fmt.Sprintf("OK (%d)", resp.StatusCode)
	}

	// Signatures info.
	sigPath := filepath.Join(home, ".forgeguardian", "signatures.json")
	sigInfo := "not found — run: fgctl update"
	if info, err := os.Stat(sigPath); err == nil {
		data, _ := os.ReadFile(sigPath)
		var store struct {
			Signatures []any `json:"signatures"`
		}
		json.Unmarshal(data, &store)
		sigInfo = fmt.Sprintf("%d sigs, modified %s", len(store.Signatures), info.ModTime().Format("2006-01-02"))
	}

	// Config info.
	cfgPath := filepath.Join(home, ".forgeguardian", "config.yaml")
	cfgInfo := "not found"
	if info, err := os.Stat(cfgPath); err == nil {
		cfgInfo = fmt.Sprintf("exists (%d bytes)", info.Size())
	}

	// Tool checks.
	tools := map[string]string{}
	for _, t := range []string{"grype", "semgrep", "trivy"} {
		if p, err := exec.LookPath(t); err == nil {
			tools[t] = p
		} else {
			tools[t] = "not found"
		}
	}

	apiKeyStatus := "NOT SET"
	if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" {
		if len(key) > 10 {
			apiKeyStatus = "SET (" + key[:7] + "..." + key[len(key)-3:] + ")"
		} else {
			apiKeyStatus = "SET"
		}
	}

	diskFree := "unknown"
	if info, err := getDiskFreeGB(); err == nil {
		diskFree = fmt.Sprintf("%.1f GB", info)
	}

	if *jsonFlag {
		out := map[string]any{
			"version":           version,
			"commit":            commit,
			"build_time":        buildTime,
			"os":                runtime.GOOS,
			"arch":              runtime.GOARCH,
			"go_version":        runtime.Version(),
			"config":            cfgInfo,
			"signatures":        sigInfo,
			"api_url":           apiURL,
			"api_status":        apiStatus,
			"tools":             tools,
			"anthropic_api_key": apiKeyStatus,
			"disk_free":         diskFree,
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	}

	fmt.Printf("\nForgeGuardian Debug Report\n")
	fmt.Printf("%s\n", strings.Repeat("─", 44))
	fmt.Printf("  Version:          %s (%s)\n", version, commit)
	fmt.Printf("  Built:            %s\n", buildTime)
	fmt.Printf("  OS/Arch:          %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Printf("  Go runtime:       %s\n", runtime.Version())
	fmt.Printf("  Config:           %s\n", cfgInfo)
	fmt.Printf("  Signatures:       %s\n", sigInfo)
	fmt.Printf("  API:              %s — %s\n", apiURL, apiStatus)
	for t, loc := range tools {
		fmt.Printf("  %-18s%s\n", t+":", loc)
	}
	fmt.Printf("  ANTHROPIC_API_KEY:%s\n", apiKeyStatus)
	fmt.Printf("  Disk free:        %s\n", diskFree)
	fmt.Printf("%s\n\n", strings.Repeat("─", 44))
	return nil
}

func getDiskFreeGB() (float64, error) {
	return diskFreeGB()
}

// ── config ────────────────────────────────────────────────────────────────────

// FGConfig is the ForgeGuardian user configuration stored in ~/.forgeguardian/config.yaml.
type FGConfig struct {
	APIUrl  string    `yaml:"api_url"`
	Scan    ScanCfg   `yaml:"scan"`
	Signing SignCfg   `yaml:"signing"`
	Notify  NotifyCfg `yaml:"notify"`
}

// NotifyCfg holds webhook delivery settings for scan alerts.
type NotifyCfg struct {
	SlackWebhookURL   string `yaml:"slack_webhook_url"`
	DiscordWebhookURL string `yaml:"discord_webhook_url"`
	GenericWebhookURL string `yaml:"webhook_url"`
	OnSeverity        string `yaml:"on_severity"` // "critical"|"high"|"medium"|"low"; "" = disabled
}

type ScanCfg struct {
	Workers     int    `yaml:"workers"`
	Timeout     string `yaml:"timeout"`
	FailOn      string `yaml:"fail_on"`
	MinSeverity string `yaml:"min_severity"`
}

type SignCfg struct {
	RekorURL string `yaml:"rekor_url"`
}

func defaultConfig() FGConfig {
	return FGConfig{
		APIUrl: "http://localhost:8080",
		Scan: ScanCfg{
			Workers: 4,
			Timeout: "10m",
		},
	}
}

func configPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".forgeguardian", "config.yaml")
}

func loadConfig() FGConfig {
	cfg := defaultConfig()
	data, err := os.ReadFile(configPath())
	if err != nil {
		return cfg
	}
	_ = yaml.Unmarshal(data, &cfg)
	return cfg
}

func saveConfig(cfg FGConfig) error {
	path := configPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// runConfig handles: fgctl config show | fgctl config set key=value
func runConfig(args []string, log *slog.Logger, p *ui.Printer) error {
	if len(args) == 0 {
		args = []string{"show"}
	}
	switch args[0] {
	case "show":
		cfg := loadConfig()
		data, _ := yaml.Marshal(cfg)
		fmt.Printf("# ForgeGuardian config — %s\n", configPath())
		fmt.Print(string(data))

	case "set":
		if len(args) < 2 {
			return fmt.Errorf("usage: fgctl config set key=value")
		}
		kv := strings.SplitN(args[1], "=", 2)
		if len(kv) != 2 {
			return fmt.Errorf("usage: fgctl config set key=value")
		}
		cfg := loadConfig()
		if err := applyConfigSet(&cfg, kv[0], kv[1]); err != nil {
			return err
		}
		if err := saveConfig(cfg); err != nil {
			return fmt.Errorf("save config: %w", err)
		}
		p.Success("Set %s = %s  (saved to %s)", kv[0], kv[1], configPath())

	case "init":
		cfg := defaultConfig()
		if err := saveConfig(cfg); err != nil {
			return fmt.Errorf("init config: %w", err)
		}
		p.Success("Default config written to %s", configPath())

	default:
		return fmt.Errorf("unknown config subcommand %q — try: show | set key=value | init", args[0])
	}
	return nil
}

func applyConfigSet(cfg *FGConfig, key, value string) error {
	switch key {
	case "api_url":
		cfg.APIUrl = value
	case "scan.workers":
		var n int
		if _, err := fmt.Sscanf(value, "%d", &n); err != nil || n < 1 {
			return fmt.Errorf("scan.workers must be a positive integer")
		}
		cfg.Scan.Workers = n
	case "scan.timeout":
		cfg.Scan.Timeout = value
	case "scan.fail_on":
		cfg.Scan.FailOn = value
	case "scan.min_severity":
		cfg.Scan.MinSeverity = value
	case "signing.rekor_url":
		cfg.Signing.RekorURL = value
	case "notify.slack_webhook_url":
		cfg.Notify.SlackWebhookURL = value
	case "notify.discord_webhook_url":
		cfg.Notify.DiscordWebhookURL = value
	case "notify.webhook_url":
		cfg.Notify.GenericWebhookURL = value
	case "notify.on_severity":
		cfg.Notify.OnSeverity = value
	default:
		return fmt.Errorf("unknown config key %q — valid keys: api_url, scan.workers, scan.timeout, scan.fail_on, scan.min_severity, signing.rekor_url, notify.slack_webhook_url, notify.discord_webhook_url, notify.webhook_url, notify.on_severity", key)
	}
	return nil
}

// ── policy ────────────────────────────────────────────────────────────────────

// runPolicy handles: fgctl policy show | init | set key=value | validate
func runPolicy(args []string, log *slog.Logger, p *ui.Printer) error {
	sub := "show"
	if len(args) > 0 {
		sub = args[0]
	}

	switch sub {
	case "show":
		data, err := os.ReadFile(policy.DefaultPolicyPath())
		if os.IsNotExist(err) {
			p.Warn("No policy file found at %s", policy.DefaultPolicyPath())
			fmt.Println("  Run 'fgctl policy init' to create a default policy.")
			return nil
		}
		if err != nil {
			return fmt.Errorf("policy show: %w", err)
		}
		fmt.Printf("# ForgeGuardian policy — %s\n", policy.DefaultPolicyPath())
		fmt.Print(string(data))
		return nil

	case "init":
		if _, err := os.Stat(policy.DefaultPolicyPath()); err == nil {
			p.Warn("Policy file already exists at %s (use 'policy show' to view)", policy.DefaultPolicyPath())
			return nil
		}
		if err := policy.Save(policy.DefaultPolicy()); err != nil {
			return fmt.Errorf("policy init: %w", err)
		}
		p.Success("Created policy file at %s", policy.DefaultPolicyPath())
		return nil

	case "set":
		if len(args) < 2 {
			return fmt.Errorf("usage: fgctl policy set key=value")
		}
		pol, err := policy.Load()
		if err != nil {
			return err
		}
		if pol == nil {
			pol = policy.DefaultPolicy()
		}
		if err := applyPolicySet(pol, args[1]); err != nil {
			return err
		}
		if err := policy.Save(pol); err != nil {
			return fmt.Errorf("policy set: %w", err)
		}
		p.Success("Updated policy: %s", args[1])
		return nil

	case "validate":
		pol, err := policy.Load()
		if err != nil {
			return fmt.Errorf("policy validate: %w", err)
		}
		if pol == nil {
			p.Warn("No policy file found — run 'fgctl policy init' to create one")
			return nil
		}
		validSeverities := map[string]bool{"": true, "critical": true, "high": true, "medium": true, "low": true}
		if !validSeverities[strings.ToLower(pol.FailOn)] {
			return fmt.Errorf("policy validate: fail_on must be critical|high|medium|low|\"\" (got %q)", pol.FailOn)
		}
		p.Success("Policy valid — %d deny rules, fail_on=%q", len(pol.DenyPackages), pol.FailOn)
		return nil

	case "deny":
		if len(args) < 2 {
			return fmt.Errorf("usage: fgctl policy deny <package-name>")
		}
		pol, err := policy.Load()
		if err != nil {
			return err
		}
		if pol == nil {
			pol = policy.DefaultPolicy()
		}
		pkg := args[1]
		for _, existing := range pol.DenyPackages {
			if existing == pkg {
				p.Warn("Package %q is already in the deny list", pkg)
				return nil
			}
		}
		pol.DenyPackages = append(pol.DenyPackages, pkg)
		if err := policy.Save(pol); err != nil {
			return fmt.Errorf("policy deny: %w", err)
		}
		p.Success("Added %q to deny list (%d total)", pkg, len(pol.DenyPackages))
		return nil

	case "allow":
		if len(args) < 2 {
			return fmt.Errorf("usage: fgctl policy allow <package-name>  (removes from deny list)")
		}
		pol, err := policy.Load()
		if err != nil {
			return err
		}
		if pol == nil {
			p.Warn("No policy file found — run 'fgctl policy init' to create one")
			return nil
		}
		pkg := args[1]
		var kept []string
		found := false
		for _, existing := range pol.DenyPackages {
			if existing == pkg {
				found = true
				continue
			}
			kept = append(kept, existing)
		}
		if !found {
			p.Warn("Package %q is not in the deny list", pkg)
			return nil
		}
		pol.DenyPackages = kept
		if err := policy.Save(pol); err != nil {
			return fmt.Errorf("policy allow: %w", err)
		}
		p.Success("Removed %q from deny list (%d remaining)", pkg, len(pol.DenyPackages))
		return nil

	default:
		return fmt.Errorf("unknown policy subcommand %q — use: show | init | set | deny | allow | validate", sub)
	}
}

func applyPolicySet(pol *policy.Policy, kv string) error {
	parts := strings.SplitN(kv, "=", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid format — use key=value")
	}
	key, val := parts[0], parts[1]
	switch key {
	case "fail_on":
		pol.FailOn = strings.ToLower(val)
	case "block_typosquatting":
		pol.BlockTyposquatting = val == "true"
	case "block_abandoned":
		pol.BlockAbandoned = val == "true"
	case "require_signing":
		pol.RequireSigning = val == "true"
	case "max_package_age_days":
		var n int
		if _, err := fmt.Sscanf(val, "%d", &n); err != nil {
			return fmt.Errorf("max_package_age_days must be an integer")
		}
		pol.MaxPackageAgeDays = n
	default:
		return fmt.Errorf("unknown policy key %q", key)
	}
	return nil
}

// ── stats ─────────────────────────────────────────────────────────────────────

// runStats prints ForgeGuardian runtime statistics from the loaded signature store.
func runStats(args []string, log *slog.Logger, p *ui.Printer) error {
	fs := flag.NewFlagSet("stats", flag.ExitOnError)
	jsonFlag := fs.Bool("json", false, "output JSON")
	storeFlag := fs.String("store", "", "signature store path (default: ~/.forgeguardian/signatures.json)")
	fs.Parse(args)

	storePath := *storeFlag
	if storePath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("stats: %w", err)
		}
		storePath = home + "/.forgeguardian/signatures.json"
	}

	store, err := intelligence.LoadStoreWithLocalFallback(storePath)
	if err != nil {
		return fmt.Errorf("stats: could not load signatures: %w\n  Run: fgctl update", err)
	}

	// Count by signature type.
	counts := map[intelligence.SignatureType]int{}
	ecosystems := map[string]bool{}
	for _, sig := range store.Signatures {
		counts[sig.Type]++
		if sig.Ecosystem != "" && sig.Ecosystem != "*" {
			ecosystems[sig.Ecosystem] = true
		}
	}

	ecoList := make([]string, 0, len(ecosystems))
	for e := range ecosystems {
		ecoList = append(ecoList, e)
	}
	sort.Strings(ecoList)

	if *jsonFlag {
		out := map[string]any{
			"total_signatures":    len(store.Signatures),
			"malware_patterns":    counts[intelligence.SigMalwarePattern],
			"typosquat_targets":   counts[intelligence.SigTypoTarget],
			"behavioral_rules":    counts[intelligence.SigBehavioral],
			"blocklist_entries":   counts[intelligence.SigBlocklisted],
			"mcp_injection_rules": counts[intelligence.SigMCPInjection],
			"ecosystems_covered":  ecoList,
			"signatures_updated":  store.UpdatedAt.Format("2006-01-02"),
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	}

	fmt.Fprintf(p.Out, "\nForgeGuardian Runtime Statistics\n")
	fmt.Fprintf(p.Out, "%s\n", strings.Repeat("─", 42))
	fmt.Fprintf(p.Out, "  %-30s %d\n", "Detection signatures:", len(store.Signatures))
	fmt.Fprintf(p.Out, "  %-30s %d\n", "Malware patterns:", counts[intelligence.SigMalwarePattern])
	fmt.Fprintf(p.Out, "  %-30s %d\n", "Typosquat targets:", counts[intelligence.SigTypoTarget])
	fmt.Fprintf(p.Out, "  %-30s %d\n", "Behavioral rules:", counts[intelligence.SigBehavioral])
	fmt.Fprintf(p.Out, "  %-30s %d\n", "Blocklist entries:", counts[intelligence.SigBlocklisted])
	fmt.Fprintf(p.Out, "  %-30s %d\n", "MCP injection rules:", counts[intelligence.SigMCPInjection])
	if len(ecoList) > 0 {
		fmt.Fprintf(p.Out, "  %-30s %s\n", "Ecosystems covered:", strings.Join(ecoList, ", "))
	}
	fmt.Fprintf(p.Out, "  %-30s %s\n", "Signatures updated:", store.UpdatedAt.Format("2006-01-02"))
	fmt.Fprintln(p.Out)
	return nil
}
