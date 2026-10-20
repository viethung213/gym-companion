-- ====================================================================
-- SOCIAL BOUNDED CONTEXT - DATABASE SCHEMA EXTENSION
-- Migration: 13-add-social-feed-search-index.sql
-- Description: GIN Trigram index trên caption cho bảng social.feed_items
-- ====================================================================

CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE INDEX IF NOT EXISTS idx_social_feed_items_caption_trgm 
    ON social.feed_items USING gin (caption gin_trgm_ops);
