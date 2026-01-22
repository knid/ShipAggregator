// Package main is the entry point for the database migration tool.
// It provides commands for running migrations up/down, checking status, and creating new migrations.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"github.com/e-port/shipping-aggregator/internal/config"
)

// Build information (set via ldflags).
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildTime = "unknown"
)

// Command represents a migration command.
type Command string

const (
	CommandUp     Command = "up"
	CommandDown   Command = "down"
	CommandStatus Command = "status"
	CommandCreate Command = "create"
	CommandReset  Command = "reset"
	CommandForce  Command = "force"
)

func main() {
	// Load .env file if it exists
	_ = godotenv.Load()

	// Parse flags
	command := flag.String("command", "up", "Migration command: up, down, status, create, reset, force")
	migrationsDir := flag.String("migrations", "migrations", "Path to migrations directory")
	steps := flag.Int("steps", 0, "Number of migrations to run (0 = all)")
	name := flag.String("name", "", "Name for new migration (used with create command)")
	version := flag.String("version", "", "Force migration version (used with force command)")
	verbose := flag.Bool("verbose", false, "Enable verbose output")
	showVersion := flag.Bool("v", false, "Show version information")
	flag.Parse()

	if *showVersion {
		fmt.Printf("migrate version %s (commit: %s, built: %s)\n", Version, Commit, BuildTime)
		os.Exit(0)
	}

	// Create logger
	logger := &Logger{verbose: *verbose}

	logger.Info("migration tool started",
		"command", *command,
		"migrations_dir", *migrationsDir,
	)

	// Handle create command separately (doesn't need database)
	if Command(*command) == CommandCreate {
		if *name == "" {
			logger.Error("migration name is required for create command")
			os.Exit(1)
		}
		if err := createMigration(*migrationsDir, *name); err != nil {
			logger.Error("failed to create migration", "error", err)
			os.Exit(1)
		}
		logger.Info("migration created successfully")
		os.Exit(0)
	}

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		// Fall back to environment variables
		logger.Debug("failed to load config, using environment variables", "error", err)
	}

	// Get database URL
	dbURL := getDatabaseURL(cfg)
	if dbURL == "" {
		logger.Error("database URL not configured")
		os.Exit(1)
	}

	// Connect to database
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		logger.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	// Verify connection
	if err := pool.Ping(ctx); err != nil {
		logger.Error("failed to ping database", "error", err)
		os.Exit(1)
	}
	logger.Debug("database connection established")

	// Create migrations table if not exists
	if err := ensureMigrationsTable(ctx, pool); err != nil {
		logger.Error("failed to create migrations table", "error", err)
		os.Exit(1)
	}

	// Execute command
	var cmdErr error
	switch Command(*command) {
	case CommandUp:
		cmdErr = runUp(ctx, pool, *migrationsDir, *steps, logger)
	case CommandDown:
		cmdErr = runDown(ctx, pool, *migrationsDir, *steps, logger)
	case CommandStatus:
		cmdErr = showStatus(ctx, pool, *migrationsDir, logger)
	case CommandReset:
		cmdErr = runReset(ctx, pool, *migrationsDir, logger)
	case CommandForce:
		if *version == "" {
			logger.Error("version is required for force command")
			os.Exit(1)
		}
		cmdErr = forceVersion(ctx, pool, *version, logger)
	default:
		logger.Error("unknown command", "command", *command)
		os.Exit(1)
	}

	if cmdErr != nil {
		logger.Error("command failed", "error", cmdErr)
		os.Exit(1)
	}

	logger.Info("migration completed successfully")
}

// Logger provides simple logging functionality.
type Logger struct {
	verbose bool
}

func (l *Logger) Info(msg string, args ...any) {
	fmt.Printf("[INFO] %s", msg)
	for i := 0; i < len(args); i += 2 {
		if i+1 < len(args) {
			fmt.Printf(" %s=%v", args[i], args[i+1])
		}
	}
	fmt.Println()
}

func (l *Logger) Debug(msg string, args ...any) {
	if !l.verbose {
		return
	}
	fmt.Printf("[DEBUG] %s", msg)
	for i := 0; i < len(args); i += 2 {
		if i+1 < len(args) {
			fmt.Printf(" %s=%v", args[i], args[i+1])
		}
	}
	fmt.Println()
}

