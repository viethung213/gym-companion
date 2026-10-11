# ADR-0002: Kiểm Soát Trần Thu Nhập Hàng Ngày (Daily Earning Caps) & Chống Gian Lận

- **Feature**: fitcoins-and-ledger
- **Date**: 2026-10-11
- **Status**: Accepted
- **Deciders**: Maintainer (User), AI Assistant

## Context and Problem Statement
Trong nền kinh tế FitCoins, các hoạt động hàng ngày (buổi tập luyện, dinh dưỡng, phá kỷ lục PR) là nguồn tạo ra coin (Inflow). 

Nếu không có cơ chế khống chế trần thu nhập hàng ngày (**Daily Earning Caps**):
- Người dùng có thể lợi dụng kẽ hở để gian lận: bấm "Bắt đầu" rồi "Kết thúc" 20 buổi tập ảo trong 1 ngày, hoặc bấm log dinh dưỡng liên tục để cày hàng trăm coin.
- Điều này dẫn đến siêu lạm phát tiền ảo, làm mất giá trị phần thưởng và phá vỡ tính công bằng của nền tảng.

Cần một cơ chế kiểm soát trần thu nhập hàng ngày đơn giản, chính xác theo ngày địa phương của người dùng (`user_local_date`), và không phụ thuộc vào cron job reset nửa đêm.

## Decision Drivers
- **Chống gian lận tuyệt đối (Zero Exploit / Anti-Inflation)**: Giới hạn tối đa 1 lần nhận thưởng bài tập ($+5$ coin) và 1 lần dinh dưỡng ($+3$ coin) mỗi ngày.
- **The 3 AM Test**: Triệt tiêu hoàn toàn cron job quét reset database lúc nửa đêm.
- **Hiệu năng & Tiết kiệm tài nguyên (O(1) Evaluation)**: Đánh giá trần thu nhập ngay trong transaction cập nhật ví mà không cần quét bảng lịch sử.

## Considered Options

### Option 1: Sử dụng Redis Rate Limiter hoặc Bảng Riêng Đếm Lượt Kèm Cron Job Reset
- **Cơ chế**: Lưu số lần nhận coin trong Redis (`INCRBY`) với TTL 24h hoặc tạo bảng `daily_earning_counter` và dùng cron job quét reset về 0 lúc nửa đêm.
- **Hạn chế**:
  - Lệch múi giờ: Mỗi gymer ở một múi giờ khác nhau (`UTC+7`, `UTC-5`...), một cron job chạy lúc 00:00 UTC sẽ reset sai ngày sinh học của người dùng.
  - Tốn thêm chi phí hạ tầng (Redis cluster hoặc cron worker), rủi ro nghẽn DB nửa đêm.

### Option 2: Lưu Trực Tiếp Cột Ngày Nhận Thưởng Trên `user_wallet` & Khóa Lũy Đẳng Trên `coin_ledger`
- **Cơ chế**:
  1. Bổ sung các cột ngày vào bảng `gamification.user_wallet`:
     - `last_workout_reward_date DATE`
     - `last_pr_reward_date DATE`
     - `last_nutrition_reward_date DATE`
  2. Khi xử lý cộng coin từ buổi tập trong transaction (đã có khóa `SELECT FOR UPDATE` trên ví):
     ```go
     if wallet.LastWorkoutRewardDate != nil && *wallet.LastWorkoutRewardDate == localDate {
         // Đã nhận thưởng trong ngày -> Bỏ qua cộng coin (vẫn ghi nhận XP bình thường)
         return nil
     }
     ```
  3. Cập nhật `last_workout_reward_date = localDate` ngay trong lệnh `UPDATE user_wallet`.
  4. Đồng thời gán `idempotency_key = "earn:workout:<session_id>"` (với ràng buộc `UNIQUE` trên `coin_ledger`).

## Decision Outcome
Chosen: **Option 2 (Lưu Trực Tiếp Cột Ngày Nhận Thưởng Trên `user_wallet` & Khóa Lũy Đẳng)**.

### Consequences
- **Positive:**
  - **Zero Cron Jobs**: Hệ thống tự động chuyển ngày theo hành động của người dùng (Lazy Day Transition). Không có bất kỳ worker nền nào phải chạy lúc nửa đêm.
  - **Hỗ trợ đa múi giờ tự nhiên**: Ngày nhận thưởng được so sánh trực tiếp với `user_local_date` của người dùng, tôn trọng chính xác nhịp sinh học địa phương.
  - **Hiệu năng O(1)**: Việc kiểm tra và cập nhật ngày diễn ra ngay trên dòng `user_wallet` đang được lock, không tốn thêm câu lệnh SQL truy vấn lịch sử.
- **Negative / Trade-offs:**
  - Bảng `user_wallet` thêm 3 cột `DATE`, nhưng bù lại loại bỏ được hoàn toàn 1 bảng phụ và hạ tầng cron job.
