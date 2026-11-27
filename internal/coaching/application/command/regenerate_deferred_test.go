package command

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/domain/guardrail"
	"github.com/viethung213/gym-companion/internal/coaching/domain/roadmap"
	"github.com/viethung213/gym-companion/internal/coaching/domain/service"
)

// flakyAgent fails while down, like a model answering 503, then returns drafts.
type flakyAgent struct {
	draftAgent
	down  bool
	calls int
}

func (a *flakyAgent) RegeneratePending(ctx context.Context, userID string, ids []string) ([]*roadmap.SessionPlanInfo, error) {
	a.calls++
	if a.down {
		return nil, errors.New("Error 503: model overloaded")
	}
	return a.draftAgent.RegeneratePending(ctx, userID, ids)
}

func newFlakyRegen(t *testing.T, clock *fakeClock) (*RegenerateScheduleHandler, *flakyAgent, *memRepo, *captureOutbox) {
	t.Helper()
	repo := newMemRepo()
	if err := repo.Save(context.Background(), regenFixture(t)); err != nil {
		t.Fatal(err)
	}
	agent := &flakyAgent{down: true, draftAgent: draftAgent{drafts: []*roadmap.SessionPlanInfo{
		{ScheduledDate: date(time.November, 26), SlotTime: "17:30-19:00", Reasoning: "new thu"},
		{ScheduledDate: date(time.November, 30), SlotTime: "17:30-19:00", Reasoning: "new mon"},
	}}}
	outbox := &captureOutbox{}
	h := NewRegenerateScheduleHandler(fakeTx{}, repo, agent,
		guardrail.NewEngine(service.NewOverloadValidator(), nil, nil), zoneProfiles{loc: time.UTC}, outbox, clock, &incrIDs{})
	return h, agent, repo, outbox
}

// Reproduces the known bug: an injury reported while the model is down (503)
// was retried three times in a few hundred ms and dropped, so it never reached
// the schedule. It is now kept on the roadmap and applied by a later retry.
func TestRegenerate_ModelDownKeepsTheChangeForALaterRetry(t *testing.T) {
	clock := &fakeClock{t: time.Date(2026, 11, 24, 2, 0, 0, 0, time.UTC)}
	h, agent, repo, outbox := newFlakyRegen(t, clock)
	changedAt := clock.t.Add(-time.Minute)

	res, err := h.Handle(context.Background(), RegenerateScheduleCommand{
		UserID: "u-1", Reason: "injury_reported", ChangedAt: changedAt,
	})
	if err != nil || !res.Deferred {
		t.Fatalf("got %+v, %v; want Deferred and no error (the event is consumed)", res, err)
	}
	// Reproduces the review finding: a deferred call returned no roadmap, and
	// the RPC answered OK with an empty one. The unchanged roadmap is returned.
	if res.Roadmap == nil || res.Roadmap.ID() != "rm-1" {
		t.Errorf("deferred result roadmap = %v, want the current roadmap", res.Roadmap)
	}
	rm, _ := repo.FindActiveByUser(context.Background(), "u-1")
	due, err := rm.PendingRegeneration()
	if err != nil || !due.ChangedAt.Equal(changedAt) || due.Reason != "injury_reported" {
		t.Fatalf("due = %+v err %v, want the change kept", due, err)
	}
	if len(outbox.events) != 0 {
		t.Errorf("events = %d, want none before the schedule changed", len(outbox.events))
	}

	// The model is back: the worker's retry applies the change in place.
	agent.down = false
	clock.t = clock.t.Add(2 * time.Hour)
	if _, err := h.RetryDue(context.Background(), "u-1"); err != nil {
		t.Fatalf("RetryDue: %v", err)
	}
	rm, _ = repo.FindActiveByUser(context.Background(), "u-1")
	if mon, ok := rm.FindSession("mon"); !ok || mon.Info().Reasoning != "new mon" {
		t.Error("the retry did not rewrite Monday in place")
	}
	if _, err := rm.PendingRegeneration(); !errors.Is(err, roadmap.ErrNoRegenerationDue) {
		t.Errorf("after the retry: %v, want the change cleared", err)
	}
	if len(outbox.events) != 1 {
		t.Errorf("events = %d, want one RoadmapAdjusted", len(outbox.events))
	}
}