func (l *Logger) Error(msg string, args ...any) {
	fmt.Fprintf(os.Stderr, "[ERROR] %s", msg)
	for i := 0; i < len(args); i += 2 {
		if i+1 < len(args) {
			fmt.Fprintf(os.Stderr, " %s=%v", args[i], args[i+1])
		}
	}
	fmt.Fprintln(os.Stderr)
}

// getDatabaseURL returns the database URL from config or environment.
func getDatabaseURL(cfg *config.Config) string {
	if cfg != nil {
		return cfg.Database.DSN()
	}

	// Fall back to environment variables
	if url := os.Getenv("DATABASE_URL"); url != "" {
		return url
	}

	// Build from individual components
	host := getEnvOrDefault("DB_HOST", "localhost")
	port := getEnvOrDefault("DB_PORT", "5432")
	user := getEnvOrDefault("DB_USER", "shipping")
	password := getEnvOrDefault("DB_PASSWORD", "shipping")
	dbname := getEnvOrDefault("DB_NAME", "shipping")
	sslmode := getEnvOrDefault("DB_SSLMODE", "disable")

	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		user, password, host, port, dbname, sslmode)
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// ensureMigrationsTable creates the schema_migrations table if it doesn't exist.
func ensureMigrationsTable(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version VARCHAR(255) PRIMARY KEY,
			name VARCHAR(255),
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			execution_time_ms INTEGER
		)
	`)
	return err
}

// Migration represents a migration file.
type Migration struct {
	Version  string
	Name     string
	UpFile   string
	DownFile string
}

// getMigrations returns all migrations from the migrations directory.
func getMigrations(migrationsDir string) ([]Migration, error) {
	files, err := filepath.Glob(filepath.Join(migrationsDir, "*.up.sql"))
	if err != nil {
		return nil, fmt.Errorf("failed to list migration files: %w", err)
	}

	migrations := make([]Migration, 0, len(files))
	for _, upFile := range files {
		base := filepath.Base(upFile)
		// Extract version and name from filename like "000001_initial_schema.up.sql"
		parts := strings.SplitN(strings.TrimSuffix(base, ".up.sql"), "_", 2)
		version := parts[0]
		name := ""
		if len(parts) > 1 {
			name = parts[1]
		}

		downFile := strings.TrimSuffix(upFile, ".up.sql") + ".down.sql"
		if _, err := os.Stat(downFile); os.IsNotExist(err) {
			downFile = ""
		}

		migrations = append(migrations, Migration{
			Version:  version,
			Name:     name,
			UpFile:   upFile,
			DownFile: downFile,
		})
	}

	// Sort by version
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})

	return migrations, nil
}

// getAppliedMigrations returns all applied migrations from the database.
func getAppliedMigrations(ctx context.Context, pool *pgxpool.Pool) (map[string]bool, error) {
	rows, err := pool.Query(ctx, "SELECT version FROM schema_migrations ORDER BY version")
	if err != nil {
		return nil, fmt.Errorf("failed to query migrations: %w", err)
	}
	defer rows.Close()

	applied := make(map[string]bool)
	for rows.Next() {
		var version string
		if err := rows.Scan(&version); err != nil {
			return nil, fmt.Errorf("failed to scan migration version: %w", err)
		}
		applied[version] = true
	}

	return applied, rows.Err()
}

// runUp runs pending migrations.
func runUp(ctx context.Context, pool *pgxpool.Pool, migrationsDir string, steps int, logger *Logger) error {
	migrations, err := getMigrations(migrationsDir)
	if err != nil {
		return err
	}

	applied, err := getAppliedMigrations(ctx, pool)
	if err != nil {
		return err
	}

	// Filter pending migrations
	var pending []Migration
	for _, m := range migrations {
		if !applied[m.Version] {
			pending = append(pending, m)
		}
	}

	if len(pending) == 0 {
		logger.Info("no pending migrations")
		return nil
	}

	// Limit steps if specified
	if steps > 0 && steps < len(pending) {
		pending = pending[:steps]
	}

	logger.Info("running migrations", "count", len(pending))

	for _, m := range pending {
		if err := runMigration(ctx, pool, m, true, logger); err != nil {
			return fmt.Errorf("migration %s failed: %w", m.Version, err)
		}
	}

	return nil
}

// runDown rolls back applied migrations.
func runDown(ctx context.Context, pool *pgxpool.Pool, migrationsDir string, steps int, logger *Logger) error {
	migrations, err := getMigrations(migrationsDir)
	if err != nil {
		return err
	}

	applied, err := getAppliedMigrations(ctx, pool)
	if err != nil {
		return err
	}

	// Filter applied migrations and reverse order
	var toRollback []Migration
	for i := len(migrations) - 1; i >= 0; i-- {
		if applied[migrations[i].Version] {
			toRollback = append(toRollback, migrations[i])
		}
	}

	if len(toRollback) == 0 {
		logger.Info("no migrations to rollback")
		return nil
	}

	// Default to 1 step for down
	if steps == 0 {
		steps = 1
	}
	if steps > len(toRollback) {
		steps = len(toRollback)
	}
	toRollback = toRollback[:steps]

	logger.Info("rolling back migrations", "count", len(toRollback))

	for _, m := range toRollback {
		if m.DownFile == "" {
			return fmt.Errorf("no down migration for version %s", m.Version)
		}
		if err := runMigration(ctx, pool, m, false, logger); err != nil {
			return fmt.Errorf("rollback %s failed: %w", m.Version, err)
		}
	}

	return nil
}

// runMigration executes a single migration.
func runMigration(ctx context.Context, pool *pgxpool.Pool, m Migration, up bool, logger *Logger) error {
	file := m.UpFile
	if !up {
		file = m.DownFile
	}

	content, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("failed to read migration file: %w", err)
	}

	direction := "up"
	if !up {
		direction = "down"
	}
	logger.Info("applying migration",
		"version", m.Version,
		"name", m.Name,
		"direction", direction,
	)

	start := time.Now()

	// Execute migration in a transaction
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, string(content)); err != nil {
		return fmt.Errorf("failed to execute migration: %w", err)
	}

	executionTime := time.Since(start).Milliseconds()

	// Update migrations table
	if up {
		_, err = tx.Exec(ctx,
			"INSERT INTO schema_migrations (version, name, execution_time_ms) VALUES ($1, $2, $3)",
			m.Version, m.Name, executionTime)
	} else {
		_, err = tx.Exec(ctx, "DELETE FROM schema_migrations WHERE version = $1", m.Version)
	}
	if err != nil {
		return fmt.Errorf("failed to update migrations table: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	logger.Debug("migration applied",
		"version", m.Version,
		"execution_time_ms", executionTime,
	)

	return nil
}

// showStatus displays the status of all migrations.
func showStatus(ctx context.Context, pool *pgxpool.Pool, migrationsDir string, logger *Logger) error {
	migrations, err := getMigrations(migrationsDir)
	if err != nil {
		return err
	}

	// Get applied migrations with details
	rows, err := pool.Query(ctx,
		"SELECT version, name, applied_at, execution_time_ms FROM schema_migrations ORDER BY version")
	if err != nil {
		return fmt.Errorf("failed to query migrations: %w", err)
	}
	defer rows.Close()

	type AppliedMigration struct {
		Version       string
		Name          string
		AppliedAt     time.Time
		ExecutionTime *int
	}

	applied := make(map[string]AppliedMigration)
	for rows.Next() {
		var m AppliedMigration
		if err := rows.Scan(&m.Version, &m.Name, &m.AppliedAt, &m.ExecutionTime); err != nil {
			return fmt.Errorf("failed to scan migration: %w", err)
		}
		applied[m.Version] = m
	}
	if err := rows.Err(); err != nil {
		return err
	}

	// Print status table
	fmt.Println()
	fmt.Printf("%-10s %-30s %-10s %-25s %-10s\n", "VERSION", "NAME", "STATUS", "APPLIED AT", "TIME (ms)")
	fmt.Println(strings.Repeat("-", 90))

	for _, m := range migrations {
		status := "pending"
		appliedAt := ""
		execTime := ""

		if am, ok := applied[m.Version]; ok {
			status = "applied"
			appliedAt = am.AppliedAt.Format("2006-01-02 15:04:05")
			if am.ExecutionTime != nil {
				execTime = fmt.Sprintf("%d", *am.ExecutionTime)
			}
		}

		name := m.Name
		if len(name) > 28 {
			name = name[:28] + ".."
		}

		fmt.Printf("%-10s %-30s %-10s %-25s %-10s\n",
			m.Version, name, status, appliedAt, execTime)
	}

	fmt.Println()

	// Summary
	pendingCount := 0
	for _, m := range migrations {
		if _, ok := applied[m.Version]; !ok {
			pendingCount++
		}
	}

	fmt.Printf("Total: %d migrations, %d applied, %d pending\n",
		len(migrations), len(applied), pendingCount)

	return nil
}

// runReset rolls back all migrations and re-applies them.
func runReset(ctx context.Context, pool *pgxpool.Pool, migrationsDir string, logger *Logger) error {
	logger.Info("resetting database (rolling back all migrations)")

	// Roll back all
	for {
		applied, err := getAppliedMigrations(ctx, pool)
		if err != nil {
			return err
		}
		if len(applied) == 0 {
			break
		}
		if err := runDown(ctx, pool, migrationsDir, 1, logger); err != nil {
			return err
		}
	}

	logger.Info("re-applying all migrations")

	// Apply all
	return runUp(ctx, pool, migrationsDir, 0, logger)
}

// forceVersion forces the migration version without running migrations.
func forceVersion(ctx context.Context, pool *pgxpool.Pool, version string, logger *Logger) error {
	logger.Info("forcing migration version", "version", version)

	// Clear all migrations
	if _, err := pool.Exec(ctx, "DELETE FROM schema_migrations"); err != nil {
		return fmt.Errorf("failed to clear migrations: %w", err)
	}

	// Insert the forced version
	if version != "0" && version != "" {
		if _, err := pool.Exec(ctx,
			"INSERT INTO schema_migrations (version, name) VALUES ($1, 'forced')",
			version); err != nil {
			return fmt.Errorf("failed to insert forced version: %w", err)
		}
	}

	logger.Info("migration version forced", "version", version)
	return nil
}

// createMigration creates a new migration file pair.
func createMigration(migrationsDir, name string) error {
	// Ensure migrations directory exists
	if err := os.MkdirAll(migrationsDir, 0755); err != nil {
		return fmt.Errorf("failed to create migrations directory: %w", err)
	}

	// Get next version number
	files, err := filepath.Glob(filepath.Join(migrationsDir, "*.up.sql"))
	if err != nil {
		return fmt.Errorf("failed to list migration files: %w", err)
	}

	nextVersion := len(files) + 1
	version := fmt.Sprintf("%06d", nextVersion)

	// Sanitize name
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, " ", "_")
	name = strings.ReplaceAll(name, "-", "_")

	// Create file names
	upFile := filepath.Join(migrationsDir, fmt.Sprintf("%s_%s.up.sql", version, name))
	downFile := filepath.Join(migrationsDir, fmt.Sprintf("%s_%s.down.sql", version, name))

	// Create up migration
	upContent := fmt.Sprintf(`-- Migration: %s_%s
