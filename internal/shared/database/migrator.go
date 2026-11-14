package database

import (
	"bytes"
	"context"
	"database/sql"
	"embed"
	"fmt"
	"log/slog"
	"sort"
	"strings"
)

//go:embed migrations/*/*.sql
var migrationFS embed.FS

//go:embed seeds/*.sql
var seedFS embed.FS

// migrationAdvisoryLockID is a 64-bit integer derived from "FITAI" in hex (0x4649544149).
// It ensures that only one process or Pod runs migrations/seeds concurrently across the cluster.
const migrationAdvisoryLockID = 0x4649544149

// RunAutoMigrations executes versioned SQL migration scripts and seeds embedded in the binary.
// targets specifies which subdirectories to run ("core", "ai"). If omitted or empty, all targets ("core", "ai") are run.
// It is protected by a PostgreSQL distributed advisory lock to guarantee concurrency safety across multiple pods/processes.
func RunAutoMigrations(ctx context.Context, db *sql.DB, targets ...string) error {
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
			slog.Warn("Failed to release migration advisory lock", "error", unlockErr)
		}
	}()

	var dirs []string
	if len(targets) == 0 {
		dirs = []string{"core", "ai"}
	} else {
		for _, t := range targets {
			t = strings.ToLower(strings.TrimSpace(t))
			if t == "all" || t == "" {
				dirs = []string{"core", "ai"}
				break
			}
			dirs = append(dirs, t)
		}
	}

	hasCore := false
	for _, dir := range dirs {
		if dir == "core" {
			hasCore = true
		}
		subDirPath := "migrations/" + dir
		entries, readDirErr := migrationFS.ReadDir(subDirPath)
		if readDirErr != nil {
			return fmt.Errorf("read embedded migrations directory %s: %w", subDirPath, readDirErr)
		}

		var files []string
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
				files = append(files, entry.Name())
			}
		}
		sort.Strings(files)

		slog.InfoContext(ctx, "Executing embedded SQL migrations", "count", len(files), "dir", dir)

		for _, file := range files {
			filePath := subDirPath + "/" + file
			content, readFileErr := migrationFS.ReadFile(filePath)
			if readFileErr != nil {
				return fmt.Errorf("read embedded migration file %s: %w", filePath, readFileErr)
			}

			// Strip UTF-8 Byte Order Mark (BOM) if present to avoid syntax errors in PostgreSQL parser
			content = bytes.TrimPrefix(content, []byte("\xef\xbb\xbf"))

			if _, execErr := conn.ExecContext(ctx, string(content)); execErr != nil {
				return fmt.Errorf("execute embedded migration %s: %w", filePath, execErr)
			}
			slog.InfoContext(ctx, "Successfully applied SQL migration", "dir", dir, "file", file)
		}
	}

	if hasCore {
		if seedErr := runSeedsOnConn(ctx, conn); seedErr != nil {
			return fmt.Errorf("run seeds: %w", seedErr)
		}
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

	slog.InfoContext(ctx, "Executing embedded SQL seeds", "count", len(files))

	for _, file := range files {
		content, readFileErr := seedFS.ReadFile("seeds/" + file)
		if readFileErr != nil {
			return fmt.Errorf("read embedded seed file %s: %w", file, readFileErr)
		}

		content = bytes.TrimPrefix(content, []byte("\xef\xbb\xbf"))

		if _, execErr := conn.ExecContext(ctx, string(content)); execErr != nil {
			return fmt.Errorf("execute embedded seed %s: %w", file, execErr)
		}
		slog.InfoContext(ctx, "Successfully applied SQL seed", "file", file)
	}

	return nil
}
