CREATE TABLE IF NOT EXISTS coaching_daily_workout_plans (
    id VARCHAR(36) PRIMARY KEY,
    schedule_id VARCHAR(36) NOT NULL,
    user_id VARCHAR(36) NOT NULL,
    status VARCHAR(20) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_coaching_daily_workout_plans_schedule_id ON coaching_daily_workout_plans (schedule_id);
CREATE INDEX IF NOT EXISTS idx_coaching_daily_workout_plans_user_id ON coaching_daily_workout_plans (user_id);
CREATE INDEX IF NOT EXISTS idx_coaching_daily_workout_plans_status ON coaching_daily_workout_plans (status);


CREATE TABLE IF NOT EXISTS coaching_planned_exercises (
    id VARCHAR(36) PRIMARY KEY,
    plan_id VARCHAR(36) NOT NULL,
    exercise_id VARCHAR(36) NOT NULL,
    sets INT NOT NULL,
    reps INT NOT NULL,
    weight NUMERIC(10, 2) NOT NULL,
    rpe NUMERIC(4, 2) NOT NULL,
    rest_seconds INT NOT NULL,
    notes TEXT NOT NULL,
    sequence_order INT NOT NULL,
    CONSTRAINT fk_coaching_planned_exercises_plan_id FOREIGN KEY (plan_id) REFERENCES coaching_daily_workout_plans(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_coaching_planned_exercises_plan_id ON coaching_planned_exercises (plan_id);
CREATE INDEX IF NOT EXISTS idx_coaching_planned_exercises_exercise_id ON coaching_planned_exercises (exercise_id);
