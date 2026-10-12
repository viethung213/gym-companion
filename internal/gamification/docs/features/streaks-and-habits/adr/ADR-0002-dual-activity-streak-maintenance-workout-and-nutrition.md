# ADR-0002: Duy Trì Chuỗi Thói Quen Kép (Tập Luyện & Dinh Dưỡng) Bảo Vệ Phục Hồi Sinh Học

- **Feature**: streaks-and-habits
- **Date**: 2026-10-11
- **Status**: Accepted
- **Deciders**: Maintainer (User), AI Assistant

## Context and Problem Statement
Trong các ứng dụng Gamification thông thường (như học ngoại ngữ), việc duy trì chuỗi có thể thực hiện 7 ngày/tuần. Tuy nhiên, trong thể hình:
- Cơ bắp cần 48–72h nghỉ ngơi để tổng hợp protein và phục hồi cơ sinh học sau các buổi tập kháng lực nặng.
- Giáo án tiêu chuẩn của gymer thường gồm 3–5 buổi tập/tuần kết hợp 2–4 ngày nghỉ ngơi (Rest Days).
- Nếu ứng dụng bắt buộc ngày nào cũng phải đến phòng gym nâng tạ mới giữ được Streak, gymer sẽ bị đẩy vào tình trạng tập luyện quá tải (Overtraining), tăng nguy cơ chấn thương gân khớp và kiệt sức.

Hệ thống cần một định nghĩa thói quen thể hình toàn diện, vừa tôn vinh kỷ luật, vừa bảo vệ tính khoa học phục hồi sinh học.

## Decision Drivers
- **Khoa học thể hình & Phục hồi cơ bắp**: Không ép người dùng nâng tạ vào ngày nghỉ giáo án.
- **Kỷ luật thực chất (No Fake Streaks)**: Chỉ tính nỗ lực thực sự (tập luyện hoặc dinh dưỡng). Tuyệt đối không tính khi chỉ mở ứng dụng xem tin tức.
- **Đơn giản trong xử lý nghiệp vụ**: Ngày hoạt động (`active_day`) là một cờ Boolean duy nhất cho mỗi ngày địa phương.

## Considered Options

### Option 1: Chỉ Tính Buổi Tập (Workout-Only Streak)
- **Hạn chế**: Buộc người dùng phải đến phòng gym tập luyện cả vào các ngày nghỉ giáo án trong tuần $\rightarrow$ Gây ức chế tâm lý, chấn thương do quá tải và phá sản triết lý củng cố kỷ luật.

### Option 2: Tính Mở Ứng Dụng (Check-in / App Open)
- **Hạn chế**: Tạo thói quen ảo (Fake Habit). Người dùng chỉ vào mở app rồi tắt đi mà không hề tập luyện hay ăn uống kỷ luật, làm mất đi giá trị cốt lõi của nền tảng Gym Companion.

### Option 3: Kỷ Luật Thể Hình Kép (Workout OR Nutrition Adherence)
- **Cơ chế**:
  1. Ngày đi tập: Hoàn thành buổi tập hợp lệ ($\ge 15$ phút, $\ge 3$ set) $\rightarrow$ Ghi nhận hoàn thành chuỗi ngày.
  2. Ngày nghỉ tập: Ghi nhật ký ăn uống đạt chuẩn Calo (±10%) và Protein (≥90%) $\rightarrow$ Ghi nhận hoàn thành chuỗi ngày mà không cần đến phòng gym.
  3. Cả hai hoạt động đều thể hiện sự kỷ luật cao độ của gymer ("Cơ bắp xây dựng trong phòng tập, hoàn thiện trong nhà bếp").
  4. Nếu trong 1 ngày người dùng vừa tập vừa ăn đúng chuẩn: Chuỗi chỉ tăng 1 lần duy nhất cho ngày đó (Idempotent).

## Decision Outcome
Chosen: **Option 3 (Kỷ Luật Thể Hình Kép - Workout OR Nutrition Adherence)**.

### Consequences
- **Positive:**
  - Hoàn toàn tương thích với giáo án thể hình chuyên nghiệp (3–5 ngày tập + 2–4 ngày nghỉ phục hồi dinh dưỡng).
  - Tôn vinh kỷ luật ăn uống — yếu tố quyết định 70% thành công của thể hình.
  - Loại bỏ hoàn toàn thói quen ảo (mở app không được tính điểm).
- **Negative / Trade-offs:**
  - Module Gamification cần lắng nghe hai luồng sự kiện từ hai Bounded Context khác nhau: `workout_execution.events` và `nutrition.events`.
  - Cả hai Consumer đều gọi chung một Application Command `MaintainDailyStreakCommand` với logic khóa dòng bảo vệ tương tranh.
