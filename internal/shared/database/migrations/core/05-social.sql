-- =====================================================================
-- BOUNDED CONTEXT: social (Social Network, Feed Items, Follows & Reactions)
-- Initial Schema, Transactional Outbox, Tables, and Search Indexes
-- =====================================================================

CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE SCHEMA IF NOT EXISTS social;

-- 1. TRANSACTIONAL OUTBOX & AUDIT LOG
CREATE TABLE IF NOT EXISTS social.outbox (
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

CREATE TABLE IF NOT EXISTS social.outbox_log (
    id UUID PRIMARY KEY,
    event_id UUID NOT NULL,
    event_type VARCHAR(255) NOT NULL,
    payload JSONB NOT NULL,
    partition_key VARCHAR(255) NOT NULL,
    processed_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    status VARCHAR(50) NOT NULL,
    error_message TEXT
);

CREATE INDEX IF NOT EXISTS idx_social_outbox_log_event_status ON social.outbox_log (event_id, status);

-- 2. FOLLOWS (Social Graph)
CREATE TABLE IF NOT EXISTS social.follows (
    id UUID PRIMARY KEY,
    follower_id UUID NOT NULL,
    following_id UUID NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT chk_no_self_follow CHECK (follower_id <> following_id),
    CONSTRAINT uq_social_follows UNIQUE (follower_id, following_id)
);

CREATE INDEX IF NOT EXISTS idx_social_follows_follower_id ON social.follows (follower_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_social_follows_following_id ON social.follows (following_id, created_at DESC);

-- 3. FEED ITEMS (Posts & Workout Activities)
CREATE TABLE IF NOT EXISTS social.feed_items (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL,
    item_type VARCHAR(30) NOT NULL,
    caption TEXT NOT NULL DEFAULT '',
    media_urls JSONB NOT NULL DEFAULT '[]'::jsonb,
    data JSONB NOT NULL DEFAULT '{}'::jsonb,
    visibility VARCHAR(30) NOT NULL DEFAULT 'PUBLIC',
    reaction_count INT NOT NULL DEFAULT 0,
    comment_count INT NOT NULL DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_social_feed_items_user_created ON social.feed_items (user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_social_feed_items_caption_trgm ON social.feed_items USING gin (caption gin_trgm_ops);

-- 4. REACTIONS
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type t JOIN pg_namespace n ON t.typnamespace = n.oid WHERE t.typname = 'reaction_type' AND n.nspname = 'social') THEN
        CREATE TYPE social.reaction_type AS ENUM ('LIKE', 'FIRE', 'MUSCLE', 'CLAP');
    END IF;
END$$;

CREATE TABLE IF NOT EXISTS social.reactions (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL,
    feed_item_id UUID NOT NULL REFERENCES social.feed_items(id) ON DELETE CASCADE,
    reaction_type social.reaction_type NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_social_reactions UNIQUE (user_id, feed_item_id)
);

CREATE INDEX IF NOT EXISTS idx_social_reactions_item ON social.reactions (feed_item_id);

-- 5. COMMENTS
CREATE TABLE IF NOT EXISTS social.comments (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL,
    feed_item_id UUID NOT NULL REFERENCES social.feed_items(id) ON DELETE CASCADE,
    parent_id UUID NULL REFERENCES social.comments(id) ON DELETE CASCADE,
    content TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_social_comments_item ON social.comments (feed_item_id, created_at ASC);
CREATE INDEX IF NOT EXISTS idx_social_comments_parent_id ON social.comments (parent_id);

-- 6. USERS SNAPSHOT
CREATE TABLE IF NOT EXISTS social.users (
    id UUID PRIMARY KEY,
    full_name VARCHAR(255) NOT NULL DEFAULT '',
    avatar_url TEXT NOT NULL DEFAULT 'https://lqhhwimtzrcrmyfhlizv.supabase.co/storage/v1/object/public/gym-companion-assets/avatar_default/male/avatar-vo-tri-loopy-cute.jpg',
    role VARCHAR(50) NOT NULL DEFAULT 'user',
    follower_count INT NOT NULL DEFAULT 0,
    following_count INT NOT NULL DEFAULT 0,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_social_users_updated_at ON social.users (updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_social_users_role ON social.users (role);
CREATE INDEX IF NOT EXISTS idx_social_users_full_name_trgm ON social.users USING gin (full_name gin_trgm_ops);

