package service

import (
	"math"
	"strings"

	"github.com/viethung213/gym-companion/internal/workout_execution/domain/vo"
)

// RuleEvaluator defines the interface for evaluating landmarks against exercise motion rules.
type RuleEvaluator interface {
	Evaluate(landmarks []vo.SkeletonPoint, rule *vo.ExerciseMotionRule, cameraAngle string) vo.EvaluationResult
}

// DefaultRuleEvaluator implements RuleEvaluator using 3D vector geometry.
type DefaultRuleEvaluator struct{}

var _ RuleEvaluator = (*DefaultRuleEvaluator)(nil)

// NewDefaultRuleEvaluator constructs a DefaultRuleEvaluator.
func NewDefaultRuleEvaluator() *DefaultRuleEvaluator {
	return &DefaultRuleEvaluator{}
}

// CalculateJointAngle computes the angle in degrees at vertex B formed by line segments BA and BC.
// It seamlessly supports both 2D landmarks (Z=0) and 3D landmarks (Z!=0).
func CalculateJointAngle(a, b, c vo.SkeletonPoint) (float32, bool) {
	baX := float64(a.X - b.X)
	baY := float64(a.Y - b.Y)
	baZ := float64(a.Z - b.Z)

	bcX := float64(c.X - b.X)
	bcY := float64(c.Y - b.Y)
	bcZ := float64(c.Z - b.Z)

	dot := baX*bcX + baY*bcY + baZ*bcZ
	magBA := math.Sqrt(baX*baX + baY*baY + baZ*baZ)
	magBC := math.Sqrt(bcX*bcX + bcY*bcY + bcZ*bcZ)

	if magBA < 1e-6 || magBC < 1e-6 {
		return 0, false
	}

	cosTheta := dot / (magBA * magBC)
	if cosTheta > 1.0 {
		cosTheta = 1.0
	} else if cosTheta < -1.0 {
		cosTheta = -1.0
	}

	angle := math.Acos(cosTheta) * (180.0 / math.Pi)
	return float32(angle), true
}

// CalculatePointToLineDistanceRatio computes the perpendicular distance ratio from vertex B (e.g. hip)
// to the line segment AC (e.g. shoulder to ankle), normalized by the distance AB (torso length).
func CalculatePointToLineDistanceRatio(a, b, c vo.SkeletonPoint) (float32, bool) {
	torsoDx := float64(b.X - a.X)
	torsoDy := float64(b.Y - a.Y)
	torsoLen := math.Sqrt(torsoDx*torsoDx + torsoDy*torsoDy)
	if torsoLen < 1e-6 {
		return 0, false
	}

	lineDx := float64(c.X - a.X)
	lineDy := float64(c.Y - a.Y)
	lineLen := math.Sqrt(lineDx*lineDx + lineDy*lineDy)
	if lineLen < 1e-6 {
		return 0, false
	}

	cross := lineDx*float64(b.Y-a.Y) - lineDy*float64(b.X-a.X)
	dist := math.Abs(cross / lineLen)
	ratio := dist / torsoLen
	return float32(ratio), true
}

