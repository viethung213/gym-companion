package roadmap

import (
	"fmt"
	"strings"
	"time"
)

// RoadmapInfo is the value-object snapshot of the Roadmap aggregate root.
type RoadmapInfo struct {
	RoadmapID string
	UserID    string
	Status    RoadmapStatus
	StartDate time.Time
	EndDate   time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Roadmap is the aggregate root managing a 4-week training program.
type Roadmap struct {
	info  RoadmapInfo
	weeks []*WeekPlan
}

// NewRoadmap constructs a new ACTIVE roadmap with the given weeks attached.
// weeks may be nil or partial; final validation happens on Save (Persist).
func NewRoadmap(info *RoadmapInfo, weeks []*WeekPlan, now time.Time) (*Roadmap, error) {
	if info == nil {
		return nil, fmt.Errorf("%w: nil RoadmapInfo", ErrInvalidRoadmap)
	}
	normInfo := normalizeRoadmapInfo(info)
	normInfo.Status = RoadmapStatusActive
	normInfo.CreatedAt = now
	normInfo.UpdatedAt = now

	r := &Roadmap{info: *normInfo, weeks: weeks}
	if err := r.validate(); err != nil {
		return nil, err
	}
	return r, nil
}

// RehydrateRoadmap loads a Roadmap from persistence with its children.
func RehydrateRoadmap(info *RoadmapInfo, weeks []*WeekPlan) (*Roadmap, error) {
	if info == nil {
		return nil, fmt.Errorf("%w: nil RoadmapInfo", ErrInvalidRoadmap)
	}
	normInfo := normalizeRoadmapInfo(info)
	if !normInfo.Status.Valid() {
		return nil, fmt.Errorf("%w: %s", ErrInvalidStatus, normInfo.Status)
	}
	r := &Roadmap{info: *normInfo, weeks: weeks}
	if err := r.validate(); err != nil {
		return nil, err
	}
	return r, nil
}

// Info returns the aggregate root snapshot.
func (r *Roadmap) Info() RoadmapInfo { return r.info }

// ID returns the roadmap identifier.
func (r *Roadmap) ID() string { return r.info.RoadmapID }

// UserID returns the owning user identifier.
func (r *Roadmap) UserID() string { return r.info.UserID }

// Status returns the current lifecycle status.
func (r *Roadmap) Status() RoadmapStatus { return r.info.Status }

// Weeks returns the child week plans.
func (r *Roadmap) Weeks() []*WeekPlan {
	if r.weeks == nil {
		return nil
	}
	out := make([]*WeekPlan, len(r.weeks))
	copy(out, r.weeks)
	return out
}

// MarkCompleted transitions ACTIVE → COMPLETED. Idempotent when already COMPLETED.
func (r *Roadmap) MarkCompleted(now time.Time) error {
	if r.info.Status == RoadmapStatusCompleted {
		return nil
	}
	if r.info.Status != RoadmapStatusActive {
		return fmt.Errorf("%w: %s to %s", ErrInvalidTransition, r.info.Status, RoadmapStatusCompleted)
	}
	r.info.Status = RoadmapStatusCompleted
	r.info.UpdatedAt = now
	return nil
}

// Touch bumps updated_at without changing status. Used by mutation handlers.
func (r *Roadmap) Touch(now time.Time) { r.info.UpdatedAt = now }

// FindSession searches all weeks/days for a session by ID.
func (r *Roadmap) FindSession(sessionPlanID string) (*SessionPlan, bool) {
	for _, w := range r.weeks {
		if s, ok := w.FindSession(sessionPlanID); ok {
			return s, true
		}
	}
	return nil, false
}

// PendingSessionsFrom returns all sessions with status PENDING and
// scheduled_date >= from. Used by RegenerateSchedule (D3).
func (r *Roadmap) PendingSessionsFrom(from time.Time) []*SessionPlan {
	var out []*SessionPlan
	for _, w := range r.weeks {
		for _, d := range w.Days() {
			for _, s := range d.Sessions() {
				if s.Status() == SessionPlanStatusPending && !s.ScheduledDate().Before(from) {
					out = append(out, s)
				}
			}
		}
	}
	return out
}

func (r *Roadmap) validate() error {
	i := r.info
	if i.RoadmapID == "" {
		return fmt.Errorf("%w: roadmap_id is required", ErrInvalidRoadmap)
	}
	if i.UserID == "" {
		return fmt.Errorf("%w: user_id is required", ErrInvalidRoadmap)
	}
	if !i.Status.Valid() {
		return fmt.Errorf("%w: %s", ErrInvalidStatus, i.Status)
	}
	if i.StartDate.IsZero() || i.EndDate.IsZero() {
		return fmt.Errorf("%w: start/end date required", ErrInvalidRoadmap)
	}
	if !i.EndDate.After(i.StartDate) {
		return fmt.Errorf("%w: end_date must be after start_date", ErrInvalidRoadmap)
	}
	// Enforce weekly cap per week (defense in depth; primary enforcement in WeekPlan.AddDay).
	for _, w := range r.weeks {
		if w.TotalSessions() > MaxSessionsPerWeek {
			return fmt.Errorf("%w: week %d has %d sessions", ErrWeeklyCapExceeded, w.WeekNumber(), w.TotalSessions())
		}
	}
	return nil
}

// ValidateFullStructure checks that the roadmap has exactly 4 weeks. Called by
// InitiateRoadmapHandler before Save. Rehydration doesn't enforce this so that
// partial reads work.
func (r *Roadmap) ValidateFullStructure() error {
	if len(r.weeks) != WeeksPerRoadmap {
		return fmt.Errorf("%w: has %d weeks", ErrInvalidWeekCount, len(r.weeks))
	}
	return nil
}

func normalizeRoadmapInfo(info *RoadmapInfo) *RoadmapInfo {
	if info == nil {
		return &RoadmapInfo{}
	}
	cp := *info
	cp.RoadmapID = strings.TrimSpace(cp.RoadmapID)
	cp.UserID = strings.TrimSpace(cp.UserID)
	return &cp
}
