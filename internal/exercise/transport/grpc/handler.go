// Package grpc contains RPC adapters for Exercise.
package grpc

import (
	"context"
	"errors"
	"strings"

	"github.com/viethung213/gym-companion/internal/exercise/application/command"
	"github.com/viethung213/gym-companion/internal/exercise/application/port"
	"github.com/viethung213/gym-companion/internal/exercise/application/query"
	"github.com/viethung213/gym-companion/internal/exercise/domain"
	"github.com/viethung213/gym-companion/internal/exercise/infrastructure/protomap"
	exercisemsg "github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/exercise/v1/message"
	exercisesvc "github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/exercise/v1/service"
	"github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/exercise/v1/service/exercisev1serviceconnect"
	"github.com/viethung213/gym-companion/internal/shared/middleware"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ExerciseServer struct {
	exercisesvc.UnimplementedExerciseServiceServer

	createHandler    *command.CreateExerciseHandler
	updateHandler    *command.UpdateExerciseHandler
	approveHandler   *command.ApproveExerciseHandler
	archiveHandler   *command.ArchiveExerciseHandler
	restoreHandler   *command.RestoreExerciseHandler
	getHandler       *query.GetExerciseHandler
	getRandomHandler *query.GetRandomExerciseHandler
	searchHandler    *query.SearchExercisesHandler

	// mediaBaseURL prefixes relative image paths in responses (D3).
	mediaBaseURL string
}

var _ exercisesvc.ExerciseServiceServer = (*ExerciseServer)(nil)

func NewExerciseServer(
	createHandler *command.CreateExerciseHandler,
	updateHandler *command.UpdateExerciseHandler,
	approveHandler *command.ApproveExerciseHandler,
	archiveHandler *command.ArchiveExerciseHandler,
	restoreHandler *command.RestoreExerciseHandler,
	getHandler *query.GetExerciseHandler,
	getRandomHandler *query.GetRandomExerciseHandler,
	searchHandler *query.SearchExercisesHandler,
	mediaBaseURL string,
) *ExerciseServer {
	return &ExerciseServer{
		createHandler:    createHandler,
		updateHandler:    updateHandler,
		approveHandler:   approveHandler,
		archiveHandler:   archiveHandler,
		restoreHandler:   restoreHandler,
		getHandler:       getHandler,
		getRandomHandler: getRandomHandler,
		searchHandler:    searchHandler,
		mediaBaseURL:     mediaBaseURL,
	}
}

func (s *ExerciseServer) SearchExercises(
	ctx context.Context,
	req *exercisemsg.SearchExercisesRequest,
) (*exercisemsg.SearchExercisesResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	filters, err := toSearchFilters(req)
	if err != nil {
		return nil, rpcError(err)
	}
	result, err := s.searchHandler.Handle(ctx, query.SearchExercisesQuery{Filters: filters})
	if err != nil {
		return nil, rpcError(err)
	}

	response := &exercisemsg.SearchExercisesResponse{
		Exercises:  make([]*exercisemsg.ExerciseInfo, 0, len(result.Exercises)),
		TotalCount: result.TotalCount,
	}
	for _, exercise := range result.Exercises {
		info := exercise.Info()
		response.Exercises = append(response.Exercises, s.toProtoExercise(&info))
	}

	return response, nil
}

func (s *ExerciseServer) GetExercise(
	ctx context.Context,
	req *exercisemsg.GetExerciseRequest,
) (*exercisemsg.GetExerciseResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	exercise, err := s.getHandler.Handle(ctx, query.GetExerciseQuery{ID: req.GetId()})
	if err != nil {
		return nil, rpcError(err)
	}

	info := exercise.Info()
	return &exercisemsg.GetExerciseResponse{
		Exercise: s.toProtoExercise(&info),
	}, nil
}

func (s *ExerciseServer) GetRandomExercise(
	ctx context.Context,
	_ *exercisemsg.GetRandomExerciseRequest,
) (*exercisemsg.GetRandomExerciseResponse, error) {
	exercise, err := s.getRandomHandler.Handle(ctx, query.GetRandomExerciseQuery{})
	if err != nil {
		return nil, rpcError(err)
	}

	info := exercise.Info()
	return &exercisemsg.GetRandomExerciseResponse{
		Exercise: s.toProtoExercise(&info),
	}, nil
}

func (s *ExerciseServer) CreateExercise(
	ctx context.Context,
	req *exercisemsg.CreateExerciseRequest,
) (*exercisemsg.CreateExerciseResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	info, err := infoFromRequest(req)
	if err != nil {
		return nil, rpcError(err)
	}
	info.Slug = req.GetSlug()
	info.Status = toDomainStatus(req.GetStatus())

	exercise, err := s.createHandler.Handle(ctx, &command.CreateExerciseCommand{Info: info})
	if err != nil {
		return nil, rpcError(err)
	}

	saved := exercise.Info()
	return &exercisemsg.CreateExerciseResponse{
		Exercise: s.toProtoExercise(&saved),
	}, nil
}

func (s *ExerciseServer) UpdateExercise(
	ctx context.Context,
	req *exercisemsg.UpdateExerciseRequest,
) (*exercisemsg.UpdateExerciseResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	info, err := infoFromRequest(req)
	if err != nil {
		return nil, rpcError(err)
	}

	exercise, err := s.updateHandler.Handle(ctx, &command.UpdateExerciseCommand{
		ID:   req.GetId(),
		Info: info,
	})
	if err != nil {
		return nil, rpcError(err)
	}

	saved := exercise.Info()
	return &exercisemsg.UpdateExerciseResponse{
		Exercise: s.toProtoExercise(&saved),
	}, nil
}

func (s *ExerciseServer) ApproveExercise(
	ctx context.Context,
	req *exercisemsg.ApproveExerciseRequest,
) (*exercisemsg.ApproveExerciseResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	exercise, err := s.approveHandler.Handle(ctx, command.ApproveExerciseCommand{ID: req.GetId()})
	if err != nil {
		return nil, rpcError(err)
	}

	info := exercise.Info()
	return &exercisemsg.ApproveExerciseResponse{
		Exercise: s.toProtoExercise(&info),
	}, nil
}

func (s *ExerciseServer) DeleteExercise(
	ctx context.Context,
	req *exercisemsg.DeleteExerciseRequest,
) (*exercisemsg.DeleteExerciseResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	err := s.archiveHandler.Handle(
		ctx,
		command.ArchiveExerciseCommand{ID: req.GetId()},
	)
	if err != nil {
		return nil, rpcError(err)
	}

	return &exercisemsg.DeleteExerciseResponse{Success: true}, nil
}

func (s *ExerciseServer) RestoreExercise(
	ctx context.Context,
	req *exercisemsg.RestoreExerciseRequest,
) (*exercisemsg.RestoreExerciseResponse, error) {
	if req == nil || req.GetId() == "" {
		return nil, status.Error(codes.InvalidArgument, "exercise id is required")
	}

	exercise, err := s.restoreHandler.Handle(ctx, command.RestoreExerciseCommand{
		ID: req.GetId(),
	})
	if err != nil {
		return nil, rpcError(err)
	}

	info := exercise.Info()
	return &exercisemsg.RestoreExerciseResponse{
		Exercise: s.toProtoExercise(&info),
	}, nil
}

// exerciseRequest is the editable field set shared by create and update.
type exerciseRequest interface {
	GetName() string
	GetPrimaryMuscle() exercisemsg.MuscleGroup
	GetSecondaryMuscles() []exercisemsg.MuscleGroup
	GetEquipment() exercisemsg.Equipment
	GetCategory() exercisemsg.ExerciseCategory
	GetLevel() exercisemsg.ExerciseLevel
	GetForce() exercisemsg.ExerciseForce
	GetMechanic() exercisemsg.ExerciseMechanic
	GetInstructions() []string
	GetImages() []string
	GetVideoUrl() string
	GetDefaultRestSeconds() int32
	GetMotionSlug() string
	GetMeasurementType() exercisemsg.ExerciseMeasurementType
}

// infoFromRequest validates the taxonomy enums (§6.7): UNSPECIFIED or unknown
// values are rejected, except force and mechanic where UNSPECIFIED means none.
func infoFromRequest(req exerciseRequest) (domain.Info, error) {
	primary, err := protomap.MuscleGroupFromProto(req.GetPrimaryMuscle())
	if err != nil {
		return domain.Info{}, err
	}
	secondary, err := protomap.MuscleGroupsFromProto(req.GetSecondaryMuscles())
	if err != nil {
		return domain.Info{}, err
	}
	equipment, err := protomap.EquipmentFromProto(req.GetEquipment())
	if err != nil {
		return domain.Info{}, err
	}
	category, err := protomap.CategoryFromProto(req.GetCategory())
	if err != nil {
		return domain.Info{}, err
	}
	level, err := protomap.LevelFromProto(req.GetLevel())
	if err != nil {
		return domain.Info{}, err
	}
	force, err := protomap.OptionalForceFromProto(req.GetForce())
	if err != nil {
		return domain.Info{}, err
	}
	mechanic, err := protomap.OptionalMechanicFromProto(req.GetMechanic())
	if err != nil {
		return domain.Info{}, err
	}

	return domain.Info{
		Name:               req.GetName(),
		PrimaryMuscle:      primary,
		SecondaryMuscles:   secondary,
		Equipment:          equipment,
		Category:           category,
		Level:              level,
		Force:              force,
		Mechanic:           mechanic,
		Instructions:       req.GetInstructions(),
		Images:             req.GetImages(),
		VideoURL:           req.GetVideoUrl(),
		DefaultRestSeconds: req.GetDefaultRestSeconds(),
		MotionSlug:         req.GetMotionSlug(),
		MeasurementType:    toDomainMeasurementType(req.GetMeasurementType()),
	}, nil
}

// toSearchFilters rejects UNSPECIFIED or unknown values in any filter list;
// force and mechanic filter only when set.
func toSearchFilters(req *exercisemsg.SearchExercisesRequest) (*port.SearchFilters, error) {
	primary, err := protomap.MuscleGroupsFromProto(req.GetPrimaryMuscles())
	if err != nil {
		return nil, err
	}
	secondary, err := protomap.MuscleGroupsFromProto(req.GetSecondaryMuscles())
	if err != nil {
		return nil, err
	}
	excluded, err := protomap.MuscleGroupsFromProto(req.GetExcludeMuscles())
	if err != nil {
		return nil, err
	}
	equipment, err := protomap.EquipmentListFromProto(req.GetEquipment())
	if err != nil {
		return nil, err
	}
	categories, err := protomap.CategoriesFromProto(req.GetCategories())
	if err != nil {
		return nil, err
	}
	levels, err := protomap.LevelsFromProto(req.GetLevels())
	if err != nil {
		return nil, err
	}

	filters := &port.SearchFilters{
		PrimaryMuscles:     primary,
		SecondaryMuscles:   secondary,
		Equipment:          equipment,
		Categories:         categories,
		Levels:             levels,
		ExcludeMuscles:     excluded,
		HasAISupported:     req.HasAiSupported,
		Keyword:            req.GetKeyword(),
		Limit:              req.GetLimit(),
		Offset:             req.GetOffset(),
		MeasurementType:    toDomainMeasurementTypeString(req.GetMeasurementType()),
		Status:             toDomainStatus(req.GetStatus()),
		IncludeAllStatuses: req.GetIncludeAllStatuses(),
		HasMotionSpec:      req.HasMotionSpec,
		HasVoiceSpec:       req.HasVoiceSpec,
	}
	if req.Force != nil {
		if filters.Force, err = protomap.ForceFromProto(req.GetForce()); err != nil {
			return nil, err
		}
	}
	if req.Mechanic != nil {
		if filters.Mechanic, err = protomap.MechanicFromProto(req.GetMechanic()); err != nil {
			return nil, err
		}
	}

	return filters, nil
}

func (s *ExerciseServer) toProtoExercise(info *domain.Info) *exercisemsg.ExerciseInfo {
	images := make([]string, len(info.Images))
	for i, image := range info.Images {
		images[i] = resolveMediaURL(s.mediaBaseURL, image)
	}

	return &exercisemsg.ExerciseInfo{
		Id:                 info.ID,
		Slug:               info.Slug,
		Name:               info.Name,
		PrimaryMuscle:      protomap.MuscleGroupToProto(info.PrimaryMuscle),
		SecondaryMuscles:   protomap.MuscleGroupsToProto(info.SecondaryMuscles),
		Equipment:          protomap.EquipmentToProto(info.Equipment),
		Category:           protomap.CategoryToProto(info.Category),
		Level:              protomap.LevelToProto(info.Level),
		Force:              protomap.ForceToProto(info.Force),
		Mechanic:           protomap.MechanicToProto(info.Mechanic),
		Instructions:       info.Instructions,
		Images:             images,
		VideoUrl:           info.VideoURL,
		DefaultRestSeconds: info.DefaultRestSeconds,
		MotionSlug:         info.MotionSlug,
		Status:             toProtoStatus(info.Status),
		HasAiSupported:     info.HasAISupported,
		MeasurementType:    toProtoMeasurementType(info.MeasurementType),
	}
}

// resolveMediaURL joins a relative image path to the media base URL; absolute
// URLs pass through.
func resolveMediaURL(baseURL, path string) string {
	if baseURL == "" || strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}

	return strings.TrimRight(baseURL, "/") + "/" + strings.TrimLeft(path, "/")
}

