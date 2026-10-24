package command

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/viethung213/gym-companion/internal/workout_execution/application/apperror"
	"github.com/viethung213/gym-companion/internal/workout_execution/application/port"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/aggregate"
	domainEvent "github.com/viethung213/gym-companion/internal/workout_execution/domain/event"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/repository"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/service"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/vo"
)

// StreamFrameResult holds the evaluated output for a single streaming frame.
type StreamFrameResult struct {
	MyConfirmedValue        int32
	MyFormScore             float32
	IsValidForm             bool
	MyStatus                string
	IsCompleted             bool
	OpponentValue           int32
	OpponentFormScore       float32
	OpponentStatus          string
	DetectedErrors          []string
	AudioCue                string
	RemainingCeilingSeconds int32
	MatchEnded              bool
}

// SoloTrackingService coordinates streaming sessions, referee evaluation, live sync, session persistence, and outbox event publishing.
type SoloTrackingService struct {
	ruleProvider  repository.ExerciseRuleProvider
	ruleEvaluator service.RuleEvaluator
	liveSync      port.SoloMatchLiveSyncPort
	outbox        port.OutboxWriter
	sessionRepo   repository.WorkoutSessionRepository
	txManager     port.TxManager
}

// NewSoloTrackingService constructs a SoloTrackingService with all required dependencies.
func NewSoloTrackingService(
	ruleProvider repository.ExerciseRuleProvider,
	ruleEvaluator service.RuleEvaluator,
	liveSync port.SoloMatchLiveSyncPort,
	outbox port.OutboxWriter,
	sessionRepo repository.WorkoutSessionRepository,
	txManager port.TxManager,
) *SoloTrackingService {
	if ruleEvaluator == nil {
		ruleEvaluator = service.NewDefaultRuleEvaluator()
	}
	return &SoloTrackingService{
		ruleProvider:  ruleProvider,
		ruleEvaluator: ruleEvaluator,
		liveSync:      liveSync,
		outbox:        outbox,
		sessionRepo:   sessionRepo,
		txManager:     txManager,
	}
}

// InitializeSession loads exercise rules, registers live sync, and creates a WorkoutSession aggregate in DB.
func (s *SoloTrackingService) InitializeSession(
	ctx context.Context,
	matchID, userID, exerciseID string,
	challengeType vo.ChallengeType,
	timeCeilingSeconds int32,
	planID string,
) (referee *service.SoloRefereeSession, rule *vo.ExerciseMotionRule, err error) {
	planID = strings.TrimSpace(planID)
	if planID == "" {
		return nil, nil, fmt.Errorf("%w: plan_id is required", apperror.ErrInvalidInput)
	}

	var registered bool
	if s.liveSync != nil {
		if regErr := s.liveSync.RegisterParticipant(ctx, matchID, userID, exerciseID); regErr != nil {
			return nil, nil, regErr
		}
		registered = true
	}

	defer func() {
		if err != nil && registered && s.liveSync != nil {
			if unregErr := s.liveSync.UnregisterParticipant(ctx, matchID, userID); unregErr != nil {
				slog.WarnContext(ctx, "failed to unregister participant on init failure",
					slog.String("match_id", matchID),
					slog.String("user_id", userID),
					slog.Any("err", unregErr),
				)
			}
		}
	}()

	rule, err = s.ruleProvider.GetRule(ctx, exerciseID)
	if err != nil {
		return nil, nil, fmt.Errorf("load exercise rule: %w", err)
	}
	if rule == nil {
		return nil, nil, fmt.Errorf("exercise motion rule not configured for exercise_id: %s", exerciseID)
	}

	var ceiling time.Duration
	if timeCeilingSeconds > 0 {
		ceiling = time.Duration(timeCeilingSeconds) * time.Second
	}

	referee, err = service.NewSoloRefereeSession(exerciseID, challengeType, ceiling, rule)
	if err != nil {
		return nil, nil, fmt.Errorf("create referee session: %w", err)
	}

	// If match room does not yet have 2 participants, stay in WAITING state
	if s.liveSync != nil {
		ready, readyErr := s.liveSync.IsMatchReady(ctx, matchID)
		if readyErr != nil {
			slog.WarnContext(ctx, "failed to check match readiness during initialization",
				slog.String("match_id", matchID),
				slog.Any("err", readyErr),
			)
		} else if !ready {
			referee.SetStatus(service.ParticipantStatusWaiting)
		}
	}

	// Create and persist WorkoutSession aggregate in DB in IN_PROGRESS state
	if s.sessionRepo != nil {
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
				return nil, nil, fmt.Errorf("persist workout session in transaction: %w", err)
			}
		} else {
			if err := saveFunc(ctx); err != nil {
				return nil, nil, fmt.Errorf("persist workout session: %w", err)
			}
		}
	}

	return referee, rule, nil
}

