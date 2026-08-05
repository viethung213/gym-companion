package transport

import (
	"context"
	"time"

	"connectrpc.com/connect"
	nutritionv1msg "github.com/viethung213/gym-companion/internal/gen/go/contracts/core/nutrition/v1/message"
	nutritionv1service "github.com/viethung213/gym-companion/internal/gen/go/contracts/core/nutrition/v1/service"
	"github.com/viethung213/gym-companion/internal/gen/go/contracts/core/nutrition/v1/service/nutritionv1serviceconnect"
	"github.com/viethung213/gym-companion/internal/nutrition/application/command"
	"github.com/viethung213/gym-companion/internal/nutrition/application/query"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/service"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GRPCHandler struct {
	nutritionv1service.UnimplementedNutritionServiceServer
	genPlanHdlr             *command.GenerateDailyPlanHandler
	recalPlanHdlr           *command.RecalibratePlanWithPantryHandler
	logMealHdlr             *command.LogMealHandler
	createFoodItemHdlr      *command.CreateFoodItemHandler
	approveFoodItemHandler  *command.ApproveFoodItemHandler
	getTodayMenuHdlr        *query.GetTodayMenuHandler
	getNutritionHistoryHdlr *query.GetNutritionHistoryHandler
	getNutritionSummaryHdlr *query.GetNutritionSummaryHandler
	getNutritionInsightHdlr *query.GetNutritionInsightHandler
}

func NewGRPCHandler(
	genPlanHdlr *command.GenerateDailyPlanHandler,
	recalPlanHdlr *command.RecalibratePlanWithPantryHandler,
	logMealHdlr *command.LogMealHandler,
	createFoodItemHdlr *command.CreateFoodItemHandler,
	approveFoodItemHandler *command.ApproveFoodItemHandler,
	getTodayMenuHdlr *query.GetTodayMenuHandler,
	getNutritionHistoryHdlr *query.GetNutritionHistoryHandler,
	getNutritionSummaryHdlr *query.GetNutritionSummaryHandler,
	getNutritionInsightHdlr *query.GetNutritionInsightHandler,
) *GRPCHandler {
	return &GRPCHandler{
		genPlanHdlr:             genPlanHdlr,
		recalPlanHdlr:           recalPlanHdlr,
		logMealHdlr:             logMealHdlr,
		createFoodItemHdlr:      createFoodItemHdlr,
		approveFoodItemHandler:  approveFoodItemHandler,
		getTodayMenuHdlr:        getTodayMenuHdlr,
		getNutritionHistoryHdlr: getNutritionHistoryHdlr,
		getNutritionSummaryHdlr: getNutritionSummaryHdlr,
		getNutritionInsightHdlr: getNutritionInsightHdlr,
	}
}

