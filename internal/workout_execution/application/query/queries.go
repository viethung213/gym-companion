package query

import (
	"context"
	"fmt"
	"time"

	"github.com/viethung213/gym-companion/internal/workout_execution/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/derror"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/repository"
)

// MotionSpecificationDTO represents read-only motion specification data for query responses.
type MotionSpecificationDTO struct {
	ExerciseID             string    `json:"exerciseId"`
	OnnxDetectorURL        string    `json:"onnxDetectorUrl"`
	OnnxSkeletonURL        string    `json:"onnxSkeletonUrl"`
	LocalRulesURL          string    `json:"localRulesUrl"`
	DialogueEngineURL      string    `json:"dialogueEngineUrl"`
	RecommendedCameraAngle string    `json:"recommendedCameraAngle"`
	IsReady                bool      `json:"isReady"`
	CreatedAt              time.Time `json:"createdAt"`
	UpdatedAt              time.Time `json:"updatedAt"`
}

// GetMotionSpecificationQueryHandler handles fetching motion spec config.
type GetMotionSpecificationQueryHandler struct {
	motionRepo repository.MotionSpecificationRepository
}

// NewGetMotionSpecificationQueryHandler constructs handler.
func NewGetMotionSpecificationQueryHandler(motionRepo repository.MotionSpecificationRepository) *GetMotionSpecificationQueryHandler {
	return &GetMotionSpecificationQueryHandler{
		motionRepo: motionRepo,
	}
}

// Handle executes query.
func (h *GetMotionSpecificationQueryHandler) Handle(ctx context.Context, exerciseID, _ string) (*MotionSpecificationDTO, error) {
	if exerciseID == "" {
		return nil, derror.ErrMotionSpecNotFound
	}

	spec, err := h.motionRepo.FindByExerciseID(ctx, exerciseID)
	if err != nil {
		return nil, fmt.Errorf("fetch motion spec: %w", err)
	}

	if spec == nil {
		return nil, derror.ErrMotionSpecNotFound
	}

	return &MotionSpecificationDTO{
		ExerciseID:             spec.ExerciseID(),
		OnnxDetectorURL:        spec.OnnxDetectorURL(),
		OnnxSkeletonURL:        spec.OnnxSkeletonURL(),
		LocalRulesURL:          spec.LocalRulesURL(),
		DialogueEngineURL:      spec.DialogueEngineURL(),
		RecommendedCameraAngle: spec.RecommendedCameraAngle(),
		IsReady:                spec.IsReady(),
		CreatedAt:              spec.CreatedAt(),
		UpdatedAt:              spec.UpdatedAt(),
	}, nil
}

// GetPersonalRecordsQueryHandler handles fetching PR records.
type GetPersonalRecordsQueryHandler struct {
	prRepo repository.PersonalRecordRepository
}

// NewGetPersonalRecordsQueryHandler constructs handler.
func NewGetPersonalRecordsQueryHandler(prRepo repository.PersonalRecordRepository) *GetPersonalRecordsQueryHandler {
	return &GetPersonalRecordsQueryHandler{
		prRepo: prRepo,
	}
}

// Handle executes query.
func (h *GetPersonalRecordsQueryHandler) Handle(ctx context.Context, userID string, exerciseIDs []string) ([]*aggregate.PersonalRecord, error) {
	if userID == "" {
		return nil, derror.ErrPersonalRecordNotFound
	}

	records, err := h.prRepo.FindByUserIDAndExerciseIDs(ctx, userID, exerciseIDs)
	if err != nil {
		return nil, fmt.Errorf("fetch personal records: %w", err)
	}

	return records, nil
}

// GetWorkoutSessionErrorsQueryHandler handles fetching posture errors.
type GetWorkoutSessionErrorsQueryHandler struct {
	sessionRepo repository.WorkoutSessionRepository
}

// NewGetWorkoutSessionErrorsQueryHandler constructs handler.
func NewGetWorkoutSessionErrorsQueryHandler(sessionRepo repository.WorkoutSessionRepository) *GetWorkoutSessionErrorsQueryHandler {
	return &GetWorkoutSessionErrorsQueryHandler{
		sessionRepo: sessionRepo,
	}
}

// Handle executes query.
func (h *GetWorkoutSessionErrorsQueryHandler) Handle(ctx context.Context, sessionID, userID string) ([]aggregate.SessionError, error) {
	session, err := h.sessionRepo.FindByID(ctx, sessionID)
	if err != nil || session == nil {
		return nil, derror.ErrWorkoutSessionNotFound
	}

	if userID != "" && session.UserID() != userID {
		return nil, derror.ErrForbidden
	}

	return session.Errors(), nil
}

// GetWorkoutHistoryQueryHandler handles fetching session history.
type GetWorkoutHistoryQueryHandler struct {
	sessionRepo repository.WorkoutSessionRepository
}

// NewGetWorkoutHistoryQueryHandler constructs handler.
func NewGetWorkoutHistoryQueryHandler(sessionRepo repository.WorkoutSessionRepository) *GetWorkoutHistoryQueryHandler {
	return &GetWorkoutHistoryQueryHandler{
		sessionRepo: sessionRepo,
	}
}

// Handle executes query.
func (h *GetWorkoutHistoryQueryHandler) Handle(ctx context.Context, userID string, limit, offset int) ([]*aggregate.WorkoutSession, error) {
	if limit <= 0 {
		limit = 10
	}
	if offset < 0 {
		offset = 0
	}

	sessions, err := h.sessionRepo.FindHistoryByUserID(ctx, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("fetch workout history: %w", err)
	}

	return sessions, nil
}
