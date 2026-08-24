package handlers

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

var allowedCommands = map[string]bool{
	"scan":       true,
	"sbom":       true,
	"sign":       true,
	"verify":     true,
	"advisory":   true,
	"audit":      true,
	"doctor":     true,
	"update":     true,
	"version":    true,
	"help":       true,
	"provenance": true,
	"config":     true,
	"intel":      true,
	"sig":        true,
	"policy":     true,
	"stats":      true,
	"license":    true,
	"patch":      true,
	"monitor":    true,
	"debug":      true,
}

var blockedCommands = map[string]bool{
	"serve": true,
	"setup": true,
}

func fgctlPath() string {
	if p, err := exec.LookPath("fgctl"); err == nil {
		return p
	}
	home, _ := os.UserHomeDir()
	if home != "" {
		candidates := []string{
			filepath.Join(home, ".local", "bin", "fgctl"),
			filepath.Join(home, ".local", "bin", "fgctl.exe"),
		}
		for _, c := range candidates {
			if _, err := os.Stat(c); err == nil {
				return c
			}
		}
	}
	return "fgctl"
}

func (h *Handler) TerminalExec(c *gin.Context) {
	var req struct {
		Command string `json:"command"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	cmd := strings.TrimSpace(req.Command)
	if cmd == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "empty command"})
		return
	}

	// Strip leading "fgctl " if present
	cmd = strings.TrimPrefix(cmd, "fgctl ")

	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "empty command"})
		return
	}

	subCmd := parts[0]
	if blockedCommands[subCmd] {
		c.JSON(http.StatusForbidden, gin.H{"error": fmt.Sprintf("command %q is not allowed via web terminal", subCmd)})
		return
	}
	if !allowedCommands[subCmd] {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("unknown command %q — type 'help' for available commands", subCmd)})
		return
	}

	// Validate no shell injection characters
	for _, ch := range cmd {
		if ch == '|' || ch == '&' || ch == ';' || ch == '`' || ch == '$' || ch == '(' || ch == ')' || ch == '{' || ch == '}' || ch == '<' || ch == '>' {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid characters in command"})
			return
		}
	}

	args := make([]string, 0, len(parts)+2)
	args = append(args, parts[0])
	args = append(args, "--no-color", "--no-banner")
	args = append(args, parts[1:]...)

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Minute)
	defer cancel()

	proc := exec.CommandContext(ctx, fgctlPath(), args...)
	proc.Env = append(proc.Environ(), "NO_COLOR=1", "TERM=dumb")

	stdout, err := proc.StdoutPipe()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create pipe"})
		return
	}
	proc.Stderr = proc.Stdout

	if err := proc.Start(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to start: %v", err)})
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	flusher, _ := c.Writer.(http.Flusher)

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		fmt.Fprintf(c.Writer, "data: %s\n\n", line)
		if flusher != nil {
			flusher.Flush()
		}
	}

	exitCode := 0
	if err := proc.Wait(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}
	}

	fmt.Fprintf(c.Writer, "event: done\ndata: %d\n\n", exitCode)
	if flusher != nil {
		flusher.Flush()
	}
}

func (h *Handler) TerminalCompletions(c *gin.Context) {
	commands := make([]string, 0, len(allowedCommands))
	for cmd := range allowedCommands {
		commands = append(commands, cmd)
	}

	examples := []gin.H{
		{"cmd": "scan npm/lodash@4.17.21", "desc": "Scan a specific package"},
		{"cmd": "scan .", "desc": "Scan current directory"},
		{"cmd": "scan npm/express@4.17.1 --sync", "desc": "Scan and sync to dashboard"},
		{"cmd": "scan npm/lodash@4.17.20 --sync --workspace Production", "desc": "Scan and sync to workspace"},
		{"cmd": "sbom --recipe=npm --package=lodash --version=4.17.21", "desc": "Generate SBOM"},
		{"cmd": "audit system", "desc": "Audit all installed packages"},
		{"cmd": "doctor", "desc": "Check environment health"},
		{"cmd": "doctor --fix", "desc": "Auto-repair failing checks"},
		{"cmd": "update", "desc": "Update threat signatures"},
		{"cmd": "version", "desc": "Show version info"},
		{"cmd": "intel stats", "desc": "Show intelligence stats"},
		{"cmd": "help", "desc": "Show all commands"},
	}

	c.JSON(http.StatusOK, gin.H{
		"commands": commands,
		"examples": examples,
	})
}

// TerminalStream handles SSE streaming for the terminal — used by the
// streaming variant of terminal exec. Kept as alias for TerminalExec since
// the handler already streams via SSE.
func (h *Handler) TerminalStream(c *gin.Context) {
	h.TerminalExec(c)
}

// pipe drains an io.Reader and returns its content as a string.
func pipe(r io.Reader) string {
	b, _ := io.ReadAll(r)
	return string(b)
}
