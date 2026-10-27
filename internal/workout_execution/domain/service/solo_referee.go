package service

import (
	"fmt"
	"sync"
	"time"

	"github.com/viethung213/gym-companion/internal/workout_execution/domain/vo"
)

// ParticipantStatus defines the active stream status of a participant.
type ParticipantStatus string

const (
	ParticipantStatusWaiting    ParticipantStatus = "WAITING"
	ParticipantStatusActive     ParticipantStatus = "ACTIVE"
	ParticipantStatusSpectating ParticipantStatus = "SPECTATING"
	ParticipantStatusCompleted  ParticipantStatus = "COMPLETED"
)

// RepPhase represents the active phase in a rep cycle.
type RepPhase string

const (
	RepPhaseRest       RepPhase = "REST"
	RepPhaseTransition RepPhase = "TRANSITION"
	RepPhaseActive     RepPhase = "ACTIVE"
)

// SoloRefereeSession coordinates state, time ceilings, scores, and 5s violation tracking for one participant.
type SoloRefereeSession struct {
	mu sync.RWMutex

	exerciseID       string
	challengeType    vo.ChallengeType
	timeCeiling      time.Duration
	workoutSessionID string
	cameraAngle      string

	status           ParticipantStatus
	isCompleted      bool
	isFinalized      bool
	completionReason string

	confirmedValue  int32
	elapsedDuration time.Duration
	startedAt       time.Time
	scoreSum        float32
	scoreCount      int32

	// Duration challenge specific
	accumulatedDuration          time.Duration
	consecutiveViolationDuration time.Duration
	maxViolationThreshold        time.Duration

	// Rep challenge specific - Strict 3-Phase Sequential Progression
	repPhase           RepPhase
	hasTransitionedIn  bool // Checkpoint 1: REST -> TRANSITION (Bắt đầu chuyển động)
	hasReachedActive   bool // Checkpoint 2: TRANSITION -> ACTIVE (Đạt đỉnh co cơ)
	hasTransitionedOut bool // Checkpoint 3: ACTIVE -> TRANSITION (Đang trở về)
	phaseDetection     *vo.PhaseDetectionConfig
}

// NewSoloRefereeSession creates a new referee session for an exercise and challenge type.
// It strictly requires a valid, non-nil ExerciseMotionRule (and non-nil PhaseDetection for REPS challenges).
func NewSoloRefereeSession(
	exerciseID string,
	challengeType vo.ChallengeType,
	timeCeiling time.Duration,
	rule *vo.ExerciseMotionRule,
) (*SoloRefereeSession, error) {
	if rule == nil {
		return nil, fmt.Errorf("exercise motion rule is required to initialize solo referee session: %s", exerciseID)
	}

	const (
		fixedDurationCeiling = 30 * time.Minute
		defaultRepCeiling    = 10 * time.Minute
	)

	normChallengeType := challengeType
	if !normChallengeType.IsValid() {
		if rule.ChallengeType.IsValid() {
			normChallengeType = rule.ChallengeType
		} else {
			normChallengeType = vo.ChallengeTypeReps
		}
	} else if rule.ChallengeType == vo.ChallengeTypeDuration {
		normChallengeType = vo.ChallengeTypeDuration
	}

	// 1. Validate exercise motion rule preconditions
	if normChallengeType == vo.ChallengeTypeReps && rule.PhaseDetection == nil {
		return nil, fmt.Errorf("exercise motion rule must configure phase_detection for rep challenge: %s", exerciseID)
	}
	if normChallengeType == vo.ChallengeTypeDuration && len(rule.Rules) == 0 {
		return nil, fmt.Errorf("exercise motion rule must configure validation criteria for duration challenge: %s", exerciseID)
	}

	// 2. Resolve time ceiling
	ceiling := defaultRepCeiling
	if normChallengeType == vo.ChallengeTypeDuration {
		ceiling = fixedDurationCeiling
	} else if timeCeiling > 0 {
		ceiling = timeCeiling
	}

	return &SoloRefereeSession{
		exerciseID:            exerciseID,
		challengeType:         normChallengeType,
		timeCeiling:           ceiling,
		status:                ParticipantStatusActive,
		startedAt:             time.Now(),
		maxViolationThreshold: 5 * time.Second,
		repPhase:              RepPhaseRest,
		hasTransitionedIn:     false,
		hasReachedActive:      false,
		hasTransitionedOut:    false,
		phaseDetection:        rule.PhaseDetection,
	}, nil
}

