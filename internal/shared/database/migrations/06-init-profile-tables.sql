-- ==========================================
-- PROFILE BOUNDED CONTEXT TABLES INIT SCRIPT
-- ==========================================

CREATE SCHEMA IF NOT EXISTS profile;

-- 1. Bảng Thông tin cơ bản người dùng
CREATE TABLE IF NOT EXISTS profile.users (
    user_id UUID PRIMARY KEY,
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

-- 2. Bảng Theo dõi Lịch sử Chỉ số Cơ thể (Body Metrics History)
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

-- 3. Bảng Vết chấn thương người dùng
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
CREATE INDEX IF NOT EXISTS idx_profile_outbox_log_event_status ON profile.outbox_log (event_id, status);
