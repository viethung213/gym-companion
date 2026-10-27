# Đặc Tả Yêu Cầu Nghiệp Vụ — Bounded Context Solo Match (Business Requirements)

Tài liệu này định nghĩa toàn bộ mục tiêu, phạm vi, ranh giới trách nhiệm, các kịch bản và cách xử lý nghiệp vụ thuộc trách nhiệm độc quyền của Bounded Context **Solo Match** (Ghép trận và Quản lý kết quả đối kháng 1v1).

> [!IMPORTANT]
> **Ranh giới Bounded Context (Domain Boundary)**:
> - **Module `solo_match` (Tài liệu này)**: Chịu trách nhiệm về hàng đợi tìm trận, ghép cặp (PvP / Bot), quản lý vòng đời trận đấu (`PENDING_READY`, `IN_PROGRESS`, `COMPLETED`, `FORFEITED`, `CANCELLED`), kiểm soát hạn chót điểm danh 60 giây (`ready_deadline`), phân định thắng/thua/hòa từ điểm số nộp về, và xử lý vi phạm/bỏ cuộc.
> - **Module `workout_execution`**: Chịu trách nhiệm về việc mở camera AI, tracking khung xương, trọng tài số chấm điểm, đếm Rep/Giây, mô phỏng Bot ảo thời gian thực, và đồng bộ tiến độ trực tiếp giữa hai bên. Xem chi tiết tại [**Tài liệu Solo Tracking của Workout Execution**](../../workout_execution/docs/solo_tracking_design.md).

---

## 1. Giới Thiệu & Mục Tiêu Nghiệp Vụ Của Solo Match

Module **Solo Match** đóng vai trò là cơ quan điều phối thi đấu thể thao điện tử (Matchmaker & Tournament Authority):
1. **Tìm trận siêu tốc & Không nghẽn I/O (Pure Redis Matchmaker)**: Quản lý hàng đợi và phân giải vé tìm trận hoàn toàn trên RAM Redis (ZSet FIFO + Lua Script), giảm tải ghi đĩa PostgreSQL về 0 khi đang tìm trận.
2. **Không để người dùng chờ đợi vô hạn (Bot Fallback sau 5 phút)**: Nếu sau 5 phút trong hàng đợi không có người chơi thật, tự động bốc ngẫu nhiên một tài khoản Bot có sẵn trong Database (`solo_match.system_bots`) để tạo trận.
3. **Quản lý vòng đời trận đấu minh bạch & Chặt chẽ**:
   - Trận đấu được tạo ở trạng thái `PENDING_READY` với thời hạn điểm danh vào phòng: `ready_deadline = now + 60s`.
   - Chuyển sang `IN_PROGRESS` khi nhận được thông báo phòng thi đấu đã sẵn sàng (`SoloMatchRoomReady`).
   - Nếu quá 60 giây không vào phòng, tự động hủy trận đấu (`CANCELLED`) để giải phóng tài nguyên.
4. **Phân định kết quả công bằng (Match Outcome Resolution)**:
   - Tiếp nhận điểm số cuối cùng được chứng thực bởi AI từ `workout_execution` để xác định người chiến thắng (`WIN`), người thua (`LOSE`) hoặc hòa (`DRAW`).
   - Xử lý thua tức thì (`Fast Forfeit`) khi có người chơi bỏ cuộc hoặc mất kết nối quá thời gian quy định.
5. **Độc lập ranh giới (Domain Isolation)**:
   - Module `solo_match` không can thiệp vào camera, không xử lý tọa độ skeleton, không đếm rep trực tiếp.
   - Giao tiếp với `workout_execution` hoàn toàn bất đồng bộ thông qua chuẩn **CloudEvents 1.0** trên nền **Kafka Transactional Outbox**.

---

## 2. Các Giai Đoạn Nghiệp Vụ Của Module Solo Match

