-- =====================================================================
-- BOUNDED CONTEXT: workout_execution (Workout Sessions, Sets & Real-Time Tracking)
-- Initial Schema, Transactional Outbox, Tables, and Motion Specifications
-- =====================================================================

CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE SCHEMA IF NOT EXISTS workout_execution;

-- 1. TRANSACTIONAL OUTBOX & AUDIT LOG
CREATE TABLE IF NOT EXISTS workout_execution.outbox (
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

CREATE TABLE IF NOT EXISTS workout_execution.outbox_log (
    id UUID PRIMARY KEY,
    event_id UUID NOT NULL,
    event_type VARCHAR(255) NOT NULL,
    payload JSONB NOT NULL,
    partition_key VARCHAR(255) NOT NULL,
    processed_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    status VARCHAR(50) NOT NULL,
    error_message TEXT
);

CREATE INDEX IF NOT EXISTS idx_workout_execution_outbox_log_event_status ON workout_execution.outbox_log (event_id, status);

-- 2. WORKOUT SESSIONS
CREATE TABLE IF NOT EXISTS workout_execution.workout_sessions (
    id VARCHAR(255) PRIMARY KEY,
    user_id VARCHAR(255) NOT NULL,
    plan_id VARCHAR(255) NOT NULL DEFAULT '',
    status VARCHAR(50) NOT NULL DEFAULT 'SCHEDULED',
    total_sets INT DEFAULT 0,
    total_volume NUMERIC(10, 2) DEFAULT 0.0,
    average_form_score NUMERIC(5, 2),
    average_rpe NUMERIC(5, 2),
    scheduled_at TIMESTAMP WITH TIME ZONE,
    started_at TIMESTAMP WITH TIME ZONE,
    ended_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    version INT NOT NULL DEFAULT 1,
    is_shared BOOLEAN NOT NULL DEFAULT FALSE,
    CONSTRAINT chk_workout_session_status CHECK (
        status IN ('SCHEDULED', 'IN_PROGRESS', 'COMPLETED', 'ABORTED', 'ANOMALOUS')
    )
);

CREATE INDEX IF NOT EXISTS idx_workout_sessions_user_id ON workout_execution.workout_sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_workout_sessions_status ON workout_execution.workout_sessions(status);
CREATE INDEX IF NOT EXISTS idx_workout_sessions_user_status ON workout_execution.workout_sessions(user_id, status);
CREATE UNIQUE INDEX IF NOT EXISTS uq_workout_sessions_active_user ON workout_execution.workout_sessions(user_id) WHERE status = 'IN_PROGRESS';

-- 3. WORKOUT SET LOGS
CREATE TABLE IF NOT EXISTS workout_execution.workout_set_logs (
    id VARCHAR(255) PRIMARY KEY,
    session_id VARCHAR(255) NOT NULL REFERENCES workout_execution.workout_sessions(id) ON DELETE CASCADE,
    set_number INT NOT NULL,
    exercise_id VARCHAR(255) NOT NULL,
    target_reps INT NOT NULL DEFAULT 0,
    actual_reps INT NOT NULL DEFAULT 0,
    weight NUMERIC(10, 2) NOT NULL DEFAULT 0.0,
    form_score NUMERIC(5, 2),
    rpe NUMERIC(5, 2) DEFAULT 0.0,
    camera_angle VARCHAR(50),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_workout_set_logs_session_id ON workout_execution.workout_set_logs(session_id);
CREATE INDEX IF NOT EXISTS idx_workout_set_logs_exercise_id ON workout_execution.workout_set_logs(exercise_id);

-- 4. REP LOGS
CREATE TABLE IF NOT EXISTS workout_execution.rep_logs (
    id VARCHAR(255) PRIMARY KEY,
    set_log_id VARCHAR(255) NOT NULL REFERENCES workout_execution.workout_set_logs(id) ON DELETE CASCADE,
    rep_number INT NOT NULL,
    rom_percentage NUMERIC(5, 2) NOT NULL,
    error_codes JSONB,
    joint_angles JSONB,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_rep_logs_set_log_id ON workout_execution.rep_logs(set_log_id);

-- 5. SESSION ERRORS
CREATE TABLE IF NOT EXISTS workout_execution.session_errors (
    id VARCHAR(255) PRIMARY KEY,
    session_id VARCHAR(255) NOT NULL REFERENCES workout_execution.workout_sessions(id) ON DELETE CASCADE,
    set_number INT NOT NULL,
    rep_number INT NOT NULL,
    exercise_id VARCHAR(255) NOT NULL,
    error_code VARCHAR(100) NOT NULL,
    severity VARCHAR(50) NOT NULL,
    timestamp TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_session_errors_session_id ON workout_execution.session_errors(session_id);

-- 6. PERSONAL RECORDS
CREATE TABLE IF NOT EXISTS workout_execution.personal_records (
    id VARCHAR(255) PRIMARY KEY,
    user_id VARCHAR(255) NOT NULL,
    exercise_id VARCHAR(255) NOT NULL,
    one_rep_max NUMERIC(10, 2) NOT NULL,
    weight NUMERIC(10, 2) NOT NULL,
    reps INT NOT NULL,
    form_verified BOOLEAN DEFAULT TRUE,
    achieved_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_user_exercise_pr UNIQUE (user_id, exercise_id)
);

CREATE INDEX IF NOT EXISTS idx_personal_records_user_id ON workout_execution.personal_records(user_id);
CREATE INDEX IF NOT EXISTS idx_personal_records_user_exercise ON workout_execution.personal_records(user_id, exercise_id);

-- 7. MOTION SPECIFICATIONS
CREATE TABLE IF NOT EXISTS workout_execution.motion_specifications (
    exercise_id VARCHAR(255) PRIMARY KEY,
    exercise_name VARCHAR(255) DEFAULT '',
    onnx_detector_url VARCHAR(1024),
    onnx_skeleton_url VARCHAR(1024),
    local_rules_url VARCHAR(1024),
    dialogue_engine_url VARCHAR(1024),
    recommended_camera_angle VARCHAR(50),
    is_ready BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_motion_specifications_name_trgm ON workout_execution.motion_specifications USING gin (exercise_name gin_trgm_ops);

