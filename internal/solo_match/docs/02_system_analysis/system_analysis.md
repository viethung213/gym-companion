# Phân Tích Hệ Thống — Bounded Context Solo Match (System Analysis)

Tài liệu này trình bày phân tích chi tiết về ca sử dụng (Use Cases), tác nhân (Actors), sơ đồ tuần tự (Sequence Diagrams), phân tích sự kiện liên module (Event Storming) và máy trạng thái hữu hạn (FSM) thuộc phạm vi quản lý của Bounded Context **Solo Match**.

---

## 1. Tác Nhân Hệ Thống (Actors)

| Tác nhân | Phân loại | Vai trò và tương tác với Solo Match |
| :--- | :--- | :--- |
| **Gym User (Player)** | Con người (User) | Gửi yêu cầu tìm trận (`JoinQueue`), hủy tìm trận (`CancelQueue`), tra cứu chi tiết trận (`GetMatch`), xem lịch sử (`GetMatchHistory`). |
| **Redis Matchmaker Engine** | Hạ tầng thời gian thực | Lưu trữ vé chờ trong ZSet, thực thi Lua script bốc vé nguyên tử, phát thông báo qua kênh Pub/Sub. |
| **System Bot Pool** | Tài nguyên hệ thống | Bảng cơ sở dữ liệu `solo_match.system_bots` cung cấp danh sách Bot hợp lệ khi kích hoạt Bot Fallback sau 5 phút. |
| **MatchTimeoutWorker** | Daemon Worker | Chạy ngầm định kỳ: quét trận `PENDING_READY` quá hạn 60s điểm danh (`CancelOnNoShow`) và quét trận `IN_PROGRESS` treo quá 35 phút (`TimeoutInProgress`). |
| **Exercise Provider** | Outbound Port (Service) | Cung cấp bài tập thể lực thi đấu ngẫu nhiên kèm thể thức `REPS` hoặc `DURATION`. |
| **Workout Execution Module** | Module bên ngoài (Event) | Nơi diễn ra buổi tập camera thực tế; phát các sự kiện `SoloMatchRoomReady`, `SoloTrackingCompleted`, `SoloTrackingAborted` về cho `solo_match`. |
| **Auth & Profile Modules** | Module bên ngoài (Event) | Phát sự kiện `UserRegistered`, `UserIdentityUpdated` để `solo_match` duy trì bản sao danh tính `user_snapshots`. |

---

## 2. Biểu Đồ Ca Sử Dụng (Use Case Diagram)

```mermaid
flowchart LR
    User([Gym User])
    TimeoutWorker([Match Timeout Worker])
    WorkoutMod([Workout Execution Module])
    AuthMod([Auth / Profile Modules])

    subgraph Solo_Match_Bounded_Context [Bounded Context: Solo Match]
        UC1[UC-SM.1: Vào hàng đợi tìm trận\nJoin Queue via gRPC Stream]
        UC2[UC-SM.2: Hủy tìm trận\nCancel Queue]
        UC3[UC-SM.3: Ghép cặp 2 Người chơi\nPvP Atomic Match]
        UC4[UC-SM.4: Ghép cặp với Bot sau 5 phút\nBot Fallback from System Pool]
        UC5[UC-SM.5: Kích hoạt trận đấu\nActivate Match on Room Ready]
        UC6[UC-SM.6: Hủy trận do quá 60s không vào\nCancel on Ready Deadline Timeout]
        UC7[UC-SM.7: Tiếp nhận kết quả & Chấm điểm\nRecord Result & Resolve Outcome]
        UC8[UC-SM.8: Xử thua tức thì khi đối thủ bỏ cuộc\nFast Forfeit on Tracking Aborted]
        UC9[UC-SM.9: Cưỡng chế trận treo quá 35 phút\nIn-Progress Safety Net Timeout]
        UC10[UC-SM.10: Đồng bộ bản sao danh tính\nSync User Snapshot]
    end

    User --> UC1
    User --> UC2
    
    UC1 -.-> UC3
    UC1 -.-> UC4
    
    TimeoutWorker --> UC6
    TimeoutWorker --> UC9

    WorkoutMod -->|Event: SoloMatchRoomReady| UC5
    WorkoutMod -->|Event: SoloTrackingCompleted| UC7
    WorkoutMod -->|Event: SoloTrackingAborted| UC8

    AuthMod -->|Event: UserRegistered / UserIdentityUpdated| UC10
```

