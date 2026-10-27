# Thiết Kế Hệ Thống — Bounded Context Solo Match (System Design)

Tài liệu này đặc tả chi tiết kiến trúc kỹ thuật của Bounded Context **Solo Match** theo mô hình **Hexagonal Architecture (Ports & Adapters)** và các tiêu chuẩn khắt khe trong `AGENTS.md`.

---

## 1. Cấu Trúc Mã Nguồn Module (`/internal/solo_match/`)

Toàn bộ mã nguồn cốt lõi của module được đóng gói hoàn toàn trong `/internal/solo_match/`:

```
internal/solo_match/
├── domain/                          # LÕI NGHIỆP VỤ (Pure Go, không ORM/JSON tag)
│   ├── aggregate/                   # SoloMatch (Aggregate Root), MatchmakingTicket
│   ├── entity/                      # MatchParticipant, UserSnapshot
│   ├── value_object/                # MatchStatus, MatchResult, ChallengeType, ExerciseChallenge, ParticipantScore
│   ├── event/                       # DomainEvent interface, SoloMatchStarted, SoloMatchCompleted, SoloMatchForfeited, SoloMatchCancelled
│   ├── repository/                  # Outbound Ports: SoloMatchRepository, UserSnapshotRepository, BotRepository, OutboxRepository, OutboxLogRepository
│   ├── service/                     # Domain Service: MatchResolver (Phân định thắng/thua/hòa)
│   └── derror/                      # Domain Business Errors (ErrMatchNotActive, ErrMatchNotPendingReady, ...)
│
├── application/                     # APPLICATION LAYER (Điều phối use cases theo CQRS)
│   ├── command/                     # Command Handlers: JoinMatchmakingQueue, CancelMatchmaking, RecordParticipantResult, RecordParticipantAbort, HandleMatchTimeout, SyncUserSnapshot
│   ├── query/                       # Query Handlers: GetSoloMatchByID, GetUserMatchHistory, GetActiveMatch
│   ├── port/                        # Outbound Ports: MatchmakerQueuePort, ExerciseProvider, TxManager, EventPublisher
│   └── dto/                         # Data Transfer Objects (SoloMatchDTO, ParticipantDTO, MatchmakingTicketDTO)
│
├── infrastructure/                  # TẦNG HẠ TẦNG (Triển khai công nghệ)
│   ├── matchmaker/                  # RedisMatchmakerAdapter (Redis ZSet + Lua Script + Pub/Sub)
│   ├── persistence/                 # GORM PostgreSQL Repositories (schema solo_match.*)
│   │   ├── gorm_solo_match_repository.go
│   │   ├── gorm_user_snapshot_repository.go
│   │   ├── gorm_bot_repository.go   # In-Memory Cache O(1)
│   │   ├── gorm_outbox_repository.go # Khóa FOR UPDATE SKIP LOCKED
│   │   ├── gorm_outbox_log_repository.go # Idempotency logger
│   │   ├── models.go                # GORM Schema Models
│   │   └── tx_manager.go            # Quản lý Database Transaction
│   ├── worker/                      # MatchTimeoutWorker, OutboxPublisherWorker
│   ├── exercise_client/             # ExerciseProvider Adapter
│   └── event/                       # OutboxWriter Adapter (Đóng gói CloudEvents)
│
├── transport/                       # DRIVING ADAPTERS (Giao diện ra bên ngoài)
│   ├── grpc/                        # gRPC & ConnectRPC Service Handler (JoinQueue stream, CancelQueue, GetMatch, GetMatchHistory)
│   └── consumer/                    # Kafka Consumers: SoloMatchRoomReadyConsumer, WorkoutCompletedConsumer, WorkoutAbortedConsumer, UserRegisteredConsumer, UserIdentityUpdatedConsumer
│
├── docs/                            # HỆ THỐNG TÀI LIỆU KỸ THUẬT
└── module.go                        # DEPENDENCY INJECTION ENTRYPOINT
```

---

## 2. Thiết Kế Cơ Sở Dữ Liệu (PostgreSQL Schema: `solo_match.*`)

Áp dụng quy chuẩn **Schema Isolation**, toàn bộ bảng dữ liệu nằm trong schema `solo_match` (Migrations: `13-create-solo-match-tables.sql` và `14-alter-solo-matches-deadline.sql`):

