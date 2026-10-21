# Tài Liệu Bounded Context Solo Match — Gym Companion

Hệ thống tài liệu kỹ thuật hoàn chỉnh cho module **Solo Match (1v1 Đối Kháng Thể Lực)** được tổ chức thành 4 phân hệ tài liệu chuẩn hóa theo kiến trúc Hexagonal (Ports & Adapters) và chuẩn mực kỹ thuật của dự án (`AGENTS.md`):

```
internal/solo_match/docs/
├── README.md                          # Mục lục & Điều hướng tổng quan
├── 01_business/                       # 1. Nghiệp vụ (Business Requirements)
│   └── business_requirements.md
├── 02_system_analysis/                # 2. Phân tích hệ thống (System Analysis)
│   └── system_analysis.md
├── 03_system_design/                  # 3. Thiết kế hệ thống (System Design)
│   └── system_design.md
└── 04_implementation/                 # 4. Hiện thực hóa (Implementation Summary)
    └── implementation_summary.md
```

---

## 1. [Nghiệp Vụ (Business Requirements)](./01_business/business_requirements.md)
- Mục tiêu và vai trò thúc đẩy tinh thần luyện tập qua thi đấu đối kháng 1v1 ngẫu nhiên.
- Hàng đợi tìm trận Pure Redis In-Memory Matchmaker (ZSet + Lua Script + Pub/Sub).
- Cơ chế ghép cặp thông minh, tự động chọn bài tập ngẫu nhiên theo thể thức (Reps vs Duration).
- Cơ chế Bot Fallback sau 5 phút: bốc ngẫu nhiên tài khoản Bot cấu hình sẵn trong DB (System Bot Pool), điểm chuẩn căn cứ theo kỷ lục cá nhân (PR).
- Quy tắc phân định thắng, thua, hòa và cơ chế xử lý vi phạm / bỏ cuộc (Two-Tier Forfeit Mechanism).
- Độc lập ranh giới với module `workout_execution`: không can thiệp chi tiết buổi tập, đồng bộ kết quả qua Kafka Event Bus.

## 2. [Phân Tích Hệ Thống (System Analysis)](./02_system_analysis/system_analysis.md)
- Bảng phân tích tác nhân hệ thống (Gym User, System Bot, Matchmaker Worker, Kafka Event Bus).
- Biểu đồ ca sử dụng (Use Case Diagram) chi tiết cho Client và Background Workers.
- Phân tích luồng sự kiện (Event Storming) và tích hợp liên module qua Kafka CloudEvents (Inbound & Outbound).
- Sơ đồ tuần tự (Sequence Diagrams) cho luồng Tìm trận qua Server-Streaming gRPC, luồng Ghép Bot sau 5 phút, và luồng Xử thua tức thì (Fast Forfeit).

## 3. [Thiết Kế Hệ Thống (System Design)](./03_system_design/system_design.md)
- Kiến trúc Hexagonal (Ports & Adapters) chi tiết từng tầng: Domain, Application (CQRS), Infrastructure (Redis & GORM), Transport (gRPC / ConnectRPC & Kafka Consumers).
- Thiết kế cơ sở dữ liệu PostgreSQL (Schema Isolation `solo_match.*`, bảng `solo_matches`, `match_participants`, `user_snapshots`, `outbox_events`, `outbox_logs`, `system_bots`).
- Thiết kế hàng đợi Pure Redis Matchmaker: Lua Script phân giải vé nguyên tử, kênh Pub/Sub đánh thức luồng stream gRPC (0% CPU, 0 I/O DB).
- Mô hình Transactional Outbox với khóa bi quan `FOR UPDATE SKIP LOCKED`.
- Chuẩn hóa API Contract-First qua Protobuf / gRPC / REST Gateway theo `contracts/core/solo_match/v1/`.
- **[Tổng hợp 8 Kỹ Thuật Chuyên Sâu (Technical Deep Dive)](./03_system_design/technical_innovations.md)**: Pure Redis ZSet, Lua Atomic Dequeue, Non-blocking Pub/Sub Sleep/Wakeup, System Bot Pool & PR Benchmark, Two-Tier Forfeit, Idempotent Outbox Logging, Transactional Outbox SKIP LOCKED, Active Match Guard.

## 4. [Hiện Thực Hóa & Báo Cáo Hoàn Thành (Implementation Summary)](./04_implementation/implementation_summary.md)
- Bảng đối chiếu mã nguồn thực tế đã xây dựng (Domain, Application, Infrastructure, Transport).
- Hiện trạng kiểm thử tự động (Unit Test, Concurrency Test, InMemory Redis miniredis, Mock tests).
- Báo cáo kết quả kiểm thử toàn diện (Statement Coverage, Race Condition Detection).
- Hướng dẫn vận hành, kiểm thử và mở rộng luồng real-time stream.
