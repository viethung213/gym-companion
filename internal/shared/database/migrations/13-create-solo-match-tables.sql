-- =====================================================================
-- Migration 13: Create solo_match schema and tables
-- =====================================================================

CREATE SCHEMA IF NOT EXISTS solo_match;

-- 1. Table: user_snapshots (Local snapshot for fast matchmaking & schema isolation)
CREATE TABLE IF NOT EXISTS solo_match.user_snapshots (
    user_id UUID PRIMARY KEY,
    display_name VARCHAR(128) NOT NULL,
    avatar_url TEXT NOT NULL DEFAULT '',
    synced_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- 2. Table: solo_matches (Match lifecycle & final outcome)
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

-- 3. Table: match_participants (Participants details: real user or AI bot)
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

-- 4. Table: system_bots (Predefined AI Bot athlete profiles for matchmaking)
-- (Lưu ý: Bảng solo_match.outbox và solo_match.outbox_log đã được khởi tạo tập trung tại 01-init-schemas.sql)
CREATE TABLE IF NOT EXISTS solo_match.system_bots (
    user_id UUID PRIMARY KEY REFERENCES solo_match.user_snapshots(user_id) ON DELETE CASCADE,
    difficulty_level VARCHAR(32) NOT NULL DEFAULT 'INTERMEDIATE',
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);
