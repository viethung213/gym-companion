package connect

import (
	"context"

	"connectrpc.com/connect"
	workoutmsg "github.com/viethung213/gym-companion/internal/gen/go/contracts/core/workout_execution/v1/message"
	workoutv1serviceconnect "github.com/viethung213/gym-companion/internal/gen/go/contracts/core/workout_execution/v1/service/workoutexecutionv1serviceconnect"
	"github.com/viethung213/gym-companion/internal/shared/connectutil"
	"github.com/viethung213/gym-companion/internal/workout_execution/infrastructure/transport"
)

// Adapter wraps GRPCHandler to implement WorkoutExecutionServiceHandler.
type Adapter struct {
	workoutv1serviceconnect.UnimplementedWorkoutExecutionServiceHandler
	grpc *transport.GRPCHandler
}

var _ workoutv1serviceconnect.WorkoutExecutionServiceHandler = (*Adapter)(nil)

// NewAdapter creates a new Connect adapter wrapping the existing gRPC handler.
func NewAdapter(grpc *transport.GRPCHandler) *Adapter {
	return &Adapter{grpc: grpc}
}

func (a *Adapter) StartWorkoutSession(
	ctx context.Context,
	req *connect.Request[workoutmsg.StartWorkoutSessionRequest],
) (*connect.Response[workoutmsg.StartWorkoutSessionResponse], error) {
	resp, err := a.grpc.StartWorkoutSession(ctx, req.Msg)
	if err != nil {
		return nil, connectutil.FromGRPCError(err)
	}
	return connect.NewResponse(resp), nil
}

func (a *Adapter) StartScheduledWorkoutSession(
	ctx context.Context,
	req *connect.Request[workoutmsg.StartScheduledWorkoutSessionRequest],
) (*connect.Response[workoutmsg.StartScheduledWorkoutSessionResponse], error) {
	resp, err := a.grpc.StartScheduledWorkoutSession(ctx, req.Msg)
	if err != nil {
		return nil, connectutil.FromGRPCError(err)
	}
	return connect.NewResponse(resp), nil
}

func (a *Adapter) LogWorkoutSet(
	ctx context.Context,
	req *connect.Request[workoutmsg.LogWorkoutSetRequest],
) (*connect.Response[workoutmsg.LogWorkoutSetResponse], error) {
	resp, err := a.grpc.LogWorkoutSet(ctx, req.Msg)
	if err != nil {
		return nil, connectutil.FromGRPCError(err)
	}
	return connect.NewResponse(resp), nil
}

func (a *Adapter) AbortWorkoutSession(
	ctx context.Context,
	req *connect.Request[workoutmsg.AbortWorkoutSessionRequest],
) (*connect.Response[workoutmsg.AbortWorkoutSessionResponse], error) {
	resp, err := a.grpc.AbortWorkoutSession(ctx, req.Msg)
	if err != nil {
		return nil, connectutil.FromGRPCError(err)
	}
	return connect.NewResponse(resp), nil
}

func (a *Adapter) CompleteWorkoutSession(
	ctx context.Context,
	req *connect.Request[workoutmsg.CompleteWorkoutSessionRequest],
) (*connect.Response[workoutmsg.CompleteWorkoutSessionResponse], error) {
	resp, err := a.grpc.CompleteWorkoutSession(ctx, req.Msg)
	if err != nil {
		return nil, connectutil.FromGRPCError(err)
	}
	return connect.NewResponse(resp), nil
}

func (a *Adapter) SyncWorkoutLogs(
	ctx context.Context,
	req *connect.Request[workoutmsg.SyncWorkoutLogsRequest],
) (*connect.Response[workoutmsg.SyncWorkoutLogsResponse], error) {
	resp, err := a.grpc.SyncWorkoutLogs(ctx, req.Msg)
	if err != nil {
		return nil, connectutil.FromGRPCError(err)
	}
	return connect.NewResponse(resp), nil
}

func (a *Adapter) GetMotionSpecification(
	ctx context.Context,
	req *connect.Request[workoutmsg.GetMotionSpecificationRequest],
) (*connect.Response[workoutmsg.GetMotionSpecificationResponse], error) {
	resp, err := a.grpc.GetMotionSpecification(ctx, req.Msg)
	if err != nil {
		return nil, connectutil.FromGRPCError(err)
	}
	return connect.NewResponse(resp), nil
}

func (a *Adapter) GetPersonalRecords(
	ctx context.Context,
	req *connect.Request[workoutmsg.GetPersonalRecordsRequest],
) (*connect.Response[workoutmsg.GetPersonalRecordsResponse], error) {
	resp, err := a.grpc.GetPersonalRecords(ctx, req.Msg)
	if err != nil {
		return nil, connectutil.FromGRPCError(err)
	}
	return connect.NewResponse(resp), nil
}

