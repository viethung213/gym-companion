package command

import (
	"context"
	"fmt"
	"time"

	"github.com/viethung213/gym-companion/internal/workout_execution/application/apperror"
	"github.com/viethung213/gym-companion/internal/workout_execution/application/port"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/derror"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/repository"
)

// ShareWorkoutSessionCommand defines parameters for sharing a completed workout session.
type ShareWorkoutSessionCommand struct {
	SessionID  string
	UserID     string
	Caption    string
	MediaURLs  []string
	Visibility string
}

// ShareWorkoutSessionResult contains outcome data of session sharing.
type ShareWorkoutSessionResult struct {
	SessionID string
	IsShared  bool
	SharedAt  time.Time
}

// ShareWorkoutSessionHandler coordinates the business logic for sharing a workout session.
type ShareWorkoutSessionHandler struct {
	sessionRepo repository.WorkoutSessionRepository
	motionRepo  repository.MotionSpecificationRepository
	outbox      port.OutboxWriter
	txManager   port.TxManager
}

// NewShareWorkoutSessionHandler constructs a new ShareWorkoutSessionHandler.
func NewShareWorkoutSessionHandler(
	sessionRepo repository.WorkoutSessionRepository,
	motionRepo repository.MotionSpecificationRepository,
	outbox port.OutboxWriter,
	txManager port.TxManager,
) *ShareWorkoutSessionHandler {
	return &ShareWorkoutSessionHandler{
		sessionRepo: sessionRepo,
		motionRepo:  motionRepo,
		outbox:      outbox,
		txManager:   txManager,
	}
}

// Handle processes the share workout session command.
func (h *ShareWorkoutSessionHandler) Handle(
	ctx context.Context,
	cmd ShareWorkoutSessionCommand,
) (*ShareWorkoutSessionResult, error) {
	if cmd.SessionID == "" {
		return nil, apperror.ErrInvalidInput
	}

	var result *ShareWorkoutSessionResult
	shareFunc := func(txCtx context.Context) error {
		session, err := h.sessionRepo.FindByIDForUpdate(txCtx, cmd.SessionID)
		if err != nil {
			return fmt.Errorf("failed to fetch session: %w", err)
		}
		if session == nil {
			return derror.ErrWorkoutSessionNotFound
		}
		if cmd.UserID != "" && session.UserID() != cmd.UserID {
			return derror.ErrForbidden
		}

		workoutTitle := ""
		exerciseCount := int32(0)
		sets := session.Sets()
		if len(sets) > 0 {
			var uniqueExerciseIDs []string
			seen := make(map[string]bool)
			for _, set := range sets {
				if set.ExerciseID != "" && !seen[set.ExerciseID] {
					seen[set.ExerciseID] = true
					uniqueExerciseIDs = append(uniqueExerciseIDs, set.ExerciseID)
				}
			}

			exerciseCount = int32(len(uniqueExerciseIDs))
			if exerciseCount > 0 {
				firstExID := uniqueExerciseIDs[0]
				firstName := firstExID
				if h.motionRepo != nil {
					spec, specErr := h.motionRepo.FindByExerciseID(txCtx, firstExID)
					if specErr == nil && spec != nil && spec.ExerciseName() != "" {
						firstName = spec.ExerciseName()
					}
				}

				if exerciseCount == 1 {
					workoutTitle = firstName
				} else {
					workoutTitle = fmt.Sprintf("%s + %d bài khác", firstName, exerciseCount-1)
				}
			}
		}

		if shareErr := session.Share(cmd.Caption, cmd.MediaURLs, cmd.Visibility, workoutTitle, exerciseCount); shareErr != nil {
			return shareErr
		}

		if err := h.sessionRepo.Save(txCtx, session); err != nil {
			return err
		}

		events := session.PopEvents()
		if len(events) > 0 && h.outbox != nil {
			if err := h.outbox.WriteEvents(txCtx, "WorkoutSession", session.ID(), events); err != nil {
				return err
			}
		}

		result = &ShareWorkoutSessionResult{
			SessionID: session.ID(),
			IsShared:  session.IsShared(),
			SharedAt:  session.UpdatedAt(),
		}
		return nil
	}

	if h.txManager != nil {
		if err := h.txManager.WithTransaction(ctx, shareFunc); err != nil {
			return nil, fmt.Errorf("failed to share workout session tx: %w", err)
		}
	} else {
		if err := shareFunc(ctx); err != nil {
			return nil, err
		}
	}

	return result, nil
}
