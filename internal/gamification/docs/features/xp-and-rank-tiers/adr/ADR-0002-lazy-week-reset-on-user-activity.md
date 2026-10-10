# ADR-0002: Cơ Chế Làm Mới Tuần Lười Biếng (Lazy Week Reset on User Activity)

- **Feature**: xp-and-rank-tiers
- **Date**: 2026-10-09
- **Status**: Accepted
- **Deciders**: Maintainer (User), AI Assistant

## Context and Problem Statement
Trong mô hình Giải Đấu Tuần (Weekly Leagues), điểm số `weekly_xp` của người dùng phải được khởi tạo lại về 0 vào đầu mỗi tuần mới (00:00 Thứ Hai).

Khi hệ thống có quy mô hàng chục nghìn đến hàng trăm nghìn người dùng:
- Nếu chạy một tác vụ ngầm định kỳ (Scheduled Batch Job / Cron) vào đúng 23:59:59 Chủ Nhật để thực thi:
  ```sql
  UPDATE gamification.user_xp SET weekly_xp = 0;
  ```
- **Rủi ro sản xuất ("Bài toán 3 AM")**:
  - Gây khóa bảng trên diện rộng (Table/Page Locks), làm treo các request tập luyện muộn của người dùng.
  - Gây đỉnh tải I/O (I/O Spike) và làm phình to transaction log (WAL logs).
  - Tốn tài nguyên tính toán vô ích cho hàng nghìn tài khoản không hoạt động (inactive users).

Hệ thống cần một chiến lược làm mới `weekly_xp` an toàn, phân tán và không gây nghẽn database.

## Decision Drivers
- **The 3 AM Test**: Loại bỏ hoàn toàn điểm nghẽn tập trung lúc nửa đêm; database phải vận hành ổn định mà không có tác vụ batch update cồng kềnh.
- **Tiết kiệm tài nguyên**: Chỉ xử lý reset cho những người dùng thực sự quay lại tập luyện trong tuần mới.
- **Tính nhất quán ACID**: Việc reset tuần phải diễn ra ngay trong transaction của hoạt động đầu tiên của user đó.

## Considered Options
- **Option 1 (Scheduled Batch Reset qua Cron Job)**: Chạy cron job quét toàn bộ bảng `user_xp` lúc 23:59:59 Chủ Nhật để gán `weekly_xp = 0`.
  * *Hạn chế*: Khóa bảng, nguy cơ fail job khi dữ liệu lớn, spike tài nguyên.
- **Option 2 (Lazy Reset on User Activity - Làm mới lười biếng)**: Lưu thêm cột `current_week_number VARCHAR(10)` (ví dụ: `'2026-W41'`) trong bảng `user_xp`. Khi có sự kiện tập luyện hoặc log dinh dưỡng mới:
  * So sánh tuần của sự kiện (`event_week`) với `current_week_number` của user.
  * Nếu `event_week > current_week_number`: tự động gán `weekly_xp = new_xp` và cập nhật `current_week_number = event_week`.
  * Nếu trong cùng tuần: cộng dồn `weekly_xp = weekly_xp + new_xp`.

## Decision Outcome
Chosen: **Option 2 (Lazy Reset on User Activity)**.

### Consequences
- **Positive:**
  - Triệt tiêu 100% rủi ro nghẽn database lúc 00:00 Thứ Hai.
  - Chi phí reset được chia nhỏ tự nhiên theo từng request của user (O(1) per active user).
  - Người dùng không hoạt động không tốn bất kỳ tài nguyên I/O nào để reset.
- **Trade-offs:**
  - Bảng `gamification.user_xp` cần thêm cột `current_week_number VARCHAR(10) NOT NULL`.
  - Nghiệp vụ của Aggregate `UserXp` cần nhận thêm tham số tuần hiện tại khi thực thi method `AddWorkoutXp(...)` hoặc `AddNutritionXp(...)`.
