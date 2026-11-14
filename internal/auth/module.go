package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"connectrpc.com/connect/v2"
	"github.com/viethung213/gym-companion/internal/auth/application/apperror"
	"github.com/viethung213/gym-companion/internal/auth/application/command"
	"github.com/viethung213/gym-companion/internal/auth/application/query"
	"github.com/viethung213/gym-companion/internal/auth/infrastructure/config"
	"github.com/viethung213/gym-companion/internal/auth/infrastructure/crypto"
	authEvent "github.com/viethung213/gym-companion/internal/auth/infrastructure/event"
	"github.com/viethung213/gym-companion/internal/auth/infrastructure/jwt"
	authKafka "github.com/viethung213/gym-companion/internal/auth/infrastructure/kafka"
	"github.com/viethung213/gym-companion/internal/auth/infrastructure/oauth"
	"github.com/viethung213/gym-companion/internal/auth/infrastructure/persistence"
	"github.com/viethung213/gym-companion/internal/auth/infrastructure/worker"
	authGRPC "github.com/viethung213/gym-companion/internal/auth/transport/grpc"
	"github.com/viethung213/gym-companion/internal/gen/go/contracts/generic/auth/v1/service/authv1serviceconnect"
	"github.com/viethung213/gym-companion/internal/shared/database"
	sharedKafka "github.com/viethung213/gym-companion/internal/shared/kafka"
)

// ModuleDeps holds the external database connection and gRPC server instances needed by Auth.
type ModuleDeps struct {
	DB            *sql.DB
	KafkaRegistry *sharedKafka.Registry
}

