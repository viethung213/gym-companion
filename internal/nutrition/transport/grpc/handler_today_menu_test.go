package grpc_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	_ "time/tzdata"

	nutritionv1msg "github.com/viethung213/gym-companion/internal/gen/go/contracts/core/nutrition/v1/message"
	"github.com/viethung213/gym-companion/internal/nutrition/application/command"
	"github.com/viethung213/gym-companion/internal/nutrition/application/query"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/repository"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/service"
	"github.com/viethung213/gym-companion/internal/nutrition/transport/grpc"
	"github.com/viethung213/gym-companion/internal/shared/middleware"
)

// syncPlanRepo makes the transport plan-repo mock safe for concurrent callers.
type syncPlanRepo struct {
	mu sync.Mutex
	mockPlanRepoForTransport
}

func (r *syncPlanRepo) FindByUserIDAndDate(ctx context.Context, userID string, date time.Time) (*aggregate.NutritionPlan, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.mockPlanRepoForTransport.FindByUserIDAndDate(ctx, userID, date)
}

func (r *syncPlanRepo) Save(ctx context.Context, plan *aggregate.NutritionPlan) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.mockPlanRepoForTransport.Save(ctx, plan)
}

func (r *syncPlanRepo) Update(ctx context.Context, plan *aggregate.NutritionPlan) error {
	return r.Save(ctx, plan)
}

// gatedAIService counts LLM calls and blocks each one until release is closed.
type gatedAIService struct {
	mockAIServiceForTransport
	calls   atomic.Int32
	started chan struct{}
	once    sync.Once
	release chan struct{}
}

func newGatedAIService() *gatedAIService {
	return &gatedAIService{started: make(chan struct{}), release: make(chan struct{})}
}

func (g *gatedAIService) SelectCreativeMealOptions(
	ctx context.Context,
	prompt repository.AIMenuPromptContext,
) ([]repository.GeneratedRecipeResult, error) {
	g.calls.Add(1)
	g.once.Do(func() { close(g.started) })
	<-g.release
	return g.mockAIServiceForTransport.SelectCreativeMealOptions(ctx, prompt)
}

func newTodayMenuHandler(planRepo *syncPlanRepo, ai *gatedAIService) *grpc.GRPCHandler {
	historyRepo := &mockHistoryRepoForTransport{}
	foodRepo := &mockFoodRepoForTransport{items: make(map[string]*aggregate.FoodItem)}
	menuGen := service.NewMenuGenerator(&mockRecipeCacheRepoForTransport{}, foodRepo, ai)
	return grpc.NewGRPCHandler(
		command.NewGenerateDailyPlanHandler(planRepo, historyRepo, nil, service.NewTDEECalculator(), menuGen),
		command.NewRecalibratePlanWithPantryHandler(planRepo, historyRepo, nil, menuGen),
		command.NewLogMealHandler(planRepo, historyRepo, nil, ai),
		command.NewCreateFoodItemHandler(foodRepo),
		command.NewApproveFoodItemHandler(foodRepo),
		command.NewUpdateMealScheduleHandler(planRepo),
		query.NewGetTodayMenuHandler(planRepo),
		query.NewGetNutritionHistoryHandler(historyRepo),
		query.NewGetNutritionSummaryHandler(planRepo, historyRepo),
		query.NewGetNutritionInsightHandler(nil),
	)
}

func newSyncPlanRepo() *syncPlanRepo {
	return &syncPlanRepo{mockPlanRepoForTransport: mockPlanRepoForTransport{plans: make(map[string]*aggregate.NutritionPlan)}}
}

