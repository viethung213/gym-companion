# ADR-0004: Tích Hợp Điểm Thưởng Kỷ Luật Dinh Dưỡng (+30 XP/ngày) Vào Hệ Thống XP

- **Feature**: xp-and-rank-tiers
- **Date**: 2026-10-09
- **Status**: Accepted
- **Deciders**: Maintainer (User), AI Assistant

## Context and Problem Statement
Trong thể hình, chế độ dinh dưỡng (đặc biệt là lượng Protein và Calo) quyết định tới $70\%$ kết quả phát triển cơ bắp và phục hồi thể chất. Tuy nhiên, việc ghi chép nhật ký ăn uống hàng ngày thường mang lại cảm giác nhàm chán và khó duy trì hơn nhiều so với việc đến phòng tập.

Cần một cơ chế động lực để thúc đẩy gymer duy trì kỷ luật dinh dưỡng, nhưng phải tuyệt đối tuân thủ triết lý củng cố tích cực của hệ thống Gamification.

## Decision Drivers
- **100% Củng cố tích cực**: Chỉ khen thưởng khi đạt mục tiêu; tuyệt đối không phạt trừ điểm khi người dùng quên ghi log, ghi thiếu, hoặc có ngày xả (cheat meal). Phạt trừ điểm sẽ dẫn đến ức chế và người dùng sẽ bỏ hoàn toàn tính năng dinh dưỡng.
- **Tính chuẩn xác và đơn giản**: Điểm thưởng dinh dưỡng được cố định ở mức $+30$ XP/ngày khi đạt cả mục tiêu Calo và Protein.
- **Tính lũy đẳng**: Mỗi ngày chỉ được nhận thưởng tối đa 1 lần duy nhất cho mỗi `user_id`.

## Considered Options
- **Option 1 (Cả thưởng lẫn phạt)**: Thưởng khi ăn đúng mục tiêu, trừ điểm khi ăn lố Calo hoặc không log.
  * *Hạn chế*: Phạt trừ điểm gây ra tác dụng ngược, phá hủy động lực của người dùng.
- **Option 2 (Thưởng dương cố định 100% - Fixed Bonus Only)**: Nhận cố định $+30$ XP/ngày khi sự kiện `DailyNutritionGoalAchieved` được bắn từ module dinh dưỡng.
  * *Ưu điểm*: Khuyến khích người dùng hình thành thói quen ăn uống lành mạnh mà không tạo áp lực tiêu cực.

## Decision Outcome
Chosen: **Option 2 (Thưởng dương cố định $+30$ XP/ngày)**.

### Consequences
- **Positive:**
  - Khuyến khích tính kỷ luật dinh dưỡng một cách tự nhiên và tích cực.
  - Tách bạch ranh giới: Module Gamification chỉ là Consumer tiếp nhận sự kiện từ Module Dinh dưỡng (Nutrition) qua Kafka CloudEvents.
  - Bảo vệ lũy đẳng qua index `uq_xp_history_user_date_reason` trên bảng `gamification.xp_history`.
- **Trade-offs:**
  - Cần lắng nghe thêm topic sự kiện dinh dưỡng `nutrition.events` bên cạnh topic tập luyện `workout.events`.
