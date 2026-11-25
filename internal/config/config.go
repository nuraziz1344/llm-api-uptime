package config

import (
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	Port             string
	DataDir          string
	DBPath           string
	EncryptionKeyEnv string
	DevMode          bool
	AllowedOrigin    string
}

func Load() Config {
	port := getEnv("PORT", "8080")
	dataDir := getEnv("DATA_DIR", "./data")
	dbPath := getEnv("DB_PATH", filepath.Join(dataDir, "uptime.db"))
	devMode := strings.EqualFold(getEnv("DEV_MODE", "false"), "true") || getEnv("DEV_MODE", "") == "1"
	allowedOrigin := getEnv("CORS_ORIGIN", "http://localhost:5173")

	return Config{
		Port:             port,
		DataDir:          dataDir,
		DBPath:           dbPath,
		EncryptionKeyEnv: os.Getenv("APP_ENCRYPTION_KEY"),
		DevMode:          devMode,
		AllowedOrigin:    allowedOrigin,
	}
}

func getEnv(key, fallback string) string {
	if val := strings.TrimSpace(os.Getenv(key)); val != "" {
		return val
	}
	return fallback
}
