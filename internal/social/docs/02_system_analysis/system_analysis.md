# Phân Tích Hệ Thống — Module Social (System Analysis)

Tài liệu này trình bày phân tích ca sử dụng (Use Cases), tác nhân (Actors), phân tích sự kiện (Event Storming), và luồng dữ liệu (Data Flow) của Bounded Context **Social**.

---

## 1. Tác Nhân Hệ Thống (Actors)

| Tác nhân | Vai trò trong hệ thống |
| :--- | :--- |
| **Gym User (Athlete)** | Người dùng tập gym, đăng bài viết, chia sẻ buổi tập, theo dõi bạn bè, thả reaction, bình luận. |
| **Follower / Friend** | Bạn bè trong mạng lưới theo dõi, tiếp nhận thông báo và bảng tin cá nhân hóa từ người họ follow. |
| **Workout Execution Module** | Module bên ngoài phát sự kiện khi người dùng hoàn thành và chia sẻ buổi tập luyện. |
| **Auth / Profile Module** | Các module danh tính phát sự kiện cập nhật tài khoản để Social đồng bộ bản sao User Snapshot. |

---

## 2. Biểu Đồ Ca Sử Dụng (Use Case Diagram)

```mermaid
flowchart LR
    User([Gym User])

    subgraph Social_Bounded_Context [Module Social & Feed]
        UC1[Tạo bài viết cá nhân]
        UC2[Xem Bảng tin kết hợp Feed]
        UC3[Xem Trang cá nhân người khác]
        UC4[Thả/Đổi Cảm xúc Reaction]
        UC5[Xem danh sách Reactions]
        UC6[Bình luận / Trả lời bình luận]
        UC7[Follow / Unfollow người dùng]
        UC8[Xem danh sách Follower/Following]
        UC9[Xóa bài viết / Bình luận của mình]
    end

    subgraph Background_Workers [Async Event Ingestion]
        UC10[Nạp hoạt động buổi tập chia sẻ]
        UC11[Đồng bộ User Snapshot]
    end

    User --> UC1
    User --> UC2
    User --> UC3
    User --> UC4
    User --> UC5
    User --> UC6
    User --> UC7
    User --> UC8
    User --> UC9

    WorkoutModule([Workout Execution]) -->|Kafka: workoutSessionShared| UC10
    AuthModule([Auth Module]) -->|Kafka: userRegistered| UC11
    ProfileModule([Profile Module]) -->|Kafka: UserIdentityUpdated| UC11
```

---

## 3. Phân Tích Sự Kiện (Event Storming & Domain Events)

Module Social là cả nhà xuất bản (Publisher) và người tiêu thụ (Consumer) sự kiện bất đồng bộ:

### 3.1. Sự kiện Nội bộ Module Phát ra (Outbound Domain Events)
Các sự kiện này được ghi vào bảng `social.outbox` trong cùng transaction với thay đổi nghiệp vụ, sau đó Outbox Worker quét và phát tán ra Kafka:

| Tên sự kiện (CloudEvent Type) | Kafka Topic | Partition Key | Điều kiện kích hoạt | Dữ liệu chính (Payload) |
| :--- | :--- | :--- | :--- | :--- |
| `contracts.supporting.social.v1.postCreated` | `social.events` | `author_id` | Người dùng tạo bài viết hoặc chia sẻ buổi tập thành công | `post_id`, `author_id`, `caption`, `created_at` |
| `contracts.supporting.social.v1.postReacted` | `social.events` | `reactor_id` | Người dùng thả hoặc đổi cảm xúc trên bài viết | `post_id`, `author_id`, `reactor_id`, `reaction_type` |
| `contracts.supporting.social.v1.postCommented` | `social.events` | `commenter_id` | Người dùng thêm bình luận vào bài viết | `post_id`, `author_id`, `commenter_id`, `comment_id`, `content` |
| `contracts.supporting.social.v1.userFollowed` | `social.events` | `follower_id` | Người dùng theo dõi một gymer khác | `follower_id`, `following_id`, `created_at` |
| `contracts.supporting.social.v1.userUnfollowed` | `social.events` | `follower_id` | Người dùng hủy theo dõi một gymer khác | `follower_id`, `following_id`, `unfollowed_at` |