// ProcessFrame updates the session state given the evaluation result and the elapsed time since last frame.
func (s *SoloRefereeSession) ProcessFrame(result vo.EvaluationResult, delta time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.isCompleted || s.status == ParticipantStatusWaiting {
		return
	}

	if s.startedAt.IsZero() {
		s.startedAt = time.Now()
	}

	curElapsed := s.getElapsedLocked()
	if curElapsed > s.elapsedDuration {
		s.elapsedDuration = curElapsed
	}

	// Calculate allowed delta up to time ceiling
	remaining := s.timeCeiling - s.elapsedDuration
	isLastFrame := false
	effectiveDelta := delta

	if effectiveDelta >= remaining {
		effectiveDelta = remaining
		isLastFrame = true
	}

	s.elapsedDuration += effectiveDelta

	// Update average form score for all evaluated frames with a person present
	if result.FormScore >= 0 {
		isNoPerson := len(result.DetectedErrors) == 1 && result.DetectedErrors[0] == "ERR_NO_PERSON_DETECTED"
		if !isNoPerson {
			s.scoreSum += result.FormScore
			s.scoreCount++
		}
	}

	if s.challengeType == vo.ChallengeTypeDuration {
		s.processDurationFrame(result, effectiveDelta)
	} else {
		s.processRepFrame(result)
	}

	// Check time ceiling limit
	if isLastFrame || s.elapsedDuration >= s.timeCeiling {
		s.elapsedDuration = s.timeCeiling
		s.isCompleted = true
		s.status = ParticipantStatusCompleted
		if s.completionReason == "" {
			s.completionReason = "TIMEOUT"
		}
	}
}

func (s *SoloRefereeSession) processDurationFrame(result vo.EvaluationResult, delta time.Duration) {
	if result.Passed {
		// Valid form: continue accumulating time and reset consecutive violation timer
		s.accumulatedDuration += delta
		if s.accumulatedDuration >= s.timeCeiling {
			s.accumulatedDuration = s.timeCeiling
			s.isCompleted = true
			s.status = ParticipantStatusCompleted
			s.completionReason = "TIMEOUT"
		}
		s.consecutiveViolationDuration = 0
		s.confirmedValue = int32(s.accumulatedDuration.Seconds())
	} else {
		// Invalid form: pause accumulation and increment consecutive violation timer
		s.consecutiveViolationDuration += delta
		if s.consecutiveViolationDuration >= s.maxViolationThreshold {
			s.isCompleted = true
			s.status = ParticipantStatusSpectating
			s.completionReason = "FORM_VIOLATION_5S"
		}
	}
}

