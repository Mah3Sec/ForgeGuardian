package main

import (
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/fatih/color"
	"gopkg.in/yaml.v3"

	"github.com/mah3sec/forgeguardian/internal/intelligence"
)

type doctorStatus int

const (
	doctorPass doctorStatus = iota
	doctorWarn
	doctorFail
)

type doctorCheck struct {
	Name   string
	Status doctorStatus
	Detail string
}

// runDoctor checks the ForgeGuardian runtime environment and reports status.
// With --fix, attempts to auto-repair failing checks.
func runDoctor(args []string, log *slog.Logger) error {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	doFix := fs.Bool("fix", false, "attempt to auto-repair failing checks")
	if err := fs.Parse(args); err != nil {
		return err
	}

	checks := []doctorCheck{
		checkGoVersion(),
		checkBinary("grype"),
		checkBinary("semgrep"),
		checkBinary("trivy"),
		checkEnvKey("ANTHROPIC_API_KEY", "advisory and auto-fix commands unavailable"),
		checkSignatures(),
		checkSignaturesFreshness(),
		checkDiskSpace(),
		checkAPIConn(),
		checkDockerAvailable(),
		checkNodeVersion(),
		checkDashboardBuild(),
		checkConfigYAML(),
	}

	width := 0
	for _, c := range checks {
		if len(c.Name) > width {
			width = len(c.Name)
		}
	}

	fmt.Printf("\nfgctl doctor — ForgeGuardian %s\n", version)
	fmt.Printf("%s\n", strings.Repeat("─", 50))

	failures, warnings := 0, 0
	for i, c := range checks {
		var badge *color.Color
		var label string
		switch c.Status {
		case doctorPass:
			badge = color.New(color.FgGreen, color.Bold)
			label = "PASS"
		case doctorWarn:
			badge = color.New(color.FgYellow, color.Bold)
			label = "WARN"
			warnings++
		case doctorFail:
			badge = color.New(color.FgRed, color.Bold)
			label = "FAIL"
			failures++
		}
		fmt.Printf("[%s] %-*s  %s\n", badge.Sprint(label), width, c.Name, c.Detail)

		if *doFix && c.Status != doctorPass {
			fixed := attemptFix(c.Name)
			if fixed != "" {
				color.New(color.FgGreen).Printf("       [FIX] %s\n", fixed)
				// Re-check after fix. Checks with a real post-fix re-check
				// (grype/semgrep/trivy: does the binary exist now?) get their
				// live status; everything else just re-runs its own check
				// function fresh rather than reusing the stale pre-fix result,
				// so e.g. config.yaml correctly flips to PASS once the fix
				// actually created it.
				var updated doctorCheck
				switch c.Name {
				case "grype", "semgrep", "trivy":
					updated = checkBinary(c.Name)
				case "signatures":
					updated = checkSignatures()
				case "signatures freshness":
					updated = checkSignaturesFreshness()
				case "config.yaml":
					updated = checkConfigYAML()
				default:
					updated = c
				}
				if updated.Status == doctorPass {
					checks[i] = updated
					// Only decrement whichever counter the ORIGINAL status
					// actually incremented — decrementing `failures` for a
					// check that was only ever a WARN drove the total
					// negative (a real bug this comment replaces).
					switch c.Status {
					case doctorFail:
						failures--
					case doctorWarn:
						warnings--
					}
				}
			}
		}
	}

	fmt.Printf("%s\n", strings.Repeat("─", 50))
	if failures == 0 && warnings == 0 {
		color.New(color.FgGreen).Println("  All checks passed.")
	} else {
		msg := fmt.Sprintf("  %d failure(s), %d warning(s).", failures, warnings)
		if failures > 0 {
			color.New(color.FgRed).Println(msg)
		} else {
			color.New(color.FgYellow).Println(msg)
		}
		if !*doFix {
			fmt.Println("  Run 'fgctl doctor --fix' to attempt automatic repair.")
		}
	}
	fmt.Println()

	if failures > 0 {
		return fmt.Errorf("doctor: %d check(s) failed", failures)
	}
	return nil
}

// attemptFix tries to auto-repair a named check. Returns a description of
// what was done (or attempted), or "" if no fix is available.
func attemptFix(name string) string {
	switch name {
	case "grype":
		return fixTool("grype", "brew install anchore/grype/grype", "pip install grype")
	case "semgrep":
		return fixTool("semgrep", "pip install semgrep", "pip3 install semgrep")
	case "trivy":
		return fixTool("trivy", "brew install trivy", "apt-get install -y trivy")
	case "ANTHROPIC_API_KEY":
		return "set ANTHROPIC_API_KEY in your shell profile to enable AI features"
	default:
		return ensureForgeDir()
	}
}

