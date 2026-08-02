package command

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/viethung213/gym-companion/internal/workout_execution/application/port"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/repository"
)

// ProcessCompletedSessionForPRHandler handles async 1RM personal record calculation
// upon session completion.
type ProcessCompletedSessionForPRHandler struct {
	sessionRepo repository.WorkoutSessionRepository
	prRepo      repository.PersonalRecordRepository
	outbox      port.OutboxWriter
	txManager   port.TxManager
}

// NewProcessCompletedSessionForPRHandler creates a new handler instance.
func NewProcessCompletedSessionForPRHandler(
	sessionRepo repository.WorkoutSessionRepository,
	prRepo repository.PersonalRecordRepository,
	outbox port.OutboxWriter,
	txManager port.TxManager,
) *ProcessCompletedSessionForPRHandler {
	return &ProcessCompletedSessionForPRHandler{
		sessionRepo: sessionRepo,
		prRepo:      prRepo,
		outbox:      outbox,
		txManager:   txManager,
	}
}

// HandleProcess evaluates 1RM PRs for all sets in a completed session.
func (h *ProcessCompletedSessionForPRHandler) HandleProcess(
	ctx context.Context,
	sessionID, userID string,
) error {
	session, err := h.sessionRepo.FindByID(ctx, sessionID)
	if err != nil || session == nil {
		return fmt.Errorf("session not found for PR calculation: %w", err)
	}

	if session.Status() == aggregate.StatusAnomalous {
		// Anomalous sessions are excluded from 1RM PR calculations per UC-03.5 E2.
		return nil
	}

	bestSets := make(map[string]aggregate.WorkoutSetLog)
	sets := session.Sets()
	for i := range sets {
		set := sets[i]
		if set.ActualReps <= 0 || set.Weight <= 0 {
			continue
		}
		currentBest, exists := bestSets[set.ExerciseID]
		if !exists {
			bestSets[set.ExerciseID] = set
			continue
		}

		current1RM := aggregate.Calculate1RMEpley(currentBest.Weight, currentBest.ActualReps)
		new1RM := aggregate.Calculate1RMEpley(set.Weight, set.ActualReps)
		if new1RM > current1RM {
			bestSets[set.ExerciseID] = set
		}
	}

	for exerciseID, set := range bestSets {
		exerciseID := exerciseID
		set := set
		processExercisePR := func(txCtx context.Context) error {
			formVerified := set.FormScore != nil && *set.FormScore >= 70.0
			existingPR, findErr := h.prRepo.FindByUserIDAndExerciseIDForUpdate(txCtx, userID, exerciseID)
			if findErr != nil {
				return fmt.Errorf("find personal record for update: %w", findErr)
			}

			var pr *aggregate.PersonalRecord
			achievedAt := session.EndedAt()
			var t time.Time
			if achievedAt != nil {
				t = *achievedAt
			} else {
				t = time.Now().UTC()
			}

			if existingPR == nil {
				prID := uuid.NewString()
				pr = aggregate.NewPersonalRecord(
					prID, userID, exerciseID, set.Weight, set.ActualReps, formVerified, t,
				)
			} else {
				pr = existingPR
				updated := pr.UpdateIfHigher(set.Weight, set.ActualReps, formVerified, t)
				if !updated {
					return nil
				}
			}

			if saveErr := h.prRepo.Save(txCtx, pr); saveErr != nil {
				return saveErr
			}
			events := pr.PopEvents()
			if len(events) > 0 && h.outbox != nil {
				if writeErr := h.outbox.WriteEvents(txCtx, "PersonalRecord", pr.ID(), events); writeErr != nil {
					return writeErr
				}
			}
			return nil
		}

		var txErr error
		if h.txManager != nil {
			txErr = h.txManager.WithTransaction(ctx, processExercisePR)
		} else {
			txErr = processExercisePR(ctx)
		}
		if txErr != nil {
			return fmt.Errorf("failed to save PR for exercise %s: %w", exerciseID, txErr)
		}
	}

	return nil
}
