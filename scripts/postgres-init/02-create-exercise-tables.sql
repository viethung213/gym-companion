-- ==========================================
-- SCHEMA: exercise - Tables definitions
-- ==========================================

-- 1. Table: body_parts
CREATE TABLE IF NOT EXISTS exercise.body_parts (
    id VARCHAR(255) PRIMARY KEY,
    name VARCHAR(255) NOT NULL
);

-- 2. Table: equipments
CREATE TABLE IF NOT EXISTS exercise.equipments (
    id VARCHAR(255) PRIMARY KEY,
    name VARCHAR(255) NOT NULL
);

-- 3. Table: muscles
CREATE TABLE IF NOT EXISTS exercise.muscles (
    id VARCHAR(255) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    body_part_id VARCHAR(255) NOT NULL REFERENCES exercise.body_parts(id)
);

-- 4. Table: tags
CREATE TABLE IF NOT EXISTS exercise.tags (
    id VARCHAR(255) PRIMARY KEY,
    name VARCHAR(255) NOT NULL
);

-- 5. Table: exercises
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
    archived_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT chk_exercises_status CHECK (
        status IN ('DRAFT', 'PENDING_APPROVAL', 'ACTIVE', 'ARCHIVED')
    )
);

-- Indexing for Exercises Foreign Keys
CREATE INDEX IF NOT EXISTS idx_exercises_body_part ON exercise.exercises(body_part_id);
CREATE INDEX IF NOT EXISTS idx_exercises_equipment ON exercise.exercises(equipment_id);
CREATE INDEX IF NOT EXISTS idx_exercises_target_muscle ON exercise.exercises(target_muscle_id);
CREATE INDEX IF NOT EXISTS idx_exercises_status ON exercise.exercises(status);

-- 6. Table: exercise_secondary_muscles (Many-to-Many)
CREATE TABLE IF NOT EXISTS exercise.exercise_secondary_muscles (
    exercise_id VARCHAR(255) NOT NULL REFERENCES exercise.exercises(id) ON DELETE CASCADE,
    muscle_id VARCHAR(255) NOT NULL REFERENCES exercise.muscles(id) ON DELETE CASCADE,
    PRIMARY KEY (exercise_id, muscle_id)
);

CREATE INDEX IF NOT EXISTS idx_ex_sec_muscle_muscle ON exercise.exercise_secondary_muscles(muscle_id);

-- 7. Table: exercise_tags (Many-to-Many)
CREATE TABLE IF NOT EXISTS exercise.exercise_tags (
    exercise_id VARCHAR(255) NOT NULL REFERENCES exercise.exercises(id) ON DELETE CASCADE,
    tag_id VARCHAR(255) NOT NULL REFERENCES exercise.tags(id) ON DELETE CASCADE,
    PRIMARY KEY (exercise_id, tag_id)
);

CREATE INDEX IF NOT EXISTS idx_ex_tag_tag ON exercise.exercise_tags(tag_id);

-- 8. Table: motion_specifications
CREATE TABLE IF NOT EXISTS exercise.motion_specifications (
    exercise_id VARCHAR(255) PRIMARY KEY REFERENCES exercise.exercises(id) ON DELETE CASCADE,
    min_rom_percent INT DEFAULT 70,
    calibration_distance_min NUMERIC(5, 2) DEFAULT 1.5,
    calibration_distance_max NUMERIC(5, 2) DEFAULT 2.0,
    calibration_angle NUMERIC(5, 2) DEFAULT 0.0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
