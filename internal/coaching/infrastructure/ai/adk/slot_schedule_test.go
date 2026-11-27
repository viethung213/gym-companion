package adk

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/domain/roadmap"
)

// userSlots mirrors the reported profile:
// mon/thu/fri/sat/sun 17:30-19:00 plus thu 20:00-21:00.
func userSlots() []WorkoutSlot {
	return []WorkoutSlot{
		{DayOfWeek: int(time.Monday), StartTime: "17:30", EndTime: "19:00"},
		{DayOfWeek: int(time.Thursday), StartTime: "17:30", EndTime: "19:00"},
		{DayOfWeek: int(time.Thursday), StartTime: "20:00", EndTime: "21:00"},
		{DayOfWeek: int(time.Friday), StartTime: "17:30", EndTime: "19:00"},
		{DayOfWeek: int(time.Saturday), StartTime: "17:30", EndTime: "19:00"},
		{DayOfWeek: int(time.Sunday), StartTime: "17:30", EndTime: "19:00"},
	}
}

// userPlanStart is Monday 2026-11-23 23:41 UTC: that day's 17:30 slot is gone.
func userPlanStart() time.Time {
	return time.Date(2026, 11, 23, 23, 41, 0, 0, time.UTC)
}

func dailySlots() []WorkoutSlot {
	out := make([]WorkoutSlot, 0, 7)
	for d := range 7 {
		out = append(out, WorkoutSlot{DayOfWeek: d, StartTime: "07:00", EndTime: "08:00"})
	}
	return out
}

func TestExpectedWeekSessions(t *testing.T) {
	monday := time.Date(2026, 11, 23, 0, 0, 0, 0, time.UTC)
	thursdayPair := []WorkoutSlot{
		{DayOfWeek: int(time.Thursday), StartTime: "17:30", EndTime: "19:00"},
		{DayOfWeek: int(time.Thursday), StartTime: "20:00", EndTime: "21:00"},
	}
	mondayOnly := []WorkoutSlot{{DayOfWeek: int(time.Monday), StartTime: "17:30", EndTime: "19:00"}}

	tests := []struct {
		name  string
		slots []WorkoutSlot
		start time.Time
		week  int
		want  int
	}{
		{"week 1 partial: today's slot already started", userSlots(), userPlanStart(), 1, 5},
		{"later week holds every slot", userSlots(), userPlanStart(), 2, 6},
		{"deload week holds every slot", userSlots(), userPlanStart(), 4, 6},
		{"two slots on one day are two sessions", thursdayPair, monday, 1, 2},
		{"seven daily slots are all sessions (extras become recovery)", dailySlots(), monday, 2, 7},
		{"slot starting exactly now is not open", mondayOnly,
			time.Date(2026, 11, 23, 17, 30, 0, 0, time.UTC), 1, 0},
		{"slot one minute ahead is open", mondayOnly,
			time.Date(2026, 11, 23, 17, 29, 0, 0, time.UTC), 1, 1},
		{"repeated slot counts once", append(mondayOnly, mondayOnly...), monday, 1, 1},
		{"no slots", nil, monday, 1, 0},
		{"week number below one", userSlots(), monday, 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := expectedWeekSessions(tt.slots, tt.start, tt.week, time.UTC); got != tt.want {
				t.Errorf("got %d, want %d", got, tt.want)
			}
		})
	}
}

