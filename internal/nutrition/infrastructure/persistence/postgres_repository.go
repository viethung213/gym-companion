package persistence

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/repository"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/vo"
)

type PostgresFoodItemRepository struct {
	db *gorm.DB
}

var _ repository.FoodItemRepository = (*PostgresFoodItemRepository)(nil)

func NewPostgresFoodItemRepository(db *gorm.DB) *PostgresFoodItemRepository {
	return &PostgresFoodItemRepository{db: db}
}

func (r *PostgresFoodItemRepository) FindByID(ctx context.Context, id string) (*aggregate.FoodItem, error) {
	var gormItem GormFoodItem
	db := getDB(ctx, r.db)
	if err := db.First(&gormItem, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("postgres food repo find id: %w", err)
	}
	return gormItem.ToDomain(), nil
}

func (r *PostgresFoodItemRepository) FindByName(ctx context.Context, name string) (*aggregate.FoodItem, error) {
	var gormItem GormFoodItem
	db := getDB(ctx, r.db)
	if err := db.First(&gormItem, "LOWER(name) = LOWER(?)", name).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("postgres food repo find name: %w", err)
	}
	return gormItem.ToDomain(), nil
}

func (r *PostgresFoodItemRepository) FindActiveCatalog(ctx context.Context) ([]vo.FoodNutrient, error) {
	var gormItems []GormFoodItem
	db := getDB(ctx, r.db)
	if err := db.Where("status = ?", aggregate.FoodStatusActive).Find(&gormItems).Error; err != nil {
		return nil, fmt.Errorf("postgres food repo active catalog: %w", err)
	}

	result := make([]vo.FoodNutrient, 0, len(gormItems))
	for _, item := range gormItems {
		result = append(result, item.ToNutrientDomain())
	}
	return result, nil
}

func (r *PostgresFoodItemRepository) FindNutiFoodProducts(ctx context.Context) ([]vo.FoodNutrient, error) {
	var gormItems []GormFoodItem
	db := getDB(ctx, r.db)
	if err := db.Where("is_nutifood_product = ? AND status = ?", true, aggregate.FoodStatusActive).Find(&gormItems).Error; err != nil {
		return nil, fmt.Errorf("postgres food repo nutifood products: %w", err)
	}

	result := make([]vo.FoodNutrient, 0, len(gormItems))
	for _, item := range gormItems {
		result = append(result, item.ToNutrientDomain())
	}
	return result, nil
}

func (r *PostgresFoodItemRepository) Save(ctx context.Context, item *aggregate.FoodItem) error {
	gormModel := FromDomainFoodItem(item)
	db := getDB(ctx, r.db)
	if err := db.Create(gormModel).Error; err != nil {
		return fmt.Errorf("postgres food repo save: %w", err)
	}
	return nil
}

func (r *PostgresFoodItemRepository) Update(ctx context.Context, item *aggregate.FoodItem) error {
	gormModel := FromDomainFoodItem(item)
	db := getDB(ctx, r.db)
	if err := db.Save(gormModel).Error; err != nil {
		return fmt.Errorf("postgres food repo update: %w", err)
	}
	return nil
}

type PostgresRecipeCacheRepository struct {
	db *gorm.DB
}

var _ repository.RecipeCacheRepository = (*PostgresRecipeCacheRepository)(nil)

func NewPostgresRecipeCacheRepository(db *gorm.DB) *PostgresRecipeCacheRepository {
	return &PostgresRecipeCacheRepository{db: db}
}

func (r *PostgresRecipeCacheRepository) FindByHash(ctx context.Context, hash string) (*repository.CachedRecipe, error) {
	var gormRecipe GormRecipe
	db := getDB(ctx, r.db)
	if err := db.First(&gormRecipe, "ingredient_hash = ?", hash).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("postgres recipe cache repo find: %w", err)
	}
	return gormRecipe.ToDomain(), nil
}

