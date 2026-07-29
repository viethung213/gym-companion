// Package grpc implements the CoachingService gRPC endpoint.
package grpc

import (
	"context"
	"errors"

	"github.com/viethung213/gym-companion/internal/coaching/application/command"
	"github.com/viethung213/gym-companion/internal/coaching/application/query"
	"github.com/viethung213/gym-companion/internal/coaching/domain/roadmap"
	pbmsg "github.com/viethung213/gym-companion/internal/gen/go/contracts/core/coaching/v1/message"
	pbsvc "github.com/viethung213/gym-companion/internal/gen/go/contracts/core/coaching/v1/service"
	"google.golang.org/genproto/googleapis/type/date"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Server implements pbsvc.CoachingServiceServer.
type Server struct {
	pbsvc.UnimplementedCoachingServiceServer
	initiate   *command.InitiateRoadmapHandler
	regenerate *command.RegenerateScheduleHandler
	queries    *query.Handlers
}

// NewServer wires the gRPC handler.
func NewServer(
	initiate *command.InitiateRoadmapHandler,
	regenerate *command.RegenerateScheduleHandler,
	queries *query.Handlers,
) *Server {
	return &Server{initiate: initiate, regenerate: regenerate, queries: queries}
}

// InitiateRoadmap implements UC-02.1.
func (s *Server) InitiateRoadmap(ctx context.Context, req *pbmsg.InitiateRoadmapRequest) (*pbmsg.InitiateRoadmapResponse, error) {
	if req.GetUserId() == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	res, err := s.initiate.Handle(ctx, command.InitiateRoadmapCommand{UserID: req.GetUserId()})

	if err != nil {
		return nil, mapDomainErr(err)
	}

	return &pbmsg.InitiateRoadmapResponse{Roadmap: toPBRoadmap(res.Roadmap)}, nil
}

// GetActiveRoadmap returns the active roadmap for a user.
func (s *Server) GetActiveRoadmap(ctx context.Context, req *pbmsg.GetActiveRoadmapRequest) (*pbmsg.GetActiveRoadmapResponse, error) {
	if req.GetUserId() == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	rm, err := s.queries.GetActiveRoadmap(ctx, query.GetActiveRoadmapQuery{UserID: req.GetUserId()})

	if err != nil {
		return nil, mapDomainErr(err)
	}

	return &pbmsg.GetActiveRoadmapResponse{Roadmap: toPBRoadmap(rm)}, nil
}

// GetRoadmap returns a specific roadmap.
func (s *Server) GetRoadmap(ctx context.Context, req *pbmsg.GetRoadmapRequest) (*pbmsg.GetRoadmapResponse, error) {
	if req.GetUserId() == "" || req.GetRoadmapId() == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id and roadmap_id required")
	}

	rm, err := s.queries.GetRoadmap(ctx, query.GetRoadmapQuery{
		UserID: req.GetUserId(), RoadmapID: req.GetRoadmapId(),
	})

	if err != nil {
		return nil, mapDomainErr(err)
	}

	return &pbmsg.GetRoadmapResponse{Roadmap: toPBRoadmap(rm)}, nil
}

// ListRoadmaps returns paginated roadmaps.
func (s *Server) ListRoadmaps(ctx context.Context, req *pbmsg.ListRoadmapsRequest) (*pbmsg.ListRoadmapsResponse, error) {
	if req.GetUserId() == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id required")
	}

	rms, err := s.queries.ListRoadmaps(ctx, query.ListRoadmapsQuery{
		UserID: req.GetUserId(),

		Status: pbStatusToDomain(req.GetStatus()),

		Limit: int(req.GetPageSize()),
	})

	if err != nil {
		return nil, mapDomainErr(err)
	}

	resp := &pbmsg.ListRoadmapsResponse{Roadmaps: make([]*pbmsg.Roadmap, 0, len(rms))}

	for _, rm := range rms {
		resp.Roadmaps = append(resp.Roadmaps, toPBRoadmap(rm))
	}

	return resp, nil
}

// GetSessionPlan returns a specific session plan.
func (s *Server) GetSessionPlan(ctx context.Context, req *pbmsg.GetSessionPlanRequest) (*pbmsg.GetSessionPlanResponse, error) {
	if req.GetUserId() == "" || req.GetSessionPlanId() == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id and session_plan_id required")
	}

	sp, err := s.queries.GetSessionPlan(ctx, query.GetSessionPlanQuery{
		UserID: req.GetUserId(), SessionPlanID: req.GetSessionPlanId(),
	})

	if err != nil {
		return nil, mapDomainErr(err)
	}

	return &pbmsg.GetSessionPlanResponse{SessionPlan: toPBSession(sp)}, nil
}

