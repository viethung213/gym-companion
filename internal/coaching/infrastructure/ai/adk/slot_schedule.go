package adk

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/domain/roadmap"
)

// slotClockLayout is the wall-clock layout of a slot's start and end.
const slotClockLayout = "15:04"

// slotSchedule lays the athlete's selected weekly slots over the initial
// roadmap. Week N covers the seven calendar days starting (N-1)*7 days after
// the plan's creation date, and a slot occurrence is open only when it starts
// strictly after creation. Slot times and scheduled dates are wall clock in
// the athlete's zone (loc); start is an instant.
type slotSchedule struct {
	start time.Time
	loc   *time.Location
	slots []WorkoutSlot
	kinds map[string]string // "date start" -> session kind, built on first kindOf
}

func newSlotSchedule(slots []WorkoutSlot, start time.Time, loc *time.Location) *slotSchedule {
	if loc == nil {
		loc = time.UTC
	}
	return &slotSchedule{start: start, loc: loc, slots: slices.Clone(slots)}
}

// initSlotSchedule returns the initial roadmap's schedule. It is nil for any
// other flow (rewrites keep the slots of the sessions they revise) and when the
// athlete selected no slot, since there is then nothing to cover.
func initSlotSchedule(in *CoachInput) (*slotSchedule, error) {
	if in.Flow != FlowInitiate4Week || len(in.Profile.AvailableSlots) == 0 {
		return nil, nil
	}
	start, err := time.Parse(time.RFC3339, in.CurrentTime)
	if err != nil {
		return nil, fmt.Errorf("parse current_time %q: %w", in.CurrentTime, err)
	}
	return newSlotSchedule(in.Profile.AvailableSlots, start, roadmap.LoadLocation(in.Timezone)), nil
}

// weekSlotPlans lists every week's open slots and expected session count.
func (s *slotSchedule) weekSlotPlans() []WeekSlotPlan {
	out := make([]WeekSlotPlan, 0, roadmap.WeeksPerRoadmap)
	for week := 1; week <= roadmap.WeeksPerRoadmap; week++ {
		open := s.weekSlots(week)
		out = append(out, WeekSlotPlan{
			WeekNumber:       week,
			ExpectedSessions: len(open),
			TrainingSessions: countKind(open, kindTraining),
			Slots:            open,
		})
	}
	return out
}

// expectedSessions is how many sessions, training and recovery, the week must hold.
func (s *slotSchedule) expectedSessions(weekNumber int) int {
	return len(s.weekSlots(weekNumber))
}

// weekSlots returns the week's open slots with their assigned session kind.
func (s *slotSchedule) weekSlots(weekNumber int) []DatedSlot {
	open := weekSlotOccurrences(s.slots, s.start, weekNumber, s.loc)
	return assignKinds(open[:min(len(open), maxSlotsPerWeek)], maxSessionsPerWeek)
}

// kindOf returns the kind assigned to the slot starting at slotTime on date
// (YYYY-MM-DD), TRAINING when the slot is not one of the schedule's.
func (s *slotSchedule) kindOf(date, slotTime string) string {
	if s.kinds == nil {
		s.kinds = make(map[string]string)
		for week := 1; week <= roadmap.WeeksPerRoadmap; week++ {
			for _, ds := range s.weekSlots(week) {
				s.kinds[ds.ScheduledDate+" "+slotStartClock(ds.SlotTime)] = ds.SessionKind
			}
		}
	}
	if k, ok := s.kinds[date+" "+slotStartClock(slotTime)]; ok {
		return k
	}
	return kindTraining
}

// isPast reports whether a session on date at slotTime has already started.
// Without a parseable start the date alone decides.
func (s *slotSchedule) isPast(date time.Time, slotTime string) bool {
	if at, ok := slotStartAt(date, slotStartClock(slotTime), s.loc); ok {
		return !at.After(s.start)
	}
	return date.Before(roadmap.CalendarDate(s.start, s.loc))
}

// daySlots returns the selected slots on weekday, rendered as slot_time.
func (s *slotSchedule) daySlots(weekday time.Weekday) []string {
	var out []string
	for _, sl := range sortedSlots(s.slots) {
		if time.Weekday(sl.DayOfWeek) == weekday {
			out = append(out, slotTimeOf(sl))
		}
	}
	return out
}

// matchesSlot reports whether slotTime starts at a slot selected for weekday.
func (s *slotSchedule) matchesSlot(weekday time.Weekday, slotTime string) bool {
	clock := slotStartClock(slotTime)
	for _, sl := range s.slots {
		if time.Weekday(sl.DayOfWeek) == weekday && strings.TrimSpace(sl.StartTime) == clock {
			return true
		}
	}
	return false
}

// expectedWeekSessions is the number of selected slot occurrences in the
// week's seven-day range that start after planStart, capped at the weekly
// session limit.
func expectedWeekSessions(slots []WorkoutSlot, planStart time.Time, weekNumber int, loc *time.Location) int {
	return min(len(weekSlotOccurrences(slots, planStart, weekNumber, loc)), maxSlotsPerWeek)
}

