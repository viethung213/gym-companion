package matchmaker_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/glebarez/sqlite"
	"github.com/redis/go-redis/v9"
	"github.com/viethung213/gym-companion/internal/solo_match/application/port"
	"github.com/viethung213/gym-companion/internal/solo_match/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/solo_match/domain/derror"
	"github.com/viethung213/gym-companion/internal/solo_match/domain/entity"
	"github.com/viethung213/gym-companion/internal/solo_match/domain/event"
	"github.com/viethung213/gym-companion/internal/solo_match/domain/repository"
	valueobject "github.com/viethung213/gym-companion/internal/solo_match/domain/value_object"
	"github.com/viethung213/gym-companion/internal/solo_match/infrastructure/matchmaker"
	"github.com/viethung213/gym-companion/internal/solo_match/infrastructure/persistence"
	"gorm.io/gorm"
)

type mockTestExerciseProvider struct{}

func (mockTestExerciseProvider) GetRandomChallenge(_ context.Context) (*port.ExerciseChallenge, error) {
	return &port.ExerciseChallenge{
		ExerciseID:    "ex_pushup",
		ExerciseName:  "Push-Up",
		ChallengeType: valueobject.ChallengeTypeReps,
		Duration:      15 * time.Minute,
	}, nil
}

type mockEventPublisher struct {
	mu     sync.Mutex
	events []any
}

func (m *mockEventPublisher) PublishEvents(_ context.Context, events []any) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, events...)
	return nil
}

