# ADR-0001: Kiến Trúc Điểm Kinh Nghiệm (XP) & Cấp Độ (Level 1–100)

- **Feature**: xp-and-levels
- **Date**: 2026-10-09
- **Status**: Accepted
- **Deciders**: Maintainer (User), AI Assistant

## Context and Problem Statement
Trong nền tảng thể hình FITAI, người tập gym cần một cơ chế trực quan, liên tục để cảm nhận được sự tiến bộ lũy kế của bản thân qua từng buổi tập (tổng nỗ lực, khối lượng tạ, kỹ thuật chuẩn xác và tính kỷ luật).

Tập gym về bản chất là hành trình nỗ lực cá nhân (PvE). Việc áp dụng các cơ chế trừng phạt tiêu cực (trừ điểm, phạt tụt cấp, suy giảm điểm khi nghỉ ngơi hay trong tuần deload phục hồi) sẽ gây ức chế tâm lý, trái ngược với nguyên tắc phục hồi cơ sinh học và dẫn đến tỷ lệ rời bỏ ứng dụng (churn rate) cao.

Hệ thống cần một đơn vị đo lường thống nhất, đơn giản và tuân thủ tuyệt đối nguyên tắc củng cố tích cực.

## Decision Drivers
- **100% Củng cố tích cực (Positive Reinforcement)**: Mọi nỗ lực tập luyện hợp lệ đều được cộng điểm. Tuyệt đối không có cơ chế trừ điểm, suy giảm điểm (decay), hay tụt Cấp độ (Level). Tuần deload hay ngày nghỉ không bao giờ bị trừng phạt.
- **Tối giản cực đại (Simplicity First)**: Sử dụng duy nhất một chỉ số đo lường **XP (`xp`)** trong toàn bộ hệ thống; không duy trì các biến thể điểm số phức tạp.
- **Hàm cấp độ trực quan (Level 1–100)**: Cấp độ tăng dần độ khó theo hàm căn bậc hai, phản ánh đúng quy luật nỗ lực tăng tiến trong thể hình.

## Considered Options
- **Option 1 (Nhiều loại điểm số song song: XP, Weekly XP, Elo)**:
  * *Hạn chế*: Phức tạp hóa kiến trúc, phát sinh logic quản lý chuyển tuần (Lazy Week Reset), dễ gây lỗi đồng bộ.
- **Option 2 (Đơn vị đo lường duy nhất: XP & Level)**:
  * Toàn bộ hệ thống chỉ lưu một chỉ số duy nhất là `xp`.
  * `level` được tính toán trực tiếp từ `xp`.
  * Bảng xếp hạng và các tính năng khác đều dùng chung chỉ số `xp` này.

## Decision Outcome
Chosen: **Option 2 (Đơn vị đo lường duy nhất: XP & Level)**.

### Architecture & Mechanics
1. **Chỉ số XP (`xp`)**:
   - `xp` được cộng dồn vĩnh viễn sau mỗi buổi tập và ngày dinh dưỡng hợp lệ.
   - `xp` là hàm đơn điệu tăng, không bao giờ bị giảm.
2. **Cấp độ (Level 1–100)**:
   - Cấp độ xác định động qua hàm toán học:
     $$\text{Level} = \min(\lfloor \sqrt{X / 50} \rfloor + 1, 100) \quad (\text{với } X \text{ là } \texttt{xp})$$
   - Cấp độ vĩnh viễn không bao giờ bị hạ bậc.
3. **Mô hình dữ liệu tinh gọn**:
   - Bảng `gamification.user_xp`: chỉ gồm `user_id`, `xp`, `level`, `updated_at`.

### Consequences
- **Positive:**
  - 100% phản hồi tích cực, không gây áp lực tiêu cực lên người dùng khi cần nghỉ ngơi.
  - Code Aggregate, Repository, và API siêu gọn; không cần quản lý tuần, không cần reset dữ liệu định kỳ.
  - Triệt tiêu hoàn toàn các lỗi liên quan đến múi giờ hoặc chuyển tuần.
- **Trade-offs:**
  - Không có cơ chế xếp hạng theo tuần riêng biệt; bảng xếp hạng dùng trực tiếp tổng `xp`.
