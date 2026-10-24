package rule

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/viethung213/gym-companion/internal/workout_execution/application/port"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/derror"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/repository"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/vo"
)

// CachedRuleProvider implements repository.ExerciseRuleProvider by downloading real rule and dialogue
// asset files from object storage/HTTP based on MotionSpecification and caching them in-memory.
type CachedRuleProvider struct {
	specRepo        repository.MotionSpecificationRepository
	storageProvider port.ObjectStorageProvider
	httpClient      *http.Client
	cache           sync.Map
}

var _ repository.ExerciseRuleProvider = (*CachedRuleProvider)(nil)

// NewCachedRuleProvider creates a new CachedRuleProvider.
func NewCachedRuleProvider(
	specRepo repository.MotionSpecificationRepository,
	storageProvider port.ObjectStorageProvider,
) *CachedRuleProvider {
	return &CachedRuleProvider{
		specRepo:        specRepo,
		storageProvider: storageProvider,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// SetRule allows explicitly populating or overriding an exercise rule in the in-memory cache (useful for testing or cache warming).
// testing
func (p *CachedRuleProvider) SetRule(exerciseID string, rule *vo.ExerciseMotionRule) {
	if exerciseID != "" && rule != nil {
		p.cache.Store(exerciseID, rule)
	}
}

// GetRule returns the ExerciseMotionRule for the given exerciseID.
// It checks in-memory cache first, then fetches MotionSpecification from DB, downloads rule and dialogue assets,
// parses and merges them, and saves the result in cache.
func (p *CachedRuleProvider) GetRule(ctx context.Context, exerciseID string) (*vo.ExerciseMotionRule, error) {
	exerciseID = strings.TrimSpace(exerciseID)
	if exerciseID == "" {
		return nil, fmt.Errorf("exercise_id cannot be empty")
	}

	// 1. Check in-memory cache
	if val, ok := p.cache.Load(exerciseID); ok {
		if rule, okRule := val.(*vo.ExerciseMotionRule); okRule {
			return rule, nil
		}
	}

	// 2. Fetch MotionSpecification from repository
	if p.specRepo == nil {
		return nil, fmt.Errorf("motion specification repository is not configured")
	}

	spec, err := p.specRepo.FindByExerciseID(ctx, exerciseID)
	if err != nil {
		if errors.Is(err, derror.ErrMotionSpecNotFound) {
			return nil, fmt.Errorf("motion specification not found for exercise_id: %s: %w", exerciseID, err)
		}
		return nil, fmt.Errorf("find motion specification for %s: %w", exerciseID, err)
	}
	if spec == nil {
		return nil, fmt.Errorf("motion specification not found for exercise_id: %s", exerciseID)
	}

	rulesURL := spec.LocalRulesURL()
	if strings.TrimSpace(rulesURL) == "" {
		return nil, fmt.Errorf("local_rules_url is not configured for exercise_id: %s", exerciseID)
	}

	// 3. Download rules JSON asset
	ruleBytes, err := p.fetchAsset(ctx, rulesURL, "rules", exerciseID)
	if err != nil {
		return nil, fmt.Errorf("fetch rules asset for exercise %s (%s): %w", exerciseID, rulesURL, err)
	}

	// 4. Download dialogues JSON asset (optional: if present, fetch and merge)
	var dialogueBytes []byte
	dialogueURL := spec.DialogueEngineURL()
	if strings.TrimSpace(dialogueURL) != "" {
		diaData, diaErr := p.fetchAsset(ctx, dialogueURL, "dialogues", exerciseID)
		if diaErr == nil && len(diaData) > 0 {
			dialogueBytes = diaData
		}
	}

	// 5. Parse and merge both JSON files into vo.ExerciseMotionRule
	motionRule, err := parseMotionRule(exerciseID, spec.ExerciseName(), spec.RecommendedCameraAngle(), ruleBytes, dialogueBytes)
	if err != nil {
		return nil, fmt.Errorf("parse motion rule asset for exercise %s: %w", exerciseID, err)
	}

	// 6. Cache and return
	p.cache.Store(exerciseID, motionRule)
	return motionRule, nil
}

func (p *CachedRuleProvider) fetchAsset(ctx context.Context, fileURL, assetType, exerciseID string) ([]byte, error) {
	fileURL = strings.TrimSpace(fileURL)
	if fileURL == "" {
		return nil, fmt.Errorf("empty file URL")
	}

	// 1. Nếu link đã có sẵn thành phần http/https (như link Supabase Storage public trong DB), tải trực tiếp qua HTTP GET
	if strings.HasPrefix(fileURL, "http://") || strings.HasPrefix(fileURL, "https://") {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, fileURL, nil)
		if err == nil {
			client := p.httpClient
			if client == nil {
				client = &http.Client{Timeout: 10 * time.Second}
			}
			resp, doErr := client.Do(req)
			if doErr == nil {
				defer resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					data, readErr := io.ReadAll(resp.Body)
					if readErr == nil && len(data) > 0 {
						return data, nil
					}
				}
			}
		}
	}

	// 2. Thử ObjectStorageProvider (cho internal S3 object key hoặc mock storage trong test)
	if p.storageProvider != nil {
		objectKey := deriveObjectKey(fileURL, assetType, exerciseID)
		if objectKey != "" {
			data, err := p.storageProvider.GetObject(ctx, objectKey)
			if err == nil && len(data) > 0 {
				return data, nil
			}
		}
	}

	return nil, fmt.Errorf("could not fetch asset from %s", fileURL)
}

