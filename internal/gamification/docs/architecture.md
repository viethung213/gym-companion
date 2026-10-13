# Kiến Trúc Module Gamification (Architecture Specification)

Tài liệu này đặc tả kiến trúc tổng thể, ranh giới Bounded Context và các tầng kiến trúc của module **Gamification** trong hệ thống FITAI.

---

## 1. Vị Trí Trong Hệ Thống Modular Monolith

FITAI được xây dựng theo mô hình **Domain-Encapsulated Modular Monolith**. Mỗi Bounded Context được cô lập thành một thư mục độc lập dưới `/internal/<module_name>/`.

Module **Gamification** đóng vai trò là phân hệ bổ trợ (**Supporting Domain**), tiếp nhận các sự kiện nghiệp vụ từ hệ thống để vận hành cơ chế tích lũy kinh nghiệm (XP & Cấp độ), chuỗi ngày tập luyện (Streaks), bảng xếp hạng toàn hệ thống (Leaderboards), và hệ thống huy hiệu/thành tựu (Badges & Achievements):

```mermaid
flowchart TD
    subgraph CoreDomains["Inbound Events (Nguồn sự kiện)"]
        AU["auth<br/>(Sự kiện đăng ký tài khoản userRegistered)"]
        WE["workout_execution<br/>(Sự kiện hoàn thành buổi tập)"]
        NU["nutrition<br/>(Sự kiện đạt mục tiêu calo/protein)"]
        PR["profile<br/>(Sự kiện hoàn tất hồ sơ)"]
        SO["social<br/>(Sự kiện chia sẻ hoạt động)"]
    end

    subgraph EventBus["Kafka Event Bus (CloudEvents 1.0)"]
        TOPIC_IN["Các Topic Sự Kiện Hệ Thống"]
        TOPIC_GA["gamification.events"]
    end

    subgraph GamificationContext["internal/gamification/ (Gamification Context)"]
        direction TB
        G_CONSUMER["Kafka Consumers"]
        G_APP["Application CQRS"]
        G_DOMAIN["Domain Layer<br/>(XP, Streaks, Badges)"]
        G_REPO["Persistence<br/>(Schema: gamification.*)"]
        G_OUTBOX["Transactional Outbox"]
    end

    AU & WE & NU & PR & SO --> TOPIC_IN
    TOPIC_IN --> G_CONSUMER
    G_CONSUMER --> G_APP --> G_DOMAIN --> G_REPO
    G_DOMAIN --> G_OUTBOX --> TOPIC_GA
```

---

## 2. Kiến Trúc Hexagonal (Ports & Adapters)

Module Gamification tuân thủ mô hình **Ports & Adapters**, giữ tầng Domain hoàn toàn sạch và độc lập ở mức high-level:

```text
internal/gamification/
├── domain/                      # LÕI NGHIỆP VỤ (Pure Go - Không phụ thuộc DB, Framework)
│   ├── aggregate/               # Quản lý trạng thái và quy tắc nghiệp vụ XP, Streak, Badges
│   ├── entity/                  # Các thực thể nghiệp vụ (Lịch sử điểm, Lịch sử chuỗi)
│   ├── vo/                      # Value Objects (Cấp độ, Điểm thưởng, Loại hoạt động)
│   ├── event/                   # Domain Events nội bộ
│   └── repository/              # Port Interfaces (Định nghĩa hợp đồng lưu trữ)
│
├── application/                 # TẦNG ĐIỀU PHỐI (Use Cases)
│   ├── command/                 # Xử lý các tác vụ ghi (Cộng XP, Cập nhật Streak, Lưu snapshot)
│   ├── query/                   # Xử lý các tác vụ đọc (Thông tin XP, Chuỗi ngày, Bảng xếp hạng)
│   └── port/                    # Output Ports giao tiếp ngoài (Transaction, Outbox)
│
├── infrastructure/              # TẦNG KỸ THUẬT (Driven Adapters)
│   ├── persistence/             # Hiện thực hóa lưu trữ (PostgreSQL GORM)
│   └── worker/                  # Outbox Worker quét đẩy sự kiện ra Kafka
│
└── transport/                   # TẦNG GIAO DIỆN (Driving Adapters)
    ├── grpc/                    # ConnectRPC Service Handlers
    └── consumer/                # Kafka Message Consumers
        ├── user_registered_consumer.go        # Khởi tạo tức thì hồ sơ game hóa khi đăng ký
        ├── activity_ingestion_coordinator.go   # Điều phối tuần tự sự kiện buổi tập (XP -> Streak)
        └── nutrition_event_consumer.go        # Consumer đạt mục tiêu dinh dưỡng
```

