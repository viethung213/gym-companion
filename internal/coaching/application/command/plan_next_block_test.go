package command

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	domainevent "github.com/viethung213/gym-companion/internal/coaching/domain/event"
	"github.com/viethung213/gym-companion/internal/coaching/domain/guardrail"
	"github.com/viethung213/gym-companion/internal/coaching/domain/roadmap"
	"github.com/viethung213/gym-companion/internal/coaching/domain/service"
)

// rotationRepo is an in-memory roadmap store with the database's rules: one
// ACTIVE and one PLANNED roadmap per user.
type rotationRepo struct {
	byID    map[string]*roadmap.Roadmap
	deleted []string
}

func newRotationRepo(rms ...*roadmap.Roadmap) *rotationRepo {
	r := &rotationRepo{byID: map[string]*roadmap.Roadmap{}}
	for _, rm := range rms {
		r.byID[rm.ID()] = rm
	}
	return r
}

func (r *rotationRepo) Save(_ context.Context, rm *roadmap.Roadmap) error {
	for id, other := range r.byID {
		if id == rm.ID() || other.UserID() != rm.UserID() || other.Status() != rm.Status() {
			continue
		}
		switch rm.Status() {
		case roadmap.StatusActive:
			return roadmap.ErrActiveRoadmapExists
		case roadmap.StatusPlanned:
			return roadmap.ErrPlannedRoadmapExists
		}
	}
	r.byID[rm.ID()] = rm
	return nil
}

func (r *rotationRepo) FindByID(_ context.Context, id string) (*roadmap.Roadmap, error) {
	if rm, ok := r.byID[id]; ok {
		return rm, nil
	}
	return nil, roadmap.ErrRoadmapNotFound
}

func (r *rotationRepo) byStatus(userID string, s roadmap.Status) (*roadmap.Roadmap, error) {
	for _, rm := range r.byID {
		if rm.UserID() == userID && rm.Status() == s {
			return rm, nil
		}
	}
	return nil, roadmap.ErrRoadmapNotFound
}

func (r *rotationRepo) FindActiveByUser(_ context.Context, userID string) (*roadmap.Roadmap, error) {
	return r.byStatus(userID, roadmap.StatusActive)
}

func (r *rotationRepo) FindPlannedByUser(_ context.Context, userID string) (*roadmap.Roadmap, error) {
	return r.byStatus(userID, roadmap.StatusPlanned)
}

func (r *rotationRepo) DeletePlannedRoadmap(_ context.Context, id string) error {
	delete(r.byID, id)
	r.deleted = append(r.deleted, id)
	return nil
}

func (r *rotationRepo) ListByUser(context.Context, string, []roadmap.Status, int, int) ([]*roadmap.Roadmap, error) {
	return nil, nil
}

func (r *rotationRepo) FindSessionByID(_ context.Context, sid string) (*roadmap.Roadmap, error) {
	for _, rm := range r.byID {
		if _, ok := rm.FindSession(sid); ok {
			return rm, nil
		}
	}
	return nil, roadmap.ErrSessionNotFound
}

func (r *rotationRepo) FindPendingSessionsStarting(context.Context, time.Time, time.Time) ([]port.ScheduledSession, error) {
	return nil, nil
}

func (r *rotationRepo) ActiveRoadmapsEndingBy(context.Context, time.Time) ([]port.RoadmapRef, error) {
	return nil, nil
}

func (r *rotationRepo) UsersWithRegenerationDue(context.Context) ([]string, error) { return nil, nil }