func deriveObjectKey(fileURL, assetType, exerciseID string) string {
	if fileURL != "" {
		clean := strings.TrimSpace(fileURL)
		if idx := strings.Index(clean, "/gym-companion-assets/"); idx != -1 {
			return strings.TrimPrefix(clean[idx+len("/gym-companion-assets/"):], "/")
		}
		if idx := strings.Index(clean, "/public/"); idx != -1 {
			sub := clean[idx+len("/public/"):]
			parts := strings.SplitN(sub, "/", 2)
			if len(parts) == 2 {
				return strings.TrimPrefix(parts[1], "/")
			}
		}
		if !strings.HasPrefix(clean, "http://") && !strings.HasPrefix(clean, "https://") {
			return strings.TrimPrefix(clean, "/")
		}
	}

	if assetType == "rules" {
		return fmt.Sprintf("rules/%s_rule.json", exerciseID)
	}
	return fmt.Sprintf("dialogues/%s_dialogue.json", exerciseID)
}

// Data structures for JSON unmarshaling
type rawRuleFile struct {
	DisplayName          string                   `json:"display_name"`
	RepType              string                   `json:"rep_type"`
	Notes                string                   `json:"notes"`
	PhaseDetection       *vo.PhaseDetectionConfig `json:"phase_detection"`
	Rules                []rawRuleCriterion       `json:"rules"`
	AudioCooldownSeconds map[string]float32       `json:"audio_cooldown_seconds"`
}

type rawRuleCriterion struct {
	Metric         string         `json:"metric"`
	Description    string         `json:"description"`
	EvaluationType string         `json:"evaluation_type"`
	ApplyInPhases  []string       `json:"apply_in_phases"`
	ApplyForView   string         `json:"apply_for_view"`
	Ranges         []rawRange     `json:"ranges"`
	Calculation    rawCalculation `json:"calculation"`
}

type rawRange struct {
	Severity int      `json:"severity"`
	Status   string   `json:"status"`
	Min      *float32 `json:"min"`
	Max      *float32 `json:"max"`
	Comment  string   `json:"comment"`
}

type rawCalculation struct {
	Landmarks   []string `json:"landmarks"`
	Joints      []int    `json:"joints"`
	Vectors     []string `json:"vectors"`
	Formula     string   `json:"formula"`
	Description string   `json:"description"`
}

type rawDialogueFile struct {
	Exercise       string                      `json:"exercise"`
	DisplayName    string                      `json:"display_name"`
	Description    string                      `json:"description"`
	VoiceFeedbacks map[string]rawVoiceFeedback `json:"voice_feedbacks"`
}

type rawVoiceFeedback struct {
	MetricName  string                      `json:"metric_name"`
	Description string                      `json:"description"`
	Severities  map[string]rawSeverityLevel `json:"severities"`
}

type rawSeverityLevel struct {
	SeverityLevel int                      `json:"severity_level"`
	Status        string                   `json:"status"`
	Styles        map[string]rawAudioStyle `json:"styles"`
}

type rawAudioStyle struct {
	Text string `json:"text"`
	URL  string `json:"url"`
}

var landmarkNumberRegex = regexp.MustCompile(`[pP]?(\d+)`)

func parseLandmarkNumbers(landmarks []string) []int {
	var joints []int
	for _, lm := range landmarks {
		matches := landmarkNumberRegex.FindAllStringSubmatch(lm, -1)
		if len(matches) > 0 {
			if val, err := strconv.Atoi(matches[0][1]); err == nil {
				joints = append(joints, val)
			}
		}
	}
	return joints
}

func extractDialogueText(fb rawVoiceFeedback) string {
	// Priority 1: Warning normal style
	if warn, ok := fb.Severities["warning"]; ok {
		if style, okStyle := warn.Styles["normal"]; okStyle && style.Text != "" {
			return style.Text
		}
		for _, s := range warn.Styles {
			if s.Text != "" {
				return s.Text
			}
		}
	}
	// Priority 2: Danger normal style
	if dang, ok := fb.Severities["danger"]; ok {
		if style, okStyle := dang.Styles["normal"]; okStyle && style.Text != "" {
			return style.Text
		}
		for _, s := range dang.Styles {
			if s.Text != "" {
				return s.Text
			}
		}
	}
	return ""
}