// ProcessFrame evaluates landmarks against the rule, updates referee state, syncs progress, and returns realtime feedback.
func (s *SoloTrackingService) ProcessFrame(
	ctx context.Context,
	referee *service.SoloRefereeSession,
	rule *vo.ExerciseMotionRule,
	landmarks []vo.SkeletonPoint,
	delta time.Duration,
	matchID, userID, exerciseID string,
	planID string,
	cameraAngle string,
) (StreamFrameResult, error) {
	if referee == nil || rule == nil {
		return StreamFrameResult{}, fmt.Errorf("session not initialized: referee session and motion rule are required")
	}

	// 0. Check if match has become ready (both participants present)
	if referee.IsWaiting() && s.liveSync != nil {
		ready, readyErr := s.liveSync.IsMatchReady(ctx, matchID)
		if readyErr != nil {
			slog.WarnContext(ctx, "failed to check match readiness in frame loop",
				slog.String("match_id", matchID),
				slog.Any("err", readyErr),
			)
		} else if ready {
			referee.Activate()
		}
	}

	activeView := strings.ToLower(strings.TrimSpace(cameraAngle))
	if activeView == "" {
		activeView = referee.CameraAngle()
	}
	if activeView == "" && rule != nil {
		activeView = strings.ToLower(strings.TrimSpace(rule.RecommendedCameraAngle))
	}
	if activeView != "" {
		referee.SetCameraAngle(activeView)
	}

	// 1. Evaluate form against declarative rules filtered by camera view
	evalResult := s.ruleEvaluator.Evaluate(landmarks, rule, activeView)

	wasCompletedBefore := referee.IsCompleted()

	// 2. Feed result into referee session (if WAITING, referee ignores frame progression)
	referee.ProcessFrame(evalResult, delta)

	// 3. If session just transitioned to completed, commit DB session and SoloTrackingCompleted atomically
	if !wasCompletedBefore && referee.IsCompleted() {
		if err := s.completeWorkoutSessionInDB(ctx, referee, matchID, userID, exerciseID, planID); err != nil {
			slog.ErrorContext(ctx, "failed to complete workout session in frame loop",
				slog.String("match_id", matchID),
				slog.String("user_id", userID),
				slog.Any("err", err),
			)
		}
	}

	// 4. Publish my progress to live sync hub
	myProgress := port.ParticipantProgress{
		UserID:    userID,
		Value:     referee.ConfirmedValue(),
		FormScore: referee.AverageFormScore(),
		Status:    string(referee.Status()),
	}
	if s.liveSync != nil {
		if pubErr := s.liveSync.PublishMyProgress(ctx, matchID, myProgress); pubErr != nil {
			slog.WarnContext(ctx, "failed to publish participant progress in frame loop",
				slog.String("match_id", matchID),
				slog.String("user_id", userID),
				slog.Any("err", pubErr),
			)
		}
	}

	// 5. Fetch opponent's latest progress
	var (
		oppVal    int32
		oppScore  float32
		oppStatus = "WAITING"
	)
	if s.liveSync != nil {
		opp, oppErr := s.liveSync.GetOpponentProgress(ctx, matchID, userID)
		if oppErr != nil {
			slog.WarnContext(ctx, "failed to get opponent progress in frame loop",
				slog.String("match_id", matchID),
				slog.String("user_id", userID),
				slog.Any("err", oppErr),
			)
		} else if opp != nil {
			oppVal = opp.Value
			oppScore = opp.FormScore
			if opp.Status != "" {
				oppStatus = opp.Status
			}
		}
	}

	audioCue := evalResult.AudioCue
	if referee.IsWaiting() {
		if audioCue == "" {
			audioCue = "Đang chờ đối thủ vào phòng..."
		}
	}

	matchEnded := referee.IsCompleted() && (oppStatus == "COMPLETED" || oppStatus == "SPECTATING" || oppStatus == "ABORTED")
	if matchEnded && referee.IsCompleted() {
		referee.MarkCompleted(referee.CompletionReason())
	}

	return StreamFrameResult{
		MyConfirmedValue:        referee.ConfirmedValue(),
		MyFormScore:             evalResult.FormScore,
		IsValidForm:             evalResult.Passed,
		MyStatus:                string(referee.Status()),
		IsCompleted:             referee.IsCompleted(),
		OpponentValue:           oppVal,
		OpponentFormScore:       oppScore,
		OpponentStatus:          oppStatus,
		DetectedErrors:          evalResult.DetectedErrors,
		AudioCue:                audioCue,
		RemainingCeilingSeconds: referee.RemainingCeilingSeconds(),
		MatchEnded:              matchEnded,
	}, nil
}

