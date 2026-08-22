package server

import "os"

// Config holds all runtime configuration for the API server.
type Config struct {
	Port            string
	DatabaseURL     string
	AnthropicAPIKey string
	MinIOEndpoint   string
	MinIOAccessKey  string
	MinIOSecretKey  string
	RekorURL        string
	DepTrackURL     string
	DepTrackAPIKey  string
	IntelStorePath  string
	APIKey          string
	AdminEmail      string
	AdminPassword   string
	SessionSecret   string
	CookieSecure    bool
	DashboardOrigin string
	DashboardDir    string
}

// LoadConfig reads configuration from environment variables.
func LoadConfig() *Config {
	return &Config{
		Port:            getEnv("PORT", "8080"),
		DatabaseURL:     os.Getenv("DATABASE_URL"),
		AnthropicAPIKey: os.Getenv("ANTHROPIC_API_KEY"),
		MinIOEndpoint:   getEnv("MINIO_ENDPOINT", "localhost:9000"),
		MinIOAccessKey:  getEnv("MINIO_ACCESS_KEY", "minioadmin"),
		MinIOSecretKey:  getEnv("MINIO_SECRET_KEY", "minioadmin"),
		RekorURL:        getEnv("REKOR_URL", ""),
		DepTrackURL:     getEnv("DEPENDENCY_TRACK_URL", "http://localhost:8081"),
		DepTrackAPIKey:  os.Getenv("DEPENDENCY_TRACK_API_KEY"),
		IntelStorePath:  getEnv("INTEL_STORE_PATH", "~/.forgeguardian/signatures.json"),
		APIKey:          os.Getenv("FG_API_KEY"),
		AdminEmail:      os.Getenv("FG_ADMIN_EMAIL"),
		AdminPassword:   os.Getenv("FG_ADMIN_PASSWORD"),
		SessionSecret:   os.Getenv("FG_SESSION_SECRET"),
		CookieSecure:    getEnv("FG_COOKIE_SECURE", "true") == "true",
		DashboardOrigin: getEnv("FG_DASHBOARD_ORIGIN", "http://localhost:3000"),
		DashboardDir:    os.Getenv("DASHBOARD_DIR"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
