package roadmap

import (
	"fmt"
	"strings"
	"time"
)

// DayPlanInfo is the value-object snapshot of a DayPlan entity.
type DayPlanInfo struct {
	DayPlanID     string
	WeekPlanID    string
	RoadmapID     string
	UserID        string
	ScheduledDate time.Time
}

// DayPlan is an entity holding N SessionPlans for a single calendar day.
type DayPlan struct {
	info     DayPlanInfo
	sessions []*SessionPlan
}

// NewDayPlan constructs an empty DayPlan.
func NewDayPlan(info *DayPlanInfo) (*DayPlan, error) {
	if info == nil {
		return nil, fmt.Errorf("%w: nil DayPlanInfo", ErrInvalidRoadmap)
	}

	normInfo := normalizeDayInfo(info)

	dp := &DayPlan{info: *normInfo, sessions: nil}

	if err := dp.validate(); err != nil {
		return nil, err
	}

	return dp, nil
}

// RehydrateDayPlan reconstructs a DayPlan with its sessions.
func RehydrateDayPlan(info *DayPlanInfo, sessions []*SessionPlan) (*DayPlan, error) {
	if info == nil {
		return nil, fmt.Errorf("%w: nil DayPlanInfo", ErrInvalidRoadmap)
	}

	normInfo := normalizeDayInfo(info)

	dp := &DayPlan{info: *normInfo, sessions: sessions}

	if err := dp.validate(); err != nil {
		return nil, err
	}

	return dp, nil
}

// Info returns a deep copy of the day plan snapshot.
func (d *DayPlan) Info() DayPlanInfo { return d.info }

// ID returns the day plan identifier.
func (d *DayPlan) ID() string { return d.info.DayPlanID }

// ScheduledDate returns the calendar date of this day plan.
func (d *DayPlan) ScheduledDate() time.Time { return d.info.ScheduledDate }

// Sessions returns the current sessions.
// Modifying the returned slice does not mutate the aggregate.
func (d *DayPlan) Sessions() []*SessionPlan {
	if d.sessions == nil {
		return nil
	}

	out := make([]*SessionPlan, len(d.sessions))

	copy(out, d.sessions)

	return out
}

// AddSession appends a new SessionPlan.
func (d *DayPlan) AddSession(s *SessionPlan) error {
	if s == nil {
		return fmt.Errorf("%w: nil session plan", ErrInvalidRoadmap)
	}

	d.sessions = append(d.sessions, s)

	return nil
}

// FindSession looks up a session by ID.
func (d *DayPlan) FindSession(sessionPlanID string) (*SessionPlan, bool) {
	for _, s := range d.sessions {
		if s.info.SessionPlanID == sessionPlanID {
			return s, true
		}
	}

	return nil, false
}

// SessionCount returns the number of sessions in this day.
func (d *DayPlan) SessionCount() int { return len(d.sessions) }
func (d *DayPlan) validate() error {
	i := d.info

	if i.DayPlanID == "" {
		return fmt.Errorf("%w: day_plan_id is required", ErrInvalidRoadmap)
	}

	if i.WeekPlanID == "" {
		return fmt.Errorf("%w: week_plan_id is required", ErrInvalidRoadmap)
	}

	if i.UserID == "" {
		return fmt.Errorf("%w: user_id is required", ErrInvalidRoadmap)
	}

	if i.ScheduledDate.IsZero() {
		return fmt.Errorf("%w: scheduled_date is required", ErrInvalidRoadmap)
	}

	return nil
}

func normalizeDayInfo(info *DayPlanInfo) *DayPlanInfo {
	if info == nil {
		return &DayPlanInfo{}
	}

	cp := *info

	cp.DayPlanID = strings.TrimSpace(cp.DayPlanID)

	cp.WeekPlanID = strings.TrimSpace(cp.WeekPlanID)

	cp.RoadmapID = strings.TrimSpace(cp.RoadmapID)

	cp.UserID = strings.TrimSpace(cp.UserID)

	return &cp
}
