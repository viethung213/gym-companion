// Package grpc implements the NutritionService gRPC endpoint.
package grpc

import (
	"context"
	"errors"
	"strings"
	"time"

	nutritionv1msg "github.com/viethung213/gym-companion/internal/gen/go/contracts/core/nutrition/v1/message"
	nutritionv1service "github.com/viethung213/gym-companion/internal/gen/go/contracts/core/nutrition/v1/service"
	"github.com/viethung213/gym-companion/internal/gen/go/contracts/core/nutrition/v1/service/nutritionv1serviceconnect"
	"github.com/viethung213/gym-companion/internal/nutrition/application/command"
	"github.com/viethung213/gym-companion/internal/nutrition/application/query"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/domainerror"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/service"
	"github.com/viethung213/gym-companion/internal/shared/middleware"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

//nolint:revive // GRPCHandler name is consistent across bounded contexts
type GRPCHandler struct {
	nutritionv1service.UnimplementedNutritionServiceServer
	genPlanHdlr             *command.GenerateDailyPlanHandler
	recalPlanHdlr           *command.RecalibratePlanWithPantryHandler
	logMealHdlr             *command.LogMealHandler
	createFoodItemHdlr      *command.CreateFoodItemHandler
	approveFoodItemHandler  *command.ApproveFoodItemHandler
	updateMealScheduleHdlr  *command.UpdateMealScheduleHandler
	getTodayMenuHdlr        *query.GetTodayMenuHandler
	getNutritionHistoryHdlr *query.GetNutritionHistoryHandler
	getNutritionSummaryHdlr *query.GetNutritionSummaryHandler
	getNutritionInsightHdlr *query.GetNutritionInsightHandler
	getUserAllergiesHdlr    *query.GetUserAllergiesHandler
	updateUserAllergiesHdlr *command.UpdateUserAllergiesHandler
}

func NewGRPCHandler(
	genPlanHdlr *command.GenerateDailyPlanHandler,
	recalPlanHdlr *command.RecalibratePlanWithPantryHandler,
	logMealHdlr *command.LogMealHandler,
	createFoodItemHdlr *command.CreateFoodItemHandler,
	approveFoodItemHandler *command.ApproveFoodItemHandler,
	updateMealScheduleHdlr *command.UpdateMealScheduleHandler,
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
		updateMealScheduleHdlr:  updateMealScheduleHdlr,
		getTodayMenuHdlr:        getTodayMenuHdlr,
		getNutritionHistoryHdlr: getNutritionHistoryHdlr,
		getNutritionSummaryHdlr: getNutritionSummaryHdlr,
		getNutritionInsightHdlr: getNutritionInsightHdlr,
	}
}

func (h *GRPCHandler) WithAllergyHandlers(
	getUserAllergiesHdlr *query.GetUserAllergiesHandler,
	updateUserAllergiesHdlr *command.UpdateUserAllergiesHandler,
) *GRPCHandler {
	h.getUserAllergiesHdlr = getUserAllergiesHdlr
	h.updateUserAllergiesHdlr = updateUserAllergiesHdlr
	return h
}

