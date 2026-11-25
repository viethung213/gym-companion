package roadmap

import (
	"fmt"
	"time"
)

// EndsAt is the instant the week is over: midnight, in loc, of its EndDate.
func (w *WeekPlan) EndsAt(loc *time.Location) time.Time {
	y, m, d := w.info.EndDate.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, loc)
}

// Reviewed reports whether the week's review is done.
func (w *WeekPlan) Reviewed() bool { return w.info.ReviewedAt != nil }

// WeeksDueForReview returns, in week order, the weeks not yet reviewed that
// ended at or before cutoff in the roadmap's zone.
func (r *Roadmap) WeeksDueForReview(cutoff time.Time) []*WeekPlan {
	loc := r.Location()
	var due []*WeekPlan
	for _, w := range r.Weeks() {
		if !w.Reviewed() && !w.EndsAt(loc).After(cutoff) {
			due = append(due, w)
		}
	}
	return due
}

// SkipMissedSessions turns the week's sessions still PENDING into SKIPPED: the
// week is over, so they were not trained. Repeating it changes nothing.
func (r *Roadmap) SkipMissedSessions(weekPlanID string) error {
	w, err := r.week(weekPlanID)
	if err != nil {
		return err
	}
	for _, d := range w.days {
		for _, s := range d.sessions {
			if s.Status() != SessionPlanStatusPending {
				continue
			}
			if err := s.MarkSkipped(); err != nil {
				return fmt.Errorf("skip session %s: %w", s.ID(), err)
			}
		}
	}
	return nil
}

// MarkReviewed stamps the week's review as done, so it is never counted
// again. Marking a reviewed week keeps its first stamp.
func (r *Roadmap) MarkReviewed(weekPlanID string, now time.Time) error {
	w, err := r.week(weekPlanID)
	if err != nil {
		return err
	}
	if w.Reviewed() {
		return nil
	}
	reviewedAt := now
	w.info.ReviewedAt = &reviewedAt
	r.info.UpdatedAt = now
	return nil
}

func (r *Roadmap) week(weekPlanID string) (*WeekPlan, error) {
	for _, w := range r.weeks {
		if w.ID() == weekPlanID {
			return w, nil
		}
	}
	return nil, fmt.Errorf("%w: week %s", ErrInvalidRoadmap, weekPlanID)
}
