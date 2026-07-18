package transport

import (
	"context"

	"github.com/viethung213/gym-companion/internal/coaching/application/command"
	"github.com/viethung213/gym-companion/internal/coaching/application/query"
	"github.com/viethung213/gym-companion/internal/coaching/domain"
	coachingmsg "github.com/viethung213/gym-companion/internal/gen/go/contracts/core/coaching/v1/message"
	coachingsvc "github.com/viethung213/gym-companion/internal/gen/go/contracts/core/coaching/v1/service"
)

const plannerVersion = "deterministic-rules-v1"

type CoachingServer struct {
	coachingsvc.UnimplementedCoachingServiceServer

	initiateRoadmap   *command.InitiateRoadmapHandler
	generateSchedule  *command.GenerateWeeklyScheduleHandler
	generateDailyPlan *command.GenerateDailyPlanHandler
	queries           *query.Handler
}

var _ coachingsvc.CoachingServiceServer = (*CoachingServer)(nil)

func NewCoachingServer(
	initiateRoadmap *command.InitiateRoadmapHandler,
	generateSchedule *command.GenerateWeeklyScheduleHandler,
	generateDailyPlan *command.GenerateDailyPlanHandler,
	queries *query.Handler,
) *CoachingServer {
	return &CoachingServer{
		initiateRoadmap:   initiateRoadmap,
		generateSchedule:  generateSchedule,
		generateDailyPlan: generateDailyPlan,
		queries:           queries,
	}
}

func (s *CoachingServer) InitiateRoadmap(
	ctx context.Context,
	req *coachingmsg.InitiateRoadmapRequest,
) (*coachingmsg.InitiateRoadmapResponse, error) {
	if err := authorizeRequest(ctx, req.GetUserId()); err != nil {
		return nil, rpcError(err)
	}
	input, err := planningInput(req.GetPayload())
	if err != nil {
		return nil, rpcError(err)
	}
	result, err := s.initiateRoadmap.Handle(ctx, &command.InitiateRoadmap{
		UserID:         req.GetUserId(),
		PlanningInput:  input,
		PlannerVersion: plannerVersion,
	})
	if err != nil {
		return nil, rpcError(err)
	}
	return &coachingmsg.InitiateRoadmapResponse{
		Roadmap:             toProtoRoadmap(result.Roadmap),
		FirstWeeklySchedule: toProtoSchedule(result.Schedule),
	}, nil
}

func (s *CoachingServer) ListWorkoutRoadmaps(
	ctx context.Context,
	req *coachingmsg.ListWorkoutRoadmapsRequest,
) (*coachingmsg.ListWorkoutRoadmapsResponse, error) {
	if err := authorizeRequest(ctx, req.GetUserId()); err != nil {
		return nil, rpcError(err)
	}
	roadmaps, err := s.queries.ListRoadmaps(ctx, req.GetUserId())
	if err != nil {
		return nil, rpcError(err)
	}
	filtered := make([]*domain.WorkoutRoadmap, 0, len(roadmaps))
	for _, roadmap := range roadmaps {
		if req.GetStatus() == coachingmsg.RoadmapStatus_ROADMAP_STATUS_UNSPECIFIED ||
			toProtoRoadmapStatus(roadmap.Status) == req.GetStatus() {
			filtered = append(filtered, roadmap)
		}
	}
	page, nextToken, err := pageItems(filtered, req.GetPageSize(), req.GetPageToken())
	if err != nil {
		return nil, rpcError(err)
	}
	response := &coachingmsg.ListWorkoutRoadmapsResponse{NextPageToken: nextToken}
	for _, roadmap := range page {
		response.Roadmaps = append(response.Roadmaps, toProtoRoadmap(roadmap))
	}
	return response, nil
}

func (s *CoachingServer) GetWorkoutRoadmap(
	ctx context.Context,
	req *coachingmsg.GetWorkoutRoadmapRequest,
) (*coachingmsg.GetWorkoutRoadmapResponse, error) {
	if err := authorizeRequest(ctx, req.GetUserId()); err != nil {
		return nil, rpcError(err)
	}
	roadmap, err := s.queries.GetRoadmap(ctx, req.GetUserId(), req.GetRoadmapId())
	if err != nil {
		return nil, rpcError(err)
	}
	return &coachingmsg.GetWorkoutRoadmapResponse{Roadmap: toProtoRoadmap(roadmap)}, nil
}

