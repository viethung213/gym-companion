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

type resultMatchRepoMock struct {
	matches map[string]*aggregate.SoloMatch
}

func (m *resultMatchRepoMock) Save(ctx context.Context, match *aggregate.SoloMatch) error {
	m.matches[match.ID()] = match
	return nil
}

func (m *resultMatchRepoMock) FindByID(ctx context.Context, id string) (*aggregate.SoloMatch, error) {
	return m.matches[id], nil
}

func (m *resultMatchRepoMock) FindActiveByUserID(ctx context.Context, userID string) (*aggregate.SoloMatch, error) {
	return nil, nil
}

func (m *resultMatchRepoMock) FindExpiredMatches(ctx context.Context, now time.Time, limit int) ([]*aggregate.SoloMatch, error) {
	return nil, nil
}

func (m *resultMatchRepoMock) ListByUserID(ctx context.Context, userID string, offset, limit int) ([]*aggregate.SoloMatch, error) {
	return nil, nil
}

type resultEventPubMock struct {
	events []any
}

func (m *resultEventPubMock) PublishEvents(ctx context.Context, events []any) error {
	m.events = append(m.events, events...)
	return nil
}

type resultTxManagerMock struct{}

func (t *resultTxManagerMock) WithTransaction(ctx context.Context, fn func(txCtx context.Context) error) error {
	return fn(ctx)
}

type resultBotRepoMock struct{}

func (m *resultBotRepoMock) IsBot(ctx context.Context, userID string) (bool, error) {
	return false, nil
}

func (m *resultBotRepoMock) GetRandomBotUserID(ctx context.Context) (string, error) {
	return "", nil
}

func TestRecordParticipantResultHandler(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()

	t.Run("records participant 1 result and then participant 2 completes match", func(t *testing.T) {
		matchRepo := &resultMatchRepoMock{matches: make(map[string]*aggregate.SoloMatch)}
		eventPub := &resultEventPubMock{}
		txManager := &resultTxManagerMock{}
		botRepo := &resultBotRepoMock{}

		p1, _ := entity.NewMatchParticipant("p_1", "m_1", "user_1")
		p2, _ := entity.NewMatchParticipant("p_2", "m_1", "user_2")
		match, _ := aggregate.NewSoloMatch(
			"m_1",
			"ex_pushup",
			"Push-Up",
			valueobject.ChallengeTypeReps,
			15*time.Minute,
			[]*entity.MatchParticipant{p1, p2},
			now,
		)
		_ = matchRepo.Save(ctx, match)

		handler := command.NewRecordParticipantResultHandler(matchRepo, eventPub, txManager, nil, botRepo)

		// 1. Participant 1 records result (20 reps)
		err := handler.Handle(ctx, command.RecordParticipantResultCommand{
			MatchID:     "m_1",
			UserID:      "user_1",
			ScoreValue:  20,
			CompletedAt: now,
		})
		if err != nil {
			t.Fatalf("unexpected error for p1: %v", err)
		}

		savedMatch := matchRepo.matches["m_1"]
		if savedMatch.Status() != valueobject.MatchStatusInProgress {
			t.Errorf("expected IN_PROGRESS status after 1 participant finished, got %s", savedMatch.Status())
		}

		// 2. Participant 2 records result (25 reps) -> Match completes, user_2 wins
		err = handler.Handle(ctx, command.RecordParticipantResultCommand{
			MatchID:     "m_1",
			UserID:      "user_2",
			ScoreValue:  25,
			CompletedAt: now,
		})
		if err != nil {
			t.Fatalf("unexpected error for p2: %v", err)
		}

		completedMatch := matchRepo.matches["m_1"]
		if completedMatch.Status() != valueobject.MatchStatusCompleted {
			t.Errorf("expected COMPLETED status, got %s", completedMatch.Status())
		}
		if completedMatch.WinnerID() == nil || *completedMatch.WinnerID() != "user_2" {
			t.Errorf("expected user_2 to win, got %v", completedMatch.WinnerID())
		}
		if len(eventPub.events) == 0 {
			t.Errorf("expected outbox event published, got none")
		}
	})

	t.Run("fails when match not found", func(t *testing.T) {
		matchRepo := &resultMatchRepoMock{matches: make(map[string]*aggregate.SoloMatch)}
		handler := command.NewRecordParticipantResultHandler(matchRepo, &resultEventPubMock{}, &resultTxManagerMock{}, nil, &resultBotRepoMock{})

		err := handler.Handle(ctx, command.RecordParticipantResultCommand{
			MatchID:     "non_existent",
			UserID:      "user_1",
			ScoreValue:  10,
			CompletedAt: now,
		})
		if err == nil {
			t.Errorf("expected error for non-existent match, got nil")
		}
	})

	t.Run("fails when user is not a participant", func(t *testing.T) {
		matchRepo := &resultMatchRepoMock{matches: make(map[string]*aggregate.SoloMatch)}
		p1, _ := entity.NewMatchParticipant("p_1", "m_2", "user_1")
		p2, _ := entity.NewMatchParticipant("p_2", "m_2", "user_2")
		match, _ := aggregate.NewSoloMatch(
			"m_2",
			"ex_pushup",
			"Push-Up",
			valueobject.ChallengeTypeReps,
			15*time.Minute,
			[]*entity.MatchParticipant{p1, p2},
			now,
		)
		_ = matchRepo.Save(ctx, match)

		handler := command.NewRecordParticipantResultHandler(matchRepo, &resultEventPubMock{}, &resultTxManagerMock{}, nil, &resultBotRepoMock{})
		err := handler.Handle(ctx, command.RecordParticipantResultCommand{
			MatchID:     "m_2",
			UserID:      "intruder_user",
			ScoreValue:  10,
			CompletedAt: now,
		})
		if err == nil {
			t.Errorf("expected error for non-participant user, got nil")
		}
	})
}
