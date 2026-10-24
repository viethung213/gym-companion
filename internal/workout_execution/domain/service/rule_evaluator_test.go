package service

import (
	"math"
	"testing"

	"github.com/viethung213/gym-companion/internal/workout_execution/domain/vo"
)

func TestCalculateJointAngle(t *testing.T) {
	tests := []struct {
		name       string
		pA, pB, pC vo.SkeletonPoint
		wantAngle  float32
		wantOk     bool
	}{
		{
			name:      "Pure 2D right angle 90 degrees (Z=0)",
			pA:        vo.SkeletonPoint{X: 1, Y: 0, Z: 0},
			pB:        vo.SkeletonPoint{X: 0, Y: 0, Z: 0},
			pC:        vo.SkeletonPoint{X: 0, Y: 1, Z: 0},
			wantAngle: 90.0,
			wantOk:    true,
		},
		{
			name:      "Pure 2D straight line 180 degrees (Z=0)",
			pA:        vo.SkeletonPoint{X: -1, Y: 0, Z: 0},
			pB:        vo.SkeletonPoint{X: 0, Y: 0, Z: 0},
			pC:        vo.SkeletonPoint{X: 1, Y: 0, Z: 0},
			wantAngle: 180.0,
			wantOk:    true,
		},
		{
			name:      "3D right angle 90 degrees across X and Z axes",
			pA:        vo.SkeletonPoint{X: 1, Y: 0, Z: 0},
			pB:        vo.SkeletonPoint{X: 0, Y: 0, Z: 0},
			pC:        vo.SkeletonPoint{X: 0, Y: 0, Z: 1},
			wantAngle: 90.0,
			wantOk:    true,
		},
		{
			name:      "Zero length vector returns false",
			pA:        vo.SkeletonPoint{X: 0, Y: 0, Z: 0},
			pB:        vo.SkeletonPoint{X: 0, Y: 0, Z: 0},
			pC:        vo.SkeletonPoint{X: 1, Y: 0, Z: 0},
			wantAngle: 0,
			wantOk:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotAngle, gotOk := CalculateJointAngle(tt.pA, tt.pB, tt.pC)
			if gotOk != tt.wantOk {
				t.Fatalf("CalculateJointAngle() gotOk = %v, wantOk = %v", gotOk, tt.wantOk)
			}
			if tt.wantOk && math.Abs(float64(gotAngle-tt.wantAngle)) > 0.01 {
				t.Errorf("CalculateJointAngle() gotAngle = %v, wantAngle = %v", gotAngle, tt.wantAngle)
			}
		})
	}
}

