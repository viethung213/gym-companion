package grpc

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	profilev1message "github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/profile/v1/message"
	profilev1service "github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/profile/v1/service"
	"github.com/viethung213/gym-companion/internal/profile/application/command"
	"github.com/viethung213/gym-companion/internal/profile/application/query"
	"github.com/viethung213/gym-companion/internal/profile/domain/derror"
	"github.com/viethung213/gym-companion/internal/shared/middleware"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

//nolint:revive // GRPCHandler stutters with package name but matches module convention
type GRPCHandler struct {
	profilev1service.UnimplementedProfileServiceServer
	saveHealthProfileHandler     *command.SaveHealthProfileHandler
	updateProfileHandler         *command.UpdateProfileHandler
	logPeriodicMetricsHandler    *command.LogPeriodicMetricsHandler
	reportInjuryHandler          *command.ReportInjuryHandler
	recoverInjuryHandler         *command.RecoverInjuryHandler
	getProfileHandler            *query.GetProfileHandler
	getBodyMetricsHistoryHandler *query.GetBodyMetricsHistoryHandler
	getInjuryHistoryHandler      *query.GetInjuryHistoryHandler
}

func NewGRPCHandler(
	saveHealthProfileHandler *command.SaveHealthProfileHandler,
	updateProfileHandler *command.UpdateProfileHandler,
	logPeriodicMetricsHandler *command.LogPeriodicMetricsHandler,
	reportInjuryHandler *command.ReportInjuryHandler,
	recoverInjuryHandler *command.RecoverInjuryHandler,
	getProfileHandler *query.GetProfileHandler,
	getBodyMetricsHistoryHandler *query.GetBodyMetricsHistoryHandler,
	getInjuryHistoryHandler *query.GetInjuryHistoryHandler,
) *GRPCHandler {
	return &GRPCHandler{
		saveHealthProfileHandler:     saveHealthProfileHandler,
		updateProfileHandler:         updateProfileHandler,
		logPeriodicMetricsHandler:    logPeriodicMetricsHandler,
		reportInjuryHandler:          reportInjuryHandler,
		recoverInjuryHandler:         recoverInjuryHandler,
		getProfileHandler:            getProfileHandler,
		getBodyMetricsHistoryHandler: getBodyMetricsHistoryHandler,
		getInjuryHistoryHandler:      getInjuryHistoryHandler,
	}
}

func resolveUserID(ctx context.Context, reqUserID string) (string, error) {
	actor, err := middleware.RequireAuthenticated(ctx)
	if err != nil {
		// Fallback for unauthenticated testing mode if reqUserID is present
		if reqUserID != "" {
			return reqUserID, nil
		}
		return "", status.Error(codes.Unauthenticated, "authentication required")
	}

	// Admin user can specify target user_id in request payload/url
	if actor.IsAdmin() && reqUserID != "" {
		return reqUserID, nil
	}

	// Normal user: UserID is extracted strictly from gRPC context metadata (JWT token claim)
	return actor.UserID, nil
}

