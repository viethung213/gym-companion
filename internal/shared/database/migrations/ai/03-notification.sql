-- =====================================================================
-- BOUNDED CONTEXT: notification (In-App, Push Notifications & User Settings)
-- Initial Schema, Transactional Outbox, Tables, and Delivery Indexes
-- =====================================================================

CREATE SCHEMA IF NOT EXISTS notification;

-- 1. TRANSACTIONAL OUTBOX & AUDIT LOG
CREATE TABLE IF NOT EXISTS notification.outbox (
    id UUID PRIMARY KEY,
    event_id UUID NOT NULL UNIQUE,
    event_type VARCHAR(255) NOT NULL,
    payload JSONB NOT NULL,
    partition_key VARCHAR(255) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    published BOOLEAN DEFAULT FALSE,
    published_at TIMESTAMP WITH TIME ZONE,
    status VARCHAR(50) DEFAULT 'PENDING' NOT NULL,
    locked_until TIMESTAMP WITH TIME ZONE
);

CREATE TABLE IF NOT EXISTS notification.outbox_log (
    id UUID PRIMARY KEY,
    event_id UUID NOT NULL,
    event_type VARCHAR(255) NOT NULL,
    payload JSONB NOT NULL,
    partition_key VARCHAR(255) NOT NULL,
    processed_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    status VARCHAR(50) NOT NULL,
    error_message TEXT
);

CREATE INDEX IF NOT EXISTS idx_notification_outbox_log_event_status ON notification.outbox_log (event_id, status);

-- 2. USER DEVICES (Device Tokens for FCM/APNs)
CREATE TABLE IF NOT EXISTS notification.user_devices (
    id UUID PRIMARY KEY,
    user_id VARCHAR(255) NOT NULL,
    device_token TEXT NOT NULL,
    device_type VARCHAR(50) NOT NULL,
    is_active BOOLEAN DEFAULT TRUE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL,
    last_used_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT user_device_token_unique UNIQUE (user_id, device_token)
);

CREATE INDEX IF NOT EXISTS idx_notification_user_devices_user_id 
    ON notification.user_devices (user_id, is_active);

-- 3. USER SETTINGS
CREATE TABLE IF NOT EXISTS notification.user_settings (
    user_id VARCHAR(255) PRIMARY KEY,
    enable_push BOOLEAN DEFAULT TRUE NOT NULL,
    enable_email BOOLEAN DEFAULT TRUE NOT NULL,
    enable_sms BOOLEAN DEFAULT TRUE NOT NULL,
    quiet_hours_start VARCHAR(10) DEFAULT '' NOT NULL,
    quiet_hours_end VARCHAR(10) DEFAULT '' NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL
);

-- 4. IN-APP NOTIFICATIONS
CREATE TABLE IF NOT EXISTS notification.in_app_notifications (
    id UUID PRIMARY KEY,
    user_id VARCHAR(255) NOT NULL,
    title VARCHAR(255) NOT NULL,
    body TEXT NOT NULL,
    data JSONB DEFAULT '{}'::jsonb NOT NULL,
    is_read BOOLEAN DEFAULT FALSE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_notification_in_app_notifications_user_id 
    ON notification.in_app_notifications (user_id, created_at DESC);

