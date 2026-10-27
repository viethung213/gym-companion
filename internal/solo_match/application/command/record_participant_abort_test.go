package command_test

import (
	"context"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/solo_match/application/command"
	"github.com/viethung213/gym-companion/internal/solo_match/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/solo_match/domain/entity"
	"github.com/viethung213/gym-companion/internal/solo_match/domain/event"
	valueobject "github.com/viethung213/gym-companion/internal/solo_match/domain/value_object"
)

type abortMatchRepoMock struct {
	matches map[string]*aggregate.SoloMatch
}

func (m *abortMatchRepoMock) Save(ctx context.Context, match *aggregate.SoloMatch) error {
	m.matches[match.ID()] = match
	return nil
}

func (m *abortMatchRepoMock) FindByID(ctx context.Context, id string) (*aggregate.SoloMatch, error) {
	return m.matches[id], nil
}

func (m *abortMatchRepoMock) FindActiveByUserID(ctx context.Context, userID string) (*aggregate.SoloMatch, error) {
	return nil, nil
}

func (m *abortMatchRepoMock) FindUnreadyExpiredMatches(ctx context.Context, now time.Time, limit int) ([]*aggregate.SoloMatch, error) {
	return nil, nil
}

func (m *abortMatchRepoMock) FindExpiredInProgressMatches(ctx context.Context, startedBefore time.Time, limit int) ([]*aggregate.SoloMatch, error) {
	return nil, nil
}

func (m *abortMatchRepoMock) ListByUserID(ctx context.Context, userID string, offset, limit int) ([]*aggregate.SoloMatch, error) {
	return nil, nil
}

type abortEventPubMock struct {
	events []any
}

func (m *abortEventPubMock) PublishEvents(ctx context.Context, events []any) error {
	m.events = append(m.events, events...)
	return nil
}

type abortTxManagerMock struct{}

func (t *abortTxManagerMock) WithTransaction(ctx context.Context, fn func(txCtx context.Context) error) error {
	return fn(ctx)
}

type abortBotRepoMock struct{}

func (m *abortBotRepoMock) IsBot(ctx context.Context, userID string) (bool, error) {
	if userID == "u_bot" {
		return true, nil
	}
	return false, nil
}

func (m *abortBotRepoMock) GetRandomBotUserID(ctx context.Context) (string, error) {
	return "u_bot", nil
}

func TestRecordParticipantAbortHandler(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()

	t.Run("successfully forfeits match when human participant aborts", func(t *testing.T) {
		matchRepo := &abortMatchRepoMock{matches: make(map[string]*aggregate.SoloMatch)}
		eventPub := &abortEventPubMock{}
		txManager := &abortTxManagerMock{}
		botRepo := &abortBotRepoMock{}

		p1, _ := entity.NewMatchParticipant("p_1", "m_abort_1", "user_1")
		p2, _ := entity.NewMatchParticipant("p_2", "m_abort_1", "user_2")
		match, _ := aggregate.NewSoloMatch(
			"m_abort_1",
			"ex_pushup",
			"Push-Up",
			valueobject.ChallengeTypeReps,
			[]*entity.MatchParticipant{p1, p2},
			now,
		)
		_ = match.ActivateRoom(now)
		_ = matchRepo.Save(ctx, match)

		handler := command.NewRecordParticipantAbortHandler(matchRepo, eventPub, txManager, botRepo)

		// user_1 aborts -> user_2 wins by forfeit
		err := handler.Handle(ctx, command.RecordParticipantAbortCommand{
			MatchID: "m_abort_1",
			UserID:  "user_1",
			Reason:  "camera_disconnected",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		forfeitedMatch := matchRepo.matches["m_abort_1"]
		if forfeitedMatch.Status() != valueobject.MatchStatusForfeited {
			t.Errorf("expected FORFEITED status, got %s", forfeitedMatch.Status())
		}
		if forfeitedMatch.WinnerID() == nil || *forfeitedMatch.WinnerID() != "user_2" {
			t.Errorf("expected winner to be user_2, got %v", forfeitedMatch.WinnerID())
		}
		if len(eventPub.events) != 2 {
			t.Errorf("expected 2 outbox events (Forfeited + Completed), got %d", len(eventPub.events))
		}
	})

	t.Run("successfully identifies bot when opponent is bot", func(t *testing.T) {
		matchRepo := &abortMatchRepoMock{matches: make(map[string]*aggregate.SoloMatch)}
		botRepo := &abortBotRepoMock{}
		eventPub := &abortEventPubMock{}
		p1, _ := entity.NewMatchParticipant("p_1", "m_abort_bot", "user_1")
		pBot, _ := entity.NewMatchParticipant("p_bot", "m_abort_bot", "u_bot")
		match, _ := aggregate.NewSoloMatch(
			"m_abort_bot",
			"ex_pushup",
			"Push-Up",
			valueobject.ChallengeTypeReps,
			[]*entity.MatchParticipant{p1, pBot},
			now,
		)
		_ = match.ActivateRoom(now)
		_ = matchRepo.Save(ctx, match)

		handler := command.NewRecordParticipantAbortHandler(matchRepo, eventPub, &abortTxManagerMock{}, botRepo)

		// user_1 aborts -> u_bot wins, BotUserID must be identified as u_bot
		err := handler.Handle(ctx, command.RecordParticipantAbortCommand{
			MatchID: "m_abort_bot",
			UserID:  "user_1",
			Reason:  "quit",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(eventPub.events) != 2 {
			t.Fatalf("expected 2 outbox events, got %d", len(eventPub.events))
		}
		forfeitedEv, ok := eventPub.events[0].(*event.SoloMatchForfeited)
		if !ok || forfeitedEv.BotUserID != "u_bot" {
			t.Errorf("expected BotUserID to be u_bot, got %v", forfeitedEv)
		}
	})

	t.Run("fails when match not found", func(t *testing.T) {
		matchRepo := &abortMatchRepoMock{matches: make(map[string]*aggregate.SoloMatch)}
		handler := command.NewRecordParticipantAbortHandler(matchRepo, &abortEventPubMock{}, &abortTxManagerMock{}, &abortBotRepoMock{})

		err := handler.Handle(ctx, command.RecordParticipantAbortCommand{
			MatchID: "unknown_match",
			UserID:  "user_1",
			Reason:  "quit",
		})
		if err == nil {
			t.Errorf("expected error for non-existent match, got nil")
		}
	})
}
