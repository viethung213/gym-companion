BEGIN;

ALTER TABLE exercise.exercises
    ADD COLUMN IF NOT EXISTS archived_at TIMESTAMP WITH TIME ZONE;

ALTER TABLE exercise.exercises
    ALTER COLUMN status SET DEFAULT 'DRAFT';

UPDATE exercise.exercises
SET status = 'DRAFT'
WHERE status IS NULL;

ALTER TABLE exercise.exercises
    ALTER COLUMN status SET NOT NULL;

ALTER TABLE exercise.exercises
    DROP CONSTRAINT IF EXISTS chk_exercises_status;

ALTER TABLE exercise.exercises
    ADD CONSTRAINT chk_exercises_status
    CHECK (status IN ('DRAFT', 'PENDING_APPROVAL', 'ACTIVE', 'ARCHIVED'));

COMMIT;
