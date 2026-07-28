package transport

import (
	"context"
	"errors"
	"strings"
	"time"

	workoutexecutionv1message "github.com/viethung213/gym-companion/internal/gen/go/contracts/core/workout_execution/v1/message"
	workoutexecutionv1service "github.com/viethung213/gym-companion/internal/gen/go/contracts/core/workout_execution/v1/service"
	"github.com/viethung213/gym-companion/internal/shared/middleware"
	"github.com/viethung213/gym-companion/internal/workout_execution/application/apperror"
	"github.com/viethung213/gym-companion/internal/workout_execution/application/command"
	"github.com/viethung213/gym-companion/internal/workout_execution/application/query"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/derror"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/vo"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// GRPCHandler implements WorkoutExecutionServiceServer and AdminWorkoutExecutionServiceServer.
type GRPCHandler struct {
	workoutexecutionv1service.UnimplementedWorkoutExecutionServiceServer
	workoutexecutionv1service.UnimplementedAdminWorkoutExecutionServiceServer
	startSessionHandler          *command.StartWorkoutSessionHandler
	startScheduledSessionHandler *command.StartScheduledWorkoutSessionHandler
	logSetHandler                *command.LogWorkoutSetHandler
	completeSessionHandler       *command.CompleteWorkoutSessionHandler
	abortSessionHandler          *command.AbortWorkoutSessionHandler
	syncLogsHandler              *command.SyncWorkoutLogsHandler
	getMotionSpecQuery           *query.GetMotionSpecificationQueryHandler
	getPRsQuery                  *query.GetPersonalRecordsQueryHandler
	getErrorsQuery               *query.GetWorkoutSessionErrorsQueryHandler
	getHistoryQuery              *query.GetWorkoutHistoryQueryHandler
}

var _ workoutexecutionv1service.WorkoutExecutionServiceServer = (*GRPCHandler)(nil)
var _ workoutexecutionv1service.AdminWorkoutExecutionServiceServer = (*GRPCHandler)(nil)

// NewGRPCHandler constructs GRPCHandler.
func NewGRPCHandler(
	startSessionHandler *command.StartWorkoutSessionHandler,
	startScheduledSessionHandler *command.StartScheduledWorkoutSessionHandler,
	logSetHandler *command.LogWorkoutSetHandler,
	completeSessionHandler *command.CompleteWorkoutSessionHandler,
	abortSessionHandler *command.AbortWorkoutSessionHandler,
	syncLogsHandler *command.SyncWorkoutLogsHandler,
	getMotionSpecQuery *query.GetMotionSpecificationQueryHandler,
	getPRsQuery *query.GetPersonalRecordsQueryHandler,
	getErrorsQuery *query.GetWorkoutSessionErrorsQueryHandler,
	getHistoryQuery *query.GetWorkoutHistoryQueryHandler,
) *GRPCHandler {
	return &GRPCHandler{
		startSessionHandler:          startSessionHandler,
		startScheduledSessionHandler: startScheduledSessionHandler,
		logSetHandler:                logSetHandler,
		completeSessionHandler:       completeSessionHandler,
		abortSessionHandler:          abortSessionHandler,
		syncLogsHandler:              syncLogsHandler,
		getMotionSpecQuery:           getMotionSpecQuery,
		getPRsQuery:                  getPRsQuery,
		getErrorsQuery:               getErrorsQuery,
		getHistoryQuery:              getHistoryQuery,
	}
}

func extractUserID(ctx context.Context) (string, error) {
	actor, err := middleware.RequireAuthenticated(ctx)
	if err == nil && actor.UserID != "" {
		return actor.UserID, nil
	}

	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if vals := md.Get("x-user-id"); len(vals) > 0 && vals[0] != "" {
			return vals[0], nil
		}
		if vals := md.Get("user_id"); len(vals) > 0 && vals[0] != "" {
			return vals[0], nil
		}
	}

	return "", status.Errorf(codes.Unauthenticated, "unauthenticated request: missing or invalid JWT identity: %v", err)
}

