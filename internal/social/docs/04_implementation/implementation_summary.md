# Báo Cáo Hiện Thực Hóa — Module Social (Implementation Summary)

Tài liệu này tổng kết toàn bộ hiện trạng triển khai thực tế, mã nguồn hoàn thiện, các bộ kiểm thử và kiểm chứng chất lượng cho Bounded Context **Social & Activity Feed**.

---

## 1. Danh Mục Các Thành Phần Đã Triển Khai Thực Tế

### 1.1. Tầng Domain (`/internal/social/domain/`)
- [x] **Aggregates & Unit Tests Độc Lập (Tách 1:1)**:
  - `aggregate/feed_item.go` & `aggregate/feed_item_test.go`: Aggregate gốc cho Post và WorkoutActivity (`NewPostItem`, `NewWorkoutActivityItem`).
  - `aggregate/follow.go` & `aggregate/follow_test.go`: Aggregate quản lý quan hệ theo dõi đồ thị bạn bè (`NewFollow`, chặn tự follow).
- [x] **Entities & Unit Tests Độc Lập (Tách 1:1)**:
  - `entity/comment.go` & `entity/comment_test.go`: Thực thể bình luận và phản hồi lồng nhau trên bài viết.
  - `entity/reaction.go` & `entity/reaction_test.go`: Thực thể biểu cảm người dùng trên bài viết (`NewReaction`, `ChangeReactionType`).
  - `entity/user_snapshot.go` & `entity/user_snapshot_test.go`: Bản sao định danh người dùng đồng bộ từ Profile/Auth.
- [x] **Value Objects**:
  - `vo/item_type.go`: Định nghĩa kiểu bài viết (`POST`, `WORKOUT_ACTIVITY`).
  - `vo/visibility.go`: Cấp độ hiển thị (`PUBLIC`, `FOLLOWERS_ONLY`, `PRIVATE`).
  - `vo/reaction_type.go`: Strongly-typed enum cho cảm xúc (`LIKE`, `FIRE`, `MUSCLE`, `CLAP`).
  - `vo/workout_metrics.go`: Đóng gói dữ liệu buổi tập (Title, Duration, Volume, Sets, PR).
  - `vo/target_type.go`: Phân loại đối tượng tương tác.
- [x] **Domain Events**:
  - `event/events.go`: Khai báo các sự kiện nghiệp vụ (`PostCreatedEvent`, `PostReactedEvent`, `PostCommentedEvent`, `UserFollowedEvent`, `UserUnfollowedEvent`).
- [x] **Repository Ports (Interfaces)**:
  - Khai báo các giao diện trừu tượng cho tầng Persistence: `FeedItemRepository`, `FollowRepository`, `InteractionRepository` (quản lý Reaction & Comment), `UserSnapshotRepository`.

---

### 1.2. Tầng Application (`/internal/social/application/`)
- [x] **Command Handlers & Unit Tests Độc Lập (Tách 1:1)**:
  - `command/create_post.go` & `command/create_post_test.go`: Xử lý tạo bài viết cá nhân.
  - `command/delete_feed_item.go` & `command/delete_feed_item_test.go`: Xử lý xóa bài viết / hoạt động.
  - `command/ingest_workout_activity.go` & `command/ingest_workout_activity_test.go`: Nạp hoạt động tập luyện từ Kafka.
  - `command/react_target.go` & `command/react_target_test.go`: Thả, đổi hoặc hủy reaction (Idempotent Toggle).
  - `command/add_comment.go` & `command/add_comment_test.go`: Thêm bình luận đa cấp.
  - `command/delete_comment.go` & `command/delete_comment_test.go`: Xóa bình luận.
  - `command/follow_user.go` & `command/follow_user_test.go`: Xử lý theo dõi người dùng & phát sự kiện.
  - `command/unfollow_user.go` & `command/unfollow_user_test.go`: Xử lý hủy theo dõi người dùng & validation.
  - `command/sync_user_snapshot.go` & `command/sync_user_snapshot_test.go`: Cập nhật bản sao người dùng từ Profile/Auth.
  - `command/mocks_test.go`: Mock repository và transaction manager dùng chung cho các test suite command.
