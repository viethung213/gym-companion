package aggregate

import (
	"fmt"
	"time"

	"github.com/viethung213/gym-companion/internal/workout_execution/domain/derror"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/event"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/vo"
)

// SessionStatus defines the lifecycle status of a WorkoutSession.
type SessionStatus int

const (
	StatusScheduled SessionStatus = iota + 1
	StatusInProgress
	StatusCompleted
	StatusAborted
	StatusAnomalous
)

func (s SessionStatus) String() string {
	switch s {
	case StatusScheduled:
		return "SCHEDULED"
	case StatusInProgress:
		return "IN_PROGRESS"
	case StatusCompleted:
		return "COMPLETED"
	case StatusAborted:
		return "ABORTED"
	case StatusAnomalous:
		return "ANOMALOUS"
	default:
		return "UNKNOWN"
	}
}

// ParseSessionStatus converts string representation to SessionStatus.
func ParseSessionStatus(str string) SessionStatus {
	switch str {
	case "SCHEDULED":
		return StatusScheduled
	case "IN_PROGRESS":
		return StatusInProgress
	case "COMPLETED":
		return StatusCompleted
	case "ABORTED":
		return StatusAborted
	case "ANOMALOUS":
		return StatusAnomalous
	default:
		return StatusInProgress
	}
}

// WorkoutSetLog represents a set performed in a session.
type WorkoutSetLog struct {
	ID          string
	SessionID   string
	SetNumber   int
	ExerciseID  string
	TargetReps  int
	ActualReps  int
	Weight      float32
	FormScore   *float32 // nil if non-AI
	RPE         float32
	CameraAngle string
	Reps        []vo.RepLog
	CreatedAt   time.Time
}

// DeepCopy returns a deep copy of WorkoutSetLog, cloning pointer and slice fields.
func (l WorkoutSetLog) DeepCopy() WorkoutSetLog {
	cp := l
	if l.FormScore != nil {
		fs := *l.FormScore
		cp.FormScore = &fs
	}
	if len(l.Reps) > 0 {
		cp.Reps = make([]vo.RepLog, len(l.Reps))
		for i, r := range l.Reps {
			cp.Reps[i] = vo.NewRepLog(r.RepNumber, r.ROMPercentage, r.GetErrorCodes(), r.GetJointAngles())
		}
	} else if l.Reps != nil {
		cp.Reps = make([]vo.RepLog, 0)
	}
	return cp
}

// SessionError represents posture errors logged during an AI set.
type SessionError struct {
	ID         string
	SessionID  string
	SetNumber  int
	RepNumber  int
	ExerciseID string
	ErrorCode  string
	Severity   string
	Timestamp  time.Time
}

// WorkoutSession is the aggregate root controlling a workout execution lifecycle.
type WorkoutSession struct {
	id           string
	userID       string
	planID       string
	status       SessionStatus
	sets         []WorkoutSetLog
	errors       []SessionError
	scheduledAt  *time.Time
	startedAt    *time.Time
	endedAt      *time.Time
	createdAt    time.Time
	updatedAt    time.Time
	version      int
	domainEvents []interface{}
}

// NewWorkoutSession initializes a new WorkoutSession immediately in IN_PROGRESS state (JIT).
func NewWorkoutSession(id, userID, planID string) (*WorkoutSession, error) {
	if id == "" || userID == "" || planID == "" {
		return nil, fmt.Errorf("id, userID and planID must not be empty: %w", derror.ErrInvalidSetNumber)
	}

	now := time.Now().UTC()
	session := &WorkoutSession{
		id:        id,
		userID:    userID,
		planID:    planID,
		status:    StatusInProgress,
		sets:      make([]WorkoutSetLog, 0),
		errors:    make([]SessionError, 0),
		startedAt: &now,
		createdAt: now,
		updatedAt: now,
		version:   1,
	}

	session.addDomainEvent(&event.WorkoutSessionStarted{
		SessionID: id,
		UserID:    userID,
		PlanID:    planID,
		StartedAt: now,
	})

	return session, nil
}

// NewScheduledWorkoutSession initializes a new WorkoutSession in SCHEDULED state.
func NewScheduledWorkoutSession(id, userID, planID string, scheduledAt time.Time) (*WorkoutSession, error) {
	if id == "" || userID == "" || planID == "" {
		return nil, fmt.Errorf("id, userID and planID must not be empty: %w", derror.ErrInvalidSetNumber)
	}

	now := time.Now().UTC()
	return &WorkoutSession{
		id:          id,
		userID:      userID,
		planID:      planID,
		status:      StatusScheduled,
		sets:        make([]WorkoutSetLog, 0),
		errors:      make([]SessionError, 0),
		scheduledAt: &scheduledAt,
		startedAt:   nil,
		createdAt:   now,
		updatedAt:   now,
		version:     1,
	}, nil
}

