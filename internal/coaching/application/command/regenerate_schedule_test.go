package command

import (
	"context"
	"testing"
	"time"
	_ "time/tzdata"

	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	"github.com/viethung213/gym-companion/internal/coaching/domain/guardrail"
	"github.com/viethung213/gym-companion/internal/coaching/domain/roadmap"
	"github.com/viethung213/gym-companion/internal/coaching/domain/service"
)

// draftAgent returns fixed drafts from RegeneratePending.
type draftAgent struct {
	fakeCoachAgent
	drafts []*roadmap.SessionPlanInfo
}

func (a *draftAgent) RegeneratePending(_ context.Context, _ string, _ []string) ([]*roadmap.SessionPlanInfo, error) {
	return a.drafts, nil
}

// zoneProfiles reports a profile in the given zone.
type zoneProfiles struct{ loc *time.Location }

func (z zoneProfiles) GetProfile(_ context.Context, userID string) (port.Profile, error) {
	return port.Profile{UserID: userID, Location: z.loc}, nil
}

func date(m time.Month, d int) time.Time { return time.Date(2026, m, d, 0, 0, 0, 0, time.UTC) }

// regenFixture is a UTC roadmap from Mon 2026-11-23 with a completed Monday
// session and pending Thursday / next-Monday sessions at 17:30.
func regenFixture(t *testing.T) *roadmap.Roadmap {
	t.Helper()
	start := date(time.November, 23)
	weeks := make([]*roadmap.WeekPlan, 0, roadmap.WeeksPerRoadmap)
	for n := range roadmap.WeeksPerRoadmap {
		ws := start.AddDate(0, 0, n*7)
		w, err := roadmap.NewWeekPlan(&roadmap.WeekPlanInfo{
			WeekPlanID: "w" + string(rune('1'+n)), RoadmapID: "rm-1", UserID: "u-1", WeekNumber: int32(n + 1),
			Phase: roadmap.PhaseAccumulation, TargetRPE: 7, StartDate: ws, EndDate: ws.AddDate(0, 0, 7),
		})
		if err != nil {
			t.Fatal(err)
		}
		weeks = append(weeks, w)
	}
	add := func(w int, id string, d time.Time, status roadmap.SessionPlanStatus) {
		dp, err := roadmap.NewDayPlan(&roadmap.DayPlanInfo{
			DayPlanID: "d-" + id, WeekPlanID: weeks[w].ID(), RoadmapID: "rm-1", UserID: "u-1", ScheduledDate: d,
		})
		if err != nil {
			t.Fatal(err)
		}
		s, err := roadmap.RehydrateSessionPlan(&roadmap.SessionPlanInfo{
			SessionPlanID: id, DayPlanID: dp.ID(), WeekPlanID: weeks[w].ID(), RoadmapID: "rm-1", UserID: "u-1",
			ScheduledDate: d, SlotTime: "17:30-19:00", Status: status, Source: roadmap.SessionPlanSourceScheduled,
			Reasoning: "old",
		})
		if err != nil {
			t.Fatal(err)
		}
		if err := dp.AddSession(s); err != nil {
			t.Fatal(err)
		}
		if err := weeks[w].AddDay(dp); err != nil {
			t.Fatal(err)
		}
	}
	add(0, "done", start, roadmap.SessionPlanStatusCompleted)
	add(0, "thu", date(time.November, 26), roadmap.SessionPlanStatusPending)
	add(1, "mon", date(time.November, 30), roadmap.SessionPlanStatusPending)

	rm, err := roadmap.RehydrateRoadmap(&roadmap.Info{
		RoadmapID: "rm-1", UserID: "u-1", Status: roadmap.StatusActive,
		StartDate: start, EndDate: start.AddDate(0, 0, 28),
	}, weeks)
	if err != nil {
		t.Fatal(err)
	}
	return rm
}

func newRegenHandler(t *testing.T, rm *roadmap.Roadmap, drafts []*roadmap.SessionPlanInfo, loc *time.Location) (*RegenerateScheduleHandler, *memRepo, *captureOutbox) {
	t.Helper()
	repo := newMemRepo()
	if err := repo.Save(context.Background(), rm); err != nil {
		t.Fatal(err)
	}
	outbox := &captureOutbox{}
	clock := &fakeClock{t: time.Date(2026, 11, 24, 2, 0, 0, 0, time.UTC)} // Tue
	h := NewRegenerateScheduleHandler(fakeTx{}, repo, &draftAgent{drafts: drafts},
		guardrail.NewEngine(service.NewOverloadValidator(), nil, nil), zoneProfiles{loc: loc}, outbox, clock, &incrIDs{})
	return h, repo, outbox
}

