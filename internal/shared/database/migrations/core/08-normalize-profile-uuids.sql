-- Normalize legacy enum strings in profile.users and profile.injuries to real UUIDs
-- from exercise.equipments and exercise.muscles. Idempotent: already-valid UUIDs are preserved.

-- 1. Normalize profile.users.available_equipment (JSONB array of strings)
UPDATE profile.users u
SET available_equipment = COALESCE(
    (
        SELECT jsonb_agg(
            COALESCE(
                (
                    SELECT e.id::text
                    FROM exercise.equipments e
                    WHERE LOWER(e.name) = LOWER(
                        CASE elem.val
                            WHEN 'BODYWEIGHT' THEN 'Body Weight'
                            WHEN 'BODY_WEIGHT' THEN 'Body Weight'
                            WHEN 'RESISTANCE_BAND' THEN 'Resistance Band'
                            WHEN 'PULL_UP_BAR' THEN 'Pull-up Bar'
                            ELSE REPLACE(elem.val, '_', ' ')
                        END
                    )
                    LIMIT 1
                ),
                elem.val
            )
        )
        FROM jsonb_array_elements_text(u.available_equipment) AS elem(val)
    ),
    '[]'::jsonb
)
WHERE (CASE WHEN jsonb_typeof(u.available_equipment) = 'array' THEN jsonb_array_length(u.available_equipment) ELSE 0 END) > 0;

-- 2. Normalize profile.users.preferred_muscle_groups (JSONB array of strings)
UPDATE profile.users u
SET preferred_muscle_groups = COALESCE(
    (
        SELECT jsonb_agg(
            COALESCE(
                (
                    SELECT m.id::text
                    FROM exercise.muscles m
                    WHERE LOWER(m.name) = LOWER(REPLACE(elem.val, '_', ' '))
                    LIMIT 1
                ),
                elem.val
            )
        )
        FROM jsonb_array_elements_text(u.preferred_muscle_groups) AS elem(val)
    ),
    '[]'::jsonb
)
WHERE (CASE WHEN jsonb_typeof(u.preferred_muscle_groups) = 'array' THEN jsonb_array_length(u.preferred_muscle_groups) ELSE 0 END) > 0;

-- 3. Normalize profile.injuries.muscle_group (VARCHAR)
UPDATE profile.injuries i
SET muscle_group = m.id::text
FROM exercise.muscles m
WHERE LOWER(m.name) = LOWER(REPLACE(i.muscle_group, '_', ' '))
  AND i.muscle_group !~* '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$';
