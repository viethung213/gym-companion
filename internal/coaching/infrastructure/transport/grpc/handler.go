package grpc

import (
	"context"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	datepb "google.golang.org/genproto/googleapis/type/date"
	timestamppb "google.golang.org/protobuf/types/known/timestamppb"

	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	"github.com/viethung213/gym-companion/internal/coaching/domain"
	coachingv1event "github.com/viethung213/gym-companion/internal/gen/go/contracts/core/coaching/v1/event"
	coachingv1message "github.com/viethung213/gym-companion/internal/gen/go/contracts/core/coaching/v1/message"
	coachingv1service "github.com/viethung213/gym-companion/internal/gen/go/contracts/core/coaching/v1/service"
)

type CoachingGRPCHandler struct {
	coachingv1service.UnimplementedCoachingServiceServer
	initiateRoadmapUC  port.InitiateRoadmapUseCase
	generateDailyPlanUC port.GenerateDailyPlanUseCase
	processPostWorkoutUC port.ProcessPostWorkoutUseCase
	roadmapRepo        domain.WorkoutRoadmapRepository
	scheduleRepo       domain.WeeklyScheduleRepository
	dailyPlanRepo      domain.DailyWorkoutPlanRepository
}

func NewCoachingGRPCHandler(
	initiateRoadmapUC port.InitiateRoadmapUseCase,
	generateDailyPlanUC port.GenerateDailyPlanUseCase,
	processPostWorkoutUC port.ProcessPostWorkoutUseCase,
	roadmapRepo domain.WorkoutRoadmapRepository,
	scheduleRepo domain.WeeklyScheduleRepository,
	dailyPlanRepo domain.DailyWorkoutPlanRepository,
) *CoachingGRPCHandler {
	return &CoachingGRPCHandler{
		initiateRoadmapUC:   initiateRoadmapUC,
		generateDailyPlanUC:  generateDailyPlanUC,
		processPostWorkoutUC: processPostWorkoutUC,
		roadmapRepo:         roadmapRepo,
		scheduleRepo:        scheduleRepo,
		dailyPlanRepo:       dailyPlanRepo,
	}
}

func (h *CoachingGRPCHandler) InitiateRoadmap(ctx context.Context, req *coachingv1message.InitiateRoadmapRequest) (*coachingv1message.InitiateRoadmapResponse, error) {
	if req == nil || req.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	payloadID := ""
	if req.Payload != nil {
		payloadID = req.Payload.ProfileSnapshotId
	}

	result, err := h.initiateRoadmapUC.Execute(ctx, port.InitiateRoadmapCommand{
		UserID:             req.UserId,
		ProfileSnapshotID:  payloadID,
		Goal:               "Hypertrophy",
		ExperienceLevel:    "Intermediate",
		AvailableSlots:     []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat"},
		AvailableEquipment: []string{"Barbell", "Dumbbell"},
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to initiate roadmap: %v", err)
	}

	return &coachingv1message.InitiateRoadmapResponse{
		Roadmap:              mapDomainRoadmapToProto(result.Roadmap),
		FirstWeeklySchedule: mapDomainScheduleToProto(result.WeeklySchedule),
	}, nil
}

func (h *CoachingGRPCHandler) GetActiveRoadmap(ctx context.Context, req *coachingv1message.GetActiveRoadmapRequest) (*coachingv1message.GetActiveRoadmapResponse, error) {
	if req == nil || req.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	roadmap, err := h.roadmapRepo.FindActiveByUserID(ctx, req.UserId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to fetch active roadmap: %v", err)
	}
	if roadmap == nil {
		return nil, status.Error(codes.NotFound, "active roadmap not found")
	}

	schedule, _ := h.scheduleRepo.FindCurrentByRoadmapID(ctx, roadmap.ID(), 1)

	return &coachingv1message.GetActiveRoadmapResponse{
		Roadmap:               mapDomainRoadmapToProto(roadmap),
		CurrentWeeklySchedule: mapDomainScheduleToProto(schedule),
	}, nil
}

func (h *CoachingGRPCHandler) GetDailyWorkoutPlan(ctx context.Context, req *coachingv1message.GetDailyWorkoutPlanRequest) (*coachingv1message.GetDailyWorkoutPlanResponse, error) {
	if req == nil || req.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	scheduledDate := time.Now().UTC()
	if req.ScheduledDate != nil {
		scheduledDate = time.Date(int(req.ScheduledDate.Year), time.Month(req.ScheduledDate.Month), int(req.ScheduledDate.Day), 0, 0, 0, 0, time.UTC)
	}

	plan, err := h.dailyPlanRepo.FindByUserAndDate(ctx, req.UserId, scheduledDate)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to query daily plan: %v", err)
	}

	if plan == nil {
		// JIT Generate Daily Workout Plan
		generatedPlan, err := h.generateDailyPlanUC.Execute(ctx, port.GenerateDailyPlanCommand{
			UserID:        req.UserId,
			ScheduledDate: scheduledDate,
		})
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to generate daily plan: %v", err)
		}
		plan = generatedPlan
	}

	return &coachingv1message.GetDailyWorkoutPlanResponse{
		DailyWorkoutPlan: mapDomainDailyPlanToProto(plan),
	}, nil
}

