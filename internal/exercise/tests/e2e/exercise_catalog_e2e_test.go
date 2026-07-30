//go:build e2e

package e2e

import (
	"context"
	"testing"

	"google.golang.org/grpc/metadata"

	exercisemsg "github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/exercise/v1/message"
)

func TestBodyPartCatalog_Lifecycle_E2E(t *testing.T) {
	suite := SetupE2ESuite(t)
	defer suite.StopServer()

	adminCtx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("x-user-id", "admin-1", "x-user-role", "Admin"))
	userCtx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("x-user-id", "user-1", "x-user-role", "User"))

	// 1. Create BodyPart (Admin)
	createResp, err := suite.Client.CreateBodyPart(adminCtx, &exercisemsg.CreateBodyPartRequest{
		Name: "Upper Body",
	})
	if err != nil {
		t.Fatalf("CreateBodyPart failed: %v", err)
	}

	bpID := createResp.GetBodyPart().GetId()
	if bpID == "" {
		t.Fatalf("CreateBodyPart returned empty ID")
	}

	// 2. Get BodyPart (User can read)
	getResp, err := suite.Client.GetBodyPart(userCtx, &exercisemsg.GetBodyPartRequest{
		Id: bpID,
	})
	if err != nil {
		t.Fatalf("GetBodyPart failed: %v", err)
	}

	if got, want := getResp.GetBodyPart().GetName(), "Upper Body"; got != want {
		t.Errorf("expected name %s, got %s", want, got)
	}

	// 3. List BodyParts (User can read)
	listResp, err := suite.Client.ListBodyParts(userCtx, &exercisemsg.ListBodyPartsRequest{
		Limit:  50,
		Offset: 0,
	})
	if err != nil {
		t.Fatalf("ListBodyParts failed: %v", err)
	}

	if len(listResp.GetBodyParts()) == 0 {
		t.Errorf("expected non-empty body parts list")
	}

	// 4. Update BodyPart (Admin)
	updateResp, err := suite.Client.UpdateBodyPart(adminCtx, &exercisemsg.UpdateBodyPartRequest{
		Id:   bpID,
		Name: "Upper Body Updated",
	})
	if err != nil {
		t.Fatalf("UpdateBodyPart failed: %v", err)
	}

	if got, want := updateResp.GetBodyPart().GetName(), "Upper Body Updated"; got != want {
		t.Errorf("expected name %s, got %s", want, got)
	}

	// 5. Delete BodyPart (Admin)
	delResp, err := suite.Client.DeleteBodyPart(adminCtx, &exercisemsg.DeleteBodyPartRequest{
		Id: bpID,
	})
	if err != nil {
		t.Fatalf("DeleteBodyPart failed: %v", err)
	}

	if !delResp.GetSuccess() {
		t.Errorf("expected delete success = true")
	}

	// 6. Get deleted BodyPart should fail
	_, err = suite.Client.GetBodyPart(userCtx, &exercisemsg.GetBodyPartRequest{
		Id: bpID,
	})
	if err == nil {
		t.Fatalf("expected error getting deleted body part")
	}
}

func TestEquipmentCatalog_CRUD_E2E(t *testing.T) {
	suite := SetupE2ESuite(t)
	defer suite.StopServer()

	adminCtx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("x-user-id", "admin-1", "x-user-role", "Admin"))
	userCtx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("x-user-id", "user-1", "x-user-role", "User"))

	// Create Equipment
	createResp, err := suite.Client.CreateEquipment(adminCtx, &exercisemsg.CreateEquipmentRequest{
		Name: "Barbell",
	})
	if err != nil {
		t.Fatalf("CreateEquipment failed: %v", err)
	}

	eqID := createResp.GetEquipment().GetId()

	// Get Equipment
	getResp, err := suite.Client.GetEquipment(userCtx, &exercisemsg.GetEquipmentRequest{
		Id: eqID,
	})
	if err != nil {
		t.Fatalf("GetEquipment failed: %v", err)
	}

	if got, want := getResp.GetEquipment().GetName(), "Barbell"; got != want {
		t.Errorf("expected name %s, got %s", want, got)
	}

	// List Equipments
	listResp, err := suite.Client.ListEquipments(userCtx, &exercisemsg.ListEquipmentsRequest{
		Limit:  50,
		Offset: 0,
	})
	if err != nil {
		t.Fatalf("ListEquipments failed: %v", err)
	}

	if len(listResp.GetEquipments()) == 0 {
		t.Errorf("expected non-empty equipments list")
	}

	// Update Equipment
	updateResp, err := suite.Client.UpdateEquipment(adminCtx, &exercisemsg.UpdateEquipmentRequest{
		Id:   eqID,
		Name: "Olympic Barbell",
	})
	if err != nil {
		t.Fatalf("UpdateEquipment failed: %v", err)
	}

	if got, want := updateResp.GetEquipment().GetName(), "Olympic Barbell"; got != want {
		t.Errorf("expected name %s, got %s", want, got)
	}

	// Delete Equipment
	delResp, err := suite.Client.DeleteEquipment(adminCtx, &exercisemsg.DeleteEquipmentRequest{
		Id: eqID,
	})
	if err != nil {
		t.Fatalf("DeleteEquipment failed: %v", err)
	}

	if !delResp.GetSuccess() {
		t.Errorf("expected delete success = true")
	}
}

