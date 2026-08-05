package transport

import (
	"context"
	"errors"
	"strings"
	"time"

	"connectrpc.com/connect"
	workoutexecutionv1message "github.com/viethung213/gym-companion/internal/gen/go/contracts/core/workout_execution/v1/message"
	workoutexecutionv1service "github.com/viethung213/gym-companion/internal/gen/go/contracts/core/workout_execution/v1/service"
	"github.com/viethung213/gym-companion/internal/gen/go/contracts/core/workout_execution/v1/service/workoutexecutionv1serviceconnect"
	"github.com/viethung213/gym-companion/internal/shared/middleware"
	"github.com/viethung213/gym-companion/internal/workout_execution/application/apperror"
	"github.com/viethung213/gym-companion/internal/workout_execution/application/command"
	"github.com/viethung213/gym-companion/internal/workout_execution/application/query"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/derror"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/vo"
	"google.golang.org/grpc/codes"
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
	updateMotionSpecHandler      *command.UpdateMotionSpecificationHandler
	deleteMotionSpecHandler      *command.DeleteMotionSpecificationHandler
	listMotionSpecsQuery         *query.ListMotionSpecificationsQueryHandler
	getPresignedUploadURLQuery   *query.GetPresignedUploadURLQueryHandler
	patchMotionSpecAssetHandler  *command.PatchMotionSpecificationAssetHandler
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
	updateMotionSpecHandler *command.UpdateMotionSpecificationHandler,
	deleteMotionSpecHandler *command.DeleteMotionSpecificationHandler,
	listMotionSpecsQuery *query.ListMotionSpecificationsQueryHandler,
	getPresignedUploadURLQuery *query.GetPresignedUploadURLQueryHandler,
	patchMotionSpecAssetHandler *command.PatchMotionSpecificationAssetHandler,
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
		updateMotionSpecHandler:      updateMotionSpecHandler,
		deleteMotionSpecHandler:      deleteMotionSpecHandler,
		listMotionSpecsQuery:         listMotionSpecsQuery,
		getPresignedUploadURLQuery:   getPresignedUploadURLQuery,
		patchMotionSpecAssetHandler:  patchMotionSpecAssetHandler,
	}
}

