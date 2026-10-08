//go:build unit

package grpc_test

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	profilev1message "github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/profile/v1/message"
	"github.com/viethung213/gym-companion/internal/profile/application/command"
	"github.com/viethung213/gym-companion/internal/profile/application/query"
	"github.com/viethung213/gym-companion/internal/profile/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/profile/domain/derror"
	"github.com/viethung213/gym-companion/internal/profile/domain/entity"
	"github.com/viethung213/gym-companion/internal/profile/domain/vo"
	"github.com/viethung213/gym-companion/internal/profile/transport/grpc"
	"github.com/viethung213/gym-companion/internal/shared/middleware"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type mockRepo struct {
	profiles map[string]*aggregate.UserProfile
	failRepo bool
}

func (m *mockRepo) Save(ctx context.Context, p *aggregate.UserProfile) error {
	if m.failRepo {
		return errors.New("db save fail")
	}
	m.profiles[p.UserID()] = p
	return nil
}

func (m *mockRepo) FindByUserID(ctx context.Context, userID string) (*aggregate.UserProfile, error) {
	if m.failRepo {
		return nil, errors.New("db find fail")
	}
	p, ok := m.profiles[userID]
	if !ok {
		return nil, derror.ErrProfileNotFound
	}
	return p, nil
}

func (m *mockRepo) Update(ctx context.Context, p *aggregate.UserProfile) error {
	if m.failRepo {
		return errors.New("db update fail")
	}
	m.profiles[p.UserID()] = p
	return nil
}

func (m *mockRepo) FindBodyMetricsHistory(ctx context.Context, userID string) ([]vo.PeriodicMetric, error) {
	if m.failRepo {
		return nil, errors.New("db metrics history fail")
	}
	p, ok := m.profiles[userID]
	if !ok {
		return nil, nil
	}
	return p.PeriodicMetrics(), nil
}

func (m *mockRepo) FindInjuryHistory(ctx context.Context, userID string) ([]*entity.Injury, error) {
	if m.failRepo {
		return nil, errors.New("db injury history fail")
	}
	p, ok := m.profiles[userID]
	if !ok {
		return nil, nil
	}
	return p.Injuries(), nil
}

type mockTxManager struct{}

func (m *mockTxManager) WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

type mockEventPub struct{}

func (m *mockEventPub) PublishEvents(ctx context.Context, events []any) error {
	return nil
}

func setupGRPCHandler(repo *mockRepo) *grpc.GRPCHandler {
	txManager := &mockTxManager{}
	eventPub := &mockEventPub{}

	saveHandler := command.NewSaveHealthProfileHandler(repo, eventPub, txManager)
	updateHandler := command.NewUpdateProfileHandler(repo, eventPub, txManager)
	updateIdentityHandler := command.NewUpdateIdentityHandler(repo, eventPub, txManager)
	logHandler := command.NewLogPeriodicMetricsHandler(repo, eventPub, txManager)
	reportHandler := command.NewReportInjuryHandler(repo, eventPub, txManager)
	recoverHandler := command.NewRecoverInjuryHandler(repo, eventPub, txManager)
	getHandler := query.NewGetProfileHandler(repo)
	getMetricsHandler := query.NewGetBodyMetricsHistoryHandler(repo)
	getInjuriesHandler := query.NewGetInjuryHistoryHandler(repo)

	return grpc.NewGRPCHandler(
		saveHandler,
		updateHandler,
		updateIdentityHandler,
		logHandler,
		reportHandler,
		recoverHandler,
		getHandler,
		getMetricsHandler,
		getInjuriesHandler,
	)
}

