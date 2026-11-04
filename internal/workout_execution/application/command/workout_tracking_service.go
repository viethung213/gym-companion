package command

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/viethung213/gym-companion/internal/workout_execution/domain/repository"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/service"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/vo"
)

// WorkoutTrackingResult represents instantaneous biomechanical feedback for a single-player workout set.
type WorkoutTrackingResult struct {
	RepsCompleted       int32
	HoldDurationSeconds float32
	FormScore           float32
	IsValidForm         bool
	IsSetCompleted      bool
	ActiveErrors        []string
	AudioCue            string
	CurrentAngle        float32
	RepPhase            string
	PersonDetected      bool
}

// WorkoutTrackingService manages live AI camera referee evaluation for individual workout sets.
// It is intentionally decoupled from database session lifecycle (no auto-completion or auto-aborting of parent WorkoutSession).
type WorkoutTrackingService struct {
	ruleProvider  repository.ExerciseRuleProvider
	ruleEvaluator service.RuleEvaluator
}

// NewWorkoutTrackingService constructs a pure referee streaming service for workout sets.
func NewWorkoutTrackingService(
	ruleProvider repository.ExerciseRuleProvider,
	ruleEvaluator service.RuleEvaluator,
) *WorkoutTrackingService {
	if ruleEvaluator == nil {
		ruleEvaluator = service.NewDefaultRuleEvaluator()
	}
	return &WorkoutTrackingService{
		ruleProvider:  ruleProvider,
		ruleEvaluator: ruleEvaluator,
	}
}

// InitializeSession loads exercise motion rules and initializes a pure in-memory MotionRefereeSession.
func (s *WorkoutTrackingService) InitializeSession(
	ctx context.Context,
	sessionID, userID, exerciseID, cameraAngle string,
	challengeType vo.ChallengeType,
	targetReps, targetDuration int32,
) (*service.MotionRefereeSession, *vo.ExerciseMotionRule, error) {
	if s.ruleProvider == nil {
		return nil, nil, errors.New("exercise rule provider is not configured")
	}

	exerciseID = strings.TrimSpace(exerciseID)
	if exerciseID == "" {
		return nil, nil, errors.New("exercise_id is required")
	}

	rule, err := s.ruleProvider.GetRule(ctx, exerciseID)
	if err != nil {
		return nil, nil, fmt.Errorf("load exercise rule for %s: %w", exerciseID, err)
	}
	if rule == nil {
		return nil, nil, fmt.Errorf("exercise motion rule not found: %s", exerciseID)
	}

	effectiveChallengeType := challengeType
	if !effectiveChallengeType.IsValid() {
		if rule.ChallengeType.IsValid() {
			effectiveChallengeType = rule.ChallengeType
		} else {
			effectiveChallengeType = vo.ChallengeTypeReps
		}
	}

	timeCeiling := rule.TimeCeiling
	if timeCeiling <= 0 {
		if effectiveChallengeType == vo.ChallengeTypeDuration {
			timeCeiling = 30 * time.Minute
		} else {
			timeCeiling = 10 * time.Minute
		}
	}

	referee, err := service.NewMotionRefereeSession(exerciseID, effectiveChallengeType, timeCeiling, rule)
	if err != nil {
		return nil, nil, fmt.Errorf("create motion referee session: %w", err)
	}

	referee.Activate()
	return referee, rule, nil
}

// ProcessFrame evaluates a single video landmark frame and updates the referee state.
func (s *WorkoutTrackingService) ProcessFrame(
	ctx context.Context,
	referee *service.MotionRefereeSession,
	rule *vo.ExerciseMotionRule,
	landmarks []vo.SkeletonPoint,
	delta time.Duration,
	cameraAngle string,
) (WorkoutTrackingResult, error) {
	if referee == nil || rule == nil {
		return WorkoutTrackingResult{}, errors.New("referee session and rule are required")
	}

	activeView := cameraAngle
	if activeView == "" {
		activeView = rule.RecommendedCameraAngle
	}

	evalResult := s.ruleEvaluator.Evaluate(landmarks, rule, activeView)
	referee.ProcessFrame(evalResult, delta)

	repsCompleted := int32(0)
	holdDuration := float32(0)

	if referee.ChallengeType() == vo.ChallengeTypeDuration {
		holdDuration = float32(referee.ConfirmedValue())
	} else {
		repsCompleted = referee.ConfirmedValue()
	}

	return WorkoutTrackingResult{
		RepsCompleted:       repsCompleted,
		HoldDurationSeconds: holdDuration,
		FormScore:           evalResult.FormScore,
		IsValidForm:         len(referee.ConfirmedErrors()) == 0,
		IsSetCompleted:      referee.IsCompleted(),
		ActiveErrors:        referee.ConfirmedErrors(),
		AudioCue:            referee.ConfirmedAudioCue(),
		CurrentAngle:        referee.CurrentAngle(),
		RepPhase:            string(referee.RepPhase()),
		PersonDetected:      referee.PersonDetected(),
	}, nil
}

// SubmitSet finalizes the current set's referee session and returns the final score and rep count.
func (s *WorkoutTrackingService) SubmitSet(
	ctx context.Context,
	referee *service.MotionRefereeSession,
) (WorkoutTrackingResult, error) {
	if referee == nil {
		return WorkoutTrackingResult{}, errors.New("referee session is required")
	}

	referee.MarkCompleted("SET_SUBMITTED")

	repsCompleted := int32(0)
	holdDuration := float32(0)
	if referee.ChallengeType() == vo.ChallengeTypeDuration {
		holdDuration = float32(referee.ConfirmedValue())
	} else {
		repsCompleted = referee.ConfirmedValue()
	}

	return WorkoutTrackingResult{
		RepsCompleted:       repsCompleted,
		HoldDurationSeconds: holdDuration,
		FormScore:           referee.AverageFormScore(),
		IsValidForm:         len(referee.ConfirmedErrors()) == 0,
		IsSetCompleted:      true,
		ActiveErrors:        referee.ConfirmedErrors(),
		AudioCue:            referee.ConfirmedAudioCue(),
		CurrentAngle:        referee.CurrentAngle(),
		RepPhase:            string(referee.RepPhase()),
		PersonDetected:      referee.PersonDetected(),
	}, nil
}
