-- =====================================================================
-- BOUNDED CONTEXT: auth (Identity & Authentication Service)
-- Initial Schema, Transactional Outbox, Tables, and Security Indexes
-- =====================================================================

CREATE SCHEMA IF NOT EXISTS auth;

-- 1. TRANSACTIONAL OUTBOX & AUDIT LOG
CREATE TABLE IF NOT EXISTS auth.outbox (
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

ALTER TABLE auth.outbox ADD COLUMN IF NOT EXISTS aggregate_type VARCHAR(255);
ALTER TABLE auth.outbox ADD COLUMN IF NOT EXISTS aggregate_id VARCHAR(255);

CREATE TABLE IF NOT EXISTS auth.outbox_log (
    id UUID PRIMARY KEY,
    event_id UUID NOT NULL,
    event_type VARCHAR(255) NOT NULL,
    payload JSONB NOT NULL,
    partition_key VARCHAR(255) NOT NULL,
    processed_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    status VARCHAR(50) NOT NULL,
    error_message TEXT
);

CREATE INDEX IF NOT EXISTS idx_auth_outbox_log_event_status ON auth.outbox_log (event_id, status);

-- 2. ROLES
CREATE TABLE IF NOT EXISTS auth.roles (
    id VARCHAR(50) PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    description TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Seed default system roles
INSERT INTO auth.roles (id, name, description)
VALUES 
    ('role_admin', 'Admin', 'System administrator with full permissions'),
    ('role_user', 'User', 'Standard authenticated athlete'),
    ('role_coach', 'Coach', 'Personal coach / fitness trainer'),
    ('role_brand', 'Brand', 'Enterprise nutrition and equipment brand partner')
ON CONFLICT (id) DO NOTHING;

-- 3. USERS
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type t JOIN pg_namespace n ON t.typnamespace = n.oid WHERE t.typname = 'user_status' AND n.nspname = 'auth') THEN
        CREATE TYPE auth.user_status AS ENUM ('active', 'inactive', 'locked', 'suspended');
    END IF;
END$$;

CREATE TABLE IF NOT EXISTS auth.users (
    id VARCHAR(255) PRIMARY KEY,
    full_name VARCHAR(255),
    role_id VARCHAR(50) NOT NULL REFERENCES auth.roles(id) ON UPDATE CASCADE,
    status auth.user_status NOT NULL DEFAULT 'active',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_users_role_id ON auth.users (role_id);

-- 4. USER IDENTITIES
CREATE TABLE IF NOT EXISTS auth.user_identities (
    id VARCHAR(255) PRIMARY KEY,
    user_id VARCHAR(255) NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    identity_type VARCHAR(50) NOT NULL,
    identifier VARCHAR(255) NOT NULL,
    credential_data VARCHAR(255),
    metadata JSONB,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_identities_user_id UNIQUE (user_id),
    CONSTRAINT uq_identity_type_identifier UNIQUE (identity_type, identifier)
);

CREATE INDEX IF NOT EXISTS idx_identities_user_id ON auth.user_identities (user_id);
CREATE INDEX IF NOT EXISTS idx_identities_lookup ON auth.user_identities (identity_type, identifier);

-- 5. OTPS
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type t JOIN pg_namespace n ON t.typnamespace = n.oid WHERE t.typname = 'otp_purpose' AND n.nspname = 'auth') THEN
        CREATE TYPE auth.otp_purpose AS ENUM (
            'register',
            'login',
            'reset_password',
            'verify_phone',
            'verify_email',
            'change_password'
        );
    END IF;
END$$;

CREATE TABLE IF NOT EXISTS auth.otps (
    id VARCHAR(255) PRIMARY KEY,
    identifier VARCHAR(255) NOT NULL,
    otp_hash VARCHAR(255) NOT NULL,
    purpose auth.otp_purpose NOT NULL,
    attempts INT NOT NULL DEFAULT 0,
    max_attempts INT NOT NULL DEFAULT 5,
    expires_at TIMESTAMP NOT NULL,
    resend_available_at TIMESTAMP NOT NULL,
    is_used BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_otps_verification ON auth.otps (identifier, purpose, is_used, expires_at);
CREATE UNIQUE INDEX IF NOT EXISTS uq_active_otp_per_identifier_purpose ON auth.otps (identifier, purpose) WHERE is_used = FALSE;

-- 6. JWK KEYS & SESSIONS
CREATE TABLE IF NOT EXISTS auth.jwk_keys (
    id VARCHAR(255) PRIMARY KEY,
    private_key_pem TEXT NOT NULL,
    public_key_pem TEXT NOT NULL,
    algorithm VARCHAR(50) NOT NULL,
    status VARCHAR(50) NOT NULL,
    created_at TIMESTAMP NOT NULL,
    expires_at TIMESTAMP NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_jwk_keys_status ON auth.jwk_keys (status);

CREATE TABLE IF NOT EXISTS auth.sessions (
    token VARCHAR(255) PRIMARY KEY,
    user_id VARCHAR(255) NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    created_at TIMESTAMP NOT NULL,
    expires_at TIMESTAMP NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON auth.sessions (user_id);

-- 7. PASSWORD RESET TOKENS
CREATE TABLE IF NOT EXISTS auth.password_reset_tokens (
    token VARCHAR(255) PRIMARY KEY,
    user_id VARCHAR(255) NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    expires_at TIMESTAMP NOT NULL,
    is_used BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_password_reset_tokens_user_id ON auth.password_reset_tokens (user_id);
CREATE INDEX IF NOT EXISTS idx_password_reset_tokens_validity ON auth.password_reset_tokens (token, is_used, expires_at);

-- 8. BRAND REQUESTS
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type t JOIN pg_namespace n ON t.typnamespace = n.oid WHERE t.typname = 'brand_request_status' AND n.nspname = 'auth') THEN
        CREATE TYPE auth.brand_request_status AS ENUM ('pending', 'approved', 'rejected');
    END IF;
END$$;

CREATE TABLE IF NOT EXISTS auth.brand_requests (
    id VARCHAR(255) PRIMARY KEY,
    user_id VARCHAR(255) NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    brand_name VARCHAR(255) NOT NULL,
    description TEXT,
    contact_phone VARCHAR(50) NOT NULL,
    address TEXT NOT NULL,
    status auth.brand_request_status NOT NULL DEFAULT 'pending',
    rejection_reason TEXT,
    reviewed_by VARCHAR(255) REFERENCES auth.users(id) ON DELETE SET NULL,
    reviewed_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_brand_requests_user_id ON auth.brand_requests (user_id);
CREATE INDEX IF NOT EXISTS idx_brand_requests_status ON auth.brand_requests (status);
CREATE INDEX IF NOT EXISTS idx_brand_requests_created_at ON auth.brand_requests (created_at DESC);
CREATE UNIQUE INDEX IF NOT EXISTS uq_brand_requests_user_pending ON auth.brand_requests (user_id) WHERE status = 'pending';

