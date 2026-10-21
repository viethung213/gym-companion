package command_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/solo_match/application/command"
	"github.com/viethung213/gym-companion/internal/solo_match/application/port"
	"github.com/viethung213/gym-companion/internal/solo_match/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/solo_match/domain/derror"
	"github.com/viethung213/gym-companion/internal/solo_match/domain/entity"
	valueobject "github.com/viethung213/gym-companion/internal/solo_match/domain/value_object"
)

type joinQueuePortMock struct {
	enqueued []*aggregate.MatchmakingTicket
}

func (q *joinQueuePortMock) Enqueue(ctx context.Context, ticket *aggregate.MatchmakingTicket) error {
	q.enqueued = append(q.enqueued, ticket)
	return nil
}

func (q *joinQueuePortMock) Cancel(ctx context.Context, ticketID, userID string) error {
	return nil
}

func (q *joinQueuePortMock) ListenMatch(ctx context.Context, ticketID string) (<-chan *port.MatchFoundEvent, error) {
	return nil, nil
}

func (q *joinQueuePortMock) Close() error {
	return nil
}

type joinMatchRepoMock struct {
	activeMatch *aggregate.SoloMatch
	err         error
}

func (m *joinMatchRepoMock) Save(ctx context.Context, match *aggregate.SoloMatch) error {
	return nil
}

func (m *joinMatchRepoMock) FindByID(ctx context.Context, id string) (*aggregate.SoloMatch, error) {
	return nil, nil
}

func (m *joinMatchRepoMock) FindActiveByUserID(ctx context.Context, userID string) (*aggregate.SoloMatch, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.activeMatch, nil
}

func (m *joinMatchRepoMock) FindExpiredMatches(ctx context.Context, now time.Time, limit int) ([]*aggregate.SoloMatch, error) {
	return nil, nil
}

func (m *joinMatchRepoMock) ListByUserID(ctx context.Context, userID string, offset, limit int) ([]*aggregate.SoloMatch, error) {
	return nil, nil
}

func TestJoinMatchmakingQueueHandler(t *testing.T) {
	ctx := context.Background()

	t.Run("successfully enqueues user when no active match exists", func(t *testing.T) {
		queuePort := &joinQueuePortMock{enqueued: make([]*aggregate.MatchmakingTicket, 0)}
		matchRepo := &joinMatchRepoMock{}
		handler := command.NewJoinMatchmakingQueueHandler(queuePort, matchRepo)

		cmd := command.JoinMatchmakingQueueCommand{
			UserID: "user_player_1",
		}

		res, err := handler.Handle(ctx, cmd)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.TicketID == "" {
			t.Errorf("expected non-empty ticket ID, got empty")
		}
		if len(queuePort.enqueued) != 1 {
			t.Errorf("expected 1 enqueued ticket, got %d", len(queuePort.enqueued))
		}
		if queuePort.enqueued[0].UserID() != "user_player_1" {
			t.Errorf("expected user ID user_player_1, got %s", queuePort.enqueued[0].UserID())
		}
	})

	t.Run("fails when user already has an active match", func(t *testing.T) {
		p1, _ := entity.NewMatchParticipant("p_1", "m_1", "user_player_1")
		p2, _ := entity.NewMatchParticipant("p_2", "m_1", "user_player_2")
		activeMatch, _ := aggregate.NewSoloMatch("m_1", "ex_1", "Push Up", valueobject.ChallengeTypeReps, 15*time.Minute, []*entity.MatchParticipant{p1, p2}, time.Now().UTC())

		queuePort := &joinQueuePortMock{enqueued: make([]*aggregate.MatchmakingTicket, 0)}
		matchRepo := &joinMatchRepoMock{activeMatch: activeMatch}
		handler := command.NewJoinMatchmakingQueueHandler(queuePort, matchRepo)

		cmd := command.JoinMatchmakingQueueCommand{
			UserID: "user_player_1",
		}

		_, err := handler.Handle(ctx, cmd)
		if !errors.Is(err, derror.ErrActiveMatchAlreadyExists) {
			t.Errorf("expected ErrActiveMatchAlreadyExists, got %v", err)
		}
		if len(queuePort.enqueued) != 0 {
			t.Errorf("expected 0 tickets enqueued, got %d", len(queuePort.enqueued))
		}
	})

	t.Run("fails when checking active match returns error", func(t *testing.T) {
		queuePort := &joinQueuePortMock{enqueued: make([]*aggregate.MatchmakingTicket, 0)}
		matchRepo := &joinMatchRepoMock{err: errors.New("db error")}
		handler := command.NewJoinMatchmakingQueueHandler(queuePort, matchRepo)

		cmd := command.JoinMatchmakingQueueCommand{
			UserID: "user_player_1",
		}

		_, err := handler.Handle(ctx, cmd)
		if err == nil {
			t.Errorf("expected error, got nil")
		}
	})

	t.Run("fails when user ID is empty", func(t *testing.T) {
		queuePort := &joinQueuePortMock{enqueued: make([]*aggregate.MatchmakingTicket, 0)}
		matchRepo := &joinMatchRepoMock{}
		handler := command.NewJoinMatchmakingQueueHandler(queuePort, matchRepo)

		cmd := command.JoinMatchmakingQueueCommand{
			UserID: "",
		}

		_, err := handler.Handle(ctx, cmd)
		if err == nil {
			t.Errorf("expected error for empty user ID, got nil")
		}
	})
}