---

## 3. Ranh Giới Dữ Liệu & Giao Tiếp

* **Schema Isolation**: Toàn bộ dữ liệu của Gamification được cô lập trong PostgreSQL schema riêng (`gamification.*`). Không thực hiện truy vấn `JOIN` chéo sang schema của các module khác.
* **Transactional Outbox Pattern**: Mọi sự kiện phát sinh từ Gamification đều được lưu trữ cùng transaction với dữ liệu nghiệp vụ và đẩy bất đồng bộ ra Kafka topic `gamification.events`.
* **Idempotency**: Các Consumer phía tiếp nhận dữ liệu luôn kiểm tra tính lũy đẳng (Idempotent Consumer qua khóa duy nhất nghiệp vụ hoặc Outbox Log Guard) để đảm bảo không xử lý lặp lại sự kiện.

---

## 4. Chuẩn Transactional Outbox Dùng Chung (Module-wide Outbox - SSOT)

> [!IMPORTANT]
> Mục 4 này là **Nguồn Chân Lý Duy Nhất (Single Source of Truth - SSOT)** cho hạ tầng Transactional Outbox Pattern của toàn bộ module Gamification. Tất cả các tài liệu tính năng con (`features/*/02-design.md`, `03-plan.md`) không được định nghĩa lại bảng DDL hay worker lặp lại, mà phải dẫn chiếu trực tiếp về đây.

Nhằm đảm bảo tính tin cậy tuyệt đối (At-Least-Once Delivery), loại trừ triệt để nguy cơ Dual-Write lỗi giữa Database và Kafka broker lúc mạng chập chờn, toàn bộ các tính năng con trong `internal/gamification/` đều chia sẻ chung một hạ tầng Transactional Outbox tại PostgreSQL schema `gamification.*`.

### 4.1 Schema DDL Chuẩn (`gamification.outbox` & `gamification.outbox_log`)

```sql
-- Bảng Outbox chính: Lưu trữ tạm thời các Domain Events cùng Transaction nghiệp vụ
CREATE TABLE IF NOT EXISTS gamification.outbox (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_id UUID NOT NULL UNIQUE,
    aggregate_type VARCHAR(255),
    aggregate_id VARCHAR(255),
    event_type VARCHAR(255) NOT NULL,
    payload JSONB NOT NULL,
    partition_key VARCHAR(255) NOT NULL,             -- Khóa phân vùng Kafka (= user_id)
    published BOOLEAN NOT NULL DEFAULT FALSE,
    published_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    status VARCHAR(50) NOT NULL DEFAULT 'PENDING',  -- PENDING | PROCESSING | PUBLISHED | FAILED | DEAD_LETTER
    retry_count INT NOT NULL DEFAULT 0,
    max_retries INT NOT NULL DEFAULT 5,
    last_error TEXT,
    locked_until TIMESTAMPTZ
);

-- Chỉ mục Partial tối ưu cho Worker quét batch hiệu năng cao, bỏ qua các bản ghi đã xuất bản và dead-letter
CREATE INDEX IF NOT EXISTS idx_gamification_outbox_unprocessed
    ON gamification.outbox (created_at ASC)
    WHERE published = FALSE AND status != 'DEAD_LETTER';

-- Bảng Outbox Log: Lưu vết lịch sử xuất bản phục vụ Audit và Idempotent Consumer
CREATE TABLE IF NOT EXISTS gamification.outbox_log (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_id UUID NOT NULL,
    event_type VARCHAR(255) NOT NULL,
    payload JSONB NOT NULL,
    partition_key VARCHAR(255) NOT NULL,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    status VARCHAR(50) NOT NULL,                     -- PUBLISHED | FAILED
    error_message TEXT
);

CREATE INDEX IF NOT EXISTS idx_gamification_outbox_log_event_status
    ON gamification.outbox_log (event_id, status);

CREATE UNIQUE INDEX IF NOT EXISTS ux_gamification_outbox_log_event_id
    ON gamification.outbox_log (event_id);
```