func requireAdminOrCoach(ctx context.Context) (middleware.Actor, error) {
	actor, err := middleware.RequireAuthenticated(ctx)
	if err != nil {
		return middleware.Actor{}, status.Errorf(codes.Unauthenticated, "unauthenticated request: %v", err)
	}
	if !actor.IsAdmin() && !strings.EqualFold(actor.Role, "Coach") {
		return middleware.Actor{}, status.Errorf(codes.PermissionDenied, "forbidden: admin or coach role required")
	}
	return actor, nil
}

func (h *GRPCHandler) StartWorkoutSession(ctx context.Context, req *workoutexecutionv1message.StartWorkoutSessionRequest) (*workoutexecutionv1message.StartWorkoutSessionResponse, error) {
	userID, err := extractUserID(ctx)
	if err != nil {
		return nil, err
	}

	cmd := command.StartWorkoutSessionCommand{
		UserID: userID,
		PlanID: req.GetPlanId(),
	}

	res, err := h.startSessionHandler.Handle(ctx, cmd)
	if err != nil {
		return nil, toGRPCError("failed to start workout session", err)
	}

	return &workoutexecutionv1message.StartWorkoutSessionResponse{
		SessionId: res.SessionID,
		StartedAt: parseTimestampOrNow(res.StartedAt),
	}, nil
}

func (h *GRPCHandler) StartScheduledWorkoutSession(ctx context.Context, req *workoutexecutionv1message.StartScheduledWorkoutSessionRequest) (*workoutexecutionv1message.StartScheduledWorkoutSessionResponse, error) {
	userID, err := extractUserID(ctx)
	if err != nil {
		return nil, err
	}

	cmd := command.StartScheduledWorkoutSessionCommand{
		SessionID: req.GetSessionId(),
		UserID:    userID,
	}

	res, err := h.startScheduledSessionHandler.Handle(ctx, cmd)
	if err != nil {
		return nil, toGRPCError("failed to start scheduled workout session", err)
	}

	return &workoutexecutionv1message.StartScheduledWorkoutSessionResponse{
		SessionId: res.SessionID,
		StartedAt: parseTimestampOrNow(res.StartedAt),
	}, nil
}

func (h *GRPCHandler) LogWorkoutSet(ctx context.Context, req *workoutexecutionv1message.LogWorkoutSetRequest) (*workoutexecutionv1message.LogWorkoutSetResponse, error) {
	reps := make([]vo.RepLog, 0, len(req.GetReps()))
	for _, r := range req.GetReps() {
		reps = append(reps, vo.NewRepLog(int(r.GetRepNumber()), r.GetRomPercentage(), r.GetErrorCodes(), r.GetJointAngles()))
	}

	cmd := command.LogWorkoutSetCommand{
		SessionID:   req.GetSessionId(),
		SetNumber:   int(req.GetSetNumber()),
		ExerciseID:  req.GetExerciseId(),
		TargetReps:  int(req.GetTargetReps()),
		ActualReps:  int(req.GetActualReps()),
		Weight:      req.GetWeight(),
		FormScore:   req.FormScore,
		RPE:         req.GetRpe(),
		CameraAngle: req.GetCameraAngle(),
		Reps:        reps,
	}

	res, err := h.logSetHandler.Handle(ctx, cmd)
	if err != nil {
		return nil, toGRPCError("failed to log workout set", err)
	}

	return &workoutexecutionv1message.LogWorkoutSetResponse{
		SetLogId: res.SetLogID,
	}, nil
}