func (h *GRPCHandler) GetTodayMenu(ctx context.Context, req *nutritionv1msg.GetTodayMenuRequest) (*nutritionv1msg.GetTodayMenuResponse, error) {
	if req.GetUserId() == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	now := time.Now()
	plan, err := h.getTodayMenuHdlr.Handle(ctx, query.GetTodayMenuQuery{
		UserID:   req.GetUserId(),
		PlanDate: now,
	})
	if err != nil || plan == nil {
		bio := service.BiologicalMetrics{
			WeightKg: 70.0,
			HeightCm: 170.0,
			Age:      25,
			Gender:   "MALE",
		}

		genPlan, genErr := h.genPlanHdlr.Handle(ctx, command.GenerateDailyPlanCommand{
			UserID:            req.GetUserId(),
			PlanDate:          now,
			BiologicalMetrics: bio,
			UserRestrictions:  nil,
		})
		if genErr != nil {
			return nil, status.Errorf(codes.Internal, "failed to generate daily menu: %v", genErr)
		}
		plan = genPlan
	}

	alloc := plan.CalorieAllocation()
	breakfastOptions := make([]*nutritionv1msg.MealOption, 0)
	lunchOptions := make([]*nutritionv1msg.MealOption, 0)
	dinnerOptions := make([]*nutritionv1msg.MealOption, 0)
	snackOptions := make([]*nutritionv1msg.MealOption, 0)

	for _, m := range plan.DailyMeals() {
		for _, opt := range m.Options() {
			pbOpt := &nutritionv1msg.MealOption{
				MealName: opt.MealName(),
				Calories: float32(opt.Calories()),
				Protein:  float32(opt.ProteinGrams()),
				Carbs:    float32(opt.CarbGrams()),
				Fat:      float32(opt.FatGrams()),
			}
			switch m.MealType() {
			case "BREAKFAST":
				breakfastOptions = append(breakfastOptions, pbOpt)
			case "LUNCH":
				lunchOptions = append(lunchOptions, pbOpt)
			case "DINNER":
				dinnerOptions = append(dinnerOptions, pbOpt)
			default:
				snackOptions = append(snackOptions, pbOpt)
			}
		}
	}

	return &nutritionv1msg.GetTodayMenuResponse{
		TargetCalories: float32(alloc.TargetCalories()),
		Macros: &nutritionv1msg.Macros{
			ProteinGrams: float32(alloc.ProteinGrams()),
			CarbGrams:    float32(alloc.CarbGrams()),
			FatGrams:     float32(alloc.FatGrams()),
		},
		Meals: &nutritionv1msg.DailyMeals{
			Breakfast: breakfastOptions,
			Lunch:     lunchOptions,
			Dinner:    dinnerOptions,
			Snack:     snackOptions,
		},
	}, nil
}