func (a *Adapter) GetWorkoutSessionErrors(
	ctx context.Context,
	req *connect.Request[workoutmsg.GetWorkoutSessionErrorsRequest],
) (*connect.Response[workoutmsg.GetWorkoutSessionErrorsResponse], error) {
	resp, err := a.grpc.GetWorkoutSessionErrors(ctx, req.Msg)
	if err != nil {
		return nil, connectutil.FromGRPCError(err)
	}
	return connect.NewResponse(resp), nil
}

func (a *Adapter) GetWorkoutHistory(
	ctx context.Context,
	req *connect.Request[workoutmsg.GetWorkoutHistoryRequest],
) (*connect.Response[workoutmsg.GetWorkoutHistoryResponse], error) {
	resp, err := a.grpc.GetWorkoutHistory(ctx, req.Msg)
	if err != nil {
		return nil, connectutil.FromGRPCError(err)
	}
	return connect.NewResponse(resp), nil
}

// AdminAdapter wraps GRPCHandler to implement AdminWorkoutExecutionServiceHandler.
type AdminAdapter struct {
	workoutv1serviceconnect.UnimplementedAdminWorkoutExecutionServiceHandler
	grpc *transport.GRPCHandler
}

var _ workoutv1serviceconnect.AdminWorkoutExecutionServiceHandler = (*AdminAdapter)(nil)

// NewAdminAdapter creates a new Connect adapter wrapping the existing gRPC handler.
func NewAdminAdapter(grpc *transport.GRPCHandler) *AdminAdapter {
	return &AdminAdapter{grpc: grpc}
}

func (a *AdminAdapter) AdminGetPersonalRecords(
	ctx context.Context,
	req *connect.Request[workoutmsg.AdminGetPersonalRecordsRequest],
) (*connect.Response[workoutmsg.AdminGetPersonalRecordsResponse], error) {
	resp, err := a.grpc.AdminGetPersonalRecords(ctx, req.Msg)
	if err != nil {
		return nil, connectutil.FromGRPCError(err)
	}
	return connect.NewResponse(resp), nil
}

func (a *AdminAdapter) AdminGetWorkoutHistory(
	ctx context.Context,
	req *connect.Request[workoutmsg.AdminGetWorkoutHistoryRequest],
) (*connect.Response[workoutmsg.AdminGetWorkoutHistoryResponse], error) {
	resp, err := a.grpc.AdminGetWorkoutHistory(ctx, req.Msg)
	if err != nil {
		return nil, connectutil.FromGRPCError(err)
	}
	return connect.NewResponse(resp), nil
}

func (a *AdminAdapter) GetPresignedUploadURL(
	ctx context.Context,
	req *connect.Request[workoutmsg.GetPresignedUploadURLRequest],
) (*connect.Response[workoutmsg.GetPresignedUploadURLResponse], error) {
	resp, err := a.grpc.GetPresignedUploadURL(ctx, req.Msg)
	if err != nil {
		return nil, connectutil.FromGRPCError(err)
	}
	return connect.NewResponse(resp), nil
}

func (a *AdminAdapter) UpdateMotionSpecification(
	ctx context.Context,
	req *connect.Request[workoutmsg.UpdateMotionSpecificationRequest],
) (*connect.Response[workoutmsg.UpdateMotionSpecificationResponse], error) {
	resp, err := a.grpc.UpdateMotionSpecification(ctx, req.Msg)
	if err != nil {
		return nil, connectutil.FromGRPCError(err)
	}
	return connect.NewResponse(resp), nil
}

func (a *AdminAdapter) PatchMotionSpecificationAsset(
	ctx context.Context,
	req *connect.Request[workoutmsg.PatchMotionSpecificationAssetRequest],
) (*connect.Response[workoutmsg.PatchMotionSpecificationAssetResponse], error) {
	resp, err := a.grpc.PatchMotionSpecificationAsset(ctx, req.Msg)
	if err != nil {
		return nil, connectutil.FromGRPCError(err)
	}
	return connect.NewResponse(resp), nil
}

func (a *AdminAdapter) DeleteMotionSpecification(
	ctx context.Context,
	req *connect.Request[workoutmsg.DeleteMotionSpecificationRequest],
) (*connect.Response[workoutmsg.DeleteMotionSpecificationResponse], error) {
	resp, err := a.grpc.DeleteMotionSpecification(ctx, req.Msg)
	if err != nil {
		return nil, connectutil.FromGRPCError(err)
	}
	return connect.NewResponse(resp), nil
}

func (a *AdminAdapter) ListMotionSpecifications(
	ctx context.Context,
	req *connect.Request[workoutmsg.ListMotionSpecificationsRequest],
) (*connect.Response[workoutmsg.ListMotionSpecificationsResponse], error) {
	resp, err := a.grpc.ListMotionSpecifications(ctx, req.Msg)
	if err != nil {
		return nil, connectutil.FromGRPCError(err)
	}
	return connect.NewResponse(resp), nil
}
