package persistence_test

import (
	"context"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/viethung213/gym-companion/internal/solo_match/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/solo_match/domain/entity"
	"github.com/viethung213/gym-companion/internal/solo_match/domain/service"
	valueobject "github.com/viethung213/gym-companion/internal/solo_match/domain/value_object"
	"github.com/viethung213/gym-companion/internal/solo_match/infrastructure/persistence"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupSoloMatchTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:solo_match_repo_test?mode=memory&cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql.DB: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)

	_ = db.Exec("ATTACH DATABASE ':memory:' AS solo_match;").Error

	schemaSQL := `
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
	`
	if err := db.Exec(schemaSQL).Error; err != nil {
		t.Fatalf("create solo_match test schema: %v", err)
	}

	// Clean tables before tests
	_ = db.Exec("DELETE FROM solo_match.match_participants;").Error
	_ = db.Exec("DELETE FROM solo_match.solo_matches;").Error

	return db
}

func TestGORMSoloMatchRepository_SaveAndFindByID(t *testing.T) {
	db := setupSoloMatchTestDB(t)
	repo := persistence.NewGORMSoloMatchRepository(db)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	p1, err := entity.NewMatchParticipant("p_1", "m_1", "u_1")
	if err != nil {
		t.Fatalf("create p1: %v", err)
	}
	p2, err := entity.NewMatchParticipant("p_2", "m_1", "u_2")
	if err != nil {
		t.Fatalf("create p2: %v", err)
	}

	match, err := aggregate.NewSoloMatch(
		"m_1", "ex_1", "Push-ups", valueobject.ChallengeTypeReps,
		[]*entity.MatchParticipant{p1, p2}, now,
	)
	if err != nil {
		t.Fatalf("create match: %v", err)
	}
	_ = match.ActivateRoom(now)

	// 1. Save new match
	err = repo.Save(ctx, match)
	if err != nil {
		t.Fatalf("repo.Save failed: %v", err)
	}

	// 2. FindByID existing match
	found, err := repo.FindByID(ctx, "m_1")
	if err != nil {
		t.Fatalf("repo.FindByID failed: %v", err)
	}
	if found == nil {
		t.Fatalf("expected match m_1 to be found")
	}
	if found.ID() != "m_1" {
		t.Errorf("got ID %s, want m_1", found.ID())
	}
	if found.ExerciseID() != "ex_1" || found.ExerciseName() != "Push-ups" {
		t.Errorf("exercise mismatch: %s / %s", found.ExerciseID(), found.ExerciseName())
	}
	if found.ChallengeType() != valueobject.ChallengeTypeReps {
		t.Errorf("challenge type mismatch: %v", found.ChallengeType())
	}
	if found.Status() != valueobject.MatchStatusInProgress {
		t.Errorf("status got %v, want IN_PROGRESS", found.Status())
	}
	if len(found.Participants()) != 2 {
		t.Fatalf("got %d participants, want 2", len(found.Participants()))
	}

	// 3. FindByID non-existent match
	notFound, err := repo.FindByID(ctx, "m_nonexistent")
	if err != nil {
		t.Fatalf("unexpected err for non-existent match: %v", err)
	}
	if notFound != nil {
		t.Errorf("expected nil for non-existent match, got: %v", notFound)
	}
}

