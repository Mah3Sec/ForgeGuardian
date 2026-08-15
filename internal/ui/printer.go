package ui

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/briandowns/spinner"
	"github.com/fatih/color"
	"github.com/mah3sec/forgeguardian/internal/core"
	"github.com/mah3sec/forgeguardian/internal/localscanner"
	"github.com/mah3sec/forgeguardian/internal/scanner"
)

var bannerOnce sync.Once

const asciiLogo = `
   ███████╗ ██████╗  ██████╗  ██████╗ ███████╗
   ██╔════╝██╔═══██╗██╔══██╗██╔════╝ ██╔════╝
   █████╗  ██║   ██║██████╔╝██║  ███╗█████╗
   ██╔══╝  ██║   ██║██╔══██╗██║   ██║██╔══╝
   ██║     ╚██████╔╝██║  ██║╚██████╔╝███████╗
   ╚═╝      ╚═════╝ ╚═╝  ╚═╝ ╚═════╝ ╚══════╝
             GUARDIAN · local-first AI-native supply chain security · %s
`

// Printer is a terminal output helper for all fgctl commands.
type Printer struct {
	NoColor        bool
	JSON           bool
	Verbose        bool // show informational findings and expand all grouped output
	Debug          bool // show engine errors, raw metadata
	NoBanner       bool // suppress the ASCII banner
	Quiet          bool // suppress all output except errors
	Out            io.Writer
	Err            io.Writer
	version        string
	sigCount       int
	heuristicCount int
	engineCount    int
}

// New creates a Printer. When JSON is true, all decoration is suppressed.
func New(noColor, jsonMode bool, version string) *Printer {
	if noColor {
		color.NoColor = true
	}
	return &Printer{
		NoColor: noColor,
		JSON:    jsonMode,
		Out:     os.Stdout,
		Err:     os.Stderr,
		version: version,
	}
}

// SetSigCount sets the loaded signature count shown in the banner.
func (p *Printer) SetSigCount(n int) { p.sigCount = n }

// SetHeuristicCount sets the malware heuristic signature count for banner display.
func (p *Printer) SetHeuristicCount(n int) { p.heuristicCount = n }

// SetEngineCount sets the active scan engine count for banner display.
func (p *Printer) SetEngineCount(n int) { p.engineCount = n }

// Banner prints the ASCII art logo once per process. Suppressed in JSON mode or when NoBanner is set.
func (p *Printer) Banner() {
	if p.NoBanner || p.JSON {
		return
	}
	bannerOnce.Do(func() {
		v := p.version
		if v == "" {
			v = "dev"
		}
		fmt.Fprintf(p.Out, asciiLogo, v)
		if p.sigCount > 0 {
			parts := fmt.Sprintf("• %d detection signatures", p.sigCount)
			if p.heuristicCount > 0 {
				parts += fmt.Sprintf("  • %d malware heuristics", p.heuristicCount)
			}
			if p.engineCount > 0 {
				parts += fmt.Sprintf("  • %d scan engines", p.engineCount)
			}
			color.New(color.FgWhite).Fprintf(p.Out, "             Loaded: %s\n", parts)
		}
		fmt.Fprintln(p.Out)
	})
}

// Spinner starts and returns a spinner with the given message.
// Call s.Stop() when done. Suppressed in JSON mode.
func (p *Printer) Spinner(msg string) *spinner.Spinner {
	if p.JSON {
		return nil
	}
	s := spinner.New(spinner.CharSets[14], 80*time.Millisecond, spinner.WithWriter(p.Err))
	s.Suffix = "  " + msg
	s.Color("cyan")
	s.Start()
	return s
}

// StopSpinner stops a spinner if it is non-nil (safe to call with nil).
func StopSpinner(s *spinner.Spinner) {
	if s != nil {
		s.Stop()
	}
}

// SeverityColor returns a color.Color for a severity value.
func SeverityColor(sev core.Severity) *color.Color {
	switch sev {
	case core.SeverityCritical:
		return color.New(color.FgRed, color.Bold)
	case core.SeverityHigh:
		return color.New(color.FgHiRed)
	case core.SeverityMedium:
		return color.New(color.FgYellow)
	case core.SeverityLow:
		return color.New(color.FgCyan)
	default:
		return color.New(color.FgWhite)
	}
}

