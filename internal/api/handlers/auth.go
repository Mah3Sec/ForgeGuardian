package handlers

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/mah3sec/forgeguardian/internal/auth"
)

// sessionCookieName is the name of the dashboard session cookie.
const sessionCookieName = "fg_session"

// sessionTTL is how long an issued session token is valid for.
const sessionTTL = 24 * time.Hour

// Login authenticates the bootstrapped dashboard admin and, on success,
// sets a signed session cookie.
// POST /api/v1/auth/login   body: {"email":"...","password":"..."}
func (h *Handler) Login(c *gin.Context) {
	admin := h.cfg.GetAdminIdentity()
	if admin == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "login is not enabled on this server"})
		return
	}

	var req struct {
		Email    string `json:"email"    binding:"required"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Run the password check regardless of whether the email matches, then
	// combine both checks — avoids a cheap early-exit timing tell between
	// "unknown email" and "wrong password".
	pwErr := auth.CheckPassword(admin.PasswordHash, req.Password)
	emailMatches := strings.EqualFold(req.Email, admin.Email)
	if pwErr != nil || !emailMatches {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid email or password"})
		return
	}

	token, err := auth.IssueToken(admin.Email, h.cfg.GetSessionSecret(), sessionTTL)
	if err != nil {
		h.log.Error("failed to issue session token", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to issue session"})
		return
	}

	http.SetCookie(c.Writer, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   int(sessionTTL.Seconds()),
		Secure:   h.cfg.GetCookieSecure(),
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
	resp := gin.H{"ok": true}
	if admin.PasswordMustChange {
		resp["password_must_change"] = true
	}
	c.JSON(http.StatusOK, resp)
}

// Logout clears the dashboard session cookie.
// POST /api/v1/auth/logout
func (h *Handler) Logout(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		Secure:   h.cfg.GetCookieSecure(),
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ChangePassword lets the logged-in admin change their password.
// POST /api/v1/auth/password   body: {"current_password":"...","new_password":"..."}
func (h *Handler) ChangePassword(c *gin.Context) {
	admin := h.cfg.GetAdminIdentity()
	if admin == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "login is not enabled on this server"})
		return
	}

	tok, err := c.Cookie(sessionCookieName)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	if _, err := auth.ParseToken(tok, h.cfg.GetSessionSecret()); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}

	var req struct {
		CurrentPassword string `json:"current_password" binding:"required"`
		NewPassword     string `json:"new_password"     binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := auth.UpdateAdminPassword(admin, req.CurrentPassword, req.NewPassword); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.log.Info("admin password changed")
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// AuthMe reports the current session's authentication state.
// GET /api/v1/auth/me
func (h *Handler) AuthMe(c *gin.Context) {
	admin := h.cfg.GetAdminIdentity()
	if admin == nil {
		c.JSON(http.StatusOK, gin.H{"auth_enabled": false, "authenticated": false})
		return
	}

	tok, err := c.Cookie(sessionCookieName)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"auth_enabled": true, "authenticated": false})
		return
	}

	claims, err := auth.ParseToken(tok, h.cfg.GetSessionSecret())
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"auth_enabled": true, "authenticated": false})
		return
	}

	resp := gin.H{"auth_enabled": true, "authenticated": true, "email": claims.Email}
	if admin.PasswordMustChange {
		resp["password_must_change"] = true
	}
	c.JSON(http.StatusOK, resp)
}
