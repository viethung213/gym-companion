-- Seed workout_execution.motion_specifications for the 12 AI-supported exercises
INSERT INTO workout_execution.motion_specifications (
    exercise_id, exercise_name, onnx_detector_url, onnx_skeleton_url, local_rules_url, dialogue_engine_url, recommended_camera_angle, is_ready
)
SELECT
    e.id,
    e.name,
    '/models/yolo11n-pose.onnx',
    '/models/yolo11n-pose.onnx',
    'rules/' || CASE e.name
        WHEN 'Dumbbell Biceps Curl' THEN 'bicep_curl'
        WHEN 'Bicep Curl' THEN 'bicep_curl'
        WHEN 'Dumbbell Lateral Raise' THEN 'lateral_raise'
        WHEN 'Lateral Raise' THEN 'lateral_raise'
        WHEN 'Barbell Lunge' THEN 'lunge'
        WHEN 'Dumbbell Lunge' THEN 'lunge'
        WHEN 'Lunge' THEN 'lunge'
        WHEN 'Weighted Front Plank' THEN 'plank'
        WHEN 'Front Plank with Twist' THEN 'plank'
        WHEN 'Plank' THEN 'plank'
        WHEN 'Pull-up' THEN 'pull_up'
        WHEN 'Push-up' THEN 'push_up'
        WHEN 'Dumbbell Seated Shoulder Press' THEN 'shoulder_press'
        WHEN 'Cable Shoulder Press' THEN 'shoulder_press'
        WHEN 'Shoulder Press' THEN 'shoulder_press'
        WHEN '3/4 Sit-up' THEN 'sit_up'
        WHEN 'Sit-up' THEN 'sit_up'
        WHEN 'Weighted Squat' THEN 'squat'
        WHEN 'Barbell Full Squat' THEN 'squat'
        WHEN 'Squat' THEN 'squat'
        WHEN 'Barbell Bench Press' THEN 'bench_press'
        WHEN 'Barbell Deadlift' THEN 'deadlift'
        WHEN 'Triceps Dip' THEN 'triceps_dip'
    END || '_rule.json',
    'dialogues/' || CASE e.name
        WHEN 'Dumbbell Biceps Curl' THEN 'bicep_curl'
        WHEN 'Bicep Curl' THEN 'bicep_curl'
        WHEN 'Dumbbell Lateral Raise' THEN 'lateral_raise'
        WHEN 'Lateral Raise' THEN 'lateral_raise'
        WHEN 'Barbell Lunge' THEN 'lunge'
        WHEN 'Dumbbell Lunge' THEN 'lunge'
        WHEN 'Lunge' THEN 'lunge'
        WHEN 'Weighted Front Plank' THEN 'plank'
        WHEN 'Front Plank with Twist' THEN 'plank'
        WHEN 'Plank' THEN 'plank'
        WHEN 'Pull-up' THEN 'pull_up'
        WHEN 'Push-up' THEN 'push_up'
        WHEN 'Dumbbell Seated Shoulder Press' THEN 'shoulder_press'
        WHEN 'Cable Shoulder Press' THEN 'shoulder_press'
        WHEN 'Shoulder Press' THEN 'shoulder_press'
        WHEN '3/4 Sit-up' THEN 'sit_up'
        WHEN 'Sit-up' THEN 'sit_up'
        WHEN 'Weighted Squat' THEN 'squat'
        WHEN 'Barbell Full Squat' THEN 'squat'
        WHEN 'Squat' THEN 'squat'
        WHEN 'Barbell Bench Press' THEN 'bench_press'
        WHEN 'Barbell Deadlift' THEN 'deadlift'
        WHEN 'Triceps Dip' THEN 'triceps_dip'
    END || '_dialogue.json',
    CASE
        WHEN e.name IN ('Shoulder Press', 'Dumbbell Seated Shoulder Press', 'Cable Shoulder Press', 'Pull-up', 'Lateral Raise', 'Dumbbell Lateral Raise') THEN 'FRONT'
        ELSE 'SIDE'
    END,
    TRUE
FROM exercise.exercises e
WHERE e.name IN (
    'Dumbbell Biceps Curl', 'Bicep Curl',
    'Dumbbell Lateral Raise', 'Lateral Raise',
    'Barbell Lunge', 'Dumbbell Lunge', 'Lunge',
    'Weighted Front Plank', 'Front Plank with Twist', 'Plank',
    'Pull-up',
    'Push-up',
    'Dumbbell Seated Shoulder Press', 'Cable Shoulder Press', 'Shoulder Press',
    '3/4 Sit-up', 'Sit-up',
    'Weighted Squat', 'Barbell Full Squat', 'Squat',
    'Barbell Bench Press',
    'Barbell Deadlift',
    'Triceps Dip'
)
ON CONFLICT (exercise_id) DO UPDATE SET
    exercise_name = EXCLUDED.exercise_name,
    onnx_detector_url = EXCLUDED.onnx_detector_url,
    onnx_skeleton_url = EXCLUDED.onnx_skeleton_url,
    local_rules_url = EXCLUDED.local_rules_url,
    dialogue_engine_url = EXCLUDED.dialogue_engine_url,
    recommended_camera_angle = EXCLUDED.recommended_camera_angle,
    is_ready = TRUE;

-- Automatically sync has_ai_supported flag for exercises backed by motion specifications
UPDATE exercise.exercises
SET has_ai_supported = TRUE
WHERE id IN (SELECT exercise_id FROM workout_execution.motion_specifications);
