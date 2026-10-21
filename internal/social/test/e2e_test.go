// Package test chứa E2E test cho module Social.
// E2E test kiểm tra toàn bộ luồng hoàn chỉnh từ ConnectRPC / gRPC transport layer,
// đi qua Application Commands / Queries, Domain Aggregates & Entities,
// xuống tầng Infrastructure GORM Persistence với database SQLite in-memory thực tế.
package test

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"
	socialv1message "github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/social/v1/message"
	"github.com/viethung213/gym-companion/internal/shared/middleware"
	"github.com/viethung213/gym-companion/internal/social/application/command"
	"github.com/viethung213/gym-companion/internal/social/application/query"
	"github.com/viethung213/gym-companion/internal/social/test/testutil"
	socialGRPC "github.com/viethung213/gym-companion/internal/social/transport/grpc"
	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
)

type e2eFixture struct {
	handler   *socialGRPC.GRPCHandler
	repos     *testutil.Repositories
	publisher *testutil.MockEventPublisher
	syncUser  *command.SyncUserSnapshotHandler
	ingestAct *command.IngestWorkoutActivityHandler
}

func newE2EFixture(t *testing.T) *e2eFixture {
	t.Helper()

	_, repos := testutil.NewTestDB(t)
	pub := &testutil.MockEventPublisher{}

	followUserHandler := command.NewFollowUserHandler(repos.FollowRepo, repos.UserSnapshotRepo, pub, repos.TxManager)
	unfollowUserHandler := command.NewUnfollowUserHandler(repos.FollowRepo, repos.TxManager)
	createPostHandler := command.NewCreatePostHandler(repos.FeedItemRepo, pub, repos.TxManager)
	deleteFeedItemHandler := command.NewDeleteFeedItemHandler(repos.FeedItemRepo, repos.TxManager)
	reactTargetHandler := command.NewReactTargetHandler(repos.InteractionRepo, repos.FeedItemRepo, repos.UserSnapshotRepo, pub, repos.TxManager)
	addCommentHandler := command.NewAddCommentHandler(repos.InteractionRepo, repos.FeedItemRepo, repos.UserSnapshotRepo, pub, repos.TxManager)
	deleteCommentHandler := command.NewDeleteCommentHandler(repos.InteractionRepo, repos.FeedItemRepo, repos.TxManager)
	ingestWorkoutHandler := command.NewIngestWorkoutActivityHandler(repos.FeedItemRepo, repos.TxManager)
	syncUserHandler := command.NewSyncUserSnapshotHandler(repos.UserSnapshotRepo, repos.TxManager)

	getActivityFeedHandler := query.NewGetActivityFeedHandler(repos.FollowRepo, repos.FeedItemRepo, repos.InteractionRepo, repos.UserSnapshotRepo)
	getUserProfileFeedHandler := query.NewGetUserProfileFeedHandler(repos.FeedItemRepo, repos.UserSnapshotRepo)
	getSocialSummaryHandler := query.NewGetSocialSummaryHandler(repos.FollowRepo, repos.FeedItemRepo)
	getFollowersHandler := query.NewGetFollowersHandler(repos.FollowRepo, repos.UserSnapshotRepo)
	getFollowingHandler := query.NewGetFollowingHandler(repos.FollowRepo, repos.UserSnapshotRepo)
	listCommentsHandler := query.NewListCommentsHandler(repos.InteractionRepo, repos.UserSnapshotRepo)
	listReactionsHandler := query.NewListReactionsHandler(repos.InteractionRepo, repos.UserSnapshotRepo)

	searchUsersHandler := query.NewSearchUsersHandler(repos.UserSnapshotRepo, repos.FollowRepo)
	searchPostsHandler := query.NewSearchPostsHandler(repos.FeedItemRepo, repos.InteractionRepo, repos.UserSnapshotRepo)

	h := socialGRPC.NewGRPCHandler(
		followUserHandler,
		unfollowUserHandler,
		createPostHandler,
		deleteFeedItemHandler,
		reactTargetHandler,
		addCommentHandler,
		deleteCommentHandler,
		getActivityFeedHandler,
		getUserProfileFeedHandler,
		getSocialSummaryHandler,
		getFollowersHandler,
		getFollowingHandler,
		listCommentsHandler,
		listReactionsHandler,
		searchUsersHandler,
		searchPostsHandler,
	)

	return &e2eFixture{
		handler:   h,
		repos:     repos,
		publisher: pub,
		syncUser:  syncUserHandler,
		ingestAct: ingestWorkoutHandler,
	}
}