func (h *GRPCHandler) AbortWorkoutSession(ctx context.Context, req *workoutexecutionv1message.AbortWorkoutSessionRequest) (*workoutexecutionv1message.AbortWorkoutSessionResponse, error) {
	cmd := command.AbortWorkoutSessionCommand{
		SessionID: req.GetSessionId(),
		Reason:    req.GetReason(),
	}

	res, err := h.abortSessionHandler.Handle(ctx, cmd)
	if err != nil {
		return nil, toGRPCError("failed to abort workout session", err)
	}

	return &workoutexecutionv1message.AbortWorkoutSessionResponse{
		SessionId: res.SessionID,
		AbortedAt: parseTimestampOrNow(res.AbortedAt),
	}, nil
}

func (h *GRPCHandler) CompleteWorkoutSession(ctx context.Context, req *workoutexecutionv1message.CompleteWorkoutSessionRequest) (*workoutexecutionv1message.CompleteWorkoutSessionResponse, error) {
	cmd := command.CompleteWorkoutSessionCommand{
		SessionID: req.GetSessionId(),
	}

	res, err := h.completeSessionHandler.Handle(ctx, cmd)
	if err != nil {
		return nil, toGRPCError("failed to complete workout session", err)
	}

	return &workoutexecutionv1message.CompleteWorkoutSessionResponse{
		SessionId:        res.SessionID,
		CompletedAt:      parseTimestampOrNow(res.CompletedAt),
		TotalSets:        int32(res.TotalSets),
		TotalVolume:      res.TotalVolume,
		AverageFormScore: res.AverageFormScore,
		AverageRpe:       res.AverageRPE,
	}, nil
}

func (h *GRPCHandler) SyncWorkoutLogs(ctx context.Context, req *workoutexecutionv1message.SyncWorkoutLogsRequest) (*workoutexecutionv1message.SyncWorkoutLogsResponse, error) {
	errorsDTO := make([]command.ErrorLogDTO, 0, len(req.GetErrors()))
	for _, e := range req.GetErrors() {
		var ts timestamppb.Timestamp
		if e.GetTimestamp() != nil {
			ts = *e.GetTimestamp()
		}
		errorsDTO = append(errorsDTO, command.ErrorLogDTO{
			ErrorCode:  e.GetErrorCode(),
			Severity:   e.GetSeverity(),
			Timestamp:  ts.AsTime(),
			SetNumber:  int(e.GetSetNumber()),
			RepNumber:  int(e.GetRepNumber()),
			ExerciseID: e.GetExerciseId(),
		})
	}

	cmd := command.SyncWorkoutLogsCommand{
		SessionID: req.GetSessionId(),
		Errors:    errorsDTO,
	}

	if err := h.syncLogsHandler.Handle(ctx, cmd); err != nil {
		return nil, toGRPCError("failed to sync logs", err)
	}

	return &workoutexecutionv1message.SyncWorkoutLogsResponse{}, nil
}

func (h *GRPCHandler) GetMotionSpecification(ctx context.Context, req *workoutexecutionv1message.GetMotionSpecificationRequest) (*workoutexecutionv1message.GetMotionSpecificationResponse, error) {
	spec, err := h.getMotionSpecQuery.Handle(ctx, req.GetExerciseId(), req.GetCoachPersonality())
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "motion spec not found: %v", err)
	}

	dEngine := spec.DialogueEngine()
	return &workoutexecutionv1message.GetMotionSpecificationResponse{
		ExerciseId:             spec.ExerciseID(),
		OnnxModelUrl:           spec.OnnxModelURL(),
		LocalRulesUrl:          spec.LocalRulesURL(),
		RecommendedCameraAngle: spec.RecommendedCameraAngle(),
		DialogueEngine: &workoutexecutionv1message.DialogueEngineConfig{
			PersonalityId: dEngine.PersonalityID,
		},
	}, nil
}

