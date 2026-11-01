package service

import (
	"context"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/workout_execution/domain/vo"
	"github.com/viethung213/gym-companion/internal/workout_execution/infrastructure/rule"
)

// poseGenFunc generates a realistic 17-keypoint COCO skeleton for a given normalized progress t in [0, 1]
// where t=0 is the REST position and t=1 is the peak ACTIVE contraction.
type poseGenFunc func(t float64, faultMode string, rightSideDominant bool) []vo.SkeletonPoint

type exerciseBenchSpec struct {
	slug              string
	cameraAngle       string
	expectedErrorCode string
	poseGen           poseGenFunc
}

func makeBaseSkeleton(rightSideDominant bool) []vo.SkeletonPoint {
	visL := float32(0.96)
	visR := float32(0.92)
	if rightSideDominant {
		// Simulate Left side self-occluded (camera on the user's right side)
		visL = float32(0.15)
		visR = float32(0.98)
	}
	pts := make([]vo.SkeletonPoint, 17)
	for i := 0; i < 17; i++ {
		vis := visL
		if i%2 == 0 && i > 0 {
			vis = visR
		}
		pts[i] = vo.SkeletonPoint{Index: i, X: 0.5, Y: 0.5, Visibility: vis}
	}
	pts[0].Visibility = 0.98
	return pts
}

func setBilateral(pts []vo.SkeletonPoint, leftIdx, rightIdx int, x, y float64, spreadX float64) {
	pts[leftIdx].X = float32(x - spreadX)
	pts[leftIdx].Y = float32(y)
	pts[rightIdx].X = float32(x + spreadX)
	pts[rightIdx].Y = float32(y)
}

// pointFromAngle places a child joint at distance len from parent (px, py) such that the interior angle
// at parent between grandparent (gx, gy) and child is angleDeg.
// dirSign (+1 or -1) controls the rotation direction in the 2D plane.
func pointFromAngle(gx, gy, px, py, lenSeg, angleDeg float64, dirSign float64) (float64, float64) {
	vx := gx - px
	vy := gy - py
	baseAngle := math.Atan2(vy, vx)
	rad := angleDeg * (math.Pi / 180.0)
	targetAngle := baseAngle + dirSign*rad
	return px + lenSeg*math.Cos(targetAngle), py + lenSeg*math.Sin(targetAngle)
}