func (m *mockEventPublisher) Events() []any {
	m.mu.Lock()
	defer m.mu.Unlock()
	copied := make([]any, len(m.events))
	copy(copied, m.events)
	return copied
}

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
		ready_deadline DATETIME NOT NULL,
		started_at DATETIME,
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
	evtPub := &mockEventPublisher{}

	adapter := matchmaker.NewRedisMatchmakerAdapter(
		rdb,
		matchRepo,
		botRepo,
		mockTestExerciseProvider{},
		evtPub,
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

func TestRedisMatchmakerAdapter_BotFallback(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	defer rdb.Close()

	db := setupTestDB(t)
	matchRepo := persistence.NewGORMSoloMatchRepository(db)
	botRepo := persistence.NewGORMBotRepository(db)
	evtPub := &mockEventPublisher{}

	adapter := matchmaker.NewRedisMatchmakerAdapter(
		rdb,
		matchRepo,
		botRepo,
		mockTestExerciseProvider{},
		evtPub,
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
	case matchFound := <-listenCh:
		if matchFound == nil || matchFound.Match == nil {
			t.Fatalf("expected valid match event, got nil")
		}
		var foundBot bool
		for _, p := range matchFound.Match.Participants() {
			if p.UserID() == "bot_athlete_1" {
				foundBot = true
				if p.IsCompleted() {
					t.Errorf("expected bot participant to be pending initially, but got completed")
				}
				if p.Score().Value() != 0 {
					t.Errorf("expected bot score to be 0 initially, got %f", p.Score().Value())
				}
			}
		}
		if !foundBot {
			t.Errorf("expected bot participant bot_athlete_1 in match, but not found")
		}

		// Verify event SoloMatchStarted was published with botUserID
		var foundStartedEvent bool
		for _, ev := range evtPub.Events() {
			if startEv, ok := ev.(*event.SoloMatchStarted); ok {
				if startEv.BotUserID == "bot_athlete_1" && startEv.Player1ID == "user_bot_test" {
					foundStartedEvent = true
					break
				}
			}
		}
		if !foundStartedEvent {
			t.Errorf("expected SoloMatchStarted event with bot_athlete_1, but not found in published events")
		}
	case <-time.After(3 * time.Second):
		t.Fatalf("timeout waiting for bot match fallback")
	}
}

type mockFailingChallengeProvider struct{}

func (mockFailingChallengeProvider) GetRandomChallenge(_ context.Context) (*port.ExerciseChallenge, error) {
	return nil, errors.New("exercise provider unavailable")
}

func TestRedisMatchmakerAdapter_ChallengeErrorNotifiesClient(t *testing.T) {
	ctx := context.Background()
	db := setupTestDB(t)
	mr := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	matchRepo := persistence.NewGORMSoloMatchRepository(db)
	failingProvider := mockFailingChallengeProvider{}

	adapter := matchmaker.NewRedisMatchmakerAdapter(
		redisClient,
		matchRepo,
		nil,
		failingProvider,
		nil,
		10*time.Millisecond,
		1*time.Second,
	)
	defer adapter.Close()

	listenCtx, listenCancel := context.WithTimeout(ctx, 3*time.Second)
	defer listenCancel()

	listenCh, err := adapter.ListenMatch(listenCtx, "t_err_1")
	if err != nil {
		t.Fatalf("ListenMatch err: %v", err)
	}

	t1, _ := aggregate.NewMatchmakingTicket("t_err_1", "user_err_1", time.Now().UTC())
	t2, _ := aggregate.NewMatchmakingTicket("t_err_2", "user_err_2", time.Now().UTC())
	_ = adapter.Enqueue(ctx, t1)
	_ = adapter.Enqueue(ctx, t2)

	select {
	case ev := <-listenCh:
		if ev == nil {
			t.Fatalf("expected error event, got nil channel read")
		}
		if ev.Err == nil {
			t.Fatalf("expected ev.Err to be non-nil when challenge fails, got nil")
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timeout waiting for error notification when challenge fails, client was hung")
	}
}

type mockFailingMatchRepo struct {
	repository.SoloMatchRepository
}

func (mockFailingMatchRepo) Save(_ context.Context, _ *aggregate.SoloMatch) error {
	return errors.New("database connection lost")
}

func (mockFailingMatchRepo) FindByID(_ context.Context, _ string) (*aggregate.SoloMatch, error) {
	return nil, nil
}

func TestRedisMatchmakerAdapter_SaveErrorNotifiesClient(t *testing.T) {
	ctx := context.Background()
	mr := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	exerciseProv := mockTestExerciseProvider{}
	failingRepo := mockFailingMatchRepo{}

	adapter := matchmaker.NewRedisMatchmakerAdapter(
		redisClient,
		failingRepo,
		nil,
		exerciseProv,
		nil,
		10*time.Millisecond,
		1*time.Second,
	)
	defer adapter.Close()

	listenCtx, listenCancel := context.WithTimeout(ctx, 3*time.Second)
	defer listenCancel()

	listenCh, err := adapter.ListenMatch(listenCtx, "t_fail_1")
	if err != nil {
		t.Fatalf("ListenMatch err: %v", err)
	}

	t1, _ := aggregate.NewMatchmakingTicket("t_fail_1", "user_fail_1", time.Now().UTC())
	t2, _ := aggregate.NewMatchmakingTicket("t_fail_2", "user_fail_2", time.Now().UTC())
	_ = adapter.Enqueue(ctx, t1)
	_ = adapter.Enqueue(ctx, t2)

	select {
	case ev := <-listenCh:
		if ev == nil {
			t.Fatalf("expected error event, got nil channel read")
		}
		if ev.Err == nil {
			t.Fatalf("expected ev.Err to be non-nil when save fails, got nil")
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timeout waiting for error notification, client was hung")
	}
}

type mockSimpleMatchRepo struct {
	repository.SoloMatchRepository
	err           error
	matchToReturn *aggregate.SoloMatch
}

func (m *mockSimpleMatchRepo) FindByID(_ context.Context, _ string) (*aggregate.SoloMatch, error) {
	return m.matchToReturn, m.err
}

func TestRedisMatchmakerAdapter_FindByIDErrorNotifiesClient(t *testing.T) {
	ctx := context.Background()
	mr := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	repo := &mockSimpleMatchRepo{
		err: errors.New("db connection failure"),
	}

	adapter := matchmaker.NewRedisMatchmakerAdapter(
		redisClient,
		repo,
		nil,
		mockTestExerciseProvider{},
		nil,
		10*time.Millisecond,
		1*time.Second,
	)
	defer adapter.Close()

	listenCtx, listenCancel := context.WithTimeout(ctx, 2*time.Second)
	defer listenCancel()

	listenCh, err := adapter.ListenMatch(listenCtx, "t_repo_err")
	if err != nil {
		t.Fatalf("ListenMatch err: %v", err)
	}

	// Giả lập message matchID đã được publish
	_ = redisClient.Publish(ctx, matchmaker.RedisEventPrefix+"t_repo_err", "m_err_123")

	select {
	case ev := <-listenCh:
		if ev == nil {
			t.Fatalf("expected non-nil event")
		}
		if ev.Err == nil {
			t.Fatalf("got nil Err, want non-nil db error")
		}
	case <-time.After(1 * time.Second):
		t.Fatalf("timeout waiting for error event when FindByID fails")
	}
}

func TestRedisMatchmakerAdapter_FindByIDNotFoundNotifiesClient(t *testing.T) {
	ctx := context.Background()
	mr := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	repo := &mockSimpleMatchRepo{
		err:           nil,
		matchToReturn: nil, // returns nil, nil
	}

	adapter := matchmaker.NewRedisMatchmakerAdapter(
		redisClient,
		repo,
		nil,
		mockTestExerciseProvider{},
		nil,
		10*time.Millisecond,
		1*time.Second,
	)
	defer adapter.Close()

	listenCtx, listenCancel := context.WithTimeout(ctx, 2*time.Second)
	defer listenCancel()

	listenCh, err := adapter.ListenMatch(listenCtx, "t_not_found")
	if err != nil {
		t.Fatalf("ListenMatch err: %v", err)
	}

	_ = redisClient.Publish(ctx, matchmaker.RedisEventPrefix+"t_not_found", "m_missing")

	select {
	case ev := <-listenCh:
		if ev == nil {
			t.Fatalf("expected non-nil event")
		}
		if ev.Err == nil {
			t.Fatalf("got nil Err, want match not found error")
		}
	case <-time.After(1 * time.Second):
		t.Fatalf("timeout waiting for error event when match not found")
	}
}

func TestRedisMatchmakerAdapter_FindByIDSuccessNotifiesClient(t *testing.T) {
	ctx := context.Background()
	mr := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	now := time.Now().UTC()
	p1, _ := entity.NewMatchParticipant("p_1", "m_ok_1", "u_1")
	p2, _ := entity.NewMatchParticipant("p_2", "m_ok_1", "u_2")
	expectedMatch, err := aggregate.NewSoloMatch(
		"m_ok_1",
		"ex_1",
		"Push-Up",
		valueobject.ChallengeTypeReps,
		[]*entity.MatchParticipant{p1, p2},
		now,
	)
	if err != nil {
		t.Fatalf("NewSoloMatch err: %v", err)
	}

	repo := &mockSimpleMatchRepo{
		matchToReturn: expectedMatch,
	}

	adapter := matchmaker.NewRedisMatchmakerAdapter(
		redisClient,
		repo,
		nil,
		mockTestExerciseProvider{},
		nil,
		10*time.Millisecond,
		1*time.Second,
	)
	defer adapter.Close()

	listenCtx, listenCancel := context.WithTimeout(ctx, 2*time.Second)
	defer listenCancel()

	listenCh, err := adapter.ListenMatch(listenCtx, "t_ok")
	if err != nil {
		t.Fatalf("ListenMatch err: %v", err)
	}

	_ = redisClient.Publish(ctx, matchmaker.RedisEventPrefix+"t_ok", "m_ok_1")

	select {
	case ev := <-listenCh:
		if ev == nil {
			t.Fatalf("expected non-nil event")
		}
		if ev.Err != nil {
			t.Fatalf("got unexpected err: %v", ev.Err)
		}
		if ev.Match == nil || ev.MatchID != "m_ok_1" {
			t.Fatalf("got matchID %q, want %q", ev.MatchID, "m_ok_1")
		}
	case <-time.After(1 * time.Second):
		t.Fatalf("timeout waiting for match event")
	}
}

func TestRedisMatchmakerAdapter_PreventDuplicateUserAndSelfMatch(t *testing.T) {
	ctx := context.Background()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	db := setupTestDB(t)
	matchRepo := persistence.NewGORMSoloMatchRepository(db)
	botRepo := persistence.NewGORMBotRepository(db)
	evtPub := &mockEventPublisher{}

	adapter := matchmaker.NewRedisMatchmakerAdapter(
		rdb,
		matchRepo,
		botRepo,
		mockTestExerciseProvider{},
		evtPub,
		20*time.Millisecond,
		5*time.Second,
	)
	defer adapter.Close()

	now := time.Now().UTC()

	// 1. Kiểm tra IsUserInQueue ban đầu trả về false
	inQueue, err := adapter.IsUserInQueue(ctx, "user_dup")
	if err != nil {
		t.Fatalf("IsUserInQueue err: %v", err)
	}
	if inQueue {
		t.Errorf("expected user_dup not in queue, got inQueue=true")
	}

	// 2. Enqueue vé thứ nhất thành công
	t1, _ := aggregate.NewMatchmakingTicket("t_dup_1", "user_dup", now)
	err = adapter.Enqueue(ctx, t1)
	if err != nil {
		t.Fatalf("Enqueue t1 err: %v", err)
	}

	// 3. IsUserInQueue giờ phải trả về true
	inQueue, err = adapter.IsUserInQueue(ctx, "user_dup")
	if err != nil {
		t.Fatalf("IsUserInQueue err: %v", err)
	}
	if !inQueue {
		t.Errorf("expected user_dup in queue, got inQueue=false")
	}

	// 4. Enqueue vé thứ 2 của cùng user phải bị từ chối với ErrUserAlreadyInQueue
	t2, _ := aggregate.NewMatchmakingTicket("t_dup_2", "user_dup", now.Add(time.Second))
	err = adapter.Enqueue(ctx, t2)
	if !errors.Is(err, derror.ErrUserAlreadyInQueue) {
		t.Fatalf("got err: %v, want ErrUserAlreadyInQueue", err)
	}

	// Hủy vé t_dup_1 để hàng đợi trống trước khi test bước tiếp theo
	_ = adapter.Cancel(ctx, "t_dup_1", "user_dup")
	inQueue, _ = adapter.IsUserInQueue(ctx, "user_dup")
	if inQueue {
		t.Fatalf("expected user_dup not in queue after cancel")
	}

	// 5. Kiểm tra matchPairs không tự ghép 2 vé của cùng 1 user
	// Giả lập đưa 2 vé của cùng 1 user_same vào Redis queue
	_ = rdb.ZAdd(ctx, matchmaker.RedisQueueKey, redis.Z{
		Score:  float64(now.Unix()),
		Member: "t_raw_1:user_same",
	}).Err()
	_ = rdb.ZAdd(ctx, matchmaker.RedisQueueKey, redis.Z{
		Score:  float64(now.Unix() + 1),
		Member: "t_raw_2:user_same",
	}).Err()

	// Chờ xem matchPairs có ghép đôi 2 vé này không (không được ghép vì cùng user_same!)
	time.Sleep(60 * time.Millisecond)

	count, _ := rdb.ZCard(ctx, matchmaker.RedisQueueKey).Result()
	if count != 2 {
		t.Errorf("expected 2 tickets remaining in queue (no self-matching), got %d", count)
	}

	// 6. Thêm vé của đối thủ khác (user_other)
	listenCtx, listenCancel := context.WithTimeout(ctx, 3*time.Second)
	defer listenCancel()

	listenCh, err := adapter.ListenMatch(listenCtx, "t_raw_1")
	if err != nil {
		t.Fatalf("ListenMatch err: %v", err)
	}

	tOther, _ := aggregate.NewMatchmakingTicket("t_other_1", "user_other", now.Add(2*time.Second))
	_ = adapter.Enqueue(ctx, tOther)

	select {
	case ev := <-listenCh:
		if ev == nil || ev.Match == nil {
			t.Fatalf("expected valid match event, got nil")
		}
		participants := ev.Match.Participants()
		if len(participants) != 2 {
			t.Fatalf("expected 2 participants, got %d", len(participants))
		}
		if participants[0].UserID() == participants[1].UserID() {
			t.Fatalf("participants have identical user id: %s", participants[0].UserID())
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timeout waiting for valid match between different users")
	}
}