### 3.2. Sự kiện Lắng Nghe từ Module Khác (Inbound Events)

| Nguồn phát | Kafka Topic | Consumer Group | Tên sự kiện (CloudEvent Type) | Hành động trong Social |
| :--- | :--- | :--- | :--- | :--- |
| **Workout Execution** | `workout_execution.events` | `social-workout-shared-group` | `contracts.core.workout_execution.v1.workoutSessionShared` | Tạo `FeedItem` loại `WORKOUT_ACTIVITY` và đưa lên bảng tin |
| **Auth** | `auth.events` | `social-user-registered-group` | `contracts.generic.auth.v1.userRegistered` | Tạo bản ghi mới trong `social.user_snapshots` |
| **Profile** | `profile.events` | `social-user-identity-updated-group` | `contracts.supporting.profile.v1.event.UserIdentityUpdated` | Cập nhật `full_name`, `avatar_url` trong `social.user_snapshots` |

---

## 4. Sơ Đồ Luồng Dữ Liệu Chi Tiết (Data Flow Diagrams)

### 4.1. Luồng Chia Sẻ Buổi Tập (Privacy-First Ingestion)

```mermaid
sequenceDiagram
    autonumber
    actor Athlete as Người Tập Gym
    participant WorkoutService as Workout Execution Module
    participant Kafka as Kafka Broker (workout_execution.events)
    participant Consumer as WorkoutSharedConsumer
    participant OutboxLog as social.outbox_log
    participant SocialDB as social.feed_items
    participant SocialOutbox as social.outbox

    Athlete->>WorkoutService: Kết thúc buổi tập & Bấm "Chia sẻ lên cộng đồng"
    WorkoutService->>Kafka: Phát CloudEvent `workoutSessionShared`
    Kafka->>Consumer: Gửi message
    Consumer->>OutboxLog: Kiểm tra IsProcessed(cloudEvent.ID)
    alt Đã xử lý rồi (Duplicate Event)
        Consumer-->>Kafka: Bỏ qua (ACK)
    else Chưa xử lý
        Consumer->>SocialDB: INSERT INTO social.feed_items (type=WORKOUT_ACTIVITY, metrics, data)
        Consumer->>SocialOutbox: INSERT INTO social.outbox (postCreated)
        Consumer->>OutboxLog: INSERT INTO social.outbox_log (status='PROCESSED')
        Consumer-->>Kafka: Commit offset thành công
    end
```

### 4.2. Luồng Lấy Bảng Tin Cá Nhân Hóa (Personalized Activity Feed)

```mermaid
sequenceDiagram
    autonumber
    actor User as Người dùng
    participant API as gRPC / REST Gateway
    participant QueryHandler as GetPersonalizedFeedHandler
    participant FollowRepo as FollowRepository
    participant FeedRepo as FeedItemRepository
    participant SnapshotRepo as UserSnapshotRepository

    User->>API: GET /api/v1/social/feed?limit=20&cursor=...
    API->>QueryHandler: Handle(GetPersonalizedFeedQuery)
    QueryHandler->>FollowRepo: Lấy danh sách following_ids của User
    Note over QueryHandler: Danh sách tác giả = following_ids + [User.ID]
    QueryHandler->>FeedRepo: SELECT * FROM social.feed_items WHERE user_id IN (...) ORDER BY created_at DESC LIMIT 20
    QueryHandler->>SnapshotRepo: Lấy thông tin UserSnapshot theo batch các author_ids
    QueryHandler-->>API: Trả về DTO FeedItems (đã kèm FullName, AvatarUrl, Metrics)
    API-->>User: Hiển thị bảng tin Newsfeed mượt mà
```

