// Package remotescan discovers and pulls dependency manifest files from a
// remote host over SSH so they can be scanned through the existing local
// scan pipeline (internal/localscanner). It never writes to the remote
// host and never installs anything there — only read-only exec commands
// (echo, find, cat) are ever run remotely.
package remotescan

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
)

// Config holds connection parameters for a remote scan.
type Config struct {
	User             string // parsed from "user@host"
	Host             string // parsed from "user@host"
	Port             int    // default 22
	IdentityPath     string // explicit --identity path (CLI), "" = auto-discover
	PrivateKeyPEM    []byte // explicit in-memory key material (API/dashboard callers) — takes priority over IdentityPath/agent when set, never written to disk or logged
	AcceptNewHostKey bool   // opt-in trust for a host absent from known_hosts
	RemotePath       string // remote search root; "" = resolved to $HOME
	MaxDepth         int    // find -maxdepth; 0 = unlimited
	ConnectTimeout   time.Duration
}

// Client wraps an established SSH connection scoped to one scan run.
type Client struct {
	conn *ssh.Client
	cfg  Config
}

// ParseTarget splits "user@host" or "user@host:port" into a Config. port is
// used when the target string does not itself specify a port.
func ParseTarget(target string, port int) (Config, error) {
	at := strings.Index(target, "@")
	if at < 0 {
		return Config{}, fmt.Errorf("remote target %q must be in the form user@host", target)
	}
	user := target[:at]
	hostport := target[at+1:]
	if user == "" || hostport == "" {
		return Config{}, fmt.Errorf("remote target %q must be in the form user@host", target)
	}

	host := hostport
	if h, p, err := net.SplitHostPort(hostport); err == nil {
		host = h
		if pn, err := strconv.Atoi(p); err == nil {
			port = pn
		}
	}
	if host == "" {
		return Config{}, fmt.Errorf("remote target %q must be in the form user@host", target)
	}
	return Config{User: user, Host: host, Port: port}, nil
}

// Connect dials the target, verifies the host key against ~/.ssh/known_hosts,
// and authenticates via SSH agent then identity files. It never silently
// downgrades host-key security: a key mismatch against a known_hosts entry
// always fails regardless of flags; a host absent from known_hosts requires
// cfg.AcceptNewHostKey.
func Connect(cfg Config) (*Client, error) {
	hostKeyCallback, err := buildHostKeyCallback(cfg.AcceptNewHostKey)
	if err != nil {
		return nil, err
	}

	authMethods, authDesc, err := buildAuthMethods(cfg.IdentityPath, cfg.PrivateKeyPEM)
	if err != nil {
		return nil, err
	}

	addr := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
	sshCfg := &ssh.ClientConfig{
		User:            cfg.User,
		Auth:            authMethods,
		HostKeyCallback: hostKeyCallback,
		Timeout:         cfg.ConnectTimeout,
	}

	conn, err := ssh.Dial("tcp", addr, sshCfg)
	if err != nil {
		return nil, classify(err, cfg, addr, authDesc)
	}
	return &Client{conn: conn, cfg: cfg}, nil
}

// Close closes the underlying SSH connection. There is no remote state to
// clean up — nothing was ever written to the remote host.
func (c *Client) Close() error {
	if c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

// buildHostKeyCallback wraps knownhosts.New with MITM-safe semantics: a
// mismatch against a recorded key always fails; an absent host is accepted
// only when acceptNew is true, and is never written back to known_hosts.
func buildHostKeyCallback(acceptNew bool) (ssh.HostKeyCallback, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("remotescan: resolve home dir: %w", err)
	}
	knownHostsPath := filepath.Join(home, ".ssh", "known_hosts")

	if _, err := os.Stat(knownHostsPath); os.IsNotExist(err) {
		// No known_hosts file at all — every host is "new".
		if !acceptNew {
			return nil, fmt.Errorf(
				"no ~/.ssh/known_hosts found — connect once with the system ssh client to record " +
					"the host key, or re-run with --accept-new-host-key to trust it for this run only",
			)
		}
		return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			return nil
		}, nil
	}

	base, err := knownhosts.New(knownHostsPath)
	if err != nil {
		return nil, fmt.Errorf("remotescan: parse known_hosts: %w", err)
	}

	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		err := base(hostname, remote, key)
		if err == nil {
			return nil
		}
		var keyErr *knownhosts.KeyError
		if !errors.As(err, &keyErr) {
			return err // malformed known_hosts or other real error — never swallow
		}
		if len(keyErr.Want) > 0 {
			// A different key was recorded for this host — classic MITM
			// signature. Always hard-fail, no flag can override this.
			return fmt.Errorf(
				"REMOTE HOST IDENTIFICATION HAS CHANGED for %s! Expected a previously-recorded key "+
					"but got fingerprint %s. This could indicate a man-in-the-middle attack — refusing to connect.\n"+
					"If you know the host key legitimately changed, remove the old entry from ~/.ssh/known_hosts "+
					"and re-run with --accept-new-host-key.",
				hostname, ssh.FingerprintSHA256(key),
			)
		}
		// Host simply not present in known_hosts.
		if !acceptNew {
			return fmt.Errorf(
				"host %s is not in ~/.ssh/known_hosts (fingerprint %s). "+
					"Verify this fingerprint out-of-band, then re-run with --accept-new-host-key to proceed.",
				hostname, ssh.FingerprintSHA256(key),
			)
		}
		return nil
	}, nil
}

