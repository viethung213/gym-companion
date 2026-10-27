# Thiết Kế Kỹ Thuật — Thi Đấu Đối Kháng Thời Gian Thực Trong Workout Execution (Solo Tracking)

Tài liệu này đặc tả chi tiết toàn bộ kiến trúc kỹ thuật, luồng xử lý thời gian thực, trọng tài ảo AI, cơ chế mô phỏng Bot và đồng bộ trực tiếp hai chiều thuộc phạm vi trách nhiệm độc quyền của Bounded Context **Workout Execution**.

> [!IMPORTANT]
> **Ranh giới Bounded Context**:
> - **Module `workout_execution` (Tài liệu này)**: Chịu trách nhiệm về luồng stream hai chiều gRPC / ConnectRPC (`StreamSoloMatchTracking`), nhận diện khung xương AI (MediaPipe Landmarks), trọng tài ảo (`SoloRefereeSession`), máy trạng thái đếm Reps 3 pha tuần tự, quy tắc xử phạt 5 giây sai form đối với Duration, điều phối Bot ảo thời gian thực (`SoloBotManager`), cơ chế ân hạn mất kết nối 45 giây và đồng bộ tiến độ thời gian thực qua Redis Live Sync.
> - **Module `solo_match`**: Chịu trách nhiệm về hàng đợi tìm trận, ghép cặp (Matchmaking) và phân định thắng/thua chung cuộc. Xem chi tiết tại [**Tài liệu Solo Match**](../../solo_match/docs/README.md).

---

## 1. Kiến Trúc & Cấu Trúc Mã Nguồn Phân Hệ Solo Trong Workout Execution

Các thành phần phục vụ luồng Solo Tracking được tổ chức chặt chẽ theo kiến trúc Hexagonal bên trong `/internal/workout_execution/`:

```
internal/workout_execution/
├── domain/
│   ├── service/
│   │   ├── solo_referee.go          # Trọng tài ảo: Máy trạng thái 3 pha Reps, vi phạm 5s Duration, trần thời gian
│   │   └── solo_referee_test.go     # Unit tests bao phủ toàn bộ logic trọng tài
│   └── event/
│       └── events.go                # Domain Events: SoloMatchRoomReady, SoloTrackingCompleted, SoloTrackingAborted
│
├── application/
│   ├── command/
│   │   ├── solo_tracking_service.go # Điều phối session, 45s grace period, Reconnect restore progress
│   │   ├── solo_tracking_room.go    # Kiểm tra phòng đủ 2 người & phát sự kiện SoloMatchRoomReady
│   │   └── solo_tracking_persistence.go # Quản lý bản ghi WorkoutSession trong PostgreSQL
│   └── port/
│       ├── solo_match_live_sync.go  # Interface đồng bộ Redis Live Sync (Hash + Pub/Sub)
│       └── solo_bot_manager.go      # Interface điều phối Bot ảo
│
├── infrastructure/
│   ├── sync/
│   │   ├── redis_live_sync.go       # Triển khai Redis Live Sync (Hash phòng & Pub/Sub tiến độ)
│   │   └── redis_live_sync_test.go  # Kiểm thử đồng bộ tiến độ
│   └── worker/
│       ├── solo_bot_manager.go      # Virtual Bot Runner mô phỏng người thật theo PR benchmark
│       └── solo_bot_manager_test.go # Kiểm thử vòng đời Bot ảo
│
└── transport/
    ├── grpc/
    │   ├── solo_stream_runner.go    # Single Source of Truth stream runner cho cả gRPC và ConnectRPC
    │   └── solo_stream_test.go      # Kiểm thử luồng stream thời gian thực
    └── consumer/
        ├── solo_match_started_consumer.go   # Lắng nghe SoloMatchStarted để kích hoạt Bot runner
        └── solo_match_cancelled_consumer.go # Lắng nghe SoloMatchCancelled để dọn dẹp phòng Redis
```

---

## 2. Toàn Bộ Quy Trình Thực Thi Của Workout Execution Trong Luồng Solo