func (h *GRPCHandler) GetTodayMenu(ctx context.Context, _ *nutritionv1msg.GetTodayMenuRequest) (*nutritionv1msg.GetTodayMenuResponse, error) {
	actor, err := middleware.RequireAuthenticated(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "authentication required")
	}
	userID := actor.UserID

	now := time.Now()
	plan, err := h.getTodayMenuHdlr.Handle(ctx, query.GetTodayMenuQuery{
		UserID:   userID,
		PlanDate: now,
	})
	if err != nil || plan == nil {
		bio := service.BiologicalMetrics{
			WeightKg: 70.0,
			HeightCm: 170.0,
			Age:      25,
			Gender:   "MALE",
		}

		var userRestrictions []string
		if h.getUserAllergiesHdlr != nil {
			allergies, aErr := h.getUserAllergiesHdlr.Handle(ctx, query.GetUserAllergiesQuery{UserID: userID})
			if aErr == nil {
				userRestrictions = allergies
			}
		}

		genPlan, genErr := h.genPlanHdlr.Handle(ctx, command.GenerateDailyPlanCommand{
			UserID:            userID,
			PlanDate:          now,
			BiologicalMetrics: bio,
			UserRestrictions:  userRestrictions,
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
				MealName:    opt.MealName(),
				Calories:    float32(opt.Calories()),
				Protein:     float32(opt.ProteinGrams()),
				Carbs:       float32(opt.CarbGrams()),
				Fat:         float32(opt.FatGrams()),
				RecipeSteps: opt.CookingSteps(),
			}
			switch strings.ToUpper(strings.TrimSpace(m.MealType())) {
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
	actor, err := middleware.RequireAuthenticated(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "authentication required")
	}
	userID := actor.UserID

	mealLog, err := h.logMealHdlr.Handle(ctx, command.LogMealCommand{
		UserID:   userID,
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
		PricePer100g:      float64(req.GetPricePer_100G()),
		CostTier:          req.GetCostTier(),
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
	_ *nutritionv1msg.GetNutritionHistoryRequest,
) (*nutritionv1msg.GetNutritionHistoryResponse, error) {
	actor, err := middleware.RequireAuthenticated(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "authentication required")
	}
	userID := actor.UserID

	start := time.Now().AddDate(0, 0, -7)
	end := time.Now()

	history, err := h.getNutritionHistoryHdlr.Handle(ctx, query.GetNutritionHistoryQuery{
		UserID:    userID,
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
	_ *nutritionv1msg.GetNutritionSummaryRequest,
) (*nutritionv1msg.GetNutritionSummaryResponse, error) {
	actor, err := middleware.RequireAuthenticated(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "authentication required")
	}
	userID := actor.UserID

	summary, err := h.getNutritionSummaryHdlr.Handle(ctx, query.GetNutritionSummaryQuery{
		UserID:   userID,
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

func (h *GRPCHandler) UpdateMealSchedule(
	ctx context.Context, req *nutritionv1msg.UpdateMealScheduleRequest,
) (*nutritionv1msg.UpdateMealScheduleResponse, error) {
	actor, err := middleware.RequireAuthenticated(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "authentication required")
	}
	userID := actor.UserID

	inputs := make([]command.MealScheduleItemInput, 0, len(req.GetSchedules()))
	for _, item := range req.GetSchedules() {
		inputs = append(inputs, command.MealScheduleItemInput{
			MealType:      item.GetMealType(),
			ScheduledTime: item.GetScheduledTime(),
		})
	}

	res, err := h.updateMealScheduleHdlr.Handle(ctx, command.UpdateMealScheduleCommand{
		UserID:    userID,
		Schedules: inputs,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update meal schedule: %v", err)
	}

	pbSchedules := make([]*nutritionv1msg.MealScheduleItem, 0, len(res.Schedules))
	for _, item := range res.Schedules {
		pbSchedules = append(pbSchedules, &nutritionv1msg.MealScheduleItem{
			MealType:      item.MealType,
			ScheduledTime: item.ScheduledTime,
		})
	}

	return &nutritionv1msg.UpdateMealScheduleResponse{
		Success:   res.Success,
		Message:   res.Message,
		Schedules: pbSchedules,
	}, nil
}

func (h *GRPCHandler) RecalibratePlanWithPantry(
	ctx context.Context,
	req *nutritionv1msg.RecalibratePlanWithPantryRequest,
) (*nutritionv1msg.RecalibratePlanWithPantryResponse, error) {
	actor, err := middleware.RequireAuthenticated(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "authentication required")
	}
	userID := actor.UserID
	if strings.TrimSpace(req.GetMealType()) == "" {
		return nil, status.Error(codes.InvalidArgument, "meal_type is required")
	}

	var userRestrictions []string
	if h.getUserAllergiesHdlr != nil {
		allergies, aErr := h.getUserAllergiesHdlr.Handle(ctx, query.GetUserAllergiesQuery{UserID: userID})
		if aErr == nil {
			userRestrictions = allergies
		}
	}

	now := time.Now()
	plan, err := h.recalPlanHdlr.Handle(ctx, &command.RecalibratePlanWithPantryCommand{
		UserID:               userID,
		PlanDate:             now,
		MealType:             req.GetMealType(),
		AvailableIngredients: req.GetAvailableIngredients(),
		UserRestrictions:     userRestrictions,
	})
	if err != nil {
		if errors.Is(err, domainerror.ErrAllergenConflict) {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		if errors.Is(err, domainerror.ErrMealTypeRequired) {
			return nil, status.Error(codes.InvalidArgument, "meal_type is required")
		}
		if errors.Is(err, domainerror.ErrMealTypeNotFound) {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		if errors.Is(err, domainerror.ErrMealAlreadyLogged) {
			return nil, status.Error(codes.FailedPrecondition, err.Error())
		}
		return nil, status.Errorf(codes.Internal, "failed to recalibrate plan with pantry: %v", err)
	}

	breakfastOptions := make([]*nutritionv1msg.MealOption, 0)
	lunchOptions := make([]*nutritionv1msg.MealOption, 0)
	dinnerOptions := make([]*nutritionv1msg.MealOption, 0)
	snackOptions := make([]*nutritionv1msg.MealOption, 0)

	for _, m := range plan.DailyMeals() {
		for _, opt := range m.Options() {
			pbOpt := &nutritionv1msg.MealOption{
				MealName:    opt.MealName(),
				Calories:    float32(opt.Calories()),
				Protein:     float32(opt.ProteinGrams()),
				Carbs:       float32(opt.CarbGrams()),
				Fat:         float32(opt.FatGrams()),
				RecipeSteps: opt.CookingSteps(),
			}
			switch strings.ToUpper(strings.TrimSpace(m.MealType())) {
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

	alloc := plan.CalorieAllocation()
	return &nutritionv1msg.RecalibratePlanWithPantryResponse{
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
		Message: "Recalibrated plan successfully with available pantry ingredients",
	}, nil
}

func (h *GRPCHandler) GetNutritionInsight(
	ctx context.Context,
	req *nutritionv1msg.GetNutritionInsightRequest,
) (*nutritionv1msg.GetNutritionInsightResponse, error) {
	actor, err := middleware.RequireAuthenticated(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "authentication required")
	}
	userID := actor.UserID

	insight, err := h.getNutritionInsightHdlr.Handle(ctx, query.GetNutritionInsightQuery{
		UserID:        userID,
		WeekStartDate: req.GetWeekStartDate(),
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get nutrition insight: %v", err)
	}
	if insight == nil {
		return nil, status.Error(codes.NotFound, "weekly nutrition insight not found")
	}

	areas := make([]*nutritionv1msg.ImprovementAreaProto, 0, len(insight.ImprovementAreas()))
	for _, a := range insight.ImprovementAreas() {
		areas = append(areas, &nutritionv1msg.ImprovementAreaProto{
			Nutrient:    a.Nutrient,
			Action:      a.Action,
			DeltaAmount: float32(a.DeltaAmount),
			Suggestion:  a.Suggestion,
		})
	}

	sum := insight.WeeklySummary()
	weeklySummary := &nutritionv1msg.WeeklyMacroSummaryProto{
		ConsumedCalories: float32(sum.ConsumedCalories),
		TargetCalories:   float32(sum.TargetCalories),
		ConsumedProtein:  float32(sum.ConsumedProtein),
		TargetProtein:    float32(sum.TargetProtein),
		ConsumedCarbs:    float32(sum.ConsumedCarbs),
		TargetCarbs:      float32(sum.TargetCarbs),
		ConsumedFat:      float32(sum.ConsumedFat),
		TargetFat:        float32(sum.TargetFat),
		LoggedMealsCount: int32(sum.LoggedMealsCount),
		TotalMealsCount:  int32(sum.TotalMealsCount),
	}

	return &nutritionv1msg.GetNutritionInsightResponse{
		OverallScore:          int32(insight.OverallScore()),
		Summary:               insight.Summary(),
		PredictedCookingStyle: insight.PredictedCookingStyle(),
		PreferredIngredients:  insight.PreferredIngredients(),
		ImprovementAreas:      areas,
		StartDate:             insight.StartDate().Format("2006-01-02"),
		EndDate:               insight.EndDate().Format("2006-01-02"),
		EvaluatedAtUnix:       insight.UpdatedAt().Unix(),
		WeeklySummary:         weeklySummary,
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
	req *nutritionv1msg.GetTodayMenuRequest,
) (*nutritionv1msg.GetTodayMenuResponse, error) {
	res, err := c.grpcHandler.GetTodayMenu(ctx, req)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (c *ConnectNutritionHandler) LogMeal(
	ctx context.Context,
	req *nutritionv1msg.LogMealRequest,
) (*nutritionv1msg.LogMealResponse, error) {
	res, err := c.grpcHandler.LogMeal(ctx, req)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (c *ConnectNutritionHandler) GetNutritionHistory(
	ctx context.Context,
	req *nutritionv1msg.GetNutritionHistoryRequest,
) (*nutritionv1msg.GetNutritionHistoryResponse, error) {
	res, err := c.grpcHandler.GetNutritionHistory(ctx, req)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (c *ConnectNutritionHandler) GetNutritionSummary(
	ctx context.Context,
	req *nutritionv1msg.GetNutritionSummaryRequest,
) (*nutritionv1msg.GetNutritionSummaryResponse, error) {
	res, err := c.grpcHandler.GetNutritionSummary(ctx, req)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (c *ConnectNutritionHandler) RecalibratePlanWithPantry(
	ctx context.Context,
	req *nutritionv1msg.RecalibratePlanWithPantryRequest,
) (*nutritionv1msg.RecalibratePlanWithPantryResponse, error) {
	res, err := c.grpcHandler.RecalibratePlanWithPantry(ctx, req)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (c *ConnectNutritionHandler) CreateFoodItem(
	ctx context.Context,
	req *nutritionv1msg.CreateFoodItemRequest,
) (*nutritionv1msg.CreateFoodItemResponse, error) {
	res, err := c.grpcHandler.CreateFoodItem(ctx, req)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (c *ConnectNutritionHandler) GetNutritionInsight(
	ctx context.Context,
	req *nutritionv1msg.GetNutritionInsightRequest,
) (*nutritionv1msg.GetNutritionInsightResponse, error) {
	res, err := c.grpcHandler.GetNutritionInsight(ctx, req)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (c *ConnectNutritionHandler) UpdateMealSchedule(
	ctx context.Context,
	req *nutritionv1msg.UpdateMealScheduleRequest,
) (*nutritionv1msg.UpdateMealScheduleResponse, error) {
	res, err := c.grpcHandler.UpdateMealSchedule(ctx, req)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (h *GRPCHandler) GetUserAllergies(
	ctx context.Context,
	_ *nutritionv1msg.GetUserAllergiesRequest,
) (*nutritionv1msg.GetUserAllergiesResponse, error) {
	if h.getUserAllergiesHdlr == nil {
		return nil, status.Error(codes.Unimplemented, "user allergy handler is not configured")
	}
	actor, err := middleware.RequireAuthenticated(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "authentication required")
	}
	targetUserID := actor.UserID

	allergies, err := h.getUserAllergiesHdlr.Handle(ctx, query.GetUserAllergiesQuery{UserID: targetUserID})
	if err != nil {
		if errors.Is(err, domainerror.ErrUserIDRequired) {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		return nil, status.Errorf(codes.Internal, "failed to get user allergies: %v", err)
	}
	return &nutritionv1msg.GetUserAllergiesResponse{
		Allergies: allergies,
	}, nil
}

func (h *GRPCHandler) UpdateUserAllergies(
	ctx context.Context,
	req *nutritionv1msg.UpdateUserAllergiesRequest,
) (*nutritionv1msg.UpdateUserAllergiesResponse, error) {
	if h.updateUserAllergiesHdlr == nil {
		return nil, status.Error(codes.Unimplemented, "user allergy handler is not configured")
	}
	actor, err := middleware.RequireAuthenticated(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "authentication required")
	}
	targetUserID := actor.UserID

	allergies, err := h.updateUserAllergiesHdlr.Handle(ctx, command.UpdateUserAllergiesCommand{
		UserID:    targetUserID,
		Allergies: req.GetAllergies(),
	})
	if err != nil {
		if errors.Is(err, domainerror.ErrUserIDRequired) {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		return nil, status.Errorf(codes.Internal, "failed to update user allergies: %v", err)
	}
	return &nutritionv1msg.UpdateUserAllergiesResponse{
		Allergies: allergies,
	}, nil
}

func (c *ConnectNutritionHandler) GetUserAllergies(
	ctx context.Context,
	req *nutritionv1msg.GetUserAllergiesRequest,
) (*nutritionv1msg.GetUserAllergiesResponse, error) {
	return c.grpcHandler.GetUserAllergies(ctx, req)
}

func (c *ConnectNutritionHandler) UpdateUserAllergies(
	ctx context.Context,
	req *nutritionv1msg.UpdateUserAllergiesRequest,
) (*nutritionv1msg.UpdateUserAllergiesResponse, error) {
	return c.grpcHandler.UpdateUserAllergies(ctx, req)
}
