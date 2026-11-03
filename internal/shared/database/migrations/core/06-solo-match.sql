-- =====================================================================
-- BOUNDED CONTEXT: solo_match (Real-Time 1v1 PvP & Solo Matchmaking)
-- Initial Schema, Transactional Outbox, Tables, and System Bots
-- =====================================================================

CREATE SCHEMA IF NOT EXISTS solo_match;

-- 1. TRANSACTIONAL OUTBOX & AUDIT LOG
CREATE TABLE IF NOT EXISTS solo_match.outbox (
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

ALTER TABLE solo_match.outbox ADD COLUMN IF NOT EXISTS aggregate_type VARCHAR(255);
ALTER TABLE solo_match.outbox ADD COLUMN IF NOT EXISTS aggregate_id VARCHAR(255);

CREATE TABLE IF NOT EXISTS solo_match.outbox_log (
    id UUID PRIMARY KEY,
    event_id UUID NOT NULL,
    event_type VARCHAR(255) NOT NULL,
    payload JSONB NOT NULL,
    partition_key VARCHAR(255) NOT NULL,
    processed_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    status VARCHAR(50) NOT NULL,
    error_message TEXT
);

CREATE INDEX IF NOT EXISTS idx_solo_match_outbox_log_event_status ON solo_match.outbox_log (event_id, status);

-- 2. USER SNAPSHOTS (Local snapshot for fast matchmaking & schema isolation)
CREATE TABLE IF NOT EXISTS solo_match.user_snapshots (
    user_id UUID PRIMARY KEY,
    display_name VARCHAR(128) NOT NULL,
    avatar_url TEXT NOT NULL DEFAULT '',
    synced_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- 3. SOLO MATCHES (Match lifecycle & final outcome)
CREATE TABLE IF NOT EXISTS solo_match.solo_matches (
    id UUID PRIMARY KEY,
    exercise_id VARCHAR(64) NOT NULL,
    exercise_name VARCHAR(128) NOT NULL,
    challenge_type VARCHAR(32) NOT NULL,
    status VARCHAR(32) NOT NULL,
    winner_id UUID NULL,
    is_draw BOOLEAN NOT NULL DEFAULT FALSE,
    ready_deadline TIMESTAMP WITH TIME ZONE NOT NULL,
    started_at TIMESTAMP WITH TIME ZONE NULL,
    ended_at TIMESTAMP WITH TIME ZONE NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_solo_matches_status ON solo_match.solo_matches(status);
CREATE INDEX IF NOT EXISTS idx_solo_matches_ready_deadline ON solo_match.solo_matches(ready_deadline) WHERE status = 'PENDING_READY';

-- 4. MATCH PARTICIPANTS (Real user or AI bot)
CREATE TABLE IF NOT EXISTS solo_match.match_participants (
    id UUID PRIMARY KEY,
    match_id UUID NOT NULL REFERENCES solo_match.solo_matches(id) ON DELETE CASCADE,
    user_id UUID NOT NULL,
    score_value NUMERIC(10, 2) NOT NULL DEFAULT 0,
    is_completed BOOLEAN NOT NULL DEFAULT FALSE,
    completed_at TIMESTAMP WITH TIME ZONE NULL,
    result VARCHAR(32) NOT NULL DEFAULT 'PENDING',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_match_participant UNIQUE (match_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_match_participants_user_id ON solo_match.match_participants(user_id);
CREATE INDEX IF NOT EXISTS idx_match_participants_match_id ON solo_match.match_participants(match_id);

-- 5. SYSTEM BOTS (AI Bot athlete profiles for matchmaking)
CREATE TABLE IF NOT EXISTS solo_match.system_bots (
    user_id UUID PRIMARY KEY REFERENCES solo_match.user_snapshots(user_id) ON DELETE CASCADE,
    difficulty_level VARCHAR(32) NOT NULL DEFAULT 'INTERMEDIATE',
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