// RegenerateSchedule triggers UC-02.3.
func (s *Server) RegenerateSchedule(ctx context.Context, req *pbmsg.RegenerateScheduleRequest) (*pbmsg.RegenerateScheduleResponse, error) {
	if req.GetUserId() == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id required")
	}

	res, err := s.regenerate.Handle(ctx, command.RegenerateScheduleCommand{
		UserID: req.GetUserId(),

		Reason: req.GetReason(),
	})

	if err != nil {
		return nil, mapDomainErr(err)
	}

	return &pbmsg.RegenerateScheduleResponse{Roadmap: toPBRoadmap(res.Roadmap)}, nil
}

// -------- error mapping --------
func mapDomainErr(err error) error {
	if err == nil {
		return nil
	}

	switch {
	case errors.Is(err, roadmap.ErrRoadmapNotFound),

		errors.Is(err, roadmap.ErrSessionNotFound):
		return status.Error(codes.NotFound, err.Error())

	case errors.Is(err, roadmap.ErrActiveRoadmapExists):
		return status.Error(codes.AlreadyExists, err.Error())

	case errors.Is(err, roadmap.ErrInvalidRoadmap),

		errors.Is(err, roadmap.ErrInvalidStatus),

		errors.Is(err, roadmap.ErrInvalidPhase),

		errors.Is(err, roadmap.ErrInvalidTransition),

		errors.Is(err, roadmap.ErrWeeklyCapExceeded),

		errors.Is(err, roadmap.ErrSessionAlreadyFinal):
		return status.Error(codes.FailedPrecondition, err.Error())

	default:
		return status.Error(codes.Internal, err.Error())
	}
}

// -------- domain -> protobuf mapping --------
func toPBRoadmap(rm *roadmap.Roadmap) *pbmsg.Roadmap {
	if rm == nil {
		return nil
	}

	info := rm.Info()

	pb := &pbmsg.Roadmap{
		RoadmapId: info.RoadmapID,

		UserId: info.UserID,

		Status: domainStatusToPB(info.Status),

		StartDate: dateToPB(info.StartDate.Year(), int(info.StartDate.Month()), info.StartDate.Day()),

		EndDate: dateToPB(info.EndDate.Year(), int(info.EndDate.Month()), info.EndDate.Day()),
	}

	for _, w := range rm.Weeks() {
		pb.WeekPlans = append(pb.WeekPlans, toPBWeek(w))
	}

	return pb
}

func toPBWeek(w *roadmap.WeekPlan) *pbmsg.WeekPlan {
	if w == nil {
		return nil
	}

	info := w.Info()

	pb := &pbmsg.WeekPlan{
		WeekPlanId: info.WeekPlanID,

		RoadmapId: info.RoadmapID,

		UserId: info.UserID,

		WeekNumber: info.WeekNumber,

		Phase: domainPhaseToPB(info.Phase),

		TargetRpe: info.TargetRPE,

		StartDate: dateToPB(info.StartDate.Year(), int(info.StartDate.Month()), info.StartDate.Day()),

		EndDate: dateToPB(info.EndDate.Year(), int(info.EndDate.Month()), info.EndDate.Day()),

		MuscleSplitType: info.MuscleSplitType,
	}

	for _, d := range w.Days() {
		pb.DayPlans = append(pb.DayPlans, toPBDay(d))
	}

	return pb
}

func toPBDay(d *roadmap.DayPlan) *pbmsg.DayPlan {
	if d == nil {
		return nil
	}

	info := d.Info()

	pb := &pbmsg.DayPlan{
		DayPlanId:     info.DayPlanID,
		WeekPlanId:    info.WeekPlanID,
		RoadmapId:     info.RoadmapID,
		UserId:        info.UserID,
		ScheduledDate: dateToPB(info.ScheduledDate.Year(), int(info.ScheduledDate.Month()), info.ScheduledDate.Day()),
	}

	for _, s := range d.Sessions() {
		pb.SessionPlans = append(pb.SessionPlans, toPBSession(s))
	}

	return pb
}