// buildAuthMethods assembles SSH auth methods in priority order: an
// explicit in-memory private key (API/dashboard callers) first, then
// ssh-agent, then identity files (CLI callers). Password auth is
// intentionally out of scope for this pass. Returns a human-readable
// description of what was tried, for use in auth-failure error messages —
// this description never includes key material, only its source.
func buildAuthMethods(identityPath string, privateKeyPEM []byte) ([]ssh.AuthMethod, string, error) {
	var methods []ssh.AuthMethod
	var tried []string

	if len(privateKeyPEM) > 0 {
		signer, err := ssh.ParsePrivateKey(privateKeyPEM)
		if err != nil {
			return nil, "", fmt.Errorf("remotescan: parse provided private key: %w", err)
		}
		methods = append(methods, ssh.PublicKeys(signer))
		tried = append(tried, "provided private key")
		// An explicitly-provided key is the caller's sole stated intent —
		// skip agent/identity-file auto-discovery entirely rather than
		// silently trying other credentials the caller didn't ask for.
		return methods, strings.Join(tried, ", "), nil
	}

	if sock := os.Getenv("SSH_AUTH_SOCK"); sock != "" {
		if conn, err := net.Dial("unix", sock); err == nil {
			agentClient := agent.NewClient(conn)
			methods = append(methods, ssh.PublicKeysCallback(agentClient.Signers))
			if signers, err := agentClient.Signers(); err == nil {
				tried = append(tried, fmt.Sprintf("ssh-agent (%d keys)", len(signers)))
			} else {
				tried = append(tried, "ssh-agent (0 keys)")
			}
		}
		// Non-fatal if the agent socket exists but dialing fails — fall
		// through to identity files.
	}

	var candidates []string
	if identityPath != "" {
		candidates = []string{identityPath}
	} else {
		home, _ := os.UserHomeDir()
		if home != "" {
			candidates = []string{
				filepath.Join(home, ".ssh", "id_ed25519"),
				filepath.Join(home, ".ssh", "id_rsa"),
			}
		}
	}

	for _, path := range candidates {
		data, err := os.ReadFile(path)
		if err != nil {
			if identityPath != "" && path == identityPath {
				return nil, "", fmt.Errorf("remotescan: read identity file %s: %w", path, err)
			}
			continue // auto-discovered candidate that doesn't exist — skip silently
		}
		signer, err := ssh.ParsePrivateKey(data)
		if err != nil {
			if identityPath != "" && path == identityPath {
				return nil, "", fmt.Errorf("remotescan: parse identity file %s: %w", path, err)
			}
			// Encrypted or unparseable auto-discovered key — key-only,
			// non-interactive auth is out of scope for this pass; skip.
			continue
		}
		methods = append(methods, ssh.PublicKeys(signer))
		tried = append(tried, "identity file "+path)
	}

	if len(methods) == 0 {
		return nil, "", fmt.Errorf(
			"no usable SSH credentials found: no ssh-agent (SSH_AUTH_SOCK unset or empty) and no " +
				"identity file at ~/.ssh/id_ed25519, ~/.ssh/id_rsa, or --identity path",
		)
	}
	return methods, strings.Join(tried, ", "), nil
}

// classify wraps a dial/auth error with a friendlier message, without
// discarding the underlying error.
func classify(err error, cfg Config, addr, authDesc string) error {
	var netErr net.Error
	if errors.As(err, &netErr) {
		return fmt.Errorf(
			"cannot reach %s — check the hostname/port and that sshd is running: %w", addr, err,
		)
	}
	if strings.Contains(err.Error(), "unable to authenticate") {
		return fmt.Errorf(
			"authentication failed for %s@%s — tried: %s. Check --identity or SSH_AUTH_SOCK: %w",
			cfg.User, cfg.Host, authDesc, err,
		)
	}
	return fmt.Errorf("remotescan: connect to %s: %w", addr, err)
}