// blockStarting is a valid 4-week block (one session a week, Monday 18:00)
// for user u-1, in UTC.
func blockStarting(t *testing.T, id string, start time.Time) *roadmap.Roadmap {
	t.Helper()
	phases := []roadmap.Phase{roadmap.PhaseAccumulation, roadmap.PhaseOverload, roadmap.PhasePeak, roadmap.PhaseDeload}
	rpes := []float32{6.5, 7.5, 8.5, 5.5}
	sets := []int32{3, 3, 3, 2}

	weeks := make([]*roadmap.WeekPlan, 0, 4)
	for i := range 4 {
		ws := start.AddDate(0, 0, 7*i)
		wid := fmt.Sprintf("%s-w%d", id, i+1)
		did := wid + "-d"
		s, err := roadmap.NewSessionPlan(&roadmap.SessionPlanInfo{
			SessionPlanID: wid + "-s", DayPlanID: did, WeekPlanID: wid, RoadmapID: id, UserID: "u-1",
			ScheduledDate: ws, SlotTime: "18:00-19:00", EstimatedDurationMinutes: 60,
			Prescription: roadmap.WorkoutPrescription{MainExercises: []roadmap.PrescribedExercise{{
				ExerciseID: "ex-1", ExerciseName: "Squat", TargetSets: sets[i], TargetReps: 8,
				TargetWeight: 60, TargetRPE: rpes[i],
			}}},
		}, start)
		if err != nil {
			t.Fatal(err)
		}
		d, err := roadmap.RehydrateDayPlan(&roadmap.DayPlanInfo{
			DayPlanID: did, WeekPlanID: wid, RoadmapID: id, UserID: "u-1", ScheduledDate: ws,
		}, []*roadmap.SessionPlan{s})
		if err != nil {
			t.Fatal(err)
		}
		w, err := roadmap.RehydrateWeekPlan(&roadmap.WeekPlanInfo{
			WeekPlanID: wid, RoadmapID: id, UserID: "u-1", WeekNumber: int32(i + 1), Phase: phases[i],
			TargetRPE: rpes[i], StartDate: ws, EndDate: ws.AddDate(0, 0, 7),
		}, []*roadmap.DayPlan{d})
		if err != nil {
			t.Fatal(err)
		}
		weeks = append(weeks, w)
	}
	rm, err := roadmap.NewRoadmap(&roadmap.Info{
		RoadmapID: id, UserID: "u-1", StartDate: start, EndDate: start.AddDate(0, 0, 28),
	}, weeks, start)
	if err != nil {
		t.Fatal(err)
	}
	return rm
}

// stubPlanner drafts a block from the requested date and counts its calls.
type stubPlanner struct {
	t      *testing.T
	calls  int
	asked  time.Time
	offset int // days the draft starts after the requested date, to fake a wrong plan
}

func (p *stubPlanner) PlanRoadmapFrom(_ context.Context, _ string, start time.Time) (*roadmap.Roadmap, error) {
	p.calls++
	p.asked = start
	return blockStarting(p.t, fmt.Sprintf("next-%d", p.calls), start.AddDate(0, 0, p.offset)), nil
}

var _ port.NextBlockPlanner = (*stubPlanner)(nil)

func newPlanHandler(repo *rotationRepo, planner *stubPlanner, now time.Time) *PlanNextBlockHandler {
	return NewPlanNextBlockHandler(fakeTx{}, repo, repo, planner,
		guardrail.NewEngine(service.NewOverloadValidator(), nil, nil), zoneProfiles{loc: time.UTC}, &fakeClock{t: now})
}