func TestWeekSlotOccurrences_ChronologicalAndDated(t *testing.T) {
	got := weekSlotOccurrences(userSlots(), userPlanStart(), 1, time.UTC)
	want := []DatedSlot{
		{ScheduledDate: "2026-11-26", SlotTime: "17:30-19:00"},
		{ScheduledDate: "2026-11-26", SlotTime: "20:00-21:00"},
		{ScheduledDate: "2026-11-27", SlotTime: "17:30-19:00"},
		{ScheduledDate: "2026-11-28", SlotTime: "17:30-19:00"},
		{ScheduledDate: "2026-11-29", SlotTime: "17:30-19:00"},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d occurrences %v, want %d", len(got), got, len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("occurrence %d: got %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestSlotSchedule_WeekSlotPlansCoverFourWeeks(t *testing.T) {
	plans := newSlotSchedule(userSlots(), userPlanStart(), time.UTC).weekSlotPlans()
	wantExpected := []int{5, 6, 6, 6}
	if len(plans) != len(wantExpected) {
		t.Fatalf("got %d weeks, want %d", len(plans), len(wantExpected))
	}
	for i, p := range plans {
		if p.WeekNumber != i+1 || p.ExpectedSessions != wantExpected[i] {
			t.Errorf("week %d: got number %d expected %d, want %d",
				i+1, p.WeekNumber, p.ExpectedSessions, wantExpected[i])
		}
	}
	if got := plans[1].Slots[0]; got.ScheduledDate != "2026-11-30" {
		t.Errorf("got week 2 first slot %+v, want 2026-11-30", got)
	}
}

func TestInitSlotSchedule_OnlyForInitialRoadmap(t *testing.T) {
	tests := []struct {
		name     string
		flow     string
		slots    []WorkoutSlot
		wantNone bool
	}{
		{"initial roadmap gets the rule", FlowInitiate4Week, userSlots(), false},
		{"regenerate is unaffected", FlowRegenerate, userSlots(), true},
		{"adaptive cycle is unaffected", FlowAdaptiveCycle, userSlots(), true},
		{"ad-hoc is unaffected", FlowSuggestAdHoc, userSlots(), true},
		{"no selected slot means no rule", FlowInitiate4Week, nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := &CoachInput{
				Flow:        tt.flow,
				CurrentTime: userPlanStart().Format(time.RFC3339),
				Profile:     UserProfile{AvailableSlots: tt.slots},
			}
			got, err := initSlotSchedule(in)
			if err != nil {
				t.Fatalf("initSlotSchedule returned error: %v", err)
			}
			if (got == nil) != tt.wantNone {
				t.Errorf("got schedule %v, want none=%t", got, tt.wantNone)
			}
		})
	}
}

func TestInitSlotSchedule_RejectsUnparseableCurrentTime(t *testing.T) {
	in := &CoachInput{
		Flow:        FlowInitiate4Week,
		CurrentTime: "yesterday",
		Profile:     UserProfile{AvailableSlots: userSlots()},
	}
	if _, err := initSlotSchedule(in); err == nil {
		t.Error("got nil error, want a parse failure")
	}
}

// slotSession is a valid session on date at slotTime.
func slotSession(date, slotTime string) SessionPlan {
	s := sessionOf(date, "bench-press")
	s.SlotTime = slotTime
	return s
}

// fullWeekOne schedules every slot still open in week 1 of the user's plan.
func fullWeekOne() []SessionPlan {
	return []SessionPlan{
		slotSession("2026-11-26", "17:30-19:00"),
		slotSession("2026-11-26", "20:00-21:00"),
		slotSession("2026-11-27", "17:30-19:00"),
		slotSession("2026-11-28", "17:30-19:00"),
		slotSession("2026-11-29", "17:30-19:00"),
	}
}

func scheduledValidator() *planValidator {
	return newPlanValidator(newFakeCatalog("bench-press")).
		withSchedule(newSlotSchedule(userSlots(), userPlanStart(), time.UTC))
}

func issuesContaining(issues []planIssue, fragment string) int {
	n := 0
	for _, iss := range issues {
		if strings.Contains(iss.String(), fragment) {
			n++
		}
	}
	return n
}

func TestPlanValidator_Schedule_FullCoveragePasses(t *testing.T) {
	got, err := scheduledValidator().validate(context.Background(), planOf(fullWeekOne()...), false)
	if err != nil {
		t.Fatalf("validate returned error: %v", err)
	}
	if n := issuesContaining(got.Issues, "week 1"); n != 0 {
		t.Errorf("got issues %v, want none for week 1", got.Issues)
	}
}

// Before weeks were numbered by the backend, a plan holding only week 1 passed
// validation and failed later as a roadmap with too few weeks, unretried.
func TestPlanValidator_Schedule_MissingWeeksAreRetryable(t *testing.T) {
	plan := planOf(fullWeekOne()...)

	strict, err := scheduledValidator().validate(context.Background(), plan, false)
	if err != nil {
		t.Fatalf("validate returned error: %v", err)
	}
	if n := issuesContaining(strict.Issues, "is missing"); n != 3 {
		t.Errorf("got issues %v, want weeks 2-4 reported missing", strict.Issues)
	}

	salvaged, err := scheduledValidator().validate(context.Background(), plan, true)
	if err != nil {
		t.Fatalf("validate returned error: %v", err)
	}
	if len(salvaged.Plan.Weeks) != 4 {
		t.Fatalf("salvage kept %d weeks, want 4", len(salvaged.Plan.Weeks))
	}
	for i, wp := range salvaged.Plan.Weeks {
		if wp.WeekNumber != i+1 || wp.Phase != string(roadmap.PhaseForWeek(i+1)) {
			t.Errorf("week %d = number %d phase %s", i+1, wp.WeekNumber, wp.Phase)
		}
	}
	if !salvaged.Degraded {
		t.Error("got Degraded=false, want true after adding missing weeks")
	}
}

func TestPlanValidator_Schedule_ShortfallIsRetryable(t *testing.T) {
	plan := planOf(fullWeekOne()[:3]...)

	got, err := scheduledValidator().validate(context.Background(), plan, false)
	if err != nil {
		t.Fatalf("validate returned error: %v", err)
	}
	if n := issuesContaining(got.Issues, "has 3 sessions on open slots but expects 5"); n != 1 {
		t.Errorf("got issues %v, want one week-1 shortfall", got.Issues)
	}
}

func TestPlanValidator_Schedule_SalvageKeepsPlanWithShortfall(t *testing.T) {
	plan := planOf(fullWeekOne()[:3]...)

	res, err := runWithRetries(context.Background(), scheduledValidator(), alwaysReturns(plan).fn, nil)
	if err != nil {
		t.Fatalf("runWithRetries returned error: %v", err)
	}
	if got := planSessionCount(res.Plan); got != 3 {
		t.Errorf("got %d sessions, want the 3 the model produced", got)
	}
	found := false
	for _, iss := range res.Issues {
		if strings.Contains(iss, "expects 5") {
			found = true
		}
	}
	if !found {
		t.Errorf("got issues %v, want the shortfall carried as a caution", res.Issues)
	}
}

func TestPlanValidator_Schedule_PastSessionRejectedThenDropped(t *testing.T) {
	sessions := append([]SessionPlan{slotSession("2026-11-23", "17:30-19:00")}, fullWeekOne()...)
	plan := planOf(sessions...)

	strict, err := scheduledValidator().validate(context.Background(), plan, false)
	if err != nil {
		t.Fatalf("validate returned error: %v", err)
	}
	if n := issuesContaining(strict.Issues, "starts before the plan was created"); n != 1 {
		t.Errorf("got issues %v, want one past-session issue", strict.Issues)
	}

	salvaged, err := scheduledValidator().validate(context.Background(), plan, true)
	if err != nil {
		t.Fatalf("validate returned error: %v", err)
	}
	if got := planSessionCount(salvaged.Plan); got != 5 {
		t.Errorf("got %d sessions, want 5 after dropping the past one", got)
	}
	for _, s := range salvaged.Plan.Weeks[0].Sessions {
		if s.ScheduledDate == "2026-11-23" {
			t.Errorf("past session on %s survived salvage", s.ScheduledDate)
		}
	}
	if !salvaged.Degraded {
		t.Error("got Degraded=false, want true after dropping a session")
	}
	if n := issuesContaining(salvaged.Issues, "week 1: has"); n != 0 {
		t.Errorf("got shortfall %v, want none: the remaining sessions cover week 1", salvaged.Issues)
	}
}

func TestPlanValidator_Schedule_DuplicateSlot(t *testing.T) {
	sessions := append(fullWeekOne(), slotSession("2026-11-27", "17:30"))
	plan := planOf(sessions...)

	strict, err := scheduledValidator().validate(context.Background(), plan, false)
	if err != nil {
		t.Fatalf("validate returned error: %v", err)
	}
	if n := issuesContaining(strict.Issues, "already used by another session"); n != 1 {
		t.Errorf("got issues %v, want one duplicate-slot issue", strict.Issues)
	}

	salvaged, err := scheduledValidator().validate(context.Background(), plan, true)
	if err != nil {
		t.Fatalf("validate returned error: %v", err)
	}
	if got := planSessionCount(salvaged.Plan); got != 5 {
		t.Errorf("got %d sessions, want 5 after dropping the duplicate", got)
	}
}

func TestPlanValidator_Schedule_SlotTimeMustMatchSelectedSlot(t *testing.T) {
	sessions := fullWeekOne()
	sessions[2].SlotTime = "06:00-07:30" // Friday has only 17:30-19:00

	got, err := scheduledValidator().validate(context.Background(), planOf(sessions...), false)
	if err != nil {
		t.Fatalf("validate returned error: %v", err)
	}
	if n := issuesContaining(got.Issues, "matches no selected slot for fri"); n != 1 {
		t.Errorf("got issues %v, want one slot mismatch", got.Issues)
	}
}

// TestPlanValidator_NoSchedule_RewriteUnaffected pins that a validator without
// a schedule (every rewrite flow) applies none of the slot rules.
func TestPlanValidator_NoSchedule_RewriteUnaffected(t *testing.T) {
	plan := planOf(
		slotSession("2026-11-23", "17:30-19:00"),
		slotSession("2026-11-23", "17:30-19:00"),
	)
	v := newPlanValidator(newFakeCatalog("bench-press")).expecting(2)

	got, err := v.validate(context.Background(), plan, true)
	if err != nil {
		t.Fatalf("validate returned error: %v", err)
	}
	if len(got.Issues) != 0 {
		t.Errorf("got issues %v, want none without a schedule", got.Issues)
	}
	if n := planSessionCount(got.Plan); n != 2 {
		t.Errorf("got %d sessions, want both kept", n)
	}
}

func TestWeekSlotOccurrences_UsesAthleteZone(t *testing.T) {
	ict, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		t.Fatal(err)
	}
	// Mon 2026-11-23 23:41 ICT = 16:41 UTC: that evening's 17:30 slot is over in
	// Vietnam although 17:30 UTC is still ahead.
	created := time.Date(2026, 11, 23, 23, 41, 0, 0, ict)

	got := weekSlotOccurrences(userSlots(), created, 1, ict)
	if len(got) != 5 || got[0].ScheduledDate != "2026-11-26" {
		t.Fatalf("ICT week 1 = %+v, want 5 slots starting Thu 2026-11-26", got)
	}
	if utc := weekSlotOccurrences(userSlots(), created, 1, time.UTC); len(utc) != 6 || utc[0].ScheduledDate != "2026-11-23" {
		t.Fatalf("UTC week 1 = %+v, want 6 slots starting Mon 2026-11-23", utc)
	}

	s := newSlotSchedule(userSlots(), created, ict)
	monday := time.Date(2026, 11, 23, 0, 0, 0, 0, time.UTC)
	if !s.isPast(monday, "17:30-19:00") {
		t.Fatal("Mon 17:30 ICT must count as past at 23:41 ICT")
	}
	if s.isPast(monday.AddDate(0, 0, 3), "17:30-19:00") {
		t.Fatal("Thu 17:30 ICT must still be open")
	}
}