```mermaid
flowchart TD
    subgraph S1 [1. Hàng Đợi & Ghép Trận (Matchmaking)]
        Q1[User gửi JoinQueue] --> Q2{Tìm thấy đối thủ trong <= 5m?}
        Q2 -->|Có User khác| M1[Ghép cặp PvP: Bốc 2 vé nguyên tử bằng Lua]
        Q2 -->|Hết 5 phút| M2[Bot Fallback: Bốc 1 vé lẻ & Bốc Bot từ DB]
        Q2 -->|User hủy/ngắt mạng| C1[Hủy vé & Rút khỏi ZSet an toàn]
    end

    subgraph S2 [2. Khởi Tạo & Kích Hoạt Trận Đấu]
        M1 --> R1[Tạo SoloMatch: PENDING_READY\nready_deadline = now + 60s]
        M2 --> R1
        R1 --> R2[Ghi Outbox event: SoloMatchStarted]
        R2 --> R3{Nhận SoloMatchRoomReady trong 60s?}
        R3 -->|Nhận sự kiện| R4[ActivateRoom: Chuyển sang IN_PROGRESS]
        R3 -->|Quá 60s không nhận| R5[CancelOnNoShow: Chuyển sang CANCELLED\nPhát SoloMatchCancelled]
    end

    subgraph S3 [3. Phân Định Kết Quả Trận Đấu]
        R4 --> O1{Nhận sự kiện từ workout_execution}
        O1 -->|SoloTrackingCompleted| O2{Đã đủ 2 bên nộp bài?}
        O2 -->|Đủ 2 bên| O3[MatchResolver: So sánh điểm số\nChuyển COMPLETED\nPhát SoloMatchCompleted]
        O2 -->|Mới có 1 bên| O4[Ghi nhận điểm số, chờ bên còn lại]
        O1 -->|SoloTrackingAborted| O5[Fast Forfeit: Xử thua bên bỏ cuộc\nĐối thủ WIN\nChuyển FORFEITED\nPhát SoloMatchForfeited + SoloMatchCompleted]
        O1 -->|Treo quá 35 phút| O6[Safety Net Sweep: Xử lý trận treo\nPhát Forfeited hoặc Cancelled]
    end
```

---

## 3. Chi Tiết Từng Phần, Từng Nhánh Rẽ Thuộc Solo Match

### 3.1. Hàng Đợi Tìm Trận (Matchmaking Queue)

#### Nhánh 1.1: Gia nhập hàng đợi (`JoinQueue`)
- **Điều kiện tiên quyết**:
  - Người dùng có JWT hợp lệ.
  - **Active Match Guard**: Không có trận đấu nào đang dở dang (`PENDING_READY` hoặc `IN_PROGRESS`). Nếu có $\rightarrow$ Trả về lỗi `ErrActiveMatchAlreadyExists` (`FAILED_PRECONDITION`).
  - **Queue Duplicate Guard**: Không có vé nào đang chờ trong hàng đợi Redis $\rightarrow$ Trả về lỗi `ErrUserAlreadyInQueue` (`FAILED_PRECONDITION`).
- **Thực thi**:
  - Sinh mã vé `ticketID = "ticket_" + UUID`.
  - Mở kênh nhận thông báo Redis Pub/Sub: `solo_match:events:{ticketID}`.
  - Thêm vé vào Redis Sorted Set `solo_match:queue:waiting` với `score = timestamp hiện tại`.
  - Giữ luồng gRPC Server-Streaming mở, gửi sự kiện `MATCH_STREAM_EVENT_TYPE_QUEUED` kèm thời gian chờ (`elapsedSeconds`) cập nhật mỗi 1 giây.

#### Nhánh 1.2: Người dùng chủ động hủy tìm trận (`CancelQueue`)
- Client gọi RPC `CancelQueue(ticketID)`.
- Hệ thống xóa vé khỏi Redis ZSet (`ZREM`) và đóng luồng stream an toàn.

#### Nhánh 1.3: Người dùng ngắt kết nối mạng hoặc thoát app khi đang tìm trận
- Kết nối gRPC Server-Streaming bị đứt $\rightarrow$ `ctx.Done()` kích hoạt.
- Khối `defer` gọi `CancelMatchmakingHandler` tự động xóa vé khỏi Redis ZSet, tránh tồn đọng vé "ma".

#### Nhánh 1.4: Ghép cặp Người vs Người (PvP Pairing)
- Worker chạy Lua script nguyên tử:
  - Bốc 2 vé đầu tiên trong ZSet có `user_id` khác nhau và xóa ngay khỏi ZSet.
  - Lấy bài tập ngẫu nhiên từ `ExerciseProvider` (`REPS` hoặc `DURATION`).
  - Khởi tạo thực thể `SoloMatch`: `status = PENDING_READY`, `ready_deadline = now + 60s`.
  - Lưu vào PostgreSQL (`solo_match.solo_matches` và `solo_match.match_participants`).
  - Ghi sự kiện `SoloMatchStarted` vào Outbox (`player1Id`, `player2Id`, `botUserId = ""`).
  - Gửi `matchID` qua Pub/Sub cho 2 vé. Luồng stream gửi `MATCH_STREAM_EVENT_TYPE_MATCHED` về cho cả 2 người dùng.