func TestMuscleCatalog_WithBodyPart_E2E(t *testing.T) {
	suite := SetupE2ESuite(t)
	defer suite.StopServer()

	adminCtx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("x-user-id", "admin-1", "x-user-role", "Admin"))
	userCtx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("x-user-id", "user-1", "x-user-role", "User"))

	// Create BodyPart first
	bpResp, err := suite.Client.CreateBodyPart(adminCtx, &exercisemsg.CreateBodyPartRequest{
		Name: "Chest",
	})
	if err != nil {
		t.Fatalf("CreateBodyPart failed: %v", err)
	}

	bpID := bpResp.GetBodyPart().GetId()

	// Create Muscle with body_part_id
	createResp, err := suite.Client.CreateMuscle(adminCtx, &exercisemsg.CreateMuscleRequest{
		Name:       "Pectoralis Major",
		BodyPartId: bpID,
	})
	if err != nil {
		t.Fatalf("CreateMuscle failed: %v", err)
	}

	muscleID := createResp.GetMuscle().GetId()
	if muscleID == "" {
		t.Fatalf("CreateMuscle returned empty ID")
	}

	// Get Muscle
	getResp, err := suite.Client.GetMuscle(userCtx, &exercisemsg.GetMuscleRequest{
		Id: muscleID,
	})
	if err != nil {
		t.Fatalf("GetMuscle failed: %v", err)
	}

	if got, want := getResp.GetMuscle().GetBodyPartId(), bpID; got != want {
		t.Errorf("expected body_part_id %s, got %s", want, got)
	}

	// List Muscles by body_part_id
	listResp, err := suite.Client.ListMuscles(userCtx, &exercisemsg.ListMusclesRequest{
		BodyPartId: bpID,
		Limit:      50,
		Offset:     0,
	})
	if err != nil {
		t.Fatalf("ListMuscles failed: %v", err)
	}

	if len(listResp.GetMuscles()) == 0 {
		t.Errorf("expected non-empty muscles list for body part")
	}

	// Update Muscle
	updateResp, err := suite.Client.UpdateMuscle(adminCtx, &exercisemsg.UpdateMuscleRequest{
		Id:         muscleID,
		Name:       "Pectoralis Major Updated",
		BodyPartId: bpID,
	})
	if err != nil {
		t.Fatalf("UpdateMuscle failed: %v", err)
	}

	if got, want := updateResp.GetMuscle().GetName(), "Pectoralis Major Updated"; got != want {
		t.Errorf("expected name %s, got %s", want, got)
	}

	// Delete Muscle
	delResp, err := suite.Client.DeleteMuscle(adminCtx, &exercisemsg.DeleteMuscleRequest{
		Id: muscleID,
	})
	if err != nil {
		t.Fatalf("DeleteMuscle failed: %v", err)
	}

	if !delResp.GetSuccess() {
		t.Errorf("expected delete success = true")
	}
}

func TestTagCatalog_CRUD_E2E(t *testing.T) {
	suite := SetupE2ESuite(t)
	defer suite.StopServer()

	adminCtx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("x-user-id", "admin-1", "x-user-role", "Admin"))
	userCtx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("x-user-id", "user-1", "x-user-role", "User"))

	// Create Tag
	createResp, err := suite.Client.CreateTag(adminCtx, &exercisemsg.CreateTagRequest{
		Name: "Compound",
	})
	if err != nil {
		t.Fatalf("CreateTag failed: %v", err)
	}

	tagID := createResp.GetTag().GetId()

	// Get Tag
	getResp, err := suite.Client.GetTag(userCtx, &exercisemsg.GetTagRequest{
		Id: tagID,
	})
	if err != nil {
		t.Fatalf("GetTag failed: %v", err)
	}

	if got, want := getResp.GetTag().GetName(), "Compound"; got != want {
		t.Errorf("expected name %s, got %s", want, got)
	}

	// List Tags
	listResp, err := suite.Client.ListTags(userCtx, &exercisemsg.ListTagsRequest{
		Limit:  50,
		Offset: 0,
	})
	if err != nil {
		t.Fatalf("ListTags failed: %v", err)
	}

	if len(listResp.GetTags()) == 0 {
		t.Errorf("expected non-empty tags list")
	}

	// Update Tag
	updateResp, err := suite.Client.UpdateTag(adminCtx, &exercisemsg.UpdateTagRequest{
		Id:   tagID,
		Name: "Compound Movement",
	})
	if err != nil {
		t.Fatalf("UpdateTag failed: %v", err)
	}

	if got, want := updateResp.GetTag().GetName(), "Compound Movement"; got != want {
		t.Errorf("expected name %s, got %s", want, got)
	}

	// Delete Tag
	delResp, err := suite.Client.DeleteTag(adminCtx, &exercisemsg.DeleteTagRequest{
		Id: tagID,
	})
	if err != nil {
		t.Fatalf("DeleteTag failed: %v", err)
	}

	if !delResp.GetSuccess() {
		t.Errorf("expected delete success = true")
	}
}

func TestCatalog_UnauthorizedWrite_E2E(t *testing.T) {
	suite := SetupE2ESuite(t)
	defer suite.StopServer()

	userCtx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("x-user-id", "user-1", "x-user-role", "User"))

	// User cannot create BodyPart
	_, err := suite.Client.CreateBodyPart(userCtx, &exercisemsg.CreateBodyPartRequest{
		Name: "Legs",
	})
	if err == nil {
		t.Fatalf("expected error for non-admin user creating body part")
	}

	// User cannot update BodyPart
	_, err = suite.Client.UpdateBodyPart(userCtx, &exercisemsg.UpdateBodyPartRequest{
		Id:   "some-id",
		Name: "Updated",
	})
	if err == nil {
		t.Fatalf("expected error for non-admin user updating body part")
	}

	// User cannot delete BodyPart
	_, err = suite.Client.DeleteBodyPart(userCtx, &exercisemsg.DeleteBodyPartRequest{
		Id: "some-id",
	})
	if err == nil {
		t.Fatalf("expected error for non-admin user deleting body part")
	}
}
