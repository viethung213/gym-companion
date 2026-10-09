# ADR-0001: Tích Hợp Điểm Thưởng Kỷ Luật Dinh Dưỡng Vào Hệ Thống ELO

- **Feature**: elo-and-rank-tiers
- **Date**: 2026-10-09
- **Status**: Accepted
- **Deciders**: Maintainer (User), AI Assistant

## Context and Problem Statement
Trong thể hình, dinh dưỡng chiếm vai trò quyết định đến hiệu quả phát triển cơ bắp và phục hồi thể lực. Nếu hệ thống ELO chỉ đo lường khối lượng và chất lượng buổi tập (thuần `workout_execution`) mà bỏ qua hành vi ăn uống, chỉ số ELO sẽ chưa phản ánh trọn vẹn năng lực thể chất và tính kỷ luật của gymer.

Cần xác định cơ chế tích hợp dữ liệu từ module `nutrition` vào công thức tính ELO, đồng thời giải quyết bài toán: Có trừ điểm ELO khi người dùng ăn sai mục tiêu hoặc quên ghi nhật ký bữa ăn hay không?

## Decision Drivers
- Phản ánh trung thực tính kỷ luật toàn diện của người tập (Tập luyện + Dinh dưỡng).
- Bảo vệ độ sạch của dữ liệu: Không thúc đẩy người dùng gian lận hoặc nhập khống dữ liệu dinh dưỡng chỉ để giữ điểm số.
- Đơn giản, thực dụng, dễ giải thích cho người dùng cuối.

## Considered Options
- **Option 1 (Thưởng & Phạt hai chiều)**: Ăn chuẩn mục tiêu được cộng ELO; ăn vượt calo hoặc quên ghi nhật ký bữa ăn bị trừ ELO.
- **Option 2 (Chỉ Thưởng Dương - Positive Reinforcement Only)**: Ăn đạt chuẩn Calo/Protein mục tiêu được cộng thêm điểm thưởng ELO ($\Delta ELO_{\text{nutrition}} = +3 \rightarrow +5$ ELO) hoặc áp dụng hệ số nhân cho buổi tập ngày hôm đó; tuyệt đối không trừ ELO khi ăn lệch hoặc không ghi log.
- **Option 3 (Tách riêng ELO Dinh Dưỡng)**: Xây dựng một thang điểm Nutrition Score độc lập, không gộp vào `elo_rating`.

## Decision Outcome
Chosen: **Option 2 (Chỉ Thưởng Dương)** vì:
1. **Tránh làm hỏng dữ liệu dinh dưỡng (3 AM Test)**: Nếu áp dụng hình phạt trừ ELO khi quên log hoặc ăn lố calo, người dùng sẽ ghi log giả mạo (fake-log) để bảo vệ điểm ELO. Dữ liệu giả sẽ phá vỡ các thuật toán gợi ý thực đơn và thích ứng của AI Coach.
2. **Khuyến khích hành vi tích cực**: Điểm thưởng dinh dưỡng đóng vai trò như một cú hích tích cực (Dopamine hit) động viên gymer theo dõi chế độ ăn uống khoa học.

### Consequences
- **Positive:**
  - Khuyến khích người dùng ghi nhật ký bữa ăn trung thực mà không có tâm lý sợ bị trừ điểm.
  - Phản ánh đúng nguyên lý thể hình: Dinh dưỡng tốt giúp nâng cao hiệu suất thể chất.
- **Trade-offs / Negatives:**
  - Cần thêm logic tổng hợp nhật ký ăn uống cuối ngày (`DailyNutritionEvaluated`) hoặc xử lý sự kiện từ module `nutrition`.
- **Mitigation:**
  - Giới hạn trần thưởng dinh dưỡng tối đa $+5$ ELO/ngày để không làm lu mờ vai trò cốt lõi của việc tập luyện nặng.
