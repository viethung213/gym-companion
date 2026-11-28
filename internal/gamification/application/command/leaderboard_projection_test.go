package command_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/gamification/application/command"
	"github.com/viethung213/gym-companion/internal/gamification/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/gamification/domain/entity"
	"github.com/viethung213/gym-companion/internal/gamification/domain/event"
)

// fakeProjection records every call in order so tests can assert sequencing.
type fakeProjection struct {
	calls     []string
	upserts   [][]entity.LeaderboardScore
	lockFree  bool
	upsertErr error
	extendErr error
	readyErr  error
	failAfter int // fail UpsertScores once this many upserts succeeded (0 = never)
}

func newFakeProjection() *fakeProjection { return &fakeProjection{lockFree: true} }

func (f *fakeProjection) UpsertScores(_ context.Context, scores []entity.LeaderboardScore) error {
	f.calls = append(f.calls, "upsert")
	if f.upsertErr != nil {
		return f.upsertErr
	}
	if f.failAfter > 0 && len(f.upserts) >= f.failAfter {
		return errors.New("redis connection lost")
	}
	f.upserts = append(f.upserts, scores)
	return nil
}

func (f *fakeProjection) Reset(context.Context) error {
	f.calls = append(f.calls, "reset")
	return nil
}

func (f *fakeProjection) MarkReady(context.Context) error {
	f.calls = append(f.calls, "ready")
	return f.readyErr
}

func (f *fakeProjection) BeginRebuild(context.Context) error {
	f.calls = append(f.calls, "begin")
	return nil
}

func (f *fakeProjection) ExtendRebuildLock(context.Context) error {
	f.calls = append(f.calls, "extend")
	return f.extendErr
}

func (f *fakeProjection) TryAcquireRebuildLock(context.Context) (bool, error) {
	f.calls = append(f.calls, "lock")
	return f.lockFree, nil
}

func (f *fakeProjection) ReleaseRebuildLock(context.Context) error {
	f.calls = append(f.calls, "unlock")
	return nil
}

func (f *fakeProjection) count(call string) int {
	n := 0
	for _, c := range f.calls {
		if c == call {
			n++
		}
	}
	return n
}

type fakeScoreSource struct {
	ids     []string // sorted ascending
	cursors []string
}

func (f *fakeScoreSource) ListScores(_ context.Context, afterUserID string, limit int) ([]entity.LeaderboardScore, error) {
	f.cursors = append(f.cursors, afterUserID)
	var page []entity.LeaderboardScore
	for _, id := range f.ids {
		if id > afterUserID && len(page) < limit {
			page = append(page, entity.LeaderboardScore{UserID: id, Xp: 10})
		}
	}
	return page, nil
}

func (f *fakeScoreSource) GetScore(context.Context, string) (*entity.LeaderboardScore, error) {
	return nil, nil
}

func TestProjectLeaderboardScore_Valid(t *testing.T) {
	projection := newFakeProjection()
	handler := command.NewProjectLeaderboardScoreHandler(projection)
	at := time.Date(2026, 11, 27, 9, 0, 0, 0, time.UTC)

	err := handler.Handle(context.Background(), command.ProjectLeaderboardScoreCommand{UserID: "u-1", Xp: 120, UpdatedAt: at})
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if len(projection.upserts) != 1 || len(projection.upserts[0]) != 1 {
		t.Fatalf("upserts = %+v, want one score", projection.upserts)
	}
	if got := projection.upserts[0][0]; got.UserID != "u-1" || got.Xp != 120 || !got.UpdatedAt.Equal(at) {
		t.Errorf("projected score = %+v", got)
	}
}

func TestProjectLeaderboardScore_RejectsInvalid(t *testing.T) {
	tests := []struct {
		name string
		give command.ProjectLeaderboardScoreCommand
	}{
		{name: "empty user", give: command.ProjectLeaderboardScoreCommand{Xp: 10, UpdatedAt: time.Now()}},
		{name: "negative xp", give: command.ProjectLeaderboardScoreCommand{UserID: "u-1", Xp: -1, UpdatedAt: time.Now()}},
		{name: "missing time", give: command.ProjectLeaderboardScoreCommand{UserID: "u-1", Xp: 10}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projection := newFakeProjection()
			handler := command.NewProjectLeaderboardScoreHandler(projection)
			if err := handler.Handle(context.Background(), tt.give); err == nil {
				t.Fatal("Handle err = nil, want validation error")
			}
			if len(projection.calls) != 0 {
				t.Errorf("projection called for invalid command: %v", projection.calls)
			}
		})
	}
}

