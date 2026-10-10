# ADR-0001: Tích Hợp Điểm Thưởng Kỷ Luật Dinh Dưỡng Vào Hệ Thống XP

- **Feature**: xp-and-rank-tiers
- **Date**: 2026-10-09
- **Status**: Accepted
- **Deciders**: Maintainer (User), AI Assistant

## Context and Problem Statement
Trong thể hình, dinh dưỡng chiếm vai trò quyết định đến hiệu quả phát triển cơ bắp và phục hồi thể lực. Nếu hệ thống Gamification chỉ đo lường khối lượng buổi tập (thuần `workout_execution`) mà bỏ qua hành vi ăn uống, chỉ số nỗ lực sẽ chưa phản ánh trọn vẹn tính kỷ luật của gymer.

Cần xác định cơ chế tích hợp dữ liệu từ module `nutrition` vào công thức thưởng XP, đồng thời giải quyết bài toán: Có trừ điểm khi người dùng ăn sai mục tiêu hoặc quên ghi nhật ký bữa ăn hay không?

## Decision Drivers
- Phản ánh trung thực tính kỷ luật toàn diện của người tập (Tập luyện + Dinh dưỡng).
- Bảo vệ độ sạch của dữ liệu: Không thúc đẩy người dùng gian lận hoặc nhập khống dữ liệu dinh dưỡng chỉ để giữ điểm số.
- Đơn giản, thực dụng, dễ giải thích cho người dùng cuối.

## Considered Options
- **Option 1 (Thưởng & Phạt hai chiều)**: Ăn chuẩn mục tiêu được cộng XP; ăn vượt calo hoặc quên ghi nhật ký bữa ăn bị trừ XP.
- **Option 2 (Chỉ Thưởng Dương - Positive Reinforcement Only)**: Ăn đạt chuẩn Calo/Protein mục tiêu được cộng thêm điểm thưởng XP cố định ($\Delta XP_{\text{nutrition}} = +30$ XP/ngày); tuyệt đối không trừ XP khi ăn lệch hoặc không ghi log.

## Decision Outcome
Chosen: **Option 2 (Chỉ Thưởng Dương)** vì:
1. **Tránh làm hỏng dữ liệu dinh dưỡng (3 AM Test)**: Nếu áp dụng hình phạt trừ điểm khi quên log hoặc ăn lố calo, người dùng sẽ ghi log giả mạo (fake-log) để bảo vệ thứ hạng. Dữ liệu giả sẽ phá vỡ các thuật toán gợi ý thực đơn và thích ứng của AI Coach.
2. **Khuyến khích hành vi tích cực**: Điểm thưởng dinh dưỡng đóng vai trò như một cú hích tích cực (Dopamine hit) động viên gymer theo dõi chế độ ăn uống khoa học.
3. **Đơn giản hóa trạng thái (State Minimization)**: Thưởng cố định $+30$ XP/ngày dựa trên ngày địa phương (`last_nutrition_reward_date`), không theo dõi chuỗi ngày (streak count) phức tạp trong module Gamification.

### Consequences
- **Positive:**
  - Khuyến khích người dùng ghi nhật ký bữa ăn trung thực mà không có tâm lý sợ bị phạt.
  - Phản ánh đúng nguyên lý thể hình: Dinh dưỡng tốt giúp nâng cao hiệu suất thể chất.
  - Schema dữ liệu tinh gọn, không cần lưu trữ hoặc tính toán chuỗi ngày dinh dưỡng.
- **Trade-offs / Negatives:**
  - Cần thêm logic tổng hợp nhật ký ăn uống cuối ngày hoặc xử lý sự kiện từ module `nutrition`.
- **Mitigation:**
  - Giới hạn trần thưởng dinh dưỡng cố định $+30$ XP/ngày (tối đa 1 lần/ngày) để không làm lu mờ vai trò cốt lõi của việc tập luyện nặng.
