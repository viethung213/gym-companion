package persistence_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"regexp"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/viethung213/gym-companion/internal/nutrition/application/port"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/repository"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/vo"
	"github.com/viethung213/gym-companion/internal/nutrition/infrastructure/persistence"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// schemaStripPool là ConnPool tùy chỉnh bọc *sql.DB và rewrite SQL trước khi thực thi.
// glebarez SQLite tự strip schema cho INSERT nhưng giữ nguyên cho SELECT/FROM.
// Pool này đảm bảo mọi SQL đều dùng tên bảng đơn giản (không schema prefix).
type schemaStripPool struct {
	*sql.DB
	re *regexp.Regexp
}

func newSchemaStripPool(db *sql.DB) *schemaStripPool {
	// Khớp `schema`.`table` trong SQL do glebarez/GORM sinh ra (dùng backtick).
	// VD: `nutrition`.`food_items` → `food_items`
	re := regexp.MustCompile("`[^`]+`\\.(`[^`]+`)")
	return &schemaStripPool{DB: db, re: re}
}

func (p *schemaStripPool) strip(query string) string {
	// Bỏ schema prefix: `nutrition`.`food_items` → `food_items`
	return p.re.ReplaceAllString(query, "$1")
}

func (p *schemaStripPool) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	stripped := p.strip(query)
	return p.DB.ExecContext(ctx, stripped, args...)
}

func (p *schemaStripPool) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	stripped := p.strip(query)
	return p.DB.QueryContext(ctx, stripped, args...)
}

func (p *schemaStripPool) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	stripped := p.strip(query)
	return p.DB.QueryRowContext(ctx, stripped, args...)
}

func (p *schemaStripPool) PrepareContext(ctx context.Context, query string) (*sql.Stmt, error) {
	stripped := p.strip(query)
	return p.DB.PrepareContext(ctx, stripped)
}

// BeginTx triển khai gorm.ConnPoolBeginner — trả về schemaTxPool bọc *sql.Tx
// để các câu SQL trong transaction cũng được strip schema prefix.
func (p *schemaStripPool) BeginTx(ctx context.Context, opts *sql.TxOptions) (gorm.ConnPool, error) {
	tx, err := p.DB.BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}
	return &schemaTxPool{Tx: tx, re: p.re}, nil
}

// schemaTxPool bọc *sql.Tx với cùng logic strip schema.
// Được trả về bởi BeginTx để đảm bảo SQL trong transaction cũng được rewrite.
type schemaTxPool struct {
	*sql.Tx
	re *regexp.Regexp
}

func (p *schemaTxPool) strip(query string) string {
	return p.re.ReplaceAllString(query, "$1")
}

func (p *schemaTxPool) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return p.Tx.ExecContext(ctx, p.strip(query), args...)
}

func (p *schemaTxPool) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	return p.Tx.QueryContext(ctx, p.strip(query), args...)
}

func (p *schemaTxPool) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return p.Tx.QueryRowContext(ctx, p.strip(query), args...)
}

func (p *schemaTxPool) PrepareContext(ctx context.Context, query string) (*sql.Stmt, error) {
	return p.Tx.PrepareContext(ctx, p.strip(query))
}

// setupTestDB mở SQLite in-memory với schemaStripPool để strip `schema`.`table`
// khỏi toàn bộ SQL do GORM/glebarez sinh ra, rồi tạo bảng bằng tên đơn giản.
func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	const dsn = "file::memory:?cache=shared&_loc=UTC"

	// Mở raw DB để tạo bảng
	rawDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("failed to open raw sql.DB: %v", err)
	}
	t.Cleanup(func() { _ = rawDB.Close() })

	if err = createTestTables(rawDB); err != nil {
		t.Fatalf("failed to create test tables: %v", err)
	}

	// Mở GORM DB
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("failed to open gorm db: %v", err)
	}

	// Lấy underlying *sql.DB và inject schemaStripPool vào cả hai vị trí:
	// - db.ConnPool: dùng cho các operations không có statement
	// - db.Statement.ConnPool: dùng cho query execution (path chính của GORM)
	underlyingDB, err := db.DB()
	if err != nil {
		t.Fatalf("failed to get sql.DB: %v", err)
	}
	pool := newSchemaStripPool(underlyingDB)
	db.ConnPool = pool
	db.Statement.ConnPool = pool

	return db
}

