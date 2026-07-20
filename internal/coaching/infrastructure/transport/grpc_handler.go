package transport

import (
	"context"
	"errors"
	"log"

	"github.com/viethung213/gym-companion/internal/coaching/application/command"
	coachingmsg "github.com/viethung213/gym-companion/internal/gen/go/contracts/core/coaching/v1/message"
	coachingsvc "github.com/viethung213/gym-companion/internal/gen/go/contracts/core/coaching/v1/service"
	"github.com/viethung213/gym-companion/internal/shared/middleware"
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

	actor, err := middleware.RequireAuthenticated(ctx)
	if errors.Is(err, middleware.ErrUnauthorized) {
		return nil, status.Error(codes.Unauthenticated, "authentication required")
	}
	if err != nil {
		return nil, status.Error(codes.Internal, "resolve authenticated user")
	}
	if actor.UserID != req.UserId {
		return nil, status.Error(codes.PermissionDenied, "user_id does not match caller")
	}

	cmd := &command.InitiateRoadmapCommand{
		UserID:          req.UserId,
		ExperienceLevel: "beginner", // ponytail: default fallback for manual API
	}

	if err := s.initiateRoadmap.Handle(ctx, cmd); err != nil {
		log.Printf("ERROR: initiate roadmap for user %s: %v", actor.UserID, err)
		return nil, status.Error(codes.Internal, "initiate roadmap")
	}

	return &coachingmsg.InitiateRoadmapResponse{}, nil
}
