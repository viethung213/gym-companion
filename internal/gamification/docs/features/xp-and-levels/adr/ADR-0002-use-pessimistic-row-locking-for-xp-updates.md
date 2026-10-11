# ADR-0002: Sử Dụng Khóa Dòng Bi Quan (SELECT FOR UPDATE) Cho Cập Nhật Điểm XP

- **Feature**: xp-and-levels
- **Date**: 2026-10-09
- **Status**: Accepted
- **Deciders**: Maintainer (User), AI Assistant

## Context and Problem Statement
Khi người dùng hoàn thành buổi tập hoặc cập nhật chỉ số dinh dưỡng, các sự kiện từ Kafka hoặc HTTP API có thể được gửi đến đồng thời hoặc với khoảng cách thời gian rất ngắn (ví dụ: kết thúc buổi tập đúng lúc hệ thống dinh dưỡng tự động chốt ngày).

Nếu hai giao dịch cùng đọc bản ghi `user_xp`, cùng tính toán và ghi lại vào database, sự kiện đến sau có thể ghi đè làm mất điểm của sự kiện trước (**Lost Update Anomaly**).

Cần lựa chọn cơ chế kiểm soát tương tranh (Concurrency Control) để bảo vệ số dư điểm `xp`.

## Decision Drivers
- **Tính toàn vẹn dữ liệu tuyệt đối (Zero Lost Updates)**: Không bao giờ được phép mất dù chỉ 1 điểm XP nỗ lực của gymer.
- **Thực tế hành vi người dùng**: Tần suất phát sinh sự kiện ghi điểm trên cùng một `user_id` là rất thưa thớt (1-2 buổi tập/ngày, tỷ lệ đụng độ mili-giây $< 0.01\%$).
- **Đơn giản hóa mã nguồn (Simplicity First)**: Tránh cơ chế retry phức tạp của Optimistic Locking ở tầng ứng dụng khi bị xung đột version.

## Considered Options
- **Option 1 (Optimistic Locking với cột version)**: Đọc không khóa, khi cập nhật kiểm tra `WHERE version = current_version`. Nếu trùng lặp thì rollback và retry ở tầng application.
  * *Hạn chế*: Tốn tài nguyên CPU cho logic retry, code phức tạp hơn mà không mang lại giá trị thực tế do xác suất đụng độ giữa các request của cùng 1 user là cực thấp.
- **Option 2 (Pessimistic Row Locking - `SELECT ... FOR UPDATE`)**: Khóa dòng của người dùng trong transaction khi bắt đầu đọc aggregate và giải phóng khi commit transaction.
  * *Ưu điểm*: Đơn giản, tự nhiên với PostgreSQL, đảm bảo tuần tự hóa tuyệt đối các sự kiện cùng 1 user mà không cần retry ở tầng app.

## Decision Outcome
Chosen: **Option 2 (Pessimistic Row Locking - `SELECT ... FOR UPDATE`)**.

```sql
SELECT user_id, xp, level, updated_at
FROM gamification.user_xp
WHERE user_id = $1
FOR UPDATE;
```

### Consequences
- **Positive:**
  - Triệt tiêu 100% rủi ro Lost Update.
  - Tầng Application và Domain không cần xử lý retry phức tạp.
  - Do chỉ khóa duy nhất 1 dòng của `user_id` đang xử lý trong thời gian cực ngắn (< 5ms), hoàn toàn không ảnh hưởng đến người dùng khác hay gây nghẽn toàn cục.
- **Trade-offs:**
  - Phải luôn đảm bảo transaction được commit/rollback nhanh chóng để tránh giữ lock lâu.
