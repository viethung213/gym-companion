package grpc

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	exercisemsg "github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/exercise/v1/message"
	profilev1message "github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/profile/v1/message"
	profilev1service "github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/profile/v1/service"
	"github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/profile/v1/service/profilev1serviceconnect"
	"github.com/viethung213/gym-companion/internal/profile/application/command"
	"github.com/viethung213/gym-companion/internal/profile/application/query"
	"github.com/viethung213/gym-companion/internal/profile/domain/derror"
	"github.com/viethung213/gym-companion/internal/profile/domain/entity"
	"github.com/viethung213/gym-companion/internal/profile/domain/vo"
	"github.com/viethung213/gym-companion/internal/profile/infrastructure/protomap"
	"github.com/viethung213/gym-companion/internal/shared/middleware"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

//nolint:revive // GRPCHandler name is consistent across bounded contexts
type GRPCHandler struct {
	profilev1service.UnimplementedProfileServiceServer
	saveHealthProfileHandler     *command.SaveHealthProfileHandler
	updateProfileHandler         *command.UpdateProfileHandler
	updateIdentityHandler        *command.UpdateIdentityHandler
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
	updateIdentityHandler *command.UpdateIdentityHandler,
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
		updateIdentityHandler:        updateIdentityHandler,
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
		return "", status.Error(codes.Unauthenticated, "authentication required")
	}

	if reqUserID != "" && reqUserID != actor.UserID {
		if !actor.IsAdmin() {
			return "", status.Error(codes.PermissionDenied, "access denied: only admins can view or modify another user's profile")
		}
		return reqUserID, nil
	}

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
		input, mapErr := injuryInputFromProto(in)
		if mapErr != nil {
			return nil, status.Error(codes.InvalidArgument, mapErr.Error())
		}
		input.ID = uuid.New().String()
		injuries = append(injuries, input)
	}
	equipment, muscles, err := trainingFromProto(req)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	tz, err := optionalTimezone(req.GetTimezone())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
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
		AvailableEquipment:    equipment,
		PreferredMuscleGroups: muscles,
		CoachStyle:            req.GetCoachStyle(),
		TargetWeightKg:        float64(req.GetTargetWeightKg()),
		TargetBodyFatPercent:  float64(req.GetTargetBodyFatPercent()),
		Injuries:              injuries,
		Timezone:              tz,
	}

	res, err := h.saveHealthProfileHandler.Handle(ctx, cmd)
	if err != nil {
		if errors.Is(err, derror.ErrInvalidTaxonomy) || errors.Is(err, derror.ErrTooManyWorkoutSlots) {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
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
		injuriesProto = append(injuriesProto, toProtoInjury(inj))
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
		AvailableEquipment:    protomap.EquipmentListToProto(profile.AvailableEquipment()),
		PreferredMuscleGroups: protomap.MuscleGroupsToProto(profile.PreferredMuscleGroups()),
		CoachStyle:            profile.CoachStyle(),
		TargetWeightKg:        float32(profile.TargetWeightKg()),
		TargetBodyFatPercent:  float32(profile.TargetBodyFatPercent()),
		Injuries:              injuriesProto,
		CreatedAt:             profile.CreatedAt().Format(time.RFC3339),
		UpdatedAt:             profile.UpdatedAt().Format(time.RFC3339),
		CompletionRate:        float32(profile.CompletionRate()),
		BodyFatPercent:        latestBf,
		FullName:              profile.FullName(),
		AvatarUrl:             profile.AvatarURL(),
		Timezone:              profile.Timezone().String(),
	}, nil
}

func (h *GRPCHandler) UpdateProfile(ctx context.Context, req *profilev1message.UpdateProfileRequest) (*profilev1message.UpdateProfileResponse, error) {
	targetUserID, err := resolveUserID(ctx, "")
	if err != nil {
		return nil, err
	}

	equipment, muscles, err := trainingFromProto(req)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	tz, err := optionalTimezone(req.GetTimezone())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
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
		AvailableEquipment:    equipment,
		PreferredMuscleGroups: muscles,
		CoachStyle:            req.GetCoachStyle(),
		TargetWeightKg:        float64(req.GetTargetWeightKg()),
		TargetBodyFatPercent:  float64(req.GetTargetBodyFatPercent()),
		Timezone:              tz,
	}

	err = h.updateProfileHandler.Handle(ctx, cmd)
	if err != nil {
		if errors.Is(err, derror.ErrProfileNotFound) {
			return nil, status.Error(codes.NotFound, "user profile not found")
		}
		if errors.Is(err, derror.ErrInvalidTaxonomy) || errors.Is(err, derror.ErrTooManyWorkoutSlots) {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		return nil, status.Errorf(codes.Internal, "update profile failed: %v", err)
	}

	return &profilev1message.UpdateProfileResponse{
		Success: true,
		Message: "Profile updated successfully",
	}, nil
}

