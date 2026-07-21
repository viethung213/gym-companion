-- ==========================================
-- SCHEMA: coaching - Tables definitions
-- Aggregate root: WorkoutRoadmap
--   roadmap → weekly_schedules → schedule_days
--   roadmap → daily_workout_plans → prescribed_exercises
-- Note: schema `coaching` + outbox/outbox_log already created in 01-init-schemas.sql
-- ==========================================

-- 1. Table: workout_roadmaps (Aggregate root)
--    Snapshot toàn bộ ngữ cảnh personalization từ ProfileCompleted event.
--    Enforce 1 ACTIVE roadmap per user (partial unique index).
CREATE TABLE IF NOT EXISTS coaching.workout_roadmaps (
    roadmap_id UUID PRIMARY KEY,
    user_id UUID NOT NULL,
    status VARCHAR(32) NOT NULL,
    primary_goal VARCHAR(32) NOT NULL,
    secondary_goals JSONB NOT NULL DEFAULT '[]',
    planning_tier VARCHAR(16) NOT NULL,
    sessions_per_week SMALLINT NOT NULL,
    muscle_split_type VARCHAR(32) NOT NULL,
    total_weeks SMALLINT NOT NULL DEFAULT 4,
    start_date DATE NOT NULL,
    end_date DATE NOT NULL,
    injuries_snapshot JSONB NOT NULL DEFAULT '[]',
    availability_snapshot JSONB NOT NULL DEFAULT '[]',
    preferred_muscle_groups JSONB NOT NULL DEFAULT '[]',
    available_equipment JSONB NOT NULL DEFAULT '[]',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT chk_roadmap_status CHECK (status IN ('ACTIVE', 'COMPLETED')),
    CONSTRAINT chk_roadmap_planning_tier CHECK (planning_tier IN ('BEGINNER', 'EXPERIENCED')),
    CONSTRAINT chk_roadmap_split CHECK (muscle_split_type IN ('FullBody', 'UpperLower', 'PushPullLegs')),
    CONSTRAINT chk_roadmap_sessions CHECK (sessions_per_week BETWEEN 2 AND 6)
);
CREATE UNIQUE INDEX IF NOT EXISTS ux_coaching_roadmap_active
    ON coaching.workout_roadmaps (user_id) WHERE status = 'ACTIVE';
CREATE INDEX IF NOT EXISTS ix_coaching_roadmap_user ON coaching.workout_roadmaps (user_id);

-- 2. Table: weekly_schedules (1 tuần trong roadmap)
CREATE TABLE IF NOT EXISTS coaching.weekly_schedules (
    weekly_schedule_id UUID PRIMARY KEY,
    roadmap_id UUID NOT NULL REFERENCES coaching.workout_roadmaps(roadmap_id) ON DELETE CASCADE,
    user_id UUID NOT NULL,
    week_number SMALLINT NOT NULL,
    start_date DATE NOT NULL,
    end_date DATE NOT NULL,
    muscle_split_type VARCHAR(32) NOT NULL,
    generated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (roadmap_id, week_number),
    CONSTRAINT chk_weekly_week_number CHECK (week_number BETWEEN 1 AND 52)
);
CREATE INDEX IF NOT EXISTS ix_coaching_weekly_user ON coaching.weekly_schedules (user_id);
CREATE INDEX IF NOT EXISTS ix_coaching_weekly_end_date ON coaching.weekly_schedules (end_date);

-- 3. Table: schedule_days (7 ngày Mon-Sun trong 1 week)
--    daily_workout_plan_id nullable — JIT/pre-cache fill sau (UC-02.2)
CREATE TABLE IF NOT EXISTS coaching.schedule_days (
    id UUID PRIMARY KEY,
    weekly_schedule_id UUID NOT NULL REFERENCES coaching.weekly_schedules(weekly_schedule_id) ON DELETE CASCADE,
    scheduled_date DATE NOT NULL,
    day_of_week VARCHAR(10) NOT NULL,
    status VARCHAR(16) NOT NULL,
    target_muscle_groups JSONB NOT NULL DEFAULT '[]',
    daily_workout_plan_id UUID,
    time_window VARCHAR(64),
    planned_duration_minutes INT,
    UNIQUE (weekly_schedule_id, scheduled_date),
    CONSTRAINT chk_schedule_day_of_week CHECK (
        day_of_week IN ('MONDAY','TUESDAY','WEDNESDAY','THURSDAY','FRIDAY','SATURDAY','SUNDAY')
    ),
    CONSTRAINT chk_schedule_day_status CHECK (
        status IN ('TRAINING','REST','SKIPPED','RESCHEDULED')
    )
);
CREATE INDEX IF NOT EXISTS ix_schedule_days_weekly ON coaching.schedule_days (weekly_schedule_id);
CREATE INDEX IF NOT EXISTS ix_schedule_days_date ON coaching.schedule_days (scheduled_date);

