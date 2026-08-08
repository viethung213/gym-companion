package adk

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/domain/roadmap"
)

// incrIDs hands out predictable identifiers so tests can assert on the exact
// IDs the mapper assigns, mirroring the fake in initiate_roadmap_test.go.
type incrIDs struct{ n int }

func (i *incrIDs) NewID() string {
	i.n++
	return "id-" + strconv.Itoa(i.n)
}

// mapperFor builds an agent wired with just the pieces domain mapping needs.
func mapperFor(t *testing.T, catalogIDs ...string) *CoachingContextAgent {
	t.Helper()
	cat := newFakeCatalog(catalogIDs...)
	return &CoachingContextAgent{
		catalog:   cat,
		idgen:     &incrIDs{},
		validator: newPlanValidator(cat),
	}
}

func getMapNow() time.Time {
	return time.Date(2026, 8, 1, 10, 30, 0, 0, time.UTC)
}

// TestMapToDomainRoadmap_GroupsSessionsByDate is the headline case: two
// sessions on the same date must share one DayPlan rather than each minting
// their own, which WeekPlan.AddDay would reject as a duplicate date.
func TestMapToDomainRoadmap_GroupsSessionsByDate(t *testing.T) {
	c := mapperFor(t, "bench-press", "squat")
	plan := planOf(
		sessionOf("2026-08-03", "bench-press"),
		sessionOf("2026-08-03", "squat"),
		sessionOf("2026-08-05", "bench-press"),
	)

	rm, err := c.mapToDomainRoadmap(context.Background(), plan, nil, "user-1", getMapNow())
	if err != nil {
		t.Fatalf("mapToDomainRoadmap returned error: %v", err)
	}

	days := rm.Weeks()[0].Days()
	if len(days) != 2 {
		t.Fatalf("got %d day plans, want 2 (two distinct dates)", len(days))
	}
	if got := days[0].SessionCount(); got != 2 {
		t.Errorf("got %d sessions on the first day, want 2", got)
	}

	first, second := days[0].Sessions()[0], days[0].Sessions()[1]
	if first.Info().DayPlanID != second.Info().DayPlanID {
		t.Errorf("got DayPlanIDs %q and %q, want both sessions on one day plan",
			first.Info().DayPlanID, second.Info().DayPlanID)
	}
	if first.Info().DayPlanID != days[0].ID() {
		t.Errorf("got session DayPlanID %q, want the owning day's ID %q",
			first.Info().DayPlanID, days[0].ID())
	}
}

// TestMapToDomainRoadmap_SetsAllIdentityFields is the regression test for the
// bug where DayPlanID was never set, so NewSessionPlan always failed.
func TestMapToDomainRoadmap_SetsAllIdentityFields(t *testing.T) {
	c := mapperFor(t, "bench-press")
	plan := planOf(sessionOf("2026-08-03", "bench-press"))

	rm, err := c.mapToDomainRoadmap(context.Background(), plan, nil, "user-1", getMapNow())
	if err != nil {
		t.Fatalf("mapToDomainRoadmap returned error: %v", err)
	}

	week := rm.Weeks()[0]
	day := week.Days()[0]
	info := day.Sessions()[0].Info()

	checks := map[string]string{
		"SessionPlanID": info.SessionPlanID,
		"DayPlanID":     info.DayPlanID,
		"WeekPlanID":    info.WeekPlanID,
		"RoadmapID":     info.RoadmapID,
		"UserID":        info.UserID,
	}
	for name, val := range checks {
		if val == "" {
			t.Errorf("got empty %s, want it populated", name)
		}
	}

	if info.UserID != "user-1" {
		t.Errorf("got UserID %q, want user-1", info.UserID)
	}
	if info.RoadmapID != rm.ID() {
		t.Errorf("got session RoadmapID %q, want %q", info.RoadmapID, rm.ID())
	}
	if info.WeekPlanID != week.ID() {
		t.Errorf("got session WeekPlanID %q, want %q", info.WeekPlanID, week.ID())
	}
	if info.Status != roadmap.SessionPlanStatusPending {
		t.Errorf("got status %q, want PENDING", info.Status)
	}
	if info.SlotTime != "06:00-07:30" {
		t.Errorf("got SlotTime %q, want 06:00-07:30", info.SlotTime)
	}
	if info.EstimatedDurationMinutes != 60 {
		t.Errorf("got EstimatedDurationMinutes %d, want 60", info.EstimatedDurationMinutes)
	}
}

