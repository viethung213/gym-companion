package command_test

import (
	"context"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/solo_match/application/command"
	"github.com/viethung213/gym-companion/internal/solo_match/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/solo_match/domain/entity"
	valueobject "github.com/viethung213/gym-companion/internal/solo_match/domain/value_object"
)

type timeoutMatchRepoMock struct {
	matches map[string]*aggregate.SoloMatch
}

func (m *timeoutMatchRepoMock) Save(ctx context.Context, match *aggregate.SoloMatch) error {
	m.matches[match.ID()] = match
	return nil
}

func (m *timeoutMatchRepoMock) FindByID(ctx context.Context, id string) (*aggregate.SoloMatch, error) {
	return m.matches[id], nil
}

func (m *timeoutMatchRepoMock) FindActiveByUserID(ctx context.Context, userID string) (*aggregate.SoloMatch, error) {
	return nil, nil
}

func (m *timeoutMatchRepoMock) FindExpiredMatches(ctx context.Context, now time.Time, limit int) ([]*aggregate.SoloMatch, error) {
	var res []*aggregate.SoloMatch
	for _, match := range m.matches {
		if match.Status() == valueobject.MatchStatusInProgress && now.After(match.TimeoutAt()) {
			res = append(res, match)
			if len(res) >= limit {
				break
			}
		}
	}
	return res, nil
}

func (m *timeoutMatchRepoMock) ListByUserID(ctx context.Context, userID string, offset, limit int) ([]*aggregate.SoloMatch, error) {
	return nil, nil
}

type timeoutEventPubMock struct {
	events []any
}

func (m *timeoutEventPubMock) PublishEvents(ctx context.Context, events []any) error {
	m.events = append(m.events, events...)
	return nil
}

type timeoutTxManagerMock struct{}

func (t *timeoutTxManagerMock) WithTransaction(ctx context.Context, fn func(txCtx context.Context) error) error {
	return fn(ctx)
}

type timeoutBotRepoMock struct{}

func (m *timeoutBotRepoMock) IsBot(ctx context.Context, userID string) (bool, error) {
	return false, nil
}

func (m *timeoutBotRepoMock) GetRandomBotUserID(ctx context.Context) (string, error) {
	return "", nil
}

func TestHandleMatchTimeoutHandler(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()

	t.Run("cancels match when neither participant completed before timeout", func(t *testing.T) {
		matchRepo := &timeoutMatchRepoMock{matches: make(map[string]*aggregate.SoloMatch)}
		eventPub := &timeoutEventPubMock{}
		txManager := &timeoutTxManagerMock{}
		botRepo := &timeoutBotRepoMock{}

		p1, _ := entity.NewMatchParticipant("p_1", "m_timeout_1", "user_1")
		p2, _ := entity.NewMatchParticipant("p_2", "m_timeout_1", "user_2")
		match, _ := aggregate.NewSoloMatch(
			"m_timeout_1",
			"ex_pushup",
			"Push-Up",
			valueobject.ChallengeTypeReps,
			15*time.Minute,
			[]*entity.MatchParticipant{p1, p2},
			now,
		)
		_ = matchRepo.Save(ctx, match)

		handler := command.NewHandleMatchTimeoutHandler(matchRepo, eventPub, txManager, botRepo)

		// Sweep at 20 minutes later (past 15m timeout)
		sweepTime := now.Add(20 * time.Minute)
		count, err := handler.Handle(ctx, command.HandleMatchTimeoutCommand{
			Now:   sweepTime,
			Limit: 10,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if count != 1 {
			t.Errorf("expected 1 match to be timed out, got %d", count)
		}

		timedOutMatch := matchRepo.matches["m_timeout_1"]
		if timedOutMatch.Status() != valueobject.MatchStatusCancelled {
			t.Errorf("expected CANCELLED status when neither completed, got %s", timedOutMatch.Status())
		}
	})

	t.Run("forfeits match and awards win to completed participant on timeout", func(t *testing.T) {
		matchRepo := &timeoutMatchRepoMock{matches: make(map[string]*aggregate.SoloMatch)}
		eventPub := &timeoutEventPubMock{}
		txManager := &timeoutTxManagerMock{}
		botRepo := &timeoutBotRepoMock{}

		p1, _ := entity.NewMatchParticipant("p_1", "m_timeout_2", "user_1")
		p2, _ := entity.NewMatchParticipant("p_2", "m_timeout_2", "user_2")
		_ = p1.RecordResult(25, now.Add(5*time.Minute))

		match, _ := aggregate.NewSoloMatch(
			"m_timeout_2",
			"ex_pushup",
			"Push-Up",
			valueobject.ChallengeTypeReps,
			15*time.Minute,
			[]*entity.MatchParticipant{p1, p2},
			now,
		)
		_ = matchRepo.Save(ctx, match)

		handler := command.NewHandleMatchTimeoutHandler(matchRepo, eventPub, txManager, botRepo)

		sweepTime := now.Add(20 * time.Minute)
		count, err := handler.Handle(ctx, command.HandleMatchTimeoutCommand{
			Now:   sweepTime,
			Limit: 10,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if count != 1 {
			t.Errorf("expected 1 match to be timed out, got %d", count)
		}

		timedOutMatch := matchRepo.matches["m_timeout_2"]
		if timedOutMatch.Status() != valueobject.MatchStatusForfeited {
			t.Errorf("expected FORFEITED status, got %s", timedOutMatch.Status())
		}
		if timedOutMatch.WinnerID() == nil || *timedOutMatch.WinnerID() != "user_1" {
			t.Errorf("expected user_1 to win by forfeit, got %v", timedOutMatch.WinnerID())
		}
	})

	t.Run("returns zero count when no matches have expired", func(t *testing.T) {
		matchRepo := &timeoutMatchRepoMock{matches: make(map[string]*aggregate.SoloMatch)}
		eventPub := &timeoutEventPubMock{}
		txManager := &timeoutTxManagerMock{}
		botRepo := &timeoutBotRepoMock{}

		p1, _ := entity.NewMatchParticipant("p_1", "m_timeout_3", "user_1")
		p2, _ := entity.NewMatchParticipant("p_2", "m_timeout_3", "user_2")
		match, _ := aggregate.NewSoloMatch(
			"m_timeout_3",
			"ex_pushup",
			"Push-Up",
			valueobject.ChallengeTypeReps,
			15*time.Minute,
			[]*entity.MatchParticipant{p1, p2},
			now,
		)
		_ = matchRepo.Save(ctx, match)

		handler := command.NewHandleMatchTimeoutHandler(matchRepo, eventPub, txManager, botRepo)

		sweepTime := now.Add(5 * time.Minute)
		count, err := handler.Handle(ctx, command.HandleMatchTimeoutCommand{
			Now:   sweepTime,
			Limit: 10,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if count != 0 {
			t.Errorf("expected 0 matches timed out, got %d", count)
		}
	})
}
