package command

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	"github.com/viethung213/gym-companion/internal/coaching/domain/guardrail"
	"github.com/viethung213/gym-companion/internal/coaching/domain/roadmap"
	"github.com/viethung213/gym-companion/internal/coaching/domain/service"
	"github.com/viethung213/gym-companion/internal/coaching/domain/taxonomy"
)

// weekOneEnd is when week 1 of reviewFixture is over (UTC roadmap).
func weekOneEnd() time.Time { return time.Date(2026, 11, 9, 0, 0, 0, 0, time.UTC) }

// reviewFixture is a 2-week UTC roadmap from Mon 2026-11-02: three scheduled
// training sessions a week (Mon/Wed/Fri 18:00) plus a recovery session in week 1.
func reviewFixture(t *testing.T) *roadmap.Roadmap {
	t.Helper()
	start := time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)
	phases := []roadmap.Phase{roadmap.PhaseAccumulation, roadmap.PhaseOverload}
	rpes := []float32{6.5, 7.5}

	weeks := make([]*roadmap.WeekPlan, 0, 2)
	for wi := range 2 {
		ws := start.AddDate(0, 0, 7*wi)
		wid := "w" + string(rune('1'+wi))
		var days []*roadmap.DayPlan
		for di, offset := range []int{0, 2, 4} {
			date := ws.AddDate(0, 0, offset)
			did := wid + "-d" + string(rune('0'+di))
			sessions := []*roadmap.SessionPlan{reviewSession(t, wid+"-s"+string(rune('0'+di)), did, wid, date,
				roadmap.SessionKindTraining, rpes[wi])}
			if wi == 0 && di == 2 {
				sessions = append(sessions, reviewSession(t, "w1-rec", did, wid, date, roadmap.SessionKindRecovery, 3))
			}
			day, err := roadmap.RehydrateDayPlan(&roadmap.DayPlanInfo{
				DayPlanID: did, WeekPlanID: wid, RoadmapID: "rm-r", UserID: "u-r", ScheduledDate: date,
			}, sessions)
			if err != nil {
				t.Fatal(err)
			}
			days = append(days, day)
		}
		w, err := roadmap.RehydrateWeekPlan(&roadmap.WeekPlanInfo{
			WeekPlanID: wid, RoadmapID: "rm-r", UserID: "u-r", WeekNumber: int32(wi + 1),
			Phase: phases[wi], TargetRPE: rpes[wi], StartDate: ws, EndDate: ws.AddDate(0, 0, 7),
		}, days)
		if err != nil {
			t.Fatal(err)
		}
		weeks = append(weeks, w)
	}

	rm, err := roadmap.RehydrateRoadmap(&roadmap.Info{
		RoadmapID: "rm-r", UserID: "u-r", Status: roadmap.StatusActive,
		StartDate: start, EndDate: start.AddDate(0, 0, 28), CreatedAt: start,
	}, weeks)
	if err != nil {
		t.Fatal(err)
	}
	return rm
}

func reviewSession(t *testing.T, id, dayID, weekID string, date time.Time, kind roadmap.SessionKind, rpe float32) *roadmap.SessionPlan {
	t.Helper()
	sp, err := roadmap.NewSessionPlan(&roadmap.SessionPlanInfo{
		SessionPlanID: id, DayPlanID: dayID, WeekPlanID: weekID, RoadmapID: "rm-r", UserID: "u-r",
		ScheduledDate: date, SlotTime: "18:00-19:00", EstimatedDurationMinutes: 60, Kind: kind,
		Prescription: roadmap.WorkoutPrescription{MainExercises: []roadmap.PrescribedExercise{{
			ExerciseID: "ex-1", ExerciseName: "Squat", TargetSets: 3, TargetReps: 8, TargetWeight: 60, TargetRPE: rpe,
		}}},
	}, date)
	if err != nil {
		t.Fatal(err)
	}
	return sp
}

// complete marks the given week-1 sessions done with the given SCR and ΔRPE.
func complete(t *testing.T, rm *roadmap.Roadmap, scr, delta float32, ids ...string) {
	t.Helper()
	for _, id := range ids {
		s, ok := rm.FindSession(id)
		if !ok {
			t.Fatalf("no session %s", id)
		}
		if err := s.MarkCompleted(scr, delta, weekOneEnd().Add(-time.Hour)); err != nil {
			t.Fatal(err)
		}
	}
}

