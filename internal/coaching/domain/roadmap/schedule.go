package roadmap

import (
	"fmt"
	"strings"
	"time"
)

// DefaultTimezone applies to roadmaps created before the athlete's zone was known.
const DefaultTimezone = "UTC"

// slotClockLayout is the wall-clock layout of a slot start ("HH:MM").
const slotClockLayout = "15:04"

// Location returns the roadmap's time zone. scheduled_date and slot_time are
// wall clock in it; an unknown or empty name falls back to UTC.
func (r *Roadmap) Location() *time.Location {
	return LoadLocation(r.info.Timezone)
}

// SetTimezone records the athlete's current zone; empty keeps the stored one.
func (r *Roadmap) SetTimezone(name string) {
	if name = strings.TrimSpace(name); name != "" {
		r.info.Timezone = name
	}
}

// LoadLocation resolves an IANA name, falling back to UTC.
func LoadLocation(name string) *time.Location {
	if name = strings.TrimSpace(name); name == "" {
		return time.UTC
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return time.UTC
	}
	return loc
}

// CalendarDate returns the calendar day of t in loc, encoded the way
// scheduled_date is stored: midnight UTC of that day.
func CalendarDate(t time.Time, loc *time.Location) time.Time {
	y, m, d := t.In(loc).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// StartsAt is the instant the session begins in loc. A slot_time without a
// parseable "HH:MM" start counts as the start of its day.
func (s *SessionPlan) StartsAt(loc *time.Location) time.Time {
	y, m, d := s.info.ScheduledDate.Date()
	start, _, _ := strings.Cut(s.info.SlotTime, "-")
	clock, err := time.Parse(slotClockLayout, strings.TrimSpace(start))
	if err != nil {
		return time.Date(y, m, d, 0, 0, 0, 0, loc)
	}
	return time.Date(y, m, d, clock.Hour(), clock.Minute(), 0, 0, loc)
}

// UpcomingPendingSessions returns, in schedule order, the PENDING sessions
// that have not started at now. These are the sessions a regenerate or an
// adaptation may still rewrite or move.
func (r *Roadmap) UpcomingPendingSessions(now time.Time) []*SessionPlan {
	loc := r.Location()
	var out []*SessionPlan
	for _, w := range r.weeks {
		for _, d := range w.Days() {
			for _, s := range d.Sessions() {
				if s.Status() == SessionPlanStatusPending && s.StartsAt(loc).After(now) {
					out = append(out, s)
				}
			}
		}
	}
	return out
}

// ReschedulePending replaces every upcoming PENDING session with sessions
// built from drafts, used when the athlete's weekly slots changed. Each draft
// lands in the week whose date range holds its scheduled_date, reusing that
// day's DayPlan or creating one; days left empty are dropped. Finished and
// past sessions are never touched. A draft whose slot has already started, or
// that a kept session already occupies, is skipped: a slot never holds two
// sessions. newID mints session and day identifiers.
func (r *Roadmap) ReschedulePending(now time.Time, drafts []*SessionPlanInfo, newID func() string) error {
	if newID == nil {
		return fmt.Errorf("%w: nil id generator", ErrInvalidRoadmap)
	}

	removed := make(map[string]struct{})
	for _, s := range r.UpcomingPendingSessions(now) {
		removed[s.ID()] = struct{}{}
		r.removedSessions = append(r.removedSessions, s.ID())
	}
	for _, w := range r.weeks {
		r.removedDays = append(r.removedDays, w.removeSessions(removed)...)
	}

	loc := r.Location()
	occupied := make(map[string]bool)
	for _, w := range r.weeks {
		for _, d := range w.days {
			for _, s := range d.sessions {
				occupied[slotKey(s.info.ScheduledDate, s.info.SlotTime)] = true
			}
		}
	}

	for i, draft := range drafts {
		if draft == nil {
			return fmt.Errorf("%w: nil draft at %d", ErrInvalidRoadmap, i)
		}
		date := CalendarDate(draft.ScheduledDate, time.UTC)
		key := slotKey(date, draft.SlotTime)
		probe := SessionPlan{info: SessionPlanInfo{ScheduledDate: date, SlotTime: draft.SlotTime}}
		if occupied[key] || !probe.StartsAt(loc).After(now) {
			continue
		}
		occupied[key] = true
		w := r.weekContaining(date)
		if w == nil {
			return fmt.Errorf("%w: %s is outside the roadmap", ErrInvalidRoadmap, date.Format(time.DateOnly))
		}
		day, err := w.dayOn(date, newID)
		if err != nil {
			return err
		}

		info := *draft
		info.SessionPlanID = newID()
		info.DayPlanID = day.ID()
		info.WeekPlanID = w.ID()
		info.RoadmapID = r.ID()
		info.UserID = r.UserID()
		info.ScheduledDate = date
		info.Source = SessionPlanSourceScheduled
		s, err := NewSessionPlan(&info, now)
		if err != nil {
			return fmt.Errorf("new session on %s: %w", date.Format(time.DateOnly), err)
		}
		if err := day.AddSession(s); err != nil {
			return err
		}
	}

	for _, w := range r.weeks {
		if w.TrainingSessions() > MaxSessionsPerWeek {
			return fmt.Errorf("%w: week %d would hold %d training sessions (max %d)",
				ErrInvalidRoadmap, w.WeekNumber(), w.TrainingSessions(), MaxSessionsPerWeek)
		}
		if w.TotalSessions() > MaxSlotsPerWeek {
			return fmt.Errorf("%w: week %d would hold %d sessions (max %d)",
				ErrInvalidRoadmap, w.WeekNumber(), w.TotalSessions(), MaxSlotsPerWeek)
		}
	}
	r.info.UpdatedAt = now
	return nil
}

// weekContaining returns the week whose [StartDate, EndDate) holds date.
func (r *Roadmap) weekContaining(date time.Time) *WeekPlan {
	for _, w := range r.weeks {
		start := CalendarDate(w.info.StartDate, time.UTC)
		end := CalendarDate(w.info.EndDate, time.UTC)
		if !date.Before(start) && date.Before(end) {
			return w
		}
	}
	return nil
}

// RemovedChildren returns the session and day identifiers dropped since load,
// for persistence to delete.
func (r *Roadmap) RemovedChildren() (sessionIDs, dayIDs []string) {
	return append([]string(nil), r.removedSessions...), append([]string(nil), r.removedDays...)
}

// removeSessions drops the given sessions and returns the days left without one,
// which it drops too.
func (w *WeekPlan) removeSessions(ids map[string]struct{}) (droppedDays []string) {
	kept := w.days[:0]
	for _, d := range w.days {
		d.removeSessions(ids)
		if d.SessionCount() > 0 {
			kept = append(kept, d)
		} else {
			droppedDays = append(droppedDays, d.ID())
		}
	}
	w.days = kept
	return droppedDays
}

// dayOn returns the DayPlan for date, creating it when the week has none.
func (w *WeekPlan) dayOn(date time.Time, newID func() string) (*DayPlan, error) {
	for _, d := range w.days {
		if CalendarDate(d.info.ScheduledDate, time.UTC).Equal(date) {
			return d, nil
		}
	}
	d, err := NewDayPlan(&DayPlanInfo{
		DayPlanID:     newID(),
		WeekPlanID:    w.ID(),
		RoadmapID:     w.info.RoadmapID,
		UserID:        w.info.UserID,
		ScheduledDate: date,
	})
	if err != nil {
		return nil, err
	}
	if err := w.AddDay(d); err != nil {
		return nil, err
	}
	return d, nil
}

// removeSessions drops the given sessions from the day.
func (d *DayPlan) removeSessions(ids map[string]struct{}) {
	kept := d.sessions[:0]
	for _, s := range d.sessions {
		if _, drop := ids[s.ID()]; !drop {
			kept = append(kept, s)
		}
	}
	d.sessions = kept
}

// slotKey identifies a slot occurrence: calendar day and "HH:MM" start.
func slotKey(date time.Time, slotTime string) string {
	start, _, _ := strings.Cut(slotTime, "-")
	return CalendarDate(date, time.UTC).Format(time.DateOnly) + " " + strings.TrimSpace(start)
}
