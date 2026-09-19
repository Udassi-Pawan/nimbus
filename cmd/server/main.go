package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/Udassi-Pawan/nimbus/internal/config"
	"github.com/Udassi-Pawan/nimbus/internal/store"

	"github.com/Udassi-Pawan/nimbus/internal/api"
	"github.com/Udassi-Pawan/nimbus/internal/auth"
	"github.com/Udassi-Pawan/nimbus/internal/k8s"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(fmt.Sprintf("config error: %v", err))
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	migrationsDir := filepath.Join(cfg.RepoRoot, "migrations")
	if err := store.RunMigrations(cfg.DatabaseURL, migrationsDir); err != nil {
		logger.Error("migrations failed", "error", err)
		os.Exit(1)
	}

	ctx := context.Background()
	st, err := store.New(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("database connection failed", "error", err)
		os.Exit(1)
	}
	defer st.Close()

	authService := auth.NewService(cfg.JWTSecret)

	var k8sClient *k8s.Client
	k8sClient, err = k8s.NewClient(cfg.KubeconfigPath, cfg.KubernetesContext)
	if err != nil {
		logger.Warn("kubernetes client unavailable", "error", err)
	} else {
		logger.Info("kubernetes client ready", "context", k8sClient.ContextName())
	}

	apiServer := api.NewServer(st, authService, cfg.GeneratedServicesDir, cfg.TemplatesDir, k8sClient)

	mux := http.NewServeMux()

	mux.Handle("/api/", apiServer.Router())  // note: /api/ prefix match

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, `{"status":"ok"}`)
	})

	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		if err := st.Ping(r.Context()); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprintln(w, `{"status":"not ready","reason":"database unavailable"}`)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, `{"status":"ready"}`)
	})

	addr := fmt.Sprintf("127.0.0.1:%d", cfg.Port)
	server := &http.Server{Addr: addr, Handler: mux}

	go func() {
		logger.Info("nimbus control plane starting",
			"addr", addr,
			"repo_root", cfg.RepoRoot,
			"templates_dir", cfg.TemplatesDir,
			"generated_dir", cfg.GeneratedServicesDir,
		)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server failed", "error", err)
		 os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	logger.Info("nimbus shutting down")
	_ = server.Shutdown(shutdownCtx)
}