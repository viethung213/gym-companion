-- =====================================================================
-- Migration: 13-create-gamification-tables.sql
-- Bounded Context: Gamification Engine (XP, Streaks, Leaderboard, Outbox)
-- Standards: SSOT PostgreSQL Schema gamification.*, Zero Cross-Schema JOINs
-- =====================================================================

-- ---------------------------------------------------------------------
-- 1. SCHEMA: gamification
-- ---------------------------------------------------------------------
CREATE SCHEMA IF NOT EXISTS gamification;

-- ---------------------------------------------------------------------
-- 2. TRANSACTIONAL OUTBOX & AUDIT LOG (Module-wide SSOT)
-- ---------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS gamification.outbox (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_id UUID NOT NULL UNIQUE,
    aggregate_type VARCHAR(255),
    aggregate_id VARCHAR(255),
    event_type VARCHAR(255) NOT NULL,
    payload JSONB NOT NULL,
    partition_key VARCHAR(255) NOT NULL,
    published BOOLEAN NOT NULL DEFAULT FALSE,
    published_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    status VARCHAR(50) NOT NULL DEFAULT 'PENDING',
    retry_count INT NOT NULL DEFAULT 0,
    max_retries INT NOT NULL DEFAULT 5,
    last_error TEXT,
    locked_until TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_gamification_outbox_unprocessed
    ON gamification.outbox (created_at ASC)
    WHERE published = FALSE AND status != 'DEAD_LETTER';

CREATE TABLE IF NOT EXISTS gamification.outbox_log (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_id UUID NOT NULL,
    event_type VARCHAR(255) NOT NULL,
    payload JSONB NOT NULL,
    partition_key VARCHAR(255) NOT NULL,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    status VARCHAR(50) NOT NULL,
    error_message TEXT
);

CREATE INDEX IF NOT EXISTS idx_gamification_outbox_log_event_status
    ON gamification.outbox_log (event_id, status);

CREATE UNIQUE INDEX IF NOT EXISTS ux_gamification_outbox_log_event_id
    ON gamification.outbox_log (event_id);

-- ---------------------------------------------------------------------
-- 3. XP & LEVEL PROGRESSION (XP, Level 1-100, Append-Only History)
-- ---------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS gamification.user_xp (
    user_id UUID PRIMARY KEY,
    xp BIGINT NOT NULL DEFAULT 0,
    level INT NOT NULL DEFAULT 1,
    last_nutrition_reward_date DATE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_user_xp_non_negative CHECK (xp >= 0),
    CONSTRAINT chk_user_level_range CHECK (level >= 1 AND level <= 100)
);

-- Chỉ mục B-tree tối ưu hóa truy vấn Leaderboard Top 50 theo thời gian thực (< 5ms)
-- Tie-breaking: xp DESC, updated_at ASC, user_id ASC để đảm bảo thứ tự phân trang tất định tuyệt đối
CREATE INDEX IF NOT EXISTS idx_user_xp_leaderboard
    ON gamification.user_xp (xp DESC, updated_at ASC, user_id ASC);

CREATE TABLE IF NOT EXISTS gamification.xp_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES gamification.user_xp(user_id) ON DELETE CASCADE,
    amount INT NOT NULL,
    source VARCHAR(50) NOT NULL,
    session_id UUID,
    activity_date DATE NOT NULL DEFAULT CURRENT_DATE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_xp_history_amount_positive CHECK (amount > 0),
    CONSTRAINT uq_xp_history_workout_session UNIQUE (user_id, session_id)
);

CREATE INDEX IF NOT EXISTS idx_xp_history_user_created
    ON gamification.xp_history (user_id, created_at DESC);

-- Khóa chặn 100% rủi ro cộng điểm dinh dưỡng lặp trong cùng 1 ngày (PostgreSQL Partial Unique Index)
CREATE UNIQUE INDEX IF NOT EXISTS uq_xp_history_nutrition_daily
    ON gamification.xp_history (user_id, activity_date)
    WHERE source = 'NUTRITION';

-- ---------------------------------------------------------------------
-- 4. STREAKS & HABITS ENGINE (Continuous Discipline, Activity Tracking)
-- ---------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS gamification.user_streak (
    user_id UUID PRIMARY KEY,
    current_streak INT NOT NULL DEFAULT 0,
    longest_streak INT NOT NULL DEFAULT 0,
    last_active_date DATE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_user_streak_non_negative CHECK (current_streak >= 0),
    CONSTRAINT chk_user_streak_longest CHECK (longest_streak >= current_streak)
);

CREATE TABLE IF NOT EXISTS gamification.streak_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES gamification.user_streak(user_id) ON DELETE CASCADE,
    activity_date DATE NOT NULL,
    activity_type VARCHAR(50) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_streak_history_user_date UNIQUE (user_id, activity_date)
);

CREATE INDEX IF NOT EXISTS idx_streak_history_user_date
    ON gamification.streak_history (user_id, activity_date DESC);

-- ---------------------------------------------------------------------
-- 5. LEADERBOARD USER SNAPSHOTS (Decoupled Identity for Top 50 Views)
-- ---------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS gamification.user_snapshots (
    user_id UUID PRIMARY KEY,
    display_name VARCHAR(100) NOT NULL DEFAULT 'Gymer',
    avatar_url TEXT,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ---------------------------------------------------------------------
-- 6. BADGES & ACHIEVEMENTS (Milestone Honors & Collection)
-- ---------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS gamification.user_badges (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES gamification.user_xp(user_id) ON DELETE CASCADE,
    badge_code VARCHAR(50) NOT NULL,
    unlocked_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    metadata JSONB DEFAULT '{}'::jsonb,
    CONSTRAINT uq_user_badge UNIQUE (user_id, badge_code)
);

CREATE INDEX IF NOT EXISTS idx_user_badges_user_unlocked
    ON gamification.user_badges (user_id, unlocked_at DESC);


-- ---------------------------------------------------------------------
-- 7. FITCOINS VIRTUAL WALLET & IMMUTABLE LEDGER
-- ---------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS gamification.user_wallet (
    user_id UUID PRIMARY KEY,
    balance BIGINT NOT NULL DEFAULT 0,
    last_workout_reward_date DATE,
    last_pr_reward_date DATE,
    last_nutrition_reward_date DATE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_user_wallet_balance_non_negative CHECK (balance >= 0)
);

CREATE TABLE IF NOT EXISTS gamification.coin_ledger (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES gamification.user_wallet(user_id) ON DELETE CASCADE,
    amount BIGINT NOT NULL,
    balance_after BIGINT NOT NULL,
    reason VARCHAR(50) NOT NULL,
    reference_id VARCHAR(100),
    idempotency_key VARCHAR(150) NOT NULL,
    activity_date DATE NOT NULL DEFAULT CURRENT_DATE,
    admin_id UUID,
    note TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_coin_ledger_amount_non_zero CHECK (amount <> 0),
    CONSTRAINT chk_coin_ledger_balance_after_non_negative CHECK (balance_after >= 0),
    CONSTRAINT uq_coin_ledger_user_idempotency UNIQUE (user_id, idempotency_key)
);

CREATE INDEX IF NOT EXISTS idx_coin_ledger_user_created
    ON gamification.coin_ledger (user_id, created_at DESC, id DESC);

CREATE UNIQUE INDEX IF NOT EXISTS uq_coin_ledger_daily_cap
    ON gamification.coin_ledger (user_id, reason, activity_date)
    WHERE reason IN ('EARN_WORKOUT', 'EARN_PR', 'EARN_NUTRITION');