// adaptingAgent rewrites week 2's sessions and remembers the decision.
type adaptingAgent struct {
	fakeCoachAgent
	rm       *roadmap.Roadmap
	decision *service.AdaptationDecision // nil until Adapt is called
	failures int                         // Adapt fails this many times first, as when the model is overloaded
}

func (a *adaptingAgent) Adapt(_ context.Context, _ string, decision *service.AdaptationDecision) ([]*roadmap.SessionPlanInfo, error) {
	if a.failures > 0 {
		a.failures--
		return nil, errors.New("model overloaded")
	}
	a.decision = decision
	pending := a.rm.UpcomingPendingSessions(weekOneEnd())
	drafts := make([]*roadmap.SessionPlanInfo, 0, len(pending))
	for _, s := range pending {
		info := s.Info()
		info.Reasoning = "adapted"
		drafts = append(drafts, &info)
	}
	return drafts, nil
}

func newReviewHandler(rm *roadmap.Roadmap, now time.Time) (*ReviewWeeksHandler, *adaptingAgent, *memRepo) {
	repo := newMemRepo()
	_ = repo.Save(context.Background(), rm)
	clock := &fakeClock{t: now}
	agent := &adaptingAgent{rm: rm}
	adapt := NewApplyAdaptiveCycleHandler(fakeTx{}, repo, agent,
		guardrail.NewEngine(service.NewOverloadValidator(), nil, nil), zoneProfiles{loc: time.UTC}, &captureOutbox{}, clock, nil,
		barbellCatalog(), nil)
	return NewReviewWeeksHandler(fakeTx{}, repo, adapt, clock), agent, repo
}

// barbellCatalog resolves every exercise to a barbell lift.
func barbellCatalog() *exerciseCatalogMock {
	return &exerciseCatalogMock{fn: func(_ context.Context, id string) (port.Exercise, error) {
		return port.Exercise{ExerciseID: id, Name: id, PrimaryMuscle: taxonomy.MuscleGroupQuadriceps,
			Equipment: taxonomy.EquipmentBarbell, Level: taxonomy.LevelIntermediate, MeasurementType: "REPS"}, nil
	}}
}

// A fully trained week at the planned effort progresses the load, and the
// remaining sessions are rewritten with that decision.
func TestReviewWeeks_CompletedWeekProgressesLoad(t *testing.T) {
	rm := reviewFixture(t)
	complete(t, rm, 100, 0, "w1-s0", "w1-s1", "w1-s2")
	h, agent, _ := newReviewHandler(rm, weekOneEnd().Add(8*time.Hour))

	res, err := h.Handle(context.Background(), "rm-r")
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if len(res.DueWeeks) != 1 || res.DueWeeks[0] != 1 {
		t.Fatalf("closed weeks = %v, want [1]", res.DueWeeks)
	}
	m := res.Metrics[0]
	if m.TotalSessions != 3 || m.SCR != 100 {
		t.Errorf("metrics = %+v, want 3 training sessions (recovery left out) at SCR 100", m)
	}
	if res.Adaptation == nil || res.Adaptation.Decision.Kind != service.AdaptationIncreaseLoad {
		t.Fatalf("adaptation = %+v, want IncreaseLoad", res.Adaptation)
	}
	if agent.decision == nil || agent.decision.Kind != service.AdaptationIncreaseLoad {
		t.Errorf("decision %+v, want the progression passed to the agent", agent.decision)
	}
	if s, _ := rm.FindSession("w2-s0"); s.Info().Reasoning != "adapted" {
		t.Error("week 2 was not rewritten")
	}
}