// fixTool attempts to install a tool using the first installer available.
func fixTool(toolName, macCmd, linuxCmd string) string {
	// Try brew (macOS)
	if _, err := exec.LookPath("brew"); err == nil {
		parts := strings.Fields(macCmd)
		fmt.Printf("  Running: %s\n", macCmd)
		out, err := exec.Command(parts[0], parts[1:]...).CombinedOutput()
		if err == nil {
			return fmt.Sprintf("%s installed via brew", toolName)
		}
		return fmt.Sprintf("brew install failed: %s — install manually: %s", strings.TrimSpace(string(out)), macCmd)
	}
	// Try pip
	if strings.HasPrefix(macCmd, "pip") {
		if _, err := exec.LookPath("pip3"); err == nil {
			fixCmd := "pip3 install " + toolName
			fmt.Printf("  Running: %s\n", fixCmd)
			parts := strings.Fields(fixCmd)
			if err := exec.Command(parts[0], parts[1:]...).Run(); err == nil {
				return fmt.Sprintf("%s installed via pip3", toolName)
			}
		}
		if _, err := exec.LookPath("pip"); err == nil {
			fixCmd := "pip install " + toolName
			fmt.Printf("  Running: %s\n", fixCmd)
			parts := strings.Fields(fixCmd)
			if err := exec.Command(parts[0], parts[1:]...).Run(); err == nil {
				return fmt.Sprintf("%s installed via pip", toolName)
			}
		}
	}
	return fmt.Sprintf("auto-install unavailable — install manually: %s", linuxCmd)
}

// ensureForgeDir creates the ~/.forgeguardian directory and default files.
func ensureForgeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	dir := filepath.Join(home, ".forgeguardian")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return ""
	}
	created := []string{}

	cfgPath := filepath.Join(dir, "config.yaml")
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		cfg := defaultConfig()
		if data, err := yaml.Marshal(cfg); err == nil {
			if err := os.WriteFile(cfgPath, data, 0o600); err == nil {
				created = append(created, "config.yaml")
			}
		}
	}

	if len(created) == 0 {
		return ""
	}
	return "created " + strings.Join(created, ", ") + " in " + dir
}

func checkGoVersion() doctorCheck {
	v := runtime.Version()
	return doctorCheck{Name: "Go runtime", Status: doctorPass, Detail: v}
}

func checkBinary(name string) doctorCheck {
	path, err := exec.LookPath(name)
	if err != nil {
		return doctorCheck{
			Name:   name,
			Status: doctorWarn,
			Detail: "not found in PATH — some scan engines disabled",
		}
	}
	// Try to get version
	out, err := exec.Command(path, "--version").Output()
	ver := ""
	if err == nil {
		ver = strings.TrimSpace(strings.Split(string(out), "\n")[0])
	}
	detail := fmt.Sprintf("found at %s", path)
	if ver != "" {
		detail = fmt.Sprintf("found at %s  (%s)", path, ver)
	}
	return doctorCheck{Name: name, Status: doctorPass, Detail: detail}
}

func checkEnvKey(key, warnMsg string) doctorCheck {
	if os.Getenv(key) != "" {
		return doctorCheck{Name: key, Status: doctorPass, Detail: "set"}
	}
	return doctorCheck{Name: key, Status: doctorWarn, Detail: "not set — " + warnMsg}
}

func checkSignatures() doctorCheck {
	storePath, err := intelligence.DefaultStorePath()
	if err != nil {
		return doctorCheck{Name: "signatures", Status: doctorWarn, Detail: "cannot determine home directory"}
	}
	// Includes the local signatures/ dir fallback (running from a git clone)
	// — without it this check would WARN "not found, run fgctl update" even
	// when scans are already using 24 real signatures loaded from disk.
	store, err := intelligence.LoadStoreWithLocalFallback(storePath)
	if err != nil || len(store.Signatures) == 0 {
		return doctorCheck{
			Name:   "signatures",
			Status: doctorWarn,
			Detail: "not found — run 'fgctl update' to download community signatures",
		}
	}
	return doctorCheck{
		Name:   "signatures",
		Status: doctorPass,
		Detail: fmt.Sprintf("%d detection signatures loaded", len(store.Signatures)),
	}
}

func checkDiskSpace() doctorCheck {
	dir := os.TempDir()
	freeGB, err := diskFreeGB()
	if err != nil {
		return doctorCheck{Name: "disk space", Status: doctorWarn, Detail: "cannot check: " + err.Error()}
	}
	if freeGB < 0.5 {
		return doctorCheck{
			Name:   "disk space",
			Status: doctorFail,
			Detail: fmt.Sprintf("%.1f GB free in %s (minimum 500 MB required)", freeGB, dir),
		}
	}
	return doctorCheck{
		Name:   "disk space",
		Status: doctorPass,
		Detail: fmt.Sprintf("%.1f GB free in %s", freeGB, dir),
	}
}

