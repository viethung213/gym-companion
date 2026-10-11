# Feature: Hệ Thống Điểm Kinh Nghiệm (XP) & Cấp Độ (XP & Level Progression)

## Summary
Tính năng **Hệ Thống Điểm Kinh Nghiệm (XP) & Cấp Độ (XP & Level Progression)** cung cấp cơ chế tạo động lực cốt lõi cho nền tảng Gym Companion:
- Đơn vị đo lường nỗ lực duy nhất: **Điểm Kinh Nghiệm (`xp`)**.
- Tích lũy vĩnh viễn (chỉ tăng, không bao giờ giảm, không suy giảm theo thời gian), 100% củng cố tích cực (Positive Reinforcement).
- Nâng cấp Level từ 1 đến 100 để vinh danh sự bền bỉ và quá trình rèn luyện của gymer.

---

## Problem and desired outcome

### Problem
- Tập gym là hành trình bền bỉ đầy thử thách; người mới tập rất dễ nản lòng và bỏ cuộc nếu không nhận được sự ghi nhận kịp thời và trực quan cho từng nỗ lực hàng ngày.
- Người tập thể hình cần chu kỳ nghỉ ngơi và phục hồi cơ bắp (deload), nên bất kỳ cơ chế phạt trừ điểm nào khi nghỉ ngơi đều gây ức chế tâm lý và phá vỡ quy luật sinh học.

### Desired outcome
- **100% Khích lệ tích cực (Positive Reinforcement)**: Bất kỳ nỗ lực nào tại phòng gym đều được cộng điểm XP. Tuyệt đối không trừ điểm khi tập nhẹ, nghỉ ngơi, hay deload.
- **Minh bạch, dễ hiểu**: Công thức tính XP rõ ràng theo từng thành phần (Base check-in, Volume tạ, Kỹ thuật form AI, Kỷ lục PR).
- **Tiến trình cấp độ trực quan (Level 1–100)**: Điểm số tự động ánh xạ sang cấp độ tăng tiến, mang lại cảm giác chinh phục liên tục.

---

## Actors and entry points
- **Gym User**: Xem tổng điểm XP, cấp độ hiện tại (Level), số XP cần để lên cấp tiếp theo (`xp_to_next_level`) và lịch sử biến động điểm trên ứng dụng.
- **Kafka Consumer**:
  - Tiếp nhận sự kiện buổi tập: `WorkoutSessionCompleted` (`workout_execution.events`).
  - Tiếp nhận sự kiện dinh dưỡng: `MealLogged` (`nutrition.events`).
- **Transactional Outbox Worker**: Đẩy sự kiện `XpEarned`, `UserLeveledUp` sang Kafka topic `gamification.events`.

---

## Scope

### In scope
- Khởi tạo hồ sơ XP cơ sở (`xp = 0`, `level = 1`) theo cơ chế Lazy Onboarding khi người dùng mới phát sinh hoạt động hoặc truy vấn lần đầu.
- Tính toán điểm thưởng $\Delta XP$ sau mỗi buổi tập hoàn thành: Base check-in ($+50$ XP), Khối lượng nâng ($+10 \rightarrow +30$ XP), Kỹ thuật động tác ($+10 \rightarrow +20$ XP), và Kỷ lục cá nhân PR ($+25$ XP).
- Thưởng kỷ luật dinh dưỡng hàng ngày cố định ($+30$ XP/ngày) khi đạt mục tiêu Calo/Protein (ADR-0003).
- Tích hợp hệ số nhân chuỗi ngày tập (Streak Multiplier: $+10\%$ nếu streak $\ge 3$, $+20\%$ nếu streak $\ge 7$).
- Tự động thăng cấp Level ($1 \rightarrow 100$) khi `xp` chạm các ngưỡng mốc (ADR-0001).
- Phát CloudEvents: `XpEarned`, `UserLeveledUp`.
- Bảo vệ tuần tự hóa và chống Lost Update bằng khóa dòng bi quan (ADR-0002).

### Out of scope & Deferred
- **Computer Vision**: Đếm rep, chấm điểm form (thuộc module `workout_execution`).
- **Solo PvP 1v1 [DEFERRED]**: Đấu đối kháng thời gian thực tạm hoãn chờ module Competition.
- **Bảng Xếp Hạng [SEPARATE FEATURE]**: Bảng xếp hạng toàn hệ thống được đặc tả riêng tại `leaderboards`.

---

## Current behavior
Module gamification đang trong giai đoạn hoàn thiện tài liệu đặc tả, chưa có mã nguồn triển khai trong database hay production.

---

## User scenarios / use cases