// Sessions left pending when the week ends were missed: they become SKIPPED
// and count as 0% done. One session out of three is no reason to adapt.
func TestReviewWeeks_MissedSessionsAreSkipped(t *testing.T) {
	rm := reviewFixture(t)
	complete(t, rm, 100, 0, "w1-s0")
	h, agent, _ := newReviewHandler(rm, weekOneEnd().Add(8*time.Hour))

	res, err := h.Handle(context.Background(), "rm-r")
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	for _, id := range []string{"w1-s1", "w1-s2", "w1-rec"} {
		if s, _ := rm.FindSession(id); s.Status() != roadmap.SessionPlanStatusSkipped {
			t.Errorf("%s = %s, want SKIPPED", id, s.Status())
		}
	}
	if got := res.Metrics[0].SCR; math.Abs(got-100.0/3) > 0.01 {
		t.Errorf("SCR = %v, want 33.3", got)
	}
	if res.Adaptation == nil || res.Adaptation.Decision.Kind != service.AdaptationNoOp || agent.decision != nil {
		t.Errorf("adaptation = %+v, agent got %+v: want a no-op without calling the agent", res.Adaptation, agent.decision)
	}
}

// Three sessions felt at RPE 9 (target 6.5 + 2.5) mean excessive fatigue.
func TestReviewWeeks_ExhaustingWeekUnloads(t *testing.T) {
	rm := reviewFixture(t)
	complete(t, rm, 100, 2.5, "w1-s0", "w1-s1", "w1-s2")
	h, agent, _ := newReviewHandler(rm, weekOneEnd().Add(8*time.Hour))

	res, err := h.Handle(context.Background(), "rm-r")
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if res.Metrics[0].HighRPESessions != 3 {
		t.Errorf("high-RPE sessions = %d, want 3", res.Metrics[0].HighRPESessions)
	}
	if res.Adaptation == nil || res.Adaptation.Decision.Kind != service.AdaptationTriggerDeload {
		t.Fatalf("adaptation = %+v, want TriggerDeload", res.Adaptation)
	}
	// The agent kept 3 sets; the handler enforces the 30% cut: 3 -> 2.
	for _, id := range []string{"w2-s0", "w2-s1", "w2-s2"} {
		if s, _ := rm.FindSession(id); s.MainSets() != 2 {
			t.Errorf("%s main sets = %d, want 2 after the deload cut", id, s.MainSets())
		}
	}
	if agent.decision == nil || agent.decision.Kind != service.AdaptationTriggerDeload {
		t.Errorf("decision %+v, want the deload passed to the agent", agent.decision)
	}
}

// A week closes only after the grace period, and never twice.
func TestReviewWeeks_GraceAndIdempotence(t *testing.T) {
	rm := reviewFixture(t)
	early, _, _ := newReviewHandler(rm, weekOneEnd().Add(time.Hour))
	if res, err := early.Handle(context.Background(), "rm-r"); err != nil || len(res.DueWeeks) != 0 {
		t.Fatalf("within the grace period: closed %v, err %v", res.DueWeeks, err)
	}

	h, _, _ := newReviewHandler(rm, weekOneEnd().Add(8*time.Hour))
	if _, err := h.Handle(context.Background(), "rm-r"); err != nil {
		t.Fatal(err)
	}
	again, err := h.Handle(context.Background(), "rm-r")
	if err != nil || len(again.DueWeeks) != 0 {
		t.Errorf("second review closed %v (err %v), want nothing", again.DueWeeks, err)
	}
}

// A failed adaptation (model overloaded) leaves the week open, so the next run
// retries it with the same metrics; the week is stamped only once it worked.
func TestReviewWeeks_FailedAdaptationIsRetried(t *testing.T) {
	rm := reviewFixture(t)
	complete(t, rm, 100, 0, "w1-s0", "w1-s1", "w1-s2")
	h, agent, _ := newReviewHandler(rm, weekOneEnd().Add(7*time.Hour))
	agent.failures = 1

	first, err := h.Handle(context.Background(), "rm-r")
	if err == nil || !first.Deferred {
		t.Fatalf("first run: err %v deferred %v, want a deferred failure", err, first.Deferred)
	}
	if rm.Weeks()[0].Reviewed() {
		t.Fatal("week 1 stamped reviewed although its adaptation failed")
	}

	second, err := h.Handle(context.Background(), "rm-r")
	if err != nil {
		t.Fatalf("retry: %v", err)
	}
	if second.Adaptation == nil || second.Adaptation.Decision.Kind != service.AdaptationIncreaseLoad {
		t.Fatalf("retry adaptation = %+v, want IncreaseLoad", second.Adaptation)
	}
	if !rm.Weeks()[0].Reviewed() {
		t.Error("week 1 not stamped after a successful retry")
	}
}

