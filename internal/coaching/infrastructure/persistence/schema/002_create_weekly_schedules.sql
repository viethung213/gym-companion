CREATE TABLE IF NOT EXISTS coaching_weekly_schedules (
    id VARCHAR(36) PRIMARY KEY,
    roadmap_id VARCHAR(36) NOT NULL,
    user_id VARCHAR(36) NOT NULL,
    week_number INT NOT NULL,
    start_date TIMESTAMP WITH TIME ZONE NOT NULL,
    end_date TIMESTAMP WITH TIME ZONE NOT NULL,
    muscle_split_type VARCHAR(50) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_coaching_weekly_schedules_roadmap_id ON coaching_weekly_schedules (roadmap_id);
CREATE INDEX IF NOT EXISTS idx_coaching_weekly_schedules_user_id ON coaching_weekly_schedules (user_id);

CREATE TABLE IF NOT EXISTS coaching_schedule_days (
    id VARCHAR(36) PRIMARY KEY,
    schedule_id VARCHAR(36) NOT NULL,
    scheduled_date TIMESTAMP WITH TIME ZONE NOT NULL,
    day_of_week INT NOT NULL,
    status VARCHAR(20) NOT NULL,
    target_muscle_groups JSONB NOT NULL,
    daily_workout_plan_id VARCHAR(36) NOT NULL,
    time_window VARCHAR(50) NOT NULL,
    planned_duration_minutes INT NOT NULL,
    CONSTRAINT fk_coaching_schedule_days_schedule_id FOREIGN KEY (schedule_id) REFERENCES coaching_weekly_schedules(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_coaching_schedule_days_schedule_id ON coaching_schedule_days (schedule_id);
CREATE INDEX IF NOT EXISTS idx_coaching_schedule_days_scheduled_date ON coaching_schedule_days (scheduled_date);