func authContext(userID string) context.Context {
	ctx := context.WithValue(context.Background(), middleware.UserIDKey, userID)
	return context.WithValue(ctx, middleware.UserRoleKey, "USER")
}

// TestE2E_FullSocialJourney kiểm thử hành trình mạng xã hội thể hình hoàn chỉnh:
// 1. Đồng bộ snapshot định danh của 2 gymer: User Alice (tác giả) & User Bob (người theo dõi).
// 2. Bob follow Alice -> kiểm tra quan hệ trong DB và danh sách followers/following.
// 3. Alice tạo bài viết chia sẻ ảnh form tập -> kiểm tra FeedItem trong DB.
// 4. Hệ thống nạp hoạt động tập luyện (Workout Activity) được chia sẻ từ Workout Execution module.
// 5. Bob tương tác thả cảm xúc FIRE lên bài viết -> đếm phản ứng tăng lên 1, query danh sách người thả cảm xúc.
// 6. Bob bấm lại cảm xúc FIRE -> cơ chế Toggle hoạt động, phản ứng giảm về 0.
// 7. Bob thêm bình luận và reply bình luận -> đếm bình luận tăng, lấy danh sách phân trang cursor.
// 8. Bob kiểm tra bảng tin Newfeed cá nhân hóa -> hiển thị đầy đủ bài tập và bài viết của Alice kèm Avatar/Tên.
// 9. Kiểm tra trang tổng quan thể hình (Social Summary) của Alice.
// 10. Kiểm tra phân quyền bảo mật: Bob không thể xóa bài của Alice; Alice xóa thành công bài của chính mình.
func TestE2E_FullSocialJourney(t *testing.T) {
	f := newE2EFixture(t)
	aliceCtx := authContext("user-alice")
	bobCtx := authContext("user-bob")

	// ── Step 1: Sync User Snapshots (mô phỏng nhận event từ Auth & Profile) ──
	err := f.syncUser.Handle(context.Background(), command.SyncUserSnapshotCommand{
		UserID:    "user-alice",
		FullName:  "Alice Wonder",
		AvatarURL: "https://gym.com/avatar/alice.png",
	})
	if err != nil {
		t.Fatalf("[Step 1] Sync Alice Snapshot failed: %v", err)
	}

	err = f.syncUser.Handle(context.Background(), command.SyncUserSnapshotCommand{
		UserID:    "user-bob",
		FullName:  "Bob Builder",
		AvatarURL: "https://gym.com/avatar/bob.png",
	})
	if err != nil {
		t.Fatalf("[Step 1] Sync Bob Snapshot failed: %v", err)
	}

	// ── Step 2: Bob follows Alice ───────────────────────────────────────────
	followRes, err := f.handler.FollowUser(bobCtx, connect.NewRequest(&socialv1message.FollowUserRequest{
		FollowingId: "user-alice",
	}))
	if err != nil {
		t.Fatalf("[Step 2] FollowUser failed: %v", err)
	}
	if !followRes.Msg.GetSuccess() {
		t.Fatal("[Step 2] expected follow success true")
	}

	// Kiểm tra danh sách Followers của Alice
	followersRes, err := f.handler.GetFollowers(bobCtx, connect.NewRequest(&socialv1message.GetFollowersRequest{
		UserId:   "user-alice",
		PageSize: 10,
	}))
	if err != nil {
		t.Fatalf("[Step 2] GetFollowers failed: %v", err)
	}
	if len(followersRes.Msg.GetFollowers()) != 1 || followersRes.Msg.GetFollowers()[0].GetUserId() != "user-bob" {
		t.Fatalf("[Step 2] expected Bob in Alice followers, got %+v", followersRes.Msg.GetFollowers())
	}
	if followersRes.Msg.GetFollowers()[0].GetFullName() != "Bob Builder" {
		t.Errorf("[Step 2] expected Bob Builder, got %s", followersRes.Msg.GetFollowers()[0].GetFullName())
	}

	// Kiểm tra danh sách Following của Bob
	followingRes, err := f.handler.GetFollowing(bobCtx, connect.NewRequest(&socialv1message.GetFollowingRequest{
		UserId:   "user-bob",
		PageSize: 10,
	}))
	if err != nil {
		t.Fatalf("[Step 2] GetFollowing failed: %v", err)
	}
	if len(followingRes.Msg.GetFollowing()) != 1 || followingRes.Msg.GetFollowing()[0].GetUserId() != "user-alice" {
		t.Fatalf("[Step 2] expected Alice in Bob following, got %+v", followingRes.Msg.GetFollowing())
	}

	// ── Step 3: Alice tạo bài viết (CreatePost) ─────────────────────────────
	postRes, err := f.handler.CreatePost(aliceCtx, connect.NewRequest(&socialv1message.CreatePostRequest{
		Caption:    "Leg Day squats - 120kg PR!",
		MediaUrls:  []string{"https://gym.com/squat.jpg"},
		Visibility: "PUBLIC",
	}))
	if err != nil {
		t.Fatalf("[Step 3] CreatePost failed: %v", err)
	}
	postID := postRes.Msg.GetItem().GetId()
	if postID == "" {
		t.Fatal("[Step 3] expected non-empty post ID")
	}

	// ── Step 4: Nạp buổi tập chia sẻ (Ingest Workout Activity) ──────────────
	err = f.ingestAct.Handle(context.Background(), command.IngestWorkoutActivityCommand{
		SessionID:       "sess-123",
		UserID:          "user-alice",
		Title:           "Push Day Heavy",
		Caption:         "Chest pump was insane!",
		MediaURLs:       []string{"https://gym.com/bench.jpg"},
		DurationSeconds: 4200,
		TotalVolumeKg:   5400.0,
		TotalSets:       16,
		SharedAt:        time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("[Step 4] Ingest Workout Activity failed: %v", err)
	}

	// ── Step 5: Bob thả cảm xúc FIRE lên bài viết của Alice ──────────────────
	reactRes1, err := f.handler.ReactTarget(bobCtx, connect.NewRequest(&socialv1message.ReactTargetRequest{
		FeedItemId:   postID,
		ReactionType: socialv1message.ReactionType_REACTION_TYPE_FIRE,
	}))
	if err != nil {
		t.Fatalf("[Step 5] ReactTarget failed: %v", err)
	}
	if reactRes1.Msg.GetCurrentReaction() != "FIRE" {
		t.Errorf("[Step 5] expected reaction 'FIRE', got %v", reactRes1.Msg.GetCurrentReaction())
	}

	// Kiểm tra API ListReactions
	listReactionsRes, err := f.handler.ListReactions(bobCtx, connect.NewRequest(&socialv1message.ListReactionsRequest{
		FeedItemId: postID,
	}))
	if err != nil {
		t.Fatalf("[Step 5] ListReactions failed: %v", err)
	}
	if listReactionsRes.Msg.GetTotalCount() != 1 || len(listReactionsRes.Msg.GetReactions()) != 1 {
		t.Fatalf("[Step 5] expected 1 reaction, got total=%d", listReactionsRes.Msg.GetTotalCount())
	}
	if listReactionsRes.Msg.GetReactions()[0].GetAuthorName() != "Bob Builder" {
		t.Errorf("[Step 5] expected Bob Builder as reactor, got %s", listReactionsRes.Msg.GetReactions()[0].GetAuthorName())
	}

	// ── Step 6: Bob bấm lại cùng cảm xúc FIRE (Toggle OFF) ──────────────────
	reactRes2, err := f.handler.ReactTarget(bobCtx, connect.NewRequest(&socialv1message.ReactTargetRequest{
		FeedItemId:   postID,
		ReactionType: socialv1message.ReactionType_REACTION_TYPE_FIRE,
	}))
	if err != nil {
		t.Fatalf("[Step 6] ReactTarget Toggle failed: %v", err)
	}
	if reactRes2.Msg.GetCurrentReaction() != "" {
		t.Errorf("[Step 6] expected reaction cleared, got %v", reactRes2.Msg.GetCurrentReaction())
	}

	// ── Step 7: Bob thêm bình luận và reply ─────────────────────────────────
	commentRes1, err := f.handler.AddComment(bobCtx, connect.NewRequest(&socialv1message.AddCommentRequest{
		FeedItemId: postID,
		Content:    "Awesome form Alice, keep it up!",
	}))
	if err != nil {
		t.Fatalf("[Step 7] AddComment 1 failed: %v", err)
	}
	parentCommentID := commentRes1.Msg.GetComment().GetId()

	// Alice phản hồi lại bình luận của Bob (Threaded Reply)
	_, err = f.handler.AddComment(aliceCtx, connect.NewRequest(&socialv1message.AddCommentRequest{
		FeedItemId: postID,
		ParentId:   &parentCommentID,
		Content:    "Thanks Bob! Next week going for 130kg.",
	}))
	if err != nil {
		t.Fatalf("[Step 7] AddComment 2 (Reply) failed: %v", err)
	}

	// List comments kiểm tra cả 2 bình luận
	commentsRes, err := f.handler.ListComments(bobCtx, connect.NewRequest(&socialv1message.ListCommentsRequest{
		FeedItemId: postID,
		PageSize:   10,
	}))
	if err != nil {
		t.Fatalf("[Step 7] ListComments failed: %v", err)
	}
	if len(commentsRes.Msg.GetComments()) != 2 {
		t.Fatalf("[Step 7] expected 2 comments, got %d", len(commentsRes.Msg.GetComments()))
	}

	// ── Step 8: Bob xem Newfeed tổng hợp (Activity Feed) ─────────────────────
	feedRes, err := f.handler.GetActivityFeed(bobCtx, connect.NewRequest(&socialv1message.GetActivityFeedRequest{
		PageSize: 10,
	}))
	if err != nil {
		t.Fatalf("[Step 8] GetActivityFeed failed: %v", err)
	}
	// Alice có 2 bài: 1 post + 1 workout activity
	if len(feedRes.Msg.GetItems()) != 2 {
		t.Fatalf("[Step 8] expected 2 items in Bob feed, got %d", len(feedRes.Msg.GetItems()))
	}
	for _, item := range feedRes.Msg.GetItems() {
		if item.GetAuthorName() != "Alice Wonder" {
			t.Errorf("[Step 8] expected author 'Alice Wonder', got '%s'", item.GetAuthorName())
		}
	}

	// ── Step 9: Kiểm tra thống kê thể hình (Social Summary) của Alice ────────
	summaryRes, err := f.handler.GetSocialSummary(bobCtx, connect.NewRequest(&socialv1message.GetSocialSummaryRequest{
		UserId: "user-alice",
	}))
	if err != nil {
		t.Fatalf("[Step 9] GetSocialSummary failed: %v", err)
	}
	if summaryRes.Msg.GetFollowerCount() != 1 {
		t.Errorf("[Step 9] expected 1 follower, got %d", summaryRes.Msg.GetFollowerCount())
	}
	if summaryRes.Msg.GetPostCount() != 2 {
		t.Errorf("[Step 9] expected 2 posts (1 post + 1 workout), got %d", summaryRes.Msg.GetPostCount())
	}
	if !summaryRes.Msg.GetIsFollowing() {
		t.Errorf("[Step 9] expected IsFollowing to be true for Bob viewing Alice")
	}

	// ── Step 10: Bảo mật & Xóa bài viết ────────────────────────────────────
	// Bob cố tình xóa bình luận của Alice -> Bị chặn Unauthorized / Permission Denied
	_, err = f.handler.DeleteComment(bobCtx, connect.NewRequest(&socialv1message.DeleteCommentRequest{
		CommentId: commentRes1.Msg.GetComment().GetId(),
	}))
	// Bob is the author of comment 1, so Bob CAN delete comment 1
	// Let's test Bob trying to delete Alice's post
	_, err = f.handler.DeleteFeedItem(bobCtx, connect.NewRequest(&socialv1message.DeleteFeedItemRequest{
		FeedItemId: postID,
	}))
	if err == nil {
		t.Fatal("[Step 10] expected Bob deleting Alice post to be rejected, got nil error")
	}

	// Alice xóa bài viết của chính mình -> Thành công
	deleteRes, err := f.handler.DeleteFeedItem(aliceCtx, connect.NewRequest(&socialv1message.DeleteFeedItemRequest{
		FeedItemId: postID,
	}))
	if err != nil {
		t.Fatalf("[Step 10] Alice DeleteFeedItem failed: %v", err)
	}
	if !deleteRes.Msg.GetSuccess() {
		t.Fatal("[Step 10] expected success true")
	}

	// Feed cá nhân của Alice giờ chỉ còn 1 bài (workout activity)
	aliceProfileFeed, err := f.handler.GetUserProfileFeed(aliceCtx, connect.NewRequest(&socialv1message.GetUserProfileFeedRequest{
		UserId:   "user-alice",
		PageSize: 10,
	}))
	if err != nil {
		t.Fatalf("[Step 10] GetUserProfileFeed failed: %v", err)
	}
	if len(aliceProfileFeed.Msg.GetItems()) != 1 {
		t.Fatalf("[Step 10] expected 1 remaining item after deletion, got %d", len(aliceProfileFeed.Msg.GetItems()))
	}

	// Bob unfollow Alice
	unfollowRes, err := f.handler.UnfollowUser(bobCtx, connect.NewRequest(&socialv1message.UnfollowUserRequest{
		FollowingId: "user-alice",
	}))
	if err != nil {
		t.Fatalf("[Step 10] UnfollowUser failed: %v", err)
	}
	if !unfollowRes.Msg.GetSuccess() {
		t.Errorf("[Step 10] expected unfollow success true")
	}
}

// TestE2E_ValidationAndSecurityRules kiểm tra các quy tắc nghiệp vụ biên & bảo mật:
// 1. Chặn người dùng tự theo dõi chính mình (ErrSelfFollow).
// 2. Chặn tương tác khi chưa xác thực danh tính.
// 3. Chặn tạo bài viết hoàn toàn rỗng không có caption lẫn hình ảnh.
func TestE2E_ValidationAndSecurityRules(t *testing.T) {
	f := newE2EFixture(t)
	unauthCtx := context.Background() // Không có UserID trong context
	aliceCtx := authContext("user-alice")

	// 1. Chặn tự follow chính mình
	_, err := f.handler.FollowUser(aliceCtx, connect.NewRequest(&socialv1message.FollowUserRequest{
		FollowingId: "user-alice",
	}))
	if err == nil {
		t.Fatal("expected self-follow to be rejected, got nil error")
	}
	if connect.CodeOf(err) != connect.CodeInvalidArgument && grpcstatus.Code(err) != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument code, got %v", err)
	}

	// 2. Chặn tạo bài viết khi chưa đăng nhập
	_, err = f.handler.CreatePost(unauthCtx, connect.NewRequest(&socialv1message.CreatePostRequest{
		Caption: "Unauthenticated post",
	}))
	if err == nil {
		t.Fatal("expected unauthenticated create post to be rejected, got nil error")
	}
	if connect.CodeOf(err) != connect.CodeUnauthenticated && grpcstatus.Code(err) != codes.Unauthenticated {
		t.Errorf("expected Unauthenticated code, got %v", err)
	}

	// 3. Chặn tạo bài viết rỗng cả caption và media
	_, err = f.handler.CreatePost(aliceCtx, connect.NewRequest(&socialv1message.CreatePostRequest{
		Caption:   "   ",
		MediaUrls: []string{},
	}))
	if err == nil {
		t.Fatal("expected empty post to be rejected, got nil error")
	}
}