// blockA runs 2026-11-02 to 2026-11-30; its final week starts 2026-11-23.
// The athlete trained its first session.
func blockA(t *testing.T) *roadmap.Roadmap {
	t.Helper()
	rm := untrainedBlockA(t)
	s, _ := rm.FindSession("a-w1-s")
	if err := s.MarkCompleted(100, 0, time.Date(2026, 11, 2, 19, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	return rm
}

// untrainedBlockA is blockA with no session completed.
func untrainedBlockA(t *testing.T) *roadmap.Roadmap {
	return blockStarting(t, "a", time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC))
}

// A block nobody trained in is not followed by another: no generation.
func TestPlanNextBlock_SkipsABlockNeverTrainedIn(t *testing.T) {
	repo := newRotationRepo(untrainedBlockA(t))
	planner := &stubPlanner{t: t}

	got, err := newPlanHandler(repo, planner, time.Date(2026, 11, 24, 0, 0, 0, 0, time.UTC)).Handle(context.Background(), "a")
	if err != nil || got != nil || planner.calls != 0 {
		t.Fatalf("got %v err %v calls %d; want nothing planned for an absent athlete", got, err, planner.calls)
	}
}

// A block already over is the roll-over's business, not the planner's.
func TestPlanNextBlock_SkipsAnEndedBlock(t *testing.T) {
	repo := newRotationRepo(blockA(t))
	planner := &stubPlanner{t: t}

	got, err := newPlanHandler(repo, planner, time.Date(2026, 11, 30, 1, 0, 0, 0, time.UTC)).Handle(context.Background(), "a")
	if err != nil || got != nil || planner.calls != 0 {
		t.Fatalf("got %v err %v calls %d; want nothing planned once the block ended", got, err, planner.calls)
	}
}

func TestPlanNextBlock_WaitsForTheFinalWeek(t *testing.T) {
	repo := newRotationRepo(blockA(t))
	planner := &stubPlanner{t: t}

	got, err := newPlanHandler(repo, planner, time.Date(2026, 11, 22, 23, 0, 0, 0, time.UTC)).Handle(context.Background(), "a")
	if err != nil || got != nil || planner.calls != 0 {
		t.Fatalf("before the final week: got %v err %v calls %d; want nothing planned", got, err, planner.calls)
	}
}

func TestPlanNextBlock_PlansFromTheDayTheBlockEnds(t *testing.T) {
	repo := newRotationRepo(blockA(t))
	planner := &stubPlanner{t: t}

	got, err := newPlanHandler(repo, planner, time.Date(2026, 11, 23, 1, 0, 0, 0, time.UTC)).Handle(context.Background(), "a")
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if want := time.Date(2026, 11, 30, 0, 0, 0, 0, time.UTC); !planner.asked.Equal(want) {
		t.Errorf("planned from %s, want %s", planner.asked.Format(time.DateOnly), want.Format(time.DateOnly))
	}
	if got == nil || got.Status() != roadmap.StatusPlanned {
		t.Fatalf("got %v, want a PLANNED block", got)
	}
	if active, _ := repo.FindActiveByUser(context.Background(), "u-1"); active.ID() != "a" {
		t.Errorf("active roadmap = %s, want the current block still active", active.ID())
	}
}

// A successor already planned is kept: no second generation.
func TestPlanNextBlock_KeepsAPlanThatFollows(t *testing.T) {
	next := blockStarting(t, "b", time.Date(2026, 11, 30, 0, 0, 0, 0, time.UTC))
	if err := next.HoldUntilStart(); err != nil {
		t.Fatal(err)
	}
	repo := newRotationRepo(blockA(t), next)
	planner := &stubPlanner{t: t}

	got, err := newPlanHandler(repo, planner, time.Date(2026, 11, 24, 0, 0, 0, 0, time.UTC)).Handle(context.Background(), "a")
	if err != nil || got != nil || planner.calls != 0 {
		t.Fatalf("got %v err %v calls %d; want nothing newly planned and no generation", got, err, planner.calls)
	}
	if kept, _ := repo.FindPlannedByUser(context.Background(), "u-1"); kept == nil || kept.ID() != "b" {
		t.Errorf("planned block = %v, want b kept", kept)
	}
}

// A planned block left over from another roadmap is replaced.
func TestPlanNextBlock_ReplacesAStalePlan(t *testing.T) {
	stale := blockStarting(t, "old", time.Date(2026, 12, 14, 0, 0, 0, 0, time.UTC))
	if err := stale.HoldUntilStart(); err != nil {
		t.Fatal(err)
	}
	repo := newRotationRepo(blockA(t), stale)
	planner := &stubPlanner{t: t}

	got, err := newPlanHandler(repo, planner, time.Date(2026, 11, 24, 0, 0, 0, 0, time.UTC)).Handle(context.Background(), "a")
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if len(repo.deleted) != 1 || repo.deleted[0] != "old" {
		t.Errorf("deleted %v, want the stale plan", repo.deleted)
	}
	if got == nil || planner.calls != 1 {
		t.Errorf("got %v calls %d, want a fresh plan", got, planner.calls)
	}
}

// A draft that does not start the day the block ends is refused, not stored.
func TestPlanNextBlock_RefusesADraftThatDoesNotFollow(t *testing.T) {
	repo := newRotationRepo(blockA(t))
	planner := &stubPlanner{t: t, offset: 1}

	if _, err := newPlanHandler(repo, planner, time.Date(2026, 11, 24, 0, 0, 0, 0, time.UTC)).Handle(context.Background(), "a"); err == nil {
		t.Fatal("want an error for a draft starting a day late")
	}
	if _, err := repo.FindPlannedByUser(context.Background(), "u-1"); !errors.Is(err, roadmap.ErrRoadmapNotFound) {
		t.Errorf("a misdated draft was stored: %v", err)
	}
}

func newRollOver(repo *rotationRepo, outbox *captureOutbox, now time.Time) *RollOverHandler {
	return NewRollOverHandler(fakeTx{}, repo, repo, outbox, &fakeClock{t: now})
}

func TestRollOver_KeepsABlockThatHasNotEnded(t *testing.T) {
	repo := newRotationRepo(blockA(t))
	res, err := newRollOver(repo, &captureOutbox{}, time.Date(2026, 11, 29, 23, 0, 0, 0, time.UTC)).Handle(context.Background(), "u-1")
	if err != nil || res.RolledOver || res.Active == nil || res.Active.ID() != "a" || res.Active.Status() != roadmap.StatusActive {
		t.Fatalf("got %+v err %v; want block a still active, nothing rolled over", res, err)
	}
}

// At the end the old block completes (its missed sessions skipped, its weeks
// closed) and the planned one takes over with no wait.
func TestRollOver_ActivatesTheNextBlock(t *testing.T) {
	current := blockA(t)
	next := blockStarting(t, "b", time.Date(2026, 11, 30, 0, 0, 0, 0, time.UTC))
	if err := next.HoldUntilStart(); err != nil {
		t.Fatal(err)
	}
	repo := newRotationRepo(current, next)
	outbox := &captureOutbox{}

	res, err := newRollOver(repo, outbox, time.Date(2026, 11, 30, 0, 30, 0, 0, time.UTC)).Handle(context.Background(), "u-1")
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if !res.RolledOver {
		t.Error("RolledOver = false, want true")
	}
	got := res.Active
	if got == nil || got.ID() != "b" || got.Status() != roadmap.StatusActive {
		t.Fatalf("got %v, want block b active", got)
	}
	if current.Status() != roadmap.StatusCompleted {
		t.Errorf("old block status %s, want COMPLETED", current.Status())
	}
	if s, _ := current.FindSession("a-w4-s"); s.Status() != roadmap.SessionPlanStatusSkipped {
		t.Errorf("untrained final-week session = %s, want SKIPPED", s.Status())
	}
	for _, w := range current.Weeks() {
		if !w.Reviewed() {
			t.Errorf("week %d of the old block left open", w.WeekNumber())
		}
	}
	if len(outbox.events) != 1 {
		t.Fatalf("events = %d, want one RoadmapInitiated", len(outbox.events))
	}
	if e, ok := outbox.events[0].(*domainevent.RoadmapInitiated); !ok || e.RoadmapID != "b" {
		t.Errorf("event = %+v, want RoadmapInitiated for b", outbox.events[0])
	}
}

// Without a planned block the old one still completes, so the athlete can
// start a new roadmap (the app offers it when none is active).
func TestRollOver_WithoutAPlanCompletesTheBlock(t *testing.T) {
	current := blockA(t)
	repo := newRotationRepo(current)

	res, err := newRollOver(repo, &captureOutbox{}, time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)).Handle(context.Background(), "u-1")
	if err != nil || !res.RolledOver || res.Active != nil {
		t.Fatalf("got %+v err %v; want rolled over with no active roadmap", res, err)
	}
	if current.Status() != roadmap.StatusCompleted {
		t.Errorf("old block status %s, want COMPLETED", current.Status())
	}
}

