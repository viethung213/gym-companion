# Phân Tích Hệ Thống — Module Solo Match (System Analysis)

Tài liệu này trình bày phân tích ca sử dụng (Use Cases), tác nhân (Actors), dữ liệu khởi tạo trận & các cổng tích hợp đồng bộ (Match Data Provisioning), phân tích sự kiện (Event Storming), sơ đồ tuần tự (Sequence Diagrams) và máy trạng thái (State Machine) của Bounded Context **Solo Match**.

---

## 1. Tác Nhân Hệ Thống (Actors)

| Tác nhân | Phân loại | Vai trò trong hệ thống |
| :--- | :--- | :--- |
| **Gym User (Player)** | Con người (User) | Người dùng đăng ký tìm trận, hủy tìm trận, thực hiện bài tập đối kháng, xem kết quả và xem lịch sử thi đấu. |
| **System Bot (AI Opponent)** | Tác nhân hệ thống | Tài khoản gymer mẫu cấu hình sẵn trong DB (`solo_match.system_bots`), bốc ngẫu nhiên qua In-Memory Cache tham gia trận đấu khi thời gian chờ của người dùng vượt quá 5 phút. |
| **Redis Matchmaker Engine** | Hạ tầng thời gian thực | Quản lý vé chờ (ZSet), phân giải vé nguyên tử bằng Lua script, và phát thông báo qua Redis Pub/Sub. |
| **Match Timeout Worker** | Tác nhân nền (Daemon) | Worker chạy ngầm định kỳ quét và xử lý các trận đấu `IN_PROGRESS` đã vượt quá mốc `timeout_at`. |
| **Exercise Management Service** | Module bên ngoài / Port | Cung cấp danh mục bài tập thi đấu đối kháng ngẫu nhiên (`ExerciseID`, `ExerciseName`, `ChallengeType` REPS/DURATION, thời lượng `Duration`) qua cổng `port.ExerciseProvider`. |
| **Workout History / Execution Service** | Module bên ngoài / Port | Cung cấp kỷ lục cá nhân (Personal Record - PR) của người dùng qua cổng `port.UserPRProvider` để tính toán độ khó của Bot. |
| **Workout Execution Module** | Module bên ngoài (Event) | Đơn vị tracking camera AI, phát sự kiện hoàn thành (`workoutSessionCompleted`) hoặc hủy bỏ (`workoutSessionAborted`). |
| **Auth & Identity Module** | Module bên ngoài (Event) | Phát sự kiện `UserRegistered` trên topic `auth.events` khi người dùng đăng ký tài khoản mới. |
| **Profile Module** | Module bên ngoài (Event) | Phát sự kiện `UserIdentityUpdated` trên topic `profile.events` khi người dùng cập nhật thông tin cá nhân (họ tên, avatar). |

---

## 2. Biểu Đồ Ca Sử Dụng (Use Case Diagram)

```mermaid
flowchart LR
    User([Gym User])
    TimeoutWorker([Timeout Daemon Worker])
    ExerciseMod([Exercise Service])
    WorkoutQuery([Workout History / Service])
    WorkoutMod([Workout Execution])
    AuthMod([Auth / Identity])
    ProfileMod([Profile Module])

    subgraph Solo_Match_Bounded_Context [Module Solo Match]
        UC1[Vào hàng đợi tìm trận\nJoin Queue via gRPC Stream]
        UC2[Hủy tìm trận\nCancel Queue]
        UC3[Xem chi tiết trận đấu\nGet Match by ID]
        UC4[Lấy trận đang diễn ra\nGet Active Match]
        UC5[Xem lịch sử đấu\nGet Match History]
        
        UC6[Ghép cặp nguyên tử 2 User\nLua Script Atomic Match]
        UC7[Kích hoạt Bot Fallback 5 phút\nBốc Random Bot DB & Tính PR]
        UC8[Tiếp nhận kết quả bài tập\nRecord Participant Result]
        UC9[Xử thua tức thì khi đối thủ thoát\nFast Forfeit via Workout Aborted]
        UC10[Quét xử lý trận quá hạn chót\nHandle Expired Matches]
        UC11[Đồng bộ User Snapshot\nSync User Identity]

        UC12[Lấy thử thách bài tập ngẫu nhiên\nFetch Random Exercise Challenge]
        UC13[Truy vấn kỷ lục cá nhân Gymer\nFetch User Best PR]
    end

    User --> UC1
    User --> UC2
    User --> UC3
    User --> UC4
    User --> UC5

    UC1 -.-> UC6
    UC1 -.-> UC7

    UC6 -.-> UC12
    UC7 -.-> UC12
    UC7 -.-> UC13

    ExerciseMod -.->|Sync Port: GetRandomChallenge| UC12
    WorkoutQuery -.->|Sync Port: GetBestPR| UC13

    WorkoutMod -->|Kafka: workoutSessionCompleted| UC8
    WorkoutMod -->|Kafka: workoutSessionAborted| UC9
    TimeoutWorker --> UC10
    AuthMod -->|Kafka: UserRegistered| UC11
    ProfileMod -->|Kafka: UserIdentityUpdated| UC11
```