// Past the retry window the week closes anyway, so a lasting outage does not
// keep calling the model every run.
func TestReviewWeeks_GivesUpAfterRetryWindow(t *testing.T) {
	rm := reviewFixture(t)
	complete(t, rm, 100, 0, "w1-s0", "w1-s1", "w1-s2")
	h, agent, _ := newReviewHandler(rm, weekOneEnd().Add(adaptRetryWindow+time.Hour))
	agent.failures = 1

	res, err := h.Handle(context.Background(), "rm-r")
	if err == nil || res.Deferred {
		t.Fatalf("err %v deferred %v, want the failure reported but not deferred", err, res.Deferred)
	}
	if !rm.Weeks()[0].Reviewed() {
		t.Error("week 1 left open past the retry window")
	}
}

// A week trained in full without any RPE gives no effort signal: the plan is
// held. Reading the missing ΔRPE as 0 ("on target") would raise the load every
// week with nothing to back it.
func TestReviewWeeks_UnratedWeekHoldsThePlan(t *testing.T) {
	rm := reviewFixture(t)
	for _, id := range []string{"w1-s0", "w1-s1", "w1-s2"} {
		s, _ := rm.FindSession(id)
		if err := s.MarkCompletedUnrated(100, weekOneEnd().Add(-time.Hour)); err != nil {
			t.Fatal(err)
		}
	}
	h, _, _ := newReviewHandler(rm, weekOneEnd().Add(8*time.Hour))

	res, err := h.Handle(context.Background(), "rm-r")
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if m := res.Metrics[0]; m.SCR != 100 || !m.EffortUnreported {
		t.Errorf("metrics = %+v, want SCR 100 with effort unreported", m)
	}
	if res.Adaptation == nil || res.Adaptation.Decision.Kind != service.AdaptationNoOp {
		t.Errorf("adaptation = %+v, want the plan held (NoOp)", res.Adaptation)
	}
}

