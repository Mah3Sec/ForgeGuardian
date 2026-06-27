package remotescan

import (
	"bytes"
	"fmt"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/mah3sec/forgeguardian/internal/localscanner"
)

// DiscoveredFile is one manifest found on the remote host.
type DiscoveredFile struct {
	RemotePath string // absolute path on the remote host
	RelPath    string // path relative to the search root, used for local mirroring
	Ecosystem  string
}

// ResolveHome runs `echo $HOME` over SSH to resolve the default search root
// when no explicit remote path is configured. This is a plain read-only
// shell builtin, not a filesystem write.
func (c *Client) ResolveHome() (string, error) {
	out, err := c.run("echo $HOME")
	if err != nil {
		return "", fmt.Errorf("remotescan: resolve remote $HOME: %w", err)
	}
	home := strings.TrimSpace(out)
	if home == "" {
		return "", fmt.Errorf("remotescan: remote $HOME resolved to an empty string")
	}
	return home, nil
}

// Discover runs a single read-only `find` over SSH and returns matching
// manifest files. The filename and prune lists come from
// localscanner.ManifestNames / SkipDirNames — the same source of truth the
// local walker uses — so the two never drift out of sync.
func (c *Client) Discover(searchRoot string, maxDepth int) ([]DiscoveredFile, error) {
	cmd := buildFindCommand(searchRoot, maxDepth)
	out, err := c.runCapturingStderr(cmd)
	if err != nil {
		return nil, fmt.Errorf("remote discovery failed: %w", err)
	}

	var files []DiscoveredFile
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		base := path.Base(line)
		eco, ok := localscanner.ManifestNames[base]
		if !ok {
			continue
		}
		rel := strings.TrimPrefix(line, searchRoot)
		rel = strings.TrimPrefix(rel, "/")
		files = append(files, DiscoveredFile{RemotePath: line, RelPath: rel, Ecosystem: eco})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].RemotePath < files[j].RemotePath })
	return files, nil
}

// buildFindCommand constructs a find(1) invocation from
// localscanner.SkipDirNames / ManifestNames, so a new ecosystem or prune
// directory added to the local walker is automatically picked up here too.
func buildFindCommand(searchRoot string, maxDepth int) string {
	var b strings.Builder
	b.WriteString("find ")
	b.WriteString(shellQuote(searchRoot))
	if maxDepth > 0 {
		b.WriteString(" -maxdepth ")
		b.WriteString(strconv.Itoa(maxDepth))
	}

	pruneNames := sortedKeys(localscanner.SkipDirNames)
	b.WriteString(" \\( -type d \\( ")
	for i, name := range pruneNames {
		if i > 0 {
			b.WriteString(" -o ")
		}
		b.WriteString("-name ")
		b.WriteString(shellQuote(name))
	}
	b.WriteString(" \\) -prune \\)")

	manifestNames := sortedKeysStr(localscanner.ManifestNames)
	b.WriteString(" -o -type f \\( ")
	for i, name := range manifestNames {
		if i > 0 {
			b.WriteString(" -o ")
		}
		b.WriteString("-name ")
		b.WriteString(shellQuote(name))
	}
	b.WriteString(" \\) -print")

	return b.String()
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func sortedKeysStr(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// shellQuote single-quotes s for safe inclusion in a remote shell command,
// escaping any embedded single quotes. Used for both the search root (which
// may come from --remote-path, user-controlled) and manifest/dir names
// (which are program-controlled but quoted identically for consistency).
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// run executes cmd over a fresh SSH session and returns stdout. Non-zero
// exit is treated as an error with stdout discarded, matching ssh.Session.Output.
func (c *Client) run(cmd string) (string, error) {
	session, err := c.conn.NewSession()
	if err != nil {
		return "", fmt.Errorf("open ssh session: %w", err)
	}
	defer session.Close()

	out, err := session.Output(cmd)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// runCapturingStderr executes cmd over a fresh SSH session and returns
// stdout; on a non-zero exit, the returned error includes the remote
// stderr text so failures like "Permission denied" are visible to the user.
func (c *Client) runCapturingStderr(cmd string) (string, error) {
	session, err := c.conn.NewSession()
	if err != nil {
		return "", fmt.Errorf("open ssh session: %w", err)
	}
	defer session.Close()

	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	if err := session.Run(cmd); err != nil {
		stderrText := strings.TrimSpace(stderr.String())
		if stderrText != "" {
			return "", fmt.Errorf("%s: %w", stderrText, err)
		}
		return "", err
	}
	return stdout.String(), nil
}