func TestProjectBestEffort_NilHandlerNoPanic(t *testing.T) {
	var handler *command.ProjectLeaderboardScoreHandler
	handler.ProjectBestEffort(context.Background(), command.ProjectLeaderboardScoreCommand{UserID: "u-1", Xp: 1, UpdatedAt: time.Now()})
}

func TestProjectBestEffort_SwallowsErrorAndContinues(t *testing.T) {
	projection := newFakeProjection()
	projection.upsertErr = errors.New("redis down")
	handler := command.NewProjectLeaderboardScoreHandler(projection)

	handler.ProjectBestEffort(context.Background(),
		command.ProjectLeaderboardScoreCommand{UserID: "u-1", Xp: 1, UpdatedAt: time.Now()},
		command.ProjectLeaderboardScoreCommand{UserID: "u-2", Xp: 2, UpdatedAt: time.Now()},
	)
	if projection.count("upsert") != 2 {
		t.Fatalf("upsert attempts = %d, want 2 (error must not stop later projections)", projection.count("upsert"))
	}
}

func TestXpEarnedProjections_PicksOnlyXpEarned(t *testing.T) {
	at := time.Date(2026, 11, 27, 9, 0, 0, 0, time.UTC)
	events := []event.Event{
		&event.WorkoutActivityIngested{UserID: "u-1"},
		&event.XpEarned{UserID: "u-1", DeltaXp: 50, NewXp: 150, EarnedAt: at},
		&event.UserLeveledUp{UserID: "u-1", NewLevel: 2},
		event.XpEarned{UserID: "u-2", DeltaXp: 25, NewXp: 25, EarnedAt: at.Add(time.Minute)},
		(*event.XpEarned)(nil),
	}

	got := command.XpEarnedProjections(events)
	if len(got) != 2 {
		t.Fatalf("projections = %+v, want 2", got)
	}
	if got[0].UserID != "u-1" || got[0].Xp != 150 || !got[0].UpdatedAt.Equal(at) {
		t.Errorf("first projection = %+v, want u-1 NewXp 150", got[0])
	}
	if got[1].UserID != "u-2" || got[1].Xp != 25 || !got[1].UpdatedAt.Equal(at.Add(time.Minute)) {
		t.Errorf("second projection = %+v, want u-2 NewXp 25", got[1])
	}
}

func userIDs(n int) []string {
	ids := make([]string, n)
	for i := range ids {
		ids[i] = fmt.Sprintf("u-%02d", i+1)
	}
	return ids
}

func TestRebuild_PagesAllAndMarksReady(t *testing.T) {
	projection := newFakeProjection()
	source := &fakeScoreSource{ids: userIDs(5)}
	handler := command.NewRebuildLeaderboardHandler(source, projection, 2)

	res, err := handler.Handle(context.Background(), command.RebuildLeaderboardCommand{})
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if res.Skipped || res.Projected != 5 {
		t.Errorf("result = %+v, want Projected 5", res)
	}
	wantCursors := []string{"", "u-02", "u-04"}
	if fmt.Sprint(source.cursors) != fmt.Sprint(wantCursors) {
		t.Errorf("cursors = %v, want %v", source.cursors, wantCursors)
	}
	wantCalls := []string{
		"lock", "begin", "upsert", "extend", "upsert", "extend", "upsert", "extend", "ready", "unlock",
	}
	if fmt.Sprint(projection.calls) != fmt.Sprint(wantCalls) {
		t.Errorf("calls = %v, want %v", projection.calls, wantCalls)
	}
}

func TestRebuild_ExactMultipleOfBatch(t *testing.T) {
	projection := newFakeProjection()
	source := &fakeScoreSource{ids: userIDs(4)}

	res, err := command.NewRebuildLeaderboardHandler(source, projection, 2).Handle(context.Background(), command.RebuildLeaderboardCommand{})
	if err != nil || res.Projected != 4 || projection.count("ready") != 1 {
		t.Fatalf("result = %+v, %v, calls %v", res, err, projection.calls)
	}
}

func TestRebuild_EmptySource_StillMarksReady(t *testing.T) {
	projection := newFakeProjection()

	res, err := command.NewRebuildLeaderboardHandler(&fakeScoreSource{}, projection, 2).Handle(context.Background(), command.RebuildLeaderboardCommand{})
	if err != nil || res.Projected != 0 || projection.count("ready") != 1 {
		t.Fatalf("result = %+v, %v, calls %v; want ready with 0 users", res, err, projection.calls)
	}
}

