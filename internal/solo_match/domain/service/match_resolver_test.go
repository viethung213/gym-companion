package service_test

import (
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/solo_match/domain/entity"
	"github.com/viethung213/gym-companion/internal/solo_match/domain/service"
	valueobject "github.com/viethung213/gym-companion/internal/solo_match/domain/value_object"
)

func TestMatchResolver_ResolveOutcome(t *testing.T) {
	resolver := service.NewMatchResolver()
	now := time.Now().UTC()

	tests := []struct {
		name        string
		cType       valueobject.ChallengeType
		score1      float64
		score2      float64
		p1Completed bool
		p2Completed bool
		wantWinner  string
		wantDraw    bool
		wantErr     bool
	}{
		{
			name:        "p1 wins reps",
			cType:       valueobject.ChallengeTypeReps,
			score1:      30,
			score2:      25,
			p1Completed: true,
			p2Completed: true,
			wantWinner:  "user_1",
			wantDraw:    false,
			wantErr:     false,
		},
		{
			name:        "p2 wins reps",
			cType:       valueobject.ChallengeTypeReps,
			score1:      15,
			score2:      20,
			p1Completed: true,
			p2Completed: true,
			wantWinner:  "user_2",
			wantDraw:    false,
			wantErr:     false,
		},
		{
			name:        "draw reps",
			cType:       valueobject.ChallengeTypeReps,
			score1:      20,
			score2:      20,
			p1Completed: true,
			p2Completed: true,
			wantWinner:  "",
			wantDraw:    true,
			wantErr:     false,
		},
		{
			name:        "duration p2 wins",
			cType:       valueobject.ChallengeTypeDuration,
			score1:      120,
			score2:      180,
			p1Completed: true,
			p2Completed: true,
			wantWinner:  "user_2",
			wantDraw:    false,
			wantErr:     false,
		},
		{
			name:        "error when not completed",
			cType:       valueobject.ChallengeTypeReps,
			score1:      10,
			score2:      10,
			p1Completed: true,
			p2Completed: false,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			p1, _ := entity.NewMatchParticipant("part_1", "match_1", "user_1")
			p2, _ := entity.NewMatchParticipant("part_2", "match_1", "user_2")

			if tt.p1Completed {
				_ = p1.RecordResult(tt.score1, now)
			}
			if tt.p2Completed {
				_ = p2.RecordResult(tt.score2, now)
			}

			winnerID, isDraw, err := resolver.ResolveOutcome(tt.cType, p1, p2)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ResolveOutcome() err got %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr {
				if winnerID != tt.wantWinner {
					t.Errorf("ResolveOutcome() winnerID got %v, want %v", winnerID, tt.wantWinner)
				}
				if isDraw != tt.wantDraw {
					t.Errorf("ResolveOutcome() isDraw got %v, want %v", isDraw, tt.wantDraw)
				}
			}
		})
	}
}
