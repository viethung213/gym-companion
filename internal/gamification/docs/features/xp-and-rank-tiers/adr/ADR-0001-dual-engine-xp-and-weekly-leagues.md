# ADR-0001: Kiến Trúc Động Cơ XP Kép: XP Trọn Đời (Lifetime XP) & Giải Đấu Tuần (Weekly Leagues)

- **Feature**: xp-and-rank-tiers
- **Date**: 2026-10-09
- **Status**: Accepted
- **Deciders**: Maintainer (User), AI Assistant

## Context and Problem Statement
Trong nền tảng thể hình FITAI, động lực tập luyện của gymer cần được duy trì ở cả hai chiều thời gian:
1. **Dài hạn (Long-term Mastery)**: Người dùng cần cảm nhận được sự tiến bộ lũy kế không ngừng theo năm tháng (thâm niên, tổng khối lượng đã nâng, sự kiên trì).
2. **Ngắn hạn (Short-term Engagement)**: Người dùng cần một sân chơi thi đấu vừa sức hàng tuần để tạo sự hào hứng quay lại tập luyện mỗi ngày.

Đồng thời, tập gym về bản chất là hành trình nỗ lực cá nhân (PvE). Việc áp dụng các cơ chế trừng phạt tiêu cực (trừ điểm, phạt tụt cấp, suy giảm điểm khi nghỉ ngơi hay trong tuần deload phục hồi) sẽ gây ức chế tâm lý, trái ngược với nguyên tắc phục hồi cơ sinh học và dẫn đến tỷ lệ rời bỏ ứng dụng (churn rate) cao.

Hệ thống cần một kiến trúc tính điểm tạo động lực bền vững, công bằng và tôn trọng chu kỳ sinh học của gymer.

## Decision Drivers
- **100% Củng cố tích cực (Positive Reinforcement)**: Mọi nỗ lực tập luyện hợp lệ đều được ghi nhận điểm số. Tuyệt đối không có cơ chế trừ điểm, suy giảm điểm (decay), hay tụt Cấp độ (Level). Tuần deload hay ngày nghỉ không bị trừng phạt.
- **Tách bạch động lực dài hạn và cạnh tranh tuần**:
  - Dài hạn: Cấp độ trọn đời vĩnh viễn (Level 1–100) dựa trên Lifetime XP.
  - Ngắn hạn: Giải đấu tuần nhóm 30 người (Cohort of 30) dựa trên Weekly XP.
- **Tính thực dụng & Trực quan (Duolingo Mechanics)**: Mô hình bảng đấu 30 người với các bậc giải đấu (Đồng $\rightarrow$ Kim Cương) đã được kiểm chứng thành công trên quy mô hàng trăm triệu người dùng toàn cầu.

## Considered Options
- **Option 1 (Chỉ có điểm tích lũy trọn đời - Lifetime XP Only)**: Chỉ có 1 chỉ số XP duy nhất tăng dần.
  * *Hạn chế*: Người mới gia nhập nhìn vào top người dùng thâm niên 2-3 năm (hàng trăm nghìn XP) sẽ hoàn toàn mất động lực cạnh tranh.
- **Option 2 (Chỉ có điểm tuần reset hoàn toàn - Weekly XP Only)**: Điểm số chỉ có ý nghĩa trong tuần, hết tuần reset về 0.
  * *Hạn chế*: Không vinh danh được quá trình gắn bó và thành tựu lâu dài của gymer.
- **Option 3 (Động cơ XP Kép - Dual-Engine XP: Lifetime XP + Weekly Leagues)**: Kết hợp cả hai thước đo:
  * `total_xp`: Tích lũy trọn đời, ánh xạ Cấp độ (Level 1–100), chỉ tăng không giảm.
  * `weekly_xp`: Tích lũy trong tuần, xếp hạng trong nhóm 30 người cùng năng động, chốt thăng/rớt bậc giải đấu hàng tuần.

## Decision Outcome
Chosen: **Option 3 (Động cơ XP Kép - Dual-Engine XP)**.

### Architecture & Mechanics
1. **Lifetime XP & Cấp Độ Trọn Đời**:
   - `total_xp` được cộng dồn vĩnh viễn sau mỗi buổi tập và ngày dinh dưỡng hợp lệ.
   - Cấp độ xác định động qua hàm toán học:
     $$\text{Level} = \min(\lfloor \sqrt{\text{total\_xp} / 50} \rfloor + 1, 100)$$
   - Cấp độ vĩnh viễn không bao giờ bị hạ bậc.
2. **Weekly XP & Giải Đấu Tuần 30 Người**:
   - `weekly_xp` chỉ tính các hoạt động trong tuần hiện tại (00:00 Thứ Hai đến 23:59 Chủ Nhật).
   - Người dùng được ghép vào nhóm 30 người (Cohort of 30) ở bậc giải đấu tương ứng (Đồng, Bạc, Vàng, Bạch Kim, Kim Cương).
   - Cuối tuần: Top 7 thăng hạng (Promotion Zone), Bottom 5 rớt hạng (Demotion Zone), còn lại trụ hạng.

### Consequences
- **Positive:**
  - Giải quyết triệt để sự mất cân bằng giữa người mới và người cũ nhờ bảng đấu 30 người.
  - 100% phản hồi tích cực, không gây áp lực tiêu cực lên người dùng khi cần nghỉ ngơi.
  - Tách bạch rõ ràng giữa tiến trình cá nhân (Level) và xếp hạng cộng đồng (League).
- **Trade-offs:**
  - Cần duy trì cả hai cột `total_xp` và `weekly_xp` trong bảng dữ liệu `user_xp`.
