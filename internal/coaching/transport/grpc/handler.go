// Package grpc implements the CoachingService gRPC endpoint.
package grpc

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	"github.com/viethung213/gym-companion/internal/coaching/application/command"
	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	"github.com/viethung213/gym-companion/internal/coaching/application/query"
	"github.com/viethung213/gym-companion/internal/coaching/domain/roadmap"
	pbmsg "github.com/viethung213/gym-companion/internal/gen/go/contracts/core/coaching/v1/message"
	pbsvc "github.com/viethung213/gym-companion/internal/gen/go/contracts/core/coaching/v1/service"
	"github.com/viethung213/gym-companion/internal/gen/go/contracts/core/coaching/v1/service/coachingv1serviceconnect"
	"google.golang.org/genproto/googleapis/type/date"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Server implements pbsvc.CoachingServiceServer.
type Server struct {
	pbsvc.UnimplementedCoachingServiceServer
	initiate     *command.InitiateRoadmapHandler
	regenerate   *command.RegenerateScheduleHandler
	createAdhoc  *command.CreateAdhocSessionHandler
	suggestAdHoc *query.SuggestAdHocSessionHandler
	queries      *query.Handlers
}

// NewServer wires the gRPC handler.
func NewServer(
	initiate *command.InitiateRoadmapHandler,
	regenerate *command.RegenerateScheduleHandler,
	createAdhoc *command.CreateAdhocSessionHandler,
	queries *query.Handlers,
) *Server {
	return &Server{initiate: initiate, regenerate: regenerate, createAdhoc: createAdhoc, queries: queries}
}

