-- ==========================================
-- SCHEMA: coaching - Tables definitions
-- Issue #80: Coaching Event Listener
-- ==========================================

-- 1. Aggregate Root: WorkoutRoadmap
CREATE TABLE IF NOT EXISTS coaching.workout_roadmaps (
    id VARCHAR(255) PRIMARY KEY,
    user_id VARCHAR(255) NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'ACTIVE',
    start_date DATE NOT NULL,
    end_date DATE NOT NULL,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT chk_roadmaps_status CHECK (status IN ('ACTIVE', 'COMPLETED', 'PAUSED'))
);
CREATE INDEX IF NOT EXISTS idx_roadmaps_user_status ON coaching.workout_roadmaps(user_id, status);

-- 2. Aggregate Root: WeeklySchedule
CREATE TABLE IF NOT EXISTS coaching.weekly_schedules (
    id VARCHAR(255) PRIMARY KEY,
    roadmap_id VARCHAR(255) NOT NULL,
    user_id VARCHAR(255) NOT NULL,
    week_number INT NOT NULL,
    start_date DATE NOT NULL,
    end_date DATE NOT NULL,
    muscle_split_type VARCHAR(100) NOT NULL,
    schedule_days JSONB NOT NULL,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_weekly_schedules_roadmap ON coaching.weekly_schedules(roadmap_id);
CREATE INDEX IF NOT EXISTS idx_weekly_schedules_user ON coaching.weekly_schedules(user_id);

-- 3. Aggregate Root: DailyWorkoutPlan
CREATE TABLE IF NOT EXISTS coaching.daily_workout_plans (
    id VARCHAR(255) PRIMARY KEY,
    weekly_schedule_id VARCHAR(255) NOT NULL,
    roadmap_id VARCHAR(255) NOT NULL,
    user_id VARCHAR(255) NOT NULL,
    scheduled_date DATE NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'DRAFT',
    workout_prescription JSONB NOT NULL,
    reasoning_explanation TEXT,
    adjustment_explanation TEXT,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT chk_daily_plans_status CHECK (status IN ('DRAFT', 'ACTIVE', 'COMPLETED', 'SKIPPED'))
);
CREATE INDEX IF NOT EXISTS idx_daily_plans_user_date ON coaching.daily_workout_plans(user_id, scheduled_date);
CREATE INDEX IF NOT EXISTS idx_daily_plans_schedule ON coaching.daily_workout_plans(weekly_schedule_id);
