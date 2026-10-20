// Package seeder provides application-level data seeding for system bots and virtual accounts.
package seeder

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"

	authv1message "github.com/viethung213/gym-companion/internal/gen/go/contracts/generic/auth/v1/message"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// BotProfileSeed defines the seed data structure for an AI bot athlete.
type BotProfileSeed struct {
	Email           string
	Password        string
	FullName        string
	Gender          string
	DateOfBirth     string
	AvatarURL       string
	DifficultyLevel string // BEGINNER, INTERMEDIATE, ADVANCED
}

// DefaultBotProfiles contains predefined system bot profiles for Solo Match matchmaking.
var DefaultBotProfiles = []BotProfileSeed{
	{
		Email:           "bot.alex@fitai.bot",
		Password:        "FitAIBot@2026!",
		FullName:        "Alex Vance (AI Beast)",
		Gender:          "MALE",
		DateOfBirth:     "1995-04-12",
		AvatarURL:       "https://images.unsplash.com/photo-1534528741775-53994a69daeb?auto=format&fit=crop&w=300&q=80",
		DifficultyLevel: "ADVANCED",
	},
	{
		Email:           "bot.sarah@fitai.bot",
		Password:        "FitAIBot@2026!",
		FullName:        "Sarah Jenkins (AI Pro)",
		Gender:          "FEMALE",
		DateOfBirth:     "1998-08-23",
		AvatarURL:       "https://images.unsplash.com/photo-1517838277536-f5f99be501cd?auto=format&fit=crop&w=300&q=80",
		DifficultyLevel: "INTERMEDIATE",
	},
	{
		Email:           "bot.leo@fitai.bot",
		Password:        "FitAIBot@2026!",
		FullName:        "Leo Martinez (AI Rookie)",
		Gender:          "MALE",
		DateOfBirth:     "2001-01-15",
		AvatarURL:       "https://images.unsplash.com/photo-1507003211169-0a1dd7228f2d?auto=format&fit=crop&w=300&q=80",
		DifficultyLevel: "BEGINNER",
	},
	{
		Email:           "bot.maya@fitai.bot",
		Password:        "FitAIBot@2026!",
		FullName:        "Maya Lin (AI Athletic)",
		Gender:          "FEMALE",
		DateOfBirth:     "1997-11-30",
		AvatarURL:       "https://images.unsplash.com/photo-1544005313-94ddf0286df2?auto=format&fit=crop&w=300&q=80",
		DifficultyLevel: "INTERMEDIATE",
	},
	{
		Email:           "bot.marcus@fitai.bot",
		Password:        "FitAIBot@2026!",
		FullName:        "Marcus Stone (AI Titan)",
		Gender:          "MALE",
		DateOfBirth:     "1993-06-19",
		AvatarURL:       "https://images.unsplash.com/photo-1500648767791-00dcc994a43e?auto=format&fit=crop&w=300&q=80",
		DifficultyLevel: "ADVANCED",
	},
}

// AuthRegistrar defines the API contract required to register user accounts.
// Implemented by authv1service.AuthServiceServer and *authGRPC.GRPCHandler.
type AuthRegistrar interface {
	Register(ctx context.Context, req *authv1message.RegisterRequest) (*authv1message.RegisterResponse, error)
}

// SeedSystemBots seeds bot accounts by calling the AuthService API:
// 1. Checks if all default bot profiles already exist; if so, skips execution to prevent redundant operations.
// 2. Calls authService.Register for each bot profile through the API layer (Transport -> Application -> Domain -> Infrastructure).
// 3. The system's event-driven pipeline (Outbox -> Kafka -> Consumers) handles downstream profile creation & synchronization.
func SeedSystemBots(ctx context.Context, db *sql.DB, authService AuthRegistrar) error {
	if authService == nil {
		return errors.New("authService cannot be nil")
	}

	// 1. Kiểm tra nếu tất cả bot đã được tạo rùi thì không cần chạy lại
	if db != nil {
		allSeeded := true
		for _, bot := range DefaultBotProfiles {
			var exists bool
			err := db.QueryRowContext(ctx, `
				SELECT EXISTS (
					SELECT 1 
					FROM auth.user_identities 
					WHERE identity_type = 'email' AND identifier = $1
				)
			`, bot.Email).Scan(&exists)
			if err != nil || !exists {
				allSeeded = false
				break
			}
		}

		if allSeeded {
			log.Printf("🤖 System bots already created (%d bots exist). Skipping bot seeder.", len(DefaultBotProfiles))
			return nil
		}
	}

	log.Printf("🤖 Starting API-driven System Bot seeding (%d bots)...", len(DefaultBotProfiles))

	for _, bot := range DefaultBotProfiles {
		req := &authv1message.RegisterRequest{
			Identifier:  bot.Email,
			Password:    bot.Password,
			FullName:    bot.FullName,
			Gender:      bot.Gender,
			DateOfBirth: bot.DateOfBirth,
		}

		// Gọi trực tiếp qua API Register (đi qua Transport -> Application -> Domain -> Infra)
		_, err := authService.Register(ctx, req)
		if err != nil {
			st, ok := status.FromError(err)
			if ok && st.Code() == codes.AlreadyExists {
				log.Printf("  ℹ️ Bot already exists: %s (%s)", bot.FullName, bot.Email)
				continue
			}
			return fmt.Errorf("register bot %s via auth API: %w", bot.Email, err)
		}

		log.Printf("  ✅ Bot registered via Auth API: %s (%s)", bot.FullName, bot.Email)
	}

	log.Printf("🎉 Successfully seeded %d System Bots via Auth API.", len(DefaultBotProfiles))
	return nil
}
