//go:build e2e || integration

package e2e

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	exercisemsg "github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/exercise/v1/message"
	profilev1event "github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/profile/v1/event"
	profilev1message "github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/profile/v1/message"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
)

func TestProfile_FullLifecycle_E2E(t *testing.T) {
	suite := SetupE2ESuite(t)
	defer suite.StopServer()

	userID := uuid.NewString()
	userCtx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("x-user-id", userID, "x-user-role", "User"))

	// 1. Initial SaveHealthProfile
	dob := time.Date(1995, 5, 20, 0, 0, 0, 0, time.UTC).Format(time.RFC3339)
	saveResp, err := suite.Client.SaveHealthProfile(userCtx, &profilev1message.SaveHealthProfileRequest{
		WeightKg:              75.5,
		HeightCm:              178.0,
		BodyFatPercent:        18.5,
		DateOfBirth:           dob,
		Gender:                "MALE",
		Goals:                 []string{"BUILD_MUSCLE", "LOSE_FAT"},
		ExperienceLevel:       "INTERMEDIATE",
		PreferredWorkoutTimes: []string{"MORNING", "EVENING"},
		AvailableEquipment:    []exercisemsg.Equipment{exercisemsg.Equipment_EQUIPMENT_BARBELL, exercisemsg.Equipment_EQUIPMENT_DUMBBELL},
		PreferredMuscleGroups: []exercisemsg.MuscleGroup{exercisemsg.MuscleGroup_MUSCLE_GROUP_CHEST, exercisemsg.MuscleGroup_MUSCLE_GROUP_LATS},
		Injuries: []*profilev1message.InjuryInput{
			{
				Region:   exercisemsg.JointRegion_JOINT_REGION_LOWER_BACK,
				Severity: exercisemsg.InjurySeverity_INJURY_SEVERITY_MILD,
				Notes:    "Slight strain during deadlifts",
			},
		},
	})
	if err != nil {
		t.Fatalf("SaveHealthProfile failed: %v", err)
	}
	if saveResp.GetUserId() != userID {
		t.Errorf("Expected UserID %s, got %s", userID, saveResp.GetUserId())
	}
	if saveResp.GetCompletionRate() <= 0 {
		t.Errorf("Expected CompletionRate > 0, got %v", saveResp.GetCompletionRate())
	}

	// 2. Query GetProfile
	getResp, err := suite.Client.GetProfile(userCtx, &profilev1message.GetProfileRequest{})
	if err != nil {
		t.Fatalf("GetProfile failed: %v", err)
	}
	if getResp.GetUserId() != userID {
		t.Errorf("Expected UserID %s, got %s", userID, getResp.GetUserId())
	}
	if getResp.GetWeightKg() != 75.5 {
		t.Errorf("Expected Weight 75.5, got %v", getResp.GetWeightKg())
	}
	if getResp.GetCompletionRate() <= 0 {
		t.Errorf("Expected CompletionRate > 0, got %v", getResp.GetCompletionRate())
	}
	if got := getResp.GetAvailableEquipment(); len(got) != 2 || got[1] != exercisemsg.Equipment_EQUIPMENT_DUMBBELL {
		t.Errorf("Unexpected available equipment: %v", got)
	}
	if got := getResp.GetInjuries(); len(got) != 1 ||
		got[0].GetRegion() != exercisemsg.JointRegion_JOINT_REGION_LOWER_BACK ||
		got[0].GetSide() != exercisemsg.BodySide_BODY_SIDE_BOTH {
		t.Errorf("Unexpected injuries: %v", got)
	}

	// 3. UpdateProfile Preferences
	updateResp, err := suite.Client.UpdateProfile(userCtx, &profilev1message.UpdateProfileRequest{
		Goals:                 []string{"STRENGTH_BUILDING"},
		PreferredWorkoutTimes: []string{"MORNING"},
		AvailableEquipment: []exercisemsg.Equipment{
			exercisemsg.Equipment_EQUIPMENT_BARBELL, exercisemsg.Equipment_EQUIPMENT_DUMBBELL, exercisemsg.Equipment_EQUIPMENT_CABLE,
		},
		PreferredMuscleGroups: []exercisemsg.MuscleGroup{
			exercisemsg.MuscleGroup_MUSCLE_GROUP_CHEST, exercisemsg.MuscleGroup_MUSCLE_GROUP_QUADRICEPS,
		},
		CoachStyle:           "MOTIVATIONAL",
		TargetWeightKg:       80.0,
		TargetBodyFatPercent: 15.0,
	})
	if err != nil {
		t.Fatalf("UpdateProfile failed: %v", err)
	}
	if !updateResp.GetSuccess() {
		t.Errorf("Expected UpdateProfile success = true")
	}

	// Verify updated profile via GetProfile
	updatedGetResp, err := suite.Client.GetProfile(userCtx, &profilev1message.GetProfileRequest{})
	if err != nil {
		t.Fatalf("GetProfile after update failed: %v", err)
	}
	if len(updatedGetResp.GetGoals()) != 1 || updatedGetResp.GetGoals()[0] != "STRENGTH_BUILDING" {
		t.Errorf("Unexpected goals after update: %v", updatedGetResp.GetGoals())
	}

	// 4. LogPeriodicMetrics
	logMetricsResp, err := suite.Client.LogPeriodicMetrics(userCtx, &profilev1message.LogPeriodicMetricsRequest{
		WeightKg:         76.0,
		HeightCm:         178.0,
		BodyFatPercent:   18.0,
		ProgressPhotoUrl: "https://storage.fitai.com/photos/progress1.jpg",
	})
	if err != nil {
		t.Fatalf("LogPeriodicMetrics failed: %v", err)
	}
	if logMetricsResp.GetWeightKg() != 76.0 {
		t.Errorf("Expected weight 76.0, got %v", logMetricsResp.GetWeightKg())
	}

	// 5. GetBodyMetricsHistory
	metricsHistResp, err := suite.Client.GetBodyMetricsHistory(userCtx, &profilev1message.GetBodyMetricsHistoryRequest{
		UserId: userID,
	})
	if err != nil {
		t.Fatalf("GetBodyMetricsHistory failed: %v", err)
	}
	if len(metricsHistResp.GetMetrics()) < 2 {
		t.Errorf("Expected at least 2 metrics in history (initial + periodic), got %d", len(metricsHistResp.GetMetrics()))
	}

	// 6. ReportInjury & GetInjuryHistory
	reportInjResp, err := suite.Client.ReportInjury(userCtx, &profilev1message.ReportInjuryRequest{
		Region:   exercisemsg.JointRegion_JOINT_REGION_SHOULDER,
		Muscle:   exercisemsg.MuscleGroup_MUSCLE_GROUP_SHOULDERS,
		Side:     exercisemsg.BodySide_BODY_SIDE_RIGHT,
		Severity: exercisemsg.InjurySeverity_INJURY_SEVERITY_MODERATE,
		Notes:    "Right rotator cuff discomfort",
	})
	if err != nil {
		t.Fatalf("ReportInjury failed: %v", err)
	}
	newInjuryID := reportInjResp.GetInjuryId()
	if newInjuryID == "" {
		t.Fatalf("ReportInjury returned empty injury ID")
	}

	assertInjuryReportedEvent(t, suite, newInjuryID)

	injHistResp, err := suite.Client.GetInjuryHistory(userCtx, &profilev1message.GetInjuryHistoryRequest{
		UserId: userID,
	})
	if err != nil {
		t.Fatalf("GetInjuryHistory failed: %v", err)
	}
	if len(injHistResp.GetInjuries()) < 2 {
		t.Errorf("Expected at least 2 injuries in history, got %d", len(injHistResp.GetInjuries()))
	}

	// 7. RecoverInjury
	recoverResp, err := suite.Client.RecoverInjury(userCtx, &profilev1message.RecoverInjuryRequest{
		InjuryId: newInjuryID,
	})
	if err != nil {
		t.Fatalf("RecoverInjury failed: %v", err)
	}
	if !recoverResp.GetSuccess() {
		t.Errorf("Expected RecoverInjury success = true")
	}

	// 8. Verify Outbox Records Created
	var outboxCount int64
	if err := suite.DB.Table("profile.outbox").Count(&outboxCount).Error; err != nil {
		t.Fatalf("Failed to count profile outbox records: %v", err)
	}
	if outboxCount == 0 {
		t.Errorf("Expected outbox events to be created in profile.outbox, got 0")
	}
}