// A new roadmap is started without asking only for an athlete who trained in
// the block that just ended; a block long over or never trained in is not
// followed by a generation (the first run after a deploy finds many such).
func TestRollOver_StartFreshOnlyForARecentTrainedBlock(t *testing.T) {
	justEnded := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name  string
		block func(*testing.T) *roadmap.Roadmap
		now   time.Time
		want  bool
	}{
		{"trained, just ended", blockA, justEnded, true},
		{"never trained in", untrainedBlockA, justEnded, false},
		{"ended months ago", blockA, time.Date(2027, 3, 1, 0, 0, 0, 0, time.UTC), false},
	}
	for _, tt := range tests {
		res, err := newRollOver(newRotationRepo(tt.block(t)), &captureOutbox{}, tt.now).Handle(context.Background(), "u-1")
		if err != nil || !res.RolledOver || res.Active != nil || res.StartFresh != tt.want {
			t.Errorf("%s: got %+v err %v; want rolled over, StartFresh %v", tt.name, res, err, tt.want)
		}
	}

	next := blockStarting(t, "b", time.Date(2026, 11, 30, 0, 0, 0, 0, time.UTC))
	if err := next.HoldUntilStart(); err != nil {
		t.Fatal(err)
	}
	res, err := newRollOver(newRotationRepo(blockA(t), next), &captureOutbox{}, justEnded).Handle(context.Background(), "u-1")
	if err != nil || res.StartFresh {
		t.Errorf("with a planned successor: got %+v err %v; want no fresh start", res, err)
	}
}

