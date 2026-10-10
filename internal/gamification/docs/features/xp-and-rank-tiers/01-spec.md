# Feature: XP (Experience Points) & Weekly Leagues (Duolingo Style)

## Summary
Tính năng **XP (Experience Points) & Weekly Leagues** cung cấp cơ chế tạo động lực hai trục (Dual-Engine) chuẩn theo mô hình thành công của Duolingo cho nền tảng Gym Companion:
1. **Trục Dài Hạn (Lifetime XP & Levels)**: Điểm kinh nghiệm tích lũy vĩnh viễn (chỉ có chiều tăng dương, không bao giờ bị giảm, không decay), nâng cấp Level từ 1 đến 100 vinh danh sự bền bỉ.
2. **Trục Ngắn Hạn (Weekly XP & Leagues)**: Điểm kinh nghiệm kiếm được trong tuần hiện tại (Thứ Hai $\rightarrow$ Chủ Nhật), làm cơ sở phân hạng giải đấu tuần qua 5 bậc hạng (Bronze $\rightarrow$ Diamond).

## Problem and desired outcome
- **Problem**: 
  - Tập gym là hành trình bền bỉ đầy thử thách; người mới tập rất dễ nản lòng và bỏ cuộc nếu không nhận được sự ghi nhận kịp thời và trực quan cho từng nỗ lực hàng ngày.
  - Các hệ thống xếp hạng thông thường thường thiếu sự cân bằng: hoặc chỉ có điểm tích lũy dài hạn khiến người mới không thể bắt kịp người cũ, hoặc chỉ có điểm ngắn hạn khiến nỗ lực gắn bó lâu năm không được vinh danh.
  - Người tập thể hình cần chu kỳ nghỉ ngơi và phục hồi cơ bắp (deload), nên bất kỳ cơ chế phạt trừ điểm nào khi nghỉ ngơi đều gây ức chế tâm lý và phá vỡ quy luật sinh học.
- **Desired outcome**:
  - **100% Khích lệ tích cực (Positive Reinforcement)**: Bất kỳ nỗ lực nào tại phòng gym đều được cộng điểm XP. Tuyệt đối không trừ điểm khi tập nhẹ, nghỉ ngơi, hay deload.
  - **Minh bạch, dễ hiểu**: Công thức tính XP rõ ràng theo từng thành phần (Base check-in, Volume tạ, Kỹ thuật form AI, Kỷ lục PR).
  - **Động lực kép (Dual-Engine)**: Cày cấp độ dài hạn (Level 1..100) kết hợp với đua top giải đấu tuần nhóm 30 người (Weekly Leagues).

## Actors and entry points
- **Gym User**: Xem tổng XP, cấp độ hiện tại (Level), XP tuần, bậc giải đấu và lịch sử tích lũy trên ứng dụng.
- **Kafka Consumer**:
  - Tiếp nhận sự kiện buổi tập: `WorkoutSessionCompleted` (`workout_execution.events`).
  - Tiếp nhận sự kiện dinh dưỡng: `MealLogged` (`nutrition.events`).
- **Transactional Outbox Worker**: Đẩy sự kiện `XpEarned`, `UserLeveledUp` sang Kafka topic `gamification.events`.

## Scope

### In scope
- Khởi tạo hồ sơ XP cơ sở (`total_xp = 0`, `current_level = 1`, `weekly_xp = 0`, `rank_tier = BRONZE`) theo cơ chế Lazy Onboarding.
- Tính toán điểm thưởng $\Delta XP$ sau mỗi buổi tập hoàn thành: Base check-in ($+50$ XP), Khối lượng nâng ($+10 \rightarrow +30$ XP), Kỹ thuật động tác ($+10 \rightarrow +20$ XP), và Kỷ lục cá nhân PR ($+25$ XP).
- Thưởng kỷ luật dinh dưỡng hàng ngày cố định ($+30$ XP/ngày) khi đạt mục tiêu Calo/Protein (ADR-0004).
- Tích hợp hệ số nhân chuỗi ngày tập (Streak Multiplier: $+10\%$ nếu streak $\ge 3$, $+20\%$ nếu streak $\ge 7$).
- Cơ chế **Lazy Reset on Activity** cho `weekly_xp` dựa trên chu kỳ tuần ISO (`current_week_number`), triệt tiêu 100% rủi ro nghẽn DB lúc nửa đêm Chủ nhật (ADR-0002).
- Tự động thăng cấp Level ($1 \rightarrow 100$) khi `total_xp` chạm các ngưỡng mốc.
- Phát CloudEvents: `XpEarned`, `UserLeveledUp`.

