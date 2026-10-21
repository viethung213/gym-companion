package grpc_test

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"
	solomatchv1message "github.com/viethung213/gym-companion/internal/gen/go/contracts/core/solo_match/v1/message"
	"github.com/viethung213/gym-companion/internal/shared/middleware"
	"github.com/viethung213/gym-companion/internal/solo_match/application/command"
	"github.com/viethung213/gym-companion/internal/solo_match/application/port"
	"github.com/viethung213/gym-companion/internal/solo_match/application/query"
	"github.com/viethung213/gym-companion/internal/solo_match/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/solo_match/domain/entity"
	valueobject "github.com/viethung213/gym-companion/internal/solo_match/domain/value_object"
	transportgrpc "github.com/viethung213/gym-companion/internal/solo_match/transport/grpc"
)

func TestConnectSoloMatchHandler(t *testing.T) {
	mockQueue := &mockQueuePort{
		eventCh: make(chan *port.MatchFoundEvent, 1),
	}
	mockMatchRepo := &mockMatchRepo{
		matches: make(map[string]*aggregate.SoloMatch),
	}
	mockSnapshotRepo := &mockSnapshotRepo{
		snapshots: make(map[string]*entity.UserSnapshot),
	}

	joinHandler := command.NewJoinMatchmakingQueueHandler(mockQueue, mockMatchRepo)
	cancelHandler := command.NewCancelMatchmakingHandler(mockQueue)
	getMatchHandler := query.NewGetSoloMatchByIDHandler(mockMatchRepo, mockSnapshotRepo)
	getHistoryHandler := query.NewGetUserMatchHistoryHandler(mockMatchRepo, mockSnapshotRepo)

	grpcHandler := transportgrpc.NewHandler(
		joinHandler,
		cancelHandler,
		getMatchHandler,
		getHistoryHandler,
		mockSnapshotRepo,
	)

	connectHandler := transportgrpc.NewConnectSoloMatchHandler(grpcHandler)

	ctx := context.WithValue(context.Background(), middleware.UserIDKey, "user_test_1")

	// 1. CancelQueue test
	cancelReq := connect.NewRequest(&solomatchv1message.CancelQueueRequest{
		TicketId: "ticket_123",
	})
	cancelRes, err := connectHandler.CancelQueue(ctx, cancelReq)
	if err != nil {
		t.Fatalf("unexpected cancel queue err: %v", err)
	}
	if !cancelRes.Msg.Success {
		t.Errorf("expected success true")
	}

	// 2. GetMatch test (not found)
	getMatchReq := connect.NewRequest(&solomatchv1message.GetMatchRequest{
		MatchId: "non_existent",
	})
	_, err = connectHandler.GetMatch(ctx, getMatchReq)
	if err == nil {
		t.Fatalf("expected error for non existent match")
	}

	// 3. GetMatch with found match
	now := time.Now().UTC()
	p1, _ := entity.NewMatchParticipant("p1", "match_found", "user_test_1")
	p2, _ := entity.NewMatchParticipant("p2", "match_found", "usr_bot_001")
	match, err := aggregate.NewSoloMatch(
		"match_found",
		"ex_pushup",
		"Push Up Challenge",
		valueobject.ChallengeTypeReps,
		120*time.Second,
		[]*entity.MatchParticipant{p1, p2},
		now,
	)
	if err != nil {
		t.Fatalf("create match: %v", err)
	}
	mockMatchRepo.matches["match_found"] = match

	snap, _ := entity.NewUserSnapshot("user_test_1", "Alice", "", now)
	mockSnapshotRepo.snapshots["user_test_1"] = snap

	getMatchRes, err := connectHandler.GetMatch(ctx, connect.NewRequest(&solomatchv1message.GetMatchRequest{
		MatchId: "match_found",
	}))
	if err != nil {
		t.Fatalf("unexpected get match err: %v", err)
	}
	if getMatchRes.Msg.Match.MatchId != "match_found" {
		t.Errorf("got match id %s, want match_found", getMatchRes.Msg.Match.MatchId)
	}

	// 4. GetMatchHistory test
	historyRes, err := connectHandler.GetMatchHistory(ctx, connect.NewRequest(&solomatchv1message.GetMatchHistoryRequest{
		PageSize: 10,
	}))
	if err != nil {
		t.Fatalf("unexpected get history err: %v", err)
	}
	if len(historyRes.Msg.Matches) != 1 {
		t.Errorf("got %d matches, want 1", len(historyRes.Msg.Matches))
	}
}