```mermaid
flowchart TD
    subgraph Init [1. Khởi Tạo Phòng & Chuẩn Bị]
        E1[SoloMatchStarted từ solo_match] --> E2{Có botUserId?}
        E2 -->|Có Bot| B1[SoloBotManager.StartBot: Đăng ký Bot status WAITING]
        E2 -->|Không có Bot| B2[Chờ 2 người chơi vào]
        
        U1[Client gửi StreamSoloMatchTracking] --> U2[InitializeSession: Nạp Rule, Lưu DB IN_PROGRESS]
        U2 --> U3[RegisterParticipant vào Redis Hash]
        U3 --> U4{Phòng đã đủ 2 người?}
        U4 -->|Chưa đủ| U5[Trọng tài status = WAITING\nBáo: Đang chờ đối thủ vào phòng]
        U4 -->|Đủ 2 người| U6[Phát SoloMatchRoomReady vào Outbox\nKích hoạt cả 2 bên sang ACTIVE]
    end

    subgraph Streaming [2. Thi Đấu Camera & Live Sync Thời Gian Thực]
        U6 --> F1[Client gửi Landmarks khung hình]
        F1 --> F2[RuleEvaluator: Đánh giá Form theo góc quay camera]
        F2 --> F3[SoloRefereeSession.ProcessFrame]
        
        F3 --> F4{Thể thức thi đấu?}
        F4 -->|REPS| R1[Máy trạng thái 3 pha: REST -> TRANSITION -> ACTIVE -> TRANSITION -> REST]
        F4 -->|DURATION| D1[Tích lũy giây đúng form\nNếu sai form liên tục >= 5s: Phạt SPECTATING]
        
        R1 --> P1[PublishMyProgress lên Redis Pub/Sub]
        D1 --> P1
        P1 --> P2[Đọc tiến độ đối thủ từ LiveSync trả về Frame Response]
    end

    subgraph BotLifecycle [3. Mô Phỏng Bot Ảo (SoloBotManager)]
        B1 --> B3[Lắng nghe User vào phòng]
        B3 -->|User vào ACTIVE| B4[Chuyển Bot sang ACTIVE]
        B4 --> B5[Đếm nhịp: 2.5s/rep hoặc 1.0s/giây\nPublish tiến độ đều đặn lên Redis]
        B5 -->|Đạt botScore| B6[Bot status COMPLETED\nPhát SoloTrackingCompleted]
        B5 -->|User bỏ cuộc ABORTED| B7[Bot hoàn thành sớm & dừng runner]
    end

    subgraph Terminate [4. Kết Thúc & Xử Lý Ngoại Lệ]
        F3 --> T1{Tình huống kết thúc?}
        T1 -->|Bấm Hoàn thành Submit| T2[Chuyển SPECTATING\nLưu DB COMPLETED\nPhát SoloTrackingCompleted]
        T1 -->|Hết giờ trần Ceiling| T3[Cưỡng chế TIMEOUT\nLưu DB COMPLETED\nPhát SoloTrackingCompleted]
        T1 -->|Bấm Bỏ cuộc Abort| T4[Lưu DB ABORTED\nPhát SoloTrackingAborted]
        T1 -->|Rớt mạng đột ngột| T5[Ân hạn 45 giây: Reconnect khôi phục tiến độ\nQuá 45s: Tự động phát SoloTrackingAborted]
    end
```

---

## 3. Chi Tiết Các Thành Phần Cốt Lõi

### 3.1. Trọng Tài Ảo Thời Gian Thực (`SoloRefereeSession`)

Trọng tài ảo độc lập cho từng người tham gia trong luồng stream:
- **Kiểm soát thời gian trần (`timeCeiling`)**:
  - Mặc định 10 phút đối với bài `REPS`, 30 phút đối với bài `DURATION` (hoặc cấu hình theo `time_ceiling_seconds`).
  - Hệ thống tính toán dựa trên cả thời gian tích lũy khung hình và đồng hồ thực tế (`wallElapsed`). Khi chạm trần, tự động cưỡng chế kết thúc với lý do `TIMEOUT`.