func TestProfile_AccessControl_E2E(t *testing.T) {
	suite := SetupE2ESuite(t)
	defer suite.StopServer()

	user1ID := uuid.NewString()
	user2ID := uuid.NewString()

	user1Ctx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("x-user-id", user1ID, "x-user-role", "User"))
	user2Ctx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("x-user-id", user2ID, "x-user-role", "User"))
	adminCtx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("x-user-id", "admin-id", "x-user-role", "Admin"))

	// User 1 creates profile
	_, err := suite.Client.SaveHealthProfile(user1Ctx, &profilev1message.SaveHealthProfileRequest{
		WeightKg: 70.0,
		HeightCm: 170.0,
	})
	if err != nil {
		t.Fatalf("SaveHealthProfile for User 1 failed: %v", err)
	}

	// User 2 attempts to fetch User 1's profile -> Should be denied
	_, err = suite.Client.GetProfile(user2Ctx, &profilev1message.GetProfileRequest{
		UserId: user1ID,
	})
	if err == nil {
		t.Errorf("Expected PermissionDenied when User 2 accesses User 1's profile, got nil error")
	}

	// Admin fetches User 1's profile -> Should succeed
	adminGetResp, err := suite.Client.GetProfile(adminCtx, &profilev1message.GetProfileRequest{
		UserId: user1ID,
	})
	if err != nil {
		t.Fatalf("Admin GetProfile for User 1 failed: %v", err)
	}
	if adminGetResp.GetUserId() != user1ID {
		t.Errorf("Admin got profile UserID = %s, want %s", adminGetResp.GetUserId(), user1ID)
	}
}