-- 4. Table: daily_workout_plans (giáo án 1 buổi, UC-02.2)
--    Sinh JIT hoặc pre-cache sau CompleteSession
CREATE TABLE IF NOT EXISTS coaching.daily_workout_plans (
    daily_workout_plan_id UUID PRIMARY KEY,
    user_id UUID NOT NULL,
    roadmap_id UUID NOT NULL REFERENCES coaching.workout_roadmaps(roadmap_id) ON DELETE CASCADE,
    weekly_schedule_id UUID NOT NULL REFERENCES coaching.weekly_schedules(weekly_schedule_id) ON DELETE CASCADE,
    scheduled_date DATE NOT NULL,
    status VARCHAR(32) NOT NULL,
    reasoning TEXT,
    adjustment_reason TEXT,
    injuries_snapshot JSONB NOT NULL DEFAULT '[]',
    equipment_snapshot JSONB NOT NULL DEFAULT '[]',
    planning_tier VARCHAR(16) NOT NULL,
    generated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT chk_daily_plan_status CHECK (
        status IN ('DRAFT','ACTIVE','COMPLETED','SKIPPED')
    ),
    CONSTRAINT chk_daily_plan_tier CHECK (planning_tier IN ('BEGINNER','EXPERIENCED'))
);
CREATE INDEX IF NOT EXISTS ix_daily_plan_user_date ON coaching.daily_workout_plans (user_id, scheduled_date);
CREATE INDEX IF NOT EXISTS ix_daily_plan_weekly ON coaching.daily_workout_plans (weekly_schedule_id);
CREATE INDEX IF NOT EXISTS ix_daily_plan_roadmap ON coaching.daily_workout_plans (roadmap_id);

-- FK schedule_days.daily_workout_plan_id → daily_workout_plans (deferred vì daily_workout_plans
-- được định nghĩa SAU schedule_days).
ALTER TABLE coaching.schedule_days
    ADD CONSTRAINT fk_schedule_days_daily_plan
    FOREIGN KEY (daily_workout_plan_id)
    REFERENCES coaching.daily_workout_plans(daily_workout_plan_id)
    ON DELETE SET NULL;

-- 5. Table: prescribed_exercises (bài tập cụ thể trong 1 daily_workout_plan)
--    section: WARMUP | MAIN | COOLDOWN — thứ tự bởi seq (0-based)
CREATE TABLE IF NOT EXISTS coaching.prescribed_exercises (
    id UUID PRIMARY KEY,
    daily_workout_plan_id UUID NOT NULL REFERENCES coaching.daily_workout_plans(daily_workout_plan_id) ON DELETE CASCADE,
    section VARCHAR(16) NOT NULL,
    seq SMALLINT NOT NULL,
    exercise_id VARCHAR(64) NOT NULL,
    exercise_name VARCHAR(255) NOT NULL,
    movement_pattern VARCHAR(64),
    category VARCHAR(32),
    target_sets SMALLINT NOT NULL DEFAULT 0,
    target_reps SMALLINT NOT NULL DEFAULT 0,
    target_weight NUMERIC(6,2),
    duration_seconds INT,
    notes TEXT,
    UNIQUE (daily_workout_plan_id, section, seq),
    CONSTRAINT chk_prescribed_section CHECK (
        section IN ('WARMUP','MAIN','COOLDOWN')
    )
);
CREATE INDEX IF NOT EXISTS ix_prescribed_daily ON coaching.prescribed_exercises (daily_workout_plan_id);
