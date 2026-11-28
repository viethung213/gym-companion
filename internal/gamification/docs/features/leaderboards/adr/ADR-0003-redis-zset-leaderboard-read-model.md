# ADR-0003: Chuyển Bảng Xếp Hạng XP Sang Redis Sorted Set (Read Model) — Postgres Vẫn Là Nguồn Sự Thật

- **Feature**: leaderboards
- **Date**: 2026-11-27
- **Status**: Accepted (implemented 2026-11-27)
- **Supersedes**: [ADR-0001](./ADR-0001-database-indexed-all-time-leaderboard-query.md)
- **Deciders**: Maintainer (User), AI Assistant

## 1. Bối Cảnh & Lý Do

ADR-0001 chọn PostgreSQL B-tree cho giai đoạn MVP và hoãn Redis ZSET tới Giai đoạn 3 (> 500k users). Số lượng user hiện tại **chưa** chạm ngưỡng đó. Quyết định này được đưa ra **vì mục tiêu học tập**: triển khai đúng chuẩn một CQRS read model trên Redis ZSET, đồng thời giải quyết triệt để 3 rủi ro mà ADR-0001 đã nêu:

1. **Lệch dữ liệu (Cache Desync)** giữa Postgres và Redis.
2. **Mất dữ liệu khi Redis restart / không kết nối được.**
3. **Quy tắc hòa điểm** `xp DESC, updated_at ASC, user_id ASC` khó mã hóa vào score float64.

### Mục tiêu
- API `GetGlobalLeaderboard` trả về **cùng kết quả** như hiện tại (cùng thứ tự, cùng rank, cùng field) — ngoại lệ duy nhất được chấp nhận ghi ở §4.3.
- Redis là **read model có thể vứt bỏ**: xóa sạch Redis bất cứ lúc nào thì hệ thống vẫn trả kết quả đúng (fallback Postgres) và tự dựng lại.
- Không thay đổi hợp đồng proto / frontend.

### Non-goals
- Không làm leaderboard theo tuần/tháng, theo nhóm bạn bè, phân trang sâu.
- Không đổi luồng ghi XP (`UserXp` aggregate, outbox) — chỉ **đọc thêm** event có sẵn.
- Không lưu tên/avatar của **mọi** user vào Redis: chỉ cache đọc xuyên những profile thực sự được đọc (§3.5a). `gamification.user_snapshots` (Postgres) vẫn là nguồn sự thật.

### Anti-goals (coi là thất bại dù code chạy được)
- Bảng xếp hạng trống hoặc sai khi Redis vừa restart.
- Điểm trên bảng **lùi** về giá trị cũ do event đến trễ / bị replay.
- Logic nghiệp vụ nằm trong Kafka consumer hoặc gRPC handler.
- Module không khởi động được khi không có Redis.

## 2. Các Phương Án Đã Cân Nhắc

| Phương án | Mô tả | Đánh giá |
| :--- | :--- | :--- |
| **A. Read model ZSET đồng bộ qua Kafka `XpEarned`** | Postgres giữ sự thật; consumer cập nhật ZSET; rebuild/reconcile từ Postgres; fallback khi Redis lỗi | **Chọn** — giải quyết đủ 3 rủi ro, giá trị học cao nhất |
| B. Dual-write trong command sau commit | Ghi Redis ngay trong handler | Loại — ghi Redis lỗi thì lệch vĩnh viễn, đúng rủi ro ADR-0001 cảnh báo |
| C. Cache-aside Top 50 có TTL | Cache response, `my_rank` vẫn `COUNT(*)` | Loại — gần như không dùng ZSET, không phục vụ mục tiêu học |

## 3. Quyết Định

Chọn **Phương án A**.

### 3.1 Redis keys

| Key | Kiểu | Nội dung |
| :--- | :--- | :--- |
| `gamification:leaderboard:xp` | ZSET | member = `user_id` (UUID lowercase), score = điểm mã hóa (§3.2); **cộng thêm 1 member sentinel** `__ready__` |
| `gamification:leaderboard:xp:rebuild_lock` | STRING | node ID, `SET NX EX 120` — chống 2 instance rebuild cùng lúc |
| `gamification:profile:{user_id}` | HASH | `display_name`, `avatar_url`, `updated_at` — cache đọc xuyên, TTL 24h (§3.5a) |

