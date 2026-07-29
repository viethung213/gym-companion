package command

import (
	"context"
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

	if _, ok := outbox.events[0].(*domainevent.SessionPlanExecuted); !ok {
		t.Errorf("expected SessionPlanExecuted, got %T", outbox.events[0])
	}
}

func TestAbortSession_MarksSkipped(t *testing.T) {
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

	if sp.Status() != roadmap.SessionPlanStatusSkipped {
		t.Errorf("session status=%s, want SKIPPED", sp.Status())
	}

	if len(outbox.events) != 1 {
		t.Errorf("expected 1 RoadmapAdjusted, got %d", len(outbox.events))
	}

	_ = port.OutboxRecord{} // keep port import
}