// createTestTables tạo các bảng với tên đơn giản (không schema prefix).
// glebarez dialect strip schema thành tên này khi sinh SQL cho INSERT,
// và schemaStripPool strip cho SELECT/WHERE.
func createTestTables(db *sql.DB) error {
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

func TestPostgresFoodItemRepository_Integration(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	repo := persistence.NewPostgresFoodItemRepository(db)
	ctx := context.Background()

	item := aggregate.NewFoodItem("fi-100", "Thịt bò nạc", "PROTEIN", 250, 26, 0, 15, []string{"Beef"}, "BEEF", "", false)
	_ = item.SubmitForApproval()
	_ = item.Approve()

	// 1. Save
	if err := repo.Save(ctx, item); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// 2. FindByID
	found, err := repo.FindByID(ctx, "fi-100")
	if err != nil || found == nil {
		t.Fatalf("FindByID: %v", err)
	}
	if got := found.Name(); got != "Thịt bò nạc" {
		t.Fatalf("got name %q, want %q", got, "Thịt bò nạc")
	}

	// 3. FindByName
	foundByName, err := repo.FindByName(ctx, "thịt bò nạc")
	if err != nil || foundByName == nil {
		t.Fatalf("FindByName: %v", err)
	}

	// 4. FindActiveCatalog
	catalog, err := repo.FindActiveCatalog(ctx)
	if err != nil || len(catalog) < 1 {
		t.Fatalf("FindActiveCatalog: got len=%d, err=%v", len(catalog), err)
	}
}

func TestPostgresNutritionPlanRepository_Integration(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	repo := persistence.NewPostgresNutritionPlanRepository(db)
	historyRepo := persistence.NewPostgresMealHistoryRepository(db)
	ctx := context.Background()

	now := time.Now().UTC()
	alloc, _ := vo.NewCalorieAllocation(2000, 150, 200, 66)
	plan := aggregate.NewNutritionPlan("plan-db-1", "user-db-1", now, alloc, nil)

	// 1. Save Plan
	if err := repo.Save(ctx, plan); err != nil {
		t.Fatalf("Save plan: %v", err)
	}

	// 2. FindByUserIDAndDate
	found, err := repo.FindByUserIDAndDate(ctx, "user-db-1", now)
	if err != nil || found == nil {
		t.Fatalf("FindByUserIDAndDate: %v", err)
	}
	if got := found.ID(); got != "plan-db-1" {
		t.Fatalf("got plan ID %q, want %q", got, "plan-db-1")
	}

	// 3. Lưu MealHistory để test FindActiveUserIDs
	history := aggregate.NewMealHistory("hist-db-1", "user-db-1", vo.NewLockoutRegistry(nil))
	logItem := aggregate.NewMealLog("log-db-1", "hist-db-1", "user-db-1", "Lunch", "Ức gà", "1 dĩa", 400, 35, 10, 5, now)
	history.AddMealLog(logItem)
	_ = historyRepo.Save(ctx, history)

	// 4. FindActiveUserIDs
	activeUsers, err := repo.FindActiveUserIDs(ctx, 7)
	if err != nil {
		t.Fatalf("FindActiveUserIDs: %v", err)
	}
	if len(activeUsers) != 1 || activeUsers[0] != "user-db-1" {
		t.Fatalf("got active users %v, want ['user-db-1']", activeUsers)
	}
}

func TestPostgresRecipeCacheRepository_Integration(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	repo := persistence.NewPostgresRecipeCacheRepository(db)
	ctx := context.Background()

	recipe := &repository.CachedRecipe{
		ID:             "rec-1",
		IngredientHash: "hash1234",
		RecipeName:     "Ức gà hấp",
		CookingStyle:   "Hấp",
		CreatedAt:      time.Now(),
	}

	if err := repo.Save(ctx, recipe); err != nil {
		t.Fatalf("Save recipe: %v", err)
	}

	found, err := repo.FindByHash(ctx, "hash1234")
	if err != nil || found == nil {
		t.Fatalf("FindByHash: %v", err)
	}
	if got := found.RecipeName; got != "Ức gà hấp" {
		t.Fatalf("got recipe name %q, want %q", got, "Ức gà hấp")
	}
}

func TestPostgresOutboxRepository_Integration(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	repo := persistence.NewPostgresOutboxRepository(db)
	ctx := context.Background()

	payload, _ := json.Marshal(map[string]string{"event": "plan_generated", "user": "u-1"})

	// 1. Save
	record := &port.OutboxRecord{
		ID:           "ob-1",
		EventID:      "ev-1",
		EventType:    "nutrition.plan.generated",
		Payload:      payload,
		PartitionKey: "u-1",
	}
	if err := repo.Save(ctx, record); err != nil {
		t.Fatalf("Save outbox: %v", err)
	}

	// 2. SaveEvent helper
	type evtData struct {
		UserID string `json:"userId"`
	}
	if err := repo.SaveEvent(ctx, "ev-2", "nutrition.meal.logged", "u-2", evtData{UserID: "u-2"}); err != nil {
		t.Fatalf("SaveEvent: %v", err)
	}

	// 3. FetchUnpublished
	unpublished, err := repo.FetchUnpublished(ctx, 10)
	if err != nil {
		t.Fatalf("FetchUnpublished: %v", err)
	}
	if len(unpublished) < 2 {
		t.Fatalf("got %d unpublished, want >= 2", len(unpublished))
	}

	// 4. MarkAsPublished
	if err = repo.MarkAsPublished(ctx, []string{"ob-1"}); err != nil {
		t.Fatalf("MarkAsPublished: %v", err)
	}

	// 5. FetchUnpublished after mark — should return 1 less
	unpublished2, err := repo.FetchUnpublished(ctx, 10)
	if err != nil {
		t.Fatalf("FetchUnpublished after mark: %v", err)
	}
	if len(unpublished2) != len(unpublished)-1 {
		t.Fatalf("got %d unpublished after mark, want %d", len(unpublished2), len(unpublished)-1)
	}

	// 6. MarkPublished (single)
	if err = repo.MarkPublished(ctx, "ev-2"); err != nil {
		t.Fatalf("MarkPublished: %v", err)
	}

	// 7. ProcessBatch — với callback mock
	// Thêm 1 record mới để ProcessBatch có dữ liệu
	_ = repo.Save(ctx, &port.OutboxRecord{
		ID: "ob-3", EventID: "ev-3", EventType: "nutrition.plan.generated",
		Payload: payload, PartitionKey: "u-3",
	})

	called := false
	err = repo.ProcessBatch(ctx, 10, func(_ context.Context, records []port.OutboxRecord) error {
		called = true
		if len(records) == 0 {
			t.Error("ProcessBatch callback received 0 records")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("ProcessBatch: %v", err)
	}
	if !called {
		t.Fatal("ProcessBatch callback was not called")
	}
}

func TestPostgresOutboxLogRepository_Integration(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	repo := persistence.NewPostgresOutboxLogRepository(db)
	ctx := context.Background()

	payload, _ := json.Marshal(map[string]string{"key": "val"})

	// 1. IsProcessed — không tồn tại
	ok, err := repo.IsProcessed(ctx, "ev-log-1")
	if err != nil {
		t.Fatalf("IsProcessed (not found): %v", err)
	}
	if ok {
		t.Fatal("expected IsProcessed=false for non-existent event")
	}

	// 2. IsProcessed — empty eventID
	ok, err = repo.IsProcessed(ctx, "")
	if err != nil || ok {
		t.Fatalf("IsProcessed(\"\") = %v, %v; want false, nil", ok, err)
	}

	// 3. SaveLog
	logRecord := &port.OutboxLogRecord{
		ID:           "olog-1",
		EventID:      "ev-log-1",
		EventType:    "nutrition.plan.generated",
		Payload:      payload,
		PartitionKey: "u-1",
		Status:       "PROCESSED",
	}
	if err = repo.SaveLog(ctx, logRecord); err != nil {
		t.Fatalf("SaveLog: %v", err)
	}

	// 4. IsProcessed — setelah SaveLog dengan status PROCESSED
	ok, err = repo.IsProcessed(ctx, "ev-log-1")
	if err != nil {
		t.Fatalf("IsProcessed after SaveLog: %v", err)
	}
	if !ok {
		t.Fatal("expected IsProcessed=true after SaveLog with PROCESSED status")
	}
}