func extractUserID(ctx context.Context) (string, error) {
	actor, err := middleware.RequireAuthenticated(ctx)
	if err == nil && actor.UserID != "" {
		return actor.UserID, nil
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
	userID, err := extractUserID(ctx)
	if err != nil {
		return nil, err
	}

	reps := make([]vo.RepLog, 0, len(req.GetReps()))
	for _, r := range req.GetReps() {
		reps = append(reps, vo.NewRepLog(int(r.GetRepNumber()), r.GetRomPercentage(), r.GetErrorCodes(), r.GetJointAngles()))
	}

	cmd := command.LogWorkoutSetCommand{
		SessionID:   req.GetSessionId(),
		UserID:      userID,
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
	userID, err := extractUserID(ctx)
	if err != nil {
		return nil, err
	}

	cmd := command.AbortWorkoutSessionCommand{
		SessionID: req.GetSessionId(),
		UserID:    userID,
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
	userID, err := extractUserID(ctx)
	if err != nil {
		return nil, err
	}

	var weightUpdate *float32
	if req.WeightUpdateKg != nil {
		w := req.GetWeightUpdateKg()
		weightUpdate = &w
	}

	cmd := command.CompleteWorkoutSessionCommand{
		SessionID:       req.GetSessionId(),
		UserID:          userID,
		ConfirmOverload: req.GetConfirmOverload(),
		WeightUpdateKg:  weightUpdate,
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
	userID, err := extractUserID(ctx)
	if err != nil {
		return nil, err
	}

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
		UserID:    userID,
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

	return &workoutexecutionv1message.GetMotionSpecificationResponse{
		ExerciseId:             spec.ExerciseID,
		OnnxDetectorUrl:        spec.OnnxDetectorURL,
		OnnxSkeletonUrl:        spec.OnnxSkeletonURL,
		LocalRulesUrl:          spec.LocalRulesURL,
		DialogueEngineUrl:      spec.DialogueEngineURL,
		RecommendedCameraAngle: spec.RecommendedCameraAngle,
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
	userID, err := extractUserID(ctx)
	if err != nil {
		return nil, err
	}

	errs, err := h.getErrorsQuery.Handle(ctx, req.GetSessionId(), userID)
	if err != nil {
		return nil, toGRPCError("failed to fetch session errors", err)
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

// GetPresignedUploadURL generates a presigned URL allowing the Client to upload files directly to Cloud Storage.
func (h *GRPCHandler) GetPresignedUploadURL(ctx context.Context, req *workoutexecutionv1message.GetPresignedUploadURLRequest) (*workoutexecutionv1message.GetPresignedUploadURLResponse, error) {
	if _, err := requireAdminOrCoach(ctx); err != nil {
		return nil, err
	}

	if req == nil || req.GetFileName() == "" {
		return nil, status.Errorf(codes.InvalidArgument, "file_name is required")
	}

	res, err := h.getPresignedUploadURLQuery.Handle(ctx, query.GetPresignedUploadURLQuery{
		FileName:    req.GetFileName(),
		ContentType: req.GetContentType(),
	})
	if err != nil {
		return nil, toGRPCError("GetPresignedUploadURL failed", err)
	}

	return &workoutexecutionv1message.GetPresignedUploadURLResponse{
		UploadUrl: res.UploadURL,
		FileUrl:   res.FileURL,
		FileName:  res.FileName,
	}, nil
}

// UpdateMotionSpecification updates motion specification files/rules for an exercise.
func (h *GRPCHandler) UpdateMotionSpecification(ctx context.Context, req *workoutexecutionv1message.UpdateMotionSpecificationRequest) (*workoutexecutionv1message.UpdateMotionSpecificationResponse, error) {
	if _, err := requireAdminOrCoach(ctx); err != nil {
		return nil, err
	}

	if req == nil || req.GetExerciseId() == "" {
		return nil, status.Errorf(codes.InvalidArgument, "exercise_id is required")
	}

	spec, err := h.updateMotionSpecHandler.Handle(ctx, command.UpdateMotionSpecificationCommand{
		ExerciseID:             req.GetExerciseId(),
		OnnxDetectorURL:        req.GetOnnxDetectorUrl(),
		OnnxSkeletonURL:        req.GetOnnxSkeletonUrl(),
		LocalRulesURL:          req.GetLocalRulesUrl(),
		DialogueEngineURL:      req.GetDialogueEngineUrl(),
		RecommendedCameraAngle: req.GetRecommendedCameraAngle(),
	})
	if err != nil {
		return nil, toGRPCError("UpdateMotionSpecification failed", err)
	}

	return &workoutexecutionv1message.UpdateMotionSpecificationResponse{
		ExerciseId: spec.ExerciseID(),
		UpdatedAt:  timestamppb.New(spec.UpdatedAt()),
		IsReady:    spec.IsComplete(),
	}, nil
}

// PatchMotionSpecificationAsset updates partial JSON contents of pose rules or dialogue config on S3.
func (h *GRPCHandler) PatchMotionSpecificationAsset(ctx context.Context, req *workoutexecutionv1message.PatchMotionSpecificationAssetRequest) (*workoutexecutionv1message.PatchMotionSpecificationAssetResponse, error) {
	if _, err := requireAdminOrCoach(ctx); err != nil {
		return nil, err
	}

	if req == nil || req.GetExerciseId() == "" {
		return nil, status.Errorf(codes.InvalidArgument, "exercise_id is required")
	}

	assetTypeStr := req.GetAssetType().String()

	res, err := h.patchMotionSpecAssetHandler.Handle(ctx, command.PatchMotionSpecificationAssetCommand{
		ExerciseID: req.GetExerciseId(),
		AssetType:  assetTypeStr,
		PatchJSON:  req.GetPatchJson(),
		DeleteKeys: req.GetDeleteKeys(),
	})
	if err != nil {
		return nil, toGRPCError("PatchMotionSpecificationAsset failed", err)
	}

	return &workoutexecutionv1message.PatchMotionSpecificationAssetResponse{
		ExerciseId: res.ExerciseID,
		AssetType:  req.GetAssetType(),
		FileUrl:    res.FileURL,
		UpdatedAt:  timestamppb.New(res.Spec.UpdatedAt()),
	}, nil
}

// DeleteMotionSpecification deletes a MotionSpecification.
func (h *GRPCHandler) DeleteMotionSpecification(ctx context.Context, req *workoutexecutionv1message.DeleteMotionSpecificationRequest) (*workoutexecutionv1message.DeleteMotionSpecificationResponse, error) {
	if _, err := requireAdminOrCoach(ctx); err != nil {
		return nil, err
	}

	if req == nil || req.GetExerciseId() == "" {
		return nil, status.Errorf(codes.InvalidArgument, "exercise_id is required")
	}

	err := h.deleteMotionSpecHandler.Handle(ctx, command.DeleteMotionSpecificationCommand{
		ExerciseID: req.GetExerciseId(),
	})
	if err != nil {
		return nil, toGRPCError("DeleteMotionSpecification failed", err)
	}

	return &workoutexecutionv1message.DeleteMotionSpecificationResponse{
		ExerciseId: req.GetExerciseId(),
		Success:    true,
	}, nil
}

// ListMotionSpecifications lists MotionSpecifications with pagination.
func (h *GRPCHandler) ListMotionSpecifications(ctx context.Context, req *workoutexecutionv1message.ListMotionSpecificationsRequest) (*workoutexecutionv1message.ListMotionSpecificationsResponse, error) {
	pageSize := req.GetPageSize()
	if pageSize <= 0 {
		pageSize = 20
	}

	res, err := h.listMotionSpecsQuery.Handle(ctx, query.ListMotionSpecificationsQuery{
		Limit:  int(pageSize),
		Offset: 0,
	})
	if err != nil {
		return nil, toGRPCError("ListMotionSpecifications failed", err)
	}

	pbList := make([]*workoutexecutionv1message.GetMotionSpecificationResponse, len(res.Items))
	for i, s := range res.Items {
		pbList[i] = &workoutexecutionv1message.GetMotionSpecificationResponse{
			ExerciseId:             s.ExerciseID(),
			OnnxDetectorUrl:        s.OnnxDetectorURL(),
			OnnxSkeletonUrl:        s.OnnxSkeletonURL(),
			LocalRulesUrl:          s.LocalRulesURL(),
			DialogueEngineUrl:      s.DialogueEngineURL(),
			RecommendedCameraAngle: s.RecommendedCameraAngle(),
		}
	}

	return &workoutexecutionv1message.ListMotionSpecificationsResponse{
		MotionSpecifications: pbList,
		TotalCount:           int32(res.TotalCount),
	}, nil
}

func mapProtoToDialogueEngine(cfg *workoutexecutionv1message.DialogueEngineConfig) vo.DialogueEngineConfig {
	if cfg == nil {
		return vo.DialogueEngineConfig{}
	}
	dMap := make(map[string]vo.DialogueSeverities)
	for errKey, sevs := range cfg.GetDialogueMap() {
		var s1, s2 []vo.DialogueOption
		if sevs != nil {
			for _, opt := range sevs.GetSeverity_1() {
				s1 = append(s1, vo.DialogueOption{Text: opt.GetText(), AudioURL: opt.GetAudioUrl()})
			}
			for _, opt := range sevs.GetSeverity_2() {
				s2 = append(s2, vo.DialogueOption{Text: opt.GetText(), AudioURL: opt.GetAudioUrl()})
			}
		}
		dMap[errKey] = vo.DialogueSeverities{Severity1: s1, Severity2: s2}
	}
	return vo.DialogueEngineConfig{
		PersonalityID: cfg.GetPersonalityId(),
		Cooldowns:     cfg.GetCooldowns(),
		DialogueMap:   dMap,
	}
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

	// 403 Forbidden / Permission Denied
	case errors.Is(err, derror.ErrForbidden):
		return status.Errorf(codes.PermissionDenied, "%s: %v", msg, err)

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

// --- ConnectRPC Adapter ---

type ConnectWorkoutExecutionHandler struct {
	grpcHandler *GRPCHandler
}

var _ workoutexecutionv1serviceconnect.WorkoutExecutionServiceHandler = (*ConnectWorkoutExecutionHandler)(nil)
var _ workoutexecutionv1serviceconnect.AdminWorkoutExecutionServiceHandler = (*ConnectWorkoutExecutionHandler)(nil)

func NewConnectWorkoutExecutionHandler(grpcHandler *GRPCHandler) *ConnectWorkoutExecutionHandler {
	return &ConnectWorkoutExecutionHandler{grpcHandler: grpcHandler}
}

func (c *ConnectWorkoutExecutionHandler) StartWorkoutSession(ctx context.Context, req *connect.Request[workoutexecutionv1message.StartWorkoutSessionRequest]) (*connect.Response[workoutexecutionv1message.StartWorkoutSessionResponse], error) {
	res, err := c.grpcHandler.StartWorkoutSession(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}

func (c *ConnectWorkoutExecutionHandler) StartScheduledWorkoutSession(ctx context.Context, req *connect.Request[workoutexecutionv1message.StartScheduledWorkoutSessionRequest]) (*connect.Response[workoutexecutionv1message.StartScheduledWorkoutSessionResponse], error) {
	res, err := c.grpcHandler.StartScheduledWorkoutSession(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}

func (c *ConnectWorkoutExecutionHandler) LogWorkoutSet(ctx context.Context, req *connect.Request[workoutexecutionv1message.LogWorkoutSetRequest]) (*connect.Response[workoutexecutionv1message.LogWorkoutSetResponse], error) {
	res, err := c.grpcHandler.LogWorkoutSet(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}

func (c *ConnectWorkoutExecutionHandler) CompleteWorkoutSession(ctx context.Context, req *connect.Request[workoutexecutionv1message.CompleteWorkoutSessionRequest]) (*connect.Response[workoutexecutionv1message.CompleteWorkoutSessionResponse], error) {
	res, err := c.grpcHandler.CompleteWorkoutSession(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}

func (c *ConnectWorkoutExecutionHandler) AbortWorkoutSession(ctx context.Context, req *connect.Request[workoutexecutionv1message.AbortWorkoutSessionRequest]) (*connect.Response[workoutexecutionv1message.AbortWorkoutSessionResponse], error) {
	res, err := c.grpcHandler.AbortWorkoutSession(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}

func (c *ConnectWorkoutExecutionHandler) SyncWorkoutLogs(ctx context.Context, req *connect.Request[workoutexecutionv1message.SyncWorkoutLogsRequest]) (*connect.Response[workoutexecutionv1message.SyncWorkoutLogsResponse], error) {
	res, err := c.grpcHandler.SyncWorkoutLogs(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}

func (c *ConnectWorkoutExecutionHandler) GetMotionSpecification(ctx context.Context, req *connect.Request[workoutexecutionv1message.GetMotionSpecificationRequest]) (*connect.Response[workoutexecutionv1message.GetMotionSpecificationResponse], error) {
	res, err := c.grpcHandler.GetMotionSpecification(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}

func (c *ConnectWorkoutExecutionHandler) GetPersonalRecords(ctx context.Context, req *connect.Request[workoutexecutionv1message.GetPersonalRecordsRequest]) (*connect.Response[workoutexecutionv1message.GetPersonalRecordsResponse], error) {
	res, err := c.grpcHandler.GetPersonalRecords(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}

func (c *ConnectWorkoutExecutionHandler) GetWorkoutSessionErrors(ctx context.Context, req *connect.Request[workoutexecutionv1message.GetWorkoutSessionErrorsRequest]) (*connect.Response[workoutexecutionv1message.GetWorkoutSessionErrorsResponse], error) {
	res, err := c.grpcHandler.GetWorkoutSessionErrors(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}

func (c *ConnectWorkoutExecutionHandler) GetWorkoutHistory(ctx context.Context, req *connect.Request[workoutexecutionv1message.GetWorkoutHistoryRequest]) (*connect.Response[workoutexecutionv1message.GetWorkoutHistoryResponse], error) {
	res, err := c.grpcHandler.GetWorkoutHistory(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}

// Admin Methods
func (c *ConnectWorkoutExecutionHandler) AdminGetPersonalRecords(ctx context.Context, req *connect.Request[workoutexecutionv1message.AdminGetPersonalRecordsRequest]) (*connect.Response[workoutexecutionv1message.AdminGetPersonalRecordsResponse], error) {
	res, err := c.grpcHandler.AdminGetPersonalRecords(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}

func (c *ConnectWorkoutExecutionHandler) AdminGetWorkoutHistory(ctx context.Context, req *connect.Request[workoutexecutionv1message.AdminGetWorkoutHistoryRequest]) (*connect.Response[workoutexecutionv1message.AdminGetWorkoutHistoryResponse], error) {
	res, err := c.grpcHandler.AdminGetWorkoutHistory(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}

func (c *ConnectWorkoutExecutionHandler) UpdateMotionSpecification(ctx context.Context, req *connect.Request[workoutexecutionv1message.UpdateMotionSpecificationRequest]) (*connect.Response[workoutexecutionv1message.UpdateMotionSpecificationResponse], error) {
	res, err := c.grpcHandler.UpdateMotionSpecification(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}

func (c *ConnectWorkoutExecutionHandler) DeleteMotionSpecification(ctx context.Context, req *connect.Request[workoutexecutionv1message.DeleteMotionSpecificationRequest]) (*connect.Response[workoutexecutionv1message.DeleteMotionSpecificationResponse], error) {
	res, err := c.grpcHandler.DeleteMotionSpecification(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}

func (c *ConnectWorkoutExecutionHandler) ListMotionSpecifications(ctx context.Context, req *connect.Request[workoutexecutionv1message.ListMotionSpecificationsRequest]) (*connect.Response[workoutexecutionv1message.ListMotionSpecificationsResponse], error) {
	res, err := c.grpcHandler.ListMotionSpecifications(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}

func (c *ConnectWorkoutExecutionHandler) GetPresignedUploadURL(ctx context.Context, req *connect.Request[workoutexecutionv1message.GetPresignedUploadURLRequest]) (*connect.Response[workoutexecutionv1message.GetPresignedUploadURLResponse], error) {
	res, err := c.grpcHandler.GetPresignedUploadURL(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}

func (c *ConnectWorkoutExecutionHandler) PatchMotionSpecificationAsset(ctx context.Context, req *connect.Request[workoutexecutionv1message.PatchMotionSpecificationAssetRequest]) (*connect.Response[workoutexecutionv1message.PatchMotionSpecificationAssetResponse], error) {
	res, err := c.grpcHandler.PatchMotionSpecificationAsset(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}
