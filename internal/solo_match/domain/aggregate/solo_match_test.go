package aggregate_test

import (
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/solo_match/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/solo_match/domain/entity"
	"github.com/viethung213/gym-companion/internal/solo_match/domain/service"
	valueobject "github.com/viethung213/gym-companion/internal/solo_match/domain/value_object"
)

func TestSoloMatch_Lifecycle(t *testing.T) {
	now := time.Now().UTC()
	resolver := service.NewMatchResolver()

	p1, err := entity.NewMatchParticipant("part_1", "match_1", "user_1")
	if err != nil {
		t.Fatalf("NewMatchParticipant p1 err: %v", err)
	}
	p2, err := entity.NewMatchParticipant("part_2", "match_1", "user_2")
	if err != nil {
		t.Fatalf("NewMatchParticipant p2 err: %v", err)
	}

	match, err := aggregate.NewSoloMatch(
		"match_1",
		"ex_pushup",
		"Push Up",
		valueobject.ChallengeTypeReps,
		[]*entity.MatchParticipant{p1, p2},
		now,
	)
	if err != nil {
		t.Fatalf("NewSoloMatch err: %v", err)
	}

	if match.Status() != valueobject.MatchStatusPendingReady {
		t.Errorf("match status got %v, want PENDING_READY", match.Status())
	}
	if err := match.ActivateRoom(now); err != nil {
		t.Fatalf("ActivateRoom err: %v", err)
	}
	if match.Status() != valueobject.MatchStatusInProgress {
		t.Errorf("match status got %v, want IN_PROGRESS", match.Status())
	}

	// User 1 hoàn thành bài tập trước với 30 reps
	err = match.RecordParticipantResult("user_1", 30, now.Add(2*time.Minute), resolver)
	if err != nil {
		t.Fatalf("RecordParticipantResult p1 err: %v", err)
	}
	// Trận đấu vẫn phải IN_PROGRESS vì User 2 chưa xong
	if match.Status() != valueobject.MatchStatusInProgress {
		t.Errorf("match status after p1 got %v, want IN_PROGRESS", match.Status())
	}

	// User 2 hoàn thành bài tập với 25 reps
	err = match.RecordParticipantResult("user_2", 25, now.Add(3*time.Minute), resolver)
	if err != nil {
		t.Fatalf("RecordParticipantResult p2 err: %v", err)
	}

	// Cả 2 đã xong -> Trận đấu phải COMPLETED, Winner = user_1
	if match.Status() != valueobject.MatchStatusCompleted {
		t.Errorf("match status after both completed got %v, want COMPLETED", match.Status())
	}
	if match.WinnerID() == nil || *match.WinnerID() != "user_1" {
		t.Errorf("match winner got %v, want user_1", match.WinnerID())
	}
	if match.IsDraw() {
		t.Errorf("match isDraw got true, want false")
	}
}

func TestSoloMatch_ForfeitParticipant(t *testing.T) {
	now := time.Now().UTC()
	p1, _ := entity.NewMatchParticipant("part_1", "match_2", "user_1")
	p2, _ := entity.NewMatchParticipant("part_2", "match_2", "user_2")

	match, err := aggregate.NewSoloMatch(
		"match_2",
		"ex_pushup",
		"Push Up",
		valueobject.ChallengeTypeReps,
		[]*entity.MatchParticipant{p1, p2},
		now,
	)
	if err != nil {
		t.Fatalf("NewSoloMatch err: %v", err)
	}

	if err := match.ActivateRoom(now); err != nil {
		t.Fatalf("ActivateRoom err: %v", err)
	}

	// User 2 bỏ cuộc hoặc đứt stream (Fast Forfeit)
	err = match.ForfeitParticipant("user_2", "connection_lost")
	if err != nil {
		t.Fatalf("ForfeitParticipant err: %v", err)
	}

	if match.Status() != valueobject.MatchStatusForfeited {
		t.Errorf("match status got %v, want FORFEITED", match.Status())
	}
	if match.WinnerID() == nil || *match.WinnerID() != "user_1" {
		t.Errorf("match winner got %v, want user_1", match.WinnerID())
	}

	// Trường hợp người chơi đã hoàn thành bài tập không được phép forfeit
	p3, _ := entity.NewMatchParticipant("part_3", "match_2b", "user_3")
	p4, _ := entity.NewMatchParticipant("part_4", "match_2b", "user_4")
	_ = p4.RecordResult(25, now) // p4 đã hoàn thành
	match2b, _ := aggregate.NewSoloMatch(
		"match_2b",
		"ex_pushup",
		"Push Up",
		valueobject.ChallengeTypeReps,
		[]*entity.MatchParticipant{p3, p4},
		now,
	)
	_ = match2b.ActivateRoom(now)
	err = match2b.ForfeitParticipant("user_4", "give_up")
	if err == nil {
		t.Errorf("ForfeitParticipant on completed participant want error, got nil")
	}
}

