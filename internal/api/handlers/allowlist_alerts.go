package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	dbsqlc "github.com/mah3sec/forgeguardian/internal/api/db/sqlc"
)

// ─── Allowlist endpoints ─────────────────────────────────────────────────────

// ListAllowlist returns all allowlisted packages.
// GET /api/v1/allowlist
func (h *Handler) ListAllowlist(c *gin.Context) {
	if h.db == nil {
		c.JSON(http.StatusOK, gin.H{"allowlist": []any{}, "total": 0})
		return
	}
	entries, err := h.db.ListAllowlist(c.Request.Context())
	if err != nil {
		h.log.Error("ListAllowlist failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if entries == nil {
		entries = []dbsqlc.AllowlistEntry{}
	}
	c.JSON(http.StatusOK, gin.H{"allowlist": entries, "total": len(entries)})
}

// AddAllowlist adds or updates a package in the allowlist.
// POST /api/v1/allowlist   body: {"ecosystem":"npm","package":"lodash","reason":"internal fork","added_by":"alice"}
func (h *Handler) AddAllowlist(c *gin.Context) {
	if h.db == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "allowlist persistence requires a database — set DATABASE_URL to enable"})
		return
	}
	var req struct {
		Ecosystem string `json:"ecosystem"`
		Package   string `json:"package"   binding:"required"`
		Reason    string `json:"reason"`
		AddedBy   string `json:"added_by"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.AddedBy == "" {
		req.AddedBy = "api"
	}
	entry, err := h.db.UpsertAllowlist(c.Request.Context(), req.Ecosystem, req.Package, req.Reason, req.AddedBy)
	if err != nil {
		h.log.Error("UpsertAllowlist failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, entry)
}

// DeleteAllowlist removes a package from the allowlist by ID.
// DELETE /api/v1/allowlist/:id
func (h *Handler) DeleteAllowlist(c *gin.Context) {
	if h.db == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "allowlist persistence requires a database — set DATABASE_URL to enable"})
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := h.db.DeleteAllowlist(c.Request.Context(), id); err != nil {
		h.log.Error("DeleteAllowlist failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// CheckAllowlist checks whether a specific package is allowlisted.
// GET /api/v1/allowlist/check?package=lodash&ecosystem=npm
func (h *Handler) CheckAllowlist(c *gin.Context) {
	if h.db == nil {
		pkg := c.Query("package")
		eco := c.Query("ecosystem")
		c.JSON(http.StatusOK, gin.H{"allowlisted": false, "package": pkg, "ecosystem": eco})
		return
	}
	pkg := c.Query("package")
	eco := c.Query("ecosystem")
	if pkg == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "package query param required"})
		return
	}
	ok, err := h.db.IsAllowlisted(c.Request.Context(), pkg, eco)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"allowlisted": ok, "package": pkg, "ecosystem": eco})
}

// ─── Alerts endpoints ────────────────────────────────────────────────────────

// ListAlerts returns paginated alerts.
// GET /api/v1/alerts?page=1&page_size=50&severity=HIGH&dismissed=false
func (h *Handler) ListAlerts(c *gin.Context) {
	if h.db == nil {
		c.JSON(http.StatusOK, gin.H{"page": 1, "page_size": 50, "total": 0, "alerts": []any{}})
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("page_size", "50"))
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 200 {
		size = 50
	}
	offset := int32((page - 1) * size)
	severity := c.Query("severity")

	var dismissed *bool
	if d := c.Query("dismissed"); d != "" {
		v := d == "true"
		dismissed = &v
	}

	alerts, err := h.db.ListAlerts(c.Request.Context(), dismissed, severity, int32(size), offset)
	if err != nil {
		h.log.Error("ListAlerts failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	total, err := h.db.CountAlerts(c.Request.Context(), dismissed, severity)
	if err != nil {
		h.log.Error("CountAlerts failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if alerts == nil {
		alerts = []dbsqlc.Alert{}
	}
	c.JSON(http.StatusOK, gin.H{
		"page":      page,
		"page_size": size,
		"total":     total,
		"alerts":    alerts,
	})
}

// CreateAlert persists a new alert. Used by worker/scanner to surface events.
// POST /api/v1/alerts   body: {"type":"finding_critical","message":"...","severity":"CRITICAL","package_name":"lodash","ecosystem":"npm","version":"4.17.20"}
func (h *Handler) CreateAlert(c *gin.Context) {
	if !h.dbAvailable(c) {
		return
	}
	var req struct {
		Type        string  `json:"type"         binding:"required"`
		Message     string  `json:"message"      binding:"required"`
		Severity    string  `json:"severity"`
		PackageName *string `json:"package_name"`
		Ecosystem   *string `json:"ecosystem"`
		Version     *string `json:"version"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Severity == "" {
		req.Severity = "INFORMATIONAL"
	}
	alert, err := h.db.InsertAlert(c.Request.Context(), req.Type, req.Message, req.Severity, req.PackageName, req.Ecosystem, req.Version, nil)
	if err != nil {
		h.log.Error("InsertAlert failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// Fan out to SSE listeners so AlertsPage sees it immediately.
	agentBus.publish(AgentEvent{
		Type:    "step",
		Message: req.Message,
	})
	c.JSON(http.StatusCreated, alert)
}

// DismissAlert marks an alert as acknowledged.
// POST /api/v1/alerts/:id/dismiss
func (h *Handler) DismissAlert(c *gin.Context) {
	if !h.dbAvailable(c) {
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := h.db.DismissAlert(c.Request.Context(), id); err != nil {
		h.log.Error("DismissAlert failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
