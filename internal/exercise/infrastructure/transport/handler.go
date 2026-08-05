// Package transport contains RPC adapters for Exercise.
package transport

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	"github.com/viethung213/gym-companion/internal/exercise/application/command"
	"github.com/viethung213/gym-companion/internal/exercise/application/port"
	"github.com/viethung213/gym-companion/internal/exercise/application/query"
	"github.com/viethung213/gym-companion/internal/exercise/domain"
	exercisemsg "github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/exercise/v1/message"
	exercisesvc "github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/exercise/v1/service"
	"github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/exercise/v1/service/exercisev1serviceconnect"
	"github.com/viethung213/gym-companion/internal/shared/middleware"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ExerciseServer struct {
	exercisesvc.UnimplementedExerciseServiceServer

	createHandler            *command.CreateExerciseHandler
	updateHandler            *command.UpdateExerciseHandler
	submitForApprovalHandler *command.SubmitExerciseForApprovalHandler
	approveHandler           *command.ApproveExerciseHandler
	archiveHandler           *command.ArchiveExerciseHandler
	getHandler               *query.GetExerciseHandler
	searchHandler            *query.SearchExercisesHandler
	metadataHandler          *query.GetCatalogMetadataHandler

	createBodyPartHandler *command.CreateBodyPartHandler
	updateBodyPartHandler *command.UpdateBodyPartHandler
	deleteBodyPartHandler *command.DeleteBodyPartHandler
	getBodyPartHandler    *query.GetBodyPartHandler
	listBodyPartsHandler  *query.ListBodyPartsHandler

	createEquipmentHandler *command.CreateEquipmentHandler
	updateEquipmentHandler *command.UpdateEquipmentHandler
	deleteEquipmentHandler *command.DeleteEquipmentHandler
	getEquipmentHandler    *query.GetEquipmentHandler
	listEquipmentsHandler  *query.ListEquipmentsHandler

	createMuscleHandler *command.CreateMuscleHandler
	updateMuscleHandler *command.UpdateMuscleHandler
	deleteMuscleHandler *command.DeleteMuscleHandler
	getMuscleHandler    *query.GetMuscleHandler
	listMusclesHandler  *query.ListMusclesHandler

	createTagHandler *command.CreateTagHandler
	updateTagHandler *command.UpdateTagHandler
	deleteTagHandler *command.DeleteTagHandler
	getTagHandler    *query.GetTagHandler
	listTagsHandler  *query.ListTagsHandler
}

var _ exercisesvc.ExerciseServiceServer = (*ExerciseServer)(nil)

func NewExerciseServer(
	createHandler *command.CreateExerciseHandler,
	updateHandler *command.UpdateExerciseHandler,
	submitForApprovalHandler *command.SubmitExerciseForApprovalHandler,
	approveHandler *command.ApproveExerciseHandler,
	archiveHandler *command.ArchiveExerciseHandler,
	getHandler *query.GetExerciseHandler,
	searchHandler *query.SearchExercisesHandler,
	metadataHandler *query.GetCatalogMetadataHandler,
	createBodyPartHandler *command.CreateBodyPartHandler,
	updateBodyPartHandler *command.UpdateBodyPartHandler,
	deleteBodyPartHandler *command.DeleteBodyPartHandler,
	getBodyPartHandler *query.GetBodyPartHandler,
	listBodyPartsHandler *query.ListBodyPartsHandler,
	createEquipmentHandler *command.CreateEquipmentHandler,
	updateEquipmentHandler *command.UpdateEquipmentHandler,
	deleteEquipmentHandler *command.DeleteEquipmentHandler,
	getEquipmentHandler *query.GetEquipmentHandler,
	listEquipmentsHandler *query.ListEquipmentsHandler,
	createMuscleHandler *command.CreateMuscleHandler,
	updateMuscleHandler *command.UpdateMuscleHandler,
	deleteMuscleHandler *command.DeleteMuscleHandler,
	getMuscleHandler *query.GetMuscleHandler,
	listMusclesHandler *query.ListMusclesHandler,
	createTagHandler *command.CreateTagHandler,
	updateTagHandler *command.UpdateTagHandler,
	deleteTagHandler *command.DeleteTagHandler,
	getTagHandler *query.GetTagHandler,
	listTagsHandler *query.ListTagsHandler,
) *ExerciseServer {
	return &ExerciseServer{
		createHandler:            createHandler,
		updateHandler:            updateHandler,
		submitForApprovalHandler: submitForApprovalHandler,
		approveHandler:           approveHandler,
		archiveHandler:           archiveHandler,
		getHandler:               getHandler,
		searchHandler:            searchHandler,
		metadataHandler:          metadataHandler,
		createBodyPartHandler:    createBodyPartHandler,
		updateBodyPartHandler:    updateBodyPartHandler,
		deleteBodyPartHandler:    deleteBodyPartHandler,
		getBodyPartHandler:       getBodyPartHandler,
		listBodyPartsHandler:     listBodyPartsHandler,
		createEquipmentHandler:   createEquipmentHandler,
		updateEquipmentHandler:   updateEquipmentHandler,
		deleteEquipmentHandler:   deleteEquipmentHandler,
		getEquipmentHandler:      getEquipmentHandler,
		listEquipmentsHandler:    listEquipmentsHandler,
		createMuscleHandler:      createMuscleHandler,
		updateMuscleHandler:      updateMuscleHandler,
		deleteMuscleHandler:      deleteMuscleHandler,
		getMuscleHandler:         getMuscleHandler,
		listMusclesHandler:       listMusclesHandler,
		createTagHandler:         createTagHandler,
		updateTagHandler:         updateTagHandler,
		deleteTagHandler:         deleteTagHandler,
		getTagHandler:            getTagHandler,
		listTagsHandler:          listTagsHandler,
	}
}