func toDomainStatus(s exercisemsg.ExerciseStatus) domain.Status {
	switch s {
	case exercisemsg.ExerciseStatus_EXERCISE_STATUS_DRAFT:
		return domain.StatusDraft
	case exercisemsg.ExerciseStatus_EXERCISE_STATUS_ACTIVE:
		return domain.StatusActive
	case exercisemsg.ExerciseStatus_EXERCISE_STATUS_ARCHIVED:
		return domain.StatusArchived
	default:
		return ""
	}
}

func toProtoStatus(status domain.Status) exercisemsg.ExerciseStatus {
	switch status {
	case domain.StatusDraft:
		return exercisemsg.ExerciseStatus_EXERCISE_STATUS_DRAFT
	case domain.StatusActive:
		return exercisemsg.ExerciseStatus_EXERCISE_STATUS_ACTIVE
	case domain.StatusArchived:
		return exercisemsg.ExerciseStatus_EXERCISE_STATUS_ARCHIVED
	default:
		return exercisemsg.ExerciseStatus_EXERCISE_STATUS_UNSPECIFIED
	}
}

func toProtoMeasurementType(m domain.MeasurementType) exercisemsg.ExerciseMeasurementType {
	switch m {
	case domain.MeasurementTypeReps:
		return exercisemsg.ExerciseMeasurementType_EXERCISE_MEASUREMENT_TYPE_REPS
	case domain.MeasurementTypeDuration:
		return exercisemsg.ExerciseMeasurementType_EXERCISE_MEASUREMENT_TYPE_DURATION
	default:
		return exercisemsg.ExerciseMeasurementType_EXERCISE_MEASUREMENT_TYPE_UNSPECIFIED
	}
}

