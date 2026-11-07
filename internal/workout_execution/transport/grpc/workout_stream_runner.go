package grpc

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"time"

	"connectrpc.com/connect"
	workoutexecutionv1message "github.com/viethung213/gym-companion/internal/gen/go/contracts/core/workout_execution/v1/message"
	"github.com/viethung213/gym-companion/internal/workout_execution/application/command"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/service"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/vo"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// workoutStreamServer abstracts the bi-directional stream operations between gRPC and ConnectRPC for single-player workout sets.
type workoutStreamServer interface {
	Context() context.Context
	Send(*workoutexecutionv1message.StreamWorkoutTrackingResponse) error
	Recv() (*workoutexecutionv1message.StreamWorkoutTrackingRequest, error)
}

// connectWorkoutStreamAdapter adapts a ConnectRPC BidiStream to workoutStreamServer.
type connectWorkoutStreamAdapter struct {
	ctx    context.Context
	stream *connect.BidiStream[
		workoutexecutionv1message.StreamWorkoutTrackingRequest,
		workoutexecutionv1message.StreamWorkoutTrackingResponse,
	]
}

func (a *connectWorkoutStreamAdapter) Context() context.Context {
	return a.ctx
}

func (a *connectWorkoutStreamAdapter) Send(resp *workoutexecutionv1message.StreamWorkoutTrackingResponse) error {
	return a.stream.Send(resp)
}

func (a *connectWorkoutStreamAdapter) Recv() (*workoutexecutionv1message.StreamWorkoutTrackingRequest, error) {
	return a.stream.Receive()
}

// RunWorkoutTrackingLoop runs the authoritative referee loop for an individual workout set.
// It is intentionally 100% decoupled from database session lifecycle (no premature DB completion or abortion).
func RunWorkoutTrackingLoop(
	ctx context.Context,
	stream workoutStreamServer,
	svc *command.WorkoutTrackingService,
) error {
	userID, err := extractUserID(ctx)
	if err != nil {
		return err
	}

	var (
		referee       *service.MotionRefereeSession
		rule          *vo.ExerciseMotionRule
		cameraAngle   string
		lastTimestamp time.Time
		lastClientTS  time.Time
		sendMu        sync.Mutex
	)

	safeSend := func(resp *workoutexecutionv1message.StreamWorkoutTrackingResponse) error {
		sendMu.Lock()
		defer sendMu.Unlock()
		return stream.Send(resp)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		req, err := stream.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, context.Canceled) {
				return nil
			}
			return err
		}

		switch payload := req.GetPayload().(type) {
		case *workoutexecutionv1message.StreamWorkoutTrackingRequest_Init:
			initReq := payload.Init
			if initReq == nil {
				continue
			}

			cameraAngle = initReq.GetCameraAngle()
			challengeType := vo.ChallengeTypeReps
			if initReq.GetChallengeType() == workoutexecutionv1message.WorkoutTrackingChallengeType_WORKOUT_TRACKING_CHALLENGE_TYPE_DURATION {
				challengeType = vo.ChallengeTypeDuration
			}

			referee, rule, err = svc.InitializeSession(
				ctx,
				initReq.GetSessionId(),
				userID,
				initReq.GetExerciseId(),
				cameraAngle,
				challengeType,
				initReq.GetTargetReps(),
				initReq.GetTargetDurationSeconds(),
			)
			if err != nil {
				slog.ErrorContext(ctx, "failed to initialize workout tracking referee",
					slog.String("exercise_id", initReq.GetExerciseId()),
					slog.Any("err", err),
				)
				return err
			}

			lastTimestamp = time.Now()

			_ = safeSend(&workoutexecutionv1message.StreamWorkoutTrackingResponse{
				RepsCompleted:       0,
				HoldDurationSeconds: 0,
				FormScore:           100,
				IsValidForm:         true,
				IsSetCompleted:      false,
				Timestamp:           timestamppb.Now(),
				RepPhase:            string(referee.RepPhase()),
				PersonDetected:      false,
			})

		case *workoutexecutionv1message.StreamWorkoutTrackingRequest_Frame:
			if referee == nil || rule == nil {
				continue
			}

			frm := payload.Frame
			if frm == nil {
				continue
			}

			landmarks := make([]vo.SkeletonPoint, 0, len(frm.GetLandmarks()))
			for _, p := range frm.GetLandmarks() {
				landmarks = append(landmarks, vo.SkeletonPoint{
					Index:      int(p.GetLandmarkIndex()),
					X:          p.GetX(),
					Y:          p.GetY(),
					Z:          p.GetZ(),
					Visibility: p.GetVisibility(),
					Presence:   p.GetPresence(),
				})
			}

			now := time.Now()
			delta := frameDelta(frm.GetTimestamp(), lastClientTS, now.Sub(lastTimestamp))
			if frm.GetTimestamp() != nil && frm.GetTimestamp().IsValid() {
				lastClientTS = frm.GetTimestamp().AsTime()
			}
			lastTimestamp = now

			activeAngle := frm.GetCameraAngle()
			if activeAngle == "" {
				activeAngle = cameraAngle
			}

			result, procErr := svc.ProcessFrame(ctx, referee, rule, landmarks, delta, activeAngle)
			if procErr != nil {
				slog.WarnContext(ctx, "error processing workout frame", slog.Any("err", procErr))
				continue
			}

			resp := &workoutexecutionv1message.StreamWorkoutTrackingResponse{
				RepsCompleted:       result.RepsCompleted,
				HoldDurationSeconds: result.HoldDurationSeconds,
				FormScore:           result.FormScore,
				IsValidForm:         result.IsValidForm,
				IsSetCompleted:      result.IsSetCompleted,
				DetectedErrors:      result.ActiveErrors,
				AudioCue:            result.AudioCue,
				Timestamp:           timestamppb.Now(),
				CurrentAngle:        result.CurrentAngle,
				RepPhase:            result.RepPhase,
				PersonDetected:      result.PersonDetected,
			}

			if sendErr := safeSend(resp); sendErr != nil {
				return sendErr
			}

		case *workoutexecutionv1message.StreamWorkoutTrackingRequest_Submit:
			if referee == nil {
				return nil
			}

			result, submitErr := svc.SubmitSet(ctx, referee)
			if submitErr != nil {
				slog.ErrorContext(ctx, "failed to submit workout set", slog.Any("err", submitErr))
				return submitErr
			}

			resp := &workoutexecutionv1message.StreamWorkoutTrackingResponse{
				RepsCompleted:       result.RepsCompleted,
				HoldDurationSeconds: result.HoldDurationSeconds,
				FormScore:           result.FormScore,
				IsValidForm:         result.IsValidForm,
				IsSetCompleted:      true,
				DetectedErrors:      result.ActiveErrors,
				AudioCue:            result.AudioCue,
				Timestamp:           timestamppb.Now(),
				CurrentAngle:        result.CurrentAngle,
				RepPhase:            result.RepPhase,
				PersonDetected:      result.PersonDetected,
			}

			_ = safeSend(resp)
			return nil

		case *workoutexecutionv1message.StreamWorkoutTrackingRequest_Heartbeat:
			_ = safeSend(&workoutexecutionv1message.StreamWorkoutTrackingResponse{
				Timestamp: timestamppb.Now(),
			})
		}
	}
}

// WorkoutMatchStream is the transport-agnostic interface for workout tracking.
type WorkoutMatchStream = workoutStreamServer
