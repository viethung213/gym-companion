-- ==========================================
ao -- SEED DATA: exercise schema (14 Exercises + Metadata)
-- Self-contained seed script. Safe to run multiple times.
-- ==========================================

-- 1. Seed body_parts
INSERT INTO exercise.body_parts (id, name) VALUES ('bp_waist', 'Waist') ON CONFLICT (id) DO NOTHING;
INSERT INTO exercise.body_parts (id, name) VALUES ('bp_upper_legs', 'Upper Legs') ON CONFLICT (id) DO NOTHING;
INSERT INTO exercise.body_parts (id, name) VALUES ('bp_back', 'Back') ON CONFLICT (id) DO NOTHING;
INSERT INTO exercise.body_parts (id, name) VALUES ('bp_lower_legs', 'Lower Legs') ON CONFLICT (id) DO NOTHING;
INSERT INTO exercise.body_parts (id, name) VALUES ('bp_chest', 'Chest') ON CONFLICT (id) DO NOTHING;
INSERT INTO exercise.body_parts (id, name) VALUES ('bp_upper_arms', 'Upper Arms') ON CONFLICT (id) DO NOTHING;
INSERT INTO exercise.body_parts (id, name) VALUES ('bp_cardio', 'Cardio') ON CONFLICT (id) DO NOTHING;
INSERT INTO exercise.body_parts (id, name) VALUES ('bp_shoulders', 'Shoulders') ON CONFLICT (id) DO NOTHING;
INSERT INTO exercise.body_parts (id, name) VALUES ('bp_lower_arms', 'Lower Arms') ON CONFLICT (id) DO NOTHING;

-- 2. Seed equipments
INSERT INTO exercise.equipments (id, name) VALUES ('eq_body_weight', 'Body Weight') ON CONFLICT (id) DO NOTHING;
INSERT INTO exercise.equipments (id, name) VALUES ('eq_cable', 'Cable') ON CONFLICT (id) DO NOTHING;
INSERT INTO exercise.equipments (id, name) VALUES ('eq_leverage_machine', 'Leverage Machine') ON CONFLICT (id) DO NOTHING;
INSERT INTO exercise.equipments (id, name) VALUES ('eq_assisted', 'Assisted') ON CONFLICT (id) DO NOTHING;
INSERT INTO exercise.equipments (id, name) VALUES ('eq_medicine_ball', 'Medicine Ball') ON CONFLICT (id) DO NOTHING;
INSERT INTO exercise.equipments (id, name) VALUES ('eq_stability_ball', 'Stability Ball') ON CONFLICT (id) DO NOTHING;
INSERT INTO exercise.equipments (id, name) VALUES ('eq_band', 'Band') ON CONFLICT (id) DO NOTHING;
INSERT INTO exercise.equipments (id, name) VALUES ('eq_barbell', 'Barbell') ON CONFLICT (id) DO NOTHING;