func (h *GRPCHandler) SaveHealthProfile(
	ctx context.Context,
	req *profilev1message.SaveHealthProfileRequest,
) (*profilev1message.SaveHealthProfileResponse, error) {
	targetUserID, err := resolveUserID(ctx, "")
	if err != nil {
		return nil, err
	}

	injuries := make([]command.InjuryInput, 0, len(req.GetInjuries()))
	for _, in := range req.GetInjuries() {
		injuries = append(injuries, command.InjuryInput{
			ID:          uuid.New().String(),
			MuscleGroup: in.GetMuscleGroup(),
			Severity:    in.GetSeverity(),
			Notes:       in.GetNotes(),
		})
	}

	cmd := command.SaveHealthProfileCommand{
		UserID:                targetUserID,
		WeightKg:              float64(req.GetWeightKg()),
		HeightCm:              float64(req.GetHeightCm()),
		BodyFatPercent:        float64(req.GetBodyFatPercent()),
		DateOfBirth:           req.GetDateOfBirth(),
		Gender:                req.GetGender(),
		Goals:                 req.GetGoals(),
		ExperienceLevel:       req.GetExperienceLevel(),
		PreferredWorkoutTimes: req.GetPreferredWorkoutTimes(),
		AvailableEquipment:    req.GetAvailableEquipment(),
		PreferredMuscleGroups: req.GetPreferredMuscleGroups(),
		CoachStyle:            req.GetCoachStyle(),
		TargetWeightKg:        float64(req.GetTargetWeightKg()),
		TargetBodyFatPercent:  float64(req.GetTargetBodyFatPercent()),
		Injuries:              injuries,
	}

	res, err := h.saveHealthProfileHandler.Handle(ctx, cmd)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "save health profile failed: %v", err)
	}

	return &profilev1message.SaveHealthProfileResponse{
		UserId:           res.UserID,
		CompletionRate:   float32(res.CompletionRate),
		AiCoachActivated: res.AICoachActivated,
		Message:          "Health profile saved successfully",
	}, nil
}

func (h *GRPCHandler) GetProfile(ctx context.Context, req *profilev1message.GetProfileRequest) (*profilev1message.GetProfileResponse, error) {
	targetUserID, err := resolveUserID(ctx, req.GetUserId())
	if err != nil {
		return nil, err
	}

	profile, err := h.getProfileHandler.Handle(ctx, query.GetProfileQuery{UserID: targetUserID})
	if err != nil {
		if errors.Is(err, derror.ErrProfileNotFound) {
			return nil, status.Error(codes.NotFound, "user profile not found")
		}
		return nil, status.Errorf(codes.Internal, "get profile failed: %v", err)
	}

	bio := profile.BiologicalMetrics()
	injuriesProto := make([]*profilev1message.Injury, 0, len(profile.Injuries()))
	for _, inj := range profile.Injuries() {
		var recAt string
		if inj.RecoveredAt() != nil {
			recAt = inj.RecoveredAt().Format(time.RFC3339)
		}
		injuriesProto = append(injuriesProto, &profilev1message.Injury{
			InjuryId:    inj.ID(),
			MuscleGroup: inj.MuscleGroup(),
			Severity:    inj.Severity(),
			Notes:       inj.Notes(),
			ReportedAt:  inj.ReportedAt().Format(time.RFC3339),
			IsRecovered: inj.IsRecovered(),
			RecoveredAt: recAt,
		})
	}

	dobStr := ""
	if !bio.DateOfBirth().IsZero() {
		dobStr = bio.DateOfBirth().Format("2006-01-02")
	}

	latestBf := float32(0)
	if len(profile.PeriodicMetrics()) > 0 {
		latestBf = float32(profile.PeriodicMetrics()[len(profile.PeriodicMetrics())-1].BodyFatPercent())
	}

	return &profilev1message.GetProfileResponse{
		UserId:                profile.UserID(),
		WeightKg:              float32(bio.WeightKg()),
		HeightCm:              float32(bio.HeightCm()),
		DateOfBirth:           dobStr,
		Age:                   bio.Age(),
		Gender:                bio.Gender(),
		Goals:                 profile.Goals(),
		ExperienceLevel:       profile.ExperienceLevel(),
		PreferredWorkoutTimes: profile.PreferredWorkoutTimes(),
		AvailableEquipment:    profile.AvailableEquipment(),
		PreferredMuscleGroups: profile.PreferredMuscleGroups(),
		CoachStyle:            profile.CoachStyle(),
		TargetWeightKg:        float32(profile.TargetWeightKg()),
		TargetBodyFatPercent:  float32(profile.TargetBodyFatPercent()),
		Injuries:              injuriesProto,
		CreatedAt:             profile.CreatedAt().Format(time.RFC3339),
		UpdatedAt:             profile.UpdatedAt().Format(time.RFC3339),
		CompletionRate:        float32(profile.CompletionRate()),
		BodyFatPercent:        latestBf,
	}, nil
}

