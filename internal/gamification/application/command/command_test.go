package command_test

import (
	"context"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/gamification/application/command"
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

func newMockUserXpRepo() *mockUserXpRepo {
	return &mockUserXpRepo{
		users:   make(map[string]*aggregate.UserXp),
		history: make([]*entity.XpHistoryEntry, 0),
	}
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

func newMockUserStreakRepo() *mockUserStreakRepo {
	return &mockUserStreakRepo{
		streaks: make(map[string]*aggregate.UserStreak),
		history: make([]*entity.StreakHistoryEntry, 0),
	}
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

type mockLeaderboardRepo struct {
	snapshots map[string]*entity.UserSnapshot
}

func newMockLeaderboardRepo() *mockLeaderboardRepo {
	return &mockLeaderboardRepo{
		snapshots: make(map[string]*entity.UserSnapshot),
	}
}

func (m *mockLeaderboardRepo) GetGlobalTop(ctx context.Context, limit, offset int) ([]*entity.LeaderboardUser, int64, error) {
	return nil, 0, nil
}

func (m *mockLeaderboardRepo) GetUserRank(ctx context.Context, userID string) (int64, *entity.LeaderboardUser, error) {
	return 0, nil, nil
}

func (m *mockLeaderboardRepo) SaveSnapshot(ctx context.Context, snapshot *entity.UserSnapshot) error {
	m.snapshots[snapshot.UserID] = snapshot
	return nil
}

func (m *mockLeaderboardRepo) GetSnapshot(ctx context.Context, userID string) (*entity.UserSnapshot, error) {
	return m.snapshots[userID], nil
}

func TestProcessWorkoutXpHandler(t *testing.T) {
	repo := newMockUserXpRepo()
	tx := &mockTxManager{}
	writer := &mockOutboxWriter{}
	handler := command.NewProcessWorkoutXpHandler(repo, tx, writer)

	ctx := context.Background()
	now := time.Now().UTC()

	// 1. Proof of Workout failure (sets < 3)
	_, err := handler.Handle(ctx, &command.ProcessWorkoutXpCommand{
		UserID:       "u-1",
		SessionID:    "s-1",
		ActivityDate: "2026-10-13",
		TotalSets:    2, // < 3 sets
		CompletedAt:  now,
	})
	if err == nil {
		t.Fatal("expected error for insufficient workout sets, got nil")
	}

	// 2. Successful workout
	res, err := handler.Handle(ctx, &command.ProcessWorkoutXpCommand{
		UserID:        "u-1",
		SessionID:     "s-1",
		ActivityDate:  "2026-10-13",
		TotalSets:     8,
		TotalVolumeKg: 2500,
		CompletedAt:   now,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.DeltaXp != 70 { // Base 50 + Vol 20 = 70 XP
		t.Errorf("DeltaXp = %d, want 70", res.DeltaXp)
	}
	if res.NewXp != 70 || res.NewLevel != 2 {
		t.Errorf("state mismatch: xp = %d, level = %d", res.NewXp, res.NewLevel)
	}
	if len(repo.history) != 1 {
		t.Errorf("xp_history count = %d, want 1", len(repo.history))
	}
	if len(writer.events) == 0 {
		t.Errorf("expected outbox events written, got 0")
	}

	// 3. Repeat workout with same session ID (Kafka retry simulation -> Idempotent Early Return)
	eventsCountBefore := len(writer.events)
	resRetry, err := handler.Handle(ctx, &command.ProcessWorkoutXpCommand{
		UserID:        "u-1",
		SessionID:     "s-1",
		ActivityDate:  "2026-10-13",
		TotalSets:     8,
		TotalVolumeKg: 2500,
		CompletedAt:   now,
	})
	if err != nil {
		t.Fatalf("unexpected error on idempotent retry: %v", err)
	}
	if resRetry.DeltaXp != 0 {
		t.Errorf("expected DeltaXp = 0 on retry, got %d", resRetry.DeltaXp)
	}
	if resRetry.NewXp != 70 {
		t.Errorf("expected NewXp = 70 without duplicate increment, got %d", resRetry.NewXp)
	}
	if len(repo.history) != 1 {
		t.Errorf("expected xp_history count to stay 1, got %d", len(repo.history))
	}
	if len(writer.events) != eventsCountBefore {
		t.Errorf("expected no duplicate outbox events, got %d vs %d", len(writer.events), eventsCountBefore)
	}
}

func TestProcessNutritionXpHandler(t *testing.T) {
	repo := newMockUserXpRepo()
	tx := &mockTxManager{}
	writer := &mockOutboxWriter{}
	handler := command.NewProcessNutritionXpHandler(repo, tx, writer)

	ctx := context.Background()
	now := time.Now().UTC()

	// 1. First claim on 2026-10-13 -> Claimed: true, +30 XP
	res, err := handler.Handle(ctx, command.ProcessNutritionXpCommand{
		UserID:       "u-1",
		ActivityDate: "2026-10-13",
		AchievedAt:   now,
	})
	if err != nil {
		t.Fatalf("Handle nutrition xp error: %v", err)
	}
	if !res.Claimed || res.DeltaXp != 30 || res.NewXp != 30 {
		t.Errorf("unexpected nutrition result: %+v", res)
	}

	// 2. Second claim on SAME date -> Claimed: false, 0 delta
	res2, err := handler.Handle(ctx, command.ProcessNutritionXpCommand{
		UserID:       "u-1",
		ActivityDate: "2026-10-13",
		AchievedAt:   now.Add(2 * time.Hour),
	})
	if err != nil {
		t.Fatalf("Handle duplicate nutrition xp error: %v", err)
	}
	if res2.Claimed || res2.DeltaXp != 0 {
		t.Errorf("duplicate claim must return Claimed: false, got: %+v", res2)
	}
}

func TestRecordStreakActivityHandler(t *testing.T) {
	repo := newMockUserStreakRepo()
	tx := &mockTxManager{}
	writer := &mockOutboxWriter{}
	handler := command.NewRecordStreakActivityHandler(repo, tx, writer)

	ctx := context.Background()
	now := time.Now().UTC()

	// Day 1
	res, err := handler.Handle(ctx, command.RecordStreakActivityCommand{
		UserID:       "u-1",
		ActivityDate: "2026-10-10",
		ActivityType: "WORKOUT",
		RecordedAt:   now,
	})
	if err != nil || !res.Advanced || res.CurrentStreak != 1 {
		t.Fatalf("Day 1 streak unexpected: %+v, err: %v", res, err)
	}

	// Same Day second activity (Idempotent)
	res2, err := handler.Handle(ctx, command.RecordStreakActivityCommand{
		UserID:       "u-1",
		ActivityDate: "2026-10-10",
		ActivityType: "NUTRITION",
		RecordedAt:   now.Add(3 * time.Hour),
	})
	if err != nil || res2.Advanced || res2.CurrentStreak != 1 {
		t.Fatalf("Same day streak should not advance: %+v, err: %v", res2, err)
	}

	// Day 2 (Consecutive)
	res3, err := handler.Handle(ctx, command.RecordStreakActivityCommand{
		UserID:       "u-1",
		ActivityDate: "2026-10-11",
		ActivityType: "WORKOUT",
		RecordedAt:   now.Add(24 * time.Hour),
	})
	if err != nil || !res3.Advanced || res3.CurrentStreak != 2 {
		t.Fatalf("Day 2 streak unexpected: %+v, err: %v", res3, err)
	}
}

func TestSyncUserSnapshotHandler(t *testing.T) {
	repo := newMockLeaderboardRepo()
	handler := command.NewSyncUserSnapshotHandler(repo)

	err := handler.Handle(context.Background(), command.SyncUserSnapshotCommand{
		UserID:      "u-1",
		DisplayName: "Iron Man",
		AvatarURL:   "https://cdn.example.com/iron.jpg",
	})
	if err != nil {
		t.Fatalf("SyncUserSnapshot error: %v", err)
	}
	if snap := repo.snapshots["u-1"]; snap == nil || snap.DisplayName != "Iron Man" {
		t.Errorf("snapshot mismatch: %+v", snap)
	}
}

func TestInitUserGamificationHandler(t *testing.T) {
	xpRepo := newMockUserXpRepo()
	streakRepo := newMockUserStreakRepo()
	lbRepo := newMockLeaderboardRepo()
	tx := &mockTxManager{}

	handler := command.NewInitUserGamificationHandler(xpRepo, streakRepo, lbRepo, tx)

	err := handler.Handle(context.Background(), command.InitUserGamificationCommand{
		UserID:      "u-fresh",
		DisplayName: "New Gymer",
		AvatarURL:   "",
	})
	if err != nil {
		t.Fatalf("InitUserGamification error: %v", err)
	}

	if xpRepo.users["u-fresh"] == nil || xpRepo.users["u-fresh"].Xp() != 0 {
		t.Errorf("expected baseline user_xp with 0 XP")
	}
	if streakRepo.streaks["u-fresh"] == nil || streakRepo.streaks["u-fresh"].CurrentStreak() != 0 {
		t.Errorf("expected baseline user_streak with 0 days")
	}
	if lbRepo.snapshots["u-fresh"] == nil || lbRepo.snapshots["u-fresh"].DisplayName != "New Gymer" {
		t.Errorf("expected baseline user_snapshot")
	}
}

type mockBadgeRepo struct {
	badges map[string][]*aggregate.UserBadge
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
	for _, b := range badges {
		m.badges[b.UserID()] = append(m.badges[b.UserID()], b)
	}
	return nil
}

func TestProcessPersonalRecordHandler(t *testing.T) {
	xpRepo := newMockUserXpRepo()
	badgeRepo := &mockBadgeRepo{badges: make(map[string][]*aggregate.UserBadge)}
	tx := &mockTxManager{}
	outbox := &mockOutboxWriter{}

	handler := command.NewProcessPersonalRecordHandler(xpRepo, badgeRepo, tx, outbox)

	now := time.Now().UTC()
	err := handler.Handle(context.Background(), command.ProcessPersonalRecordCommand{
		UserID:       "u-pr-1",
		ExerciseID:   "bench_press",
		Weight:       100.0,
		Reps:         5,
		ActivityDate: "2026-10-22",
		AchievedAt:   now,
	})
	if err != nil {
		t.Fatalf("ProcessPersonalRecord error: %v", err)
	}

	userXp := xpRepo.users["u-pr-1"]
	if userXp == nil || userXp.Xp() != 25 {
		t.Errorf("expected 25 XP for PR, got: %+v", userXp)
	}

	if len(badgeRepo.badges["u-pr-1"]) != 1 || badgeRepo.badges["u-pr-1"][0].BadgeCode() != "PR_BREAKER" {
		t.Errorf("expected PR_BREAKER badge to be unlocked, got: %+v", badgeRepo.badges["u-pr-1"])
	}
}

func TestProcessNutritionActivityHandler(t *testing.T) {
	streakRepo := newMockUserStreakRepo()
	xpRepo := newMockUserXpRepo()
	tx := &mockTxManager{}
	outbox := &mockOutboxWriter{}

	streakHandler := command.NewRecordStreakActivityHandler(streakRepo, tx, outbox)
	nutritionHandler := command.NewProcessNutritionXpHandler(xpRepo, tx, outbox)

	handler := command.NewProcessNutritionActivityHandler(streakHandler, nutritionHandler, tx)

	now := time.Now().UTC()
	err := handler.Handle(context.Background(), command.ProcessNutritionActivityCommand{
		UserID:       "u-nut-1",
		ActivityDate: "2026-10-22",
		RecordedAt:   now,
	})
	if err != nil {
		t.Fatalf("ProcessNutritionActivity error: %v", err)
	}

	userStreak := streakRepo.streaks["u-nut-1"]
	if userStreak == nil || userStreak.CurrentStreak() != 1 {
		t.Errorf("expected streak to advance to 1, got: %+v", userStreak)
	}

	userXp := xpRepo.users["u-nut-1"]
	if userXp == nil || userXp.Xp() != 30 {
		t.Errorf("expected 30 XP for nutrition, got: %+v", userXp)
	}
}