// TestE2E_SearchUsers_FilteringAndFollowStatus kiểm tra toàn diện API SearchUsers:
// 1. Đồng bộ người dùng với các role: "user", "brand", "admin".
// 2. Chặn hoàn toàn tài khoản role "admin" không xuất hiện trong kết quả tìm kiếm.
// 3. Lọc theo role ("user", "brand") và chặn client truy vấn role "admin".
// 4. Tìm kiếm theo tên (query ILIKE / prefix).
// 5. Kiểm tra trường is_following phản ánh chính xác trạng thái theo dõi của user đang đăng nhập.
func TestE2E_SearchUsers_FilteringAndFollowStatus(t *testing.T) {
	f := newE2EFixture(t)
	bobCtx := authContext("user-bob")

	// 1. Sync các user snapshots với các role khác nhau
	usersToSync := []struct {
		id   string
		name string
		role string
	}{
		{"user-alice", "Alice Wonder", "user"},
		{"brand-nike", "Nike Vietnam Gym", "brand"},
		{"brand-adidas", "Adidas Training", "brand"},
		{"admin-boss", "Boss Admin", "admin"},
		{"user-charlie", "Charlie Workout", "user"},
	}

	for _, u := range usersToSync {
		err := f.syncUser.Handle(context.Background(), command.SyncUserSnapshotCommand{
			UserID:    u.id,
			FullName:  u.name,
			AvatarURL: "https://gym.com/avatar/" + u.id + ".png",
			Role:      u.role,
		})
		if err != nil {
			t.Fatalf("sync user %s failed: %v", u.id, err)
		}
	}

	// Bob follow Alice trước
	_, err := f.handler.FollowUser(bobCtx, connect.NewRequest(&socialv1message.FollowUserRequest{
		FollowingId: "user-alice",
	}))
	if err != nil {
		t.Fatalf("Bob follow Alice failed: %v", err)
	}

	// 2. Lấy danh sách không truyền query hay role: Phải có Alice, Nike, Adidas, Charlie NHƯNG KHÔNG CÓ Admin!
	resAll, err := f.handler.SearchUsers(bobCtx, connect.NewRequest(&socialv1message.SearchUsersRequest{
		PageSize: 20,
	}))
	if err != nil {
		t.Fatalf("SearchUsers all failed: %v", err)
	}

	for _, item := range resAll.Msg.GetUsers() {
		if item.GetRole() == "admin" || item.GetUserId() == "admin-boss" {
			t.Fatalf("Security violation: admin account %s was returned in search results", item.GetUserId())
		}
		if item.GetUserId() == "user-alice" {
			if !item.GetIsFollowing() {
				t.Errorf("expected is_following to be true for Alice, got false")
			}
		} else {
			if item.GetIsFollowing() {
				t.Errorf("expected is_following to be false for %s, got true", item.GetUserId())
			}
		}
	}

	// 3. Lọc theo role: "brand"
	resBrand, err := f.handler.SearchUsers(bobCtx, connect.NewRequest(&socialv1message.SearchUsersRequest{
		Role:     "brand",
		PageSize: 10,
	}))
	if err != nil {
		t.Fatalf("SearchUsers brand failed: %v", err)
	}
	if len(resBrand.Msg.GetUsers()) != 2 {
		t.Fatalf("expected 2 brands, got %d", len(resBrand.Msg.GetUsers()))
	}
	for _, item := range resBrand.Msg.GetUsers() {
		if item.GetRole() != "brand" {
			t.Errorf("expected role brand, got %s", item.GetRole())
		}
	}

	// 4. Lọc theo role: "user"
	resUser, err := f.handler.SearchUsers(bobCtx, connect.NewRequest(&socialv1message.SearchUsersRequest{
		Role:     "user",
		PageSize: 10,
	}))
	if err != nil {
		t.Fatalf("SearchUsers user failed: %v", err)
	}
	if len(resUser.Msg.GetUsers()) != 2 {
		t.Fatalf("expected 2 users (Alice & Charlie), got %d", len(resUser.Msg.GetUsers()))
	}

	// 5. Tìm kiếm theo query: "Nike"
	resNike, err := f.handler.SearchUsers(bobCtx, connect.NewRequest(&socialv1message.SearchUsersRequest{
		Query:    "Nike",
		PageSize: 10,
	}))
	if err != nil {
		t.Fatalf("SearchUsers Nike failed: %v", err)
	}
	if len(resNike.Msg.GetUsers()) != 1 || resNike.Msg.GetUsers()[0].GetUserId() != "brand-nike" {
		t.Fatalf("expected brand-nike, got %v", resNike.Msg.GetUsers())
	}
	if resNike.Msg.GetUsers()[0].GetIsFollowing() {
		t.Errorf("expected Bob not following Nike yet")
	}

	// 6. Truy vấn cố tình tìm role: "admin" -> Kết quả phải là danh sách rỗng
	resAdmin, err := f.handler.SearchUsers(bobCtx, connect.NewRequest(&socialv1message.SearchUsersRequest{
		Role:     "admin",
		PageSize: 10,
	}))
	if err != nil {
		t.Fatalf("SearchUsers admin should succeed with empty result, got error: %v", err)
	}
	if len(resAdmin.Msg.GetUsers()) != 0 {
		t.Fatalf("expected 0 users when searching for role admin, got %d", len(resAdmin.Msg.GetUsers()))
	}

	// 7. Bob follow Nike -> Kiểm tra lại is_following chuyển sang true
	_, err = f.handler.FollowUser(bobCtx, connect.NewRequest(&socialv1message.FollowUserRequest{
		FollowingId: "brand-nike",
	}))
	if err != nil {
		t.Fatalf("Bob follow Nike failed: %v", err)
	}

	resNikeAfter, err := f.handler.SearchUsers(bobCtx, connect.NewRequest(&socialv1message.SearchUsersRequest{
		Query:    "Nike",
		PageSize: 10,
	}))
	if err != nil {
		t.Fatalf("SearchUsers Nike after follow failed: %v", err)
	}
	if len(resNikeAfter.Msg.GetUsers()) != 1 || !resNikeAfter.Msg.GetUsers()[0].GetIsFollowing() {
		t.Fatalf("expected is_following to be true for Nike after follow")
	}
}

