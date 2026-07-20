-- Harden Coaching invariants and CloudEvents inbox compatibility.

ALTER TABLE coaching.outbox_log
    ALTER COLUMN event_id TYPE VARCHAR(255) USING event_id::text;

CREATE UNIQUE INDEX IF NOT EXISTS uq_coaching_outbox_log_event_id
    ON coaching.outbox_log(event_id);

CREATE UNIQUE INDEX IF NOT EXISTS uq_coaching_active_roadmap_per_user
    ON coaching.workout_roadmaps(user_id)
    WHERE status = 'ACTIVE';
