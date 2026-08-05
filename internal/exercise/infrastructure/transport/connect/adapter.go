package connect

import (
	"context"

	"connectrpc.com/connect"
	exercisemsg "github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/exercise/v1/message"
	exercisev1serviceconnect "github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/exercise/v1/service/exercisev1serviceconnect"
	"github.com/viethung213/gym-companion/internal/shared/connectutil"
	"github.com/viethung213/gym-companion/internal/exercise/infrastructure/transport"
)

// Adapter wraps ExerciseServer to implement exercisev1serviceconnect.ExerciseServiceHandler.
type Adapter struct {
	exercisev1serviceconnect.UnimplementedExerciseServiceHandler
	grpc *transport.ExerciseServer
}

var _ exercisev1serviceconnect.ExerciseServiceHandler = (*Adapter)(nil)

// NewAdapter creates a new Connect adapter wrapping the existing gRPC server.
func NewAdapter(grpc *transport.ExerciseServer) *Adapter {
	return &Adapter{grpc: grpc}
}

func (a *Adapter) SearchExercises(
	ctx context.Context,
	req *connect.Request[exercisemsg.SearchExercisesRequest],
) (*connect.Response[exercisemsg.SearchExercisesResponse], error) {
	resp, err := a.grpc.SearchExercises(ctx, req.Msg)
	if err != nil {
		return nil, connectutil.FromGRPCError(err)
	}
	return connect.NewResponse(resp), nil
}

func (a *Adapter) GetCatalogMetadata(
	ctx context.Context,
	req *connect.Request[exercisemsg.GetCatalogMetadataRequest],
) (*connect.Response[exercisemsg.GetCatalogMetadataResponse], error) {
	resp, err := a.grpc.GetCatalogMetadata(ctx, req.Msg)
	if err != nil {
		return nil, connectutil.FromGRPCError(err)
	}
	return connect.NewResponse(resp), nil
}

func (a *Adapter) GetExercise(
	ctx context.Context,
	req *connect.Request[exercisemsg.GetExerciseRequest],
) (*connect.Response[exercisemsg.GetExerciseResponse], error) {
	resp, err := a.grpc.GetExercise(ctx, req.Msg)
	if err != nil {
		return nil, connectutil.FromGRPCError(err)
	}
	return connect.NewResponse(resp), nil
}

func (a *Adapter) CreateExercise(
	ctx context.Context,
	req *connect.Request[exercisemsg.CreateExerciseRequest],
) (*connect.Response[exercisemsg.CreateExerciseResponse], error) {
	resp, err := a.grpc.CreateExercise(ctx, req.Msg)
	if err != nil {
		return nil, connectutil.FromGRPCError(err)
	}
	return connect.NewResponse(resp), nil
}

func (a *Adapter) UpdateExercise(
	ctx context.Context,
	req *connect.Request[exercisemsg.UpdateExerciseRequest],
) (*connect.Response[exercisemsg.UpdateExerciseResponse], error) {
	resp, err := a.grpc.UpdateExercise(ctx, req.Msg)
	if err != nil {
		return nil, connectutil.FromGRPCError(err)
	}
	return connect.NewResponse(resp), nil
}

func (a *Adapter) SubmitExerciseForApproval(
	ctx context.Context,
	req *connect.Request[exercisemsg.SubmitExerciseForApprovalRequest],
) (*connect.Response[exercisemsg.SubmitExerciseForApprovalResponse], error) {
	resp, err := a.grpc.SubmitExerciseForApproval(ctx, req.Msg)
	if err != nil {
		return nil, connectutil.FromGRPCError(err)
	}
	return connect.NewResponse(resp), nil
}

func (a *Adapter) ApproveExercise(
	ctx context.Context,
	req *connect.Request[exercisemsg.ApproveExerciseRequest],
) (*connect.Response[exercisemsg.ApproveExerciseResponse], error) {
	resp, err := a.grpc.ApproveExercise(ctx, req.Msg)
	if err != nil {
		return nil, connectutil.FromGRPCError(err)
	}
	return connect.NewResponse(resp), nil
}

func (a *Adapter) DeleteExercise(
	ctx context.Context,
	req *connect.Request[exercisemsg.DeleteExerciseRequest],
) (*connect.Response[exercisemsg.DeleteExerciseResponse], error) {
	resp, err := a.grpc.DeleteExercise(ctx, req.Msg)
	if err != nil {
		return nil, connectutil.FromGRPCError(err)
	}
	return connect.NewResponse(resp), nil
}

