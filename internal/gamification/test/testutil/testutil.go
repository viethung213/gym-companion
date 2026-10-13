package testutil

import (
	"context"
	"database/sql"
	"regexp"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/viethung213/gym-companion/internal/gamification/application/port"
	"github.com/viethung213/gym-companion/internal/gamification/domain/repository"
	"github.com/viethung213/gym-companion/internal/gamification/infrastructure/persistence"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// SchemaStripPool wraps *sql.DB and strips `gamification.` prefix for SQLite compatibility.
type SchemaStripPool struct {
	*sql.DB
	re *regexp.Regexp
}

// NewSchemaStripPool creates SchemaStripPool.
func NewSchemaStripPool(db *sql.DB) *SchemaStripPool {
	re := regexp.MustCompile(`(?i)(["` + "`" + `]?gamification["` + "`" + `]?\.)(["` + "`" + `]?([a-zA-Z0-9_]+)["` + "`" + `]?)`)
	return &SchemaStripPool{DB: db, re: re}
}

func (p *SchemaStripPool) strip(query string) string {
	cleaned := p.re.ReplaceAllString(query, "$2")
	cleaned = strings.ReplaceAll(cleaned, "FOR UPDATE SKIP LOCKED", "")
	cleaned = strings.ReplaceAll(cleaned, "FOR UPDATE", "")
	return cleaned
}

func (p *SchemaStripPool) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return p.DB.ExecContext(ctx, p.strip(query), args...)
}

func (p *SchemaStripPool) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return p.DB.QueryContext(ctx, p.strip(query), args...)
}

func (p *SchemaStripPool) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return p.DB.QueryRowContext(ctx, p.strip(query), args...)
}

func (p *SchemaStripPool) PrepareContext(ctx context.Context, query string) (*sql.Stmt, error) {
	return p.DB.PrepareContext(ctx, p.strip(query))
}

func (p *SchemaStripPool) BeginTx(ctx context.Context, opts *sql.TxOptions) (gorm.ConnPool, error) {
	tx, err := p.DB.BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}
	return &SchemaTxPool{Tx: tx, re: p.re}, nil
}

type SchemaTxPool struct {
	*sql.Tx
	re *regexp.Regexp
}

func (p *SchemaTxPool) strip(query string) string {
	cleaned := p.re.ReplaceAllString(query, "$2")
	cleaned = strings.ReplaceAll(cleaned, "FOR UPDATE SKIP LOCKED", "")
	cleaned = strings.ReplaceAll(cleaned, "FOR UPDATE", "")
	return cleaned
}

func (p *SchemaTxPool) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return p.Tx.ExecContext(ctx, p.strip(query), args...)
}

func (p *SchemaTxPool) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return p.Tx.QueryContext(ctx, p.strip(query), args...)
}

func (p *SchemaTxPool) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return p.Tx.QueryRowContext(ctx, p.strip(query), args...)
}

func (p *SchemaTxPool) PrepareContext(ctx context.Context, query string) (*sql.Stmt, error) {
	return p.Tx.PrepareContext(ctx, p.strip(query))
}

// Repositories contains all GORM repository implementations wired to the in-memory SQLite DB.
type Repositories struct {
	DB              *gorm.DB
	UserXpRepo      repository.UserXpRepository
	UserStreakRepo  repository.UserStreakRepository
	LeaderboardRepo repository.LeaderboardRepository
	BadgeRepo       repository.BadgeRepository
	OutboxRepo      port.OutboxRepository
	OutboxLogRepo   port.OutboxLogRepository
	TxManager       port.TransactionManager
}

