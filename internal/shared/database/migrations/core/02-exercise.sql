-- =====================================================================
-- BOUNDED CONTEXT: exercise (Exercise Catalog & Metadata Engine)
-- Initial Schema, Transactional Outbox, Master Tables, and Trigram Search
-- =====================================================================

CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE SCHEMA IF NOT EXISTS exercise;

-- 1. TRANSACTIONAL OUTBOX & AUDIT LOG
CREATE TABLE IF NOT EXISTS exercise.outbox (
    id UUID PRIMARY KEY,
    event_id UUID NOT NULL UNIQUE,
    event_type VARCHAR(255) NOT NULL,
    payload JSONB NOT NULL,
    partition_key VARCHAR(255) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    published BOOLEAN DEFAULT FALSE,
    published_at TIMESTAMP WITH TIME ZONE,
    status VARCHAR(50) DEFAULT 'PENDING' NOT NULL,
    locked_until TIMESTAMP WITH TIME ZONE
);

CREATE TABLE IF NOT EXISTS exercise.outbox_log (
    id UUID PRIMARY KEY,
    event_id UUID NOT NULL,
    event_type VARCHAR(255) NOT NULL,
    payload JSONB NOT NULL,
    partition_key VARCHAR(255) NOT NULL,
    processed_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    status VARCHAR(50) NOT NULL,
    error_message TEXT
);

CREATE INDEX IF NOT EXISTS idx_exercise_outbox_log_event_status ON exercise.outbox_log (event_id, status);

-- 2. MASTER LOOKUP TABLES
CREATE TABLE IF NOT EXISTS exercise.body_parts (
    id VARCHAR(255) PRIMARY KEY,
    name VARCHAR(255) NOT NULL
);

CREATE TABLE IF NOT EXISTS exercise.equipments (
    id VARCHAR(255) PRIMARY KEY,
    name VARCHAR(255) NOT NULL
);

CREATE TABLE IF NOT EXISTS exercise.muscles (
    id VARCHAR(255) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    body_part_id VARCHAR(255) NOT NULL REFERENCES exercise.body_parts(id)
);

CREATE TABLE IF NOT EXISTS exercise.tags (
    id VARCHAR(255) PRIMARY KEY,
    name VARCHAR(255) NOT NULL
);

-- 3. EXERCISES
CREATE TABLE IF NOT EXISTS exercise.exercises (
    id VARCHAR(255) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    body_part_id VARCHAR(255) NOT NULL REFERENCES exercise.body_parts(id),
    equipment_id VARCHAR(255) NOT NULL REFERENCES exercise.equipments(id),
    target_muscle_id VARCHAR(255) NOT NULL REFERENCES exercise.muscles(id),
    instructions TEXT,
    thumbnail_url VARCHAR(1024),
    media_url VARCHAR(1024),
    video_url VARCHAR(1024),
    difficulty VARCHAR(50) DEFAULT 'Beginner',
    default_rest_seconds INT DEFAULT 60,
    status VARCHAR(50) DEFAULT 'DRAFT',
    has_ai_supported BOOLEAN NOT NULL DEFAULT FALSE,
    measurement_type VARCHAR(32) NOT NULL DEFAULT 'REPS',
    archived_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT chk_exercises_status CHECK (
        status IN ('DRAFT', 'PENDING_APPROVAL', 'ACTIVE', 'ARCHIVED')
    ),
    CONSTRAINT chk_exercises_measurement_type CHECK (
        measurement_type IN ('REPS', 'DURATION')
    )
);

CREATE INDEX IF NOT EXISTS idx_exercises_body_part ON exercise.exercises(body_part_id);
CREATE INDEX IF NOT EXISTS idx_exercises_equipment ON exercise.exercises(equipment_id);
CREATE INDEX IF NOT EXISTS idx_exercises_target_muscle ON exercise.exercises(target_muscle_id);
CREATE INDEX IF NOT EXISTS idx_exercises_status ON exercise.exercises(status);
CREATE INDEX IF NOT EXISTS idx_exercises_measurement_type ON exercise.exercises(measurement_type);
CREATE INDEX IF NOT EXISTS idx_exercises_name_trgm ON exercise.exercises USING gin (name gin_trgm_ops);

-- 4. EXERCISE SECONDARY MUSCLES (Many-to-Many)
CREATE TABLE IF NOT EXISTS exercise.exercise_secondary_muscles (
    exercise_id VARCHAR(255) NOT NULL REFERENCES exercise.exercises(id) ON DELETE CASCADE,
    muscle_id VARCHAR(255) NOT NULL REFERENCES exercise.muscles(id) ON DELETE CASCADE,
    PRIMARY KEY (exercise_id, muscle_id)
);

CREATE INDEX IF NOT EXISTS idx_ex_sec_muscle_muscle ON exercise.exercise_secondary_muscles(muscle_id);

-- 5. EXERCISE TAGS (Many-to-Many)
CREATE TABLE IF NOT EXISTS exercise.exercise_tags (
    exercise_id VARCHAR(255) NOT NULL REFERENCES exercise.exercises(id) ON DELETE CASCADE,
    tag_id VARCHAR(255) NOT NULL REFERENCES exercise.tags(id) ON DELETE CASCADE,
    PRIMARY KEY (exercise_id, tag_id)
);

CREATE INDEX IF NOT EXISTS idx_ex_tag_tag ON exercise.exercise_tags(tag_id);

-- 6. EXERCISE STATS (One-to-One Performance Tracking)
CREATE TABLE IF NOT EXISTS exercise.exercise_stats (
    exercise_id VARCHAR(255) PRIMARY KEY REFERENCES exercise.exercises(id) ON DELETE CASCADE,
    total_logs INT NOT NULL DEFAULT 0,
    average_rating NUMERIC(3, 2) NOT NULL DEFAULT 0.0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