Không đặt TTL cho ZSET.

**Marker "ready" nằm NGAY TRONG ZSET** (member `__ready__`, score `+inf`), không phải key riêng. Lý do — môi trường thực tế (`infra/redis/redis.conf`):
- `maxmemory 256mb` + `maxmemory-policy allkeys-lru`, dùng chung với module social → Redis **có thể evict** key ZSET.
- `appendonly yes` → restart Redis **giữ lại** dữ liệu (mất tối đa ~1s ghi).

Nếu marker là key riêng, eviction có thể xóa ZSET nhưng giữ marker → phục vụ bảng rỗng/thiếu với trạng thái "ready". Đặt sentinel trong ZSET thì ZSET và marker **luôn mất cùng nhau** (eviction/DEL xóa nguyên key). ZSET bị consumer tạo lại sau khi mất sẽ không có sentinel ⇒ "chưa ready" ⇒ fallback Postgres + rebuild. **Không thay đổi cấu hình Redis dùng chung** (tránh ảnh hưởng module social).

Hệ quả cho sentinel: score `+inf` ⇒ luôn đứng **cuối** theo `ZRANGE` (điểm thật đều âm) ⇒ không ảnh hưởng `ZRANK` của bất kỳ user nào. Khi đọc Top N phải **lọc bỏ** `__ready__` nếu nó lọt vào kết quả (chỉ xảy ra khi tổng user < N). `GetUserRank("__ready__")` không thể xảy ra vì `user_id` là UUID; adapter vẫn phải từ chối member này.

### 3.2 Mã hóa score (giải quyết rủi ro #3)

```
EPOCH       = 2020-01-01T00:00:00Z   # đủ sớm để dữ liệu lịch sử vẫn hòa điểm đúng theo thời gian; tới 2088
TIME_BITS   = 31                     # 2^31 giây ≈ 68 năm tính từ EPOCH
TIME_MAX    = 2^31 - 1
XP_MAX      = 2^22 - 1 = 4_194_303   # 22 + 31 = 53 bit = độ chính xác số nguyên float64

s           = clamp(floor(updated_at - EPOCH in seconds), 0, TIME_MAX)
x           = clamp(xp, 0, XP_MAX)
composite   = x * 2^31 + (TIME_MAX - s)          # lớn hơn = hạng cao hơn
score       = -composite                         # lưu số ÂM
```

Vì sao lưu **số âm** và đọc bằng `ZRANGE`/`ZRANK` (chiều tăng) thay vì `ZREVRANGE`/`ZREVRANK`:
- Redis sắp các member **cùng score** theo thứ tự từ điển **tăng dần** của member.
- Với `ZRANGE` trên score âm: điểm cao nhất đứng đầu, và khi hòa tuyệt đối thì `user_id` **ASC** — khớp đúng tie-breaker cuối của Postgres (`uuid` so sánh theo byte = so sánh chuỗi hex lowercase).
- Dùng `ZREVRANGE` sẽ cho `user_id DESC` → lệch với Postgres.

Giải mã (để trả `xp`, `updated_at` mà không cần đọc Postgres):
```
composite  = -score            (là số nguyên chính xác vì < 2^53)
xp         = composite >> 31
s          = TIME_MAX - (composite & TIME_MAX)
updated_at = EPOCH + s seconds
level      = vo.CalculateLevel(xp)   # aggregate luôn giữ level == CalculateLevel(xp)
```

Ràng buộc biên (phải có unit test):
- `xp > XP_MAX` → clamp + log `WARN` (mức này ≈ 8.5 lần XP cần cho level 100, coi như không xảy ra).
- `updated_at < EPOCH` → s = 0; `updated_at` quá xa tương lai → s = TIME_MAX.
- Hàm encode/decode là **hàm thuần**, đặt trong adapter Redis (chi tiết lưu trữ, không phải nghiệp vụ).

### 3.3 Luồng ghi (giải quyết rủi ro #1)