// FinalizeSession writes the final Outbox event if not already written, and notifies opponents.
func (s *SoloTrackingService) FinalizeSession(
	ctx context.Context,
	referee *service.SoloRefereeSession,
	matchID, userID, exerciseID string,
	planID string,
) error {
	if referee == nil {
		return nil
	}

	reason := referee.CompletionReason()
	isAbortedReason := reason == "USER_DISCONNECTED" || reason == "USER_FORFEITED"

	if !referee.IsCompleted() {
		referee.MarkCompleted("USER_DISCONNECTED")
		if err := s.abortWorkoutSessionInDB(ctx, referee, matchID, userID, exerciseID, "USER_DISCONNECTED", planID); err != nil {
			return fmt.Errorf("abort workout session in db: %w", err)
		}
	} else if !isAbortedReason {
		referee.MarkCompleted(reason)
		if err := s.completeWorkoutSessionInDB(ctx, referee, matchID, userID, exerciseID, planID); err != nil {
			return fmt.Errorf("complete workout session in db: %w", err)
		}
	}

	finalStatus := "COMPLETED"
	if isAbortedReason {
		finalStatus = "ABORTED"
	}

	finalProgress := port.ParticipantProgress{
		UserID:    userID,
		Value:     referee.ConfirmedValue(),
		FormScore: referee.AverageFormScore(),
		Status:    finalStatus,
	}
	if s.liveSync != nil {
		if err := s.liveSync.PublishMyProgress(ctx, matchID, finalProgress); err != nil {
			return fmt.Errorf("publish final progress: %w", err)
		}
		if err := s.liveSync.UnregisterParticipant(ctx, matchID, userID); err != nil {
			return fmt.Errorf("unregister participant: %w", err)
		}
	}

	return nil
}

// SubmitSession handles an explicit submission/finish from the user.
// Case 1: If opponent is still active, transitions user to SPECTATING so they can watch opponent until completion.
// Case 2: If both participants have submitted/completed, marks MatchEnded = true to stop stream immediately and finalize.
func (s *SoloTrackingService) SubmitSession(
	ctx context.Context,
	referee *service.SoloRefereeSession,
	matchID, userID, exerciseID string,
	planID string,
) (StreamFrameResult, error) {
	if referee == nil {
		return StreamFrameResult{}, fmt.Errorf("session not initialized")
	}

	wasCompletedBefore := referee.IsCompleted()
	if !wasCompletedBefore {
		referee.MarkSpectating("USER_SUBMITTED")

		if err := s.completeWorkoutSessionInDB(ctx, referee, matchID, userID, exerciseID, planID); err != nil {
			slog.ErrorContext(ctx, "failed to persist completed workout session in submit",
				slog.String("match_id", matchID),
				slog.String("user_id", userID),
				slog.Any("err", err),
			)
		}
	}

	var (
		oppVal    int32
		oppScore  float32
		oppStatus = "WAITING"
	)
	if s.liveSync != nil {
		opp, oppErr := s.liveSync.GetOpponentProgress(ctx, matchID, userID)
		if oppErr != nil {
			slog.WarnContext(ctx, "failed to get opponent progress in submit",
				slog.String("match_id", matchID),
				slog.String("user_id", userID),
				slog.Any("err", oppErr),
			)
		} else if opp != nil {
			oppVal = opp.Value
			oppScore = opp.FormScore
			if opp.Status != "" {
				oppStatus = opp.Status
			}
		}
	}

	bothDone := oppStatus == "COMPLETED" || oppStatus == "SPECTATING" || oppStatus == "ABORTED"

	myStatus := "SPECTATING"
	if bothDone {
		myStatus = "COMPLETED"
		referee.MarkCompleted("USER_SUBMITTED")
	}

	myProgress := port.ParticipantProgress{
		UserID:    userID,
		Value:     referee.ConfirmedValue(),
		FormScore: referee.AverageFormScore(),
		Status:    myStatus,
	}
	if s.liveSync != nil {
		if err := s.liveSync.PublishMyProgress(ctx, matchID, myProgress); err != nil {
			return StreamFrameResult{}, fmt.Errorf("publish submit progress: %w", err)
		}
	}

	return StreamFrameResult{
		MyConfirmedValue:        referee.ConfirmedValue(),
		MyFormScore:             referee.AverageFormScore(),
		IsValidForm:             true,
		MyStatus:                myStatus,
		IsCompleted:             true,
		OpponentValue:           oppVal,
		OpponentFormScore:       oppScore,
		OpponentStatus:          oppStatus,
		RemainingCeilingSeconds: referee.RemainingCeilingSeconds(),
		MatchEnded:              bothDone,
	}, nil
}

