package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/gamification/application/command"
	appService "github.com/viethung213/gym-companion/internal/gamification/application/service"
	"github.com/viethung213/gym-companion/internal/gamification/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/gamification/domain/entity"
)

type mockTxManager struct{}

func (m *mockTxManager) WithTransaction(ctx context.Context, fn func(txCtx context.Context) error) error {
	return fn(ctx)
}

type mockOutboxWriter struct {
	events []any
}

func (m *mockOutboxWriter) WriteEvents(ctx context.Context, aggregateType, aggregateID string, events []any) error {
	m.events = append(m.events, events...)
	return nil
}

type mockUserXpRepo struct {
	users   map[string]*aggregate.UserXp
	history []*entity.XpHistoryEntry
}

func (m *mockUserXpRepo) FindByID(ctx context.Context, userID string) (*aggregate.UserXp, error) {
	return m.users[userID], nil
}

func (m *mockUserXpRepo) FindByIDForUpdate(ctx context.Context, userID string) (*aggregate.UserXp, error) {
	return m.users[userID], nil
}

func (m *mockUserXpRepo) Save(ctx context.Context, userXp *aggregate.UserXp) error {
	m.users[userXp.UserID()] = userXp
	return nil
}

func (m *mockUserXpRepo) SaveHistory(ctx context.Context, history *entity.XpHistoryEntry) error {
	m.history = append(m.history, history)
	return nil
}

func (m *mockUserXpRepo) GetHistory(ctx context.Context, userID string, limit, offset int) ([]*entity.XpHistoryEntry, int64, error) {
	return m.history, int64(len(m.history)), nil
}

func (m *mockUserXpRepo) HasSession(ctx context.Context, userID, sessionID string) (bool, error) {
	for _, h := range m.history {
		if h.UserID == userID && h.SessionID != nil && *h.SessionID == sessionID {
			return true, nil
		}
	}
	return false, nil
}

type mockUserStreakRepo struct {
	streaks map[string]*aggregate.UserStreak
	history []*entity.StreakHistoryEntry
}

func (m *mockUserStreakRepo) FindByID(ctx context.Context, userID string) (*aggregate.UserStreak, error) {
	return m.streaks[userID], nil
}

func (m *mockUserStreakRepo) FindByIDForUpdate(ctx context.Context, userID string) (*aggregate.UserStreak, error) {
	return m.streaks[userID], nil
}

func (m *mockUserStreakRepo) Save(ctx context.Context, userStreak *aggregate.UserStreak) error {
	m.streaks[userStreak.UserID()] = userStreak
	return nil
}

func (m *mockUserStreakRepo) SaveHistory(ctx context.Context, history *entity.StreakHistoryEntry) error {
	m.history = append(m.history, history)
	return nil
}

func (m *mockUserStreakRepo) GetHistory(ctx context.Context, userID string, limit, offset int) ([]*entity.StreakHistoryEntry, int64, error) {
	return m.history, int64(len(m.history)), nil
}

type mockBadgeRepo struct {
	badges       map[string][]*aggregate.UserBadge
	saveBatchErr error
}

func (m *mockBadgeRepo) FindByUserID(ctx context.Context, userID string) ([]*aggregate.UserBadge, error) {
	return m.badges[userID], nil
}

func (m *mockBadgeRepo) HasBadge(ctx context.Context, userID, badgeCode string) (bool, error) {
	for _, b := range m.badges[userID] {
		if b.BadgeCode() == badgeCode {
			return true, nil
		}
	}
	return false, nil
}

func (m *mockBadgeRepo) Save(ctx context.Context, badge *aggregate.UserBadge) error {
	m.badges[badge.UserID()] = append(m.badges[badge.UserID()], badge)
	return nil
}

func (m *mockBadgeRepo) SaveBatch(ctx context.Context, badges []*aggregate.UserBadge) error {
	if m.saveBatchErr != nil {
		return m.saveBatchErr
	}
	for _, b := range badges {
		m.badges[b.UserID()] = append(m.badges[b.UserID()], b)
	}
	return nil
}

