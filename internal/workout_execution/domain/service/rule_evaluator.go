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

// DefaultRuleEvaluator implements RuleEvaluator using 2D/3D vector geometry and COCO-17 / MediaPipe biomechanics.
type DefaultRuleEvaluator struct{}

var _ RuleEvaluator = (*DefaultRuleEvaluator)(nil)

// NewDefaultRuleEvaluator constructs a DefaultRuleEvaluator.
func NewDefaultRuleEvaluator() *DefaultRuleEvaluator {
	return &DefaultRuleEvaluator{}
}

// cocoBilateralMirror maps COCO-17 left-side joint indices to right-side joint indices.
func cocoBilateralMirror(j int) (int, bool) {
	switch j {
	case 1:
		return 2, true // eye
	case 3:
		return 4, true // ear
	case 5:
		return 6, true // shoulder
	case 7:
		return 8, true // elbow
	case 9:
		return 10, true // wrist
	case 11:
		return 12, true // hip
	case 13:
		return 14, true // knee
	case 15:
		return 16, true // ankle
	default:
		return 0, false
	}
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

// CalculateVerticalAngle computes the angle in degrees [0, 90] between segment AB and the vertical axis.
func CalculateVerticalAngle(a, b vo.SkeletonPoint) (float32, bool) {
	dx := math.Abs(float64(a.X - b.X))
	dy := math.Abs(float64(a.Y - b.Y))
	if dx < 1e-6 && dy < 1e-6 {
		return 0, false
	}
	angle := math.Atan2(dx, dy) * (180.0 / math.Pi)
	return float32(angle), true
}

// CalculateHorizontalAngle computes the angle in degrees [0, 90] between segment AB and the horizontal axis.
func CalculateHorizontalAngle(a, b vo.SkeletonPoint) (float32, bool) {
	dx := math.Abs(float64(a.X - b.X))
	dy := math.Abs(float64(a.Y - b.Y))
	if dx < 1e-6 && dy < 1e-6 {
		return 0, false
	}
	angle := math.Atan2(dy, dx) * (180.0 / math.Pi)
	return float32(angle), true
}

// CalculateHorizontalOffsetRatio computes |a.X - b.X| normalized by torsoLen.
func CalculateHorizontalOffsetRatio(a, b vo.SkeletonPoint, torsoLen float64) (float32, bool) {
	if torsoLen < 1e-6 {
		dx := float64(b.X - a.X)
		dy := float64(b.Y - a.Y)
		torsoLen = math.Sqrt(dx*dx + dy*dy)
		if torsoLen < 1e-6 {
			return 0, false
		}
	}
	offset := math.Abs(float64(b.X-a.X)) / torsoLen
	return float32(offset), true
}

// CalculateVerticalDiffRatio computes |a.Y - b.Y| normalized by torsoLen.
func CalculateVerticalDiffRatio(a, b vo.SkeletonPoint, torsoLen float64) (float32, bool) {
	if torsoLen < 1e-6 {
		dx := float64(b.X - a.X)
		dy := float64(b.Y - a.Y)
		torsoLen = math.Sqrt(dx*dx + dy*dy)
		if torsoLen < 1e-6 {
			return 0, false
		}
	}
	diff := math.Abs(float64(b.Y-a.Y)) / torsoLen
	return float32(diff), true
}

func resolveTorsoLength(landmarkMap map[int]vo.SkeletonPoint, minConfidence float32) float64 {
	// Try COCO left torso (5 -> 11) and right torso (6 -> 12)
	sL, okSL := landmarkMap[5]
	hL, okHL := landmarkMap[11]
	var lenL float64
	if okSL && okHL && sL.Visibility >= minConfidence && hL.Visibility >= minConfidence {
		dx := float64(sL.X - hL.X)
		dy := float64(sL.Y - hL.Y)
		lenL = math.Sqrt(dx*dx + dy*dy)
	}

	sR, okSR := landmarkMap[6]
	hR, okHR := landmarkMap[12]
	var lenR float64
	if okSR && okHR && sR.Visibility >= minConfidence && hR.Visibility >= minConfidence {
		dx := float64(sR.X - hR.X)
		dy := float64(sR.Y - hR.Y)
		lenR = math.Sqrt(dx*dx + dy*dy)
	}

	if lenL > 1e-6 && lenR > 1e-6 {
		if sR.Visibility+hR.Visibility > sL.Visibility+hL.Visibility+0.05 {
			return lenR
		}
		return lenL
	}
	if lenL > 1e-6 {
		return lenL
	}
	if lenR > 1e-6 {
		return lenR
	}

	// Fallback to MediaPipe torso (11 -> 23) if used
	mpS, okMPS := landmarkMap[11]
	mpH, okMPH := landmarkMap[23]
	if okMPS && okMPH && mpS.Visibility >= minConfidence && mpH.Visibility >= minConfidence {
		dx := float64(mpS.X - mpH.X)
		dy := float64(mpS.Y - mpH.Y)
		return math.Sqrt(dx*dx + dy*dy)
	}
	return 0
}

// resolveDominantJoints selects either the specified joints or their COCO bilateral mirror
// when the mirrored side is clearly visible and the primary side is occluded or less visible.
func resolveDominantJoints(landmarkMap map[int]vo.SkeletonPoint, joints []int, minConfidence float32) ([]vo.SkeletonPoint, bool) {
	if len(joints) == 0 {
		return nil, false
	}

	primaryPts := make([]vo.SkeletonPoint, len(joints))
	primaryValid := true
	var primaryVisSum float32

	mirrorPts := make([]vo.SkeletonPoint, len(joints))
	mirrorValid := true
	var mirrorVisSum float32

	for i, j := range joints {
		p, ok := landmarkMap[j]
		if !ok || p.Visibility < minConfidence {
			primaryValid = false
		} else {
			primaryPts[i] = p
			primaryVisSum += p.Visibility
		}

		if mj, hasMirror := cocoBilateralMirror(j); hasMirror {
			mp, okM := landmarkMap[mj]
			if !okM || mp.Visibility < minConfidence {
				mirrorValid = false
			} else {
				mirrorPts[i] = mp
				mirrorVisSum += mp.Visibility
			}
		} else if j == 0 {
			// Nose (0) is central and shared by both sides
			if !ok || p.Visibility < minConfidence {
				mirrorValid = false
			} else {
				mirrorPts[i] = p
				mirrorVisSum += p.Visibility
			}
		} else {
			mirrorValid = false
		}
	}

	if mirrorValid && (!primaryValid || mirrorVisSum > primaryVisSum+0.05) {
		return mirrorPts, true
	}
	if primaryValid {
		return primaryPts, true
	}
	return nil, false
}

func calculateBilateralWidthRatio(landmarkMap map[int]vo.SkeletonPoint, joints []int, minConfidence float32) (float32, bool) {
	if len(joints) < 2 {
		return 0, false
	}
	jRef := joints[0]
	jTarget := joints[1]

	refR, okRefMirror := cocoBilateralMirror(jRef)
	targetR, okTargetMirror := cocoBilateralMirror(jTarget)
	if !okRefMirror || !okTargetMirror {
		return 0, false
	}

	pRefL, ok1 := landmarkMap[jRef]
	pRefR, ok2 := landmarkMap[refR]
	pTarL, ok3 := landmarkMap[jTarget]
	pTarR, ok4 := landmarkMap[targetR]
	if !ok1 || !ok2 || !ok3 || !ok4 {
		return 0, false
	}
	if pRefL.Visibility < minConfidence || pRefR.Visibility < minConfidence ||
		pTarL.Visibility < minConfidence || pTarR.Visibility < minConfidence {
		return 0, false
	}

	refWidth := math.Abs(float64(pRefL.X - pRefR.X))
	if refWidth < 1e-6 {
		return 0, false
	}
	tarWidth := math.Abs(float64(pTarL.X - pTarR.X))
	return float32(tarWidth / refWidth), true
}

func evaluateCriterionMetric(
	landmarkMap map[int]vo.SkeletonPoint,
	formula vo.FormulaType,
	name string,
	joints []int,
	minConfidence float32,
	torsoLen float64,
) (float32, bool) {
	if formula == vo.FormulaBilateralWidthRatio {
		return calculateBilateralWidthRatio(landmarkMap, joints, minConfidence)
	}

	// Special case: bilateral wrist/shoulder symmetry (e.g., wrist_symmetry with joints [9, 10] or [9])
	if mirrorJ, hasM := cocoBilateralMirror(joints[0]); formula == vo.FormulaVerticalDiffRatio && len(joints) == 2 && hasM && mirrorJ == joints[1] {
		pL, okL := landmarkMap[joints[0]]
		pR, okR := landmarkMap[joints[1]]
		if !okL || !okR || pL.Visibility < minConfidence || pR.Visibility < minConfidence {
			return 0, false
		}
		// Require minimum confidence 0.65 on both joints for bilateral symmetry to avoid false positives when one limb is partially foreshortened
		if pL.Visibility < 0.65 || pR.Visibility < 0.65 {
			return 0, false
		}
		return CalculateVerticalDiffRatio(pL, pR, torsoLen)
	}

	if name == "ankle_lift" && len(joints) == 2 {
		// Supine sit-up: ankle is lifted only when ankle Y rises noticeably above hip Y (in screen coords, Y_ankle < Y_hip)
		pts, ok := resolveDominantJoints(landmarkMap, joints, minConfidence)
		if !ok || torsoLen < 1e-6 {
			return 0, false
		}
		pHip, pAnkle := pts[0], pts[1]
		if pAnkle.Y >= pHip.Y {
			return 0, true
		}
		lift := float64(pHip.Y-pAnkle.Y) / torsoLen
		return float32(lift), true
	}

	if name == "knee_over_toe" && len(joints) >= 2 {
		// Evaluate both Left and Right knee-over-toe; return the max deviation across limbs
		jKneeL, jAnkleL := joints[0], joints[1]
		jKneeR, _ := cocoBilateralMirror(jKneeL)
		jAnkleR, _ := cocoBilateralMirror(jAnkleL)

		valL, okL := float32(0), false
		if pKL, ok1 := landmarkMap[jKneeL]; ok1 && pKL.Visibility >= minConfidence {
			if pAL, ok2 := landmarkMap[jAnkleL]; ok2 && pAL.Visibility >= minConfidence {
				valL, okL = CalculateHorizontalOffsetRatio(pKL, pAL, torsoLen)
			}
		}
		valR, okR := float32(0), false
		if pKR, ok1 := landmarkMap[jKneeR]; ok1 && pKR.Visibility >= minConfidence {
			if pAR, ok2 := landmarkMap[jAnkleR]; ok2 && pAR.Visibility >= minConfidence {
				valR, okR = CalculateHorizontalOffsetRatio(pKR, pAR, torsoLen)
			}
		}
		if okL && okR {
			if valR > valL {
				return valR, true
			}
			return valL, true
		}
		if okL {
			return valL, true
		}
		if okR {
			return valR, true
		}
	}

	if (name == "knee_angle" || name == "elbow_angle") && len(joints) == 3 {
		ptsL, okL := resolveDominantJoints(landmarkMap, joints, minConfidence)
		mj0, _ := cocoBilateralMirror(joints[0])
		mj1, _ := cocoBilateralMirror(joints[1])
		mj2, _ := cocoBilateralMirror(joints[2])
		mirrorJoints := []int{mj0, mj1, mj2}
		ptsR, okR := resolveDominantJoints(landmarkMap, mirrorJoints, minConfidence)
		if okL && okR {
			angL, _ := CalculateJointAngle(ptsL[0], ptsL[1], ptsL[2])
			angR, _ := CalculateJointAngle(ptsR[0], ptsR[1], ptsR[2])
			if angR < angL {
				return angR, true
			}
			return angL, true
		}
	}

	pts, ok := resolveDominantJoints(landmarkMap, joints, minConfidence)
	if !ok {
		return 0, false
	}

	if len(pts) >= 3 {
		if formula == vo.FormulaDistance || name == "signed_hip_y_diff" {
			return CalculatePointToLineDistanceRatio(pts[0], pts[1], pts[2])
		}
		return CalculateJointAngle(pts[0], pts[1], pts[2])
	}

	if len(pts) == 2 {
		switch formula {
		case vo.FormulaHorizontalAngle:
			return CalculateHorizontalAngle(pts[0], pts[1])
		case vo.FormulaVerticalAngle:
			return CalculateVerticalAngle(pts[0], pts[1])
		case vo.FormulaVerticalDiffRatio:
			return CalculateVerticalDiffRatio(pts[0], pts[1], torsoLen)
		case vo.FormulaHorizontalOffsetRatio, vo.FormulaDistance:
			return CalculateHorizontalOffsetRatio(pts[0], pts[1], torsoLen)
		default:
			return CalculateVerticalAngle(pts[0], pts[1])
		}
	}

	return 0, false
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

	torsoLen := resolveTorsoLength(landmarkMap, minConfidence)

	var (
		detectedErrors     []string
		firstAudioCue      string
		totalScore         float32 = 100.0
		primaryAngle       float32
		hasPrimaryAngle    bool
		trajectoryAngle    float32
		hasTrajectoryAngle bool
		activePhaseName    string
	)

	// 1. Calculate trajectory angle directly from PhaseDetection configuration if defined (supports 2 or 3 joints)
	if rule.PhaseDetection != nil && len(rule.PhaseDetection.Calculation.Joints) >= 2 {
		pdFormula := rule.PhaseDetection.Calculation.Formula
		if pdFormula == "" {
			if len(rule.PhaseDetection.Calculation.Joints) >= 3 {
				pdFormula = vo.FormulaAngle3D
			} else {
				pdFormula = vo.FormulaHorizontalAngle
			}
		}
		if ang, ok := evaluateCriterionMetric(
			landmarkMap,
			pdFormula,
			rule.PhaseDetection.Metric,
			rule.PhaseDetection.Calculation.Joints,
			minConfidence,
			torsoLen,
		); ok {
			trajectoryAngle = ang
			hasTrajectoryAngle = true
			activePhaseName = resolvePhaseName(rule.PhaseDetection.Thresholds, ang, rule.ChallengeType)
		}
	}

	activeView := strings.ToLower(strings.TrimSpace(cameraAngle))
	if activeView == "auto" {
		activeView = inferCameraView(landmarkMap, torsoLen, minConfidence)
	}

	// 2. Evaluate form criteria in rule.Rules
	for _, criterion := range rule.Rules {
		// Filter by camera view if specified on the criterion
		if !matchesView(criterion.ApplyForView, activeView) {
			continue
		}

		// Filter by phase if PhaseDetection is active and criterion restricts phases
		if hasTrajectoryAngle && !matchesPhase(criterion.ApplyInPhases, activePhaseName) {
			continue
		}

		if len(criterion.Joints) < 2 {
			continue
		}

		metricVal, ok := evaluateCriterionMetric(
			landmarkMap,
			criterion.Formula,
			criterion.Name,
			criterion.Joints,
			minConfidence,
			torsoLen,
		)
		if !ok {
			continue
		}

		if len(criterion.Joints) >= 3 && criterion.Formula == vo.FormulaAngle3D && !hasTrajectoryAngle && !hasPrimaryAngle {
			primaryAngle = metricVal
			hasPrimaryAngle = true
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
	if !hasTrajectoryAngle && hasPrimaryAngle {
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

func resolvePhaseName(th vo.PhaseThresholds, angle float32, challengeType vo.ChallengeType) string {
	if challengeType == vo.ChallengeTypeDuration {
		if (th.Idle.GT > 0 || th.Idle.GTE > 0 || th.Idle.LT > 0 || th.Idle.LTE > 0) && th.Idle.Matches(angle) {
			return "idle"
		}
		return "always"
	}
	if (th.Rest.GT > 0 || th.Rest.GTE > 0 || th.Rest.LT > 0 || th.Rest.LTE > 0) && th.Rest.Matches(angle) {
		return "rest"
	}
	if (th.Active.GT > 0 || th.Active.GTE > 0 || th.Active.LT > 0 || th.Active.LTE > 0) && th.Active.Matches(angle) {
		return "active"
	}
	if (th.Transition.GT > 0 || th.Transition.GTE > 0 || th.Transition.LT > 0 || th.Transition.LTE > 0) && th.Transition.Matches(angle) {
		return "transition"
	}
	return ""
}

func matchesPhase(applyInPhases []string, activePhase string) bool {
	if len(applyInPhases) == 0 || activePhase == "" {
		return true
	}
	for _, ph := range applyInPhases {
		p := strings.ToLower(strings.TrimSpace(ph))
		if p == "all" || p == activePhase {
			return true
		}
		if p == "always" && activePhase != "idle" {
			return true
		}
	}
	return false
}

func inferCameraView(landmarkMap map[int]vo.SkeletonPoint, torsoLen float64, minConfidence float32) string {
	if torsoLen <= 1e-6 {
		return "front"
	}
	sL, okL := landmarkMap[5]
	sR, okR := landmarkMap[6]
	if okL && okR && sL.Visibility >= minConfidence && sR.Visibility >= minConfidence {
		shoulderSpan := math.Abs(float64(sL.X - sR.X))
		if shoulderSpan/torsoLen >= 0.45 {
			return "front"
		}
		return "side"
	}
	return "side"
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