// WithSuggestAdHoc attaches the SuggestAdHocSessionHandler.
func (s *Server) WithSuggestAdHoc(suggestAdHoc *query.SuggestAdHocSessionHandler) *Server {
	s.suggestAdHoc = suggestAdHoc
	return s
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

// CreateAdhocSessionPlan creates an ad-hoc session plan (Flow 2.1).
func (s *Server) CreateAdhocSessionPlan(
	ctx context.Context,
	req *pbmsg.CreateAdhocSessionPlanRequest,
) (*pbmsg.CreateAdhocSessionPlanResponse, error) {
	if req.GetUserId() == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	res, err := s.createAdhoc.Handle(ctx, command.CreateAdhocSessionCommand{
		UserID:      req.GetUserId(),
		ExerciseIDs: req.GetExerciseIds(),
	})

	if err != nil {
		return nil, mapDomainErr(err)
	}

	return &pbmsg.CreateAdhocSessionPlanResponse{SessionPlan: toPBSession(res.SessionPlan)}, nil
}

// SuggestAdHocSession suggests an ad-hoc workout prescription.
func (s *Server) SuggestAdHocSession(
	ctx context.Context,
	req *pbmsg.SuggestAdHocSessionRequest,
) (*pbmsg.SuggestAdHocSessionResponse, error) {
	if req.GetUserId() == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	if s.suggestAdHoc == nil {
		return nil, status.Error(codes.Unimplemented, "suggest adhoc handler not configured")
	}

	hint := port.AdHocHint{}
	if req.GetHint() != nil {
		hint.FreeText = req.GetHint().GetFreeText()
		hint.MuscleGroups = req.GetHint().GetMuscleGroups()
		hint.AvailableEquipment = req.GetHint().GetAvailableEquipment()
		hint.DurationMinutes = int(req.GetHint().GetDurationMinutes())
		hint.IntensityHint = req.GetHint().GetIntensityHint()
	}

	res, err := s.suggestAdHoc.Handle(ctx, &query.SuggestAdHocSessionQuery{
		UserID: req.GetUserId(),
		Hint:   hint,
	})

	if err != nil {
		return nil, mapDomainErr(err)
	}

	return &pbmsg.SuggestAdHocSessionResponse{
		MuscleGroups: res.MuscleGroups,
		Prescription: toPBPrescription(res.Prescription),
		Reasoning:    res.Reasoning,
		EstimatedRpe: res.EstimatedRPE,
	}, nil
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
		Source:             domainSourceToPB(info.Source),
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

	case roadmap.SessionPlanStatusAborted:
		return pbmsg.SessionPlanStatus_SESSION_PLAN_STATUS_ABORTED

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

func domainSourceToPB(src roadmap.SessionPlanSource) pbmsg.SessionPlanSource {
	switch src {
	case roadmap.SessionPlanSourceScheduled:
		return pbmsg.SessionPlanSource_SESSION_PLAN_SOURCE_COACH_SCHEDULED

	case roadmap.SessionPlanSourceAdHoc:
		return pbmsg.SessionPlanSource_SESSION_PLAN_SOURCE_USER_ADHOC

	default:
		return pbmsg.SessionPlanSource_SESSION_PLAN_SOURCE_UNSPECIFIED
	}
}

func pbSourceToDomain(src pbmsg.SessionPlanSource) roadmap.SessionPlanSource {
	switch src {
	case pbmsg.SessionPlanSource_SESSION_PLAN_SOURCE_COACH_SCHEDULED:
		return roadmap.SessionPlanSourceScheduled

	case pbmsg.SessionPlanSource_SESSION_PLAN_SOURCE_USER_ADHOC:
		return roadmap.SessionPlanSourceAdHoc

	default:
		return ""
	}
}

// --- ConnectRPC Adapter ---

type ConnectCoachingHandler struct {
	server *Server
}

var _ coachingv1serviceconnect.CoachingServiceHandler = (*ConnectCoachingHandler)(nil)

func NewConnectCoachingHandler(server *Server) coachingv1serviceconnect.CoachingServiceHandler {
	return &ConnectCoachingHandler{server: server}
}

func (c *ConnectCoachingHandler) InitiateRoadmap(
	ctx context.Context,
	req *connect.Request[pbmsg.InitiateRoadmapRequest],
) (*connect.Response[pbmsg.InitiateRoadmapResponse], error) {
	res, err := c.server.InitiateRoadmap(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}

func (c *ConnectCoachingHandler) ListRoadmaps(
	ctx context.Context,
	req *connect.Request[pbmsg.ListRoadmapsRequest],
) (*connect.Response[pbmsg.ListRoadmapsResponse], error) {
	res, err := c.server.ListRoadmaps(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}

func (c *ConnectCoachingHandler) GetRoadmap(
	ctx context.Context,
	req *connect.Request[pbmsg.GetRoadmapRequest],
) (*connect.Response[pbmsg.GetRoadmapResponse], error) {
	res, err := c.server.GetRoadmap(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}

func (c *ConnectCoachingHandler) GetActiveRoadmap(
	ctx context.Context,
	req *connect.Request[pbmsg.GetActiveRoadmapRequest],
) (*connect.Response[pbmsg.GetActiveRoadmapResponse], error) {
	res, err := c.server.GetActiveRoadmap(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}

func (c *ConnectCoachingHandler) GetSessionPlan(
	ctx context.Context,
	req *connect.Request[pbmsg.GetSessionPlanRequest],
) (*connect.Response[pbmsg.GetSessionPlanResponse], error) {
	res, err := c.server.GetSessionPlan(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}

func (c *ConnectCoachingHandler) RegenerateSchedule(
	ctx context.Context,
	req *connect.Request[pbmsg.RegenerateScheduleRequest],
) (*connect.Response[pbmsg.RegenerateScheduleResponse], error) {
	res, err := c.server.RegenerateSchedule(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}

func (c *ConnectCoachingHandler) CreateAdhocSessionPlan(
	ctx context.Context,
	req *connect.Request[pbmsg.CreateAdhocSessionPlanRequest],
) (*connect.Response[pbmsg.CreateAdhocSessionPlanResponse], error) {
	res, err := c.server.CreateAdhocSessionPlan(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}

func (c *ConnectCoachingHandler) SuggestAdHocSession(
	ctx context.Context,
	req *connect.Request[pbmsg.SuggestAdHocSessionRequest],
) (*connect.Response[pbmsg.SuggestAdHocSessionResponse], error) {
	res, err := c.server.SuggestAdHocSession(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}