func toDomainMeasurementType(m exercisemsg.ExerciseMeasurementType) domain.MeasurementType {
	switch m {
	case exercisemsg.ExerciseMeasurementType_EXERCISE_MEASUREMENT_TYPE_DURATION:
		return domain.MeasurementTypeDuration
	case exercisemsg.ExerciseMeasurementType_EXERCISE_MEASUREMENT_TYPE_REPS:
		return domain.MeasurementTypeReps
	default:
		return ""
	}
}

func toDomainMeasurementTypeString(m exercisemsg.ExerciseMeasurementType) string {
	switch m {
	case exercisemsg.ExerciseMeasurementType_EXERCISE_MEASUREMENT_TYPE_DURATION:
		return string(domain.MeasurementTypeDuration)
	case exercisemsg.ExerciseMeasurementType_EXERCISE_MEASUREMENT_TYPE_REPS:
		return string(domain.MeasurementTypeReps)
	default:
		return ""
	}
}

// Body Part RPCs
func rpcError(err error) error {
	switch {
	case errors.Is(err, middleware.ErrUnauthorized):
		return status.Error(codes.Unauthenticated, err.Error())
	case errors.Is(err, middleware.ErrForbidden):
		return status.Error(codes.PermissionDenied, err.Error())
	case errors.Is(err, domain.ErrExerciseNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, domain.ErrDuplicateSlug):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, domain.ErrMotionSlugRequired):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, domain.ErrInvalidExercise),
		errors.Is(err, domain.ErrInvalidStatus),
		errors.Is(err, domain.ErrInvalidTransition),
		errors.Is(err, domain.ErrArchivedExercise):
		return status.Error(codes.InvalidArgument, err.Error())
	default:
		return status.Error(codes.Internal, err.Error())
	}
}