func (r *PostgresRecipeCacheRepository) Save(ctx context.Context, recipe *repository.CachedRecipe) error {
	stepsJSON, _ := json.Marshal(recipe.CookingSteps)
	ingsJSON, _ := json.Marshal(recipe.Ingredients)

	gormModel := &GormRecipe{
		ID:               recipe.ID,
		IngredientHash:   recipe.IngredientHash,
		RecipeName:       recipe.RecipeName,
		CookingStyle:     recipe.CookingStyle,
		IngredientsJSON:  ingsJSON,
		CookingStepsJSON: stepsJSON,
		CreatedAt:        recipe.CreatedAt,
	}

	db := getDB(ctx, r.db)
	if err := db.Create(gormModel).Error; err != nil {
		return fmt.Errorf("postgres recipe cache repo save: %w", err)
	}
	return nil
}

type PostgresNutritionPlanRepository struct {
	db *gorm.DB
}

var _ repository.NutritionPlanRepository = (*PostgresNutritionPlanRepository)(nil)

func NewPostgresNutritionPlanRepository(db *gorm.DB) *PostgresNutritionPlanRepository {
	return &PostgresNutritionPlanRepository{db: db}
}

func (r *PostgresNutritionPlanRepository) FindByUserIDAndDate(ctx context.Context, userID string, date time.Time) (*aggregate.NutritionPlan, error) {
	var gormPlan GormNutritionPlan
	dateStr := date.Format("2006-01-02")
	db := getDB(ctx, r.db)
	if err := db.First(&gormPlan, "user_id = ? AND DATE(plan_date) = ?", userID, dateStr).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("postgres nutrition plan repo find: %w", err)
	}

	alloc, _ := vo.NewCalorieAllocation(gormPlan.TargetCalories, gormPlan.TargetProtein, gormPlan.TargetCarbs, gormPlan.TargetFat)
	var meals []aggregate.DailyMeal
	if len(gormPlan.MealsJSON) > 0 {
		_ = json.Unmarshal(gormPlan.MealsJSON, &meals)
	}

	return aggregate.NewNutritionPlan(gormPlan.ID, gormPlan.UserID, gormPlan.PlanDate, alloc, meals), nil
}

func (r *PostgresNutritionPlanRepository) Save(ctx context.Context, plan *aggregate.NutritionPlan) error {
	mealsJSON, _ := json.Marshal(plan.DailyMeals())
	alloc := plan.CalorieAllocation()

	gormModel := &GormNutritionPlan{
		ID:             plan.ID(),
		UserID:         plan.UserID(),
		PlanDate:       plan.PlanDate(),
		TargetCalories: alloc.TargetCalories(),
		TargetProtein:  alloc.ProteinGrams(),
		TargetCarbs:    alloc.CarbGrams(),
		TargetFat:      alloc.FatGrams(),
		MealsJSON:      mealsJSON,
		CreatedAt:      plan.CreatedAt(),
		UpdatedAt:      plan.UpdatedAt(),
	}

	db := getDB(ctx, r.db)
	if err := db.Create(gormModel).Error; err != nil {
		return fmt.Errorf("postgres nutrition plan repo save: %w", err)
	}
	return nil
}

func (r *PostgresNutritionPlanRepository) Update(ctx context.Context, plan *aggregate.NutritionPlan) error {
	mealsJSON, _ := json.Marshal(plan.DailyMeals())
	alloc := plan.CalorieAllocation()

	gormModel := &GormNutritionPlan{
		ID:             plan.ID(),
		UserID:         plan.UserID(),
		PlanDate:       plan.PlanDate(),
		TargetCalories: alloc.TargetCalories(),
		TargetProtein:  alloc.ProteinGrams(),
		TargetCarbs:    alloc.CarbGrams(),
		TargetFat:      alloc.FatGrams(),
		MealsJSON:      mealsJSON,
		UpdatedAt:      time.Now(),
	}

	db := getDB(ctx, r.db)
	if err := db.Save(gormModel).Error; err != nil {
		return fmt.Errorf("postgres nutrition plan repo update: %w", err)
	}
	return nil
}