func TestMapToDomainRoadmap_MintsIdsDeterministically(t *testing.T) {
	c := mapperFor(t, "bench-press")
	plan := planOf(sessionOf("2026-08-03", "bench-press"))

	rm, err := c.mapToDomainRoadmap(context.Background(), plan, nil, "user-1", getMapNow())
	if err != nil {
		t.Fatalf("mapToDomainRoadmap returned error: %v", err)
	}

	// Allocation order is roadmap, week, day, session.
	if rm.ID() != "id-1" {
		t.Errorf("got roadmap id %q, want id-1", rm.ID())
	}
	if got := rm.Weeks()[0].ID(); got != "id-2" {
		t.Errorf("got week id %q, want id-2", got)
	}
	if got := rm.Weeks()[0].Days()[0].ID(); got != "id-3" {
		t.Errorf("got day id %q, want id-3", got)
	}
	if got := rm.Weeks()[0].Days()[0].Sessions()[0].ID(); got != "id-4" {
		t.Errorf("got session id %q, want id-4", got)
	}
}

func TestMapToDomainRoadmap_EnrichesExerciseNamesFromValidator(t *testing.T) {
	c := mapperFor(t, "bench-press")
	plan := planOf(sessionOf("2026-08-03", "bench-press"))
	names := map[string]string{"bench-press": "Barbell Bench Press"}

	rm, err := c.mapToDomainRoadmap(context.Background(), plan, names, "user-1", getMapNow())
	if err != nil {
		t.Fatalf("mapToDomainRoadmap returned error: %v", err)
	}

	got := rm.Weeks()[0].Days()[0].Sessions()[0].Info().Prescription.MainExercises[0]
	if got.ExerciseName != "Barbell Bench Press" {
		t.Errorf("got ExerciseName %q, want the name supplied by validation", got.ExerciseName)
	}
}

func TestMapToDomainRoadmap_FallsBackToCatalogForMissingName(t *testing.T) {
	c := mapperFor(t, "bench-press")
	cat, ok := c.catalog.(*fakeCatalog)
	if !ok {
		t.Fatal("expected *fakeCatalog")
	}
	plan := planOf(sessionOf("2026-08-03", "bench-press"))

	// No names map: the mapper must consult the catalog rather than persist "".
	rm, err := c.mapToDomainRoadmap(context.Background(), plan, nil, "user-1", getMapNow())
	if err != nil {
		t.Fatalf("mapToDomainRoadmap returned error: %v", err)
	}

	got := rm.Weeks()[0].Days()[0].Sessions()[0].Info().Prescription.MainExercises[0]
	if got.ExerciseName != "Name of bench-press" {
		t.Errorf("got ExerciseName %q, want the catalog name", got.ExerciseName)
	}
	if cat.calls["bench-press"] == 0 {
		t.Error("got no catalog lookup, want one for the unresolved name")
	}
}

func TestMapToDomainRoadmap_RejectsMalformedDate(t *testing.T) {
	c := mapperFor(t, "bench-press")
	plan := planOf(sessionOf("not-a-date", "bench-press"))

	_, err := c.mapToDomainRoadmap(context.Background(), plan, nil, "user-1", getMapNow())
	if err == nil {
		t.Fatal("got nil error, want a rejection naming the bad date")
	}
	if !strings.Contains(err.Error(), "scheduled_date") {
		t.Errorf("got error %q, want it to name scheduled_date", err)
	}
}