// Seen before the fix: once the block ended, starting a free workout failed
// ("date is outside the roadmap") and the app's fallback could not create a
// roadmap because the ended one was still ACTIVE.
func TestCreateAdhoc_AfterTheBlockEndedLandsInTheNextOne(t *testing.T) {
	next := blockStarting(t, "b", time.Date(2026, 11, 30, 0, 0, 0, 0, time.UTC))
	if err := next.HoldUntilStart(); err != nil {
		t.Fatal(err)
	}
	repo := newRotationRepo(blockA(t), next)
	now := time.Date(2026, 11, 30, 0, 10, 0, 0, time.UTC) // before the worker rolled over
	n := 0
	h := NewCreateAdhocSessionHandler(fakeTx{}, repo, benchOnly(), zoneProfiles{loc: time.UTC},
		&idGeneratorMock{fn: func() string { n++; return fmt.Sprintf("id-%d", n) }}, &clockMock{now: now}).
		WithRollover(newRollOver(repo, &captureOutbox{}, now))

	res, err := h.Handle(context.Background(), CreateAdhocSessionCommand{UserID: "u-1", ExerciseIDs: []string{"ex-1"}})
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if got := res.SessionPlan.Info().RoadmapID; got != "b" {
		t.Errorf("session placed in roadmap %s, want the next block b", got)
	}
}

// freshStarter stands in for InitiateRoadmapHandler: it activates roadmap rm.
type freshStarter struct {
	repo  *rotationRepo
	rm    *roadmap.Roadmap
	calls int
}

func (f *freshStarter) Handle(context.Context, InitiateRoadmapCommand) (*InitiateRoadmapResult, error) {
	f.calls++
	f.repo.byID[f.rm.ID()] = f.rm
	return &InitiateRoadmapResult{Roadmap: f.rm}, nil
}