// assertInjuryReportedEvent checks the outbox CloudEvent carries the typed
// injury location (docs/05 section 6.5, 6.6).
func assertInjuryReportedEvent(t *testing.T, suite *TestSuite, injuryID string) {
	t.Helper()

	var payloads [][]byte
	if err := suite.DB.Table("profile.outbox").
		Where("event_type = ?", "contracts.supporting.profile.v1.event.InjuryReported").
		Pluck("payload", &payloads).Error; err != nil {
		t.Fatalf("query InjuryReported outbox: %v", err)
	}

	for _, payload := range payloads {
		var envelope struct {
			Data json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(payload, &envelope); err != nil {
			t.Fatalf("unmarshal envelope: %v", err)
		}
		var event profilev1event.InjuryReported
		if err := protojson.Unmarshal(envelope.Data, &event); err != nil {
			t.Fatalf("unmarshal InjuryReported: %v", err)
		}
		if event.GetInjury().GetInjuryId() != injuryID {
			continue
		}

		inj := event.GetInjury()
		if inj.GetRegion() != exercisemsg.JointRegion_JOINT_REGION_SHOULDER ||
			inj.GetMuscle() != exercisemsg.MuscleGroup_MUSCLE_GROUP_SHOULDERS ||
			inj.GetSide() != exercisemsg.BodySide_BODY_SIDE_RIGHT ||
			inj.GetSeverity() != exercisemsg.InjurySeverity_INJURY_SEVERITY_MODERATE {
			t.Errorf("unexpected InjuryReported payload: %v", inj)
		}
		return
	}
	t.Errorf("no InjuryReported event for injury %s in %d outbox rows", injuryID, len(payloads))
}

func TestProfile_ReportInjuryRejectsUnspecifiedRegion_E2E(t *testing.T) {
	suite := SetupE2ESuite(t)
	defer suite.StopServer()

	userCtx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("x-user-id", uuid.NewString(), "x-user-role", "User"))
	if _, err := suite.Client.SaveHealthProfile(userCtx, &profilev1message.SaveHealthProfileRequest{WeightKg: 70, HeightCm: 170}); err != nil {
		t.Fatalf("SaveHealthProfile failed: %v", err)
	}

	_, err := suite.Client.ReportInjury(userCtx, &profilev1message.ReportInjuryRequest{
		Severity: exercisemsg.InjurySeverity_INJURY_SEVERITY_MILD,
	})
	if got := status.Code(err); got != codes.InvalidArgument {
		t.Errorf("got code %v, want InvalidArgument", got)
	}
}
