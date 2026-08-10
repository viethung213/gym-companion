-- ==========================================
-- 11. Add pg_trgm extension and GIN trigram indexes for exercise & motion substring search
-- ==========================================

-- Enable pg_trgm extension for substring/trigram matching in public schema
CREATE SCHEMA IF NOT EXISTS public;
CREATE EXTENSION IF NOT EXISTS pg_trgm SCHEMA public;

-- Fast substring search (ILIKE '%keyword%') for exercises table
CREATE INDEX IF NOT EXISTS idx_exercises_name_trgm 
ON exercise.exercises USING gin (name gin_trgm_ops);

-- Fast substring search (ILIKE '%keyword%') for motion_specifications table
CREATE INDEX IF NOT EXISTS idx_motion_specifications_name_trgm 
ON workout_execution.motion_specifications USING gin (exercise_name gin_trgm_ops);