func getTwelveExerciseSpecs() []exerciseBenchSpec {
	return []exerciseBenchSpec{
		{
			slug:              "bicep_curl",
			cameraAngle:       "SIDE",
			expectedErrorCode: "ERR_ELBOW_SWAY",
			poseGen: func(t float64, faultMode string, rightSideDominant bool) []vo.SkeletonPoint {
				pts := makeBaseSkeleton(rightSideDominant)
				// Standing upright: Shoulder(0.50, 0.30), Hip(0.50, 0.60) -> torsoLen = 0.30
				sx, sy := 0.50, 0.30
				hx, hy := 0.50, 0.60
				if faultMode == "bad_form" {
					// Lean back spine sway + swing elbow forward
					sx = 0.44
				}
				setBilateral(pts, 5, 6, sx, sy, 0.0)
				setBilateral(pts, 11, 12, hx, hy, 0.0)
				setBilateral(pts, 13, 14, 0.50, 0.80, 0.0)
				setBilateral(pts, 15, 16, 0.50, 0.95, 0.0)

				ex, ey := sx, sy+0.15
				if faultMode == "bad_form" {
					// Elbow sway > 0.09 * torsoLen (0.30 * 0.25 = 0.075)
					ex = sx + 0.08
				}
				setBilateral(pts, 7, 8, ex, ey, 0.0)

				// Elbow angle: t=0 -> 170 deg, t=1 -> 60 deg (active <= 80)
				targetAngle := 170.0 - t*(170.0-60.0)
				if faultMode == "half_rep" {
					targetAngle = 170.0 - t*(170.0-105.0) // bottoms out at 105 deg (> 80)
				}
				wx, wy := pointFromAngle(sx, sy, ex, ey, 0.14, targetAngle, 1.0)
				setBilateral(pts, 9, 10, wx, wy, 0.0)
				return pts
			},
		},
		{
			slug:              "squat",
			cameraAngle:       "SIDE",
			expectedErrorCode: "ERR_TORSO_LEAN",
			poseGen: func(t float64, faultMode string, rightSideDominant bool) []vo.SkeletonPoint {
				pts := makeBaseSkeleton(rightSideDominant)
				// Ankle fixed at (0.50, 0.90), Knee at (0.52, 0.72)
				ax, ay := 0.50, 0.90
				kx, ky := 0.52, 0.72
				setBilateral(pts, 15, 16, ax, ay, 0.0)
				setBilateral(pts, 13, 14, kx, ky, 0.0)

				// Knee angle: t=0 -> 172 deg, t=1 -> 85 deg (active <= 100)
				kneeAngle := 172.0 - t*(172.0-85.0)
				if faultMode == "half_rep" {
					kneeAngle = 172.0 - t*(172.0-120.0) // stops at 120 deg (> 100)
				}
				hx, hy := pointFromAngle(ax, ay, kx, ky, 0.22, kneeAngle, -1.0)
				setBilateral(pts, 11, 12, hx, hy, 0.0)

				// Torso lean angle from vertical: good = 20 deg, bad_form = 55 deg (> 42 deg)
				leanDeg := 15.0 * t
				if faultMode == "bad_form" {
					leanDeg = 55.0
				}
				leanRad := leanDeg * (math.Pi / 180.0)
				torsoLen := 0.28
				sx := hx + torsoLen*math.Sin(leanRad)
				sy := hy - torsoLen*math.Cos(leanRad)
				setBilateral(pts, 5, 6, sx, sy, 0.0)
				return pts
			},
		},
		{
			slug:              "lunge",
			cameraAngle:       "SIDE",
			expectedErrorCode: "ERR_TORSO_LEAN",
			poseGen: func(t float64, faultMode string, rightSideDominant bool) []vo.SkeletonPoint {
				pts := makeBaseSkeleton(rightSideDominant)
				ax, ay := 0.50, 0.90
				kx, ky := 0.52, 0.72
				setBilateral(pts, 15, 16, ax, ay, 0.0)
				setBilateral(pts, 13, 14, kx, ky, 0.0)

				kneeAngle := 172.0 - t*(172.0-88.0)
				if faultMode == "half_rep" {
					kneeAngle = 172.0 - t*(172.0-150.0)
				}
				hx, hy := pointFromAngle(ax, ay, kx, ky, 0.22, kneeAngle, -1.0)
				setBilateral(pts, 11, 12, hx, hy, 0.0)

				// Upright torso in lunge: good = 8 deg, bad_form = 35 deg (> 22 deg)
				leanDeg := 8.0 * t
				if faultMode == "bad_form" {
					leanDeg = 35.0
				}
				leanRad := leanDeg * (math.Pi / 180.0)
				torsoLen := 0.28
				sx := hx + torsoLen*math.Sin(leanRad)
				sy := hy - torsoLen*math.Cos(leanRad)
				setBilateral(pts, 5, 6, sx, sy, 0.0)
				return pts
			},
		},
		{
			slug:              "deadlift",
			cameraAngle:       "SIDE",
			expectedErrorCode: "ERR_BAR_DRIFT",
			poseGen: func(t float64, faultMode string, rightSideDominant bool) []vo.SkeletonPoint {
				pts := makeBaseSkeleton(rightSideDominant)
				// Hip at (0.45, 0.58), Knee at (0.50, 0.76), Ankle at (0.50, 0.94) -> knee angle ~165 deg (>=115)
				hx, hy := 0.45, 0.58
				kx, ky := 0.50, 0.76
				ax, ay := 0.50, 0.94
				setBilateral(pts, 11, 12, hx, hy, 0.0)
				setBilateral(pts, 13, 14, kx, ky, 0.0)
				setBilateral(pts, 15, 16, ax, ay, 0.0)

				// Hip angle (Shoulder-Hip-Knee): t=0 -> 172 deg, t=1 -> 88 deg (active <= 112)
				hipAngle := 172.0 - t*(172.0-88.0)
				if faultMode == "half_rep" {
					hipAngle = 172.0 - t*(172.0-128.0)
				}
				sx, sy := pointFromAngle(kx, ky, hx, hy, 0.28, hipAngle, -1.0)
				setBilateral(pts, 5, 6, sx, sy, 0.0)

				// Wrist tracks close to ankle X (0.50). If bad_form -> bar_drift (> 0.52 * torsoLen)
				wx := ax + 0.02
				if faultMode == "bad_form" {
					wx = ax + 0.18 // |0.68 - 0.50| / 0.28 = 0.64 > 0.52
				}
				wy := sy + 0.22
				setBilateral(pts, 7, 8, (sx+wx)/2, (sy+wy)/2, 0.0)
				setBilateral(pts, 9, 10, wx, wy, 0.0)
				return pts
			},
		},
		{
			slug:              "push_up",
			cameraAngle:       "SIDE",
			expectedErrorCode: "ERR_SIGNED_HIP_Y_DIFF",
			poseGen: func(t float64, faultMode string, rightSideDominant bool) []vo.SkeletonPoint {
				pts := makeBaseSkeleton(rightSideDominant)
				// Horizontal body: Ankle(0.20, 0.70), Shoulder(0.76, 0.54)
				ax, ay := 0.20, 0.70
				sx, sy := 0.76, 0.54
				// Collinear hip midpoint at (0.48, 0.62)
				hx, hy := 0.48, 0.62
				if faultMode == "bad_form" {
					// Sagging hip way below the Shoulder-Ankle line
					hy = 0.76
				}
				setBilateral(pts, 5, 6, sx, sy, 0.0)
				setBilateral(pts, 11, 12, hx, hy, 0.0)
				setBilateral(pts, 13, 14, 0.34, 0.66, 0.0)
				setBilateral(pts, 15, 16, ax, ay, 0.0)

				// Elbow angle (Shoulder-Elbow-Wrist): t=0 -> 168 deg, t=1 -> 85 deg (active <= 98)
				elbowAngle := 168.0 - t*(168.0-85.0)
				if faultMode == "half_rep" {
					elbowAngle = 168.0 - t*(168.0-120.0)
				}
				ex, ey := sx-0.06, sy+0.12
				wx, wy := pointFromAngle(sx, sy, ex, ey, 0.12, elbowAngle, 1.0)
				setBilateral(pts, 7, 8, ex, ey, 0.0)
				setBilateral(pts, 9, 10, wx, wy, 0.0)
				return pts
			},
		},
		{
			slug:              "bench_press",
			cameraAngle:       "FRONT",
			expectedErrorCode: "ERR_WRIST_SYMMETRY",
			poseGen: func(t float64, faultMode string, rightSideDominant bool) []vo.SkeletonPoint {
				pts := makeBaseSkeleton(rightSideDominant)
				// Keep both wrists visible for bilateral wrist_symmetry check, while testing rightSideDominant on elbow
				pts[9].Visibility = 0.95
				pts[10].Visibility = 0.95
				sx, sy := 0.50, 0.55
				hx, hy := 0.22, 0.55 // torsoLen = 0.28
				setBilateral(pts, 5, 6, sx, sy, 0.0)
				setBilateral(pts, 11, 12, hx, hy, 0.0)
				setBilateral(pts, 13, 14, 0.12, 0.70, 0.0)
				setBilateral(pts, 15, 16, 0.12, 0.88, 0.0)

				// Place Elbow at ~60 deg Hip-Shoulder-Elbow angle (<= 95 deg max for elbow_flare)
				ex, ey := pointFromAngle(hx, hy, sx, sy, 0.14, 60.0, 1.0)
				setBilateral(pts, 7, 8, ex, ey, 0.0)

				// Elbow angle: t=0 -> 168 deg, t=1 -> 82 deg (active <= 95)
				elbowAngle := 168.0 - t*(168.0-82.0)
				if faultMode == "half_rep" {
					elbowAngle = 168.0 - t*(168.0-115.0)
				}
				wx, wy := pointFromAngle(sx, sy, ex, ey, 0.14, elbowAngle, 1.0)
				setBilateral(pts, 9, 10, wx, wy, 0.0)
				if faultMode == "bad_form" {
					// Uneven bar press: left wrist much higher than right wrist
					pts[10].Y = float32(wy - 0.09) // Perturb right wrist Y so left-side elbow angle (P5-P7-P9) remains intact
				}
				return pts
			},
		},
		{
			slug:              "shoulder_press",
			cameraAngle:       "FRONT",
			expectedErrorCode: "ERR_WRIST_SYMMETRY",
			poseGen: func(t float64, faultMode string, rightSideDominant bool) []vo.SkeletonPoint {
				pts := makeBaseSkeleton(rightSideDominant)
				pts[9].Visibility = 0.95
				pts[10].Visibility = 0.95
				sx, sy := 0.50, 0.35
				hx, hy := 0.50, 0.65 // torsoLen = 0.30, vertical spine (0 deg sway)
				setBilateral(pts, 5, 6, sx, sy, 0.0)
				setBilateral(pts, 11, 12, hx, hy, 0.0)
				setBilateral(pts, 13, 14, 0.50, 0.82, 0.0)
				setBilateral(pts, 15, 16, 0.50, 0.96, 0.0)

				ex, ey := sx+0.10, sy+0.04
				setBilateral(pts, 7, 8, ex, ey, 0.0)

				// Ascending angle: t=0 -> 80 deg (rest <= 98), t=1 -> 168 deg (active >= 148)
				elbowAngle := 80.0 + t*(168.0-80.0)
				if faultMode == "half_rep" {
					elbowAngle = 80.0 + t*(130.0-80.0) // stops at 130 deg (< 148)
				}
				wx, wy := pointFromAngle(sx, sy, ex, ey, 0.14, elbowAngle, -1.0)
				setBilateral(pts, 9, 10, wx, wy, 0.0)
				if faultMode == "bad_form" {
					pts[10].Y = float32(wy - 0.09) // Perturb right wrist Y so left-side elbow angle (P5-P7-P9) remains intact
				}
				return pts
			},
		},
		{
			slug:              "pull_up",
			cameraAngle:       "FRONT",
			expectedErrorCode: "ERR_KIPPING_SWING",
			poseGen: func(t float64, faultMode string, rightSideDominant bool) []vo.SkeletonPoint {
				pts := makeBaseSkeleton(rightSideDominant)
				sx, sy := 0.50, 0.35
				hx, hy := 0.50, 0.65 // vertical torso
				setBilateral(pts, 5, 6, sx, sy, 0.0)
				setBilateral(pts, 11, 12, hx, hy, 0.0)

				// Hip angle (Shoulder-Hip-Knee): straight legs = 175 deg (>=150), kipping = 110 deg (<150)
				hipAngle := 175.0
				if faultMode == "bad_form" {
					hipAngle = 110.0
				}
				kx, ky := pointFromAngle(sx, sy, hx, hy, 0.20, hipAngle, 1.0)
				setBilateral(pts, 13, 14, kx, ky, 0.0)
				setBilateral(pts, 15, 16, kx, ky+0.15, 0.0)

				ex, ey := sx+0.10, sy-0.05
				setBilateral(pts, 7, 8, ex, ey, 0.0)

				// Elbow angle: t=0 -> 168 deg (rest > 155), t=1 -> 72 deg (active <= 85)
				elbowAngle := 168.0 - t*(168.0-72.0)
				if faultMode == "half_rep" {
					elbowAngle = 168.0 - t*(168.0-110.0)
				}
				wx, wy := pointFromAngle(sx, sy, ex, ey, 0.14, elbowAngle, -1.0)
				setBilateral(pts, 9, 10, wx, wy, 0.0)
				return pts
			},
		},
		{
			slug:              "lateral_raise",
			cameraAngle:       "FRONT",
			expectedErrorCode: "ERR_BENT_ELBOW_CHEAT",
			poseGen: func(t float64, faultMode string, rightSideDominant bool) []vo.SkeletonPoint {
				pts := makeBaseSkeleton(rightSideDominant)
				pts[9].Visibility = 0.95
				pts[10].Visibility = 0.95
				sx, sy := 0.50, 0.35
				hx, hy := 0.50, 0.65
				setBilateral(pts, 5, 6, sx, sy, 0.0)
				setBilateral(pts, 11, 12, hx, hy, 0.0)
				setBilateral(pts, 13, 14, 0.50, 0.82, 0.0)
				setBilateral(pts, 15, 16, 0.50, 0.96, 0.0)

				// Shoulder abduction angle (Hip-Shoulder-Elbow): t=0 -> 15 deg (<=28), t=1 -> 85 deg (>=75)
				abdAngle := 15.0 + t*(85.0-15.0)
				if faultMode == "half_rep" {
					abdAngle = 15.0 + t*(55.0-15.0) // stops at 55 deg (< 75)
				}
				ex, ey := pointFromAngle(hx, hy, sx, sy, 0.15, abdAngle, -1.0)
				setBilateral(pts, 7, 8, ex, ey, 0.0)

				// Elbow angle (Shoulder-Elbow-Wrist): good = 165 deg (>=135), bad_form = 95 deg (<135)
				elbowAngle := 165.0
				if faultMode == "bad_form" {
					elbowAngle = 95.0
				}
				wx, wy := pointFromAngle(sx, sy, ex, ey, 0.14, elbowAngle, -1.0)
				setBilateral(pts, 9, 10, wx, wy, 0.0)
				return pts
			},
		},
		{
			slug:              "triceps_dip",
			cameraAngle:       "SIDE",
			expectedErrorCode: "ERR_HIPS_TOO_FAR",
			poseGen: func(t float64, faultMode string, rightSideDominant bool) []vo.SkeletonPoint {
				pts := makeBaseSkeleton(rightSideDominant)
				sx, sy := 0.50, 0.38
				hx, hy := 0.50, 0.66 // torsoLen = 0.28, vertical torso (0 deg lean)
				setBilateral(pts, 5, 6, sx, sy, 0.0)
				setBilateral(pts, 11, 12, hx, hy, 0.0)
				setBilateral(pts, 13, 14, 0.62, 0.72, 0.0)
				setBilateral(pts, 15, 16, 0.68, 0.90, 0.0)

				// Elbow angle: t=0 -> 168 deg (>156), t=1 -> 86 deg (<=142)
				elbowAngle := 168.0 - t*(168.0-86.0)
				if faultMode == "half_rep" {
					elbowAngle = 168.0 - t*(168.0-148.0)
				}
				// Place wrist close to hip X (0.50). If bad_form -> shift hip far from wrist X
				if faultMode == "bad_form" {
					// Move hips far forward so |X_hip - X_wrist| / torsoLen > 0.42
					pts[11].X = 0.68
					pts[12].X = 0.68
					pts[5].X = 0.68
					pts[6].X = 0.68
					sx = 0.68
				}
				ex, ey := sx-0.06, sy+0.12
				wxCalc, wyCalc := pointFromAngle(sx, sy, ex, ey, 0.12, elbowAngle, 1.0)
				if faultMode != "bad_form" {
					// shift elbow/wrist X together so wrist X is close to hip X (0.50)
					dx := 0.48 - wxCalc
					ex += dx
					wxCalc += dx
				} else {
					// keep wrist far behind at X=0.45 while hip is at X=0.68
					dx := 0.45 - wxCalc
					ex += dx
					wxCalc += dx
					pts[5].X = float32(sx + dx)
					pts[6].X = float32(sx + dx)
				}
				setBilateral(pts, 7, 8, ex, ey, 0.0)
				setBilateral(pts, 9, 10, wxCalc, wyCalc, 0.0)
				return pts
			},
		},
		{
			slug:              "sit_up",
			cameraAngle:       "SIDE",
			expectedErrorCode: "ERR_ANKLE_LIFT",
			poseGen: func(t float64, faultMode string, rightSideDominant bool) []vo.SkeletonPoint {
				pts := makeBaseSkeleton(rightSideDominant)
				// Supine on floor: Hip at (0.50, 0.75), Knee bent at (0.65, 0.62), Ankle on floor at (0.78, 0.75)
				hx, hy := 0.50, 0.75
				kx, ky := 0.65, 0.62
				ax, ay := 0.78, 0.75
				if faultMode == "bad_form" {
					// Lift feet high off the floor: |Y_hip - Y_ankle| / torsoLen > 0.16
					ay = 0.65 // |0.75 - 0.65| / 0.28 = 0.357 > 0.16
				}
				setBilateral(pts, 11, 12, hx, hy, 0.0)
				setBilateral(pts, 13, 14, kx, ky, 0.0)
				setBilateral(pts, 15, 16, ax, ay, 0.0)

				// Hip flexion angle (Shoulder-Hip-Knee): t=0 -> 135 deg (>125), t=1 -> 65 deg (<=102)
				hipAngle := 135.0 - t*(135.0-65.0)
				if faultMode == "half_rep" {
					hipAngle = 135.0 - t*(135.0-112.0)
				}
				sx, sy := pointFromAngle(kx, ky, hx, hy, 0.28, hipAngle, -1.0)
				setBilateral(pts, 5, 6, sx, sy, 0.0)
				setBilateral(pts, 7, 8, sx+0.05, sy+0.05, 0.0)
				setBilateral(pts, 9, 10, sx+0.08, sy, 0.0)
				return pts
			},
		},
		{
			slug:              "plank",
			cameraAngle:       "SIDE",
			expectedErrorCode: "ERR_SIGNED_HIP_Y_DIFF",
			poseGen: func(t float64, faultMode string, rightSideDominant bool) []vo.SkeletonPoint {
				pts := makeBaseSkeleton(rightSideDominant)
				if faultMode == "half_rep" {
					// For timed Plank, "half_rep" / idle mode simulates standing/kneeling upright (incline > 35 deg)
					// so hold timer must pause!
					setBilateral(pts, 5, 6, 0.50, 0.30, 0.0)
					setBilateral(pts, 11, 12, 0.50, 0.60, 0.0)
					setBilateral(pts, 15, 16, 0.50, 0.90, 0.0)
					setBilateral(pts, 7, 8, 0.50, 0.45, 0.0)
					setBilateral(pts, 9, 10, 0.50, 0.55, 0.0)
					return pts
				}
				// Horizontal plank: Shoulder(0.75, 0.55), Ankle(0.20, 0.65) -> incline ~ 10.3 deg (<= 25 deg)
				sx, sy := 0.75, 0.55
				ax, ay := 0.20, 0.65
				// Collinear midpoint for Hip: (0.475, 0.60)
				hx, hy := 0.475, 0.60
				if faultMode == "bad_form" {
					// Hip sagging strongly below the Shoulder-Ankle line
					hy = 0.72
				}
				setBilateral(pts, 5, 6, sx, sy, 0.0)
				setBilateral(pts, 11, 12, hx, hy, 0.0)
				setBilateral(pts, 13, 14, 0.34, 0.625, 0.0)
				setBilateral(pts, 15, 16, ax, ay, 0.0)

				// Arm to torso angle (Hip-Shoulder-Elbow) ~ 90 deg ([70, 110])
				ex, ey := pointFromAngle(hx, hy, sx, sy, 0.14, 90.0, -1.0)
				// Forearm on floor: Elbow angle (Shoulder-Elbow-Wrist) ~ 90 deg ([70, 110])
				wx, wy := pointFromAngle(sx, sy, ex, ey, 0.14, 90.0, -1.0)
				setBilateral(pts, 7, 8, ex, ey, 0.0)
				setBilateral(pts, 9, 10, wx, wy, 0.0)
				return pts
			},
		},
	}
}