-- 3. Seed muscles
INSERT INTO exercise.muscles (id, name, body_part_id) VALUES ('muscle_abs', 'Abs', 'bp_waist') ON CONFLICT (id) DO NOTHING;
INSERT INTO exercise.muscles (id, name, body_part_id) VALUES ('muscle_hip_flexors', 'Hip Flexors', 'bp_waist') ON CONFLICT (id) DO NOTHING;
INSERT INTO exercise.muscles (id, name, body_part_id) VALUES ('muscle_lower_back', 'Lower Back', 'bp_waist') ON CONFLICT (id) DO NOTHING;
INSERT INTO exercise.muscles (id, name, body_part_id) VALUES ('muscle_obliques', 'Obliques', 'bp_waist') ON CONFLICT (id) DO NOTHING;
INSERT INTO exercise.muscles (id, name, body_part_id) VALUES ('muscle_quads', 'Quads', 'bp_upper_legs') ON CONFLICT (id) DO NOTHING;
INSERT INTO exercise.muscles (id, name, body_part_id) VALUES ('muscle_hamstrings', 'Hamstrings', 'bp_upper_legs') ON CONFLICT (id) DO NOTHING;
INSERT INTO exercise.muscles (id, name, body_part_id) VALUES ('muscle_glutes', 'Glutes', 'bp_upper_legs') ON CONFLICT (id) DO NOTHING;
INSERT INTO exercise.muscles (id, name, body_part_id) VALUES ('muscle_lats', 'Lats', 'bp_back') ON CONFLICT (id) DO NOTHING;
INSERT INTO exercise.muscles (id, name, body_part_id) VALUES ('muscle_biceps', 'Biceps', 'bp_upper_arms') ON CONFLICT (id) DO NOTHING;
INSERT INTO exercise.muscles (id, name, body_part_id) VALUES ('muscle_rhomboids', 'Rhomboids', 'bp_back') ON CONFLICT (id) DO NOTHING;
INSERT INTO exercise.muscles (id, name, body_part_id) VALUES ('muscle_calves', 'Calves', 'bp_lower_legs') ON CONFLICT (id) DO NOTHING;
INSERT INTO exercise.muscles (id, name, body_part_id) VALUES ('muscle_ankle_stabilizers', 'Ankle Stabilizers', 'bp_lower_legs') ON CONFLICT (id) DO NOTHING;
INSERT INTO exercise.muscles (id, name, body_part_id) VALUES ('muscle_forearms', 'Forearms', 'bp_lower_arms') ON CONFLICT (id) DO NOTHING;
INSERT INTO exercise.muscles (id, name, body_part_id) VALUES ('muscle_pectorals', 'Pectorals', 'bp_chest') ON CONFLICT (id) DO NOTHING;
INSERT INTO exercise.muscles (id, name, body_part_id) VALUES ('muscle_triceps', 'Triceps', 'bp_upper_arms') ON CONFLICT (id) DO NOTHING;
INSERT INTO exercise.muscles (id, name, body_part_id) VALUES ('muscle_shoulders', 'Shoulders', 'bp_chest') ON CONFLICT (id) DO NOTHING;
INSERT INTO exercise.muscles (id, name, body_part_id) VALUES ('muscle_core', 'Core', 'bp_chest') ON CONFLICT (id) DO NOTHING;
INSERT INTO exercise.muscles (id, name, body_part_id) VALUES ('muscle_back', 'Back', 'bp_waist') ON CONFLICT (id) DO NOTHING;
INSERT INTO exercise.muscles (id, name, body_part_id) VALUES ('muscle_quadriceps', 'Quadriceps', 'bp_upper_legs') ON CONFLICT (id) DO NOTHING;
INSERT INTO exercise.muscles (id, name, body_part_id) VALUES ('muscle_adductors', 'Adductors', 'bp_upper_legs') ON CONFLICT (id) DO NOTHING;
INSERT INTO exercise.muscles (id, name, body_part_id) VALUES ('muscle_chest', 'Chest', 'bp_upper_arms') ON CONFLICT (id) DO NOTHING;
INSERT INTO exercise.muscles (id, name, body_part_id) VALUES ('muscle_cardiovascular_system', 'Cardiovascular System', 'bp_cardio') ON CONFLICT (id) DO NOTHING;
INSERT INTO exercise.muscles (id, name, body_part_id) VALUES ('muscle_spine', 'Spine', 'bp_back') ON CONFLICT (id) DO NOTHING;
INSERT INTO exercise.muscles (id, name, body_part_id) VALUES ('muscle_upper_back', 'Upper Back', 'bp_back') ON CONFLICT (id) DO NOTHING;
INSERT INTO exercise.muscles (id, name, body_part_id) VALUES ('muscle_rear_deltoids', 'Rear Deltoids', 'bp_back') ON CONFLICT (id) DO NOTHING;
INSERT INTO exercise.muscles (id, name, body_part_id) VALUES ('muscle_delts', 'Delts', 'bp_shoulders') ON CONFLICT (id) DO NOTHING;
INSERT INTO exercise.muscles (id, name, body_part_id) VALUES ('muscle_traps', 'Traps', 'bp_back') ON CONFLICT (id) DO NOTHING;
INSERT INTO exercise.muscles (id, name, body_part_id) VALUES ('muscle_trapezius', 'Trapezius', 'bp_shoulders') ON CONFLICT (id) DO NOTHING;
INSERT INTO exercise.muscles (id, name, body_part_id) VALUES ('muscle_ankles', 'Ankles', 'bp_lower_legs') ON CONFLICT (id) DO NOTHING;
INSERT INTO exercise.muscles (id, name, body_part_id) VALUES ('muscle_feet', 'Feet', 'bp_lower_legs') ON CONFLICT (id) DO NOTHING;