```sql
CREATE SCHEMA IF NOT EXISTS solo_match;

-- 1. Bảng User Snapshot (Bản sao danh tính siêu tốc < 1ms)
CREATE TABLE IF NOT EXISTS solo_match.user_snapshots (
    user_id VARCHAR(64) PRIMARY KEY,
    display_name VARCHAR(128) NOT NULL,
    avatar_url TEXT NOT NULL DEFAULT '',
    synced_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- 2. Bảng Solo Matches (Quản lý trạng thái trận đấu & mốc thời gian)
CREATE TABLE IF NOT EXISTS solo_match.solo_matches (
    id VARCHAR(64) PRIMARY KEY,
    exercise_id VARCHAR(64) NOT NULL,
    exercise_name VARCHAR(128) NOT NULL,
    challenge_type VARCHAR(32) NOT NULL, -- 'REPS' hoặc 'DURATION'
    status VARCHAR(32) NOT NULL,         -- 'PENDING_READY', 'IN_PROGRESS', 'COMPLETED', 'FORFEITED', 'CANCELLED'
    winner_id VARCHAR(64) NULL,          -- NULL nếu đang diễn ra hoặc kết quả HÒA (DRAW)
    is_draw BOOLEAN NOT NULL DEFAULT FALSE,
    started_at TIMESTAMP WITH TIME ZONE NULL,
    ready_deadline TIMESTAMP WITH TIME ZONE NULL, -- Hạn chót vào phòng (now + 60s)
    ended_at TIMESTAMP WITH TIME ZONE NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_solo_matches_status ON solo_match.solo_matches(status);
CREATE INDEX IF NOT EXISTS idx_solo_matches_ready_deadline ON solo_match.solo_matches(ready_deadline) WHERE status = 'PENDING_READY';
CREATE INDEX IF NOT EXISTS idx_solo_matches_started_at ON solo_match.solo_matches(started_at) WHERE status = 'IN_PROGRESS';

-- 3. Bảng Người Tham Gia Trận Đấu (Điểm số & Kết quả cá nhân)
CREATE TABLE IF NOT EXISTS solo_match.match_participants (
    id VARCHAR(64) PRIMARY KEY,
    match_id VARCHAR(64) NOT NULL REFERENCES solo_match.solo_matches(id) ON DELETE CASCADE,
    user_id VARCHAR(64) NOT NULL,
    score DOUBLE PRECISION NOT NULL DEFAULT 0,
    result VARCHAR(32) NOT NULL DEFAULT 'PENDING', -- 'PENDING', 'WIN', 'LOSE', 'DRAW', 'FORFEITED'
    completed_at TIMESTAMP WITH TIME ZONE NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_match_participants_match_id ON solo_match.match_participants(match_id);
CREATE INDEX IF NOT EXISTS idx_match_participants_user_id ON solo_match.match_participants(user_id);

-- 4. Bảng Kho Tài Khoản Bot Hệ Thống (System Bot Pool)
CREATE TABLE IF NOT EXISTS solo_match.system_bots (
    id VARCHAR(64) PRIMARY KEY,
    user_id VARCHAR(64) NOT NULL UNIQUE REFERENCES solo_match.user_snapshots(user_id) ON DELETE CASCADE,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- 5. Bảng Transactional Outbox (Xuất bản sự kiện tin cậy)
CREATE TABLE IF NOT EXISTS solo_match.outbox_events (
    id VARCHAR(64) PRIMARY KEY,
    aggregate_type VARCHAR(64) NOT NULL,
    aggregate_id VARCHAR(64) NOT NULL,
    event_type VARCHAR(128) NOT NULL,
    payload JSONB NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'PENDING', -- 'PENDING', 'PUBLISHED', 'FAILED'
    retry_count INT NOT NULL DEFAULT 0,
    last_error TEXT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_outbox_events_status_created_at ON solo_match.outbox_events(status, created_at) WHERE status = 'PENDING';

-- 6. Bảng Nhật Ký Lũy Đẳng Consumer (Consumer Idempotency Log)
CREATE TABLE IF NOT EXISTS solo_match.outbox_logs (
    id VARCHAR(64) PRIMARY KEY,
    event_id VARCHAR(64) NOT NULL UNIQUE,
    event_type VARCHAR(128) NOT NULL,
    payload JSONB NOT NULL,
    partition_key VARCHAR(64) NOT NULL,
    status VARCHAR(32) NOT NULL, -- 'PROCESSED', 'FAILED'
    error_message TEXT NULL,
    processed_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);
```

