# Tài Liệu Bounded Context Social — Gym Companion

Hệ thống tài liệu kỹ thuật hoàn chỉnh cho module **Social & Activity Feed** được tổ chức thành 4 phân hệ tài liệu chuẩn hóa:

```
internal/social/docs/
├── README.md                          # Mục lục & Điều hướng tổng quan
├── 01_business/                       # 1. Nghiệp vụ (Business Requirements)
│   └── business_requirements.md
├── 02_system_analysis/                # 2. Phân tích hệ thống (System Analysis)
│   └── system_analysis.md
├── 03_system_design/                  # 3. Thiết kế hệ thống (System Design)
│   └── system_design.md
└── 04_implementation/                 # 4. Đã thực hiện (Implementation Summary)
    └── implementation_summary.md
```

---

## 1. [Nghiệp Vụ (Business Requirements)](./01_business/business_requirements.md)
- Đặc tả ranh giới nghiệp vụ của Social Graph và Activity Feed.
- Quy tắc chia sẻ buổi tập tôn trọng quyền riêng tư (Privacy-first).
- Hệ thống tương tác (Reactions đa dạng cảm xúc, Bình luận đa cấp, Theo dõi người dùng).
- Bản sao danh tính cục bộ (User Snapshot) phục vụ hiển thị siêu tốc.

## 2. [Phân Tích Hệ Thống (System Analysis)](./02_system_analysis/system_analysis.md)
- Sơ đồ Actor & Use Case Diagrams.
- Phân tích Event Storming và các sự kiện Domain/CloudEvents.
- Tương tác giao tiếp liên module qua Kafka Event Bus (Inbound & Outbound).
- Sơ đồ luồng dữ liệu (Data Flow Diagrams) cho các kịch bản trọng yếu.

## 3. [Thiết Kế Hệ Thống (System Design)](./03_system_design/system_design.md)
- Kiến trúc Hexagonal (Ports & Adapters) chi tiết từng tầng (Domain, Application, Infrastructure, Transport).
- Thiết kế cơ sở dữ liệu PostgreSQL (Schema Isolation `social.*`), cơ chế Single-Table Feed Item và Index tối ưu.
- Mô hình Transactional Outbox & Outbox Log với khóa `FOR UPDATE SKIP LOCKED`.
- Chuẩn hóa API Contract-First qua Protobuf / gRPC / REST Gateway.

## 4. [Hiện Thực Hóa & Báo Cáo Hoàn Thành (Implementation Summary)](./04_implementation/implementation_summary.md)
- Bảng đối chiếu mã nguồn thực tế đã xây dựng.
- Kết quả kiểm thử tự động (Unit Test, Integration Test, Concurrency Test).
- Hướng dẫn vận hành, kiểm thử và mở rộng.
