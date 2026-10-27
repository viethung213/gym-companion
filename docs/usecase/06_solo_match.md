# UC-06 Solo Match (Ghép Trận & Quản Lý Đối Kháng 1v1)

> Nguồn: [BRD](../NGHIEP_VU_COT_LOI_BABOK.md) · [Bounded Context](../02_bounded_context.md) · [Tactical Design](../03_ddd_tactical_design.md) · [Solo Match Docs](../../internal/solo_match/docs/README.md)

**Actor**: `User` (Gymer) · `System` (Redis Matchmaker Engine / MatchTimeoutWorker / WorkoutCompletedConsumer / WorkoutAbortedConsumer)  
**Format mỗi Use Case**: Precondition → Main Flow → Alternative Flow → Error / Edge Cases → Postcondition → Domain Events

---

### UC-06.1 JoinAndMatchmake (Vào Hàng Đợi & Ghép Cặp)

| | |
|---|---|
| **Actor** | User |
| **Precondition** | User đã đăng nhập hợp lệ. User không có trận đấu nào đang dở dang (`PENDING_READY` hoặc `IN_PROGRESS`). User chưa có vé nào đang chờ trong hàng đợi. |

**Main Flow (Ghép cặp Người vs Người - PvP)**
1. User nhấn "Tìm trận 1v1", Client mở kết nối gRPC / ConnectRPC Server-Streaming `JoinQueue`.
2. System sinh vé `MatchmakingTicket`, mở kênh Redis Pub/Sub `solo_match:events:{ticketID}`, và đẩy vé vào Redis Sorted Set `solo_match:queue:waiting`.
3. System gửi sự kiện định kỳ `MATCH_STREAM_EVENT_TYPE_QUEUED` kèm thời gian chờ (`elapsedSeconds`) mỗi 1 giây.
4. Worker phát hiện có $\ge 2$ vé có `user_id` khác nhau, chạy Lua script nguyên tử rút 2 vé ra khỏi hàng đợi.
5. System lấy một bài tập ngẫu nhiên từ `ExerciseProvider` (`REPS` hoặc `DURATION`).
6. System tạo thực thể `SoloMatch` ở trạng thái `PENDING_READY`, gán mốc hạn chót vào phòng `ready_deadline = now + 60s`, lưu vào PostgreSQL.
7. System ghi sự kiện `SoloMatchStarted` vào Outbox.
8. System gửi `matchID` qua kênh Redis Pub/Sub của cả 2 vé.
9. Stream của Client nhận được `matchID`, gửi sự kiện `MATCH_STREAM_EVENT_TYPE_MATCHED` kèm thông tin chi tiết trận đấu (`SoloMatchView`) và kết thúc luồng stream tìm trận.

**Alternative Flow**
- **A1: Ghép cặp với Bot sau 5 phút (Bot Fallback)**:
  - Nếu sau 5 phút (300 giây) không tìm thấy người chơi khác, Worker bốc 1 vé quá hạn bằng Lua script.
  - System bốc ngẫu nhiên một tài khoản Bot từ bảng `solo_match.system_bots` thông qua In-Memory Cache ($O(1)$).
  - System tạo `SoloMatch` (`PENDING_READY`, `ready_deadline = now + 60s`), lưu DB, ghi `SoloMatchStarted` (có `botUserId`) vào Outbox và báo về Client qua Pub/Sub.
  - Màn hình Client hiển thị danh tính Bot như người thật (ẩn danh tính AI).
- **A2: Người dùng chủ động hủy tìm trận**:
  - User nhấn nút "Hủy tìm trận", Client gọi RPC `CancelQueue` gửi kèm `ticketID`.
  - System xóa vé khỏi Redis ZSet (`ZREM`) và đóng luồng stream an toàn.
- **A3: Người dùng mất mạng hoặc thoát app khi đang chờ**:
  - Luồng gRPC stream bị ngắt (`ctx.Done()`), hàm defer tự động gọi thu hồi vé khỏi Redis ZSet.

**Error / Edge Cases**
- **E1: Đang có trận dở dang**: System từ chối với lỗi `ErrActiveMatchAlreadyExists` (`FAILED_PRECONDITION`).
- **E2: Đã có vé trong hàng đợi**: System từ chối với lỗi `ErrUserAlreadyInQueue` (`FAILED_PRECONDITION`).
- **E3: Lỗi khởi tạo trận đấu (Lỗi DB / Provider)**: System gửi `ERROR:<msg>` qua Pub/Sub, stream Client nhận `MATCH_STREAM_EVENT_TYPE_CANCELLED` và yêu cầu thử lại.

**Postcondition**: `SoloMatch` được tạo với trạng thái `PENDING_READY`. Vé tìm trận được dọn sạch khỏi Redis.  
**Domain Events**: `SoloMatchStarted`

---

### UC-06.2 ActivateMatchOnRoomReady (Kích Hoạt Trận Đấu)

| | |
|---|---|
| **Actor** | System (`SoloMatchRoomReadyConsumer`) |
| **Precondition** | Nhận được sự kiện `SoloMatchRoomReady` từ module `workout_execution`. |

**Main Flow**
1. Consumer `SoloMatchRoomReadyConsumer` nhận sự kiện `SoloMatchRoomReady` từ Kafka.
2. System kiểm tra tính lũy đẳng qua `outbox_logs`.
3. System tìm trận đấu `SoloMatch` theo `matchID`:
   - Nếu trận đang ở trạng thái `PENDING_READY`:
     - System gọi `match.ActivateRoom(startedAt)`.
     - Trạng thái chuyển sang `IN_PROGRESS`, cập nhật `started_at = startedAt`.
     - Lưu trạng thái mới vào PostgreSQL.