// NewTestDB boots in-memory SQLite and creates tables matching 13-create-gamification-tables.sql.
func NewTestDB(t *testing.T) (*gorm.DB, *Repositories) {
	t.Helper()

	dsn := "file:" + t.Name() + "?mode=memory&cache=shared&_loc=UTC"
	rawDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("failed to open sqlite in-memory db: %v", err)
	}
	t.Cleanup(func() { _ = rawDB.Close() })

	ddls := []string{
		`CREATE TABLE IF NOT EXISTS user_xp (
			user_id TEXT PRIMARY KEY,
			xp INTEGER NOT NULL DEFAULT 0,
			level INTEGER NOT NULL DEFAULT 1,
			last_nutrition_reward_date DATE,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS xp_history (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			amount INTEGER NOT NULL,
			source TEXT NOT NULL,
			session_id TEXT,
			activity_date DATE NOT NULL,
			created_at DATETIME NOT NULL,
			UNIQUE (user_id, session_id)
		);`,
		`CREATE TABLE IF NOT EXISTS user_streak (
			user_id TEXT PRIMARY KEY,
			current_streak INTEGER NOT NULL DEFAULT 0,
			longest_streak INTEGER NOT NULL DEFAULT 0,
			last_active_date DATE,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS streak_history (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			activity_date DATE NOT NULL,
			activity_type TEXT NOT NULL,
			created_at DATETIME NOT NULL,
			UNIQUE (user_id, activity_date)
		);`,
		`CREATE TABLE IF NOT EXISTS user_snapshots (
			user_id TEXT PRIMARY KEY,
			display_name TEXT NOT NULL DEFAULT 'Gymer',
			avatar_url TEXT,
			updated_at DATETIME NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS user_badges (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			badge_code TEXT NOT NULL,
			unlocked_at DATETIME NOT NULL,
			metadata TEXT,
			UNIQUE (user_id, badge_code)
		);`,
		`CREATE TABLE IF NOT EXISTS outbox (
			id TEXT PRIMARY KEY,
			event_id TEXT NOT NULL UNIQUE,
			aggregate_type TEXT,
			aggregate_id TEXT,
			event_type TEXT NOT NULL,
			payload TEXT NOT NULL,
			partition_key TEXT NOT NULL,
			published INTEGER NOT NULL DEFAULT 0,
			published_at DATETIME,
			created_at DATETIME NOT NULL,
			status TEXT NOT NULL DEFAULT 'PENDING',
			retry_count INTEGER NOT NULL DEFAULT 0,
			max_retries INTEGER NOT NULL DEFAULT 5,
			last_error TEXT,
			locked_until DATETIME
		);`,
		`CREATE TABLE IF NOT EXISTS outbox_log (
			id TEXT PRIMARY KEY,
			event_id TEXT NOT NULL UNIQUE,
			event_type TEXT NOT NULL,
			payload TEXT NOT NULL,
			partition_key TEXT NOT NULL,
			status TEXT NOT NULL,
			error_message TEXT,
			processed_at DATETIME NOT NULL
		);`,
	}

	for _, ddl := range ddls {
		if _, err := rawDB.Exec(ddl); err != nil {
			t.Fatalf("failed to execute DDL on test db: %v\nDDL: %s", err, ddl)
		}
	}

	pool := NewSchemaStripPool(rawDB)
	gormDB, err := gorm.Open(sqlite.Dialector{Conn: pool}, &gorm.Config{
		Logger:                 logger.Default.LogMode(logger.Silent),
		SkipDefaultTransaction: true,
	})
	if err != nil {
		t.Fatalf("failed to open gorm db for test: %v", err)
	}

	repos := &Repositories{
		DB:              gormDB,
		UserXpRepo:      persistence.NewGormUserXpRepository(gormDB),
		UserStreakRepo:  persistence.NewGormUserStreakRepository(gormDB),
		LeaderboardRepo: persistence.NewGormLeaderboardRepository(gormDB),
		BadgeRepo:       persistence.NewGormBadgeRepository(gormDB),
		OutboxRepo:      persistence.NewGormOutboxRepository(gormDB),
		OutboxLogRepo:   persistence.NewGormOutboxLogRepository(gormDB),
		TxManager:       persistence.NewSQLTransactionManager(gormDB),
	}

	return gormDB, repos
}