func TestRegenerateSchedule_RewritesInPlaceWhenSlotsUnchanged(t *testing.T) {
	rm := regenFixture(t)
	drafts := []*roadmap.SessionPlanInfo{
		{ScheduledDate: date(time.November, 26), SlotTime: "17:30-19:00", Reasoning: "new thu"},
		{ScheduledDate: date(time.November, 30), SlotTime: "17:30-19:00", Reasoning: "new mon"},
	}
	h, _, outbox := newRegenHandler(t, rm, drafts, time.UTC)

	res, err := h.Handle(context.Background(), RegenerateScheduleCommand{UserID: "u-1", Reason: "profile_updated"})
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	thu, ok := res.Roadmap.FindSession("thu")
	if !ok || thu.Info().Reasoning != "new thu" {
		t.Fatalf("thu session should be rewritten in place, got ok=%v %+v", ok, thu)
	}
	if len(outbox.events) == 0 {
		t.Fatal("RoadmapAdjusted should be enqueued")
	}
}

func TestRegenerateSchedule_ReschedulesWhenSlotsChanged(t *testing.T) {
	rm := regenFixture(t)
	ict, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		t.Fatal(err)
	}
	drafts := []*roadmap.SessionPlanInfo{
		{ScheduledDate: date(time.November, 25), SlotTime: "06:00-07:00", Reasoning: "wed"},
		{ScheduledDate: date(time.November, 27), SlotTime: "06:00-07:00", Reasoning: "fri"},
		{ScheduledDate: date(time.December, 2), SlotTime: "06:00-07:00", Reasoning: "next wed"},
	}
	h, repo, _ := newRegenHandler(t, rm, drafts, ict)

	if _, err := h.Handle(context.Background(), RegenerateScheduleCommand{UserID: "u-1", Reason: "profile_updated"}); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	saved, _ := repo.FindActiveByUser(context.Background(), "u-1")
	if _, ok := saved.FindSession("thu"); ok {
		t.Error("old Thursday session should be replaced")
	}
	if _, ok := saved.FindSession("done"); !ok {
		t.Error("completed session must be kept")
	}
	if got := saved.Info().Timezone; got != "Asia/Ho_Chi_Minh" {
		t.Errorf("roadmap timezone = %q, want the profile's zone", got)
	}
	upcoming := saved.UpcomingPendingSessions(time.Date(2026, 11, 24, 2, 0, 0, 0, time.UTC))
	got := make([]string, 0, len(upcoming))
	for _, s := range upcoming {
		got = append(got, s.Info().Reasoning)
	}
	if len(got) != 3 || got[0] != "wed" || got[1] != "fri" || got[2] != "next wed" {
		t.Fatalf("upcoming sessions = %v, want [wed fri next wed]", got)
	}
}

// A draft the agent could not salvage is missing: the session it would have
// rewritten stays as it was, and nothing is deleted or shifted.
func TestRegenerateSchedule_DroppedDraftLeavesItsSessionUntouched(t *testing.T) {
	rm := regenFixture(t)
	drafts := []*roadmap.SessionPlanInfo{
		{ScheduledDate: date(time.November, 30), SlotTime: "17:30-19:00", Reasoning: "new mon"},
	}
	h, repo, _ := newRegenHandler(t, rm, drafts, time.UTC)

	if _, err := h.Handle(context.Background(), RegenerateScheduleCommand{UserID: "u-1", Reason: "injury_reported"}); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	saved, _ := repo.FindActiveByUser(context.Background(), "u-1")
	thu, ok := saved.FindSession("thu")
	if !ok || thu.Info().Reasoning != "old" {
		t.Fatalf("thu must be kept unchanged, got ok=%v %+v", ok, thu)
	}
	mon, _ := saved.FindSession("mon")
	if mon.Info().Reasoning != "new mon" {
		t.Fatalf("mon reasoning = %q, want rewritten", mon.Info().Reasoning)
	}
}

