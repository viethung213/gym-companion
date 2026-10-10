# Feature: ELO Rating & Rank Tiers

## Summary
Tính năng **ELO Rating & Rank Tiers** cung cấp thước đo năng lực thể chất chuẩn hóa duy nhất (`elo_rating` từ 1,000 đến 3,000) cho người dùng trong nền tảng Gym Companion. Hệ thống tự động tính toán biến động điểm ELO sau mỗi buổi tập hợp lệ, thưởng kỷ luật dinh dưỡng hàng ngày, và phân loại người dùng vào 5 bậc hạng (Bronze $\rightarrow$ Diamond) theo cơ chế ánh xạ tức thì không trạng thái.

## Problem and desired outcome
- **Problem**: Gymer thường mất động lực vì thiếu thước đo định lượng khách quan sự tiến bộ thể chất trong ngắn hạn. Các cơ chế xếp hạng mùa giải hoặc bảng đấu tuần truyền thống đòi hỏi vận hành cron job phức tạp và tạo cảm giác xa vời cho người mới bắt đầu.
- **Desired outcome**:
  - Người dùng nhận phản hồi điểm số minh bạch ngay sau mỗi buổi tập hoàn thành.
  - Thăng/giáng bậc hạng diễn ra theo thời gian thực khi chạm mốc điểm.
  - Điểm số phân hóa chính xác trình độ thể lực, chống lạm phát điểm (anti-inflation).

## Actors and entry points
- **Gym User**: Xem điểm ELO, khoảng cách thăng hạng và lịch sử biến động trên ứng dụng.
- **Kafka Consumer**:
  - Tiếp nhận sự kiện buổi tập: `WorkoutSessionCompleted`, `NewPersonalRecordAchieved` (`workout_execution.events`).
  - Tiếp nhận sự kiện dinh dưỡng: `MealLogged` (`nutrition.events`).
- **Background Worker**: Quét định kỳ trừ điểm suy giảm bất hoạt (Inactivity Decay) và gửi Outbox events.

## Scope

### In scope
- Khởi tạo hồ sơ ELO cơ sở (1,000 ELO, Bronze) cho người dùng mới.
- Tính toán biến động $\Delta ELO$ theo hiệu suất tập: Khối lượng nâng (Volume), Kỹ thuật động tác (Form Score), và Kỷ lục cá nhân (PR).
- Thưởng kỷ luật dinh dưỡng hàng ngày ($+3 \rightarrow +5$ ELO/ngày) khi đạt mục tiêu Calo/Protein (ADR-0001).
- Ánh xạ bậc hạng tức thì không trạng thái (ADR-0003).
- Cơ chế suy giảm điểm bất hoạt khi nghỉ tập $> 14$ ngày.
- Phát CloudEvents: `EloScoreUpdated`, `RankTierPromoted`, `RankTierDemoted`.

### Out of scope & Deferred
- **Computer Vision**: Đếm rep, chấm điểm form (thuộc module `workout_execution`).
- **Solo PvP 1v1 [DEFERRED]**: Cơ chế tính điểm đối kháng tạm hoãn cho đến khi phát triển module Competition (ADR-0002).

## Current behavior
Chưa có module tính toán điểm ELO. Dữ liệu buổi tập hoàn thành chỉ lưu tại `workout_execution`. Người dùng chưa có thước đo tổng hợp năng lực thể chất.

## User scenarios / use cases

### UC-ELO-01: Cập nhật ELO sau buổi tập hoàn thành
- **Actor**: Kafka Consumer (`workout_execution.events`).
- **Preconditions**: Buổi tập hoàn thành hợp lệ, có `session_id`, `user_id`.
- **Trigger**: Nhận CloudEvent `WorkoutSessionCompleted`.
- **Main Flow**:
  1. Kiểm tra tính lũy đẳng qua `session_id`. Nếu đã xử lý, bỏ qua.
  2. Nạp hồ sơ ELO của người dùng (khởi tạo mặc định 1,000 ELO nếu chưa tồn tại).
  3. Tính toán $\Delta ELO$ dựa trên Volume Ratio, Form Score, cờ PR và hệ số $K$ của bậc hạng hiện tại.
  4. Cập nhật ELO mới (kẹp trần sàn $[1000, 3000]$) và ghi nhận vào lịch sử `elo_history`.
  5. Đánh giá bậc hạng mới: Nếu đổi bậc, phát sinh sự kiện `RankTierPromoted` hoặc `RankTierDemoted`.
  6. Lưu sự kiện vào Transactional Outbox trong cùng một transaction nghiệp vụ.
- **Postconditions**: ELO được cập nhật tuần tự, sự kiện sẵn sàng đẩy ra Kafka topic `gamification.events`.

