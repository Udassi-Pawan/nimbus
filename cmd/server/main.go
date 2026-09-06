package main

import (
	"fmt"
	"net/http"

	"github.com/Udassi-Pawan/nimbus/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(fmt.Sprintf("config error: %v", err))
	}

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, `{"status":"ok"}`)
	})

	addr := fmt.Sprintf("127.0.0.1:%d", cfg.Port)
	fmt.Printf("nimbus listening on http://%s (log_level=%s)\n", addr, cfg.LogLevel)
	if err := http.ListenAndServe(addr, nil); err != nil {
		panic(err)
	}
}