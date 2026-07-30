package repository

import (
	"context"
	"time"

	"github.com/viethung213/gym-companion/internal/workout_execution/domain/aggregate"
)

// WorkoutSessionRepository defines persistence operations for WorkoutSession aggregate.

type WorkoutSessionRepository interface {
	Save(ctx context.Context, session *aggregate.WorkoutSession) error

	FindByID(ctx context.Context, id string) (*aggregate.WorkoutSession, error)

	FindActiveSessionByUserID(ctx context.Context, userID string) (*aggregate.WorkoutSession, error)

	FindTimedOutSessions(ctx context.Context, maxDurationMinutes int) ([]*aggregate.WorkoutSession, error)

	FindHistoryByUserID(ctx context.Context, userID string, limit, offset int) ([]*aggregate.WorkoutSession, error)

	// FindSessionsWithCriticalInactivity returns IN_PROGRESS sessions that

	// have at least one critical posture error and have not been updated

	// within the given inactivity threshold.

	FindSessionsWithCriticalInactivity(ctx context.Context, inactivityThreshold time.Duration) ([]*aggregate.WorkoutSession, error)
}

// PersonalRecordRepository defines persistence operations for PersonalRecord aggregate.

type PersonalRecordRepository interface {
	Save(ctx context.Context, pr *aggregate.PersonalRecord) error

	FindByUserIDAndExerciseID(ctx context.Context, userID, exerciseID string) (*aggregate.PersonalRecord, error)

	FindByUserIDAndExerciseIDs(ctx context.Context, userID string, exerciseIDs []string) ([]*aggregate.PersonalRecord, error)
}

// MotionSpecificationRepository defines persistence operations for MotionSpecification aggregate.

type MotionSpecificationRepository interface {
	Save(ctx context.Context, spec *aggregate.MotionSpecification) error

	FindByExerciseID(ctx context.Context, exerciseID string) (*aggregate.MotionSpecification, error)

	Delete(ctx context.Context, exerciseID string) error

	List(ctx context.Context, limit, offset int) ([]*aggregate.MotionSpecification, int, error)
}