func (h *GRPCHandler) GetPersonalRecords(ctx context.Context, req *workoutexecutionv1message.GetPersonalRecordsRequest) (*workoutexecutionv1message.GetPersonalRecordsResponse, error) {
	userID, err := extractUserID(ctx)
	if err != nil {
		return nil, err
	}

	records, err := h.getPRsQuery.Handle(ctx, userID, req.GetExerciseIds())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to fetch PRs: %v", err)
	}

	pbRecords := make([]*workoutexecutionv1message.PersonalRecord, len(records))
	for i, r := range records {
		pbRecords[i] = &workoutexecutionv1message.PersonalRecord{
			ExerciseId: r.ExerciseID(),
			OneRepMax:  r.OneRepMax(),
			Weight:     r.Weight(),
			Reps:       int32(r.Reps()),
			AchievedAt: timestamppb.New(r.AchievedAt()),
		}
	}

	return &workoutexecutionv1message.GetPersonalRecordsResponse{
		Records: pbRecords,
	}, nil
}

func (h *GRPCHandler) GetWorkoutSessionErrors(ctx context.Context, req *workoutexecutionv1message.GetWorkoutSessionErrorsRequest) (*workoutexecutionv1message.GetWorkoutSessionErrorsResponse, error) {
	errs, err := h.getErrorsQuery.Handle(ctx, req.GetSessionId())
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "failed to fetch session errors: %v", err)
	}

	pbErrors := make([]*workoutexecutionv1message.ErrorLog, len(errs))
	for i, e := range errs {
		pbErrors[i] = &workoutexecutionv1message.ErrorLog{
			ErrorCode:  e.ErrorCode,
			Severity:   e.Severity,
			Timestamp:  timestamppb.New(e.Timestamp),
			SetNumber:  int32(e.SetNumber),
			RepNumber:  int32(e.RepNumber),
			ExerciseId: e.ExerciseID,
		}
	}

	return &workoutexecutionv1message.GetWorkoutSessionErrorsResponse{
		SessionId: req.GetSessionId(),
		Errors:    pbErrors,
	}, nil
}

func (h *GRPCHandler) GetWorkoutHistory(ctx context.Context, req *workoutexecutionv1message.GetWorkoutHistoryRequest) (*workoutexecutionv1message.GetWorkoutHistoryResponse, error) {
	userID, err := extractUserID(ctx)
	if err != nil {
		return nil, err
	}

	sessions, err := h.getHistoryQuery.Handle(ctx, userID, int(req.GetLimit()), int(req.GetOffset()))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to fetch workout history: %v", err)
	}

	pbSessions := make([]*workoutexecutionv1message.WorkoutSessionSummary, len(sessions))
	for i, s := range sessions {
		summary := s.CalculateSummary()
		var sessionDate time.Time
		if s.StartedAt() != nil {
			sessionDate = *s.StartedAt()
		} else {
			sessionDate = s.CreatedAt()
		}

		pbSessions[i] = &workoutexecutionv1message.WorkoutSessionSummary{
			SessionId:        s.ID(),
			Date:             timestamppb.New(sessionDate),
			TotalSets:        int32(summary.TotalSets),
			TotalVolume:      summary.TotalVolume,
			AverageFormScore: summary.AverageFormScore,
		}
	}

	return &workoutexecutionv1message.GetWorkoutHistoryResponse{
		Sessions: pbSessions,
	}, nil
}

func (h *GRPCHandler) AdminGetPersonalRecords(ctx context.Context, req *workoutexecutionv1message.AdminGetPersonalRecordsRequest) (*workoutexecutionv1message.AdminGetPersonalRecordsResponse, error) {
	if _, err := requireAdminOrCoach(ctx); err != nil {
		return nil, err
	}

	userID := req.GetUserId()
	if userID == "" {
		return nil, status.Errorf(codes.InvalidArgument, "user_id is required for admin lookup")
	}

	records, err := h.getPRsQuery.Handle(ctx, userID, req.GetExerciseIds())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to fetch user PRs: %v", err)
	}

	pbRecords := make([]*workoutexecutionv1message.PersonalRecord, len(records))
	for i, r := range records {
		pbRecords[i] = &workoutexecutionv1message.PersonalRecord{
			ExerciseId: r.ExerciseID(),
			OneRepMax:  r.OneRepMax(),
			Weight:     r.Weight(),
			Reps:       int32(r.Reps()),
			AchievedAt: timestamppb.New(r.AchievedAt()),
		}
	}

	return &workoutexecutionv1message.AdminGetPersonalRecordsResponse{
		Records: pbRecords,
	}, nil
}

