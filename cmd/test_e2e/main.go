package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/segmentio/kafka-go"
	"github.com/viethung213/gym-companion/internal/gamification"
	workoutv1event "github.com/viethung213/gym-companion/internal/gen/go/contracts/core/workout_execution/v1/event"
	authv1event "github.com/viethung213/gym-companion/internal/gen/go/contracts/generic/auth/v1/event"
	gamificationv1msg "github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/gamification/v1/message"
	sharedKafka "github.com/viethung213/gym-companion/internal/shared/kafka"
	"github.com/viethung213/gym-companion/internal/shared/middleware"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type CloudEventEnvelope struct {
	SpecVersion     string          `json:"specversion"`
	ID              string          `json:"id"`
	Source          string          `json:"source"`
	Type            string          `json:"type"`
	Time            string          `json:"time"`
	DataContentType string          `json:"datacontenttype"`
	Data            json.RawMessage `json:"data"`
}

func main() {
	if err := run(); err != nil {
		log.Fatalf("fatal error: %v", err) //nolint:forbidigo // allowed in main
	}
}

func run() error {
	log.Println("🚀 === BẮT ĐẦU CHẠY KIỂM THỬ E2E GAMIFICATION TRÊN LOCAL DOCKER ===")

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:postgres@localhost:5432/fitai?sslmode=disable"
	}
	kafkaBroker := os.Getenv("KAFKA_BROKERS")
	if kafkaBroker == "" {
		kafkaBroker = "localhost:9094"
	}
	_ = os.Setenv("KAFKA_BROKERS", kafkaBroker)

	log.Printf("📌 Kết nối Database: %s", dbURL)
	log.Printf("📌 Kết nối Kafka Broker: %s", kafkaBroker)

	// 1. Kết nối DB
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()

	if pingErr := db.Ping(); pingErr != nil {
		return fmt.Errorf("ping database: %w", pingErr)
	}
	log.Println("✅ Kết nối PostgreSQL thành công!")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 2. Khởi tạo Kafka Registry & Gamification Module
	kafkaRegistry := sharedKafka.GetRegistry()
	defer kafkaRegistry.CloseAll()

	grpcHandler, shutdown, err := gamification.Initialize(ctx, gamification.ModuleDeps{
		DB:            db,
		KafkaRegistry: kafkaRegistry,
	})
	if err != nil {
		return fmt.Errorf("initialize gamification: %w", err)
	}
	defer shutdown()
	log.Println("✅ Gamification Module và 5 Kafka Consumers đã sẵn sàng lắng nghe!")

	log.Println("⏳ Chờ 3s để Kafka Consumer Group rebalance và gán partition...")
	time.Sleep(3 * time.Second)

	// 3. Chuẩn bị Test User ID (UUID chuẩn)
	testUserID := uuid.New().String()
	log.Printf("👤 Test User ID: %s", testUserID)

	// 4. Bắn Event 1: User Registered
	log.Println("\n--- [BƯỚC 1] BẮN EVENT USER REGISTERED (auth.events) ---")
	authWriter := &kafka.Writer{
		Addr:     kafka.TCP(kafkaBroker),
		Topic:    "auth.events",
		Balancer: &kafka.LeastBytes{},
	}
	defer authWriter.Close()

	userRegisteredProto := &authv1event.UserRegistered{
		UserId:       testUserID,
		IdentityType: "EMAIL",
		Identifier:   "gymer@test.com",
		FullName:     "Nguyen Van Gymer",
		AvatarUrl:    "https://example.com/avatar.jpg",
		RegisteredAt: timestamppb.Now(),
	}
	userRegisteredData, _ := protojson.Marshal(userRegisteredProto)

	sendCloudEvent(ctx, authWriter, testUserID, "contracts.generic.auth.v1.userRegistered", userRegisteredData)
	log.Println("📤 Đã bắn UserRegistered vào topic auth.events!")

	// Polling kiểm tra DB tối đa 10s
	var xpCount, streakCount, snapshotCount int
	for i := 0; i < 20; i++ {
		time.Sleep(500 * time.Millisecond)
		_ = db.QueryRow("SELECT COUNT(*) FROM gamification.user_xp WHERE user_id = $1", testUserID).Scan(&xpCount)
		_ = db.QueryRow("SELECT COUNT(*) FROM gamification.user_streak WHERE user_id = $1", testUserID).Scan(&streakCount)
		_ = db.QueryRow("SELECT COUNT(*) FROM gamification.user_snapshots WHERE user_id = $1", testUserID).Scan(&snapshotCount)
		if xpCount > 0 && streakCount > 0 && snapshotCount > 0 {
			break
		}
	}

	fmt.Printf("🔍 [DB Check 1] user_xp: %d, user_streak: %d, user_snapshots: %d\n", xpCount, streakCount, snapshotCount)
	if xpCount == 0 || streakCount == 0 || snapshotCount == 0 {
		return errors.New("user not created in db")
	}
	log.Println("✅ [BƯỚC 1 THÀNH CÔNG] User đã được eager initialize trong Gamification DB!")

	// 5. Bắn Event 2: Workout Session Completed
	log.Println("\n--- [BƯỚC 2] BẮN EVENT WORKOUT SESSION COMPLETED (workout_execution.events) ---")
	workoutWriter := &kafka.Writer{
		Addr:     kafka.TCP(kafkaBroker),
		Topic:    "workout_execution.events",
		Balancer: &kafka.LeastBytes{},
	}
	defer workoutWriter.Close()

	sessionID := uuid.New().String()
	avgFormScore := float32(92.5)
	workoutProto := &workoutv1event.WorkoutSessionCompleted{
		SessionId:        sessionID,
		UserId:           testUserID,
		TotalSets:        5,
		TotalVolume:      3200.0,
		AverageFormScore: &avgFormScore,
		AverageRpe:       8.0,
		CompletedAt:      timestamppb.Now(),
	}
	workoutData, _ := protojson.Marshal(workoutProto)

	sendCloudEvent(ctx, workoutWriter, testUserID, "contracts.core.workout_execution.v1.event.WorkoutSessionCompleted", workoutData)
	log.Println("📤 Đã bắn WorkoutSessionCompleted vào topic workout_execution.events!")

	// Polling kiểm tra DB sau Bước 2
	var currentXp, currentLevel int64
	var currentStreak int
	var firstWorkoutBadgeCount int
	for i := 0; i < 20; i++ {
		time.Sleep(500 * time.Millisecond)
		_ = db.QueryRow("SELECT xp, level FROM gamification.user_xp WHERE user_id = $1", testUserID).Scan(&currentXp, &currentLevel)
		_ = db.QueryRow("SELECT current_streak FROM gamification.user_streak WHERE user_id = $1", testUserID).Scan(&currentStreak)
		_ = db.QueryRow("SELECT COUNT(*) FROM gamification.user_badges WHERE user_id = $1 AND badge_code = 'FIRST_WORKOUT'", testUserID).Scan(&firstWorkoutBadgeCount)
		if currentXp > 0 && currentStreak == 1 && firstWorkoutBadgeCount == 1 {
			break
		}
	}

	fmt.Printf("🔍 [DB Check 2] XP: %d, Level: %d, Current Streak: %d, Badge FIRST_WORKOUT: %d\n", currentXp, currentLevel, currentStreak, firstWorkoutBadgeCount)
	if currentXp == 0 || currentStreak != 1 || firstWorkoutBadgeCount != 1 {
		return errors.New("unexpected workout completion state in db")
	}
	log.Println("✅ [BƯỚC 2 THÀNH CÔNG] XP đã được cộng, Streak = 1, Huy hiệu FIRST_WORKOUT đã mở khóa!")

	// 6. Bắn Event 3: Personal Record Achieved
	log.Println("\n--- [BƯỚC 3] BẮN EVENT PERSONAL RECORD ACHIEVED (workout_execution.events) ---")
	prProto := &workoutv1event.NewPersonalRecordAchieved{
		UserId:     testUserID,
		ExerciseId: uuid.New().String(),
		OneRepMax:  100.0,
		Weight:     100.0,
		Reps:       1,
		AchievedAt: timestamppb.Now(),
	}
	prData, _ := protojson.Marshal(prProto)

	sendCloudEvent(ctx, workoutWriter, testUserID, "contracts.core.workout_execution.v1.event.NewPersonalRecordAchieved", prData)
	log.Println("📤 Đã bắn NewPersonalRecordAchieved vào topic workout_execution.events!")

	// Polling kiểm tra DB sau Bước 3
	var prXp int64
	var prBadgeCount int
	for i := 0; i < 20; i++ {
		time.Sleep(500 * time.Millisecond)
		_ = db.QueryRow("SELECT xp FROM gamification.user_xp WHERE user_id = $1", testUserID).Scan(&prXp)
		_ = db.QueryRow("SELECT COUNT(*) FROM gamification.user_badges WHERE user_id = $1 AND badge_code = 'PR_BREAKER'", testUserID).Scan(&prBadgeCount)
		if prXp == currentXp+25 && prBadgeCount == 1 {
			break
		}
	}

	fmt.Printf("🔍 [DB Check 3] XP mới: %d (trước đó: %d), Badge PR_BREAKER: %d\n", prXp, currentXp, prBadgeCount)
	if prXp != currentXp+25 || prBadgeCount != 1 {
		return errors.New("unexpected personal record state in db")
	}
	log.Println("✅ [BƯỚC 3 THÀNH CÔNG] Đã cộng đúng +25 XP và mở khóa huy hiệu PR_BREAKER!")

	// 7. Bắn Event 4: Nutrition Goal Achieved
	log.Println("\n--- [BƯỚC 4] BẮN EVENT NUTRITION GOAL ACHIEVED (nutrition.events) ---")
	nutritionWriter := &kafka.Writer{
		Addr:     kafka.TCP(kafkaBroker),
		Topic:    "nutrition.events",
		Balancer: &kafka.LeastBytes{},
	}
	defer nutritionWriter.Close()

	nutritionPayload := map[string]interface{}{
		"userId":       testUserID,
		"activityDate": time.Now().Format("2006-01-02"),
	}
	nutritionData, _ := json.Marshal(nutritionPayload)

	sendCloudEvent(ctx, nutritionWriter, testUserID, "contracts.supporting.nutrition.v1.event.DailyNutritionGoalAchieved", nutritionData)
	log.Println("📤 Đã bắn DailyNutritionGoalAchieved vào topic nutrition.events!")

	var nutritionXp int64
	for i := 0; i < 20; i++ {
		time.Sleep(500 * time.Millisecond)
		_ = db.QueryRow("SELECT xp FROM gamification.user_xp WHERE user_id = $1", testUserID).Scan(&nutritionXp)
		if nutritionXp == prXp+30 {
			break
		}
	}

	fmt.Printf("🔍 [DB Check 4] XP mới: %d (trước đó: %d)\n", nutritionXp, prXp)
	if nutritionXp != prXp+30 {
		return errors.New("unexpected nutrition xp state in db")
	}
	log.Println("✅ [BƯỚC 4 THÀNH CÔNG] Đã cộng đúng +30 XP cho dinh dưỡng!")

	// 8. Kiểm tra Outbox Table
	log.Println("\n--- [BƯỚC 5] KIỂM TRA TRANSACTIONAL OUTBOX TABLE ---")
	var outboxCount int
	_ = db.QueryRow("SELECT COUNT(*) FROM gamification.outbox WHERE aggregate_id = $1", testUserID).Scan(&outboxCount)
	fmt.Printf("🔍 [DB Check 5] Tổng số sự kiện Outbox đã sinh ra: %d\n", outboxCount)

	rows, rErr := db.Query("SELECT event_type, status, created_at FROM gamification.outbox WHERE aggregate_id = $1 ORDER BY created_at ASC", testUserID)
	if rErr == nil {
		for rows.Next() {
			var evType, status string
			var createdAt time.Time
			_ = rows.Scan(&evType, &status, &createdAt)
			fmt.Printf("   -> Outbox Event: %s | Status: %s | CreatedAt: %s\n", evType, status, createdAt.Format(time.RFC3339))
		}
		rows.Close()
	}

	// 9. Kiểm tra API Query qua ConnectRPC Handler
	log.Println("\n--- [BƯỚC 6] GỌI TRỰC TIẾP API HANDLER ĐỂ QUERY THÔNG TIN ---")
	userCtx := context.WithValue(ctx, middleware.UserIDKey, testUserID)

	// A. GetMyXp
	xpReq := connect.NewRequest(&gamificationv1msg.GetMyXpRequest{})
	xpReq.Header().Set("X-User-Id", testUserID)
	xpResp, err := grpcHandler.GetMyXp(userCtx, xpReq)
	if err != nil {
		log.Printf("⚠️ GetMyXp lỗi: %v", err)
	} else if xpResp.Msg.GetXpSummary() != nil {
		summary := xpResp.Msg.GetXpSummary()
		fmt.Printf("📊 [API GetMyXp] Total XP: %d | Level: %d | Level Progress: %.1f%% | To Next Level: %d\n",
			summary.GetXp(), summary.GetLevel(), summary.GetLevelProgressPercentage(), summary.GetXpToNextLevel())
	}

	// B. GetStreak
	streakReq := connect.NewRequest(&gamificationv1msg.GetStreakRequest{})
	streakReq.Header().Set("X-User-Id", testUserID)
	streakResp, err := grpcHandler.GetStreak(userCtx, streakReq)
	if err != nil {
		log.Printf("⚠️ GetStreak lỗi: %v", err)
	} else if streakResp.Msg.GetStreakSummary() != nil {
		streak := streakResp.Msg.GetStreakSummary()
		fmt.Printf("🔥 [API GetStreak] Current: %d | Longest: %d | Is Active: %v | Last Active: %s\n",
			streak.GetCurrentStreak(), streak.GetLongestStreak(), streak.GetIsActiveToday(), streak.GetLastActiveDate())
	}

	// C. GetMyBadges
	badgeReq := connect.NewRequest(&gamificationv1msg.GetMyBadgesRequest{})
	badgeReq.Header().Set("X-User-Id", testUserID)
	badgeResp, err := grpcHandler.GetMyBadges(userCtx, badgeReq)
	if err != nil {
		log.Printf("⚠️ GetMyBadges lỗi: %v", err)
	} else {
		fmt.Printf("🎖️  [API GetMyBadges] Tổng số huy hiệu sở hữu: %d\n", len(badgeResp.Msg.GetBadges()))
		for _, b := range badgeResp.Msg.GetBadges() {
			fmt.Printf("   -> Badge: %s (%s) - Mở lúc: %s\n", b.GetBadgeCode(), b.GetName(), b.GetUnlockedAt().AsTime().Format(time.RFC3339))
		}
	}

	// D. GetGlobalLeaderboard
	boardReq := connect.NewRequest(&gamificationv1msg.GetGlobalLeaderboardRequest{Limit: 10})
	boardResp, err := grpcHandler.GetGlobalLeaderboard(userCtx, boardReq)
	if err != nil {
		log.Printf("⚠️ GetGlobalLeaderboard lỗi: %v", err)
	} else {
		fmt.Printf("🏆 [API Leaderboard] Top %d Gymer:\n", len(boardResp.Msg.GetTopEntries()))
		for _, entry := range boardResp.Msg.GetTopEntries() {
			fmt.Printf("   #%d: %s | Level %d | %d XP\n", entry.GetRank(), entry.GetDisplayName(), entry.GetLevel(), entry.GetXp())
		}
		if boardResp.Msg.GetMyStanding() != nil {
			my := boardResp.Msg.GetMyStanding()
			fmt.Printf("   👉 Thứ hạng của bạn: #%d (%s) - Level %d - %d XP\n", my.GetRank(), my.GetDisplayName(), my.GetLevel(), my.GetXp())
		}
	}

	log.Println("\n🎉🎉🎉 === TẤT CẢ CÁC BƯỚC KIỂM THỬ E2E GAMIFICATION ĐÃ HOÀN TOÀN THÀNH CÔNG! ===")
	return nil
}

func sendCloudEvent(ctx context.Context, writer *kafka.Writer, partitionKey, eventType string, data []byte) {
	ce := CloudEventEnvelope{
		SpecVersion:     "1.0",
		ID:              uuid.New().String(),
		Source:          "gym-companion/test-suite",
		Type:            eventType,
		Time:            time.Now().UTC().Format(time.RFC3339),
		DataContentType: "application/json",
		Data:            data,
	}
	bytes, _ := json.Marshal(ce)

	err := writer.WriteMessages(ctx, kafka.Message{
		Key:   []byte(partitionKey),
		Value: bytes,
	})
	if err != nil {
		log.Printf("❌ Lỗi gửi message vào Kafka (topic %s): %v", writer.Topic, err)
	}
}
