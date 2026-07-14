package database

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	_ "github.com/lib/pq" // PostgreSQL driver registration.
)

// Registry manages and isolates database connection pools for all modules.
type Registry struct {
	mu    sync.RWMutex
	pools map[string]*sql.DB
}

var (
	//nolint:gochecknoglobals // Singleton database registry instance.
	instance *Registry
	//nolint:gochecknoglobals // Ensures singleton initialization once.
	once sync.Once
)

// GetRegistry returns the singleton instance of the database connection Registry.
func GetRegistry() *Registry {
	once.Do(func() {
		instance = &Registry{
			pools: make(map[string]*sql.DB),
		}
	})
	return instance
}

// GetPool retrieves or instantiates a PostgreSQL connection pool dedicated to a specific module.
// It resolves the connection string by reading module-specific environment variables
// (e.g. AUTH_DATABASE_URL).
// Fallback is database-wide DATABASE_URL with a schema search_path query parameter.
func (r *Registry) GetPool(module string) (*sql.DB, error) {
	module = strings.ToLower(module)

	// Thread-safe read lock check
	r.mu.RLock()
	db, exists := r.pools[module]
	r.mu.RUnlock()

	if exists {
		return db, nil
	}

	// Lock for writing and double-check
	r.mu.Lock()
	defer r.mu.Unlock()

	db, exists = r.pools[module]
	if exists {
		return db, nil
	}

	// 1. Resolve Connection String
	// Look up module-specific URL (e.g., AUTH_DATABASE_URL)
	envKey := strings.ToUpper(module) + "_DATABASE_URL"
	dbURL := os.Getenv(envKey)

	if dbURL == "" {
		// Fallback to global DATABASE_URL
		globalURL := os.Getenv("DATABASE_URL")
		if globalURL == "" {
			globalURL = "postgres://postgres:postgres@localhost:5432/fitai?sslmode=disable"
		}

		// Inject search_path parameter to enforce module schema isolation at query time
		if strings.Contains(globalURL, "?") {
			dbURL = fmt.Sprintf("%s&search_path=%s", globalURL, module)
		} else {
			dbURL = fmt.Sprintf("%s?search_path=%s", globalURL, module)
		}
	}

	// 2. Open Connection
	conn, err := sql.Open("postgres", dbURL)
	if err != nil {
		return nil, fmt.Errorf("open db pool for module %s: %w", module, err)
	}

	// Configure pool parameters
	conn.SetMaxOpenConns(25)
	conn.SetMaxIdleConns(25)
	conn.SetConnMaxLifetime(5 * time.Minute)

	// Verify connection
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := conn.PingContext(ctx); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("ping db pool for module %s: %w", module, err)
	}

	r.pools[module] = conn
	return conn, nil
}

// CloseAll closes all open database connection pools in the registry.
func (r *Registry) CloseAll() {
	r.mu.Lock()
	defer r.mu.Unlock()

	for module, pool := range r.pools {
		if pool != nil {
			_ = pool.Close()
		}
		delete(r.pools, module)
	}
}
