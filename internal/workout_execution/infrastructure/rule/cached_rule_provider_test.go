package rule

import (
	"context"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/workout_execution/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/derror"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/repository"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/vo"
)

type mockMotionSpecRepo struct {
	specs map[string]*aggregate.MotionSpecification
}

func newMockMotionSpecRepo() *mockMotionSpecRepo {
	return &mockMotionSpecRepo{specs: make(map[string]*aggregate.MotionSpecification)}
}

func (m *mockMotionSpecRepo) FindByExerciseID(ctx context.Context, exerciseID string) (*aggregate.MotionSpecification, error) {
	spec, ok := m.specs[exerciseID]
	if !ok {
		return nil, derror.ErrMotionSpecNotFound
	}
	return spec, nil
}

func (m *mockMotionSpecRepo) Save(ctx context.Context, spec *aggregate.MotionSpecification) error {
	m.specs[spec.ExerciseID()] = spec
	return nil
}

func (m *mockMotionSpecRepo) Update(ctx context.Context, spec *aggregate.MotionSpecification) error {
	m.specs[spec.ExerciseID()] = spec
	return nil
}

func (m *mockMotionSpecRepo) Delete(ctx context.Context, exerciseID string) error {
	delete(m.specs, exerciseID)
	return nil
}

func (m *mockMotionSpecRepo) List(ctx context.Context, limit, offset int) ([]*aggregate.MotionSpecification, int, error) {
	var list []*aggregate.MotionSpecification
	for _, s := range m.specs {
		list = append(list, s)
	}
	return list, len(list), nil
}

func (m *mockMotionSpecRepo) Search(ctx context.Context, keyword string, limit, offset int) ([]*aggregate.MotionSpecification, int, error) {
	return m.List(ctx, limit, offset)
}

func (m *mockMotionSpecRepo) GetStats(ctx context.Context) (total int, activeRules int, activeVoice int, readySpecs int, err error) {
	return len(m.specs), len(m.specs), len(m.specs), len(m.specs), nil
}

var _ repository.MotionSpecificationRepository = (*mockMotionSpecRepo)(nil)

type mockStorageProvider struct {
	storage map[string][]byte
	hits    map[string]int
}

func newMockStorageProvider() *mockStorageProvider {
	return &mockStorageProvider{
		storage: make(map[string][]byte),
		hits:    make(map[string]int),
	}
}

func (m *mockStorageProvider) GeneratePresignedUploadURL(ctx context.Context, fileName, contentType string) (string, string, error) {
	return "http://upload/" + fileName, "http://cdn/" + fileName, nil
}

func (m *mockStorageProvider) GetObject(ctx context.Context, objectKey string) ([]byte, error) {
	m.hits[objectKey]++
	data, ok := m.storage[objectKey]
	if !ok {
		return nil, nil
	}
	return data, nil
}

func (m *mockStorageProvider) PutObject(ctx context.Context, objectKey string, data []byte, contentType string) (string, error) {
	m.storage[objectKey] = data
	return "http://cdn/" + objectKey, nil
}