// The weeks reviewed are stamped in the same save as the adaptation, so a
// review whose own stamping fails is not adapted a second time on retry.
func TestReviewWeeks_AdaptationStampsTheWeek(t *testing.T) {
	rm := reviewFixture(t)
	complete(t, rm, 100, 0, "w1-s0", "w1-s1", "w1-s2")
	h, agent, _ := newReviewHandler(rm, weekOneEnd().Add(8*time.Hour))
	if _, err := h.Handle(context.Background(), "rm-r"); err != nil {
		t.Fatal(err)
	}
	if !rm.Weeks()[0].Reviewed() {
		t.Fatal("week 1 not stamped")
	}

	// Replay the adaptation for week 1, as a retry would: nothing is applied.
	agent.decision = nil
	again, err := h.adapt.Handle(context.Background(), ApplyAdaptiveCycleCommand{
		UserID: "u-r", Metrics: []service.WeeklyMetrics{{WeekNumber: 1, SCR: 100, TotalSessions: 3}},
		ReviewedWeekIDs: []string{rm.Weeks()[0].ID()},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !again.AlreadyApplied || agent.decision != nil {
		t.Errorf("replay: AlreadyApplied %v, agent got %+v; want a no-op", again.AlreadyApplied, agent.decision)
	}
}

// When the newest due week had no scheduled training, the latest metrics are an
// older week that was already adapted to: it must not be applied again.
func TestReviewWeeks_NoAdaptationWithoutMetricsForTheNewestWeek(t *testing.T) {
	rm := reviewFixture(t)
	h, agent, _ := newReviewHandler(rm, weekOneEnd().Add(8*time.Hour))
	res := &ReviewWeeksResult{Metrics: []service.WeeklyMetrics{{WeekNumber: 1, SCR: 100, TotalSessions: 3}}}

	err := h.runAdaptation(context.Background(), rm, []*roadmap.WeekPlan{rm.Weeks()[1]}, res, weekOneEnd())
	if err != nil || res.Adaptation != nil || agent.decision != nil {
		t.Errorf("err %v adaptation %+v agent got %+v; want nothing applied", err, res.Adaptation, agent.decision)
	}
}

// RPE is relative to the plan: sessions felt just under one RPE above their
// target are hard but on plan, one RPE above is a fatigue signal.
func TestReviewWeeks_HighRPEIsRelativeToThePlan(t *testing.T) {
	for _, tc := range []struct {
		delta float32
		high  int
	}{{0.9, 0}, {1.0, 3}} {
		rm := reviewFixture(t)
		complete(t, rm, 100, tc.delta, "w1-s0", "w1-s1", "w1-s2")
		h, _, _ := newReviewHandler(rm, weekOneEnd().Add(8*time.Hour))

		res, err := h.Handle(context.Background(), "rm-r")
		if err != nil {
			t.Fatalf("Handle: %v", err)
		}
		if got := res.Metrics[0].HighRPESessions; got != tc.high {
			t.Errorf("ΔRPE %+.1f over target 6.5: high-RPE sessions = %d, want %d", tc.delta, got, tc.high)
		}
	}
}

// A session stopped for pain is left out of the week: the app promises it
// does not count against the athlete. Stopped for lack of time, it counts.
func TestReviewWeeks_PainStopIsLeftOut(t *testing.T) {
	for _, tc := range []struct {
		reason string
		scr    float64
	}{{"pain: lower back", 100}, {"out-of-time", 200.0 / 3}} {
		rm := reviewFixture(t)
		complete(t, rm, 100, 0, "w1-s0", "w1-s1")
		s, _ := rm.FindSession("w1-s2")
		if err := s.MarkAborted(tc.reason); err != nil {
			t.Fatal(err)
		}
		h, _, _ := newReviewHandler(rm, weekOneEnd().Add(8*time.Hour))

		res, err := h.Handle(context.Background(), "rm-r")
		if err != nil {
			t.Fatalf("Handle: %v", err)
		}
		if got := res.Metrics[0].SCR; math.Abs(got-tc.scr) > 0.01 {
			t.Errorf("%q: SCR = %v, want %v", tc.reason, got, tc.scr)
		}
	}
}

// A week trained as planned progresses the rest of the block by the rules,
// not by the model: week 2's barbell squat, 3 x 8 at 60 kg, gains reps in the
// hypertrophy range (8 -> 10) at the same load, whatever the model sent back.
func TestReviewWeeks_ProgressionFollowsTheRules(t *testing.T) {
	rm := reviewFixture(t)
	complete(t, rm, 100, 0, "w1-s0", "w1-s1", "w1-s2")
	h, _, _ := newReviewHandler(rm, weekOneEnd().Add(8*time.Hour))

	res, err := h.Handle(context.Background(), "rm-r")
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if res.Adaptation == nil || res.Adaptation.Decision.Kind != service.AdaptationIncreaseLoad {
		t.Fatalf("adaptation = %+v, want IncreaseLoad", res.Adaptation)
	}
	for _, id := range []string{"w2-s0", "w2-s1", "w2-s2"} {
		s, _ := rm.FindSession(id)
		m := s.Info().Prescription.MainExercises[0]
		if m.TargetSets != 3 || m.TargetReps != 10 || m.TargetWeight != 60 {
			t.Errorf("%s: %dx%d at %.1f kg, want 3x10 at 60 kg", id, m.TargetSets, m.TargetReps, m.TargetWeight)
		}
	}
}

// An exercise an injury limits keeps its load: progression never adds work to
// an injured area (a mild knee injury touches the barbell squat).
func TestReviewWeeks_ProgressionHoldsAnInjuredExercise(t *testing.T) {
	rm := reviewFixture(t)
	complete(t, rm, 100, 0, "w1-s0", "w1-s1", "w1-s2")
	repo := newMemRepo()
	_ = repo.Save(context.Background(), rm)
	clock := &fakeClock{t: weekOneEnd().Add(8 * time.Hour)}
	knee := injuredProfiles{loc: time.UTC, injuries: []port.InjuryStatus{
		{Region: taxonomy.JointRegionKnee, Severity: taxonomy.InjurySeverityMild},
	}}
	adapt := NewApplyAdaptiveCycleHandler(fakeTx{}, repo, &adaptingAgent{rm: rm},
		guardrail.NewEngine(service.NewOverloadValidator(), nil, nil), knee, &captureOutbox{}, clock, nil,
		barbellCatalog(), nil)

	if _, err := NewReviewWeeksHandler(fakeTx{}, repo, adapt, clock).Handle(context.Background(), "rm-r"); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	s, _ := rm.FindSession("w2-s0")
	if m := s.Info().Prescription.MainExercises[0]; m.TargetReps != 8 || m.TargetWeight != 60 {
		t.Errorf("injured squat %dx%d at %.1f kg, want held at 3x8 60 kg", m.TargetSets, m.TargetReps, m.TargetWeight)
	}
}

// Without a record the progressed load stays within the bound generation
// uses (UnrecordedLoadFactor x bodyweight): a 70 kg athlete at the top of the
// rep range on a 105 kg lift does not step past 105 kg.
func TestReviewWeeks_ProgressionWithoutARecordStaysUnderTheBodyweightBound(t *testing.T) {
	rm := reviewFixture(t)
	complete(t, rm, 100, 0, "w1-s0", "w1-s1", "w1-s2")
	for _, id := range []string{"w2-s0", "w2-s1", "w2-s2"} {
		s, _ := rm.FindSession(id)
		p := s.Info().Prescription
		p.MainExercises[0].TargetWeight, p.MainExercises[0].TargetReps = 105, 12
		if err := s.RewritePrescription(p, s.Info().TargetMuscleGroups, s.Info().Reasoning, weekOneEnd()); err != nil {
			t.Fatal(err)
		}
	}
	repo := newMemRepo()
	_ = repo.Save(context.Background(), rm)
	clock := &fakeClock{t: weekOneEnd().Add(8 * time.Hour)}
	adapt := NewApplyAdaptiveCycleHandler(fakeTx{}, repo, &adaptingAgent{rm: rm},
		guardrail.NewEngine(service.NewOverloadValidator(), nil, nil), weighedProfiles{kg: 70}, &captureOutbox{}, clock, nil,
		barbellCatalog(), nil)

	if _, err := NewReviewWeeksHandler(fakeTx{}, repo, adapt, clock).Handle(context.Background(), "rm-r"); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	s, _ := rm.FindSession("w2-s0")
	if m := s.Info().Prescription.MainExercises[0]; m.TargetWeight != 105 {
		t.Errorf("load %.1f kg, want held at 105 kg (1.5 x 70 kg)", m.TargetWeight)
	}
}

// weighedProfiles reports a profile with the given bodyweight.
type weighedProfiles struct{ kg float64 }

func (p weighedProfiles) GetProfile(_ context.Context, userID string) (port.Profile, error) {
	return port.Profile{UserID: userID, Location: time.UTC, WeightKg: p.kg}, nil
}

// A failed personal-record read holds the loads (reps still climb): without
// the record ceiling a load step could be refused by the guardrail, whose own
// read may succeed, failing the whole adaptation.
func TestReviewWeeks_ProgressionHoldsLoadsWhenRecordsAreUnavailable(t *testing.T) {
	rm := reviewFixture(t)
	complete(t, rm, 100, 0, "w1-s0", "w1-s1", "w1-s2")
	for _, id := range []string{"w2-s0", "w2-s1", "w2-s2"} {
		s, _ := rm.FindSession(id)
		p := s.Info().Prescription
		p.MainExercises[0].TargetReps = 12 // top of the range: next is a load step
		if err := s.RewritePrescription(p, s.Info().TargetMuscleGroups, s.Info().Reasoning, weekOneEnd()); err != nil {
			t.Fatal(err)
		}
	}
	repo := newMemRepo()
	_ = repo.Save(context.Background(), rm)
	clock := &fakeClock{t: weekOneEnd().Add(8 * time.Hour)}
	unavailable := func(context.Context, string) map[string]float64 { return nil }
	adapt := NewApplyAdaptiveCycleHandler(fakeTx{}, repo, &adaptingAgent{rm: rm},
		guardrail.NewEngine(service.NewOverloadValidator(), nil, nil), zoneProfiles{loc: time.UTC}, &captureOutbox{}, clock, nil,
		barbellCatalog(), unavailable)

	if _, err := NewReviewWeeksHandler(fakeTx{}, repo, adapt, clock).Handle(context.Background(), "rm-r"); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	s, _ := rm.FindSession("w2-s0")
	if m := s.Info().Prescription.MainExercises[0]; m.TargetWeight != 60 || m.TargetReps != 12 {
		t.Errorf("%dx%d at %.1f kg, want held at 3x12 60 kg", m.TargetSets, m.TargetReps, m.TargetWeight)
	}
}
