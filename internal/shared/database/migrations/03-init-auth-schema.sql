-- Setup schema for Identity & Authentication Service inside auth schema

-- 1. BẢNG ROLES (Quản lý vai trò người dùng)
CREATE TABLE IF NOT EXISTS auth.roles (
    id VARCHAR(50) PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    description TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Enum trạng thái người dùng trong schema auth
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type t JOIN pg_namespace n ON t.typnamespace = n.oid WHERE t.typname = 'user_status' AND n.nspname = 'auth') THEN
        CREATE TYPE auth.user_status AS ENUM ('active', 'inactive', 'locked', 'suspended');
    END IF;
END$$;

-- 2. BẢNG USERS (Hồ sơ thông tin cơ bản của người dùng trong Identity Bounded Context)
CREATE TABLE IF NOT EXISTS auth.users (
    id VARCHAR(255) PRIMARY KEY,
    full_name VARCHAR(255),
    role_id VARCHAR(50) NOT NULL REFERENCES auth.roles(id) ON UPDATE CASCADE,
    status auth.user_status NOT NULL DEFAULT 'active',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 3. BẢNG USER_IDENTITIES (Các phương thức đăng nhập: Google, Facebook, Gmail, SĐT)
CREATE TABLE IF NOT EXISTS auth.user_identities (
    id VARCHAR(255) PRIMARY KEY,
    user_id VARCHAR(255) NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    identity_type VARCHAR(50) NOT NULL,         -- 'email', 'phone', 'google', 'facebook'
    identifier VARCHAR(255) NOT NULL,           -- Địa chỉ email, SĐT (+84...), hoặc Google/Facebook Sub ID
    credential_data VARCHAR(255),               -- Password hash (Argon2id/bcrypt) nếu đăng nhập bằng mật khẩu
    metadata JSONB,                             -- Lưu thêm thông tin phụ từ OAuth provider nếu có
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_identities_user_id UNIQUE (user_id),
    CONSTRAINT uq_identity_type_identifier UNIQUE (identity_type, identifier)
);

-- Enum mục đích sử dụng mã OTP trong schema auth
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

-- 4. BẢNG OTPS (Lưu và quản lý mã OTP xác thực qua SMS / Email)
-- Cột `id` đóng vai trò là `otp_token` (UUID v4) trả về client ở SendOTP và gửi ngầm ở VerifyOTP
CREATE TABLE IF NOT EXISTS auth.otps (
    id VARCHAR(255) PRIMARY KEY,                    -- otp_token duy nhất cho mỗi phiên OTP
    identifier VARCHAR(255) NOT NULL,               -- Email hoặc SĐT nhận OTP
    otp_hash VARCHAR(255) NOT NULL,                 -- Mã OTP đã được hash bảo mật
    purpose auth.otp_purpose NOT NULL,              -- Mục đích sử dụng mã OTP (enum)
    attempts INT NOT NULL DEFAULT 0,                -- Đếm số lần nhập sai
    max_attempts INT NOT NULL DEFAULT 5,            -- Giới hạn số lần thử để chống tấn công Brute-Force
    expires_at TIMESTAMP NOT NULL,                  -- Thời gian hết hạn của OTP (ví dụ: 3-5 phút)
    resend_available_at TIMESTAMP NOT NULL,          -- Thời điểm sớm nhất được phép gửi lại (cooldown 60s)
    is_used BOOLEAN NOT NULL DEFAULT FALSE,         -- Đánh dấu mã đã sử dụng hoặc bị hủy
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 5. BẢNG JWK KEYS & SESSIONS
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
    token VARCHAR(255) PRIMARY KEY,                 -- Refresh token phiên làm việc
    user_id VARCHAR(255) NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    created_at TIMESTAMP NOT NULL,
    expires_at TIMESTAMP NOT NULL
);

-- 6. BẢNG PASSWORD_RESET_TOKENS (Lưu token xác thực một lần để đổi mật khẩu sau khi verify OTP)
CREATE TABLE IF NOT EXISTS auth.password_reset_tokens (
    token VARCHAR(255) PRIMARY KEY,                 -- Mã reset token ngẫu nhiên cấp sau khi VerifyOTP thành công
    user_id VARCHAR(255) NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    expires_at TIMESTAMP NOT NULL,                  -- Thời gian hết hạn của reset token (ví dụ: 10-15 phút)
    is_used BOOLEAN NOT NULL DEFAULT FALSE,         -- Đảm bảo chỉ dùng 1 lần duy nhất
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 7. TỐI ƯU HÓA CHỈ MỤC (INDEXES)
CREATE INDEX IF NOT EXISTS idx_users_role_id ON auth.users (role_id);
CREATE INDEX IF NOT EXISTS idx_identities_user_id ON auth.user_identities (user_id);
CREATE INDEX IF NOT EXISTS idx_identities_lookup ON auth.user_identities (identity_type, identifier);
CREATE INDEX IF NOT EXISTS idx_otps_verification ON auth.otps (identifier, purpose, is_used, expires_at);
-- RÀNG BUỘC CỨNG: Tại một thời điểm chỉ duy nhất 1 OTP được active (is_used = FALSE) cho mỗi (identifier, purpose)
CREATE UNIQUE INDEX IF NOT EXISTS uq_active_otp_per_identifier_purpose ON auth.otps (identifier, purpose) WHERE is_used = FALSE;
CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON auth.sessions (user_id);
CREATE INDEX IF NOT EXISTS idx_password_reset_tokens_user_id ON auth.password_reset_tokens (user_id);
CREATE INDEX IF NOT EXISTS idx_password_reset_tokens_validity ON auth.password_reset_tokens (token, is_used, expires_at);
CREATE INDEX IF NOT EXISTS idx_jwk_keys_status ON auth.jwk_keys (status);
CREATE INDEX IF NOT EXISTS idx_auth_outbox_log_event_status ON auth.outbox_log (event_id, status);

-- 8. BẢNG BRAND_REQUESTS (Yêu cầu nâng cấp tài khoản người dùng thành Brand)
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

