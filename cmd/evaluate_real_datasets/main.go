package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/viethung213/gym-companion/internal/workout_execution/domain/service"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/vo"
	"github.com/viethung213/gym-companion/internal/workout_execution/infrastructure/rule"
)

type PoseDoc struct {
	VideoID          string      `json:"video_id"`
	ExerciseType     string      `json:"exercise_type"`
	TotalFrames      int         `json:"total_frames"`
	FPS              float64     `json:"fps"`
	DurationSeconds  float64     `json:"duration_seconds"`
	DetectionRatePct float64     `json:"detection_rate_pct"`
	Frames           []PoseFrame `json:"frames"`
}

type PoseFrame struct {
	FrameIndex       int             `json:"frame_index"`
	TimestampMs      int64           `json:"timestamp_ms"`
	Detected         bool            `json:"detected"`
	PersonConfidence float64         `json:"person_confidence"`
	Keypoints        []PoseLandmark  `json:"keypoints"`
}

type PoseLandmark struct {
	Index      int     `json:"index"`
	Name       string  `json:"name"`
	X          float32 `json:"x"`
	Y          float32 `json:"y"`
	Confidence float32 `json:"confidence"`
	Visibility float32 `json:"visibility"`
}

type AnnotationsDoc struct {
	VideoID          string          `json:"video_id"`
	ExerciseType     string          `json:"exercise_type"`
	TotalRepetitions int32           `json:"total_repetitions"`
	IsTimed          bool            `json:"is_timed"`
	TargetDuration   float64         `json:"target_duration_seconds"`
	ValidDuration    float64         `json:"valid_duration_seconds"`
	Reps             []RepAnnotation `json:"reps"`
}

type RepAnnotation struct {
	RepID          int      `json:"rep_id"`
	IsCorrect      bool     `json:"is_correct"`
	StartFrame     int      `json:"start_frame"`
	PeakFrame      int      `json:"peak_frame"`
	EndFrame       int      `json:"end_frame"`
	DetectedErrors []string `json:"detected_errors"`
	Severity       string   `json:"severity"`
	AnnotatorNotes string   `json:"annotator_notes"`
}

type MetadataDoc struct {
	VideoID                 string                 `json:"video_id"`
	ExerciseType            string                 `json:"exercise_type"`
	EnvironmentalConditions EnvironmentalCondition `json:"environmental_conditions"`
}

type EnvironmentalCondition struct {
	CameraAngle string `json:"camera_angle"`
}

type FrameTimelineItem struct {
	FrameIndex      int      `json:"frame_index"`
	Phase           string   `json:"phase"`
	Angle           float32  `json:"angle"`
	ConfirmedValue  int32    `json:"confirmed_value"`
	DetectedErrors  []string `json:"detected_errors"`
	ConfirmedErrors []string `json:"confirmed_errors"`
}

type PredictionResult struct {
	VideoID           string              `json:"video_id"`
	ExerciseType      string              `json:"exercise_type"`
	ExpectedReps      int32               `json:"expected_reps"`
	PredictedReps     int32               `json:"predicted_reps"`
	RepAbsError       int32               `json:"rep_abs_error"`
	ExactMatch        bool                `json:"exact_match"`
	ExpectedErrors    []string            `json:"expected_errors"`
	PredictedErrors   []string            `json:"predicted_errors"`
	ConfirmedAudioCue string              `json:"confirmed_audio_cue"`
	FramesEvaluated   int                 `json:"frames_evaluated"`
	EvaluatedAt       string              `json:"evaluated_at"`
	Timeline          []FrameTimelineItem `json:"timeline,omitempty"`
}

type ErrorMetric struct {
	TP        int     `json:"true_positives"`
	FP        int     `json:"false_positives"`
	FN        int     `json:"false_negatives"`
	TN        int     `json:"true_negatives"`
	Precision float64 `json:"precision"`
	Recall    float64 `json:"recall"`
	F1Score   float64 `json:"f1_score"`
}

