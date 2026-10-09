# Implementation Plan: ELO Rating & Rank Tiers

## Goal and scope
Hiện thực hóa tính năng **ELO Rating & Rank Tiers** cho module Gamification (`internal/gamification/`), cung cấp thước đo năng lực thể chất chuẩn hóa duy nhất ($1,000 \rightarrow 3,000$ ELO) kết hợp phân cấp 5 bậc hạng (Bronze $\rightarrow$ Diamond), cơ chế thưởng kỷ luật dinh dưỡng, và khóa bi quan chống xung đột đồng thời.

Tài liệu này dựa trên các đặc tả và quyết định kiến trúc đã được phê duyệt:
- [01-spec.md](./01-spec.md): Yêu cầu nghiệp vụ và tiêu chí nghiệm thu.
- [02-design.md](./02-design.md): Thiết kế kỹ thuật Hexagonal, Database DDL, và Sequence Flow.
- [ADR-0001: Thưởng Dinh Dưỡng](./adr/ADR-0001-incorporate-nutrition-adherence-bonus-into-elo.md)
- [ADR-0002: Hoãn Solo PvP](./adr/ADR-0002-defer-solo-pvp-elo-scoring-mechanism.md)
- [ADR-0003: Ánh Xạ Trực Tiếp Không Trạng Thái](./adr/ADR-0003-stateless-direct-rank-mapping-without-demotion-shield.md)
- [ADR-0004: Khóa Bi Quan FOR UPDATE](./adr/ADR-0004-use-pessimistic-row-locking-for-elo-updates.md)
- [ADR-0005: Trần Cứng 3000 ELO](./adr/ADR-0005-establish-elo-range-and-hard-cap.md)

---

## Approach
Tuân thủ nghiêm ngặt các nguyên tắc kiến trúc cốt lõi của dự án:
1. **Contract-First (SSOT)**: Định nghĩa hợp đồng Protobuf tại `proto/contracts/supporting/gamification/v1/`, sinh mã nguồn tự động qua `buf generate`. Cấm viết tay router HTTP hoặc gRPC stubs.
2. **Hexagonal Architecture (Ports & Adapters)**: Tầng Domain (`internal/gamification/domain/`) hoàn toàn độc lập, không import GORM, Gin, hay DB tags. Interface được định nghĩa ở nơi sử dụng (Domain/Application), không định nghĩa ở Infrastructure.
3. **Database Schema Isolation**: Toàn bộ dữ liệu nằm trong schema riêng `gamification.*`. Sử dụng khóa bi quan `SELECT ... FOR UPDATE` cho các thao tác ghi để bảo vệ tính toàn vẹn ELO.
4. **Event-Driven Standards & Outbox Pattern**: Lắng nghe CloudEvents từ Kafka qua Idempotent Inbox Guard (`processed_events`). Xuất sự kiện qua Transactional Outbox Pattern (`outbox_events`).

---

## Phases

### Phase 1: API & Event Contracts (Protobuf & Buf Generation)
- **Khu vực tác động**: `proto/contracts/supporting/gamification/v1/` và `internal/gen/go/`
- **Tiền điều kiện**: Buf CLI được cấu hình sẵn trong repository (`buf.yaml`, `buf.gen.yaml`).
- **Nội dung bàn giao**:
  - `message/gamification_messages.proto`: Khai báo Enum `RankTier`, Message `GetMyEloRequest/Response`, `GetEloHistoryRequest/Response`.
  - `service/gamification_service.proto`: Khai báo RPC service `GamificationService`.
  - `event/elo_events.proto`: Khai báo các sự kiện CloudEvents `EloScoreUpdated`, `RankTierPromoted`, `RankTierDemoted`.
  - Chạy `buf generate` để sinh Go stubs tại `internal/gen/go/contracts/supporting/gamification/v1/`.
- **Nghiệm thu**: Lệnh `buf lint` pass $100\%$, mã nguồn Go sinh ra biên dịch không có lỗi cú pháp.