// SevBadge returns a consistently styled severity badge string.
func (p *Printer) SevBadge(sev core.Severity) string {
	switch sev {
	case core.SeverityCritical:
		return color.New(color.FgWhite, color.Bold, color.BgRed).Sprintf(" CRITICAL ")
	case core.SeverityHigh:
		return color.New(color.FgHiRed, color.Bold).Sprintf("[ HIGH ]")
	case core.SeverityMedium:
		return color.New(color.FgYellow, color.Bold).Sprintf("[ MEDIUM ]")
	case core.SeverityLow:
		return color.New(color.FgCyan).Sprintf("[ LOW ]")
	default:
		return color.New(color.FgWhite).Sprintf("[ INFO ]")
	}
}

// PrintFindings renders the findings table for a single package scan.
func (p *Printer) PrintFindings(pkg string, findings []core.Finding, sum scanner.ScanSummary) {
	if p.JSON {
		return
	}
	header := color.New(color.FgHiWhite, color.Bold)
	header.Fprintf(p.Out, "\nForgeGuardian Scan — %s\n", pkg)
	fmt.Fprintf(p.Out, "%s\n\n", strings.Repeat("═", 60))

	if len(findings) == 0 {
		color.New(color.FgGreen).Fprintf(p.Out, "  ✓  No findings\n\n")
		return
	}

	for _, f := range findings {
		sev := SeverityColor(f.Severity)
		label := sev.Sprintf("  [%-12s]", string(f.Severity))
		fmt.Fprintf(p.Out, "%s %-24s — %s\n", label, f.ID, f.Title)
	}

	fmt.Fprintln(p.Out)
	p.PrintScanSummary(sum)
}

// PrintLocalScanHeader prints the header for a local project scan.
func (p *Printer) PrintLocalScanHeader(rootDir string) {
	if p.JSON {
		return
	}
	header := color.New(color.FgHiWhite, color.Bold)
	header.Fprintf(p.Out, "\nForgeGuardian — Local Project Scan\n")
	fmt.Fprintf(p.Out, "%s\n", strings.Repeat("═", 60))
	fmt.Fprintf(p.Out, "  Project: %s\n\n", rootDir)
}

// PrintManifestHeader prints a manifest file section header.
func (p *Printer) PrintManifestHeader(manifestPath, ecosystem string) {
	if p.JSON {
		return
	}
	color.New(color.FgHiWhite).Fprintf(p.Out, "  %s (%s)\n", manifestPath, ecosystem)
}

// PrintLocalFinding prints one finding from a local scan with an optional fix hint.
func (p *Printer) PrintLocalFinding(name, version string, f core.Finding) {
	if p.JSON {
		return
	}
	sev := SeverityColor(f.Severity)
	label := sev.Sprintf("    [%-8s]", string(f.Severity))
	fmt.Fprintf(p.Out, "%s %-30s  %s\n", label, name+"@"+version, f.Title)
	if f.FixedVersion != "" {
		color.New(color.FgGreen, color.Bold).Fprintf(p.Out, "               → upgrade to %s\n", f.FixedVersion)
	}
}

// PrintLocalScanSkip prints a [SKIP] notice for unparseable version.
func (p *Printer) PrintLocalScanSkip(name, raw string) {
	if p.JSON {
		return
	}
	color.New(color.FgWhite).Fprintf(p.Out, "    [SKIP]    %s  (version range '%s' — pin to scan)\n", name, raw)
}

// PrintScanSummary prints the aggregate summary line.
func (p *Printer) PrintScanSummary(sum scanner.ScanSummary) {
	if p.JSON {
		return
	}
	fmt.Fprintf(p.Out, "  Summary: %d finding(s)", sum.Total)
	parts := []string{}
	if sum.Critical > 0 {
		parts = append(parts, color.New(color.FgRed, color.Bold).Sprintf("%d CRITICAL", sum.Critical))
	}
	if sum.High > 0 {
		parts = append(parts, color.New(color.FgHiRed).Sprintf("%d HIGH", sum.High))
	}
	if sum.Medium > 0 {
		parts = append(parts, color.New(color.FgYellow).Sprintf("%d MEDIUM", sum.Medium))
	}
	if sum.Low > 0 {
		parts = append(parts, color.New(color.FgCyan).Sprintf("%d LOW", sum.Low))
	}
	if len(parts) > 0 {
		fmt.Fprintf(p.Out, "  (%s)", strings.Join(parts, " · "))
	}
	fmt.Fprintln(p.Out)
}

