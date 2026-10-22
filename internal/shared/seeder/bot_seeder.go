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
	Gender          authv1message.Gender
	DateOfBirth     string
	AvatarURL       string
	DifficultyLevel string // BEGINNER, INTERMEDIATE, ADVANCED
}

// DefaultBotProfiles returns predefined system bot profiles for Solo Match matchmaking.
func DefaultBotProfiles() []BotProfileSeed {
	return []BotProfileSeed{
		{
			Email:           "bot.hung@fitai.bot",
			Password:        "FitAIBot@2026!",
			FullName:        "Nguyễn Văn Hùng (AI Beast)",
			Gender:          authv1message.Gender_GENDER_MALE,
			DateOfBirth:     "1995-04-12",
			AvatarURL:       "https://images.unsplash.com/photo-1534528741775-53994a69daeb?auto=format&fit=crop&w=300&q=80",
			DifficultyLevel: "ADVANCED",
		},
		{
			Email:           "bot.mai@fitai.bot",
			Password:        "FitAIBot@2026!",
			FullName:        "Trần Thị Mai (AI Fit Pro)",
			Gender:          authv1message.Gender_GENDER_FEMALE,
			DateOfBirth:     "1998-08-23",
			AvatarURL:       "https://images.unsplash.com/photo-1517838277536-f5f99be501cd?auto=format&fit=crop&w=300&q=80",
			DifficultyLevel: "INTERMEDIATE",
		},
		{
			Email:           "bot.nam@fitai.bot",
			Password:        "FitAIBot@2026!",
			FullName:        "Lê Hoàng Nam (AI Rookie)",
			Gender:          authv1message.Gender_GENDER_MALE,
			DateOfBirth:     "2001-01-15",
			AvatarURL:       "https://images.unsplash.com/photo-1507003211169-0a1dd7228f2d?auto=format&fit=crop&w=300&q=80",
			DifficultyLevel: "BEGINNER",
		},
		{
			Email:           "bot.ha@fitai.bot",
			Password:        "FitAIBot@2026!",
			FullName:        "Phạm Thu Hà (AI Athletic)",
			Gender:          authv1message.Gender_GENDER_FEMALE,
			DateOfBirth:     "1997-11-30",
			AvatarURL:       "https://images.unsplash.com/photo-1544005313-94ddf0286df2?auto=format&fit=crop&w=300&q=80",
			DifficultyLevel: "INTERMEDIATE",
		},
		{
			Email:           "bot.bao@fitai.bot",
			Password:        "FitAIBot@2026!",
			FullName:        "Đặng Quốc Bảo (AI Titan)",
			Gender:          authv1message.Gender_GENDER_MALE,
			DateOfBirth:     "1993-06-19",
			AvatarURL:       "https://images.unsplash.com/photo-1500648767791-00dcc994a43e?auto=format&fit=crop&w=300&q=80",
			DifficultyLevel: "ADVANCED",
		},
		{
			Email:           "bot.duc@fitai.bot",
			Password:        "FitAIBot@2026!",
			FullName:        "Vũ Minh Đức (AI Iron)",
			Gender:          authv1message.Gender_GENDER_MALE,
			DateOfBirth:     "1996-09-10",
			AvatarURL:       "https://images.unsplash.com/photo-1506794778202-cad84cf45f1d?auto=format&fit=crop&w=300&q=80",
			DifficultyLevel: "ADVANCED",
		},
		{
			Email:           "bot.linh@fitai.bot",
			Password:        "FitAIBot@2026!",
			FullName:        "Hoàng Thùy Linh (AI Cardio)",
			Gender:          authv1message.Gender_GENDER_FEMALE,
			DateOfBirth:     "2000-03-25",
			AvatarURL:       "https://images.unsplash.com/photo-1494790108377-be9c29b29330?auto=format&fit=crop&w=300&q=80",
			DifficultyLevel: "BEGINNER",
		},
		{
			Email:           "bot.dung@fitai.bot",
			Password:        "FitAIBot@2026!",
			FullName:        "Bùi Tiến Dũng (AI Striker)",
			Gender:          authv1message.Gender_GENDER_MALE,
			DateOfBirth:     "1999-12-05",
			AvatarURL:       "https://images.unsplash.com/photo-1519085360753-af0119f7cbe7?auto=format&fit=crop&w=300&q=80",
			DifficultyLevel: "INTERMEDIATE",
		},
		{
			Email:           "bot.thao@fitai.bot",
			Password:        "FitAIBot@2026!",
			FullName:        "Đỗ Phương Thảo (AI Core)",
			Gender:          authv1message.Gender_GENDER_FEMALE,
			DateOfBirth:     "1998-07-14",
			AvatarURL:       "https://images.unsplash.com/photo-1524504388940-b1c1722653e1?auto=format&fit=crop&w=300&q=80",
			DifficultyLevel: "INTERMEDIATE",
		},
		{
			Email:           "bot.hai@fitai.bot",
			Password:        "FitAIBot@2026!",
			FullName:        "Ngô Quang Hải (AI Power)",
			Gender:          authv1message.Gender_GENDER_MALE,
			DateOfBirth:     "1994-05-18",
			AvatarURL:       "https://images.unsplash.com/photo-1492562080023-ab3db95bfbce?auto=format&fit=crop&w=300&q=80",
			DifficultyLevel: "ADVANCED",
		},
	}
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

	profiles := DefaultBotProfiles()

	// 1. Kiểm tra nếu tất cả bot đã được tạo rùi thì không cần chạy lại
	if db != nil {
		allSeeded := true
		for _, bot := range profiles {
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
			log.Printf("🤖 System bots already created (%d bots exist). Skipping bot seeder.", len(profiles))
			return nil
		}
	}

	log.Printf("🤖 Starting API-driven System Bot seeding (%d bots)...", len(profiles))

	for _, bot := range profiles {
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

	log.Printf("🎉 Successfully seeded %d System Bots via Auth API.", len(profiles))
	return nil
}
