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

// MotionRefereeSession coordinates state, time ceilings, scores, biomechanics evaluation, and form violation tracking for an exercise set or match participant.
type MotionRefereeSession struct {
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

	currentAngle   float32
	personDetected bool

	// Persistence filter state for transient form errors (requires >= 3 frames or >= 150ms)
	errorStreakFrames   map[string]int
	errorStreakDuration map[string]time.Duration
	confirmedErrors     []string
	confirmedAudioCue   string

	// Duration challenge specific
	accumulatedDuration          time.Duration
	consecutiveViolationDuration time.Duration
	maxViolationThreshold        time.Duration

	// Rep challenge specific - Strict 3-Phase Sequential Progression
	repPhase           RepPhase
	hasTransitionedIn  bool // Checkpoint 1: REST -> TRANSITION (Bắt đầu chuyển động)
	hasReachedActive   bool // Checkpoint 2: TRANSITION -> ACTIVE (Đạt đỉnh co cơ)
	hasTransitionedOut bool // Checkpoint 3: ACTIVE -> TRANSITION (Đang trở về)
	phaseFrames        int  // Số frame đã đi qua kể từ khi rời REST
	phaseDuration      time.Duration
	phaseDetection     *vo.PhaseDetectionConfig
}

// NewMotionRefereeSession creates a new referee session for an exercise and challenge type.
// It strictly requires a valid, non-nil ExerciseMotionRule (and non-nil PhaseDetection for REPS challenges).
// SoloRefereeSession is a type alias for MotionRefereeSession for backward compatibility.
type SoloRefereeSession = MotionRefereeSession

// NewSoloRefereeSession is an alias for NewMotionRefereeSession for backward compatibility.
func NewSoloRefereeSession(
	exerciseID string,
	challengeType vo.ChallengeType,
	timeCeiling time.Duration,
	rule *vo.ExerciseMotionRule,
) (*MotionRefereeSession, error) {
	return NewMotionRefereeSession(exerciseID, challengeType, timeCeiling, rule)
}

func NewMotionRefereeSession(
	exerciseID string,
	challengeType vo.ChallengeType,
	timeCeiling time.Duration,
	rule *vo.ExerciseMotionRule,
) (*MotionRefereeSession, error) {
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

	return &MotionRefereeSession{
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
		errorStreakFrames:     make(map[string]int),
		errorStreakDuration:   make(map[string]time.Duration),
	}, nil
}

// ProcessFrame updates the session state given the evaluation result and the elapsed time since last frame.
func (s *MotionRefereeSession) ProcessFrame(result vo.EvaluationResult, delta time.Duration) {
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

	isNoPerson := len(result.DetectedErrors) == 1 && result.DetectedErrors[0] == "ERR_NO_PERSON_DETECTED"
	s.personDetected = !isNoPerson
	if result.CalculatedAngle > 0 {
		s.currentAngle = result.CalculatedAngle
	}

	// Apply Persistence Filter (K >= 3 frames or >= 150ms) to suppress 1-2 frame camera glitches
	filteredResult := s.applyPersistenceFilterLocked(result, effectiveDelta, isNoPerson)

	// Update average form score for all evaluated frames with a person present
	if filteredResult.FormScore >= 0 && !isNoPerson {
		s.scoreSum += filteredResult.FormScore
		s.scoreCount++
	}

	if s.challengeType == vo.ChallengeTypeDuration {
		s.processDurationFrame(filteredResult, effectiveDelta)
	} else {
		s.processRepFrame(filteredResult, effectiveDelta)
	}

	// Check time ceiling limit
	if isLastFrame || s.elapsedDuration >= s.timeCeiling {
		s.elapsedDuration = s.timeCeiling
		s.isCompleted = true
		s.status = ParticipantStatusCompleted
		if s.completionReason == "" {
			s.completionReason = "TIMEOUT"
		}
		if s.challengeType == vo.ChallengeTypeDuration && filteredResult.Passed {
			if s.accumulatedDuration+delta >= s.timeCeiling {
				s.accumulatedDuration = s.timeCeiling
				s.confirmedValue = int32(s.timeCeiling.Seconds())
			}
		}
	}
}

