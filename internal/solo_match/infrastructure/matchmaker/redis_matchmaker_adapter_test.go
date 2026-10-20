package matchmaker_test

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/glebarez/sqlite"
	"github.com/redis/go-redis/v9"
	"github.com/viethung213/gym-companion/internal/solo_match/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/solo_match/domain/service"
	exerciseclient "github.com/viethung213/gym-companion/internal/solo_match/infrastructure/exercise_client"
	"github.com/viethung213/gym-companion/internal/solo_match/infrastructure/matchmaker"
	"github.com/viethung213/gym-companion/internal/solo_match/infrastructure/persistence"
	workoutclient "github.com/viethung213/gym-companion/internal/solo_match/infrastructure/workout_client"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:matchmaker_test?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Open sqlite in-memory db err: %v", err)
	}

	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)

	db.Exec("ATTACH DATABASE ':memory:' AS solo_match;")

	err = db.Exec(`
	CREATE TABLE IF NOT EXISTS solo_match.solo_matches (
		id TEXT PRIMARY KEY,
		exercise_id TEXT NOT NULL,
		exercise_name TEXT NOT NULL,
		challenge_type TEXT NOT NULL,
		status TEXT NOT NULL,
		winner_id TEXT,
		is_draw NUMERIC DEFAULT 0,
		timeout_at DATETIME NOT NULL,
		started_at DATETIME NOT NULL,
		ended_at DATETIME,
		created_at DATETIME,
		updated_at DATETIME
	);
	CREATE TABLE IF NOT EXISTS solo_match.match_participants (
		id TEXT PRIMARY KEY,
		match_id TEXT NOT NULL,
		user_id TEXT NOT NULL,
		score_value REAL DEFAULT 0,
		is_completed NUMERIC DEFAULT 0,
		completed_at DATETIME,
		result TEXT DEFAULT 'PENDING',
		created_at DATETIME,
		updated_at DATETIME
	);
	CREATE TABLE IF NOT EXISTS solo_match.system_bots (
		user_id TEXT PRIMARY KEY,
		difficulty_level TEXT DEFAULT 'INTERMEDIATE',
		is_active NUMERIC DEFAULT 1,
		created_at DATETIME
	);
	INSERT OR IGNORE INTO solo_match.system_bots (user_id, is_active) VALUES ('bot_athlete_1', 1);
	`).Error
	if err != nil {
		t.Fatalf("Create tables err: %v", err)
	}

	return db
}

func TestRedisMatchmakerAdapter(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	defer rdb.Close()

	db := setupTestDB(t)
	matchRepo := persistence.NewGORMSoloMatchRepository(db)
	botRepo := persistence.NewGORMBotRepository(db)
	botCalc := service.NewBotBenchmarkCalculator()

	adapter := matchmaker.NewRedisMatchmakerAdapter(
		rdb,
		matchRepo,
		botCalc,
		botRepo,
		exerciseclient.NewStubExerciseClient(),
		workoutclient.NewStubWorkoutClient(),
		50*time.Millisecond,
		500*time.Millisecond,
	)
	defer func() {
		_ = adapter.Close()
	}()

	ctx := context.Background()
	now := time.Now().UTC()

	// 1. Enqueue 2 tickets và nhận sự kiện qua Redis Pub/Sub
	t1, _ := aggregate.NewMatchmakingTicket("t_1", "user_1", now)
	t2, _ := aggregate.NewMatchmakingTicket("t_2", "user_2", now)

	listenCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	listenCh1, err := adapter.ListenMatch(listenCtx, "t_1")
	if err != nil {
		t.Fatalf("ListenMatch t1 err: %v", err)
	}

	err = adapter.Enqueue(ctx, t1)
	if err != nil {
		t.Fatalf("Enqueue t1 err: %v", err)
	}

	err = adapter.Enqueue(ctx, t2)
	if err != nil {
		t.Fatalf("Enqueue t2 err: %v", err)
	}

	select {
	case event := <-listenCh1:
		if event == nil {
			t.Fatalf("listenCh1 received nil event")
		}
		if event.MatchID == "" || event.Match == nil {
			t.Errorf("listenCh1 unexpected event: %+v", event)
		}
	case <-time.After(3 * time.Second):
		t.Fatalf("timeout waiting for match on t_1")
	}

	// 2. Test Cancel
	t3, _ := aggregate.NewMatchmakingTicket("t_3", "user_3", now)
	_ = adapter.Enqueue(ctx, t3)
	err = adapter.Cancel(ctx, "t_3", "user_3")
	if err != nil {
		t.Fatalf("Cancel t_3 err: %v", err)
	}

	// Xác nhận vé t_3 đã bị xóa khỏi Redis ZSET
	score, _ := rdb.ZScore(ctx, matchmaker.RedisQueueKey, "t_3:user_3").Result()
	if score != 0 {
		t.Errorf("expected score 0 (not in set), got %f", score)
	}
}

type mockPRProvider struct {
	prValue *float64
}

func (m *mockPRProvider) GetBestPR(ctx context.Context, userID, exerciseID string) (*float64, error) {
	return m.prValue, nil
}

func TestRedisMatchmakerAdapter_BotFallback_WithPR(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	defer rdb.Close()

	db := setupTestDB(t)
	matchRepo := persistence.NewGORMSoloMatchRepository(db)
	botRepo := persistence.NewGORMBotRepository(db)
	botCalc := service.NewBotBenchmarkCalculator()

	targetPR := 35.0
	prProvider := &mockPRProvider{prValue: &targetPR}

	adapter := matchmaker.NewRedisMatchmakerAdapter(
		rdb,
		matchRepo,
		botCalc,
		botRepo,
		exerciseclient.NewStubExerciseClient(),
		prProvider,
		20*time.Millisecond,
		50*time.Millisecond,
	)
	defer func() {
		_ = adapter.Close()
	}()

	ctx := context.Background()
	listenCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	listenCh, err := adapter.ListenMatch(listenCtx, "t_bot_user")
	if err != nil {
		t.Fatalf("ListenMatch err: %v", err)
	}

	// Tạo ticket có timestamp cũ để trigger bot fallback
	pastTime := time.Now().UTC().Add(-2 * time.Second)
	tUser, _ := aggregate.NewMatchmakingTicket("t_bot_user", "user_bot_test", pastTime)
	_ = adapter.Enqueue(ctx, tUser)

	select {
	case event := <-listenCh:
		if event == nil || event.Match == nil {
			t.Fatalf("expected valid match event, got nil")
		}
		var foundBot bool
		for _, p := range event.Match.Participants() {
			if p.UserID() == "bot_athlete_1" {
				foundBot = true
				if p.Score().Value() != 35.0 {
					t.Errorf("expected bot score to match user PR 35.0, got %f", p.Score().Value())
				}
			}
		}
		if !foundBot {
			t.Errorf("expected bot participant bot_athlete_1 in match, but not found")
		}
	case <-time.After(3 * time.Second):
		t.Fatalf("timeout waiting for bot match fallback")
	}
}