func (s *ExerciseServer) SearchExercises(
	ctx context.Context,
	req *exercisemsg.SearchExercisesRequest,
) (*exercisemsg.SearchExercisesResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	exercises, err := s.searchHandler.Handle(ctx, query.SearchExercisesQuery{
		Filters: &port.SearchFilters{
			BodyPartID:         req.GetBodyPartId(),
			EquipmentID:        req.GetEquipmentId(),
			TargetMuscleID:     req.GetTargetMuscleId(),
			SecondaryMuscleIDs: req.GetSecondaryMuscleIds(),
			TagIDs:             req.GetTagIds(),
			Keyword:            req.GetKeyword(),
			Difficulty:         req.GetDifficulty(),
			Limit:              req.GetLimit(),
			Offset:             req.GetOffset(),
		},
	})
	if err != nil {
		return nil, rpcError(err)
	}

	response := &exercisemsg.SearchExercisesResponse{
		Exercises: make([]*exercisemsg.ExerciseInfo, 0, len(exercises)),
	}
	for _, exercise := range exercises {
		info := exercise.Info()
		response.Exercises = append(response.Exercises, toProtoExercise(&info))
	}

	return response, nil
}

func (s *ExerciseServer) GetCatalogMetadata(
	ctx context.Context,
	_ *exercisemsg.GetCatalogMetadataRequest,
) (*exercisemsg.GetCatalogMetadataResponse, error) {
	metadata, err := s.metadataHandler.Handle(ctx, query.GetCatalogMetadataQuery{})
	if err != nil {
		return nil, rpcError(err)
	}

	return toProtoMetadata(&metadata), nil
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
		Exercise: toProtoExercise(&info),
	}, nil
}

func (s *ExerciseServer) CreateExercise(
	ctx context.Context,
	req *exercisemsg.CreateExerciseRequest,
) (*exercisemsg.CreateExerciseResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	exercise, err := s.createHandler.Handle(ctx, &command.CreateExerciseCommand{
		Info: domain.Info{
			Name:               req.GetName(),
			BodyPartID:         req.GetBodyPartId(),
			EquipmentID:        req.GetEquipmentId(),
			TargetMuscleID:     req.GetTargetMuscleId(),
			Instructions:       req.GetInstructions(),
			SecondaryMuscleIDs: req.GetSecondaryMuscleIds(),
			ThumbnailURL:       req.GetThumbnailUrl(),
			MediaURL:           req.GetMediaUrl(),
			VideoURL:           req.GetVideoUrl(),
			Difficulty:         req.GetDifficulty(),
			DefaultRestSeconds: req.GetDefaultRestSeconds(),
			TagIDs:             req.GetTagIds(),
		},
	})
	if err != nil {
		return nil, rpcError(err)
	}

	info := exercise.Info()
	return &exercisemsg.CreateExerciseResponse{
		Exercise: toProtoExercise(&info),
	}, nil
}