func (s *MotionRefereeSession) applyPersistenceFilterLocked(
	result vo.EvaluationResult,
	delta time.Duration,
	isNoPerson bool,
) vo.EvaluationResult {
	if isNoPerson {
		s.confirmedErrors = result.DetectedErrors
		s.confirmedAudioCue = result.AudioCue
		return result
	}

	if len(result.DetectedErrors) == 0 {
		for k := range s.errorStreakFrames {
			delete(s.errorStreakFrames, k)
			delete(s.errorStreakDuration, k)
		}
		s.confirmedErrors = nil
		s.confirmedAudioCue = ""
		return result
	}

	activeSet := make(map[string]bool, len(result.DetectedErrors))
	var persisted []string
	for _, code := range result.DetectedErrors {
		activeSet[code] = true
		s.errorStreakFrames[code]++
		s.errorStreakDuration[code] += delta
		if s.errorStreakFrames[code] >= 3 || s.errorStreakDuration[code] >= 150*time.Millisecond {
			persisted = append(persisted, code)
		}
	}

	for k := range s.errorStreakFrames {
		if !activeSet[k] {
			delete(s.errorStreakFrames, k)
			delete(s.errorStreakDuration, k)
		}
	}

	s.confirmedErrors = persisted
	if len(persisted) == 0 {
		s.confirmedAudioCue = ""
		return vo.EvaluationResult{
			Passed:          true,
			FormScore:       100.0,
			DetectedErrors:  nil,
			AudioCue:        "",
			CalculatedAngle: result.CalculatedAngle,
		}
	}

	s.confirmedAudioCue = result.AudioCue
	return vo.EvaluationResult{
		Passed:          false,
		FormScore:       result.FormScore,
		DetectedErrors:  persisted,
		AudioCue:        result.AudioCue,
		CalculatedAngle: result.CalculatedAngle,
	}
}

func (s *MotionRefereeSession) processDurationFrame(result vo.EvaluationResult, delta time.Duration) {
	// If PhaseDetection is configured for a duration exercise (e.g. Plank idle > 35 deg vs always <= 35 deg),
	// pause accumulation while the user is still standing/getting into position (Idle).
	if s.phaseDetection != nil && result.CalculatedAngle > 0 {
		idleTh := s.phaseDetection.Thresholds.Idle
		if (idleTh.GT > 0 || idleTh.GTE > 0 || idleTh.LT > 0 || idleTh.LTE > 0) && idleTh.Matches(result.CalculatedAngle) {
			s.repPhase = RepPhaseRest
			return
		}
		s.repPhase = RepPhaseActive
	}

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

func (s *MotionRefereeSession) processRepFrame(result vo.EvaluationResult, delta time.Duration) {
	angle := result.CalculatedAngle
	if angle <= 0 || s.phaseDetection == nil {
		return
	}

	// Phân loại vùng góc (Rest, Transition, Active) dựa trên cấu hình ngưỡng từ Rule JSON
	th := s.phaseDetection.Thresholds
	isRest := th.Rest.Matches(angle)
	isActive := th.Active.Matches(angle) && !isRest
	isTransition := th.Transition.Matches(angle) && !isRest && !isActive

	// Strict Sequential State Machine (REST -> TRANSITION -> ACTIVE -> TRANSITION -> REST)
	switch s.repPhase {
	case RepPhaseRest:
		if isTransition {
			// Bắt đầu chu trình: Từ REST đi vào TRANSITION
			s.repPhase = RepPhaseTransition
			s.hasTransitionedIn = true
			s.hasReachedActive = false
			s.hasTransitionedOut = false
			s.phaseFrames = 1
			s.phaseDuration = delta
		} else if isActive {
			// Bỏ qua transition: vi phạm trình tự (nhảy cóc), không đánh dấu hợp lệ
			s.repPhase = RepPhaseActive
			s.hasTransitionedIn = false
			s.hasReachedActive = false
			s.hasTransitionedOut = false
			s.phaseFrames = 1
			s.phaseDuration = delta
		}

	case RepPhaseTransition:
		s.phaseFrames++
		s.phaseDuration += delta
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
			s.phaseFrames = 0
			s.phaseDuration = 0
		} else if isTransition {
			// Đang duy trì trong dải chuyển động:
			// Nếu đã từng chạm Active, đánh dấu đang trong giai đoạn Returning Transition
			if s.hasReachedActive {
				s.hasTransitionedOut = true
			}
		}

	case RepPhaseActive:
		s.phaseFrames++
		s.phaseDuration += delta
		if isTransition {
			// Rời khỏi đỉnh Active bắt đầu trở về: ghi nhận mốc di chuyển ra
			if s.hasReachedActive {
				s.hasTransitionedOut = true
			}
			s.repPhase = RepPhaseTransition
		} else if isRest {
			// Nếu đã đi qua Transition vào + đạt Active và chu trình đủ dài (>= 8 frames và >= 400ms),
			// cho phép hoàn tất rep khi video 15fps bước qua dải Transition hẹp ở pha lên.
			// Nếu nhảy cóc tức thời (<= 4 frames), coi là nhiễu nhảy cóc và hủy rep.
			if s.hasTransitionedIn && s.hasReachedActive && s.phaseFrames >= 8 && s.phaseDuration >= 400*time.Millisecond {
				s.confirmedValue++
			}
			s.repPhase = RepPhaseRest
			s.hasTransitionedIn = false
			s.hasReachedActive = false
			s.hasTransitionedOut = false
			s.phaseFrames = 0
			s.phaseDuration = 0
		} else if isActive {
			// Nếu 1 frame nhiễu camera làm nhảy qua Transition vào nhưng người tập thực sự giữ ở đáy Active
			// liên tục từ 15 frames trở lên (>= 1.0s), công nhận đã đạt đỉnh Active để khi đi qua Transition ra vẫn tính rep.
			if !s.hasReachedActive && s.phaseFrames >= 15 {
				s.hasTransitionedIn = true
				s.hasReachedActive = true
			}
		}
	}
}