### 4.3. Luồng Tương Tác Cảm Xúc (Idempotent Reaction Toggle)

```mermaid
sequenceDiagram
    autonumber
    actor User as Người dùng
    participant API as gRPC / REST Gateway
    participant ReactHandler as ReactTargetHandler
    participant InteractionRepo as InteractionRepository
    participant FeedRepo as FeedItemRepository
    participant Outbox as OutboxRepository

    User->>API: POST /api/v1/social/feed-items/{id}/reactions (reaction_type="FIRE")
    API->>ReactHandler: Handle(ReactTargetCommand)
    ReactHandler->>InteractionRepo: Tìm reaction hiện tại của User trên feed_item_id
    alt Chưa có reaction
        ReactHandler->>InteractionRepo: SaveReaction(Reaction)
        ReactHandler->>FeedRepo: UpdateReactionCount(delta = +1)
        ReactHandler->>Outbox: Record PostReactedEvent
    else Đã có cùng loại ("FIRE")
        ReactHandler->>InteractionRepo: DeleteReaction(Reaction) [Toggle tắt]
        ReactHandler->>FeedRepo: UpdateReactionCount(delta = -1)
    else Đã có khác loại (ví dụ "LIKE" -> "FIRE")
        ReactHandler->>InteractionRepo: SaveReaction(ReactionType = "FIRE")
        ReactHandler->>Outbox: Record PostReactedEvent (loại mới)
    end
    ReactHandler-->>API: Thành công
    API-->>User: Trả về trạng thái cảm xúc mới
```

### 4.4. Luồng Lấy Dòng Hoạt Động Trang Cá Nhân (User Profile Feed)

```mermaid
sequenceDiagram
    autonumber
    actor Viewer as Người xem
    participant API as gRPC / REST Gateway
    participant QueryHandler as GetUserProfileFeedHandler
    participant FeedRepo as FeedItemRepository
    participant SnapshotRepo as UserSnapshotRepository

    Viewer->>API: GET /api/v1/social/users/{user_id}/feed?page_size=20&cursor=...
    API->>QueryHandler: Handle(GetUserProfileFeedQuery)
    QueryHandler->>FeedRepo: SELECT * FROM social.feed_items WHERE user_id = target_user_id AND created_at < cursor ORDER BY created_at DESC LIMIT 20
    QueryHandler->>SnapshotRepo: Lấy FullName & AvatarUrl của target_user_id
    Note over QueryHandler: Đọc trực tiếp reaction_count & comment_count từ bảng feed_items (không query lặp N+1)
    QueryHandler-->>API: Trả về DTO FeedItems kèm next_cursor
    API-->>Viewer: Hiển thị dòng thời gian trang cá nhân
    Note over Viewer,API: Người xem muốn xem chi tiết ai reaction/bình luận thì nhấn vào gọi riêng API ListReactions / ListComments
```

### 4.5. Luồng Đồ Thị Quan Hệ Bạn Bè (Follow & Notification Dispatch)

```mermaid
sequenceDiagram
    autonumber
    actor UserA as Người Dùng A
    participant API as gRPC / REST Gateway
    participant FollowHandler as FollowUserHandler
    participant FollowRepo as FollowRepository
    participant Outbox as OutboxRepository
    actor UserB as Người Dùng B

    UserA->>API: POST /api/v1/social/follow (following_id = UserB.ID)
    API->>FollowHandler: Handle(FollowUserCommand)
    Note over FollowHandler: Kiểm tra ràng buộc cấm tự follow chính mình (UserA != UserB)
    FollowHandler->>FollowRepo: Kiểm tra cặp (follower_id, following_id)
    alt Đã follow trước đó
        FollowHandler-->>API: Bỏ qua không lỗi (Idempotent)
    else Chưa follow
        FollowHandler->>FollowRepo: INSERT INTO social.follows (UserA.ID, UserB.ID)
        FollowHandler->>Outbox: INSERT INTO social.outbox (userFollowed)
    end
    FollowHandler-->>API: Trả về thành công
    Note over Outbox: Outbox Worker đẩy event userFollowed lên Kafka -> Module Notification gửi Push cho User B
```

