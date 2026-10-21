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
		10*time.Minute, []*entity.MatchParticipant{p1, p2}, now,
	)
	if err != nil {
		t.Fatalf("create match: %v", err)
	}

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
		15*time.Minute, []*entity.MatchParticipant{p1, p2}, now,
	)
	if err != nil {
		t.Fatalf("create match: %v", err)
	}

	err = repo.Save(ctx, match)
	if err != nil {
		t.Fatalf("initial save: %v", err)
	}

	// Cập nhật kết quả thi đấu: p1 thắng
	resolver := service.NewMatchResolver()
	_ = match.RecordParticipantResult("u_10", 30, now.Add(2*time.Minute), resolver)
	_ = match.RecordParticipantResult("u_20", 20, now.Add(3*time.Minute), resolver)

	// Lưu lại (OnConflict Update)
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
		} else if p.UserID() == "u_20" {
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
		5*time.Minute, []*entity.MatchParticipant{p1, p2}, now,
	)
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

func TestGORMSoloMatchRepository_FindExpiredMatches(t *testing.T) {
	db := setupSoloMatchTestDB(t)
	repo := persistence.NewGORMSoloMatchRepository(db)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	// Match 1: Expired earlier (timeout_at = now - 10m)
	p1_1, _ := entity.NewMatchParticipant("p1_1", "m_exp_1", "u1")
	p1_2, _ := entity.NewMatchParticipant("p1_2", "m_exp_1", "u2")
	m1, _ := aggregate.NewSoloMatch("m_exp_1", "ex_1", "Squats", valueobject.ChallengeTypeReps, 5*time.Minute, []*entity.MatchParticipant{p1_1, p1_2}, now.Add(-15*time.Minute))
	_ = repo.Save(ctx, m1)

	// Match 2: Expired later (timeout_at = now - 2m)
	p2_1, _ := entity.NewMatchParticipant("p2_1", "m_exp_2", "u3")
	p2_2, _ := entity.NewMatchParticipant("p2_2", "m_exp_2", "u4")
	m2, _ := aggregate.NewSoloMatch("m_exp_2", "ex_1", "Squats", valueobject.ChallengeTypeReps, 5*time.Minute, []*entity.MatchParticipant{p2_1, p2_2}, now.Add(-7*time.Minute))
	_ = repo.Save(ctx, m2)

	// Match 3: Future timeout (timeout_at = now + 10m) -> Should not be expired
	p3_1, _ := entity.NewMatchParticipant("p3_1", "m_future", "u5")
	p3_2, _ := entity.NewMatchParticipant("p3_2", "m_future", "u6")
	m3, _ := aggregate.NewSoloMatch("m_future", "ex_1", "Squats", valueobject.ChallengeTypeReps, 15*time.Minute, []*entity.MatchParticipant{p3_1, p3_2}, now.Add(-5*time.Minute))
	_ = repo.Save(ctx, m3)

	// Match 4: Already completed -> Should not be returned even if timeout_at <= now
	p4_1, _ := entity.NewMatchParticipant("p4_1", "m_exp_completed", "u7")
	p4_2, _ := entity.NewMatchParticipant("p4_2", "m_exp_completed", "u8")
	m4, _ := aggregate.NewSoloMatch("m_exp_completed", "ex_1", "Squats", valueobject.ChallengeTypeReps, 5*time.Minute, []*entity.MatchParticipant{p4_1, p4_2}, now.Add(-20*time.Minute))
	res := service.NewMatchResolver()
	_ = m4.RecordParticipantResult("u7", 10, now.Add(-18*time.Minute), res)
	_ = m4.RecordParticipantResult("u8", 8, now.Add(-18*time.Minute), res)
	_ = repo.Save(ctx, m4)

	t.Run("finds expired matches sorted by timeout_at ASC", func(t *testing.T) {
		expired, err := repo.FindExpiredMatches(ctx, now, 10)
		if err != nil {
			t.Fatalf("find expired matches: %v", err)
		}
		if len(expired) != 2 {
			t.Fatalf("got %d expired matches, want 2", len(expired))
		}
		if expired[0].ID() != "m_exp_1" {
			t.Errorf("first match got %s, want m_exp_1 (earlier timeout)", expired[0].ID())
		}
		if expired[1].ID() != "m_exp_2" {
			t.Errorf("second match got %s, want m_exp_2", expired[1].ID())
		}
	})

	t.Run("respects limit", func(t *testing.T) {
		limited, err := repo.FindExpiredMatches(ctx, now, 1)
		if err != nil {
			t.Fatalf("find expired matches with limit 1: %v", err)
		}
		if len(limited) != 1 {
			t.Fatalf("got %d matches, want 1", len(limited))
		}
		if limited[0].ID() != "m_exp_1" {
			t.Errorf("got %s, want m_exp_1", limited[0].ID())
		}
	})
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
			10*time.Minute, []*entity.MatchParticipant{pA, pB}, now.Add(time.Duration(i)*time.Hour),
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
