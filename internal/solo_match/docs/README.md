# Tài Liệu Bounded Context Solo Match — Gym Companion

Hệ thống tài liệu kỹ thuật hoàn chỉnh cho module **Solo Match (Hàng Đợi Ghép Trận & Phân Định Đối Kháng 1v1)**, được tổ chức theo kiến trúc Hexagonal (Ports & Adapters) và chuẩn mực kỹ thuật của dự án (`AGENTS.md`):

```
internal/solo_match/docs/
├── README.md                          # Mục lục & Điều hướng tổng quan
├── 01_business/                       # 1. Nghiệp vụ (Business Requirements)
│   └── business_requirements.md
├── 02_system_analysis/                # 2. Phân tích hệ thống (System Analysis)
│   └── system_analysis.md
├── 03_system_design/                  # 3. Thiết kế hệ thống (System Design)
│   ├── system_design.md
│   └── technical_innovations.md
└── 04_implementation/                 # 4. Hiện thực hóa (Implementation Summary)
    └── implementation_summary.md
```

> [!NOTE]
> **Tài liệu phân hệ Workout Execution tương ứng**:
> Toàn bộ logic camera AI, trọng tài ảo đếm Reps/Giây, mô phỏng Bot ảo thời gian thực (`SoloBotManager`), cơ chế ân hạn mất kết nối 45s và đồng bộ tiến độ hai chiều thuộc trách nhiệm của module `workout_execution`. Xem chi tiết tại [**`internal/workout_execution/docs/solo_tracking_design.md`**](../../workout_execution/docs/solo_tracking_design.md).

---

## 1. [Nghiệp Vụ (Business Requirements)](./01_business/business_requirements.md)
- **Mục tiêu & Tầm nhìn**: Điều phối hàng đợi thi đấu đối kháng 1v1 ngẫu nhiên, độ trễ cực thấp, không nghẽn I/O Database.
- **Các phân hệ nghiệp vụ chính**:
  1. *Hàng đợi tìm trận*: Pure Redis In-Memory Matchmaker (ZSet + Lua Script + Pub/Sub).
  2. *Khởi tạo & Kích hoạt trận*: Trạng thái ban đầu `PENDING_READY`, mốc hạn chót vào phòng 60 giây (`ready_deadline`), kích hoạt `IN_PROGRESS` khi nhận `SoloMatchRoomReady`, hủy trận (`CancelOnNoShow`) khi quá 60s.
  3. *Phân định thắng thua*: So sánh điểm số AI nộp về từ `workout_execution` (`MatchResolver`), xử thua tức thì (`Fast Forfeit`) khi có người bỏ cuộc hoặc mất mạng quá 45s, và lưới an toàn 35 phút.
- **Cơ chế Bot Fallback sau 5 phút**: Bốc ngẫu nhiên Bot từ `solo_match.system_bots` thông qua In-Memory Cache $O(1)$, ẩn danh tính AI.
- **Bảng tổng hợp 10 kịch bản nghiệp vụ của Solo Match** (`SM-01` $\rightarrow$ `SM-10`).

---

## 2. [Phân Tích Hệ Thống (System Analysis)](./02_system_analysis/system_analysis.md)
- **Bảng phân tích tác nhân hệ thống**: Gym User, Redis Matchmaker Adapter, System Bot Pool, MatchTimeoutWorker, Workout Execution Module, Auth & Profile Modules.
- **Biểu đồ ca sử dụng (Use Case Diagram)**: Toàn bộ Use Cases thuộc thẩm quyền của Solo Match.
- **Hệ thống 6 Sơ đồ tuần tự (Sequence Diagrams)**:
  1. Luồng ghép cặp PvP & Kích hoạt trận đấu (`SoloMatchRoomReady`).
  2. Luồng Bot Fallback sau 5 phút (`System Bot Pool`).
  3. Luồng hủy trận do quá hạn 60s điểm danh không vào phòng (`CancelOnNoShow`).
  4. Luồng phân định thắng thua khi cả hai cùng hoàn thành (`SoloTrackingCompleted`).
  5. Luồng xử thua tức thì khi đối thủ bỏ cuộc (`SoloTrackingAborted`).
  6. Luồng lưới an toàn cưỡng chế trận đấu treo quá 35 phút (`TimeoutInProgress`).
- **Phân tích sự kiện liên module (Event Storming)**: 4 Outbound CloudEvents và 5 Inbound CloudEvents chuẩn ISO 1.0 JSON.
- **Máy trạng thái hữu hạn của trận đấu** (`SoloMatch.Status`): `PENDING_READY` $\rightarrow$ `IN_PROGRESS` $\rightarrow$ `COMPLETED` / `FORFEITED` / `CANCELLED`.

---

## 3. [Thiết Kế Hệ Thống (System Design)](./03_system_design/system_design.md)
- **Kiến trúc Hexagonal (Ports & Adapters)**: Đóng gói khép kín trong `/internal/solo_match/` (Domain, Application, Infrastructure, Transport).
- **Thiết kế cơ sở dữ liệu PostgreSQL**: Schema Isolation `solo_match.*` (các bảng `solo_matches`, `match_participants`, `user_snapshots`, `system_bots`, `outbox_events`, `outbox_logs`) kèm migration 13 & 14 (`ready_deadline`).
- **Thiết kế bộ đệm Redis**: ZSet hàng đợi `solo_match:queue:waiting` và kênh Pub/Sub đánh thức vé `solo_match:events:{ticketID}`.
- **Chuẩn hóa API Protobuf & ConnectRPC**: Triển khai từ `/proto/contracts/core/solo_match/v1/service/solo_match_service.proto`.
- **[Tổng hợp 7 Kỹ Thuật Chuyên Sâu Của Solo Match (Technical Deep Dive)](./03_system_design/technical_innovations.md)**:
  1. *Pure Redis In-Memory Matchmaker (ZSet FIFO)*
  2. *Atomic Dequeue qua Lua Script (Chống Race Condition Multi-Pod)*
  3. *Non-blocking Stream Wakeup via Redis Pub/Sub*
  4. *System Bot Pool & In-Memory Cache O(1)*
  5. *Check-in No-Show 60 Giây (`ready_deadline`) & Cancellation*
  6. *Kiến Trúc Xử Thua 2 Tầng (Fast Forfeit via Event + 35m Safety Net)*
  7. *Transactional Outbox (SKIP LOCKED) & Idempotent Consumer Logging*

---

## 4. [Hiện Thực Hóa & Báo Cáo Hoàn Thành (Implementation Summary)](./04_implementation/implementation_summary.md)
- **Bảng danh mục mã nguồn thực tế**: Tầng Domain, Application, Infrastructure, Transport của module `solo_match`.
- **Hiện trạng kiểm thử tự động**: Unit Test, Concurrency Test với `miniredis`, Statement Coverage > 85% – 95%.
