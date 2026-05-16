package main

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/mah3sec/forgeguardian/internal/localscanner"
	"github.com/mah3sec/forgeguardian/internal/remotescan"
	"github.com/mah3sec/forgeguardian/internal/ui"
)

// remoteScanOpts holds --remote-specific flags, plus the same localScanOpts
// used by a local scan so output formatting is identical either way.
type remoteScanOpts struct {
	port             int
	identity         string
	remotePath       string
	acceptNewHostKey bool
	maxDepth         int
	keepTemp         bool
	local            localScanOpts
}

// runRemoteScan connects to a remote host over SSH, discovers dependency
// manifest files, pulls them into a local temp dir, and scans them through
// the same pipeline a local directory scan uses. Nothing is ever written to
// the remote host and no agent/binary is left behind — only read-only
// exec (echo/find/cat) commands are run remotely, and the local temp dir is
// removed when the scan finishes unless --keep-temp was passed.
func runRemoteScan(target string, ropts remoteScanOpts, log *slog.Logger, p *ui.Printer) error {
	cfg, err := remotescan.ParseTarget(target, ropts.port)
	if err != nil {
		return err
	}
	cfg.IdentityPath = ropts.identity
	cfg.RemotePath = ropts.remotePath
	cfg.AcceptNewHostKey = ropts.acceptNewHostKey
	cfg.MaxDepth = ropts.maxDepth
	cfg.ConnectTimeout = 15 * time.Second

	sp := p.Spinner(fmt.Sprintf("Connecting to %s...", target))
	client, err := remotescan.Connect(cfg)
	ui.StopSpinner(sp)
	if err != nil {
		return err
	}
	defer client.Close()

	searchRoot := cfg.RemotePath
	if searchRoot == "" {
		home, err := client.ResolveHome()
		if err != nil {
			return err
		}
		searchRoot = home
	}

	sp2 := p.Spinner("Discovering dependency manifests...")
	files, err := client.Discover(searchRoot, cfg.MaxDepth)
	ui.StopSpinner(sp2)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("no dependency manifests found on %s under %s", target, searchRoot)
	}

	tmpDir, err := remotescan.NewTempDir()
	if err != nil {
		return err
	}
	defer remotescan.Cleanup(tmpDir, ropts.keepTemp, log)

	sp3 := p.Spinner(fmt.Sprintf("Pulling %d manifest file(s)...", len(files)))
	pullResults := client.Pull(files, tmpDir)
	ui.StopSpinner(sp3)
	printPullWarnings(pullResults, p)

	pulledAny := false
	for _, r := range pullResults {
		if r.Err == nil {
			pulledAny = true
			break
		}
	}
	if !pulledAny {
		return fmt.Errorf("failed to pull any of the %d discovered manifest file(s) from %s", len(files), target)
	}

	storePath, _ := signaturesStorePath()
	ls := localscanner.New(storePath)
	ls.Workers = ropts.local.workers
	ls.ProdOnly = ropts.local.prodOnly

	scanStart := time.Now()
	sp4 := p.Spinner("Scanning pulled manifests...")
	ctx, cancel := context.WithTimeout(context.Background(), ropts.local.timeout)
	defer cancel()
	result, err := ls.Scan(ctx, tmpDir, func(done, total int) {
		if sp4 != nil {
			sp4.Suffix = fmt.Sprintf("  Scanning %d/%d packages...", done, total)
		}
	})
	ui.StopSpinner(sp4)
	if err != nil {
		return fmt.Errorf("remote scan: %w", err)
	}

	displayLabel := fmt.Sprintf("%s:%s", target, searchRoot)
	return printLocalScanResult(result, ropts.local, p, scanStart, displayLabel)
}

// printPullWarnings prints a consolidated, non-fatal warning listing any
// files that failed to pull, mirroring the style of PrintMissingToolsWarning
// elsewhere in this CLI — a partial pull is not treated as a scan failure.
func printPullWarnings(results []remotescan.PullResult, p *ui.Printer) {
	var failed []remotescan.PullResult
	for _, r := range results {
		if r.Err != nil {
			failed = append(failed, r)
		}
	}
	if len(failed) == 0 {
		return
	}
	p.Warn("%d of %d manifest file(s) could not be pulled:", len(failed), len(results))
	for _, r := range failed {
		p.Warn("  %s: %v", r.Remote.RemotePath, r.Err)
	}
}