---

## 3. Phân Tích Dữ Liệu Khởi Tạo Trận & Các Cổng Tích Hợp Đồng Bộ (Match Data Provisioning)

Bên cạnh việc lắng nghe các sự kiện bất đồng bộ qua Kafka, việc khởi tạo một trận đấu (cả PvP lẫn Bot) đòi hỏi quá trình tổng hợp dữ liệu đồng bộ từ các dịch vụ bên ngoài thông qua các cổng **Outbound Ports**:

```mermaid
flowchart TD
    subgraph Match_Creation_Engine [Động Cơ Tạo Trận Đấu]
        M1[Bốc 2 vé hoặc 1 vé quá 5 phút]
        M2[Tổng hợp dữ liệu khởi tạo trận]
        M3[Khởi tạo Aggregate SoloMatch]
    end

    subgraph Outbound_Ports [Các Cổng Tích Hợp Đồng Bộ]
        P_Ex[ExerciseProvider: GetRandomChallenge]
        P_PR[UserPRProvider: GetBestPR]
        P_Snap[UserSnapshotRepository: FindByID]
        P_Bot[BotRepository: GetRandomBotUserID]
    end

    subgraph Data_Sources [Nguồn Dữ Liệu Cung Cấp]
        S_Ex[Exercise Catalog Service: Bài tập, ChallengeType, Thời lượng]
        S_Work[Workout History Service: Kỷ lục cá nhân PR của User]
        S_Snap[(solo_match.user_snapshots: Tên & Avatar)]
        S_Bot[(solo_match.system_bots: Danh sách Bot)]
    end

    M1 --> M2
    M2 -->|Gọi đồng bộ| P_Ex --> S_Ex
    M2 -->|Ghép Bot: Gọi đồng bộ| P_PR --> S_Work
    M2 -->|Đọc danh tính đối thủ| P_Snap --> S_Snap
    M2 -->|Ghép Bot: Lấy ID Bot| P_Bot --> S_Bot

    M2 --> M3
```

### 3.1. Danh mục thông tin cần thu thập để tạo trận đấu
Để khởi tạo thành công một Aggregate Root `SoloMatch`, hệ thống bắt buộc phải thu thập đủ 4 nhóm dữ liệu cốt lõi:
1. **Thông tin thử thách bài tập (`ExerciseChallenge`)**:
   - `ExerciseID` & `ExerciseName`: Mã và tên bài tập đối kháng (ví dụ: `ex_pushup` - `Push-Up`).
   - `ChallengeType`: Hình thức thi đấu (`REPS` - đếm số lượt, hoặc `DURATION` - giữ thời gian tư thế).
   - `Duration`: Thời lượng tối đa cho phép làm bài (ví dụ: 15 phút), dùng để tính mốc `timeout_at = now + duration`.
2. **Kỷ lục cá nhân Gymer (`UserPR`) - Phục vụ riêng cho trận đấu với Bot**:
   - Kỷ lục thành tích cao nhất của người dùng đối với bài tập được chọn (`*float64`).
3. **Danh tính người chơi (`User Identity Snapshot`)**:
   - `DisplayName` và `AvatarURL` của cả 2 đấu thủ (lấy từ bảng cô lập `solo_match.user_snapshots`) để phản hồi ngay lập tức cho gRPC stream mà không cần query chéo DB.
