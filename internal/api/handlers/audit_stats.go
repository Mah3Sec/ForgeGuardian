package handlers

import (
	"net/http"
	"sort"

	"github.com/gin-gonic/gin"

	"github.com/mah3sec/forgeguardian/internal/intelligence"
)

// AuditStats returns runtime statistics about the loaded detection signature
// store — the same data `fgctl stats --json` prints from the CLI.
// GET /api/v1/audit/stats
func (h *Handler) AuditStats(c *gin.Context) {
	store, err := intelligence.LoadStoreWithLocalFallback(h.cfg.GetIntelStorePath())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "could not load signatures — run: fgctl update (" + err.Error() + ")",
		})
		return
	}

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

	// A store that's never been saved (fresh install, run: fgctl update)
	// has a zero-value UpdatedAt — formatting that yields "0001-01-01",
	// which renders in the dashboard as a bogus "1/1/1" date instead of
	// blank. Send empty string so the frontend's `data.signatures_updated
	// &&` guard actually hides it.
	updatedAt := ""
	if !store.UpdatedAt.IsZero() {
		updatedAt = store.UpdatedAt.Format("2006-01-02")
	}

	c.JSON(http.StatusOK, gin.H{
		"total_signatures":    len(store.Signatures),
		"malware_patterns":    counts[intelligence.SigMalwarePattern],
		"typosquat_targets":   counts[intelligence.SigTypoTarget],
		"behavioral_rules":    counts[intelligence.SigBehavioral],
		"blocklist_entries":   counts[intelligence.SigBlocklisted],
		"mcp_injection_rules": counts[intelligence.SigMCPInjection],
		"ecosystems_covered":  ecoList,
		"signatures_updated":  updatedAt,
	})
}
