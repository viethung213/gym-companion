package command

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/aggregate"
	domainEvent "github.com/viethung213/gym-companion/internal/workout_execution/domain/event"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/service"
)

// initWorkoutSessionInDB creates and persists a WorkoutSession aggregate in DB in IN_PROGRESS state.
// If an active session already exists for the user:
// - If it belongs to the same planID, reuses it.
// - If it belongs to a different planID, aborts it first to ensure only 1 active session exists.
func (s *SoloTrackingService) initWorkoutSessionInDB(
	ctx context.Context,
	referee *service.SoloRefereeSession,
	userID, planID string,
) error {
	if s.sessionRepo == nil {
		return nil
	}

	saveFunc := func(txCtx context.Context) error {
		active, err := s.sessionRepo.FindActiveSessionByUserID(txCtx, userID)
		if err != nil {
			return fmt.Errorf("find active session: %w", err)
		}
		if active != nil {
			if active.PlanID() == planID {
				referee.SetWorkoutSessionID(active.ID())
				return nil
			}
			if abortErr := active.Abort("NEW_SOLO_MATCH_STARTED"); abortErr != nil {
				return fmt.Errorf("abort previous active session: %w", abortErr)
			}
			if saveErr := s.sessionRepo.Save(txCtx, active); saveErr != nil {
				return fmt.Errorf("save aborted active session: %w", saveErr)
			}
			abortedEvents := active.PopEvents()
			if len(abortedEvents) > 0 && s.outbox != nil {
				if outboxErr := s.outbox.WriteEvents(txCtx, "WorkoutSession", active.ID(), abortedEvents); outboxErr != nil {
					return fmt.Errorf("write outbox events for aborted session: %w", outboxErr)
				}
			}
		}

		sessionID := uuid.NewString()
		workoutSess, err := aggregate.NewWorkoutSession(sessionID, userID, planID)
		if err != nil {
			return fmt.Errorf("create workout session: %w", err)
		}
		if err := s.sessionRepo.Save(txCtx, workoutSess); err != nil {
			return fmt.Errorf("save workout session: %w", err)
		}
		events := workoutSess.PopEvents()
		if len(events) > 0 && s.outbox != nil {
			if err := s.outbox.WriteEvents(txCtx, "WorkoutSession", workoutSess.ID(), events); err != nil {
				return fmt.Errorf("write outbox events: %w", err)
			}
		}
		referee.SetWorkoutSessionID(sessionID)
		return nil
	}

	if s.txManager != nil {
		if err := s.txManager.WithTransaction(ctx, saveFunc); err != nil {
			return fmt.Errorf("persist workout session in transaction: %w", err)
		}
		return nil
	}

	if err := saveFunc(ctx); err != nil {
		return fmt.Errorf("persist workout session: %w", err)
	}
	return nil
}

// completeWorkoutSessionInDB logs final set, completes the WorkoutSession, and publishes SoloTrackingCompleted event.
func (s *SoloTrackingService) completeWorkoutSessionInDB(
	ctx context.Context,
	referee *service.SoloRefereeSession,
	matchID, userID, exerciseID, planID string,
) error {
	if referee == nil {
		return nil
	}

	soloEvent := &domainEvent.SoloTrackingCompleted{
		MatchID:              matchID,
		UserID:               userID,
		ExerciseID:           exerciseID,
		ChallengeType:        referee.ChallengeType(),
		ConfirmedValue:       referee.ConfirmedValue(),
		AverageFormScore:     referee.AverageFormScore(),
		TotalDurationSeconds: referee.TotalDurationSeconds(),
		CompletedAt:          time.Now().UTC(),
		PlanID:               planID,
	}

	if s.sessionRepo == nil || referee.WorkoutSessionID() == "" {
		if s.outbox != nil {
			if err := s.outbox.WriteEvents(ctx, "SoloMatch", matchID, []interface{}{soloEvent}); err != nil {
				return fmt.Errorf("write solo match completed outbox: %w", err)
			}
		}
		return nil
	}

	saveFunc := func(txCtx context.Context) error {
		sess, err := s.sessionRepo.FindByID(txCtx, referee.WorkoutSessionID())
		if err != nil {
			return fmt.Errorf("find workout session: %w", err)
		}
		if sess == nil {
			return fmt.Errorf("workout session %s not found", referee.WorkoutSessionID())
		}
		if sess.Status() == aggregate.StatusCompleted {
			return nil
		}

		setLog := buildSoloWorkoutSetLog(sess.ID(), exerciseID, referee.ConfirmedValue(), referee.AverageFormScore())
		if logErr := sess.LogSet(setLog); logErr != nil {
			return fmt.Errorf("log set: %w", logErr)
		}
		if compErr := sess.Complete(false, false); compErr != nil {
			return fmt.Errorf("complete aggregate: %w", compErr)
		}

		if err := s.sessionRepo.Save(txCtx, sess); err != nil {
			return fmt.Errorf("save completed session: %w", err)
		}

		if s.outbox != nil {
			events := sess.PopEvents()
			if len(events) > 0 {
				if err := s.outbox.WriteEvents(txCtx, "WorkoutSession", sess.ID(), events); err != nil {
					return fmt.Errorf("write session outbox: %w", err)
				}
			}

			if err := s.outbox.WriteEvents(txCtx, "SoloMatch", matchID, []interface{}{soloEvent}); err != nil {
				return fmt.Errorf("write solo match completed outbox: %w", err)
			}
		}
		return nil
	}

	if s.txManager != nil {
		return s.txManager.WithTransaction(ctx, saveFunc)
	}
	return saveFunc(ctx)
}