### Out of scope & Deferred
- **Computer Vision**: Đếm rep, chấm điểm form (thuộc module `workout_execution`).
- **Solo PvP 1v1 [DEFERRED]**: Đấu đối kháng thời gian thực tạm hoãn chờ module Competition.
- **Cohort Matching 30 người**: Thuật toán chia nhóm bảng đấu chi tiết thuộc module `leaderboards`.

## Current behavior
Module gamification đang trong giai đoạn hoàn thiện tài liệu đặc tả, chưa có mã nguồn triển khai trong database hay production.

## User scenarios / use cases

### UC-XP-01: Nhận XP sau buổi tập hoàn thành
- **Actor**: Kafka Consumer (`workout_execution.events`).
- **Preconditions**: Buổi tập hoàn thành hợp lệ ($\ge 15$ phút, $\ge 3$ hiệp), có `session_id`, `user_id`.
- **Trigger**: Nhận CloudEvent `WorkoutSessionCompleted`.
- **Main Flow**:
  1. Kiểm tra tính lũy đẳng: Nếu `session_id` đã tồn tại trong `xp_history` (`source_event_id = session_id`), bỏ qua an toàn và commit offset.
  2. Nạp hồ sơ XP người dùng với khóa dòng bi quan `SELECT ... FOR UPDATE` (ADR-0003). Khởi tạo mặc định nếu chưa tồn tại (Lazy Onboarding).
  3. Kiểm tra chu kỳ tuần: Nếu `user.current_week_number != current_week`, đặt `weekly_xp = 0` và gán `current_week_number = current_week` (Lazy Reset).
  4. Tính toán $\Delta XP$ từ Base ($50$), Volume Ratio, Form Score, cờ PR và Streak Multiplier (nếu có).
  5. Cập nhật `total_xp += delta_xp`, `weekly_xp += delta_xp`, cập nhật `last_workout_at = workout_time`.
  6. Đánh giá cấp độ mới: Nếu `total_xp` vượt ngưỡng cấp độ tiếp theo, tăng `current_level` và phát sự kiện `UserLeveledUp`.
  7. Ghi nhận biến động vào sổ cái `xp_history` với `source_event_id = session_id`.
  8. Ghi nhận sự kiện `XpEarned` (và `UserLeveledUp` nếu có) vào Transactional Outbox trong cùng một transaction.
- **Postconditions**: Điểm XP và Level cập nhật tuần tự, sự kiện sẵn sàng đẩy ra Kafka.

### UC-XP-02: Thưởng XP từ kỷ luật dinh dưỡng hàng ngày
- **Actor**: Kafka Consumer (`nutrition.events`).
- **Preconditions**: Người dùng ghi nhật ký bữa ăn trong ngày qua module `nutrition`.
- **Trigger**: Nhận CloudEvent `MealLogged`.
- **Main Flow**:
  1. Kiểm tra ngày nhận thưởng: Nếu `last_nutrition_reward_date == user_local_date`, bỏ qua.
  2. Đánh giá chỉ số ngày: Calo đạt $\pm 10\%$ và Protein đạt $\ge 90\%$ mục tiêu do AI Coach đề ra.
  3. Nếu đạt chuẩn:
     - Mở transaction khóa dòng bi quan trên `user_xp`.
     - Kiểm tra chu kỳ tuần và Lazy Reset `weekly_xp` nếu sang tuần mới.
     - Cộng cố định $+30$ XP vào `total_xp` và `weekly_xp`.
     - Cập nhật `last_nutrition_reward_date = user_local_date`.
     - Đánh giá Level Up nếu chạm mốc.
     - Ghi log `xp_history` lý do `NUTRITION_ADHERENCE`, lưu sự kiện `XpEarned` vào Outbox.
  4. Nếu không đạt chuẩn hoặc quên ghi log: Không phạt, nhận 0 XP (ADR-0004).
- **Postconditions**: Người dùng nhận tối đa 30 XP dinh dưỡng mỗi ngày.