#### Nhánh 1.5: Bot Fallback sau 5 phút
- Sau 5 phút, nếu vé có `score <= now - 5m`:
  - Lua script rút 1 vé quá hạn ra khỏi ZSet.
  - Bốc ngẫu nhiên 1 tài khoản Bot từ `solo_match.system_bots` qua In-Memory Cache ($O(1)$).
  - Khởi tạo `SoloMatch`: `status = PENDING_READY`, `ready_deadline = now + 60s`.
  - Ghi sự kiện `SoloMatchStarted` vào Outbox với trường `botUserId` được điền rõ ràng.
  - Báo `matchID` qua Pub/Sub cho người dùng.
  - Phía Client nhận thông tin hiển thị của Bot từ `user_snapshots` (ẩn hoàn toàn danh tính AI, không có cờ `is_bot`).

#### Nhánh 1.6: Lỗi khi khởi tạo trận
- Nếu việc lấy bài tập hoặc ghi Database thất bại:
  - Gửi `ERROR:<msg>` qua Pub/Sub tới vé liên quan.
  - Stream gửi `MATCH_STREAM_EVENT_TYPE_CANCELLED` kèm thông báo lỗi.

---

### 3.2. Quản Lý Vòng Đời & Kích Hoạt Trận Đấu

#### Nhánh 2.1: Kích hoạt trận đấu chính thức (`SoloMatchRoomReady`)
- Module `workout_execution` sau khi kiểm tra phòng camera đã đủ 2 người sẽ phát sự kiện `SoloMatchRoomReady`.
- Consumer `SoloMatchRoomReadyConsumer` nhận sự kiện:
  - Gọi method `match.ActivateRoom(startedAt)`: Chuyển trạng thái từ `PENDING_READY` sang `IN_PROGRESS`, gán `started_at = now`.
  - Lưu trận đấu vào PostgreSQL.
  - **Quy tắc trạng thái hữu hạn (Terminal State Rule)**: Nếu trận đấu đã bị `CANCELLED`, `COMPLETED` hoặc `FORFEITED` $\rightarrow$ Tuyệt đối không bao giờ kích hoạt lại (`ErrMatchNotEligibleForActivation`).

#### Nhánh 2.2: Quá hạn 60 giây không vào phòng (Check-in No-Show Timeout)
- **Bối cảnh**: Sau khi ghép trận thành công, một hoặc cả hai người chơi không chịu mở màn hình thi đấu, tắt app hoặc mất mạng.
- **Xử lý**:
  - `MatchTimeoutWorker` định kỳ 15 giây quét các trận `status = PENDING_READY` có `now >= ready_deadline`.
  - Gọi method `match.CancelOnNoShow(now)`:
    - Trận đấu chuyển sang `CANCELLED`.
    - Cả hai bên tham gia đều bị gán `result = FORFEITED`.
    - Cập nhật `ended_at = now`.
  - Ghi sự kiện `SoloMatchCancelled` (lý do: `READY_TIMEOUT`) vào Outbox.
  - Sự kiện này được Kafka chuyển tới `workout_execution` để dọn sạch phòng Redis và hủy Bot runner.

---

### 3.3. Phân Định Kết Quả Trận Đấu (Outcome Resolution)

#### Nhánh 3.1: Tiếp nhận kết quả thi đấu (`SoloTrackingCompleted`)
- Khi một bên hoàn thành bài tập, `workout_execution` phát sự kiện `SoloTrackingCompleted`.
- Consumer `WorkoutCompletedConsumer` trong `solo_match` nhận sự kiện:
  - Kiểm tra tính lũy đẳng (Idempotency) qua bảng `solo_match.outbox_logs`.
  - Gọi `RecordParticipantResult(userID, scoreValue, completedAt)`:
    - Ghi nhận điểm số của người chơi đó trong bảng `match_participants`.
    - **Nếu cả 2 bên đều đã nộp kết quả (`IsCompleted() == true`)**:
      - `MatchResolver` so sánh điểm số hai bên:
        - `Score1 > Score2`: P1 Thắng (`WIN`), P2 Thua (`LOSE`).
        - `Score2 > Score1`: P2 Thắng (`WIN`), P1 Thua (`LOSE`).
        - `Score1 == Score2`: Kết quả **HÒA (`DRAW`)**, `winner_id = NULL`, `is_draw = true`.
      - Trận đấu chuyển sang trạng thái `COMPLETED`, cập nhật `ended_at = now`.
      - Ghi sự kiện `SoloMatchCompleted` vào Outbox để phân phối tiếp tới các phân hệ Gamification (thưởng XP, nâng huy hiệu), Streak và Social.

