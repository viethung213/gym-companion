-- =====================================================================
-- BOUNDED CONTEXT: nutrition (Nutrition Planning, Food Catalog & Meal Logs)
-- Initial Schema, Transactional Outbox, Tables, and Lookup Indexes
-- =====================================================================

CREATE SCHEMA IF NOT EXISTS nutrition;

-- 1. TRANSACTIONAL OUTBOX & AUDIT LOG
CREATE TABLE IF NOT EXISTS nutrition.outbox (
    id UUID PRIMARY KEY,
    event_id UUID NOT NULL UNIQUE,
    aggregate_type VARCHAR(255),
    aggregate_id VARCHAR(255),
    event_type VARCHAR(255) NOT NULL,
    payload JSONB NOT NULL,
    partition_key VARCHAR(255) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    published BOOLEAN DEFAULT FALSE,
    published_at TIMESTAMP WITH TIME ZONE,
    status VARCHAR(50) DEFAULT 'PENDING' NOT NULL,
    locked_until TIMESTAMP WITH TIME ZONE
);

ALTER TABLE nutrition.outbox ADD COLUMN IF NOT EXISTS aggregate_type VARCHAR(255);
ALTER TABLE nutrition.outbox ADD COLUMN IF NOT EXISTS aggregate_id VARCHAR(255);

CREATE TABLE IF NOT EXISTS nutrition.outbox_log (
    id UUID PRIMARY KEY,
    event_id UUID NOT NULL,
    event_type VARCHAR(255) NOT NULL,
    payload JSONB NOT NULL,
    partition_key VARCHAR(255) NOT NULL,
    processed_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    status VARCHAR(50) NOT NULL,
    error_message TEXT
);

CREATE INDEX IF NOT EXISTS idx_nutrition_outbox_log_event_status ON nutrition.outbox_log (event_id, status);

-- 2. FOOD ITEMS (Catalog & Partner Products per 100g)
CREATE TABLE IF NOT EXISTS nutrition.food_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    category VARCHAR(100) NOT NULL,
    calories_per_100g NUMERIC(6, 2) NOT NULL,
    protein_per_100g NUMERIC(6, 2) NOT NULL,
    carbs_per_100g NUMERIC(6, 2) NOT NULL,
    fat_per_100g NUMERIC(6, 2) NOT NULL,
    allergen_tags JSONB DEFAULT '[]'::jsonb,
    protein_source VARCHAR(100),
    carb_source VARCHAR(100),
    is_nutifood_product BOOLEAN DEFAULT FALSE,
    status VARCHAR(50) NOT NULL DEFAULT 'Active',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_food_items_category ON nutrition.food_items(category);
CREATE INDEX IF NOT EXISTS idx_food_items_status ON nutrition.food_items(status);

-- 3. RECIPES (Recipe Cache from AI)
CREATE TABLE IF NOT EXISTS nutrition.recipes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ingredient_hash VARCHAR(255) NOT NULL UNIQUE,
    recipe_name VARCHAR(255) NOT NULL,
    cooking_style VARCHAR(100) NOT NULL,
    ingredients_json JSONB NOT NULL,
    cooking_steps JSONB NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_recipes_hash ON nutrition.recipes(ingredient_hash);

-- 4. NUTRITION PLANS (Daily Menu Plans)
CREATE TABLE IF NOT EXISTS nutrition.nutrition_plans (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    plan_date DATE NOT NULL,
    target_calories NUMERIC(6, 2) NOT NULL,
    target_protein NUMERIC(6, 2) NOT NULL,
    target_carbs NUMERIC(6, 2) NOT NULL,
    target_fat NUMERIC(6, 2) NOT NULL,
    meals_json JSONB NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT unique_user_plan_date UNIQUE(user_id, plan_date)
);

CREATE INDEX IF NOT EXISTS idx_nutrition_plans_user_date ON nutrition.nutrition_plans(user_id, plan_date);

-- 5. MEAL HISTORIES & LOGS
CREATE TABLE IF NOT EXISTS nutrition.meal_histories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL UNIQUE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS nutrition.meal_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    history_id UUID NOT NULL REFERENCES nutrition.meal_histories(id) ON DELETE CASCADE,
    user_id UUID NOT NULL,
    meal_type VARCHAR(50) NOT NULL,
    meal_name VARCHAR(255) NOT NULL,
    portion VARCHAR(100),
    calories NUMERIC(6, 2) NOT NULL,
    protein NUMERIC(6, 2) NOT NULL,
    carbs NUMERIC(6, 2) NOT NULL,
    fat NUMERIC(6, 2) NOT NULL,
    logged_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_meal_logs_user_logged ON nutrition.meal_logs(user_id, logged_at);

-- 6. LOCKOUT REGISTRIES
CREATE TABLE IF NOT EXISTS nutrition.lockout_registries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    item_type VARCHAR(50) NOT NULL,
    item_name VARCHAR(255) NOT NULL,
    unlocked_at TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_lockout_user ON nutrition.lockout_registries(user_id, unlocked_at);

-- 7. USER MEAL SCHEDULES
CREATE TABLE IF NOT EXISTS nutrition.user_meal_schedules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    meal_type VARCHAR(50) NOT NULL,
    scheduled_time VARCHAR(10) NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT unique_user_meal_type UNIQUE(user_id, meal_type)
);

CREATE INDEX IF NOT EXISTS idx_user_meal_schedules_user ON nutrition.user_meal_schedules(user_id);