### 4.2 Cơ Chế Worker & Quét Khóa (`SELECT ... FOR UPDATE SKIP LOCKED`)

Background Worker (`internal/gamification/infrastructure/worker/outbox_worker.go`) vận hành theo chu kỳ:
1. **Claim Batch**: Quét các bản ghi chưa xuất bản bằng câu lệnh non-blocking:
   ```sql
   SELECT id, event_id, event_type, payload, partition_key, retry_count, max_retries 
   FROM gamification.outbox
   WHERE published = FALSE AND status != 'DEAD_LETTER' AND (status = 'PENDING' OR locked_until < NOW())
   ORDER BY created_at ASC
   LIMIT 100
   FOR UPDATE SKIP LOCKED;
   ```
2. **Locking Window**: Đánh dấu `status = 'PROCESSING'` kèm thời hạn `locked_until = NOW() + 30s`.
3. **Kafka Publish**: Đẩy mẻ sự kiện sang Kafka topic `gamification.events` với Partition Key = `partition_key` (luôn là `user_id` để bảo toàn thứ tự).
4. **Mark Published & Archiving**: Đánh dấu `published = TRUE`, `status = 'PUBLISHED'` và ghi nhật ký vào `gamification.outbox_log`.
5. **Cơ chế chống Poison Pill**: Nếu publish thất bại, tăng `retry_count += 1` và cập nhật `last_error`. Nếu `retry_count >= max_retries`, chuyển `status = 'DEAD_LETTER'` và phát cảnh báo metric; worker tiếp tục xử lý các sự kiện phía sau mà không bị kẹt vô tận.

### 4.3 Chuẩn CloudEvents Envelope (CloudEvents 1.0 JSON)

Mọi bản ghi `payload` trong `gamification.outbox` đều đóng gói chuẩn CloudEvents và sử dụng contract hợp nhất tại `proto/contracts/supporting/gamification/v1/event/gamification_events.proto`:
```json
{
  "specversion": "1.0",
  "id": "evt_01JABC1234XYZ...",
  "source": "services/gamification",
  "type": "contracts.supporting.gamification.v1.xpEarned",
  "datacontenttype": "application/json",
  "time": "2026-10-11T12:00:00Z",
  "partitionkey": "usr_9988-7766-5544",
  "data": {
    "userId": "usr_9988-7766-5544",
    "deltaXp": 100,
    "newXp": 1500,
    "newLevel": 6
  }
}
```

### 4.4 Danh Mục Sự Kiện

#### Sự Kiện Module Gamification Phát Sinh (`gamification.events`)
Tất cả các sự kiện bên dưới được định nghĩa tách riêng theo chuẩn CloudEvents 1.0 trong `proto/contracts/supporting/gamification/v1/event/` (tuân thủ quy chuẩn camelCase theo `docs/04_event_driven_standards.md`):

| Tên Sự Kiện (`event_type`) | Bounded Context Phát Sinh | Mục Đích & Module Lắng Nghe |
| :--- | :--- | :--- |
| `contracts.supporting.gamification.v1.xpEarned` | `xp-and-levels` | Ghi nhận tích lũy XP. `analytics`, `notification`. |
| `contracts.supporting.gamification.v1.userLeveledUp` | `xp-and-levels` | Người dùng thăng cấp. `notification` (Push/Popup), `profile` (Cập nhật huy hiệu), `social` (Đăng bài tự động). |
| `contracts.supporting.gamification.v1.streakAdvanced` | `streaks-and-habits` | Tăng chuỗi ngày. `notification`. |
| `contracts.supporting.gamification.v1.streakReset` | `streaks-and-habits` | Đứt chuỗi ngày sau ngày nghỉ. `notification`. |
| `contracts.supporting.gamification.v1.streakMilestoneReached` | `streaks-and-habits` | Chạm mốc chuỗi kỷ luật (7, 14, 30, 50, 100, 365 ngày). `notification`, `social`. |
| `contracts.supporting.gamification.v1.badgeUnlocked` | `badges` | Người dùng mở khóa huy hiệu mới. `notification` (Push/Popup vinh danh), `social` (Khoe bài). |