---

## 3. Sơ Đồ Tuần Tự Chi Tiết (Sequence Diagrams)

### 3.1. Luồng Ghép Cặp PvP & Kích Hoạt Trận Đấu

```mermaid
sequenceDiagram
    autonumber
    actor User1 as Gymer 1
    actor User2 as Gymer 2
    participant Handler as SoloMatch gRPC Handler
    participant Redis as Redis (ZSet & Pub/Sub)
    participant Worker as Matchmaker Worker
    participant DB as PostgreSQL (solo_match)
    participant RoomConsumer as SoloMatchRoomReadyConsumer

    User1->>Handler: JoinQueue (gRPC Stream)
    Handler->>Redis: ZADD solo_match:queue:waiting (ticket_1:user_1)
    Handler->>Redis: SUBSCRIBE solo_match:events:ticket_1

    User2->>Handler: JoinQueue (gRPC Stream)
    Handler->>Redis: ZADD solo_match:queue:waiting (ticket_2:user_2)
    Handler->>Redis: SUBSCRIBE solo_match:events:ticket_2

    Note over Worker,Redis: Worker phát hiện >= 2 vé khác user_id
    Worker->>Redis: EVAL Lua Script (Bốc 2 vé nguyên tử)
    Redis-->>Worker: Trả về {ticket_1, ticket_2}
    Worker->>DB: Lưu SoloMatch (status=PENDING_READY, ready_deadline=now+60s)
    Worker->>DB: Ghi Outbox event SoloMatchStarted
    Worker->>Redis: PUBLISH solo_match:events:ticket_1 (matchID)
    Worker->>Redis: PUBLISH solo_match:events:ticket_2 (matchID)

    Handler-->>User1: Stream Event: MATCHED (SoloMatchView)
    Handler-->>User2: Stream Event: MATCHED (SoloMatchView)

    Note over RoomConsumer,DB: Khi cả 2 người chơi vào phòng camera tại workout_execution
    RoomConsumer->>RoomConsumer: Nhận CloudEvent SoloMatchRoomReady
    RoomConsumer->>DB: ActivateRoom() -> Chuyển PENDING_READY sang IN_PROGRESS
    Note over DB: started_at = now
```

---

### 3.2. Luồng Bot Fallback Sau 5 Phút

```mermaid
sequenceDiagram
    autonumber
    actor User as Gymer
    participant Handler as SoloMatch gRPC Handler
    participant Redis as Redis (ZSet & Pub/Sub)
    participant Worker as Matchmaker Worker
    participant DB as PostgreSQL (solo_match)

    User->>Handler: JoinQueue (Chờ trong hàng đợi > 5 phút)
    Note over Worker,Redis: Worker quét vé có score <= now - 5m
    Worker->>Redis: EVAL Lua Script (Bốc 1 vé quá hạn)
    Redis-->>Worker: Trả về ticket của User
    Worker->>DB: Bốc ngẫu nhiên Bot ID từ solo_match.system_bots (In-Memory Cache O(1))
    Worker->>DB: Lưu SoloMatch (User vs Bot, status=PENDING_READY, ready_deadline=now+60s)
    Worker->>DB: Ghi Outbox event SoloMatchStarted (botUserId != "")
    Worker->>Redis: PUBLISH solo_match:events:ticket (matchID)
    Handler-->>User: Stream Event: MATCHED (Thông tin trận đấu, ẩn danh tính Bot)
```

---

### 3.3. Luồng Quá Hạn 60s Điểm Danh Không Vào Phòng (Check-in No-Show)