// weekSlotOccurrences returns, in chronological order, the selected slots in
// weekNumber's seven-day range (counted from planStart's calendar day in loc)
// that start strictly after planStart.
func weekSlotOccurrences(slots []WorkoutSlot, planStart time.Time, weekNumber int, loc *time.Location) []DatedSlot {
	if weekNumber < 1 {
		return nil
	}
	weekStart := scheduleWeekStart(planStart, loc, weekNumber)
	return slotOccurrences(slots, weekStart, weekStart.AddDate(0, 0, 7), planStart, loc)
}

// slotOccurrences returns, in chronological order, every selected slot on the
// calendar days [fromDate, toDate) that starts strictly after notBefore. Dates
// use the scheduled_date encoding (midnight UTC of the calendar day); slot
// times are wall clock in loc. Each selected slot occurs once per day; a
// repeated slot counts once.
func slotOccurrences(slots []WorkoutSlot, fromDate, toDate, notBefore time.Time, loc *time.Location) []DatedSlot {
	ordered := sortedSlots(slots)
	var out []DatedSlot
	for date := fromDate; date.Before(toDate); date = date.AddDate(0, 0, 1) {
		seen := make(map[string]struct{})
		for _, sl := range ordered {
			if time.Weekday(sl.DayOfWeek) != date.Weekday() {
				continue
			}
			clock := strings.TrimSpace(sl.StartTime)
			if _, dup := seen[clock]; dup {
				continue
			}
			at, ok := slotStartAt(date, clock, loc)
			if !ok || !at.After(notBefore) {
				continue
			}
			seen[clock] = struct{}{}
			out = append(out, DatedSlot{
				ScheduledDate: date.Format(scheduledDateISO),
				SlotTime:      slotTimeOf(sl),
			})
		}
	}
	return out
}

// sortedSlots returns a copy ordered by weekday, then start time.
func sortedSlots(slots []WorkoutSlot) []WorkoutSlot {
	out := slices.Clone(slots)
	slices.SortStableFunc(out, func(a, b WorkoutSlot) int {
		if c := cmp.Compare(a.DayOfWeek, b.DayOfWeek); c != 0 {
			return c
		}
		return cmp.Compare(strings.TrimSpace(a.StartTime), strings.TrimSpace(b.StartTime))
	})
	return out
}

// slotTimeOf renders a slot the way preferred_workout_times shows it.
func slotTimeOf(sl WorkoutSlot) string {
	start := strings.TrimSpace(sl.StartTime)
	if end := strings.TrimSpace(sl.EndTime); end != "" {
		return start + "-" + end
	}
	return start
}

// slotStartClock extracts "HH:MM" from "HH:MM" or "HH:MM-HH:MM".
func slotStartClock(slotTime string) string {
	start, _, _ := strings.Cut(slotTime, "-")
	return strings.TrimSpace(start)
}

// slotStartAt combines a calendar date with an "HH:MM" wall-clock start in loc.
func slotStartAt(date time.Time, clock string, loc *time.Location) (time.Time, bool) {
	t, err := time.Parse(slotClockLayout, clock)
	if err != nil {
		return time.Time{}, false
	}
	y, m, d := date.Date()
	return time.Date(y, m, d, t.Hour(), t.Minute(), 0, 0, loc), true
}

// Session kinds as the model and the domain spell them.
const (
	kindTraining = string(roadmap.SessionKindTraining)
	kindRecovery = string(roadmap.SessionKindRecovery)
)

// assignKinds marks, in chronological order, up to trainingBudget slots as
// TRAINING and the rest as RECOVERY. Training spreads first: one per calendar
// day, then a second slot on a day only once every day has one. A rest from
// hard work is thereby guaranteed whenever the athlete selects more slots than
// the weekly training cap.
func assignKinds(slots []DatedSlot, trainingBudget int) []DatedSlot {
	out := slices.Clone(slots)
	for i := range out {
		out[i].SessionKind = kindRecovery
	}
	used := make(map[string]bool)
	for pass := 0; pass < 2 && trainingBudget > 0; pass++ {
		for i := range out {
			if trainingBudget == 0 {
				break
			}
			if out[i].SessionKind == kindTraining || (pass == 0 && used[out[i].ScheduledDate]) {
				continue
			}
			out[i].SessionKind = kindTraining
			used[out[i].ScheduledDate] = true
			trainingBudget--
		}
	}
	return out
}

// countKind counts the slots of the given kind.
func countKind(slots []DatedSlot, kind string) int {
	n := 0
	for _, s := range slots {
		if s.SessionKind == kind {
			n++
		}
	}
	return n
}

// weekStart is the first calendar day of weekNumber in this schedule.
func (s *slotSchedule) weekStart(weekNumber int) time.Time {
	return scheduleWeekStart(s.start, s.loc, weekNumber)
}

// scheduleWeekStart is the first calendar day of weekNumber (scheduled_date
// encoding): week 1 starts on the plan's creation day in the athlete's zone,
// and the roadmap's weeks use the same anchor.
func scheduleWeekStart(planStart time.Time, loc *time.Location, weekNumber int) time.Time {
	return roadmap.CalendarDate(planStart, loc).AddDate(0, 0, (weekNumber-1)*7)
}
