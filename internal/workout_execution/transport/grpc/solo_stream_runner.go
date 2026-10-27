package grpc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"time"

	"connectrpc.com/connect"
	workoutexecutionv1message "github.com/viethung213/gym-companion/internal/gen/go/contracts/core/workout_execution/v1/message"
	"github.com/viethung213/gym-companion/internal/workout_execution/application/apperror"
	"github.com/viethung213/gym-companion/internal/workout_execution/application/command"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/service"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/vo"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// soloMatchStreamServer abstracts the bi-directional stream operations between gRPC and ConnectRPC.
type soloMatchStreamServer interface {
	Context() context.Context
	Send(*workoutexecutionv1message.StreamSoloMatchTrackingResponse) error
	Recv() (*workoutexecutionv1message.StreamSoloMatchTrackingRequest, error)
}

// connectSoloStreamAdapter adapts a ConnectRPC BidiStream to soloMatchStreamServer.
type connectSoloStreamAdapter struct {
	ctx    context.Context
	stream *connect.BidiStream[
		workoutexecutionv1message.StreamSoloMatchTrackingRequest,
		workoutexecutionv1message.StreamSoloMatchTrackingResponse,
	]
}

func (a *connectSoloStreamAdapter) Context() context.Context {
	return a.ctx
}

func (a *connectSoloStreamAdapter) Send(resp *workoutexecutionv1message.StreamSoloMatchTrackingResponse) error {
	return a.stream.Send(resp)
}

func (a *connectSoloStreamAdapter) Recv() (*workoutexecutionv1message.StreamSoloMatchTrackingRequest, error) {
	return a.stream.Receive()
}

// streamErrorFactory allows gRPC and ConnectRPC to generate protocol-native errors.
type streamErrorFactory interface {
	Unauthenticated(err error) error
	InvalidArgument(msg string, err error) error
	PermissionDenied(msg string, err error) error
	FailedPrecondition(msg string, err error) error
	Internal(msg string, err error) error
}

type grpcStreamErrorFactory struct{}

func (grpcStreamErrorFactory) Unauthenticated(err error) error {
	return status.Errorf(codes.Unauthenticated, "unauthenticated stream: %v", err)
}

func (grpcStreamErrorFactory) InvalidArgument(msg string, err error) error {
	if err != nil {
		return status.Errorf(codes.InvalidArgument, "%s: %v", msg, err)
	}
	return status.Errorf(codes.InvalidArgument, "%s", msg)
}

func (grpcStreamErrorFactory) PermissionDenied(msg string, err error) error {
	if err != nil {
		return status.Errorf(codes.PermissionDenied, "%s: %v", msg, err)
	}
	return status.Errorf(codes.PermissionDenied, "%s", msg)
}

func (grpcStreamErrorFactory) FailedPrecondition(msg string, err error) error {
	if err != nil {
		return status.Errorf(codes.FailedPrecondition, "%s: %v", msg, err)
	}
	return status.Errorf(codes.FailedPrecondition, "%s", msg)
}

func (grpcStreamErrorFactory) Internal(msg string, err error) error {
	if err != nil {
		return status.Errorf(codes.Internal, "%s: %v", msg, err)
	}
	return status.Errorf(codes.Internal, "%s", msg)
}

type connectStreamErrorFactory struct{}

func (connectStreamErrorFactory) Unauthenticated(err error) error {
	return connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("unauthenticated stream: %w", err))
}

func (connectStreamErrorFactory) InvalidArgument(msg string, err error) error {
	if err != nil {
		return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("%s: %w", msg, err))
	}
	return connect.NewError(connect.CodeInvalidArgument, errors.New(msg))
}

func (connectStreamErrorFactory) PermissionDenied(msg string, err error) error {
	if err != nil {
		return connect.NewError(connect.CodePermissionDenied, fmt.Errorf("%s: %w", msg, err))
	}
	return connect.NewError(connect.CodePermissionDenied, errors.New(msg))
}

func (connectStreamErrorFactory) FailedPrecondition(msg string, err error) error {
	if err != nil {
		return connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("%s: %w", msg, err))
	}
	return connect.NewError(connect.CodeFailedPrecondition, errors.New(msg))
}

func (connectStreamErrorFactory) Internal(msg string, err error) error {
	if err != nil {
		return connect.NewError(connect.CodeInternal, fmt.Errorf("%s: %w", msg, err))
	}
	return connect.NewError(connect.CodeInternal, errors.New(msg))
}