func TestRebuild_LockNotAcquired_Skipped(t *testing.T) {
	projection := newFakeProjection()
	projection.lockFree = false

	res, err := command.NewRebuildLeaderboardHandler(&fakeScoreSource{ids: userIDs(3)}, projection, 2).
		Handle(context.Background(), command.RebuildLeaderboardCommand{Reset: true})
	if err != nil || !res.Skipped {
		t.Fatalf("result = %+v, %v; want Skipped", res, err)
	}
	if fmt.Sprint(projection.calls) != "[lock]" {
		t.Errorf("calls = %v, want only [lock]", projection.calls)
	}
}

func TestRebuild_ResetCalledBeforeListing(t *testing.T) {
	projection := newFakeProjection()

	_, err := command.NewRebuildLeaderboardHandler(&fakeScoreSource{ids: userIDs(1)}, projection, 2).
		Handle(context.Background(), command.RebuildLeaderboardCommand{Reset: true})
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	want := []string{"lock", "reset", "begin", "upsert", "extend", "ready", "unlock"}
	if fmt.Sprint(projection.calls) != fmt.Sprint(want) {
		t.Errorf("calls = %v, want %v", projection.calls, want)
	}
}

func TestRebuild_ErrorMidway_NoMarkReady_LockReleased(t *testing.T) {
	projection := newFakeProjection()
	projection.failAfter = 1

	_, err := command.NewRebuildLeaderboardHandler(&fakeScoreSource{ids: userIDs(5)}, projection, 2).
		Handle(context.Background(), command.RebuildLeaderboardCommand{})
	if err == nil {
		t.Fatal("Handle err = nil, want upsert failure")
	}
	if projection.count("ready") != 0 {
		t.Error("MarkReady called after a failed rebuild")
	}
	if projection.count("unlock") != 1 {
		t.Errorf("unlock calls = %d, want 1", projection.count("unlock"))
	}
}

// failingCommitTx runs the transaction body, then fails the way a COMMIT would.
type failingCommitTx struct{}

func (failingCommitTx) WithTransaction(ctx context.Context, fn func(context.Context) error) error {
	if err := fn(ctx); err != nil {
		return err
	}
	return errors.New("commit failed")
}

func TestInitUser_ProjectsCurrentXpAfterCommit(t *testing.T) {
	at := time.Date(2026, 11, 20, 8, 0, 0, 0, time.UTC)
	tests := []struct {
		name   string
		seed   *aggregate.UserXp
		wantXp int64
	}{
		{name: "no row yet", wantXp: 0},
		{
			// The PostgreSQL repository seeds this baseline row inside FindByIDForUpdate.
			name:   "baseline row seeded by the repository",
			seed:   aggregate.ReconstituteUserXp("u-1", 0, 1, nil, at),
			wantXp: 0,
		},
		{
			name:   "replayed registration of a user with progress",
			seed:   aggregate.ReconstituteUserXp("u-1", 900, 5, nil, at),
			wantXp: 900,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			xpRepo := newMockUserXpRepo()
			if tt.seed != nil {
				xpRepo.users["u-1"] = tt.seed
			}
			projection := newFakeProjection()
			handler := command.NewInitUserGamificationHandler(
				xpRepo, newMockUserStreakRepo(), newMockSnapshotRepo(), &mockTxManager{},
			).WithLeaderboardProjector(command.NewProjectLeaderboardScoreHandler(projection))

			if err := handler.Handle(context.Background(), command.InitUserGamificationCommand{UserID: "u-1"}); err != nil {
				t.Fatalf("Handle: %v", err)
			}
			if len(projection.upserts) != 1 || len(projection.upserts[0]) != 1 {
				t.Fatalf("upserts = %+v, want one score", projection.upserts)
			}
			got := projection.upserts[0][0]
			want := xpRepo.users["u-1"]
			if got.UserID != "u-1" || got.Xp != tt.wantXp || !got.UpdatedAt.Equal(want.UpdatedAt()) {
				t.Errorf("projected %+v, want u-1 %d XP at %v", got, tt.wantXp, want.UpdatedAt())
			}
		})
	}
}