### Phase 2: Database Schema & Migration (PostgreSQL DDL)
- **Khu vực tác động**: `internal/shared/database/migrations/13-create-gamification-tables.sql`
- **Tiền điều kiện**: Đã có schema migration 01-12.
- **Nội dung bàn giao**:
  - Tạo schema `gamification`.
  - Tạo bảng `user_elo` với ràng buộc `CHECK (current_elo >= 1000 AND current_elo <= 3000)`.
  - Tạo bảng `elo_history` lưu vết biến động append-only kèm metadata.
  - Tạo bảng `processed_events` làm Idempotent Inbox Guard.
  - Tạo bảng `outbox_events` phục vụ Transactional Outbox.
  - Tạo các chỉ mục B-Tree: `idx_user_elo_ranking`, `idx_elo_history_user_created`, `idx_outbox_events_pending`.
- **Nghiệm thu**: Script SQL thực thi thành công trên PostgreSQL, các ràng buộc check và unique constraint hoạt động chuẩn xác.

### Phase 3: Domain Core & Unit Tests (Pure Go)
- **Khu vực tác động**: `internal/gamification/domain/`
- **Tiền điều kiện**: Phase 1 hoàn thành (có kiểu dữ liệu generated nếu cần).
- **Nội dung bàn giao**:
  - `vo/rank_tier.go`: Định nghĩa 5 bậc hạng và hàm thuần túy `DetermineRankTier(elo int32)`.
  - `service/elo_calculator.go`: Thuật toán tính $K$-factor, hiệu suất buổi tập, clamp biến động $[-25, +40]$, kẹp trần sàn $[1000, 3000]$.
  - `aggregate/user_elo.go`: Aggregate Root `UserElo` với các nghiệp vụ `ApplyWorkoutResult()`, `ApplyNutritionBonus()`, `ApplyInactivityDecay()`.
  - `event/elo_events.go`: Domain events nội bộ.
  - `repository/user_elo_repository.go`: Port interface cho Persistence.
- **Nghiệm thu**: Bộ Unit Tests cho Domain đạt độ bao phủ statement coverage $> 90\%$, bao gồm:
  - Test thăng hạng tức thì khi chạm mốc $1200, 1500, 1800, 2200$.
  - Test hạ hạng tức thì khi rớt điểm (ADR-0003).
  - Test chạm trần cứng 3000 ELO (ADR-0005).
  - Test thưởng dinh dưỡng $+3 \rightarrow +5$ ELO (ADR-0001).

### Phase 4: Infrastructure Persistence & Integration Tests
- **Khu vực tác động**: `internal/gamification/infrastructure/persistence/`
- **Tiền điều kiện**: Phase 2 và Phase 3 hoàn thành.
- **Nội dung bàn giao**:
  - Data Mapper: `ToDomain()` và `ToPersistence()` chuyển đổi giữa Aggregate và GORM/SQL Model.
  - `postgres/user_elo_repository.go`: Hiện thực hóa `FindByID`, `GetForUpdate` (sử dụng `SELECT ... FOR UPDATE`), `Save`.
  - `postgres/elo_history_repository.go`: Ghi log biến động điểm.
  - `postgres/inbox_repository.go`: Kiểm tra và đánh dấu sự kiện đã xử lý (`processed_events`).
  - `postgres/outbox_repository.go`: Ghi CloudEvents vào `outbox_events` trong cùng transaction.
- **Nghiệm thu**: Integration Tests kiểm thử transaction:
  - Test khóa `FOR UPDATE` tuần tự hóa 2 transaction đồng thời (ADR-0004).
  - Test Idempotency: Gửi 2 lần cùng một `event_id` không gây lỗi và không cộng trùng điểm.

### Phase 5: Application Layer (CQRS Use Cases)
- **Khu vực tác động**: `internal/gamification/application/`
- **Tiền điều kiện**: Phase 3 và Phase 4 hoàn thành.
- **Nội dung bàn giao**:
  - `command/process_workout_elo.go`: Điều phối Use Case hoàn thành buổi tập.
  - `command/process_nutrition_elo.go`: Điều phối Use Case thưởng dinh dưỡng hàng ngày.
  - `command/process_inactivity_decay.go`: Điều phối Use Case trừ điểm bất hoạt.
  - `query/get_my_elo.go`: Lấy ELO cá nhân và khoảng cách điểm tới bậc kế tiếp.
  - `query/get_elo_history.go`: Lấy danh sách lịch sử phân trang.