func TestDefaultRuleEvaluator_Evaluate(t *testing.T) {
	evaluator := NewDefaultRuleEvaluator()

	rule := &vo.ExerciseMotionRule{
		ExerciseID:    "plank_01",
		ChallengeType: vo.ChallengeTypeDuration,
		MinConfidence: 0.5,
		Rules: []vo.RuleCriterion{
			{
				Name:    "spine_alignment",
				Formula: vo.FormulaAngle3D,
				Joints:  []int{11, 23, 27},
				ValidRange: vo.ValidAngleRange{
					Min: 165.0,
					Max: 195.0,
				},
				ErrorCodeBelow:  "ERR_HIPS_TOO_HIGH",
				ErrorCodeAbove:  "ERR_BACK_ARCH",
				AudioCue:        "Thẳng lưng lên!",
				PenaltyPerError: 10.0,
			},
		},
	}

	t.Run("Valid form passes evaluation", func(t *testing.T) {
		landmarks := []vo.SkeletonPoint{
			{Index: 11, X: -1, Y: 0, Z: 0, Visibility: 0.9},
			{Index: 23, X: 0, Y: 0, Z: 0, Visibility: 0.9},
			{Index: 27, X: 1, Y: 0, Z: 0, Visibility: 0.9},
		}

		result := evaluator.Evaluate(landmarks, rule, "")
		if !result.Passed {
			t.Errorf("expected evaluation to pass, got errors: %v", result.DetectedErrors)
		}
		if result.FormScore != 100.0 {
			t.Errorf("expected form score 100, got %v", result.FormScore)
		}
	})

	t.Run("Hips too high fails evaluation with below error", func(t *testing.T) {
		// ~90 degrees angle instead of 165-195
		landmarks := []vo.SkeletonPoint{
			{Index: 11, X: 1, Y: 0, Z: 0, Visibility: 0.9},
			{Index: 23, X: 0, Y: 0, Z: 0, Visibility: 0.9},
			{Index: 27, X: 0, Y: 1, Z: 0, Visibility: 0.9},
		}

		result := evaluator.Evaluate(landmarks, rule, "")
		if result.Passed {
			t.Errorf("expected evaluation to fail")
		}
		if len(result.DetectedErrors) != 1 || result.DetectedErrors[0] != "ERR_HIPS_TOO_HIGH" {
			t.Errorf("expected error ERR_HIPS_TOO_HIGH, got %v", result.DetectedErrors)
		}
		if result.AudioCue != "Thẳng lưng lên!" {
			t.Errorf("expected audio cue 'Thẳng lưng lên!', got '%s'", result.AudioCue)
		}
		if result.FormScore != 90.0 {
			t.Errorf("expected form score 90, got %v", result.FormScore)
		}
	})

	t.Run("Low visibility landmark skipped gracefully", func(t *testing.T) {
		landmarks := []vo.SkeletonPoint{
			{Index: 11, X: -1, Y: 0, Z: 0, Visibility: 0.2}, // below minConfidence 0.5
			{Index: 23, X: 0, Y: 0, Z: 0, Visibility: 0.9},
			{Index: 27, X: 1, Y: 0, Z: 0, Visibility: 0.9},
		}

		result := evaluator.Evaluate(landmarks, rule, "")
		if !result.Passed {
			t.Errorf("expected pass when skipped due to low visibility, got errors: %v", result.DetectedErrors)
		}
	})

	t.Run("Empty landmarks returns ERR_NO_PERSON_DETECTED", func(t *testing.T) {
		result := evaluator.Evaluate([]vo.SkeletonPoint{}, rule, "")
		if result.Passed {
			t.Errorf("expected fail on empty landmarks")
		}
		if len(result.DetectedErrors) != 1 || result.DetectedErrors[0] != "ERR_NO_PERSON_DETECTED" {
			t.Errorf("expected ERR_NO_PERSON_DETECTED, got %v", result.DetectedErrors)
		}
		if result.AudioCue != "Vui lòng đứng vào trong khung hình camera!" {
			t.Errorf("expected cue to stand in frame, got %s", result.AudioCue)
		}
	})

	t.Run("Nil rule returns ERR_RULE_NOT_CONFIGURED", func(t *testing.T) {
		result := evaluator.Evaluate([]vo.SkeletonPoint{{Index: 11, X: 1, Y: 1}}, nil, "")
		if result.Passed {
			t.Errorf("expected fail on nil rule")
		}
		if len(result.DetectedErrors) != 1 || result.DetectedErrors[0] != "ERR_RULE_NOT_CONFIGURED" {
			t.Errorf("expected ERR_RULE_NOT_CONFIGURED, got %v", result.DetectedErrors)
		}
	})

	t.Run("PhaseDetection calculation computes trajectory angle directly", func(t *testing.T) {
		ruleWithPhase := &vo.ExerciseMotionRule{
			ExerciseID: "curl_01",
			PhaseDetection: &vo.PhaseDetectionConfig{
				Metric: "elbow_angle",
				Calculation: vo.PhaseCalculation{
					Joints: []int{5, 7, 9}, // Shoulder, Elbow, Wrist
				},
			},
			Rules: []vo.RuleCriterion{
				{
					Name:       "spine_sway",
					Joints:     []int{1, 2, 3}, // Completely different joints for form check
					ValidRange: vo.ValidAngleRange{Min: 160, Max: 180},
				},
			},
		}

		landmarks := []vo.SkeletonPoint{
			// Phase detection joints: 90 deg right angle
			{Index: 5, X: 1, Y: 0, Z: 0, Visibility: 0.9},
			{Index: 7, X: 0, Y: 0, Z: 0, Visibility: 0.9},
			{Index: 9, X: 0, Y: 1, Z: 0, Visibility: 0.9},
			// Form check joints: 180 deg straight line
			{Index: 1, X: -1, Y: 0, Z: 0, Visibility: 0.9},
			{Index: 2, X: 0, Y: 0, Z: 0, Visibility: 0.9},
			{Index: 3, X: 1, Y: 0, Z: 0, Visibility: 0.9},
		}

		res := evaluator.Evaluate(landmarks, ruleWithPhase, "")
		if !res.Passed {
			t.Fatalf("expected pass, got errors: %v", res.DetectedErrors)
		}
		// CalculatedAngle MUST be 90 deg (from PhaseDetection joints [5, 7, 9]), NOT 180 deg (from Rule [1, 2, 3])!
		if math.Abs(float64(res.CalculatedAngle-90.0)) > 0.1 {
			t.Errorf("expected CalculatedAngle 90 deg from PhaseDetection, got %f", res.CalculatedAngle)
		}
	})

	t.Run("Camera angle filters rules by view", func(t *testing.T) {
		ruleMultiView := &vo.ExerciseMotionRule{
			ExerciseID:    "multi_view_test",
			MinConfidence: 0.5,
			Rules: []vo.RuleCriterion{
				{
					Name:            "side_only_rule",
					ApplyForView:    "side",
					Joints:          []int{11, 23, 27},
					ValidRange:      vo.ValidAngleRange{Min: 160.0, Max: 180.0},
					ErrorCodeBelow:  "ERR_SIDE_FAIL",
					PenaltyPerError: 15.0,
				},
				{
					Name:            "front_only_rule",
					ApplyForView:    "front",
					Joints:          []int{11, 23, 27},
					ValidRange:      vo.ValidAngleRange{Min: 160.0, Max: 180.0},
					ErrorCodeBelow:  "ERR_FRONT_FAIL",
					PenaltyPerError: 20.0,
				},
			},
		}

		// Bad landmarks (~90 deg angle, will fail any 160-180 range)
		badLandmarks := []vo.SkeletonPoint{
			{Index: 11, X: 1, Y: 0, Z: 0, Visibility: 0.9},
			{Index: 23, X: 0, Y: 0, Z: 0, Visibility: 0.9},
			{Index: 27, X: 0, Y: 1, Z: 0, Visibility: 0.9},
		}

		// When evaluating with cameraAngle = "side", front_only_rule should be skipped
		resSide := evaluator.Evaluate(badLandmarks, ruleMultiView, "side")
		if len(resSide.DetectedErrors) != 1 || resSide.DetectedErrors[0] != "ERR_SIDE_FAIL" {
			t.Errorf("expected only ERR_SIDE_FAIL, got %v", resSide.DetectedErrors)
		}

		// When evaluating with cameraAngle = "front", side_only_rule should be skipped
		resFront := evaluator.Evaluate(badLandmarks, ruleMultiView, "front")
		if len(resFront.DetectedErrors) != 1 || resFront.DetectedErrors[0] != "ERR_FRONT_FAIL" {
			t.Errorf("expected only ERR_FRONT_FAIL, got %v", resFront.DetectedErrors)
		}

		// When evaluating without camera angle, both rules should run
		resAll := evaluator.Evaluate(badLandmarks, ruleMultiView, "")
		if len(resAll.DetectedErrors) != 2 {
			t.Errorf("expected both rules to evaluate when view is unspecified, got %v", resAll.DetectedErrors)
		}
	})
}
