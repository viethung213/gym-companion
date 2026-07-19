package transport

import (
	"context"

	"github.com/viethung213/gym-companion/internal/coaching/application/command"
	coachingsvc "github.com/viethung213/gym-companion/internal/gen/go/contracts/core/coaching/v1/service"
	coachingmsg "github.com/viethung213/gym-companion/internal/gen/go/contracts/core/coaching/v1/message"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type CoachingServer struct {
	coachingsvc.UnimplementedCoachingServiceServer
	initiateRoadmap *command.InitiateRoadmapHandler
}

var _ coachingsvc.CoachingServiceServer = (*CoachingServer)(nil)

func NewCoachingServer(initiateRoadmap *command.InitiateRoadmapHandler) *CoachingServer {
	return &CoachingServer{
		initiateRoadmap: initiateRoadmap,
	}
}

func (s *CoachingServer) InitiateRoadmap(
	ctx context.Context,
	req *coachingmsg.InitiateRoadmapRequest,
) (*coachingmsg.InitiateRoadmapResponse, error) {
	if req == nil || req.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	cmd := &command.InitiateRoadmapCommand{
		UserID:          req.UserId,
		ExperienceLevel: "beginner", // ponytail: default fallback for manual API
	}

	if err := s.initiateRoadmap.Handle(ctx, cmd); err != nil {
		return nil, status.Errorf(codes.Internal, "initiate roadmap: %v", err)
	}

	return &coachingmsg.InitiateRoadmapResponse{}, nil
}