#### Nhánh 3.2: Xử thua tức thì khi đối thủ bỏ cuộc (`SoloTrackingAborted`)
- Khi một bên chủ động bấm bỏ cuộc, hoặc bị mất kết nối quá thời gian ân hạn, `workout_execution` phát sự kiện `SoloTrackingAborted`.
- Consumer `WorkoutAbortedConsumer` trong `solo_match` nhận sự kiện:
  - Kiểm tra tính lũy đẳng qua `outbox_logs`.
  - Gọi `RecordParticipantAbortHandler`:
    - Bên bỏ cuộc bị gán `result = FORFEITED`.
    - Đối thủ còn lại được xử Thắng (`result = WIN`).
    - Trận đấu chuyển sang trạng thái `FORFEITED`, `winner_id = đối thủ`.
    - Ghi đồng thời 2 sự kiện vào Outbox:
      1. `SoloMatchForfeited`: Ghi nhận lý do và người vi phạm.
      2. `SoloMatchCompleted`: Giúp các phân hệ bên ngoài trao thưởng cho người chiến thắng ngay lập tức.

#### Nhánh 3.3: Lưới an toàn cưỡng chế trận treo quá 35 phút
- Worker định kỳ quét các trận `status = IN_PROGRESS` có `started_at <= now - 35m`:
  - Gọi `match.TimeoutInProgress(now)`:
    - *Trường hợp 1 bên đã nộp, 1 bên treo*: Bên đã nộp Thắng (`WIN`), bên treo bị xử Thua (`FORFEITED`), trạng thái sang `FORFEITED`. Phát `SoloMatchForfeited` + `SoloMatchCompleted`.
    - *Trường hợp cả hai bên cùng treo*: Cả hai bị `FORFEITED`, trạng thái chuyển sang `CANCELLED`. Phát `SoloMatchCancelled`.

---

### 3.4. Đồng Bộ Bản Sao Danh Tính (User Snapshot Synchronizer)

- Tuân thủ **Schema Isolation**: Bảng của `solo_match` tuyệt đối **không JOIN sang bảng users** của module Auth.
- Duy trì bảng cục bộ `solo_match.user_snapshots` (`user_id`, `display_name`, `avatar_url`).
- Lắng nghe sự kiện Kafka:
  - `UserRegistered` (Module `auth`): Tự động tạo bản ghi snapshot khi có tài khoản mới.
  - `UserIdentityUpdated` (Module `profile`): Cập nhật lại tên đại diện và avatar khi người dùng đổi hồ sơ.

---

## 4. Bảng Kịch Bản Nghiệp Vụ Của Module Solo Match

| Mã Kịch Bản | Ngữ Cảnh Xử Lý Tại Solo Match | Trạng Thái Trước | Trạng Thái Sau | Kết Quả Từng Bên | Sự Kiện Phát Ra |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **SM-01** | Hai gymer vào hàng đợi trong vòng 5 phút | Không có | `PENDING_READY` | P1: `PENDING`<br>P2: `PENDING` | `SoloMatchStarted` (PvP) |
| **SM-02** | Gymer đợi quá 5 phút trong hàng đợi | Không có | `PENDING_READY` | User: `PENDING`<br>Bot: `PENDING` | `SoloMatchStarted` (Bot) |
| **SM-03** | Gymer hủy tìm trận hoặc mất mạng lúc đợi | Không có | Đã xóa vé ZSet | Không phát sinh | Không có |
| **SM-04** | Nhận `SoloMatchRoomReady` trong vòng 60 giây | `PENDING_READY` | `IN_PROGRESS` | Cả hai: `PENDING` | Không có (kích hoạt nội bộ) |
| **SM-05** | Quá 60 giây không nhận `SoloMatchRoomReady` | `PENDING_READY` | `CANCELLED` | Cả hai: `FORFEITED` | `SoloMatchCancelled` (`READY_TIMEOUT`) |
| **SM-06** | Nhận đủ kết quả 2 bên (P1 điểm cao hơn) | `IN_PROGRESS` | `COMPLETED` | P1: `WIN`<br>P2: `LOSE` | `SoloMatchCompleted` (`winner = P1`) |
| **SM-07** | Nhận đủ kết quả 2 bên (Điểm bằng nhau) | `IN_PROGRESS` | `COMPLETED` | P1: `DRAW`<br>P2: `DRAW` | `SoloMatchCompleted` (`isDraw = true`) |
| **SM-08** | Nhận `SoloTrackingAborted` từ 1 bên | `IN_PROGRESS` | `FORFEITED` | Bên abort: `FORFEITED`<br>Bên kia: `WIN` | `SoloMatchForfeited` + `SoloMatchCompleted` |
| **SM-09** | Quét trận treo quá 35 phút (1 bên đã nộp) | `IN_PROGRESS` | `FORFEITED` | Bên nộp: `WIN`<br>Bên treo: `FORFEITED` | `SoloMatchForfeited` + `SoloMatchCompleted` |
| **SM-10** | Quét trận treo quá 35 phút (cả 2 chưa nộp) | `IN_PROGRESS` | `CANCELLED` | Cả hai: `FORFEITED` | `SoloMatchCancelled` |