func TestGRPCHandler_Endpoints(t *testing.T) {
	repo := &mockRepo{profiles: make(map[string]*aggregate.UserProfile)}
	handler := setupGRPCHandler(repo)
	ctx := context.WithValue(context.Background(), middleware.UserIDKey, "grpc-user-1")

	// 1. SaveHealthProfile
	t.Run("SaveHealthProfile", func(t *testing.T) {
		req := &profilev1message.SaveHealthProfileRequest{
			WeightKg:              75.0,
			HeightCm:              180.0,
			DateOfBirth:           "1998-05-15",
			Gender:                "MALE",
			Goals:                 []string{"MUSCLE_GAIN"},
			ExperienceLevel:       "INTERMEDIATE",
			PreferredWorkoutTimes: []string{"MORNING"},
			AvailableEquipment:    []string{"DUMBBELL"},
			PreferredMuscleGroups: []string{"CHEST"},
			CoachStyle:            "FRIENDLY",
			TargetWeightKg:        78.0,
			TargetBodyFatPercent:  12.0,
			Injuries: []*profilev1message.InjuryInput{
				{MuscleGroup: "Knee", Severity: "MILD", Notes: "Strain"},
			},
		}

		res, err := handler.SaveHealthProfile(ctx, req)
		require.NoError(t, err)
		assert.Equal(t, "grpc-user-1", res.GetUserId())
		assert.True(t, res.GetAiCoachActivated())

		// Internal error case
		repo.failRepo = true
		_, err = handler.SaveHealthProfile(ctx, req)
		assert.Error(t, err)
		repo.failRepo = false
	})

	// 2. GetProfile
	t.Run("GetProfile", func(t *testing.T) {
		req := &profilev1message.GetProfileRequest{}
		res, err := handler.GetProfile(ctx, req)
		require.NoError(t, err)
		assert.Equal(t, "grpc-user-1", res.GetUserId())
		assert.Equal(t, float32(75.0), res.GetWeightKg())

		// Not found case
		nonExistCtx := context.WithValue(context.Background(), middleware.UserIDKey, "non-existent")
		_, err = handler.GetProfile(nonExistCtx, &profilev1message.GetProfileRequest{})
		assert.Error(t, err)

		// Internal error case
		repo.failRepo = true
		_, err = handler.GetProfile(ctx, req)
		assert.Error(t, err)
		repo.failRepo = false
	})

	// 3. UpdateProfile
	t.Run("UpdateProfile", func(t *testing.T) {
		req := &profilev1message.UpdateProfileRequest{
			WeightKg:        76.0,
			HeightCm:        180.0,
			DateOfBirth:     "1998-05-15",
			Gender:          "MALE",
			Goals:           []string{"MUSCLE_GAIN"},
			ExperienceLevel: "ADVANCED",
		}
		res, err := handler.UpdateProfile(ctx, req)
		require.NoError(t, err)
		assert.True(t, res.GetSuccess())

		// Not found case
		nonExistCtx := context.WithValue(context.Background(), middleware.UserIDKey, "non-existent")
		_, err = handler.UpdateProfile(nonExistCtx, &profilev1message.UpdateProfileRequest{})
		assert.Error(t, err)

		// Internal error case
		repo.failRepo = true
		_, err = handler.UpdateProfile(ctx, req)
		assert.Error(t, err)
		repo.failRepo = false
	})

	// 4. LogPeriodicMetrics
	t.Run("LogPeriodicMetrics", func(t *testing.T) {
		req := &profilev1message.LogPeriodicMetricsRequest{
			WeightKg:       77.0,
			HeightCm:       180.0,
			BodyFatPercent: 14.5,
		}
		res, err := handler.LogPeriodicMetrics(ctx, req)
		require.NoError(t, err)
		assert.NotEmpty(t, res.GetLogId())

		// Not found case
		nonExistCtx := context.WithValue(context.Background(), middleware.UserIDKey, "non-existent")
		_, err = handler.LogPeriodicMetrics(nonExistCtx, &profilev1message.LogPeriodicMetricsRequest{})
		assert.Error(t, err)

		// Internal error case
		repo.failRepo = true
		_, err = handler.LogPeriodicMetrics(ctx, req)
		assert.Error(t, err)
		repo.failRepo = false
	})

	// 5. GetBodyMetricsHistory
	t.Run("GetBodyMetricsHistory", func(t *testing.T) {
		req := &profilev1message.GetBodyMetricsHistoryRequest{}
		res, err := handler.GetBodyMetricsHistory(ctx, req)
		require.NoError(t, err)
		assert.Equal(t, "grpc-user-1", res.GetUserId())
		assert.NotEmpty(t, res.GetMetrics())

		// Internal error case
		repo.failRepo = true
		_, err = handler.GetBodyMetricsHistory(ctx, req)
		assert.Error(t, err)
		repo.failRepo = false
	})

	// 6. ReportInjury & RecoverInjury & GetInjuryHistory
	t.Run("ReportAndRecoverInjuryAndHistory", func(t *testing.T) {
		repReq := &profilev1message.ReportInjuryRequest{
			MuscleGroup: "Shoulder",
			Severity:    "MILD",
			Notes:       "Pain during press",
		}
		repRes, err := handler.ReportInjury(ctx, repReq)
		require.NoError(t, err)
		assert.True(t, repRes.GetSuccess())
		injuryID := repRes.GetInjuryId()

		// Report Injury Not Found case
		nonExistCtx := context.WithValue(context.Background(), middleware.UserIDKey, "non-existent")
		_, err = handler.ReportInjury(nonExistCtx, repReq)
		assert.Error(t, err)

		// Report Injury Internal Error case
		repo.failRepo = true
		_, err = handler.ReportInjury(ctx, repReq)
		assert.Error(t, err)
		repo.failRepo = false

		// GetInjuryHistory
		histReq := &profilev1message.GetInjuryHistoryRequest{}
		histRes, err := handler.GetInjuryHistory(ctx, histReq)
		require.NoError(t, err)
		assert.NotEmpty(t, histRes.GetInjuries())

		// GetInjuryHistory Internal Error
		repo.failRepo = true
		_, err = handler.GetInjuryHistory(ctx, histReq)
		assert.Error(t, err)
		repo.failRepo = false

		// RecoverInjury missing injury_id
		_, err = handler.RecoverInjury(ctx, &profilev1message.RecoverInjuryRequest{InjuryId: ""})
		assert.Error(t, err)

		// RecoverInjury Non-existent injury
		_, err = handler.RecoverInjury(ctx, &profilev1message.RecoverInjuryRequest{InjuryId: "inj-not-found"})
		assert.Error(t, err)

		// RecoverInjury Internal Error
		repo.failRepo = true
		_, err = handler.RecoverInjury(ctx, &profilev1message.RecoverInjuryRequest{InjuryId: injuryID})
		assert.Error(t, err)
		repo.failRepo = false

		// RecoverInjury Success
		recReq := &profilev1message.RecoverInjuryRequest{
			InjuryId: injuryID,
		}
		recRes, err := handler.RecoverInjury(ctx, recReq)
		require.NoError(t, err)
		assert.True(t, recRes.GetSuccess())

		// Re-fetch GetProfile and GetInjuryHistory to cover RecoveredAt != nil branches
		pRes, err := handler.GetProfile(ctx, &profilev1message.GetProfileRequest{})
		require.NoError(t, err)
		assert.Len(t, pRes.GetInjuries(), 2)
		assert.NotEmpty(t, pRes.GetInjuries()[1].GetRecoveredAt())

		hRes, err := handler.GetInjuryHistory(ctx, &profilev1message.GetInjuryHistoryRequest{})
		require.NoError(t, err)
		assert.Len(t, hRes.GetInjuries(), 2)
		assert.NotEmpty(t, hRes.GetInjuries()[1].GetRecoveredAt())
	})

	// 7. Security verification (Auth Bypass & BOLA protection)
	t.Run("Security Checks", func(t *testing.T) {
		// Case A: Unauthenticated request with target user_id in payload must return Unauthenticated
		_, err := handler.GetProfile(context.Background(), &profilev1message.GetProfileRequest{UserId: "victim-user-123"})
		require.Error(t, err)
		st, ok := status.FromError(err)
		assert.True(t, ok)
		assert.Equal(t, codes.Unauthenticated, st.Code())

		// Case B: Normal User attempting to access another user's profile must return PermissionDenied (BOLA protection)
		normalUserCtx := context.WithValue(context.Background(), middleware.UserIDKey, "normal-user-1")
		normalUserCtx = context.WithValue(normalUserCtx, middleware.UserRoleKey, "User")
		_, err = handler.GetProfile(normalUserCtx, &profilev1message.GetProfileRequest{UserId: "other-user-999"})
		require.Error(t, err)
		st, ok = status.FromError(err)
		assert.True(t, ok)
		assert.Equal(t, codes.PermissionDenied, st.Code())

		// Case C: Admin User accessing another user's profile must succeed
		adminUserCtx := context.WithValue(context.Background(), middleware.UserIDKey, "admin-1")
		adminUserCtx = context.WithValue(adminUserCtx, middleware.UserRoleKey, "Admin")
		res, err := handler.GetProfile(adminUserCtx, &profilev1message.GetProfileRequest{UserId: "grpc-user-1"})
		require.NoError(t, err)
		assert.Equal(t, "grpc-user-1", res.GetUserId())
	})

	// 8. UpdateIdentity
	t.Run("UpdateIdentity", func(t *testing.T) {
		name := "Le Van D"
		avatar := "https://example.com/avatar_d.png"

		// Success updating both
		req := &profilev1message.UpdateIdentityRequest{
			FullName:  &name,
			AvatarUrl: &avatar,
		}
		res, err := handler.UpdateIdentity(ctx, req)
		require.NoError(t, err)
		assert.Equal(t, "grpc-user-1", res.GetUserId())
		assert.Equal(t, name, res.GetFullName())
		assert.Equal(t, avatar, res.GetAvatarUrl())

		// Verify GetProfile returns updated name & avatar
		pRes, err := handler.GetProfile(ctx, &profilev1message.GetProfileRequest{})
		require.NoError(t, err)
		assert.Equal(t, name, pRes.GetFullName())
		assert.Equal(t, avatar, pRes.GetAvatarUrl())

		// Update only full name
		newName := "Le Van E"
		resOnlyName, err := handler.UpdateIdentity(ctx, &profilev1message.UpdateIdentityRequest{FullName: &newName})
		require.NoError(t, err)
		assert.Equal(t, newName, resOnlyName.GetFullName())
		assert.Equal(t, avatar, resOnlyName.GetAvatarUrl())

		// Update only avatar
		newAvatar := "https://example.com/avatar_e.png"
		resOnlyAvatar, err := handler.UpdateIdentity(ctx, &profilev1message.UpdateIdentityRequest{AvatarUrl: &newAvatar})
		require.NoError(t, err)
		assert.Equal(t, newName, resOnlyAvatar.GetFullName())
		assert.Equal(t, newAvatar, resOnlyAvatar.GetAvatarUrl())

		// Update only avatar with empty string resets to DefaultAvatarURL
		emptyAvatar := ""
		resEmptyAvatar, err := handler.UpdateIdentity(ctx, &profilev1message.UpdateIdentityRequest{AvatarUrl: &emptyAvatar})
		require.NoError(t, err)
		assert.Equal(t, aggregate.DefaultAvatarURL, resEmptyAvatar.GetAvatarUrl())

		// Error: Unauthenticated
		_, err = handler.UpdateIdentity(context.Background(), req)
		require.Error(t, err)
		st, ok := status.FromError(err)
		assert.True(t, ok)
		assert.Equal(t, codes.Unauthenticated, st.Code())

		// Error: Profile not found
		nonExistCtx := context.WithValue(context.Background(), middleware.UserIDKey, "non-existent")
		_, err = handler.UpdateIdentity(nonExistCtx, req)
		require.Error(t, err)
		st, ok = status.FromError(err)
		assert.True(t, ok)
		assert.Equal(t, codes.NotFound, st.Code())

		// Error: Invalid argument (empty update)
		_, err = handler.UpdateIdentity(ctx, &profilev1message.UpdateIdentityRequest{})
		require.Error(t, err)
		st, ok = status.FromError(err)
		assert.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())

		// Error: Invalid argument (name > 100)
		longName := ""
		for i := 0; i < 101; i++ {
			longName += "x"
		}
		_, err = handler.UpdateIdentity(ctx, &profilev1message.UpdateIdentityRequest{FullName: &longName})
		require.Error(t, err)
		st, ok = status.FromError(err)
		assert.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())

		// Error: Internal repo error
		repo.failRepo = true
		_, err = handler.UpdateIdentity(ctx, req)
		require.Error(t, err)
		st, ok = status.FromError(err)
		assert.True(t, ok)
		assert.Equal(t, codes.Internal, st.Code())
		repo.failRepo = false

		// ConnectRPC adapter coverage
		connectHandler := grpc.NewConnectProfileHandler(handler)
		cRes, err := connectHandler.UpdateIdentity(ctx, connect.NewRequest(&profilev1message.UpdateIdentityRequest{
			FullName: &name,
		}))
		require.NoError(t, err)
		assert.Equal(t, "grpc-user-1", cRes.Msg.GetUserId())
		assert.Equal(t, name, cRes.Msg.GetFullName())
	})
}
