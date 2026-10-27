package persistence_test

import (
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/solo_match/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/solo_match/domain/entity"
	valueobject "github.com/viethung213/gym-companion/internal/solo_match/domain/value_object"
	"github.com/viethung213/gym-companion/internal/solo_match/infrastructure/persistence"
)

func TestModels_Mapping(t *testing.T) {
	now := time.Now().UTC()

	// 1. User Snapshot Model
	snapshot, err := entity.NewUserSnapshot("u_1", "Nguyen Van A", "https://avatar.png", now)
	if err != nil {
		t.Fatalf("NewUserSnapshot err: %v", err)
	}
	modelSnapshot := persistence.FromDomainUserSnapshot(snapshot)
	if modelSnapshot.UserID != "u_1" || modelSnapshot.DisplayName != "Nguyen Van A" {
		t.Errorf("FromDomainUserSnapshot mismatch")
	}
	recoveredSnapshot, err := modelSnapshot.ToDomain()
	if err != nil {
		t.Fatalf("ToDomain snapshot err: %v", err)
	}
	if recoveredSnapshot.UserID() != "u_1" || recoveredSnapshot.DisplayName() != "Nguyen Van A" {
		t.Errorf("recoveredSnapshot mismatch")
	}

	// 2. Participant Model
	part, err := entity.NewMatchParticipant("p_1", "m_1", "u_1")
	if err != nil {
		t.Fatalf("NewMatchParticipant err: %v", err)
	}
	_ = part.RecordResult(25, now)
	modelPart := persistence.FromDomainParticipant(part)
	if modelPart.ID != "p_1" || modelPart.ScoreValue != 25 {
		t.Errorf("FromDomainParticipant mismatch")
	}
	recoveredPart, err := modelPart.ToDomain()
	if err != nil {
		t.Fatalf("ToDomain participant err: %v", err)
	}
	if recoveredPart.ID() != "p_1" || recoveredPart.Score().Value() != 25 {
		t.Errorf("recoveredPart mismatch")
	}

	// 3. Solo Match Model
	part2, _ := entity.NewMatchParticipant("p_2", "m_1", "u_2")
	_ = part2.RecordResult(20, now)

	match, err := aggregate.NewSoloMatch(
		"m_1", "ex_1", "Push Up", valueobject.ChallengeTypeReps,
		[]*entity.MatchParticipant{part, part2}, now,
	)
	if err != nil {
		t.Fatalf("NewSoloMatch err: %v", err)
	}
	modelMatch := persistence.FromDomainSoloMatch(match)
	if modelMatch.ID != "m_1" || len(modelMatch.Participants) != 2 {
		t.Errorf("FromDomainSoloMatch mismatch")
	}
	recoveredMatch, err := modelMatch.ToDomain()
	if err != nil {
		t.Fatalf("ToDomain match err: %v", err)
	}
	if recoveredMatch.ID() != "m_1" || len(recoveredMatch.Participants()) != 2 {
		t.Errorf("recoveredMatch mismatch")
	}

	// 4. System Bot Model
	botModel := persistence.SystemBotModel{
		UserID:          "u_bot_1",
		DifficultyLevel: "INTERMEDIATE",
		IsActive:        true,
		UserSnapshot:    modelSnapshot,
	}
	if botModel.TableName() != "solo_match.system_bots" {
		t.Errorf("SystemBotModel TableName mismatch: %s", botModel.TableName())
	}
	if botModel.UserSnapshot.DisplayName != "Nguyen Van A" {
		t.Errorf("UserSnapshot mapping mismatch")
	}
}
