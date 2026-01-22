// Package main is the entry point for the database migration tool.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
)

func main() {
	// Load .env file if it exists
	_ = godotenv.Load()

	// Setup structured logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	slog.SetDefault(logger)

	// Parse flags
	direction := flag.String("direction", "up", "Migration direction: up or down")
	migrationsDir := flag.String("migrations", "migrations", "Path to migrations directory")
	seed := flag.Bool("seed", false, "Run seed migrations only")
	flag.Parse()

	logger.Info("starting migration tool",
		"direction", *direction,
		"migrations_dir", *migrationsDir,
		"seed", *seed,
	)

	// Get database URL from environment
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		// Build from individual components
		host := getEnvOrDefault("DB_HOST", "localhost")
		port := getEnvOrDefault("DB_PORT", "5432")
		user := getEnvOrDefault("DB_USER", "shipping")
		password := getEnvOrDefault("DB_PASSWORD", "shipping")
		dbname := getEnvOrDefault("DB_NAME", "shipping")
		sslmode := getEnvOrDefault("DB_SSLMODE", "disable")
		dbURL = fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
			user, password, host, port, dbname, sslmode)
	}

	// Connect to database
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, dbURL)
	if err != nil {
		logger.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer conn.Close(ctx)

	// Create migrations table if not exists
	_, err = conn.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version VARCHAR(255) PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		logger.Error("failed to create migrations table", "error", err)
		os.Exit(1)
	}

	// Get list of migration files
	suffix := ".up.sql"
	if *direction == "down" {
		suffix = ".down.sql"
	}

	files, err := filepath.Glob(filepath.Join(*migrationsDir, "*"+suffix))
	if err != nil {
		logger.Error("failed to list migration files", "error", err)
		os.Exit(1)
	}

	// Sort files
	sort.Strings(files)
	if *direction == "down" {
		// Reverse order for down migrations
		for i, j := 0, len(files)-1; i < j; i, j = i+1, j-1 {
			files[i], files[j] = files[j], files[i]
		}
	}

	// Filter seed migrations if needed
	if *seed {
		var seedFiles []string
		for _, f := range files {
			if strings.Contains(filepath.Base(f), "seed") {
				seedFiles = append(seedFiles, f)
			}
		}
		files = seedFiles
	}

	// Run migrations
	for _, file := range files {
		version := extractVersion(filepath.Base(file))

		// Check if already applied (for up migrations)
		if *direction == "up" {
			var exists bool
			err := conn.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1)", version).Scan(&exists)
			if err != nil {
				logger.Error("failed to check migration status", "version", version, "error", err)
				os.Exit(1)
			}
			if exists {
				logger.Info("skipping already applied migration", "version", version)
				continue
			}
		}

		// Read migration file
		content, err := os.ReadFile(file)
		if err != nil {
			logger.Error("failed to read migration file", "file", file, "error", err)
			os.Exit(1)
		}

		// Execute migration
		logger.Info("applying migration", "version", version, "direction", *direction)
		_, err = conn.Exec(ctx, string(content))
		if err != nil {
			logger.Error("failed to apply migration", "version", version, "error", err)
			os.Exit(1)
		}

		// Update migrations table
		if *direction == "up" {
			_, err = conn.Exec(ctx, "INSERT INTO schema_migrations (version) VALUES ($1)", version)
		} else {
			_, err = conn.Exec(ctx, "DELETE FROM schema_migrations WHERE version = $1", version)
		}
		if err != nil {
			logger.Error("failed to update migrations table", "version", version, "error", err)
			os.Exit(1)
		}

		logger.Info("migration applied successfully", "version", version)
	}

	logger.Info("all migrations completed")
}

func extractVersion(filename string) string {
	// Extract version from filename like "000001_initial_schema.up.sql"
	parts := strings.SplitN(filename, "_", 2)
	if len(parts) > 0 {
		return parts[0]
	}
	return filename
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