func TestGORMSoloMatchRepository_UpdateOnConflict(t *testing.T) {
	db := setupSoloMatchTestDB(t)
	repo := persistence.NewGORMSoloMatchRepository(db)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	p1, _ := entity.NewMatchParticipant("p_10", "m_10", "u_10")
	p2, _ := entity.NewMatchParticipant("p_20", "m_10", "u_20")
	match, err := aggregate.NewSoloMatch(
		"m_10", "ex_pullup", "Pull-ups", valueobject.ChallengeTypeReps,
		[]*entity.MatchParticipant{p1, p2}, now,
	)
	if err != nil {
		t.Fatalf("create match: %v", err)
	}
	// 1. Lưu ban đầu khi vừa tạo (PENDING_READY, startedAt là nil)
	err = repo.Save(ctx, match)
	if err != nil {
		t.Fatalf("initial save: %v", err)
	}

	initMatch, err := repo.FindByID(ctx, "m_10")
	if err != nil || initMatch == nil {
		t.Fatalf("find initial match err: %v", err)
	}
	if initMatch.StartedAt() != nil {
		t.Errorf("expected initial startedAt to be nil, got %v", initMatch.StartedAt())
	}

	// 2. Kích hoạt phòng thi đấu (IN_PROGRESS, startedAt được gán)
	_ = match.ActivateRoom(now)
	err = repo.Save(ctx, match)
	if err != nil {
		t.Fatalf("save activated match: %v", err)
	}

	activeMatch, err := repo.FindByID(ctx, "m_10")
	if err != nil || activeMatch == nil {
		t.Fatalf("find active match err: %v", err)
	}
	if activeMatch.Status() != valueobject.MatchStatusInProgress {
		t.Errorf("status got %v, want IN_PROGRESS", activeMatch.Status())
	}
	if activeMatch.StartedAt() == nil {
		t.Fatalf("expected startedAt to be updated on conflict, got nil")
	}

	// 3. Cập nhật kết quả thi đấu: p1 thắng
	resolver := service.NewMatchResolver()
	_ = match.RecordParticipantResult("u_10", 30, now.Add(2*time.Minute), resolver)
	_ = match.RecordParticipantResult("u_20", 20, now.Add(3*time.Minute), resolver)

	// Lưu lại (OnConflict Update sang COMPLETED)
	err = repo.Save(ctx, match)
	if err != nil {
		t.Fatalf("save updated match: %v", err)
	}

	reloaded, err := repo.FindByID(ctx, "m_10")
	if err != nil || reloaded == nil {
		t.Fatalf("find updated match err: %v", err)
	}
	if reloaded.Status() != valueobject.MatchStatusCompleted {
		t.Errorf("status got %v, want COMPLETED", reloaded.Status())
	}
	if reloaded.WinnerID() == nil || *reloaded.WinnerID() != "u_10" {
		t.Errorf("winnerID got %v, want u_10", reloaded.WinnerID())
	}
	if reloaded.EndedAt() == nil {
		t.Errorf("expected ended_at to be populated")
	}

	for _, p := range reloaded.Participants() {
		if p.UserID() == "u_10" {
			if p.Result() != valueobject.MatchResultWin {
				t.Errorf("u_10 result got %v, want WIN", p.Result())
			}
			if p.Score().Value() != 30 {
				t.Errorf("u_10 score got %v, want 30", p.Score().Value())
			}
		}
		if p.UserID() == "u_20" {
			if p.Result() != valueobject.MatchResultLose {
				t.Errorf("u_20 result got %v, want LOSE", p.Result())
			}
			if p.Score().Value() != 20 {
				t.Errorf("u_20 score got %v, want 20", p.Score().Value())
			}
		}
	}
}

func TestGORMSoloMatchRepository_FindActiveByUserID(t *testing.T) {
	db := setupSoloMatchTestDB(t)
	repo := persistence.NewGORMSoloMatchRepository(db)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	p1, _ := entity.NewMatchParticipant("p_active_1", "m_active", "u_user_active")
	p2, _ := entity.NewMatchParticipant("p_active_2", "m_active", "u_opponent")
	matchActive, _ := aggregate.NewSoloMatch(
		"m_active", "ex_1", "Plank", valueobject.ChallengeTypeDuration,
		[]*entity.MatchParticipant{p1, p2}, now,
	)
	_ = matchActive.ActivateRoom(now)
	_ = repo.Save(ctx, matchActive)

	t.Run("returns active match for participant", func(t *testing.T) {
		found, err := repo.FindActiveByUserID(ctx, "u_user_active")
		if err != nil {
			t.Fatalf("find active match: %v", err)
		}
		if found == nil || found.ID() != "m_active" {
			t.Fatalf("expected m_active, got %v", found)
		}
	})

	t.Run("returns nil when user has no active match", func(t *testing.T) {
		found, err := repo.FindActiveByUserID(ctx, "u_inactive_user")
		if err != nil {
			t.Fatalf("find active match: %v", err)
		}
		if found != nil {
			t.Fatalf("expected nil for inactive user, got %v", found)
		}
	})

	t.Run("returns nil when match is already completed", func(t *testing.T) {
		res := service.NewMatchResolver()
		_ = matchActive.RecordParticipantResult("u_user_active", 60, now.Add(time.Minute), res)
		_ = matchActive.RecordParticipantResult("u_opponent", 45, now.Add(time.Minute), res)
		_ = repo.Save(ctx, matchActive)

		found, err := repo.FindActiveByUserID(ctx, "u_user_active")
		if err != nil {
			t.Fatalf("find active match err: %v", err)
		}
		if found != nil {
			t.Fatalf("expected nil when match completed, got %v", found)
		}
	})
}

