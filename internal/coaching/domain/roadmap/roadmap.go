package roadmap

import (
	"fmt"
	"strings"
	"time"
)

// Info is the value-object snapshot of the Roadmap aggregate root.
type Info struct {
	RoadmapID string
	UserID    string
	Status    Status
	StartDate time.Time
	EndDate   time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
	// Timezone is the IANA zone the schedule's wall clock is in (see Location).
	Timezone string
	// Version counts the saves of this roadmap. A save that does not start
	// from the stored version is refused (ErrRoadmapChanged): it was built on a
	// copy that another writer has since changed. 0 for a roadmap never saved.
	Version int64
}

// Roadmap is the aggregate root managing a 4-week training program.
type Roadmap struct {
	info  Info
	weeks []*WeekPlan

	// removedSessions / removedDays are the children ReschedulePending dropped
	// since load; persistence deletes exactly these rows.
	removedSessions []string
	removedDays     []string
}

// NewRoadmap constructs a new ACTIVE roadmap with the given weeks attached.
// weeks may be nil or partial; final validation happens on Save (Persist).
func NewRoadmap(info *Info, weeks []*WeekPlan, now time.Time) (*Roadmap, error) {
	if info == nil {
		return nil, fmt.Errorf("%w: nil Info", ErrInvalidRoadmap)
	}

	normInfo := normalizeRoadmapInfo(info)

	normInfo.Status = StatusActive

	normInfo.CreatedAt = now

	normInfo.UpdatedAt = now

	r := &Roadmap{info: *normInfo, weeks: weeks}

	if err := r.validate(); err != nil {
		return nil, err
	}

	return r, nil
}

// RehydrateRoadmap loads a Roadmap from persistence with its children.
func RehydrateRoadmap(info *Info, weeks []*WeekPlan) (*Roadmap, error) {
	if info == nil {
		return nil, fmt.Errorf("%w: nil Info", ErrInvalidRoadmap)
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
func (r *Roadmap) Info() Info { return r.info }

// ID returns the roadmap identifier.
func (r *Roadmap) ID() string { return r.info.RoadmapID }

// UserID returns the owning user identifier.
func (r *Roadmap) UserID() string { return r.info.UserID }

// Status returns the current lifecycle status.
func (r *Roadmap) Status() Status { return r.info.Status }

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
	if r.info.Status == StatusCompleted {
		return nil
	}

	if r.info.Status != StatusActive {
		return fmt.Errorf("%w: %s to %s", ErrInvalidTransition, r.info.Status, StatusCompleted)
	}

	r.info.Status = StatusCompleted

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

func normalizeRoadmapInfo(info *Info) *Info {
	if info == nil {
		return &Info{}
	}

	cp := *info

	cp.RoadmapID = strings.TrimSpace(cp.RoadmapID)

	cp.UserID = strings.TrimSpace(cp.UserID)

	return &cp
}

// Version is the stored version this copy was read at (see Info.Version).
func (r *Roadmap) Version() int64 { return r.info.Version }

// MarkSaved records that this copy was stored as the given version, so it can
// be saved again.
func (r *Roadmap) MarkSaved(version int64) { r.info.Version = version }

// StampPlannedAt dates a fresh draft by when its planning started, before the
// athlete's profile was read. Every profile change made before that instant is
// reflected in the plan (see RegenerateScheduleCommand.ChangedAt). Only a
// draft never saved can be stamped.
func (r *Roadmap) StampPlannedAt(t time.Time) error {
	if r.info.Version != 0 {
		return fmt.Errorf("%w: roadmap %s is already saved", ErrInvalidTransition, r.info.RoadmapID)
	}
	r.info.CreatedAt = t
	r.info.UpdatedAt = t
	return nil
}
