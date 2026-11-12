package seeder

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"time"

	socialv1message "github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/social/v1/message"
	"github.com/viethung213/gym-companion/internal/shared/middleware"
)

// SocialAPI defines the API contract required to perform social actions.
// Implemented directly by socialv1serviceconnect.SocialServiceHandler and *socialGRPC.GRPCHandler.
type SocialAPI interface {
	FollowUser(ctx context.Context, req *socialv1message.FollowUserRequest) (*socialv1message.FollowUserResponse, error)
	CreatePost(ctx context.Context, req *socialv1message.CreatePostRequest) (*socialv1message.CreatePostResponse, error)
	ReactTarget(ctx context.Context, req *socialv1message.ReactTargetRequest) (*socialv1message.ReactTargetResponse, error)
	AddComment(ctx context.Context, req *socialv1message.AddCommentRequest) (*socialv1message.AddCommentResponse, error)
}

func defaultPostCaptions() []string {
	return []string{
		"Phá kỷ lục cá nhân hôm nay! 5 hiệp Squat 150kg mượt mà 🔥💪",
		"Đổ mồ hôi trên sàn tập để cơ bắp phát triển từng ngày 🎯",
		"Buổi sáng cardio 45 phút đốt mỡ, khởi động tuần mới tràn đầy năng lượng!",
		"Giáo án tuần 4 hoàn thành xuất sắc! Cảm giác pump cơ cực tốt 🚀",
		"Deadlift 180kg nhẹ nhàng sau 4 tuần rèn luyện. Form chuẩn chỉ ⚡",
		"Ngày tập ngực bùng nổ cùng giáo án Hypertrophy. Không có gì ngăn cản được!",
		"Chiến thắng lớn nhất mỗi sáng là thức dậy và bước chân vào phòng gym.",
		"Tập lưng xô kết hợp cẳng tay. Từng rep đều siết chặt cơ!",
		"Kỷ luật là cầu nối giữa mục tiêu và thành tựu. Cứ kiên trì từng ngày anh em nhé!",
		"Bữa ăn phục hồi sau tập: 200g ức gà + 1 củ khoai lang + súp lơ luộc. Đơn giản mà đủ chất 🥦🍗",
		"Đừng so sánh sự khởi đầu của bạn với đỉnh cao của người khác. Mỗi ngày tốt hơn 1% là đủ!",
		"Uống đủ 2.5 - 3 lít nước mỗi ngày giúp cơ bắp tổng hợp protein tốt hơn và giảm chuột rút.",
		"Nghỉ ngơi và giấc ngủ cũng quan trọng không kém việc nâng tạ. Ngủ đủ 7-8 tiếng nhé anh em! 😴",
		"10 cái chống đẩy mỗi sáng khi mới ngủ dậy cũng là một bước tiến vượt bậc.",
		"Tập luyện vì sức khỏe và sự tự tin của chính mình, không phải để làm hài lòng người khác.",
		"Bí quyết để duy trì streak tập luyện: Biến việc tập gym thành thói quen như đánh răng mỗi ngày.",
	}
}

func defaultCommentTemplates() []string {
	return []string{
		"Form quá chuẩn luôn bạn ơi! 🔥",
		"Động lực tập luyện bùng nổ! Chúc mừng PR mới 👏",
		"Xin giáo án tập buổi này với ạ!",
		"Quá khủng khiếp, tiếp tục phát huy nhé 💪",
		"Tập chăm chỉ thế này cơ lên nhanh lắm đấy!",
		"Nhìn tạ uy tín quá anh em ơi! 🚀",
		"Cố lên, mục tiêu cuối năm sắp đạt được rồi!",
	}
}

func defaultReactionTypes() []socialv1message.ReactionType {
	return []socialv1message.ReactionType{
		socialv1message.ReactionType_REACTION_TYPE_FIRE,
		socialv1message.ReactionType_REACTION_TYPE_MUSCLE,
		socialv1message.ReactionType_REACTION_TYPE_CLAP,
	}
}

