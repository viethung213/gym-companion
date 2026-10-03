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

// migrationAdvisoryLockID is a 64-bit integer derived from "FITAI" in hex (0x4649544149).
// It ensures that only one process or Pod runs migrations/seeds concurrently across the cluster.
const migrationAdvisoryLockID = 0x4649544149

// RunAutoMigrations executes versioned SQL migration scripts and seeds embedded in the binary.
// It is protected by a PostgreSQL distributed advisory lock to guarantee concurrency safety across multiple pods/processes.
func RunAutoMigrations(ctx context.Context, db *sql.DB) error {
	conn, connErr := db.Conn(ctx)
	if connErr != nil {
		return fmt.Errorf("acquire dedicated connection for migrations: %w", connErr)
	}
	defer conn.Close()

	if _, lockErr := conn.ExecContext(ctx, "SELECT pg_advisory_lock($1)", migrationAdvisoryLockID); lockErr != nil {
		return fmt.Errorf("acquire migration advisory lock: %w", lockErr)
	}
	defer func() {
		if _, unlockErr := conn.ExecContext(context.Background(), "SELECT pg_advisory_unlock($1)", migrationAdvisoryLockID); unlockErr != nil {
			log.Printf("Warning: release migration advisory lock failed: %v", unlockErr)
		}
	}()

	entries, readDirErr := migrationFS.ReadDir("migrations")
	if readDirErr != nil {
		return fmt.Errorf("read embedded migrations directory: %w", readDirErr)
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
		content, readFileErr := migrationFS.ReadFile("migrations/" + file)
		if readFileErr != nil {
			return fmt.Errorf("read embedded migration file %s: %w", file, readFileErr)
		}

		// Strip UTF-8 Byte Order Mark (BOM) if present to avoid syntax errors in PostgreSQL parser
		content = bytes.TrimPrefix(content, []byte("\xef\xbb\xbf"))

		if _, execErr := conn.ExecContext(ctx, string(content)); execErr != nil {
			return fmt.Errorf("execute embedded migration %s: %w", file, execErr)
		}
		log.Printf("Successfully applied SQL migration: %s", file)
	}

	if seedErr := runSeedsOnConn(ctx, conn); seedErr != nil {
		return fmt.Errorf("run seeds: %w", seedErr)
	}

	return nil
}

// RunSeeds executes initial seed SQL scripts embedded in the binary.
func RunSeeds(ctx context.Context, db *sql.DB) error {
	conn, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("acquire dedicated connection for seeds: %w", err)
	}
	defer conn.Close()
	return runSeedsOnConn(ctx, conn)
}

func runSeedsOnConn(ctx context.Context, conn *sql.Conn) error {
	entries, readDirErr := seedFS.ReadDir("seeds")
	if readDirErr != nil {
		return fmt.Errorf("read embedded seeds directory: %w", readDirErr)
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
		content, readFileErr := seedFS.ReadFile("seeds/" + file)
		if readFileErr != nil {
			return fmt.Errorf("read embedded seed file %s: %w", file, readFileErr)
		}

		content = bytes.TrimPrefix(content, []byte("\xef\xbb\xbf"))

		if _, execErr := conn.ExecContext(ctx, string(content)); execErr != nil {
			return fmt.Errorf("execute embedded seed %s: %w", file, execErr)
		}
		log.Printf("Successfully applied SQL seed: %s", file)
	}

	return nil
}
