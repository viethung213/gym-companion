package seeder_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	socialv1message "github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/social/v1/message"
	"github.com/viethung213/gym-companion/internal/shared/seeder"
	"gorm.io/gorm"
)

type mockSocialAPI struct {
	createdPosts []*socialv1message.CreatePostRequest
	reactions    []*socialv1message.ReactTargetRequest
	comments     []*socialv1message.AddCommentRequest
	follows      []*socialv1message.FollowUserRequest
}

func (m *mockSocialAPI) FollowUser(ctx context.Context, req *socialv1message.FollowUserRequest) (*socialv1message.FollowUserResponse, error) {
	m.follows = append(m.follows, req)
	return &socialv1message.FollowUserResponse{Success: true}, nil
}

func (m *mockSocialAPI) CreatePost(ctx context.Context, req *socialv1message.CreatePostRequest) (*socialv1message.CreatePostResponse, error) {
	m.createdPosts = append(m.createdPosts, req)
	return &socialv1message.CreatePostResponse{
		Item: &socialv1message.FeedItem{
			Id: uuid.New().String(),
		},
	}, nil
}

func (m *mockSocialAPI) ReactTarget(ctx context.Context, req *socialv1message.ReactTargetRequest) (*socialv1message.ReactTargetResponse, error) {
	m.reactions = append(m.reactions, req)
	return &socialv1message.ReactTargetResponse{Success: true}, nil
}

func (m *mockSocialAPI) AddComment(ctx context.Context, req *socialv1message.AddCommentRequest) (*socialv1message.AddCommentResponse, error) {
	m.comments = append(m.comments, req)
	return &socialv1message.AddCommentResponse{
		Comment: &socialv1message.CommentItem{Id: uuid.New().String()},
	}, nil
}

func setupSocialTestDB(t *testing.T) (*sql.DB, func()) {
	t.Helper()
	dbName := fmt.Sprintf("file:social_seeder_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	gormDB, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{
		SkipDefaultTransaction: true,
	})
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}

	db, err := gormDB.DB()
	if err != nil {
		t.Fatalf("failed to get sql.DB: %v", err)
	}
	db.SetMaxOpenConns(1)

	initSQL := `
		ATTACH DATABASE ':memory:' AS auth;
		ATTACH DATABASE ':memory:' AS social;

		CREATE TABLE IF NOT EXISTS auth.user_identities (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			identity_type TEXT NOT NULL,
			identifier TEXT NOT NULL
		);

		CREATE TABLE IF NOT EXISTS social.feed_items (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			data TEXT
		);
	`
	if _, err := db.Exec(initSQL); err != nil {
		t.Fatalf("failed to init test tables: %v", err)
	}

	cleanup := func() {
		_ = db.Close()
	}
	return db, cleanup
}

func TestSeedBotSocialActivities_Success(t *testing.T) {
	db, cleanup := setupSocialTestDB(t)
	defer cleanup()

	// Insert 5 test bot user identities
	for i := 1; i <= 5; i++ {
		_, err := db.Exec(`
			INSERT INTO auth.user_identities (id, user_id, identity_type, identifier)
			VALUES ($1, $2, 'email', $3)
		`, fmt.Sprintf("ident-%d", i), fmt.Sprintf("bot-uid-%d", i), fmt.Sprintf("bot.%d@fitai.bot", i))
		if err != nil {
			t.Fatalf("failed to insert mock bot identity: %v", err)
		}
	}

	api := &mockSocialAPI{}
	ctx := context.Background()

	err := seeder.SeedBotSocialActivities(ctx, db, api)
	if err != nil {
		t.Fatalf("unexpected error during seeding: %v", err)
	}

	if len(api.follows) == 0 {
		t.Errorf("expected bot follows to be seeded, got 0")
	}

	totalItems := len(api.createdPosts)
	if totalItems < 200 {
		t.Errorf("expected at least 200 items across 5 bots, got %d", totalItems)
	}

	// Verify idempotency: running again should detect existing count if count >= 200
	for i := range api.createdPosts {
		_, _ = db.Exec(`INSERT INTO social.feed_items (id, user_id) VALUES ($1, $2)`, fmt.Sprintf("p-%d", i), fmt.Sprintf("bot-uid-%d", (i%5)+1))
	}

	apiSecondRun := &mockSocialAPI{}
	err = seeder.SeedBotSocialActivities(ctx, db, apiSecondRun)
	if err != nil {
		t.Fatalf("second run failed: %v", err)
	}
	if len(apiSecondRun.createdPosts) != 0 {
		t.Errorf("expected second run to be skipped due to idempotency, but created items")
	}
}

func TestSeedBotSocialActivities_Validation(t *testing.T) {
	ctx := context.Background()

	// Nil API
	if err := seeder.SeedBotSocialActivities(ctx, nil, nil); !errors.Is(err, errors.New("social API cannot be nil")) && err == nil {
		t.Errorf("expected error for nil social API, got nil")
	}

	// Nil DB
	api := &mockSocialAPI{}
	if err := seeder.SeedBotSocialActivities(ctx, nil, api); !errors.Is(err, errors.New("database connection cannot be nil")) && err == nil {
		t.Errorf("expected error for nil db, got nil")
	}
}

func TestSeedBotSocialActivities_FewerThanTwoBots(t *testing.T) {
	db, cleanup := setupSocialTestDB(t)
	defer cleanup()

	// Insert only 1 bot
	_, err := db.Exec(`
		INSERT INTO auth.user_identities (id, user_id, identity_type, identifier)
		VALUES ('ident-1', 'bot-uid-1', 'email', 'bot.single@fitai.bot')
	`)
	if err != nil {
		t.Fatalf("failed to insert single bot: %v", err)
	}

	api := &mockSocialAPI{}
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	err = seeder.SeedBotSocialActivities(ctx, db, api)
	if err != nil {
		t.Fatalf("expected graceful skip with nil error for <2 bots, got: %v", err)
	}
	if len(api.createdPosts) > 0 {
		t.Errorf("expected no activities created, got %d posts", len(api.createdPosts))
	}
}

func TestSeedBotSocialActivities_ContextCancelled(t *testing.T) {
	db, cleanup := setupSocialTestDB(t)
	defer cleanup()

	api := &mockSocialAPI{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := seeder.SeedBotSocialActivities(ctx, db, api)
	if err == nil {
		t.Errorf("expected context cancellation error, got nil")
	}
}