```
UserXp.EarnXp ──(tx)──> user_xp + outbox ──OutboxWorker──> Kafka gamification.events
                                                              │ type = contracts.supporting.gamification.v1.xpEarned
                                                              ▼
                     transport/consumer/LeaderboardXpEarnedConsumer   (group: gamification-leaderboard-group)
                                                              │ chỉ parse user_id → command
                                                              ▼
                     application/command/RefreshLeaderboardScoreHandler
                                                              │ LeaderboardScoreSource.GetScore(userID)  (PK lookup user_xp)
                                                              │ không có row → bỏ qua
                                                              ▼
                     infrastructure/redis/LeaderboardProjection:  ZADD gamification:leaderboard:xp LT <score> <userID>
```

- **Event chỉ là tín hiệu "user này vừa đổi"; điểm luôn đọc lại từ Postgres.** Lý do (phát hiện khi chạy thật 2026-11-27): consumer group mới đọc lại toàn bộ lịch sử `gamification.events`; khi Postgres đã bị reset/restore mà Kafka còn giữ event, việc tin dữ liệu trong event đưa **45 "user ma"** (không còn trong `user_xp`) vào bảng xếp hạng. Đọc lại từ nguồn sự thật loại bỏ hẳn lớp lỗi này (event cũ, replay, event mồ côi). Chi phí: một truy vấn theo khóa chính mỗi event.
- Idempotent (đọc trạng thái hiện tại + upsert) → không cần `outbox_log` dedupe — truyền `nil` cho `outboxLogRepo`. Lỗi DB/Redis → trả lỗi để `runConsumerLoop` retry.
- **`ZADD LT`** (score âm ⇒ "nhỏ hơn" = tốt hơn): chỉ cập nhật khi score mới tốt hơn. XP chỉ tăng (aggregate cấm delta ≤ 0), và bit XP chiếm phần cao của score, nên ghi muộn/ghi lặp **không bao giờ làm lùi điểm**. `LT` vẫn thêm member mới nếu chưa tồn tại.
- Đường nhanh (bên dưới) dùng trực tiếp `NewXp`/`EarnedAt` của event vừa commit: `UserXp.EarnXp` gán `updatedAt = earnedAt` nên score khớp score dựng từ Postgres (ở độ chính xác giây), và user chắc chắn tồn tại.
#### Đường nhanh: ghi ngay sau commit (application layer)

Đường Kafka có độ trễ ~0–5s (chu kỳ OutboxWorker) và **không phủ user mới 0 XP** (`InitUserGamification` không phát `XpEarned`). Vì vậy thêm đường nhanh, best-effort:
- `ActivityIngestionCoordinator.runPipeline`: gom các `*event.XpEarned` trong `pipelineEvents` (closure); **sau khi** `WithTransaction` trả `nil` → `ProjectLeaderboardScoreHandler.ProjectBestEffort(ctx, XpEarnedProjections(events)...)` (→ `UpsertScores`).
- `InitUserGamificationHandler.Handle`: sau commit **luôn** gọi `ProjectBestEffort(ctx, {userID, xp, updatedAt})` với dòng `UserXp` vừa đọc/tạo trong transaction. Không dựa vào "vừa tạo mới": `GormUserXpRepository.FindByIDForUpdate` tự seed dòng 0 XP khi chưa có, nên điều kiện đó không bao giờ đúng ở production (phát hiện khi review). Chiếu lại điểm hiện tại là idempotent (`ZADD LT`), kể cả khi event đăng ký bị gửi lại.
- Lỗi Redis ở đường nhanh: log `WARN`, **không** trả lỗi cho command (Postgres đã commit; đường Kafka + reconcile sẽ chữa).
- Port `LeaderboardProjection` là optional (nil khi không có Redis) → handler bỏ qua.
- Ghi đôi (đường nhanh + Kafka) an toàn vì `ZADD LT` với XP tuyệt đối là idempotent và đơn điệu.
- Builder phải xác minh không có đường cộng XP nào khác ngoài coordinator (các `Process*XpHandler.Handle` gọi trực tiếp, nếu có, cũng phải được phủ).

