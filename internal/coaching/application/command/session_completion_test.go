package command

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	domainevent "github.com/viethung213/gym-companion/internal/coaching/domain/event"
	"github.com/viethung213/gym-companion/internal/coaching/domain/roadmap"
	"github.com/viethung213/gym-companion/internal/coaching/domain/service"
)

func seedRoadmap(t *testing.T, repo *memRepo) *roadmap.Roadmap {
	t.Helper()

	base := time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC)

	wp, _ := roadmap.NewWeekPlan(&roadmap.WeekPlanInfo{
		WeekPlanID: "wp-1", RoadmapID: "rm-1", UserID: "user-1",
		WeekNumber: 1, Phase: roadmap.PhaseAccumulation, TargetRPE: 7.0,
		StartDate: base, EndDate: base.AddDate(0, 0, 6),
	})

	dp, _ := roadmap.NewDayPlan(&roadmap.DayPlanInfo{
		DayPlanID: "dp-1", WeekPlanID: "wp-1", RoadmapID: "rm-1", UserID: "user-1",
		ScheduledDate: base,
	})

	sp, _ := roadmap.NewSessionPlan(&roadmap.SessionPlanInfo{
		SessionPlanID: "sp-1", DayPlanID: "dp-1", WeekPlanID: "wp-1", RoadmapID: "rm-1",
		UserID: "user-1", ScheduledDate: base,
	}, time.Now())

	_ = dp.AddSession(sp)

	_ = wp.AddDay(dp)

	weeks := []*roadmap.WeekPlan{wp}

	// Fill remaining weeks minimally to satisfy structural checks (optional here).

	for i := 2; i <= 4; i++ {
		w, _ := roadmap.NewWeekPlan(&roadmap.WeekPlanInfo{
			WeekPlanID: "wp-" + string(rune('0'+i)), RoadmapID: "rm-1", UserID: "user-1",
			WeekNumber: int32(i),
			Phase:      []roadmap.Phase{roadmap.PhaseOverload, roadmap.PhasePeak, roadmap.PhaseDeload}[i-2],
			TargetRPE:  7.0,
			StartDate:  base.AddDate(0, 0, (i-1)*7),
			EndDate:    base.AddDate(0, 0, (i-1)*7+6),
		})

		weeks = append(weeks, w)
	}

	r, _ := roadmap.NewRoadmap(&roadmap.Info{
		RoadmapID: "rm-1", UserID: "user-1",
		StartDate: base, EndDate: base.AddDate(0, 0, 28),
	}, weeks, time.Now())

	_ = repo.Save(context.Background(), r)

	return r
}

func TestCompleteSession_MarksCompletedAndEmitsEvent(t *testing.T) {
	repo := newMemRepo()

	seedRoadmap(t, repo)

	outbox := &captureOutbox{}

	clock := &fakeClock{t: time.Date(2026, 8, 3, 8, 0, 0, 0, time.UTC)}

	h := NewCompleteSessionHandler(fakeTx{}, repo, service.NewSCRCalculator(), outbox, clock)

	res, err := h.Handle(context.Background(), CompleteSessionCommand{
		SessionPlanID:       "sp-1",
		TotalActualSets:     12,
		TotalPrescribedSets: 15,
		AverageActualRPE:    7.5,
	})

	if err != nil {
		t.Fatalf("Handle: %v", err)
	}

	if res.SCR != 80.0 {
		t.Errorf("SCR=%v, want 80.0", res.SCR)
	}

	if res.DeltaRPE != 0.5 { // 7.5 - 7.0

		t.Errorf("DeltaRPE=%v, want 0.5", res.DeltaRPE)
	}

	rm := repo.byID["rm-1"]

	sp, _ := rm.FindSession("sp-1")

	if sp.Status() != roadmap.SessionPlanStatusCompleted {
		t.Errorf("session status=%s", sp.Status())
	}

	if len(outbox.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(outbox.events))
	}

	evt, ok := outbox.events[0].(*domainevent.SessionPlanExecuted)
	if !ok {
		t.Fatalf("expected SessionPlanExecuted, got %T", outbox.events[0])
	}
	if !evt.EffortReported || evt.SessionDeltaRPE != 0.5 {
		t.Errorf("event effort reported %v ΔRPE %v, want true and 0.5", evt.EffortReported, evt.SessionDeltaRPE)
	}
}

func TestAbortSession_MarksAborted(t *testing.T) {
	repo := newMemRepo()

	seedRoadmap(t, repo)

	outbox := &captureOutbox{}

	h := NewAbortSessionHandler(fakeTx{}, repo, outbox, &fakeClock{t: time.Now()})

	err := h.Handle(context.Background(), AbortSessionCommand{SessionPlanID: "sp-1", Reason: "user_cancelled"})

	if err != nil {
		t.Fatalf("Handle: %v", err)
	}

	rm := repo.byID["rm-1"]

	sp, _ := rm.FindSession("sp-1")

	if sp.Status() != roadmap.SessionPlanStatusAborted {
		t.Errorf("session status=%s, want ABORTED", sp.Status())
	}

	if len(outbox.events) != 1 {
		t.Errorf("expected 1 RoadmapAdjusted, got %d", len(outbox.events))
	}

	_ = port.OutboxRecord{} // keep port import
}

