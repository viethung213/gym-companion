# Thiết Kế Kỹ Thuật: Bảng Xếp Hạng Toàn Hệ Thống Theo Điểm XP (XP Leaderboard)

## 1. Kiến Trúc Hexagonal (Ports & Adapters)

Tính năng **Bảng Xếp Hạng Toàn Hệ Thống (XP Leaderboard)** là một Use Case thuần đọc (**Read-Only Query Use Case**), tuân thủ nghiêm ngặt mô hình Ports & Adapters:

```mermaid
flowchart TD
    subgraph DrivingAdapters["Transport Layer (Driving)"]
        GRPC["LeaderboardGrpcHandler<br/>(ConnectRPC)"]
    end

    subgraph ApplicationLayer["Application Layer (Use Case)"]
        HANDLER["GetGlobalLeaderboardQueryHandler"]
        PORT_REPO["LeaderboardRepository (Output Port)"]
        PORT_PROFILE["ProfileReaderPort (Output Port)"]
    end

    subgraph DrivenAdapters["Infrastructure Layer (Driven)"]
        REPO_IMPL["PostgresLeaderboardRepository<br/>(GORM / raw SQL)"]
        PROFILE_ADAPTER["ProfileReaderAdapter<br/>(In-process Cross-Module Adapter)"]
    end

    subgraph External["External Modules & Storage"]
        DB[(PostgreSQL<br/>gamification.user_xp)]
        PROFILE_MOD["Profile Module<br/>(profile.users)"]
    end

    GRPC -->|Dispatch Query| HANDLER
    HANDLER -->|Query Top 50 & Rank| PORT_REPO
    HANDLER -->|Enrich Display Info| PORT_PROFILE
    PORT_REPO -.->|Implement| REPO_IMPL
    PORT_PROFILE -.->|Implement| PROFILE_ADAPTER
    REPO_IMPL -->|Index Scan| DB
    PROFILE_ADAPTER -->|In-memory Call| PROFILE_MOD
```

### Ranh giới trách nhiệm giữa các tầng:
1. **Transport Layer**: Tiếp nhận request ConnectRPC `GetGlobalLeaderboard`, xác thực Bearer Token, trích xuất `user_id` và chuyển giao cho Application Query Handler.
2. **Application Layer**:
   - `GetGlobalLeaderboardQueryHandler`: Điều phối truy vấn Top 50 từ cơ sở dữ liệu và gọi `ProfileReaderPort` để làm giàu thông tin tên/ảnh của gymer.
   - Định nghĩa hai Output Ports: `LeaderboardRepository` và `ProfileReaderPort`.
3. **Infrastructure Layer**:
   - `PostgresLeaderboardRepository`: Thực thi các truy vấn SQL tối ưu hóa trên bảng `gamification.user_xp`.
   - `ProfileReaderAdapter`: Hiện thực hóa `ProfileReaderPort` bằng cách gọi hàm in-memory sang Application Service của module `profile` với timeout $500\text{ ms}$ và cơ chế fallback an toàn.

---

## 2. Thiết Kế Cơ Sở Dữ Liệu & Chỉ Mục (Database & Index DDL)

Tính năng vận hành trực tiếp trên bảng `gamification.user_xp` sẵn có từ module `xp-and-levels`, không tạo bảng trung gian hay nhân bản dữ liệu.

### Chỉ mục B-Tree Composite Index
Để đáp ứng chuẩn hiệu năng "The 3 AM Test" (độ trễ $< 5\text{ ms}$), hệ thống kích hoạt chỉ mục kết hợp:

```sql
-- Migration: Tạo index phục vụ truy vấn Bảng Xếp Hạng
CREATE INDEX IF NOT EXISTS idx_user_xp_leaderboard 
ON gamification.user_xp (xp DESC, updated_at ASC);
```

### Phân tích cơ chế truy vấn vật lý (Physical Query Mechanics):

#### Truy vấn 1: Lấy danh sách Top 50 dẫn đầu
```sql
SELECT user_id, xp, level, updated_at
FROM gamification.user_xp
ORDER BY xp DESC, updated_at ASC
LIMIT 50;
```
* **Kế hoạch thực thi (Execution Plan)**: PostgreSQL sử dụng **Index Scan** trên `idx_user_xp_leaderboard`. Engine duyệt tuần tự đúng 50 leaf nodes đầu tiên của cây B-tree và dừng lại ngay lập tức (không cần sắp xếp lại toàn bộ bảng).
* **Thời gian thực thi**: $< 2\text{ ms}$.