Ba lớp: đường nhanh (tức thì) → Kafka consumer (vài giây, khi đường nhanh lỗi/app chết sau commit) → reconcile 15 phút (khi cả hai lỗi).

- Consumer lỗi Redis: `sharedKafka.RunConsumerLoop` retry 3 lần (cùng mặc định với các consumer khác của module), rồi `OnDLQ` ghi `WARN` và **bỏ** message (offset được commit). **Không** đẩy vào DLQ hệ thống: event chỉ là tín hiệu "user này vừa đổi", phát lại sau không thêm thông tin, và trong lúc Redis sập mọi message đều lỗi ⇒ DLQ sẽ ngập; cũng không để message chưa commit vì consumer sẽ kẹt. Sai lệch được chữa bởi reconcile định kỳ (§3.4).

### 3.4 Rebuild & Reconcile (giải quyết rủi ro #2 và phần còn lại của #1)

`application/command/RebuildLeaderboardHandler` — nhận cờ `Reset bool`:
1. `projection.TryAcquireRebuildLock(ctx)` — không lấy được thì return `nil` (instance khác đang làm).
2. Nếu `Reset` → `projection.Reset(ctx)` = `DEL gamification:leaderboard:xp` (xóa luôn sentinel ⇒ đọc lập tức fallback Postgres). Sau đó `BeginRebuild` = `ZADD key +inf __building__:<nodeID>`.
3. Duyệt `gamification.user_xp` theo **keyset pagination** `WHERE user_id > :cursor ORDER BY user_id ASC LIMIT 1000` qua port mới `LeaderboardScoreSource.ListScores(ctx, afterUserID, limit)` (Postgres).
4. Mỗi batch → `projection.UpsertScores(ctx, batch)` rồi `ExtendRebuildLock` (Lua compare-and-PEXPIRE 120s; mất lock ⇒ dừng với `ErrRebuildInterrupted`). `UpsertScores` = `ZADD ... LT`, member chuẩn hóa chữ thường (cùng ngữ nghĩa với consumer ⇒ chạy song song với consumer không lùi điểm, không mất cập nhật, không cần key tạm/RENAME).
5. Xong toàn bộ → `projection.MarkReady(ctx)` = **Lua có fencing**: chỉ khi lock vẫn là của node này **và** marker `__building__:<nodeID>` vẫn còn trong ZSET (key không bị DEL/evict giữa chừng) thì `ZREM` marker + `ZADD +inf __ready__`; ngược lại `ErrRebuildInterrupted` (worker chạy lại). Lỗi giữa chừng ⇒ **không** MarkReady. Mỗi lượt chạy của worker bị giới hạn 10 phút. Luôn `ReleaseRebuildLock` (defer, chỉ xóa nếu value == nodeID — dùng Lua compare-and-delete).

Vì sao `Reset` an toàn với consumer chạy song song: sau `DEL`, consumer có thể `ZADD LT` vào key mới — giá trị đó là điểm mới nhất, rebuild `ZADD LT` sau đó không làm lùi. Sentinel chỉ được thêm khi rebuild đã quét hết ⇒ không bao giờ phục vụ bảng thiếu.

Kích hoạt bởi `infrastructure/worker/LeaderboardReconcileWorker`:
- **Khi khởi động app**: `Reset = true` — dựng lại từ đầu. Chữa trường hợp Postgres bị reset/restore/sửa tay trong khi Redis (AOF) vẫn giữ dữ liệu cũ ⇒ không có "user ma" hay điểm phồng tồn tại vĩnh viễn. Quy trình chữa thủ công: restart app.
- **Định kỳ mỗi 15 phút** (`LEADERBOARD_RECONCILE_INTERVAL`, mặc định `15m`): `Reset = false` — chữa event bị consumer bỏ qua khi Redis lỗi, và ~1s ghi mất khi Redis restart (AOF `everysec`).
- Khi đọc phát hiện **chưa ready** → gửi tín hiệu non-blocking (channel buffer 1) cho worker để chạy sớm (`Reset = false`). Tín hiệu phát sinh **trong lúc** một lượt chạy thành công (đọc thấy not-ready vì chính lượt đó đang dựng) bị bỏ sau khi lượt đó xong, tránh rebuild toàn bộ thừa; bảng vẫn chưa ready thì lần đọc kế tiếp lại gửi tín hiệu. Lượt chạy lỗi giữ nguyên tín hiệu ⇒ chạy lại.