#### A. Thể thức Đếm Reps (`ChallengeTypeReps`) — Máy Trạng Thái 3 Pha Tuần Tự
Để triệt tiêu tình trạng rung lắc camera hoặc ăn gian nhấp nhả góc khớp:
```
           Checkpoint 1              Checkpoint 2               Checkpoint 3
[ REST ] ───────────────> [ TRANSITION ] ───────────────> [ ACTIVE ] ───────────────> [ TRANSITION ] ──> [ REST ]
         (hasTransitionIn)               (hasReachedActive)          (hasTransitionOut)                   (Cộng 1 Rep)
```
1. `hasTransitionedIn`: Bắt đầu hạ người từ vùng nghỉ (`REST`) vào vùng chuyển động (`TRANSITION`).
2. `hasReachedActive`: Đạt đến đỉnh co cơ tối đa quy định của bài tập (`ACTIVE`).
3. `hasTransitionedOut`: Thực hiện pha đẩy ngược trở về có kiểm soát (`TRANSITION`).
4. Khi quay về chạm `REST`: Chỉ khi **đủ cả 3 checkpoint tuần tự** thì `confirmedValue++`. Sau đó các cờ được reset về `false`.
5. Mọi hành vi nhảy cóc (từ `REST` nhảy thẳng sang `ACTIVE` hoặc từ `ACTIVE` nhảy thẳng về `REST`) đều bị từ chối và không tính rep.

#### B. Thể thức Giữ Thời Gian (`ChallengeTypeDuration`) — Xử Phạt Vi Phạm Form 5 Giây
- **Tập đúng Form**: Thời gian giữ tư thế được tích lũy liên tục vào `confirmedValue` (số giây), biến đếm vi phạm liên tục `consecutiveViolationDuration` được reset về 0.
- **Tập sai Form (Lệch hông, sụp lưng, sai góc)**: Tạm dừng tích lũy thời gian thi đấu, bắt đầu đếm thời gian vi phạm liên tục.
- **Xử phạt vi phạm 5 giây liên tục (`FORM_VIOLATION_5S`)**:
  - Nếu người chơi sai tư thế liên tục $\ge 5$ giây:
  - Trọng tài tự động ngắt phần thi: `isCompleted = true`, lý do `FORM_VIOLATION_5S`.
  - Chuyển người chơi sang trạng thái **Khán giả (`SPECTATING`)**, dừng đồng hồ và chuyển màn hình sang chế độ theo dõi đối thủ.

---

### 3.2. Mô Phỏng Bot Ảo Thời Gian Thực (`SoloBotManager`)

`SoloBotManager` giải quyết bài toán thi đấu với Bot chân thật như người:
1. **Điểm mục tiêu động (`resolveBotScore`)**:
   - Truy vấn kỷ lục cá nhân (PR) của người chơi đối với bài tập đó: `prRepo.FindByUserIDAndExerciseID(userPlayerID, exerciseID)`.
   - Nếu có PR: Bot lấy chính điểm số PR này làm đích đến (`botScore = userPR`), tạo cảm giác như người chơi đang thi đấu với chính phiên bản xuất sắc nhất của mình.
   - Nếu chưa có PR: Đặt mặc định 5 reps hoặc 300 giây (5 phút).
2. **Vòng đời 4 giai đoạn của Virtual Bot Runner**:
   - *Giai đoạn 1 (Chờ đối thủ)*: Đăng ký Bot vào Redis Live Sync ở trạng thái `WAITING` (`value = 0`).
   - *Giai đoạn 2 (Kích hoạt)*: Khi người thật vào phòng (`ACTIVE` hoặc `WAITING`), Bot lập tức chuyển sang `ACTIVE`.
   - *Giai đoạn 3 (Đua nhịp độ thực tế)*:
     - Bài Reps: Cứ mỗi **2.5 giây** đếm 1 rep.
     - Bài Duration: Cứ mỗi **1.0 giây** đếm 1 giây.
     - Mỗi bước nhảy, Bot gọi `PublishMyProgress` phát cập nhật lên Redis Pub/Sub. Người chơi nhìn thấy chỉ số của Bot tăng đều đặn trên màn hình đối thủ.
   - *Giai đoạn 4 (Hoàn thành)*: Khi đạt `botScore`, Bot chuyển sang `COMPLETED`, phát sự kiện `SoloTrackingCompleted` vào Transactional Outbox.