func (a *Adapter) CreateBodyPart(
	ctx context.Context,
	req *connect.Request[exercisemsg.CreateBodyPartRequest],
) (*connect.Response[exercisemsg.CreateBodyPartResponse], error) {
	resp, err := a.grpc.CreateBodyPart(ctx, req.Msg)
	if err != nil {
		return nil, connectutil.FromGRPCError(err)
	}
	return connect.NewResponse(resp), nil
}

func (a *Adapter) GetBodyPart(
	ctx context.Context,
	req *connect.Request[exercisemsg.GetBodyPartRequest],
) (*connect.Response[exercisemsg.GetBodyPartResponse], error) {
	resp, err := a.grpc.GetBodyPart(ctx, req.Msg)
	if err != nil {
		return nil, connectutil.FromGRPCError(err)
	}
	return connect.NewResponse(resp), nil
}

func (a *Adapter) ListBodyParts(
	ctx context.Context,
	req *connect.Request[exercisemsg.ListBodyPartsRequest],
) (*connect.Response[exercisemsg.ListBodyPartsResponse], error) {
	resp, err := a.grpc.ListBodyParts(ctx, req.Msg)
	if err != nil {
		return nil, connectutil.FromGRPCError(err)
	}
	return connect.NewResponse(resp), nil
}

func (a *Adapter) UpdateBodyPart(
	ctx context.Context,
	req *connect.Request[exercisemsg.UpdateBodyPartRequest],
) (*connect.Response[exercisemsg.UpdateBodyPartResponse], error) {
	resp, err := a.grpc.UpdateBodyPart(ctx, req.Msg)
	if err != nil {
		return nil, connectutil.FromGRPCError(err)
	}
	return connect.NewResponse(resp), nil
}

func (a *Adapter) DeleteBodyPart(
	ctx context.Context,
	req *connect.Request[exercisemsg.DeleteBodyPartRequest],
) (*connect.Response[exercisemsg.DeleteBodyPartResponse], error) {
	resp, err := a.grpc.DeleteBodyPart(ctx, req.Msg)
	if err != nil {
		return nil, connectutil.FromGRPCError(err)
	}
	return connect.NewResponse(resp), nil
}

func (a *Adapter) CreateEquipment(
	ctx context.Context,
	req *connect.Request[exercisemsg.CreateEquipmentRequest],
) (*connect.Response[exercisemsg.CreateEquipmentResponse], error) {
	resp, err := a.grpc.CreateEquipment(ctx, req.Msg)
	if err != nil {
		return nil, connectutil.FromGRPCError(err)
	}
	return connect.NewResponse(resp), nil
}

func (a *Adapter) GetEquipment(
	ctx context.Context,
	req *connect.Request[exercisemsg.GetEquipmentRequest],
) (*connect.Response[exercisemsg.GetEquipmentResponse], error) {
	resp, err := a.grpc.GetEquipment(ctx, req.Msg)
	if err != nil {
		return nil, connectutil.FromGRPCError(err)
	}
	return connect.NewResponse(resp), nil
}

func (a *Adapter) ListEquipments(
	ctx context.Context,
	req *connect.Request[exercisemsg.ListEquipmentsRequest],
) (*connect.Response[exercisemsg.ListEquipmentsResponse], error) {
	resp, err := a.grpc.ListEquipments(ctx, req.Msg)
	if err != nil {
		return nil, connectutil.FromGRPCError(err)
	}
	return connect.NewResponse(resp), nil
}

func (a *Adapter) UpdateEquipment(
	ctx context.Context,
	req *connect.Request[exercisemsg.UpdateEquipmentRequest],
) (*connect.Response[exercisemsg.UpdateEquipmentResponse], error) {
	resp, err := a.grpc.UpdateEquipment(ctx, req.Msg)
	if err != nil {
		return nil, connectutil.FromGRPCError(err)
	}
	return connect.NewResponse(resp), nil
}

func (a *Adapter) DeleteEquipment(
	ctx context.Context,
	req *connect.Request[exercisemsg.DeleteEquipmentRequest],
) (*connect.Response[exercisemsg.DeleteEquipmentResponse], error) {
	resp, err := a.grpc.DeleteEquipment(ctx, req.Msg)
	if err != nil {
		return nil, connectutil.FromGRPCError(err)
	}
	return connect.NewResponse(resp), nil
}