Giả định ghi rõ: user không bao giờ bị xóa khỏi `user_xp` (đã kiểm tra: không có code nào xóa). Nếu sau này có luồng xóa user → cần `ZREM` trong luồng đó (ngoài phạm vi); restart app cũng dọn sạch nhờ `Reset`.

### 3.5 Luồng đọc & fallback

Tách interface hiện tại thành 2 (domain/repository):
```go
// Ranking — có 2 implementation: Gorm (hiện tại) và Redis
type LeaderboardRepository interface {
    GetGlobalTop(ctx context.Context, limit, offset int) ([]*entity.LeaderboardUser, int64, error)
    GetUserRank(ctx context.Context, userID string) (int64, *entity.LeaderboardUser, error)
}

// Snapshot tên/avatar — chỉ Gorm
type UserSnapshotRepository interface {
    SaveSnapshot(ctx context.Context, snapshot *entity.UserSnapshot) error
    GetSnapshot(ctx context.Context, userID string) (*entity.UserSnapshot, error)
    GetSnapshots(ctx context.Context, userIDs []string) (map[string]*entity.UserSnapshot, error) // MỚI: 1 query IN
}
```
`GormLeaderboardRepository` giữ nguyên SQL, implement cả hai (struct không đổi, chỉ thêm `GetSnapshots`). `InitUserGamificationHandler` và `SyncUserSnapshotHandler` chuyển sang nhận `UserSnapshotRepository`. `GetGlobalLeaderboardHandler` không đổi.

`infrastructure/redis/RedisLeaderboardRepository` (implement `LeaderboardRepository`):
- **Kiểm tra ready**: `ZSCORE key __ready__` nằm **cùng MULTI/EXEC (TxPipeline)** với lệnh đọc ⇒ không có khe hở giữa kiểm tra và đọc; không có ⇒ `ErrLeaderboardNotReady`. Lỗi lệnh Redis được bọc `ErrRedisUnavailable` (phân biệt với lỗi Postgres khi tải snapshot). Mọi member bắt đầu bằng `__` là nội bộ và bị lọc.
- **GetGlobalTop(limit, offset)**: `ZRANGE key offset offset+limit-1 WITHSCORES` → bỏ member `__ready__` → decode → `snapshots.GetSnapshots(ids)` → map (`DisplayName` mặc định `"Gymer"`, `AvatarURL` mặc định `""` — y hệt Gorm). `Rank = offset + i + 1`. Trả về `total = len(result)` (giữ nguyên ngữ nghĩa hiện tại).
- **GetUserRank(userID)**: chuẩn hóa chữ thường; MULTI/EXEC ready-check + `ZRANK` + `ZSCORE`.
  - Có member → `rank = ZRANK + 1`, decode score, lấy snapshot.
  - **Không có member** (user mới đăng ký 0 XP chưa reconcile, hoặc event bị bỏ qua) → **read-repair**: đọc `scoreSource.GetScore(userID)` từ Postgres; không có row → `(0, nil, nil)` như hiện tại; có row → `ZADD LT` rồi đọc lại **kèm ready-check** (key mất giữa chừng ⇒ `ErrLeaderboardNotReady`, không trả hạng trong tập thiếu).

`infrastructure/redis/FallbackLeaderboardRepository` (implement `LeaderboardRepository`, bọc Redis + Gorm):
- Gọi Redis; nếu lỗi **bất kỳ** (kể cả `ErrLeaderboardNotReady`) → log `WARN` + gọi Gorm + gửi tín hiệu rebuild nếu là not-ready.
- **Circuit breaker**: chỉ lỗi `ErrRedisUnavailable` khi caller chưa hủy context (không tính `context.Canceled`/`DeadlineExceeded` hay lỗi Postgres trong repo Redis) → bỏ qua Redis trong `30s` rồi thử lại. Lý do (đo khi chạy thật 2026-11-28): Redis tắt ⇒ mỗi request chờ timeout kết nối ≈ 5,4s. Client Redis dùng chung không bật `ContextTimeoutEnabled` nên không thể giới hạn bằng context mà không ảnh hưởng module social; breaker giới hạn chi phí còn **một** request chậm mỗi 30s/instance. Not-ready không kích hoạt breaker (Redis vẫn khỏe, đang rebuild).
- Đây là resilience của hạ tầng, không phải nghiệp vụ — nên đặt ở infrastructure.