func TestGORMSoloMatchRepository_FindUnreadyExpiredMatches(t *testing.T) {
	db := setupSoloMatchTestDB(t)
	repo := persistence.NewGORMSoloMatchRepository(db)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	pUnready1, _ := entity.NewMatchParticipant("pu_1", "m_unready_exp", "u9")
	pUnready2, _ := entity.NewMatchParticipant("pu_2", "m_unready_exp", "u10")
	mUnready, _ := aggregate.NewSoloMatch("m_unready_exp", "ex_1", "Squats", valueobject.ChallengeTypeReps, []*entity.MatchParticipant{pUnready1, pUnready2}, now.Add(-2*time.Minute))
	// stays PENDING_READY, ready_deadline is now - 1m
	_ = repo.Save(ctx, mUnready)

	unreadyMatches, err := repo.FindUnreadyExpiredMatches(ctx, now, 10)
	if err != nil {
		t.Fatalf("find unready expired matches: %v", err)
	}
	if len(unreadyMatches) != 1 {
		t.Fatalf("got %d unready matches, want 1", len(unreadyMatches))
	}
	if unreadyMatches[0].ID() != "m_unready_exp" {
		t.Errorf("got %s, want m_unready_exp", unreadyMatches[0].ID())
	}
}

func TestGORMSoloMatchRepository_FindExpiredInProgressMatches(t *testing.T) {
	db := setupSoloMatchTestDB(t)
	repo := persistence.NewGORMSoloMatchRepository(db)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	// Match 1: IN_PROGRESS started 40 minutes ago (expired for 35m threshold)
	p1A, _ := entity.NewMatchParticipant("p_inprog_1a", "m_inprog_exp", "u21")
	p1B, _ := entity.NewMatchParticipant("p_inprog_1b", "m_inprog_exp", "u22")
	mExpired, _ := aggregate.NewSoloMatch("m_inprog_exp", "ex_1", "Push-ups", valueobject.ChallengeTypeReps, []*entity.MatchParticipant{p1A, p1B}, now.Add(-50*time.Minute))
	_ = mExpired.ActivateRoom(now.Add(-40 * time.Minute))
	_ = repo.Save(ctx, mExpired)

	// Match 2: IN_PROGRESS started 10 minutes ago (not expired)
	p2A, _ := entity.NewMatchParticipant("p_inprog_2a", "m_inprog_fresh", "u23")
	p2B, _ := entity.NewMatchParticipant("p_inprog_2b", "m_inprog_fresh", "u24")
	mFresh, _ := aggregate.NewSoloMatch("m_inprog_fresh", "ex_1", "Push-ups", valueobject.ChallengeTypeReps, []*entity.MatchParticipant{p2A, p2B}, now.Add(-15*time.Minute))
	_ = mFresh.ActivateRoom(now.Add(-10 * time.Minute))
	_ = repo.Save(ctx, mFresh)

	// Match 3: PENDING_READY (should not match)
	p3A, _ := entity.NewMatchParticipant("p_inprog_3a", "m_unready_other", "u25")
	p3B, _ := entity.NewMatchParticipant("p_inprog_3b", "m_unready_other", "u26")
	mPending, _ := aggregate.NewSoloMatch("m_unready_other", "ex_1", "Push-ups", valueobject.ChallengeTypeReps, []*entity.MatchParticipant{p3A, p3B}, now.Add(-50*time.Minute))
	_ = repo.Save(ctx, mPending)

	startedBefore := now.Add(-35 * time.Minute)
	expiredMatches, err := repo.FindExpiredInProgressMatches(ctx, startedBefore, 10)
	if err != nil {
		t.Fatalf("find expired in-progress matches: %v", err)
	}
	if len(expiredMatches) != 1 {
		t.Fatalf("got %d expired matches, want 1", len(expiredMatches))
	}
	if expiredMatches[0].ID() != "m_inprog_exp" {
		t.Errorf("got %s, want m_inprog_exp", expiredMatches[0].ID())
	}
}

