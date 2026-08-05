// Package connect provides the Connect-protocol transport handler for WorkoutExecutionService.
package connect

import (
	"context"

	"connectrpc.com/connect"

	workoutmsg "github.com/viethung213/gym-companion/internal/gen/go/contracts/core/workout_execution/v1/message"
	workoutv1serviceconnect "github.com/viethung213/gym-companion/internal/gen/go/contracts/core/workout_execution/v1/service/workoutexecutionv1serviceconnect"
	grpctransport "github.com/viethung213/gym-companion/internal/workout_execution/infrastructure/transport"
)

// Handler delegates Connect-protocol calls to the existing gRPC handler.
// Admin methods fall through to UnimplementedWorkoutExecutionServiceHandler (CodeUnimplemented).
type Handler struct {
	workoutv1serviceconnect.UnimplementedWorkoutExecutionServiceHandler
	grpc *grpctransport.GRPCHandler
}

var _ workoutv1serviceconnect.WorkoutExecutionServiceHandler = (*Handler)(nil)

func NewHandler(grpc *grpctransport.GRPCHandler) *Handler {
	return &Handler{grpc: grpc}
}

func (h *Handler) StartWorkoutSession(ctx context.Context, req *connect.Request[workoutmsg.StartWorkoutSessionRequest]) (*connect.Response[workoutmsg.StartWorkoutSessionResponse], error) {
	resp, err := h.grpc.StartWorkoutSession(ctx, req.Msg)
	if err != nil {
		return nil, connect.NewError(connect.CodeOf(err), err)
	}
	return connect.NewResponse(resp), nil
}

func (h *Handler) StartScheduledWorkoutSession(ctx context.Context, req *connect.Request[workoutmsg.StartScheduledWorkoutSessionRequest]) (*connect.Response[workoutmsg.StartScheduledWorkoutSessionResponse], error) {
	resp, err := h.grpc.StartScheduledWorkoutSession(ctx, req.Msg)
	if err != nil {
		return nil, connect.NewError(connect.CodeOf(err), err)
	}
	return connect.NewResponse(resp), nil
}

func (h *Handler) LogWorkoutSet(ctx context.Context, req *connect.Request[workoutmsg.LogWorkoutSetRequest]) (*connect.Response[workoutmsg.LogWorkoutSetResponse], error) {
	resp, err := h.grpc.LogWorkoutSet(ctx, req.Msg)
	if err != nil {
		return nil, connect.NewError(connect.CodeOf(err), err)
	}
	return connect.NewResponse(resp), nil
}

func (h *Handler) AbortWorkoutSession(ctx context.Context, req *connect.Request[workoutmsg.AbortWorkoutSessionRequest]) (*connect.Response[workoutmsg.AbortWorkoutSessionResponse], error) {
	resp, err := h.grpc.AbortWorkoutSession(ctx, req.Msg)
	if err != nil {
		return nil, connect.NewError(connect.CodeOf(err), err)
	}
	return connect.NewResponse(resp), nil
}

func (h *Handler) CompleteWorkoutSession(ctx context.Context, req *connect.Request[workoutmsg.CompleteWorkoutSessionRequest]) (*connect.Response[workoutmsg.CompleteWorkoutSessionResponse], error) {
	resp, err := h.grpc.CompleteWorkoutSession(ctx, req.Msg)
	if err != nil {
		return nil, connect.NewError(connect.CodeOf(err), err)
	}
	return connect.NewResponse(resp), nil
}

func (h *Handler) GetMotionSpecification(ctx context.Context, req *connect.Request[workoutmsg.GetMotionSpecificationRequest]) (*connect.Response[workoutmsg.GetMotionSpecificationResponse], error) {
	resp, err := h.grpc.GetMotionSpecification(ctx, req.Msg)
	if err != nil {
		return nil, connect.NewError(connect.CodeOf(err), err)
	}
	return connect.NewResponse(resp), nil
}

func (h *Handler) GetPersonalRecords(ctx context.Context, req *connect.Request[workoutmsg.GetPersonalRecordsRequest]) (*connect.Response[workoutmsg.GetPersonalRecordsResponse], error) {
	resp, err := h.grpc.GetPersonalRecords(ctx, req.Msg)
	if err != nil {
		return nil, connect.NewError(connect.CodeOf(err), err)
	}
	return connect.NewResponse(resp), nil
}

func (h *Handler) GetWorkoutHistory(ctx context.Context, req *connect.Request[workoutmsg.GetWorkoutHistoryRequest]) (*connect.Response[workoutmsg.GetWorkoutHistoryResponse], error) {
	resp, err := h.grpc.GetWorkoutHistory(ctx, req.Msg)
	if err != nil {
		return nil, connect.NewError(connect.CodeOf(err), err)
	}
	return connect.NewResponse(resp), nil
}

func (h *Handler) SyncWorkoutLogs(ctx context.Context, req *connect.Request[workoutmsg.SyncWorkoutLogsRequest]) (*connect.Response[workoutmsg.SyncWorkoutLogsResponse], error) {
	resp, err := h.grpc.SyncWorkoutLogs(ctx, req.Msg)
	if err != nil {
		return nil, connect.NewError(connect.CodeOf(err), err)
	}
	return connect.NewResponse(resp), nil
}

func (h *Handler) GetWorkoutSessionErrors(ctx context.Context, req *connect.Request[workoutmsg.GetWorkoutSessionErrorsRequest]) (*connect.Response[workoutmsg.GetWorkoutSessionErrorsResponse], error) {
	resp, err := h.grpc.GetWorkoutSessionErrors(ctx, req.Msg)
	if err != nil {
		return nil, connect.NewError(connect.CodeOf(err), err)
	}
	return connect.NewResponse(resp), nil
}
