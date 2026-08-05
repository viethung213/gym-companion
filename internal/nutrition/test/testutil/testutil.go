// Package testutil cung cấp các helper dùng chung cho integration test và e2e test
// của module nutrition. Bao gồm: khởi tạo SQLite in-memory với schema strip,
// mock factories cho các port/repository interface.
package testutil

import (
	"context"
	"database/sql"
	"regexp"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/repository"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/vo"
	"github.com/viethung213/gym-companion/internal/nutrition/infrastructure/persistence"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ─────────────────────────────────────────────────────────────────────────────
// SQLite schema-strip pool (giống persistence_test nhưng export ra để tái sử dụng)
// ─────────────────────────────────────────────────────────────────────────────

// SchemaStripPool bọc *sql.DB và rewrite SQL trước khi thực thi,
// đổi `nutrition`.`table` → `table` để tương thích với SQLite in-memory.
type SchemaStripPool struct {
	*sql.DB
	re *regexp.Regexp
}

// NewSchemaStripPool tạo SchemaStripPool từ *sql.DB.
func NewSchemaStripPool(db *sql.DB) *SchemaStripPool {
	re := regexp.MustCompile("`[^`]+`\\.(`[^`]+`)")
	return &SchemaStripPool{DB: db, re: re}
}

func (p *SchemaStripPool) strip(query string) string {
	return p.re.ReplaceAllString(query, "$1")
}

// ExecContext implements gorm.ConnPool.
func (p *SchemaStripPool) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return p.DB.ExecContext(ctx, p.strip(query), args...)
}

// QueryContext implements gorm.ConnPool.
func (p *SchemaStripPool) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	return p.DB.QueryContext(ctx, p.strip(query), args...)
}

// QueryRowContext implements gorm.ConnPool.
func (p *SchemaStripPool) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return p.DB.QueryRowContext(ctx, p.strip(query), args...)
}

// PrepareContext implements gorm.ConnPool.
func (p *SchemaStripPool) PrepareContext(ctx context.Context, query string) (*sql.Stmt, error) {
	return p.DB.PrepareContext(ctx, p.strip(query))
}

// BeginTx implements gorm.ConnPoolBeginner — trả về SchemaTxPool để SQL trong
// transaction cũng được strip schema.
func (p *SchemaStripPool) BeginTx(ctx context.Context, opts *sql.TxOptions) (gorm.ConnPool, error) {
	tx, err := p.DB.BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}
	return &SchemaTxPool{Tx: tx, re: p.re}, nil
}

// SchemaTxPool bọc *sql.Tx với cùng logic strip schema.
type SchemaTxPool struct {
	*sql.Tx
	re *regexp.Regexp
}

func (p *SchemaTxPool) strip(query string) string {
	return p.re.ReplaceAllString(query, "$1")
}

// ExecContext implements gorm.ConnPool.
func (p *SchemaTxPool) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return p.Tx.ExecContext(ctx, p.strip(query), args...)
}

// QueryContext implements gorm.ConnPool.
func (p *SchemaTxPool) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	return p.Tx.QueryContext(ctx, p.strip(query), args...)
}

// QueryRowContext implements gorm.ConnPool.
func (p *SchemaTxPool) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return p.Tx.QueryRowContext(ctx, p.strip(query), args...)
}

// PrepareContext implements gorm.ConnPool.
func (p *SchemaTxPool) PrepareContext(ctx context.Context, query string) (*sql.Stmt, error) {
	return p.Tx.PrepareContext(ctx, p.strip(query))
}

// ─────────────────────────────────────────────────────────────────────────────
// Database setup
// ─────────────────────────────────────────────────────────────────────────────

