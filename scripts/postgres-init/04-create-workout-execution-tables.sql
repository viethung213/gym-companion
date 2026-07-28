-- ==========================================
-- SCHEMA: workout_execution - Tables definitions
-- ==========================================

CREATE SCHEMA IF NOT EXISTS workout_execution;

-- 1. Table: workout_sessions
CREATE TABLE IF NOT EXISTS workout_execution.workout_sessions (
    id VARCHAR(255) PRIMARY KEY,
    user_id VARCHAR(255) NOT NULL,
    plan_id VARCHAR(255) NOT NULL,
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
    CONSTRAINT chk_workout_session_status CHECK (
        status IN ('SCHEDULED', 'IN_PROGRESS', 'COMPLETED', 'ABORTED', 'ANOMALOUS')
    )
);

CREATE INDEX IF NOT EXISTS idx_workout_sessions_user_id ON workout_execution.workout_sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_workout_sessions_status ON workout_execution.workout_sessions(status);
CREATE INDEX IF NOT EXISTS idx_workout_sessions_user_status ON workout_execution.workout_sessions(user_id, status);

-- 2. Table: workout_set_logs
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

-- 3. Table: rep_logs
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

-- 4. Table: session_errors
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

-- 5. Table: personal_records
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

-- 6. Table: motion_specifications
CREATE TABLE IF NOT EXISTS workout_execution.motion_specifications (
    exercise_id VARCHAR(255) PRIMARY KEY,
    onnx_model_url VARCHAR(1024),
    local_rules_url VARCHAR(1024),
    dialogue_engine_json JSONB,
    recommended_camera_angle VARCHAR(50),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
