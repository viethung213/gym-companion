package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/viethung213/gym-companion/internal/auth/application/apperror"
	"github.com/viethung213/gym-companion/internal/auth/application/command"
	"github.com/viethung213/gym-companion/internal/auth/application/query"
	"github.com/viethung213/gym-companion/internal/auth/infrastructure/config"
	"github.com/viethung213/gym-companion/internal/auth/infrastructure/crypto"
	authEvent "github.com/viethung213/gym-companion/internal/auth/infrastructure/event"
	"github.com/viethung213/gym-companion/internal/auth/infrastructure/jwt"
	authKafka "github.com/viethung213/gym-companion/internal/auth/infrastructure/kafka"
	"github.com/viethung213/gym-companion/internal/auth/infrastructure/oauth"
	"github.com/viethung213/gym-companion/internal/auth/infrastructure/persistence/postgres"
	grpcAuth "github.com/viethung213/gym-companion/internal/auth/infrastructure/transport/grpc"
	"github.com/viethung213/gym-companion/internal/auth/infrastructure/worker"
	authv1service "github.com/viethung213/gym-companion/internal/gen/go/contracts/generic/auth/v1/service"
	"github.com/viethung213/gym-companion/internal/shared/middleware"
	"google.golang.org/grpc"
	gormPostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// ModuleDeps holds the external database connection and gRPC server instances needed by Auth.
type ModuleDeps struct {
	DB                *sql.DB
	GRPCServer        *grpc.Server
	AssignKeyProvider func(middleware.KeyProvider)
}

// Initialize bootstraps all layers of the Auth Bounded Context.
func Initialize(ctx context.Context, deps ModuleDeps) (func(), error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	// 2. Initialize GORM DB wrapper over sql.DB
	gormDB, err := gorm.Open(gormPostgres.New(gormPostgres.Config{
		Conn: deps.DB,
	}), &gorm.Config{
		SkipDefaultTransaction: true,
	})
	if err != nil {
		return nil, fmt.Errorf("wrap connection pool in gorm: %w", err)
	}

	// 3. Initialize Repositories
	userRepo := postgres.NewUserRepository(gormDB)
	keyRepo := postgres.NewKeyRepository(gormDB)
	sessRepo := postgres.NewSessionRepository(gormDB)
	outboxRepo := postgres.NewOutboxRepository(gormDB)

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
	if deps.AssignKeyProvider != nil {
		deps.AssignKeyProvider(&grpcAuth.AuthKeyProvider{KeyRepo: keyRepo})
	}
	keyGen := crypto.NewRSAKeyGenerator()
	txManager := postgres.NewSQLTransactionManager(gormDB)
	eventPub := authEvent.NewOutboxWriter(outboxRepo)

	// 5. Initialize Application Handlers
	oauthLoginHandler := command.NewOAuthLoginHandler(
		userRepo,
		keyRepo,
		sessRepo,
		tokenServ,
		oauthServ,
		eventPub,
		txManager,
	)
	logoutHandler := command.NewLogoutHandler(sessRepo)
	rotateKeysHandler := command.NewRotateKeysHandler(keyRepo, keyGen)
	getJWKSHandler := query.NewGetJWKSHandler(keyRepo)
	getOAuthLoginURLHandler := query.NewGetOAuthLoginURLHandler(oauthServ)

	// 6. Ensure there is at least one active key in DB at startup
	_, err = keyRepo.GetActiveKey(ctx)
	if err != nil {
		if errors.Is(err, apperror.ErrKeyNotFound) {
			log.Println("No active signing key found. Generating initial active key...")
			_, err = rotateKeysHandler.Handle(ctx, command.RotateKeysCommand{
				KeyTTL: cfg.KeyRotationTTL,
			})
			if err != nil {
				return nil, fmt.Errorf("generate initial active key: %w", err)
			}
		} else {
			return nil, fmt.Errorf("get active key on startup failed: %w", err)
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
				log.Printf("PANIC RECOVERED in background key rotation check: %v", r)
			}
		}()

		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-wCtx.Done():
				return
			case <-ticker.C:
				log.Println("Running background active key rotation check...")
				activeKey, err := keyRepo.GetActiveKey(wCtx)
				if err == nil {
					if time.Until(activeKey.ExpiresAt) < 24*time.Hour {
						log.Println("Active key near expiration. Triggering key rotation...")
						_, err = rotateKeysHandler.Handle(wCtx, command.RotateKeysCommand{
							KeyTTL: cfg.KeyRotationTTL,
						})
						if err != nil {
							log.Printf("ERROR: Background automated key rotation failed: %v", err)
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

	kafkaPub := authKafka.NewPublisher(kafkaBrokers)
	outboxWorker := worker.NewOutboxWorker(outboxRepo, kafkaPub, 1*time.Second)

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer func() {
			if r := recover(); r != nil {
				log.Printf("PANIC RECOVERED in Outbox background worker: %v", r)
			}
		}()
		outboxWorker.Start(workerCtx)
	}()

	// Shutdown callback function
	var shutdownOnce sync.Once
	shutdown := func() {
		shutdownOnce.Do(func() {
			log.Println("Shutting down Auth Bounded Context background workers...")
			cancelWorkers()
			wg.Wait()
			log.Println("Auth Bounded Context background workers stopped. Closing Kafka publisher...")
			if err := kafkaPub.Close(); err != nil {
				log.Printf("WARNING: failed to close auth Kafka publisher: %v", err)
			}
			log.Println("Auth Bounded Context Kafka publisher closed successfully.")
		})
	}

	refreshTokenHandler := command.NewRefreshTokenHandler(userRepo, keyRepo, sessRepo, tokenServ)

	// 9. Register AuthService Server to gRPC Server
	grpcHandler := grpcAuth.NewGRPCHandler(
		oauthLoginHandler,
		logoutHandler,
		rotateKeysHandler,
		refreshTokenHandler,
		getJWKSHandler,
		getOAuthLoginURLHandler,
	)
	authv1service.RegisterAuthServiceServer(deps.GRPCServer, grpcHandler)

	log.Println("Auth Bounded Context initialized successfully.")
	return shutdown, nil
}

// RegisterGateway configures and registers the gRPC-Gateway multiplexer for the Auth module.
func RegisterGateway(
	ctx context.Context,
	mux *http.ServeMux,
	grpcEndpoint string,
	opts []grpc.DialOption,
) error {
	gwmux := runtime.NewServeMux()
	err := authv1service.RegisterAuthServiceHandlerFromEndpoint(ctx, gwmux, grpcEndpoint, opts)
	if err != nil {
		return fmt.Errorf("register auth service gateway handler: %w", err)
	}

	// Mount gRPC-Gateway onto the main HTTP multiplexer
	mux.Handle("/", gwmux)
	return nil
}