func parseMotionRule(exerciseID, specName, recommendedCameraAngle string, ruleBytes, dialogueBytes []byte) (*vo.ExerciseMotionRule, error) {
	var rawRule rawRuleFile
	if err := json.Unmarshal(ruleBytes, &rawRule); err != nil {
		return nil, fmt.Errorf("unmarshal rule json: %w", err)
	}

	var rawDialogue rawDialogueFile
	if len(dialogueBytes) > 0 {
		_ = json.Unmarshal(dialogueBytes, &rawDialogue)
	}

	name := rawRule.DisplayName
	if name == "" {
		name = specName
	}
	if name == "" {
		name = exerciseID
	}

	repType := strings.ToLower(strings.TrimSpace(rawRule.RepType))
	challengeType := vo.ChallengeTypeReps
	if repType == "timed" || repType == "duration" {
		challengeType = vo.ChallengeTypeDuration
	}

	timeCeiling := 10 * time.Minute
	if challengeType == vo.ChallengeTypeDuration {
		timeCeiling = 30 * time.Minute
	}

	// 1. Process PhaseDetection
	phaseDetection := rawRule.PhaseDetection
	if phaseDetection != nil {
		formulaStr := strings.ToLower(string(phaseDetection.Calculation.Formula))
		if strings.Contains(formulaStr, "arccos") || strings.Contains(formulaStr, "arctan2") || strings.Contains(formulaStr, "angle") {
			phaseDetection.Calculation.Formula = vo.FormulaAngle3D
		} else if phaseDetection.Calculation.Formula == "" {
			phaseDetection.Calculation.Formula = vo.FormulaAngle3D
		}

		// For timed/DURATION challenges (like Plank): map always -> Active, idle -> Rest
		if challengeType == vo.ChallengeTypeDuration {
			if phaseDetection.Thresholds.Active.LTE == 0 && phaseDetection.Thresholds.Always.LTE > 0 {
				phaseDetection.Thresholds.Active = phaseDetection.Thresholds.Always
			}
			if phaseDetection.Thresholds.Rest.GT == 0 && phaseDetection.Thresholds.Idle.GT > 0 {
				phaseDetection.Thresholds.Rest = phaseDetection.Thresholds.Idle
			}
		}

		if len(phaseDetection.Calculation.Joints) == 0 && len(phaseDetection.Calculation.Landmarks) > 0 {
			phaseDetection.Calculation.Joints = parseLandmarkNumbers(phaseDetection.Calculation.Landmarks)
		}
	}

	// 2. Process criteria rules and map dialogue audio cues
	var criteria []vo.RuleCriterion
	for _, r := range rawRule.Rules {
		crit := vo.RuleCriterion{
			Name:            r.Metric,
			PenaltyPerError: 10.0,
			ApplyForView:    strings.ToLower(strings.TrimSpace(r.ApplyForView)),
		}

		evalType := strings.ToLower(strings.TrimSpace(r.EvaluationType))
		metricUpper := strings.ToUpper(r.Metric)
		if evalType == "two_sided" {
			crit.ErrorCodeBelow = "ERR_" + metricUpper + "_TOO_LOW"
			crit.ErrorCodeAbove = "ERR_" + metricUpper + "_TOO_HIGH"
		} else {
			crit.ErrorCodeAbove = "ERR_" + metricUpper
		}

		calcFormula := strings.ToLower(r.Calculation.Formula)
		if strings.Contains(calcFormula, "sway") || strings.Contains(calcFormula, "distance") || strings.Contains(calcFormula, "torso_length") || strings.Contains(calcFormula, "diff") || strings.Contains(calcFormula, "ratio") {
			crit.Formula = vo.FormulaDistance
		} else {
			crit.Formula = vo.FormulaAngle3D
		}

		if len(r.Calculation.Joints) > 0 {
			crit.Joints = r.Calculation.Joints
		} else {
			crit.Joints = parseLandmarkNumbers(r.Calculation.Landmarks)
		}

		for _, rng := range r.Ranges {
			if rng.Severity == 0 {
				if rng.Min != nil {
					crit.ValidRange.Min = *rng.Min
				}
				if rng.Max != nil {
					crit.ValidRange.Max = *rng.Max
				}
				break
			}
		}

		if len(rawDialogue.VoiceFeedbacks) > 0 {
			if fb, ok := rawDialogue.VoiceFeedbacks[r.Metric]; ok {
				crit.AudioCue = extractDialogueText(fb)
			}
		}

		criteria = append(criteria, crit)
	}

	motionRule := &vo.ExerciseMotionRule{
		ExerciseID:             exerciseID,
		ExerciseName:           name,
		DisplayName:            name,
		ChallengeType:          challengeType,
		RepType:                rawRule.RepType,
		Notes:                  rawRule.Notes,
		MinConfidence:          0.5,
		PhaseDetection:         phaseDetection,
		Rules:                  criteria,
		PassCondition:          "ALL_MATCH",
		TimeCeiling:            timeCeiling,
		ViolationMaxDuration:   5 * time.Second,
		RecommendedCameraAngle: recommendedCameraAngle,
	}

	return motionRule, nil
}