### 3.5a Cache tên/avatar (Redis Hash, bổ sung 2026-11-28)

Lý do: test 1 triệu user cho thấy truy vấn `user_snapshots WHERE user_id IN (50 id)` có thể bị planner chọn Seq Scan khi thống kê bảng cũ (đo được 830–960 ms cho Top 50). Cache loại Postgres khỏi đường đọc nóng.

`infrastructure/redis/CachedUserSnapshotRepository` — decorator implement `repository.UserSnapshotRepository`, bọc `GormUserSnapshotRepository` (vẫn là nguồn sự thật):
- **Đọc (`GetSnapshots`/`GetSnapshot`)**: pipeline `HMGET` cho mọi id → id thiếu (hoặc entry hỏng) được nạp từ Postgres trong **một** truy vấn → ghi lại bằng Lua *"chỉ ghi nếu key chưa tồn tại"* + `PEXPIRE 24h`. User không có snapshot thì **không** cache. Key chuẩn hóa chữ thường.
- **Ghi (`SaveSnapshot`)**: lưu Postgres trước, rồi **write-through** (MULTI `HSET` + `PEXPIRE`) ghi đè profile mới. Kết hợp với quy tắc "reader chỉ ghi khi key chưa có", một reader đã đọc tên **cũ** từ Postgres trước khi đổi tên commit sẽ **không thể** ghi đè tên mới (khe hở cache-aside kinh điển; có test tái hiện). Không cần so sánh đồng hồ giữa các instance.
- **Redis lỗi**: không bao giờ làm hỏng lời gọi — đọc chuyển sang Postgres, ghi vẫn lưu Postgres (log `WARN`).
- **Write-through thất bại** (phát hiện khi test thủ công: đổi tên lúc Redis sập, Redis khởi động lại từ RDB vẫn giữ tên cũ): instance tăng bộ đếm `missedWrites`. Khi `missedWrites ≠ flushedWrites`, cache **không được tin**: lần đọc kế tiếp (một request giữ `TryLock`, các request khác đọc Postgres) `SCAN gamification:profile:*` + `UNLINK` theo lô 1000, thành công thì `flushedWrites = missedWrites` (log `INFO`), cache tự nạp lại từ Postgres. Flush lỗi ⇒ tiếp tục đọc Postgres. Write-through lỗi trong lúc đang flush không bị bỏ sót (bộ đếm vẫn lệch ⇒ flush lại). Giới hạn còn lại: instance chết trước khi kịp flush ⇒ TTL 24h chặn trên; instance khác chưa từng lỗi vẫn đọc cache cũ cho tới khi instance lỗi flush (ngay ở request leaderboard tiếp theo của nó).
- **Bộ nhớ**: ~200 byte/profile (đo thực tế), chỉ tỉ lệ với số profile được đọc trong 24h (Top N luôn nóng), không phải tổng số user.
- **Phạm vi dùng**: chỉ ở chế độ Redis. Read repo Redis, `SyncUserSnapshotHandler` và `InitUserGamificationHandler` dùng **chung** instance cache (để đổi tên đến được cache). Đường dự phòng Gorm vẫn `LEFT JOIN user_snapshots`.
- **Ghi sau commit**: `InitUserGamificationHandler` lưu snapshot **sau** khi transaction (streak/xp/wallet) commit, không dùng `txCtx` — write-through không bao giờ ghi profile của một lần khởi tạo bị rollback. Lưu snapshot lỗi ⇒ trả lỗi để consumer retry; khởi tạo idempotent nên chạy lại an toàn. Trong khoảng giữa commit và lưu snapshot, user hiện tên mặc định `Gymer`.
- Top 50 = 1 MULTI (`ZRANGE` + ready) + 1 pipeline `HMGET` — 2 lượt Redis, 0 truy vấn Postgres khi cache nóng.

