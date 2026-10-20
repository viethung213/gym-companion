package grpc_test

import (
	"context"
	"testing"
	"time"

	solomatchv1message "github.com/viethung213/gym-companion/internal/gen/go/contracts/core/solo_match/v1/message"
	"github.com/viethung213/gym-companion/internal/shared/middleware"
	"github.com/viethung213/gym-companion/internal/solo_match/application/command"
	"github.com/viethung213/gym-companion/internal/solo_match/application/port"
	"github.com/viethung213/gym-companion/internal/solo_match/application/query"
	"github.com/viethung213/gym-companion/internal/solo_match/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/solo_match/domain/entity"
	valueobject "github.com/viethung213/gym-companion/internal/solo_match/domain/value_object"
	transportgrpc "github.com/viethung213/gym-companion/internal/solo_match/transport/grpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type mockMatchRepo struct {
	matches     map[string]*aggregate.SoloMatch
	activeMatch *aggregate.SoloMatch
}

func (m *mockMatchRepo) Save(ctx context.Context, match *aggregate.SoloMatch) error {
	m.matches[match.ID()] = match
	return nil
}

func (m *mockMatchRepo) FindByID(ctx context.Context, id string) (*aggregate.SoloMatch, error) {
	return m.matches[id], nil
}

func (m *mockMatchRepo) FindActiveByUserID(ctx context.Context, userID string) (*aggregate.SoloMatch, error) {
	if m.activeMatch != nil {
		return m.activeMatch, nil
	}
	return nil, nil
}

func (m *mockMatchRepo) FindExpiredMatches(ctx context.Context, now time.Time, limit int) ([]*aggregate.SoloMatch, error) {
	return nil, nil
}

func (m *mockMatchRepo) ListByUserID(ctx context.Context, userID string, offset, limit int) ([]*aggregate.SoloMatch, error) {
	var res []*aggregate.SoloMatch
	for _, match := range m.matches {
		for _, p := range match.Participants() {
			if p.UserID() == userID {
				res = append(res, match)
				break
			}
		}
	}
	return res, nil
}

type mockSnapshotRepo struct {
	snapshots map[string]*entity.UserSnapshot
}

func (s *mockSnapshotRepo) Upsert(ctx context.Context, snap *entity.UserSnapshot) error {
	s.snapshots[snap.UserID()] = snap
	return nil
}

func (s *mockSnapshotRepo) FindByUserID(ctx context.Context, userID string) (*entity.UserSnapshot, error) {
	return s.snapshots[userID], nil
}

func (s *mockSnapshotRepo) FindByUserIDs(ctx context.Context, userIDs []string) (map[string]*entity.UserSnapshot, error) {
	res := make(map[string]*entity.UserSnapshot)
	for _, id := range userIDs {
		if snap, ok := s.snapshots[id]; ok {
			res[id] = snap
		}
	}
	return res, nil
}

type mockQueuePort struct {
	eventCh chan *port.MatchFoundEvent
}

func (q *mockQueuePort) Enqueue(ctx context.Context, ticket *aggregate.MatchmakingTicket) error {
	return nil
}

func (q *mockQueuePort) Cancel(ctx context.Context, ticketID string, userID string) error {
	return nil
}

func (q *mockQueuePort) ListenMatch(ctx context.Context, ticketID string) (<-chan *port.MatchFoundEvent, error) {
	return q.eventCh, nil
}

func (q *mockQueuePort) Close() error {
	return nil
}

type mockStreamServer struct {
	grpc.ServerStreamingServer[solomatchv1message.JoinQueueResponse]
	ctx      context.Context
	sentMsgs []*solomatchv1message.JoinQueueResponse
}

func (s *mockStreamServer) Context() context.Context {
	return s.ctx
}

func (s *mockStreamServer) Send(m *solomatchv1message.JoinQueueResponse) error {
	s.sentMsgs = append(s.sentMsgs, m)
	return nil
}