// PrintMissingToolsWarning emits a single consolidated warning for unavailable scan engines.
// Call once before printing findings rather than letting per-package results repeat the noise.
func (p *Printer) PrintMissingToolsWarning(tools []string) {
	if p.JSON || len(tools) == 0 {
		return
	}
	color.New(color.FgYellow).Fprintf(p.Out, "  [WARN] %d scan engine(s) unavailable: %s\n",
		len(tools), strings.Join(tools, ", "))
	fmt.Fprintf(p.Out, "         Install missing tools for better coverage — run: fgctl doctor\n\n")
}

// PrintFindingCompact prints one finding in compact single-line format.
func (p *Printer) PrintFindingCompact(name, version string, f core.Finding) {
	if p.JSON {
		return
	}
	sev := SeverityColor(f.Severity)
	label := sev.Sprintf("[%-8s]", string(f.Severity))
	pkg := name + "@" + version
	fmt.Fprintf(p.Out, "  %s %-32s  %s\n", label, pkg, f.Title)
	if f.FixedVersion != "" {
		color.New(color.FgGreen, color.Bold).Fprintf(p.Out, "             → upgrade to %s\n", f.FixedVersion)
	}
}

// PrintCompactGrouped renders findings in true compact grouped format:
// one line per (package, version) pair showing severity counts and minimum fix version.
func (p *Printer) PrintCompactGrouped(results []localscanner.LocalScanResult) {
	if p.JSON {
		return
	}
	type pkgKey struct{ name, version string }
	type pkgAgg struct {
		critical, high, medium, low int
		minFix                      string
		total                       int
	}

	var order []pkgKey
	aggs := map[pkgKey]*pkgAgg{}

	for _, r := range results {
		if len(r.Findings) == 0 {
			continue
		}
		k := pkgKey{r.Entry.Name, r.Entry.Version}
		if _, ok := aggs[k]; !ok {
			aggs[k] = &pkgAgg{}
			order = append(order, k)
		}
		agg := aggs[k]
		for _, f := range r.Findings {
			if !p.Verbose && f.Severity == core.SeverityInformational {
				continue
			}
			agg.total++
			switch f.Severity {
			case core.SeverityCritical:
				agg.critical++
			case core.SeverityHigh:
				agg.high++
			case core.SeverityMedium:
				agg.medium++
			case core.SeverityLow:
				agg.low++
			}
			if f.FixedVersion != "" && agg.minFix == "" {
				agg.minFix = f.FixedVersion
			}
		}
	}

	for _, k := range order {
		agg := aggs[k]
		pkg := color.New(color.FgHiWhite, color.Bold).Sprintf("%s@%s", k.name, k.version)
		counts := fmt.Sprintf("[CRIT:%d HIGH:%d MED:%d LOW:%d]", agg.critical, agg.high, agg.medium, agg.low)
		fixStr := ""
		if agg.minFix != "" {
			fixStr = color.New(color.FgGreen, color.Bold).Sprintf("  fix: >=%s", agg.minFix)
		}
		fmt.Fprintf(p.Out, "  %s  — %d finding(s) %s%s\n", pkg, agg.total, counts, fixStr)
	}
}

// PrintSummaryTable prints just the severity breakdown table with no per-finding details.
func (p *Printer) PrintSummaryTable(scanned int, sum scanner.ScanSummary) {
	if p.JSON {
		return
	}
	fmt.Fprintf(p.Out, "\n  Packages scanned: %d\n", scanned)
	fmt.Fprintf(p.Out, "  %-12s  %d\n", "CRITICAL", sum.Critical)
	fmt.Fprintf(p.Out, "  %-12s  %d\n", "HIGH", sum.High)
	fmt.Fprintf(p.Out, "  %-12s  %d\n", "MEDIUM", sum.Medium)
	fmt.Fprintf(p.Out, "  %-12s  %d\n", "LOW", sum.Low)
	fmt.Fprintf(p.Out, "  %-12s  %d\n", "TOTAL", sum.Total)
	fmt.Fprintln(p.Out)
}

// Success prints a green success message.
func (p *Printer) Success(format string, args ...any) {
	if p.JSON {
		return
	}
	color.New(color.FgGreen).Fprintf(p.Out, "  ✓  "+format+"\n", args...)
}

// Warn prints an amber warning message.
func (p *Printer) Warn(format string, args ...any) {
	if p.JSON {
		return
	}
	color.New(color.FgYellow).Fprintf(p.Out, "  ⚠  "+format+"\n", args...)
}

// Error prints a red error message.
func (p *Printer) Error(format string, args ...any) {
	color.New(color.FgRed).Fprintf(p.Err, "  ✗  "+format+"\n", args...)
}