func mustNewSession(t testing.TB, motionRule *vo.ExerciseMotionRule) *MotionRefereeSession {
	sess, err := NewMotionRefereeSession(motionRule.ExerciseID, motionRule.ChallengeType, motionRule.TimeCeiling, motionRule)
	if err != nil {
		t.Fatalf("NewMotionRefereeSession(%s) error: %v", motionRule.ExerciseID, err)
	}
	return sess
}

// runTrajectoryStream feeds a sequence of frames at 20 FPS (50ms delta) into MotionRefereeSession.
func runTrajectoryStream(
	ref *MotionRefereeSession,
	evaluator RuleEvaluator,
	motionRule *vo.ExerciseMotionRule,
	cameraAngle string,
	frames [][]vo.SkeletonPoint,
) (confirmedErrFrames int, maxErrors []string) {
	delta := 50 * time.Millisecond
	for _, pts := range frames {
		res := evaluator.Evaluate(pts, motionRule, cameraAngle)
		ref.ProcessFrame(res, delta)
		errs := ref.ConfirmedErrors()
		if len(errs) > 0 {
			confirmedErrFrames++
			maxErrors = errs
		}
	}
	return confirmedErrFrames, maxErrors
}

// generateRepTrajectory produces nReps full cycles at 20 FPS (30 frames per rep: 12 frames REST->ACTIVE, 6 frames hold ACTIVE, 12 frames ACTIVE->REST).
func generateRepTrajectory(spec exerciseBenchSpec, nReps int, faultMode string, rightSideDominant bool, injectGlitch bool) [][]vo.SkeletonPoint {
	var frames [][]vo.SkeletonPoint
	// Start with 5 frames in REST
	for i := 0; i < 5; i++ {
		frames = append(frames, spec.poseGen(0.0, faultMode, rightSideDominant))
	}
	for rep := 0; rep < nReps; rep++ {
		// Eccentric / Concentric phase to ACTIVE (12 frames)
		for i := 1; i <= 12; i++ {
			t := float64(i) / 12.0
			mode := faultMode
			// If injectGlitch is true, inject a 2-frame bad_form spike at frame 5 & 6 (100ms < 150ms persistence threshold)
			if injectGlitch && rep == 1 && (i == 5 || i == 6) {
				mode = "bad_form"
			}
			frames = append(frames, spec.poseGen(t, mode, rightSideDominant))
		}
		// Hold ACTIVE peak (6 frames)
		for i := 0; i < 6; i++ {
			frames = append(frames, spec.poseGen(1.0, faultMode, rightSideDominant))
		}
		// Return to REST (12 frames)
		for i := 11; i >= 0; i-- {
			t := float64(i) / 12.0
			frames = append(frames, spec.poseGen(t, faultMode, rightSideDominant))
		}
	}
	return frames
}

