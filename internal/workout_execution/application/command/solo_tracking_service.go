package command

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/viethung213/gym-companion/internal/workout_execution/application/apperror"
	"github.com/viethung213/gym-companion/internal/workout_execution/application/port"
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
	ruleProvider          repository.ExerciseRuleProvider
	ruleEvaluator         service.RuleEvaluator
	liveSync              port.SoloMatchLiveSyncPort
	outbox                port.OutboxWriter
	sessionRepo           repository.WorkoutSessionRepository
	txManager             port.TxManager
	readyEmitted          sync.Map
	disconnectGracePeriod time.Duration
	disconnectCancelers   sync.Map
}

const defaultDisconnectGracePeriod = 45 * time.Second

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
		ruleProvider:          ruleProvider,
		ruleEvaluator:         ruleEvaluator,
		liveSync:              liveSync,
		outbox:                outbox,
		sessionRepo:           sessionRepo,
		txManager:             txManager,
		disconnectGracePeriod: defaultDisconnectGracePeriod,
	}
}

// SetDisconnectGracePeriodForTesting overrides the default grace period for testing.
func (s *SoloTrackingService) SetDisconnectGracePeriodForTesting(d time.Duration) {
	s.disconnectGracePeriod = d
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

	timerKey := matchID + ":" + userID
	if oldCancel, ok := s.disconnectCancelers.LoadAndDelete(timerKey); ok && oldCancel != nil {
		if cancelFn, isFn := oldCancel.(context.CancelFunc); isFn {
			cancelFn()
		}
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

	// Reconnect recovery: restore progress if participant was previously active in this match
	if s.liveSync != nil {
		if prevProg, prevErr := s.liveSync.GetMyProgress(ctx, matchID, userID); prevErr == nil && prevProg != nil {
			if prevProg.Value > 0 {
				referee.RestoreProgress(prevProg.Value, prevProg.FormScore)
			}
		}
	}

	// If match room does not yet have 2 participants, stay in WAITING state.
	// If ready (both participants present), activate referee and notify opponent immediately.
	if s.liveSync != nil {
		ready, readyErr := s.checkAndEmitRoomReady(ctx, matchID)
		if readyErr != nil {
			slog.WarnContext(ctx, "failed to check match readiness during initialization",
				slog.String("match_id", matchID),
				slog.Any("err", readyErr),
			)
		} else if !ready {
			referee.SetStatus(service.ParticipantStatusWaiting)
		} else {
			referee.Activate()
			_ = s.liveSync.PublishMyProgress(ctx, matchID, port.ParticipantProgress{
				UserID:    userID,
				Value:     referee.ConfirmedValue(),
				FormScore: referee.AverageFormScore(),
				Status:    "ACTIVE",
			})
		}
	}

	// Create and persist WorkoutSession aggregate in DB in IN_PROGRESS state
	if err := s.initWorkoutSessionInDB(ctx, referee, userID, planID); err != nil {
		return nil, nil, err
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
		ready, readyErr := s.checkAndEmitRoomReady(ctx, matchID)
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

// HandleParticipantDisconnect handles sudden stream disconnection (network drop, reload, backgrounded app).
// It unregisters the participant on Redis live sync (which sets status = DISCONNECTED and disconnected_at timestamp),
// and starts a background grace period timer (default 45s). If the user does not reconnect within the grace period,
// it auto-aborts the session in DB, emits SoloTrackingAborted, and marks the participant ABORTED.
func (s *SoloTrackingService) HandleParticipantDisconnect(
	ctx context.Context,
	referee *service.SoloRefereeSession,
	matchID, userID, exerciseID, planID string,
) error {
	if referee == nil || referee.IsFinalized() || referee.IsCompleted() {
		return nil
	}

	if s.liveSync != nil {
		if err := s.liveSync.UnregisterParticipant(ctx, matchID, userID); err != nil {
			return fmt.Errorf("unregister participant on disconnect: %w", err)
		}
	}

	gracePeriod := s.disconnectGracePeriod
	if gracePeriod <= 0 {
		gracePeriod = defaultDisconnectGracePeriod
	}

	timerCtx, cancel := context.WithCancel(context.Background())
	timerKey := matchID + ":" + userID
	if oldCancel, ok := s.disconnectCancelers.Swap(timerKey, cancel); ok && oldCancel != nil {
		if cancelFn, isFn := oldCancel.(context.CancelFunc); isFn {
			cancelFn()
		}
	}

	time.AfterFunc(gracePeriod, func() {
		defer s.disconnectCancelers.Delete(timerKey)
		defer cancel()

		if timerCtx.Err() != nil {
			return
		}

		if referee.IsFinalized() || referee.IsCompleted() {
			return
		}

		// Double-check on Redis: did user reconnect?
		if s.liveSync != nil {
			prog, err := s.liveSync.GetMyProgress(context.Background(), matchID, userID)
			if err == nil && prog != nil && prog.Status != "DISCONNECTED" {
				return
			}
		}

		slog.Info("Participant disconnect grace period expired, auto-aborting session",
			slog.String("match_id", matchID),
			slog.String("user_id", userID),
		)

		cleanCtx, cleanCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanCancel()

		referee.SetCompletionReason("USER_DISCONNECTED")
		if finErr := s.FinalizeSession(cleanCtx, referee, matchID, userID, exerciseID, planID); finErr != nil {
			slog.Warn("Failed to finalize session after disconnect grace period expired",
				slog.String("match_id", matchID),
				slog.String("user_id", userID),
				slog.Any("err", finErr),
			)
		}
	})

	return nil
}

// FinalizeSession writes the final Outbox event if not already written, and notifies opponents.
// It is called ONLY when the session has actually concluded (Submitted, Timed Out, or Aborted).
func (s *SoloTrackingService) FinalizeSession(
	ctx context.Context,
	referee *service.SoloRefereeSession,
	matchID, userID, exerciseID string,
	planID string,
) error {
	if referee == nil || referee.IsFinalized() {
		return nil
	}
	defer referee.MarkFinalized()

	reason := referee.CompletionReason()
	if reason == "" {
		reason = "USER_DISCONNECTED"
	}
	isAbortedReason := reason == "USER_DISCONNECTED" || reason == "USER_FORFEITED" || strings.HasPrefix(reason, "ABORT") || reason == "SURRENDER" || reason == "GAVE_UP"

	if !referee.IsCompleted() {
		referee.MarkCompleted(reason)
		if err := s.abortWorkoutSessionInDB(ctx, referee, matchID, userID, exerciseID, reason, planID); err != nil {
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
	referee.MarkFinalized()

	return nil
}

// ForceCompleteSessionOnCeiling cưỡng chế hoàn thành session và phát SoloTrackingCompleted khi chạm thời gian trần.
func (s *SoloTrackingService) ForceCompleteSessionOnCeiling(
	ctx context.Context,
	referee *service.SoloRefereeSession,
	matchID, userID, exerciseID, planID string,
) error {
	if referee == nil {
		return nil
	}
	if !referee.IsCompleted() {
		referee.MarkCompleted("TIMEOUT")
		if err := s.completeWorkoutSessionInDB(ctx, referee, matchID, userID, exerciseID, planID); err != nil {
			return fmt.Errorf("complete session in db on ceiling: %w", err)
		}
	} else if referee.Status() == service.ParticipantStatusSpectating {
		referee.MarkCompleted(referee.CompletionReason())
	}
	return nil
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