// ReconstituteWorkoutSession restores a WorkoutSession from persistent storage.
func ReconstituteWorkoutSession(
	id, userID, planID string,
	status SessionStatus,
	sets []WorkoutSetLog,
	errors []SessionError,
	scheduledAt *time.Time,
	startedAt *time.Time,
	endedAt *time.Time,
	createdAt, updatedAt time.Time,
	version ...int,
) *WorkoutSession {
	v := 1
	if len(version) > 0 && version[0] > 0 {
		v = version[0]
	}
	return &WorkoutSession{
		id:          id,
		userID:      userID,
		planID:      planID,
		status:      status,
		sets:        sets,
		errors:      errors,
		scheduledAt: scheduledAt,
		startedAt:   startedAt,
		endedAt:     endedAt,
		createdAt:   createdAt,
		updatedAt:   updatedAt,
		version:     v,
	}
}

// ID returns session ID.
func (s *WorkoutSession) ID() string { return s.id }

// UserID returns user ID.
func (s *WorkoutSession) UserID() string { return s.userID }

// PlanID returns plan ID.
func (s *WorkoutSession) PlanID() string { return s.planID }

// Status returns current status.
func (s *WorkoutSession) Status() SessionStatus { return s.status }

// Version returns current optimistic locking version of the session aggregate.
func (s *WorkoutSession) Version() int {
	if s.version <= 0 {
		return 1
	}
	return s.version
}

// ScheduledAt returns defensive copy of planned time.
func (s *WorkoutSession) ScheduledAt() *time.Time {
	if s.scheduledAt == nil {
		return nil
	}
	t := *s.scheduledAt
	return &t
}

// StartedAt returns defensive copy of start time.
func (s *WorkoutSession) StartedAt() *time.Time {
	if s.startedAt == nil {
		return nil
	}
	t := *s.startedAt
	return &t
}

// Start transitions session from SCHEDULED to IN_PROGRESS.
func (s *WorkoutSession) Start() error {
	if s.status == StatusInProgress {
		return nil
	}
	if s.status != StatusScheduled {
		return derror.ErrSessionNotInProgress
	}
	now := time.Now().UTC()
	s.status = StatusInProgress
	s.startedAt = &now
	s.updatedAt = now

	s.addDomainEvent(&event.WorkoutSessionStarted{
		SessionID: s.id,
		UserID:    s.userID,
		PlanID:    s.planID,
		StartedAt: now,
	})
	return nil
}

// EndedAt returns defensive copy of end time.
func (s *WorkoutSession) EndedAt() *time.Time {
	if s.endedAt == nil {
		return nil
	}
	t := *s.endedAt
	return &t
}

// CreatedAt returns creation time.
func (s *WorkoutSession) CreatedAt() time.Time { return s.createdAt }

// UpdatedAt returns update time.
func (s *WorkoutSession) UpdatedAt() time.Time { return s.updatedAt }

// Sets returns defensive copy of set logs.
func (s *WorkoutSession) Sets() []WorkoutSetLog {
	if len(s.sets) == 0 {
		return nil
	}
	res := make([]WorkoutSetLog, len(s.sets))
	for i, set := range s.sets {
		res[i] = set.DeepCopy()
	}
	return res
}

// Errors returns defensive copy of session errors.
func (s *WorkoutSession) Errors() []SessionError {
	if len(s.errors) == 0 {
		return nil
	}
	res := make([]SessionError, len(s.errors))
	copy(res, s.errors)
	return res
}

// PopEvents returns and clears recorded domain events.
func (s *WorkoutSession) PopEvents() []interface{} {
	events := s.domainEvents
	s.domainEvents = nil
	return events
}

func (s *WorkoutSession) addDomainEvent(ev interface{}) {
	s.domainEvents = append(s.domainEvents, ev)
}

// CheckTimeoutAndAutoAbort checks if session has exceeded 240 minutes without activity.
func (s *WorkoutSession) CheckTimeoutAndAutoAbort(now time.Time) bool {
	if s.status != StatusInProgress {
		return false
	}

	if s.startedAt != nil && now.Sub(*s.startedAt) > 240*time.Minute {
		s.status = StatusAnomalous
		ended := now
		s.endedAt = &ended
		s.updatedAt = now
		s.addDomainEvent(&event.WorkoutSessionAborted{
			SessionID:   s.id,
			UserID:      s.userID,
			PlanID:      s.planID,
			Reason:      "Inactive session exceeded 240 minutes limit",
			IsAnomalous: true,
			AbortedAt:   now,
		})
		return true
	}
	return false
}

// LogSet adds a completed set to the session.
func (s *WorkoutSession) LogSet(setLog WorkoutSetLog) error {
	if s.status != StatusInProgress {
		return derror.ErrSessionNotInProgress
	}

	if s.CheckTimeoutAndAutoAbort(time.Now().UTC()) {
		return derror.ErrAnomalousSessionTimeout
	}

	if setLog.SetNumber <= 0 {
		return derror.ErrInvalidSetNumber
	}

	setLog.SessionID = s.id
	if setLog.CreatedAt.IsZero() {
		setLog.CreatedAt = time.Now().UTC()
	}

	s.sets = append(s.sets, setLog.DeepCopy())
	s.updatedAt = time.Now().UTC()
	return nil
}

