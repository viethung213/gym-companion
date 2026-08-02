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
	WeightUpdateKg  *float32 // optional: nil nếu người dùng không cập nhật cân nặng mới
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
	outbox         port.OutboxWriter
	txManager      port.TxManager
}

// NewCompleteWorkoutSessionHandler constructs CompleteWorkoutSessionHandler.
func NewCompleteWorkoutSessionHandler(
	sessionRepo repository.WorkoutSessionRepository,
	loadGuard *service.TrainingLoadGuard,
	exerciseClient port.ExerciseCatalogClient,
	outbox port.OutboxWriter,
	txManager port.TxManager,
) *CompleteWorkoutSessionHandler {
	return &CompleteWorkoutSessionHandler{
		sessionRepo:    sessionRepo,
		loadGuard:      loadGuard,
		exerciseClient: exerciseClient,
		outbox:         outbox,
		txManager:      txManager,
	}
}

// Handle executes session completion.
func (h *CompleteWorkoutSessionHandler) Handle(
	ctx context.Context,
	cmd CompleteWorkoutSessionCommand,
) (*CompleteWorkoutSessionResult, error) {
	if cmd.SessionID == "" {
		return nil, apperror.ErrInvalidInput
	}

	var result *CompleteWorkoutSessionResult
	saveFunc := func(txCtx context.Context) error {
		session, err := h.sessionRepo.FindByIDForUpdate(txCtx, cmd.SessionID)
		if err != nil {
			return fmt.Errorf("failed to fetch session: %w", err)
		}
		if session == nil {
			return derror.ErrWorkoutSessionNotFound
		}

		// 1. Check Training Load Overload
		var isOverloaded bool
		if h.loadGuard != nil && len(session.Sets()) > 0 {
			firstExerciseID := session.Sets()[0].ExerciseID
			muscleGroup := "Chest" // Default fallback
			if h.exerciseClient != nil {
				mg, clientErr := h.exerciseClient.GetExerciseMuscleGroup(txCtx, firstExerciseID)
				if clientErr == nil && mg != "" {
					muscleGroup = mg
				}
			}

			vol := session.CalculateTotalVolume()
			overloaded, _, guardErr := h.loadGuard.IsOverloaded(txCtx, session.UserID(), muscleGroup, vol)
			if guardErr == nil {
				isOverloaded = overloaded
			}
		}

		// 2. Transition domain state to COMPLETED
		if completeErr := session.Complete(cmd.ConfirmOverload, isOverloaded); completeErr != nil {
			return completeErr
		}

		// 3. Record optional BodyMetricUpdated event if weight update was provided by user (UC-03.4 A3)
		if cmd.WeightUpdateKg != nil && *cmd.WeightUpdateKg > 0 {
			session.RecordBodyMetricUpdate(*cmd.WeightUpdateKg)
		}

		// 4. Save and Publish Outbox Events
		summary := session.CalculateSummary()
		if err := h.sessionRepo.Save(txCtx, session); err != nil {
			return err
		}
		events := session.PopEvents()
		if len(events) > 0 && h.outbox != nil {
			if err := h.outbox.WriteEvents(txCtx, "WorkoutSession", session.ID(), events); err != nil {
				return err
			}
		}

		var completedAtStr string
		if session.EndedAt() != nil {
			completedAtStr = session.EndedAt().Format("2006-01-02T15:04:05Z07:00")
		}

		result = &CompleteWorkoutSessionResult{
			SessionID:        session.ID(),
			CompletedAt:      completedAtStr,
			TotalSets:        summary.TotalSets,
			TotalVolume:      summary.TotalVolume,
			AverageFormScore: summary.AverageFormScore,
			AverageRPE:       summary.AverageRPE,
		}
		return nil
	}

	if h.txManager != nil {
		if err := h.txManager.WithTransaction(ctx, saveFunc); err != nil {
			return nil, fmt.Errorf("failed to complete session tx: %w", err)
		}
	} else {
		if err := saveFunc(ctx); err != nil {
			return nil, err
		}
	}

	return result, nil
}