4. **Tài khoản Bot hệ thống (`System Bot`) - Phục vụ riêng cho Bot Fallback**:
   - `BotUserID`: Được bốc ngẫu nhiên từ In-Memory Cache $O(1)$ của bảng `solo_match.system_bots`.

---

### 3.2. Cổng cung cấp thử thách bài tập (`port.ExerciseProvider`)
- **Giao diện Port**:
  ```go
  type ExerciseProvider interface {
      GetRandomChallenge(ctx context.Context) (*ExerciseChallenge, error)
  }
  ```
- **Vai trò**:
  - Chọn ngẫu nhiên một bài tập trong danh mục thi đấu để hai đấu thủ tranh tài công bằng.
  - Cung cấp luật chấm điểm (`ChallengeType`) và hạn chót thi đấu (`Duration`).
- **Triển khai kiến trúc**:
  - Môi trường phát triển & kiểm thử: Sử dụng `StubExerciseClient` (mặc định thử thách `Push-Up`, `ChallengeTypeReps`, thời lượng 15 phút).
  - Môi trường Production: Kết nối qua gRPC/HTTP Client gọi sang module `exercise`.

---

### 3.3. Cổng truy vấn PR & Chiến lược cân bằng sức mạnh Bot (`port.UserPRProvider`)
- **Giao diện Port**:
  ```go
  type UserPRProvider interface {
      GetBestPR(ctx context.Context, userID, exerciseID string) (*float64, error)
  }
  ```
- **Thời điểm kích hoạt**:
  - **Chỉ kích hoạt khi người dùng chờ quá 5 phút** và chuyển sang ghép cặp với Bot.
- **Chiến lược tính toán điểm số Bot (`BotBenchmarkCalculator`)**:
  - **Trường hợp 1 (Người dùng đã có PR)**:
    - Bot sao chép chỉ số `botScore = *userPR`.
    - **Ý nghĩa nghiệp vụ**: Tạo ra trận đấu "đối đầu với kỷ lục tốt nhất của chính mình" (Ghost Runner), vừa tạo động lực vượt ngưỡng vừa đảm bảo trận đấu có tính cạnh tranh cao.
  - **Trường hợp 2 (Người dùng mới chưa có PR hoặc lỗi mạng kết nối)**:
    - Tự động fallback về điểm chuẩn mặc định của hệ thống (`DefaultBenchmark`):
      + Bài tập `REPS`: **5 reps**.
      + Bài tập `DURATION`: **300 giây (5 phút)**.
  - **Đặc điểm trận đấu Bot**: Kết quả của Bot được chốt ngay tại thời điểm tạo trận (`COMPLETED` với điểm `botScore`), còn Gymer ở trạng thái `PENDING` và biết trước mục tiêu cần đánh bại.

---

## 4. Phân Tích Sự Kiện (Event Storming & Event Integration)

Module `solo_match` là điểm giao thoa giữa giao tiếp đồng bộ thời gian thực (In-Memory Redis + gRPC Stream) và giao tiếp bất đồng bộ chịu lỗi cao (Transactional Outbox + Kafka CloudEvents):

```mermaid
flowchart TD
    subgraph External_Inbound_Events [Sự Kiện Tiếp Nhận Từ Kafka]
        E_Reg[auth.v1.userRegistered]
        E_Ident[profile.v1.userIdentityUpdated]
        E_WorkDone[workout_execution.v1.workoutSessionCompleted]
        E_WorkAbort[workout_execution.v1.workoutSessionAborted]
    end

    subgraph Solo_Match_Processing [Xử Lý Nội Bộ Module]
        C_Sync[UserSnapshot Repository: Cập nhật tên & Avatar]
        C_Record[Match Aggregate: RecordParticipantResult & ResolveOutcome]
        C_Forfeit[Match Aggregate: ForfeitParticipant & Fast Settle]
    end

    subgraph Solo_Match_Outbound_Events [Sự Kiện Xuất Bản Qua Outbox Kafka]
        E_Started[contracts.core.solo_match.v1.soloMatchStarted]
        E_Completed[contracts.core.solo_match.v1.soloMatchCompleted]
        E_Forfeited[contracts.core.solo_match.v1.soloMatchForfeited]
    end

    E_Reg --> C_Sync
    E_Ident --> C_Sync
    E_WorkDone --> C_Record
    E_WorkAbort --> C_Forfeit

    C_Record -->|Tạo trận xong| E_Started
    C_Record -->|Phân định thắng/thua/hòa| E_Completed
    C_Forfeit -->|Đối thủ bỏ cuộc| E_Forfeited
    C_Forfeit -->|Đồng thời chốt kết quả thắng| E_Completed
```