func TestAssignKinds_SpreadsTrainingAndMakesExtrasRecovery(t *testing.T) {
	// Seven daily slots plus a second Thursday slot: 8 slots, 6 training.
	slots := append(dailySlots(), WorkoutSlot{DayOfWeek: int(time.Thursday), StartTime: "20:00", EndTime: "21:00"})
	monday := time.Date(2026, 11, 23, 0, 0, 0, 0, time.UTC)
	got := newSlotSchedule(slots, monday, time.UTC).weekSlotPlans()[0]

	if got.ExpectedSessions != 8 || got.TrainingSessions != maxSessionsPerWeek {
		t.Fatalf("week 1 = %d sessions / %d training, want 8 / %d", got.ExpectedSessions, got.TrainingSessions, maxSessionsPerWeek)
	}
	trainingDays := map[string]int{}
	for _, s := range got.Slots {
		if s.SessionKind == kindTraining {
			trainingDays[s.ScheduledDate]++
		}
	}
	if len(trainingDays) != maxSessionsPerWeek {
		t.Errorf("training spans %d days, want %d (one per day before doubling up)", len(trainingDays), maxSessionsPerWeek)
	}
	// Mon..Sat train once each; Sunday and the second Thursday slot recover.
	for _, s := range got.Slots {
		wantRecovery := s.ScheduledDate == "2026-11-29" || (s.ScheduledDate == "2026-11-26" && s.SlotTime == "20:00-21:00")
		if (s.SessionKind == kindRecovery) != wantRecovery {
			t.Errorf("%s %s kind = %s", s.ScheduledDate, s.SlotTime, s.SessionKind)
		}
	}
}

func TestAssignKinds_CapsAllSlotsAtTen(t *testing.T) {
	slots := make([]WorkoutSlot, 0, 14)
	for d := range 7 {
		slots = append(slots,
			WorkoutSlot{DayOfWeek: d, StartTime: "06:00", EndTime: "07:00"},
			WorkoutSlot{DayOfWeek: d, StartTime: "18:00", EndTime: "19:00"})
	}
	monday := time.Date(2026, 11, 23, 0, 0, 0, 0, time.UTC)
	got := newSlotSchedule(slots, monday, time.UTC).weekSlotPlans()[1]
	if got.ExpectedSessions != maxSlotsPerWeek || got.TrainingSessions != maxSessionsPerWeek {
		t.Fatalf("week 2 = %d / %d training, want %d / %d", got.ExpectedSessions, got.TrainingSessions, maxSlotsPerWeek, maxSessionsPerWeek)
	}
}
