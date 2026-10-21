package dto_test

import (
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/solo_match/application/dto"
	"github.com/viethung213/gym-companion/internal/solo_match/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/solo_match/domain/entity"
	valueobject "github.com/viethung213/gym-companion/internal/solo_match/domain/value_object"
)

func TestToSoloMatchDTO(t *testing.T) {
	now := time.Now().UTC()
	p1, _ := entity.NewMatchParticipant("p1", "match_1", "user_1")
	p2, _ := entity.NewMatchParticipant("p2", "match_1", "user_2")

	match, err := aggregate.NewSoloMatch(
		"match_1",
		"ex_pushup",
		"Push Up Challenge",
		valueobject.ChallengeTypeReps,
		120*time.Second,
		[]*entity.MatchParticipant{p1, p2},
		now,
	)
	if err != nil {
		t.Fatalf("failed to create match: %v", err)
	}

	snap1, _ := entity.NewUserSnapshot("user_1", "Alice", "https://avatar/alice.png", now)
	snap2, _ := entity.NewUserSnapshot("user_2", "Bob", "https://avatar/bob.png", now)

	snapshots := map[string]*entity.UserSnapshot{
		"user_1": snap1,
		"user_2": snap2,
	}

	resultDTO := dto.ToSoloMatchDTO(match, snapshots)

	if resultDTO.MatchID != "match_1" {
		t.Errorf("got MatchID %s, want match_1", resultDTO.MatchID)
	}
	if resultDTO.ChallengeType != "REPS" {
		t.Errorf("got ChallengeType %s, want REPS", resultDTO.ChallengeType)
	}
	if len(resultDTO.Participants) != 2 {
		t.Fatalf("got %d participants, want 2", len(resultDTO.Participants))
	}
	name := resultDTO.Participants[0].User.DisplayName
	if name != "Alice" && name != "Bob" {
		t.Errorf("unexpected participant displayName %s", name)
	}
}

func TestToSoloMatchDTO_WithBotPersona(t *testing.T) {
	now := time.Now().UTC()
	botID := "bot-uuid-1234"
	p1, _ := entity.NewMatchParticipant("p1", "match_2", "user_1")
	p2, _ := entity.NewMatchParticipant("p2", "match_2", botID)

	match, err := aggregate.NewSoloMatch(
		"match_2",
		"ex_pushup",
		"Push Up Challenge",
		valueobject.ChallengeTypeReps,
		120*time.Second,
		[]*entity.MatchParticipant{p1, p2},
		now,
	)
	if err != nil {
		t.Fatalf("failed to create match: %v", err)
	}

	snap1, _ := entity.NewUserSnapshot("user_1", "Alice", "https://avatar/alice.png", now)
	snapBot, _ := entity.NewUserSnapshot(botID, "Hoàng Nam", "https://assets.gymcompanion.app/avatars/athletes/athlete_1.png", now)
	snapshots := map[string]*entity.UserSnapshot{
		"user_1": snap1,
		botID:    snapBot,
	}

	resultDTO := dto.ToSoloMatchDTO(match, snapshots, map[string]bool{botID: true})

	var botParticipant *dto.ParticipantDTO
	for i := range resultDTO.Participants {
		if resultDTO.Participants[i].User.UserID == botID {
			botParticipant = &resultDTO.Participants[i]
			break
		}
	}

	if botParticipant == nil {
		t.Fatalf("expected bot participant")
	}

	if !botParticipant.IsBot {
		t.Errorf("expected isBot to be true")
	}

	// Bot DisplayName phải là tên người thật ngẫu nhiên, KHÔNG phải GymBot AI
	if botParticipant.User.DisplayName == "GymBot AI" || botParticipant.User.DisplayName == "" {
		t.Errorf("expected realistic human name, got %q", botParticipant.User.DisplayName)
	}

	// Avatar URL phải là avatar gymer tự nhiên, KHÔNG phải bot.png
	if botParticipant.User.AvatarURL == "https://assets.gymcompanion.app/avatars/bot.png" || botParticipant.User.AvatarURL == "" {
		t.Errorf("expected realistic athlete avatar, got %q", botParticipant.User.AvatarURL)
	}
}
