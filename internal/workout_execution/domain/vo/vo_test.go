package vo_test

import (
	"testing"

	"github.com/viethung213/gym-companion/internal/workout_execution/domain/vo"
)

func TestRepLog(t *testing.T) {
	t.Run("NewRepLog defensive copy and getters", func(t *testing.T) {
		errs := []string{"ERR1"}
		angles := map[string]float32{"knee": 90.0}

		rep := vo.NewRepLog(1, 95.0, errs, angles)

		if rep.RepNumber != 1 || rep.ROMPercentage != 95.0 {
			t.Errorf("invalid rep log fields")
		}

		// Mutate original slice & map
		errs[0] = "MUTATED"
		angles["knee"] = 0.0

		copiedErrs := rep.GetErrorCodes()
		if len(copiedErrs) != 1 || copiedErrs[0] != "ERR1" {
			t.Errorf("got error codes = %v, want ERR1", copiedErrs)
		}

		copiedAngles := rep.GetJointAngles()
		if len(copiedAngles) != 1 || copiedAngles["knee"] != 90.0 {
			t.Errorf("got joint angles = %v, want 90.0", copiedAngles)
		}
	})

	t.Run("Empty RepLog getters return nil", func(t *testing.T) {
		var rep vo.RepLog
		if got := rep.GetErrorCodes(); got != nil {
			t.Errorf("got %v, want nil", got)
		}
		if got := rep.GetJointAngles(); got != nil {
			t.Errorf("got %v, want nil", got)
		}
	})
}

func TestSessionSummary(t *testing.T) {
	score := float32(92.5)
	summary := vo.NewSessionSummary(5, 500.0, &score, 8.5)

	if summary.TotalSets != 5 || summary.TotalVolume != 500.0 || summary.AverageRPE != 8.5 {
		t.Errorf("invalid session summary fields")
	}
	if summary.AverageFormScore == nil || *summary.AverageFormScore != 92.5 {
		t.Errorf("got AverageFormScore = %v, want 92.5", summary.AverageFormScore)
	}

	summaryNilScore := vo.NewSessionSummary(3, 300.0, nil, 7.0)
	if summaryNilScore.AverageFormScore != nil {
		t.Errorf("got AverageFormScore = %v, want nil", summaryNilScore.AverageFormScore)
	}
}

func TestMotionSpecVO(t *testing.T) {
	cal := vo.CalibrationConfig{MinDistanceMeters: 1.5, MaxDistanceMeters: 2.5, TargetAngle: 90.0}
	if cal.MinDistanceMeters != 1.5 {
		t.Errorf("invalid CalibrationConfig")
	}

	rules := vo.RepCountingRules{MinROMPercentage: 70.0}
	if rules.MinROMPercentage != 70.0 {
		t.Errorf("invalid RepCountingRules")
	}

	scoring := vo.FormScoringRules{PenaltyPerError: 5.0}
	if scoring.PenaltyPerError != 5.0 {
		t.Errorf("invalid FormScoringRules")
	}

	dialogue := vo.DialogueEngineConfig{
		PersonalityID: "coach1",
		Cooldowns:     map[string]float32{"ERR1": 5.0},
		DialogueMap: map[string]vo.DialogueSeverities{
			"ERR1": {
				Severity1: []vo.DialogueOption{{Text: "Keep back straight", AudioURL: "http://audio"}},
			},
		},
	}

	if dialogue.PersonalityID != "coach1" || len(dialogue.DialogueMap["ERR1"].Severity1) != 1 {
		t.Errorf("invalid DialogueEngineConfig struct")
	}
}