- **Nghiệm thu**: Unit Tests cho Application Layer giả lập Repository Mock, kiểm thử thành công các luồng happy path và error path.

### Phase 6: Transport Layer & Event Ingestion
- **Khu vực tác động**: `internal/gamification/transport/`
- **Tiền điều kiện**: Phase 5 hoàn thành.
- **Nội dung bàn giao**:
  - `consumer/workout_event_consumer.go`: Đăng ký lắng nghe topic `workout_execution.events`, giải mã CloudEvent `WorkoutSessionCompleted`.
  - `consumer/nutrition_event_consumer.go`: Đăng ký lắng nghe topic `nutrition.events`, giải mã CloudEvent `MealLogged`.
  - `grpc/gamification_handler.go`: Hiện thực hóa handler ConnectRPC theo interface sinh từ Buf stubs.
  - `worker/outbox_worker.go`: Background worker quét `outbox_events` và đẩy ra Kafka topic `gamification.events`.
- **Nghiệm thu**: Kiểm thử End-to-End: Bắn sự kiện giả lập qua Kafka, verify dữ liệu ELO trong database và sự kiện phát ra ở topic đích.

---

## Verification & TDD Strategy (Kiểm Thử Trọng Tâm Nghiệp Vụ)

Tuân thủ nguyên lý **Test-Driven Development (TDD)**:
- **The Iron Law**: Không viết code production khi chưa có failing test (`RED -> GREEN -> REFACTOR`).
- **Focus Logic**: Chỉ tập trung kiểm thử các bất biến nghiệp vụ (Business Invariants), thuật toán tính toán và ràng buộc giao dịch vật lý. **Tuyệt đối không viết test hình thức (Mocking the world)** cho các tầng trung gian không chứa logic (DTO mapper, getter/setter, CRUD râu ria).

### 4 Bộ Test Cốt Lõi (Core Logic Test Suites)

```text
 ┌────────────────────────────────────────────────────────────────────────┐
 │ 1. rank_tier_test.go (Domain VO)                                       │
 │    -> Kiểm tra logic chuyển bậc hạng tại các mốc biên (Boundary Logic) │
 ├────────────────────────────────────────────────────────────────────────┤
 │ 2. elo_calculator_test.go (Domain Service)                             │
 │    -> Kiểm tra công thức toán, K-factor, clamp [-25, +40], trần/sàn    │
 ├────────────────────────────────────────────────────────────────────────┤
 │ 3. user_elo_test.go (Domain Aggregate)                                 │
 │    -> Kiểm tra bất biến Aggregate, thăng/giáng hạng, chặn lặp dinh dưỡng│
 ├────────────────────────────────────────────────────────────────────────┤
 │ 4. repository_test.go (PostgreSQL Integration)                         │
 │    -> Kiểm tra khóa bi quan FOR UPDATE chống Lost Update & Inbox Guard │
 └────────────────────────────────────────────────────────────────────────┘
```

### Ma Trận Kịch Bản Logic Trọng Yếu