- [x] **Query Handlers & Unit Tests Độc Lập (Tách 1:1)**:
  - `query/get_activity_feed.go` & `query/get_activity_feed_test.go`: Lấy bảng tin Newfeed bạn bè và cá nhân bằng SQL tối ưu.
  - `query/get_user_profile_feed.go` & `query/get_user_profile_feed_test.go`: Lấy dòng hoạt động trang cá nhân tác giả (Cursor Pagination).
  - `query/get_social_summary.go` & `query/get_social_summary_test.go`: Tổng hợp số liệu thống kê cá nhân (PostCount, Followers, Following).
  - `query/get_followers.go` & `query/get_followers_test.go`: Truy vấn danh sách người theo dõi kèm Snapshot.
  - `query/get_following.go` & `query/get_following_test.go`: Truy vấn danh sách đang theo dõi kèm Snapshot.
  - `query/list_comments.go` & `query/list_comments_test.go`: Lấy danh sách bình luận (Cursor Pagination).
  - `query/list_reactions.go` & `query/list_reactions_test.go`: Lấy danh sách người thả cảm xúc trên bài viết.
  - `query/mocks_test.go`: Mock repository dùng chung cho các test suite query.
- [x] **Application Ports**:
  - Tách rời riêng biệt:
    - `port/outbox_repository.go` (`OutboxRecord`, `Save`, `ClaimBatch`, `MarkPublished`).
    - `port/outbox_log_repository.go` (`OutboxLogRecord`, `IsProcessed`, `Save`, `SaveLog`).
    - `port/tx.go` (`TransactionManager`).
    - `port/event_publisher.go` (`EventPublisher`).

---

### 1.3. Tầng Infrastructure (`/internal/social/infrastructure/`)
- [x] **Persistence Layer (GORM PostgreSQL)**:
  - `gorm_feed_item_repository.go`: Lưu trữ & truy vấn feed items (Cursor range scan `created_at < cursor`).
  - `gorm_follow_repository.go`: Lưu trữ & quản lý quan hệ bạn bè đồ thị xã hội.
  - `gorm_interaction_repository.go`: Quản lý toàn bộ Reaction và Comment trên bài viết.
  - `gorm_user_snapshot_repository.go`: Lưu trữ bản sao danh tính người dùng cục bộ.
  - `gorm_outbox_repository.go`: Khóa hàng đợi sự kiện đa worker song song qua `clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}`.
  - `gorm_outbox_log_repository.go`: Quản lý Inbox log kiểm tra Idempotency khi tiêu thụ Kafka event.
  - `models.go`, `mapper.go`, `tx_manager.go`: Ánh xạ cấu trúc dữ liệu GORM <-> Domain.
- [x] **Event & Outbox Engine**:
  - `event/outbox_writer.go`: Chuyển đổi Domain Events thành CloudEvents 1.0 JSON hợp lệ và lưu vào `social.outbox` trong cùng transaction.
  - `worker/outbox_worker.go`: Background worker quét bảng outbox và đẩy lên Kafka với chu kỳ định thời.

---

### 1.4. Tầng Transport (`/internal/social/transport/`)
- [x] **gRPC / ConnectRPC Handler**:
  - `grpc/handler.go`: Hiện thực hóa toàn bộ RPC methods của `SocialServiceServer` với authentication context, mapping lỗi sang gRPC status codes.
- [x] **Kafka Consumers (Tách 1:1 độc lập)**:
  - `consumer/cloudevent.go`: Định nghĩa struct chung CloudEvents 1.0.
  - `consumer/user_registered_consumer.go`: Tiêu thụ sự kiện `contracts.generic.auth.v1.userRegistered` từ Topic `auth.events` (Group: `social-user-registered-group`).
  - `consumer/user_identity_updated_consumer.go`: Tiêu thụ sự kiện `contracts.supporting.profile.v1.event.UserIdentityUpdated` từ Topic `profile.events` (Group: `social-user-identity-updated-group`).
  - `consumer/workout_shared_consumer.go`: Tiêu thụ sự kiện `contracts.core.workout_execution.v1.workoutSessionShared` từ Topic `workout_execution.events` (Group: `social-workout-shared-group`).

---

### 1.5. Cơ Sở Dữ Liệu & API Contracts
- [x] **Database Migration**:
  - `12-create-social-tables.sql`: Tạo toàn bộ 7 bảng trong schema `social` kèm Foreign Keys, Indexes, Unique constraints và Cascading Delete.
  - Khởi tạo schema `social` trong `01-init-schemas.sql`.
