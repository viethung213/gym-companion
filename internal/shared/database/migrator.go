package database

import (
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

		if _, err := db.ExecContext(ctx, string(content)); err != nil {
			return fmt.Errorf("execute embedded migration %s: %w", file, err)
		}
		log.Printf("Successfully applied SQL migration: %s", file)
	}

	return nil
}