// NewTestDB tạo GORM DB trỏ vào SQLite in-memory với SchemaStripPool.
// Tất cả bảng được tạo bằng raw SQL để tránh vấn đề AutoMigrate + schema prefix.
// DSN có thể được truyền vào (dùng cho test cần shared DB);
// truyền "" để dùng DSN mặc định (isolated per-test).
func NewTestDB(t *testing.T, dsn string) *gorm.DB {
	t.Helper()

	if dsn == "" {
		dsn = "file::memory:?cache=shared&_loc=UTC"
	}

	rawDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("testutil: failed to open raw sql.DB: %v", err)
	}
	t.Cleanup(func() { _ = rawDB.Close() })

	if err = CreateSchema(rawDB); err != nil {
		t.Fatalf("testutil: failed to create schema: %v", err)
	}

	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("testutil: failed to open gorm db: %v", err)
	}

	underlyingDB, err := db.DB()
	if err != nil {
		t.Fatalf("testutil: failed to get sql.DB: %v", err)
	}
	pool := NewSchemaStripPool(underlyingDB)
	db.ConnPool = pool
	db.Statement.ConnPool = pool

	return db
}

// CreateSchema tạo toàn bộ bảng trong SQLite với tên không có schema prefix.
// glebarez dialect tự strip "nutrition." khi sinh INSERT; SchemaStripPool strip
// "nutrition." cho SELECT/FROM. Bảng tương ứng là tên sau dấu chấm.
func CreateSchema(db *sql.DB) error {
	ddls := []string{
		`CREATE TABLE IF NOT EXISTS food_items (
			id TEXT PRIMARY KEY, name TEXT COLLATE NOCASE, category TEXT,
			calories_per_100g REAL, protein_per_100g REAL, carbs_per_100g REAL, fat_per_100g REAL,
			allergen_tags TEXT, protein_source TEXT, carb_source TEXT,
			is_nutifood_product NUMERIC DEFAULT 0, status TEXT,
			created_at DATETIME, updated_at DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS recipes (
			id TEXT PRIMARY KEY, ingredient_hash TEXT UNIQUE,
			recipe_name TEXT, cooking_style TEXT,
			ingredients_json TEXT, cooking_steps TEXT, created_at DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS nutrition_plans (
			id TEXT PRIMARY KEY, user_id TEXT, plan_date DATETIME,
			target_calories REAL, target_protein REAL, target_carbs REAL, target_fat REAL,
			meals_json TEXT, created_at DATETIME, updated_at DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS meal_histories (
			id TEXT PRIMARY KEY, user_id TEXT UNIQUE, created_at DATETIME, updated_at DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS meal_logs (
			id TEXT PRIMARY KEY, history_id TEXT, user_id TEXT,
			meal_type TEXT, meal_name TEXT, portion TEXT,
			calories REAL, protein REAL, carbs REAL, fat REAL, logged_at DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS lockout_registries (
			id TEXT PRIMARY KEY, user_id TEXT, item_type TEXT,
			item_name TEXT, unlocked_at DATETIME, created_at DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS outbox (
			id TEXT PRIMARY KEY, event_id TEXT UNIQUE, event_type TEXT,
			payload TEXT, partition_key TEXT, created_at DATETIME,
			published NUMERIC DEFAULT 0, published_at DATETIME,
			locked_until DATETIME, status TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS outbox_log (
			id TEXT PRIMARY KEY, event_id TEXT, event_type TEXT,
			payload TEXT, partition_key TEXT, processed_at DATETIME,
			status TEXT, error_message TEXT
		)`,
	}
	for _, ddl := range ddls {
		if _, err := db.Exec(ddl); err != nil {
			return err
		}
	}
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Repository factories (dùng real SQLite repositories)
// ─────────────────────────────────────────────────────────────────────────────

// Repos chứa toàn bộ repositories được khởi tạo cho một test DB.
type Repos struct {
	FoodItem      *persistence.PostgresFoodItemRepository
	NutritionPlan *persistence.PostgresNutritionPlanRepository
	MealHistory   *persistence.PostgresMealHistoryRepository
	RecipeCache   *persistence.PostgresRecipeCacheRepository
	Outbox        *persistence.PostgresOutboxRepository
	OutboxLog     *persistence.PostgresOutboxLogRepository
}

// NewRepos tạo toàn bộ repositories từ một *gorm.DB.
func NewRepos(db *gorm.DB) *Repos {
	return &Repos{
		FoodItem:      persistence.NewPostgresFoodItemRepository(db),
		NutritionPlan: persistence.NewPostgresNutritionPlanRepository(db),
		MealHistory:   persistence.NewPostgresMealHistoryRepository(db),
		RecipeCache:   persistence.NewPostgresRecipeCacheRepository(db),
		Outbox:        persistence.NewPostgresOutboxRepository(db),
		OutboxLog:     persistence.NewPostgresOutboxLogRepository(db),
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Mock factories
// ─────────────────────────────────────────────────────────────────────────────

// MockAIService là in-memory stub triển khai repository.AIService.
type MockAIService struct {
	// EstimateResult có thể được override trong từng test case.
	EstimateResult *repository.EstimatedNutrientResult
	// GenerateResults có thể được override trong từng test case.
	GenerateResults []repository.GeneratedRecipeResult
}

// SelectCreativeMealOptions implements repository.AIService.
//
//nolint:gocritic // mock implementation of domain interface
func (m *MockAIService) SelectCreativeMealOptions(
	_ context.Context,
	_ repository.AIMenuPromptContext,
	_ vo.LockoutRegistry,
) ([]repository.GeneratedRecipeResult, error) {
	if m.GenerateResults != nil {
		return m.GenerateResults, nil
	}
	return []repository.GeneratedRecipeResult{
		{
			RecipeName:   "Ức gà áp chảo",
			CookingSteps: []string{"Ướp 10 phút", "Áp chảo 15 phút"},
			SupplementaryIngredients: []aggregate.IngredientGram{
				aggregate.NewIngredientGram("Ức gà", 150, false),
				aggregate.NewIngredientGram("Khoai lang", 200, false),
				aggregate.NewIngredientGram("Bông cải xanh", 100, false),
			},
		},
	}, nil
}

// EstimateNutrient implements repository.AIService.
func (m *MockAIService) EstimateNutrient(
	_ context.Context, _, _ string,
) (*repository.EstimatedNutrientResult, error) {
	if m.EstimateResult != nil {
		return m.EstimateResult, nil
	}
	return &repository.EstimatedNutrientResult{
		Calories: 450,
		Protein:  35,
		Carbs:    40,
		Fat:      12,
	}, nil
}

// GenerateNutritionInsight implements repository.AIService.
func (m *MockAIService) GenerateNutritionInsight(
	_ context.Context, _ repository.InsightPromptContext,
) (*repository.NutritionInsightResult, error) {
	return &repository.NutritionInsightResult{
		Summary:     "stub insight",
		WeeklyTrend: "STABLE",
	}, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Domain object factories
// ─────────────────────────────────────────────────────────────────────────────

// NewApprovedFoodItem tạo FoodItem đã được approve, sẵn sàng dùng trong test.
func NewApprovedFoodItem(id, name, category string, cal, protein, carbs, fat float64) *aggregate.FoodItem {
	item := aggregate.NewFoodItem(id, name, category, cal, protein, carbs, fat, nil, "", "", false)
	_ = item.SubmitForApproval()
	_ = item.Approve()
	return item
}

// NewTestNutritionPlan tạo NutritionPlan với CalorieAllocation hợp lệ.
func NewTestNutritionPlan(id, userID string, planDate time.Time) *aggregate.NutritionPlan {
	alloc, _ := vo.NewCalorieAllocation(2000, 150, 200, 66)
	return aggregate.NewNutritionPlan(id, userID, planDate, alloc, nil)
}

// NewTestMealHistory tạo MealHistory rỗng cho userID.
func NewTestMealHistory(id, userID string) *aggregate.MealHistory {
	return aggregate.NewMealHistory(id, userID, vo.NewLockoutRegistry(nil))
}
