-- ====================================================================
-- SOCIAL BOUNDED CONTEXT - DATABASE SCHEMA INIT SCRIPT
-- Migration: 12-create-social-tables.sql
-- Pattern: Single Table Feed Items with JSONB payload
-- Modules: Social Graph (Follows), Feed Items, Reactions, Comments, Users Snapshot
-- ====================================================================

-- 1. Tạo Schema riêng biệt cho module Social
CREATE SCHEMA IF NOT EXISTS social;

-- --------------------------------------------------------------------
-- 2. Bảng: social.follows (Đồ thị quan hệ theo dõi giữa người dùng)
-- --------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS social.follows (
    id UUID PRIMARY KEY,
    follower_id UUID NOT NULL,       -- Người bấm theo dõi
    following_id UUID NOT NULL,      -- Người được theo dõi
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,

    -- Ràng buộc: Một người không thể tự theo dõi chính mình
    CONSTRAINT chk_no_self_follow CHECK (follower_id <> following_id),
    -- Ràng buộc: Mỗi cặp quan hệ chỉ tồn tại duy nhất 1 lần (chống trùng lặp)
    CONSTRAINT uq_social_follows UNIQUE (follower_id, following_id)
);

-- Index tối ưu tốc độ truy vấn Follow/Following
CREATE INDEX IF NOT EXISTS idx_social_follows_follower_id 
    ON social.follows (follower_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_social_follows_following_id 
    ON social.follows (following_id, created_at DESC);


-- --------------------------------------------------------------------
-- 3. Bảng: social.feed_items (Bảng tin duy nhất chứa cả Bài viết & Hoạt động tập gym)
-- --------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS social.feed_items (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL,                                       -- Tác giả
    item_type VARCHAR(30) NOT NULL,                              -- 'POST', 'WORKOUT_ACTIVITY'
    caption TEXT NOT NULL DEFAULT '',                           -- Lời tựa / caption
    media_urls JSONB NOT NULL DEFAULT '[]'::jsonb,              -- Danh sách URL ảnh/video
    data JSONB NOT NULL DEFAULT '{}'::jsonb,                    -- Metadata đặc thù (workout metrics, pr, session_id...)
    visibility VARCHAR(30) NOT NULL DEFAULT 'PUBLIC',           -- PUBLIC | FOLLOWERS_ONLY | PRIVATE
    reaction_count INT NOT NULL DEFAULT 0,                       -- Denormalized counter
    comment_count INT NOT NULL DEFAULT 0,                        -- Denormalized counter
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Index siêu tốc cho Feed và Trang cá nhân
CREATE INDEX IF NOT EXISTS idx_social_feed_items_user_created 
    ON social.feed_items (user_id, created_at DESC);


DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type t JOIN pg_namespace n ON t.typnamespace = n.oid WHERE t.typname = 'reaction_type' AND n.nspname = 'social') THEN
        CREATE TYPE social.reaction_type AS ENUM ('LIKE', 'FIRE', 'MUSCLE', 'CLAP');
    END IF;
END$$;

CREATE TABLE IF NOT EXISTS social.reactions (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL,                                       -- Người thả cảm xúc
    feed_item_id UUID NOT NULL REFERENCES social.feed_items(id) ON DELETE CASCADE,
    reaction_type social.reaction_type NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,

    -- Ràng buộc: Mỗi user chỉ có 1 cảm xúc duy nhất trên mỗi feed item (hỗ trợ toggle/đổi icon)
    CONSTRAINT uq_social_reactions UNIQUE (user_id, feed_item_id)
);

CREATE INDEX IF NOT EXISTS idx_social_reactions_item 
    ON social.reactions (feed_item_id);


-- --------------------------------------------------------------------
-- 5. Bảng: social.comments (Bình luận trên Feed Item)
-- --------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS social.comments (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL,                                       -- Người bình luận
    feed_item_id UUID NOT NULL REFERENCES social.feed_items(id) ON DELETE CASCADE,
    parent_id UUID NULL REFERENCES social.comments(id) ON DELETE CASCADE, -- Hỗ trợ trả lời (nested comment)
    content TEXT NOT NULL,                                       -- Nội dung bình luận
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_social_comments_item 
    ON social.comments (feed_item_id, created_at ASC);
CREATE INDEX IF NOT EXISTS idx_social_comments_parent_id 
    ON social.comments (parent_id);


-- --------------------------------------------------------------------
-- 6. Bảng: social.users (Snapshot thông tin người dùng phục vụ hiển thị trong social)
-- --------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS social.users (
    id UUID PRIMARY KEY,
    full_name VARCHAR(255) NOT NULL DEFAULT '',
    avatar_url TEXT NOT NULL DEFAULT '',
    role VARCHAR(50) NOT NULL DEFAULT 'user',
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_social_users_updated_at 
    ON social.users (updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_social_users_role 
    ON social.users (role);
CREATE INDEX IF NOT EXISTS idx_social_users_full_name_trgm 
    ON social.users USING gin (full_name gin_trgm_ops);


