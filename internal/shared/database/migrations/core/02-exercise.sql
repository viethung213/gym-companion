-- =====================================================================
-- BOUNDED CONTEXT: exercise (Exercise Catalog & Metadata Engine)
-- Initial Schema, Transactional Outbox, Catalog, and Trigram Search
-- =====================================================================

-- pg_trgm must live in public: every module pool searches "<module>,public", so an
-- extension created under the first migrating module's schema (e.g. auth) is invisible
-- to the others and the gin_trgm_ops indexes below fail on a fresh database.
CREATE EXTENSION IF NOT EXISTS pg_trgm WITH SCHEMA public;
DO $$ BEGIN
    IF (SELECT extnamespace::regnamespace::text FROM pg_extension WHERE extname = 'pg_trgm') <> 'public' THEN
        ALTER EXTENSION pg_trgm SET SCHEMA public;
    END IF;
END $$;

CREATE SCHEMA IF NOT EXISTS exercise;

-- 1. TRANSACTIONAL OUTBOX & AUDIT LOG
CREATE TABLE IF NOT EXISTS exercise.outbox (
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

ALTER TABLE exercise.outbox ADD COLUMN IF NOT EXISTS aggregate_type VARCHAR(255);
ALTER TABLE exercise.outbox ADD COLUMN IF NOT EXISTS aggregate_id VARCHAR(255);

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

-- 2. EXERCISES (catalog = free-exercise-db, docs/05 §5.1)
-- Taxonomy columns hold the short codes of the proto enums in
-- contracts/supporting/exercise/v1/message/taxonomy.proto.
CREATE TABLE IF NOT EXISTS exercise.exercises (
    id VARCHAR(255) PRIMARY KEY,
    slug TEXT NOT NULL,
    name TEXT NOT NULL,
    primary_muscle TEXT NOT NULL,
    secondary_muscles TEXT[] NOT NULL DEFAULT '{}',
    equipment TEXT NOT NULL,
    category TEXT NOT NULL,
    level TEXT NOT NULL,
    force TEXT,
    mechanic TEXT,
    instructions TEXT[] NOT NULL DEFAULT '{}',
    images TEXT[] NOT NULL DEFAULT '{}',
    video_url VARCHAR(1024),
    measurement_type VARCHAR(32) NOT NULL DEFAULT 'REPS',
    default_rest_seconds INT NOT NULL DEFAULT 60,
    motion_slug TEXT,
    has_ai_supported BOOLEAN NOT NULL DEFAULT FALSE,
    source TEXT NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'DRAFT',
    archived_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_exercises_slug UNIQUE (slug),
    CONSTRAINT chk_exercises_primary_muscle CHECK (
        primary_muscle IN (
            'ABDOMINALS', 'ABDUCTORS', 'ADDUCTORS', 'BICEPS', 'CALVES', 'CHEST', 'FOREARMS', 'GLUTES',
            'HAMSTRINGS', 'LATS', 'LOWER_BACK', 'MIDDLE_BACK', 'NECK', 'QUADRICEPS', 'SHOULDERS', 'TRAPS',
            'TRICEPS'
        )
    ),
    CONSTRAINT chk_exercises_secondary_muscles CHECK (
        secondary_muscles <@ ARRAY[
            'ABDOMINALS', 'ABDUCTORS', 'ADDUCTORS', 'BICEPS', 'CALVES', 'CHEST', 'FOREARMS', 'GLUTES',
            'HAMSTRINGS', 'LATS', 'LOWER_BACK', 'MIDDLE_BACK', 'NECK', 'QUADRICEPS', 'SHOULDERS', 'TRAPS',
            'TRICEPS'
        ]::TEXT[]
    ),
    CONSTRAINT chk_exercises_equipment CHECK (
        equipment IN (
            'BODY_ONLY', 'BARBELL', 'DUMBBELL', 'KETTLEBELLS', 'CABLE', 'MACHINE', 'BANDS',
            'EZ_CURL_BAR', 'MEDICINE_BALL', 'EXERCISE_BALL', 'FOAM_ROLL', 'OTHER'
        )
    ),
    CONSTRAINT chk_exercises_category CHECK (
        category IN (
            'STRENGTH', 'STRETCHING', 'PLYOMETRICS', 'POWERLIFTING', 'OLYMPIC_WEIGHTLIFTING',
            'STRONGMAN', 'CARDIO'
        )
    ),
    CONSTRAINT chk_exercises_level CHECK (
        level IN ('BEGINNER', 'INTERMEDIATE', 'EXPERT')
    ),
    CONSTRAINT chk_exercises_force CHECK (
        force IN ('PUSH', 'PULL', 'STATIC')
    ),
    CONSTRAINT chk_exercises_mechanic CHECK (
        mechanic IN ('COMPOUND', 'ISOLATION')
    ),
    CONSTRAINT chk_exercises_source CHECK (
        source IN ('free-exercise-db', 'custom')
    ),
    CONSTRAINT chk_exercises_ai_requires_motion_slug CHECK (
        NOT has_ai_supported OR motion_slug IS NOT NULL
    ),
    CONSTRAINT chk_exercises_status CHECK (
        status IN ('DRAFT', 'ACTIVE', 'ARCHIVED')
    ),
    CONSTRAINT chk_exercises_measurement_type CHECK (
        measurement_type IN ('REPS', 'DURATION')
    )
);

-- Lifecycle is DRAFT -> ACTIVE <-> ARCHIVED; databases created before PENDING_APPROVAL
-- was dropped get the narrower constraint too (no row can hold it: no RPC sets it).
DO $$ BEGIN
    IF EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'chk_exercises_status'
               AND pg_get_constraintdef(oid) LIKE '%PENDING_APPROVAL%') THEN
        ALTER TABLE exercise.exercises DROP CONSTRAINT chk_exercises_status;
        ALTER TABLE exercise.exercises ADD CONSTRAINT chk_exercises_status CHECK (status IN ('DRAFT', 'ACTIVE', 'ARCHIVED'));
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_exercises_status_primary_muscle ON exercise.exercises (status, primary_muscle);
CREATE INDEX IF NOT EXISTS idx_exercises_secondary_muscles ON exercise.exercises USING gin (secondary_muscles);
CREATE INDEX IF NOT EXISTS idx_exercises_name_trgm ON exercise.exercises USING gin (name gin_trgm_ops);
CREATE INDEX IF NOT EXISTS idx_exercises_equipment_category ON exercise.exercises (equipment, category);
CREATE INDEX IF NOT EXISTS idx_exercises_motion_slug ON exercise.exercises (motion_slug) WHERE motion_slug IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_exercises_ai_supported_active ON exercise.exercises (id) WHERE status = 'ACTIVE' AND has_ai_supported = true;