### 4.1. Danh mục CloudEvents Tiếp Nhận (Inbound)
1. `contracts.core.workout_execution.v1.workoutSessionCompleted`:
   - Mang thông tin buổi tập đã hoàn thành của người dùng, bao gồm tổng số rep hoặc thời gian thực hiện (`score_value` hoặc `total_sets`).
   - Consumer đối chiếu trận đấu `IN_PROGRESS` của `user_id` để ghi nhận thành tích.
2. `contracts.core.workout_execution.v1.workoutSessionAborted`:
   - Mang thông báo người chơi đã bỏ cuộc hoặc bị ngắt kết nối khi đang tập.
   - Kích hoạt luồng **Fast Forfeit**, xử thắng ngay cho đối thủ còn lại.
3. `contracts.generic.auth.v1.event.UserRegistered` (Module `auth`, Topic: `auth.events`):
   - Phát ra khi người dùng đăng ký tài khoản mới, mang `userId`, `displayName`, `avatarUrl` khởi tạo bản ghi trong `solo_match.user_snapshots`.
4. `contracts.supporting.profile.v1.event.UserIdentityUpdated` (Module `profile`, Topic: `profile.events`):
   - Phát ra khi người dùng chỉnh sửa hồ sơ cá nhân (đổi tên hiển thị `fullName`, cập nhật ảnh đại diện `avatarUrl`).
   - Consumer `UserIdentityUpdatedConsumer` tự động `Upsert` cập nhật lại bản ghi trong bảng cô lập `solo_match.user_snapshots`.

### 4.2. Danh mục CloudEvents Xuất Bản (Outbound qua Transactional Outbox)
1. `contracts.core.solo_match.v1.soloMatchCompleted`:
   - Phát ra khi trận đấu kết thúc chính thức (cả 2 bên hoàn thành, hoặc sau khi xử Forfeit).
   - Payload chứa: `matchId`, `winnerId`, `isDraw`, `exerciseId`, `botUserId`, điểm số của từng bên.
   - Phục vụ module Gamification cộng cúp, tính điểm xếp hạng Elo, và gửi push notification chúc mừng.
2. `contracts.core.solo_match.v1.soloMatchForfeited`:
   - Phát ra khi có bên bị xử thua do bỏ cuộc (`workoutSessionAborted`) hoặc quá hạn chót (`timeout_at`).
   - Chứa `forfeitedUserId`, `opponentUserId`, `botUserId`.
*(Lưu ý: Sự kiện `soloMatchStarted` được chuẩn bị sẵn trong contract/packer, nhưng trong luồng tạo trận hiện tại thông báo bắt đầu trận được đẩy trực tiếp thời gian thực qua Redis Pub/Sub đến gRPC Server-Streaming cho 2 client).*

---

## 5. Sơ Đồ Tuần Tự (Sequence Diagrams)

### 5.1. Luồng Tìm Trận Thời Gian Thực & Ghép Cặp 2 Người Chơi (P2P Matchmaking)

Cơ chế điều phối Worker hoạt động theo mô hình **Hybrid Trigger (Kích hoạt kép)**:
1. **Event-driven Trigger (Tức thì khi tạo vé)**: Khi API tiếp nhận lệnh `JoinQueue` và `ZADD` vé vào Redis, API gửi tín hiệu nội bộ qua Go channel `triggerCh <- struct{}{}` để đánh thức Matchmaker Worker chạy ghép trận ngay tức thì (Zero-latency).
2. **Periodic Ticker (Định kỳ mỗi 250ms)**: Ticker `time.NewTicker(pollInterval)` (cấu hình mặc định 250 miligiây trong `module.go`) định kỳ quét lại để chống sót vé (phòng trường hợp race condition hoặc channel buffer đầy) và quét vé phục vụ bot fallback.

