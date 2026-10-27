package grpc_test

import (
	"context"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	workoutexecutionv1message "github.com/viethung213/gym-companion/internal/gen/go/contracts/core/workout_execution/v1/message"
	"github.com/viethung213/gym-companion/internal/shared/middleware"
	"github.com/viethung213/gym-companion/internal/workout_execution/application/command"
	"github.com/viethung213/gym-companion/internal/workout_execution/application/port"
	domainEvent "github.com/viethung213/gym-companion/internal/workout_execution/domain/event"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/service"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/vo"
	infraRule "github.com/viethung213/gym-companion/internal/workout_execution/infrastructure/rule"
	infraSync "github.com/viethung213/gym-companion/internal/workout_execution/infrastructure/sync"
	"github.com/viethung213/gym-companion/internal/workout_execution/transport/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type mockStreamServer struct {
	mu      sync.Mutex
	ctx     context.Context
	queue   []*workoutexecutionv1message.StreamSoloMatchTrackingRequest
	sent    []*workoutexecutionv1message.StreamSoloMatchTrackingResponse
	recvIdx int
}

func (m *mockStreamServer) Send(resp *workoutexecutionv1message.StreamSoloMatchTrackingResponse) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sent = append(m.sent, resp)
	return nil
}

func (m *mockStreamServer) Recv() (*workoutexecutionv1message.StreamSoloMatchTrackingRequest, error) {
	if m.recvIdx >= len(m.queue) {
		return nil, io.EOF
	}
	req := m.queue[m.recvIdx]
	m.recvIdx++
	return req, nil
}

func (m *mockStreamServer) SetHeader(metadata.MD) error  { return nil }
func (m *mockStreamServer) SendHeader(metadata.MD) error { return nil }
func (m *mockStreamServer) SetTrailer(metadata.MD)       {}
func (m *mockStreamServer) Context() context.Context     { return m.ctx }
func (m *mockStreamServer) SendMsg(any) error            { return nil }
func (m *mockStreamServer) RecvMsg(any) error            { return nil }

type testOutboxWriter struct {
	events []interface{}
}

func (t *testOutboxWriter) WriteEvents(ctx context.Context, aggType, aggID string, evs []interface{}) error {
	t.events = append(t.events, evs...)
	return nil
}

