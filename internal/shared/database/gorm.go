package database

import (
	"database/sql"
	"errors"
	"fmt"

	gormPostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/plugin/opentelemetry/tracing"
)

// OpenGorm wraps an existing *sql.DB connection pool into a *gorm.DB instance,
// applying production defaults and automatically instrumenting all queries with OpenTelemetry tracing.
func OpenGorm(conn *sql.DB, cfg *gorm.Config) (*gorm.DB, error) {
	if conn == nil {
		return nil, errors.New("database connection is nil")
	}
	if cfg == nil {
		cfg = &gorm.Config{
			PrepareStmt:            false,
			SkipDefaultTransaction: true,
		}
	}

	gormDB, err := gorm.Open(gormPostgres.New(gormPostgres.Config{
		Conn: conn,
	}), cfg)
	if err != nil {
		return nil, fmt.Errorf("open gorm db: %w", err)
	}

	if err := gormDB.Use(tracing.NewPlugin(tracing.WithoutMetrics())); err != nil {
		return nil, fmt.Errorf("register gorm opentelemetry tracing: %w", err)
	}

	return gormDB, nil
}
