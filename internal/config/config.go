package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/Udassi-Pawan/nimbus/internal/paths"
)

type Config struct {
	Port                 int
	LogLevel             string
	DatabaseURL          string
	JWTSecret            string
	RepoRoot             string
	GeneratedServicesDir string
	TemplatesDir         string
	KubeconfigPath       string
	KubernetesContext    string
}

func Load() (Config, error) {
	repoRoot := os.Getenv("NIMBUS_REPO_ROOT")
	if repoRoot == "" {
		var err error
		repoRoot, err = paths.FindRepoRoot()
		if err != nil {
			return Config{}, fmt.Errorf("find repo root: %w", err)
		}
	}

	loadEnvFiles(repoRoot)

	port := 8080
	if v := os.Getenv("NIMBUS_PORT"); v != "" {
		p, err := strconv.Atoi(v)
		if err != nil {
			return Config{}, fmt.Errorf("invalid NIMBUS_PORT: %w", err)
		}
		port = p
	}

	generatedDir := os.Getenv("NIMBUS_GENERATED_DIR")
	if generatedDir == "" {
		generatedDir = "generated"
	}
	generatedDir = paths.ResolvePath(repoRoot, generatedDir)

	templatesDir := os.Getenv("NIMBUS_TEMPLATES_DIR")
	if templatesDir == "" {
		templatesDir = filepath.Join(repoRoot, "templates", "go-api")
	} else {
		templatesDir = paths.ResolvePath(repoRoot, templatesDir)
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

	kubeconfigPath := os.Getenv("KUBECONFIG")
	kubernetesContext := os.Getenv("KUBERNETES_CONTEXT")

	return Config{
		Port:                 port,
		LogLevel:             logLevel,
		DatabaseURL:          dbURL,
		JWTSecret:            jwtSecret,
		RepoRoot:             repoRoot,
		GeneratedServicesDir: generatedDir,
		TemplatesDir:         templatesDir,
		KubeconfigPath:       kubeconfigPath,
		KubernetesContext:    kubernetesContext,
	}, nil
}