### 4.6. Luồng Bình Luận Đa Cấp (Threaded Comments Flow)

```mermaid
sequenceDiagram
    autonumber
    actor Commenter as Người Bình Luận
    participant API as gRPC / REST Gateway
    participant CommentHandler as AddCommentHandler
    participant FeedRepo as FeedItemRepository
    participant InteractionRepo as InteractionRepository
    participant Outbox as OutboxRepository

    Commenter->>API: POST /api/v1/social/feed-items/{id}/comments (content, parent_id?)
    API->>CommentHandler: Handle(AddCommentCommand)
    CommentHandler->>FeedRepo: Kiểm tra bài viết feed_item có tồn tại không
    alt Nếu có parent_id (Trả lời bình luận)
        CommentHandler->>InteractionRepo: Kiểm tra bình luận cha parent_id thuộc đúng feed_item
    end
    CommentHandler->>InteractionRepo: SaveComment(Comment)
    CommentHandler->>FeedRepo: UpdateCommentCount(delta = +1)
    CommentHandler->>Outbox: INSERT INTO social.outbox (postCommented)
    CommentHandler-->>API: Trả về CommentDTO vừa tạo
    API-->>Commenter: Hiển thị bình luận mới
```

### 4.7. Luồng Đồng Bộ Danh Tính Bất Đồng Bộ (Async User Snapshot Sync)

```mermaid
sequenceDiagram
    autonumber
    participant AuthProfile as Auth / Profile Service
    participant Kafka as Kafka Broker (auth.events / profile.events)
    participant Consumer as UserSyncConsumers (UserRegistered / UserIdentityUpdated)
    participant OutboxLog as social.outbox_log
    participant SyncHandler as SyncUserSnapshotHandler
    participant SnapshotRepo as social.user_snapshots

    AuthProfile->>Kafka: Phát sự kiện (userRegistered / UserIdentityUpdated)
    Kafka->>Consumer: Gửi message
    Consumer->>OutboxLog: Kiểm tra IsProcessed(cloudEvent.ID)
    alt Đã xử lý (Trùng event)
        Consumer-->>Kafka: Bỏ qua (ACK)
    else Chưa xử lý
        Consumer->>SyncHandler: Handle(SyncUserSnapshotCommand)
        SyncHandler->>SnapshotRepo: UPSERT INTO social.user_snapshots (id, full_name, avatar_url)
        Consumer->>OutboxLog: Ghi log trạng thái 'PROCESSED'
        Consumer-->>Kafka: Commit offset thành công
    end
```

### 4.8. Luồng Quét & Đẩy Sự Kiện Ra Kafka Bus (Transactional Outbox Worker)

```mermaid
sequenceDiagram
    autonumber
    participant Worker as Outbox Worker (5s interval)
    participant OutboxRepo as social.outbox
    participant KafkaPub as Kafka Publisher (social.events)

    loop Định kỳ mỗi 5 giây
        Worker->>OutboxRepo: ClaimBatch(limit=50) với SELECT ... FOR UPDATE SKIP LOCKED
        alt Không có bản ghi unpublished
            Note over Worker: Ngủ chờ chu kỳ quét kế tiếp
        else Có danh sách sự kiện chưa gửi
            loop Duyệt từng bản ghi outbox
                Worker->>KafkaPub: Publish message (key=partition_key, value=payload)
                alt Gửi thành công
                    Worker->>OutboxRepo: MarkPublished(event_ids) -> UPDATE published = TRUE
                else Lỗi broker tạm thời
                    Note over Worker: Giữ nguyên unpublished để lần quét sau thử lại
                end
            end
        end
    end
```
