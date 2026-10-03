package database

import (
	"bytes"
	"context"
	"database/sql"
	"embed"
	"fmt"
	"log"
	"sort"
	"strings"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

//go:embed seeds/*.sql
var seedFS embed.FS

// RunAutoMigrations executes versioned SQL migration scripts embedded in the binary.
// It complies with versioned SQL migration guidelines without using GORM AutoMigrate.
func RunAutoMigrations(ctx context.Context, db *sql.DB) error {
	entries, err := migrationFS.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("read embedded migrations directory: %w", err)
	}

	var files []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			files = append(files, entry.Name())
		}
	}
	sort.Strings(files)

	log.Printf("Executing %d embedded SQL migrations...", len(files))

	for _, file := range files {
		content, err := migrationFS.ReadFile("migrations/" + file)
		if err != nil {
			return fmt.Errorf("read embedded migration file %s: %w", file, err)
		}

		// Strip UTF-8 Byte Order Mark (BOM) if present to avoid syntax errors in PostgreSQL parser
		content = bytes.TrimPrefix(content, []byte("\xef\xbb\xbf"))

		if _, err := db.ExecContext(ctx, string(content)); err != nil {
			return fmt.Errorf("execute embedded migration %s: %w", file, err)
		}
		log.Printf("Successfully applied SQL migration: %s", file)
	}

	if err := RunSeeds(ctx, db); err != nil {
		return fmt.Errorf("run seeds: %w", err)
	}

	return nil
}

// RunSeeds executes initial seed SQL scripts embedded in the binary.
func RunSeeds(ctx context.Context, db *sql.DB) error {
	entries, err := seedFS.ReadDir("seeds")
	if err != nil {
		return fmt.Errorf("read embedded seeds directory: %w", err)
	}

	var files []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			files = append(files, entry.Name())
		}
	}
	sort.Strings(files)

	log.Printf("Executing %d embedded SQL seeds...", len(files))

	for _, file := range files {
		content, err := seedFS.ReadFile("seeds/" + file)
		if err != nil {
			return fmt.Errorf("read embedded seed file %s: %w", file, err)
		}

		content = bytes.TrimPrefix(content, []byte("\xef\xbb\xbf"))

		if _, err := db.ExecContext(ctx, string(content)); err != nil {
			return fmt.Errorf("execute embedded seed %s: %w", file, err)
		}
		log.Printf("Successfully applied SQL seed: %s", file)
	}

	return nil
}