#### Sự Kiện Ngoại Vi Lắng Nghe (Inbound Events)
| Tên Sự Kiện | Module Phát | Xử Lý Phía Gamification |
| :--- | :--- | :--- |
| `contracts.generic.auth.v1.userRegistered` | `auth` | Khởi tạo tức thì bộ đôi hồ sơ Gamification (`user_xp = 0, level = 1`, `user_streak = 0`) và snapshot (`display_name`, `avatar_url`). |
| `contracts.core.workout_execution.v1.event.WorkoutSessionCompleted` | `workout_execution` | Điều phối qua `ActivityIngestionCoordinator` để cộng XP và tăng chuỗi Streak. |
| `contracts.core.workout_execution.v1.event.NewPersonalRecordAchieved` | `workout_execution` | Thưởng thêm PR Bonus (+25 XP) khi lập kỷ lục mới. |
| `contracts.core.nutrition.v1.event.DailyNutritionGoalAchieved` | `nutrition` | Thưởng $+30$ XP và duy trì Streak dinh dưỡng trong ngày nghỉ. |

---

## 5. Chuẩn Hóa Xử Lý Bất Đồng Bộ Đa Aggregate & Khử Deadlock (Activity Ingestion Coordinator)

### 5.1 Vấn Đề Khi Nhận Sự Kiện Hoàn Thành Buổi Tập (`WorkoutSessionCompleted`)
Khi một buổi tập kết thúc, hệ thống phát ra CloudEvent `contracts.core.workout_execution.v1.WorkoutSessionCompleted`. Trong module Gamification, sự kiện này tác động đồng thời lên **2 Aggregate độc lập**:
1. `UserXp`: Tính toán và cộng điểm kinh nghiệm XP, nâng cấp level.
2. `UserStreak`: Cập nhật chuỗi ngày kỷ luật tập luyện liên tục.

### 5.2 Khử Triệt Để Deadlock & Tối Ưu Hệ Số Thưởng: 3 Bước Tuần Tự
- **Rủi ro của Mega-Transaction đơn (Ôm cả 2 bảng trong 1 transaction)**:
  Nếu mở 1 Database Transaction duy nhất giữ khóa tuần tự `user_xp -> user_streak`:
  - Chiếm dụng kết nối database pool qua nhiều round-trips SQL, gây nghẽn pool ở giờ cao điểm (The 3 AM Test).
  - Xảy ra Deadlock kinh điển khi luồng dinh dưỡng đồng thời cập nhật `user_streak` hoặc `user_xp`.
- **Giải Pháp Chuẩn Hóa (Causal Consistency & Atomic Steps)**:
  - Gamification triển khai **1 Kafka Consumer duy nhất** lắng nghe `WorkoutSessionCompleted`.
  - Consumer đọc `duration_seconds` trực tiếp từ Proto Contract và chuyển giao payload vào **`ActivityIngestionCoordinator`** (Application Service).
  - Coordinator thực thi theo thứ tự nhân quả tối ưu:

```text
BƯỚC 1: ATOMIC STREAK TRANSACTION (< 2 ms)
  BEGIN;
    SELECT * FROM gamification.user_streak WHERE user_id = $1 FOR UPDATE;
    --> Cập nhật chuỗi ngày liên tục theo giờ địa phương (UserStreak)
    --> Ghi nhận Outbox streakAdvanced (và streakMilestoneReached nếu chạm mốc)
  COMMIT;  -- Nhả khóa user_streak ngay lập tức. Lấy streak mới nhất vừa cập nhật.

BƯỚC 2: ATOMIC XP TRANSACTION (< 2 ms)
  BEGIN;
    --> Kiểm tra Idempotency phiên tập: HasSession(user_id, session_id)
    --> Áp dụng hệ số Streak Multiplier chuẩn xác cho chính buổi tập hôm nay (1.1x nếu streak >= 3, 1.2x nếu streak >= 7)
    SELECT * FROM gamification.user_xp WHERE user_id = $1 FOR UPDATE;
    --> Cập nhật điểm XP, kiểm tra Level Up (UserXp)
    --> Ghi nhận Outbox xpEarned (và userLeveledUp nếu thăng cấp)
  COMMIT;  -- Nhả khóa user_xp ngay lập tức.

BƯỚC 3: BATCH BADGES TRANSACTION (Nếu có huy hiệu đủ điều kiện)
  BEGIN;
    --> Gom tất cả huy hiệu hợp lệ (FIRST_WORKOUT, PR_BREAKER, STREAK_*, LEVEL_*)
    --> Kiểm tra khoảng thăng cấp (oldLevel < X && newLevel >= X) để không bị sót khi nhảy cấp
    --> INSERT INTO gamification.user_badges BATCH ON CONFLICT DO NOTHING;
    --> Ghi nhận Outbox badgeUnlocked BATCH
  COMMIT;
```