```mermaid
sequenceDiagram
    autonumber
    actor User1 as Gymer A
    actor User2 as Gymer B
    participant API as SoloMatch gRPC Service
    participant Redis as Redis In-Memory (ZSet + PubSub)
    participant Worker as Matchmaker Worker (workerLoop)
    participant DB as PostgreSQL (solo_match.*)

    Note over Worker: Worker chạy nền liên tục (Lắng nghe triggerCh & Ticker định kỳ 250ms)

    User1->>API: JoinQueue (gRPC Stream)
    API->>Redis: ZADD solo_match:queue:waiting (Ticket A, Score=Now)
    API->>Worker: Gửi tín hiệu đánh thức (triggerCh <- struct{})
    API->>Redis: SUBSCRIBE solo_match:events:ticket_A (Ngủ chờ)
    
    Worker->>Redis: EVAL Lua Script (Kiểm tra cặp >= 2 vé)
    Redis-->>Worker: Trả về nil (Mới chỉ có 1 vé A, tiếp tục chờ)

    User2->>API: JoinQueue (gRPC Stream)
    API->>Redis: ZADD solo_match:queue:waiting (Ticket B, Score=Now)
    API->>Worker: Gửi tín hiệu đánh thức (triggerCh <- struct{})
    API->>Redis: SUBSCRIBE solo_match:events:ticket_B (Ngủ chờ)

    Worker->>Redis: EVAL Lua Script (Bốc 2 vé đầu tiên)
    Redis-->>Worker: Trả về [Ticket A, Ticket B] nguyên tử (ZREM cả 2 vé)
    
    Worker->>DB: INSERT solo_matches & match_participants
    
    Worker->>Redis: PUBLISH solo_match:events:ticket_A (MatchID)
    Worker->>Redis: PUBLISH solo_match:events:ticket_B (MatchID)
    
    Redis-->>API: Đánh thức stream của User A
    API-->>User1: Push MatchFound (MatchID, Opponent Info, Exercise, Timeout)
    
    Redis-->>API: Đánh thức stream của User B
    API-->>User2: Push MatchFound (MatchID, Opponent Info, Exercise, Timeout)

    Note over Worker: [Ticker định kỳ 250ms]: Chạy quét dọn vé sót lại & kiểm tra Bot fallback
```

---

### 5.2. Luồng Ghép Trận Với Bot Sau 5 Phút (Bot Fallback)

Khi người dùng đợi mà không có đối thủ thực tế, nhịp định kỳ của Matchmaker Worker sẽ phát hiện và chuyển hướng sang Bot:

```mermaid
sequenceDiagram
    autonumber
    actor User as Gymer A
    participant API as SoloMatch gRPC Service
    participant Redis as Redis ZSet
    participant BotWorker as Matchmaker Worker (Ticker 250ms)
    participant BotRepo as GORMBotRepository
    participant PRProvider as UserPRProvider
    participant BotCalc as BotBenchmarkCalculator
    participant DB as PostgreSQL
    
    User->>API: JoinQueue (gRPC Stream)
    API->>Redis: ZADD solo_match:queue:waiting (Score = JoinedTime)
    
    Note over User,Redis: Chờ đối thủ thực tế... (Gymer A chờ trong hàng đợi)
    Note over BotWorker: Ticker định kỳ mỗi 250ms chạy workerLoop -> fallbackBots()
    
    BotWorker->>Redis: EVAL Lua (ZRANGEBYSCORE <= Now - 5 phút & ZREM)
    Redis-->>BotWorker: Trả về TicketID & UserID của Gymer A (Vé đã chờ >= 5 phút)
    
    BotWorker->>BotRepo: GetRandomBotUserID(ctx) (Bốc từ In-Memory Cache O(1))
    BotRepo-->>BotWorker: Trả về BotUserID từ bảng solo_match.system_bots
    
    BotWorker->>PRProvider: GetBestPR(ctx, UserID, ExerciseID)
    PRProvider-->>BotWorker: userPR
    
    BotWorker->>BotCalc: CalculateBotScore(ChallengeType, userPR)
    BotCalc-->>BotWorker: botScore (= userPR hoặc fallback 5 reps / 300s)
    
    BotWorker->>DB: INSERT solo_matches (Gymer A vs Bot)
    BotWorker->>DB: INSERT match_participants (User A: Pending, Bot: Completed sẵn với botScore)
    
    BotWorker->>Redis: PUBLISH solo_match:events:ticket_A (MatchID)
    Redis-->>API: Đánh thức stream của Gymer A
    API-->>User: Push MatchFound (Hiện đối thủ từ user_snapshots như người thật)
```

