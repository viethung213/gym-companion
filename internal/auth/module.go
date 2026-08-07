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

	"connectrpc.com/connect"
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
	authGRPC "github.com/viethung213/gym-companion/internal/auth/transport/grpc"
	"github.com/viethung213/gym-companion/internal/auth/infrastructure/worker"
	"github.com/viethung213/gym-companion/internal/gen/go/contracts/generic/auth/v1/service/authv1serviceconnect"
	sharedKafka "github.com/viethung213/gym-companion/internal/shared/kafka"
	"github.com/viethung213/gym-companion/internal/shared/middleware"
	gormPostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// ModuleDeps holds the external database connection and gRPC server instances needed by Auth.
type ModuleDeps struct {
	DB                *sql.DB
	AssignKeyProvider func(middleware.KeyProvider)
	KafkaRegistry     *sharedKafka.Registry
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

	// 2. Initialize GORM DB wrapper over sql.DB
	gormDB, err := gorm.Open(gormPostgres.New(gormPostgres.Config{
		Conn: deps.DB,
	}), &gorm.Config{
		PrepareStmt:            false,
		SkipDefaultTransaction: true,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("wrap connection pool in gorm: %w", err)
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
		deps.AssignKeyProvider(&authGRPC.AuthKeyProvider{KeyRepo: keyRepo})
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
	rotateKeysHandler := command.NewRotateKeysHandler(keyRepo, keyGen, txManager)
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

	writer, err := deps.KafkaRegistry.GetWriter("auth", kafkaBrokers)
	if err != nil {
		cancelWorkers()
		return nil, nil, fmt.Errorf("get auth kafka writer: %w", err)
	}

	kafkaPub := authKafka.NewPublisher(writer)
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
	grpcHandler := authGRPC.NewGRPCHandler(
		oauthLoginHandler,
		logoutHandler,
		rotateKeysHandler,
		refreshTokenHandler,
		getJWKSHandler,
		getOAuthLoginURLHandler,
	)

	log.Println("Auth Bounded Context initialized successfully.")
	return grpcHandler, shutdown, nil
}

// RegisterConnectHandler mounts the ConnectRPC handler for the Auth module on an http.ServeMux.
func RegisterConnectHandler(
	mux *http.ServeMux,
	grpcHandler *authGRPC.GRPCHandler,
	opts ...connect.HandlerOption,
) {
	connectHandler := authGRPC.NewConnectAuthHandler(grpcHandler)
	path, handler := authv1serviceconnect.NewAuthServiceHandler(connectHandler, opts...)
	mux.Handle(path, handler)
}
