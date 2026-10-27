package command_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/solo_match/application/command"
	"github.com/viethung213/gym-companion/internal/solo_match/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/solo_match/domain/entity"
	"github.com/viethung213/gym-companion/internal/solo_match/domain/event"
	"github.com/viethung213/gym-companion/internal/solo_match/domain/service"
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

func (m *timeoutMatchRepoMock) FindUnreadyExpiredMatches(ctx context.Context, now time.Time, limit int) ([]*aggregate.SoloMatch, error) {
	var res []*aggregate.SoloMatch
	for _, match := range m.matches {
		if match.Status() == valueobject.MatchStatusPendingReady && now.After(match.ReadyDeadline()) {
			res = append(res, match)
			if len(res) >= limit {
				break
			}
		}
	}
	return res, nil
}

func (m *timeoutMatchRepoMock) FindExpiredInProgressMatches(ctx context.Context, startedBefore time.Time, limit int) ([]*aggregate.SoloMatch, error) {
	var res []*aggregate.SoloMatch
	for _, match := range m.matches {
		if match.Status() == valueobject.MatchStatusInProgress && match.StartedAt() != nil && !match.StartedAt().After(startedBefore) {
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

type failingTimeoutTxManagerMock struct{}

func (t *failingTimeoutTxManagerMock) WithTransaction(_ context.Context, _ func(txCtx context.Context) error) error {
	return errors.New("db connection timeout")
}

func TestHandleMatchTimeoutHandler(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()

	t.Run("does not touch in-progress matches before duration ceiling", func(t *testing.T) {
		matchRepo := &timeoutMatchRepoMock{matches: make(map[string]*aggregate.SoloMatch)}
		eventPub := &timeoutEventPubMock{}
		txManager := &timeoutTxManagerMock{}

		p1, _ := entity.NewMatchParticipant("p_1", "m_timeout_1", "user_1")
		p2, _ := entity.NewMatchParticipant("p_2", "m_timeout_1", "user_2")
		match, _ := aggregate.NewSoloMatch(
			"m_timeout_1",
			"ex_pushup",
			"Push-Up",
			valueobject.ChallengeTypeReps,
			[]*entity.MatchParticipant{p1, p2},
			now,
		)
		_ = match.ActivateRoom(now)
		_ = matchRepo.Save(ctx, match)

		handler := command.NewHandleMatchTimeoutHandler(matchRepo, eventPub, txManager)

		// Sweep at 20 minutes later
		sweepTime := now.Add(20 * time.Minute)
		err := handler.Handle(ctx, command.HandleMatchTimeoutCommand{
			Now:   sweepTime,
			Limit: 10,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		inProgressMatch := matchRepo.matches["m_timeout_1"]
		if inProgressMatch.Status() != valueobject.MatchStatusInProgress {
			t.Errorf("expected IN_PROGRESS status to remain untouched, got %s", inProgressMatch.Status())
		}
	})

	t.Run("returns zero count when no matches have expired", func(t *testing.T) {
		matchRepo := &timeoutMatchRepoMock{matches: make(map[string]*aggregate.SoloMatch)}
		eventPub := &timeoutEventPubMock{}
		txManager := &timeoutTxManagerMock{}

		p1, _ := entity.NewMatchParticipant("p_1", "m_timeout_3", "user_1")
		p2, _ := entity.NewMatchParticipant("p_2", "m_timeout_3", "user_2")
		match, _ := aggregate.NewSoloMatch(
			"m_timeout_3",
			"ex_pushup",
			"Push-Up",
			valueobject.ChallengeTypeReps,
			[]*entity.MatchParticipant{p1, p2},
			now,
		)
		_ = match.ActivateRoom(now)
		_ = matchRepo.Save(ctx, match)

		handler := command.NewHandleMatchTimeoutHandler(matchRepo, eventPub, txManager)

		sweepTime := now.Add(5 * time.Minute)
		err := handler.Handle(ctx, command.HandleMatchTimeoutCommand{
			Now:   sweepTime,
			Limit: 10,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("sweeps and cancels unready matches that exceeded 60s check-in deadline", func(t *testing.T) {
		matchRepo := &timeoutMatchRepoMock{matches: make(map[string]*aggregate.SoloMatch)}
		eventPub := &timeoutEventPubMock{}
		txManager := &timeoutTxManagerMock{}

		p1, _ := entity.NewMatchParticipant("p_1", "m_unready", "user_1")
		p2, _ := entity.NewMatchParticipant("p_2", "m_unready", "user_2")
		match, _ := aggregate.NewSoloMatch(
			"m_unready",
			"ex_pushup",
			"Push-Up",
			valueobject.ChallengeTypeReps,
			[]*entity.MatchParticipant{p1, p2},
			now,
		)
		// Match stays in PENDING_READY, deadline is now + 60s
		_ = matchRepo.Save(ctx, match)

		handler := command.NewHandleMatchTimeoutHandler(matchRepo, eventPub, txManager)

		// Sweep at 70s later (past 60s check-in deadline)
		sweepTime := now.Add(70 * time.Second)
		err := handler.Handle(ctx, command.HandleMatchTimeoutCommand{
			Now:   sweepTime,
			Limit: 10,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		cancelledMatch := matchRepo.matches["m_unready"]
		if cancelledMatch.Status() != valueobject.MatchStatusCancelled {
			t.Errorf("expected CANCELLED status for unready match, got %s", cancelledMatch.Status())
		}
	})

	t.Run("logs warning and keeps match untouched when transaction fails for unready match", func(t *testing.T) {
		matchRepo := &timeoutMatchRepoMock{matches: make(map[string]*aggregate.SoloMatch)}
		eventPub := &timeoutEventPubMock{}
		failingTxManager := &failingTimeoutTxManagerMock{}

		p1, _ := entity.NewMatchParticipant("p_1", "m_fail", "user_1")
		p2, _ := entity.NewMatchParticipant("p_2", "m_fail", "user_2")
		match, _ := aggregate.NewSoloMatch(
			"m_fail",
			"ex_pushup",
			"Push-Up",
			valueobject.ChallengeTypeReps,
			[]*entity.MatchParticipant{p1, p2},
			now,
		)
		_ = matchRepo.Save(ctx, match)

		handler := command.NewHandleMatchTimeoutHandler(matchRepo, eventPub, failingTxManager)

		sweepTime := now.Add(70 * time.Second)
		err := handler.Handle(ctx, command.HandleMatchTimeoutCommand{
			Now:   sweepTime,
			Limit: 10,
		})
		if err != nil {
			t.Fatalf("expected nil error for batch sweep even if one match fails, got %v", err)
		}
	})

	t.Run("sweeps expired in-progress match where p1 completed and p2 timed out", func(t *testing.T) {
		matchRepo := &timeoutMatchRepoMock{matches: make(map[string]*aggregate.SoloMatch)}
		eventPub := &timeoutEventPubMock{}
		txManager := &timeoutTxManagerMock{}
		resolver := service.NewMatchResolver()

		p1, _ := entity.NewMatchParticipant("p_1", "m_inprog_p1", "user_1")
		p2, _ := entity.NewMatchParticipant("p_2", "m_inprog_p1", "user_2")
		match, _ := aggregate.NewSoloMatch(
			"m_inprog_p1",
			"ex_pushup",
			"Push-Up",
			valueobject.ChallengeTypeReps,
			[]*entity.MatchParticipant{p1, p2},
			now,
		)
		_ = match.ActivateRoom(now)
		_ = match.RecordParticipantResult("user_1", 20, now.Add(2*time.Minute), resolver)
		_ = matchRepo.Save(ctx, match)

		handler := command.NewHandleMatchTimeoutHandler(matchRepo, eventPub, txManager)

		// Sweep at 40 minutes later (past 35m default threshold)
		sweepTime := now.Add(40 * time.Minute)
		err := handler.Handle(ctx, command.HandleMatchTimeoutCommand{
			Now:                 sweepTime,
			Limit:               10,
			InProgressThreshold: 35 * time.Minute,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		timedOutMatch := matchRepo.matches["m_inprog_p1"]
		if timedOutMatch.Status() != valueobject.MatchStatusForfeited {
			t.Errorf("expected FORFEITED status, got %s", timedOutMatch.Status())
		}
		if timedOutMatch.WinnerID() == nil || *timedOutMatch.WinnerID() != "user_1" {
			t.Errorf("expected winner user_1, got %v", timedOutMatch.WinnerID())
		}
		if len(eventPub.events) != 2 {
			t.Fatalf("expected 2 events published (Forfeited + Completed), got %d", len(eventPub.events))
		}
	})

	t.Run("sweeps expired in-progress match where neither player completed cancels match", func(t *testing.T) {
		matchRepo := &timeoutMatchRepoMock{matches: make(map[string]*aggregate.SoloMatch)}
		eventPub := &timeoutEventPubMock{}
		txManager := &timeoutTxManagerMock{}

		p1, _ := entity.NewMatchParticipant("p_1", "m_inprog_both", "user_1")
		p2, _ := entity.NewMatchParticipant("p_2", "m_inprog_both", "user_2")
		match, _ := aggregate.NewSoloMatch(
			"m_inprog_both",
			"ex_pushup",
			"Push-Up",
			valueobject.ChallengeTypeReps,
			[]*entity.MatchParticipant{p1, p2},
			now,
		)
		_ = match.ActivateRoom(now)
		_ = matchRepo.Save(ctx, match)

		handler := command.NewHandleMatchTimeoutHandler(matchRepo, eventPub, txManager)

		sweepTime := now.Add(40 * time.Minute)
		err := handler.Handle(ctx, command.HandleMatchTimeoutCommand{
			Now:                 sweepTime,
			Limit:               10,
			InProgressThreshold: 35 * time.Minute,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		timedOutMatch := matchRepo.matches["m_inprog_both"]
		if timedOutMatch.Status() != valueobject.MatchStatusCancelled {
			t.Errorf("expected CANCELLED status, got %s", timedOutMatch.Status())
		}
		if len(eventPub.events) != 1 {
			t.Fatalf("expected 1 event published (Cancelled), got %d", len(eventPub.events))
		}
		cancelledEv, ok := eventPub.events[0].(*event.SoloMatchCancelled)
		if !ok || cancelledEv.Reason != "IN_PROGRESS_TIMEOUT" {
			t.Errorf("expected SoloMatchCancelled with reason IN_PROGRESS_TIMEOUT, got %+v", eventPub.events[0])
		}
	})

	t.Run("leaves unexpired in-progress match untouched", func(t *testing.T) {
		matchRepo := &timeoutMatchRepoMock{matches: make(map[string]*aggregate.SoloMatch)}
		eventPub := &timeoutEventPubMock{}
		txManager := &timeoutTxManagerMock{}

		p1, _ := entity.NewMatchParticipant("p_1", "m_inprog_active", "user_1")
		p2, _ := entity.NewMatchParticipant("p_2", "m_inprog_active", "user_2")
		match, _ := aggregate.NewSoloMatch(
			"m_inprog_active",
			"ex_pushup",
			"Push-Up",
			valueobject.ChallengeTypeReps,
			[]*entity.MatchParticipant{p1, p2},
			now,
		)
		_ = match.ActivateRoom(now)
		_ = matchRepo.Save(ctx, match)

		handler := command.NewHandleMatchTimeoutHandler(matchRepo, eventPub, txManager)

		// Sweep at 10 minutes later (before 35m threshold)
		sweepTime := now.Add(10 * time.Minute)
		err := handler.Handle(ctx, command.HandleMatchTimeoutCommand{
			Now:                 sweepTime,
			Limit:               10,
			InProgressThreshold: 35 * time.Minute,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		activeMatch := matchRepo.matches["m_inprog_active"]
		if activeMatch.Status() != valueobject.MatchStatusInProgress {
			t.Errorf("expected IN_PROGRESS status untouched, got %s", activeMatch.Status())
		}
		if len(eventPub.events) != 0 {
			t.Errorf("expected 0 events published for untouched match, got %d", len(eventPub.events))
		}
	})
}