---

### 5.3. Luồng Xử Thua Tức Thì Khi Bỏ Cuộc (Fast Forfeit)

```mermaid
sequenceDiagram
    autonumber
    actor User1 as Gymer A (Bỏ cuộc)
    actor User2 as Gymer B (Đang tập)
    participant Workout as Workout Execution
    participant Kafka as Kafka Event Bus
    participant Consumer as WorkoutAbortedConsumer
    participant MatchApp as RecordParticipantResultHandler
    participant DB as PostgreSQL
    participant Outbox as Transactional Outbox

    User1->>Workout: Bấm "Thoát trận / Hủy buổi tập"
    Workout->>Kafka: Phát sự kiện WorkoutSessionAborted (UserID_A)
    
    Kafka->>Consumer: WorkoutAbortedConsumer nhận thông điệp
    Consumer->>MatchApp: FastForfeitMatchCommand (UserID_A)
    
    MatchApp->>DB: Tìm trận active của UserID_A (IN_PROGRESS)
    MatchApp->>DB: UPDATE User A -> FORFEITED
    MatchApp->>DB: UPDATE User B -> WIN, Match Status -> COMPLETED
    MatchApp->>Outbox: Lưu SoloMatchForfeited & SoloMatchCompleted (Winner: B)
    
    Note over User2: Gymer B được thông báo chiến thắng ngay lập tức
```

---

### 5.4. Luồng Đồng Bộ User Snapshot Khi Đổi Tên / Avatar (Profile Synchronizer)

```mermaid
sequenceDiagram
    autonumber
    actor User as Người dùng
    participant Profile as Profile Module
    participant Kafka as Kafka (profile.events)
    participant Consumer as UserIdentityUpdatedConsumer
    participant Handler as SyncUserSnapshotHandler
    participant DB as PostgreSQL (solo_match.user_snapshots)

    User->>Profile: Cập nhật thông tin (Họ tên mới, Avatar mới)
    Profile->>Kafka: Phát sự kiện UserIdentityUpdated (CloudEvent)
    
    Kafka->>Consumer: Consumer nhận message từ group solo_match_profile_consumer
    Consumer->>Consumer: Kiểm tra tính lũy đẳng (outbox_log.IsProcessed)
    Consumer->>Handler: SyncUserSnapshotCommand (UserID, FullName, AvatarURL)
    
    Handler->>DB: UPSERT solo_match.user_snapshots ON CONFLICT (user_id) DO UPDATE
    DB-->>Handler: Thành công
    Consumer->>Consumer: Đánh dấu event PROCESSED vào outbox_log
```

---

## 6. Máy Trạng Thái Trận Đấu (Match State Machine)

Trạng thái của Aggregate Root `SoloMatch` được bảo vệ nghiêm ngặt qua các quy tắc nghiệp vụ:

```mermaid
stateDiagram-v2
    [*] --> IN_PROGRESS: Tạo trận đấu thành công (P2P hoặc Bot)

    IN_PROGRESS --> COMPLETED: Cả 2 bên hoàn thành nộp bài (Phân định Win/Lose/Draw)
    IN_PROGRESS --> FORFEITED: Một bên bỏ cuộc (Fast Forfeit) hoặc quá hạn (Timeout)
    IN_PROGRESS --> CANCELLED: Cả 2 bên đều không nộp bài khi hết giờ (AFK/No-show)

    FORFEITED --> COMPLETED: Xử thắng cho bên còn lại & trao thưởng

    COMPLETED --> [*]
    CANCELLED --> [*]
```

- **Tính bất biến (Immutability)**: Khi trận đấu đã chuyển sang `COMPLETED`, `FORFEITED` hoặc `CANCELLED`, mọi yêu cầu ghi nhận điểm số tiếp theo đều bị chặn với lỗi `ErrMatchNotActive` hoặc `ErrParticipantFinished`.
