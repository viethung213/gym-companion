-- =====================================================================
-- Migration: 14-create-gamification-wallet-tables.sql
-- Bounded Context: Gamification Engine (FitCoins Wallet, Immutable Ledger)
-- Standards: SSOT PostgreSQL Schema gamification.*, Zero Cross-Schema JOINs
-- =====================================================================

-- ---------------------------------------------------------------------
-- 1. USER WALLET (Real-time Balance with Non-Negative Constraint)
-- ---------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS gamification.user_wallet (
    user_id UUID PRIMARY KEY,
    balance BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_user_wallet_balance_non_negative CHECK (balance >= 0)
);

-- ---------------------------------------------------------------------
-- 2. COIN LEDGER (Append-Only Immutable Audit Log)
-- ---------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS gamification.coin_ledger (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES gamification.user_wallet(user_id) ON DELETE RESTRICT,
    admin_id UUID,
    amount BIGINT NOT NULL,
    balance_after BIGINT NOT NULL,
    reason VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_coin_ledger_amount_nonzero CHECK (amount <> 0),
    CONSTRAINT chk_coin_ledger_balance_after_non_negative CHECK (balance_after >= 0)
);

CREATE INDEX IF NOT EXISTS idx_coin_ledger_user_created
    ON gamification.coin_ledger (user_id, created_at DESC);
