package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

func main() {

	if err := run(); err != nil {

		log.Fatalf("Fatal: %v", err)

	}

}

func run() error {

	sqlFileFlag := flag.String("sql", "", "Path to 06-seed-exercises.sql file")

	dbURLFlag := flag.String("db-url", "", "PostgreSQL database connection URL")

	flag.Parse()

	// 1. Resolve SQL file path

	sqlPath := *sqlFileFlag

	if sqlPath == "" {

		candidates := []string{

			`scripts/postgres-init/06-seed-exercises.sql`,

			`../scripts/postgres-init/06-seed-exercises.sql`,

			`../../scripts/postgres-init/06-seed-exercises.sql`,
		}

		for _, cand := range candidates {

			if _, err := os.Stat(cand); err == nil {

				sqlPath = cand

				break

			}

		}

	}

	if sqlPath == "" {

		return errors.New("06-seed-exercises.sql file not found. Please ensure scripts/postgres-init/06-seed-exercises.sql exists")

	}

	if absPath, err := filepath.Abs(sqlPath); err == nil {

		sqlPath = absPath

	}

	log.Printf("Using seed SQL file: %s", sqlPath)

	sqlBytes, err := os.ReadFile(sqlPath)

	if err != nil {

		return fmt.Errorf("failed to read SQL seed file (%s): %w", sqlPath, err)

	}

	// 2. Resolve Database URL

	dbURL := *dbURLFlag

	if dbURL == "" {

		dbURL = os.Getenv("EXERCISE_DATABASE_URL")

	}

	if dbURL == "" {

		dbURL = os.Getenv("DATABASE_URL")

	}

	if dbURL == "" {

		dbURL = "postgres://postgres:postgres@localhost:5432/fitai?sslmode=disable"

	}

	// Ensure search_path=exercise is present for schema isolation

	if !strings.Contains(dbURL, "search_path=") {

		if strings.Contains(dbURL, "?") {

			dbURL += "&search_path=exercise"

		} else {

			dbURL += "?search_path=exercise"

		}

	}

	log.Printf("Connecting to PostgreSQL database...")

	db, err := sql.Open("postgres", dbURL)

	if err != nil {

		return fmt.Errorf("failed to open database connection: %w", err)

	}

	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

	defer cancel()

	if err := db.PingContext(ctx); err != nil {

		return fmt.Errorf("failed to ping database (%s): %w", dbURL, err)

	}

	log.Printf("Executing 06-seed-exercises.sql into PostgreSQL (exercise schema)...")

	execCtx, execCancel := context.WithTimeout(context.Background(), 2*time.Minute)

	defer execCancel()

	if _, err := db.ExecContext(execCtx, string(sqlBytes)); err != nil {

		return fmt.Errorf("failed to execute SQL seed script: %w", err)

	}

	fmt.Println("==================================================")

	fmt.Println("🎉 Standalone Database Seeding for Exercise Completed!")

	fmt.Println("==================================================")

	return nil

}