func TestGORMSoloMatchRepository_ListByUserID(t *testing.T) {
	db := setupSoloMatchTestDB(t)
	repo := persistence.NewGORMSoloMatchRepository(db)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	// Create 3 matches for u_list_1
	for i := 1; i <= 3; i++ {
		pA, _ := entity.NewMatchParticipant("p_l_a_"+string(rune('0'+i)), "m_list_"+string(rune('0'+i)), "u_list_1")
		pB, _ := entity.NewMatchParticipant("p_l_b_"+string(rune('0'+i)), "m_list_"+string(rune('0'+i)), "u_other")
		m, _ := aggregate.NewSoloMatch(
			"m_list_"+string(rune('0'+i)), "ex_1", "Push-ups", valueobject.ChallengeTypeReps,
			[]*entity.MatchParticipant{pA, pB}, now.Add(time.Duration(i)*time.Hour),
		)
		_ = repo.Save(ctx, m)
		_ = db.Exec("UPDATE solo_match.solo_matches SET created_at = ? WHERE id = ?", now.Add(time.Duration(i)*time.Minute), m.ID()).Error
	}

	t.Run("returns all matches for user ordered created_at DESC", func(t *testing.T) {
		matches, err := repo.ListByUserID(ctx, "u_list_1", 0, 10)
		if err != nil {
			t.Fatalf("list matches: %v", err)
		}
		if len(matches) != 3 {
			t.Fatalf("got %d matches, want 3", len(matches))
		}
		// m_list_3 is created latest
		if matches[0].ID() != "m_list_3" {
			t.Errorf("first match got %s, want m_list_3", matches[0].ID())
		}
		if matches[2].ID() != "m_list_1" {
			t.Errorf("last match got %s, want m_list_1", matches[2].ID())
		}
	})

	t.Run("supports pagination with offset and limit", func(t *testing.T) {
		page1, err := repo.ListByUserID(ctx, "u_list_1", 0, 2)
		if err != nil || len(page1) != 2 {
			t.Fatalf("page 1 err: %v, len: %d", err, len(page1))
		}
		if page1[0].ID() != "m_list_3" || page1[1].ID() != "m_list_2" {
			t.Errorf("page 1 elements unexpected: %s, %s", page1[0].ID(), page1[1].ID())
		}

		page2, err := repo.ListByUserID(ctx, "u_list_1", 2, 2)
		if err != nil || len(page2) != 1 {
			t.Fatalf("page 2 err: %v, len: %d", err, len(page2))
		}
		if page2[0].ID() != "m_list_1" {
			t.Errorf("page 2 elements unexpected: %s", page2[0].ID())
		}
	})

	t.Run("returns empty slice when user has no matches", func(t *testing.T) {
		emptyList, err := repo.ListByUserID(ctx, "u_no_history", 0, 10)
		if err != nil {
			t.Fatalf("list matches for stranger err: %v", err)
		}
		if len(emptyList) != 0 {
			t.Errorf("expected 0 matches, got %d", len(emptyList))
		}
	})
}
