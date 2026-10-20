-- Seed script for Social module
INSERT INTO social.users (id, full_name, avatar_url, role, updated_at) VALUES
  ('00000000-0000-0000-0000-000000000002', 'Hoàng Nam', 'https://images.unsplash.com/photo-1534528741775-53994a69daeb?auto=format&fit=crop&w=300&q=80', 'user', NOW()),
  ('00000000-0000-0000-0000-000000000003', 'Coach Alex', 'https://images.unsplash.com/photo-1568602471122-7832951cc4c5?auto=format&fit=crop&w=300&q=80', 'coach', NOW()),
  ('00000000-0000-0000-0000-000000000004', 'Elena Rostova', 'https://images.unsplash.com/photo-1517841905240-472988babdf9?auto=format&fit=crop&w=300&q=80', 'user', NOW()),
  ('00000000-0000-0000-0000-000000000005', 'Marcus Vance', 'https://images.unsplash.com/photo-1507003211169-0a1dd7228f2d?auto=format&fit=crop&w=300&q=80', 'user', NOW()),
  ('00000000-0000-0000-0000-000000000006', 'Mai Linh', 'https://images.unsplash.com/photo-1494790108377-be9c29b29330?auto=format&fit=crop&w=300&q=80', 'user', NOW())
ON CONFLICT (id) DO UPDATE SET full_name = EXCLUDED.full_name, avatar_url = EXCLUDED.avatar_url;

-- Follows
INSERT INTO social.follows (id, follower_id, following_id, created_at) VALUES
  ('a0000000-0000-0000-0000-000000000001', '00000000-0000-0000-0000-000000000002', '00000000-0000-0000-0000-000000000003', NOW() - INTERVAL '5 days'),
  ('a0000000-0000-0000-0000-000000000002', '00000000-0000-0000-0000-000000000002', '00000000-0000-0000-0000-000000000004', NOW() - INTERVAL '4 days'),
  ('a0000000-0000-0000-0000-000000000003', '00000000-0000-0000-0000-000000000002', '00000000-0000-0000-0000-000000000005', NOW() - INTERVAL '3 days'),
  ('a0000000-0000-0000-0000-000000000004', '00000000-0000-0000-0000-000000000004', '00000000-0000-0000-0000-000000000002', NOW() - INTERVAL '2 days'),
  ('a0000000-0000-0000-0000-000000000005', '00000000-0000-0000-0000-000000000003', '00000000-0000-0000-0000-000000000002', NOW() - INTERVAL '1 days')
ON CONFLICT (follower_id, following_id) DO NOTHING;

-- Feed items
INSERT INTO social.feed_items (id, user_id, item_type, caption, media_urls, data, visibility, reaction_count, comment_count, created_at, updated_at) VALUES
  ('f0000000-0000-0000-0000-000000000001',
   '00000000-0000-0000-0000-000000000003',
   'WORKOUT_ACTIVITY',
   'Phá kỷ lục cá nhân buổi sáng hôm nay! 5 hiệp Squat 160kg mượt mà 🔥💪',
   '[]'::jsonb,
   '{"session_id":"s-101","workout_title":"Heavy Lower Body & Core","duration_seconds":3600,"total_volume_kg":6800.0,"exercise_count":5,"total_sets":18}'::jsonb,
   'PUBLIC',
   8,
   2,
   NOW() - INTERVAL '2 hours',
   NOW() - INTERVAL '2 hours'),

  ('f0000000-0000-0000-0000-000000000002',
   '00000000-0000-0000-0000-000000000004',
   'WORKOUT_ACTIVITY',
   'Deadlift day never disappoints! Hoàn thành giáo án tuần 4 xuất sắc 🎯',
   '[]'::jsonb,
   '{"session_id":"s-102","workout_title":"Deadlift & Back Hypertrophy","duration_seconds":2880,"total_volume_kg":5420.0,"exercise_count":4,"total_sets":14}'::jsonb,
   'PUBLIC',
   12,
   3,
   NOW() - INTERVAL '5 hours',
   NOW() - INTERVAL '5 hours'),

  ('f0000000-0000-0000-0000-000000000003',
   '00000000-0000-0000-0000-000000000005',
   'POST',
   'Kỷ luật là thứ đưa bạn đến đích khi động lực đã cạn kiệt. Chúc anh em đầu tuần tập luyện bùng nổ! 🚀',
   '[]'::jsonb,
   '{}'::jsonb,
   'PUBLIC',
   15,
   1,
   NOW() - INTERVAL '1 day',
   NOW() - INTERVAL '1 day'),

  ('f0000000-0000-0000-0000-000000000004',
   '00000000-0000-0000-0000-000000000002',
   'WORKOUT_ACTIVITY',
   'Ngày tập ngực đầu tiên sau chuỗi 7 ngày streak. Cảm giác cơ bắp căng cực tốt!',
   '[]'::jsonb,
   '{"session_id":"s-103","workout_title":"Chest & Triceps Focus","duration_seconds":3150,"total_volume_kg":4100.0,"exercise_count":6,"total_sets":16}'::jsonb,
   'PUBLIC',
   6,
   1,
   NOW() - INTERVAL '18 hours',
   NOW() - INTERVAL '18 hours')
ON CONFLICT (id) DO UPDATE SET
  caption = EXCLUDED.caption,
  data = EXCLUDED.data,
  reaction_count = EXCLUDED.reaction_count,
  comment_count = EXCLUDED.comment_count;

-- Reactions
INSERT INTO social.reactions (id, user_id, feed_item_id, reaction_type, created_at) VALUES
  ('b0000000-0000-0000-0000-000000000001', '00000000-0000-0000-0000-000000000002', 'f0000000-0000-0000-0000-000000000001', 'FIRE', NOW() - INTERVAL '1 hour'),
  ('b0000000-0000-0000-0000-000000000002', '00000000-0000-0000-0000-000000000004', 'f0000000-0000-0000-0000-000000000001', 'MUSCLE', NOW() - INTERVAL '1 hour'),
  ('b0000000-0000-0000-0000-000000000003', '00000000-0000-0000-0000-000000000003', 'f0000000-0000-0000-0000-000000000002', 'CLAP', NOW() - INTERVAL '4 hours')
ON CONFLICT (user_id, feed_item_id) DO NOTHING;

-- Comments
INSERT INTO social.comments (id, user_id, feed_item_id, parent_id, content, created_at, updated_at) VALUES
  ('c0000000-0000-0000-0000-000000000001', '00000000-0000-0000-0000-000000000002', 'f0000000-0000-0000-0000-000000000001', NULL, 'Form quá chuẩn luôn coach ơi! 👏', NOW() - INTERVAL '1 hour', NOW() - INTERVAL '1 hour'),
  ('c0000000-0000-0000-0000-000000000002', '00000000-0000-0000-0000-000000000004', 'f0000000-0000-0000-0000-000000000001', NULL, 'Quá khủng khiếp!', NOW() - INTERVAL '30 minutes', NOW() - INTERVAL '30 minutes'),
  ('c0000000-0000-0000-0000-000000000003', '00000000-0000-0000-0000-000000000003', 'f0000000-0000-0000-0000-000000000002', NULL, 'Tiến bộ vượt bậc tuần này Elena!', NOW() - INTERVAL '3 hours', NOW() - INTERVAL '3 hours')
ON CONFLICT (id) DO NOTHING;