### UC-ELO-02: Thưởng ELO từ kỷ luật dinh dưỡng hàng ngày
- **Actor**: Kafka Consumer (`nutrition.events`).
- **Preconditions**: Người dùng ghi nhật ký bữa ăn trong ngày qua module `nutrition`.
- **Trigger**: Nhận CloudEvent `MealLogged`.
- **Main Flow**:
  1. Kiểm tra ngày nhận thưởng: Nếu `last_nutrition_reward_date == user_local_date`, bỏ qua.
  2. Đánh giá chỉ số ngày: Calo đạt $\pm 10\%$ và Protein đạt $\ge 90\%$ mục tiêu do AI Coach đề ra.
  3. Nếu đạt chuẩn: Cộng $+3$ ELO (hoặc $+5$ ELO nếu đạt chuỗi 3 ngày), cập nhật `last_nutrition_reward_date = user_local_date`, ghi log `elo_history` lý do `NUTRITION_ADHERENCE`, phát sự kiện `EloScoreUpdated`.
  4. Nếu không đạt chuẩn hoặc quên ghi log: Không thực hiện trừ điểm ELO (ADR-0001).
- **Postconditions**: Người dùng nhận thưởng kỷ luật dinh dưỡng tối đa 1 lần/ngày.

### UC-ELO-03: Suy giảm điểm ELO do bất hoạt (Inactivity Decay)
- **Actor**: Background Worker.
- **Preconditions**: Người dùng có `current_elo > 1000` (Bậc Bronze sàn 1,000 không bị decay).
- **Trigger**: Quét định kỳ người dùng có `last_workout_at` cách thời điểm hiện tại $> 14$ ngày.
- **Main Flow**:
  1. Lọc danh sách người dùng bất hoạt $> 14$ ngày và chưa bị decay trong 7 ngày gần nhất.
  2. Trừ $15$ ELO (không trừ dưới sàn 1,000), cập nhật `last_decay_at = NOW()`.
  3. Ghi log `elo_history` lý do `INACTIVITY_DECAY`.
  4. Nếu điểm mới rớt xuống bậc dưới, hạ bậc tức thì và phát sự kiện `RankTierDemoted`.
- **Postconditions**: Điểm ELO của người dùng bất hoạt suy giảm định kỳ mỗi 7 ngày.

### UC-ELO-04: Truy vấn điểm ELO cá nhân và lịch sử
- **Actor**: Gym User (Mobile/Web Client).
- **Preconditions**: Người dùng đã đăng nhập, gửi Bearer Token hợp lệ.
- **Trigger**: Người dùng mở màn hình Gamification hoặc Profile.
- **Main Flow**:
  1. Client gọi API `GetMyElo`. Hệ thống trả về điểm ELO hiện tại, bậc hạng, và khoảng cách điểm tới bậc tiếp theo (`points_to_next_tier`).
  2. Client gọi API `GetEloHistory` để nhận danh sách biến động ELO phân trang (Pagination).
- **Postconditions**: Người dùng xem được tiến trình thăng hạng và đồ thị biến động điểm.

## Functional requirements
- **FR-ELO-01**: Khởi tạo hồ sơ ELO mức cơ sở 1,000 ELO (Bronze) khi người dùng mới phát sinh buổi tập hoặc truy vấn lần đầu.
- **FR-ELO-02**: Tính toán biến động $\Delta ELO$ sau mỗi buổi tập hoàn thành hợp lệ và cộng/trừ trực tiếp vào điểm người dùng.
- **FR-ELO-03**: Phân định người dùng vào đúng 1 trong 5 bậc hạng (Bronze, Silver, Gold, Platinum, Diamond) theo mốc ELO hiện tại.
- **FR-ELO-04**: Tự động hạ bậc (Demotion) ngay lập tức khi ELO tụt xuống dưới ngưỡng sàn của bậc hiện tại, không dùng khiên đệm (ADR-0003).
- **FR-ELO-05**: Áp dụng suy giảm 15 ELO mỗi 7 ngày nếu người dùng không tập luyện quá 14 ngày liên tiếp (chỉ áp dụng khi ELO > 1,000).
- **FR-ELO-06**: Lưu trữ lịch sử toàn bộ các lần biến động điểm ELO (append-only) để phục vụ truy vấn đồ thị tiến trình.
- **FR-ELO-07**: Thưởng $+3 \rightarrow +5$ ELO/ngày khi đạt mục tiêu dinh dưỡng; tuyệt đối không phạt khi ăn lệch mục tiêu (ADR-0001).

