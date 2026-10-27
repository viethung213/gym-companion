// Package seeder provides application-level data seeding for system bots and virtual accounts.
package seeder

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"time"

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
// 1. Checks if all default bot profiles already exist in auth and system_bots; if so, skips execution.
// 2. Calls authService.Register for each bot profile through the API layer (Transport -> Application -> Domain -> Infrastructure).
// 3. Retrieves each bot's user_id from auth.user_identities and inserts/upserts into solo_match.system_bots.
func SeedSystemBots(ctx context.Context, db *sql.DB, authService AuthRegistrar) error {
	if authService == nil {
		return errors.New("authService cannot be nil")
	}

	profiles := DefaultBotProfiles()

	// 1. Kiểm tra nếu tất cả bot đã được tạo trong cả auth và solo_match.system_bots thì bỏ qua
	if db != nil {
		allSeeded := true
		for _, bot := range profiles {
			var authExists bool
			err := db.QueryRowContext(ctx, `
				SELECT EXISTS (
					SELECT 1 
					FROM auth.user_identities 
					WHERE identity_type = 'email' AND identifier = $1
				)
			`, bot.Email).Scan(&authExists)
			if err != nil || !authExists {
				allSeeded = false
				break
			}

			var botExists bool
			err = db.QueryRowContext(ctx, `
				SELECT EXISTS (
					SELECT 1 
					FROM solo_match.system_bots b
					JOIN auth.user_identities i ON b.user_id = i.user_id
					WHERE i.identity_type = 'email' AND i.identifier = $1
				)
			`, bot.Email).Scan(&botExists)
			if err != nil || !botExists {
				allSeeded = false
				break
			}
		}

		if allSeeded {
			log.Printf("🤖 System bots already created and synced in solo_match.system_bots (%d bots exist). Skipping bot seeder.", len(profiles))
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
				log.Printf("  ℹ️ Bot already exists in Auth: %s (%s)", bot.FullName, bot.Email)
			} else {
				return fmt.Errorf("register bot %s via auth API: %w", bot.Email, err)
			}
		} else {
			log.Printf("  ✅ Bot registered via Auth API: %s (%s)", bot.FullName, bot.Email)
		}

		// Nạp dữ liệu vào bảng solo_match.system_bots
		if db != nil {
			var userID string
			err = db.QueryRowContext(ctx, `
				SELECT user_id 
				FROM auth.user_identities 
				WHERE identity_type = 'email' AND identifier = $1
			`, bot.Email).Scan(&userID)
			if err != nil {
				return fmt.Errorf("lookup user_id for bot %s: %w", bot.Email, err)
			}

			// Chờ sự kiện UserRegistered được Solo Match consumer đồng bộ vào solo_match.user_snapshots
			if waitErr := waitForUserSnapshot(ctx, db, userID, 15*time.Second); waitErr != nil {
				return fmt.Errorf("wait for snapshot sync for bot %s (user_id %s): %w", bot.Email, userID, waitErr)
			}

			// Thêm/cập nhật vào solo_match.system_bots
			_, err = db.ExecContext(ctx, `
				INSERT INTO solo_match.system_bots (user_id, difficulty_level, is_active, created_at)
				VALUES ($1, $2, $3, CURRENT_TIMESTAMP)
				ON CONFLICT (user_id) DO UPDATE 
				SET difficulty_level = EXCLUDED.difficulty_level, is_active = EXCLUDED.is_active
			`, userID, bot.DifficultyLevel, true)
			if err != nil {
				return fmt.Errorf("upsert solo_match.system_bots for bot %s: %w", bot.Email, err)
			}

			log.Printf("  🎯 Bot synced to solo_match.system_bots: %s (ID: %s, Difficulty: %s)", bot.FullName, userID, bot.DifficultyLevel)
		}
	}

	log.Printf("🎉 Successfully seeded and synced %d System Bots.", len(profiles))
	return nil
}

// waitForUserSnapshot polls solo_match.user_snapshots until the snapshot with the given userID is created,
// or until the timeout is reached or context is cancelled.
func waitForUserSnapshot(ctx context.Context, db *sql.DB, userID string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		var exists bool
		err := db.QueryRowContext(ctx, `
			SELECT EXISTS (
				SELECT 1 
				FROM solo_match.user_snapshots 
				WHERE user_id = $1
			)
		`, userID).Scan(&exists)
		if err == nil && exists {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if time.Now().After(deadline) {
				return fmt.Errorf("timeout waiting for snapshot sync of user_id %s in solo_match.user_snapshots (%v elapsed)", userID, timeout)
			}
		}
	}
}