// TestMapToDomainRoadmap_AnchorsOnEarliestSession pins the roadmap window to
// the plan's own dates. Anchoring on time.Now() instead would let sessions fall
// outside their week's range.
func TestMapToDomainRoadmap_AnchorsOnEarliestSession(t *testing.T) {
	c := mapperFor(t, "bench-press")
	plan := planOf(
		sessionOf("2026-08-10", "bench-press"),
		sessionOf("2026-08-04", "bench-press"),
	)

	rm, err := c.mapToDomainRoadmap(context.Background(), plan, nil, "user-1", getMapNow())
	if err != nil {
		t.Fatalf("mapToDomainRoadmap returned error: %v", err)
	}

	want := time.Date(2026, 8, 4, 0, 0, 0, 0, time.UTC)
	if got := rm.Info().StartDate; !got.Equal(want) {
		t.Errorf("got StartDate %v, want %v (the earliest scheduled session)", got, want)
	}
	if got := rm.Info().EndDate; !got.Equal(want.AddDate(0, 0, 28)) {
		t.Errorf("got EndDate %v, want 28 days after the start", got)
	}
}

func TestMapToDomainRoadmap_NormalisesDatesToUTCMidnight(t *testing.T) {
	c := mapperFor(t, "bench-press")
	plan := planOf(sessionOf("2026-08-03", "bench-press"))

	rm, err := c.mapToDomainRoadmap(context.Background(), plan, nil, "user-1", getMapNow())
	if err != nil {
		t.Fatalf("mapToDomainRoadmap returned error: %v", err)
	}

	got := rm.Weeks()[0].Days()[0].ScheduledDate()
	want := time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("got ScheduledDate %v, want %v", got, want)
	}
	if got.Location() != time.UTC {
		t.Errorf("got location %v, want UTC", got.Location())
	}
}

func TestMapToDomainRoadmap_SurfacesWeeklyCapViolation(t *testing.T) {
	c := mapperFor(t, "bench-press")

	sessions := make([]SessionPlan, 0, roadmap.MaxSessionsPerWeek+1)
	start := time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC)
	for i := range roadmap.MaxSessionsPerWeek + 1 {
		date := start.AddDate(0, 0, i).Format(scheduledDateISO)
		sessions = append(sessions, sessionOf(date, "bench-press"))
	}

	_, err := c.mapToDomainRoadmap(context.Background(), planOf(sessions...), nil, "user-1", getMapNow())
	if err == nil {
		t.Fatal("got nil error, want the weekly cap to be enforced")
	}
	if !errors.Is(err, roadmap.ErrWeeklyCapExceeded) {
		t.Errorf("got error %v, want it to wrap ErrWeeklyCapExceeded", err)
	}
}

func TestMapToDomainRoadmap_EmptyPlanIsRejected(t *testing.T) {
	c := mapperFor(t)

	_, err := c.mapToDomainRoadmap(context.Background(), &GeneratedPlan{}, nil, "user-1", getMapNow())
	if err == nil {
		t.Fatal("got nil error, want a rejection for a plan with no weeks")
	}
	if !errors.Is(err, ErrPlanGenerationFailed) {
		t.Errorf("got error %v, want it to wrap ErrPlanGenerationFailed", err)
	}
}

