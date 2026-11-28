package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/gamification/application/command"
	"github.com/viethung213/gym-companion/internal/gamification/application/port"
	appService "github.com/viethung213/gym-companion/internal/gamification/application/service"
	"github.com/viethung213/gym-companion/internal/gamification/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/gamification/domain/entity"
)

type recordingProjection struct {
	scores    []entity.LeaderboardScore
	upsertErr error
}

func (p *recordingProjection) UpsertScores(_ context.Context, scores []entity.LeaderboardScore) error {
	if p.upsertErr != nil {
		return p.upsertErr
	}
	p.scores = append(p.scores, scores...)
	return nil
}
func (p *recordingProjection) Reset(context.Context) error                         { return nil }
func (p *recordingProjection) MarkReady(context.Context) error                     { return nil }
func (p *recordingProjection) TryAcquireRebuildLock(context.Context) (bool, error) { return true, nil }
func (p *recordingProjection) ReleaseRebuildLock(context.Context) error            { return nil }
func (p *recordingProjection) BeginRebuild(context.Context) error                  { return nil }
func (p *recordingProjection) ExtendRebuildLock(context.Context) error             { return nil }

type failingTxManager struct{}

func (failingTxManager) WithTransaction(ctx context.Context, fn func(txCtx context.Context) error) error {
	if err := fn(ctx); err != nil {
		return err
	}
	return errors.New("commit failed")
}

func newFastPathCoordinator(xpRepo *mockUserXpRepo, tx port.TransactionManager) *appService.ActivityIngestionCoordinator {
	streakRepo := &mockUserStreakRepo{streaks: make(map[string]*aggregate.UserStreak)}
	badgeRepo := &mockBadgeRepo{badges: make(map[string][]*aggregate.UserBadge)}
	writer := &mockOutboxWriter{}
	return appService.NewActivityIngestionCoordinator(
		command.NewProcessWorkoutXpHandler(xpRepo, tx, writer),
		command.NewRecordStreakActivityHandler(streakRepo, tx, writer),
		badgeRepo,
		tx,
		writer,
	)
}

func workoutCommand(userID string, completedAt time.Time) *command.ProcessWorkoutXpCommand {
	return &command.ProcessWorkoutXpCommand{
		UserID:        userID,
		SessionID:     "sess-" + userID,
		ActivityDate:  completedAt.Format("2006-01-02"),
		TotalSets:     10,
		TotalVolumeKg: 3000,
		CompletedAt:   completedAt,
	}
}

func TestCoordinator_ProjectsXpAfterCommit(t *testing.T) {
	xpRepo := &mockUserXpRepo{users: make(map[string]*aggregate.UserXp)}
	projection := &recordingProjection{}
	coordinator := newFastPathCoordinator(xpRepo, &mockTxManager{}).
		WithLeaderboardProjector(command.NewProjectLeaderboardScoreHandler(projection))
	completedAt := time.Date(2026, 11, 27, 7, 30, 0, 0, time.UTC)

	if err := coordinator.IngestWorkoutSession(context.Background(), workoutCommand("u-1", completedAt)); err != nil {
		t.Fatalf("IngestWorkoutSession: %v", err)
	}

	saved := xpRepo.users["u-1"]
	if saved == nil || saved.Xp() == 0 {
		t.Fatal("expected XP to be awarded")
	}
	if len(projection.scores) != 1 {
		t.Fatalf("projected %d scores, want 1", len(projection.scores))
	}
	got := projection.scores[0]
	if got.UserID != "u-1" || got.Xp != saved.Xp() || !got.UpdatedAt.Equal(completedAt) {
		t.Errorf("projected %+v, want u-1 xp %d at %v", got, saved.Xp(), completedAt)
	}
}

func TestCoordinator_TxError_NoProjection(t *testing.T) {
	xpRepo := &mockUserXpRepo{users: make(map[string]*aggregate.UserXp)}
	projection := &recordingProjection{}
	coordinator := newFastPathCoordinator(xpRepo, failingTxManager{}).
		WithLeaderboardProjector(command.NewProjectLeaderboardScoreHandler(projection))

	if err := coordinator.IngestWorkoutSession(context.Background(), workoutCommand("u-1", time.Now().UTC())); err == nil {
		t.Fatal("IngestWorkoutSession err = nil, want commit failure")
	}
	if len(projection.scores) != 0 {
		t.Fatalf("projected %+v although the transaction failed", projection.scores)
	}
}

func TestCoordinator_ProjectionError_CommandSucceeds(t *testing.T) {
	xpRepo := &mockUserXpRepo{users: make(map[string]*aggregate.UserXp)}
	projection := &recordingProjection{upsertErr: errors.New("redis down")}
	coordinator := newFastPathCoordinator(xpRepo, &mockTxManager{}).
		WithLeaderboardProjector(command.NewProjectLeaderboardScoreHandler(projection))

	if err := coordinator.IngestWorkoutSession(context.Background(), workoutCommand("u-1", time.Now().UTC())); err != nil {
		t.Fatalf("IngestWorkoutSession err = %v, want nil despite projection failure", err)
	}
}

func TestCoordinator_NoProjector_Works(t *testing.T) {
	xpRepo := &mockUserXpRepo{users: make(map[string]*aggregate.UserXp)}
	coordinator := newFastPathCoordinator(xpRepo, &mockTxManager{})

	if err := coordinator.IngestWorkoutSession(context.Background(), workoutCommand("u-1", time.Now().UTC())); err != nil {
		t.Fatalf("IngestWorkoutSession: %v", err)
	}
}
