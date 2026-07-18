CREATE SCHEMA IF NOT EXISTS coaching;

CREATE TABLE IF NOT EXISTS coaching.workout_roadmaps (
    id VARCHAR(255) PRIMARY KEY,
    user_id VARCHAR(255) NOT NULL,
    status VARCHAR(32) NOT NULL,
    start_date DATE NOT NULL,
    end_date DATE NOT NULL,
    planning_input JSONB NOT NULL,
    planner_version VARCHAR(64) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT coaching_roadmap_status_check
        CHECK (status IN ('active', 'paused', 'completed', 'cancelled'))
);

CREATE UNIQUE INDEX IF NOT EXISTS coaching_one_active_roadmap_per_user
    ON coaching.workout_roadmaps (user_id)
    WHERE status = 'active';

CREATE INDEX IF NOT EXISTS coaching_roadmaps_user_status
    ON coaching.workout_roadmaps (user_id, status);

CREATE TABLE IF NOT EXISTS coaching.weekly_schedules (
    id VARCHAR(255) PRIMARY KEY,
    roadmap_id VARCHAR(255) NOT NULL
        REFERENCES coaching.workout_roadmaps(id) ON DELETE CASCADE,
    user_id VARCHAR(255) NOT NULL,
    week_number INTEGER NOT NULL CHECK (week_number BETWEEN 1 AND 4),
    start_date DATE NOT NULL,
    end_date DATE NOT NULL,
    schedule_days JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (roadmap_id, week_number)
);

CREATE INDEX IF NOT EXISTS coaching_schedules_user_roadmap
    ON coaching.weekly_schedules (user_id, roadmap_id, week_number);

CREATE TABLE IF NOT EXISTS coaching.outbox_events (
    id VARCHAR(255) PRIMARY KEY,
    event_type VARCHAR(255) NOT NULL,
    source VARCHAR(255) NOT NULL,
    subject VARCHAR(255) NOT NULL,
    partition_key VARCHAR(255) NOT NULL,
    event_time TIMESTAMPTZ NOT NULL,
    data JSONB NOT NULL,
    published BOOLEAN NOT NULL DEFAULT FALSE,
    published_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS coaching_outbox_unpublished
    ON coaching.outbox_events (event_time)
    WHERE published = FALSE;