func TestCachedRuleProvider_LoadAssetsAndCache(t *testing.T) {
	ctx := context.Background()

	// 1. Prepare raw JSON assets (exact structures provided by user)
	bicepRuleJSON := []byte(`{
  "display_name": "Bicep Curl",
  "rep_type": "counted",
  "notes": "Bài tập đếm rep. Cùi chỏ cố định sát sườn, không vung/lắc. Cột sống thẳng, không ngửa ra sau.",
  "phase_detection": {
    "metric": "elbow_angle",
    "thresholds": {
      "rest": {
        "gt": 155,
        "comment": "Tay duỗi thẳng (đáy rep), chuẩn bị cuốn lên, check spine + sway"
      },
      "transition": {
        "gt": 80,
        "comment": "Đang cuốn lên/xuống (giữa ROM), check form liên tục"
      },
      "active": {
        "lte": 80,
        "comment": "Đỉnh curl (tạ gần vai), peak contraction"
      }
    },
    "calculation": {
      "landmarks": [
        "Shoulder (P5, P6)",
        "Elbow (P7, P8)",
        "Wrist (P9, P10)"
      ],
      "vectors": [
        "U = Shoulder - Elbow",
        "W = Wrist - Elbow"
      ],
      "formula": "θ = arccos((U · W) / (||U|| * ||W||)) * (180 / π)",
      "description": "Góc gập khớp khuỷu tay hợp bởi cánh tay (vai-khuỷu) và cẳng tay (cổ tay-khuỷu)"
    }
  },
  "rules": [
    {
      "metric": "elbow_sway",
      "description": "Độ di chuyển cùi chỏ ra trước/sau (Elbow Sway - Cheat Curl)",
      "evaluation_type": "upper_bound",
      "apply_in_phases": ["rest", "transition", "active"],
      "apply_for_view": "side",
      "ranges": [
        {
          "severity": 0,
          "status": "Correct",
          "min": 0.0,
          "max": 0.09,
          "comment": "≤9cm: cùi chỏ cố định, biceps là cơ chính"
        },
        {
          "severity": 1,
          "status": "Warning",
          "min": 0.09,
          "max": 0.18,
          "comment": "9-18cm: cùi chỏ bắt đầu vung, vai trước mượn lực"
        },
        {
          "severity": 2,
          "status": "Danger",
          "min": 0.18,
          "max": null,
          "comment": ">18cm: cheat curl rõ"
        }
      ],
      "calculation": {
        "landmarks": [
          "Shoulder (P5, P6)",
          "Elbow (P7, P8)"
        ],
        "formula": "sway = max(|X_elbow_L - X_shoulder_L|, |X_elbow_R - X_shoulder_R|) / torso_length",
        "description": "Độ lắc ngang X của cùi chỏ so với vai chuẩn hóa theo torso_length"
      }
    },
    {
      "metric": "spine_sway_angle",
      "description": "Góc ngửa cột sống thắt lưng ra sau (Back Arching Cheat)",
      "evaluation_type": "upper_bound",
      "apply_in_phases": ["rest", "transition", "active"],
      "apply_for_view": "side",
      "ranges": [
        {
          "severity": 0,
          "status": "Correct",
          "min": 0.0,
          "max": 5.0,
          "comment": "≤5°: thân thẳng, cơ lõi ổn định"
        },
        {
          "severity": 1,
          "status": "Warning",
          "min": 5.0,
          "max": 10.0,
          "comment": "5-10°: bắt đầu ngửa lưng lấy đà"
        },
        {
          "severity": 2,
          "status": "Danger",
          "min": 10.0,
          "max": null,
          "comment": ">10°: cheat rõ rệt"
        }
      ],
      "calculation": {
        "landmarks": [
          "Mid-Shoulder (P5, P6)",
          "Mid-Hip (P11, P12)"
        ],
        "vectors": [
          "S = Mid-Shoulder - Mid-Hip",
          "V = [0, -1, 0] (Vertical Vector)"
        ],
        "formula": "θ = arccos((S · V) / (||S|| * ||V||)) * (180 / π)",
        "description": "Góc nghiêng/ưỡn của cột sống thắt lưng"
      }
    }
  ],
  "audio_cooldown_seconds": {
    "warning": 3.0,
    "danger": 1.5
  }
}`)

	bicepDialogueJSON := []byte(`{
  "exercise": "bicep_curl",
  "display_name": "Bicep Curl",
  "description": "Các câu thoại hướng dẫn giọng nói cho bài tập Bicep Curl theo từng mức độ lỗi và phong cách",
  "voice_feedbacks": {
    "elbow_sway": {
      "metric_name": "elbow_sway",
      "description": "Di động cùi chỏ ra trước (Elbow Sway)",
      "severities": {
        "warning": {
          "severity_level": 1,
          "status": "Warning",
          "styles": {
            "normal": {
              "text": "Cùi chỏ hơi đung đưa ra trước khi cuốn tạ lên.",
              "url": ""
            },
            "strict": {
              "text": "Cố định cùi chỏ! Khóa cùi chỏ sát sườn không cho vung lắc!",
              "url": ""
            }
          }
        }
      }
    },
    "spine_sway_angle": {
      "metric_name": "spine_sway_angle",
      "description": "Ngửa lưng lấy đà (Back Arching Cheat)",
      "severities": {
        "warning": {
          "severity_level": 1,
          "status": "Warning",
          "styles": {
            "normal": {
              "text": "Thân người hơi ngửa ra sau để ăn bớt tải khi cuốn tạ.",
              "url": ""
            }
          }
        }
      }
    }
  }
}`)

	// 2. Setup mock storage and repo
	storage := newMockStorageProvider()
	storage.storage["rules/bicep_curl_rule.json"] = bicepRuleJSON
	storage.storage["dialogues/bicep_curl_dialogue.json"] = bicepDialogueJSON

	specRepo := newMockMotionSpecRepo()
	spec := aggregate.RestoreMotionSpecification(
		"bicep_curl",
		"Bicep Curl",
		"http://cdn/models/detector.onnx",
		"http://cdn/models/skeleton.onnx",
		"rules/bicep_curl_rule.json",
		"dialogues/bicep_curl_dialogue.json",
		"side",
		true,
		timeNow(),
		timeNow(),
	)
	_ = specRepo.Save(ctx, spec)

	provider := NewCachedRuleProvider(specRepo, storage)

	// 3. First GetRule call: should fetch from storage, parse, merge, and cache
	rule, err := provider.GetRule(ctx, "bicep_curl")
	if err != nil {
		t.Fatalf("unexpected error fetching bicep_curl rule: %v", err)
	}

	if rule.ExerciseID != "bicep_curl" {
		t.Errorf("got ExerciseID = %s, want bicep_curl", rule.ExerciseID)
	}
	if rule.ChallengeType != vo.ChallengeTypeReps {
		t.Errorf("got ChallengeType = %s, want REPS", rule.ChallengeType)
	}
	if rule.PhaseDetection == nil {
		t.Fatalf("expected non-nil PhaseDetection")
	}
	if rule.PhaseDetection.Metric != "elbow_angle" {
		t.Errorf("got PhaseDetection.Metric = %s, want elbow_angle", rule.PhaseDetection.Metric)
	}
	if rule.PhaseDetection.Thresholds.Rest.GT != 155.0 {
		t.Errorf("got Rest.GT = %f, want 155.0", rule.PhaseDetection.Thresholds.Rest.GT)
	}
	if rule.PhaseDetection.Thresholds.Active.LTE != 80.0 {
		t.Errorf("got Active.LTE = %f, want 80.0", rule.PhaseDetection.Thresholds.Active.LTE)
	}
	// Check landmark parsing: P5, P7, P9 extracted into joints
	if len(rule.PhaseDetection.Calculation.Joints) != 3 {
		t.Fatalf("expected 3 joints parsed from landmarks, got %v", rule.PhaseDetection.Calculation.Joints)
	}
	if rule.PhaseDetection.Calculation.Joints[0] != 5 || rule.PhaseDetection.Calculation.Joints[1] != 7 || rule.PhaseDetection.Calculation.Joints[2] != 9 {
		t.Errorf("expected joints [5, 7, 9], got %v", rule.PhaseDetection.Calculation.Joints)
	}

	// Check form rules and dialogue merged cues
	if len(rule.Rules) != 2 {
		t.Fatalf("expected 2 criteria rules, got %d", len(rule.Rules))
	}
	elbowSwayRule := rule.Rules[0]
	if elbowSwayRule.Name != "elbow_sway" {
		t.Errorf("got rule name = %s, want elbow_sway", elbowSwayRule.Name)
	}
	if elbowSwayRule.ValidRange.Max != 0.09 {
		t.Errorf("got ValidRange.Max = %f, want 0.09", elbowSwayRule.ValidRange.Max)
	}
	if elbowSwayRule.AudioCue != "Cùi chỏ hơi đung đưa ra trước khi cuốn tạ lên." {
		t.Errorf("got AudioCue = '%s', want 'Cùi chỏ hơi đung đưa ra trước khi cuốn tạ lên.'", elbowSwayRule.AudioCue)
	}

	spineSwayRule := rule.Rules[1]
	if spineSwayRule.Name != "spine_sway_angle" {
		t.Errorf("got rule name = %s, want spine_sway_angle", spineSwayRule.Name)
	}
	if spineSwayRule.AudioCue != "Thân người hơi ngửa ra sau để ăn bớt tải khi cuốn tạ." {
		t.Errorf("got AudioCue = '%s', want 'Thân người hơi ngửa ra sau để ăn bớt tải khi cuốn tạ.'", spineSwayRule.AudioCue)
	}

	// 4. Second GetRule call: should hit in-memory cache directly (hits count must NOT increase)
	initialRuleHits := storage.hits["rules/bicep_curl_rule.json"]
	cachedRule, err := provider.GetRule(ctx, "bicep_curl")
	if err != nil {
		t.Fatalf("unexpected error on second call: %v", err)
	}
	if cachedRule != rule {
		t.Errorf("expected same rule pointer from cache")
	}
	if storage.hits["rules/bicep_curl_rule.json"] != initialRuleHits {
		t.Errorf("storage was hit again, cache did not work: hits = %d, initial = %d", storage.hits["rules/bicep_curl_rule.json"], initialRuleHits)
	}

	// 5. Query for unknown unconfigured exercise must fail (NO arbitrary fake seed)
	_, err = provider.GetRule(ctx, "unknown_unconfigured_exercise")
	if err == nil {
		t.Fatalf("expected error for unconfigured exercise, got nil")
	}
}

