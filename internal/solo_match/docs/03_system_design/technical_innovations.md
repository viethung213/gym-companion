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