func (h *GRPCHandler) AdminGetWorkoutHistory(ctx context.Context, req *workoutexecutionv1message.AdminGetWorkoutHistoryRequest) (*workoutexecutionv1message.AdminGetWorkoutHistoryResponse, error) {
	if _, err := requireAdminOrCoach(ctx); err != nil {
		return nil, err
	}

	userID := req.GetUserId()
	if userID == "" {
		return nil, status.Errorf(codes.InvalidArgument, "user_id is required for admin lookup")
	}

	sessions, err := h.getHistoryQuery.Handle(ctx, userID, int(req.GetLimit()), int(req.GetOffset()))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to fetch user workout history: %v", err)
	}

	pbSessions := make([]*workoutexecutionv1message.WorkoutSessionSummary, len(sessions))
	for i, s := range sessions {
		summary := s.CalculateSummary()
		var sessionDate time.Time
		if s.StartedAt() != nil {
			sessionDate = *s.StartedAt()
		} else {
			sessionDate = s.CreatedAt()
		}

		pbSessions[i] = &workoutexecutionv1message.WorkoutSessionSummary{
			SessionId:        s.ID(),
			Date:             timestamppb.New(sessionDate),
			TotalSets:        int32(summary.TotalSets),
			TotalVolume:      summary.TotalVolume,
			AverageFormScore: summary.AverageFormScore,
		}
	}

	return &workoutexecutionv1message.AdminGetWorkoutHistoryResponse{
		Sessions: pbSessions,
	}, nil
}

// parseTimestampOrNow parses an RFC3339 string into a Protobuf Timestamp, defaulting to Now() if empty or invalid.
func parseTimestampOrNow(timeStr string) *timestamppb.Timestamp {
	if timeStr != "" {
		if t, err := time.Parse(time.RFC3339, timeStr); err == nil {
			return timestamppb.New(t)
		}
	}
	return timestamppb.Now()
}

// toGRPCError maps known domain and application errors to the appropriate
// gRPC status code. Unknown errors fall back to codes.Internal.
func toGRPCError(msg string, err error) error {
	switch {
	// 404 Not Found
	case errors.Is(err, derror.ErrWorkoutSessionNotFound),
		errors.Is(err, derror.ErrMotionSpecNotFound),
		errors.Is(err, derror.ErrPersonalRecordNotFound):
		return status.Errorf(codes.NotFound, "%s: %v", msg, err)

	// 400 Bad Request / Invalid Argument
	case errors.Is(err, apperror.ErrInvalidInput),
		errors.Is(err, derror.ErrInvalidSetNumber),
		errors.Is(err, derror.ErrInvalidRepsOrWeight),
		errors.Is(err, derror.ErrInvalidROM):
		return status.Errorf(codes.InvalidArgument, "%s: %v", msg, err)

	// 409 Failed Precondition — session lifecycle violations
	case errors.Is(err, derror.ErrSessionNotInProgress),
		errors.Is(err, derror.ErrSessionAlreadyCompleted),
		errors.Is(err, derror.ErrSessionAlreadyAborted),
		errors.Is(err, derror.ErrActiveSessionAlreadyExists),
		errors.Is(err, derror.ErrAnomalousSessionTimeout):
		return status.Errorf(codes.FailedPrecondition, "%s: %v", msg, err)

	// 400 Overload confirmation required — client must explicitly confirm
	case errors.Is(err, derror.ErrOverloadConfirmationRequired):
		return status.Errorf(codes.FailedPrecondition, "%s: %v", msg, err)

	// 500 Internal — infrastructure / unknown errors
	default:
		return status.Errorf(codes.Internal, "%s: %v", msg, err)
	}
}