// SeedBotSocialActivities seeds realistic social graph, posts, and interactions for bot accounts.
// It executes exclusively through the API layer (*socialGRPC.GRPCHandler), exactly like real users calling the API.
func SeedBotSocialActivities(ctx context.Context, db *sql.DB, socialAPI SocialAPI) error {
	if socialAPI == nil {
		return errors.New("social API cannot be nil")
	}
	if db == nil {
		return errors.New("database connection cannot be nil")
	}

	// 1. Wait briefly for system bots to be created by BotSeeder if running concurrently
	botUserIDs, err := waitForBotUserIDs(ctx, db, 15*time.Second)
	if err != nil {
		return fmt.Errorf("lookup bot user IDs: %w", err)
	}
	if len(botUserIDs) < 2 {
		log.Printf("⚠️ Fewer than 2 bots found (%d). Skipping social bot seeding.", len(botUserIDs))
		return nil
	}

	// 2. Idempotency Check: If bots already have at least 200 feed items, skip
	placeholders := make([]string, len(botUserIDs))
	args := make([]interface{}, len(botUserIDs))
	for i, id := range botUserIDs {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}
	query := fmt.Sprintf(`
		SELECT COUNT(*) 
		FROM social.feed_items 
		WHERE user_id IN (%s)
	`, strings.Join(placeholders, ", "))

	var existingCount int
	err = db.QueryRowContext(ctx, query, args...).Scan(&existingCount)
	if err == nil && existingCount >= 200 {
		log.Printf("🤖 Bot social activities already seeded (%d items exist). Skipping social seeder.", existingCount)
		return nil
	}

	log.Printf("🌱 Starting Application-driven Social Activity Seeding for %d bots...", len(botUserIDs))
	r := rand.New(rand.NewSource(42)) // Deterministic seed for reproducible testing

	postCaptions := defaultPostCaptions()
	commentTemplates := defaultCommentTemplates()
	reactionTypes := defaultReactionTypes()

	// 3. Establish mutual follows between bots (each bot follows 3 to 6 other bots)
	for _, followerID := range botUserIDs {
		followCount := 3 + r.Intn(4)
		for j := 0; j < followCount; j++ {
			targetIdx := r.Intn(len(botUserIDs))
			targetID := botUserIDs[targetIdx]
			if targetID == followerID {
				continue
			}
			botCtx := middleware.WithActor(ctx, followerID, "USER")
			_, _ = socialAPI.FollowUser(botCtx, &socialv1message.FollowUserRequest{
				FollowingId: targetID,
			})
		}
	}
	log.Printf("  ✅ Bot social graph (follows) connected successfully.")

	// 4. Create ~50-60 posts per bot (approx 500-600 total items)
	// Seeded through the standard CreatePost API (identical to real user API)
	type createdItemRef struct {
		id     string
		userID string
	}
	var createdItems []createdItemRef

	for botIdx, botID := range botUserIDs {
		postsForThisBot := 50 + r.Intn(10) // 50 to 59 items per bot
		botCtx := middleware.WithActor(ctx, botID, "USER")
		for i := 0; i < postsForThisBot; i++ {
			captionIdx := (botIdx*3 + i) % len(postCaptions)
			res, postErr := socialAPI.CreatePost(botCtx, &socialv1message.CreatePostRequest{
				Caption:    postCaptions[captionIdx],
				MediaUrls:  []string{},
				Visibility: "PUBLIC",
			})
			if postErr == nil && res != nil && res.GetItem() != nil {
				createdItems = append(createdItems, createdItemRef{id: res.GetItem().GetId(), userID: botID})
			}
		}
	}
	log.Printf("  ✅ Created %d feed items across %d bots.", len(createdItems), len(botUserIDs))

	// 5. Seed Interactions (Reactions & Comments) on selected posts
	// To generate realistic Trending Pool candidates (reaction*2 + comment*5)
	reactionCount := 0
	commentCount := 0
	for _, it := range createdItems {
		// ~60% of items receive interactions
		if r.Float64() < 0.60 {
			likersCount := 1 + r.Intn(6) // 1 to 6 likers
			for l := 0; l < likersCount; l++ {
				likerID := botUserIDs[r.Intn(len(botUserIDs))]
				if likerID == it.userID {
					continue
				}
				likerCtx := middleware.WithActor(ctx, likerID, "USER")
				rxType := reactionTypes[r.Intn(len(reactionTypes))]
				_, _ = socialAPI.ReactTarget(likerCtx, &socialv1message.ReactTargetRequest{
					FeedItemId:   it.id,
					ReactionType: rxType,
				})
				reactionCount++
			}

			// ~30% of items receive comments
			if r.Float64() < 0.35 {
				commenterCount := 1 + r.Intn(3)
				for c := 0; c < commenterCount; c++ {
					commenterID := botUserIDs[r.Intn(len(botUserIDs))]
					commenterCtx := middleware.WithActor(ctx, commenterID, "USER")
					cText := commentTemplates[r.Intn(len(commentTemplates))]
					_, _ = socialAPI.AddComment(commenterCtx, &socialv1message.AddCommentRequest{
						FeedItemId: it.id,
						Content:    cText,
					})
					commentCount++
				}
			}
		}
	}

	log.Printf("🎉 Successfully seeded social feed: %d posts, %d reactions, %d comments.",
		len(createdItems), reactionCount, commentCount)
	return nil
}

func waitForBotUserIDs(ctx context.Context, db *sql.DB, timeout time.Duration) ([]string, error) {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()

	for {
		rows, err := db.QueryContext(ctx, `
			SELECT user_id 
			FROM auth.user_identities 
			WHERE identifier LIKE 'bot.%@fitai.bot'
		`)
		if err == nil {
			var ids []string
			for rows.Next() {
				var id string
				if err := rows.Scan(&id); err == nil {
					ids = append(ids, id)
				}
			}
			_ = rows.Close()
			if len(ids) >= 5 {
				return ids, nil
			}
		}

		select {
		case <-ctx.Done():
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				var ids []string
				rows, qErr := db.QueryContext(context.Background(), `SELECT user_id FROM auth.user_identities WHERE identifier LIKE 'bot.%@fitai.bot'`)
				if qErr == nil {
					for rows.Next() {
						var id string
						if scanErr := rows.Scan(&id); scanErr == nil {
							ids = append(ids, id)
						}
					}
					_ = rows.Close()
				}
				return ids, nil
			}
			return nil, ctx.Err()
		case <-ticker.C:
			if time.Now().After(deadline) {
				// Return whatever we have if timeout reached
				var ids []string
				rows, qErr := db.QueryContext(ctx, `SELECT user_id FROM auth.user_identities WHERE identifier LIKE 'bot.%@fitai.bot'`)
				if qErr == nil {
					for rows.Next() {
						var id string
						if scanErr := rows.Scan(&id); scanErr == nil {
							ids = append(ids, id)
						}
					}
					_ = rows.Close()
				}
				return ids, nil
			}
		}
	}
}
