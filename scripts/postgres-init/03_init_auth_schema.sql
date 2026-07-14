-- Setup schema for Identity Service inside auth schema

CREATE TABLE IF NOT EXISTS auth.users (
    id VARCHAR(255) PRIMARY KEY,
    email VARCHAR(255) UNIQUE NOT NULL,
    google_id VARCHAR(255) UNIQUE,
    facebook_id VARCHAR(255) UNIQUE,
    full_name VARCHAR(255),
    role VARCHAR(50) NOT NULL DEFAULT 'user',
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL
);

CREATE TABLE IF NOT EXISTS auth.jwk_keys (
    id VARCHAR(255) PRIMARY KEY,
    private_key_pem TEXT NOT NULL,
    public_key_pem TEXT NOT NULL,
    algorithm VARCHAR(50) NOT NULL,
    status VARCHAR(50) NOT NULL,
    created_at TIMESTAMP NOT NULL,
    expires_at TIMESTAMP NOT NULL
);

CREATE TABLE IF NOT EXISTS auth.sessions (
    token VARCHAR(255) PRIMARY KEY,
    user_id VARCHAR(255) NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    created_at TIMESTAMP NOT NULL,
    expires_at TIMESTAMP NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_users_google_id ON auth.users (google_id) WHERE google_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_users_facebook_id ON auth.users (facebook_id) WHERE facebook_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON auth.sessions (user_id);
CREATE INDEX IF NOT EXISTS idx_jwk_keys_status ON auth.jwk_keys (status);