**Error / Edge Cases**
- **E1: Trận đấu đã bị CANCELLED do quá hạn điểm danh 60s**: System từ chối kích hoạt (`ErrMatchNotEligibleForActivation`), bỏ qua sự kiện để đảm bảo tính bất biến của trạng thái kết thúc (Terminal State Rule).
- **E2: Trận đấu không tồn tại**: Ghi log cảnh báo và bỏ qua.

**Postcondition**: `SoloMatch` chuyển sang trạng thái `IN_PROGRESS`.  
**Domain Events**: —

---

### UC-06.3 ResolveMatchOutcome (Phân Định Thắng Thua & Xử Thua)

| | |
|---|---|
| **Actor** | System (`WorkoutCompletedConsumer` · `WorkoutAbortedConsumer` · `MatchResolver`) |
| **Precondition** | Nhận được sự kiện `SoloTrackingCompleted` hoặc `SoloTrackingAborted` từ `workout_execution`. |

**Main Flow (Phân định thắng thua bình thường)**
1. Consumer `WorkoutCompletedConsumer` nhận sự kiện `SoloTrackingCompleted`.
2. System kiểm tra tính lũy đẳng qua `outbox_logs`.
3. System ghi nhận điểm số của người tham gia qua `RecordParticipantResult`.
4. Nếu cả 2 bên tham gia đều đã nộp kết quả (`IsCompleted() == true`):
   - `MatchResolver` so sánh điểm số hai bên:
     - `Score1 > Score2`: P1 Thắng (`WIN`), P2 Thua (`LOSE`).
     - `Score2 > Score1`: P2 Thắng (`WIN`), P1 Thua (`LOSE`).
     - `Score1 == Score2`: Kết quả Hòa (`DRAW`), `winner_id = NULL`, `is_draw = true`.
   - Trận đấu chuyển sang trạng thái `COMPLETED`, cập nhật `ended_at = now`.
   - System ghi sự kiện `SoloMatchCompleted` vào Outbox để Kafka phân phối tới Gamification, Streak, Social.

**Alternative Flow (Xử thua tức thì - Fast Forfeit)**
- **A1: Đối thủ bỏ cuộc hoặc quá hạn 45s mất mạng**:
  - Consumer `WorkoutAbortedConsumer` nhận sự kiện `SoloTrackingAborted`.
  - System gọi `RecordParticipantAbortHandler`:
    - Bên bỏ cuộc bị gán `FORFEITED`.
    - Đối thủ còn lại được xử Thắng (`WIN`).
    - Trận đấu chuyển sang `FORFEITED`, `winner_id = đối thủ`.
    - Ghi đồng thời 2 sự kiện vào Outbox: `SoloMatchForfeited` và `SoloMatchCompleted`.
    - Đối thủ nhận thông báo chiến thắng ngay tức khắc.

**Error / Edge Cases**
- **E1: Trùng lặp sự kiện (Duplicate Event)**: Bảng `outbox_logs` phát hiện `event_id` đã xử lý, bỏ qua an toàn (Idempotent).
- **E2: Trận đấu không ở trạng thái IN_PROGRESS**: Bỏ qua để bảo vệ tính toàn vẹn trạng thái.

**Postcondition**: `SoloMatch` chuyển sang `COMPLETED` hoặc `FORFEITED`. Điểm và kết quả từng bên được lưu vĩnh viễn trong Database.  
**Domain Events**: `SoloMatchCompleted` | `SoloMatchForfeited`

---

### UC-06.4 SweepMatchTimeout (Quét Dọn & Lưới An Toàn Timeout)

| | |
|---|---|
| **Actor** | System (`MatchTimeoutWorker`) |
| **Precondition** | Worker chạy ngầm định kỳ mỗi 15 giây. |

**Main Flow (Xử lý No-Show quá hạn 60 giây vào phòng)**
1. Worker quét các trận `status = PENDING_READY` có `now >= ready_deadline`.
2. System gọi `match.CancelOnNoShow(now)`:
   - Trận đấu chuyển sang `CANCELLED`.
   - Cả 2 bên tham gia đều bị gán kết quả `FORFEITED`.
   - Cập nhật `ended_at = now`.
3. System ghi sự kiện `SoloMatchCancelled` (lý do: `READY_TIMEOUT`) vào Outbox.
4. Kafka chuyển sự kiện tới `workout_execution` để giải phóng phòng LiveSync và Bot runner.

**Alternative Flow (Lưới an toàn cưỡng chế trận treo quá 35 phút)**
- **A1: Trận IN_PROGRESS bắt đầu trước `now - 35m`**:
  - Worker gọi `match.TimeoutInProgress(now)`:
    - *Nếu 1 bên đã nộp, 1 bên treo*: Bên nộp thắng (`WIN`), bên treo thua (`FORFEITED`), trạng thái sang `FORFEITED`, phát `SoloMatchForfeited` + `SoloMatchCompleted`.
    - *Nếu cả 2 bên cùng treo*: Cả 2 bị `FORFEITED`, trạng thái trận sang `CANCELLED`, phát `SoloMatchCancelled`.

**Postcondition**: Toàn bộ trận đấu quá hạn hoặc bị treo được đóng dứt điểm, giải phóng tài nguyên hệ thống.  
**Domain Events**: `SoloMatchCancelled` | `SoloMatchForfeited` | `SoloMatchCompleted`