-- 3. EXERCISE STATS (One-to-One Performance Tracking)
CREATE TABLE IF NOT EXISTS exercise.exercise_stats (
    exercise_id VARCHAR(255) PRIMARY KEY REFERENCES exercise.exercises(id) ON DELETE CASCADE,
    total_logs INT NOT NULL DEFAULT 0,
    average_rating NUMERIC(3, 2) NOT NULL DEFAULT 0.0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Standing hip hinges load the lower back holding the torso, which free-exercise-db
-- does not list; injury screening needs it (seeder/data/stabilizer_muscles.json).
UPDATE exercise.exercises
SET secondary_muscles = array_append(secondary_muscles, 'LOWER_BACK'), updated_at = now()
WHERE slug IN (
    'Bent_Over_Barbell_Row',
    'Bent_Over_Two-Arm_Long_Bar_Row',
    'Bent_Over_Two-Dumbbell_Row',
    'Bent_Over_Two-Dumbbell_Row_With_Palms_In',
    'Reverse_Grip_Bent-Over_Rows',
    'Smith_Machine_Bent_Over_Row',
    'T-Bar_Row_with_Handle',
    'Standing_Bent-Over_One-Arm_Dumbbell_Triceps_Extension',
    'Standing_Bent-Over_Two-Arm_Dumbbell_Triceps_Extension',
    'Trap_Bar_Deadlift',
    'Leverage_Deadlift',
    'Clean_from_Blocks',
    'Power_Clean_from_Blocks',
    'Vertical_Swing',
    'Double_Kettlebell_Snatch',
    'One-Arm_Kettlebell_Split_Snatch'
) AND primary_muscle <> 'LOWER_BACK' AND NOT ('LOWER_BACK' = ANY(secondary_muscles));