// AddErrors appends posture errors to the session.
func (s *WorkoutSession) AddErrors(errs []SessionError) {
	for _, e := range errs {
		e.SessionID = s.id
		if e.Timestamp.IsZero() {
			e.Timestamp = time.Now().UTC()
		}
		s.errors = append(s.errors, e)
	}
	s.updatedAt = time.Now().UTC()
}

// CalculateTotalVolume sums weight * actualReps for all sets.
func (s *WorkoutSession) CalculateTotalVolume() float32 {
	var vol float32
	for i := range s.sets {
		vol += s.sets[i].Weight * float32(s.sets[i].ActualReps)
	}
	return vol
}

// CalculateSummary aggregates session stats.
func (s *WorkoutSession) CalculateSummary() vo.SessionSummary {
	totalSets := len(s.sets)
	if totalSets == 0 {
		return vo.NewSessionSummary(0, 0, nil, 0)
	}

	var totalVolume float32
	var totalRPE float32
	var totalFormScore float32
	var aiSetCount int

	for i := range s.sets {
		set := &s.sets[i]
		totalVolume += set.Weight * float32(set.ActualReps)
		totalRPE += set.RPE
		if set.FormScore != nil {
			totalFormScore += *set.FormScore
			aiSetCount++
		}
	}

	avgRPE := totalRPE / float32(totalSets)
	var avgFormScore *float32
	if aiSetCount > 0 {
		score := totalFormScore / float32(aiSetCount)
		avgFormScore = &score
	}

	return vo.NewSessionSummary(totalSets, totalVolume, avgFormScore, avgRPE)
}

// Complete transitions status to COMPLETED and emits WorkoutSessionCompleted event.
func (s *WorkoutSession) Complete(confirmOverload, isOverloaded bool) error {
	if s.status != StatusInProgress {
		return derror.ErrSessionNotInProgress
	}

	now := time.Now().UTC()
	if s.CheckTimeoutAndAutoAbort(now) {
		return derror.ErrAnomalousSessionTimeout
	}

	if isOverloaded && !confirmOverload {
		return derror.ErrOverloadConfirmationRequired
	}

	s.status = StatusCompleted
	s.endedAt = &now
	s.updatedAt = now

	summary := s.CalculateSummary()
	s.addDomainEvent(&event.WorkoutSessionCompleted{
		SessionID:   s.id,
		UserID:      s.userID,
		PlanID:      s.planID,
		CompletedAt: now,
		Summary:     summary,
	})

	return nil
}

// RecordBodyMetricUpdate emits a BodyMetricUpdated event if body weight update is provided by user.
func (s *WorkoutSession) RecordBodyMetricUpdate(weightKg float32) {
	if weightKg <= 0 {
		return
	}
	s.addDomainEvent(&event.BodyMetricUpdated{
		UserID:     s.userID,
		WeightKg:   weightKg,
		RecordedAt: time.Now().UTC(),
	})
}

// Abort transitions status to ABORTED.
func (s *WorkoutSession) Abort(reason string) error {
	if s.status == StatusCompleted || s.status == StatusAborted || s.status == StatusAnomalous {
		return derror.ErrSessionAlreadyCompleted
	}

	now := time.Now().UTC()
	s.status = StatusAborted
	s.endedAt = &now
	s.updatedAt = now

	s.addDomainEvent(&event.WorkoutSessionAborted{
		SessionID:   s.id,
		UserID:      s.userID,
		PlanID:      s.planID,
		Reason:      reason,
		IsAnomalous: false,
		AbortedAt:   now,
	})

	return nil
}

// AbortAnomalous transitions status to ANOMALOUS due to safety/critical emergency errors.
func (s *WorkoutSession) AbortAnomalous(reason string) error {
	if s.status == StatusCompleted || s.status == StatusAborted || s.status == StatusAnomalous {
		return derror.ErrSessionAlreadyCompleted
	}

	now := time.Now().UTC()
	s.status = StatusAnomalous
	s.endedAt = &now
	s.updatedAt = now

	s.addDomainEvent(&event.WorkoutSessionAborted{
		SessionID:   s.id,
		UserID:      s.userID,
		PlanID:      s.planID,
		Reason:      reason,
		IsAnomalous: true,
		AbortedAt:   now,
	})

	return nil
}

// MarkCriticalInactivity transitions status to ANOMALOUS when a critical
// posture error was the last recorded event and no user interaction has
// occurred for the configured inactivity threshold.
// lastCriticalAt is the timestamp of the most recent critical error.
func (s *WorkoutSession) MarkCriticalInactivity(now, lastCriticalAt time.Time) error {
	if s.status != StatusInProgress {
		return derror.ErrSessionNotInProgress
	}

	s.status = StatusAnomalous
	ended := now
	s.endedAt = &ended
	s.updatedAt = now

	reason := "CRITICAL_INACTIVITY: no user interaction after critical posture error at " +
		lastCriticalAt.UTC().Format(time.RFC3339)
	s.addDomainEvent(&event.WorkoutSessionAborted{
		SessionID:   s.id,
		UserID:      s.userID,
		PlanID:      s.planID,
		Reason:      reason,
		IsAnomalous: true,
		AbortedAt:   now,
	})

	return nil
}