func TestTwelveExercises_60Scenarios_BenchmarkSuite(t *testing.T) {
	ctx := context.Background()
	provider := rule.NewCachedRuleProvider(nil, nil)
	evaluator := NewDefaultRuleEvaluator()
	specs := getTwelveExerciseSpecs()

	if len(specs) != 12 {
		t.Fatalf("expected 12 exercise specs, got %d", len(specs))
	}

	var totalRepExpected, totalRepPredicted int32
	var totalRepAbsError float64
	var countedScenarios int
	var truePositives, falsePositives, falseNegatives, trueNegatives int

	for _, spec := range specs {
		motionRule, err := provider.GetRule(ctx, spec.slug)
		if err != nil {
			t.Fatalf("[%s] failed to load embedded motion rule: %v", spec.slug, err)
		}
		if len(motionRule.Rules) == 0 {
			t.Fatalf("[%s] loaded 0 rules", spec.slug)
		}

		// Scenario 1: 3 Perfect Reps (or 4.75s Perfect Hold for Plank)
		t.Run(spec.slug+"/1_PerfectForm", func(t *testing.T) {
			ref := mustNewSession(t, motionRule)
			frames := generateRepTrajectory(spec, 3, "good", false, false)
			errFrames, errs := runTrajectoryStream(ref, evaluator, motionRule, spec.cameraAngle, frames)
			if errFrames > 0 {
				t.Errorf("[%s] expected 0 confirmed error frames on perfect form, got %d (%v)", spec.slug, errFrames, errs)
				falsePositives++
			} else {
				trueNegatives++
			}

			if motionRule.ChallengeType == vo.ChallengeTypeReps {
				got := ref.ConfirmedValue()
				totalRepExpected += 3
				totalRepPredicted += got
				totalRepAbsError += math.Abs(float64(got - 3))
				countedScenarios++
				if got != 3 {
					t.Errorf("[%s] expected 3 reps, got %d", spec.slug, got)
				}
			} else {
				if ref.ConfirmedValue() < 4 {
					t.Errorf("[%s] expected ConfirmedValue >= 4s, got %d", spec.slug, ref.ConfirmedValue())
				}
			}
		})

		// Scenario 2: 3 Half-Reps (Shallow ROM) or Idle Standing for Plank -> 0 Reps / 0 Hold Time
		t.Run(spec.slug+"/2_ShallowROM_Or_Idle", func(t *testing.T) {
			ref := mustNewSession(t, motionRule)
			frames := generateRepTrajectory(spec, 3, "half_rep", false, false)
			runTrajectoryStream(ref, evaluator, motionRule, spec.cameraAngle, frames)

			if motionRule.ChallengeType == vo.ChallengeTypeReps {
				got := ref.ConfirmedValue()
				totalRepExpected += 0
				totalRepPredicted += got
				totalRepAbsError += math.Abs(float64(got - 0))
				countedScenarios++
				if got != 0 {
					t.Errorf("[%s] shallow ROM half-reps should count 0 reps, got %d", spec.slug, got)
				}
			} else {
				if ref.ConfirmedValue() > 0 {
					t.Errorf("[%s] idle standing pose should accumulate 0s hold duration, got %ds", spec.slug, ref.ConfirmedValue())
				}
			}
		})

		// Scenario 3: Persistent Biomechanical Form Fault -> Rep still counts (decoupled!), ConfirmedError + Vietnamese AudioCue emitted
		t.Run(spec.slug+"/3_PersistentFormFault", func(t *testing.T) {
			ref := mustNewSession(t, motionRule)
			frames := generateRepTrajectory(spec, 2, "bad_form", false, false)
			errFrames, errs := runTrajectoryStream(ref, evaluator, motionRule, spec.cameraAngle, frames)

			if errFrames > 0 {
				truePositives++
			} else {
				falseNegatives++
				t.Errorf("[%s] expected confirmed form error (%s) on bad_form trajectory, got 0 error frames", spec.slug, spec.expectedErrorCode)
			}

			foundExpected := false
			for _, code := range errs {
				if strings.HasPrefix(code, spec.expectedErrorCode) {
					foundExpected = true
					break
				}
			}
			if !foundExpected {
				t.Errorf("[%s] expected error prefix %s in confirmed errors %v", spec.slug, spec.expectedErrorCode, errs)
			}

			if motionRule.ChallengeType == vo.ChallengeTypeReps {
				got := ref.ConfirmedValue()
				totalRepExpected += 2
				totalRepPredicted += got
				totalRepAbsError += math.Abs(float64(got - 2))
				countedScenarios++
				if got != 2 {
					t.Errorf("[%s] reps with form warnings must still be counted (no lost-rep bug): want 2, got %d", spec.slug, got)
				}
			}
		})

		// Scenario 4: Transient 2-Frame Glitch (100ms spike) -> Persistence Filter suppresses false alarm (0 ConfirmedErrors)
		t.Run(spec.slug+"/4_TransientNoiseGlitch_Filtered", func(t *testing.T) {
			ref := mustNewSession(t, motionRule)
			frames := generateRepTrajectory(spec, 2, "good", false, true)
			errFrames, errs := runTrajectoryStream(ref, evaluator, motionRule, spec.cameraAngle, frames)
			if errFrames > 0 {
				falsePositives++
				t.Errorf("[%s] 2-frame (100ms) glitch should be suppressed by Persistence Filter, but got %d error frames (%v)", spec.slug, errFrames, errs)
			} else {
				trueNegatives++
			}
		})

		// Scenario 5: Side-View Self-Occlusion (Left landmarks occluded vis=0.15, Right landmarks visible vis=0.98)
		t.Run(spec.slug+"/5_SideViewSelfOcclusion_DominantSide", func(t *testing.T) {
			ref := mustNewSession(t, motionRule)
			frames := generateRepTrajectory(spec, 3, "good", true, false)
			errFrames, errs := runTrajectoryStream(ref, evaluator, motionRule, spec.cameraAngle, frames)
			if errFrames > 0 {
				t.Errorf("[%s] expected 0 errors under right-side dominant view, got %v", spec.slug, errs)
			}
			if !ref.PersonDetected() {
				t.Errorf("[%s] expected PersonDetected() == true when right side is visible", spec.slug)
			}
			if motionRule.ChallengeType == vo.ChallengeTypeReps {
				got := ref.ConfirmedValue()
				totalRepExpected += 3
				totalRepPredicted += got
				totalRepAbsError += math.Abs(float64(got - 3))
				countedScenarios++
				if got != 3 {
					t.Errorf("[%s] expected 3 reps when left side is occluded and right side is visible, got %d", spec.slug, got)
				}
			}
		})
	}

	mae := totalRepAbsError / float64(countedScenarios)
	precision := float64(truePositives) / float64(truePositives+falsePositives)
	recall := float64(truePositives) / float64(truePositives+falseNegatives)
	f1 := 2 * precision * recall / (precision + recall)

	t.Logf("=== 12 EXERCISES x 5 SCENARIOS (60 TEST CASES) BENCHMARK SUMMARY ===")
	t.Logf("Rep Counting MAE: %.4f reps/set (Total Expected: %d, Total Predicted: %d across %d counted sets)",
		mae, totalRepExpected, totalRepPredicted, countedScenarios)
	t.Logf("Form Fault Classification: TP=%d, FP=%d, TN=%d, FN=%d | Precision=%.2f%%, Recall=%.2f%%, F1=%.2f%%",
		truePositives, falsePositives, trueNegatives, falseNegatives, precision*100, recall*100, f1*100)

	if mae > 0.0 {
		t.Errorf("Rep Counting MAE must be 0.0 on canonical benchmark trajectories, got %.4f", mae)
	}
	if f1 < 0.99 {
		t.Errorf("Form Fault F1 score must be >= 99%%, got %.2f%%", f1*100)
	}
}

func BenchmarkTwelveExercises_RefereeThroughput(b *testing.B) {
	ctx := context.Background()
	provider := rule.NewCachedRuleProvider(nil, nil)
	evaluator := NewDefaultRuleEvaluator()
	specs := getTwelveExerciseSpecs()

	type benchItem struct {
		rule   *vo.ExerciseMotionRule
		ref    *MotionRefereeSession
		frames [][]vo.SkeletonPoint
		angle  string
	}
	items := make([]benchItem, len(specs))
	for i, sp := range specs {
		r, err := provider.GetRule(ctx, sp.slug)
		if err != nil {
			b.Fatalf("load rule %s: %v", sp.slug, err)
		}
		items[i] = benchItem{
			rule:   r,
			ref:    mustNewSession(b, r),
			frames: generateRepTrajectory(sp, 1, "good", false, false),
			angle:  sp.cameraAngle,
		}
	}

	delta := 50 * time.Millisecond
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		item := &items[i%len(items)]
		pts := item.frames[i%len(item.frames)]
		res := evaluator.Evaluate(pts, item.rule, item.angle)
		item.ref.ProcessFrame(res, delta)
	}
}
