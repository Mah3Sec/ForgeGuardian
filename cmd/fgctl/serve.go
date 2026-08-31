package main

import (
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/mah3sec/forgeguardian/internal/api/server"
	"github.com/mah3sec/forgeguardian/internal/ui"
)

func runServe(args []string, _ *slog.Logger, p *ui.Printer) error {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	portFlag := fs.String("port", "", "server port (default: PORT env or 8080)")
	dashboardFlag := fs.String("dashboard", "", "path to built dashboard files")
	fs.Parse(args)

	setDefaultEnv("FG_COOKIE_SECURE", "true")

	if os.Getenv("FG_SESSION_SECRET") == "" {
		home, _ := os.UserHomeDir()
		secretPath := filepath.Join(home, ".forgeguardian", ".session-secret")
		if data, err := os.ReadFile(secretPath); err == nil && len(data) > 0 {
			os.Setenv("FG_SESSION_SECRET", string(data))
		} else {
			b := make([]byte, 32)
			rand.Read(b)
			secret := hex.EncodeToString(b)
			os.Setenv("FG_SESSION_SECRET", secret)
			os.MkdirAll(filepath.Dir(secretPath), 0o700)
			os.WriteFile(secretPath, []byte(secret), 0o600)
		}
	}

	if *portFlag != "" {
		os.Setenv("PORT", *portFlag)
	}
	setDefaultEnv("PORT", "8080")

	if *dashboardFlag != "" {
		os.Setenv("DASHBOARD_DIR", *dashboardFlag)
	}
	if os.Getenv("DASHBOARD_DIR") == "" {
		if dir := findDashboardDir(); dir != "" {
			os.Setenv("DASHBOARD_DIR", dir)
		}
	}

	if os.Getenv("FG_DASHBOARD_ORIGIN") == "" {
		port := os.Getenv("PORT")
		os.Setenv("FG_DASHBOARD_ORIGIN", fmt.Sprintf("http://localhost:%s", port))
	}

	cfg := server.LoadConfig()

	port := cfg.Port
	dashDir := cfg.DashboardDir

	if !p.JSON {
		p.Banner()
		fmt.Println()
		fmt.Printf("  Starting ForgeGuardian API server on port %s\n", port)
		if dashDir != "" {
			fmt.Printf("  Dashboard: http://localhost:%s\n", port)
		} else {
			fmt.Printf("  API only (no dashboard directory found)\n")
		}
		if cfg.AdminEmail != "" {
			fmt.Printf("  Login: %s\n", cfg.AdminEmail)
		} else {
			fmt.Printf("  Auth: disabled (set FG_ADMIN_EMAIL + FG_ADMIN_PASSWORD to enable)\n")
		}
		fmt.Println()
	}

	return server.Run(cfg)
}

func setDefaultEnv(key, value string) {
	if os.Getenv(key) == "" {
		os.Setenv(key, value)
	}
}

func findDashboardDir() string {
	candidates := []string{
		"dashboard/dist",
		"../dashboard/dist",
	}

	exe, err := os.Executable()
	if err == nil {
		exeDir := filepath.Dir(exe)
		candidates = append(candidates,
			filepath.Join(exeDir, "dashboard"),
			filepath.Join(exeDir, "..", "dashboard", "dist"),
			filepath.Join(exeDir, "..", "share", "forgeguardian", "dashboard"),
		)
	}

	home, err := os.UserHomeDir()
	if err == nil {
		candidates = append(candidates,
			filepath.Join(home, ".forgeguardian", "dashboard"),
		)
	}

	for _, dir := range candidates {
		abs, err := filepath.Abs(dir)
		if err != nil {
			continue
		}
		if _, err := os.Stat(filepath.Join(abs, "index.html")); err == nil {
			return abs
		}
	}
	return ""
}