// runSoloMatchTrackingLoop coordinates the single source of truth streaming loop for both gRPC and ConnectRPC.
func runSoloMatchTrackingLoop(
	ctx context.Context,
	stream soloMatchStreamServer,
	soloTrackingService *command.SoloTrackingService,
	errFactory streamErrorFactory,
) error {
	userID, err := extractUserID(ctx)
	if err != nil || userID == "" {
		return errFactory.Unauthenticated(err)
	}

	streamCtx, cancelStream := context.WithCancel(ctx)
	defer cancelStream()

	var (
		session       *service.SoloRefereeSession
		rule          *vo.ExerciseMotionRule
		matchID       string
		exerciseID    string
		planID        string
		cameraAngle   string
		lastTimestamp time.Time
		sendMu        sync.Mutex
		subCancel     context.CancelFunc
	)

	safeSend := func(resp *workoutexecutionv1message.StreamSoloMatchTrackingResponse) error {
		sendMu.Lock()
		defer sendMu.Unlock()
		return stream.Send(resp)
	}

	defer func() {
		cancelStream()
		if subCancel != nil {
			subCancel()
		}
		if session != nil && soloTrackingService != nil {
			cleanCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			if session.IsFinalized() {
				return
			}

			if session.IsCompleted() {
				// Completed normally via Submit or Time Ceiling: persist results
				if finErr := soloTrackingService.FinalizeSession(
					cleanCtx, session, matchID, userID, exerciseID, planID,
				); finErr != nil {
					slog.WarnContext(cleanCtx, "failed to finalize solo session on stream exit",
						slog.String("match_id", matchID),
						slog.String("user_id", userID),
						slog.Any("err", finErr),
					)
				}
			} else {
				// Sudden connection drop without explicit Abort:
				// Mark as DISCONNECTED in Redis and schedule background grace period timer to auto-abort if user does not reconnect.
				if disErr := soloTrackingService.HandleParticipantDisconnect(
					cleanCtx, session, matchID, userID, exerciseID, planID,
				); disErr != nil {
					slog.WarnContext(cleanCtx, "failed to handle participant disconnect on stream exit",
						slog.String("match_id", matchID),
						slog.String("user_id", userID),
						slog.Any("err", disErr),
					)
				}
			}
		}
	}()

	type recvResult struct {
		req *workoutexecutionv1message.StreamSoloMatchTrackingRequest
		err error
	}
	// Buffered channel of size 1 ensures that sending the final EOF or error never deadlocks if main loop exits.
	recvChan := make(chan recvResult, 1)

	go func() {
		for {
			req, recvErr := stream.Recv()
			if streamCtx.Err() != nil {
				return
			}
			select {
			case <-streamCtx.Done():
				return
			case recvChan <- recvResult{req: req, err: recvErr}:
				if recvErr != nil {
					return
				}
			}
		}
	}()

	// drainUntilEOF allows client to gracefully close its send stream after receiving MatchEnded,
	// draining lingering frames and preventing orphaned goroutine leaks.
	drainUntilEOF := func(timeout time.Duration) {
		timer := time.NewTimer(timeout)
		defer timer.Stop()

		for {
			select {
			case res, ok := <-recvChan:
				if !ok || res.err != nil {
					return
				}
			case <-timer.C:
				return
			case <-streamCtx.Done():
				return
			}
		}
	}

	for {
		var req *workoutexecutionv1message.StreamSoloMatchTrackingRequest
		select {
		case <-streamCtx.Done():
			drainUntilEOF(1 * time.Second)
			return nil
		case res := <-recvChan:
			if res.err != nil {
				if errors.Is(res.err, io.EOF) {
					return nil
				}
				return res.err
			}
			req = res.req
		}

		switch p := req.GetPayload().(type) {
		case *workoutexecutionv1message.StreamSoloMatchTrackingRequest_Config:
			cfg := p.Config
			if cfg != nil && session == nil {
				matchID = cfg.MatchId
				exerciseID = cfg.ExerciseId
				challengeType := toDomainChallengeType(cfg.ChallengeType)
				planID = strings.TrimSpace(cfg.PlanId)
				if planID == "" {
					return errFactory.InvalidArgument("plan_id is required", nil)
				}
				ceilingSec := cfg.TimeCeilingSeconds
				cameraAngle = strings.ToLower(strings.TrimSpace(cfg.CameraAngle))
				if soloTrackingService != nil {
					sess, r, initErr := soloTrackingService.InitializeSession(
						streamCtx, matchID, userID, exerciseID, challengeType, ceilingSec, planID,
					)
					if initErr != nil {
						if errors.Is(initErr, apperror.ErrMatchRoomFull) {
							return errFactory.PermissionDenied(initErr.Error(), nil)
						}
						if errors.Is(initErr, apperror.ErrMatchExerciseMismatch) {
							return errFactory.InvalidArgument(initErr.Error(), nil)
						}
						if errors.Is(initErr, apperror.ErrInvalidInput) {
							return errFactory.InvalidArgument(initErr.Error(), nil)
						}
						return errFactory.FailedPrecondition("exercise motion rule not configured", initErr)
					}
					session = sess
					if cameraAngle != "" {
						session.SetCameraAngle(cameraAngle)
					}
					rule = r

					// Start asynchronous opponent update listener for real-time spectating and match-end sync
					subCtx, cancel := context.WithCancel(streamCtx)
					subCancel = cancel
					oppChan, unsub, subErr := soloTrackingService.SubscribeMatchUpdates(
						subCtx, matchID, userID,
					)
					if subErr != nil {
						slog.WarnContext(subCtx, "failed to subscribe match updates",
							slog.String("match_id", matchID),
							slog.String("user_id", userID),
							slog.Any("err", subErr),
						)
					}

					go func() {
						if unsub != nil {
							defer unsub()
						}
						ticker := time.NewTicker(1 * time.Second)
						defer ticker.Stop()

						for {
							select {
							case <-subCtx.Done():
								return
							case prog, ok := <-oppChan:
								if !ok {
									oppChan = nil
									continue
								}
								if session == nil {
									continue
								}

								if prog.Status == "CANCELLED" {
									resp := &workoutexecutionv1message.StreamSoloMatchTrackingResponse{
										MyConfirmedValue:        session.ConfirmedValue(),
										MyFormScore:             session.AverageFormScore(),
										IsValidForm:             true,
										MyStatus:                workoutexecutionv1message.ParticipantStreamStatus_PARTICIPANT_STREAM_STATUS_UNSPECIFIED,
										IsCompleted:             true,
										OpponentValue:           0,
										OpponentStatus:          workoutexecutionv1message.ParticipantStreamStatus_PARTICIPANT_STREAM_STATUS_UNSPECIFIED,
										AudioCue:                "Trận đấu đã bị hủy",
										RemainingCeilingSeconds: 0,
										MatchEnded:              true,
										Timestamp:               timestamppb.Now(),
									}
									_ = safeSend(resp)
									cancelStream()
									return
								}

								// If session is waiting for opponent, receiving any opponent activity confirms room is ready
								if session.IsWaiting() {
									session.Activate()
								}

								bothDone := session.IsCompleted() &&
									(prog.Status == "COMPLETED" || prog.Status == "SPECTATING" || prog.Status == "ABORTED")
								if bothDone {
									session.MarkCompleted(session.CompletionReason())
								}
								resp := &workoutexecutionv1message.StreamSoloMatchTrackingResponse{
									MyConfirmedValue:        session.ConfirmedValue(),
									MyFormScore:             session.AverageFormScore(),
									IsValidForm:             true,
									MyStatus:                mapParticipantStatusToProto(string(session.Status())),
									IsCompleted:             session.IsCompleted(),
									OpponentValue:           prog.Value,
									OpponentFormScore:       prog.FormScore,
									OpponentStatus:          mapParticipantStatusToProto(prog.Status),
									RemainingCeilingSeconds: session.RemainingCeilingSeconds(),
									MatchEnded:              bothDone,
									Timestamp:               timestamppb.Now(),
								}
								if err := safeSend(resp); err != nil {
									cancelStream()
									return
								}
								if bothDone {
									cancelStream()
									return
								}
							case <-ticker.C:
								// Proactively check if match has become ready while session is still waiting for opponent
								if session != nil && session.IsWaiting() && soloTrackingService != nil {
									ready, checkErr := soloTrackingService.CheckMatchReady(subCtx, matchID)
									if checkErr == nil && ready {
										session.Activate()
										resp := &workoutexecutionv1message.StreamSoloMatchTrackingResponse{
											MyConfirmedValue:        session.ConfirmedValue(),
											MyFormScore:             session.AverageFormScore(),
											IsValidForm:             true,
											MyStatus:                mapParticipantStatusToProto(string(session.Status())),
											IsCompleted:             false,
											RemainingCeilingSeconds: session.RemainingCeilingSeconds(),
											MatchEnded:              false,
											Timestamp:               timestamppb.Now(),
										}
										_ = safeSend(resp)
									}
								}

								if session != nil && !session.IsCompleted() && session.RemainingCeilingSeconds() <= 0 {
									if forceErr := soloTrackingService.ForceCompleteSessionOnCeiling(
										subCtx, session, matchID, userID, exerciseID, planID,
									); forceErr != nil {
										slog.ErrorContext(subCtx, "failed to force-complete session on ceiling timeout",
											slog.String("match_id", matchID),
											slog.String("user_id", userID),
											slog.Any("err", forceErr),
										)
									}
									resp := &workoutexecutionv1message.StreamSoloMatchTrackingResponse{
										MyConfirmedValue:        session.ConfirmedValue(),
										MyFormScore:             session.AverageFormScore(),
										IsValidForm:             true,
										MyStatus:                mapParticipantStatusToProto(string(session.Status())),
										IsCompleted:             true,
										RemainingCeilingSeconds: 0,
										MatchEnded:              true,
										Timestamp:               timestamppb.Now(),
									}
									if sendErr := safeSend(resp); sendErr != nil {
										slog.WarnContext(subCtx, "failed to send ceiling timeout response",
											slog.Any("err", sendErr),
										)
									}
									cancelStream()
									return
								}

								if session != nil && session.IsCompleted() {
									opp, oppErr := soloTrackingService.GetOpponentProgress(subCtx, matchID, userID)
									if oppErr != nil {
										slog.WarnContext(subCtx, "failed to poll opponent progress",
											slog.String("match_id", matchID),
											slog.Any("err", oppErr),
										)
									} else if opp != nil {
										oppGraceExpired := opp.Status == "DISCONNECTED" && opp.DisconnectedAt > 0 && (time.Now().Unix()-opp.DisconnectedAt >= 45)
										bothDone := opp.Status == "COMPLETED" || opp.Status == "SPECTATING" || opp.Status == "ABORTED" || oppGraceExpired
										if bothDone {
											session.MarkCompleted(session.CompletionReason())
											oppStatusStr := opp.Status
											if oppGraceExpired {
												oppStatusStr = "ABORTED"
											}
											resp := &workoutexecutionv1message.StreamSoloMatchTrackingResponse{
												MyConfirmedValue:        session.ConfirmedValue(),
												MyFormScore:             session.AverageFormScore(),
												IsValidForm:             true,
												MyStatus:                mapParticipantStatusToProto(string(session.Status())),
												IsCompleted:             session.IsCompleted(),
												OpponentValue:           opp.Value,
												OpponentFormScore:       opp.FormScore,
												OpponentStatus:          mapParticipantStatusToProto(oppStatusStr),
												RemainingCeilingSeconds: session.RemainingCeilingSeconds(),
												MatchEnded:              true,
												Timestamp:               timestamppb.Now(),
											}
											if sendErr := safeSend(resp); sendErr != nil {
												slog.WarnContext(subCtx, "failed to send match ended response",
													slog.Any("err", sendErr),
												)
											}
											cancelStream()
											return
										}
									}
								}
							}
						}
					}()
				}
				lastTimestamp = time.Now()

				// Send immediate initial status response so client receives acknowledgement and current room status
				if session != nil {
					oppVal := int32(0)
					oppScore := float32(0)
					oppStatus := workoutexecutionv1message.ParticipantStreamStatus_PARTICIPANT_STREAM_STATUS_UNSPECIFIED
					if soloTrackingService != nil {
						if opp, _ := soloTrackingService.GetOpponentProgress(streamCtx, matchID, userID); opp != nil {
							oppVal = opp.Value
							oppScore = opp.FormScore
							oppStatus = mapParticipantStatusToProto(opp.Status)
						}
					}

					initResp := &workoutexecutionv1message.StreamSoloMatchTrackingResponse{
						MyConfirmedValue:        session.ConfirmedValue(),
						MyFormScore:             session.AverageFormScore(),
						IsValidForm:             true,
						MyStatus:                mapParticipantStatusToProto(string(session.Status())),
						IsCompleted:             session.IsCompleted(),
						OpponentValue:           oppVal,
						OpponentFormScore:       oppScore,
						OpponentStatus:          oppStatus,
						RemainingCeilingSeconds: session.RemainingCeilingSeconds(),
						MatchEnded:              false,
						Timestamp:               timestamppb.Now(),
					}
					if err := safeSend(initResp); err != nil {
						return err
					}
				}
			}
			continue

		case *workoutexecutionv1message.StreamSoloMatchTrackingRequest_Abort:
			reason := ""
			if p.Abort != nil {
				reason = p.Abort.Reason
			}
			if reason == "" {
				reason = "USER_FORFEITED"
			}
			if soloTrackingService != nil && session != nil {
				if abortErr := soloTrackingService.AbortSession(
					streamCtx, session, matchID, userID, exerciseID, reason, planID,
				); abortErr != nil {
					slog.WarnContext(streamCtx, "failed to abort solo session",
						slog.String("match_id", matchID),
						slog.String("user_id", userID),
						slog.Any("err", abortErr),
					)
				}
			}
			cancelStream()
			return nil

		case *workoutexecutionv1message.StreamSoloMatchTrackingRequest_Submit:
			if session == nil {
				continue
			}
			if soloTrackingService != nil {
				res, submitErr := soloTrackingService.SubmitSession(
					streamCtx, session, matchID, userID, exerciseID, planID,
				)
				if submitErr != nil {
					return errFactory.Internal("submit session", submitErr)
				}
				resp := &workoutexecutionv1message.StreamSoloMatchTrackingResponse{
					MyConfirmedValue:        res.MyConfirmedValue,
					MyFormScore:             res.MyFormScore,
					IsValidForm:             res.IsValidForm,
					MyStatus:                mapParticipantStatusToProto(res.MyStatus),
					IsCompleted:             res.IsCompleted,
					OpponentValue:           res.OpponentValue,
					OpponentFormScore:       res.OpponentFormScore,
					OpponentStatus:          mapParticipantStatusToProto(res.OpponentStatus),
					DetectedErrors:          res.DetectedErrors,
					AudioCue:                res.AudioCue,
					RemainingCeilingSeconds: res.RemainingCeilingSeconds,
					MatchEnded:              res.MatchEnded,
					Timestamp:               timestamppb.Now(),
				}
				if err := safeSend(resp); err != nil {
					return err
				}
				if res.MatchEnded {
					drainUntilEOF(1 * time.Second)
					cancelStream()
					return nil
				}
			}
			continue

		case *workoutexecutionv1message.StreamSoloMatchTrackingRequest_Frame:
			frm := p.Frame
			if frm == nil {
				continue
			}

			if session == nil || rule == nil {
				return errFactory.FailedPrecondition("session not initialized: exercise motion rule is required before streaming frames", nil)
			}

			now := time.Now()
			delta := now.Sub(lastTimestamp)
			if delta <= 0 {
				delta = 50 * time.Millisecond
			}
			lastTimestamp = now

			landmarks := make([]vo.SkeletonPoint, 0, len(frm.Landmarks))
			for _, lm := range frm.Landmarks {
				landmarks = append(landmarks, vo.SkeletonPoint{
					Index:      int(lm.LandmarkIndex),
					X:          lm.X,
					Y:          lm.Y,
					Z:          lm.Z,
					Visibility: lm.Visibility,
					Presence:   lm.Presence,
				})
			}

			if frm.CameraAngle != "" {
				cameraAngle = strings.ToLower(strings.TrimSpace(frm.CameraAngle))
				session.SetCameraAngle(cameraAngle)
			}

			var result command.StreamFrameResult
			if soloTrackingService != nil {
				res, procErr := soloTrackingService.ProcessFrame(
					streamCtx, session, rule, landmarks, delta, matchID, userID, exerciseID, planID, cameraAngle,
				)
				if procErr != nil {
					return errFactory.Internal("process frame", procErr)
				}
				result = res
			}

			resp := &workoutexecutionv1message.StreamSoloMatchTrackingResponse{
				MyConfirmedValue:        result.MyConfirmedValue,
				MyFormScore:             result.MyFormScore,
				IsValidForm:             result.IsValidForm,
				MyStatus:                mapParticipantStatusToProto(result.MyStatus),
				IsCompleted:             result.IsCompleted,
				OpponentValue:           result.OpponentValue,
				OpponentFormScore:       result.OpponentFormScore,
				OpponentStatus:          mapParticipantStatusToProto(result.OpponentStatus),
				DetectedErrors:          result.DetectedErrors,
				AudioCue:                result.AudioCue,
				RemainingCeilingSeconds: result.RemainingCeilingSeconds,
				MatchEnded:              result.MatchEnded,
				Timestamp:               timestamppb.Now(),
			}

			if err := safeSend(resp); err != nil {
				return err
			}

			if result.MatchEnded {
				drainUntilEOF(1 * time.Second)
				cancelStream()
				return nil
			}

		default:
			continue
		}
	}
}