// plan_id comes from the client, so an event for another user's session must
// not complete it.
func TestCompleteSession_RejectsForeignSession(t *testing.T) {
	repo := newMemRepo()
	seedRoadmap(t, repo)
	outbox := &captureOutbox{}
	h := NewCompleteSessionHandler(fakeTx{}, repo, service.NewSCRCalculator(), outbox,
		&fakeClock{t: time.Date(2026, 8, 3, 8, 0, 0, 0, time.UTC)})

	_, err := h.Handle(context.Background(), CompleteSessionCommand{
		SessionPlanID: "sp-1", UserID: "intruder", TotalActualSets: 3, AverageActualRPE: 7,
	})
	if !errors.Is(err, roadmap.ErrSessionNotFound) {
		t.Fatalf("err = %v, want ErrSessionNotFound", err)
	}
	sp, _ := repo.byID["rm-1"].FindSession("sp-1")
	if sp.Status() != roadmap.SessionPlanStatusPending {
		t.Errorf("status = %s, want PENDING", sp.Status())
	}
	if len(outbox.events) != 0 {
		t.Errorf("emitted %d events for a foreign session", len(outbox.events))
	}
}

func TestAbortSession_RejectsForeignSession(t *testing.T) {
	repo := newMemRepo()
	seedRoadmap(t, repo)
	h := NewAbortSessionHandler(fakeTx{}, repo, &captureOutbox{}, &fakeClock{t: time.Now()})

	err := h.Handle(context.Background(), AbortSessionCommand{SessionPlanID: "sp-1", UserID: "intruder"})
	if !errors.Is(err, roadmap.ErrSessionNotFound) {
		t.Fatalf("err = %v, want ErrSessionNotFound", err)
	}
}

// ΔRPE is measured against the session's own target: an easy recovery session
// in a PEAK week finished at its RPE 3 is on target, not 5.5 points too easy.
func TestSessionTargetRPE_UsesSessionPrescription(t *testing.T) {
	base := time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC)
	wp, _ := roadmap.NewWeekPlan(&roadmap.WeekPlanInfo{
		WeekPlanID: "wp-3", RoadmapID: "rm-1", UserID: "user-1",
		WeekNumber: 3, Phase: roadmap.PhasePeak, TargetRPE: 8.5,
		StartDate: base, EndDate: base.AddDate(0, 0, 6),
	})
	rm, _ := roadmap.NewRoadmap(&roadmap.Info{
		RoadmapID: "rm-1", UserID: "user-1", StartDate: base, EndDate: base.AddDate(0, 0, 28),
	}, []*roadmap.WeekPlan{wp}, base)

	recovery, _ := roadmap.NewSessionPlan(&roadmap.SessionPlanInfo{
		SessionPlanID: "sp-r", DayPlanID: "dp", WeekPlanID: "wp-3", RoadmapID: "rm-1",
		UserID: "user-1", ScheduledDate: base, Kind: roadmap.SessionKindRecovery,
		Prescription: roadmap.WorkoutPrescription{MainExercises: []roadmap.PrescribedExercise{
			{ExerciseID: "a", TargetRPE: 3}, {ExerciseID: "b", TargetRPE: 3},
		}},
	}, base)
	if got := sessionTargetRPE(rm, recovery); got != 3 {
		t.Errorf("recovery target = %v, want 3", got)
	}

	empty, _ := roadmap.NewSessionPlan(&roadmap.SessionPlanInfo{
		SessionPlanID: "sp-e", DayPlanID: "dp", WeekPlanID: "wp-3", RoadmapID: "rm-1",
		UserID: "user-1", ScheduledDate: base,
	}, base)
	if got := sessionTargetRPE(rm, empty); got != 8.5 {
		t.Errorf("fallback target = %v, want the week's 8.5", got)
	}
}

// An athlete who does not rate the session (RPE 0) has not said it felt easy:
// ΔRPE stays unmeasured instead of reading 0 - 7 = -7.
func TestCompleteSession_UnratedLeavesDeltaUnmeasured(t *testing.T) {
	repo := newMemRepo()
	seedRoadmap(t, repo)
	outbox := &captureOutbox{}
	h := NewCompleteSessionHandler(fakeTx{}, repo, service.NewSCRCalculator(), outbox,
		&fakeClock{t: time.Date(2026, 8, 3, 8, 0, 0, 0, time.UTC)})

	res, err := h.Handle(context.Background(), CompleteSessionCommand{
		SessionPlanID: "sp-1", TotalActualSets: 15, TotalPrescribedSets: 15,
	})
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	info := mustSession(t, repo, "sp-1").Info()
	if info.SessionDeltaRPE != nil {
		t.Errorf("ΔRPE = %v, want unmeasured", *info.SessionDeltaRPE)
	}
	if res.SCR != 100 || info.SessionSCR == nil || *info.SessionSCR != 100 {
		t.Errorf("SCR = %v, want 100 recorded", res.SCR)
	}
	if evt, ok := outbox.events[0].(*domainevent.SessionPlanExecuted); !ok || evt.EffortReported {
		t.Errorf("event %T reports an effort; want SessionPlanExecuted with EffortReported false", outbox.events[0])
	}
}

// A finished solo challenge counts as fully done, with no effort reported.
func TestSoloMatchCompletion(t *testing.T) {
	repo := newMemRepo()
	seedRoadmap(t, repo)
	h := NewCompleteSessionHandler(fakeTx{}, repo, service.NewSCRCalculator(), &captureOutbox{},
		&fakeClock{t: time.Date(2026, 8, 3, 8, 0, 0, 0, time.UTC)})

	if _, err := h.Handle(context.Background(), SoloMatchCompletion("sp-1", "user-1", "")); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	info := mustSession(t, repo, "sp-1").Info()
	if info.SessionSCR == nil || *info.SessionSCR != 100 || info.SessionDeltaRPE != nil {
		t.Errorf("SCR %v ΔRPE %v, want 100 and unmeasured", info.SessionSCR, info.SessionDeltaRPE)
	}
}

func mustSession(t *testing.T, repo *memRepo, id string) *roadmap.SessionPlan {
	t.Helper()
	s, ok := repo.byID["rm-1"].FindSession(id)
	if !ok {
		t.Fatalf("no session %s", id)
	}
	return s
}