### 3.6 Wiring (`module.go`)

- Lấy client theo đúng mẫu của module social: `sharedRedis.GetRegistry().GetClient()`.
- Chế độ Redis chỉ bật khi **có cả Redis lẫn Kafka reader** cho `gamification-leaderboard-group` / `gamification.events`. Thiếu một trong hai → dùng `GormLeaderboardRepository` như hôm nay, không start consumer/worker, module vẫn khởi động. (Không chấp nhận chế độ "Redis không Kafka" vì bảng sẽ trễ tới 15 phút.)
- Kafka reader chỉ được **mở sau khi** Redis kết nối được (truyền vào dạng factory): mở group reader là join consumer group ngay, một member không bao giờ đọc sẽ giữ partition khiến instance khỏe không nhận được event.
- **Đủ cả hai**: `leaderboardRepo = Fallback(Redis, Gorm)`; start `LeaderboardXpEarnedConsumer` và `LeaderboardReconcileWorker`.
- **Giới hạn đã biết**: Redis chỉ được ping một lần lúc khởi động; Redis lỗi đúng lúc đó ⇒ instance chạy Postgres-only tới lần restart kế tiếp (vẫn đúng dữ liệu, chỉ mất lợi ích Redis).
- Shutdown dùng chung `cancelWorkers` + `wg.Wait()` sẵn có.

### 3.7 Kiểm thử

| Tầng | Công cụ | Trường hợp bắt buộc |
| :--- | :--- | :--- |
| Score codec | unit | round-trip; thứ tự: xp cao hơn thắng; cùng xp → cập nhật sớm hơn thắng; clamp XP_MAX & thời gian; score là số nguyên chính xác |
| Redis adapter | `miniredis/v2` v2.40 (đã có trong go.mod, hỗ trợ `ZADD LT`) | ZADD LT không lùi điểm; replay event cũ; hòa tuyệt đối → user_id ASC; sentinel bị lọc khỏi Top N và không đổi rank; mất key (giả lập eviction) → not-ready; ZSET tạo lại bởi consumer mà không có sentinel → not-ready; read-repair; lock compare-and-delete |
| Đường nhanh | mock port | coordinator gọi UpsertScore **chỉ sau** commit thành công, không gọi khi tx lỗi; lỗi projection không làm command lỗi; init chiếu điểm hiện tại sau commit cho cả user chưa có dòng, dòng 0 XP do repo seed, và user đã có XP; commit lỗi → không lưu snapshot, không chiếu; projection nil → bỏ qua |
| Command | mock port | Project gọi đúng tham số; Rebuild duyệt hết trang, MarkReady cuối cùng, không MarkReady khi lỗi giữa chừng, không chạy khi không lấy được lock, `Reset=true` gọi Reset trước khi duyệt |
| Consumer | mock handler | bỏ qua event type khác; gửi `RefreshLeaderboardScoreCommand{user_id}`; thiếu `user_id`/JSON lỗi → error |
| Refresh | fake source + projection | chiếu đúng điểm trong Postgres; user không còn trong `user_xp` → bỏ qua (không có user ma); lỗi source → error |
| Parity (mặc định) | SQLite testutil hiện có + miniredis | Seed dataset có hòa điểm (khác giây) → rebuild → `GetGlobalTop`/`GetUserRank` của Redis == Gorm |
| **Parity (Postgres thật)** | `//go:build integration` + `TEST_DATABASE_URL`, theo mẫu `internal/exercise/infrastructure/persistence/postgres_repository_integration_test.go` | Cùng kịch bản trên Postgres thật — kiểm tra thứ tự `uuid` và `timestamptz`. CI hiện không có Postgres ⇒ test này chạy local (`go test -tags integration`) |
| Fallback | mock | Redis lỗi → kết quả Gorm; not-ready → tín hiệu rebuild |
| Luồng đầu-cuối | repo Gorm thật (SQLite) + miniredis | cộng XP qua coordinator → điểm trên Redis bằng đúng điểm Postgres vừa commit; Redis lạnh (chưa có key) và Redis sập → `Fallback` trả đúng kết quả Postgres, Redis lạnh gửi tín hiệu rebuild; user mới đăng ký lên bảng ngay |
| Profile cache | miniredis + fake Postgres đếm lượt nạp | miss nạp 1 lần rồi cache (TTL 24h); hit một phần chỉ nạp phần thiếu; user không có snapshot không cache; avatar rỗng; key không phân biệt hoa thường; Redis sập → Postgres; lỗi Postgres → error; save write-through; save khi Redis sập vẫn thành công; save lỗi giữ nguyên cache; **reader cũ không ghi đè được tên mới**; **write-through lỗi ⇒ flush toàn bộ profile khi Redis hồi phục, rồi nạp lại**; flush chưa được ⇒ đọc Postgres |
| Quy mô | `LEADERBOARD_SCALE_USERS` + Postgres/Redis thật | seed N user (rollback), rebuild, bộ nhớ, p50/p95/p99 Redis vs Postgres, so khớp 1000 hạng |

