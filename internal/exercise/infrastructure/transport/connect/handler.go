// Package connect provides the Connect-protocol transport handler for ExerciseService.
package connect

import (
	"context"

	"connectrpc.com/connect"

	exercisemsg "github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/exercise/v1/message"
	exercisev1serviceconnect "github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/exercise/v1/service/exercisev1serviceconnect"
	grpctransport "github.com/viethung213/gym-companion/internal/exercise/infrastructure/transport"
)

// Handler delegates Connect-protocol calls to the existing gRPC handler.
// Admin write methods fall through to UnimplementedExerciseServiceHandler (CodeUnimplemented).
type Handler struct {
	exercisev1serviceconnect.UnimplementedExerciseServiceHandler
	grpc *grpctransport.ExerciseServer
}

var _ exercisev1serviceconnect.ExerciseServiceHandler = (*Handler)(nil)

func NewHandler(grpc *grpctransport.ExerciseServer) *Handler {
	return &Handler{grpc: grpc}
}

func (h *Handler) SearchExercises(ctx context.Context, req *connect.Request[exercisemsg.SearchExercisesRequest]) (*connect.Response[exercisemsg.SearchExercisesResponse], error) {
	resp, err := h.grpc.SearchExercises(ctx, req.Msg)
	if err != nil {
		return nil, connect.NewError(connect.CodeOf(err), err)
	}
	return connect.NewResponse(resp), nil
}

func (h *Handler) GetCatalogMetadata(ctx context.Context, req *connect.Request[exercisemsg.GetCatalogMetadataRequest]) (*connect.Response[exercisemsg.GetCatalogMetadataResponse], error) {
	resp, err := h.grpc.GetCatalogMetadata(ctx, req.Msg)
	if err != nil {
		return nil, connect.NewError(connect.CodeOf(err), err)
	}
	return connect.NewResponse(resp), nil
}

func (h *Handler) GetExercise(ctx context.Context, req *connect.Request[exercisemsg.GetExerciseRequest]) (*connect.Response[exercisemsg.GetExerciseResponse], error) {
	resp, err := h.grpc.GetExercise(ctx, req.Msg)
	if err != nil {
		return nil, connect.NewError(connect.CodeOf(err), err)
	}
	return connect.NewResponse(resp), nil
}
