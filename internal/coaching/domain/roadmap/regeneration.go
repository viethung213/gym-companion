package roadmap

import (
	"errors"
	"time"
)

// RegenerationGiveUpAfter is how long a profile change that could not be
// applied (the model unavailable, a draft the guardrail kept refusing) is
// retried before it is dropped.
const RegenerationGiveUpAfter = 24 * time.Hour

// MaxRegenerationReasonLen bounds a regeneration reason, in characters (the
// stored column is VARCHAR(64)).
const MaxRegenerationReasonLen = 64

// ErrNoRegenerationDue is returned when no profile change is waiting.
var ErrNoRegenerationDue = errors.New("no regeneration is due")

// RegenerationDue is a profile change the schedule still has to absorb.
type RegenerationDue struct {
	// ChangedAt is when the latest waiting change happened: a schedule
	// generated after it reflects every change waiting before it too.
	ChangedAt time.Time
	// Reason is the change ("injury_reported", "profile_updated", ...).
	Reason string
}

// MarkRegenerationDue records a profile change the schedule could not absorb
// now, so it is retried later instead of lost. The latest waiting change is
// kept: a regeneration that started after it covers the earlier ones, and one
// that started before it must not clear it.
func (r *Roadmap) MarkRegenerationDue(changedAt time.Time, reason string) {
	if r.info.RegenerationDue != nil && !changedAt.After(r.info.RegenerationDue.ChangedAt) {
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
