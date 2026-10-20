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
		15*time.Minute,
		[]*entity.MatchParticipant{p1, p2},
		now,
	)
	if err != nil {
		t.Fatalf("NewSoloMatch err: %v", err)
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
		15*time.Minute,
		[]*entity.MatchParticipant{p1, p2},
		now,
	)
	if err != nil {
		t.Fatalf("NewSoloMatch err: %v", err)
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

	// Trường hợp người chơi đã hoàn thành bài tập (hoặc Bot đã hoàn thành từ đầu) không được phép forfeit
	p3, _ := entity.NewMatchParticipant("part_3", "match_2b", "user_3")
	p4, _ := entity.NewMatchParticipant("part_4", "match_2b", "user_4")
	_ = p4.RecordResult(25, now) // p4 đã hoàn thành
	match2b, _ := aggregate.NewSoloMatch(
		"match_2b",
		"ex_pushup",
		"Push Up",
		valueobject.ChallengeTypeReps,
		15*time.Minute,
		[]*entity.MatchParticipant{p3, p4},
		now,
	)
	err = match2b.ForfeitParticipant("user_4", "give_up")
	if err == nil {
		t.Errorf("ForfeitParticipant on completed participant want error, got nil")
	}
}

func TestSoloMatch_HandleTimeout(t *testing.T) {
	now := time.Now().UTC()
	p1, _ := entity.NewMatchParticipant("part_1", "match_3", "user_1")
	p2, _ := entity.NewMatchParticipant("part_2", "match_3", "user_2")

	match, _ := aggregate.NewSoloMatch(
		"match_3",
		"ex_plank",
		"Plank",
		valueobject.ChallengeTypeDuration,
		10*time.Minute,
		[]*entity.MatchParticipant{p1, p2},
		now,
	)

	// Chưa đến timeout -> Báo lỗi
	err := match.HandleTimeout(now.Add(5 * time.Minute))
	if err == nil {
		t.Errorf("HandleTimeout before timeout want error, got nil")
	}

	// Trường hợp 1: Cả 2 đều không nộp bài khi quá timeout -> CANCELLED
	err = match.HandleTimeout(now.Add(11 * time.Minute))
	if err != nil {
		t.Fatalf("HandleTimeout err: %v", err)
	}
	if match.Status() != valueobject.MatchStatusCancelled {
		t.Errorf("match status got %v, want CANCELLED", match.Status())
	}

	// Trường hợp 2: User 1 nộp bài, User 2 không nộp -> User 2 Forfeit, User 1 Win
	p1b, _ := entity.NewMatchParticipant("part_1b", "match_4", "user_1")
	p2b, _ := entity.NewMatchParticipant("part_2b", "match_4", "user_2")
	match2, _ := aggregate.NewSoloMatch(
		"match_4",
		"ex_plank",
		"Plank",
		valueobject.ChallengeTypeDuration,
		10*time.Minute,
		[]*entity.MatchParticipant{p1b, p2b},
		now,
	)
	_ = p1b.RecordResult(120, now.Add(2*time.Minute))

	err = match2.HandleTimeout(now.Add(11 * time.Minute))
	if err != nil {
		t.Fatalf("HandleTimeout err: %v", err)
	}
	if match2.Status() != valueobject.MatchStatusForfeited {
		t.Errorf("match status got %v, want FORFEITED", match2.Status())
	}
	if match2.WinnerID() == nil || *match2.WinnerID() != "user_1" {
		t.Errorf("winner got %v, want user_1", match2.WinnerID())
	}
	// Trường hợp 3: User 2 nộp bài, User 1 không nộp -> User 1 Forfeit, User 2 Win
	p1c, _ := entity.NewMatchParticipant("part_1c", "match_5", "user_1")
	p2c, _ := entity.NewMatchParticipant("part_2c", "match_5", "user_2")
	match3, _ := aggregate.NewSoloMatch(
		"match_5",
		"ex_plank",
		"Plank",
		valueobject.ChallengeTypeDuration,
		10*time.Minute,
		[]*entity.MatchParticipant{p1c, p2c},
		now,
	)
	_ = p2c.RecordResult(150, now.Add(2*time.Minute))

	err = match3.HandleTimeout(now.Add(11 * time.Minute))
	if err != nil {
		t.Fatalf("HandleTimeout err: %v", err)
	}
	if match3.Status() != valueobject.MatchStatusForfeited {
		t.Errorf("match status got %v, want FORFEITED", match3.Status())
	}
	if match3.WinnerID() == nil || *match3.WinnerID() != "user_2" {
		t.Errorf("winner got %v, want user_2", match3.WinnerID())
	}
}