### UC-XP-01: Nhận XP sau buổi tập hoàn thành
- **Actor**: Kafka Consumer (`workout_execution.events`).
- **Preconditions**: Buổi tập hoàn thành hợp lệ ($\ge 15$ phút, $\ge 3$ hiệp), có `session_id`, `user_id`.
- **Trigger**: Nhận CloudEvent `WorkoutSessionCompleted`.
- **Main Flow**:
  1. Kiểm tra tính lũy đẳng: Nếu `session_id` đã tồn tại trong `xp_history` (`source_event_id = session_id`), bỏ qua an toàn và commit offset.
  2. Nạp hồ sơ XP người dùng với khóa dòng bi quan `SELECT ... FOR UPDATE` (ADR-0002). Khởi tạo mặc định nếu chưa tồn tại (Lazy Onboarding).
  3. Tính toán $\Delta XP$ từ Base ($50$), Volume Ratio, Form Score, cờ PR và Streak Multiplier (nếu có).
  4. Cập nhật `xp += delta_xp`, cập nhật `updated_at = now`.
  5. Đánh giá cấp độ mới: Nếu `xp` vượt ngưỡng cấp độ tiếp theo, tăng `level` và phát sự kiện `UserLeveledUp`.
  6. Ghi nhận biến động vào sổ cái `xp_history` với `source_event_id = session_id`.
  7. Ghi nhận sự kiện `XpEarned` (và `UserLeveledUp` nếu có) vào Transactional Outbox trong cùng một transaction.
- **Postconditions**: Điểm XP và Level cập nhật tuần tự, sự kiện sẵn sàng đẩy ra Kafka.

### UC-XP-02: Thưởng XP từ kỷ luật dinh dưỡng hàng ngày
- **Actor**: Kafka Consumer (`nutrition.events`).
- **Preconditions**: Người dùng ghi nhật ký bữa ăn trong ngày qua module `nutrition`.
- **Trigger**: Nhận CloudEvent `MealLogged`.
- **Main Flow**:
  1. Kiểm tra ngày nhận thưởng: Nếu `last_nutrition_reward_date == user_local_date`, bỏ qua.
  2. Đánh giá chỉ số ngày: Calo đạt $\pm 10\%$ và Protein đạt $\ge 90\%$ mục tiêu do AI Coach đề ra.
  3. Nếu đạt chuẩn:
     - Mở transaction khóa dòng bi quan trên `user_xp` (ADR-0002).
     - Cộng cố định $+30$ XP vào `xp` (ADR-0003).
     - Cập nhật `last_nutrition_reward_date = user_local_date`.
     - Đánh giá Level Up nếu chạm mốc mới.
     - Ghi log `xp_history` lý do `NUTRITION_ADHERENCE`, lưu sự kiện `XpEarned` vào Outbox.
  4. Nếu không đạt chuẩn hoặc quên ghi log: Không phạt, nhận 0 XP.
- **Postconditions**: Người dùng nhận tối đa 30 XP dinh dưỡng mỗi ngày.

### UC-XP-03: Truy vấn thông tin XP cá nhân và tiến trình cấp độ
- **Actor**: Gym User (Mobile/Web Client).
- **Preconditions**: Người dùng đã đăng nhập, gửi Bearer Token hợp lệ.
- **Trigger**: Người dùng mở màn hình Profile hoặc Home Dashboard.
- **Main Flow**:
  1. Client gửi request ConnectRPC `GetMyXp`.
  2. Server nạp bản ghi `user_xp`:
     - Nếu chưa tồn tại: Trả về trạng thái mặc định (`xp = 0`, `level = 1`, `xp_to_next_level = 100`).
     - Nếu đã tồn tại: Tính toán động `xp_to_next_level` từ `xp` hiện tại và trả về:
       - `xp`: Tổng điểm kinh nghiệm.
       - `level`: Cấp độ hiện tại ($1..100$).
       - `xp_to_next_level`: Số XP cần thêm để đạt cấp tiếp theo.
  3. Client gọi API `GetXpHistory` để nhận danh sách lịch sử nhận XP phân trang (Pagination).
- **Postconditions**: Người dùng nắm bắt trực quan tiến trình tích lũy điểm và thanh tiến độ cấp độ.

---

## Functional requirements
- **FR-XP-01**: Khởi tạo hồ sơ XP mức cơ sở (`xp = 0`, `level = 1`) khi người dùng mới phát sinh hoạt động hoặc truy vấn lần đầu.
- **FR-XP-02**: Tính toán điểm thưởng $\Delta XP \ge 0$ cho mỗi buổi tập hoàn thành hợp lệ và cộng nguyên tử vào `xp`.
- **FR-XP-03**: Thưởng cố định $+30$ XP/ngày khi đạt chuẩn dinh dưỡng, tối đa 1 lần/ngày theo `user_local_date` (ADR-0003).
- **FR-XP-04**: Tự động thăng cấp Level ($1 \rightarrow 100$) khi `xp` chạm các ngưỡng mốc, phát sự kiện `UserLeveledUp` (ADR-0001).
- **FR-XP-05**: Tuyệt đối không có hành động trừ điểm XP trong mọi tình huống (kể cả nghỉ tập, ăn cheat meal, hay tuần deload).
- **FR-XP-06**: Lưu trữ toàn bộ lịch sử biến động XP dưới dạng append-only trong `xp_history` để phục vụ đối soát và đồ thị tiến trình.
- **FR-XP-07**: Bảo đảm tính lũy đẳng: mỗi `session_id` buổi tập chỉ được nhận thưởng XP duy nhất 1 lần.

