package persistence_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/viethung213/gym-companion/internal/nutrition/application/port"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/entity"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/repository"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/vo"
	"github.com/viethung213/gym-companion/internal/nutrition/infrastructure/persistence"
	"github.com/viethung213/gym-companion/internal/nutrition/test/testutil"
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
	re := regexp.MustCompile("`[^`]+`\\.(`[^`]+`)")
	return &schemaStripPool{DB: db, re: re}
}

func (p *schemaStripPool) strip(query string) string {
	// Bỏ schema prefix: `nutrition`.`food_items` → `food_items`, nhưng giữ nguyên `excluded`.`col`
	return p.re.ReplaceAllStringFunc(query, func(match string) string {
		if strings.HasPrefix(match, "`excluded`.") {
			return match
		}
		parts := strings.SplitN(match, ".", 2)
		if len(parts) == 2 {
			return parts[1]
		}
		return match
	})
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
	return p.re.ReplaceAllStringFunc(query, func(match string) string {
		if strings.HasPrefix(match, "`excluded`.") {
			return match
		}
		parts := strings.SplitN(match, ".", 2)
		if len(parts) == 2 {
			return parts[1]
		}
		return match
	})
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

	dsn := "file:" + t.Name() + "?mode=memory&cache=shared&_loc=UTC"

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
			is_nutifood_product NUMERIC DEFAULT 0, price_per_100g REAL DEFAULT 0, cost_tier TEXT DEFAULT 'STANDARD', status TEXT,
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
			meals_json TEXT, created_at DATETIME, updated_at DATETIME,
			UNIQUE(user_id, plan_date)
		)`,
		`CREATE TABLE IF NOT EXISTS meal_histories (
			id TEXT PRIMARY KEY, user_id TEXT UNIQUE, created_at DATETIME, updated_at DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS meal_logs (
			id TEXT PRIMARY KEY, history_id TEXT, user_id TEXT,
			meal_type TEXT, meal_name TEXT, portion TEXT,
			calories REAL, protein REAL, carbs REAL, fat REAL, logged_at DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS outbox (
			id TEXT PRIMARY KEY, event_id TEXT UNIQUE, aggregate_type TEXT, aggregate_id TEXT, event_type TEXT,
			payload TEXT, partition_key TEXT, created_at DATETIME,
			published NUMERIC DEFAULT 0, published_at DATETIME,
			locked_until DATETIME, status TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS outbox_log (
			id TEXT PRIMARY KEY, event_id TEXT UNIQUE, event_type TEXT,
			payload TEXT, partition_key TEXT, processed_at DATETIME,
			status TEXT, error_message TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS weekly_insights (
			id TEXT PRIMARY KEY, user_id TEXT, start_date DATETIME, end_date DATETIME,
			overall_score INTEGER, summary TEXT, predicted_cooking_style TEXT,
			preferred_ingredients TEXT, improvement_areas TEXT,
			consumed_calories REAL, target_calories REAL,
			consumed_protein REAL, target_protein REAL,
			consumed_carbs REAL, target_carbs REAL,
			consumed_fat REAL, target_fat REAL,
			logged_meals_count INTEGER, total_meals_count INTEGER,
			created_at DATETIME, updated_at DATETIME,
			UNIQUE(user_id, start_date)
		)`,
		`CREATE TABLE IF NOT EXISTS user_allergies (
			id TEXT PRIMARY KEY, user_id TEXT, allergen_name TEXT,
			created_at DATETIME,
			UNIQUE(user_id, allergen_name)
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

	item, err := aggregate.NewFoodItem("fi-100", "Thịt bò nạc", "PROTEIN", 250, 26, 0, 15, []string{"Beef"}, "BEEF", "", false, 35000, "PREMIUM")
	if err != nil {
		t.Fatalf("NewFoodItem: %v", err)
	}
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
	history := aggregate.NewMealHistory("hist-db-1", "user-db-1")
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

	// 5. FindByUserIDAndDates
	plansDates, err := repo.FindByUserIDAndDates(ctx, "user-db-1", []time.Time{now})
	if err != nil {
		t.Fatalf("FindByUserIDAndDates: %v", err)
	}
	if len(plansDates) != 1 {
		t.Fatalf("expected 1 plan for specific dates, got %d", len(plansDates))
	}

}

func TestPostgresMealHistoryRepository_FindByUserIDAndDateRange_Integration(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	repo := persistence.NewPostgresMealHistoryRepository(db)
	ctx := context.Background()

	now := time.Now().UTC()

	// 1. Non-existent user returns nil, nil
	res, err := repo.FindByUserIDAndDateRange(ctx, "no-user", now.AddDate(0, 0, -6), now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res != nil {
		t.Fatalf("expected nil for non-existent user, got %v", res)
	}

	// 2. Setup user with logs on different dates
	history := aggregate.NewMealHistory("hist-range-1", "user-range-1")
	// Log 1: 15 ngày trước (ngoài phạm vi 7 ngày)
	history.AddMealLog(aggregate.NewMealLog("log-old", "hist-range-1", "user-range-1", "Lunch", "Món cũ", "1 dĩa", 400, 30, 40, 10, now.AddDate(0, 0, -15)))
	// Log 2: 4 ngày trước (trong phạm vi 7 ngày)
	history.AddMealLog(aggregate.NewMealLog("log-mid", "hist-range-1", "user-range-1", "Dinner", "Món 4 ngày trước", "1 bát", 500, 35, 50, 15, now.AddDate(0, 0, -4)))
	// Log 3: Hôm nay (trong phạm vi 7 ngày)
	history.AddMealLog(aggregate.NewMealLog("log-today", "hist-range-1", "user-range-1", "Breakfast", "Món hôm nay", "1 phần", 300, 20, 30, 5, now))

	if err := repo.Save(ctx, history); err != nil {
		t.Fatalf("failed to save history: %v", err)
	}

	// 3. Query 7 ngày gần nhất [now - 6 ngày, now]
	from := now.AddDate(0, 0, -6)
	to := now
	filteredHist, err := repo.FindByUserIDAndDateRange(ctx, "user-range-1", from, to)
	if err != nil {
		t.Fatalf("unexpected error on FindByUserIDAndDateRange: %v", err)
	}
	if filteredHist == nil {
		t.Fatal("expected non-nil meal history")
	}

	// Chỉ có 2 log trong 7 ngày qua, log 15 ngày trước phải bị loại bỏ hoàn toàn
	logs := filteredHist.MealLogs()
	if len(logs) != 2 {
		t.Fatalf("got %d logs, want 2 (old log should be excluded)", len(logs))
	}
	if logs[0].MealName() != "Món 4 ngày trước" {
		t.Errorf("got log[0]=%s, want 'Món 4 ngày trước'", logs[0].MealName())
	}
	if logs[1].MealName() != "Món hôm nay" {
		t.Errorf("got log[1]=%s, want 'Món hôm nay'", logs[1].MealName())
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

func TestPostgresWeeklyInsightRepository_Integration(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	repo := persistence.NewPostgresWeeklyInsightRepository(db)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Second)
	startDate := now.AddDate(0, 0, -6)

	// 1. GetLatestByUserID on empty DB returns nil, nil
	latest, err := repo.GetLatestByUserID(ctx, "non-existent")
	if err != nil {
		t.Fatalf("unexpected error getting latest: %v", err)
	}
	if latest != nil {
		t.Fatalf("expected nil insight, got %v", latest)
	}

	// 2. Save new WeeklyInsight
	insight := testutil.MustNewWeeklyInsight(
		"wi-1",
		"u-insight-1",
		startDate,
		now,
		82,
		"Tuần vừa rồi kiểm soát calo rất tốt.",
		"Hấp, luộc thanh đạm",
		[]string{"Ức gà", "Bông cải xanh", "Khoai lang"},
		[]entity.WeeklyImprovementArea{
			{
				Nutrient:    "PROTEIN",
				Action:      "INCREASE",
				DeltaAmount: 10,
				Suggestion:  "Tăng lượng đạm vào bữa phụ",
			},
		},
		&entity.WeeklyMacroSummary{
			ConsumedCalories: 14000,
			TargetCalories:   14500,
			ConsumedProtein:  850,
			TargetProtein:    900,
			ConsumedCarbs:    1500,
			TargetCarbs:      1500,
			ConsumedFat:      400,
			TargetFat:        450,
			LoggedMealsCount: 26,
			TotalMealsCount:  28,
		},
	)

	if err := repo.Save(ctx, insight); err != nil {
		t.Fatalf("failed to save weekly insight: %v", err)
	}

	// 3. GetLatestByUserID
	fetched, err := repo.GetLatestByUserID(ctx, "u-insight-1")
	if err != nil {
		t.Fatalf("unexpected error fetching latest: %v", err)
	}
	if fetched == nil {
		t.Fatal("expected non-nil insight")
	}
	if fetched.OverallScore() != 82 {
		t.Errorf("got score %d, want 82", fetched.OverallScore())
	}
	if fetched.PredictedCookingStyle() != "Hấp, luộc thanh đạm" {
		t.Errorf("got style %s, want Hấp, luộc thanh đạm", fetched.PredictedCookingStyle())
	}
	if len(fetched.PreferredIngredients()) != 3 {
		t.Fatalf("got %d ingredients, want 3", len(fetched.PreferredIngredients()))
	}
	if len(fetched.ImprovementAreas()) != 1 {
		t.Fatalf("got %d improvement areas, want 1", len(fetched.ImprovementAreas()))
	}
	if fetched.ImprovementAreas()[0].Nutrient != "PROTEIN" {
		t.Errorf("got nutrient %s, want PROTEIN", fetched.ImprovementAreas()[0].Nutrient)
	}
	if fetched.WeeklySummary().LoggedMealsCount != 26 {
		t.Errorf("got logged meals count %d, want 26", fetched.WeeklySummary().LoggedMealsCount)
	}

	// 4. GetByUserIDAndStartDate
	byDate, err := repo.GetByUserIDAndStartDate(ctx, "u-insight-1", startDate)
	if err != nil {
		t.Fatalf("unexpected error fetching by date: %v", err)
	}
	if byDate == nil {
		t.Fatal("expected non-nil insight by date")
	}
	if byDate.OverallScore() != 82 {
		t.Errorf("got score %d, want 82", byDate.OverallScore())
	}

	// 5. Upsert: Update score on same user and startDate
	updatedInsight := testutil.MustNewWeeklyInsight(
		"wi-2",
		"u-insight-1",
		startDate,
		now,
		95,
		"Đã cập nhật: điểm số xuất sắc",
		"Hấp, luộc thanh đạm",
		[]string{"Ức gà"},
		[]entity.WeeklyImprovementArea{},
		&entity.WeeklyMacroSummary{
			ConsumedCalories: 14500,
			TargetCalories:   14500,
		},
	)
	if err := repo.Save(ctx, updatedInsight); err != nil {
		t.Fatalf("failed to update weekly insight: %v", err)
	}

	fetchedUpdated, err := repo.GetLatestByUserID(ctx, "u-insight-1")
	if err != nil {
		t.Fatalf("unexpected error fetching updated: %v", err)
	}
	if fetchedUpdated.OverallScore() != 95 {
		t.Errorf("got updated score %d, want 95", fetchedUpdated.OverallScore())
	}
}

func TestPostgresUserAllergyRepository(t *testing.T) {
	db := setupTestDB(t)
	repo := persistence.NewPostgresUserAllergyRepository(db)
	ctx := context.Background()

	userID1 := "user-allergy-1"
	userID2 := "user-allergy-2"

	makeEntities := func(uid string, names ...string) []*entity.UserAllergy {
		list := make([]*entity.UserAllergy, 0, len(names))
		for _, n := range names {
			e, _ := entity.NewUserAllergy(n+"-id", uid, n, time.Now())
			list = append(list, e)
		}
		return list
	}

	// 1. Initially empty
	allergies, err := repo.FindAllergiesByUserID(ctx, userID1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(allergies) != 0 {
		t.Fatalf("expected 0 allergies, got %d", len(allergies))
	}

	// 2. SetAllergies for userID1
	err = repo.SetAllergies(ctx, userID1, makeEntities(userID1, "PEANUT", "SHRIMP"))
	if err != nil {
		t.Fatalf("unexpected error on SetAllergies: %v", err)
	}

	allergies, err = repo.FindAllergiesByUserID(ctx, userID1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(allergies) != 2 {
		t.Fatalf("expected 2 allergies, got %d", len(allergies))
	}

	// 3. SetAllergies smart merge (add dairy, remove shrimp)
	err = repo.SetAllergies(ctx, userID1, makeEntities(userID1, "PEANUT", "DAIRY"))
	if err != nil {
		t.Fatalf("unexpected error on SetAllergies update: %v", err)
	}

	allergies, err = repo.FindAllergiesByUserID(ctx, userID1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(allergies) != 2 || allergies[0].AllergenName() != "DAIRY" || allergies[1].AllergenName() != "PEANUT" {
		t.Fatalf("expected [DAIRY PEANUT], got [%s %s]", allergies[0].AllergenName(), allergies[1].AllergenName())
	}

	// 4. BatchFindAllergiesByUserIDs
	_ = repo.SetAllergies(ctx, userID2, makeEntities(userID2, "GLUTEN", "SOY"))
	batchRes, err := repo.BatchFindAllergiesByUserIDs(ctx, []string{userID1, userID2, "user-no-allergies"})
	if err != nil {
		t.Fatalf("unexpected error on BatchFind: %v", err)
	}
	if len(batchRes[userID1]) != 2 {
		t.Fatalf("expected 2 allergies for user1, got %d", len(batchRes[userID1]))
	}
	if len(batchRes[userID2]) != 2 {
		t.Fatalf("expected 2 allergies for user2, got %d", len(batchRes[userID2]))
	}
	if len(batchRes["user-no-allergies"]) != 0 {
		t.Fatalf("expected 0 allergies for non-existent user, got %d", len(batchRes["user-no-allergies"]))
	}

	// 5. SetAllergies idempotent update with identical data (should be a no-op)
	err = repo.SetAllergies(ctx, userID1, makeEntities(userID1, "DAIRY", "PEANUT"))
	if err != nil {
		t.Fatalf("unexpected error on idempotent SetAllergies: %v", err)
	}

	// 6. Clear allergies by passing empty slice
	err = repo.SetAllergies(ctx, userID1, []*entity.UserAllergy{})
	if err != nil {
		t.Fatalf("unexpected error on clearing SetAllergies: %v", err)
	}
	allergies, err = repo.FindAllergiesByUserID(ctx, userID1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(allergies) != 0 {
		t.Fatalf("expected 0 allergies after clear, got %d", len(allergies))
	}
}
