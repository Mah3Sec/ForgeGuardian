package handlers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/mah3sec/forgeguardian/internal/localscanner"
	"github.com/mah3sec/forgeguardian/internal/remotescan"
)

func errNoManifestsFound(target, searchRoot string) error {
	return fmt.Errorf("no dependency manifests found on %s under %s", target, searchRoot)
}

func errAllPullsFailed(target string, count int) error {
	return fmt.Errorf("failed to pull any of the %d discovered manifest file(s) from %s", count, target)
}

// TriggerRemoteScan connects to a remote host over SSH, discovers dependency
// manifest files, pulls them into a local temp dir, and scans them through
// the same pipeline a local project scan uses — the dashboard/API
// equivalent of `fgctl scan --remote`. Runs async on the same job registry
// TriggerScan uses, so the dashboard polls it identically.
//
// The private key (if supplied) is parsed in-memory for this one request
// and is never written to disk or included in any response — only its
// presence (not its content) is ever logged.
// POST /api/v1/scan/remote
func (h *Handler) TriggerRemoteScan(c *gin.Context) {
	var req struct {
		Target           string `json:"target"             binding:"required"` // "user@host" or "user@host:port"
		Port             int    `json:"port"`
		PrivateKey       string `json:"private_key"` // PEM text pasted by the caller; never persisted
		RemotePath       string `json:"remote_path"`
		AcceptNewHostKey bool   `json:"accept_new_host_key"`
		MaxDepth         int    `json:"max_depth"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.PrivateKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "private_key is required — the API server has no ssh-agent or identity file of its own to fall back to"})
		return
	}
	port := req.Port
	if port == 0 {
		port = 22
	}

	cfg, err := remotescan.ParseTarget(req.Target, port)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	cfg.PrivateKeyPEM = []byte(req.PrivateKey)
	cfg.RemotePath = req.RemotePath
	cfg.AcceptNewHostKey = req.AcceptNewHostKey
	cfg.MaxDepth = req.MaxDepth
	cfg.ConnectTimeout = 15 * time.Second

	storePath := h.cfg.GetIntelStorePath()

	job := h.jobs.submit(func() (any, error) {
		agentBus.publish(AgentEvent{
			Type:    "start",
			Message: fmt.Sprintf("Connecting to remote host: %s", req.Target),
		})

		client, err := remotescan.Connect(cfg)
		if err != nil {
			agentBus.publish(AgentEvent{
				Type:    "error",
				Message: fmt.Sprintf("Remote connection failed: %s", err.Error()),
			})
			return nil, err
		}
		defer client.Close()

		searchRoot := cfg.RemotePath
		if searchRoot == "" {
			home, err := client.ResolveHome()
			if err != nil {
				return nil, err
			}
			searchRoot = home
		}

		agentBus.publish(AgentEvent{
			Type:    "step",
			Message: fmt.Sprintf("Discovering manifests on %s under %s", req.Target, searchRoot),
		})

		files, err := client.Discover(searchRoot, cfg.MaxDepth)
		if err != nil {
			return nil, err
		}
		if len(files) == 0 {
			return nil, errNoManifestsFound(req.Target, searchRoot)
		}

		agentBus.publish(AgentEvent{
			Type:    "step",
			Message: fmt.Sprintf("Found %d manifests, pulling files", len(files)),
		})

		tmpDir, err := remotescan.NewTempDir()
		if err != nil {
			return nil, err
		}
		defer remotescan.Cleanup(tmpDir, false, h.log)

		pullResults := client.Pull(files, tmpDir)
		pulledAny := false
		var failedPaths []string
		for _, r := range pullResults {
			if r.Err == nil {
				pulledAny = true
			} else {
				failedPaths = append(failedPaths, r.Remote.RemotePath)
			}
		}
		if !pulledAny {
			return nil, errAllPullsFailed(req.Target, len(files))
		}
		if len(failedPaths) > 0 {
			h.log.Warn("remote scan: some manifest files failed to pull",
				"target", req.Target, "failed_count", len(failedPaths))
		}

		agentBus.publish(AgentEvent{
			Type:    "step",
			Message: fmt.Sprintf("Scanning pulled manifests from %s", req.Target),
		})

		ls := localscanner.New(storePath)
		result, err := ls.Scan(c.Request.Context(), tmpDir, nil)
		if err != nil {
			agentBus.publish(AgentEvent{
				Type:    "error",
				Message: fmt.Sprintf("Remote scan failed: %s", err.Error()),
			})
			return nil, err
		}

		agentBus.publish(AgentEvent{
			Type:    "done",
			Message: fmt.Sprintf("Remote scan complete: %s", req.Target),
		})

		return result, nil
	})

	c.JSON(http.StatusAccepted, job)
}
