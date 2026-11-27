# Các Kỹ Thuật Chuyên Sâu — Bounded Context Solo Match (Technical Deep Dive)

Tài liệu này đặc tả chi tiết 7 giải pháp kỹ thuật chuyên sâu được triển khai trong Bounded Context **Solo Match**, giải thích lý do thiết kế, phân tích độ phức tạp thuật toán và cơ chế xử lý tương tranh giải quyết các bài toán kỹ thuật cốt lõi của hàng đợi và quản lý trận đấu.

---

## Mục Lục
1. [Kỹ Thuật 1: Pure Redis In-Memory Matchmaker (ZSet FIFO)](#1-kỹ-thuật-1-pure-redis-in-memory-matchmaker-zset-fifo)
2. [Kỹ Thuật 2: Atomic Dequeue qua Lua Script (Chống Race Condition Khi Scale Ngang)](#2-kỹ-thuật-2-atomic-dequeue-qua-lua-script-chống-race-condition-khi-scale-ngang)
3. [Kỹ Thuật 3: Non-Blocking Stream Wakeup via Redis Pub/Sub & gRPC Server-Streaming](#3-kỹ-thuật-3-non-blocking-stream-wakeup-via-redis-pubsub--grpc-server-streaming)
4. [Kỹ Thuật 4: System Bot Pool & In-Memory Cache O(1)](#4-kỹ-thuật-4-system-bot-pool--in-memory-cache-o1)
5. [Kỹ Thuật 5: Check-in No-Show 60 Giây (`ready_deadline`) & Cancellation](#5-kỹ-thuật-5-check-in-no-show-60-giây-ready_deadline--cancellation)
6. [Kỹ Thuật 6: Kiến Trúc Xử Thua 2 Tầng (Fast Forfeit via Event + 35m Safety Net)](#6-kỹ-thuật-6-kiến-trúc-xử-thua-2-tầng-fast-forfeit-via-event--35m-safety-net)
7. [Kỹ Thuật 7: Transactional Outbox (SKIP LOCKED) & Idempotent Consumer Logging](#7-kỹ-thuật-7-transactional-outbox-skip-locked--idempotent-consumer-logging)
8. [Kỹ Thuật 8: Pessimistic Row Locking (`FOR UPDATE`) Trên Aggregate Root & Tuần Tự Hóa Hoàn Thành Trận Đấu](#8-kỹ-thuật-8-pessimistic-row-locking-for-update-trên-aggregate-root--tuần-tự-hóa-hoàn-thành-trận-đấu)

---

## 1. Kỹ Thuật 1: Pure Redis In-Memory Matchmaker (ZSet FIFO)

### 1.1. Bối cảnh & Vấn đề
Trong các hệ thống tìm trận thông thường, cách làm truyền thống là lưu vé tìm trận vào bảng PostgreSQL với câu lệnh `SELECT ... FOR UPDATE SKIP LOCKED`. Khi có hàng nghìn người cùng tìm trận, hủy trận hoặc rớt mạng:
- Tần suất ghi/xóa dữ liệu tạm thời (ephemeral data) tạo ra hàng triệu transaction ghi đĩa, làm phình to WAL logs và làm cạn kiệt Connection Pool của Database.

### 1.2. Giải pháp kiến trúc
Solo Match loại bỏ **100% việc ghi vé tạm xuống PostgreSQL**, chuyển toàn bộ vòng đời vé chờ lên RAM của Redis bằng cấu trúc dữ liệu **Sorted Set (`ZSet`)**:
- **Key**: `solo_match:queue:waiting`
- **Score**: Unix timestamp thời điểm gia nhập hàng đợi (`time.Now().UTC().Unix()`). Score đóng vai trò là chỉ số sắp xếp thời gian tự nhiên (FIFO).
- **Member**: Chuỗi định danh kết hợp `{ticketID}:{userID}`.

### 1.3. Hiệu quả
- **Tải I/O Database**: Giảm từ hàng nghìn write/giây xuống **0 write/giây**. Database chỉ được ghi đúng 1 lần khi trận đấu đã được tạo thành công (`solo_matches`).
- **Độ trễ**: Lệnh `ZADD` và `ZREM` đạt độ phức tạp $O(\log N)$, phản hồi trong **< 1ms**.

---

## 2. Kỹ Thuật 2: Atomic Dequeue qua Lua Script (Chống Race Condition Khi Scale Ngang)

### 2.1. Vấn đề tranh chấp dữ liệu khi chạy Multi-Instance
Khi triển khai nhiều instance backend song song trên Kubernetes, nếu worker dùng các lệnh Redis riêng lẻ:
1. Pod 1 đọc 2 vé A và B.
2. Pod 2 cũng đọc được 2 vé A và B trước khi Pod 1 kịp xóa.
3. Cả 2 Pod cùng tạo trận cho cặp A và B $\rightarrow$ **Ghép trùng trận (Duplicate Match)**.

### 2.2. Giải pháp Lua Script nguyên tử đơn luồng
Redis xử lý Lua script nguyên tử hoàn toàn trong 1 chu kỳ CPU của Redis:
```lua
local members = redis.call('ZRANGE', KEYS[1], 0, -1)
if #members < 2 then
    return {}
end

local first = members[1]
local first_user = string.match(first, ":([^:]+)$")

for i = 2, #members do
    local second = members[i]
    local second_user = string.match(second, ":([^:]+)$")
    if first_user ~= second_user then
        redis.call('ZREM', KEYS[1], first, second)
        return {first, second}
    end
end

return {}
```
- **Đảm bảo**: Tìm chính xác 2 vé có `user_id` khác nhau, xóa đồng thời khỏi ZSet. Một vé một khi đã bốc ra thì không Pod nào khác có thể nhìn thấy.

---

## 3. Kỹ Thuật 3: Non-Blocking Stream Wakeup via Redis Pub/Sub & gRPC Server-Streaming

### 3.1. Vấn đề của Polling
Nếu Client liên tục gửi request HTTP Polling mỗi 1 giây để hỏi "Đã có đối thủ chưa?", hàng chục nghìn request vô ích sẽ làm nghẽn CPU và mạng.

### 3.2. Giải pháp Pub/Sub đánh thức luồng gRPC
- Khi Client gọi `JoinQueue`, Server tạo kết nối gRPC Server-Streaming và đăng ký lắng nghe (Subscribe) kênh riêng: `solo_match:events:{ticketID}`.
- Goroutine phục vụ stream đi vào trạng thái chờ nhận kênh (I/O multiplexing với Go `select`), tiêu tốn **0% CPU** khi không có sự kiện.
- Ngay khi Matchmaker Worker tạo trận thành công, nó gửi lệnh `PUBLISH solo_match:events:{ticketID} {matchID}`.
- Redis lập tức đánh thức luồng stream của Client, gửi dữ liệu `SoloMatchView` và đóng kết nối tìm trận trong chưa đầy **2ms**.

---

## 4. Kỹ Thuật 4: System Bot Pool & In-Memory Cache O(1)

- Thay vì sinh tài khoản Bot ảo ngẫu nhiên on-the-fly, hệ thống quản lý một kho tài khoản Bot thật cấu hình sẵn trong bảng `solo_match.system_bots` liên kết với `solo_match.user_snapshots`.
- Repository [`GORMBotRepository`](../../infrastructure/persistence/gorm_bot_repository.go) bốc ngẫu nhiên 1 `user_id` của bot đang kích hoạt (`is_active = true`) thông qua **bộ nhớ đệm In-Memory (TTL 30 ngày)**, đạt độ trễ cực thấp $O(1)$ mà không gây tải cho PostgreSQL.
- Khi tạo trận, trường `botUserId` được ghi nhận trong sự kiện `SoloMatchStarted` để module `workout_execution` nhận diện và điều phối runner mô phỏng người thật.

---

## 5. Kỹ Thuật 5: Check-in No-Show 60 Giây (`ready_deadline`) & Cancellation

- Sau khi ghép trận thành công, trận đấu ở trạng thái `PENDING_READY` với mốc `ready_deadline = now + 60s`.
- Worker `MatchTimeoutWorker` định kỳ 15 giây quét các trận quá hạn:
  - Gọi method `match.CancelOnNoShow(now)` $\rightarrow$ Chuyển sang `CANCELLED`.
  - Cả hai bên tham gia bị đánh dấu `FORFEITED`.
  - Ghi sự kiện `SoloMatchCancelled` (lý do: `READY_TIMEOUT`) vào Outbox để Kafka thông báo dọn dẹp các phân hệ liên quan.
- **Terminal State Rule**: Trận đấu một khi đã bị `CANCELLED` thì không bao giờ được phép kích hoạt lại, kể cả khi có tín hiệu `SoloMatchRoomReady` đến muộn.

---

## 6. Kỹ Thuật 6: Kiến Trúc Xử Thua 2 Tầng (Fast Forfeit via Event + 35m Safety Net)

### Tầng 1: Xử thua tức thì qua Event Stream (< 500ms)
- Khi một bên nhấn "Bỏ cuộc" hoặc mất kết nối quá thời gian quy định tại buổi tập camera, `workout_execution` phát sự kiện `SoloTrackingAborted`.
- Consumer `WorkoutAbortedConsumer` trong `solo_match`:
  - Đánh dấu người bỏ cuộc là `FORFEITED`.
  - Đánh dấu đối thủ còn lại là `WIN`.
  - Phát ra cùng lúc `SoloMatchForfeited` và `SoloMatchCompleted`.
  - Đối thủ lập tức nhìn thấy màn hình chiến thắng mà không phải chờ đợi.

### Tầng 2: Lưới an toàn bảo hiểm tầng cuối (Safety Net Worker 35 Phút)
- Đối với các trường hợp ngoại lệ bất khả kháng (cả 2 bên cùng tắt máy, server sự cố):
- `MatchTimeoutWorker` quét các trận `IN_PROGRESS` có thời gian bắt đầu quá 35 phút:
  - Nếu 1 bên đã nộp, 1 bên chưa: Xử bên nộp thắng (`WIN`), bên chưa nộp thua (`FORFEITED`).
  - Nếu cả 2 đều chưa nộp: Hủy trận đấu (`CANCELLED`), phát `SoloMatchCancelled`.

---

## 7. Kỹ Thuật 7: Transactional Outbox (SKIP LOCKED) & Idempotent Consumer Logging

### 7.1. Đảm bảo tính nhất quán giữa Database và Kafka
- Áp dụng **Transactional Outbox Pattern**: Lưu bản ghi trận đấu và bản ghi sự kiện `outbox_events` trong **cùng 1 Database Transaction**.
- Worker chạy ngầm quét bảng `outbox_events` với câu lệnh SQL:
  ```sql
  SELECT * FROM solo_match.outbox_events
  WHERE status = 'PENDING'
  ORDER BY created_at ASC
  LIMIT 50
  FOR UPDATE SKIP LOCKED;
  ```
- Cho phép nhiều Pod publisher cùng quét song song mà không bao giờ bị khóa tranh chấp (Lock Contention).

### 7.2. Lũy đẳng tuyệt đối phía Consumer (`outbox_logs`)
- Mỗi Consumer trong `solo_match` đều kiểm tra qua bảng `solo_match.outbox_logs`:
  ```go
  if c.outboxLogRepo.IsProcessed(ctx, env.ID) {
      log.Printf("Event %s already processed, skipping", env.ID)
      return nil
  }
  ```
- Đảm bảo 1 sự kiện dù bị mạng phân tán gửi lại bao nhiêu lần cũng chỉ được thực thi đúng 1 lần duy nhất.

---

## 8. Kỹ Thuật 8: Pessimistic Row Locking (`FOR UPDATE`) Trên Aggregate Root & Tuần Tự Hóa Hoàn Thành Trận Đấu

### 8.1. Bối cảnh & Vấn đề Race Condition khi hoàn thành đồng thời
Trong trận đấu 1v1, khi cả hai đấu thủ hoàn thành bài tập camera AI gần như cùng một thời điểm, hai sự kiện `SoloTrackingCompleted` được phát ra từ module `workout_execution` và được tiêu thụ song song trên 2 worker goroutines của `solo_match`.
- **Rủi ro nghiêm trọng**: Nếu đọc dữ liệu ngoài transaction bằng `FindByID` thông thường, cả 2 tiến trình đều thấy đối phương "chưa hoàn thành" $\rightarrow$ Cả 2 cùng chỉ ghi nhận điểm của mình mà không bên nào chuyển trạng thái trận sang `COMPLETED`, không bắn event `SoloMatchCompleted`. Trận đấu bị treo vĩnh viễn ở `IN_PROGRESS` (Lỗi **[CRIT-SOLO-01]**). Đồng thời, các thao tác batch update trên `match_participants` có nguy cơ ghi đè dữ liệu của nhau (Lost Update) hoặc gây deadlock trên database.

### 8.2. Giải pháp Pessimistic Row Locking trên Aggregate Root
Áp dụng nguyên lý ranh giới nhất quán của Aggregate Root trong Domain-Driven Design:
1. **Khóa độc quyền ở cấp Aggregate Root**:
   Trong câu lệnh `FindByIDForUpdate`:
   ```go
   db.Clauses(clause.Locking{Strength: "UPDATE"}).
       Preload("Participants", func(pDb *gorm.DB) *gorm.DB {
           return pDb.Order("solo_match.match_participants.id ASC")
       }).
       First(&model, "id = ?", id)
   ```
   PostgreSQL phát lệnh `SELECT * FROM solo_match.solo_matches WHERE id = ? FOR UPDATE`, chiếm giữ khóa độc quyền đúng **1 dòng đại diện cho trận đấu**.
2. **Tuần tự hóa thực thi nguyên tử (Serialized Execution)**:
   - Toàn bộ chu trình: `FindByIDForUpdate` $\rightarrow$ `match.RecordParticipantResult` $\rightarrow$ `Save` $\rightarrow$ `RecordEvents` được bọc trọn vẹn trong một database transaction `txManager.WithTransaction`.
   - Tiến trình 1 vào trước khóa dòng `solo_matches`, ghi nhận điểm số của P1. Trận đấu vẫn `IN_PROGRESS`. Commit transaction và nhả khóa.
   - Tiến trình 2 bị chặn ở tầng DB cho đến khi Tiến trình 1 commit. Khi unblock, Tiến trình 2 đọc phiên bản mới nhất đã commit (chế độ Read Committed), thấy P1 đã xong $\rightarrow$ Kích hoạt `MatchResolver.ResolveOutcome`, chuyển `COMPLETED`, xác định người chiến thắng và ghi sự kiện `SoloMatchCompleted` vào Outbox trong cùng transaction.
3. **Triệt tiêu nguy cơ Deadlock**:
   - Khóa dòng Aggregate Root được chiếm giữ trước tiên, đóng vai trò là "chìa khóa phòng" (Mutex).
   - Danh sách `Participants` luôn được nạp theo thứ tự xác định `id ASC`, loại trừ hoàn toàn khả năng tranh chấp khóa chéo giữa các dòng con.
4. **Hiệu năng**:
   - Thời gian giữ khóa diễn ra trong RAM của PostgreSQL buffer pool chỉ kéo dài **~1-3ms**, hoàn toàn không tạo bottleneck và không cần hạ tầng khóa phân tán Redis phức tạp.
