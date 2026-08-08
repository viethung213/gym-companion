package persistence

import (
	"time"
)

type GormFoodItem struct {
	ID                string    `gorm:"column:id;primaryKey"`
	Name              string    `gorm:"column:name"`
	Category          string    `gorm:"column:category"`
	CaloriesPer100g   float64   `gorm:"column:calories_per_100g"`
	ProteinPer100g    float64   `gorm:"column:protein_per_100g"`
	CarbsPer100g      float64   `gorm:"column:carbs_per_100g"`
	FatPer100g        float64   `gorm:"column:fat_per_100g"`
	AllergenTagsJSON  []byte    `gorm:"column:allergen_tags;type:jsonb"`
	ProteinSource     string    `gorm:"column:protein_source"`
	CarbSource        string    `gorm:"column:carb_source"`
	IsNutiFoodProduct bool      `gorm:"column:is_nutifood_product"`
	Status            string    `gorm:"column:status"`
	CreatedAt         time.Time `gorm:"column:created_at"`
	UpdatedAt         time.Time `gorm:"column:updated_at"`
}

func (g *GormFoodItem) TableName() string {
	return "nutrition.food_items"
}

type GormRecipe struct {
	ID               string    `gorm:"column:id;primaryKey"`
	IngredientHash   string    `gorm:"column:ingredient_hash;uniqueIndex"`
	RecipeName       string    `gorm:"column:recipe_name"`
	CookingStyle     string    `gorm:"column:cooking_style"`
	IngredientsJSON  []byte    `gorm:"column:ingredients_json;type:jsonb"`
	CookingStepsJSON []byte    `gorm:"column:cooking_steps;type:jsonb"`
	CreatedAt        time.Time `gorm:"column:created_at"`
}

func (g *GormRecipe) TableName() string {
	return "nutrition.recipes"
}

type GormNutritionPlan struct {
	ID             string    `gorm:"column:id;primaryKey"`
	UserID         string    `gorm:"column:user_id;uniqueIndex:unique_user_plan_date"`
	PlanDate       time.Time `gorm:"column:plan_date;type:date;uniqueIndex:unique_user_plan_date"`
	TargetCalories float64   `gorm:"column:target_calories"`
	TargetProtein  float64   `gorm:"column:target_protein"`
	TargetCarbs    float64   `gorm:"column:target_carbs"`
	TargetFat      float64   `gorm:"column:target_fat"`
	MealsJSON      []byte    `gorm:"column:meals_json;type:jsonb"`
	CreatedAt      time.Time `gorm:"column:created_at"`
	UpdatedAt      time.Time `gorm:"column:updated_at"`
}

func (g *GormNutritionPlan) TableName() string {
	return "nutrition.nutrition_plans"
}

type GormMealHistory struct {
	ID        string    `gorm:"column:id;primaryKey"`
	UserID    string    `gorm:"column:user_id;uniqueIndex"`
	CreatedAt time.Time `gorm:"column:created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

func (g *GormMealHistory) TableName() string {
	return "nutrition.meal_histories"
}

type GormMealLog struct {
	ID        string    `gorm:"column:id;primaryKey"`
	HistoryID string    `gorm:"column:history_id"`
	UserID    string    `gorm:"column:user_id"`
	MealType  string    `gorm:"column:meal_type"`
	MealName  string    `gorm:"column:meal_name"`
	Portion   string    `gorm:"column:portion"`
	Calories  float64   `gorm:"column:calories"`
	Protein   float64   `gorm:"column:protein"`
	Carbs     float64   `gorm:"column:carbs"`
	Fat       float64   `gorm:"column:fat"`
	LoggedAt  time.Time `gorm:"column:logged_at"`
}

func (g *GormMealLog) TableName() string {
	return "nutrition.meal_logs"
}

type GormLockoutRegistry struct {
	ID         string    `gorm:"column:id;primaryKey"`
	UserID     string    `gorm:"column:user_id"`
	ItemType   string    `gorm:"column:item_type"`
	ItemName   string    `gorm:"column:item_name"`
	UnlockedAt time.Time `gorm:"column:unlocked_at"`
	CreatedAt  time.Time `gorm:"column:created_at"`
}

func (g *GormLockoutRegistry) TableName() string {
	return "nutrition.lockout_registries"
}

type GormOutbox struct {
	ID           string     `gorm:"column:id;primaryKey"`
	EventID      string     `gorm:"column:event_id;uniqueIndex"`
	EventType    string     `gorm:"column:event_type"`
	Payload      []byte     `gorm:"column:payload;type:jsonb"`
	PartitionKey string     `gorm:"column:partition_key"`
	CreatedAt    time.Time  `gorm:"column:created_at"`
	Published    bool       `gorm:"column:published"`
	PublishedAt  *time.Time `gorm:"column:published_at"`
	LockedUntil  *time.Time `gorm:"column:locked_until"`
	Status       string     `gorm:"column:status"`
}

func (g *GormOutbox) TableName() string {
	return "nutrition.outbox"
}

type GormOutboxLog struct {
	ID           string    `gorm:"column:id;primaryKey"`
	EventID      string    `gorm:"column:event_id;index"`
	EventType    string    `gorm:"column:event_type"`
	Payload      []byte    `gorm:"column:payload;type:jsonb"`
	PartitionKey string    `gorm:"column:partition_key"`
	ProcessedAt  time.Time `gorm:"column:processed_at"`
	Status       string    `gorm:"column:status"`
	ErrorMessage string    `gorm:"column:error_message"`
}

func (g *GormOutboxLog) TableName() string {
	return "nutrition.outbox_log"
}

type GormUserMealSchedule struct {
	ID            string    `gorm:"column:id;primaryKey"`
	UserID        string    `gorm:"column:user_id;index"`
	MealType      string    `gorm:"column:meal_type"`
	ScheduledTime string    `gorm:"column:scheduled_time"`
	UpdatedAt     time.Time `gorm:"column:updated_at"`
}

func (g *GormUserMealSchedule) TableName() string {
	return "nutrition.user_meal_schedules"
}
