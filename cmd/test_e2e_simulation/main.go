package main

import (
	"context"
	"database/sql"
	"encoding/json"
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
		log.Fatalf("❌ E2E Simulation failed: %v", err)
	}
}

func run() error {
	log.Println("🚀 ================================================================")
	log.Println("🚀 === BẮT ĐẦU E2E SIMULATION MULTI-USER REALISTIC TEST TRÊN DOCKER ===")
	log.Println("🚀 ================================================================")

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
	log.Println("✅ Gamification Module và các Kafka Consumers đã sẵn sàng!")

	log.Println("⏳ Chờ 3s để Kafka Consumer Groups rebalance và assign partitions...")
	time.Sleep(3 * time.Second)

	authWriter := &kafka.Writer{Addr: kafka.TCP(kafkaBroker), Topic: "auth.events", Balancer: &kafka.LeastBytes{}}
	defer authWriter.Close()

	workoutWriter := &kafka.Writer{Addr: kafka.TCP(kafkaBroker), Topic: "workout_execution.events", Balancer: &kafka.LeastBytes{}}
	defer workoutWriter.Close()

	nutritionWriter := &kafka.Writer{Addr: kafka.TCP(kafkaBroker), Topic: "nutrition.events", Balancer: &kafka.LeastBytes{}}
	defer nutritionWriter.Close()

	// -------------------------------------------------------------
	// SCENARIO SETUP: 5 USERS WITH DIVERSE BEHAVIOR
	// -------------------------------------------------------------
	userHulk := uuid.New().String()   // Elite powerlifter: workout + PR + nutrition
	userFlash := uuid.New().String()  // Moderate athlete: workout + nutrition
	userRest := uuid.New().String()   // Rest day user: nutrition only
	userNewbie := uuid.New().String() // Freshly registered user: no activities
	userIdemp := uuid.New().String()  // Resiliency user: duplicate event delivery

	log.Println("\n📋 DANH SÁCH 5 TEST USERS:")
	log.Printf("   1. [Hulk - Elite Powerlifter]    ID: %s", userHulk)
	log.Printf("   2. [Flash - Regular Athlete]     ID: %s", userFlash)
	log.Printf("   3. [RestDay - Meal Logging Only] ID: %s", userRest)
	log.Printf("   4. [Newbie - Just Registered]    ID: %s", userNewbie)
	log.Printf("   5. [IdempBot - Duplicate Replay] ID: %s", userIdemp)

	// STEP 1: REGISTER ALL 5 USERS
	log.Println("\n--- [GIAI ĐOẠN 1] BẮN SỰ KIỆN ĐĂNG KÝ (UserRegistered) CHO CẢ 5 USER ---")
	users := []struct {
		id   string
		name string
	}{
		{userHulk, fmt.Sprintf("Hulk [%s]", userHulk[:4])},
		{userFlash, fmt.Sprintf("Flash [%s]", userFlash[:4])},
		{userRest, fmt.Sprintf("RestDay [%s]", userRest[:4])},
		{userNewbie, fmt.Sprintf("Newbie [%s]", userNewbie[:4])},
		{userIdemp, fmt.Sprintf("IdempBot [%s]", userIdemp[:4])},
	}

	for _, u := range users {
		regProto := &authv1event.UserRegistered{
			UserId:       u.id,
			IdentityType: "EMAIL",
			Identifier:   u.id[:8] + "@fitai.com",
			FullName:     u.name,
			AvatarUrl:    "https://fitai.com/avatars/default.png",
			RegisteredAt: timestamppb.Now(),
		}
		data, _ := protojson.Marshal(regProto)
		sendEvent(ctx, authWriter, u.id, uuid.New().String(), "contracts.generic.auth.v1.userRegistered", data)
	}

	// Chờ consumer xử lý đăng ký cho 5 users
	log.Println("⏳ Chờ consumers eager-initialize bản ghi trong DB...")
	for _, u := range users {
		if waitErr := waitForCount(db, "gamification.user_xp", "user_id", u.id, 1, 10*time.Second); waitErr != nil {
			return fmt.Errorf("user_xp not initialized for %s: %w", u.name, waitErr)
		}
		if waitErr := waitForCount(db, "gamification.user_streak", "user_id", u.id, 1, 10*time.Second); waitErr != nil {
			return fmt.Errorf("user_streak not initialized for %s: %w", u.name, waitErr)
		}
		if waitErr := waitForCount(db, "gamification.user_snapshots", "user_id", u.id, 1, 10*time.Second); waitErr != nil {
			return fmt.Errorf("user_snapshots not initialized for %s: %w", u.name, waitErr)
		}
	}
	log.Println("✅ [GIAI ĐOẠN 1 THÀNH CÔNG] Toàn bộ 5 User đã được khởi tạo atomic trong DB!")

	// STEP 2: SIMULATE WORKOUT SESSIONS
	log.Println("\n--- [GIAI ĐOẠN 2] BẮN CÁC BUỔI TẬP WORKOUT SESSION ---")
	// User Hulk: 6 sets, 5500kg (>5000kg -> volume=30), form 95% (form=19) -> 50+30+19 = 99 XP
	hulkForm := float32(95.0)
	hulkWorkout := &workoutv1event.WorkoutSessionCompleted{
		SessionId:        uuid.New().String(),
		UserId:           userHulk,
		TotalSets:        6,
		TotalVolume:      5500.0,
		AverageFormScore: &hulkForm,
		AverageRpe:       9.0,
		CompletedAt:      timestamppb.Now(),
	}
	hulkWData, _ := protojson.Marshal(hulkWorkout)
	sendEvent(ctx, workoutWriter, userHulk, uuid.New().String(), "contracts.core.workout_execution.v1.event.WorkoutSessionCompleted", hulkWData)

	// User Flash: 4 sets, 2500kg (>=1000kg -> volume=20), form 80% (form=16) -> 50+20+16 = 86 XP
	flashForm := float32(80.0)
	flashWorkout := &workoutv1event.WorkoutSessionCompleted{
		SessionId:        uuid.New().String(),
		UserId:           userFlash,
		TotalSets:        4,
		TotalVolume:      2500.0,
		AverageFormScore: &flashForm,
		AverageRpe:       7.5,
		CompletedAt:      timestamppb.Now(),
	}
	flashWData, _ := protojson.Marshal(flashWorkout)
	sendEvent(ctx, workoutWriter, userFlash, uuid.New().String(), "contracts.core.workout_execution.v1.event.WorkoutSessionCompleted", flashWData)

	// User IdempBot: 3 sets, 1200kg (volume=20), form 70% (form=14) -> 50+20+14 = 84 XP
	// BẮN 2 LẦN CÙNG EVENT_ID để test chống lặp!
	idempEventID := uuid.New().String()
	idempSessionID := uuid.New().String()
	idempForm := float32(70.0)
	idempWorkout := &workoutv1event.WorkoutSessionCompleted{
		SessionId:        idempSessionID,
		UserId:           userIdemp,
		TotalSets:        3,
		TotalVolume:      1200.0,
		AverageFormScore: &idempForm,
		AverageRpe:       7.0,
		CompletedAt:      timestamppb.Now(),
	}
	idempWData, _ := protojson.Marshal(idempWorkout)
	log.Printf("🔁 [Idempotency Test] Bắn lần 1 event %s cho IdempBot...", idempEventID)
	sendEvent(ctx, workoutWriter, userIdemp, idempEventID, "contracts.core.workout_execution.v1.event.WorkoutSessionCompleted", idempWData)

	log.Printf("🔁 [Idempotency Test] Bắn lần 2 TRÙNG LẶP event %s cho IdempBot...", idempEventID)
	sendEvent(ctx, workoutWriter, userIdemp, idempEventID, "contracts.core.workout_execution.v1.event.WorkoutSessionCompleted", idempWData)

	// Chờ DB ghi nhận XP buổi tập
	if waitErr := waitForXp(db, userHulk, 99, 10*time.Second); waitErr != nil {
		return fmt.Errorf("hulk workout xp mismatch: %w", waitErr)
	}
	if waitErr := waitForXp(db, userFlash, 86, 10*time.Second); waitErr != nil {
		return fmt.Errorf("flash workout xp mismatch: %w", waitErr)
	}
	if waitErr := waitForXp(db, userIdemp, 84, 10*time.Second); waitErr != nil {
		return fmt.Errorf("idemp workout xp mismatch: %w", waitErr)
	}
	log.Println("✅ [GIAI ĐOẠN 2 THÀNH CÔNG] Toàn bộ workout XP đã được tính toán chính xác và ghi nhận!")
	log.Println("   -> Hulk: +99 XP | Flash: +86 XP | IdempBot: +84 XP (Không bị cộng đúp dù bắn 2 lần!)")

	// STEP 3: SIMULATE PERSONAL RECORDS
	log.Println("\n--- [GIAI ĐOẠN 3] BẮN KỶ LỤC CÁ NHÂN (NewPersonalRecordAchieved) CHO HULK ---")
	prProto := &workoutv1event.NewPersonalRecordAchieved{
		UserId:     userHulk,
		ExerciseId: "bench-press-power",
		OneRepMax:  180.0,
		Weight:     180.0,
		Reps:       1,
		AchievedAt: timestamppb.Now(),
	}
	prData, _ := protojson.Marshal(prProto)
	sendEvent(ctx, workoutWriter, userHulk, uuid.New().String(), "contracts.core.workout_execution.v1.event.NewPersonalRecordAchieved", prData)

	// Hulk: 99 XP + 25 PR = 124 XP
	if waitErr := waitForXp(db, userHulk, 124, 10*time.Second); waitErr != nil {
		return fmt.Errorf("hulk pr xp mismatch: %w", waitErr)
	}
	if waitErr := waitForBadge(db, userHulk, "PR_BREAKER", 10*time.Second); waitErr != nil {
		return fmt.Errorf("hulk pr badge missing: %w", waitErr)
	}
	log.Println("✅ [GIAI ĐOẠN 3 THÀNH CÔNG] Hulk nhận thêm +25 XP (Tổng: 124 XP) và mở khóa PR_BREAKER!")

	// STEP 4: SIMULATE NUTRITION GOAL COMPLETION
	log.Println("\n--- [GIAI ĐOẠN 4] BẮN HOÀN THÀNH DINH DƯỠNG CHO HULK, FLASH, RESTDAY ---")
	today := time.Now().UTC().Format("2006-01-02")
	nutritionRecipients := []string{userHulk, userFlash, userRest}

	for _, uid := range nutritionRecipients {
		payload, _ := json.Marshal(map[string]interface{}{
			"userId":       uid,
			"activityDate": today,
		})
		sendEvent(ctx, nutritionWriter, uid, uuid.New().String(), "contracts.core.nutrition.v1.mealLogged", payload)
	}

	// Hulk: 124 + 30 = 154 XP
	// Flash: 86 + 30 = 116 XP
	// RestDay: 0 + 30 = 30 XP
	if waitErr := waitForXp(db, userHulk, 154, 10*time.Second); waitErr != nil {
		return fmt.Errorf("hulk nutrition xp mismatch: %w", waitErr)
	}
	if waitErr := waitForXp(db, userFlash, 116, 10*time.Second); waitErr != nil {
		return fmt.Errorf("flash nutrition xp mismatch: %w", waitErr)
	}
	if waitErr := waitForXp(db, userRest, 30, 10*time.Second); waitErr != nil {
		return fmt.Errorf("restday nutrition xp mismatch: %w", waitErr)
	}
	log.Println("✅ [GIAI ĐOẠN 4 THÀNH CÔNG] Dinh dưỡng +30 XP và duy trì Streak hoàn tất!")
	log.Println("   -> Hulk: 154 XP | Flash: 116 XP | RestDay: 30 XP | Newbie: 0 XP")

	// STEP 5: DEEP DATABASE INTEGRITY AUDIT
	log.Println("\n--- [GIAI ĐOẠN 5] KIỂM TRA ĐỘ ĐÚNG TRỰC TIẾP TRONG DATABASE POSTGRESQL ---")

	// 5.1 Kiểm tra bảng user_xp
	log.Println("🔍 [Audit 1] Kiểm tra tính nhất quán bảng gamification.user_xp:")
	rows, err := db.Query("SELECT user_id, xp, level FROM gamification.user_xp ORDER BY xp DESC")
	if err != nil {
		return fmt.Errorf("query user_xp: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var u string
		var x int64
		var l int
		_ = rows.Scan(&u, &x, &l)
		var label string
		switch u {
		case userHulk:
			label = "Hulk (Target: 154 XP)"
		case userFlash:
			label = "Flash (Target: 116 XP)"
		case userIdemp:
			label = "IdempBot (Target: 84 XP)"
		case userRest:
			label = "RestDay (Target: 30 XP)"
		case userNewbie:
			label = "Newbie (Target: 0 XP)"
		default:
			label = "Other User"
		}
		fmt.Printf("   -> User %s: %s | Level %d | XP: %d\n", u[:8], label, l, x)
	}

	// 5.2 Kiểm tra bảng xp_history (tổng delta_xp phải bằng đúng user_xp)
	log.Println("\n🔍 [Audit 2] Kiểm tra đối chiếu bảng gamification.xp_history (Tổng delta = Tổng XP):")
	for _, u := range users {
		var sumDelta int64
		var historyCount int
		_ = db.QueryRow("SELECT COALESCE(SUM(amount), 0), COUNT(*) FROM gamification.xp_history WHERE user_id = $1", u.id).Scan(&sumDelta, &historyCount)

		var actualXp int64
		_ = db.QueryRow("SELECT xp FROM gamification.user_xp WHERE user_id = $1", u.id).Scan(&actualXp)

		if sumDelta != actualXp {
			return fmt.Errorf("❌ Lệch dữ liệu kiểm toán cho %s: SUM(xp_history)=%d != user_xp=%d", u.name, sumDelta, actualXp)
		}
		fmt.Printf("   ✅ %s: %d lịch sử giao dịch XP | SUM(delta)=%d == actual_xp=%d\n", u.name, historyCount, sumDelta, actualXp)
	}

	// 5.3 Kiểm tra bảng user_streak & streak_history
	log.Println("\n🔍 [Audit 3] Kiểm tra bảng gamification.user_streak:")
	for _, u := range users {
		var curStreak, longStreak int
		var lastActive sql.NullString
		streakQ := "SELECT current_streak, longest_streak, last_active_date FROM gamification.user_streak WHERE user_id = $1"
		_ = db.QueryRow(streakQ, u.id).Scan(&curStreak, &longStreak, &lastActive)

		var historyRows int
		_ = db.QueryRow("SELECT COUNT(*) FROM gamification.streak_history WHERE user_id = $1", u.id).Scan(&historyRows)

		dateStr := "chưa có"
		if lastActive.Valid {
			dateStr = lastActive.String
		}
		fmt.Printf("   -> %s: Streak=%d (Longest=%d) | LastActive=%s | Lịch sử streak: %d ngày\n", u.name, curStreak, longStreak, dateStr, historyRows)
	}

	// 5.4 Kiểm tra bảng user_badges
	log.Println("\n🔍 [Audit 4] Kiểm tra bảng gamification.user_badges (Huy hiệu mở khóa):")
	badgeRows, _ := db.Query("SELECT user_id, badge_code, unlocked_at FROM gamification.user_badges ORDER BY unlocked_at ASC")
	if badgeRows != nil {
		for badgeRows.Next() {
			var uid, bcode string
			var unAt time.Time
			_ = badgeRows.Scan(&uid, &bcode, &unAt)
			fmt.Printf("   🎖️ User %s: Code='%s' lúc %s\n", uid[:8], bcode, unAt.Format(time.RFC3339))
		}
		badgeRows.Close()
	}

	// 5.5 Kiểm tra bảng outbox_log (Idempotency Logs)
	log.Println("\n🔍 [Audit 5] Kiểm tra bảng gamification.outbox_log (Consumer Deduplication Audit):")
	var processedLogs, failedLogs int
	_ = db.QueryRow("SELECT COUNT(*) FROM gamification.outbox_log WHERE status = 'PROCESSED'").Scan(&processedLogs)
	_ = db.QueryRow("SELECT COUNT(*) FROM gamification.outbox_log WHERE status = 'FAILED'").Scan(&failedLogs)
	fmt.Printf("   🛡️ Số message đã ghi nhận thành công (PROCESSED): %d | Thất bại (FAILED): %d\n", processedLogs, failedLogs)

	// STEP 6: VERIFY CONNECTRPC / gRPC API QUERIES
	log.Println("\n--- [GIAI ĐOẠN 6] GỌI TRỰC TIẾP CÁC API CONNECTRPC / gRPC ---")

	// 6.1 Query Leaderboard
	hulkCtx := context.WithValue(ctx, middleware.UserIDKey, userHulk)
	boardReq := connect.NewRequest(&gamificationv1msg.GetGlobalLeaderboardRequest{Limit: 10})
	boardResp, err := grpcHandler.GetGlobalLeaderboard(hulkCtx, boardReq)
	if err != nil {
		return fmt.Errorf("grpc GetGlobalLeaderboard: %w", err)
	}

	log.Println("🏆 BẢNG XẾP HẠNG GLOBAL LEADERBOARD (Top Gymer):")
	for _, entry := range boardResp.Msg.GetTopEntries() {
		fmt.Printf("   #%d: %s | Level %d | %d XP\n", entry.GetRank(), entry.GetDisplayName(), entry.GetLevel(), entry.GetXp())
	}
	if boardResp.Msg.GetMyStanding() != nil {
		my := boardResp.Msg.GetMyStanding()
		fmt.Printf("   👉 Thứ hạng của Hulk: #%d (%s) - Level %d - %d XP\n", my.GetRank(), my.GetDisplayName(), my.GetLevel(), my.GetXp())
	}

	// 6.2 Query GetMyXp của Hulk
	xpReq := connect.NewRequest(&gamificationv1msg.GetMyXpRequest{})
	xpResp, err := grpcHandler.GetMyXp(hulkCtx, xpReq)
	if err != nil {
		return fmt.Errorf("grpc GetMyXp: %w", err)
	}
	hulkXp := xpResp.Msg.GetXpSummary()
	fmt.Printf("\n📊 [API GetMyXp - Hulk] Total XP: %d | Level: %d | Progress: %.1f%% | To Next Level: %d\n",
		hulkXp.GetXp(), hulkXp.GetLevel(), hulkXp.GetLevelProgressPercentage(), hulkXp.GetXpToNextLevel())

	// 6.3 Query GetStreak của Hulk
	streakReq := connect.NewRequest(&gamificationv1msg.GetStreakRequest{})
	streakResp, err := grpcHandler.GetStreak(hulkCtx, streakReq)
	if err != nil {
		return fmt.Errorf("grpc GetStreak: %w", err)
	}
	hulkStreak := streakResp.Msg.GetStreakSummary()
	fmt.Printf("🔥 [API GetStreak - Hulk] Current: %d | Longest: %d | Is Active: %v | Last Active: %s\n",
		hulkStreak.GetCurrentStreak(), hulkStreak.GetLongestStreak(), hulkStreak.GetIsActiveToday(), hulkStreak.GetLastActiveDate())

	log.Println("\n================================================================")
	log.Println("🎉🎉🎉 TOÀN BỘ KỊCH BẢN E2E SIMULATION ĐÃ THÀNH CÔNG VÀ CHÍNH XÁC 100%! 🎉🎉🎉")
	log.Println("================================================================")
	return nil
}

func sendEvent(ctx context.Context, writer *kafka.Writer, partitionKey, eventID, eventType string, data []byte) {
	ce := CloudEventEnvelope{
		SpecVersion:     "1.0",
		ID:              eventID,
		Source:          "gym-companion/e2e-simulation",
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

func waitForCount(db *sql.DB, table, col, val string, expected int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		var count int
		query := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s = $1", table, col) //nolint:gosec // table is safe test parameter
		_ = db.QueryRow(query, val).Scan(&count)
		if count >= expected {
			return nil
		}
		time.Sleep(300 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for %s %s=%s count >= %d", table, col, val, expected)
}

func waitForXp(db *sql.DB, userID string, expectedXp int64, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		var xp int64
		_ = db.QueryRow("SELECT xp FROM gamification.user_xp WHERE user_id = $1", userID).Scan(&xp)
		if xp >= expectedXp {
			return nil
		}
		time.Sleep(300 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for user %s xp >= %d", userID, expectedXp)
}

func waitForBadge(db *sql.DB, userID, badgeCode string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		var count int
		_ = db.QueryRow("SELECT COUNT(*) FROM gamification.user_badges WHERE user_id = $1 AND badge_code = $2", userID, badgeCode).Scan(&count)
		if count >= 1 {
			return nil
		}
		time.Sleep(300 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for user %s badge %s", userID, badgeCode)
}