// FindActiveUserIDs trả về danh sách userID đã log bữa ăn trong withinDays ngày gần nhất.
// Dùng bởi DailyMenuCronWorker để xác định user cần sinh thực đơn.
func (r *PostgresNutritionPlanRepository) FindActiveUserIDs(ctx context.Context, withinDays int) ([]string, error) {
	since := time.Now().AddDate(0, 0, -withinDays)
	var userIDs []string
	db := getDB(ctx, r.db)
	if err := db.Model(&GormMealLog{}).
		Where("logged_at >= ?", since).
		Distinct("user_id").
		Pluck("user_id", &userIDs).Error; err != nil {
		return nil, fmt.Errorf("postgres nutrition plan repo find active users: %w", err)
	}
	return userIDs, nil
}

type PostgresMealHistoryRepository struct {
	db *gorm.DB
}

var _ repository.MealHistoryRepository = (*PostgresMealHistoryRepository)(nil)

func NewPostgresMealHistoryRepository(db *gorm.DB) *PostgresMealHistoryRepository {
	return &PostgresMealHistoryRepository{db: db}
}

func (r *PostgresMealHistoryRepository) FindByUserID(ctx context.Context, userID string) (*aggregate.MealHistory, error) {
	var gormHist GormMealHistory
	db := getDB(ctx, r.db)
	if err := db.First(&gormHist, "user_id = ?", userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("postgres meal history repo find: %w", err)
	}

	var gormLockouts []GormLockoutRegistry
	_ = db.Where("user_id = ? AND unlocked_at > ?", userID, time.Now()).Find(&gormLockouts).Error

	lockoutItems := make([]vo.LockoutItem, 0, len(gormLockouts))
	for _, l := range gormLockouts {
		lockoutItems = append(lockoutItems, vo.NewLockoutItem(l.ItemType, l.ItemName, l.UnlockedAt))
	}
	lockoutReg := vo.NewLockoutRegistry(lockoutItems)

	history := aggregate.NewMealHistory(gormHist.ID, gormHist.UserID, lockoutReg)

	var gormLogs []GormMealLog
	_ = db.Where("history_id = ?", gormHist.ID).Find(&gormLogs).Error
	for _, log := range gormLogs {
		history.AddMealLog(aggregate.NewMealLog(
			log.ID, log.HistoryID, log.UserID, log.MealType, log.MealName, log.Portion,
			log.Calories, log.Protein, log.Carbs, log.Fat, log.LoggedAt,
		))
	}

	return history, nil
}

func (r *PostgresMealHistoryRepository) Save(ctx context.Context, history *aggregate.MealHistory) error {
	db := getDB(ctx, r.db)
	return db.Transaction(func(tx *gorm.DB) error {
		gormHist := &GormMealHistory{
			ID:        history.ID(),
			UserID:    history.UserID(),
			UpdatedAt: time.Now(),
		}
		if err := tx.Save(gormHist).Error; err != nil {
			return err
		}

		for _, log := range history.MealLogs() {
			gormLog := &GormMealLog{
				ID:        log.ID(),
				HistoryID: history.ID(),
				UserID:    log.UserID(),
				MealType:  log.MealType(),
				MealName:  log.MealName(),
				Portion:   log.Portion(),
				Calories:  log.Calories(),
				Protein:   log.Protein(),
				Carbs:     log.Carbs(),
				Fat:       log.Fat(),
				LoggedAt:  log.LoggedAt(),
			}
			if err := tx.Save(gormLog).Error; err != nil {
				return err
			}
		}

		for _, item := range history.LockoutRegistry().Items() {
			gormLock := &GormLockoutRegistry{
				ID:         fmt.Sprintf("%s-%s", history.UserID(), item.ItemName()),
				UserID:     history.UserID(),
				ItemType:   item.ItemType(),
				ItemName:   item.ItemName(),
				UnlockedAt: item.UnlockedAt(),
				CreatedAt:  time.Now(),
			}
			if err := tx.Save(gormLock).Error; err != nil {
				return err
			}
		}

		return nil
	})
}