func (h *GRPCHandler) UpdateProfile(ctx context.Context, req *profilev1message.UpdateProfileRequest) (*profilev1message.UpdateProfileResponse, error) {
	targetUserID, err := resolveUserID(ctx, "")
	if err != nil {
		return nil, err
	}

	cmd := command.UpdateProfileCommand{
		UserID:                targetUserID,
		WeightKg:              float64(req.GetWeightKg()),
		HeightCm:              float64(req.GetHeightCm()),
		BodyFatPercent:        float64(req.GetBodyFatPercent()),
		DateOfBirth:           req.GetDateOfBirth(),
		Gender:                req.GetGender(),
		Goals:                 req.GetGoals(),
		ExperienceLevel:       req.GetExperienceLevel(),
		PreferredWorkoutTimes: req.GetPreferredWorkoutTimes(),
		AvailableEquipment:    req.GetAvailableEquipment(),
		PreferredMuscleGroups: req.GetPreferredMuscleGroups(),
		CoachStyle:            req.GetCoachStyle(),
		TargetWeightKg:        float64(req.GetTargetWeightKg()),
		TargetBodyFatPercent:  float64(req.GetTargetBodyFatPercent()),
	}

	err = h.updateProfileHandler.Handle(ctx, cmd)
	if err != nil {
		if errors.Is(err, derror.ErrProfileNotFound) {
			return nil, status.Error(codes.NotFound, "user profile not found")
		}
		return nil, status.Errorf(codes.Internal, "update profile failed: %v", err)
	}

	return &profilev1message.UpdateProfileResponse{
		Success: true,
		Message: "Profile updated successfully",
	}, nil
}

func (h *GRPCHandler) LogPeriodicMetrics(
	ctx context.Context,
	req *profilev1message.LogPeriodicMetricsRequest,
) (*profilev1message.LogPeriodicMetricsResponse, error) {
	targetUserID, err := resolveUserID(ctx, "")
	if err != nil {
		return nil, err
	}

	logID := uuid.New().String()
	cmd := command.LogPeriodicMetricsCommand{
		LogID:            logID,
		UserID:           targetUserID,
		WeightKg:         float64(req.GetWeightKg()),
		HeightCm:         float64(req.GetHeightCm()),
		BodyFatPercent:   float64(req.GetBodyFatPercent()),
		ProgressPhotoURL: req.GetProgressPhotoUrl(),
	}

	res, handleErr := h.logPeriodicMetricsHandler.Handle(ctx, cmd)
	if handleErr != nil {
		if errors.Is(handleErr, derror.ErrProfileNotFound) {
			return nil, status.Error(codes.NotFound, "user profile not found")
		}
		return nil, status.Errorf(codes.Internal, "log periodic metrics failed: %v", handleErr)
	}

	return &profilev1message.LogPeriodicMetricsResponse{
		LogId:          res.LogID,
		UserId:         res.UserID,
		WeightKg:       float32(res.WeightKg),
		HeightCm:       float32(res.HeightCm),
		BodyFatPercent: float32(res.BodyFatPercent),
		SyncStatus:     res.SyncStatus,
		Message:        "Metrics logged successfully",
	}, nil
}

func (h *GRPCHandler) GetBodyMetricsHistory(
	ctx context.Context,
	req *profilev1message.GetBodyMetricsHistoryRequest,
) (*profilev1message.GetBodyMetricsHistoryResponse, error) {
	targetUserID, err := resolveUserID(ctx, req.GetUserId())
	if err != nil {
		return nil, err
	}

	metrics, err := h.getBodyMetricsHistoryHandler.Handle(ctx, query.GetBodyMetricsHistoryQuery{UserID: targetUserID})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get body metrics history failed: %v", err)
	}

	pbMetrics := make([]*profilev1message.PeriodicMetric, 0, len(metrics))
	for _, m := range metrics {
		pbMetrics = append(pbMetrics, &profilev1message.PeriodicMetric{
			Id:               m.ID(),
			WeightKg:         float32(m.WeightKg()),
			HeightCm:         float32(m.HeightCm()),
			BodyFatPercent:   float32(m.BodyFatPercent()),
			ProgressPhotoUrl: m.ProgressPhotoURL(),
			LoggedAt:         m.LoggedAt().Format(time.RFC3339),
		})
	}

	return &profilev1message.GetBodyMetricsHistoryResponse{
		UserId:  targetUserID,
		Metrics: pbMetrics,
	}, nil
}