## 4. Hệ Quả

### 4.1 Tích cực
- Top N: `O(log N + M)`; `my_rank`: `O(log N)` — không còn `COUNT(*)` quét index.
- Redis có thể xóa bất kỳ lúc nào mà không sai kết quả.
- Ứng dụng đầy đủ: read model, idempotent projection, reconcile, read-repair, fallback.
- Số đo thực tế (Postgres + Redis local trên Docker Desktop/Windows, `leaderboard_scale_integration_test.go`):

| Chỉ số | 100k user | 1 triệu user |
| :--- | :--- | :--- |
| Rebuild toàn bộ | 5,7 s | ~60 s |
| Bộ nhớ ZSET | 11 MiB | 107–113 MiB (~115 byte/user) |
| Hạng cá nhân — Postgres `COUNT` p50 / p99 | 9,8 / 18,7 ms | 44–60 / 62–131 ms |
| Hạng cá nhân — Redis p50 (cache nóng) | 1,8 ms (trước cache) | **0,56 ms** |
| Top 50 — Postgres p50 | 2,2 ms | 1,8 ms |
| Top 50 — Redis + profile cache p50 | — | **0,81 ms** |
| Kết quả Redis vs Postgres | giống hệt | giống hệt |

### 4.2 Tiêu cực
- Bình thường điểm lên bảng ngay sau commit (đường nhanh). Đường nhanh lỗi → vài giây (Kafka). Redis lỗi kéo dài qua cả hai → ≤ 1 chu kỳ reconcile (15 phút); trong lúc Redis lỗi đọc vẫn đi Postgres.
- Thêm một lời gọi Redis sau commit trong coordinator và init handler (best-effort, không ảnh hưởng kết quả command).
- Thêm thành phần vận hành (consumer group, worker), mỗi chu kỳ reconcile quét toàn bộ `user_xp`.
- Mỗi lần khởi động app, bảng phục vụ từ Postgres trong lúc rebuild (vài giây ở quy mô hiện tại).
- Redis bị evict key ZSET (LRU, chung 256MB với social) ⇒ fallback Postgres cho tới lần rebuild kế tiếp (được kích hoạt ngay khi đọc thấy not-ready).

### 4.3 Sai khác được chấp nhận so với Postgres
- Thời gian trong score chính xác tới **giây**. Hai user **cùng XP** đạt mốc **trong cùng một giây**: Postgres phân định theo micro giây, Redis phân định theo `user_id ASC`.
- `updated_at` trả về API bị làm tròn xuống giây.

## 5. Phần Để Lại Cho Builder
- Tên file/struct cụ thể, cách inject nodeID (UUID mỗi process), log format.
- Batch size (mặc định 1000) và timeout mỗi lệnh Redis.
- Cập nhật tài liệu: đổi ADR-0001 sang `Status: Superseded by ADR-0003`; cập nhật `02-design.md` (sơ đồ, §2.3, sequence, cấu trúc mã); thêm task `T-LB-09+` vào `04-tasks.md`; cập nhật dòng leaderboard trong `docs/README.md` và `architecture.md` nếu nhắc tới Postgres-only.