func TestAlignDrafts_MatchesByScheduleNotPosition(t *testing.T) {
	rm := regenFixture(t)
	pending := rm.UpcomingPendingSessions(time.Date(2026, 11, 24, 2, 0, 0, 0, time.UTC))
	drafts := []*roadmap.SessionPlanInfo{ // reversed order
		{ScheduledDate: date(time.November, 30), SlotTime: "17:30-19:00", Reasoning: "mon"},
		{ScheduledDate: date(time.November, 26), SlotTime: "17:30", Reasoning: "thu"},
	}
	aligned, ok := alignDrafts(pending, drafts)
	if !ok || aligned[0].Reasoning != "thu" || aligned[1].Reasoning != "mon" {
		t.Fatalf("aligned = %+v ok=%v, want [thu mon]", aligned, ok)
	}

	moved := []*roadmap.SessionPlanInfo{{ScheduledDate: date(time.November, 27), SlotTime: "17:30-19:00"}}
	if _, ok := alignDrafts(pending, moved); ok {
		t.Fatal("a draft on another slot must not align")
	}
	recovery := []*roadmap.SessionPlanInfo{{ScheduledDate: date(time.November, 26), SlotTime: "17:30-19:00", Kind: roadmap.SessionKindRecovery}}
	if _, ok := alignDrafts(pending, recovery); ok {
		t.Fatal("a draft of another kind must not align")
	}
}

// slowAgent advances the clock while it "generates", like a real LLM call.
type slowAgent struct {
	draftAgent
	clock *fakeClock
	until time.Time
}

func (a *slowAgent) RegeneratePending(ctx context.Context, userID string, ids []string) ([]*roadmap.SessionPlanInfo, error) {
	a.clock.t = a.until
	return a.draftAgent.RegeneratePending(ctx, userID, ids)
}

func TestRegenerateSchedule_SessionStartedDuringGenerationIsKept(t *testing.T) {
	rm := regenFixture(t)
	repo := newMemRepo()
	if err := repo.Save(context.Background(), rm); err != nil {
		t.Fatal(err)
	}
	clock := &fakeClock{t: time.Date(2026, 11, 26, 17, 29, 0, 0, time.UTC)} // Thu, a minute before its session
	agent := &slowAgent{
		draftAgent: draftAgent{drafts: []*roadmap.SessionPlanInfo{ // slots moved to Friday
			{ScheduledDate: date(time.November, 27), SlotTime: "06:00-07:00", Reasoning: "fri"},
			{ScheduledDate: date(time.December, 4), SlotTime: "06:00-07:00", Reasoning: "next fri"},
		}},
		clock: clock,
		until: time.Date(2026, 11, 26, 17, 31, 0, 0, time.UTC), // Thursday's session has started
	}
	h := NewRegenerateScheduleHandler(fakeTx{}, repo, agent,
		guardrail.NewEngine(service.NewOverloadValidator(), nil, nil), zoneProfiles{loc: time.UTC}, &captureOutbox{}, clock, &incrIDs{})

	if _, err := h.Handle(context.Background(), RegenerateScheduleCommand{UserID: "u-1", Reason: "profile_updated"}); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	saved, _ := repo.FindActiveByUser(context.Background(), "u-1")
	if _, ok := saved.FindSession("thu"); !ok {
		t.Fatal("the session that started during generation must not be deleted")
	}
	if _, ok := saved.FindSession("mon"); ok {
		t.Fatal("the still-upcoming Monday session should be replaced")
	}
}

// Onboarding with an injury emits InjuryReported next to ProfileCompleted. The
// roadmap planned from that profile already holds the injury: regenerating it
// would cost a second generation for nothing. A later change still regenerates.
func TestRegenerateSchedule_SkipsChangesTheRoadmapAlreadyReflects(t *testing.T) {
	drafts := []*roadmap.SessionPlanInfo{
		{ScheduledDate: date(time.November, 26), SlotTime: "17:30-19:00", Reasoning: "new thu"},
		{ScheduledDate: date(time.November, 30), SlotTime: "17:30-19:00", Reasoning: "new mon"},
	}

	rm := regenFixture(t)
	plannedAt := rm.Info().CreatedAt
	h, _, outbox := newRegenHandler(t, rm, drafts, time.UTC)
	_, err := h.Handle(context.Background(), RegenerateScheduleCommand{
		UserID: "u-1", Reason: "injury_reported", ChangedAt: plannedAt.Add(-time.Second),
	})
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if len(outbox.events) != 0 {
		t.Fatalf("a change older than the roadmap regenerated it (%d events)", len(outbox.events))
	}

	rm = regenFixture(t)
	h, _, outbox = newRegenHandler(t, rm, drafts, time.UTC)
	_, err = h.Handle(context.Background(), RegenerateScheduleCommand{
		UserID: "u-1", Reason: "injury_reported", ChangedAt: plannedAt.Add(time.Second),
	})
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if len(outbox.events) == 0 {
		t.Fatal("a change made after the roadmap was planned must regenerate it")
	}
}
