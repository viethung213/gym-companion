-- Seed workout_execution.motion_specifications for the 12 AI-supported exercises
INSERT INTO workout_execution.motion_specifications (
    exercise_id, exercise_name, onnx_detector_url, onnx_skeleton_url, local_rules_url, dialogue_engine_url, recommended_camera_angle, is_ready
)
SELECT
    e.id,
    e.name,
    '/models/yolox_nano.onnx',
    '/models/rtmpose_tiny.onnx',
    'rules/' || CASE e.name
        WHEN 'Bicep Curl' THEN 'bicep_curl'
        WHEN 'Lateral Raise' THEN 'lateral_raise'
        WHEN 'Lunge' THEN 'lunge'
        WHEN 'Plank' THEN 'plank'
        WHEN 'Pull-up' THEN 'pull_up'
        WHEN 'Push-up' THEN 'push_up'
        WHEN 'Shoulder Press' THEN 'shoulder_press'
        WHEN 'Sit-up' THEN 'sit_up'
        WHEN 'Squat' THEN 'squat'
        WHEN 'Barbell Bench Press' THEN 'bench_press'
        WHEN 'Barbell Deadlift' THEN 'deadlift'
        WHEN 'Triceps Dip' THEN 'triceps_dip'
    END || '_rule.json',
    'dialogues/' || CASE e.name
        WHEN 'Bicep Curl' THEN 'bicep_curl'
        WHEN 'Lateral Raise' THEN 'lateral_raise'
        WHEN 'Lunge' THEN 'lunge'
        WHEN 'Plank' THEN 'plank'
        WHEN 'Pull-up' THEN 'pull_up'
        WHEN 'Push-up' THEN 'push_up'
        WHEN 'Shoulder Press' THEN 'shoulder_press'
        WHEN 'Sit-up' THEN 'sit_up'
        WHEN 'Squat' THEN 'squat'
        WHEN 'Barbell Bench Press' THEN 'bench_press'
        WHEN 'Barbell Deadlift' THEN 'deadlift'
        WHEN 'Triceps Dip' THEN 'triceps_dip'
    END || '_dialogue.json',
    CASE
        WHEN e.name IN ('Shoulder Press', 'Pull-up', 'Lateral Raise') THEN 'FRONT'
        ELSE 'SIDE'
    END,
    TRUE
FROM exercise.exercises e
WHERE e.name IN ('Bicep Curl', 'Lateral Raise', 'Lunge', 'Plank', 'Pull-up', 'Push-up', 'Shoulder Press', 'Sit-up', 'Squat', 'Barbell Bench Press', 'Barbell Deadlift', 'Triceps Dip')
ON CONFLICT (exercise_id) DO UPDATE SET
    exercise_name = EXCLUDED.exercise_name,
    local_rules_url = EXCLUDED.local_rules_url,
    dialogue_engine_url = EXCLUDED.dialogue_engine_url,
    recommended_camera_angle = EXCLUDED.recommended_camera_angle,
    is_ready = TRUE;
