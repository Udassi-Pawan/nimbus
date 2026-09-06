package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	Port        int
	LogLevel    string
	DatabaseURL string
	JWTSecret   string
}

func Load() (Config, error) {
	port := 8080
	if v := os.Getenv("NIMBUS_PORT"); v != "" {
		p, err := strconv.Atoi(v)
		if err != nil {
			return Config{}, fmt.Errorf("invalid NIMBUS_PORT: %w", err)
		}
		port = p
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		return Config{}, fmt.Errorf("JWT_SECRET is required")
	}

	logLevel := os.Getenv("NIMBUS_LOG_LEVEL")
	if logLevel == "" {
		logLevel = "info"
	}

	return Config{
		Port:        port,
		LogLevel:    logLevel,
		DatabaseURL: dbURL,
		JWTSecret:   jwtSecret,
	}, nil
}