func (h *GRPCHandler) ReportInjury(ctx context.Context, req *profilev1message.ReportInjuryRequest) (*profilev1message.ReportInjuryResponse, error) {
	targetUserID, err := resolveUserID(ctx, "")
	if err != nil {
		return nil, err
	}

	injuryID := uuid.New().String()
	cmd := command.ReportInjuryCommand{
		InjuryID:    injuryID,
		UserID:      targetUserID,
		MuscleGroup: req.GetMuscleGroup(),
		Severity:    req.GetSeverity(),
		Notes:       req.GetNotes(),
	}

	id, err := h.reportInjuryHandler.Handle(ctx, cmd)
	if err != nil {
		if errors.Is(err, derror.ErrProfileNotFound) {
			return nil, status.Error(codes.NotFound, "user profile not found")
		}
		return nil, status.Errorf(codes.Internal, "report injury failed: %v", err)
	}

	return &profilev1message.ReportInjuryResponse{
		InjuryId: id,
		Success:  true,
		Message:  "Injury reported successfully",
	}, nil
}

func (h *GRPCHandler) RecoverInjury(ctx context.Context, req *profilev1message.RecoverInjuryRequest) (*profilev1message.RecoverInjuryResponse, error) {
	targetUserID, err := resolveUserID(ctx, "")
	if err != nil {
		return nil, err
	}
	if req.GetInjuryId() == "" {
		return nil, status.Error(codes.InvalidArgument, "injury_id is required")
	}

	cmd := command.RecoverInjuryCommand{
		UserID:   targetUserID,
		InjuryID: req.GetInjuryId(),
	}

	err = h.recoverInjuryHandler.Handle(ctx, cmd)
	if err != nil {
		if errors.Is(err, derror.ErrInjuryNotFound) {
			return nil, status.Error(codes.NotFound, "injury not found")
		}
		return nil, status.Errorf(codes.Internal, "recover injury failed: %v", err)
	}

	return &profilev1message.RecoverInjuryResponse{
		Success: true,
		Message: "Injury recovered successfully",
	}, nil
}

func (h *GRPCHandler) GetInjuryHistory(ctx context.Context, req *profilev1message.GetInjuryHistoryRequest) (*profilev1message.GetInjuryHistoryResponse, error) {
	targetUserID, err := resolveUserID(ctx, req.GetUserId())
	if err != nil {
		return nil, err
	}

	injuries, handleErr := h.getInjuryHistoryHandler.Handle(ctx, query.GetInjuryHistoryQuery{UserID: targetUserID})
	if handleErr != nil {
		return nil, status.Errorf(codes.Internal, "get injury history failed: %v", handleErr)
	}

	pbInjuries := make([]*profilev1message.Injury, 0, len(injuries))
	for _, inj := range injuries {
		var recAt string
		if inj.RecoveredAt() != nil {
			recAt = inj.RecoveredAt().Format(time.RFC3339)
		}
		pbInjuries = append(pbInjuries, &profilev1message.Injury{
			InjuryId:    inj.ID(),
			MuscleGroup: inj.MuscleGroup(),
			Severity:    inj.Severity(),
			Notes:       inj.Notes(),
			ReportedAt:  inj.ReportedAt().Format(time.RFC3339),
			IsRecovered: inj.IsRecovered(),
			RecoveredAt: recAt,
		})
	}

	return &profilev1message.GetInjuryHistoryResponse{
		UserId:   targetUserID,
		Injuries: pbInjuries,
	}, nil
}
