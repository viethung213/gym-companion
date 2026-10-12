# ADR-0001: Đánh Giá Chuỗi Lười Theo Sự Kiện (Event-Driven Lazy Streak Evaluation $O(1)$)

- **Feature**: streaks-and-habits
- **Date**: 2026-10-11
- **Status**: Accepted
- **Deciders**: Maintainer (User), AI Assistant

## Context and Problem Statement
Để tính toán số ngày chuỗi (Streak) và xử lý việc đứt chuỗi khi người dùng nghỉ tập, hệ thống có hai cách tiếp cận:
1. Chạy một Batch Cron Job hàng đêm (ví dụ: 00:00 hoặc 03:00) quét toàn bộ cơ sở dữ liệu để tìm những người dùng không hoạt động trong ngày hôm qua để reset chuỗi.
2. Đánh giá chuỗi lười (Lazy Evaluation $O(1)$) theo sự kiện phát sinh: Chỉ kiểm tra và cập nhật trạng thái chuỗi khi người dùng có hoạt động mới (hoặc khi người dùng mở ứng dụng tra cứu).

Tại quy mô hàng trăm nghìn gymer ở nhiều múi giờ khác nhau trên toàn cầu, Batch Cron Job lúc nửa đêm đối mặt với rủi ro nghẽn I/O nghiêm trọng (The 3 AM Test), xung đột múi giờ địa phương và chi phí hạ tầng lớn.

## Decision Drivers
- **Hiệu năng & Bền vững (The 3 AM Test)**: Tuyệt đối không dùng Cron Job quét hàng triệu dòng lúc nửa đêm.
- **Bất biến Múi giờ Địa phương (Timezone Invariant)**: Xử lý chính xác chu kỳ 24h theo múi giờ địa phương của từng người dùng (`user_timezone`).
- **Tối giản nghiệp vụ (Simplicity First)**: Nghiệp vụ đếm ngày thuần túy toán học dựa trên khoảng cách ngày giữa sự kiện gần nhất và sự kiện hiện tại ($O(1)$).

## Considered Options

### Option 1: Scheduled Batch Worker (Midnight Cron)
- **Cơ chế**: Worker chạy định kỳ mỗi đêm lúc 00:01 theo từng múi giờ, truy vấn `gamification.user_streak` và xử lý reset streak.
- **Hạn chế**:
  - Gây đỉnh tải I/O (Spike Load) trên PostgreSQL database.
  - Phức tạp trong việc phân cụm người dùng theo múi giờ toàn cầu.
  - Nguy cơ race condition nếu người dùng đang tập luyện muộn (23:55) trong lúc worker đang quét.

### Option 2: Đánh Giá Lười Theo Sự Kiện (Event-Driven Lazy Evaluation $O(1)$)
- **Cơ chế**:
  1. Bảng `gamification.user_streak` lưu trữ `last_active_date DATE`, `current_streak INT`, `longest_streak INT`.
  2. Khi có sự kiện hoạt động mới (`WorkoutSessionCompleted` hoặc `MealLogged`), Application Handler chuyển đổi thời gian sang ngày địa phương `current_date` (dạng chuỗi `"YYYY-MM-DD"`).
  3. Tính khoảng cách ngày: $\Delta = \text{DateDiff}($ `current_date`, `last_active_date` $)$.
     - $\Delta = 0$: Cùng ngày $\rightarrow$ Bỏ qua (Lũy đẳng).
     - $\Delta = 1$: Ngày kế tiếp liên tục $\rightarrow$ `current_streak += 1`, cập nhật `last_active_date`.
     - $\Delta \ge 2$: Nghỉ ít nhất 1 ngày không hoạt động $\rightarrow$ Chuỗi bị đứt, bắt đầu chuỗi mới (`current_streak = 1`), phát CloudEvent `StreakReset`.
  4. Khi người dùng gọi API đọc `GetStreak`: Đánh giá động `is_active_today = (last_active_date == today)`. Nếu $\Delta > 1$, hiển thị cảnh báo chuỗi đã bị ngắt quãng mà không cần ghi vào database.

## Decision Outcome
Chosen: **Option 2 (Đánh Giá Lười Theo Sự Kiện - Event-Driven Lazy Evaluation $O(1)$)**.

### Consequences
- **Positive:**
  - $0$ Cron Jobs, $0$ nguy cơ nghẽn DB lúc nửa đêm.
  - Xử lý hoàn toàn trong bộ nhớ với độ phức tạp $O(1)$.
  - Bọc trong `port.TransactionManager` với `SELECT ... FOR UPDATE` đảm bảo tính toàn vẹn 100% không bị race conditions.
- **Negative / Trade-offs:**
  - Nếu người dùng nghỉ tập 1 tháng không vào app, trạng thái trong DB vẫn lưu `current_streak` cũ cho tới lần kế tiếp họ tập hoặc truy vấn.
  - *Biện pháp*: API tra cứu `GetStreak` chỉ đọc (Read Query) tự động tính toán ảo trạng thái hiển thị nếu người dùng đã quá hạn mà không cần ghi vào DB.