### UC-XP-03: Truy vấn thông tin XP cá nhân và tiến trình cấp độ
- **Actor**: Gym User (Mobile/Web Client).
- **Preconditions**: Người dùng đã đăng nhập, gửi Bearer Token hợp lệ.
- **Trigger**: Người dùng mở màn hình Gamification hoặc Profile.
- **Main Flow**:
  1. Client gọi API `GetMyXp`.
  2. Hệ thống kiểm tra Lazy Reset (nếu đã sang tuần mới nhưng chưa có event ghi), trả về:
     - `total_xp`: Tổng XP trọn đời.
     - `current_level`: Cấp độ hiện tại ($1..100$).
     - `xp_to_next_level`: Số XP cần thêm để lên cấp tiếp theo.
     - `weekly_xp`: XP tích lũy trong tuần hiện tại.
     - `rank_tier`: Bậc giải đấu tuần hiện tại.
     - `current_week_number`: Mã tuần hiện tại (ví dụ: `2026-W41`).
  3. Client gọi API `GetXpHistory` để nhận danh sách lịch sử nhận XP phân trang (Pagination).
- **Postconditions**: Người dùng nắm bắt trực quan tiến trình cày cấp và vị thế giải đấu tuần.

## Functional requirements
- **FR-XP-01**: Khởi tạo hồ sơ XP mức cơ sở (`total_xp = 0`, `current_level = 1`, `weekly_xp = 0`, `rank_tier = BRONZE`) khi người dùng mới phát sinh hoạt động hoặc truy vấn lần đầu.
- **FR-XP-02**: Tính toán điểm thưởng $\Delta XP \ge 0$ cho mỗi buổi tập hoàn thành hợp lệ và cộng nguyên tử vào `total_xp` và `weekly_xp`.
- **FR-XP-03**: Thưởng cố định $+30$ XP/ngày khi đạt chuẩn dinh dưỡng, tối đa 1 lần/ngày theo `user_local_date` (ADR-0004).
- **FR-XP-04**: Tự động thăng cấp Level ($1 \rightarrow 100$) khi `total_xp` chạm các ngưỡng mốc, phát sự kiện `UserLeveledUp`.
- **FR-XP-05**: Tự động reset `weekly_xp = 0` khi phát hiện chu kỳ tuần mới (`current_week_number != now.WeekNumber`) theo cơ chế Lazy Reset (ADR-0002).
- **FR-XP-06**: Tuyệt đối không có hành động trừ điểm XP trong mọi tình huống (kể cả nghỉ tập, ăn cheat meal, hay tuần deload).
- **FR-XP-07**: Lưu trữ toàn bộ lịch sử biến động XP dưới dạng append-only trong `xp_history` để phục vụ đối soát và đồ thị tiến trình.
- **FR-XP-08**: Bảo đảm tính lũy đẳng: mỗi `session_id` buổi tập chỉ được nhận thưởng XP duy nhất 1 lần.

## Business rules and invariants
- **BR-XP-01 (Positive Reinforcement Only)**: Điểm biến động luôn dương ($\Delta XP > 0$). `total_xp` là hàm đơn điệu tăng, không bao giờ suy giảm.
- **BR-XP-02 (Workout Formula)**: Điểm một buổi tập là tổng của các thành phần minh bạch:
  $$\Delta XP_{\text{workout}} = \text{round}\left( (\text{BaseXP} + \text{VolumeXP} + \text{FormXP} + \text{PRBonus}) \times \text{StreakMultiplier} \right)$$
  - $\text{BaseXP} = 50$.
  - $\text{VolumeXP} = \text{clamp}(\text{round}(25 \times \text{VolumeRatio}), 10, 30)$.
  - $\text{FormXP} = \text{round}(20 \times \frac{\text{FormScore}}{100})$.
  - $\text{PRBonus} = 25$ (nếu `isPR = true`, ngược lại $0$).
  - $\text{StreakMultiplier} = 1.0$ (mặc định), $1.1$ (nếu streak $\ge 3$), $1.2$ (nếu streak $\ge 7$).
  - Dao động chuẩn trong 1 buổi tập: $70 \rightarrow 150$ XP.
- **BR-XP-03 (Nutrition Bonus)**: Thưởng dương cố định $+30$ XP/ngày, tối đa 1 lần/ngày, không trừ điểm khi ăn sai (ADR-0004).
- **BR-XP-04 (Level Thresholds)**: Cấp độ $1..100$ được tính theo công thức lũy tiến chuẩn:
  $$\text{Level}(X) = \min\left(\lfloor \sqrt{X / 50} \rfloor + 1, 100\right) \quad (\text{với } X = \text{total\_xp})$$
  - Level 1: $0$ XP.
  - Level 2: $100$ XP.
  - Level 3: $250$ XP.
  - Level 5: $800$ XP.
  - Level 10: $4,050$ XP.
  - Level 50: $120,050$ XP.