#### Truy vấn 2: Xác định vị trí người dùng gọi API (`my_rank`)
Nếu người dùng hiện tại nằm trong Top 50, vị trí chính là `index + 1` của mảng kết quả ($O(1)$).
Nếu người dùng nằm ngoài Top 50, thực thi câu lệnh đếm có chỉ mục:
```sql
SELECT COUNT(*) + 1 AS my_rank
FROM gamification.user_xp
WHERE xp > :my_xp 
   OR (xp = :my_xp AND updated_at < :my_updated_at)
   OR (xp = :my_xp AND updated_at = :my_updated_at AND user_id < :my_user_id);
```
* **Kế hoạch thực thi**: Nhờ index trên `(xp DESC, updated_at ASC)`, engine thực hiện **Index Only Scan** để đếm số lượng bản ghi thỏa mãn điều kiện.
* **Thời gian thực thi**: P99 $< 20\text{ ms}$ (áp dụng trong giai đoạn MVP $< 50,000$ users theo ADR-0001).

### 2.3 Chiến Lược Hiệu Năng Phân Tầng Theo Quy Mô (Tiered Scaling Strategy)
Nhằm kiểm soát rủi ro "The 3 AM Test" khi số lượng người dùng vượt mốc MVP:
* **Giai đoạn 1 (MVP, $< 50,000$ users)**: Truy vấn trực tiếp PostgreSQL `COUNT(*) + 1` qua B-tree index `idx_user_xp_leaderboard`.
* **Giai đoạn 2 (Growth, $50,000\text{–}500,000$ users)**: Top 50 vẫn quét B-tree index ($< 2\text{ ms}$). Vị trí ngoài Top 50 (`my_rank`) chuyển sang đọc từ **Materialized View** (`leaderboard_ranks_mv`) được refresh định kỳ mỗi 5–15 phút.
* **Giai đoạn 3 (Scale, $> 500,000$ users)**: Chuyển `LeaderboardRepository` sang **Redis Sorted Set (`ZREVRANK`)**, đồng bộ bất đồng bộ qua Transactional Outbox Worker.

---

## 3. Luồng Xử Lý Nghiệp Vụ (Sequence Diagram)

### Luồng thành công chính (Happy Path with Profile Enrichment):

```mermaid
sequenceDiagram
    autonumber
    actor Client as Gym Mobile App
    participant Grpc as LeaderboardGrpcHandler
    participant App as GetGlobalLeaderboardQueryHandler
    participant Repo as PostgresLeaderboardRepository
    participant DB as PostgreSQL (gamification.user_xp)
    participant Adapter as ProfileReaderAdapter
    participant Profile as Profile Module (In-Memory)

    Client->>Grpc: GetGlobalLeaderboard(limit = 50) [Bearer Token]
    Grpc->>App: Handle(ctx, Query{UserID: myID, Limit: 50})
    
    rect rgb(240, 248, 255)
        Note over App, DB: Bước 1: Truy vấn Top 50 & Rank từ Gamification DB
        App->>Repo: GetTopRankedUsers(ctx, 50)
        Repo->>DB: SELECT ... ORDER BY xp DESC, updated_at ASC LIMIT 50
        DB-->>Repo: Return 50 rows
        Repo-->>App: top50Entries
        
        App->>Repo: GetUserRank(ctx, myID, myXp, myUpdatedAt)
        Repo->>DB: SELECT COUNT(*) + 1 WHERE ...
        DB-->>Repo: Return myRank
        Repo-->>App: myRank
    end

    rect rgb(255, 250, 240)
        Note over App, Profile: Bước 2: Làm giàu tên & ảnh qua Port nội bộ (In-memory)
        App->>Adapter: BatchGetSummaries(ctxWithTimeout, [50 UserIDs + myID])
        Adapter->>Profile: BatchGetProfiles(ctx, userIDs)
        Profile-->>Adapter: map[UUID]ProfileSummary{DisplayName, AvatarURL}
        Adapter-->>App: profileMap
    end

    Note over App: Bước 3: Ghép dữ liệu & đóng gói Response trong RAM
    App-->>Grpc: QueryResult{Items: []LeaderboardEntry, MyStanding: LeaderboardEntry}
    Grpc-->>Client: 200 OK (GetGlobalLeaderboardResponse)
```