## Business rules and invariants
- **BR-ELO-01 (Rank Thresholds)**: 5 bậc hạng cố định: Bronze ($1,000 - 1,199$), Silver ($1,200 - 1,499$), Gold ($1,500 - 1,799$), Platinum ($1,800 - 2,199$), Diamond ($2,200 - 3,000$).
- **BR-ELO-02 (K-Factor Scaling)**: Hệ số $K$ suy giảm theo bậc: Bronze/Silver ($K=32$), Gold ($K=24$), Platinum ($K=16$), Diamond ($K=10$).
- **BR-ELO-03 (Workout Formula)**: Biến động tính từ Volume Ratio, Form Score Ratio (mặc định 75 cho bài tập không AI) và PR Bonus ($+1.0$). Biên độ dao động trong 1 buổi tập bị kẹp cứng: $-25 \le \Delta ELO \le +40$.
- **BR-ELO-04 (Direct Mapping)**: Bậc hạng phản ánh trực tiếp theo điểm số hiện tại, không có trạng thái trung gian, giáng bậc tức thì khi tụt mốc (ADR-0003).
- **BR-ELO-05 (Hard Boundaries)**: Điểm ELO luôn nằm trong đoạn $[1000, 3000]$ (ADR-0005). Không bao giờ tụt dưới 1,000 và không tích lũy vượt quá 3,000.
- **BR-ELO-06 (Concurrency & Idempotency)**: Giao dịch cập nhật điểm phải tuần tự hóa để ngăn ngừa Lost Update (ADR-0004). Mỗi `session_id` chỉ tính điểm duy nhất 1 lần.
- **BR-ELO-07 (Nutrition Adherence)**: Thưởng dương tối đa $+5$ ELO/ngày, không phạt khi cheat meal/quên log (ADR-0001).

## Validation and permissions
- Chỉ có sự kiện CloudEvents hợp lệ từ hệ thống nội bộ qua Kafka mới có quyền thay đổi điểm ELO.
- API công khai phục vụ Client chỉ cung cấp quyền đọc (Read-only).

## Error and boundary scenarios
- **Duplicate Event**: Sự kiện trùng lặp `session_id` bị chặn bởi Idempotent Inbox Guard, bỏ qua an toàn.
- **Anomalous Session**: Buổi tập thời lượng $> 240$ phút hoặc hiệp $< 1$ bị đánh dấu bất thường, không cộng điểm ELO.
- **Boundary Exact Points**: Chạm đúng 1,200 là Silver; chạm đúng 1,500 là Gold; tụt về 1,199 lập tức về Bronze.
- **Hard Cap Reached**: User 2,990 ELO nhận $+40$ ELO sẽ dừng chính xác ở mức 3,000 ELO.

## Acceptance criteria
- [ ] GIVEN user 1,180 ELO WHEN hoàn thành buổi tập đạt $+30$ ELO THEN điểm cập nhật 1,210 ELO và bậc hạng chuyển từ Bronze sang Silver.
- [ ] GIVEN user 1,210 ELO (Silver) WHEN buổi tập tiếp theo bị trừ 25 ELO xuống 1,185 ELO THEN bậc hạng tự động giáng về Bronze và phát sự kiện `RankTierDemoted`.
- [ ] GIVEN CloudEvent `WorkoutSessionCompleted` gửi lại lần 2 THEN hệ thống bỏ qua và ELO không bị cộng lặp.
- [ ] GIVEN user Bậc Vàng không tập 21 ngày THEN bị trừ đúng 15 ELO decay.
- [ ] GIVEN user 2,990 ELO hoàn thành buổi tập $+30$ ELO THEN điểm kẹp đúng trần cứng 3,000 ELO.
- [ ] GIVEN hai sự kiện cộng/trừ ELO xảy ra đồng thời THEN giao dịch chạy tuần tự, 0% Lost Update.

## Architecture Decision Records (ADRs)
- [ADR-0001: Thưởng Kỷ Luật Dinh Dưỡng Vào ELO](./adr/ADR-0001-incorporate-nutrition-adherence-bonus-into-elo.md)
- [ADR-0002: Hoãn Cơ Chế Tính Điểm Solo PvP](./adr/ADR-0002-defer-solo-pvp-elo-scoring-mechanism.md)
- [ADR-0003: Ánh Xạ Bậc Hạng Trực Tiếp Không Trạng Thái](./adr/ADR-0003-stateless-direct-rank-mapping-without-demotion-shield.md)
- [ADR-0004: Sử Dụng Khóa Bi Quan SELECT FOR UPDATE](./adr/ADR-0004-use-pessimistic-row-locking-for-elo-updates.md)
- [ADR-0005: Thiết Lập Trần Cứng 3,000 ELO và Sàn Cơ Sở 1,000 ELO](./adr/ADR-0005-establish-elo-range-and-hard-cap.md)

## Resolved decisions & confirmations
1. **Khởi tạo ELO**: Áp dụng Lazy Onboarding mức cố định 1,000 ELO (Bronze) cho toàn bộ người dùng mới; không yêu cầu bài test đầu vào phức tạp.
2. **Cơ chế Decay**: Thực thi bằng Scheduled Worker chạy ngầm hàng đêm (02:00 AM) để chủ động kiểm soát tải hệ thống.
3. **Form Score bài tập phi AI**: Gán giá trị mặc định 75/100 khi buổi tập không sử dụng AI Camera.

## Explicit assumptions
- Dữ liệu buổi tập hoàn thành từ `workout_execution` là nguồn sự thật đáng tin cậy.
- Điểm ELO không bị can thiệp thủ công trừ trường hợp xử lý vi phạm gian lận được phê duyệt.