func (h *GRPCHandler) LogMeal(ctx context.Context, req *nutritionv1msg.LogMealRequest) (*nutritionv1msg.LogMealResponse, error) {
	if req.GetUserId() == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	mealLog, err := h.logMealHdlr.Handle(ctx, command.LogMealCommand{
		UserID:   req.GetUserId(),
		PlanDate: time.Now(),
		MealType: req.GetMealType(),
		MealName: req.GetMealName(),
		Calories: float64(req.GetCalories()),
		Protein:  float64(req.GetProtein()),
		Carbs:    float64(req.GetCarbs()),
		Fat:      float64(req.GetFat()),
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to log meal: %v", err)
	}

	return &nutritionv1msg.LogMealResponse{
		MealLogId: mealLog.ID(),
		Success:   true,
		Message:   "Logged meal successfully",
	}, nil
}

func (h *GRPCHandler) CreateFoodItem(ctx context.Context, req *nutritionv1msg.CreateFoodItemRequest) (*nutritionv1msg.CreateFoodItemResponse, error) {
	if req.GetName() == "" || req.GetCategory() == "" {
		return nil, status.Error(codes.InvalidArgument, "name and category are required")
	}

	item, err := h.createFoodItemHdlr.Handle(ctx, command.CreateFoodItemCommand{
		Name:              req.GetName(),
		Category:          req.GetCategory(),
		CaloriesPer100g:   float64(req.GetCaloriesPer_100G()),
		ProteinPer100g:    float64(req.GetProteinPer_100G()),
		CarbsPer100g:      float64(req.GetCarbsPer_100G()),
		FatPer100g:        float64(req.GetFatPer_100G()),
		AllergenTags:      req.GetAllergenTags(),
		ProteinSource:     req.GetProteinSource(),
		CarbSource:        req.GetCarbSource(),
		IsNutiFoodProduct: req.GetIsNutifoodProduct(),
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create food item: %v", err)
	}

	return &nutritionv1msg.CreateFoodItemResponse{
		FoodItemId: item.ID(),
		Status:     item.Status(),
		Message:    "Created food item pending approval",
	}, nil
}

func (h *GRPCHandler) ApproveFoodItem(ctx context.Context, req *nutritionv1msg.ApproveFoodItemRequest) (*nutritionv1msg.ApproveFoodItemResponse, error) {
	if req.GetFoodItemId() == "" {
		return nil, status.Error(codes.InvalidArgument, "food_item_id is required")
	}

	_, err := h.approveFoodItemHandler.Handle(ctx, command.ApproveFoodItemCommand{
		FoodItemID: req.GetFoodItemId(),
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to approve food item: %v", err)
	}

	return &nutritionv1msg.ApproveFoodItemResponse{
		FoodItemId: req.GetFoodItemId(),
		Status:     "Active",
		Success:    true,
		Message:    "Approved food item successfully into active catalog",
	}, nil
}

func (h *GRPCHandler) GetNutritionHistory(
	ctx context.Context,
	req *nutritionv1msg.GetNutritionHistoryRequest,
) (*nutritionv1msg.GetNutritionHistoryResponse, error) {
	if req.GetUserId() == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	start := time.Now().AddDate(0, 0, -7)
	end := time.Now()

	history, err := h.getNutritionHistoryHdlr.Handle(ctx, query.GetNutritionHistoryQuery{
		UserID:    req.GetUserId(),
		StartDate: start,
		EndDate:   end,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get nutrition history: %v", err)
	}

	protoLogs := make([]*nutritionv1msg.MealLogItem, 0)
	if history != nil {
		logs := history.MealLogs()
		for i := range logs {
			l := &logs[i]
			protoLogs = append(protoLogs, &nutritionv1msg.MealLogItem{
				MealLogId: l.ID(),
				MealName:  l.MealName(),
				MealType:  l.MealType(),
				Calories:  float32(l.Calories()),
				Protein:   float32(l.Protein()),
				Carbs:     float32(l.Carbs()),
				Fat:       float32(l.Fat()),
				LoggedAt:  l.LoggedAt().Format("2006-01-02 15:04:05"),
			})
		}
	}

	return &nutritionv1msg.GetNutritionHistoryResponse{
		Meals: protoLogs,
	}, nil
}

func (h *GRPCHandler) GetNutritionSummary(
	ctx context.Context,
	req *nutritionv1msg.GetNutritionSummaryRequest,
) (*nutritionv1msg.GetNutritionSummaryResponse, error) {
	if req.GetUserId() == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	summary, err := h.getNutritionSummaryHdlr.Handle(ctx, query.GetNutritionSummaryQuery{
		UserID:   req.GetUserId(),
		PlanDate: time.Now(),
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get nutrition summary: %v", err)
	}

	return &nutritionv1msg.GetNutritionSummaryResponse{
		TargetCalories: float32(summary.TargetCalories),
		TargetMacros: &nutritionv1msg.Macros{
			ProteinGrams: float32(summary.TargetProtein),
			CarbGrams:    float32(summary.TargetCarbs),
			FatGrams:     float32(summary.TargetFat),
		},
		ConsumedCalories: float32(summary.ConsumedCalories),
		ConsumedMacros: &nutritionv1msg.Macros{
			ProteinGrams: float32(summary.ConsumedProtein),
			CarbGrams:    float32(summary.ConsumedCarbs),
			FatGrams:     float32(summary.ConsumedFat),
		},
	}, nil
}

// GetNutritionInsight gọi AI để phân tích xu hướng dinh dưỡng và trả về hướng cải thiện.
func (h *GRPCHandler) GetNutritionInsight(
	ctx context.Context,
	req *nutritionv1msg.GetNutritionInsightRequest,
) (*nutritionv1msg.GetNutritionInsightResponse, error) {
	if req.GetUserId() == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	rangeDays := int(req.GetRangeDays())
	if rangeDays <= 0 {
		rangeDays = 7
	}

	insight, err := h.getNutritionInsightHdlr.Handle(ctx, query.GetNutritionInsightQuery{
		UserID:    req.GetUserId(),
		GoalType:  req.GetGoalType(),
		RangeDays: rangeDays,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get nutrition insight: %v", err)
	}

	areas := make([]*nutritionv1msg.ImprovementAreaProto, 0, len(insight.ImprovementAreas))
	for _, a := range insight.ImprovementAreas {
		areas = append(areas, &nutritionv1msg.ImprovementAreaProto{
			Area:       a.Area,
			CurrentAvg: float32(a.CurrentAvg),
			Target:     float32(a.Target),
			Suggestion: a.Suggestion,
			Priority:   a.Priority,
		})
	}

	adj := insight.RecommendedAdjustments
	return &nutritionv1msg.GetNutritionInsightResponse{
		OverallScore:     int32(insight.OverallScore),
		Summary:          insight.Summary,
		Strengths:        insight.Strengths,
		ImprovementAreas: areas,
		WeeklyTrend:      insight.WeeklyTrend,
		RecommendedAdjustments: &nutritionv1msg.RecommendedAdjustmentsProto{
			CaloriesDelta:     float32(adj.CaloriesDelta),
			ProteinRatioDelta: float32(adj.ProteinRatioDelta),
			FocusFoods:        adj.FocusFoods,
		},
	}, nil
}

// --- ConnectRPC Adapter ---

type ConnectNutritionHandler struct {
	grpcHandler *GRPCHandler
}

var _ nutritionv1serviceconnect.NutritionServiceHandler = (*ConnectNutritionHandler)(nil)

func NewConnectNutritionHandler(grpcHandler *GRPCHandler) nutritionv1serviceconnect.NutritionServiceHandler {
	return &ConnectNutritionHandler{grpcHandler: grpcHandler}
}

func (c *ConnectNutritionHandler) GetTodayMenu(
	ctx context.Context,
	req *connect.Request[nutritionv1msg.GetTodayMenuRequest],
) (*connect.Response[nutritionv1msg.GetTodayMenuResponse], error) {
	res, err := c.grpcHandler.GetTodayMenu(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}

func (c *ConnectNutritionHandler) LogMeal(
	ctx context.Context,
	req *connect.Request[nutritionv1msg.LogMealRequest],
) (*connect.Response[nutritionv1msg.LogMealResponse], error) {
	res, err := c.grpcHandler.LogMeal(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}

func (c *ConnectNutritionHandler) GetNutritionHistory(
	ctx context.Context,
	req *connect.Request[nutritionv1msg.GetNutritionHistoryRequest],
) (*connect.Response[nutritionv1msg.GetNutritionHistoryResponse], error) {
	res, err := c.grpcHandler.GetNutritionHistory(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}

func (c *ConnectNutritionHandler) GetNutritionSummary(
	ctx context.Context,
	req *connect.Request[nutritionv1msg.GetNutritionSummaryRequest],
) (*connect.Response[nutritionv1msg.GetNutritionSummaryResponse], error) {
	res, err := c.grpcHandler.GetNutritionSummary(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}

func (c *ConnectNutritionHandler) RecalibratePlanWithPantry(
	ctx context.Context,
	req *connect.Request[nutritionv1msg.RecalibratePlanWithPantryRequest],
) (*connect.Response[nutritionv1msg.RecalibratePlanWithPantryResponse], error) {
	res, err := c.grpcHandler.RecalibratePlanWithPantry(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}

func (c *ConnectNutritionHandler) CreateFoodItem(
	ctx context.Context,
	req *connect.Request[nutritionv1msg.CreateFoodItemRequest],
) (*connect.Response[nutritionv1msg.CreateFoodItemResponse], error) {
	res, err := c.grpcHandler.CreateFoodItem(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}

func (c *ConnectNutritionHandler) GetNutritionInsight(
	ctx context.Context,
	req *connect.Request[nutritionv1msg.GetNutritionInsightRequest],
) (*connect.Response[nutritionv1msg.GetNutritionInsightResponse], error) {
	res, err := c.grpcHandler.GetNutritionInsight(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}