- **BR-XP-05 (Weekly Cycle & Lazy Reset)**: Chu kỳ tuần bắt đầu lúc Thứ Hai 00:00:00 UTC và kết thúc Chủ Nhật 23:59:59 UTC. Hệ thống theo dõi chuỗi định danh tuần ISO (ví dụ: `2026-W41`). Reset lười diễn ra ngay khi phát sinh thao tác ghi đầu tiên của tuần mới (ADR-0002).
- **BR-XP-06 (Concurrency & Idempotency)**: Mọi thao tác ghi điểm trên cùng một `user_id` phải tuần tự hóa qua khóa dòng bi quan `SELECT ... FOR UPDATE` (ADR-0003). Mỗi `session_id` chỉ tính điểm duy nhất 1 lần, bảo vệ bởi ràng buộc duy nhất trên `xp_history(user_id, source_event_id)`.

## Error and boundary scenarios
- **Duplicate Workout Event**: Sự kiện trùng lặp `session_id` bị chặn bởi unique index trên `xp_history`, transaction rollback an toàn và commit offset bỏ qua.
- **New Week First Workout**: Khi nhận buổi tập đầu tiên của tuần mới, `weekly_xp` được reset về $0$ trước khi cộng điểm mới, `total_xp` tiếp tục cộng dồn bình thường.
- **Max Level 100 Reached**: Khi người dùng đạt Level 100, `total_xp` và `weekly_xp` vẫn tiếp tục tăng bình thường, nhưng `current_level` giữ nguyên ở 100 và `xp_to_next_level = 0`.
- **Anomalous Session**: Buổi tập thời lượng $< 15$ phút hoặc hiệp $< 3$ bị coi là chưa đạt chuẩn, không cộng XP.

## Acceptance criteria
- [ ] GIVEN user mới (0 XP, Level 1) WHEN hoàn thành buổi tập chuẩn nhận 80 XP THEN `total_xp = 80`, `weekly_xp = 80`, `current_level = 1`.
- [ ] GIVEN user 90 XP (Level 1, mốc Level 2 là 100 XP) WHEN hoàn thành buổi tập nhận 80 XP THEN `total_xp = 170`, `current_level = 2` và phát sự kiện `UserLeveledUp`.
- [ ] GIVEN user có `weekly_xp = 350` ở tuần `2026-W40` WHEN hoàn thành buổi tập 80 XP ở tuần `2026-W41` THEN `weekly_xp` reset và cập nhật thành `80`, `current_week_number = '2026-W41'`.
- [ ] GIVEN CloudEvent `WorkoutSessionCompleted` gửi lại lần 2 THEN hệ thống bỏ qua và không cộng lặp XP.
- [ ] GIVEN user ghi nhận dinh dưỡng đạt chuẩn lần 1 trong ngày THEN nhận đúng $+30$ XP; gửi lần 2 trong cùng ngày THEN bỏ qua an toàn.
- [ ] GIVEN hai sự kiện cộng XP gửi đến đồng thời trên 1 user THEN thực thi tuần tự qua khóa dòng, 0% Lost Update.

## Architecture Decision Records (ADRs)
- [ADR-0001: Kiến Trúc Động Cơ XP Kép: XP Trọn Đời & Giải Đấu Tuần](./adr/ADR-0001-dual-engine-xp-and-weekly-leagues.md)
- [ADR-0002: Cơ Chế Làm Mới Tuần Lười Biếng (Lazy Week Reset)](./adr/ADR-0002-lazy-week-reset-on-user-activity.md)
- [ADR-0003: Sử Dụng Khóa Dòng Bi Quan SELECT FOR UPDATE Cho Cập Nhật Điểm XP](./adr/ADR-0003-use-pessimistic-row-locking-for-xp-updates.md)
- [ADR-0004: Tích Hợp Điểm Thưởng Kỷ Luật Dinh Dưỡng (+30 XP/ngày) Vào Hệ Thống XP](./adr/ADR-0004-incorporate-nutrition-adherence-bonus-into-xp.md)