type EvaluationReport struct {
	Title           string                  `json:"title"`
	GeneratedAt     string                  `json:"generated_at"`
	TotalVideos     int                     `json:"total_videos"`
	TotalFrames     int                     `json:"total_frames"`
	RepMetrics      RepCountingMetrics      `json:"rep_counting_metrics"`
	FormMetrics     map[string]ErrorMetric  `json:"form_error_metrics"`
	DetailedResults []PredictionResult      `json:"detailed_results"`
}

type RepCountingMetrics struct {
	TotalExpectedReps  int32   `json:"total_expected_reps"`
	TotalPredictedReps int32   `json:"total_predicted_reps"`
	MeanAbsoluteError  float64 `json:"mean_absolute_error"`
	ExactMatchAccuracy float64 `json:"exact_match_accuracy_pct"`
	OvercountCount     int     `json:"overcount_count"`
	UndercountCount    int     `json:"undercount_count"`
}

func main() {
	isOptimized := false
	if len(os.Args) > 1 && os.Args[1] == "--optimized" {
		isOptimized = true
	}

	reportName := "baseline"
	if isOptimized {
		reportName = "post_optimization"
	}

	datasetsDir := filepath.Clean("../dataset-pipeline/datasets")
	evalOutputDir := filepath.Clean("../dataset-pipeline/evaluation")
	_ = os.MkdirAll(evalOutputDir, 0755)

	ctx := context.Background()
	provider := rule.NewCachedRuleProvider(nil, nil)
	evaluator := service.NewDefaultRuleEvaluator()

	exercises := []string{
		"push_up", "squat", "plank", "lunge",
		"bicep_curl", "shoulder_press", "bench_press", "deadlift",
		"pull_up", "sit_up", "triceps_dip", "lateral_raise",
	}

	var detailedResults []PredictionResult
	var totalExpectedReps, totalPredictedReps int32
	var totalAbsError float64
	var exactMatches int
	var overcounts, undercounts int
	var countedVideos int
	totalFrames := 0

	// Biomechanical error codes tracked across all 12 exercises
	targetErrorCodes := []string{
		"ERR_SIGNED_HIP_Y_DIFF",
		"ERR_HAND_WIDTH_RATIO_TOO_HIGH",
		"ERR_TORSO_LEAN",
		"ERR_KNEE_OVER_TOE",
		"ERR_BODY_INCLINE_ANGLE",
		"ERR_ELBOW_SWAY",
		"ERR_WRIST_SYMMETRY",
		"ERR_ELBOW_FLARE",
		"ERR_EXCESSIVE_KNEE_FLEXION",
		"ERR_BAR_DRIFT",
		"ERR_KIPPING_SWING",
		"ERR_ANKLE_LIFT",
		"ERR_HIPS_TOO_FAR",
		"ERR_BENT_ELBOW_CHEAT",
	}

	errorMetrics := make(map[string]*ErrorMetric)
	for _, code := range targetErrorCodes {
		errorMetrics[code] = &ErrorMetric{}
	}

	for _, ex := range exercises {
		motionRule, err := provider.GetRule(ctx, ex)
		if err != nil {
			log.Fatalf("Failed to load rule for %s: %v", ex, err)
		}

		exDir := filepath.Join(datasetsDir, ex)
		entries, err := os.ReadDir(exDir)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			vidDir := filepath.Join(exDir, entry.Name())
			metaPath := filepath.Join(vidDir, "metadata.json")
			annPath := filepath.Join(vidDir, "annotations.json")
			posePath := filepath.Join(vidDir, "pose.json")

			if _, err := os.Stat(posePath); err != nil {
				continue
			}

			// 1. Read files
			metaBytes, _ := os.ReadFile(metaPath)
			var meta MetadataDoc
			_ = json.Unmarshal(metaBytes, &meta)

			annBytes, _ := os.ReadFile(annPath)
			var ann AnnotationsDoc
			_ = json.Unmarshal(annBytes, &ann)

			poseBytes, _ := os.ReadFile(posePath)
			var pose PoseDoc
			_ = json.Unmarshal(poseBytes, &pose)

			// 2. Initialize Referee Session
			ref, err := service.NewSoloRefereeSession(
				motionRule.ExerciseID,
				motionRule.ChallengeType,
				motionRule.TimeCeiling,
				motionRule,
			)
			if err != nil {
				log.Fatalf("Failed to initialize session for %s: %v", entry.Name(), err)
			}

			cameraAngle := "SIDE"
			switch strings.ToLower(meta.EnvironmentalConditions.CameraAngle) {
			case "front":
				cameraAngle = "FRONT"
			case "front_45":
				cameraAngle = "DIAGONAL"
			case "side":
				cameraAngle = "SIDE"
			case "rear":
				cameraAngle = "REAR"
			}

			fps := pose.FPS
			if fps <= 0 {
				fps = 30.0
			}
			frameDelta := time.Duration(1000.0/fps) * time.Millisecond

			var allConfirmedErrors []string
			var lastAudioCue string
			var timeline []FrameTimelineItem

			// 3. Stream frames
			for _, f := range pose.Frames {
				pts := make([]vo.SkeletonPoint, len(f.Keypoints))
				for i, kpt := range f.Keypoints {
					pts[i] = vo.SkeletonPoint{
						Index:      kpt.Index,
						X:          kpt.X,
						Y:          kpt.Y,
						Z:          0.0,
						Visibility: kpt.Visibility,
					}
				}

				res := evaluator.Evaluate(pts, motionRule, cameraAngle)
				ref.ProcessFrame(res, frameDelta)

				errs := ref.ConfirmedErrors()
				for _, e := range errs {
					exists := false
					for _, existing := range allConfirmedErrors {
						if existing == e {
							exists = true
							break
						}
					}
					if !exists {
						allConfirmedErrors = append(allConfirmedErrors, e)
					}
				}
				if ref.ConfirmedAudioCue() != "" {
					lastAudioCue = ref.ConfirmedAudioCue()
				}

				timeline = append(timeline, FrameTimelineItem{
					FrameIndex:      f.FrameIndex,
					Phase:           string(ref.RepPhase()),
					Angle:           res.CalculatedAngle,
					ConfirmedValue:  ref.ConfirmedValue(),
					DetectedErrors:  res.DetectedErrors,
					ConfirmedErrors: append([]string(nil), errs...),
				})
			}

			totalFrames += len(pose.Frames)

			// 4. Extract expected errors from annotations
			var expectedErrors []string
			for _, r := range ann.Reps {
				for _, de := range r.DetectedErrors {
					exists := false
					for _, existing := range expectedErrors {
						if existing == de {
							exists = true
							break
						}
					}
					if !exists {
						expectedErrors = append(expectedErrors, de)
					}
				}
			}

			predictedReps := ref.ConfirmedValue()
			expectedReps := ann.TotalRepetitions
			repAbsErr := int32(math.Abs(float64(predictedReps - expectedReps)))
			isExact := (predictedReps == expectedReps)

			if !ann.IsTimed {
				countedVideos++
				totalExpectedReps += expectedReps
				totalPredictedReps += predictedReps
				totalAbsError += float64(repAbsErr)
				if isExact {
					exactMatches++
				}
				if predictedReps > expectedReps {
					overcounts++
				} else if predictedReps < expectedReps {
					undercounts++
				}
			}

			// Update error metrics
			for _, code := range targetErrorCodes {
				exp := hasError(expectedErrors, code)
				pred := hasError(allConfirmedErrors, code)
				m := errorMetrics[code]
				if exp && pred {
					m.TP++
				} else if !exp && pred {
					m.FP++
				} else if exp && !pred {
					m.FN++
				} else {
					m.TN++
				}
			}

			predRes := PredictionResult{
				VideoID:           entry.Name(),
				ExerciseType:      ex,
				ExpectedReps:      expectedReps,
				PredictedReps:     predictedReps,
				RepAbsError:       repAbsErr,
				ExactMatch:        isExact,
				ExpectedErrors:    expectedErrors,
				PredictedErrors:   allConfirmedErrors,
				ConfirmedAudioCue: lastAudioCue,
				FramesEvaluated:   len(pose.Frames),
				EvaluatedAt:       time.Now().UTC().Format(time.RFC3339),
				Timeline:          timeline,
			}
			detailedResults = append(detailedResults, predRes)

			// Save predictions.json in dataset directory
			predBytes, _ := json.MarshalIndent(predRes, "", "  ")
			_ = os.WriteFile(filepath.Join(vidDir, "predictions.json"), predBytes, 0644)
		}
	}

	// Compute Precision, Recall, F1 for each error
	finalFormMetrics := make(map[string]ErrorMetric)
	for code, m := range errorMetrics {
		prec := 0.0
		if (m.TP + m.FP) > 0 {
			prec = float64(m.TP) / float64(m.TP+m.FP)
		}
		rec := 0.0
		if (m.TP + m.FN) > 0 {
			rec = float64(m.TP) / float64(m.TP+m.FN)
		}
		f1 := 0.0
		if (prec + rec) > 0 {
			f1 = 2 * (prec * rec) / (prec + rec)
		}
		finalFormMetrics[code] = ErrorMetric{
			TP:        m.TP,
			FP:        m.FP,
			FN:        m.FN,
			TN:        m.TN,
			Precision: math.Round(prec*1000) / 1000,
			Recall:    math.Round(rec*1000) / 1000,
			F1Score:   math.Round(f1*1000) / 1000,
		}
	}

	mae := 0.0
	exactAcc := 0.0
	if countedVideos > 0 {
		mae = totalAbsError / float64(countedVideos)
		exactAcc = (float64(exactMatches) / float64(countedVideos)) * 100.0
	}

	report := EvaluationReport{
		Title:       fmt.Sprintf("FitAI Real-World Video Evaluation Report (%s)", strings.ToUpper(reportName)),
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		TotalVideos: len(detailedResults),
		TotalFrames: totalFrames,
		RepMetrics: RepCountingMetrics{
			TotalExpectedReps:  totalExpectedReps,
			TotalPredictedReps: totalPredictedReps,
			MeanAbsoluteError:  math.Round(mae*1000) / 1000,
			ExactMatchAccuracy: math.Round(exactAcc*10) / 10,
			OvercountCount:     overcounts,
			UndercountCount:    undercounts,
		},
		FormMetrics:     finalFormMetrics,
		DetailedResults: detailedResults,
	}

	// Save JSON report
	jsonReportPath := filepath.Join(evalOutputDir, fmt.Sprintf("%s_evaluation_report.json", reportName))
	repJsonBytes, _ := json.MarshalIndent(report, "", "  ")
	_ = os.WriteFile(jsonReportPath, repJsonBytes, 0644)

	// Save Markdown report
	mdReportPath := filepath.Join(evalOutputDir, fmt.Sprintf("%s_evaluation_report.md", reportName))
	mdContent := generateMarkdownReport(report, reportName)
	_ = os.WriteFile(mdReportPath, []byte(mdContent), 0644)

	fmt.Printf("\n================================================================================\n")
	fmt.Printf("FITAI REAL VIDEO RULE EVALUATION REPORT: %s\n", strings.ToUpper(reportName))
	fmt.Printf("================================================================================\n")
	fmt.Printf("Total Videos Evaluated: %d (%d counted rep exercises)\n", len(detailedResults), countedVideos)
	fmt.Printf("Total Frames Processed: %d\n", totalFrames)
	fmt.Printf("Rep Counting MAE:       %.3f reps\n", report.RepMetrics.MeanAbsoluteError)
	fmt.Printf("Exact Match Accuracy:   %.1f%%\n", report.RepMetrics.ExactMatchAccuracy)
	fmt.Printf("Overcounts:             %d | Undercounts: %d\n", overcounts, undercounts)
	fmt.Printf("--------------------------------------------------------------------------------\n")
	fmt.Printf("%-32s | Exp | Pred | Diff | Confirmed Errors\n", "Video ID")
	fmt.Printf("--------------------------------------------------------------------------------\n")
	for _, res := range detailedResults {
		errStr := strings.Join(res.PredictedErrors, ", ")
		if errStr == "" {
			errStr = "-"
		}
		fmt.Printf("%-32s | %3d | %4d | %+4d | %s\n", res.VideoID, res.ExpectedReps, res.PredictedReps, res.PredictedReps-res.ExpectedReps, errStr)
	}
	fmt.Printf("--------------------------------------------------------------------------------\n")
	fmt.Printf("Form Error Metrics:\n")
	for code, m := range finalFormMetrics {
		fmt.Printf("  %-30s: P=%.2f, R=%.2f, F1=%.2f (TP=%d, FP=%d, FN=%d, TN=%d)\n", code, m.Precision, m.Recall, m.F1Score, m.TP, m.FP, m.FN, m.TN)
	}
	fmt.Printf("================================================================================\n")
	fmt.Printf("[+] Saved reports to:\n    %s\n    %s\n", jsonReportPath, mdReportPath)
}