func toPBSession(s *roadmap.SessionPlan) *pbmsg.SessionPlan {
	if s == nil {
		return nil
	}

	info := s.Info()

	pb := &pbmsg.SessionPlan{
		SessionPlanId:      info.SessionPlanID,
		DayPlanId:          info.DayPlanID,
		WeekPlanId:         info.WeekPlanID,
		RoadmapId:          info.RoadmapID,
		UserId:             info.UserID,
		ScheduledDate:      dateToPB(info.ScheduledDate.Year(), int(info.ScheduledDate.Month()), info.ScheduledDate.Day()),
		SlotTime:           info.SlotTime,
		Status:             domainSessionStatusToPB(info.Status),
		TargetMuscleGroups: append([]string(nil), info.TargetMuscleGroups...),
		Prescription:       toPBPrescription(info.Prescription),
		Reasoning:          info.Reasoning,
		GeneratedAt:        timestamppb.New(info.GeneratedAt),
	}

	return pb
}

func toPBPrescription(p roadmap.WorkoutPrescription) *pbmsg.WorkoutPrescription {
	out := &pbmsg.WorkoutPrescription{
		WarmUps:       make([]*pbmsg.PrescribedExercise, 0, len(p.WarmUps)),
		MainExercises: make([]*pbmsg.PrescribedExercise, 0, len(p.MainExercises)),
		CoolDowns:     make([]*pbmsg.PrescribedExercise, 0, len(p.CoolDowns)),
	}

	for i := range p.WarmUps {
		out.WarmUps = append(out.WarmUps, toPBExercise(&p.WarmUps[i]))
	}

	for i := range p.MainExercises {
		out.MainExercises = append(out.MainExercises, toPBExercise(&p.MainExercises[i]))
	}

	for i := range p.CoolDowns {
		out.CoolDowns = append(out.CoolDowns, toPBExercise(&p.CoolDowns[i]))
	}

	return out
}

func toPBExercise(e *roadmap.PrescribedExercise) *pbmsg.PrescribedExercise {
	if e == nil {
		return nil
	}

	return &pbmsg.PrescribedExercise{
		ExerciseId:      e.ExerciseID,
		ExerciseName:    e.ExerciseName,
		TargetSets:      e.TargetSets,
		TargetReps:      e.TargetReps,
		TargetWeight:    e.TargetWeight,
		DurationSeconds: e.DurationSeconds,
		Notes:           e.Notes,
		RestSetSec:      e.RestSetSec,
		RestExerciseSec: e.RestExerciseSec,
		TargetRpe:       e.TargetRPE,
	}
}

func dateToPB(y, m, d int) *date.Date {
	return &date.Date{Year: int32(y), Month: int32(m), Day: int32(d)}
}

func domainStatusToPB(s roadmap.Status) pbmsg.RoadmapStatus {
	switch s {
	case roadmap.StatusActive:
		return pbmsg.RoadmapStatus_ROADMAP_STATUS_ACTIVE

	case roadmap.StatusCompleted:
		return pbmsg.RoadmapStatus_ROADMAP_STATUS_COMPLETED

	default:
		return pbmsg.RoadmapStatus_ROADMAP_STATUS_UNSPECIFIED
	}
}

func domainPhaseToPB(p roadmap.Phase) pbmsg.RoadmapPhase {
	switch p {
	case roadmap.PhaseAccumulation:
		return pbmsg.RoadmapPhase_ROADMAP_PHASE_ACCUMULATION

	case roadmap.PhaseOverload:
		return pbmsg.RoadmapPhase_ROADMAP_PHASE_OVERLOAD

	case roadmap.PhasePeak:
		return pbmsg.RoadmapPhase_ROADMAP_PHASE_PEAK

	case roadmap.PhaseDeload:
		return pbmsg.RoadmapPhase_ROADMAP_PHASE_DELOAD

	default:
		return pbmsg.RoadmapPhase_ROADMAP_PHASE_UNSPECIFIED
	}
}

func domainSessionStatusToPB(s roadmap.SessionPlanStatus) pbmsg.SessionPlanStatus {
	switch s {
	case roadmap.SessionPlanStatusPending:
		return pbmsg.SessionPlanStatus_SESSION_PLAN_STATUS_PENDING

	case roadmap.SessionPlanStatusCompleted:
		return pbmsg.SessionPlanStatus_SESSION_PLAN_STATUS_COMPLETED

	case roadmap.SessionPlanStatusSkipped:
		return pbmsg.SessionPlanStatus_SESSION_PLAN_STATUS_SKIPPED

	default:
		return pbmsg.SessionPlanStatus_SESSION_PLAN_STATUS_UNSPECIFIED
	}
}

func pbStatusToDomain(s pbmsg.RoadmapStatus) roadmap.Status {
	switch s {
	case pbmsg.RoadmapStatus_ROADMAP_STATUS_ACTIVE:
		return roadmap.StatusActive

	case pbmsg.RoadmapStatus_ROADMAP_STATUS_COMPLETED:
		return roadmap.StatusCompleted

	default:
		return "" // no filter
	}
}