func TestCachedRuleProvider_PlankTimedExercise(t *testing.T) {
	specRepo := newMockMotionSpecRepo()
	storage := newMockStorageProvider()

	plankRuleJSON := `{
  "display_name": "Plank",
  "rep_type": "timed",
  "notes": "Bài tập tính thời gian. Yêu cầu toàn bộ cơ thể tạo thành một đường thẳng song song mặt sàn, khuỷu tay chống vuông góc đỡ thân người. Nguồn: ACE Fitness, NASM Core Assessment, NSCA.",
  "phase_detection": {
    "metric": "body_incline_angle",
    "thresholds": {
      "idle": {
        "gt": 35.0,
        "comment": "Thân người nghiêng > 35° so với phương ngang (đang đứng hoặc chưa vào tư thế sàn) -> Tạm dừng tính giờ"
      },
      "always": {
        "lte": 35.0,
        "comment": "Thân người nằm ngang sàn (≤ 35°) -> Kích hoạt đánh giá form và tính giờ giữ Plank"
      }
    },
    "calculation": {
      "landmarks": [
        "Shoulder (P5, P6)",
        "Ankle (P15, P16)"
      ],
      "formula": "θ_incline = |arctan2(Y_shoulder_mid - Y_ankle_mid, X_shoulder_mid - X_ankle_mid)| * (180 / π)",
      "description": "Góc nghiêng của trục cơ thể (Vai - Cổ chân) so với phương ngang"
    }
  },
  "rules": [
    {
      "metric": "body_incline_angle",
      "description": "Góc nghiêng cơ thể so với phương nằm ngang",
      "evaluation_type": "upper_bound",
      "apply_in_phases": ["always"],
      "apply_for_view": "side",
      "ranges": [
        {"severity": 0, "status": "Correct", "min": 0.0, "max": 25.0},
        {"severity": 1, "status": "Warning", "min": 25.0, "max": 35.0},
        {"severity": 2, "status": "Danger", "min": 35.0, "max": null}
      ],
      "calculation": {
        "landmarks": ["Shoulder (P5, P6)", "Ankle (P15, P16)"],
        "formula": "body_incline_angle = |arctan2(Y_shoulder_mid - Y_ankle_mid, X_shoulder_mid - X_ankle_mid)| * (180 / π)"
      }
    },
    {
      "metric": "arm_to_torso_angle",
      "description": "Góc giữa bắp tay và thân người Hip-Shoulder-Elbow",
      "evaluation_type": "two_sided",
      "apply_in_phases": ["always"],
      "apply_for_view": "side",
      "ranges": [
        {"severity": 0, "status": "Correct", "min": 75.0, "max": 105.0},
        {"severity": 1, "status": "Warning", "min": 60.0, "max": 120.0},
        {"severity": 2, "status": "Danger", "min": 0.0, "max": null}
      ],
      "calculation": {
        "landmarks": ["Hip (P11, P12)", "Shoulder (P5, P6)", "Elbow (P7, P8)"],
        "formula": "arm_to_torso_angle = arccos((U · W) / (||U|| * ||W||)) * (180 / π)"
      }
    },
    {
      "metric": "signed_hip_y_diff",
      "description": "Độ lệch hông so với đường thẳng vai-cổ chân",
      "evaluation_type": "absolute_upper_bound",
      "apply_in_phases": ["always"],
      "apply_for_view": "side",
      "ranges": [
        {"severity": 0, "status": "Correct", "min": 0.0, "max": 0.09},
        {"severity": 1, "status": "Warning", "min": 0.09, "max": 0.18},
        {"severity": 2, "status": "Danger", "min": 0.18, "max": null}
      ],
      "calculation": {
        "landmarks": ["Shoulder (P5, P6)", "Hip (P11, P12)", "Ankle (P15, P16)"],
        "formula": "signed_hip_y_diff = signed_distance_point_to_line(P_hip_mid, P_shoulder_mid, P_ankle_mid) / torso_length"
      }
    },
    {
      "metric": "elbow_angle",
      "description": "Góc gập khớp khuỷu tay",
      "evaluation_type": "two_sided",
      "apply_in_phases": ["always"],
      "apply_for_view": "side",
      "ranges": [
        {"severity": 0, "status": "Correct", "min": 75.0, "max": 105.0},
        {"severity": 1, "status": "Warning", "min": 60.0, "max": 120.0},
        {"severity": 2, "status": "Danger", "min": 0.0, "max": null}
      ],
      "calculation": {
        "landmarks": ["Shoulder (P5, P6)", "Elbow (P7, P8)", "Wrist (P9, P10)"],
        "formula": "elbow_angle = arccos((U · W) / (||U|| * ||W||)) * (180 / π)"
      }
    },
    {
      "metric": "hand_width_ratio",
      "description": "Độ rộng đặt hai khuỷu tay so với độ rộng vai",
      "evaluation_type": "two_sided",
      "apply_in_phases": ["always"],
      "apply_for_view": "front",
      "ranges": [
        {"severity": 0, "status": "Correct", "min": 0.85, "max": 1.2},
        {"severity": 1, "status": "Warning", "min": 0.65, "max": 1.45},
        {"severity": 2, "status": "Danger", "min": 0.0, "max": null}
      ],
      "calculation": {
        "landmarks": ["Shoulder (P5, P6)", "Elbow (P7, P8)"],
        "formula": "hand_width_ratio = |X_elbow_left - X_elbow_right| / |X_shoulder_left - X_shoulder_right|"
      }
    }
  ]
}`

	storage.storage["rules/plank_rule.json"] = []byte(plankRuleJSON)

	plankSpec := aggregate.RestoreMotionSpecification(
		"plank",
		"Plank",
		"",
		"",
		"https://lqhhwimtzrcrmyfhlizv.supabase.co/storage/v1/object/public/gym-companion-assets/rules/plank_rule.json",
		"",
		"SIDE",
		true,
		timeNow(),
		timeNow(),
	)
	_ = specRepo.Save(context.Background(), plankSpec)

	provider := NewCachedRuleProvider(specRepo, storage)
	rule, err := provider.GetRule(context.Background(), "plank")
	if err != nil {
		t.Fatalf("failed to get plank rule: %v", err)
	}

	if rule.ChallengeType != vo.ChallengeTypeDuration {
		t.Errorf("got ChallengeType = %s, want DURATION", rule.ChallengeType)
	}
	if rule.TimeCeiling != 30*time.Minute {
		t.Errorf("got TimeCeiling = %v, want 30m", rule.TimeCeiling)
	}
	if rule.PhaseDetection.Thresholds.Always.LTE != 35.0 {
		t.Errorf("got Always.LTE = %f, want 35.0", rule.PhaseDetection.Thresholds.Always.LTE)
	}
	if rule.PhaseDetection.Thresholds.Idle.GT != 35.0 {
		t.Errorf("got Idle.GT = %f, want 35.0", rule.PhaseDetection.Thresholds.Idle.GT)
	}
	// Check active mapped from always
	if rule.PhaseDetection.Thresholds.Active.LTE != 35.0 {
		t.Errorf("got Active.LTE = %f, want 35.0", rule.PhaseDetection.Thresholds.Active.LTE)
	}

	if len(rule.Rules) != 5 {
		t.Fatalf("expected 5 criteria rules, got %d", len(rule.Rules))
	}

	// Verify two_sided rule: arm_to_torso_angle
	armTorsoRule := rule.Rules[1]
	if armTorsoRule.Name != "arm_to_torso_angle" {
		t.Errorf("expected rule 1 to be arm_to_torso_angle, got %s", armTorsoRule.Name)
	}
	if armTorsoRule.ValidRange.Min != 75.0 || armTorsoRule.ValidRange.Max != 105.0 {
		t.Errorf("expected range [75, 105], got [%f, %f]", armTorsoRule.ValidRange.Min, armTorsoRule.ValidRange.Max)
	}
	if armTorsoRule.ErrorCodeBelow != "ERR_ARM_TO_TORSO_ANGLE_TOO_LOW" {
		t.Errorf("got ErrorCodeBelow = %s, want ERR_ARM_TO_TORSO_ANGLE_TOO_LOW", armTorsoRule.ErrorCodeBelow)
	}
	if armTorsoRule.ErrorCodeAbove != "ERR_ARM_TO_TORSO_ANGLE_TOO_HIGH" {
		t.Errorf("got ErrorCodeAbove = %s, want ERR_ARM_TO_TORSO_ANGLE_TOO_HIGH", armTorsoRule.ErrorCodeAbove)
	}
}

func timeNow() time.Time {
	return time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
}