3. **Phản hồi linh hoạt theo người chơi**:
   - Nếu người chơi bỏ cuộc (`ABORTED`): Bot lập tức kết thúc sớm và dừng runner.
   - Nếu người chơi mất mạng (`DISCONNECTED`): Bot vẫn duy trì bài tập bình thường trong 45 giây ân hạn.

---

### 3.3. Cơ Chế Mất Kết Nối, Ân Hạn 45 Giây & Khôi Phục Trạng Thái (Reconnect Recovery)

Khi luồng gRPC stream bị ngắt đột ngột (rớt mạng Wi-Fi/4G, switch app, reload trang):
1. **Ghi nhận mất kết nối**:
   - Hàm `HandleParticipantDisconnect` cập nhật trạng thái trên Redis Live Sync thành `DISCONNECTED` kèm timestamp mất kết nối.
   - Khởi động một Goroutine Timer **45 giây** chạy ngầm.
2. **Nhánh Reconnect thành công (trong vòng 45 giây)**:
   - Khi người chơi mở lại ứng dụng và kết nối lại stream:
   - `InitializeSession` phát hiện timer ân hạn cũ $\rightarrow$ Hủy timer ngay lập tức.
   - Đọc tiến độ cũ từ Redis Live Sync: `GetMyProgress(matchID, userID)`.
   - Gọi `referee.RestoreProgress(prevProg.Value, prevProg.FormScore)` để khôi phục số rep và điểm form.
   - Đăng ký lại trạng thái `ACTIVE` trên Redis, người chơi tiếp tục bài tập từ mốc cũ.
3. **Nhánh Hết 45 giây không kết nối lại**:
   - Timer ân hạn phát nổ sau 45 giây.
   - Tự động gọi `FinalizeSession("USER_DISCONNECTED")`.
   - Đánh dấu bản ghi `WorkoutSession` trong PostgreSQL là `ABORTED`.
   - Phát sự kiện `SoloTrackingAborted` vào Outbox để `solo_match` xử thua và trao chiến thắng cho đối thủ.

---

### 3.4. Cơ Chế Phòng Đấu & Đồng Bộ Trực Tiếp Trên Redis (`RedisLiveSync`)

Hạ tầng Live Sync lưu trữ trạng thái phòng đấu hoàn toàn trên RAM Redis:
- **Hash phòng**: `solo:match:{matchID}:room`
  - Field: `{userID}` $\rightarrow$ Giá trị: JSON `ParticipantProgress` (`value`, `formScore`, `status`, `disconnectedAt`).
  - Cho phép kiểm tra nhanh `IsMatchReady` (phòng đủ 2 người) trong $O(1)$.
- **Pub/Sub tiến độ**: `solo:match:{matchID}:progress`
  - Kênh phát sóng tiến độ hai chiều: Mỗi khi Client nộp frame, điểm số mới được publish lên kênh này.
  - Luồng stream của đối thủ subscribe kênh và đẩy ngay thông tin đối thủ về màn hình Client.
- **Dọn dẹp phòng (`DelRoom`)**: Khi trận đấu kết thúc hoặc bị hủy, xóa toàn bộ key của phòng trên Redis.

---

## 4. Danh Mục Sự Kiện Outbox Phát Ra Từ Workout Execution

| Tên sự kiện | Type định danh CloudEvent | Thời điểm kích hoạt | Ý nghĩa đối với Solo Match |
| :--- | :--- | :--- | :--- |
| **`SoloMatchRoomReady`** | `contracts.core.workout_execution.v1.soloMatchRoomReady` | Khi cả 2 người chơi (hoặc 1 người + Bot) đều đã kết nối vào phòng | Đánh thức `solo_match` chuyển trạng thái từ `PENDING_READY` sang `IN_PROGRESS`. |
| **`SoloTrackingCompleted`** | `contracts.core.workout_execution.v1.soloTrackingCompleted` | Khi một người chơi hoặc Bot hoàn thành bài tập | Gửi điểm số thành tích cuối cùng sang `solo_match` để chấm điểm và phân định thắng thua. |
| **`SoloTrackingAborted`** | `contracts.core.workout_execution.v1.soloTrackingAborted` | Khi người chơi chủ động bỏ cuộc hoặc hết 45s mất mạng | Thông báo cho `solo_match` xử thua tức thì cho người bỏ cuộc và xử thắng cho đối thủ. |