// Initialize bootstraps all layers of the Auth Bounded Context.
func Initialize(ctx context.Context, deps ModuleDeps) (*authGRPC.GRPCHandler, func(), error) {
	if deps.DB == nil {
		return nil, nil, errors.New("deps.DB is required")
	}
	if deps.KafkaRegistry == nil {
		return nil, nil, errors.New("deps.KafkaRegistry is required")
	}

	cfg, err := config.Load()
	if err != nil {
		return nil, nil, fmt.Errorf("load config: %w", err)
	}

	// 2. Initialize GORM DB wrapper over sql.DB with OpenTelemetry tracing
	gormDB, err := database.OpenGorm(deps.DB, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("wrap connection pool in gorm: %w", err)
	}

	// 3. Initialize Repositories
	userRepo := persistence.NewUserRepository(gormDB)
	keyRepo := persistence.NewKeyRepository(gormDB)
	sessRepo := persistence.NewSessionRepository(gormDB)
	outboxRepo := persistence.NewOutboxRepository(gormDB)

	// 4. Initialize Services & Adapters
	googleConfig := oauth.ProviderConfig{
		ClientID:     cfg.GoogleClientID,
		ClientSecret: cfg.GoogleClientSecret,
		RedirectURI:  cfg.GoogleRedirectURI,
	}
	facebookConfig := oauth.ProviderConfig{
		ClientID:     cfg.FacebookClientID,
		ClientSecret: cfg.FacebookClientSecret,
		RedirectURI:  cfg.FacebookRedirectURI,
	}
	oauthServ := oauth.NewOAuthProvider(googleConfig, facebookConfig, cfg.StateSecret)

	tokenServ := jwt.NewJWTSigner(keyRepo, cfg.JWTIssuer, cfg.AccessTokenTTL, cfg.RefreshTokenTTL)
	keyGen := crypto.NewRSAKeyGenerator()
	txManager := persistence.NewSQLTransactionManager(gormDB)
	eventRecorder := authEvent.NewOutboxWriter(outboxRepo)

	// 5. Initialize Application Handlers
	oauthLoginHandler := command.NewOAuthLoginHandler(
		userRepo,
		keyRepo,
		sessRepo,
		tokenServ,
		oauthServ,
		eventRecorder,
		txManager,
	)
	logoutHandler := command.NewLogoutHandler(sessRepo)
	rotateKeysHandler := command.NewRotateKeysHandler(keyRepo, keyGen, txManager)
	getJWKSHandler := query.NewGetJWKSHandler(keyRepo)
	getOAuthLoginURLHandler := query.NewGetOAuthLoginURLHandler(oauthServ)

	// 6. Ensure there is at least one active key in DB at startup
	_, err = keyRepo.GetActiveKey(ctx)
	if err != nil {
		if errors.Is(err, apperror.ErrKeyNotFound) {
			slog.Info("No active signing key found, generating initial active key")
			_, err = rotateKeysHandler.Handle(ctx, command.RotateKeysCommand{
				KeyTTL: cfg.KeyRotationTTL,
			})
			if err != nil {
				return nil, nil, fmt.Errorf("generate initial active key: %w", err)
			}
		} else {
			return nil, nil, fmt.Errorf("get active key on startup failed: %w", err)
		}
	}

	// 7. Start Background Worker for Key Rotation (runs check every 1 hour)
	workerCtx, cancelWorkers := context.WithCancel(ctx)
	var wg sync.WaitGroup

	wg.Add(1)
	go func(wCtx context.Context) {
		defer wg.Done()
		defer func() {
			if r := recover(); r != nil {
				slog.Error("PANIC RECOVERED in background key rotation check", slog.Any("panic", r))
			}
		}()

		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-wCtx.Done():
				return
			case <-ticker.C:
				slog.Debug("Running background active key rotation check")
				activeKey, err := keyRepo.GetActiveKey(wCtx)
				if err == nil {
					if time.Until(activeKey.ExpiresAt) < 24*time.Hour {
						slog.Info("Active key near expiration, triggering key rotation")
						_, err = rotateKeysHandler.Handle(wCtx, command.RotateKeysCommand{
							KeyTTL: cfg.KeyRotationTTL,
						})
						if err != nil {
							slog.Error("Background automated key rotation failed", slog.String("error", err.Error()))
						}
					}
				}
			}
		}
	}(workerCtx)

	// 8. Initialize and start Kafka Publisher and Outbox Worker
	kafkaBrokersStr := os.Getenv("AUTH_KAFKA_BROKERS")
	if kafkaBrokersStr == "" {
		kafkaBrokersStr = os.Getenv("KAFKA_BROKERS")
	}
	if kafkaBrokersStr == "" {
		kafkaBrokersStr = "localhost:9092"
	}
	kafkaBrokers := strings.Split(kafkaBrokersStr, ",")

	writer, err := deps.KafkaRegistry.GetWriter("auth.events", kafkaBrokers)
	if err != nil {
		cancelWorkers()
		return nil, nil, fmt.Errorf("get auth kafka writer: %w", err)
	}

	highPriorityWriter, hpErr := deps.KafkaRegistry.GetWriter("notification.events.high-priority", kafkaBrokers)
	if hpErr != nil {
		slog.Warn("Failed to get high-priority notification kafka writer", slog.String("error", hpErr.Error()))
	}

	kafkaPub := authKafka.NewPublisher(writer, highPriorityWriter)
	outboxWorker := worker.NewOutboxWorker(outboxRepo, kafkaPub, 1*time.Second)

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer func() {
			if r := recover(); r != nil {
				slog.Error("PANIC RECOVERED in Outbox background worker", slog.Any("panic", r))
			}
		}()
		outboxWorker.Start(workerCtx)
	}()

	// Shutdown callback function
	var shutdownOnce sync.Once
	shutdown := func() {
		shutdownOnce.Do(func() {
			slog.Info("Shutting down Auth Bounded Context background workers")
			cancelWorkers()
			wg.Wait()
			slog.Info("Auth Bounded Context background workers stopped, closing Kafka publisher")
			if err := kafkaPub.Close(); err != nil {
				slog.Warn("Failed to close auth Kafka publisher", slog.String("error", err.Error()))
			}
			slog.Info("Auth Bounded Context Kafka publisher closed successfully")
		})
	}

	refreshTokenHandler := command.NewRefreshTokenHandler(userRepo, keyRepo, sessRepo, tokenServ)
	argon2Hasher := crypto.NewArgon2Hasher()
	otpRepo := persistence.NewOTPRepository(gormDB)
	resetTokenRepo := persistence.NewPasswordResetTokenRepository(gormDB)

	registerHandler := command.NewRegisterHandler(userRepo, argon2Hasher, eventRecorder, txManager)
	credentialsLoginHandler := command.NewCredentialsLoginHandler(userRepo, argon2Hasher, keyRepo, tokenServ, sessRepo)
	sendOTPHandler := command.NewSendOTPHandler(otpRepo, argon2Hasher, eventRecorder, txManager)
	verifyOTPHandler := command.NewVerifyOTPHandler(otpRepo, resetTokenRepo, userRepo, argon2Hasher)
	changePasswordHandler := command.NewChangePasswordHandler(userRepo, argon2Hasher)
	resetPasswordHandler := command.NewResetPasswordHandler(resetTokenRepo, userRepo, argon2Hasher, txManager)
	brandRepo := persistence.NewBrandRequestRepository(gormDB)
	registerBrandHandler := command.NewRegisterBrandHandler(brandRepo, userRepo)
	approveBrandHandler := command.NewApproveBrandHandler(brandRepo, userRepo, txManager, eventRecorder)
	rejectBrandHandler := command.NewRejectBrandHandler(brandRepo)
	listBrandRequestsHandler := query.NewListBrandRequestsHandler(brandRepo)
	listUsersHandler := query.NewListUsersHandler(userRepo)
	lockUserHandler := command.NewLockUserHandler(userRepo, sessRepo)
	unlockUserHandler := command.NewUnlockUserHandler(userRepo)

	// 9. Register AuthService Server to gRPC Server
	grpcHandler := authGRPC.NewGRPCHandler(
		oauthLoginHandler,
		logoutHandler,
		rotateKeysHandler,
		refreshTokenHandler,
		getJWKSHandler,
		getOAuthLoginURLHandler,
		registerHandler,
		credentialsLoginHandler,
		sendOTPHandler,
		verifyOTPHandler,
		changePasswordHandler,
		resetPasswordHandler,
		registerBrandHandler,
		approveBrandHandler,
		rejectBrandHandler,
		listBrandRequestsHandler,
		listUsersHandler,
		lockUserHandler,
		unlockUserHandler,
	)

	slog.Info("Auth Bounded Context initialized successfully")
	return grpcHandler, shutdown, nil
}

// RegisterConnectHandler registers the ConnectRPC handler for the Auth module on a connect.Server.
func RegisterConnectHandler(
	server *connect.Server,
	grpcHandler *authGRPC.GRPCHandler,
) string {
	connectHandler := authGRPC.NewConnectAuthHandler(grpcHandler)
	authv1serviceconnect.RegisterAuthServiceHandler(server, connectHandler)
	return "/" + authv1serviceconnect.AuthServiceName + "/"
}