-- Created at: %s

-- Add your migration SQL here

`, version, name, time.Now().Format(time.RFC3339))

	if err := os.WriteFile(upFile, []byte(upContent), 0644); err != nil {
		return fmt.Errorf("failed to create up migration: %w", err)
	}

	// Create down migration
	downContent := fmt.Sprintf(`-- Rollback: %s_%s
-- Created at: %s

-- Add your rollback SQL here

`, version, name, time.Now().Format(time.RFC3339))

	if err := os.WriteFile(downFile, []byte(downContent), 0644); err != nil {
		return fmt.Errorf("failed to create down migration: %w", err)
	}

	fmt.Printf("Created migration files:\n")
	fmt.Printf("  %s\n", upFile)
	fmt.Printf("  %s\n", downFile)

	return nil
}

// Additional helper for running migrations programmatically
func RunMigrations(ctx context.Context, conn *pgx.Conn, migrationsDir string) error {
	// This function can be used by other packages to run migrations
	// For example, in tests or during application startup

	pool, err := pgxpool.New(ctx, conn.Config().ConnString())
	if err != nil {
		return fmt.Errorf("failed to create pool: %w", err)
	}
	defer pool.Close()

	if err := ensureMigrationsTable(ctx, pool); err != nil {
		return err
	}

	logger := &Logger{verbose: false}
	return runUp(ctx, pool, migrationsDir, 0, logger)
}