// A change waiting past the give-up age is dropped without a generation.
func TestRegenerate_RetryDueDropsAnExpiredChange(t *testing.T) {
	clock := &fakeClock{t: time.Date(2026, 11, 24, 2, 0, 0, 0, time.UTC)}
	h, agent, repo, _ := newFlakyRegen(t, clock)
	if _, err := h.Handle(context.Background(), RegenerateScheduleCommand{
		UserID: "u-1", Reason: "profile_updated", ChangedAt: clock.t,
	}); err != nil {
		t.Fatal(err)
	}
	agent.down, agent.calls = false, 0
	clock.t = clock.t.Add(roadmap.RegenerationGiveUpAfter + time.Hour)

	if _, err := h.RetryDue(context.Background(), "u-1"); err != nil {
		t.Fatalf("RetryDue: %v", err)
	}
	if agent.calls != 0 {
		t.Errorf("model calls = %d, want none for an expired change", agent.calls)
	}
	rm, _ := repo.FindActiveByUser(context.Background(), "u-1")
	if _, err := rm.PendingRegeneration(); !errors.Is(err, roadmap.ErrNoRegenerationDue) {
		t.Errorf("expired change still waiting: %v", err)
	}
}

// Without a waiting change RetryDue does nothing.
func TestRegenerate_RetryDueWithoutAChangeIsANoOp(t *testing.T) {
	clock := &fakeClock{t: time.Date(2026, 11, 24, 2, 0, 0, 0, time.UTC)}
	h, agent, _, outbox := newFlakyRegen(t, clock)
	agent.down = false
	if _, err := h.RetryDue(context.Background(), "u-1"); err != nil {
		t.Fatalf("RetryDue: %v", err)
	}
	if agent.calls != 0 || len(outbox.events) != 0 {
		t.Errorf("calls %d events %d, want nothing done", agent.calls, len(outbox.events))
	}
}

// midwayAgent records a new profile change on the stored roadmap while it
// generates, like an injury deferred by the consumer during a worker retry.
type midwayAgent struct {
	draftAgent
	repo      *memRepo
	changedAt time.Time
}

func (a *midwayAgent) RegeneratePending(ctx context.Context, userID string, ids []string) ([]*roadmap.SessionPlanInfo, error) {
	rm, _ := a.repo.FindActiveByUser(ctx, userID)
	rm.MarkRegenerationDue(a.changedAt, "injury_reported")
	return a.draftAgent.RegeneratePending(ctx, userID, ids)
}

// Reproduces the review finding: a change recorded while a retry generated
// from the older profile was cleared by that retry's success, so the new
// injury never reached the schedule. It stays waiting for the next retry.
func TestRegenerate_RetryKeepsAChangeRecordedWhileItGenerated(t *testing.T) {
	clock := &fakeClock{t: time.Date(2026, 11, 24, 2, 0, 0, 0, time.UTC)}
	h, _, repo, _ := newFlakyRegen(t, clock)
	if _, err := h.Handle(context.Background(), RegenerateScheduleCommand{
		UserID: "u-1", Reason: "profile_updated", ChangedAt: clock.t.Add(-time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	injuredAt := clock.t.Add(time.Minute) // after the retry starts generating
	h.agent = &midwayAgent{draftAgent: draftAgent{drafts: []*roadmap.SessionPlanInfo{
		{ScheduledDate: date(time.November, 26), SlotTime: "17:30-19:00", Reasoning: "new thu"},
		{ScheduledDate: date(time.November, 30), SlotTime: "17:30-19:00", Reasoning: "new mon"},
	}}, repo: repo, changedAt: injuredAt}

	if _, err := h.RetryDue(context.Background(), "u-1"); err != nil {
		t.Fatalf("RetryDue: %v", err)
	}
	rm, _ := repo.FindActiveByUser(context.Background(), "u-1")
	due, err := rm.PendingRegeneration()
	if err != nil || !due.ChangedAt.Equal(injuredAt) || due.Reason != "injury_reported" {
		t.Errorf("due = %+v err %v, want the injury recorded mid-generation kept", due, err)
	}
}
