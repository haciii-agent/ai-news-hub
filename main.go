// Package main is the entry point for ai-news-hub.
//
// AI News Hub — an AI news aggregation service that collects,
// classifies, and serves news from multiple RSS and HTML sources.
package main

import (
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"ai-news-hub/config"
	"ai-news-hub/internal/api"
	"ai-news-hub/internal/store"
)

func main() {
	// Determine config path (default: ./config/config.yaml).
	configPath := "config/config.yaml"
	if env := os.Getenv("NEWS_HUB_CONFIG"); env != "" {
		configPath = env
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("[main] failed to load config: %v", err)
	}

	log.Printf("[main] config loaded from %s", configPath)

	// Ensure database directory exists.
	dbDir := filepath.Dir(cfg.Database.Path)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		log.Fatalf("[main] failed to create db directory %s: %v", dbDir, err)
	}

	// Initialize SQLite database (auto-creates tables).
	db, err := store.NewDB(cfg.Database.Path)
	if err != nil {
		log.Fatalf("[main] failed to init database: %v", err)
	}
	defer db.Close()

	// Initialize API server (wires collector, classifier, store).
	srv, err := api.NewServer(db, cfg)
	if err != nil {
		log.Fatalf("[main] failed to init server: %v", err)
	}
	defer srv.Close()

	// Start HTTP server.
	addr := cfg.Server.Addr()
	httpSrv := &http.Server{
		Addr:    addr,
		Handler: srv.Handler(),
	}

	// Graceful shutdown.
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("[main] shutting down...")
		httpSrv.Close()
	}()

	log.Printf("[main] 🚀 ai-news-hub ready — listening on %s", addr)
	if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("[main] server error: %v", err)
	}
	log.Println("[main] server stopped")
}
