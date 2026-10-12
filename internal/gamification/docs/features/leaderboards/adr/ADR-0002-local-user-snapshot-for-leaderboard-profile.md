# ADR-0002: Sử Dụng Bảng User Snapshot Cục Bộ (Local User Snapshot) Thay Vì In-Process Port

- **Feature**: leaderboards
- **Date**: 2026-10-12
- **Status**: Accepted
- **Deciders**: Maintainer (User), AI Assistant

## Context and Problem Statement
Bảng xếp hạng hiển thị cho người dùng cần có Tên hiển thị (`display_name`) và Ảnh đại diện (`avatar_url`) của Top 50 gymer. 

Tuy nhiên, trong kiến trúc Modular Monolith của FITAI:
- Bảng `gamification.user_xp` chỉ sở hữu `user_id`, `xp`, `level`.
- Bảng `profile.users` mới sở hữu `display_name` và `avatar_url`.
- Quy tắc cốt lõi của dự án nghiêm cấm thực hiện câu lệnh `JOIN` chéo schema giữa `gamification` và `profile`.

Cần lựa chọn phương án tích hợp để lấy thông tin hồ sơ của Top 50 người dẫn đầu mà vẫn bảo đảm ranh giới Bounded Context độc lập và hiệu năng cao nhất.

## Decision Drivers
- **Zero Runtime Coupling**: Module Gamification phải tự chủ 100%, không phụ thuộc vào trạng thái sống/chết hay độ trễ của module `profile` khi phục vụ API Bảng Xếp Hạng.
- **Bảo toàn Schema Isolation**: Tuyệt đối không viết câu lệnh SQL `JOIN` chéo sang schema của module khác. Toàn bộ dữ liệu hiển thị Leaderboard nằm trọn trong schema `gamification.*`.
- **Hiệu năng & Trải nghiệm (Sub-5ms Latency)**: Tối ưu hóa truy vấn thành 1 câu SQL duy nhất trên PostgreSQL, loại bỏ hoàn toàn việc ghép dữ liệu trong bộ nhớ (In-memory pointer stitching) hay gọi RPC nội bộ.
- **Đồng nhất với toàn hệ thống**: Áp dụng cùng mô hình User Snapshot mà module `social` đã triển khai thành công (`social.user_snapshots`).

## Considered Options

### Option 1: BFF / Client-Side Aggregation
- **Cơ chế**: API Gamification chỉ trả về `user_id`. Tầng API Gateway (BFF) hoặc Mobile Client nhận danh sách 50 `user_id`, sau đó gửi request thứ hai sang module `profile` để lấy thông tin tên/ảnh và tự gộp lại trên client.
- **Hạn chế**:
  - Gây thêm 1 network round-trip. Trên mạng di động, độ trễ tăng thêm $50\text{–}150\text{ ms}$.
  - Tăng độ phức tạp logic xử lý ở phía Client / Gateway.

### Option 2: In-Process Port (`ProfileReaderPort`)
- **Cơ chế**: Module Gamification định nghĩa interface Output Port, tầng Infrastructure gọi trực tiếp con trỏ Go sang Application Service của `profile` trong cùng tiến trình (In-process memory call).
- **Hạn chế**:
  - **Tạo Runtime Coupling**: Dù tuân thủ Hexagonal, nhưng hai module bị phụ thuộc chặt chẽ lúc chạy. Nếu module `profile` bị nghẽn DB hoặc lỗi, API Leaderboard của Gamification bị chậm hoặc lỗi theo.
  - **Phải xử lý timeout & fallback phức tạp**: Cần cấu hình timeout $500\text{ ms}$ và logic fallback gán tên `"Gymer"`.
  - **Khó tiến hóa Microservices**: Khi tách service riêng, bắt buộc phải thay thế bằng gRPC client và chấp nhận độ trễ mạng mạng nội bộ.

### Option 3: Bảng User Snapshot Cục Bộ (`gamification.user_snapshots`) Đồng Bộ Qua Event-Driven
- **Cơ chế**:
  1. Tạo bảng bản sao cục bộ `gamification.user_snapshots` (`user_id UUID PK`, `display_name VARCHAR(100)`, `avatar_url TEXT`, `updated_at TIMESTAMPTZ`).
  2. Khởi tạo: Lắng nghe `contracts.generic.auth.v1.userRegistered` từ `auth.events` để ghi nhận ban đầu.
  3. Cập nhật: Lắng nghe `contracts.supporting.profile.v1.event.UserIdentityUpdated` từ `profile.events` để tự động cập nhật khi người dùng đổi tên hoặc ảnh đại diện.
  4. Truy vấn Leaderboard: Thực thi câu lệnh `LEFT JOIN` nội bộ ngay trong schema `gamification.*`.

## Decision Outcome
Chosen: **Option 3 (Bảng User Snapshot Cục Bộ - Local User Snapshot)**.

### Consequences
- **Positive:**
  - **Độc lập hoàn toàn (Zero Runtime Coupling)**: Gamification tự chủ 100% dữ liệu để render Leaderboard. Kể cả khi module Profile sập hoàn toàn, Bảng Xếp Hạng vẫn phản hồi mượt mà trong $< 2\text{ ms}$.
  - **Hiệu năng xuất sắc**: Chỉ cần **1 câu lệnh SQL duy nhất** (`user_xp LEFT JOIN user_snapshots`), tận dụng triệt để B-tree index scan, thời gian truy vấn $< 2\text{ ms}$.
  - **100% Tuân thủ Schema Isolation**: Không `JOIN` chéo schema. Dữ liệu snapshot thuộc quyền quản lý của schema `gamification`.
  - **Sẵn sàng cho Microservices**: Nếu tách Gamification thành microservice độc lập, không cần sửa đổi bất kỳ dòng code nào vì dữ liệu đã được cô lập cục bộ.
- **Negative / Trade-offs:**
  - **Eventual Consistency**: Khi người dùng đổi tên/avatar ở trang cá nhân, thông tin trên Bảng Xếp Hạng có độ trễ vài mili-giây qua Kafka trước khi được cập nhật đồng bộ. (Hoàn toàn chấp nhận được đối với tính năng Bảng Xếp Hạng).
