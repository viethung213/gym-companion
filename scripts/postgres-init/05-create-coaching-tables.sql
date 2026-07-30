-- ==========================================
-- SCHEMA: coaching - Tables definitions
-- 4-tier DDD: Roadmap -> WeekPlan -> DayPlan -> SessionPlan
-- ==========================================

CREATE SCHEMA IF NOT EXISTS coaching;

-- 1. Table: roadmaps (Aggregate Root, 4-week program)
CREATE TABLE IF NOT EXISTS coaching.roadmaps (
    roadmap_id  VARCHAR(255) PRIMARY KEY,
    user_id     VARCHAR(255) NOT NULL,
    status      VARCHAR(50)  NOT NULL DEFAULT 'ACTIVE',
    start_date  DATE         NOT NULL,
    end_date    DATE         NOT NULL,
    created_at  TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT chk_roadmap_status CHECK (status IN ('ACTIVE', 'COMPLETED'))
);

CREATE INDEX IF NOT EXISTS idx_roadmaps_user_id ON coaching.roadmaps(user_id);

-- Only one ACTIVE roadmap per user (BR-AC-01 invariant support)
CREATE UNIQUE INDEX IF NOT EXISTS ux_roadmaps_user_active
    ON coaching.roadmaps(user_id) WHERE status = 'ACTIVE';

-- 2. Table: week_plans (4 per roadmap, one per phase)
CREATE TABLE IF NOT EXISTS coaching.week_plans (
    week_plan_id       VARCHAR(255) PRIMARY KEY,
    roadmap_id         VARCHAR(255) NOT NULL REFERENCES coaching.roadmaps(roadmap_id) ON DELETE CASCADE,
    user_id            VARCHAR(255) NOT NULL,
    week_number        SMALLINT     NOT NULL,
    phase              VARCHAR(50)  NOT NULL,
    target_rpe         NUMERIC(3,1) NOT NULL,
    start_date         DATE         NOT NULL,
    end_date           DATE         NOT NULL,
    muscle_split_type  VARCHAR(100) NOT NULL DEFAULT '',
    created_at         TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT chk_week_number CHECK (week_number BETWEEN 1 AND 4),
    CONSTRAINT chk_week_phase  CHECK (phase IN ('ACCUMULATION', 'OVERLOAD', 'PEAK', 'DELOAD')),
    CONSTRAINT uq_week_roadmap_number UNIQUE (roadmap_id, week_number)
);

CREATE INDEX IF NOT EXISTS idx_week_plans_roadmap_id ON coaching.week_plans(roadmap_id);

-- 3. Table: day_plans (7 per week)
CREATE TABLE IF NOT EXISTS coaching.day_plans (
    day_plan_id      VARCHAR(255) PRIMARY KEY,
    week_plan_id     VARCHAR(255) NOT NULL REFERENCES coaching.week_plans(week_plan_id) ON DELETE CASCADE,
    roadmap_id       VARCHAR(255) NOT NULL,
    user_id          VARCHAR(255) NOT NULL,
    scheduled_date   DATE         NOT NULL,
    created_at       TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_day_week_date UNIQUE (week_plan_id, scheduled_date)
);

CREATE INDEX IF NOT EXISTS idx_day_plans_week_plan_id ON coaching.day_plans(week_plan_id);
CREATE INDEX IF NOT EXISTS idx_day_plans_user_date   ON coaching.day_plans(user_id, scheduled_date);

-- 4. Table: session_plans (0-N per day)
CREATE TABLE IF NOT EXISTS coaching.session_plans (
    session_plan_id       VARCHAR(255) PRIMARY KEY,
    day_plan_id           VARCHAR(255) NOT NULL REFERENCES coaching.day_plans(day_plan_id) ON DELETE CASCADE,
    week_plan_id          VARCHAR(255) NOT NULL,
    roadmap_id            VARCHAR(255) NOT NULL,
    user_id               VARCHAR(255) NOT NULL,
    scheduled_date        DATE         NOT NULL,
    slot_time             VARCHAR(50)  NOT NULL DEFAULT '',
    status                VARCHAR(50)  NOT NULL DEFAULT 'PENDING',
    source                VARCHAR(50)  NOT NULL DEFAULT 'COACH_SCHEDULED',
    target_muscle_groups  JSONB        NOT NULL DEFAULT '[]',
    prescription          JSONB        NOT NULL DEFAULT '{}',
    reasoning             TEXT         NOT NULL DEFAULT '',
    generated_at          TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    completed_at          TIMESTAMP WITH TIME ZONE,
    session_scr           NUMERIC(5,2),
    session_delta_rpe     NUMERIC(4,2),
    CONSTRAINT chk_session_status CHECK (status IN ('PENDING', 'COMPLETED', 'SKIPPED')),
    CONSTRAINT chk_session_source CHECK (source IN ('COACH_SCHEDULED', 'USER_ADHOC'))
);

CREATE INDEX IF NOT EXISTS idx_session_plans_day_plan_id  ON coaching.session_plans(day_plan_id);
CREATE INDEX IF NOT EXISTS idx_session_plans_user_date    ON coaching.session_plans(user_id, scheduled_date);

-- Partial index for pending sessions (D3: Regenerate touches only PENDING)
CREATE INDEX IF NOT EXISTS ix_session_plans_pending
    ON coaching.session_plans(user_id, scheduled_date) WHERE status = 'PENDING';

-- 5. Consumer idempotency (D9): unique event_id on outbox_log
CREATE UNIQUE INDEX IF NOT EXISTS ux_coaching_outbox_log_event_id
    ON coaching.outbox_log(event_id);
