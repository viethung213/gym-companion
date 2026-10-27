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

type historyMatchRepoMock struct {
	matches []*aggregate.SoloMatch
	err     error
}

func (m *historyMatchRepoMock) Save(ctx context.Context, match *aggregate.SoloMatch) error {
	return nil
}

func (m *historyMatchRepoMock) FindByID(ctx context.Context, id string) (*aggregate.SoloMatch, error) {
	return nil, nil
}

func (m *historyMatchRepoMock) FindActiveByUserID(ctx context.Context, userID string) (*aggregate.SoloMatch, error) {
	return nil, nil
}

func (m *historyMatchRepoMock) FindUnreadyExpiredMatches(ctx context.Context, now time.Time, limit int) ([]*aggregate.SoloMatch, error) {
	return nil, nil
}

func (m *historyMatchRepoMock) FindExpiredInProgressMatches(ctx context.Context, startedBefore time.Time, limit int) ([]*aggregate.SoloMatch, error) {
	return nil, nil
}

func (m *historyMatchRepoMock) ListByUserID(ctx context.Context, userID string, offset, limit int) ([]*aggregate.SoloMatch, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.matches, nil
}

type historySnapshotRepoMock struct {
	snapshots map[string]*entity.UserSnapshot
}

func (s *historySnapshotRepoMock) Upsert(ctx context.Context, snap *entity.UserSnapshot) error {
	return nil
}

func (s *historySnapshotRepoMock) FindByUserID(ctx context.Context, userID string) (*entity.UserSnapshot, error) {
	return s.snapshots[userID], nil
}

func (s *historySnapshotRepoMock) FindByUserIDs(ctx context.Context, userIDs []string) (map[string]*entity.UserSnapshot, error) {
	res := make(map[string]*entity.UserSnapshot)
	for _, id := range userIDs {
		if snap, ok := s.snapshots[id]; ok {
			res[id] = snap
		}
	}
	return res, nil
}

func TestGetUserMatchHistoryHandler(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()

	t.Run("successfully retrieves user match history with pagination", func(t *testing.T) {
		p1, _ := entity.NewMatchParticipant("p_1", "m_hist_1", "user_1")
		p2, _ := entity.NewMatchParticipant("p_2", "m_hist_1", "user_2")
		match1, _ := aggregate.NewSoloMatch(
			"m_hist_1",
			"ex_pushup",
			"Push-Up",
			valueobject.ChallengeTypeReps,
			[]*entity.MatchParticipant{p1, p2},
			now,
		)

		matchRepo := &historyMatchRepoMock{matches: []*aggregate.SoloMatch{match1}}
		snapRepo := &historySnapshotRepoMock{snapshots: make(map[string]*entity.UserSnapshot)}
		handler := query.NewGetUserMatchHistoryHandler(matchRepo, snapRepo)

		// PageSize <= 0 defaults to 10
		items, err := handler.Handle(ctx, query.GetUserMatchHistoryQuery{
			UserID:   "user_1",
			PageSize: 0,
			Offset:   0,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(items) != 1 {
			t.Fatalf("expected 1 history item, got %d", len(items))
		}
		if items[0].MatchID != "m_hist_1" {
			t.Errorf("expected match ID m_hist_1, got %s", items[0].MatchID)
		}
	})

	t.Run("returns empty slice when user has no matches", func(t *testing.T) {
		matchRepo := &historyMatchRepoMock{matches: []*aggregate.SoloMatch{}}
		snapRepo := &historySnapshotRepoMock{snapshots: make(map[string]*entity.UserSnapshot)}
		handler := query.NewGetUserMatchHistoryHandler(matchRepo, snapRepo)

		items, err := handler.Handle(ctx, query.GetUserMatchHistoryQuery{
			UserID:   "user_empty",
			PageSize: 10,
			Offset:   0,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(items) != 0 {
			t.Errorf("expected empty slice, got %d items", len(items))
		}
	})

	t.Run("fails when repository returns error", func(t *testing.T) {
		matchRepo := &historyMatchRepoMock{err: errors.New("db error")}
		snapRepo := &historySnapshotRepoMock{snapshots: make(map[string]*entity.UserSnapshot)}
		handler := query.NewGetUserMatchHistoryHandler(matchRepo, snapRepo)

		_, err := handler.Handle(ctx, query.GetUserMatchHistoryQuery{
			UserID:   "user_err",
			PageSize: 10,
		})
		if err == nil {
			t.Errorf("expected error when repo fails, got nil")
		}
	})
}