func TestStreamSoloMatchTracking(t *testing.T) {
	ruleProvider := infraRule.NewCachedRuleProvider(nil, nil)
	plankRule := &vo.ExerciseMotionRule{
		ExerciseID:    "exercise_plank_01",
		ExerciseName:  "Standard Plank",
		ChallengeType: vo.ChallengeTypeDuration,
		MinConfidence: 0.5,
		Rules: []vo.RuleCriterion{
			{
				Name:    "spine_alignment",
				Formula: vo.FormulaAngle3D,
				Joints:  []int{11, 23, 27},
				ValidRange: vo.ValidAngleRange{
					Min: 165.0,
					Max: 195.0,
				},
				ErrorCodeBelow: "ERR_HIPS_TOO_HIGH",
				ErrorCodeAbove: "ERR_BACK_ARCH",
			},
		},
		PassCondition:        "ALL_MATCH",
		TimeCeiling:          30 * time.Minute,
		ViolationMaxDuration: 5 * time.Second,
	}
	ruleProvider.SetRule("exercise_plank_01", plankRule)

	mr := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	liveSync := infraSync.NewRedisLiveSyncHub(redisClient, time.Hour)
	outbox := &testOutboxWriter{}
	soloService := command.NewSoloTrackingService(ruleProvider, service.NewDefaultRuleEvaluator(), liveSync, outbox, nil, nil)

	h := grpc.NewGRPCHandler(
		nil, nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		soloService,
	)

	t.Run("Unauthenticated stream returns error", func(t *testing.T) {
		stream := &mockStreamServer{
			ctx: context.Background(), // no UserID in context
		}
		err := h.StreamSoloMatchTracking(stream)
		if err == nil {
			t.Fatal("expected unauthenticated error, got nil")
		}
	})

	t.Run("Valid streaming frames accumulate duration and sync", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), middleware.UserIDKey, "user-p1")

		// Pre-populate opponent's registration and progress in liveSync hub
		_ = liveSync.RegisterParticipant(ctx, "match-101", "user-p2", "exercise_plank_01")
		_ = liveSync.PublishMyProgress(ctx, "match-101", port.ParticipantProgress{
			UserID:    "user-p2",
			Value:     15,
			FormScore: 92.0,
			Status:    "ACTIVE",
		})

		// P1 sends 2 valid plank frames
		validLandmarks := []*workoutexecutionv1message.SkeletonLandmark{
			{LandmarkIndex: 11, X: -1, Y: 0, Z: 0, Visibility: 0.9},
			{LandmarkIndex: 23, X: 0, Y: 0, Z: 0, Visibility: 0.9},
			{LandmarkIndex: 27, X: 1, Y: 0, Z: 0, Visibility: 0.9},
		}

		stream := &mockStreamServer{
			ctx: ctx,
			queue: []*workoutexecutionv1message.StreamSoloMatchTrackingRequest{
				{
					Payload: &workoutexecutionv1message.StreamSoloMatchTrackingRequest_Config{
						Config: &workoutexecutionv1message.SoloTrackingConfig{
							MatchId:            "match-101",
							ExerciseId:         "exercise_plank_01",
							ChallengeType:      workoutexecutionv1message.SoloChallengeType_SOLO_CHALLENGE_TYPE_DURATION,
							TimeCeilingSeconds: 1800,
							PlanId:             "plan-solo-101",
						},
					},
				},
				{
					Payload: &workoutexecutionv1message.StreamSoloMatchTrackingRequest_Frame{
						Frame: &workoutexecutionv1message.SoloTrackingFrame{
							Landmarks: validLandmarks,
							Timestamp: timestamppb.Now(),
						},
					},
				},
				{
					Payload: &workoutexecutionv1message.StreamSoloMatchTrackingRequest_Frame{
						Frame: &workoutexecutionv1message.SoloTrackingFrame{
							Landmarks: validLandmarks,
							Timestamp: timestamppb.Now(),
						},
					},
				},
			},
		}

		err := h.StreamSoloMatchTracking(stream)
		if err != nil {
			t.Fatalf("unexpected stream error: %v", err)
		}

		if len(stream.sent) != 3 {
			t.Fatalf("expected 3 responses sent (1 config ack + 2 frames), got %d", len(stream.sent))
		}

		// Response 0: Immediate initial config ack
		initResp := stream.sent[0]
		if initResp.OpponentValue != 15 {
			t.Errorf("expected opponent value 15 in initial ack, got %d", initResp.OpponentValue)
		}
		if initResp.MyStatus != workoutexecutionv1message.ParticipantStreamStatus_PARTICIPANT_STREAM_STATUS_ACTIVE {
			t.Errorf("expected initial status ACTIVE, got %v", initResp.MyStatus)
		}

		firstResp := stream.sent[1]
		if !firstResp.IsValidForm {
			t.Errorf("expected frame 1 to be valid form")
		}
		if firstResp.OpponentValue != 15 {
			t.Errorf("expected opponent value 15, got %d", firstResp.OpponentValue)
		}
		if firstResp.MyStatus != workoutexecutionv1message.ParticipantStreamStatus_PARTICIPANT_STREAM_STATUS_ACTIVE {
			t.Errorf("expected status ACTIVE, got %v", firstResp.MyStatus)
		}
	})

	t.Run("Duration 5s violation triggers SPECTATING and outbox event", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), middleware.UserIDKey, "user-violator")

		// Pre-populate opponent who is still ACTIVE
		_ = liveSync.RegisterParticipant(ctx, "match-violation", "user-opponent-active", "exercise_plank_01")
		_ = liveSync.PublishMyProgress(ctx, "match-violation", port.ParticipantProgress{
			UserID:    "user-opponent-active",
			Value:     20,
			FormScore: 95.0,
			Status:    "ACTIVE",
		})

		// Initialize referee directly to simulate 5 seconds continuous violation
		referee, rule, err := soloService.InitializeSession(ctx, "match-violation", "user-violator", "exercise_plank_01", vo.ChallengeTypeDuration, 1800, "plan-violator-202")
		if err != nil {
			t.Fatalf("init error: %v", err)
		}

		// Process frames totaling 5s violation
		var finalRes command.StreamFrameResult
		for i := 0; i < 5; i++ {
			points := []vo.SkeletonPoint{
				{Index: 11, X: 1, Y: 0, Z: 0, Visibility: 0.9},
				{Index: 23, X: 0, Y: 0, Z: 0, Visibility: 0.9},
				{Index: 27, X: 0, Y: 1, Z: 0, Visibility: 0.9},
			}
			res, err := soloService.ProcessFrame(
				ctx, referee, rule, points, 1*time.Second, "match-violation", "user-violator", "exercise_plank_01", "plan-violator-202", "side",
			)
			if err != nil {
				t.Fatalf("process frame error: %v", err)
			}
			finalRes = res
		}

		if !finalRes.IsCompleted {
			t.Errorf("expected session completed after 5s violation")
		}
		if finalRes.MyStatus != "SPECTATING" {
			t.Errorf("expected status SPECTATING, got %s", finalRes.MyStatus)
		}
		if finalRes.OpponentValue != 20 {
			t.Errorf("expected opponent value 20, got %d", finalRes.OpponentValue)
		}
		if len(outbox.events) == 0 {
			t.Fatalf("expected SoloTrackingCompleted event to be saved in outbox")
		}
		ev, ok := outbox.events[len(outbox.events)-1].(*domainEvent.SoloTrackingCompleted)
		if !ok {
			t.Fatalf("expected *domainEvent.SoloTrackingCompleted, got %T", outbox.events[len(outbox.events)-1])
		}
		if ev.PlanID != "plan-violator-202" {
			t.Errorf("expected PlanID plan-violator-202, got %s", ev.PlanID)
		}
	})

	t.Run("Client aborts midway with IsAbort flag writes SoloTrackingAborted", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), middleware.UserIDKey, "user-aborter")
		outbox.events = nil

		stream := &mockStreamServer{
			ctx: ctx,
			queue: []*workoutexecutionv1message.StreamSoloMatchTrackingRequest{
				{
					Payload: &workoutexecutionv1message.StreamSoloMatchTrackingRequest_Config{
						Config: &workoutexecutionv1message.SoloTrackingConfig{
							MatchId:            "match-abort-test",
							ExerciseId:         "exercise_plank_01",
							ChallengeType:      workoutexecutionv1message.SoloChallengeType_SOLO_CHALLENGE_TYPE_DURATION,
							TimeCeilingSeconds: 1800,
							PlanId:             "plan-abort-303",
						},
					},
				},
				{
					Payload: &workoutexecutionv1message.StreamSoloMatchTrackingRequest_Abort{
						Abort: &workoutexecutionv1message.SoloTrackingAbort{
							Reason: "USER_FORFEITED",
						},
					},
				},
			},
		}

		err := h.StreamSoloMatchTracking(stream)
		if err != nil {
			t.Fatalf("unexpected stream error on abort: %v", err)
		}

		if len(outbox.events) != 1 {
			t.Fatalf("expected 1 outbox event for abort, got %d", len(outbox.events))
		}
		abortEv, ok := outbox.events[0].(*domainEvent.SoloTrackingAborted)
		if !ok {
			t.Fatalf("expected *domainEvent.SoloTrackingAborted, got %T", outbox.events[0])
		}
		if abortEv.Reason != "USER_FORFEITED" || abortEv.PlanID != "plan-abort-303" {
			t.Errorf("unexpected abort event: %+v", abortEv)
		}
	})

	t.Run("Third participant is rejected with PermissionDenied", func(t *testing.T) {
		matchID := "match-reject-3rd"
		_ = liveSync.RegisterParticipant(context.Background(), matchID, "user-first", "exercise_plank_01")
		_ = liveSync.RegisterParticipant(context.Background(), matchID, "user-second", "exercise_plank_01")

		ctx := context.WithValue(context.Background(), middleware.UserIDKey, "user-intruder")
		stream := &mockStreamServer{
			ctx: ctx,
			queue: []*workoutexecutionv1message.StreamSoloMatchTrackingRequest{
				{
					Payload: &workoutexecutionv1message.StreamSoloMatchTrackingRequest_Config{
						Config: &workoutexecutionv1message.SoloTrackingConfig{
							MatchId:       matchID,
							PlanId:        "plan-reject-3rd",
							ExerciseId:    "exercise_plank_01",
							ChallengeType: workoutexecutionv1message.SoloChallengeType_SOLO_CHALLENGE_TYPE_DURATION,
						},
					},
				},
			},
		}

		err := h.StreamSoloMatchTracking(stream)
		if err == nil {
			t.Fatal("expected 3rd participant to be rejected, got nil")
		}
	})

	t.Run("Participant with different exercise is rejected with InvalidArgument", func(t *testing.T) {
		matchID := "match-mismatch-exercise-test"
		_ = liveSync.RegisterParticipant(context.Background(), matchID, "user-first", "exercise_plank_01")

		ctx := context.WithValue(context.Background(), middleware.UserIDKey, "user-mismatched")
		stream := &mockStreamServer{
			ctx: ctx,
			queue: []*workoutexecutionv1message.StreamSoloMatchTrackingRequest{
				{
					Payload: &workoutexecutionv1message.StreamSoloMatchTrackingRequest_Config{
						Config: &workoutexecutionv1message.SoloTrackingConfig{
							MatchId:       matchID,
							PlanId:        "plan-mismatch",
							ExerciseId:    "exercise_pushup_01",
							ChallengeType: workoutexecutionv1message.SoloChallengeType_SOLO_CHALLENGE_TYPE_REPS,
						},
					},
				},
			},
		}

		err := h.StreamSoloMatchTracking(stream)
		if err == nil {
			t.Fatal("expected mismatched exercise participant to be rejected, got nil")
		}
		st, ok := status.FromError(err)
		if !ok || st.Code() != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument error, got %v (code: %v)", err, st.Code())
		}
	})

	t.Run("First participant stream starts in WAITING status until opponent arrives", func(t *testing.T) {
		matchID := "match-wait-test"
		ctx := context.WithValue(context.Background(), middleware.UserIDKey, "user-alone")

		validLandmarks := []*workoutexecutionv1message.SkeletonLandmark{
			{LandmarkIndex: 11, X: -1, Y: 0, Z: 0, Visibility: 0.9},
			{LandmarkIndex: 23, X: 0, Y: 0, Z: 0, Visibility: 0.9},
			{LandmarkIndex: 27, X: 1, Y: 0, Z: 0, Visibility: 0.9},
		}

		stream := &mockStreamServer{
			ctx: ctx,
			queue: []*workoutexecutionv1message.StreamSoloMatchTrackingRequest{
				{
					Payload: &workoutexecutionv1message.StreamSoloMatchTrackingRequest_Config{
						Config: &workoutexecutionv1message.SoloTrackingConfig{
							MatchId:       matchID,
							PlanId:        "plan-wait-test",
							ExerciseId:    "exercise_plank_01",
							ChallengeType: workoutexecutionv1message.SoloChallengeType_SOLO_CHALLENGE_TYPE_DURATION,
						},
					},
				},
				{
					Payload: &workoutexecutionv1message.StreamSoloMatchTrackingRequest_Frame{
						Frame: &workoutexecutionv1message.SoloTrackingFrame{
							Landmarks: validLandmarks,
							Timestamp: timestamppb.Now(),
						},
					},
				},
			},
		}

		err := h.StreamSoloMatchTracking(stream)
		if err != nil {
			t.Fatalf("unexpected stream error: %v", err)
		}

		if len(stream.sent) != 2 {
			t.Fatalf("expected 2 responses sent (1 config ack + 1 frame), got %d", len(stream.sent))
		}

		resp := stream.sent[0]
		if resp.MyStatus != workoutexecutionv1message.ParticipantStreamStatus_PARTICIPANT_STREAM_STATUS_WAITING {
			t.Errorf("expected WAITING status in initial config ack, got %v", resp.MyStatus)
		}
		if resp.MyConfirmedValue != 0 {
			t.Errorf("expected 0 confirmed value while waiting, got %d", resp.MyConfirmedValue)
		}
	})

	t.Run("Config without plan_id is rejected with InvalidArgument", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), middleware.UserIDKey, "user-no-plan")
		stream := &mockStreamServer{
			ctx: ctx,
			queue: []*workoutexecutionv1message.StreamSoloMatchTrackingRequest{
				{
					Payload: &workoutexecutionv1message.StreamSoloMatchTrackingRequest_Config{
						Config: &workoutexecutionv1message.SoloTrackingConfig{
							MatchId:       "match-no-plan",
							PlanId:        "", // empty plan_id
							ExerciseId:    "exercise_plank_01",
							ChallengeType: workoutexecutionv1message.SoloChallengeType_SOLO_CHALLENGE_TYPE_DURATION,
						},
					},
				},
			},
		}

		err := h.StreamSoloMatchTracking(stream)
		if err == nil {
			t.Fatal("expected missing plan_id to be rejected, got nil")
		}
		st, ok := status.FromError(err)
		if !ok || st.Code() != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument error, got %v (code: %v)", err, st.Code())
		}
	})

	t.Run("Config without challenge_type defaults to REPS gracefully", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), middleware.UserIDKey, "user-no-ct")
		stream := &mockStreamServer{
			ctx: ctx,
			queue: []*workoutexecutionv1message.StreamSoloMatchTrackingRequest{
				{
					Payload: &workoutexecutionv1message.StreamSoloMatchTrackingRequest_Config{
						Config: &workoutexecutionv1message.SoloTrackingConfig{
							MatchId:       "match-no-ct",
							PlanId:        "plan-valid",
							ExerciseId:    "exercise_plank_01",
							ChallengeType: workoutexecutionv1message.SoloChallengeType_SOLO_CHALLENGE_TYPE_UNSPECIFIED,
						},
					},
				},
			},
		}

		err := h.StreamSoloMatchTracking(stream)
		if err != nil {
			t.Fatalf("expected unspecified challenge_type to default gracefully to REPS, got error: %v", err)
		}
	})

	t.Run("User1 submits while opponent active transitions to SPECTATING (Case 1), then User2 submits ending match (Case 2)", func(t *testing.T) {
		matchID := "match-submit-test"
		outbox.events = nil

		// Register opponent as ACTIVE
		_ = liveSync.RegisterParticipant(context.Background(), matchID, "user-p2", "exercise_plank_01")
		_ = liveSync.PublishMyProgress(context.Background(), matchID, port.ParticipantProgress{
			UserID:    "user-p2",
			Value:     15,
			FormScore: 90.0,
			Status:    "ACTIVE",
		})

		// User1 sends Config, 1 Frame, then Submit
		ctxP1 := context.WithValue(context.Background(), middleware.UserIDKey, "user-p1")
		streamP1 := &mockStreamServer{
			ctx: ctxP1,
			queue: []*workoutexecutionv1message.StreamSoloMatchTrackingRequest{
				{
					Payload: &workoutexecutionv1message.StreamSoloMatchTrackingRequest_Config{
						Config: &workoutexecutionv1message.SoloTrackingConfig{
							MatchId:            matchID,
							ExerciseId:         "exercise_plank_01",
							ChallengeType:      workoutexecutionv1message.SoloChallengeType_SOLO_CHALLENGE_TYPE_DURATION,
							TimeCeilingSeconds: 1800,
							PlanId:             "plan-p1-plank",
						},
					},
				},
				{
					Payload: &workoutexecutionv1message.StreamSoloMatchTrackingRequest_Submit{
						Submit: &workoutexecutionv1message.SoloTrackingSubmit{},
					},
				},
			},
		}

		err := h.StreamSoloMatchTracking(streamP1)
		if err != nil {
			t.Fatalf("unexpected error for streamP1: %v", err)
		}

		if len(streamP1.sent) == 0 {
			t.Fatalf("expected streamP1 to receive response")
		}

		// Case 1: User1 submitted while User2 active -> SPECTATING, MatchEnded = false
		lastP1 := streamP1.sent[len(streamP1.sent)-1]
		if lastP1.MyStatus != workoutexecutionv1message.ParticipantStreamStatus_PARTICIPANT_STREAM_STATUS_SPECTATING {
			t.Errorf("expected User1 status SPECTATING, got %v", lastP1.MyStatus)
		}
		if !lastP1.IsCompleted {
			t.Errorf("expected User1 IsCompleted to be true, got false")
		}
		if lastP1.MatchEnded {
			t.Errorf("expected User1 MatchEnded to be false while opponent active, got true")
		}

		// Case 2: Now User2 also submits -> Both submitted -> MatchEnded = true
		ctxP2 := context.WithValue(context.Background(), middleware.UserIDKey, "user-p2")
		streamP2 := &mockStreamServer{
			ctx: ctxP2,
			queue: []*workoutexecutionv1message.StreamSoloMatchTrackingRequest{
				{
					Payload: &workoutexecutionv1message.StreamSoloMatchTrackingRequest_Config{
						Config: &workoutexecutionv1message.SoloTrackingConfig{
							MatchId:            matchID,
							ExerciseId:         "exercise_plank_01",
							ChallengeType:      workoutexecutionv1message.SoloChallengeType_SOLO_CHALLENGE_TYPE_DURATION,
							TimeCeilingSeconds: 1800,
							PlanId:             "plan-p2-plank",
						},
					},
				},
				{
					Payload: &workoutexecutionv1message.StreamSoloMatchTrackingRequest_Submit{
						Submit: &workoutexecutionv1message.SoloTrackingSubmit{},
					},
				},
			},
		}

		err = h.StreamSoloMatchTracking(streamP2)
		if err != nil {
			t.Fatalf("unexpected error for streamP2: %v", err)
		}

		if len(streamP2.sent) == 0 {
			t.Fatalf("expected streamP2 to receive response")
		}

		lastP2 := streamP2.sent[len(streamP2.sent)-1]
		if lastP2.MyStatus != workoutexecutionv1message.ParticipantStreamStatus_PARTICIPANT_STREAM_STATUS_COMPLETED {
			t.Errorf("expected User2 status COMPLETED, got %v", lastP2.MyStatus)
		}
		if !lastP2.IsCompleted {
			t.Errorf("expected User2 IsCompleted to be true, got false")
		}
		if !lastP2.MatchEnded {
			t.Errorf("expected User2 MatchEnded to be true when both submitted, got false")
		}

		// Verify Outbox events were written
		if len(outbox.events) == 0 {
			t.Errorf("expected outbox events to be published upon submission")
		}
	})

	t.Run("Transient stream disconnect does not abort session, allows reconnect with progress restored", func(t *testing.T) {
		matchID := "match-reconnect-test"
		userID := "user-reconnector"
		ctx := context.WithValue(context.Background(), middleware.UserIDKey, userID)
		outbox.events = nil

		// Register opponent so room is ready
		_ = liveSync.RegisterParticipant(context.Background(), matchID, "user-opp-rec", "exercise_plank_01")
		_ = liveSync.PublishMyProgress(context.Background(), matchID, port.ParticipantProgress{
			UserID:    "user-opp-rec",
			Value:     10,
			FormScore: 88.0,
			Status:    "ACTIVE",
		})

		validLandmarks := []*workoutexecutionv1message.SkeletonLandmark{
			{LandmarkIndex: 11, X: -1, Y: 0, Z: 0, Visibility: 0.9},
			{LandmarkIndex: 23, X: 0, Y: 0, Z: 0, Visibility: 0.9},
			{LandmarkIndex: 27, X: 1, Y: 0, Z: 0, Visibility: 0.9},
		}

		// 1. Initial stream: Send Config + 2 valid frames
		stream1 := &mockStreamServer{
			ctx: ctx,
			queue: []*workoutexecutionv1message.StreamSoloMatchTrackingRequest{
				{
					Payload: &workoutexecutionv1message.StreamSoloMatchTrackingRequest_Config{
						Config: &workoutexecutionv1message.SoloTrackingConfig{
							MatchId:            matchID,
							ExerciseId:         "exercise_plank_01",
							ChallengeType:      workoutexecutionv1message.SoloChallengeType_SOLO_CHALLENGE_TYPE_DURATION,
							TimeCeilingSeconds: 1800,
							PlanId:             "plan-reconnect-101",
						},
					},
				},
				{
					Payload: &workoutexecutionv1message.StreamSoloMatchTrackingRequest_Frame{
						Frame: &workoutexecutionv1message.SoloTrackingFrame{
							Landmarks: validLandmarks,
							Timestamp: timestamppb.Now(),
						},
					},
				},
				{
					Payload: &workoutexecutionv1message.StreamSoloMatchTrackingRequest_Frame{
						Frame: &workoutexecutionv1message.SoloTrackingFrame{
							Landmarks: validLandmarks,
							Timestamp: timestamppb.Now(),
						},
					},
				},
			},
		}

		// Stream 1 terminates naturally with EOF (simulating network drop / socket close without abort)
		err := h.StreamSoloMatchTracking(stream1)
		if err != nil {
			t.Fatalf("unexpected stream1 error: %v", err)
		}

		// Verify:
		// 1) NO SoloTrackingAborted event in outbox!
		for _, ev := range outbox.events {
			if _, isAbort := ev.(*domainEvent.SoloTrackingAborted); isAbort {
				t.Fatalf("expected NO SoloTrackingAborted event on transient disconnect, but got one: %+v", ev)
			}
		}

		// 2) Redis live sync should record user as DISCONNECTED with a DisconnectedAt timestamp
		prog, progErr := liveSync.GetMyProgress(ctx, matchID, userID)
		if progErr != nil {
			t.Fatalf("failed to get participant progress after disconnect: %v", progErr)
		}
		if prog == nil || prog.Status != "DISCONNECTED" {
			t.Fatalf("expected status DISCONNECTED in redis, got %+v", prog)
		}
		if prog.DisconnectedAt == 0 {
			t.Fatalf("expected non-zero DisconnectedAt timestamp")
		}

		// Simulate previous confirmed progress recorded before disconnection (e.g. 15s elapsed duration)
		const previousScore int32 = 15
		_ = liveSync.PublishMyProgress(ctx, matchID, port.ParticipantProgress{
			UserID:         userID,
			Value:          previousScore,
			FormScore:      90.0,
			Status:         "DISCONNECTED",
			DisconnectedAt: prog.DisconnectedAt,
		})

		// 3. User reconnects (new stream with identical config)
		stream2 := &mockStreamServer{
			ctx: ctx,
			queue: []*workoutexecutionv1message.StreamSoloMatchTrackingRequest{
				{
					Payload: &workoutexecutionv1message.StreamSoloMatchTrackingRequest_Config{
						Config: &workoutexecutionv1message.SoloTrackingConfig{
							MatchId:            matchID,
							ExerciseId:         "exercise_plank_01",
							ChallengeType:      workoutexecutionv1message.SoloChallengeType_SOLO_CHALLENGE_TYPE_DURATION,
							TimeCeilingSeconds: 1800,
							PlanId:             "plan-reconnect-101",
						},
					},
				},
				{
					Payload: &workoutexecutionv1message.StreamSoloMatchTrackingRequest_Frame{
						Frame: &workoutexecutionv1message.SoloTrackingFrame{
							Landmarks: validLandmarks,
							Timestamp: timestamppb.Now(),
						},
					},
				},
			},
		}

		err = h.StreamSoloMatchTracking(stream2)
		if err != nil {
			t.Fatalf("unexpected stream2 error: %v", err)
		}

		if len(stream2.sent) == 0 {
			t.Fatalf("expected responses sent in stream2")
		}

		// Confirm that progress was restored and continued accumulated
		reconnectResp := stream2.sent[0]
		if reconnectResp.MyConfirmedValue < previousScore {
			t.Errorf("expected restored MyConfirmedValue >= %d, got %d", previousScore, reconnectResp.MyConfirmedValue)
		}
		if reconnectResp.MyStatus != workoutexecutionv1message.ParticipantStreamStatus_PARTICIPANT_STREAM_STATUS_ACTIVE {
			t.Errorf("expected reconnected status ACTIVE, got %v", reconnectResp.MyStatus)
		}
	})
}
