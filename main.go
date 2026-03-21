// Package main is the entry point for ai-news-hub.
//
// AI News Hub — an AI news aggregation service that collects,
// classifies, and serves news from multiple RSS and HTML sources.
package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"ai-news-hub/config"
	"ai-news-hub/internal/api"
	"ai-news-hub/internal/auth"
	"ai-news-hub/internal/store"
)

// version is injected via -ldflags "-X main.version=x.y.z" at build time.
var version = "dev"

func main() {
	// Handle admin CLI subcommands
	if len(os.Args) >= 2 && os.Args[1] == "admin" {
		handleAdminCommand(os.Args[2:])
		return
	}

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
	srv, err := api.NewServer(db, cfg, version)
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

// handleAdminCommand processes admin CLI subcommands.
func handleAdminCommand(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: ai-news-hub admin <command>")
		fmt.Fprintln(os.Stderr, "Commands:")
		fmt.Fprintln(os.Stderr, "  create-user  Create a new admin user")
		os.Exit(1)
	}

	switch args[0] {
	case "create-user":
		handleCreateUser(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "Unknown admin command: %s\n", args[0])
		os.Exit(1)
	}
}

// handleCreateUser implements the "admin create-user" CLI subcommand.
func handleCreateUser(args []string) {
	fs := flag.NewFlagSet("create-user", flag.ExitOnError)
	username := fs.String("username", "", "Username (required)")
	email := fs.String("email", "", "Email (required)")
	password := fs.String("password", "", "Password (required)")
	role := fs.String("role", "admin", "User role (admin/editor/viewer)")
	dbPath := fs.String("db", "./data/news.db", "Database path")
	fs.Parse(args)

	if *username == "" || *email == "" || *password == "" {
		fmt.Fprintln(os.Stderr, "Error: --username, --email, and --password are required")
		fs.PrintDefaults()
		os.Exit(1)
	}

	// Validate role
	validRoles := map[string]bool{"admin": true, "editor": true, "viewer": true}
	if !validRoles[*role] {
		fmt.Fprintf(os.Stderr, "Error: invalid role %q (must be admin/editor/viewer)\n", *role)
		os.Exit(1)
	}

	// Open database
	dsn := fmt.Sprintf("file:%s?_journal_mode=WAL&_busy_timeout=5000", *dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to open database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	// Run migrations
	if err := db.Ping(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to connect to database: %v\n", err)
		os.Exit(1)
	}

	// Use a simple config for auth
	authCfg := config.AuthConfig{BcryptCost: 10}

	// Create stores
	authStore := store.NewAuthStore(db)

	// Check uniqueness
	exists, err := authStore.CheckUsernameExists(*username)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to check username: %v\n", err)
		os.Exit(1)
	}
	if exists {
		fmt.Fprintf(os.Stderr, "Error: username %q already exists\n", *username)
		os.Exit(1)
	}

	exists, err = authStore.CheckEmailExists(*email)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to check email: %v\n", err)
		os.Exit(1)
	}
	if exists {
		fmt.Fprintf(os.Stderr, "Error: email %q already registered\n", *email)
		os.Exit(1)
	}

	// Hash password
	passwordHash, err := auth.HashPassword(*password, authCfg.BcryptCost)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to hash password: %v\n", err)
		os.Exit(1)
	}

	// Create user in transaction
	tx, err := db.Begin()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to begin transaction: %v\n", err)
		os.Exit(1)
	}
	defer tx.Rollback()

	user, err := authStore.CreateUser(tx, *username, *email, passwordHash)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to create user: %v\n", err)
		os.Exit(1)
	}

	// Override role if not admin (CreateUser always sets 'viewer')
	if *role != "viewer" {
		_, err = tx.Exec(`UPDATE users SET role = ? WHERE id = ?`, *role, user.ID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to set role: %v\n", err)
			os.Exit(1)
		}
		user.Role = *role
	}

	if err := tx.Commit(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to commit transaction: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ User created successfully!\n")
	fmt.Printf("   ID:       %d\n", user.ID)
	fmt.Printf("   Username: %s\n", *username)
	fmt.Printf("   Email:    %s\n", *email)
	fmt.Printf("   Role:     %s\n", *role)
}
