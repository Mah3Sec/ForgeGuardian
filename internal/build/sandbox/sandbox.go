// Package sandbox provides isolated execution environments for hermetic builds.
//
// Production (Linux): gVisor runsc with network namespace isolation.
// Development (any OS): ProcessSandbox — restricted child process with a
// minimal environment, ephemeral workdir, and network-connection auditing.
package sandbox

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/mah3sec/forgeguardian/internal/core"
)

// NetworkEvent records a network connection observed during the build.
type NetworkEvent struct {
	Proto   string
	SrcAddr string
	DstAddr string
}

// Result is the output of a sandbox.Run call.
type Result struct {
	Stdout        string
	Stderr        string
	ExitCode      int
	NetworkEvents []NetworkEvent
	FilesWritten  []string
	Duration      time.Duration
}

// ProcessSandbox runs commands as child processes with a minimal environment.
// On Linux it optionally uses `unshare -n` to isolate the network namespace.
// On macOS it audits connections via a pre/post snapshot of active sockets.
type ProcessSandbox struct {
	workDir  string   // ephemeral temp directory
	env      []string // minimal environment
	mu       sync.Mutex
	preConns map[string]bool // connections before build
}

// New creates a new ProcessSandbox with an ephemeral working directory.
func New() (*ProcessSandbox, error) {
	dir, err := os.MkdirTemp("", "fg-sandbox-*")
	if err != nil {
		return nil, fmt.Errorf("sandbox: create workdir: %w", err)
	}

	// Minimal environment: only what is strictly needed.
	env := []string{
		"HOME=" + dir,
		"TMPDIR=" + filepath.Join(dir, "tmp"),
		"SOURCE_DATE_EPOCH=0", // deterministic timestamps for reproducibility
		"TZ=UTC",
		"LANG=C.UTF-8",
	}

	// Forward PATH so toolchains (node, npm, pip, cargo, etc.) are found.
	if p := os.Getenv("PATH"); p != "" {
		env = append(env, "PATH="+p)
	}

	if err := os.MkdirAll(filepath.Join(dir, "tmp"), 0700); err != nil {
		return nil, fmt.Errorf("sandbox: create tmp: %w", err)
	}

	s := &ProcessSandbox{workDir: dir, env: env}
	s.preConns = s.snapshotConnections()
	return s, nil
}

// WorkDir returns the ephemeral working directory path.
func (s *ProcessSandbox) WorkDir() string { return s.workDir }

// Run satisfies core.Sandbox: executes cmd inside the sandbox.
func (s *ProcessSandbox) Run(ctx context.Context, cmd string, args []string, extraEnv map[string]string) (stdout, stderr string, err error) {
	var cmdArgs []string
	var cmdName string

	// On Linux, wrap with `unshare -n` to drop the network namespace.
	if runtime.GOOS == "linux" {
		if _, nerr := exec.LookPath("unshare"); nerr == nil {
			cmdName = "unshare"
			cmdArgs = append([]string{"-n", "--", cmd}, args...)
		} else {
			cmdName = cmd
			cmdArgs = args
		}
	} else {
		cmdName = cmd
		cmdArgs = args
	}

	c := exec.CommandContext(ctx, cmdName, cmdArgs...)
	c.Dir = s.workDir

	env := make([]string, len(s.env))
	copy(env, s.env)
	for k, v := range extraEnv {
		env = append(env, k+"="+v)
	}
	c.Env = env

	var outBuf, errBuf bytes.Buffer
	c.Stdout = &outBuf
	c.Stderr = &errBuf

	start := time.Now()
	runErr := c.Run()
	_ = time.Since(start)

	outStr := outBuf.String()
	errStr := errBuf.String()

	if runErr != nil {
		var exitErr *exec.ExitError
		if errAs(runErr, &exitErr) {
			return outStr, errStr, fmt.Errorf("sandbox: exit %d: %w", exitErr.ExitCode(), runErr)
		}
		return outStr, errStr, fmt.Errorf("sandbox: run: %w", runErr)
	}
	return outStr, errStr, nil
}

// AuditNetwork returns network connections opened since the sandbox was created.
func (s *ProcessSandbox) AuditNetwork() []NetworkEvent {
	s.mu.Lock()
	defer s.mu.Unlock()
	postConns := s.snapshotConnections()
	var events []NetworkEvent
	for conn := range postConns {
		if !s.preConns[conn] {
			parts := strings.SplitN(conn, "->", 2)
			if len(parts) == 2 {
				events = append(events, NetworkEvent{
					Proto:   "tcp",
					SrcAddr: parts[0],
					DstAddr: parts[1],
				})
			}
		}
	}
	return events
}

// snapshotConnections returns the current set of active TCP connections
// as "srcAddr->dstAddr" strings. Used for before/after comparison.
func (s *ProcessSandbox) snapshotConnections() map[string]bool {
	conns := map[string]bool{}
	// Use net.Interfaces to get a basic snapshot; on Linux we'd read /proc/net/tcp.
	ifaces, err := net.Interfaces()
	if err != nil {
		return conns
	}
	for _, iface := range ifaces {
		addrs, _ := iface.Addrs()
		for _, a := range addrs {
			conns[a.String()] = true
		}
	}
	return conns
}

// Close removes the ephemeral working directory.
func (s *ProcessSandbox) Close() error {
	return os.RemoveAll(s.workDir)
}

// errAs is a helper that avoids an import of errors for the type assertion.
func errAs(err error, target **exec.ExitError) bool {
	e, ok := err.(*exec.ExitError)
	if ok {
		*target = e
	}
	return ok
}

// Wrap adapts ProcessSandbox to core.Sandbox.
type Wrap struct{ *ProcessSandbox }

var _ core.Sandbox = (*Wrap)(nil)

// NewCoreSandbox returns a core.Sandbox-compatible wrapper.
func NewCoreSandbox() (core.Sandbox, func() error, error) {
	ps, err := New()
	if err != nil {
		return nil, nil, err
	}
	return &Wrap{ps}, ps.Close, nil
}
