//go:build e2e || integration

package e2e

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/application/command"
	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	"github.com/viethung213/gym-companion/internal/coaching/application/query"
	"github.com/viethung213/gym-companion/internal/coaching/domain/event"
	"github.com/viethung213/gym-companion/internal/coaching/domain/guardrail"
	"github.com/viethung213/gym-companion/internal/coaching/domain/roadmap"
	"github.com/viethung213/gym-companion/internal/coaching/domain/service"
	coachingGrpc "github.com/viethung213/gym-companion/internal/coaching/transport/grpc"
	pbsvc "github.com/viethung213/gym-companion/internal/gen/go/contracts/core/coaching/v1/service"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type TestSuite struct {
	GRPCConn   *grpc.ClientConn
	Client     pbsvc.CoachingServiceClient
	StopServer func()
}

type fakeClock struct{ t time.Time }

func (c *fakeClock) Now() time.Time { return c.t }

type fakeTx struct{}

func (fakeTx) WithTransaction(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

type memRepo struct {
	byID       map[string]*roadmap.Roadmap
	activeByID map[string]string
}

func newMemRepo() *memRepo {
	return &memRepo{byID: map[string]*roadmap.Roadmap{}, activeByID: map[string]string{}}
}

func (m *memRepo) Save(_ context.Context, r *roadmap.Roadmap) error {
	m.byID[r.ID()] = r
	if r.Status() == roadmap.StatusActive {
		m.activeByID[r.UserID()] = r.ID()
	}
	return nil
}

func (m *memRepo) FindByID(_ context.Context, id string) (*roadmap.Roadmap, error) {
	if r, ok := m.byID[id]; ok {
		return r, nil
	}
	return nil, roadmap.ErrRoadmapNotFound
}

func (m *memRepo) FindActiveByUser(_ context.Context, userID string) (*roadmap.Roadmap, error) {
	if id, ok := m.activeByID[userID]; ok {
		return m.byID[id], nil
	}
	return nil, roadmap.ErrRoadmapNotFound
}

func (m *memRepo) ListByUser(_ context.Context, userID string, _ roadmap.Status, _, _ int) ([]*roadmap.Roadmap, error) {
	var out []*roadmap.Roadmap
	for _, r := range m.byID {
		if r.UserID() == userID {
			out = append(out, r)
		}
	}
	return out, nil
}

func (m *memRepo) FindSessionByID(_ context.Context, sid string) (*roadmap.Roadmap, error) {
	for _, r := range m.byID {
		if _, ok := r.FindSession(sid); ok {
			return r, nil
		}
	}
	return nil, roadmap.ErrSessionNotFound
}

func (m *memRepo) FindPendingSessionsByDate(_ context.Context, _ time.Time) ([]*roadmap.SessionPlanInfo, error) {
	return nil, nil
}

type captureOutbox struct {
	events []event.Event
}

func (c *captureOutbox) Enqueue(_ context.Context, _ string, events ...event.Event) error {
	c.events = append(c.events, events...)
	return nil
}

type fakeCoachAgent struct {
	t time.Time
}

func (f *fakeCoachAgent) GenerateRoadmap(_ context.Context, userID string) (*roadmap.Roadmap, error) {
	info := &roadmap.Info{
		RoadmapID: "roadmap-e2e-1",
		UserID:    userID,
		Status:    roadmap.StatusActive,
		StartDate: f.t,
		EndDate:   f.t.AddDate(0, 0, 28),
		CreatedAt: f.t,
		UpdatedAt: f.t,
	}
	var weeks []*roadmap.WeekPlan
	for i := 1; i <= 4; i++ {
		var phase roadmap.Phase
		switch i {
		case 2:
			phase = roadmap.PhaseOverload
		case 3:
			phase = roadmap.PhasePeak
		case 4:
			phase = roadmap.PhaseDeload
		default:
			phase = roadmap.PhaseAccumulation
		}
		weekInfo := &roadmap.WeekPlanInfo{
			WeekPlanID: fmt.Sprintf("week-%d", i),
			RoadmapID:  "roadmap-e2e-1",
			UserID:     userID,
			WeekNumber: int32(i),
			Phase:      phase,
			TargetRPE:  6.5,
			StartDate:  f.t.AddDate(0, 0, (i-1)*7),
			EndDate:    f.t.AddDate(0, 0, i*7),
		}
		wp, _ := roadmap.NewWeekPlan(weekInfo)
		weeks = append(weeks, wp)
	}
	return roadmap.NewRoadmap(info, weeks, f.t)
}

func (f *fakeCoachAgent) RegeneratePending(_ context.Context, _ string, _ []string) ([]*roadmap.SessionPlanInfo, error) {
	return nil, nil
}

func (f *fakeCoachAgent) Adapt(_ context.Context, _, _ string) ([]*roadmap.SessionPlanInfo, error) {
	return nil, nil
}

func (f *fakeCoachAgent) SuggestAdHocSession(_ context.Context, _ string, _ *port.AdHocHint) (port.SuggestedSession, error) {
	return port.SuggestedSession{}, nil
}

func SetupCoachingE2ESuite(t *testing.T) *TestSuite {
	t.Helper()

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to listen on random port: %v", err)
	}

	clock := &fakeClock{t: time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)}
	repo := newMemRepo()
	outbox := &captureOutbox{}
	mockAgent := &fakeCoachAgent{t: clock.t}
	guard := guardrail.NewEngine(service.NewOverloadValidator(), nil, nil)

	initiateHandler := command.NewInitiateRoadmapHandler(fakeTx{}, repo, mockAgent, guard, outbox, clock)
	regenerateHandler := command.NewRegenerateScheduleHandler(fakeTx{}, repo, mockAgent, guard, outbox, clock)
	createAdhocHandler := command.NewCreateAdhocSessionHandler(fakeTx{}, repo, nil, nil, clock)
	queriesHandler := query.NewHandlers(repo)

	grpcServer := grpc.NewServer()
	srv := coachingGrpc.NewServer(initiateHandler, regenerateHandler, createAdhocHandler, queriesHandler)
	pbsvc.RegisterCoachingServiceServer(grpcServer, srv)

	go func() {
		if serveErr := grpcServer.Serve(lis); serveErr != nil {
			t.Logf("gRPC server exited: %v", serveErr)
		}
	}()

	conn, err := grpc.NewClient(
		lis.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		grpcServer.Stop()
		t.Fatalf("Failed to connect to test gRPC server: %v", err)
	}

	client := pbsvc.NewCoachingServiceClient(conn)

	return &TestSuite{
		GRPCConn: conn,
		Client:   client,
		StopServer: func() {
			conn.Close()
			grpcServer.GracefulStop()
			lis.Close()
		},
	}
}