func TestGetTodayMenu_ConcurrentCallsShareOneGeneration(t *testing.T) {
	// Baseline: how many LLM calls one generation makes.
	baseline := newGatedAIService()
	close(baseline.release)
	baseCtx := middleware.WithActor(context.Background(), "baseline-user", "User")
	if _, err := newTodayMenuHandler(newSyncPlanRepo(), baseline).GetTodayMenu(baseCtx, &nutritionv1msg.GetTodayMenuRequest{}); err != nil {
		t.Fatalf("baseline GetTodayMenu: %v", err)
	}
	perGeneration := baseline.calls.Load()
	if perGeneration == 0 {
		t.Fatal("baseline generation made no LLM calls")
	}

	ai := newGatedAIService()
	handler := newTodayMenuHandler(newSyncPlanRepo(), ai)
	userCtx := middleware.WithActor(context.Background(), "user-1", "User")

	const callers = 5
	errs := make(chan error, callers)
	var wg sync.WaitGroup
	call := func() {
		defer wg.Done()
		_, err := handler.GetTodayMenu(userCtx, &nutritionv1msg.GetTodayMenuRequest{})
		errs <- err
	}
	wg.Add(1)
	go call()
	<-ai.started // first generation is in flight
	for range callers - 1 {
		wg.Add(1)
		go call()
	}
	time.Sleep(50 * time.Millisecond) // let the other callers join the in-flight generation
	close(ai.release)
	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatalf("GetTodayMenu: %v", err)
		}
	}
	if got := ai.calls.Load(); got != perGeneration {
		t.Fatalf("got %d LLM calls for %d concurrent callers, want %d (one generation)", got, callers, perGeneration)
	}
}

func TestGetTodayMenu_CallerCancelStillSavesPlan(t *testing.T) {
	planRepo := newSyncPlanRepo()
	ai := newGatedAIService()
	handler := newTodayMenuHandler(planRepo, ai)

	ctx, cancel := context.WithCancel(middleware.WithActor(context.Background(), "user-2", "User"))
	done := make(chan error, 1)
	go func() {
		_, err := handler.GetTodayMenu(ctx, &nutritionv1msg.GetTodayMenuRequest{})
		done <- err
	}()
	<-ai.started
	cancel()
	if err := <-done; err == nil {
		t.Fatal("got nil error for cancelled caller, want an error")
	}

	close(ai.release)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		plan, err := planRepo.FindByUserIDAndDate(context.Background(), "user-2", time.Now())
		if err != nil {
			t.Fatalf("FindByUserIDAndDate: %v", err)
		}
		if plan != nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("plan was not saved after the caller cancelled")
}

// fixedProfile reports one athlete's metrics and zone.
type fixedProfile struct {
	m repository.UserBiologicalMetrics
}

func (f fixedProfile) GetBiologicalMetrics(context.Context, string) (*repository.UserBiologicalMetrics, error) {
	m := f.m
	return &m, nil
}

func TestGetTodayMenu_UsesProfileMetricsAndZone(t *testing.T) {
	ctx := middleware.WithActor(context.Background(), "user-3", "User")
	open := func(client repository.ProfileClient) (*syncPlanRepo, float32) {
		repo := newSyncPlanRepo()
		ai := newGatedAIService()
		close(ai.release)
		h := newTodayMenuHandler(repo, ai).WithProfileClient(client)
		res, err := h.GetTodayMenu(ctx, &nutritionv1msg.GetTodayMenuRequest{})
		if err != nil {
			t.Fatalf("GetTodayMenu: %v", err)
		}
		return repo, res.TargetCalories
	}

	_, defaultKcal := open(nil)
	repo, profileKcal := open(fixedProfile{m: repository.UserBiologicalMetrics{
		WeightKg: 110, HeightCm: 190, Age: 30, Gender: "MALE", ActivityLevel: "VERY_ACTIVE", Timezone: "Pacific/Kiritimati",
	}})
	if profileKcal <= defaultKcal {
		t.Errorf("target kcal %v with a 110 kg very active athlete, want above the default-metrics %v", profileKcal, defaultKcal)
	}

	// UTC+14: the athlete's day is usually ahead of the server's.
	today := command.AthleteProfile{Location: command.LoadLocation("Pacific/Kiritimati")}.Today(time.Now())
	if plan, _ := repo.FindByUserIDAndDate(context.Background(), "user-3", today); plan == nil {
		t.Errorf("no plan saved for the athlete's day %s", today.Format(time.DateOnly))
	}
}