-- 4. Seed tags
INSERT INTO exercise.tags (id, name) VALUES ('tag_category_waist', 'Category: Waist') ON CONFLICT (id) DO NOTHING;
INSERT INTO exercise.tags (id, name) VALUES ('tag_synergist_hip_flexors', 'Synergist: Hip Flexors') ON CONFLICT (id) DO NOTHING;
INSERT INTO exercise.tags (id, name) VALUES ('tag_synergist_obliques', 'Synergist: Obliques') ON CONFLICT (id) DO NOTHING;
INSERT INTO exercise.tags (id, name) VALUES ('tag_category_upper_legs', 'Category: Upper Legs') ON CONFLICT (id) DO NOTHING;
INSERT INTO exercise.tags (id, name) VALUES ('tag_synergist_hamstrings', 'Synergist: Hamstrings') ON CONFLICT (id) DO NOTHING;
INSERT INTO exercise.tags (id, name) VALUES ('tag_category_back', 'Category: Back') ON CONFLICT (id) DO NOTHING;
INSERT INTO exercise.tags (id, name) VALUES ('tag_synergist_biceps', 'Synergist: Biceps') ON CONFLICT (id) DO NOTHING;
INSERT INTO exercise.tags (id, name) VALUES ('tag_category_lower_legs', 'Category: Lower Legs') ON CONFLICT (id) DO NOTHING;
INSERT INTO exercise.tags (id, name) VALUES ('tag_synergist_ankle_stabilizers', 'Synergist: Ankle Stabilizers') ON CONFLICT (id) DO NOTHING;
INSERT INTO exercise.tags (id, name) VALUES ('tag_category_chest', 'Category: Chest') ON CONFLICT (id) DO NOTHING;
INSERT INTO exercise.tags (id, name) VALUES ('tag_synergist_triceps', 'Synergist: Triceps') ON CONFLICT (id) DO NOTHING;
INSERT INTO exercise.tags (id, name) VALUES ('tag_synergist_shoulders', 'Synergist: Shoulders') ON CONFLICT (id) DO NOTHING;
INSERT INTO exercise.tags (id, name) VALUES ('tag_synergist_glutes', 'Synergist: Glutes') ON CONFLICT (id) DO NOTHING;
INSERT INTO exercise.tags (id, name) VALUES ('tag_synergist_quadriceps', 'Synergist: Quadriceps') ON CONFLICT (id) DO NOTHING;
INSERT INTO exercise.tags (id, name) VALUES ('tag_category_upper_arms', 'Category: Upper Arms') ON CONFLICT (id) DO NOTHING;
INSERT INTO exercise.tags (id, name) VALUES ('tag_synergist_chest', 'Synergist: Chest') ON CONFLICT (id) DO NOTHING;
INSERT INTO exercise.tags (id, name) VALUES ('tag_category_cardio', 'Category: Cardio') ON CONFLICT (id) DO NOTHING;
INSERT INTO exercise.tags (id, name) VALUES ('tag_synergist_calves', 'Synergist: Calves') ON CONFLICT (id) DO NOTHING;
INSERT INTO exercise.tags (id, name) VALUES ('tag_synergist_forearms', 'Synergist: Forearms') ON CONFLICT (id) DO NOTHING;
INSERT INTO exercise.tags (id, name) VALUES ('tag_synergist_lower_back', 'Synergist: Lower Back') ON CONFLICT (id) DO NOTHING;
INSERT INTO exercise.tags (id, name) VALUES ('tag_category_shoulders', 'Category: Shoulders') ON CONFLICT (id) DO NOTHING;
INSERT INTO exercise.tags (id, name) VALUES ('tag_synergist_traps', 'Synergist: Traps') ON CONFLICT (id) DO NOTHING;
INSERT INTO exercise.tags (id, name) VALUES ('tag_synergist_upper_back', 'Synergist: Upper Back') ON CONFLICT (id) DO NOTHING;
INSERT INTO exercise.tags (id, name) VALUES ('tag_category_lower_arms', 'Category: Lower Arms') ON CONFLICT (id) DO NOTHING;
INSERT INTO exercise.tags (id, name) VALUES ('tag_synergist_ankles', 'Synergist: Ankles') ON CONFLICT (id) DO NOTHING;
INSERT INTO exercise.tags (id, name) VALUES ('tag_synergist_trapezius', 'Synergist: Trapezius') ON CONFLICT (id) DO NOTHING;

