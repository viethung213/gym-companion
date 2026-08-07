package consumer

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/segmentio/kafka-go"
	authv1event "github.com/viethung213/gym-companion/internal/gen/go/contracts/generic/auth/v1/event"
	profilev1event "github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/profile/v1/event"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/aggregate"
	"google.golang.org/protobuf/encoding/protojson"
)

type mockPlanRepoForConsumer struct {
	savedUserID    string
	savedSchedules map[string]string
}

func (m *mockPlanRepoForConsumer) GetUserMealSchedules(ctx context.Context, userID string) (map[string]string, error) {
	return m.savedSchedules, nil
}

func (m *mockPlanRepoForConsumer) SaveUserMealSchedules(ctx context.Context, userID string, schedules map[string]string) error {
	m.savedUserID = userID
	m.savedSchedules = schedules
	return nil
}

func (m *mockPlanRepoForConsumer) FindByUserIDAndDate(ctx context.Context, userID string, date time.Time) (*aggregate.NutritionPlan, error) {
	return nil, nil
}
func (m *mockPlanRepoForConsumer) Save(ctx context.Context, plan *aggregate.NutritionPlan) error {
	return nil
}
func (m *mockPlanRepoForConsumer) Update(ctx context.Context, plan *aggregate.NutritionPlan) error {
	return nil
}
func (m *mockPlanRepoForConsumer) FindActiveUserIDs(ctx context.Context, withinDays int) ([]string, error) {
	return nil, nil
}
func (m *mockPlanRepoForConsumer) FindPlansForDate(ctx context.Context, targetDate time.Time) ([]*aggregate.NutritionPlan, error) {
	return nil, nil
}

func TestProfileEventConsumer_UserRegistered(t *testing.T) {
	repo := &mockPlanRepoForConsumer{}
	consumer := NewProfileEventConsumer(nil, repo, nil)

	eventData := &authv1event.UserRegistered{
		UserId: "usr_reg_100",
		Email:  "test@example.com",
	}

	dataBytes, err := protojson.Marshal(eventData)
	if err != nil {
		t.Fatalf("marshal protojson: %v", err)
	}

	env := CloudEventEnvelope{
		SpecVersion: "1.0",
		ID:          "evt-100",
		Type:        "contracts.generic.auth.v1.userRegistered",
		Data:        dataBytes,
	}

	envBytes, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}

	msg := kafka.Message{
		Value: envBytes,
	}

	if err := consumer.ProcessMessage(context.Background(), msg); err != nil {
		t.Fatalf("unexpected error processing message: %v", err)
	}

	if repo.savedUserID != "usr_reg_100" {
		t.Errorf("savedUserID got %s, want usr_reg_100", repo.savedUserID)
	}
	if len(repo.savedSchedules) != 4 {
		t.Fatalf("savedSchedules count got %d, want 4", len(repo.savedSchedules))
	}
}

func TestProfileEventConsumer_ProfileUpdated_Above80Percent(t *testing.T) {
	repo := &mockPlanRepoForConsumer{}
	consumer := NewProfileEventConsumer(nil, repo, nil)

	eventData := &profilev1event.ProfileUpdated{
		UserId:         "usr_prof_85",
		WeightKg:       72.0,
		HeightCm:       175.0,
		Gender:         "MALE",
		CompletionRate: 85.0,
	}

	dataBytes, err := protojson.Marshal(eventData)
	if err != nil {
		t.Fatalf("marshal protojson: %v", err)
	}

	env := CloudEventEnvelope{
		SpecVersion: "1.0",
		ID:          "evt-85",
		Type:        "contracts.supporting.profile.v1.event.ProfileUpdated",
		Data:        dataBytes,
	}

	envBytes, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}

	msg := kafka.Message{
		Value: envBytes,
	}

	if err := consumer.ProcessMessage(context.Background(), msg); err != nil {
		t.Fatalf("unexpected error processing message: %v", err)
	}

	if repo.savedUserID != "usr_prof_85" {
		t.Errorf("savedUserID got %s, want usr_prof_85", repo.savedUserID)
	}
}

func TestProfileEventConsumer_ProfileUpdated_Below80Percent_Skipped(t *testing.T) {
	repo := &mockPlanRepoForConsumer{}
	consumer := NewProfileEventConsumer(nil, repo, nil)

	eventData := &profilev1event.ProfileUpdated{
		UserId:         "usr_prof_40",
		WeightKg:       72.0,
		HeightCm:       175.0,
		Gender:         "MALE",
		CompletionRate: 40.0,
	}

	dataBytes, err := protojson.Marshal(eventData)
	if err != nil {
		t.Fatalf("marshal protojson: %v", err)
	}

	env := CloudEventEnvelope{
		SpecVersion: "1.0",
		ID:          "evt-40",
		Type:        "contracts.supporting.profile.v1.event.ProfileUpdated",
		Data:        dataBytes,
	}

	envBytes, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}

	msg := kafka.Message{
		Value: envBytes,
	}

	if err := consumer.ProcessMessage(context.Background(), msg); err != nil {
		t.Fatalf("unexpected error processing message: %v", err)
	}

	if repo.savedUserID != "" {
		t.Errorf("expected no meal schedules saved for completion rate < 80%%, got user %s", repo.savedUserID)
	}
}