// AbortSession handles an explicit abort/forfeit from the user.
func (s *SoloTrackingService) AbortSession(
	ctx context.Context,
	referee *service.SoloRefereeSession,
	matchID, userID, exerciseID, reason, planID string,
) error {
	if referee == nil {
		return nil
	}

	// If the user's workout has already concluded (e.g. 5s violation, time ceiling, or submit),
	// requesting abort is an exit from the spectator room and must preserve the COMPLETED status.
	if referee.IsCompleted() {
		return s.FinalizeSession(ctx, referee, matchID, userID, exerciseID, planID)
	}

	if reason == "" {
		reason = "USER_FORFEITED"
	}

	referee.MarkCompleted(reason)
	if err := s.abortWorkoutSessionInDB(ctx, referee, matchID, userID, exerciseID, reason, planID); err != nil {
		return fmt.Errorf("abort workout session in db: %w", err)
	}

	finalProgress := port.ParticipantProgress{
		UserID:    userID,
		Value:     referee.ConfirmedValue(),
		FormScore: referee.AverageFormScore(),
		Status:    "ABORTED",
	}
	if s.liveSync != nil {
		if err := s.liveSync.PublishMyProgress(ctx, matchID, finalProgress); err != nil {
			return fmt.Errorf("publish aborted progress: %w", err)
		}
		if err := s.liveSync.UnregisterParticipant(ctx, matchID, userID); err != nil {
			return fmt.Errorf("unregister participant: %w", err)
		}
	}

	return nil
}

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

		avgScore := referee.AverageFormScore()
		setLog := aggregate.WorkoutSetLog{
			ID:         uuid.NewString(),
			SessionID:  sess.ID(),
			SetNumber:  1,
			ExerciseID: exerciseID,
			TargetReps: int(referee.ConfirmedValue()),
			ActualReps: int(referee.ConfirmedValue()),
			FormScore:  &avgScore,
			CreatedAt:  time.Now().UTC(),
		}
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
				avgScore := referee.AverageFormScore()
				setLog := aggregate.WorkoutSetLog{
					ID:         uuid.NewString(),
					SessionID:  sess.ID(),
					SetNumber:  1,
					ExerciseID: exerciseID,
					TargetReps: int(referee.ConfirmedValue()),
					ActualReps: int(referee.ConfirmedValue()),
					FormScore:  &avgScore,
					CreatedAt:  time.Now().UTC(),
				}
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

// SubscribeMatchUpdates subscribes to updates from the opponent in the match.
func (s *SoloTrackingService) SubscribeMatchUpdates(
	ctx context.Context,
	matchID, userID string,
) (<-chan port.ParticipantProgress, func(), error) {
	if s.liveSync == nil {
		return nil, func() {}, nil
	}
	return s.liveSync.SubscribeMatchUpdates(ctx, matchID, userID)
}

// GetOpponentProgress fetches the latest recorded progress of the opponent in the match.
func (s *SoloTrackingService) GetOpponentProgress(
	ctx context.Context,
	matchID, userID string,
) (*port.ParticipantProgress, error) {
	if s.liveSync == nil {
		return nil, nil
	}
	return s.liveSync.GetOpponentProgress(ctx, matchID, userID)
}