---

## 3. Thiết Kế Bộ Đệm Thời Gian Thực Trên Redis Của Solo Match

Hệ thống tận dụng RAM của Redis để phục vụ hàng đợi tìm trận với độ trễ < 1ms:
- **Key ZSet**: `solo_match:queue:waiting`
  - *Score*: Unix timestamp lúc gửi yêu cầu tìm trận (`time.Now().UTC().Unix()`). Đóng vai trò là hàng đợi FIFO tự nhiên.
  - *Member*: Chuỗi `{ticketID}:{userID}`.
- **Kênh Pub/Sub Đánh Thức**: `solo_match:events:{ticketID}`
  - Dùng để gửi tín hiệu `matchID` hoặc thông báo lỗi `ERROR:<msg>` đến từng luồng stream gRPC của Client.

---

## 4. Thiết Kế Các Thành Phần Xử Lý Cốt Lõi

### 4.1. `RedisMatchmakerAdapter` (Hàng đợi tìm trận)
- **Cơ chế kích hoạt kép (Hybrid Trigger)**:
  - Khi có vé mới vào: Gửi tín hiệu không khóa qua `triggerCh` để xử lý ghép cặp ngay lập tức ($0\text{ms}$ delay).
  - Ticker chu kỳ 250ms: Chạy đều đặn quét cặp đôi và kiểm tra vé lẻ quá hạn 5 phút.
- **Lua Script Ghép Cặp Nguyên Tử (Atomic PvP Pairing)**:
  - Duyệt ZSet, tìm 2 vé đầu tiên có `user_id` khác nhau.
  - Gọi lệnh `ZREM` xóa cả 2 vé khỏi ZSet và trả về kết quả trong cùng 1 transaction CPU của Redis.
  - Ngăn ngừa 100% tình trạng Race Condition khi backend scale ngang nhiều pods.
- **Lua Script Bốc Vé Bot (Atomic Bot Fallback)**:
  - Sử dụng `ZRANGEBYSCORE` với điều kiện `score <= now - 5m`.
  - Rút chính xác 1 vé và xóa ngay khỏi ZSet để ghép với tài khoản Bot từ DB.

### 4.2. `MatchTimeoutWorker` (Quản lý Timeout)
- Chạy nền định kỳ 15 giây:
  - **Quét 1 (Check-in No-Show)**: Tìm các trận `PENDING_READY` có `now >= ready_deadline`. Gọi `CancelOnNoShow`: Chuyển trận sang `CANCELLED`, đánh dấu cả 2 bên `FORFEITED`, phát `SoloMatchCancelled`.
  - **Quét 2 (In-Progress Ceiling Safety Net)**: Tìm các trận `IN_PROGRESS` bắt đầu trước mốc `now - 35m`. Gọi `TimeoutInProgress`: Xử thắng cho bên đã nộp, xử thua cho bên chưa nộp, hoặc hủy trận nếu cả 2 cùng bỏ rơi.

### 4.3. `MatchResolver` (Domain Service)
- Phân định kết quả chung cuộc dựa trên thể thức:
  - `ChallengeTypeReps` hoặc `ChallengeTypeDuration`:
    - Nếu `Score1 > Score2` $\rightarrow$ P1 thắng (`WIN`), P2 thua (`LOSE`).
    - Nếu `Score2 > Score1` $\rightarrow$ P2 thắng (`WIN`), P1 thua (`LOSE`).
    - Nếu `Score1 == Score2` $\rightarrow$ Kết quả hòa (`DRAW`), cả 2 nhận `DRAW`.

---

## 5. Chuẩn Hóa API Protobuf & ConnectRPC

Toàn bộ dịch vụ được sinh mã từ file Protobuf duy nhất tại `/proto/contracts/core/solo_match/v1/service/solo_match_service.proto`:
- `rpc JoinQueue(JoinQueueRequest) returns (stream JoinQueueResponse);`
- `rpc CancelQueue(CancelQueueRequest) returns (CancelQueueResponse);`
- `rpc GetMatch(GetMatchRequest) returns (GetMatchResponse);`
- `rpc GetMatchHistory(GetMatchHistoryRequest) returns (GetMatchHistoryResponse);`
- Tương thích song song cả **gRPC HTTP/2 thuần** và **ConnectRPC (HTTP/1.1 & HTTP/2 JSON/Protobuf)**.