// Seen in review: an ad-hoc session asked for after a trained block ended with
// no successor rolled it over, but the fresh roadmap StartFresh asks for was
// never started, and the worker no longer saw the block. It is started here.
func TestCreateAdhoc_AfterTheBlockEndedStartsAFreshRoadmap(t *testing.T) {
	repo := newRotationRepo(blockA(t))
	now := time.Date(2026, 11, 30, 0, 10, 0, 0, time.UTC)
	starter := &freshStarter{repo: repo, rm: blockStarting(t, "c", time.Date(2026, 11, 30, 0, 0, 0, 0, time.UTC))}
	n := 0
	h := NewCreateAdhocSessionHandler(fakeTx{}, repo, benchOnly(), zoneProfiles{loc: time.UTC},
		&idGeneratorMock{fn: func() string { n++; return fmt.Sprintf("id-%d", n) }}, &clockMock{now: now}).
		WithRollover(newRollOver(repo, &captureOutbox{}, now)).
		WithFreshStart(starter)

	res, err := h.Handle(context.Background(), CreateAdhocSessionCommand{UserID: "u-1", ExerciseIDs: []string{"ex-1"}})
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if starter.calls != 1 {
		t.Errorf("fresh starts = %d, want 1", starter.calls)
	}
	if got := res.SessionPlan.Info().RoadmapID; got != "c" {
		t.Errorf("session placed in roadmap %s, want the fresh roadmap c", got)
	}
}

// Seen in review: "today" came from the profile's zone while the roll-over
// decides by the roadmap's. With the profile 7h ahead of a roadmap stored in
// UTC, 21:00 UTC the day before the end was "after the end" for the session
// but not for the roll-over, and the session was refused for 7 hours. Both now
// read the roadmap's zone: the block has not ended, the session lands in it.
func TestCreateAdhoc_ZonesAgreeOnTheBlockEnd(t *testing.T) {
	hcm, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		t.Skip("tzdata unavailable")
	}
	next := blockStarting(t, "b", time.Date(2026, 11, 30, 0, 0, 0, 0, time.UTC))
	if err := next.HoldUntilStart(); err != nil {
		t.Fatal(err)
	}
	repo := newRotationRepo(blockA(t), next) // stored in UTC
	now := time.Date(2026, 11, 29, 21, 0, 0, 0, time.UTC)
	n := 0
	h := NewCreateAdhocSessionHandler(fakeTx{}, repo, benchOnly(), zoneProfiles{loc: hcm},
		&idGeneratorMock{fn: func() string { n++; return fmt.Sprintf("id-%d", n) }}, &clockMock{now: now}).
		WithRollover(newRollOver(repo, &captureOutbox{}, now))

	res, err := h.Handle(context.Background(), CreateAdhocSessionCommand{UserID: "u-1", ExerciseIDs: []string{"ex-1"}})
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if got := res.SessionPlan.Info().RoadmapID; got != "a" {
		t.Errorf("session placed in roadmap %s, want the current block a", got)
	}
}

// A profile change after the next block was planned discards it, so it is
// planned again from the current profile; an older change keeps it. A change
// whose time is unknown (zero ChangedAt) is taken as newer than the plan.
func TestRegenerate_DiscardsAPlanDraftedBeforeTheProfileChange(t *testing.T) {
	for _, tt := range []struct {
		name        string
		changedBy   time.Duration // relative to when the plan was drafted
		unknownTime bool
		discard     bool
	}{
		{"change after the plan", time.Hour, false, true},
		{"change before the plan", -time.Hour, false, false},
		{"change at an unknown time", 0, true, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			next := blockStarting(t, "b", time.Date(2026, 11, 30, 0, 0, 0, 0, time.UTC))
			if err := next.HoldUntilStart(); err != nil {
				t.Fatal(err)
			}
			repo := newRotationRepo(blockA(t), next)
			now := time.Date(2026, 11, 24, 0, 0, 0, 0, time.UTC)
			changedAt := next.Info().CreatedAt.Add(tt.changedBy)
			if tt.unknownTime {
				now = next.Info().CreatedAt.Add(time.Hour) // a plan is always drafted before now
				changedAt = time.Time{}
			}
			h := NewRegenerateScheduleHandler(fakeTx{}, repo, &draftAgent{},
				guardrail.NewEngine(service.NewOverloadValidator(), nil, nil), zoneProfiles{loc: time.UTC},
				&captureOutbox{}, &fakeClock{t: now}, &incrIDs{}).
				WithPlannedBlocks(repo)

			_, _ = h.Handle(context.Background(), RegenerateScheduleCommand{
				UserID: "u-1", Reason: "injury_reported", ChangedAt: changedAt,
			})
			_, err := repo.FindPlannedByUser(context.Background(), "u-1")
			if discarded := errors.Is(err, roadmap.ErrRoadmapNotFound); discarded != tt.discard {
				t.Errorf("plan discarded = %v, want %v", discarded, tt.discard)
			}
		})
	}
}