---

## Business rules and invariants
- **BR-XP-01 (Positive Reinforcement Only)**: Điểm biến động luôn dương ($\Delta XP > 0$). `xp` là hàm đơn điệu tăng, không bao giờ suy giảm.
- **BR-XP-02 (Workout Formula)**: Điểm một buổi tập là tổng của các thành phần minh bạch:
  $$\Delta XP_{\text{workout}} = \text{round}\left( (\text{BaseXP} + \text{VolumeXP} + \text{FormXP} + \text{PRBonus}) \times \text{StreakMultiplier} \right)$$
  - $\text{BaseXP} = 50$.
  - $\text{VolumeXP} = \text{clamp}(\text{round}(25 \times \text{VolumeRatio}), 10, 30)$.
  - $\text{FormXP} = \text{round}(20 \times \frac{\text{FormScore}}{100})$.
  - $\text{PRBonus} = 25$ (nếu `isPR = true`, ngược lại $0$).
  - $\text{StreakMultiplier} = 1.0$ (mặc định), $1.1$ (nếu streak $\ge 3$), $1.2$ (nếu streak $\ge 7$).
  - Dao động chuẩn trong 1 buổi tập: $70 \rightarrow 150$ XP.
- **BR-XP-03 (Nutrition Bonus)**: Thưởng dương cố định $+30$ XP/ngày, tối đa 1 lần/ngày, không trừ điểm khi ăn sai (ADR-0003).
- **BR-XP-04 (Level Thresholds)**: Cấp độ $1..100$ được tính theo công thức lũy tiến chuẩn:
  $$\text{Level}(X) = \min\left(\lfloor \sqrt{X / 50} \rfloor + 1, 100\right) \quad (\text{với } X \text{ là } \texttt{xp})$$
  - Level 1: $0$ XP.
  - Level 2: $100$ XP.
  - Level 3: $250$ XP.
  - Level 5: $800$ XP.
  - Level 10: $4,050$ XP.
  - Level 50: $120,050$ XP.
- **BR-XP-05 (Concurrency & Idempotency)**: Mọi thao tác ghi điểm trên cùng một `user_id` phải tuần tự hóa qua khóa dòng bi quan `SELECT ... FOR UPDATE` (ADR-0002). Mỗi `session_id` chỉ tính điểm duy nhất 1 lần, bảo vệ bởi ràng buộc duy nhất trên `xp_history(user_id, source_event_id)`.

---

## Error and boundary scenarios
- **Duplicate Workout Event**: Sự kiện trùng lặp `session_id` bị chặn bởi unique index trên `xp_history`, transaction rollback an toàn và commit offset bỏ qua.
- **Max Level 100 Reached**: Khi người dùng đạt Level 100, `xp` vẫn tiếp tục tăng bình thường, nhưng `level` giữ nguyên ở 100 và `xp_to_next_level = 0`.
- **Anomalous Session**: Buổi tập thời lượng $< 15$ phút hoặc hiệp $< 3$ bị coi là chưa đạt chuẩn, không cộng XP.

---

## Acceptance criteria
- [ ] GIVEN user mới (0 XP, Level 1) WHEN hoàn thành buổi tập chuẩn nhận 80 XP THEN `xp = 80`, `level = 1`.
- [ ] GIVEN user 90 XP (Level 1, mốc Level 2 là 100 XP) WHEN hoàn thành buổi tập nhận 80 XP THEN `xp = 170`, `level = 2` và phát sự kiện `UserLeveledUp`.
- [ ] GIVEN CloudEvent `WorkoutSessionCompleted` gửi lại lần 2 THEN hệ thống bỏ qua và không cộng lặp XP.
- [ ] GIVEN user ghi nhận dinh dưỡng đạt chuẩn lần 1 trong ngày THEN nhận đúng $+30$ XP; gửi lần 2 trong cùng ngày THEN bỏ qua an toàn.
- [ ] GIVEN hai sự kiện cộng XP gửi đến đồng thời trên 1 user THEN thực thi tuần tự qua khóa dòng, 0% Lost Update.

---

## Architecture Decision Records (ADRs)
- [ADR-0001: Kiến Trúc Điểm Kinh Nghiệm (XP) & Cấp Độ (Level 1–100)](./adr/ADR-0001-xp-and-level-progression-architecture.md)
- [ADR-0002: Sử Dụng Khóa Dòng Bi Quan SELECT FOR UPDATE Cho Cập Nhật Điểm XP](./adr/ADR-0002-use-pessimistic-row-locking-for-xp-updates.md)
- [ADR-0003: Tích Hợp Điểm Thưởng Kỷ Luật Dinh Dưỡng (+30 XP/ngày) Vào Hệ Thống XP](./adr/ADR-0003-incorporate-nutrition-adherence-bonus-into-xp.md)