func (s *SoloRefereeSession) processRepFrame(result vo.EvaluationResult) {
	if !result.Passed {
		return
	}

	angle := result.CalculatedAngle
	if angle <= 0 || s.phaseDetection == nil {
		return
	}

	// Phân loại vùng góc (Rest, Transition, Active) dựa trên cấu hình ngưỡng từ Rule JSON
	th := s.phaseDetection.Thresholds
	isRest := th.Rest.Matches(angle)
	isActive := th.Active.Matches(angle) && !isRest
	isTransition := th.Transition.Matches(angle) && !isRest && !isActive

	// 2. Strict Sequential State Machine (REST -> TRANSITION -> ACTIVE -> TRANSITION -> REST)
	switch s.repPhase {
	case RepPhaseRest:
		if isTransition {
			// Bắt đầu chu trình: Từ REST đi vào TRANSITION
			s.repPhase = RepPhaseTransition
			s.hasTransitionedIn = true
			s.hasReachedActive = false
			s.hasTransitionedOut = false
		} else if isActive {
			// Bỏ qua transition: vi phạm trình tự (nhảy cóc), không đánh dấu hợp lệ
			s.repPhase = RepPhaseActive
			s.hasTransitionedIn = false
			s.hasReachedActive = false
			s.hasTransitionedOut = false
		}

	case RepPhaseTransition:
		if isActive {
			// Đạt đỉnh co cơ (Active) CHỈ HỢP LỆ khi đã đi qua mốc di chuyển Inward
			if s.hasTransitionedIn {
				s.hasReachedActive = true
			}
			s.repPhase = RepPhaseActive
		} else if isRest {
			// Quay về Rest:
			// ĐIỀU KIỆN TIÊN QUYẾT TĂNG 1 REP: Phải đi đúng và đủ toàn bộ trình tự
			// (Đã Transition vào -> Đã đạt đỉnh Active -> Đã Transition ra có kiểm soát)
			if s.hasTransitionedIn && s.hasReachedActive && s.hasTransitionedOut {
				s.confirmedValue++
			}
			// Kết thúc chu trình hoặc hủy bỏ rep dở dang: reset trạng thái và các cờ
			s.repPhase = RepPhaseRest
			s.hasTransitionedIn = false
			s.hasReachedActive = false
			s.hasTransitionedOut = false
		} else if isTransition {
			// Đang duy trì trong dải chuyển động:
			// Nếu đã từng chạm Active, đánh dấu đang trong giai đoạn Returning Transition
			if s.hasReachedActive {
				s.hasTransitionedOut = true
			}
		}

	case RepPhaseActive:
		if isTransition {
			// Rời khỏi đỉnh Active bắt đầu trở về: ghi nhận mốc di chuyển ra
			if s.hasReachedActive {
				s.hasTransitionedOut = true
			}
			s.repPhase = RepPhaseTransition
		} else if isRest {
			// Nhảy cóc thẳng từ Active về Rest mà không qua frame Transition:
			// Vi phạm trình tự chuyển động có kiểm soát -> Không tăng rep, reset
			s.repPhase = RepPhaseRest
			s.hasTransitionedIn = false
			s.hasReachedActive = false
			s.hasTransitionedOut = false
		}
	}
}

// RepPhase returns the current active phase of the rep cycle.
func (s *SoloRefereeSession) RepPhase() RepPhase {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.repPhase
}

// MarkCompleted transitions session to COMPLETED.
func (s *SoloRefereeSession) MarkCompleted(reason string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	curElapsed := s.getElapsedLocked()
	if curElapsed > s.elapsedDuration {
		s.elapsedDuration = curElapsed
	}
	s.isCompleted = true
	s.status = ParticipantStatusCompleted
	if s.completionReason == "" {
		s.completionReason = reason
	}
}

// MarkSpectating transitions session to completed with SPECTATING status so user can observe opponent.
func (s *SoloRefereeSession) MarkSpectating(reason string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	curElapsed := s.getElapsedLocked()
	if curElapsed > s.elapsedDuration {
		s.elapsedDuration = curElapsed
	}
	s.isCompleted = true
	s.status = ParticipantStatusSpectating
	if s.completionReason == "" {
		s.completionReason = reason
	}
}

// Status returns current ParticipantStatus.
func (s *SoloRefereeSession) Status() ParticipantStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.status
}

// IsCompleted returns whether this participant has concluded their workout.
func (s *SoloRefereeSession) IsCompleted() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.isCompleted
}

// CompletionReason returns why the session completed (e.g. "TIMEOUT", "FORM_VIOLATION_5S").
func (s *SoloRefereeSession) CompletionReason() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.completionReason
}

// SetCompletionReason sets or updates the completion reason.
func (s *SoloRefereeSession) SetCompletionReason(reason string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.completionReason = reason
}

// ConfirmedValue returns confirmed reps or duration in seconds.
func (s *SoloRefereeSession) ConfirmedValue() int32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.confirmedValue
}

// AverageFormScore returns the calculated average form score across all frames.
func (s *SoloRefereeSession) AverageFormScore() float32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.scoreCount == 0 {
		return 100.0
	}
	return s.scoreSum / float32(s.scoreCount)
}

