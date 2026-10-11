# ADR-0002: Sử Dụng Port Nội Bộ (In-process Port) Để Làm Giàu Thông Tin Profile Trên Leaderboard

- **Feature**: leaderboards
- **Date**: 2026-10-11
- **Status**: Accepted
- **Deciders**: Maintainer (User), AI Assistant

## Context and Problem Statement
Bảng xếp hạng hiển thị cho người dùng cần có Tên hiển thị (`display_name`) và Ảnh đại diện (`avatar_url`) của Top 50 gymer. 

Tuy nhiên, trong kiến trúc Modular Monolith của FITAI:
- Bảng `gamification.user_xp` chỉ sở hữu `user_id`, `xp`, `level`.
- Bảng `profile.users` mới sở hữu `display_name` và `avatar_url`.
- Quy tắc cốt lõi của dự án nghiêm cấm thực hiện câu lệnh `JOIN` chéo schema giữa `gamification` và `profile`.

Cần lựa chọn phương án tích hợp để lấy thông tin hồ sơ của Top 50 người dẫn đầu mà vẫn bảo đảm ranh giới Bounded Context và hiệu năng cao.

## Decision Drivers
- **Bảo toàn Schema Isolation**: Tuyệt đối không viết câu lệnh SQL `JOIN` sang schema của module khác.
- **Hiệu năng & Trải nghiệm (Sub-10ms Latency)**: Tối thiểu hóa số lượng request mạng từ ứng dụng di động của người dùng.
- **Tính toàn vẹn & Tinh gọn (Single Source of Truth)**: Tránh sao chép dữ liệu thừa thãi (`display_name`, `avatar_url`) sang module Gamification qua Kafka gây desync khi người dùng đổi tên/ảnh.
- **Triết lý Modular Monolith**: Tận dụng lợi thế các module chạy chung trong một tiến trình duy nhất (Single OS Process).

## Considered Options

### Option 1: BFF / Client-Side Aggregation
- **Cơ chế**: API Gamification chỉ trả về `user_id`. Tầng API Gateway (BFF) hoặc Mobile Client nhận danh sách 50 `user_id`, sau đó gửi request thứ hai sang module `profile` để lấy thông tin tên/ảnh và tự gộp lại trên client.
- **Hạn chế**:
  - Gây thêm 1 network round-trip. Trên mạng 4G/3G di động, độ trễ tăng thêm $50\text{–}150\text{ ms}$.
  - Tăng độ phức tạp logic xử lý ở phía Client / Gateway.

### Option 2: Event-Driven Read Model (Kafka Sync Cache)
- **Cơ chế**: Module Gamification lắng nghe sự kiện `UserProfileUpdated` từ Kafka và lưu bản sao `display_name`, `avatar_url` ngay trong bảng `gamification.user_xp`.
- **Hạn chế**:
  - Vi phạm Single Source of Truth: Dữ liệu hồ sơ bị trùng lặp ở 2 nơi.
  - Phải quản lý thêm Kafka Consumer, xử lý sự kiện out-of-order hoặc trễ tin nhắn (Eventual Consistency).

### Option 3: Port Nội Bộ (In-process Cross-Module Application Port)
- **Cơ chế**: Áp dụng nguyên lý Đảo ngược phụ thuộc (Dependency Inversion Principle) của Hexagonal Architecture:
  1. Tầng Application của Gamification định nghĩa một Output Port:
     ```go
     type ProfileSummary struct {
         DisplayName string
         AvatarURL   string
     }
     type ProfileReaderPort interface {
         BatchGetSummaries(ctx context.Context, userIDs []uuid.UUID) (map[uuid.UUID]ProfileSummary, error)
     }
     ```
  2. Tầng Infrastructure của Gamification hiện thực adapter gọi trực tiếp sang Application Service / Facade của module `profile` trong cùng một tiến trình (In-memory pointer call, độ trễ $0\text{ ms}$).
  3. Module `profile` thực thi truy vấn nội bộ: `SELECT id, display_name, avatar_url FROM profile.users WHERE id = ANY($1)`.
  4. Query Handler của Gamification gộp dữ liệu trong RAM và trả về response hoàn chỉnh cho Client.

## Decision Outcome
Chosen: **Option 3 (Port Nội Bộ - In-process Cross-Module Application Port)**.

### Consequences
- **Positive:**
  - **Hiệu năng xuất sắc**: Tổng thời gian xử lý chỉ $\sim 3\text{–}5\text{ ms}$ (Gamification B-tree index scan $\sim 2\text{ ms}$ + In-memory call $0\text{ ms}$ + Profile PK index query $\sim 2\text{ ms}$).
  - **100% Nhất quán (Strong Consistency)**: Khi người dùng đổi tên/ảnh, bảng xếp hạng cập nhật tức thì, không có độ trễ sync.
  - **Không vi phạm Schema Isolation**: Module nào tự truy vấn schema của module đó; Gamification không hề biết cấu trúc bảng của `profile`.
  - **Khả năng tiến hóa Microservices**: Nếu sau này hệ thống tách ra các service độc lập, ta chỉ cần thay đổi phần hiện thực của `ProfileAdapter` thành gRPC Client mà không phải sửa bất kỳ dòng code nghiệp vụ nào trong Domain/Application của Gamification.
- **Negative / Trade-offs:**
  - Tạo phụ thuộc thời gian chạy (Runtime coupling) giữa Gamification và Profile trong cùng binary.
  - **Biện pháp phòng vệ (Resilience Guard)**: Cần bọc lệnh gọi Port bằng context timeout ($500\text{ ms}$). Nếu module Profile lỗi hoặc timeout, Gamification tự động fallback hiển thị tên mặc định `"Gymer"` và `avatar_url = ""` thay vì làm sập toàn bộ response bảng xếp hạng.