### Cơ chế phòng vệ khi Module Profile gặp sự cố (Resilience & Fallback Guard):
* Lệnh gọi sang `ProfileReaderPort` được bọc bởi `context.WithTimeout(ctx, 500*time.Millisecond)`.
* Nếu module `profile` bị lỗi, timeout hoặc trả về thiếu bản ghi của một `user_id`:
  * Tầng Application **không làm crash request**.
  * Tự động áp dụng giá trị fallback mặc định: `display_name = "Gymer"`, `avatar_url = ""`.
  * Bảng xếp hạng vẫn được hiển thị nguyên vẹn cho người dùng.

---

## 4. Hợp Đồng Giao Diện API (Protobuf ConnectRPC)

Tệp hợp đồng dịch vụ: `proto/contracts/supporting/gamification/v1/service/gamification_service.proto`
Tệp thông điệp: `proto/contracts/supporting/gamification/v1/message/gamification_messages.proto`

```protobuf
syntax = "proto3";

package contracts.supporting.gamification.v1.service;

import "google/api/annotations.proto";
import "protoc-gen-openapiv2/options/annotations.proto";
import "contracts/supporting/gamification/v1/message/gamification_messages.proto";

option go_package = "github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/gamification/v1/service;gamificationservicev1";

service GamificationService {
  // Tra cứu bảng vàng Top 50 và thứ hạng cá nhân theo thời gian thực
  rpc GetGlobalLeaderboard(contracts.supporting.gamification.v1.message.GetGlobalLeaderboardRequest) 
      returns (contracts.supporting.gamification.v1.message.GetGlobalLeaderboardResponse) {
    option (google.api.http) = {
      get: "/api/v1/gamification/leaderboard"
    };
  }
}
```

```protobuf
syntax = "proto3";

package contracts.supporting.gamification.v1.message;

option go_package = "github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/gamification/v1/message;gamificationmessagev1";

message GetGlobalLeaderboardRequest {
  // Số lượng thành viên tối đa cần lấy (Mặc định: 50, Tối đa: 100)
  int32 limit = 1;
}

message LeaderboardEntry {
  // Vị trí thứ hạng (Bắt đầu từ 1)
  int64 rank = 1;
  // ID định danh người dùng
  string user_id = 2;
  // Tên hiển thị công khai (Lấy từ module profile)
  string display_name = 3;
  // Đường dẫn ảnh đại diện (Lấy từ module profile)
  string avatar_url = 4;
  // Tổng điểm kinh nghiệm tích lũy trọn đời
  int64 xp = 5;
  // Cấp độ trọn đời hiện tại (Level 1-100)
  int32 level = 6;
}

message GetGlobalLeaderboardResponse {
  // Danh sách Top người dùng dẫn đầu hệ thống (Sắp xếp theo rank tăng dần)
  repeated LeaderboardEntry items = 1;
  // Vị trí và thành tích của chính người dùng đang gọi API
  LeaderboardEntry my_standing = 2;
}
```

---

## 5. Cấu Trúc Mã Nguồn (Code Organization)

> Leaderboard là một **CQRS Read Model / Query Service** của phân hệ XP, truy vấn trực tiếp trên bảng `gamification.user_xp`. Cấu trúc tổng thể của toàn module xem tại [architecture.md#2](../../architecture.md#2-kiến-trúc-hexagonal-ports--adapters).

```text
internal/gamification/
├── application/
│   ├── port/
│   │   ├── leaderboard_repository.go     # Output Port truy vấn DB bảng user_xp
│   │   └── profile_reader.go             # Output Port nạp thông tin tên/avatar
│   └── query/
│       ├── get_global_leaderboard.go     # Query DTO & Handler chính
│       └── get_global_leaderboard_test.go# Unit tests với mock repository & port
├── infrastructure/
│   ├── persistence/
│   │   └── leaderboard_repository.go     # GORM implementation đọc user_xp qua index B-tree
│   └── adapter/
│       └── profile_adapter.go            # In-process adapter gọi sang module profile (kèm timeout & fallback)
└── transport/
    └── grpc/
        └── leaderboard_handler.go        # ConnectRPC Server Handler
```
