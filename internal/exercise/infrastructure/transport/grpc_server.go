// Package transport contains RPC adapters for Exercise.
package transport

import (
	"context"
	"errors"

	"github.com/viethung213/gym-companion/internal/exercise/application"
	"github.com/viethung213/gym-companion/internal/exercise/domain"
	exercisemsg "github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/exercise/v1/message"
	exercisesvc "github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/exercise/v1/service"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ExerciseServer struct {
	exercisesvc.UnimplementedExerciseServiceServer

	service *application.Service
}

var _ exercisesvc.ExerciseServiceServer = (*ExerciseServer)(nil)

func NewExerciseServer(service *application.Service) *ExerciseServer {
	return &ExerciseServer{service: service}
}

func (s *ExerciseServer) SearchExercises(
	ctx context.Context,
	req *exercisemsg.SearchExercisesRequest,
) (*exercisemsg.SearchExercisesResponse, error) {
	exercises, err := s.service.SearchExercises(ctx, &application.SearchFilters{
		BodyPartID:         req.GetBodyPartId(),
		EquipmentID:        req.GetEquipmentId(),
		TargetMuscleID:     req.GetTargetMuscleId(),
		SecondaryMuscleIDs: req.GetSecondaryMuscleIds(),
		TagIDs:             req.GetTagIds(),
		Keyword:            req.GetKeyword(),
		Difficulty:         req.GetDifficulty(),
		Limit:              req.GetLimit(),
		Offset:             req.GetOffset(),
	})
	if err != nil {
		return nil, rpcError(err)
	}

	response := &exercisemsg.SearchExercisesResponse{
		Exercises: make([]*exercisemsg.ExerciseInfo, 0, len(exercises)),
	}
	for _, exercise := range exercises {
		response.Exercises = append(response.Exercises, toProtoExercise(exercise.Info()))
	}

	return response, nil
}

func (s *ExerciseServer) GetCatalogMetadata(
	ctx context.Context,
	_ *exercisemsg.GetCatalogMetadataRequest,
) (*exercisemsg.GetCatalogMetadataResponse, error) {
	metadata, err := s.service.GetCatalogMetadata(ctx)
	if err != nil {
		return nil, rpcError(err)
	}

	return toProtoMetadata(&metadata), nil
}

func (s *ExerciseServer) GetExercise(
	ctx context.Context,
	req *exercisemsg.GetExerciseRequest,
) (*exercisemsg.GetExerciseResponse, error) {
	exercise, err := s.service.GetExercise(ctx, req.GetId())
	if err != nil {
		return nil, rpcError(err)
	}

	return &exercisemsg.GetExerciseResponse{
		Exercise: toProtoExercise(exercise.Info()),
	}, nil
}

func (s *ExerciseServer) CreateExercise(
	ctx context.Context,
	req *exercisemsg.CreateExerciseRequest,
) (*exercisemsg.CreateExerciseResponse, error) {
	exercise, err := s.service.CreateExercise(ctx, domain.Info{
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
	})
	if err != nil {
		return nil, rpcError(err)
	}

	return &exercisemsg.CreateExerciseResponse{
		Exercise: toProtoExercise(exercise.Info()),
	}, nil
}

func (s *ExerciseServer) UpdateExercise(
	ctx context.Context,
	req *exercisemsg.UpdateExerciseRequest,
) (*exercisemsg.UpdateExerciseResponse, error) {
	exercise, err := s.service.UpdateExercise(ctx, req.GetId(), domain.Info{
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
	})
	if err != nil {
		return nil, rpcError(err)
	}

	return &exercisemsg.UpdateExerciseResponse{
		Exercise: toProtoExercise(exercise.Info()),
	}, nil
}

func (s *ExerciseServer) SubmitExerciseForApproval(
	ctx context.Context,
	req *exercisemsg.SubmitExerciseForApprovalRequest,
) (*exercisemsg.SubmitExerciseForApprovalResponse, error) {
	exercise, err := s.service.SubmitExerciseForApproval(ctx, req.GetId())
	if err != nil {
		return nil, rpcError(err)
	}

	return &exercisemsg.SubmitExerciseForApprovalResponse{
		Exercise: toProtoExercise(exercise.Info()),
	}, nil
}

func (s *ExerciseServer) ApproveExercise(
	ctx context.Context,
	req *exercisemsg.ApproveExerciseRequest,
) (*exercisemsg.ApproveExerciseResponse, error) {
	exercise, err := s.service.ApproveExercise(ctx, req.GetId())
	if err != nil {
		return nil, rpcError(err)
	}

	return &exercisemsg.ApproveExerciseResponse{
		Exercise: toProtoExercise(exercise.Info()),
	}, nil
}

func (s *ExerciseServer) DeleteExercise(
	ctx context.Context,
	req *exercisemsg.DeleteExerciseRequest,
) (*exercisemsg.DeleteExerciseResponse, error) {
	if err := s.service.ArchiveExercise(ctx, req.GetId()); err != nil {
		return nil, rpcError(err)
	}

	return &exercisemsg.DeleteExerciseResponse{Success: true}, nil
}

//nolint:gocritic // info is a value object copy to protect aggregate encapsulation
func toProtoExercise(info domain.Info) *exercisemsg.ExerciseInfo {
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
	}
}

func toProtoMetadata(metadata *application.Metadata) *exercisemsg.GetCatalogMetadataResponse {
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

func rpcError(err error) error {
	switch {
	case errors.Is(err, domain.ErrUnauthorized):
		return status.Error(codes.Unauthenticated, err.Error())
	case errors.Is(err, domain.ErrForbidden):
		return status.Error(codes.PermissionDenied, err.Error())
	case errors.Is(err, domain.ErrExerciseNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, domain.ErrInvalidExercise),
		errors.Is(err, domain.ErrInvalidStatus),
		errors.Is(err, domain.ErrInvalidTransition),
		errors.Is(err, domain.ErrArchivedExercise):
		return status.Error(codes.InvalidArgument, err.Error())
	default:
		return status.Error(codes.Internal, err.Error())
	}
}
