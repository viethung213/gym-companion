package query_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/solo_match/application/query"
	"github.com/viethung213/gym-companion/internal/solo_match/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/solo_match/domain/entity"
	valueobject "github.com/viethung213/gym-companion/internal/solo_match/domain/value_object"
)

type soloMatchRepoMock struct {
	match *aggregate.SoloMatch
	err   error
}

func (m *soloMatchRepoMock) Save(ctx context.Context, match *aggregate.SoloMatch) error {
	return nil
}

func (m *soloMatchRepoMock) FindByID(ctx context.Context, id string) (*aggregate.SoloMatch, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.match, nil
}

func (m *soloMatchRepoMock) FindActiveByUserID(ctx context.Context, userID string) (*aggregate.SoloMatch, error) {
	return nil, nil
}

func (m *soloMatchRepoMock) FindUnreadyExpiredMatches(ctx context.Context, now time.Time, limit int) ([]*aggregate.SoloMatch, error) {
	return nil, nil
}

func (m *soloMatchRepoMock) FindExpiredInProgressMatches(ctx context.Context, startedBefore time.Time, limit int) ([]*aggregate.SoloMatch, error) {
	return nil, nil
}

func (m *soloMatchRepoMock) ListByUserID(ctx context.Context, userID string, offset, limit int) ([]*aggregate.SoloMatch, error) {
	return nil, nil
}

type soloSnapshotRepoMock struct {
	snapshots map[string]*entity.UserSnapshot
}

func (s *soloSnapshotRepoMock) Upsert(ctx context.Context, snap *entity.UserSnapshot) error {
	return nil
}

func (s *soloSnapshotRepoMock) FindByUserID(ctx context.Context, userID string) (*entity.UserSnapshot, error) {
	return s.snapshots[userID], nil
}

func (s *soloSnapshotRepoMock) FindByUserIDs(ctx context.Context, userIDs []string) (map[string]*entity.UserSnapshot, error) {
	res := make(map[string]*entity.UserSnapshot)
	for _, id := range userIDs {
		if snap, ok := s.snapshots[id]; ok {
			res[id] = snap
		}
	}
	return res, nil
}

func TestGetSoloMatchByIDHandler(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()

	t.Run("successfully retrieves match by ID", func(t *testing.T) {
		p1, _ := entity.NewMatchParticipant("p_1", "m_100", "user_1")
		p2, _ := entity.NewMatchParticipant("p_2", "m_100", "user_2")
		match, _ := aggregate.NewSoloMatch(
			"m_100",
			"ex_pushup",
			"Push-Up",
			valueobject.ChallengeTypeReps,
			[]*entity.MatchParticipant{p1, p2},
			now,
		)

		snap1, _ := entity.NewUserSnapshot("user_1", "Alice", "https://avatar/1", now)
		snapRepo := &soloSnapshotRepoMock{
			snapshots: map[string]*entity.UserSnapshot{
				"user_1": snap1,
			},
		}

		matchRepo := &soloMatchRepoMock{match: match}
		handler := query.NewGetSoloMatchByIDHandler(matchRepo, snapRepo)

		dto, err := handler.Handle(ctx, query.GetSoloMatchByIDQuery{MatchID: "m_100"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if dto == nil {
			t.Fatal("expected match DTO to be returned, got nil")
		}
		if dto.MatchID != "m_100" {
			t.Errorf("expected match ID m_100, got %s", dto.MatchID)
		}
		if dto.ExerciseName != "Push-Up" {
			t.Errorf("expected Push-Up, got %s", dto.ExerciseName)
		}
		if len(dto.Participants) != 2 {
			t.Errorf("expected 2 participants, got %d", len(dto.Participants))
		}
	})

	t.Run("returns nil when match not found", func(t *testing.T) {
		matchRepo := &soloMatchRepoMock{match: nil}
		snapRepo := &soloSnapshotRepoMock{snapshots: make(map[string]*entity.UserSnapshot)}
		handler := query.NewGetSoloMatchByIDHandler(matchRepo, snapRepo)

		dto, err := handler.Handle(ctx, query.GetSoloMatchByIDQuery{MatchID: "non_existent"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if dto != nil {
			t.Errorf("expected nil DTO for non-existent match, got %+v", dto)
		}
	})

	t.Run("fails when repository returns error", func(t *testing.T) {
		matchRepo := &soloMatchRepoMock{err: errors.New("database timeout")}
		snapRepo := &soloSnapshotRepoMock{snapshots: make(map[string]*entity.UserSnapshot)}
		handler := query.NewGetSoloMatchByIDHandler(matchRepo, snapRepo)

		_, err := handler.Handle(ctx, query.GetSoloMatchByIDQuery{MatchID: "m_err"})
		if err == nil {
			t.Errorf("expected error when repo fails, got nil")
		}
	})
}