func (s *SoloRefereeSession) getElapsedLocked() time.Duration {
	elapsed := s.elapsedDuration
	if !s.startedAt.IsZero() && (s.status == ParticipantStatusActive || s.status == ParticipantStatusSpectating) {
		wallElapsed := time.Since(s.startedAt)
		if wallElapsed > elapsed {
			elapsed = wallElapsed
		}
	}
	if elapsed > s.timeCeiling {
		elapsed = s.timeCeiling
	}
	return elapsed
}

// TotalDurationSeconds returns total elapsed time in seconds.
func (s *SoloRefereeSession) TotalDurationSeconds() int32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return int32(s.getElapsedLocked().Seconds())
}

// RemainingCeilingSeconds returns the countdown seconds until the time ceiling expires.
// It accounts for both frame-accumulated time and wall-clock time so that the ceiling
// countdown progresses even if no frames are sent by the client.
func (s *SoloRefereeSession) RemainingCeilingSeconds() int32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rem := s.timeCeiling - s.getElapsedLocked()
	if rem <= 0 {
		return 0
	}
	return int32((rem + time.Second - time.Nanosecond) / time.Second)
}

// ChallengeType returns the normalized ChallengeType for this session.
func (s *SoloRefereeSession) ChallengeType() vo.ChallengeType {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.challengeType
}

// ExerciseID returns the exercise ID for this session.
func (s *SoloRefereeSession) ExerciseID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.exerciseID
}

// SetStatus directly assigns the session's participant status.
func (s *SoloRefereeSession) SetStatus(status ParticipantStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status = status
	switch status {
	case ParticipantStatusActive:
		if s.startedAt.IsZero() {
			s.startedAt = time.Now()
		}
	case ParticipantStatusWaiting:
		s.startedAt = time.Time{}
	}
}

// Activate transitions a waiting session to ACTIVE.
func (s *SoloRefereeSession) Activate() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.status == ParticipantStatusWaiting {
		s.status = ParticipantStatusActive
		if s.startedAt.IsZero() {
			s.startedAt = time.Now()
		}
	}
}

// StartedAt returns the timestamp when the session became active.
func (s *SoloRefereeSession) StartedAt() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.startedAt
}

// SetStartedAt updates the startedAt timestamp (useful for reconnects or time simulation).
func (s *SoloRefereeSession) SetStartedAt(t time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.startedAt = t
}

// Tick advances elapsed time by a specific delta (useful for deterministic tests).
func (s *SoloRefereeSession) Tick(delta time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if delta > 0 {
		s.elapsedDuration += delta
	}
}

// IsWaiting reports whether the session is currently waiting for opponent.
func (s *SoloRefereeSession) IsWaiting() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.status == ParticipantStatusWaiting
}

// WorkoutSessionID returns the associated workout session aggregate ID in DB.
func (s *SoloRefereeSession) WorkoutSessionID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.workoutSessionID
}

// SetWorkoutSessionID sets the associated workout session aggregate ID in DB.
func (s *SoloRefereeSession) SetWorkoutSessionID(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.workoutSessionID = id
}

// CameraAngle returns the active camera view angle (e.g. "front", "side").
func (s *SoloRefereeSession) CameraAngle() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cameraAngle
}

// SetCameraAngle updates the active camera view angle.
func (s *SoloRefereeSession) SetCameraAngle(angle string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cameraAngle = angle
}

// RestoreProgress restores score and accumulated metrics upon reconnecting to an active match.
func (s *SoloRefereeSession) RestoreProgress(confirmedValue int32, formScore float32) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if confirmedValue > s.confirmedValue {
		s.confirmedValue = confirmedValue
		if s.challengeType == vo.ChallengeTypeDuration {
			s.accumulatedDuration = time.Duration(confirmedValue) * time.Second
			s.elapsedDuration = s.accumulatedDuration
		}
	}
	if formScore > 0 && s.scoreCount == 0 {
		s.scoreSum = formScore
		s.scoreCount = 1
	}
}

// IsFinalized returns whether the session has already been finalized and cleaned up.
func (s *SoloRefereeSession) IsFinalized() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.isFinalized
}

// MarkFinalized marks the session as finalized.
func (s *SoloRefereeSession) MarkFinalized() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.isFinalized = true
}
