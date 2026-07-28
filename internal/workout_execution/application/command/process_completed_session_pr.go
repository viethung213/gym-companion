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

// ProcessCompletedSessionForPRHandler handles async 1RM personal record calculation upon session completion.
type ProcessCompletedSessionForPRHandler struct {
	sessionRepo repository.WorkoutSessionRepository
	prRepo      repository.PersonalRecordRepository
	outbox      port.OutboxWriter
	txManager   port.TxManager
}

// NewProcessCompletedSessionForPRHandler constructs handler.
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

// HandleProcess executes 1RM evaluation for a completed session.
func (h *ProcessCompletedSessionForPRHandler) HandleProcess(ctx context.Context, sessionID, userID string) error {
	session, err := h.sessionRepo.FindByID(ctx, sessionID)
	if err != nil || session == nil {
		return fmt.Errorf("session not found for PR calculation: %w", err)
	}

	if session.Status() == aggregate.StatusAnomalous {
		// Anomalous sessions are excluded from 1RM PR calculations per UC-03.5 E2.
		return nil
	}

	bestSets := make(map[string]aggregate.WorkoutSetLog)
	for _, set := range session.Sets() {
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
		formVerified := set.FormScore != nil && *set.FormScore >= 70.0
		existingPR, err := h.prRepo.FindByUserIDAndExerciseID(ctx, userID, exerciseID)
		if err != nil {
			continue
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
			pr = aggregate.NewPersonalRecord(prID, userID, exerciseID, set.Weight, set.ActualReps, formVerified, t)
		} else {
			pr = existingPR
			updated := pr.UpdateIfHigher(set.Weight, set.ActualReps, formVerified, t)
			if !updated {
				continue
			}
		}

		err = h.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
			if err := h.prRepo.Save(txCtx, pr); err != nil {
				return err
			}
			events := pr.PopEvents()
			if len(events) > 0 && h.outbox != nil {
				if err := h.outbox.WriteEvents(txCtx, "PersonalRecord", pr.ID(), events); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("failed to save PR for exercise %s: %w", exerciseID, err)
		}
	}

	return nil
}
