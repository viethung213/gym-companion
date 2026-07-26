-- ==========================================
-- FITAI SCHEMA: coaching
-- Domain persistence for WorkoutRoadmap, WeeklySchedule, DailyWorkoutPlan, Outbox
-- ==========================================

CREATE SCHEMA IF NOT EXISTS coaching;

CREATE TABLE IF NOT EXISTS coaching.workout_roadmaps (
    id VARCHAR(255) PRIMARY KEY,
    user_id VARCHAR(255) NOT NULL,
    status INT NOT NULL DEFAULT 1,
    start_date DATE NOT NULL,
    end_date DATE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_roadmaps_user_status ON coaching.workout_roadmaps(user_id, status);

CREATE TABLE IF NOT EXISTS coaching.weekly_schedules (
    id VARCHAR(255) PRIMARY KEY,
    roadmap_id VARCHAR(255) NOT NULL,
    user_id VARCHAR(255) NOT NULL,
    week_number INT NOT NULL,
    start_date DATE NOT NULL,
    end_date DATE NOT NULL,
    muscle_split_type VARCHAR(100) NOT NULL,
    schedule_days JSONB NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_schedules_roadmap_week ON coaching.weekly_schedules(roadmap_id, week_number);

CREATE TABLE IF NOT EXISTS coaching.daily_workout_plans (
    id VARCHAR(255) PRIMARY KEY,
    user_id VARCHAR(255) NOT NULL,
    roadmap_id VARCHAR(255) NOT NULL,
    weekly_schedule_id VARCHAR(255) NOT NULL,
    scheduled_date DATE NOT NULL,
    status INT NOT NULL DEFAULT 1,
    prescription JSONB NOT NULL,
    reasoning_explanation TEXT,
    adjustment_explanation TEXT,
    generated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_daily_plans_user_date ON coaching.daily_workout_plans(user_id, scheduled_date);

CREATE TABLE IF NOT EXISTS coaching.outbox (
    id UUID PRIMARY KEY,
    event_id UUID NOT NULL UNIQUE,
    event_type VARCHAR(255) NOT NULL,
    payload JSONB NOT NULL,
    partition_key VARCHAR(255) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    published BOOLEAN DEFAULT FALSE,
    published_at TIMESTAMP WITH TIME ZONE
);

CREATE TABLE IF NOT EXISTS coaching.outbox_log (
    id UUID PRIMARY KEY,
    event_id UUID NOT NULL,
    event_type VARCHAR(255) NOT NULL,
    payload JSONB NOT NULL,
    partition_key VARCHAR(255) NOT NULL,
    processed_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    status VARCHAR(50) NOT NULL,
    error_message TEXT
);
