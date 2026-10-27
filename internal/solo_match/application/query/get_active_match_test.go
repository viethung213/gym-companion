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

type activeMatchRepoMock struct {
	activeMatch *aggregate.SoloMatch
	err         error
}

func (m *activeMatchRepoMock) Save(ctx context.Context, match *aggregate.SoloMatch) error {
	return nil
}

func (m *activeMatchRepoMock) FindByID(ctx context.Context, id string) (*aggregate.SoloMatch, error) {
	return nil, nil
}

func (m *activeMatchRepoMock) FindActiveByUserID(ctx context.Context, userID string) (*aggregate.SoloMatch, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.activeMatch, nil
}

func (m *activeMatchRepoMock) FindUnreadyExpiredMatches(ctx context.Context, now time.Time, limit int) ([]*aggregate.SoloMatch, error) {
	return nil, nil
}

func (m *activeMatchRepoMock) FindExpiredInProgressMatches(ctx context.Context, startedBefore time.Time, limit int) ([]*aggregate.SoloMatch, error) {
	return nil, nil
}

func (m *activeMatchRepoMock) ListByUserID(ctx context.Context, userID string, offset, limit int) ([]*aggregate.SoloMatch, error) {
	return nil, nil
}

type activeSnapshotRepoMock struct {
	snapshots map[string]*entity.UserSnapshot
}

func (s *activeSnapshotRepoMock) Upsert(ctx context.Context, snap *entity.UserSnapshot) error {
	return nil
}

func (s *activeSnapshotRepoMock) FindByUserID(ctx context.Context, userID string) (*entity.UserSnapshot, error) {
	return s.snapshots[userID], nil
}

func (s *activeSnapshotRepoMock) FindByUserIDs(ctx context.Context, userIDs []string) (map[string]*entity.UserSnapshot, error) {
	res := make(map[string]*entity.UserSnapshot)
	for _, id := range userIDs {
		if snap, ok := s.snapshots[id]; ok {
			res[id] = snap
		}
	}
	return res, nil
}

func TestGetActiveMatchHandler(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()

	t.Run("successfully retrieves active match for user", func(t *testing.T) {
		p1, _ := entity.NewMatchParticipant("p_1", "m_active", "user_1")
		p2, _ := entity.NewMatchParticipant("p_2", "m_active", "user_2")
		match, _ := aggregate.NewSoloMatch(
			"m_active",
			"ex_squat",
			"Bodyweight Squat",
			valueobject.ChallengeTypeReps,
			[]*entity.MatchParticipant{p1, p2},
			now,
		)
		_ = match.ActivateRoom(now)

		matchRepo := &activeMatchRepoMock{activeMatch: match}
		snapRepo := &activeSnapshotRepoMock{snapshots: make(map[string]*entity.UserSnapshot)}
		handler := query.NewGetActiveMatchHandler(matchRepo, snapRepo)

		dto, err := handler.Handle(ctx, query.GetActiveMatchQuery{UserID: "user_1"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if dto == nil {
			t.Fatal("expected active match DTO to be returned, got nil")
		}
		if dto.MatchID != "m_active" {
			t.Errorf("expected match ID m_active, got %s", dto.MatchID)
		}
		if dto.Status != valueobject.MatchStatusInProgress.String() {
			t.Errorf("expected status IN_PROGRESS, got %s", dto.Status)
		}
	})

	t.Run("returns nil when user has no active match", func(t *testing.T) {
		matchRepo := &activeMatchRepoMock{activeMatch: nil}
		snapRepo := &activeSnapshotRepoMock{snapshots: make(map[string]*entity.UserSnapshot)}
		handler := query.NewGetActiveMatchHandler(matchRepo, snapRepo)

		dto, err := handler.Handle(ctx, query.GetActiveMatchQuery{UserID: "user_idle"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if dto != nil {
			t.Errorf("expected nil DTO for user without active match, got %+v", dto)
		}
	})

	t.Run("fails when repository returns error", func(t *testing.T) {
		matchRepo := &activeMatchRepoMock{err: errors.New("db query failed")}
		snapRepo := &activeSnapshotRepoMock{snapshots: make(map[string]*entity.UserSnapshot)}
		handler := query.NewGetActiveMatchHandler(matchRepo, snapRepo)

		_, err := handler.Handle(ctx, query.GetActiveMatchQuery{UserID: "user_err"})
		if err == nil {
			t.Errorf("expected error when repo fails, got nil")
		}
	})
}