func TestSoloMatch_EdgeCasesAndGetters(t *testing.T) {
	now := time.Now().UTC()
	p1, _ := entity.NewMatchParticipant("p_1", "m_1", "u_1")
	p2, _ := entity.NewMatchParticipant("p_2", "m_1", "u_2")

	// 1. Invalid ID
	_, err := aggregate.NewSoloMatch("", "ex_1", "Push Up", valueobject.ChallengeTypeReps, []*entity.MatchParticipant{p1, p2}, now)
	if err == nil {
		t.Errorf("NewSoloMatch empty ID want error, got nil")
	}

	// 2. Invalid count
	_, err = aggregate.NewSoloMatch("m_1", "ex_1", "Push Up", valueobject.ChallengeTypeReps, []*entity.MatchParticipant{p1}, now)
	if err == nil {
		t.Errorf("NewSoloMatch 1 participant want error, got nil")
	}

	// 3. Valid creation & Getters
	match, err := aggregate.NewSoloMatch("m_1", "ex_1", "Push Up", valueobject.ChallengeTypeReps, []*entity.MatchParticipant{p1, p2}, now)
	if err != nil {
		t.Fatalf("NewSoloMatch err: %v", err)
	}
	if match.ID() != "m_1" || match.ExerciseID() != "ex_1" || match.ExerciseName() != "Push Up" || match.ChallengeType() != valueobject.ChallengeTypeReps {
		t.Errorf("mismatch on getters")
	}
	if len(match.Participants()) != 2 {
		t.Errorf("participants len got %d, want 2", len(match.Participants()))
	}
	if match.StartedAt() != nil {
		t.Errorf("startedAt should be nil initially in PENDING_READY, got %v", match.StartedAt())
	}
	if match.ReadyDeadline().Before(now) {
		t.Errorf("readyDeadline should be after now")
	}
	if match.EndedAt() != nil {
		t.Errorf("endedAt should be nil initially")
	}

	// 4. Record result participant not found
	resolver := service.NewMatchResolver()
	err = match.RecordParticipantResult("unknown_user", 10, now, resolver)
	if err == nil {
		t.Errorf("RecordParticipantResult unknown user want error, got nil")
	}

	// 5. Forfeit participant not found
	err = match.ForfeitParticipant("unknown_user", "quit")
	if err == nil {
		t.Errorf("ForfeitParticipant unknown user want error, got nil")
	}

	// 6. Clear domain events
	match.ClearDomainEvents()
	if len(match.DomainEvents()) != 0 {
		t.Errorf("ClearDomainEvents got %d events, want 0", len(match.DomainEvents()))
	}

	// 7. Reconstitute
	winner := "u_1"
	ended := now.Add(5 * time.Minute)
	reconstituted := aggregate.Reconstitute(
		"m_rec", "ex_1", "Push Up", valueobject.ChallengeTypeReps,
		valueobject.MatchStatusCompleted, []*entity.MatchParticipant{p1, p2},
		&winner, false, &now, now.Add(time.Minute), &ended,
	)
	if reconstituted.ID() != "m_rec" || *reconstituted.WinnerID() != "u_1" || reconstituted.EndedAt() == nil {
		t.Errorf("Reconstitute mismatch: %+v", reconstituted)
	}

	// Record result on finished match -> ErrMatchNotActive
	err = reconstituted.RecordParticipantResult("u_1", 20, now, resolver)
	if err == nil {
		t.Errorf("RecordParticipantResult on finished match want error, got nil")
	}
	err = reconstituted.ForfeitParticipant("u_1", "quit")
	if err == nil {
		t.Errorf("ForfeitParticipant on finished match want error, got nil")
	}
}

func TestMatchmakingTicket(t *testing.T) {
	now := time.Now().UTC()

	// Empty ID or UserID
	_, err := aggregate.NewMatchmakingTicket("", "user_1", now)
	if err == nil {
		t.Errorf("NewMatchmakingTicket empty ID want error, got nil")
	}
	_, err = aggregate.NewMatchmakingTicket("t_1", "", now)
	if err == nil {
		t.Errorf("NewMatchmakingTicket empty UserID want error, got nil")
	}

	ticket, err := aggregate.NewMatchmakingTicket("t_1", "user_1", time.Time{})
	if err != nil {
		t.Fatalf("NewMatchmakingTicket err: %v", err)
	}
	if ticket.ID() != "t_1" || ticket.UserID() != "user_1" {
		t.Errorf("mismatch ticket ID/UserID")
	}
	if ticket.JoinedAt().IsZero() {
		t.Errorf("ticket JoinedAt should not be zero")
	}

	ticketWithTime, err := aggregate.NewMatchmakingTicket("t_2", "user_2", now)
	if err != nil {
		t.Fatalf("NewMatchmakingTicket with time err: %v", err)
	}
	if !ticketWithTime.JoinedAt().Equal(now) {
		t.Errorf("ticket JoinedAt mismatch: got %v, want %v", ticketWithTime.JoinedAt(), now)
	}
}