func (s *ExerciseServer) UpdateExercise(
	ctx context.Context,
	req *exercisemsg.UpdateExerciseRequest,
) (*exercisemsg.UpdateExerciseResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	exercise, err := s.updateHandler.Handle(ctx, &command.UpdateExerciseCommand{
		ID: req.GetId(),
		Info: domain.Info{
			Name:               req.GetName(),
			BodyPartID:         req.GetBodyPartId(),
			EquipmentID:        req.GetEquipmentId(),
			TargetMuscleID:     req.GetTargetMuscleId(),
			Instructions:       req.GetInstructions(),
			SecondaryMuscleIDs: req.GetSecondaryMuscleIds(),
			ThumbnailURL:       req.GetThumbnailUrl(),
			MediaURL:           req.GetMediaUrl(),
			VideoURL:           req.GetVideoUrl(),
			Difficulty:         req.GetDifficulty(),
			DefaultRestSeconds: req.GetDefaultRestSeconds(),
			TagIDs:             req.GetTagIds(),
		},
	})
	if err != nil {
		return nil, rpcError(err)
	}

	info := exercise.Info()
	return &exercisemsg.UpdateExerciseResponse{
		Exercise: toProtoExercise(&info),
	}, nil
}

func (s *ExerciseServer) SubmitExerciseForApproval(
	ctx context.Context,
	req *exercisemsg.SubmitExerciseForApprovalRequest,
) (*exercisemsg.SubmitExerciseForApprovalResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	exercise, err := s.submitForApprovalHandler.Handle(
		ctx,
		command.SubmitExerciseForApprovalCommand{ID: req.GetId()},
	)
	if err != nil {
		return nil, rpcError(err)
	}

	info := exercise.Info()
	return &exercisemsg.SubmitExerciseForApprovalResponse{
		Exercise: toProtoExercise(&info),
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
		Exercise: toProtoExercise(&info),
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

func toProtoExercise(info *domain.Info) *exercisemsg.ExerciseInfo {
	return &exercisemsg.ExerciseInfo{
		Id:                 info.ID,
		Name:               info.Name,
		BodyPartId:         info.BodyPartID,
		EquipmentId:        info.EquipmentID,
		TargetMuscleId:     info.TargetMuscleID,
		Instructions:       info.Instructions,
		SecondaryMuscleIds: info.SecondaryMuscleIDs,
		ThumbnailUrl:       info.ThumbnailURL,
		MediaUrl:           info.MediaURL,
		VideoUrl:           info.VideoURL,
		Difficulty:         info.Difficulty,
		DefaultRestSeconds: info.DefaultRestSeconds,
		TagIds:             info.TagIDs,
		Status:             toProtoStatus(info.Status),
		HasAiSupported:     info.HasAISupported,
	}
}

func toProtoMetadata(metadata *port.Metadata) *exercisemsg.GetCatalogMetadataResponse {
	response := &exercisemsg.GetCatalogMetadataResponse{
		BodyParts:  make([]*exercisemsg.BodyPart, 0, len(metadata.BodyParts)),
		Equipments: make([]*exercisemsg.Equipment, 0, len(metadata.Equipments)),
		Muscles:    make([]*exercisemsg.Muscle, 0, len(metadata.Muscles)),
		Tags:       make([]*exercisemsg.Tag, 0, len(metadata.Tags)),
	}
	for _, bodyPart := range metadata.BodyParts {
		response.BodyParts = append(response.BodyParts, &exercisemsg.BodyPart{
			Id:   bodyPart.ID,
			Name: bodyPart.Name,
		})
	}
	for _, equipment := range metadata.Equipments {
		response.Equipments = append(response.Equipments, &exercisemsg.Equipment{
			Id:   equipment.ID,
			Name: equipment.Name,
		})
	}
	for _, muscle := range metadata.Muscles {
		response.Muscles = append(response.Muscles, &exercisemsg.Muscle{
			Id:         muscle.ID,
			Name:       muscle.Name,
			BodyPartId: muscle.BodyPartID,
		})
	}
	for _, tag := range metadata.Tags {
		response.Tags = append(response.Tags, &exercisemsg.Tag{
			Id:   tag.ID,
			Name: tag.Name,
		})
	}

	return response
}

func toProtoStatus(status domain.Status) exercisemsg.ExerciseStatus {
	switch status {
	case domain.StatusDraft:
		return exercisemsg.ExerciseStatus_EXERCISE_STATUS_DRAFT
	case domain.StatusPendingApproval:
		return exercisemsg.ExerciseStatus_EXERCISE_STATUS_PENDING_APPROVAL
	case domain.StatusActive:
		return exercisemsg.ExerciseStatus_EXERCISE_STATUS_ACTIVE
	case domain.StatusArchived:
		return exercisemsg.ExerciseStatus_EXERCISE_STATUS_ARCHIVED
	default:
		return exercisemsg.ExerciseStatus_EXERCISE_STATUS_UNSPECIFIED
	}
}

// Body Part RPCs
func (s *ExerciseServer) CreateBodyPart(ctx context.Context, req *exercisemsg.CreateBodyPartRequest) (*exercisemsg.CreateBodyPartResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	bp, err := s.createBodyPartHandler.Handle(ctx, &command.CreateBodyPartCommand{Name: req.GetName()})
	if err != nil {
		return nil, rpcError(err)
	}
	return &exercisemsg.CreateBodyPartResponse{BodyPart: toProtoBodyPart(bp)}, nil
}

func (s *ExerciseServer) GetBodyPart(ctx context.Context, req *exercisemsg.GetBodyPartRequest) (*exercisemsg.GetBodyPartResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	bp, err := s.getBodyPartHandler.Handle(ctx, query.GetBodyPartQuery{ID: req.GetId()})
	if err != nil {
		return nil, rpcError(err)
	}
	return &exercisemsg.GetBodyPartResponse{BodyPart: toProtoBodyPart(bp)}, nil
}

func (s *ExerciseServer) ListBodyParts(ctx context.Context, req *exercisemsg.ListBodyPartsRequest) (*exercisemsg.ListBodyPartsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	bps, total, err := s.listBodyPartsHandler.Handle(ctx, query.ListBodyPartsQuery{
		Limit:  int(req.GetLimit()),
		Offset: int(req.GetOffset()),
	})
	if err != nil {
		return nil, rpcError(err)
	}
	pbBps := make([]*exercisemsg.BodyPart, len(bps))
	for i := range bps {
		pbBps[i] = toProtoBodyPart(&bps[i])
	}
	return &exercisemsg.ListBodyPartsResponse{BodyParts: pbBps, Total: int32(total)}, nil
}

func (s *ExerciseServer) UpdateBodyPart(ctx context.Context, req *exercisemsg.UpdateBodyPartRequest) (*exercisemsg.UpdateBodyPartResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	bp, err := s.updateBodyPartHandler.Handle(ctx, &command.UpdateBodyPartCommand{
		ID:   req.GetId(),
		Name: req.GetName(),
	})
	if err != nil {
		return nil, rpcError(err)
	}
	return &exercisemsg.UpdateBodyPartResponse{BodyPart: toProtoBodyPart(bp)}, nil
}

func (s *ExerciseServer) DeleteBodyPart(ctx context.Context, req *exercisemsg.DeleteBodyPartRequest) (*exercisemsg.DeleteBodyPartResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	err := s.deleteBodyPartHandler.Handle(ctx, &command.DeleteBodyPartCommand{ID: req.GetId()})
	if err != nil {
		return nil, rpcError(err)
	}
	return &exercisemsg.DeleteBodyPartResponse{Success: true}, nil
}

// Equipment RPCs
func (s *ExerciseServer) CreateEquipment(ctx context.Context, req *exercisemsg.CreateEquipmentRequest) (*exercisemsg.CreateEquipmentResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	eq, err := s.createEquipmentHandler.Handle(ctx, &command.CreateEquipmentCommand{Name: req.GetName()})
	if err != nil {
		return nil, rpcError(err)
	}
	return &exercisemsg.CreateEquipmentResponse{Equipment: toProtoEquipment(eq)}, nil
}

func (s *ExerciseServer) GetEquipment(ctx context.Context, req *exercisemsg.GetEquipmentRequest) (*exercisemsg.GetEquipmentResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	eq, err := s.getEquipmentHandler.Handle(ctx, query.GetEquipmentQuery{ID: req.GetId()})
	if err != nil {
		return nil, rpcError(err)
	}
	return &exercisemsg.GetEquipmentResponse{Equipment: toProtoEquipment(eq)}, nil
}

func (s *ExerciseServer) ListEquipments(ctx context.Context, req *exercisemsg.ListEquipmentsRequest) (*exercisemsg.ListEquipmentsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	eqs, total, err := s.listEquipmentsHandler.Handle(ctx, query.ListEquipmentsQuery{
		Limit:  int(req.GetLimit()),
		Offset: int(req.GetOffset()),
	})
	if err != nil {
		return nil, rpcError(err)
	}
	pbEqs := make([]*exercisemsg.Equipment, len(eqs))
	for i := range eqs {
		pbEqs[i] = toProtoEquipment(&eqs[i])
	}
	return &exercisemsg.ListEquipmentsResponse{Equipments: pbEqs, Total: int32(total)}, nil
}

func (s *ExerciseServer) UpdateEquipment(ctx context.Context, req *exercisemsg.UpdateEquipmentRequest) (*exercisemsg.UpdateEquipmentResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	eq, err := s.updateEquipmentHandler.Handle(ctx, &command.UpdateEquipmentCommand{
		ID:   req.GetId(),
		Name: req.GetName(),
	})
	if err != nil {
		return nil, rpcError(err)
	}
	return &exercisemsg.UpdateEquipmentResponse{Equipment: toProtoEquipment(eq)}, nil
}

func (s *ExerciseServer) DeleteEquipment(ctx context.Context, req *exercisemsg.DeleteEquipmentRequest) (*exercisemsg.DeleteEquipmentResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	err := s.deleteEquipmentHandler.Handle(ctx, &command.DeleteEquipmentCommand{ID: req.GetId()})
	if err != nil {
		return nil, rpcError(err)
	}
	return &exercisemsg.DeleteEquipmentResponse{Success: true}, nil
}

// Muscle RPCs
func (s *ExerciseServer) CreateMuscle(ctx context.Context, req *exercisemsg.CreateMuscleRequest) (*exercisemsg.CreateMuscleResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	m, err := s.createMuscleHandler.Handle(ctx, &command.CreateMuscleCommand{
		Name:       req.GetName(),
		BodyPartID: req.GetBodyPartId(),
	})
	if err != nil {
		return nil, rpcError(err)
	}
	return &exercisemsg.CreateMuscleResponse{Muscle: toProtoMuscle(m)}, nil
}

func (s *ExerciseServer) GetMuscle(ctx context.Context, req *exercisemsg.GetMuscleRequest) (*exercisemsg.GetMuscleResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	m, err := s.getMuscleHandler.Handle(ctx, query.GetMuscleQuery{ID: req.GetId()})
	if err != nil {
		return nil, rpcError(err)
	}
	return &exercisemsg.GetMuscleResponse{Muscle: toProtoMuscle(m)}, nil
}

func (s *ExerciseServer) ListMuscles(ctx context.Context, req *exercisemsg.ListMusclesRequest) (*exercisemsg.ListMusclesResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	ms, total, err := s.listMusclesHandler.Handle(ctx, query.ListMusclesQuery{
		BodyPartID: req.GetBodyPartId(),
		Limit:      int(req.GetLimit()),
		Offset:     int(req.GetOffset()),
	})
	if err != nil {
		return nil, rpcError(err)
	}
	pbMs := make([]*exercisemsg.Muscle, len(ms))
	for i := range ms {
		pbMs[i] = toProtoMuscle(&ms[i])
	}
	return &exercisemsg.ListMusclesResponse{Muscles: pbMs, Total: int32(total)}, nil
}

func (s *ExerciseServer) UpdateMuscle(ctx context.Context, req *exercisemsg.UpdateMuscleRequest) (*exercisemsg.UpdateMuscleResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	m, err := s.updateMuscleHandler.Handle(ctx, &command.UpdateMuscleCommand{
		ID:         req.GetId(),
		Name:       req.GetName(),
		BodyPartID: req.GetBodyPartId(),
	})
	if err != nil {
		return nil, rpcError(err)
	}
	return &exercisemsg.UpdateMuscleResponse{Muscle: toProtoMuscle(m)}, nil
}

func (s *ExerciseServer) DeleteMuscle(ctx context.Context, req *exercisemsg.DeleteMuscleRequest) (*exercisemsg.DeleteMuscleResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	err := s.deleteMuscleHandler.Handle(ctx, &command.DeleteMuscleCommand{ID: req.GetId()})
	if err != nil {
		return nil, rpcError(err)
	}
	return &exercisemsg.DeleteMuscleResponse{Success: true}, nil
}

// Tag RPCs
func (s *ExerciseServer) CreateTag(ctx context.Context, req *exercisemsg.CreateTagRequest) (*exercisemsg.CreateTagResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	t, err := s.createTagHandler.Handle(ctx, &command.CreateTagCommand{Name: req.GetName()})
	if err != nil {
		return nil, rpcError(err)
	}
	return &exercisemsg.CreateTagResponse{Tag: toProtoTag(t)}, nil
}

func (s *ExerciseServer) GetTag(ctx context.Context, req *exercisemsg.GetTagRequest) (*exercisemsg.GetTagResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	t, err := s.getTagHandler.Handle(ctx, query.GetTagQuery{ID: req.GetId()})
	if err != nil {
		return nil, rpcError(err)
	}
	return &exercisemsg.GetTagResponse{Tag: toProtoTag(t)}, nil
}

func (s *ExerciseServer) ListTags(ctx context.Context, req *exercisemsg.ListTagsRequest) (*exercisemsg.ListTagsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	ts, total, err := s.listTagsHandler.Handle(ctx, query.ListTagsQuery{
		Limit:  int(req.GetLimit()),
		Offset: int(req.GetOffset()),
	})
	if err != nil {
		return nil, rpcError(err)
	}
	pbTs := make([]*exercisemsg.Tag, len(ts))
	for i := range ts {
		pbTs[i] = toProtoTag(&ts[i])
	}
	return &exercisemsg.ListTagsResponse{Tags: pbTs, Total: int32(total)}, nil
}

func (s *ExerciseServer) UpdateTag(ctx context.Context, req *exercisemsg.UpdateTagRequest) (*exercisemsg.UpdateTagResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	t, err := s.updateTagHandler.Handle(ctx, &command.UpdateTagCommand{ID: req.GetId(), Name: req.GetName()})
	if err != nil {
		return nil, rpcError(err)
	}
	return &exercisemsg.UpdateTagResponse{Tag: toProtoTag(t)}, nil
}

func (s *ExerciseServer) DeleteTag(ctx context.Context, req *exercisemsg.DeleteTagRequest) (*exercisemsg.DeleteTagResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	err := s.deleteTagHandler.Handle(ctx, &command.DeleteTagCommand{ID: req.GetId()})
	if err != nil {
		return nil, rpcError(err)
	}
	return &exercisemsg.DeleteTagResponse{Success: true}, nil
}

func toProtoBodyPart(bp *port.BodyPart) *exercisemsg.BodyPart {
	return &exercisemsg.BodyPart{Id: bp.ID, Name: bp.Name}
}

func toProtoEquipment(eq *port.Equipment) *exercisemsg.Equipment {
	return &exercisemsg.Equipment{Id: eq.ID, Name: eq.Name}
}

func toProtoMuscle(m *port.Muscle) *exercisemsg.Muscle {
	return &exercisemsg.Muscle{Id: m.ID, Name: m.Name, BodyPartId: m.BodyPartID}
}

func toProtoTag(t *port.Tag) *exercisemsg.Tag {
	return &exercisemsg.Tag{Id: t.ID, Name: t.Name}
}

func rpcError(err error) error {
	switch {
	case errors.Is(err, middleware.ErrUnauthorized):
		return status.Error(codes.Unauthenticated, err.Error())
	case errors.Is(err, middleware.ErrForbidden):
		return status.Error(codes.PermissionDenied, err.Error())
	case errors.Is(err, domain.ErrExerciseNotFound),
		errors.Is(err, domain.ErrBodyPartNotFound),
		errors.Is(err, domain.ErrEquipmentNotFound),
		errors.Is(err, domain.ErrMuscleNotFound),
		errors.Is(err, domain.ErrTagNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, domain.ErrInvalidExercise),
		errors.Is(err, domain.ErrInvalidBodyPart),
		errors.Is(err, domain.ErrInvalidEquipment),
		errors.Is(err, domain.ErrInvalidMuscle),
		errors.Is(err, domain.ErrInvalidTag),
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
	req *connect.Request[exercisemsg.SearchExercisesRequest],
) (*connect.Response[exercisemsg.SearchExercisesResponse], error) {
	res, err := c.server.SearchExercises(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}

func (c *ConnectExerciseHandler) GetCatalogMetadata(
	ctx context.Context,
	req *connect.Request[exercisemsg.GetCatalogMetadataRequest],
) (*connect.Response[exercisemsg.GetCatalogMetadataResponse], error) {
	res, err := c.server.GetCatalogMetadata(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}

func (c *ConnectExerciseHandler) GetExercise(
	ctx context.Context,
	req *connect.Request[exercisemsg.GetExerciseRequest],
) (*connect.Response[exercisemsg.GetExerciseResponse], error) {
	res, err := c.server.GetExercise(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}

func (c *ConnectExerciseHandler) CreateExercise(
	ctx context.Context,
	req *connect.Request[exercisemsg.CreateExerciseRequest],
) (*connect.Response[exercisemsg.CreateExerciseResponse], error) {
	res, err := c.server.CreateExercise(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}

func (c *ConnectExerciseHandler) UpdateExercise(
	ctx context.Context,
	req *connect.Request[exercisemsg.UpdateExerciseRequest],
) (*connect.Response[exercisemsg.UpdateExerciseResponse], error) {
	res, err := c.server.UpdateExercise(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}

func (c *ConnectExerciseHandler) SubmitExerciseForApproval(
	ctx context.Context,
	req *connect.Request[exercisemsg.SubmitExerciseForApprovalRequest],
) (*connect.Response[exercisemsg.SubmitExerciseForApprovalResponse], error) {
	res, err := c.server.SubmitExerciseForApproval(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}

func (c *ConnectExerciseHandler) ApproveExercise(
	ctx context.Context,
	req *connect.Request[exercisemsg.ApproveExerciseRequest],
) (*connect.Response[exercisemsg.ApproveExerciseResponse], error) {
	res, err := c.server.ApproveExercise(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}

func (c *ConnectExerciseHandler) DeleteExercise(
	ctx context.Context,
	req *connect.Request[exercisemsg.DeleteExerciseRequest],
) (*connect.Response[exercisemsg.DeleteExerciseResponse], error) {
	res, err := c.server.DeleteExercise(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}

func (c *ConnectExerciseHandler) CreateBodyPart(
	ctx context.Context,
	req *connect.Request[exercisemsg.CreateBodyPartRequest],
) (*connect.Response[exercisemsg.CreateBodyPartResponse], error) {
	res, err := c.server.CreateBodyPart(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}

func (c *ConnectExerciseHandler) GetBodyPart(
	ctx context.Context,
	req *connect.Request[exercisemsg.GetBodyPartRequest],
) (*connect.Response[exercisemsg.GetBodyPartResponse], error) {
	res, err := c.server.GetBodyPart(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}

func (c *ConnectExerciseHandler) ListBodyParts(
	ctx context.Context,
	req *connect.Request[exercisemsg.ListBodyPartsRequest],
) (*connect.Response[exercisemsg.ListBodyPartsResponse], error) {
	res, err := c.server.ListBodyParts(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}

func (c *ConnectExerciseHandler) UpdateBodyPart(
	ctx context.Context,
	req *connect.Request[exercisemsg.UpdateBodyPartRequest],
) (*connect.Response[exercisemsg.UpdateBodyPartResponse], error) {
	res, err := c.server.UpdateBodyPart(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}

func (c *ConnectExerciseHandler) DeleteBodyPart(
	ctx context.Context,
	req *connect.Request[exercisemsg.DeleteBodyPartRequest],
) (*connect.Response[exercisemsg.DeleteBodyPartResponse], error) {
	res, err := c.server.DeleteBodyPart(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}

func (c *ConnectExerciseHandler) CreateEquipment(
	ctx context.Context,
	req *connect.Request[exercisemsg.CreateEquipmentRequest],
) (*connect.Response[exercisemsg.CreateEquipmentResponse], error) {
	res, err := c.server.CreateEquipment(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}

func (c *ConnectExerciseHandler) GetEquipment(
	ctx context.Context,
	req *connect.Request[exercisemsg.GetEquipmentRequest],
) (*connect.Response[exercisemsg.GetEquipmentResponse], error) {
	res, err := c.server.GetEquipment(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}

func (c *ConnectExerciseHandler) ListEquipments(
	ctx context.Context,
	req *connect.Request[exercisemsg.ListEquipmentsRequest],
) (*connect.Response[exercisemsg.ListEquipmentsResponse], error) {
	res, err := c.server.ListEquipments(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}

func (c *ConnectExerciseHandler) UpdateEquipment(
	ctx context.Context,
	req *connect.Request[exercisemsg.UpdateEquipmentRequest],
) (*connect.Response[exercisemsg.UpdateEquipmentResponse], error) {
	res, err := c.server.UpdateEquipment(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}

func (c *ConnectExerciseHandler) DeleteEquipment(
	ctx context.Context,
	req *connect.Request[exercisemsg.DeleteEquipmentRequest],
) (*connect.Response[exercisemsg.DeleteEquipmentResponse], error) {
	res, err := c.server.DeleteEquipment(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}

func (c *ConnectExerciseHandler) CreateMuscle(
	ctx context.Context,
	req *connect.Request[exercisemsg.CreateMuscleRequest],
) (*connect.Response[exercisemsg.CreateMuscleResponse], error) {
	res, err := c.server.CreateMuscle(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}

func (c *ConnectExerciseHandler) GetMuscle(
	ctx context.Context,
	req *connect.Request[exercisemsg.GetMuscleRequest],
) (*connect.Response[exercisemsg.GetMuscleResponse], error) {
	res, err := c.server.GetMuscle(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}

func (c *ConnectExerciseHandler) ListMuscles(
	ctx context.Context,
	req *connect.Request[exercisemsg.ListMusclesRequest],
) (*connect.Response[exercisemsg.ListMusclesResponse], error) {
	res, err := c.server.ListMuscles(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}

func (c *ConnectExerciseHandler) UpdateMuscle(
	ctx context.Context,
	req *connect.Request[exercisemsg.UpdateMuscleRequest],
) (*connect.Response[exercisemsg.UpdateMuscleResponse], error) {
	res, err := c.server.UpdateMuscle(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}

func (c *ConnectExerciseHandler) DeleteMuscle(
	ctx context.Context,
	req *connect.Request[exercisemsg.DeleteMuscleRequest],
) (*connect.Response[exercisemsg.DeleteMuscleResponse], error) {
	res, err := c.server.DeleteMuscle(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}

func (c *ConnectExerciseHandler) CreateTag(
	ctx context.Context,
	req *connect.Request[exercisemsg.CreateTagRequest],
) (*connect.Response[exercisemsg.CreateTagResponse], error) {
	res, err := c.server.CreateTag(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}

func (c *ConnectExerciseHandler) GetTag(
	ctx context.Context,
	req *connect.Request[exercisemsg.GetTagRequest],
) (*connect.Response[exercisemsg.GetTagResponse], error) {
	res, err := c.server.GetTag(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}

func (c *ConnectExerciseHandler) ListTags(
	ctx context.Context,
	req *connect.Request[exercisemsg.ListTagsRequest],
) (*connect.Response[exercisemsg.ListTagsResponse], error) {
	res, err := c.server.ListTags(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}

func (c *ConnectExerciseHandler) UpdateTag(
	ctx context.Context,
	req *connect.Request[exercisemsg.UpdateTagRequest],
) (*connect.Response[exercisemsg.UpdateTagResponse], error) {
	res, err := c.server.UpdateTag(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}

func (c *ConnectExerciseHandler) DeleteTag(
	ctx context.Context,
	req *connect.Request[exercisemsg.DeleteTagRequest],
) (*connect.Response[exercisemsg.DeleteTagResponse], error) {
	res, err := c.server.DeleteTag(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}
