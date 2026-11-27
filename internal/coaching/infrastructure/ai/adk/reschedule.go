package adk

import (
	"slices"
	"strings"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/domain/roadmap"
)

// defaultSessionMinutes is a rescheduled session's length when its slot has no end.
const defaultSessionMinutes = 60

// rescheduleTargets lays the athlete's current weekly slots over the rest of
// the roadmap: every selected slot still ahead of now becomes one session,
// keeping each roadmap week within the weekly caps (training and total) next
// to the sessions it already finished or missed; extra slots become recovery. It returns nil when the upcoming PENDING
// sessions already sit exactly on those slots, so the rewrite keeps them.
func rescheduleTargets(rm *roadmap.Roadmap, slots []WorkoutSlot, now time.Time) []DatedSlot {
	if rm == nil || len(slots) == 0 {
		return nil
	}
	loc := rm.Location()
	upcoming := rm.UpcomingPendingSessions(now)
	moving := make(map[string]struct{}, len(upcoming))
	for _, s := range upcoming {
		moving[s.ID()] = struct{}{}
	}

	var targets []DatedSlot
	for _, w := range rm.Weeks() {
		kept, keptTraining := 0, 0
		taken := make(map[string]bool) // slots a kept session already holds
		for _, d := range w.Days() {
			for _, s := range d.Sessions() {
				if _, ok := moving[s.ID()]; !ok {
					kept++
					if s.Kind() == roadmap.SessionKindTraining {
						keptTraining++
					}
					info := s.Info()
					taken[info.ScheduledDate.Format(scheduledDateISO)+" "+slotStartClock(info.SlotTime)] = true
				}
			}
		}
		budget := roadmap.MaxSlotsPerWeek - kept
		if budget <= 0 {
			continue
		}
		info := w.Info()
		open := slotOccurrences(slots,
			roadmap.CalendarDate(info.StartDate, time.UTC), roadmap.CalendarDate(info.EndDate, time.UTC), now, loc)
		// A slot whose session was trained early (or stopped) is not open: it
		// is still ahead on the clock, and counting it made unchanged slots look
		// changed, rescheduling every upcoming session under new IDs.
		open = slices.DeleteFunc(open, func(o DatedSlot) bool {
			return taken[o.ScheduledDate+" "+slotStartClock(o.SlotTime)]
		})
		trainingBudget := max(0, roadmap.MaxSessionsPerWeek-keptTraining)
		weekTargets := assignKinds(open[:min(budget, len(open))], trainingBudget)
		for i := range weekTargets {
			weekTargets[i].Phase = string(w.Phase())
		}
		targets = append(targets, weekTargets...)
	}

	current := make([]string, 0, len(upcoming))
	for _, s := range upcoming {
		info := s.Info()
		current = append(current, info.ScheduledDate.Format(scheduledDateISO)+" "+slotStartClock(info.SlotTime)+" "+string(info.Kind))
	}
	wanted := make([]string, 0, len(targets))
	for _, t := range targets {
		wanted = append(wanted, t.ScheduledDate+" "+slotStartClock(t.SlotTime)+" "+t.SessionKind)
	}
	if slices.Equal(current, wanted) {
		return nil
	}
	return targets
}

// rescheduledSessions turns target slots into fresh sessions to plan.
func rescheduledSessions(targets []DatedSlot) []SessionToRevise {
	out := make([]SessionToRevise, 0, len(targets))
	for _, t := range targets {
		out = append(out, SessionToRevise{
			ScheduledDate:            t.ScheduledDate,
			SlotTime:                 t.SlotTime,
			SessionKind:              t.SessionKind,
			Phase:                    t.Phase,
			EstimatedDurationMinutes: slotMinutes(t.SlotTime),
		})
	}
	return out
}

// slotMinutes is the length of an "HH:MM-HH:MM" slot, defaultSessionMinutes otherwise.
func slotMinutes(slotTime string) int32 {
	start, end, ok := cutSlot(slotTime)
	if !ok || !end.After(start) {
		return defaultSessionMinutes
	}
	return int32(end.Sub(start) / time.Minute)
}

func cutSlot(slotTime string) (start, end time.Time, ok bool) {
	startClock, endClock, found := strings.Cut(slotTime, "-")
	if !found {
		return start, end, false
	}
	s, err1 := time.Parse(slotClockLayout, strings.TrimSpace(startClock))
	e, err2 := time.Parse(slotClockLayout, strings.TrimSpace(endClock))
	return s, e, err1 == nil && err2 == nil
}

// pinRewriteSlots sets every rewritten session's date, slot and kind to the entry
// it revises, so the model may change what a session prescribes but never when
// it happens. It runs before validation, while the plan still holds exactly one
// session per entry (planValidator.expecting); salvage may drop sessions later,
// and callers match the survivors by date, slot and kind, not by position.
func pinRewriteSlots(plan *GeneratedPlan, revise []SessionToRevise) {
	if plan == nil {
		return
	}
	i := 0
	for w := range plan.Weeks {
		for s := range plan.Weeks[w].Sessions {
			if i >= len(revise) {
				return
			}
			sp := &plan.Weeks[w].Sessions[s]
			sp.ScheduledDate = revise[i].ScheduledDate
			sp.SlotTime = revise[i].SlotTime
			sp.SessionKind = revise[i].SessionKind
			sp.phase = roadmap.Phase(revise[i].Phase)
			if sp.SessionKind == "" {
				sp.SessionKind = kindTraining
			}
			if sp.EstimatedDurationMinutes <= 0 {
				sp.EstimatedDurationMinutes = revise[i].EstimatedDurationMinutes
			}
			i++
		}
	}
}
