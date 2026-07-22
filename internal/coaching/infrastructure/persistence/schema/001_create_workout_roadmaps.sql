CREATE TABLE IF NOT EXISTS coaching_workout_roadmaps (
    id VARCHAR(36) PRIMARY KEY,
    user_id VARCHAR(36) NOT NULL,
    status VARCHAR(20) NOT NULL,
    planning_tier VARCHAR(20) NOT NULL,
    start_date TIMESTAMP WITH TIME ZONE NOT NULL,
    end_date TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_coaching_workout_roadmaps_user_id ON coaching_workout_roadmaps (user_id);
CREATE INDEX IF NOT EXISTS idx_coaching_workout_roadmaps_status ON coaching_workout_roadmaps (status);