func hasError(errs []string, code string) bool {
	for _, e := range errs {
		if strings.HasPrefix(e, code) {
			return true
		}
	}
	return false
}

func generateMarkdownReport(rep EvaluationReport, phase string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# FitAI Exercise Analysis Real-World Validation Report (%s)\n\n", strings.ToUpper(phase)))
	sb.WriteString(fmt.Sprintf("> **Generated**: %s  \n", rep.GeneratedAt))
	sb.WriteString("> **Dataset**: 38 Verified Real Human Workout Videos Across 12 Canonical Exercises  \n")
	sb.WriteString("> **Inference Engine**: Standalone ONNX Pose Estimation (yolov8n-pose.onnx, COCO-17)  \n")
	sb.WriteString("> **Authoritative Referee Engine**: gym-companion Go Service  \n\n")

	sb.WriteString("## 1. Executive Summary & Rep Counting Metrics\n\n")
	sb.WriteString("| Metric | Value |\n|---|:---:|\n")
	sb.WriteString(fmt.Sprintf("| Total Real Videos Evaluated | **%d** |\n", rep.TotalVideos))
	sb.WriteString(fmt.Sprintf("| Total Video Frames Processed | **%d** |\n", rep.TotalFrames))
	sb.WriteString(fmt.Sprintf("| Total Expected Reps (Ground Truth) | **%d** |\n", rep.RepMetrics.TotalExpectedReps))
	sb.WriteString(fmt.Sprintf("| Total Predicted Reps (Go Referee) | **%d** |\n", rep.RepMetrics.TotalPredictedReps))
	sb.WriteString(fmt.Sprintf("| **Mean Absolute Error (MAE)** | **%.3f reps** |\n", rep.RepMetrics.MeanAbsoluteError))
	sb.WriteString(fmt.Sprintf("| **Exact Match Accuracy** | **%.1f%%** |\n", rep.RepMetrics.ExactMatchAccuracy))
	sb.WriteString(fmt.Sprintf("| Overcount Rate | %d videos |\n", rep.RepMetrics.OvercountCount))
	sb.WriteString(fmt.Sprintf("| Undercount Rate | %d videos |\n\n", rep.RepMetrics.UndercountCount))

	sb.WriteString("## 2. Form Error Detection Metrics (Precision / Recall / F1)\n\n")
	sb.WriteString("| Biomechanical Error Code | TP | FP | FN | TN | Precision | Recall | F1-Score |\n|---|:---:|:---:|:---:|:---:|:---:|:---:|:---:|\n")
	for code, m := range rep.FormMetrics {
		sb.WriteString(fmt.Sprintf("| `%s` | %d | %d | %d | %d | %.2f | %.2f | **%.2f** |\n", code, m.TP, m.FP, m.FN, m.TN, m.Precision, m.Recall, m.F1Score))
	}
	sb.WriteString("\n## 3. Detailed Per-Video Empirical Results\n\n")
	sb.WriteString("| Video ID | Exercise | Expected | Predicted | Error | Match | Confirmed Biomechanical Errors |\n|---|:---:|:---:|:---:|:---:|:---:|---|\n")
	for _, res := range rep.DetailedResults {
		matchStr := "PASS"
		if !res.ExactMatch {
			matchStr = "FAIL"
		}
		errs := strings.Join(res.PredictedErrors, ", ")
		if errs == "" {
			errs = "None"
		}
		sb.WriteString(fmt.Sprintf("| `%s` | %s | %d | %d | %+d | **%s** | %s |\n", res.VideoID, res.ExerciseType, res.ExpectedReps, res.PredictedReps, res.PredictedReps-res.ExpectedReps, matchStr, errs))
	}
	return sb.String()
}