func TestSoloMatch_CancelOnNoShow(t *testing.T) {
	now := time.Now().UTC()
	p1, _ := entity.NewMatchParticipant("p1", "m_ns", "u1")
	p2, _ := entity.NewMatchParticipant("p2", "m_ns", "u2")
	match, err := aggregate.NewSoloMatch(
		"m_ns", "ex_1", "Push Up", valueobject.ChallengeTypeReps,
		[]*entity.MatchParticipant{p1, p2}, now,
	)
	if err != nil {
		t.Fatalf("NewSoloMatch err: %v", err)
	}

	// Before 60s deadline -> ErrMatchNotTimedOutYet
	if err := match.CancelOnNoShow(now.Add(30 * time.Second)); err == nil {
		t.Errorf("expected error cancelling before ready_deadline, got nil")
	}

	// After 60s deadline -> Cancelled
	if err := match.CancelOnNoShow(now.Add(65 * time.Second)); err != nil {
		t.Fatalf("CancelOnNoShow err: %v", err)
	}
	if match.Status() != valueobject.MatchStatusCancelled {
		t.Errorf("got status %v, want CANCELLED", match.Status())
	}

	// Terminal State Rule: Cancelled match cannot be activated
	if err := match.ActivateRoom(now.Add(70 * time.Second)); err == nil {
		t.Errorf("expected error activating cancelled match, got nil")
	}
}

func TestSoloMatch_TimeoutInProgress(t *testing.T) {
	now := time.Now().UTC()
	resolver := service.NewMatchResolver()

	t.Run("p1 completed and p2 timed out awards win to p1", func(t *testing.T) {
		p1, _ := entity.NewMatchParticipant("p1", "m_tip1", "u1")
		p2, _ := entity.NewMatchParticipant("p2", "m_tip1", "u2")
		match, err := aggregate.NewSoloMatch(
			"m_tip1", "ex_1", "Push Up", valueobject.ChallengeTypeReps,
			[]*entity.MatchParticipant{p1, p2}, now,
		)
		if err != nil {
			t.Fatalf("NewSoloMatch err: %v", err)
		}
		_ = match.ActivateRoom(now)

		// p1 completes, p2 does not
		_ = match.RecordParticipantResult("u1", 20, now.Add(time.Minute), resolver)

		if err := match.TimeoutInProgress(now.Add(36 * time.Minute)); err != nil {
			t.Fatalf("TimeoutInProgress error: %v", err)
		}
		if match.Status() != valueobject.MatchStatusForfeited {
			t.Errorf("got status %s, want FORFEITED", match.Status())
		}
		if match.WinnerID() == nil || *match.WinnerID() != "u1" {
			t.Errorf("got winner %v, want u1", match.WinnerID())
		}
	})

	t.Run("p2 completed and p1 timed out awards win to p2", func(t *testing.T) {
		p1, _ := entity.NewMatchParticipant("p1", "m_tip2", "u1")
		p2, _ := entity.NewMatchParticipant("p2", "m_tip2", "u2")
		match, _ := aggregate.NewSoloMatch(
			"m_tip2", "ex_1", "Push Up", valueobject.ChallengeTypeReps,
			[]*entity.MatchParticipant{p1, p2}, now,
		)
		_ = match.ActivateRoom(now)

		// p2 completes, p1 does not
		_ = match.RecordParticipantResult("u2", 25, now.Add(time.Minute), resolver)

		if err := match.TimeoutInProgress(now.Add(36 * time.Minute)); err != nil {
			t.Fatalf("TimeoutInProgress error: %v", err)
		}
		if match.Status() != valueobject.MatchStatusForfeited {
			t.Errorf("got status %s, want FORFEITED", match.Status())
		}
		if match.WinnerID() == nil || *match.WinnerID() != "u2" {
			t.Errorf("got winner %v, want u2", match.WinnerID())
		}
	})

	t.Run("neither completed cancels match", func(t *testing.T) {
		p1, _ := entity.NewMatchParticipant("p1", "m_tip3", "u1")
		p2, _ := entity.NewMatchParticipant("p2", "m_tip3", "u2")
		match, _ := aggregate.NewSoloMatch(
			"m_tip3", "ex_1", "Push Up", valueobject.ChallengeTypeReps,
			[]*entity.MatchParticipant{p1, p2}, now,
		)
		_ = match.ActivateRoom(now)

		if err := match.TimeoutInProgress(now.Add(36 * time.Minute)); err != nil {
			t.Fatalf("TimeoutInProgress error: %v", err)
		}
		if match.Status() != valueobject.MatchStatusCancelled {
			t.Errorf("got status %s, want CANCELLED", match.Status())
		}
	})

	t.Run("error if match not in progress", func(t *testing.T) {
		p1, _ := entity.NewMatchParticipant("p1", "m_tip4", "u1")
		p2, _ := entity.NewMatchParticipant("p2", "m_tip4", "u2")
		match, _ := aggregate.NewSoloMatch(
			"m_tip4", "ex_1", "Push Up", valueobject.ChallengeTypeReps,
			[]*entity.MatchParticipant{p1, p2}, now,
		)
		// Still in PENDING_READY
		if err := match.TimeoutInProgress(now.Add(36 * time.Minute)); err == nil {
			t.Errorf("expected error timing out non in-progress match, got nil")
		}
	})
}