// TestE2E_SearchPosts: Kiểm thử tìm kiếm bài viết kết hợp từ khóa, bộ lọc tác giả và phân quyền bảo mật.
func TestE2E_SearchPosts(t *testing.T) {
	f := newE2EFixture(t)
	ctx := context.Background()

	authorUUID1 := "00000000-0000-0000-0000-000000000001"
	authorUUID2 := "00000000-0000-0000-0000-000000000002"
	authorUUID3 := "00000000-0000-0000-0000-000000000003"
	strangerUUID := "00000000-0000-0000-0000-000000000009"

	// 1. Tạo snapshot 3 người dùng
	_ = f.syncUser.Handle(ctx, command.SyncUserSnapshotCommand{
		UserID:    authorUUID1,
		FullName:  "User One",
		AvatarURL: "https://avatar.png/1",
		Role:      "user",
	})
	_ = f.syncUser.Handle(ctx, command.SyncUserSnapshotCommand{
		UserID:    authorUUID2,
		FullName:  "User Two Coach",
		AvatarURL: "https://avatar.png/2",
		Role:      "user",
	})
	_ = f.syncUser.Handle(ctx, command.SyncUserSnapshotCommand{
		UserID:    authorUUID3,
		FullName:  "User Three",
		AvatarURL: "https://avatar.png/3",
		Role:      "user",
	})

	u1Ctx := authContext(authorUUID1)
	u2Ctx := authContext(authorUUID2)
	u3Ctx := authContext(authorUUID3)
	uStrangerCtx := authContext(strangerUUID)

	// 2. User 1 follow User 2
	_, err := f.handler.FollowUser(u1Ctx, connect.NewRequest(&socialv1message.FollowUserRequest{
		FollowingId: authorUUID2,
	}))
	if err != nil {
		t.Fatalf("User 1 follow User 2 failed: %v", err)
	}

	// 3. User 2 tạo 3 bài viết với 3 cấp độ hiển thị
	_, err = f.handler.CreatePost(u2Ctx, connect.NewRequest(&socialv1message.CreatePostRequest{
		Caption:    "Squat form basics for beginners",
		Visibility: "PUBLIC",
	}))
	if err != nil {
		t.Fatalf("create public squat failed: %v", err)
	}

	_, err = f.handler.CreatePost(u2Ctx, connect.NewRequest(&socialv1message.CreatePostRequest{
		Caption:    "Advanced squat technique for champions",
		Visibility: "FOLLOWERS_ONLY",
	}))
	if err != nil {
		t.Fatalf("create followers_only squat failed: %v", err)
	}

	_, err = f.handler.CreatePost(u2Ctx, connect.NewRequest(&socialv1message.CreatePostRequest{
		Caption:    "Personal squat injury private notes",
		Visibility: "PRIVATE",
	}))
	if err != nil {
		t.Fatalf("create private squat failed: %v", err)
	}

	// 4. User 3 tạo 1 bài viết public
	_, err = f.handler.CreatePost(u3Ctx, connect.NewRequest(&socialv1message.CreatePostRequest{
		Caption:    "Bench press and squat combo",
		Visibility: "PUBLIC",
	}))
	if err != nil {
		t.Fatalf("create user 3 squat failed: %v", err)
	}

	// Test Case 1: User 1 (Follower của User 2) tìm kiếm "squat"
	// Kỳ vọng: 3 bài viết (User 2 Public + User 2 Followers-only + User 3 Public)
	resU1, err := f.handler.SearchPosts(u1Ctx, connect.NewRequest(&socialv1message.SearchPostsRequest{
		Query:    "squat",
		PageSize: 10,
	}))
	if err != nil {
		t.Fatalf("User 1 search failed: %v", err)
	}
	if len(resU1.Msg.GetItems()) != 3 {
		t.Fatalf("User 1 expected 3 posts, got %d", len(resU1.Msg.GetItems()))
	}

	// Test Case 2: Stranger (không follow User 2) tìm kiếm "squat"
	// Kỳ vọng: 2 bài viết PUBLIC duy nhất (User 2 Public + User 3 Public)
	resStranger, err := f.handler.SearchPosts(uStrangerCtx, connect.NewRequest(&socialv1message.SearchPostsRequest{
		Query:    "squat",
		PageSize: 10,
	}))
	if err != nil {
		t.Fatalf("Stranger search failed: %v", err)
	}
	if len(resStranger.Msg.GetItems()) != 2 {
		t.Fatalf("Stranger expected 2 public posts, got %d", len(resStranger.Msg.GetItems()))
	}
	for _, item := range resStranger.Msg.GetItems() {
		if item.GetVisibility() != "PUBLIC" {
			t.Errorf("stranger must not see non-public post: %s", item.GetVisibility())
		}
	}

	// Test Case 3: Tác giả User 2 tìm kiếm "squat"
	// Kỳ vọng: Nhìn thấy cả 4 bài (bao gồm bài PRIVATE của chính mình)
	resU2, err := f.handler.SearchPosts(u2Ctx, connect.NewRequest(&socialv1message.SearchPostsRequest{
		Query:    "squat",
		PageSize: 10,
	}))
	if err != nil {
		t.Fatalf("User 2 search failed: %v", err)
	}
	if len(resU2.Msg.GetItems()) != 4 {
		t.Fatalf("User 2 author expected 4 posts, got %d", len(resU2.Msg.GetItems()))
	}

	// Test Case 4: Lọc theo tác giả cụ thể (User 3)
	// Kỳ vọng: Chỉ 1 bài viết của User 3
	resFilterAuthor, err := f.handler.SearchPosts(u1Ctx, connect.NewRequest(&socialv1message.SearchPostsRequest{
		Query:    "squat",
		UserId:   &authorUUID3,
		PageSize: 10,
	}))
	if err != nil {
		t.Fatalf("Author filter search failed: %v", err)
	}
	if len(resFilterAuthor.Msg.GetItems()) != 1 {
		t.Fatalf("expected 1 post from user 3, got %d", len(resFilterAuthor.Msg.GetItems()))
	}
	if resFilterAuthor.Msg.GetItems()[0].GetUserId() != authorUUID3 {
		t.Errorf("expected post of user 3, got %s", resFilterAuthor.Msg.GetItems()[0].GetUserId())
	}

	// Test Case 5: Khách vãng lai chưa đăng nhập (unauthenticated - context.Background())
	// Kỳ vọng: Public bypass hoạt động mượt mà, chỉ thấy 2 bài PUBLIC
	resGuest, err := f.handler.SearchPosts(context.Background(), connect.NewRequest(&socialv1message.SearchPostsRequest{
		Query:    "squat",
		PageSize: 10,
	}))
	if err != nil {
		t.Fatalf("Guest search failed: %v", err)
	}
	if len(resGuest.Msg.GetItems()) != 2 {
		t.Fatalf("Guest expected 2 public posts, got %d", len(resGuest.Msg.GetItems()))
	}

	// Test Case 6: Cả Query và UserId đều rỗng -> trả về CodeInvalidArgument
	_, errInvalid := f.handler.SearchPosts(u1Ctx, connect.NewRequest(&socialv1message.SearchPostsRequest{
		Query: "   ",
	}))
	if errInvalid == nil || connect.CodeOf(errInvalid) != connect.CodeInvalidArgument {
		t.Fatalf("expected CodeInvalidArgument for empty search, got %v", errInvalid)
	}

	// Test Case 7: Query < 2 ký tự khi không có author_id -> trả về CodeInvalidArgument
	_, errShort := f.handler.SearchPosts(u1Ctx, connect.NewRequest(&socialv1message.SearchPostsRequest{
		Query: "s",
	}))
	if errShort == nil || connect.CodeOf(errShort) != connect.CodeInvalidArgument {
		t.Fatalf("expected CodeInvalidArgument for search query too short, got %v", errShort)
	}

	// Test Case 8: UserId không phải UUID hợp lệ -> trả về CodeInvalidArgument
	invalidUUID := "not-a-valid-uuid"
	_, errInvalidUUID := f.handler.SearchPosts(u1Ctx, connect.NewRequest(&socialv1message.SearchPostsRequest{
		Query:  "squat",
		UserId: &invalidUUID,
	}))
	if errInvalidUUID == nil || connect.CodeOf(errInvalidUUID) != connect.CodeInvalidArgument {
		t.Fatalf("expected CodeInvalidArgument for invalid author_id UUID, got %v", errInvalidUUID)
	}
}
