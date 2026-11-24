package event_test

import (
	"encoding/json"
	"testing"
	"time"

	exercisemsg "github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/exercise/v1/message"
	profilev1event "github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/profile/v1/event"
	"github.com/viethung213/gym-companion/internal/profile/domain/entity"
	domainEvent "github.com/viethung213/gym-companion/internal/profile/domain/event"
	"github.com/viethung213/gym-companion/internal/profile/domain/vo"
	infraEvent "github.com/viethung213/gym-companion/internal/profile/infrastructure/event"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

func unpackData(t *testing.T, ev any, msg proto.Message) {
	t.Helper()

	rec, err := infraEvent.PackCloudEvent(ev)
	if err != nil {
		t.Fatalf("PackCloudEvent: %v", err)
	}
	var envelope infraEvent.CloudEventEnvelope
	if err := json.Unmarshal(rec.Payload, &envelope); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	if err := protojson.Unmarshal(envelope.Data, msg); err != nil {
		t.Fatalf("unmarshal data: %v", err)
	}
}

func TestPackCloudEvent_InjuryPayloads(t *testing.T) {
	inj, err := entity.NewInjury("inj-1", vo.JointRegionKnee, vo.MuscleGroupQuadriceps, vo.BodySideLeft,
		vo.InjurySeveritySevere, "pain", time.Now())
	if err != nil {
		t.Fatalf("new injury: %v", err)
	}
	wantRegion := exercisemsg.JointRegion_JOINT_REGION_KNEE
	wantMuscle := exercisemsg.MuscleGroup_MUSCLE_GROUP_QUADRICEPS

	var reported profilev1event.InjuryReported
	unpackData(t, domainEvent.NewInjuryReportedEvent("user-1", inj), &reported)
	got := reported.GetInjury()
	if got.GetRegion() != wantRegion || got.GetMuscle() != wantMuscle ||
		got.GetSide() != exercisemsg.BodySide_BODY_SIDE_LEFT ||
		got.GetSeverity() != exercisemsg.InjurySeverity_INJURY_SEVERITY_SEVERE {
		t.Errorf("InjuryReported injury = %v", got)
	}

	var recovered profilev1event.InjuryRecoveredData
	unpackData(t, domainEvent.NewInjuryRecoveredEvent("user-1", inj, time.Now()), &recovered)
	if recovered.GetRegion() != wantRegion || recovered.GetMuscle() != wantMuscle {
		t.Errorf("InjuryRecovered = %v", &recovered)
	}

	bio, err := vo.NewBiologicalMetrics(70, 170, 30, "MALE")
	if err != nil {
		t.Fatalf("bio: %v", err)
	}
	var completed profilev1event.ProfileCompleted
	unpackData(t, domainEvent.NewProfileCompletedEvent("user-1", bio, nil, []*entity.Injury{inj}, nil, time.Now()), &completed)
	if len(completed.GetInjuries()) != 1 || completed.GetInjuries()[0].GetRegion() != wantRegion {
		t.Errorf("ProfileCompleted injuries = %v", completed.GetInjuries())
	}
}