func TestInitUser_CommitFailure_NoSnapshotNoProjection(t *testing.T) {
	snapshots := newMockSnapshotRepo()
	projection := newFakeProjection()
	handler := command.NewInitUserGamificationHandler(
		newMockUserXpRepo(), newMockUserStreakRepo(), snapshots, failingCommitTx{},
	).WithLeaderboardProjector(command.NewProjectLeaderboardScoreHandler(projection))

	if err := handler.Handle(context.Background(), command.InitUserGamificationCommand{UserID: "u-1"}); err == nil {
		t.Fatal("Handle err = nil, want the commit error")
	}
	if len(snapshots.snapshots) != 0 {
		t.Errorf("snapshot saved although the transaction did not commit: %+v", snapshots.snapshots)
	}
	if len(projection.calls) != 0 {
		t.Errorf("projection calls = %v, want none when the transaction did not commit", projection.calls)
	}
}

type mapScoreSource struct {
	scores map[string]entity.LeaderboardScore
	err    error
}

func (m *mapScoreSource) ListScores(context.Context, string, int) ([]entity.LeaderboardScore, error) {
	return nil, errors.New("not used")
}

func (m *mapScoreSource) GetScore(_ context.Context, userID string) (*entity.LeaderboardScore, error) {
	if m.err != nil {
		return nil, m.err
	}
	s, ok := m.scores[userID]
	if !ok {
		return nil, nil
	}
	return &s, nil
}

func TestRefreshLeaderboardScore_ProjectsSourceOfTruth(t *testing.T) {
	at := time.Date(2026, 11, 27, 9, 0, 0, 0, time.UTC)
	source := &mapScoreSource{scores: map[string]entity.LeaderboardScore{"u-1": {UserID: "u-1", Xp: 900, UpdatedAt: at}}}
	projection := newFakeProjection()

	err := command.NewRefreshLeaderboardScoreHandler(source, projection).
		Handle(context.Background(), command.RefreshLeaderboardScoreCommand{UserID: "u-1"})
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if len(projection.upserts) != 1 || projection.upserts[0][0] != source.scores["u-1"] {
		t.Fatalf("upserts = %+v, want the stored score of u-1", projection.upserts)
	}
}

func TestRefreshLeaderboardScore_UnknownUser_Skipped(t *testing.T) {
	projection := newFakeProjection()

	err := command.NewRefreshLeaderboardScoreHandler(&mapScoreSource{scores: map[string]entity.LeaderboardScore{}}, projection).
		Handle(context.Background(), command.RefreshLeaderboardScoreCommand{UserID: "ghost"})
	if err != nil {
		t.Fatalf("Handle err = %v, want nil for a user missing from the source of truth", err)
	}
	if len(projection.calls) != 0 {
		t.Fatalf("projection calls = %v, want none (no ghost entries)", projection.calls)
	}
}

func TestRefreshLeaderboardScore_Errors(t *testing.T) {
	projection := newFakeProjection()
	handler := command.NewRefreshLeaderboardScoreHandler(&mapScoreSource{err: errors.New("db down")}, projection)

	if err := handler.Handle(context.Background(), command.RefreshLeaderboardScoreCommand{UserID: ""}); err == nil {
		t.Error("empty user id err = nil, want validation error")
	}
	if err := handler.Handle(context.Background(), command.RefreshLeaderboardScoreCommand{UserID: "u-1"}); err == nil {
		t.Error("source error err = nil, want error (so the consumer retries)")
	}
	if len(projection.calls) != 0 {
		t.Errorf("projection calls = %v, want none", projection.calls)
	}
}

func TestRebuild_LockLostMidway_AbortsWithoutReady(t *testing.T) {
	projection := newFakeProjection()
	projection.extendErr = errors.New("rebuild lock owned by another node")

	_, err := command.NewRebuildLeaderboardHandler(&fakeScoreSource{ids: userIDs(5)}, projection, 2).
		Handle(context.Background(), command.RebuildLeaderboardCommand{})
	if err == nil {
		t.Fatal("Handle err = nil, want lock-lost error")
	}
	if projection.count("upsert") != 1 || projection.count("ready") != 0 || projection.count("unlock") != 1 {
		t.Errorf("calls = %v, want abort after first page, no ready, unlock once", projection.calls)
	}
}

func TestRebuild_MarkReadyRefused_ReturnsError(t *testing.T) {
	projection := newFakeProjection()
	projection.readyErr = errors.New("rebuild interrupted")

	_, err := command.NewRebuildLeaderboardHandler(&fakeScoreSource{ids: userIDs(1)}, projection, 2).
		Handle(context.Background(), command.RebuildLeaderboardCommand{})
	if err == nil {
		t.Fatal("Handle err = nil, want MarkReady error so the worker retries")
	}
}