| Bộ Test | File Test | Kịch Bản Nghiệp Vụ Cần Bảo Vệ (Must-Have Assertions) |
| :--- | :--- | :--- |
| **Bậc Hạng** | `domain/vo/rank_tier_test.go` | **Biên chuyển bậc chính xác**:<br>- `1000..1199` = Bronze<br>- `1200` = Silver (chuyển bậc tức thì)<br>- `1500` = Gold<br>- `1800` = Platinum<br>- `2200` = Diamond<br>- Sàn cứng $1000$, trần cứng $3000$. |
| **Công Thức ELO** | `domain/service/elo_calculator_test.go` | **Thuật toán & Ràng buộc biên độ**:<br>- Hiệu suất tập $\rightarrow \Delta$ chính xác theo tỷ lệ volume & form.<br>- Buổi tập không AI (form = 0) $\rightarrow$ fallback chuẩn form $75\%$.<br>- Biến động bị clamp cứng trong $[-25, +40]$.<br>- Chạm trần $3000$ không được vượt; chạm sàn $1000$ không được rớt tiếp.<br>- $K$-factor suy giảm đúng theo bậc: Diamond ($K=10$) biến động nhỏ hơn Bronze ($K=32$). |
| **Vòng Đời Aggregate** | `domain/aggregate/user_elo_test.go` | **Bất biến trạng thái & Phát sinh sự kiện**:<br>- Tăng điểm vượt mốc $\rightarrow$ Tự động thăng hạng + phát sinh `RankTierPromoted`.<br>- Tụt điểm dưới mốc $\rightarrow$ Tự động giáng hạng + phát sinh `RankTierDemoted` (ADR-0003).<br>- Thưởng dinh dưỡng $+3$ ELO: nhận lần 1 thành công; nhận lần 2 trong cùng ngày bị từ chối.<br>- Bất hoạt $>14$ ngày $\rightarrow$ trừ $15$ ELO (không rớt dưới sàn $1000$). |
| **Khóa DB & Chống Trùng** | `infrastructure/persistence/postgres/repository_test.go` | **Bảo vệ toàn vẹn dữ liệu vật lý (The 3 AM Test)**:<br>- **Race Condition**: 10 Goroutines đồng thời cộng điểm trên cùng $1$ user qua `SELECT ... FOR UPDATE` $\rightarrow 0\%$ Lost Update, tổng điểm cuối cùng đúng $100\%$.<br>- **Idempotency**: Gửi 2 lần cùng một `event_id` $\rightarrow$ lần 2 bị từ chối, transaction rollback, không bị cộng trùng điểm. |

### Lệnh Thực Thi Kiểm Thử

```bash
# 1. Chạy Unit Tests thuần túy cho Domain Logic (nhanh, in-memory, zero I/O)
go test -v -race ./internal/gamification/domain/...

# 2. Chạy Integration Tests kiểm tra khóa bi quan & idempotency với PostgreSQL
go test -v -race -tags=integration ./internal/gamification/infrastructure/persistence/postgres/...

# 3. Kiểm tra độ bao phủ mã nguồn cho Domain Layer (Mục tiêu > 90%)
go test -coverprofile=coverage.out ./internal/gamification/domain/...
go tool cover -func=coverage.out
```

---

## Data migration, compatibility, and rollout
- **Migration an toàn**: Thêm mới schema `gamification` độc lập, không can thiệp hay sửa đổi bất kỳ bảng nào của các module hiện hữu (`auth`, `profile`, `workout_execution`).
- **Khởi tạo dữ liệu người dùng (Lazy Onboarding)**: Người dùng cũ chưa có bản ghi trong `gamification.user_elo` sẽ được tự động khởi tạo mặc định (1,000 ELO, Bậc Bronze) ngay trong lần đầu tiên phát sinh buổi tập hoặc khi mở ứng dụng xem rank.
- **Rollback Plan**: Xóa bảng và schema `gamification` mà không gây bất kỳ ảnh hưởng nào đến các chức năng tập luyện hay dinh dưỡng của hệ thống.

---

## Risks and open decisions

| Rủi ro | Mức độ | Biện pháp giảm thiểu |
| :--- | :---: | :--- |
| **Sự kiện trùng lặp từ Kafka** | Cao | Chặn đứng bằng bảng `processed_events` (Idempotent Inbox Guard) trong transaction. |
| **Tranh chấp khóa dòng khi tải cao** | Trung bình | Khóa chỉ giữ 1-3ms cho truy vấn nội bộ; giao dịch tuần tự hóa theo `user_id`. |
| **Sai lệch múi giờ khi tính thưởng dinh dưỡng** | Thấp | Chuyển đổi timestamp của sự kiện sang `user_local_date` trước khi kiểm tra ngày nhận thưởng. |