-- ==========================================
-- 5. Seed 14 exercises with Postgres CTE
-- ==========================================

WITH inserted_ex AS (
    INSERT INTO exercise.exercises (
        id, name, body_part_id, equipment_id, target_muscle_id, instructions, thumbnail_url, media_url, video_url, difficulty, default_rest_seconds, status, has_ai_supported
    )
    SELECT
        gen_random_uuid()::text, v.name, v.body_part_id, v.equipment_id, v.target_muscle_id, v.instructions, v.thumbnail_url, v.media_url, v.video_url, v.difficulty, v.default_rest_seconds, v.status, v.has_ai_supported
    FROM (VALUES
        ('Bicep Curl', 'bp_upper_arms', 'eq_barbell', 'muscle_biceps', 'Cùi chỏ cố định sát sườn, không vung/lắc. Cột sống thẳng, không ngửa ra sau. Gập khuỷu tay đưa tạ lên đến khi góc khuỷu ≤ 80°, sau đó từ từ hạ tạ về vị trí ban đầu.', 'https://static.exercisedb.dev/media/Yza7XrQ.gif', 'https://static.exercisedb.dev/media/Yza7XrQ.gif', 'https://www.youtube.com/watch?v=in7PaeYlhrM', 'Beginner', 60, 'ACTIVE', FALSE),
        ('Lateral Raise', 'bp_shoulders', 'eq_barbell', 'muscle_delts', 'Nâng tạ sang hai bên đến khi cánh tay song song với sàn (góc dang 80-90°). Khuỷu tay hơi gập nhẹ (160-170°). Thân người giữ thẳng đứng.', 'https://static.exercisedb.dev/media/aHDy5O5.gif', 'https://static.exercisedb.dev/media/aHDy5O5.gif', 'https://www.youtube.com/watch?v=3VcKaXpzqRo', 'Intermediate', 60, 'ACTIVE', FALSE),
        ('Lunge', 'bp_upper_legs', 'eq_body_weight', 'muscle_quads', 'Bước một chân về phía trước và hạ thấp hông cho đến khi cả hai đầu gối đều gập góc khoảng 90 độ. Chân sau giữ gối gần chạm sàn, giữ thân người thẳng đứng.', 'https://static.exercisedb.dev/media/qBcKorM.gif', 'https://static.exercisedb.dev/media/qBcKorM.gif', 'https://www.youtube.com/watch?v=QOVaHwm-Q6U', 'Beginner', 60, 'ACTIVE', FALSE),
        ('Plank', 'bp_waist', 'eq_body_weight', 'muscle_abs', 'Chống cẳng tay và mũi chân xuống sàn, giữ toàn thân tạo thành một đường thẳng từ đầu đến gót chân. Gồng chặt cơ bụng và hông, không võng lưng.', 'https://static.exercisedb.dev/media/0630.gif', 'https://static.exercisedb.dev/media/0630.gif', 'https://www.youtube.com/watch?v=pSHjTRCQxIw', 'Beginner', 60, 'ACTIVE', FALSE),
        ('Pull-up', 'bp_back', 'eq_body_weight', 'muscle_lats', 'Treo người trên xà đơn với lòng bàn tay hướng về phía trước. Kéo người lên cho đến khi cằm vượt qua xà, sau đó hạ người xuống có kiểm soát.', 'https://static.exercisedb.dev/media/72BC5Za.gif', 'https://static.exercisedb.dev/media/72BC5Za.gif', 'https://www.youtube.com/watch?v=eGo4IYlbE5g', 'Intermediate', 90, 'ACTIVE', FALSE),
        ('Push-up', 'bp_chest', 'eq_body_weight', 'muscle_pectorals', 'Nằm sấp chống hai tay rộng hơn vai, thân người thẳng. Hạ người xuống sao cho ngực gần chạm sàn (khuỷu tay mở ~45° so với thân), sau đó đẩy người lên.', 'https://static.exercisedb.dev/media/A9qxk2F.gif', 'https://static.exercisedb.dev/media/A9qxk2F.gif', 'https://www.youtube.com/watch?v=IODxDxX7oi4', 'Beginner', 60, 'ACTIVE', FALSE),
        ('Shoulder Press', 'bp_shoulders', 'eq_barbell', 'muscle_delts', 'Giữ tạ ngang tầm vai, đẩy tạ qua đầu cho đến khi hai tay duỗi thẳng. Hạ tạ xuống ngang cằm/vai có kiểm soát, giữ cột sống trung tính.', 'https://static.exercisedb.dev/media/0705.gif', 'https://static.exercisedb.dev/media/0705.gif', 'https://www.youtube.com/watch?v=qEwKCR5JCog', 'Intermediate', 90, 'ACTIVE', FALSE),
        ('Sit-up', 'bp_waist', 'eq_body_weight', 'muscle_abs', 'Nằm ngửa trên sàn, co gối và chân đặt phẳng trên mặt đất. Gập cơ bụng nâng toàn bộ thân trên lên hướng về phía đùi, sau đó hạ người từ từ về vị trí ban đầu.', 'https://static.exercisedb.dev/media/2gPfomN.gif', 'https://static.exercisedb.dev/media/2gPfomN.gif', 'https://www.youtube.com/watch?v=jDwoBqPH0jk', 'Beginner', 60, 'ACTIVE', FALSE),
        ('Squat', 'bp_upper_legs', 'eq_body_weight', 'muscle_quads', 'Đứng rộng bằng vai, hạ hông xuống như ngồi ghế cho đến khi đùi song song với mặt sàn (góc gối ≤ 100°). Giữ lưng thẳng, đẩy qua gót chân để đứng dậy.', 'https://static.exercisedb.dev/media/W9pFVv1.gif', 'https://static.exercisedb.dev/media/W9pFVv1.gif', 'https://www.youtube.com/watch?v=ultWZbUMPL8', 'Beginner', 60, 'ACTIVE', FALSE),
        ('Barbell Bench Press', 'bp_chest', 'eq_barbell', 'muscle_pectorals', 'Nằm ngửa trên ghế bành, nắm xà barbell rộng hơn vai. Hạ xà xuống chạm nhẹ giữa ngực rồi đẩy xà lên thẳng tay.', 'https://static.exercisedb.dev/media/EIeI8Vf.gif', 'https://static.exercisedb.dev/media/EIeI8Vf.gif', 'https://www.youtube.com/watch?v=rT7DgCr-3pg', 'Intermediate', 90, 'ACTIVE', FALSE),
        ('Barbell Deadlift', 'bp_back', 'eq_barbell', 'muscle_lower_back', 'Đứng trước xà barbell, cúi hông và gập nhẹ gối nắm xà. Giữ lưng thẳng, đẩy chân và duỗi hông để kéo tạ lên đứng thẳng người.', 'https://static.exercisedb.dev/media/0032.gif', 'https://static.exercisedb.dev/media/0032.gif', 'https://www.youtube.com/watch?v=op9kVnSso6Q', 'Advanced', 120, 'ACTIVE', FALSE),
        ('Leg Press', 'bp_upper_legs', 'eq_leverage_machine', 'muscle_quads', 'Nồi vào máy Leg Press, đặt hai bàn chân lên bàn đạp rộng bằng vai. Tháo khóa an toàn, hạ bàn đạp xuống đến khi gối gập 90 độ rồi đẩy mạnh lên.', 'https://static.exercisedb.dev/media/0540.gif', 'https://static.exercisedb.dev/media/0540.gif', 'https://www.youtube.com/watch?v=IZxyjW7MPJQ', 'Beginner', 90, 'ACTIVE', FALSE),
        ('Triceps Dip', 'bp_upper_arms', 'eq_body_weight', 'muscle_triceps', 'Nắm hai tay vào xà kép, nâng thân người lên thẳng tay. Hạ thân người xuống bằng cách gập khuỷu tay cho đến khi cánh tay tạo góc 90 độ rồi đẩy người lên.', 'https://static.exercisedb.dev/media/0810.gif', 'https://static.exercisedb.dev/media/0810.gif', 'https://www.youtube.com/watch?v=2z8JmcrW-As', 'Intermediate', 60, 'ACTIVE', FALSE),
        ('Russian Twist', 'bp_waist', 'eq_body_weight', 'muscle_obliques', 'Nồi trên sàn, nhấc hai chân lên nhẹ và hơi ngả lưng ra sau. Xoay thân trên sang trái rồi sang phải, gồng chặt cơ hông/bụng.', 'https://static.exercisedb.dev/media/r7cT9YD.gif', 'https://static.exercisedb.dev/media/r7cT9YD.gif', 'https://www.youtube.com/watch?v=wkD8rjkodUI', 'Beginner', 60, 'ACTIVE', FALSE)
    ) AS v(name, body_part_id, equipment_id, target_muscle_id, instructions, thumbnail_url, media_url, video_url, difficulty, default_rest_seconds, status, has_ai_supported)
    WHERE NOT EXISTS (
        SELECT 1 FROM exercise.exercises e WHERE e.name = v.name
    )
    RETURNING id, name
),
sec_muscles AS (
    INSERT INTO exercise.exercise_secondary_muscles (exercise_id, muscle_id)
    SELECT i.id, m.muscle_id
    FROM inserted_ex i
    JOIN (VALUES
        ('Bicep Curl', 'muscle_forearms'),
        ('Lateral Raise', 'muscle_traps'),
        ('Lateral Raise', 'muscle_upper_back'),
        ('Lunge', 'muscle_glutes'),
        ('Lunge', 'muscle_hamstrings'),
        ('Plank', 'muscle_core'),
        ('Plank', 'muscle_lower_back'),
        ('Pull-up', 'muscle_biceps'),
        ('Pull-up', 'muscle_rhomboids'),
        ('Push-up', 'muscle_triceps'),
        ('Push-up', 'muscle_delts'),
        ('Shoulder Press', 'muscle_triceps'),
        ('Sit-up', 'muscle_hip_flexors'),
        ('Squat', 'muscle_glutes'),
        ('Squat', 'muscle_hamstrings'),
        ('Barbell Bench Press', 'muscle_triceps'),
        ('Barbell Bench Press', 'muscle_delts'),
        ('Barbell Deadlift', 'muscle_glutes'),
        ('Barbell Deadlift', 'muscle_hamstrings'),
        ('Leg Press', 'muscle_glutes'),
        ('Triceps Dip', 'muscle_pectorals'),
        ('Russian Twist', 'muscle_abs')
    ) AS m(name, muscle_id) ON i.name = m.name
    ON CONFLICT (exercise_id, muscle_id) DO NOTHING
),
ex_tags AS (
    INSERT INTO exercise.exercise_tags (exercise_id, tag_id)
    SELECT i.id, t.tag_id
    FROM inserted_ex i
    JOIN (VALUES
        ('Bicep Curl', 'tag_category_upper_arms'),
        ('Lateral Raise', 'tag_category_shoulders'),
        ('Lunge', 'tag_category_upper_legs'),
        ('Plank', 'tag_category_waist'),
        ('Pull-up', 'tag_category_back'),
        ('Push-up', 'tag_category_chest'),
        ('Shoulder Press', 'tag_category_shoulders'),
        ('Sit-up', 'tag_category_waist'),
        ('Squat', 'tag_category_upper_legs'),
        ('Barbell Bench Press', 'tag_category_chest'),
        ('Barbell Deadlift', 'tag_category_back'),
        ('Leg Press', 'tag_category_upper_legs'),
        ('Triceps Dip', 'tag_category_upper_arms'),
        ('Russian Twist', 'tag_category_waist')
    ) AS t(name, tag_id) ON i.name = t.name
    ON CONFLICT (exercise_id, tag_id) DO NOTHING
)
INSERT INTO exercise.motion_specifications (exercise_id, min_rom_percent, calibration_distance_min, calibration_distance_max, calibration_angle)
SELECT i.id, 70, 1.5, 2.0, 0.0
FROM inserted_ex i
WHERE i.name IN ('Bicep Curl', 'Lateral Raise', 'Lunge', 'Plank', 'Pull-up', 'Push-up', 'Shoulder Press', 'Sit-up', 'Squat')
ON CONFLICT (exercise_id) DO NOTHING;