// TestMapToDomainRoadmap_FourWeeksPassesValidateFullStructure exercises the
// whole path the way InitiateRoadmapHandler does. This is the case that would
// have caught the DayPlanID bug end to end.
func TestMapToDomainRoadmap_FourWeeksPassesValidateFullStructure(t *testing.T) {
	c := mapperFor(t, "bench-press", "squat")

	phases := []string{"ACCUMULATION", "OVERLOAD", "PEAK", "DELOAD"}
	start := time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC)

	weeks := make([]WeekPlan, 0, roadmap.WeeksPerRoadmap)
	for w := range roadmap.WeeksPerRoadmap {
		sessions := make([]SessionPlan, 0, 3)
		for d := range 3 {
			date := start.AddDate(0, 0, w*7+d*2).Format(scheduledDateISO)
			sessions = append(sessions, sessionOf(date, "bench-press", "squat"))
		}
		weeks = append(weeks, WeekPlan{
			WeekNumber:   w + 1,
			Phase:        phases[w],
			TargetRPEMin: 6.0,
			TargetRPEMax: 7.0,
			Sessions:     sessions,
		})
	}

	rm, err := c.mapToDomainRoadmap(context.Background(), &GeneratedPlan{Weeks: weeks}, nil, "user-1", getMapNow())
	if err != nil {
		t.Fatalf("mapToDomainRoadmap returned error: %v", err)
	}

	if err := rm.ValidateFullStructure(); err != nil {
		t.Errorf("ValidateFullStructure returned error: %v", err)
	}
	if got := len(rm.Weeks()); got != roadmap.WeeksPerRoadmap {
		t.Errorf("got %d weeks, want %d", got, roadmap.WeeksPerRoadmap)
	}
	for _, w := range rm.Weeks() {
		if got := w.TotalSessions(); got != 3 {
			t.Errorf("week %d: got %d sessions, want 3", w.WeekNumber(), got)
		}
	}
}

func TestMapToRegeneratedSessions_LeavesIdentityFieldsEmpty(t *testing.T) {
	c := mapperFor(t, "bench-press")
	plan := planOf(sessionOf("2026-08-03", "bench-press"))

	infos := c.mapToRegeneratedSessions(context.Background(), plan, nil, "user-1", getMapNow())
	if len(infos) != 1 {
		t.Fatalf("got %d session infos, want 1", len(infos))
	}

	got := infos[0]
	// Identity belongs to the caller, which already holds the sessions being
	// rewritten; minting IDs here would orphan the real rows.
	if got.SessionPlanID != "" || got.DayPlanID != "" || got.WeekPlanID != "" || got.RoadmapID != "" {
		t.Errorf("got identity fields %+v, want all empty", got)
	}
	if got.UserID != "user-1" {
		t.Errorf("got UserID %q, want user-1", got.UserID)
	}
	if len(got.Prescription.MainExercises) != 1 {
		t.Errorf("got %d main exercises, want 1", len(got.Prescription.MainExercises))
	}
}

func TestMapToRegeneratedSessions_SkipsMalformedDate(t *testing.T) {
	c := mapperFor(t, "bench-press")
	plan := planOf(
		sessionOf("nope", "bench-press"),
		sessionOf("2026-08-05", "bench-press"),
	)

	infos := c.mapToRegeneratedSessions(context.Background(), plan, nil, "user-1", getMapNow())
	if len(infos) != 1 {
		t.Fatalf("got %d session infos, want 1: the malformed one must be skipped", len(infos))
	}
	want := time.Date(2026, 8, 5, 0, 0, 0, 0, time.UTC)
	if !infos[0].ScheduledDate.Equal(want) {
		t.Errorf("got ScheduledDate %v, want %v", infos[0].ScheduledDate, want)
	}
}

func TestGroupSessionsByDate_PreservesFirstSeenOrder(t *testing.T) {
	sessions := []SessionPlan{
		sessionOf("2026-08-09", "a"),
		sessionOf("2026-08-03", "b"),
		sessionOf("2026-08-09", "c"),
		sessionOf("2026-08-05", "d"),
	}

	// Repeat: a map-ranging implementation would pass intermittently.
	for range 20 {
		_, order, err := groupSessionsByDate(sessions)
		if err != nil {
			t.Fatalf("groupSessionsByDate returned error: %v", err)
		}
		want := []string{"2026-08-09", "2026-08-03", "2026-08-05"}
		if len(order) != len(want) {
			t.Fatalf("got %v, want %v", order, want)
		}
		for i := range want {
			if order[i] != want[i] {
				t.Fatalf("got order %v, want %v", order, want)
			}
		}
	}
}