func TestSoloMatch_EdgeCasesAndGetters(t *testing.T) {
	now := time.Now().UTC()
	p1, _ := entity.NewMatchParticipant("p_1", "m_1", "u_1")
	p2, _ := entity.NewMatchParticipant("p_2", "m_1", "u_2")

	// 1. Invalid ID
	_, err := aggregate.NewSoloMatch("", "ex_1", "Push Up", valueobject.ChallengeTypeReps, 10*time.Minute, []*entity.MatchParticipant{p1, p2}, now)
	if err == nil {
		t.Errorf("NewSoloMatch empty ID want error, got nil")
	}

	// 2. Invalid count
	_, err = aggregate.NewSoloMatch("m_1", "ex_1", "Push Up", valueobject.ChallengeTypeReps, 10*time.Minute, []*entity.MatchParticipant{p1}, now)
	if err == nil {
		t.Errorf("NewSoloMatch 1 participant want error, got nil")
	}

	// 3. Valid creation & Getters
	match, err := aggregate.NewSoloMatch("m_1", "ex_1", "Push Up", valueobject.ChallengeTypeReps, 10*time.Minute, []*entity.MatchParticipant{p1, p2}, now)
	if err != nil {
		t.Fatalf("NewSoloMatch err: %v", err)
	}
	if match.ID() != "m_1" || match.ExerciseID() != "ex_1" || match.ExerciseName() != "Push Up" || match.ChallengeType() != valueobject.ChallengeTypeReps {
		t.Errorf("mismatch on getters")
	}
	if len(match.Participants()) != 2 {
		t.Errorf("participants len got %d, want 2", len(match.Participants()))
	}
	if match.TimeoutAt().Before(match.StartedAt()) {
		t.Errorf("timeoutAt should be after startedAt")
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
		&winner, false, now, now.Add(15*time.Minute), &ended,
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
	err = reconstituted.HandleTimeout(now.Add(20 * time.Minute))
	if err == nil {
		t.Errorf("HandleTimeout on finished match want error, got nil")
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

func TestSoloMatch_HandleTimeout_EmitsCompletedEvent(t *testing.T) {
	now := time.Now().UTC()
	p1, _ := entity.NewMatchParticipant("p_1", "m_to", "u_1")
	p2, _ := entity.NewMatchParticipant("p_2", "m_to", "u_2")

	match, err := aggregate.NewSoloMatch(
		"m_to", "ex_1", "Push Up", valueobject.ChallengeTypeReps,
		15*time.Minute, []*entity.MatchParticipant{p1, p2}, now,
	)
	if err != nil {
		t.Fatalf("NewSoloMatch err: %v", err)
	}

	// Case 1: p1 hoàn thành, p2 quá hạn -> p1 thắng do p2 timeout
	_ = p1.RecordResult(30, now.Add(2*time.Minute))

	// Chưa đến timeout -> ErrMatchNotTimedOutYet
	err = match.HandleTimeout(now.Add(10 * time.Minute))
	if err == nil {
		t.Errorf("expected error when handling timeout before timeout_at")
	}

	// Đã quá timeout
	err = match.HandleTimeout(now.Add(16 * time.Minute))
	if err != nil {
		t.Fatalf("HandleTimeout err: %v", err)
	}

	if match.Status() != valueobject.MatchStatusForfeited {
		t.Errorf("got status %v, want FORFEITED", match.Status())
	}
	if match.WinnerID() == nil || *match.WinnerID() != "u_1" {
		t.Errorf("got winner %v, want u_1", match.WinnerID())
	}

	// Case 2: Cả 2 đều chưa hoàn thành -> CANCELLED
	p3, _ := entity.NewMatchParticipant("p_3", "m_c", "u_3")
	p4, _ := entity.NewMatchParticipant("p_4", "m_c", "u_4")
	matchCancel, _ := aggregate.NewSoloMatch(
		"m_c", "ex_1", "Push Up", valueobject.ChallengeTypeReps,
		15*time.Minute, []*entity.MatchParticipant{p3, p4}, now,
	)
	_ = matchCancel.HandleTimeout(now.Add(16 * time.Minute))
	if matchCancel.Status() != valueobject.MatchStatusCancelled {
		t.Errorf("got status %v, want CANCELLED", matchCancel.Status())
	}
}