func TestActivityIngestionCoordinator_IngestWorkoutSession(t *testing.T) {
	xpRepo := &mockUserXpRepo{users: make(map[string]*aggregate.UserXp)}
	streakRepo := &mockUserStreakRepo{streaks: make(map[string]*aggregate.UserStreak)}
	badgeRepo := &mockBadgeRepo{badges: make(map[string][]*aggregate.UserBadge)}
	tx := &mockTxManager{}
	writer := &mockOutboxWriter{}

	xpHandler := command.NewProcessWorkoutXpHandler(xpRepo, tx, writer)
	streakHandler := command.NewRecordStreakActivityHandler(streakRepo, tx, writer)

	coordinator := appService.NewActivityIngestionCoordinator(
		xpHandler,
		streakHandler,
		streakRepo,
		badgeRepo,
		tx,
		writer,
	)

	ctx := context.Background()
	now := time.Now().UTC()

	// Ingest session
	err := coordinator.IngestWorkoutSession(ctx, &command.ProcessWorkoutXpCommand{
		UserID:        "user-hero",
		SessionID:     "sess-1",
		ActivityDate:  "2026-10-13",
		TotalSets:     10,
		TotalVolumeKg: 3000,
		CompletedAt:   now,
	})
	if err != nil {
		t.Fatalf("IngestWorkoutSession error: %v", err)
	}

	// 1. Verify Step 1 (XP) executed
	userXp := xpRepo.users["user-hero"]
	if userXp == nil || userXp.Xp() == 0 {
		t.Errorf("expected userXp to be awarded XP, got: %+v", userXp)
	}

	// 2. Verify Step 2 (Streak) executed
	userStreak := streakRepo.streaks["user-hero"]
	if userStreak == nil || userStreak.CurrentStreak() != 1 {
		t.Errorf("expected streak to be 1, got: %+v", userStreak)
	}

	// 3. Verify Step 3 (Badge FIRST_WORKOUT unlocked)
	badges := badgeRepo.badges["user-hero"]
	if len(badges) == 0 {
		t.Fatalf("expected badges unlocked for first workout, got 0")
	}
	if badges[0].BadgeCode() != "FIRST_WORKOUT" {
		t.Errorf("badge code = %q, want 'FIRST_WORKOUT'", badges[0].BadgeCode())
	}
}

func TestActivityIngestionCoordinator_StreakAdvancesMultiplierImmediately(t *testing.T) {
	xpRepo := &mockUserXpRepo{users: make(map[string]*aggregate.UserXp)}
	streakRepo := &mockUserStreakRepo{streaks: make(map[string]*aggregate.UserStreak)}
	badgeRepo := &mockBadgeRepo{badges: make(map[string][]*aggregate.UserBadge)}
	tx := &mockTxManager{}
	writer := &mockOutboxWriter{}

	// Setup user with streak = 6 (last active date = 2026-10-12)
	reconstitutedStreak := aggregate.ReconstituteUserStreak(
		"user-streak-6",
		6,
		6,
		"2026-10-12",
		time.Now().UTC(),
	)
	streakRepo.streaks["user-streak-6"] = reconstitutedStreak

	coordinator := appService.NewActivityIngestionCoordinator(
		command.NewProcessWorkoutXpHandler(xpRepo, tx, writer),
		command.NewRecordStreakActivityHandler(streakRepo, tx, writer),
		streakRepo,
		badgeRepo,
		tx,
		writer,
	)

	ctx := context.Background()
	now := time.Now().UTC()

	// Ingest session on 2026-10-13 (Day 7)
	// Base: 50 + Volume: 20 (3000kg) = 70 raw XP.
	// With streak advancing to 7 first: multiplier is 1.20 -> 70 * 1.20 = 84 XP!
	// (If streak was evaluated after: multiplier would only be 1.10 -> 70 * 1.10 = 77 XP).
	err := coordinator.IngestWorkoutSession(ctx, &command.ProcessWorkoutXpCommand{
		UserID:        "user-streak-6",
		SessionID:     "sess-streak-7",
		ActivityDate:  "2026-10-13",
		TotalSets:     10,
		TotalVolumeKg: 3000,
		CompletedAt:   now,
	})
	if err != nil {
		t.Fatalf("IngestWorkoutSession error: %v", err)
	}

	userStreak := streakRepo.streaks["user-streak-6"]
	if userStreak.CurrentStreak() != 7 {
		t.Fatalf("expected streak to advance to 7, got: %d", userStreak.CurrentStreak())
	}

	userXp := xpRepo.users["user-streak-6"]
	if userXp == nil {
		t.Fatalf("expected userXp to be created")
	}

	// 70 * 1.20 = 84 XP
	if userXp.Xp() != 84 {
		t.Errorf("expected 84 XP with 1.2x streak multiplier applied immediately, got %d", userXp.Xp())
	}
}

func TestActivityIngestionCoordinator_BadgeErrorFailsAtomicTransaction(t *testing.T) {
	xpRepo := &mockUserXpRepo{users: make(map[string]*aggregate.UserXp)}
	streakRepo := &mockUserStreakRepo{streaks: make(map[string]*aggregate.UserStreak)}
	badgeRepo := &mockBadgeRepo{
		badges:       make(map[string][]*aggregate.UserBadge),
		saveBatchErr: errors.New("simulated db connection timeout"),
	}
	tx := &mockTxManager{}
	writer := &mockOutboxWriter{}

	coordinator := appService.NewActivityIngestionCoordinator(
		command.NewProcessWorkoutXpHandler(xpRepo, tx, writer),
		command.NewRecordStreakActivityHandler(streakRepo, tx, writer),
		streakRepo,
		badgeRepo,
		tx,
		writer,
	)

	ctx := context.Background()
	now := time.Now().UTC()

	err := coordinator.IngestWorkoutSession(ctx, &command.ProcessWorkoutXpCommand{
		UserID:        "user-fail",
		SessionID:     "sess-fail",
		ActivityDate:  "2026-10-13",
		TotalSets:     10,
		TotalVolumeKg: 3000,
		CompletedAt:   now,
	})
	if err == nil {
		t.Fatalf("expected IngestWorkoutSession to fail when evaluateBadges fails, got nil")
	}
}
