// Package testutil cung cấp helper khởi tạo in-memory database và repositories
// cho E2E và integration test của module Social.
package testutil

import (
	"context"
	"database/sql"
	"regexp"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/viethung213/gym-companion/internal/social/application/port"
	"github.com/viethung213/gym-companion/internal/social/domain/repository"
	"github.com/viethung213/gym-companion/internal/social/infrastructure/persistence"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// SchemaStripPool bọc *sql.DB và rewrite SQL trước khi thực thi,
// đổi `social`.`table` hoặc "social"."table" hoặc social.table → table
// để tương thích hoàn toàn với SQLite in-memory.
type SchemaStripPool struct {
	*sql.DB
	re *regexp.Regexp
}

// NewSchemaStripPool tạo SchemaStripPool từ *sql.DB.
func NewSchemaStripPool(db *sql.DB) *SchemaStripPool {
	re := regexp.MustCompile(`(?i)(["` + "`" + `]?social["` + "`" + `]?\.)(["` + "`" + `]?([a-zA-Z0-9_]+)["` + "`" + `]?)`)
	return &SchemaStripPool{DB: db, re: re}
}

func (p *SchemaStripPool) strip(query string) string {
	res := p.re.ReplaceAllString(query, "$2")
	return strings.ReplaceAll(res, "GREATEST(", "MAX(")
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
	res := p.re.ReplaceAllString(query, "$2")
	return strings.ReplaceAll(res, "GREATEST(", "MAX(")
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

// Repositories chứa tập hợp tất cả các repository đã wire vào test DB
type Repositories struct {
	DB               *gorm.DB
	FeedItemRepo     repository.FeedItemRepository
	FollowRepo       repository.FollowRepository
	InteractionRepo  repository.InteractionRepository
	UserSnapshotRepo repository.UserSnapshotRepository
	OutboxRepo       port.OutboxRepository
	OutboxLogRepo    port.OutboxLogRepository
	TxManager        port.TransactionManager
}

// MockEventPublisher ghi nhận các events được bắn ra
type MockEventPublisher struct {
	Published []any
}

func (m *MockEventPublisher) PublishEvents(_ context.Context, events []any) error {
	m.Published = append(m.Published, events...)
	return nil
}

// NewTestDB tạo GORM DB SQLite in-memory với các bảng của module Social
func NewTestDB(t *testing.T) (*gorm.DB, *Repositories) {
	t.Helper()

	dsn := "file:" + t.Name() + "?mode=memory&cache=shared&_loc=UTC"
	rawDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("testutil: failed to open raw sql.DB: %v", err)
	}
	t.Cleanup(func() { _ = rawDB.Close() })

	ddls := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			full_name TEXT,
			avatar_url TEXT,
			role TEXT DEFAULT 'user',
			updated_at DATETIME
		);`,
		`CREATE TABLE IF NOT EXISTS follows (
			id TEXT PRIMARY KEY,
			follower_id TEXT,
			following_id TEXT,
			created_at DATETIME,
			UNIQUE (follower_id, following_id)
		);`,
		`CREATE TABLE IF NOT EXISTS feed_items (
			id TEXT PRIMARY KEY,
			user_id TEXT,
			item_type TEXT,
			caption TEXT,
			media_urls BLOB,
			data BLOB,
			visibility TEXT,
			reaction_count INTEGER DEFAULT 0,
			comment_count INTEGER DEFAULT 0,
			created_at DATETIME,
			updated_at DATETIME
		);`,
		`CREATE TABLE IF NOT EXISTS reactions (
			id TEXT PRIMARY KEY,
			user_id TEXT,
			feed_item_id TEXT,
			reaction_type TEXT,
			created_at DATETIME,
			UNIQUE (user_id, feed_item_id)
		);`,
		`CREATE TABLE IF NOT EXISTS comments (
			id TEXT PRIMARY KEY,
			user_id TEXT,
			feed_item_id TEXT,
			parent_id TEXT,
			content TEXT,
			created_at DATETIME,
			updated_at DATETIME
		);`,
		`CREATE TABLE IF NOT EXISTS outbox (
			id TEXT PRIMARY KEY,
			event_id TEXT UNIQUE,
			event_type TEXT,
			payload BLOB,
			partition_key TEXT,
			created_at DATETIME,
			published NUMERIC DEFAULT 0,
			published_at DATETIME,
			status TEXT,
			locked_until DATETIME
		);`,
		`CREATE TABLE IF NOT EXISTS outbox_log (
			id TEXT PRIMARY KEY,
			event_id TEXT,
			event_type TEXT,
			payload BLOB,
			partition_key TEXT,
			processed_at DATETIME,
			status TEXT,
			error_message TEXT
		);`,
	}

	for _, ddl := range ddls {
		if _, execErr := rawDB.Exec(ddl); execErr != nil {
			t.Fatalf("testutil: failed to execute DDL: %v", execErr)
		}
	}

	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("testutil: failed to open gorm db: %v", err)
	}

	underlyingDB, err := db.DB()
	if err != nil {
		t.Fatalf("testutil: failed to get underlying sql.DB: %v", err)
	}
	pool := NewSchemaStripPool(underlyingDB)
	db.ConnPool = pool
	db.Statement.ConnPool = pool

	feedItemRepo := persistence.NewGormFeedItemRepository(db)
	followRepo := persistence.NewGormFollowRepository(db)
	interactionRepo := persistence.NewGormInteractionRepository(db)
	userSnapshotRepo := persistence.NewGormUserSnapshotRepository(db)
	outboxRepo := persistence.NewGormOutboxRepository(db)
	outboxLogRepo := persistence.NewGormOutboxLogRepository(db)
	txManager := persistence.NewSQLTransactionManager(db)

	return db, &Repositories{
		DB:               db,
		FeedItemRepo:     feedItemRepo,
		FollowRepo:       followRepo,
		InteractionRepo:  interactionRepo,
		UserSnapshotRepo: userSnapshotRepo,
		OutboxRepo:       outboxRepo,
		OutboxLogRepo:    outboxLogRepo,
		TxManager:        txManager,
	}
}