// abortWorkoutSessionInDB aborts the WorkoutSession and publishes SoloTrackingAborted event.
func (s *SoloTrackingService) abortWorkoutSessionInDB(
	ctx context.Context,
	referee *service.SoloRefereeSession,
	matchID, userID, exerciseID, reason, planID string,
) error {
	if referee == nil {
		return nil
	}

	soloAbortedEv := &domainEvent.SoloTrackingAborted{
		MatchID:              matchID,
		UserID:               userID,
		ExerciseID:           exerciseID,
		Reason:               reason,
		AbortedAt:            time.Now().UTC(),
		PlanID:               planID,
		ConfirmedValue:       referee.ConfirmedValue(),
		TotalDurationSeconds: referee.TotalDurationSeconds(),
	}

	if s.sessionRepo == nil || referee.WorkoutSessionID() == "" {
		if s.outbox != nil {
			if err := s.outbox.WriteEvents(ctx, "SoloMatch", matchID, []interface{}{soloAbortedEv}); err != nil {
				return fmt.Errorf("write solo aborted outbox: %w", err)
			}
		}
		return nil
	}

	abortFunc := func(txCtx context.Context) error {
		sess, err := s.sessionRepo.FindByID(txCtx, referee.WorkoutSessionID())
		if err != nil {
			return fmt.Errorf("find workout session on abort: %w", err)
		}
		if sess == nil {
			return fmt.Errorf("workout session %s not found on abort", referee.WorkoutSessionID())
		}
		if sess.Status() == aggregate.StatusInProgress {
			if referee.ConfirmedValue() > 0 {
				setLog := buildSoloWorkoutSetLog(sess.ID(), exerciseID, referee.ConfirmedValue(), referee.AverageFormScore())
				if logErr := sess.LogSet(setLog); logErr != nil {
					return fmt.Errorf("log set on abort: %w", logErr)
				}
			}
			if abortErr := sess.Abort(reason); abortErr != nil {
				return fmt.Errorf("abort session aggregate: %w", abortErr)
			}
			if err := s.sessionRepo.Save(txCtx, sess); err != nil {
				return fmt.Errorf("save aborted session: %w", err)
			}
			if s.outbox != nil {
				events := sess.PopEvents()
				if len(events) > 0 {
					if err := s.outbox.WriteEvents(txCtx, "WorkoutSession", sess.ID(), events); err != nil {
						return fmt.Errorf("write aborted session outbox: %w", err)
					}
				}

				if err := s.outbox.WriteEvents(txCtx, "SoloMatch", matchID, []interface{}{soloAbortedEv}); err != nil {
					return fmt.Errorf("write solo aborted outbox: %w", err)
				}
			}
		}
		return nil
	}

	if s.txManager != nil {
		return s.txManager.WithTransaction(ctx, abortFunc)
	}
	return abortFunc(ctx)
}

func buildSoloWorkoutSetLog(sessionID, exerciseID string, confirmedValue int32, avgScore float32) aggregate.WorkoutSetLog {
	return aggregate.WorkoutSetLog{
		ID:         uuid.NewString(),
		SessionID:  sessionID,
		SetNumber:  1,
		ExerciseID: exerciseID,
		TargetReps: int(confirmedValue),
		ActualReps: int(confirmedValue),
		FormScore:  &avgScore,
		CreatedAt:  time.Now().UTC(),
	}
}