// RepPhase returns the current active phase of the rep cycle.
func (s *MotionRefereeSession) RepPhase() RepPhase {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.repPhase
}

// CurrentAngle returns the latest calculated primary joint angle in degrees.
func (s *MotionRefereeSession) CurrentAngle() float32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.currentAngle
}

// PersonDetected returns whether a person was detected in the latest frame.
func (s *MotionRefereeSession) PersonDetected() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.personDetected
}

// ConfirmedErrors returns the persistence-filtered errors for the latest frame.
func (s *MotionRefereeSession) ConfirmedErrors() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.confirmedErrors
}

// ConfirmedAudioCue returns the persistence-filtered audio cue for the latest frame.
func (s *MotionRefereeSession) ConfirmedAudioCue() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.confirmedAudioCue
}

// MarkCompleted transitions session to COMPLETED.
func (s *MotionRefereeSession) MarkCompleted(reason string) {
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
func (s *MotionRefereeSession) MarkSpectating(reason string) {
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
func (s *MotionRefereeSession) Status() ParticipantStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.status
}

// IsCompleted returns whether this participant has concluded their workout.
func (s *MotionRefereeSession) IsCompleted() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.isCompleted
}

// CompletionReason returns why the session completed (e.g. "TIMEOUT", "FORM_VIOLATION_5S").
func (s *MotionRefereeSession) CompletionReason() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.completionReason
}

// SetCompletionReason sets or updates the completion reason.
func (s *MotionRefereeSession) SetCompletionReason(reason string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.completionReason = reason
}

// ConfirmedValue returns confirmed reps or duration in seconds.
func (s *MotionRefereeSession) ConfirmedValue() int32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.confirmedValue
}

// AverageFormScore returns the calculated average form score across all frames.
func (s *MotionRefereeSession) AverageFormScore() float32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.scoreCount == 0 {
		return 100.0
	}
	return s.scoreSum / float32(s.scoreCount)
}

func (s *MotionRefereeSession) getElapsedLocked() time.Duration {
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
func (s *MotionRefereeSession) TotalDurationSeconds() int32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return int32(s.getElapsedLocked().Seconds())
}

// RemainingCeilingSeconds returns the countdown seconds until the time ceiling expires.
// It accounts for both frame-accumulated time and wall-clock time so that the ceiling
// countdown progresses even if no frames are sent by the client.
func (s *MotionRefereeSession) RemainingCeilingSeconds() int32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rem := s.timeCeiling - s.getElapsedLocked()
	if rem <= 0 {
		return 0
	}
	return int32((rem + time.Second - time.Nanosecond) / time.Second)
}

// ChallengeType returns the normalized ChallengeType for this session.
func (s *MotionRefereeSession) ChallengeType() vo.ChallengeType {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.challengeType
}

// ExerciseID returns the exercise ID for this session.
func (s *MotionRefereeSession) ExerciseID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.exerciseID
}

// SetStatus directly assigns the session's participant status.
func (s *MotionRefereeSession) SetStatus(status ParticipantStatus) {
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
func (s *MotionRefereeSession) Activate() {
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
func (s *MotionRefereeSession) StartedAt() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.startedAt
}

// SetStartedAt updates the startedAt timestamp (useful for reconnects or time simulation).
func (s *MotionRefereeSession) SetStartedAt(t time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.startedAt = t
}

// Tick advances elapsed time by a specific delta (useful for deterministic tests).
func (s *MotionRefereeSession) Tick(delta time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if delta > 0 {
		s.elapsedDuration += delta
	}
}

// IsWaiting reports whether the session is currently waiting for opponent.
func (s *MotionRefereeSession) IsWaiting() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.status == ParticipantStatusWaiting
}

// WorkoutSessionID returns the associated workout session aggregate ID in DB.
func (s *MotionRefereeSession) WorkoutSessionID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.workoutSessionID
}

// SetWorkoutSessionID sets the associated workout session aggregate ID in DB.
func (s *MotionRefereeSession) SetWorkoutSessionID(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.workoutSessionID = id
}

// CameraAngle returns the active camera view angle (e.g. "front", "side").
func (s *MotionRefereeSession) CameraAngle() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cameraAngle
}

// SetCameraAngle updates the active camera view angle.
func (s *MotionRefereeSession) SetCameraAngle(angle string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cameraAngle = angle
}

// RestoreProgress restores score and accumulated metrics upon reconnecting to an active match.
func (s *MotionRefereeSession) RestoreProgress(confirmedValue int32, formScore float32) {
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
func (s *MotionRefereeSession) IsFinalized() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.isFinalized
}

// MarkFinalized marks the session as finalized.
func (s *MotionRefereeSession) MarkFinalized() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.isFinalized = true
}