// Evaluate applies the rules defined in rule against the provided landmarks.
func (e *DefaultRuleEvaluator) Evaluate(landmarks []vo.SkeletonPoint, rule *vo.ExerciseMotionRule, cameraAngle string) vo.EvaluationResult {
	if rule == nil {
		return vo.EvaluationResult{
			Passed:         false,
			FormScore:      0,
			DetectedErrors: []string{"ERR_RULE_NOT_CONFIGURED"},
			AudioCue:       "Bài tập chưa có quy tắc chấm form!",
		}
	}

	if len(landmarks) == 0 {
		return vo.EvaluationResult{
			Passed:         false,
			FormScore:      0,
			DetectedErrors: []string{"ERR_NO_PERSON_DETECTED"},
			AudioCue:       "Vui lòng đứng vào trong khung hình camera!",
		}
	}

	landmarkMap := make(map[int]vo.SkeletonPoint, len(landmarks))
	for _, lm := range landmarks {
		landmarkMap[lm.Index] = lm
	}

	minConfidence := rule.MinConfidence
	if minConfidence <= 0 {
		minConfidence = 0.5
	}

	var (
		detectedErrors  []string
		firstAudioCue   string
		totalScore      float32 = 100.0
		primaryAngle    float32
		hasPrimaryAngle bool
		trajectoryAngle float32
	)

	// 1. Calculate trajectory angle directly from PhaseDetection configuration if defined
	if rule.PhaseDetection != nil && len(rule.PhaseDetection.Calculation.Joints) >= 3 {
		j := rule.PhaseDetection.Calculation.Joints
		pA, okA := landmarkMap[j[0]]
		pB, okB := landmarkMap[j[1]]
		pC, okC := landmarkMap[j[2]]

		if okA && okB && okC && pA.Visibility >= minConfidence && pB.Visibility >= minConfidence && pC.Visibility >= minConfidence {
			if ang, ok := CalculateJointAngle(pA, pB, pC); ok {
				trajectoryAngle = ang
			}
		}
	}

	activeView := strings.ToLower(strings.TrimSpace(cameraAngle))

	// 2. Evaluate form criteria in rule.Rules
	for _, criterion := range rule.Rules {
		// Filter by camera view if specified on the criterion
		if !matchesView(criterion.ApplyForView, activeView) {
			continue
		}

		if len(criterion.Joints) < 3 {
			continue
		}

		pA, okA := landmarkMap[criterion.Joints[0]]
		pB, okB := landmarkMap[criterion.Joints[1]]
		pC, okC := landmarkMap[criterion.Joints[2]]

		if !okA || !okB || !okC {
			continue
		}

		if pA.Visibility < minConfidence || pB.Visibility < minConfidence || pC.Visibility < minConfidence {
			continue
		}

		var (
			metricVal float32
			ok        bool
		)

		isDistance := criterion.Formula == vo.FormulaDistance ||
			criterion.Name == "signed_hip_y_diff"

		if isDistance {
			metricVal, ok = CalculatePointToLineDistanceRatio(pA, pB, pC)
		} else {
			metricVal, ok = CalculateJointAngle(pA, pB, pC)
			if ok && trajectoryAngle == 0 && !hasPrimaryAngle {
				primaryAngle = metricVal
				hasPrimaryAngle = true
			}
		}

		if !ok {
			continue
		}

		penalty := criterion.PenaltyPerError
		if penalty <= 0 {
			penalty = 5.0
		}

		if criterion.ValidRange.Min > 0 && metricVal < criterion.ValidRange.Min {
			errCode := criterion.ErrorCodeBelow
			if errCode == "" {
				errCode = "ERR_ANGLE_TOO_ACUTE"
			}
			detectedErrors = append(detectedErrors, errCode)
			totalScore -= penalty
			if firstAudioCue == "" && criterion.AudioCue != "" {
				firstAudioCue = criterion.AudioCue
			}
		} else if criterion.ValidRange.Max > 0 && metricVal > criterion.ValidRange.Max {
			errCode := criterion.ErrorCodeAbove
			if errCode == "" {
				errCode = "ERR_ANGLE_TOO_OBTUSE"
			}
			detectedErrors = append(detectedErrors, errCode)
			totalScore -= penalty
			if firstAudioCue == "" && criterion.AudioCue != "" {
				firstAudioCue = criterion.AudioCue
			}
		}
	}

	if totalScore < 0 {
		totalScore = 0
	}

	passed := len(detectedErrors) == 0

	finalAngle := trajectoryAngle
	if finalAngle == 0 && hasPrimaryAngle {
		finalAngle = primaryAngle
	}

	return vo.EvaluationResult{
		Passed:          passed,
		FormScore:       totalScore,
		DetectedErrors:  detectedErrors,
		AudioCue:        firstAudioCue,
		CalculatedAngle: finalAngle,
	}
}

func matchesView(criterionView, activeView string) bool {
	criterionView = strings.ToLower(strings.TrimSpace(criterionView))
	activeView = strings.ToLower(strings.TrimSpace(activeView))
	if criterionView == "" || criterionView == "all" || activeView == "" {
		return true
	}
	views := strings.Split(criterionView, ",")
	for _, v := range views {
		if strings.TrimSpace(v) == activeView {
			return true
		}
	}
	return false
}