```mermaid
sequenceDiagram
    autonumber
    participant TimeoutWorker as MatchTimeoutWorker
    participant DB as PostgreSQL (solo_match)
    participant OutboxPub as OutboxPublisherWorker
    participant Kafka as Kafka Event Bus

    Note over DB: Trận đấu ở PENDING_READY, ready_deadline = T + 60s
    Note over TimeoutWorker: Worker định kỳ 15s quét ready_deadline
    TimeoutWorker->>DB: FindUnreadyExpiredMatches(now)
    DB-->>TimeoutWorker: Danh sách trận quá hạn

    loop Duyệt từng trận
        TimeoutWorker->>DB: CancelOnNoShow(now)
        Note over DB: status chuyển sang CANCELLED, cả 2 bên bị FORFEITED
        TimeoutWorker->>DB: Ghi Outbox event SoloMatchCancelled (reason: READY_TIMEOUT)
    end

    OutboxPub->>DB: ClaimBatch FOR UPDATE SKIP LOCKED
    OutboxPub->>Kafka: Xuất bản CloudEvent SoloMatchCancelled
    Note over Kafka: Module workout_execution nhận event để dọn phòng Redis
```

---

### 3.4. Luồng Phân Định Thắng Thua Khi Cả Hai Cùng Hoàn Thành

```mermaid
sequenceDiagram
    autonumber
    participant Kafka as Kafka Event Bus
    participant Consumer as WorkoutCompletedConsumer
    participant DB as PostgreSQL (solo_match)
    participant Resolver as MatchResolver

    Kafka->>Consumer: Nhận SoloTrackingCompleted (P1, score=25)
    Consumer->>DB: Kiểm tra Idempotency qua outbox_logs
    Consumer->>DB: RecordParticipantResult(P1) -> P1 completed
    Note over Consumer: P2 chưa hoàn thành -> Chờ tiếp

    Kafka->>Consumer: Nhận SoloTrackingCompleted (P2, score=20)
    Consumer->>DB: RecordParticipantResult(P2) -> Cả P1 và P2 đã xong!

    Consumer->>Resolver: ResolveOutcome(REPS, P1, P2)
    Note over Resolver: So sánh 25 > 20 -> Winner: P1 (WIN), P2 (LOSE)
    Consumer->>DB: Cập nhật SoloMatch (status=COMPLETED, winner_id=P1)
    Consumer->>DB: Ghi Outbox event SoloMatchCompleted
```

---

### 3.5. Luồng Xử Thua Tức Thì Khi Có Bên Bỏ Cuộc (Fast Forfeit)

```mermaid
sequenceDiagram
    autonumber
    participant Kafka as Kafka Event Bus
    participant Consumer as WorkoutAbortedConsumer
    participant DB as PostgreSQL (solo_match)

    Kafka->>Consumer: Nhận SoloTrackingAborted (P1, reason: USER_FORFEITED)
    Consumer->>DB: Kiểm tra Idempotency qua outbox_logs
    Consumer->>DB: RecordParticipantAbortHandler(P1)
    Note over DB: match.ForfeitParticipant(P1)
    Note over DB: P1 = FORFEITED, P2 = WIN, status = FORFEITED, winner_id = P2
    Consumer->>DB: Ghi Outbox SoloMatchForfeited (P1 forfeited)
    Consumer->>DB: Ghi Outbox SoloMatchCompleted (winner_id = P2)
    Note over Kafka: Các module Gamification/Profile nhận event để trao thưởng cho P2
```

---

### 3.6. Luồng Lưới An Toàn Cưỡng Chế Trận Treo Quá 35 Phút

```mermaid
sequenceDiagram
    autonumber
    participant TimeoutWorker as MatchTimeoutWorker
    participant DB as PostgreSQL (solo_match)

    Note over DB: Trận đấu ở IN_PROGRESS kéo dài > 35 phút
    TimeoutWorker->>DB: FindExpiredInProgressMatches(startedBefore = now - 35m)
    DB-->>TimeoutWorker: Danh sách các trận treo

    loop Duyệt từng trận treo
        alt Trường hợp 1: P1 đã xong, P2 treo
            TimeoutWorker->>DB: TimeoutInProgress() -> P1 WIN, P2 FORFEITED, status FORFEITED
            TimeoutWorker->>DB: Ghi Outbox SoloMatchForfeited + SoloMatchCompleted
        else Trường hợp 2: Cả hai bên cùng treo
            TimeoutWorker->>DB: TimeoutInProgress() -> Cả 2 FORFEITED, status CANCELLED
            TimeoutWorker->>DB: Ghi Outbox SoloMatchCancelled
        end
    end
```