func (a *Adapter) CreateMuscle(
	ctx context.Context,
	req *connect.Request[exercisemsg.CreateMuscleRequest],
) (*connect.Response[exercisemsg.CreateMuscleResponse], error) {
	resp, err := a.grpc.CreateMuscle(ctx, req.Msg)
	if err != nil {
		return nil, connectutil.FromGRPCError(err)
	}
	return connect.NewResponse(resp), nil
}

func (a *Adapter) GetMuscle(
	ctx context.Context,
	req *connect.Request[exercisemsg.GetMuscleRequest],
) (*connect.Response[exercisemsg.GetMuscleResponse], error) {
	resp, err := a.grpc.GetMuscle(ctx, req.Msg)
	if err != nil {
		return nil, connectutil.FromGRPCError(err)
	}
	return connect.NewResponse(resp), nil
}

func (a *Adapter) ListMuscles(
	ctx context.Context,
	req *connect.Request[exercisemsg.ListMusclesRequest],
) (*connect.Response[exercisemsg.ListMusclesResponse], error) {
	resp, err := a.grpc.ListMuscles(ctx, req.Msg)
	if err != nil {
		return nil, connectutil.FromGRPCError(err)
	}
	return connect.NewResponse(resp), nil
}

func (a *Adapter) UpdateMuscle(
	ctx context.Context,
	req *connect.Request[exercisemsg.UpdateMuscleRequest],
) (*connect.Response[exercisemsg.UpdateMuscleResponse], error) {
	resp, err := a.grpc.UpdateMuscle(ctx, req.Msg)
	if err != nil {
		return nil, connectutil.FromGRPCError(err)
	}
	return connect.NewResponse(resp), nil
}

func (a *Adapter) DeleteMuscle(
	ctx context.Context,
	req *connect.Request[exercisemsg.DeleteMuscleRequest],
) (*connect.Response[exercisemsg.DeleteMuscleResponse], error) {
	resp, err := a.grpc.DeleteMuscle(ctx, req.Msg)
	if err != nil {
		return nil, connectutil.FromGRPCError(err)
	}
	return connect.NewResponse(resp), nil
}

func (a *Adapter) CreateTag(
	ctx context.Context,
	req *connect.Request[exercisemsg.CreateTagRequest],
) (*connect.Response[exercisemsg.CreateTagResponse], error) {
	resp, err := a.grpc.CreateTag(ctx, req.Msg)
	if err != nil {
		return nil, connectutil.FromGRPCError(err)
	}
	return connect.NewResponse(resp), nil
}

func (a *Adapter) GetTag(
	ctx context.Context,
	req *connect.Request[exercisemsg.GetTagRequest],
) (*connect.Response[exercisemsg.GetTagResponse], error) {
	resp, err := a.grpc.GetTag(ctx, req.Msg)
	if err != nil {
		return nil, connectutil.FromGRPCError(err)
	}
	return connect.NewResponse(resp), nil
}

func (a *Adapter) ListTags(
	ctx context.Context,
	req *connect.Request[exercisemsg.ListTagsRequest],
) (*connect.Response[exercisemsg.ListTagsResponse], error) {
	resp, err := a.grpc.ListTags(ctx, req.Msg)
	if err != nil {
		return nil, connectutil.FromGRPCError(err)
	}
	return connect.NewResponse(resp), nil
}

func (a *Adapter) UpdateTag(
	ctx context.Context,
	req *connect.Request[exercisemsg.UpdateTagRequest],
) (*connect.Response[exercisemsg.UpdateTagResponse], error) {
	resp, err := a.grpc.UpdateTag(ctx, req.Msg)
	if err != nil {
		return nil, connectutil.FromGRPCError(err)
	}
	return connect.NewResponse(resp), nil
}

func (a *Adapter) DeleteTag(
	ctx context.Context,
	req *connect.Request[exercisemsg.DeleteTagRequest],
) (*connect.Response[exercisemsg.DeleteTagResponse], error) {
	resp, err := a.grpc.DeleteTag(ctx, req.Msg)
	if err != nil {
		return nil, connectutil.FromGRPCError(err)
	}
	return connect.NewResponse(resp), nil
}