func (h *GRPCHandler) UpdateIdentity(
	ctx context.Context,
	req *profilev1message.UpdateIdentityRequest,
) (*profilev1message.UpdateIdentityResponse, error) {
	targetUserID, err := resolveUserID(ctx, "")
	if err != nil {
		return nil, err
	}

	var fullName *string
	if req.FullName != nil {
		fullName = req.FullName
	}

	var avatarURL *string
	if req.AvatarUrl != nil {
		avatarURL = req.AvatarUrl
	}

	cmd := command.UpdateIdentityCommand{
		UserID:    targetUserID,
		FullName:  fullName,
		AvatarURL: avatarURL,
	}

	res, err := h.updateIdentityHandler.Handle(ctx, cmd)
	if err != nil {
		if errors.Is(err, derror.ErrProfileNotFound) {
			return nil, status.Error(codes.NotFound, "user profile not found")
		}
		if errors.Is(err, derror.ErrEmptyIdentityUpdate) || errors.Is(err, derror.ErrInvalidFullName) {
			return nil, status.Errorf(codes.InvalidArgument, "%v", err)
		}
		return nil, status.Errorf(codes.Internal, "update identity failed: %v", err)
	}

	return &profilev1message.UpdateIdentityResponse{
		UserId:    res.UserID,
		FullName:  res.FullName,
		AvatarUrl: res.AvatarURL,
		Message:   "Identity updated successfully",
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

	input, err := injuryInputFromProto(req)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	cmd := command.ReportInjuryCommand{
		InjuryID: uuid.New().String(),
		UserID:   targetUserID,
		Region:   input.Region,
		Muscle:   input.Muscle,
		Side:     input.Side,
		Severity: input.Severity,
		Notes:    input.Notes,
	}

	id, err := h.reportInjuryHandler.Handle(ctx, cmd)
	if err != nil {
		if errors.Is(err, derror.ErrProfileNotFound) {
			return nil, status.Error(codes.NotFound, "user profile not found")
		}
		if errors.Is(err, derror.ErrInjuryAlreadyActive) {
			return nil, status.Error(codes.AlreadyExists, err.Error())
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
		pbInjuries = append(pbInjuries, toProtoInjury(inj))
	}

	return &profilev1message.GetInjuryHistoryResponse{
		UserId:   targetUserID,
		Injuries: pbInjuries,
	}, nil
}

// injuryRequest is the injury field set shared by InjuryInput and ReportInjuryRequest.
type injuryRequest interface {
	GetRegion() exercisemsg.JointRegion
	GetMuscle() exercisemsg.MuscleGroup
	GetSide() exercisemsg.BodySide
	GetSeverity() exercisemsg.InjurySeverity
	GetNotes() string
}

// injuryInputFromProto requires region and severity; muscle is optional and
// an unset side means BOTH (docs/05 §6.5, §6.7).
func injuryInputFromProto(req injuryRequest) (command.InjuryInput, error) {
	region, err := protomap.JointRegionFromProto(req.GetRegion())
	if err != nil {
		return command.InjuryInput{}, err
	}
	muscle, err := protomap.OptionalMuscleGroupFromProto(req.GetMuscle())
	if err != nil {
		return command.InjuryInput{}, err
	}
	side, err := protomap.BodySideFromProto(req.GetSide())
	if err != nil {
		return command.InjuryInput{}, err
	}
	severity, err := protomap.InjurySeverityFromProto(req.GetSeverity())
	if err != nil {
		return command.InjuryInput{}, err
	}

	return command.InjuryInput{
		Region:   region,
		Muscle:   muscle,
		Side:     side,
		Severity: severity,
		Notes:    req.GetNotes(),
	}, nil
}

// trainingRequest is the equipment and muscle preference field set of profile writes.
type trainingRequest interface {
	GetAvailableEquipment() []exercisemsg.Equipment
	GetPreferredMuscleGroups() []exercisemsg.MuscleGroup
}

func trainingFromProto(req trainingRequest) ([]vo.Equipment, []vo.MuscleGroup, error) {
	equipment, err := protomap.EquipmentListFromProto(req.GetAvailableEquipment())
	if err != nil {
		return nil, nil, err
	}
	muscles, err := protomap.MuscleGroupsFromProto(req.GetPreferredMuscleGroups())
	if err != nil {
		return nil, nil, err
	}
	return equipment, muscles, nil
}

func toProtoInjury(inj *entity.Injury) *profilev1message.Injury {
	var recAt string
	if inj.RecoveredAt() != nil {
		recAt = inj.RecoveredAt().Format(time.RFC3339)
	}
	return &profilev1message.Injury{
		InjuryId:    inj.ID(),
		Region:      protomap.JointRegionToProto(inj.Region()),
		Muscle:      protomap.MuscleGroupToProto(inj.Muscle()),
		Side:        protomap.BodySideToProto(inj.Side()),
		Severity:    protomap.InjurySeverityToProto(inj.Severity()),
		Notes:       inj.Notes(),
		ReportedAt:  inj.ReportedAt().Format(time.RFC3339),
		IsRecovered: inj.IsRecovered(),
		RecoveredAt: recAt,
	}
}

// --- ConnectRPC Adapter ---

type ConnectProfileHandler struct {
	grpcHandler *GRPCHandler
}

var _ profilev1serviceconnect.ProfileServiceHandler = (*ConnectProfileHandler)(nil)

func NewConnectProfileHandler(grpcHandler *GRPCHandler) profilev1serviceconnect.ProfileServiceHandler {
	return &ConnectProfileHandler{grpcHandler: grpcHandler}
}

func (c *ConnectProfileHandler) SaveHealthProfile(
	ctx context.Context,
	req *profilev1message.SaveHealthProfileRequest,
) (*profilev1message.SaveHealthProfileResponse, error) {
	res, err := c.grpcHandler.SaveHealthProfile(ctx, req)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (c *ConnectProfileHandler) GetProfile(
	ctx context.Context,
	req *profilev1message.GetProfileRequest,
) (*profilev1message.GetProfileResponse, error) {
	res, err := c.grpcHandler.GetProfile(ctx, req)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (c *ConnectProfileHandler) UpdateProfile(
	ctx context.Context,
	req *profilev1message.UpdateProfileRequest,
) (*profilev1message.UpdateProfileResponse, error) {
	res, err := c.grpcHandler.UpdateProfile(ctx, req)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (c *ConnectProfileHandler) UpdateIdentity(
	ctx context.Context,
	req *profilev1message.UpdateIdentityRequest,
) (*profilev1message.UpdateIdentityResponse, error) {
	res, err := c.grpcHandler.UpdateIdentity(ctx, req)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (c *ConnectProfileHandler) LogPeriodicMetrics(
	ctx context.Context,
	req *profilev1message.LogPeriodicMetricsRequest,
) (*profilev1message.LogPeriodicMetricsResponse, error) {
	res, err := c.grpcHandler.LogPeriodicMetrics(ctx, req)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (c *ConnectProfileHandler) GetBodyMetricsHistory(
	ctx context.Context,
	req *profilev1message.GetBodyMetricsHistoryRequest,
) (*profilev1message.GetBodyMetricsHistoryResponse, error) {
	res, err := c.grpcHandler.GetBodyMetricsHistory(ctx, req)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (c *ConnectProfileHandler) ReportInjury(
	ctx context.Context,
	req *profilev1message.ReportInjuryRequest,
) (*profilev1message.ReportInjuryResponse, error) {
	res, err := c.grpcHandler.ReportInjury(ctx, req)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (c *ConnectProfileHandler) RecoverInjury(
	ctx context.Context,
	req *profilev1message.RecoverInjuryRequest,
) (*profilev1message.RecoverInjuryResponse, error) {
	res, err := c.grpcHandler.RecoverInjury(ctx, req)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (c *ConnectProfileHandler) GetInjuryHistory(
	ctx context.Context,
	req *profilev1message.GetInjuryHistoryRequest,
) (*profilev1message.GetInjuryHistoryResponse, error) {
	res, err := c.grpcHandler.GetInjuryHistory(ctx, req)
	if err != nil {
		return nil, err
	}
	return res, nil
}

// optionalTimezone parses a client zone name; empty yields the zero zone, which keeps the stored one.
func optionalTimezone(name string) (vo.Timezone, error) {
	if name == "" {
		return "", nil
	}
	return vo.ParseTimezone(name)
}
