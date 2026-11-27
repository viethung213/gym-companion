package roadmap

import (
	"errors"
	"time"
)

// RegenerationGiveUpAfter is how long a profile change that could not be
// applied (the model unavailable, a draft the guardrail kept refusing) is
// retried before it is dropped.
const RegenerationGiveUpAfter = 24 * time.Hour

// ErrNoRegenerationDue is returned when no profile change is waiting.
var ErrNoRegenerationDue = errors.New("no regeneration is due")

// RegenerationDue is a profile change the schedule still has to absorb.
type RegenerationDue struct {
	// ChangedAt is when the profile changed; a retry plans from it, so a roadmap
	// planned after it still counts as reflecting it.
	ChangedAt time.Time
	// Reason is the change ("injury_reported", "profile_updated", ...).
	Reason string
}

// MarkRegenerationDue records a profile change the schedule could not absorb
// now, so it is retried later instead of lost. A change already waiting keeps
// the earliest time: one regeneration covers every change since.
func (r *Roadmap) MarkRegenerationDue(changedAt time.Time, reason string) {
	if r.info.RegenerationDue != nil && !changedAt.Before(r.info.RegenerationDue.ChangedAt) {
		return
	}
	r.info.RegenerationDue = &RegenerationDue{ChangedAt: changedAt, Reason: reason}
}

// ClearRegenerationDue forgets the waiting change: it was applied or dropped.
func (r *Roadmap) ClearRegenerationDue() { r.info.RegenerationDue = nil }

// PendingRegeneration returns the waiting profile change, or ErrNoRegenerationDue.
func (r *Roadmap) PendingRegeneration() (RegenerationDue, error) {
	if r.info.RegenerationDue == nil {
		return RegenerationDue{}, ErrNoRegenerationDue
	}
	return *r.info.RegenerationDue, nil
}

// RegenerationExpired reports whether the waiting change is too old to retry.
func (d RegenerationDue) RegenerationExpired(now time.Time) bool {
	return now.Sub(d.ChangedAt) > RegenerationGiveUpAfter
}
