package main

import (
	"log/slog"
	"net/http"
	"os"

	"aarukanworld/internal/api"
	"aarukanworld/internal/auth"
	"aarukanworld/internal/persist"
	"aarukanworld/internal/world"
)

func main() {
	secret := os.Getenv("PLAY_TOKEN_SECRET")
	if secret == "" {
		slog.Error("PLAY_TOKEN_SECRET environment variable is required")
		os.Exit(1)
	}
	auth.Configure(secret)

	addr := ":" + envOr("PORT", "8082")
	dataDir := envOr("DATA_DIR", "data")

	store, err := persist.NewFileStore(dataDir)
	if err != nil {
		slog.Error("persist store failed", "err", err)
		os.Exit(1)
	}

	hub := world.NewHub(store)
	server := api.NewServer(hub)

	slog.Info("world service listening", "addr", addr, "data_dir", dataDir)
	if err := http.ListenAndServe(addr, server.Handler()); err != nil {
		slog.Error("server stopped", "err", err)
		os.Exit(1)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