### 5.3 Tính Ưu Việt Của Kiến Trúc 3 Bước Tuần Tự
1. **0% Nguy Cơ Deadlock:** Tại bất kỳ thời điểm nào, tiến trình không bao giờ giữ 2 khóa dòng cùng lúc. Triệt tiêu hoàn toàn điều kiện chờ vòng (circular wait condition) với bất kỳ luồng Dinh dưỡng nào.
2. **Giải Phóng DB Connection Pool:** Mỗi bước chỉ chiếm kết nối pool trong tích tắc ($< 2\text{ ms}$) rồi nhả ngay, bảo vệ pool an toàn tuyệt đối dưới tải cao lúc 3 AM.
3. **Phần Thưởng Công Bằng & Nhất Quán:** Chuỗi kỷ luật tăng trước giúp buổi tập hôm nay được hưởng ngay lập tức hệ số nhân XP tương ứng.
4. **Batch Upsert Huy Hiệu:** Toàn bộ huy hiệu đạt được trong phiên tập được ghi nhận trong 1 transaction duy nhất, không mở nhiều transaction lẻ tẻ.

---

## 6. Khởi Tạo Hồ Sơ Game Hóa Tức Thì (Eager Initialization via UserRegistered)

### 6.1 Vấn Đề Của Khởi Tạo Lười (Lazy Initialization Anti-Pattern)
Nếu chỉ tạo bản ghi khi người dùng hoàn thành buổi tập đầu tiên (Lazy Initialization):
- Khi user vừa đăng ký tài khoản, mở app vào tab Bảng Xếp Hạng (Leaderboards) hoặc Chuỗi Ngày (Streaks), database chưa hề có bản ghi của user đó.
- Truy vấn tính thứ hạng cá nhân (`SELECT COUNT(*) + 1 FROM gamification.user_xp WHERE xp > :my_xp`) bị lỗi hoặc trả về kết quả sai do `:my_xp = NULL`.
- Truy vấn chuỗi phải xử lý các ngoại lệ `RecordNotFound`, làm tăng độ phức tạp của code tầng Query và nguy cơ crash ứng dụng lúc 3 AM.

### 6.2 Giải Pháp: Eager Initialization Với UserRegistered Consumer
Gamification triển khai `UserRegisteredConsumer` lắng nghe sự kiện `contracts.generic.auth.v1.userRegistered` từ topic `auth.events` (Consumer Group: `gamification-user-registered-group`):
- Ngay khi tài khoản được tạo ở Auth, Gamification thực thi 1 Local Database Transaction khởi tạo đồng thời với tính lũy đẳng (`ON CONFLICT (user_id) DO NOTHING`):
  1. `gamification.user_xp`: `xp = 0`, `level = 1`
  2. `gamification.user_streak`: `current_streak = 0`, `longest_streak = 0`
  3. `gamification.user_snapshots`: `display_name = full_name`, `avatar_url = avatar_url`
- **Kết quả**: 100% gymer tồn tại trong hệ thống đều có sẵn trạng thái Game hóa từ giây đầu tiên. Các API tra cứu (`GetStreak`, `GetMyXp`, `GetGlobalLeaderboard`) hoạt động mượt mà, nhất quán tuyệt đối, loại bỏ hoàn toàn các nhánh code xử lý dữ liệu khuyết thiếu.

