-- =====================================================================
-- BOUNDED CONTEXT: profile (User Profiles, Athlete Metrics & Injuries)
-- Initial Schema, Transactional Outbox, and Tables
-- =====================================================================

CREATE SCHEMA IF NOT EXISTS profile;

-- 1. TRANSACTIONAL OUTBOX & AUDIT LOG
CREATE TABLE IF NOT EXISTS profile.outbox (
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

ALTER TABLE profile.outbox ADD COLUMN IF NOT EXISTS aggregate_type VARCHAR(255);
ALTER TABLE profile.outbox ADD COLUMN IF NOT EXISTS aggregate_id VARCHAR(255);

CREATE TABLE IF NOT EXISTS profile.outbox_log (
    id UUID PRIMARY KEY,
    event_id UUID NOT NULL,
    event_type VARCHAR(255) NOT NULL,
    payload JSONB NOT NULL,
    partition_key VARCHAR(255) NOT NULL,
    processed_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    status VARCHAR(50) NOT NULL,
    error_message TEXT
);

CREATE INDEX IF NOT EXISTS idx_profile_outbox_log_event_status ON profile.outbox_log (event_id, status);

-- 2. USERS (Hồ sơ thông tin cơ bản)
CREATE TABLE IF NOT EXISTS profile.users (
    user_id UUID PRIMARY KEY,
    full_name VARCHAR(255) DEFAULT '',
    avatar_url TEXT DEFAULT 'https://lqhhwimtzrcrmyfhlizv.supabase.co/storage/v1/object/public/gym-companion-assets/avatar_default/male/avatar-vo-tri-loopy-cute.jpg',
    date_of_birth DATE,
    gender VARCHAR(20),
    experience_level VARCHAR(50) DEFAULT 'BEGINNER',
    goals JSONB NOT NULL DEFAULT '[]',
    preferred_workout_times JSONB NOT NULL DEFAULT '[]',
    available_equipment JSONB NOT NULL DEFAULT '[]',
    preferred_muscle_groups JSONB NOT NULL DEFAULT '[]',
    coach_style VARCHAR(50) DEFAULT 'FRIENDLY',
    target_weight_kg NUMERIC(5,2) DEFAULT 0,
    target_body_fat_percent NUMERIC(5,2) DEFAULT 0,
    completion_rate NUMERIC(5,2) DEFAULT 0.0,
    ai_coach_activated BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- 3. BODY METRICS (Lịch sử chỉ số cơ thể)
CREATE TABLE IF NOT EXISTS profile.body_metrics (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES profile.users(user_id) ON DELETE CASCADE,
    weight_kg NUMERIC(5,2) NOT NULL,
    height_cm NUMERIC(5,2) NOT NULL DEFAULT 0,
    body_fat_percent NUMERIC(5,2) DEFAULT 0,
    progress_photo_url TEXT DEFAULT '',
    logged_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_profile_body_metrics_user_id ON profile.body_metrics(user_id, logged_at DESC);

-- 4. INJURIES (Vết chấn thương)
CREATE TABLE IF NOT EXISTS profile.injuries (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES profile.users(user_id) ON DELETE CASCADE,
    muscle_group VARCHAR(100) NOT NULL,
    severity VARCHAR(50) NOT NULL DEFAULT 'MILD',
    notes TEXT DEFAULT '',
    reported_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    is_recovered BOOLEAN DEFAULT FALSE,
    recovered_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_profile_injuries_user_id ON profile.injuries(user_id);

