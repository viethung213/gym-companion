//go:build e2e || integration

package e2e

import (
	"context"
	"testing"

	"github.com/google/uuid"
	pbmsg "github.com/viethung213/gym-companion/internal/gen/go/contracts/core/coaching/v1/message"
)

func TestCoaching_FullLifecycle_E2E(t *testing.T) {
	suite := SetupCoachingE2ESuite(t)
	defer suite.StopServer()

	userID := uuid.NewString()
	ctx := context.Background()

	// 1. Initiate Roadmap
	initResp, err := suite.Client.InitiateRoadmap(ctx, &pbmsg.InitiateRoadmapRequest{
		UserId: userID,
	})
	if err != nil {
		t.Fatalf("InitiateRoadmap API failed: %v", err)
	}

	rm := initResp.GetRoadmap()
	if rm == nil {
		t.Fatalf("InitiateRoadmap returned nil roadmap")
	}

	roadmapID := rm.GetRoadmapId()
	if roadmapID == "" {
		t.Errorf("Expected valid roadmap_id, got empty")
	}

	if rm.GetUserId() != userID {
		t.Errorf("Expected user_id %s, got %s", userID, rm.GetUserId())
	}

	// 2. Fetch Active Roadmap
	activeResp, err := suite.Client.GetActiveRoadmap(ctx, &pbmsg.GetActiveRoadmapRequest{
		UserId: userID,
	})
	if err != nil {
		t.Fatalf("GetActiveRoadmap API failed: %v", err)
	}

	if activeResp.GetRoadmap().GetRoadmapId() != roadmapID {
		t.Errorf("GetActiveRoadmap returned roadmap_id %s, want %s", activeResp.GetRoadmap().GetRoadmapId(), roadmapID)
	}

	// 3. Fetch Roadmap by ID
	getResp, err := suite.Client.GetRoadmap(ctx, &pbmsg.GetRoadmapRequest{
		UserId:    userID,
		RoadmapId: roadmapID,
	})
	if err != nil {
		t.Fatalf("GetRoadmap API failed: %v", err)
	}

	if getResp.GetRoadmap().GetRoadmapId() != roadmapID {
		t.Errorf("GetRoadmap returned roadmap_id %s, want %s", getResp.GetRoadmap().GetRoadmapId(), roadmapID)
	}

	// 4. List Roadmaps for User
	listResp, err := suite.Client.ListRoadmaps(ctx, &pbmsg.ListRoadmapsRequest{
		UserId: userID,
	})
	if err != nil {
		t.Fatalf("ListRoadmaps API failed: %v", err)
	}

	if len(listResp.GetRoadmaps()) != 1 {
		t.Errorf("Expected 1 roadmap in list, got %d", len(listResp.GetRoadmaps()))
	}
}