func TestGRPCHandler(t *testing.T) {
	ctx := context.WithValue(context.Background(), middleware.UserIDKey, "user_1")
	now := time.Now().UTC()

	matchRepo := &mockMatchRepo{matches: make(map[string]*aggregate.SoloMatch)}
	snapshotRepo := &mockSnapshotRepo{snapshots: make(map[string]*entity.UserSnapshot)}
	eventCh := make(chan *port.MatchFoundEvent, 1)
	queuePort := &mockQueuePort{eventCh: eventCh}

	joinQueueHandler := command.NewJoinMatchmakingQueueHandler(queuePort, matchRepo)
	cancelQueueHandler := command.NewCancelMatchmakingHandler(queuePort)
	getMatchHandler := query.NewGetSoloMatchByIDHandler(matchRepo, snapshotRepo)
	getHistoryHandler := query.NewGetUserMatchHistoryHandler(matchRepo, snapshotRepo)

	handler := transportgrpc.NewGRPCHandler(
		joinQueueHandler,
		cancelQueueHandler,
		getMatchHandler,
		getHistoryHandler,
		snapshotRepo,
	)

	// 0. JoinQueue rejects when active match exists
	pActive1, _ := entity.NewMatchParticipant("p_active_1", "m_active", "user_1")
	pActive2, _ := entity.NewMatchParticipant("p_active_2", "m_active", "user_2")
	activeMatch, _ := aggregate.NewSoloMatch("m_active", "ex_1", "Push Up", valueobject.ChallengeTypeReps, 15*time.Minute, []*entity.MatchParticipant{pActive1, pActive2}, now)
	matchRepo.activeMatch = activeMatch

	activeErr := handler.JoinQueue(&solomatchv1message.JoinQueueRequest{}, &mockStreamServer{ctx: ctx})
	if status.Code(activeErr) != codes.FailedPrecondition {
		t.Fatalf("expected FailedPrecondition when active match exists, got: %v", activeErr)
	}
	matchRepo.activeMatch = nil

	// 1. JoinQueue Stream
	streamCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	stream := &mockStreamServer{ctx: streamCtx}

	p1, _ := entity.NewMatchParticipant("p_1", "m_1", "user_1")
	p2, _ := entity.NewMatchParticipant("p_2", "m_1", "user_2")
	match, _ := aggregate.NewSoloMatch("m_1", "ex_1", "Push Up", valueobject.ChallengeTypeReps, 15*time.Minute, []*entity.MatchParticipant{p1, p2}, now)
	_ = matchRepo.Save(ctx, match)

	// Bắn sự kiện match found sau 100ms
	go func() {
		time.Sleep(100 * time.Millisecond)
		eventCh <- &port.MatchFoundEvent{
			MatchID: "m_1",
			Match:   match,
		}
	}()

	err := handler.JoinQueue(&solomatchv1message.JoinQueueRequest{}, stream)
	if err != nil {
		t.Fatalf("JoinQueue err: %v", err)
	}

	if len(stream.sentMsgs) < 2 {
		t.Errorf("expected at least 2 stream messages (QUEUED and MATCHED), got %d", len(stream.sentMsgs))
	}
	lastMsg := stream.sentMsgs[len(stream.sentMsgs)-1]
	if lastMsg.Type != solomatchv1message.MatchStreamEventType_MATCH_STREAM_EVENT_TYPE_MATCHED {
		t.Errorf("expected MATCHED type, got %v", lastMsg.Type)
	}

	// 2. CancelQueue
	cancelResp, err := handler.CancelQueue(ctx, &solomatchv1message.CancelQueueRequest{TicketId: "t_1"})
	if err != nil || !cancelResp.Success {
		t.Fatalf("CancelQueue err=%v, resp=%+v", err, cancelResp)
	}

	// 3. GetMatch
	getResp, err := handler.GetMatch(ctx, &solomatchv1message.GetMatchRequest{MatchId: "m_1"})
	if err != nil || getResp.Match == nil || getResp.Match.MatchId != "m_1" {
		t.Fatalf("GetMatch err=%v, resp=%+v", err, getResp)
	}

	// 4. GetMatchHistory
	histResp, err := handler.GetMatchHistory(ctx, &solomatchv1message.GetMatchHistoryRequest{PageSize: 10})
	if err != nil || len(histResp.Matches) != 1 {
		t.Fatalf("GetMatchHistory err=%v, count=%d", err, len(histResp.Matches))
	}
}