func mapDomainRoadmapToProto(r *domain.WorkoutRoadmap) *coachingv1message.WorkoutRoadmap {
	if r == nil {
		return nil
	}
	return &coachingv1message.WorkoutRoadmap{
		RoadmapId: r.ID(),
		UserId:    r.UserID(),
		Status:    coachingv1message.RoadmapStatus(r.Status()),
		StartDate: timeToProtoDate(r.StartDate()),
		EndDate:   timeToProtoDate(r.EndDate()),
	}
}

func mapDomainScheduleToProto(s *domain.WeeklySchedule) *coachingv1message.WeeklySchedule {
	if s == nil {
		return nil
	}
	days := make([]*coachingv1message.ScheduleDay, len(s.ScheduleDays()))
	for i, d := range s.ScheduleDays() {
		days[i] = &coachingv1message.ScheduleDay{
			ScheduledDate:      timeToProtoDate(d.ScheduledDate()),
			DayOfWeek:          d.DayOfWeek(),
			Status:             coachingv1message.WorkoutDayStatus(d.Status()),
			TargetMuscleGroups: d.TargetMuscleGroups(),
			DailyWorkoutPlanId: d.DailyPlanID(),
		}
	}
	return &coachingv1message.WeeklySchedule{
		WeeklyScheduleId: s.ID(),
		RoadmapId:        s.RoadmapID(),
		UserId:           s.UserID(),
		WeekNumber:       s.WeekNumber(),
		StartDate:        timeToProtoDate(s.StartDate()),
		EndDate:          timeToProtoDate(s.EndDate()),
		MuscleSplitType:  s.MuscleSplitType(),
		ScheduleDays:     days,
	}
}

func mapDomainDailyPlanToProto(d *domain.DailyWorkoutPlan) *coachingv1message.DailyWorkoutPlan {
	if d == nil {
		return nil
	}
	mapEx := func(exs []domain.PrescribedExercise) []*coachingv1message.PrescribedExercise {
		res := make([]*coachingv1message.PrescribedExercise, len(exs))
		for i, ex := range exs {
			res[i] = &coachingv1message.PrescribedExercise{
				ExerciseId:      ex.ExerciseID(),
				ExerciseName:    ex.ExerciseName(),
				TargetSets:      ex.TargetSets(),
				TargetReps:      ex.TargetReps(),
				TargetWeight:    ex.TargetWeight(),
				DurationSeconds: ex.DurationSeconds(),
				Notes:           ex.Notes(),
				RestSetSec:      ex.RestSetSec(),
				RestExerciseSec: ex.RestExerciseSec(),
				TargetRpe:       ex.TargetRPE(),
			}
		}
		return res
	}

	prescription := &coachingv1message.WorkoutPrescription{
		WarmUps:       mapEx(d.Prescription().WarmUps()),
		MainExercises: mapEx(d.Prescription().MainExercises()),
		CoolDowns:     mapEx(d.Prescription().CoolDowns()),
	}

	return &coachingv1message.DailyWorkoutPlan{
		DailyWorkoutPlanId:    d.ID(),
		UserId:                d.UserID(),
		RoadmapId:             d.RoadmapID(),
		WeeklyScheduleId:      d.WeeklyScheduleID(),
		ScheduledDate:         timeToProtoDate(d.ScheduledDate()),
		Status:                coachingv1message.DailyPlanStatus(d.Status()),
		WorkoutPrescription:   prescription,
		ReasoningExplanation:  d.ReasoningExplanation(),
		AdjustmentExplanation: d.AdjustmentExplanation(),
		GeneratedAt:           timestamppb.New(d.GeneratedAt()),
	}
}

func timeToProtoDate(t time.Time) *datepb.Date {
	if t.IsZero() {
		return nil
	}
	return &datepb.Date{
		Year:  int32(t.Year()),
		Month: int32(t.Month()),
		Day:   int32(t.Day()),
	}
}

// Ensure unused import compiles cleanly
var _ = coachingv1event.RoadmapInitiated{}