---

## 4. Phân Tích Sự Kiện Liên Module (Event Storming)

### 4.1. Sự Kiện Xuất Bản (Outbound CloudEvents từ `solo_match`)

| Tên sự kiện | CloudEvent Type | Thời điểm phát sinh | Đích đến |
| :--- | :--- | :--- | :--- |
| **`SoloMatchStarted`** | `contracts.core.solo_match.v1.soloMatchStarted` | Ngay khi tạo xong trận đấu (`PENDING_READY`) | `workout_execution` (chuẩn bị phòng & khởi chạy Bot nếu có) |
| **`SoloMatchCancelled`** | `contracts.core.solo_match.v1.soloMatchCancelled` | Khi quá 60s không vào phòng hoặc cả 2 cùng treo quá 35m | `workout_execution` (dọn phòng Redis & hủy bot runner) |
| **`SoloMatchCompleted`** | `contracts.core.solo_match.v1.soloMatchCompleted` | Khi có kết quả chung cuộc (Thắng, Thua, Hòa) | `gamification`, `social`, `coaching` |
| **`SoloMatchForfeited`** | `contracts.core.solo_match.v1.soloMatchForfeited` | Khi có người chơi bị xử thua do bỏ cuộc hoặc timeout | `gamification`, `profile` |

### 4.2. Sự Kiện Lắng Nghe (Inbound CloudEvents tới `solo_match`)

| Tên sự kiện | CloudEvent Type | Nguồn gửi | Hành động tương ứng của `solo_match` |
| :--- | :--- | :--- | :--- |
| **`SoloMatchRoomReady`** | `contracts.core.workout_execution.v1.soloMatchRoomReady` | `workout_execution` | Kích hoạt trận đấu: chuyển từ `PENDING_READY` sang `IN_PROGRESS`. |
| **`SoloTrackingCompleted`** | `contracts.core.workout_execution.v1.soloTrackingCompleted` | `workout_execution` | Ghi nhận điểm số thành tích của người chơi và phân định thắng thua khi đủ 2 bên. |
| **`SoloTrackingAborted`** | `contracts.core.workout_execution.v1.soloTrackingAborted` | `workout_execution` | Xử thua tức thì cho bên bỏ cuộc, trao chiến thắng cho đối thủ còn lại. |
| **`UserRegistered`** | `contracts.generic.auth.v1.userRegistered` | `auth` | Tạo bản ghi định danh ban đầu trong `solo_match.user_snapshots`. |
| **`UserIdentityUpdated`** | `contracts.supporting.profile.v1.userIdentityUpdated` | `profile` | Cập nhật `display_name` và `avatar_url` trong `user_snapshots`. |

---

## 5. Máy Trạng Thái Hữu Hạn Của Trận Đấu (`SoloMatch.Status`)

```mermaid
stateDiagram-v2
    [*] --> PENDING_READY: Khởi tạo trận (ready_deadline = now + 60s)

    PENDING_READY --> IN_PROGRESS: Nhận SoloMatchRoomReady (Cả 2 đã vào phòng)
    PENDING_READY --> CANCELLED: Quá 60s không vào phòng (CancelOnNoShow)

    IN_PROGRESS --> COMPLETED: Cả 2 nộp điểm hợp lệ (MatchResolver)
    IN_PROGRESS --> FORFEITED: Có 1 bên bỏ cuộc / đứt mạng quá hạn / timeout 35m
    IN_PROGRESS --> CANCELLED: Cả 2 cùng treo quá 35m không ai nộp bài

    COMPLETED --> [*]
    FORFEITED --> [*]
    CANCELLED --> [*]

    note right of CANCELLED
        Quy tắc trạng thái hữu hạn (Terminal State Rule):
        Một khi trận đã CANCELLED, COMPLETED hoặc FORFEITED,
        tuyệt đối KHÔNG BAO GIỜ được kích hoạt lại.
    end note
```