// --- ConnectRPC Adapter ---

type ConnectExerciseHandler struct {
	server *ExerciseServer
}

var _ exercisev1serviceconnect.ExerciseServiceHandler = (*ConnectExerciseHandler)(nil)

func NewConnectExerciseHandler(server *ExerciseServer) exercisev1serviceconnect.ExerciseServiceHandler {
	return &ConnectExerciseHandler{server: server}
}

func (c *ConnectExerciseHandler) SearchExercises(
	ctx context.Context,
	req *exercisemsg.SearchExercisesRequest,
) (*exercisemsg.SearchExercisesResponse, error) {
	res, err := c.server.SearchExercises(ctx, req)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (c *ConnectExerciseHandler) GetExercise(
	ctx context.Context,
	req *exercisemsg.GetExerciseRequest,
) (*exercisemsg.GetExerciseResponse, error) {
	res, err := c.server.GetExercise(ctx, req)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (c *ConnectExerciseHandler) GetRandomExercise(
	ctx context.Context,
	req *exercisemsg.GetRandomExerciseRequest,
) (*exercisemsg.GetRandomExerciseResponse, error) {
	res, err := c.server.GetRandomExercise(ctx, req)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (c *ConnectExerciseHandler) CreateExercise(
	ctx context.Context,
	req *exercisemsg.CreateExerciseRequest,
) (*exercisemsg.CreateExerciseResponse, error) {
	res, err := c.server.CreateExercise(ctx, req)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (c *ConnectExerciseHandler) UpdateExercise(
	ctx context.Context,
	req *exercisemsg.UpdateExerciseRequest,
) (*exercisemsg.UpdateExerciseResponse, error) {
	res, err := c.server.UpdateExercise(ctx, req)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (c *ConnectExerciseHandler) ApproveExercise(
	ctx context.Context,
	req *exercisemsg.ApproveExerciseRequest,
) (*exercisemsg.ApproveExerciseResponse, error) {
	res, err := c.server.ApproveExercise(ctx, req)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (c *ConnectExerciseHandler) DeleteExercise(
	ctx context.Context,
	req *exercisemsg.DeleteExerciseRequest,
) (*exercisemsg.DeleteExerciseResponse, error) {
	res, err := c.server.DeleteExercise(ctx, req)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (c *ConnectExerciseHandler) RestoreExercise(
	ctx context.Context,
	req *exercisemsg.RestoreExerciseRequest,
) (*exercisemsg.RestoreExerciseResponse, error) {
	res, err := c.server.RestoreExercise(ctx, req)
	if err != nil {
		return nil, err
	}
	return res, nil
}
