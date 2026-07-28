package command

import (
	"context"
	"fmt"

	"github.com/viethung213/gym-companion/internal/workout_execution/application/apperror"
	"github.com/viethung213/gym-companion/internal/workout_execution/application/port"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/derror"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/repository"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/service"
)

// CompleteWorkoutSessionCommand parameters.
type CompleteWorkoutSessionCommand struct {
	SessionID       string
	ConfirmOverload bool
	WeightUpdateKg  *float32
}

// CompleteWorkoutSessionResult result.
type CompleteWorkoutSessionResult struct {
	SessionID        string
	CompletedAt      string
	TotalSets        int
	TotalVolume      float32
	AverageFormScore *float32
	AverageRPE       float32
}

// CompleteWorkoutSessionHandler handles session completion.
type CompleteWorkoutSessionHandler struct {
	sessionRepo    repository.WorkoutSessionRepository
	loadGuard      *service.TrainingLoadGuard
	exerciseClient port.ExerciseCatalogClient
	userClient     port.UserProfileClient
	outbox         port.OutboxWriter
	txManager      port.TxManager
}

// NewCompleteWorkoutSessionHandler constructs CompleteWorkoutSessionHandler.
func NewCompleteWorkoutSessionHandler(
	sessionRepo repository.WorkoutSessionRepository,
	loadGuard *service.TrainingLoadGuard,
	exerciseClient port.ExerciseCatalogClient,
	userClient port.UserProfileClient,
	outbox port.OutboxWriter,
	txManager port.TxManager,
) *CompleteWorkoutSessionHandler {
	return &CompleteWorkoutSessionHandler{
		sessionRepo:    sessionRepo,
		loadGuard:      loadGuard,
		exerciseClient: exerciseClient,
		userClient:     userClient,
		outbox:         outbox,
		txManager:      txManager,
	}
}

// Handle executes session completion.
func (h *CompleteWorkoutSessionHandler) Handle(ctx context.Context, cmd CompleteWorkoutSessionCommand) (*CompleteWorkoutSessionResult, error) {
	if cmd.SessionID == "" {
		return nil, apperror.ErrInvalidInput
	}

	session, err := h.sessionRepo.FindByID(ctx, cmd.SessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch session: %w", err)
	}
	if session == nil {
		return nil, derror.ErrWorkoutSessionNotFound
	}

	// 1. Check Training Load Overload
	var isOverloaded bool
	if h.loadGuard != nil && len(session.Sets()) > 0 {
		firstExerciseID := session.Sets()[0].ExerciseID
		muscleGroup := "Chest" // Default fallback
		if h.exerciseClient != nil {
			mg, err := h.exerciseClient.GetExerciseMuscleGroup(ctx, firstExerciseID)
			if err == nil && mg != "" {
				muscleGroup = mg
			}
		}

		vol := session.CalculateTotalVolume()
		overloaded, _, err := h.loadGuard.IsOverloaded(ctx, session.UserID(), muscleGroup, vol)
		if err == nil {
			isOverloaded = overloaded
		}
	}

	// 2. Transition domain state to COMPLETED
	if err := session.Complete(cmd.ConfirmOverload, isOverloaded); err != nil {
		return nil, err
	}

	// 3. User Weight Update (UC-03.4 Alternative Flow A3)
	if cmd.WeightUpdateKg != nil && h.userClient != nil {
		_ = h.userClient.UpdateBodyWeight(ctx, session.UserID(), *cmd.WeightUpdateKg)
	}

	// 4. Save and Publish Outbox Events
	summary := session.CalculateSummary()
	err = h.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		if err := h.sessionRepo.Save(txCtx, session); err != nil {
			return err
		}
		events := session.PopEvents()
		if len(events) > 0 && h.outbox != nil {
			if err := h.outbox.WriteEvents(txCtx, "WorkoutSession", session.ID(), events); err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to complete session tx: %w", err)
	}

	var completedAtStr string
	if session.EndedAt() != nil {
		completedAtStr = session.EndedAt().Format("2006-01-02T15:04:05Z07:00")
	}

	return &CompleteWorkoutSessionResult{
		SessionID:        session.ID(),
		CompletedAt:      completedAtStr,
		TotalSets:        summary.TotalSets,
		TotalVolume:      summary.TotalVolume,
		AverageFormScore: summary.AverageFormScore,
		AverageRPE:       summary.AverageRPE,
	}, nil
}