- [x] **Protobuf Contracts**:
  - `proto/contracts/supporting/social/v1/message/social_messages.proto`: Khai báo thông điệp.
  - `proto/contracts/supporting/social/v1/service/social_service.proto`: Khai báo RPC service & gRPC-Gateway HTTP annotations.
  - Chạy `buf generate` tạo mã nguồn Go stubs và OpenAPI Swagger specs sạch sẽ.

---

## 2. Kết Quả Kiểm Thử Tự Động (Test Suites Execution)

Tất cả các tầng kiến trúc và toàn bộ hành trình người dùng (E2E) đều sở hữu bộ kiểm thử tự động độc lập và đạt tỷ lệ vượt qua 100%:

### 2.1. Kiểm thử E2E Toàn Diện (Full User Journey & Security)
- Thư mục: `internal/social/test/` (`e2e_test.go`, `testutil/testutil.go`).
- Kiểm tra toàn bộ stack từ ConnectRPC / gRPC Client $\rightarrow$ Application Handlers $\rightarrow$ Domain Aggregates $\rightarrow$ GORM Repositories SQLite in-memory:
```text
=== RUN   TestE2E_FullSocialJourney
--- PASS: TestE2E_FullSocialJourney (0.01s)
=== RUN   TestE2E_ValidationAndSecurityRules
--- PASS: TestE2E_ValidationAndSecurityRules (0.00s)
PASS
ok      github.com/viethung213/gym-companion/internal/social/test   0.109s
```

### 2.2. Kiểm thử Tầng Transport Consumer
```text
=== RUN   TestUserIdentityUpdatedConsumer_ProcessMessage
--- PASS: TestUserIdentityUpdatedConsumer_ProcessMessage (0.00s)
=== RUN   TestUserRegisteredConsumer_ProcessMessage
--- PASS: TestUserRegisteredConsumer_ProcessMessage (0.00s)
=== RUN   TestWorkoutSharedConsumer_ProcessMessage
--- PASS: TestWorkoutSharedConsumer_ProcessMessage (0.00s)
PASS
ok      github.com/viethung213/gym-companion/internal/social/transport/consumer 0.521s
```

### 2.3. Kiểm thử Toàn Bộ Bounded Context Social
```text
ok      github.com/viethung213/gym-companion/internal/social/application/command    (cached)
ok      github.com/viethung213/gym-companion/internal/social/application/query      (cached)
ok      github.com/viethung213/gym-companion/internal/social/domain/aggregate       (cached)
ok      github.com/viethung213/gym-companion/internal/social/domain/entity          (cached)
ok      github.com/viethung213/gym-companion/internal/social/infrastructure/event   (cached)
ok      github.com/viethung213/gym-companion/internal/social/test                   0.109s
ok      github.com/viethung213/gym-companion/internal/social/transport/consumer     0.521s
ok      github.com/viethung213/gym-companion/internal/social/transport/grpc         (cached)
```

### 2.4. Kiểm tra Biên Dịch Ứng Dụng (Compile Check)
- Lệnh: `go build -o nul ./cmd/api`
- Kết quả: `Exit code: 0` (Thành công tuyệt đối, không có xung đột phụ thuộc).

---

## 3. Hướng Dẫn Vận Hành & Khởi Chạy

Module Social được tích hợp tự động vào vòng đời của ứng dụng thông qua `internal/social/module.go`:

```go
// Khởi tạo Module Social
socialModule, err := social.NewModule(ctx, social.ModuleDeps{
    DB:            db,
    KafkaRegistry: kafkaRegistry,
})
```

Khi máy chủ khởi động:
1. GORM và Database kết nối tới schema `social`.
2. Outbox Background Worker bắt đầu quét các sự kiện chưa phát hành để bắn sang Kafka broker (Topic `social.events`).
3. Ba Kafka Consumers tự động kích hoạt goroutines lắng nghe trên các topics và consumer groups tương ứng:
   - `WorkoutSharedConsumer`: Topic `workout_execution.events` (Group: `social-workout-shared-group`).
   - `UserRegisteredConsumer`: Topic `auth.events` (Group: `social-user-registered-group`).
   - `UserIdentityUpdatedConsumer`: Topic `profile.events` (Group: `social-user-identity-updated-group`).
4. Social gRPC Handler được đăng ký vào gRPC Server và REST Gateway Router.
