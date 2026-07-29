package roadmap

import (
	"fmt"
	"strings"
	"time"
)

// WeekPlanInfo is the value-object snapshot of a WeekPlan entity.

type WeekPlanInfo struct {
	WeekPlanID string

	RoadmapID string

	UserID string

	WeekNumber int32

	Phase Phase

	TargetRPE float32

	StartDate time.Time

	EndDate time.Time

	MuscleSplitType string
}

// WeekPlan is an entity holding 7 DayPlans for one training week.

type WeekPlan struct {
	info WeekPlanInfo

	days []*DayPlan
}

// NewWeekPlan constructs an empty WeekPlan.

func NewWeekPlan(info *WeekPlanInfo) (*WeekPlan, error) {

	if info == nil {

		return nil, fmt.Errorf("%w: nil WeekPlanInfo", ErrInvalidRoadmap)

	}

	normInfo := normalizeWeekInfo(info)

	wp := &WeekPlan{info: *normInfo, days: nil}

	if err := wp.validate(); err != nil {

		return nil, err

	}

	return wp, nil

}

// RehydrateWeekPlan reconstructs a WeekPlan with its days.

func RehydrateWeekPlan(info *WeekPlanInfo, days []*DayPlan) (*WeekPlan, error) {

	if info == nil {

		return nil, fmt.Errorf("%w: nil WeekPlanInfo", ErrInvalidRoadmap)

	}

	normInfo := normalizeWeekInfo(info)

	wp := &WeekPlan{info: *normInfo, days: days}

	if err := wp.validate(); err != nil {

		return nil, err

	}

	return wp, nil

}

// Info returns the week plan snapshot.

func (w *WeekPlan) Info() WeekPlanInfo { return w.info }

// ID returns the week plan identifier.

func (w *WeekPlan) ID() string { return w.info.WeekPlanID }

// WeekNumber returns 1..4.

func (w *WeekPlan) WeekNumber() int32 { return w.info.WeekNumber }

// Phase returns the periodization phase of this week.

func (w *WeekPlan) Phase() Phase { return w.info.Phase }

// Days returns the day plans.

func (w *WeekPlan) Days() []*DayPlan {

	if w.days == nil {

		return nil

	}

	out := make([]*DayPlan, len(w.days))

	copy(out, w.days)

	return out

}

// AddDay appends a DayPlan (invariant: max 7 days, unique scheduled_date).

func (w *WeekPlan) AddDay(day *DayPlan) error {

	if day == nil {

		return fmt.Errorf("%w: nil day plan", ErrInvalidRoadmap)

	}

	if len(w.days) >= DaysPerWeek {

		return fmt.Errorf("%w: week already has %d days", ErrInvalidRoadmap, DaysPerWeek)

	}

	for _, d := range w.days {

		if d.info.ScheduledDate.Equal(day.info.ScheduledDate) {

			return fmt.Errorf("%w: duplicate scheduled_date %s", ErrInvalidRoadmap, day.info.ScheduledDate.Format("2006-01-02"))

		}

	}

	// Enforce BR-AC-01 after add if the new day has sessions.

	newTotal := w.totalSessions() + day.SessionCount()

	if newTotal > MaxSessionsPerWeek {

		return fmt.Errorf("%w: would total %d sessions (cap %d)", ErrWeeklyCapExceeded, newTotal, MaxSessionsPerWeek)

	}

	w.days = append(w.days, day)

	return nil

}

// TotalSessions returns the total non-rest sessions across all days.

func (w *WeekPlan) TotalSessions() int { return w.totalSessions() }

func (w *WeekPlan) totalSessions() int {

	total := 0

	for _, d := range w.days {

		total += d.SessionCount()

	}

	return total

}

// FindSession searches all days for a session by ID.

func (w *WeekPlan) FindSession(sessionPlanID string) (*SessionPlan, bool) {

	for _, d := range w.days {

		if s, ok := d.FindSession(sessionPlanID); ok {

			return s, true

		}

	}

	return nil, false

}

func (w *WeekPlan) validate() error {

	i := w.info

	if i.WeekPlanID == "" {

		return fmt.Errorf("%w: week_plan_id is required", ErrInvalidRoadmap)

	}

	if i.UserID == "" {

		return fmt.Errorf("%w: user_id is required", ErrInvalidRoadmap)

	}

	if i.WeekNumber < 1 || i.WeekNumber > WeeksPerRoadmap {

		return fmt.Errorf("%w: week_number must be 1..%d", ErrInvalidRoadmap, WeeksPerRoadmap)

	}

	if !i.Phase.Valid() {

		return fmt.Errorf("%w: %s", ErrInvalidPhase, i.Phase)

	}

	if i.StartDate.IsZero() || i.EndDate.IsZero() {

		return fmt.Errorf("%w: start/end date required", ErrInvalidRoadmap)

	}

	if w.totalSessions() > MaxSessionsPerWeek {

		return fmt.Errorf("%w: %d sessions", ErrWeeklyCapExceeded, w.totalSessions())

	}

	return nil

}

func normalizeWeekInfo(info *WeekPlanInfo) *WeekPlanInfo {

	if info == nil {

		return &WeekPlanInfo{}

	}

	cp := *info

	cp.WeekPlanID = strings.TrimSpace(cp.WeekPlanID)

	cp.RoadmapID = strings.TrimSpace(cp.RoadmapID)

	cp.UserID = strings.TrimSpace(cp.UserID)

	cp.MuscleSplitType = strings.TrimSpace(cp.MuscleSplitType)

	return &cp

}