// editedProfiles reports a profile last edited at updatedAt.
type editedProfiles struct{ updatedAt time.Time }

func (p editedProfiles) GetProfile(_ context.Context, userID string) (port.Profile, error) {
	return port.Profile{UserID: userID, UpdatedAt: p.updatedAt}, nil
}

// An edit made while the block was being generated was missed by the
// profile-change event (no planned block existed yet): the draft is dropped
// and planned again next run, instead of shipping the old slots or injuries.
func TestPlanNextBlock_DropsADraftFromAProfileEditedMeanwhile(t *testing.T) {
	now := time.Date(2026, 11, 24, 10, 0, 0, 0, time.UTC)
	repo := newRotationRepo(blockA(t))
	h := NewPlanNextBlockHandler(fakeTx{}, repo, repo, &stubPlanner{t: t},
		guardrail.NewEngine(service.NewOverloadValidator(), nil, nil),
		editedProfiles{updatedAt: now.Add(time.Minute)}, &fakeClock{t: now})

	if _, err := h.Handle(context.Background(), "a"); !errors.Is(err, errProfileChangedWhilePlanning) {
		t.Fatalf("err = %v, want errProfileChangedWhilePlanning", err)
	}
	if _, err := repo.FindPlannedByUser(context.Background(), "u-1"); !errors.Is(err, roadmap.ErrRoadmapNotFound) {
		t.Errorf("a draft from the stale profile was stored: %v", err)
	}
}

// Reproduces the review finding: an injury stamped 100 ms before planning
// started (10:00:00.900) commits after the planner read the profile. The RPC
// rounds updated_at down to 10:00:00, the old check saw "before 10:00:01" and
// kept a block planned without the injury, and the profile event found no
// planned block to discard. A change that may have missed the read re-plans.
func TestPlanNextBlock_DropsADraftWhenTheEditMayHaveMissedTheRead(t *testing.T) {
	plannedAt := time.Date(2026, 11, 24, 10, 0, 1, 0, time.UTC)
	repo := newRotationRepo(blockA(t))
	h := NewPlanNextBlockHandler(fakeTx{}, repo, repo, &stubPlanner{t: t},
		guardrail.NewEngine(service.NewOverloadValidator(), nil, nil),
		editedProfiles{updatedAt: time.Date(2026, 11, 24, 10, 0, 0, 0, time.UTC)}, &fakeClock{t: plannedAt})

	if _, err := h.Handle(context.Background(), "a"); !errors.Is(err, errProfileChangedWhilePlanning) {
		t.Fatalf("err = %v, want errProfileChangedWhilePlanning", err)
	}

	// An edit well before planning keeps the draft.
	repo = newRotationRepo(blockA(t))
	h = NewPlanNextBlockHandler(fakeTx{}, repo, repo, &stubPlanner{t: t},
		guardrail.NewEngine(service.NewOverloadValidator(), nil, nil),
		editedProfiles{updatedAt: plannedAt.Add(-time.Hour)}, &fakeClock{t: plannedAt})
	if _, err := h.Handle(context.Background(), "a"); err != nil {
		t.Fatalf("an old edit dropped the draft: %v", err)
	}
}