// Divider prints a terminal-width-aware horizontal rule (max 80 chars).
func (p *Printer) Divider() {
	if p.JSON {
		return
	}
	w := termWidth()
	fmt.Fprintf(p.Out, "  %s\n", strings.Repeat("─", w-2))
}

func termWidth() int {
	if cols := os.Getenv("COLUMNS"); cols != "" {
		var w int
		if _, err := fmt.Sscanf(cols, "%d", &w); err == nil && w > 0 {
			if w > 80 {
				return 80
			}
			return w
		}
	}
	return 60
}

// PrintGroupedFindings renders findings grouped by (package, version).
// Default: shows top 3 per package + count of remaining.
// With p.Verbose: expands all findings for every package.
func (p *Printer) PrintGroupedFindings(results []localscanner.LocalScanResult) {
	if p.JSON {
		return
	}
	type pkgKey struct{ name, version string }
	type pkgGroup struct {
		key      pkgKey
		findings []core.Finding
		grade    string
	}

	// Build ordered groups.
	seen := map[pkgKey]int{}
	var groups []pkgGroup
	for _, r := range results {
		if len(r.Findings) == 0 {
			continue
		}
		k := pkgKey{r.Entry.Name, r.Entry.Version}
		for _, f := range r.Findings {
			if !p.Verbose && f.Severity == core.SeverityInformational {
				continue
			}
			if idx, ok := seen[k]; ok {
				groups[idx].findings = append(groups[idx].findings, f)
			} else {
				seen[k] = len(groups)
				groups = append(groups, pkgGroup{key: k, findings: []core.Finding{f}})
			}
		}
		// Set grade after all findings for this package have been collected.
		// (Grade will be recomputed below after all results are processed.)
	}
	// Recompute grades now that findings are final.
	for i := range groups {
		groups[i].grade = gradeFromFindings(groups[i].findings)
	}

	for _, g := range groups {
		gradeColor := gradeColorCode(g.grade)
		header := color.New(color.FgHiWhite, color.Bold)
		header.Fprintf(p.Out, "  %s@%s", g.key.name, g.key.version)
		gradeColor.Fprintf(p.Out, "  [grade %s · %d finding(s)]\n", g.grade, len(g.findings))

		limit := 3
		if p.Verbose {
			limit = len(g.findings)
		}
		shown := g.findings
		if len(shown) > limit {
			shown = shown[:limit]
		}
		for i, f := range shown {
			isLast := i == len(shown)-1 && (p.Verbose || len(g.findings) <= limit)
			prefix := "  ├─"
			if isLast {
				prefix = "  └─"
			}
			sev := SeverityColor(f.Severity)
			fix := ""
			if f.FixedVersion != "" {
				fix = color.New(color.FgGreen, color.Bold).Sprintf("  → fix: >= %s", f.FixedVersion)
			}
			fmt.Fprintf(p.Out, "%s %s  %-26s  %s%s\n",
				prefix, sev.Sprintf("%-8s", string(f.Severity)), f.ID, f.Title, fix)
		}
		if !p.Verbose && len(g.findings) > limit {
			remaining := len(g.findings) - limit
			color.New(color.FgWhite).Fprintf(p.Out, "  └─ +%d more  (use --verbose to expand)\n", remaining)
		}
		fmt.Fprintln(p.Out)
	}
}

func gradeFromFindings(findings []core.Finding) string {
	score := 0
	for _, f := range findings {
		switch f.Severity {
		case core.SeverityCritical:
			score += 40
		case core.SeverityHigh:
			score += 20
		case core.SeverityMedium:
			score += 10
		case core.SeverityLow:
			score += 5
		}
	}
	switch {
	case score <= 20:
		return "A"
	case score <= 40:
		return "B"
	case score <= 60:
		return "C"
	case score <= 80:
		return "D"
	default:
		return "F"
	}
}

func gradeColorCode(grade string) *color.Color {
	switch grade {
	case "A":
		return color.New(color.FgGreen, color.Bold)
	case "B":
		return color.New(color.FgGreen)
	case "C":
		return color.New(color.FgYellow)
	case "D":
		return color.New(color.FgRed)
	default:
		return color.New(color.FgRed, color.Bold)
	}
}

// RelPath returns path relative to rootDir, falling back to path on error.
func RelPath(rootDir, path string) string {
	rel, err := filepath.Rel(rootDir, path)
	if err != nil {
		return path
	}
	return rel
}
