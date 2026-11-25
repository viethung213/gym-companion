package roadmap

import (
	"fmt"
	"time"
)

// A roadmap is one 4-week block. The next block is drafted during the last
// week as PLANNED, starting the day the current one ends, and is activated
// when the current one completes, so the athlete never waits for a plan.

// EndsAt is the instant the roadmap is over: midnight, in its zone, of EndDate.
func (r *Roadmap) EndsAt() time.Time {
	y, m, d := r.info.EndDate.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, r.Location())
}

// FinalWeekStartsAt is when the last week of the block begins.
func (r *Roadmap) FinalWeekStartsAt() time.Time {
	return r.EndsAt().AddDate(0, 0, -DaysPerWeek)
}

// HoldUntilStart keeps a freshly drafted roadmap as PLANNED instead of ACTIVE.
// Only an untouched draft qualifies: every session still PENDING.
func (r *Roadmap) HoldUntilStart() error {
	if r.info.Status != StatusActive {
		return fmt.Errorf("%w: %s to %s", ErrInvalidTransition, r.info.Status, StatusPlanned)
	}
	for _, w := range r.weeks {
		for _, d := range w.days {
			for _, s := range d.sessions {
				if s.Status() != SessionPlanStatusPending {
					return fmt.Errorf("%w: only a fresh draft can be planned ahead", ErrInvalidTransition)
				}
			}
		}
	}
	r.info.Status = StatusPlanned
	return nil
}

// Activate turns a PLANNED roadmap into the ACTIVE one. Idempotent when ACTIVE.
func (r *Roadmap) Activate(now time.Time) error {
	if r.info.Status == StatusActive {
		return nil
	}
	if r.info.Status != StatusPlanned {
		return fmt.Errorf("%w: %s to %s", ErrInvalidTransition, r.info.Status, StatusActive)
	}
	r.info.Status = StatusActive
	r.info.UpdatedAt = now
	return nil
}

// Follows reports whether r is the block that starts the day prev ends.
func (r *Roadmap) Follows(prev *Roadmap) bool {
	return CalendarDate(r.info.StartDate, time.UTC).Equal(CalendarDate(prev.info.EndDate, time.UTC))
}
