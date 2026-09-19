package config

import (
	"path/filepath"

	"github.com/joho/godotenv"
)

// loadEnvFiles reads .env and .env.local from repo root into the process environment.
// Variables already set in the shell are not overwritten.
func loadEnvFiles(repoRoot string) {
	_ = godotenv.Load(filepath.Join(repoRoot, ".env"))
	_ = godotenv.Load(filepath.Join(repoRoot, ".env.local"))
}