// checkSignaturesFreshness warns if signatures.json is older than 7 days.
func checkSignaturesFreshness() doctorCheck {
	home, err := os.UserHomeDir()
	if err != nil {
		return doctorCheck{Name: "signatures freshness", Status: doctorWarn, Detail: "cannot determine home directory"}
	}
	path := filepath.Join(home, ".forgeguardian", "signatures.json")
	info, err := os.Stat(path)
	if err != nil {
		// File missing is already caught by checkSignatures; just skip here.
		return doctorCheck{Name: "signatures freshness", Status: doctorWarn, Detail: "signatures.json not found — run 'fgctl update'"}
	}
	age := time.Since(info.ModTime())
	if age > 7*24*time.Hour {
		return doctorCheck{
			Name:   "signatures freshness",
			Status: doctorWarn,
			Detail: fmt.Sprintf("last updated %.0f days ago — run 'fgctl update' to refresh", age.Hours()/24),
		}
	}
	return doctorCheck{
		Name:   "signatures freshness",
		Status: doctorPass,
		Detail: fmt.Sprintf("up to date (%.0fh ago)", age.Hours()),
	}
}

// checkDockerAvailable warns (non-blocking) if Docker is not running.
func checkDockerAvailable() doctorCheck {
	out, err := exec.Command("docker", "info").CombinedOutput()
	if err != nil {
		detail := "docker not available — container-based build sandboxing disabled"
		if len(out) > 0 {
			first := strings.SplitN(strings.TrimSpace(string(out)), "\n", 2)[0]
			detail = fmt.Sprintf("docker info failed: %s", first)
		}
		return doctorCheck{Name: "docker", Status: doctorWarn, Detail: detail}
	}
	return doctorCheck{Name: "docker", Status: doctorPass, Detail: "daemon reachable"}
}

// checkNodeVersion warns if Node.js is missing or older than v20.
func checkNodeVersion() doctorCheck {
	out, err := exec.Command("node", "--version").Output()
	if err != nil {
		return doctorCheck{Name: "node.js", Status: doctorWarn, Detail: "not found in PATH — dashboard build requires Node.js >=20"}
	}
	ver := strings.TrimSpace(string(out)) // e.g. "v18.20.0"
	major := 0
	fmt.Sscanf(strings.TrimPrefix(ver, "v"), "%d", &major)
	if major < 20 {
		return doctorCheck{
			Name:   "node.js",
			Status: doctorWarn,
			Detail: fmt.Sprintf("%s detected — dashboard build requires Node.js >=20", ver),
		}
	}
	return doctorCheck{Name: "node.js", Status: doctorPass, Detail: ver}
}

// checkDashboardBuild checks whether the dashboard has been compiled.
func checkDashboardBuild() doctorCheck {
	cwd, err := os.Getwd()
	if err != nil {
		return doctorCheck{Name: "dashboard build", Status: doctorWarn, Detail: "cannot determine working directory"}
	}
	distPath := filepath.Join(cwd, "dashboard", "dist")
	info, err := os.Stat(distPath)
	if err != nil || !info.IsDir() {
		return doctorCheck{
			Name:   "dashboard build",
			Status: doctorWarn,
			Detail: "dashboard/dist/ not found — run 'make build-dashboard' or 'cd dashboard && npm run build'",
		}
	}
	return doctorCheck{Name: "dashboard build", Status: doctorPass, Detail: distPath}
}

// checkConfigYAML warns if ~/.forgeguardian/config.yaml exists but cannot be parsed.
func checkConfigYAML() doctorCheck {
	home, err := os.UserHomeDir()
	if err != nil {
		return doctorCheck{Name: "config.yaml", Status: doctorWarn, Detail: "cannot determine home directory"}
	}
	cfgPath := filepath.Join(home, ".forgeguardian", "config.yaml")
	data, err := os.ReadFile(cfgPath)
	if os.IsNotExist(err) {
		return doctorCheck{Name: "config.yaml", Status: doctorWarn, Detail: "not found — run 'fgctl config init' to create"}
	}
	if err != nil {
		return doctorCheck{Name: "config.yaml", Status: doctorWarn, Detail: "cannot read: " + err.Error()}
	}
	var cfg map[string]any
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return doctorCheck{Name: "config.yaml", Status: doctorWarn, Detail: "parse error: " + err.Error()}
	}
	return doctorCheck{Name: "config.yaml", Status: doctorPass, Detail: fmt.Sprintf("valid (%d top-level keys)", len(cfg))}
}

func checkAPIConn() doctorCheck {
	apiURL := os.Getenv("FORGEGUARDIAN_API_URL")
	if apiURL == "" {
		apiURL = "http://localhost:8080"
	}
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(apiURL + "/healthz")
	if err != nil {
		return doctorCheck{
			Name:   "API server",
			Status: doctorWarn,
			Detail: fmt.Sprintf("not reachable at %s (CLI works offline)", apiURL),
		}
	}
	resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		return doctorCheck{Name: "API server", Status: doctorPass, Detail: fmt.Sprintf("healthy at %s", apiURL)}
	}
	return doctorCheck{
		Name:   "API server",
		Status: doctorWarn,
		Detail: fmt.Sprintf("returned %d at %s", resp.StatusCode, apiURL),
	}
}
