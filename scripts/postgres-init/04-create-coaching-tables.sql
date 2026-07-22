-- ==========================================
-- COACHING BOUNDED CONTEXT TABLES
-- ==========================================

-- ------------------------------------------
-- 1. Roadmaps Table
-- ------------------------------------------
CREATE TABLE IF NOT EXISTS coaching_roadmaps (
    id VARCHAR(36) PRIMARY KEY,
    user_id VARCHAR(36) NOT NULL,
    status VARCHAR(20) NOT NULL,
    planning_tier VARCHAR(20) NOT NULL,
    start_date TIMESTAMP WITH TIME ZONE NOT NULL,
    end_date TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_coaching_roadmaps_user_id ON coaching_roadmaps (user_id);
CREATE INDEX IF NOT EXISTS idx_coaching_roadmaps_status ON coaching_roadmaps (status);

-- ------------------------------------------
-- 2. Weekly Schedules & Schedule Days Tables
-- ------------------------------------------
CREATE TABLE IF NOT EXISTS coaching_schedules (
    id VARCHAR(36) PRIMARY KEY,
    roadmap_id VARCHAR(36) NOT NULL,
    user_id VARCHAR(36) NOT NULL,
    week_number INT NOT NULL,
    start_date TIMESTAMP WITH TIME ZONE NOT NULL,
    end_date TIMESTAMP WITH TIME ZONE NOT NULL,
    muscle_split_type VARCHAR(50) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_coaching_schedules_roadmap_id ON coaching_schedules (roadmap_id);
CREATE INDEX IF NOT EXISTS idx_coaching_schedules_user_id ON coaching_schedules (user_id);

CREATE TABLE IF NOT EXISTS coaching_schedule_days (
    id VARCHAR(36) PRIMARY KEY,
    schedule_id VARCHAR(36) NOT NULL,
    scheduled_date TIMESTAMP WITH TIME ZONE NOT NULL,
    day_of_week INT NOT NULL,
    status VARCHAR(20) NOT NULL,
    target_muscle_groups JSONB NOT NULL,
    daily_workout_plan_id VARCHAR(36) NOT NULL,
    time_window VARCHAR(50) NOT NULL,
    planned_duration_minutes INT NOT NULL,
    CONSTRAINT fk_coaching_schedule_days_schedule_id FOREIGN KEY (schedule_id) REFERENCES coaching_schedules(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_coaching_schedule_days_schedule_id ON coaching_schedule_days (schedule_id);
CREATE INDEX IF NOT EXISTS idx_coaching_schedule_days_scheduled_date ON coaching_schedule_days (scheduled_date);

-- ------------------------------------------
-- 3. Daily Sessions & Session Items Tables
-- ------------------------------------------
CREATE TABLE IF NOT EXISTS coaching_daily_sessions (
    id VARCHAR(36) PRIMARY KEY,
    schedule_id VARCHAR(36) NOT NULL,
    user_id VARCHAR(36) NOT NULL,
    status VARCHAR(20) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_coaching_daily_sessions_schedule_id ON coaching_daily_sessions (schedule_id);
CREATE INDEX IF NOT EXISTS idx_coaching_daily_sessions_user_id ON coaching_daily_sessions (user_id);
CREATE INDEX IF NOT EXISTS idx_coaching_daily_sessions_status ON coaching_daily_sessions (status);

CREATE TABLE IF NOT EXISTS coaching_session_items (
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
    CONSTRAINT fk_coaching_session_items_plan_id FOREIGN KEY (plan_id) REFERENCES coaching_daily_sessions(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_coaching_session_items_plan_id ON coaching_session_items (plan_id);
CREATE INDEX IF NOT EXISTS idx_coaching_session_items_exercise_id ON coaching_session_items (exercise_id);
