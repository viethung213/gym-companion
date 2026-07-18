ALTER TABLE IF EXISTS coaching.daily_workout_plans
    ADD COLUMN IF NOT EXISTS warm_up_items JSONB NOT NULL DEFAULT '[]'::jsonb;

ALTER TABLE IF EXISTS coaching.daily_workout_plans
    ADD COLUMN IF NOT EXISTS cool_down_items JSONB NOT NULL DEFAULT '[]'::jsonb;

ALTER TABLE IF EXISTS coaching.daily_workout_plans
    ALTER COLUMN warm_up_items DROP DEFAULT,
    ALTER COLUMN cool_down_items DROP DEFAULT;