func (s *CoachingServer) GenerateNextWeeklySchedule(
	ctx context.Context,
	req *coachingmsg.GenerateNextWeeklyScheduleRequest,
) (*coachingmsg.GenerateNextWeeklyScheduleResponse, error) {
	if err := authorizeRequest(ctx, req.GetUserId()); err != nil {
		return nil, rpcError(err)
	}
	if req.GetPayload() == nil {
		return nil, rpcError(invalidRequest("payload is required"))
	}
	schedule, err := s.generateSchedule.Handle(ctx, command.GenerateWeeklySchedule{
		UserID:                   req.GetUserId(),
		RoadmapID:                req.GetRoadmapId(),
		PreviousWeeklyScheduleID: req.GetPayload().GetPreviousWeeklyScheduleId(),
	})
	if err != nil {
		return nil, rpcError(err)
	}
	return &coachingmsg.GenerateNextWeeklyScheduleResponse{
		WeeklySchedule: toProtoSchedule(schedule),
	}, nil
}

func (s *CoachingServer) ListWeeklySchedules(
	ctx context.Context,
	req *coachingmsg.ListWeeklySchedulesRequest,
) (*coachingmsg.ListWeeklySchedulesResponse, error) {
	if err := authorizeRequest(ctx, req.GetUserId()); err != nil {
		return nil, rpcError(err)
	}
	schedules, err := s.queries.ListSchedules(ctx, req.GetUserId(), req.GetRoadmapId())
	if err != nil {
		return nil, rpcError(err)
	}
	startDate, err := optionalDate(req.GetStartDate())
	if err != nil {
		return nil, rpcError(err)
	}
	endDate, err := optionalDate(req.GetEndDate())
	if err != nil {
		return nil, rpcError(err)
	}
	filtered := make([]*domain.WeeklySchedule, 0, len(schedules))
	for _, schedule := range schedules {
		if !startDate.IsZero() && schedule.EndDate.Before(startDate) {
			continue
		}
		if !endDate.IsZero() && schedule.StartDate.After(endDate) {
			continue
		}
		filtered = append(filtered, schedule)
	}
	page, nextToken, err := pageItems(filtered, req.GetPageSize(), req.GetPageToken())
	if err != nil {
		return nil, rpcError(err)
	}
	response := &coachingmsg.ListWeeklySchedulesResponse{NextPageToken: nextToken}
	for _, schedule := range page {
		response.WeeklySchedules = append(response.WeeklySchedules, toProtoSchedule(schedule))
	}
	return response, nil
}

func (s *CoachingServer) GetWeeklySchedule(
	ctx context.Context,
	req *coachingmsg.GetWeeklyScheduleRequest,
) (*coachingmsg.GetWeeklyScheduleResponse, error) {
	if err := authorizeRequest(ctx, req.GetUserId()); err != nil {
		return nil, rpcError(err)
	}
	schedule, err := s.queries.GetSchedule(ctx, req.GetUserId(), req.GetWeeklyScheduleId())
	if err != nil {
		return nil, rpcError(err)
	}
	return &coachingmsg.GetWeeklyScheduleResponse{WeeklySchedule: toProtoSchedule(schedule)}, nil
}

func (s *CoachingServer) GenerateDailyWorkoutPlan(
	ctx context.Context,
	req *coachingmsg.GenerateDailyWorkoutPlanRequest,
) (*coachingmsg.GenerateDailyWorkoutPlanResponse, error) {
	if err := authorizeRequest(ctx, req.GetUserId()); err != nil {
		return nil, rpcError(err)
	}
	if req.GetPayload() == nil {
		return nil, rpcError(invalidRequest("payload is required"))
	}
	scheduledDate, err := requiredDate(req.GetPayload().GetScheduledDate())
	if err != nil {
		return nil, rpcError(err)
	}
	plan, err := s.generateDailyPlan.Handle(ctx, &command.GenerateDailyPlan{
		UserID:           req.GetUserId(),
		WeeklyScheduleID: req.GetWeeklyScheduleId(),
		ScheduledDate:    scheduledDate,
		NewInjuryAreas:   req.GetPayload().GetCheckIn().GetNewInjuryAreas(),
	})
	if err != nil {
		return nil, rpcError(err)
	}
	return &coachingmsg.GenerateDailyWorkoutPlanResponse{
		DailyWorkoutPlan: toProtoDailyPlan(plan),
	}, nil
}

func (s *CoachingServer) GetDailyWorkoutPlan(
	ctx context.Context,
	req *coachingmsg.GetDailyWorkoutPlanRequest,
) (*coachingmsg.GetDailyWorkoutPlanResponse, error) {
	if err := authorizeRequest(ctx, req.GetUserId()); err != nil {
		return nil, rpcError(err)
	}
	plan, err := s.queries.GetDailyPlan(ctx, req.GetUserId(), req.GetDailyWorkoutPlanId())
	if err != nil {
		return nil, rpcError(err)
	}
	return &coachingmsg.GetDailyWorkoutPlanResponse{DailyWorkoutPlan: toProtoDailyPlan(plan)}, nil
}
