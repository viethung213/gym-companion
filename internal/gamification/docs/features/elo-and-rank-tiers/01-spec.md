# Feature: ELO Rating & Rank Tiers

## Summary
Feature **ELO Rating & Rank Tiers** cung cấp thước đo năng lực thể chất chuẩn hóa duy nhất (`elo_rating`) cho người dùng trong nền tảng Gym Companion. Hệ thống tự động tính toán biến động điểm ELO sau mỗi buổi tập hợp lệ và phân loại người dùng vào 5 bậc hạng (Bronze, Silver, Gold, Platinum, Diamond) ngay lập tức mà không cần tác vụ chốt bảng tuần.

## Problem and desired outcome
- **Problem**: Người mới tập gym thường mất phương hướng vì không định lượng được sự tiến bộ thể chất của bản thân trong ngắn hạn. Các cơ chế xếp hạng theo mùa giải hoặc bảng đấu tuần truyền thống đòi hỏi vận hành cron job phức tạp và tạo cảm giác xa vời cho người mới bắt đầu.
- **Desired outcome**:
  - Người dùng nhận được phản hồi điểm số minh bạch sau mỗi buổi tập.
  - Thăng bậc hạng diễn ra ngay trong thời gian thực khi đạt mốc điểm.
  - Điểm số phân hóa chính xác trình độ thể lực, không xảy ra tình trạng cày cuốc số lượng để thổi phồng thứ hạng (anti-inflation).

## Actors and entry points
- **Gym User**: Xem điểm ELO, lịch sử biến động điểm và bậc hạng hiện tại trên ứng dụng.
- **Kafka Event Consumer**: 
  - Tiếp nhận sự kiện hoàn thành buổi tập: `contracts.core.workout_execution.v1.event.WorkoutSessionCompleted` và `contracts.core.workout_execution.v1.event.NewPersonalRecordAchieved`.
  - Tiếp nhận sự kiện dinh dưỡng: `contracts.core.nutrition.v1.event.MealLogged`.

## Scope

### In scope
- Khởi tạo hồ sơ ELO mặc định cho người dùng mới.
- Tính toán biến động $\Delta ELO$ dựa trên hiệu suất buổi tập: khối lượng nâng (Volume), độ chuẩn xác động tác (Form Score), và kỷ lục cá nhân (PR).
- Tính toán điểm thưởng kỷ luật dinh dưỡng hàng ngày (Daily Nutrition Compliance Bonus) từ module `nutrition` (tham chiếu [ADR-0001](./adr/ADR-0001-incorporate-nutrition-adherence-bonus-into-elo.md)).
- Tự động thăng bậc (Promotion) hoặc hạ bậc (Demotion) tức thì theo các mốc điểm chuẩn.
- Cơ chế suy giảm điểm khi nghỉ tập dài ngày (Inactivity ELO Decay).
- Phát sự kiện Outbound: `RankTierPromoted`, `RankTierDemoted`, `EloScoreUpdated`.

### Out of scope & Deferred
- **Thuật toán thị giác máy tính**: Đếm rep hoặc chấm điểm kỹ thuật động tác (thuộc module `workout_execution`).
- **Đấu đối kháng Solo 1v1 (PvP) [DEFERRED]**: Do hiện tại hệ thống chưa phát triển module Solo / Competition, công thức biến động điểm ELO và quy tắc trừ điểm khi thua trận Solo **chưa chốt trong giai đoạn này** và được hoãn lại (tham chiếu [ADR-0002](./adr/ADR-0002-defer-solo-pvp-elo-scoring-mechanism.md)). Khi module Solo được xây dựng, cơ chế tính điểm PvP sẽ được tích hợp bổ sung.

## Current behavior
Hiện tại hệ thống chưa có module tính toán điểm ELO. Dữ liệu buổi tập hoàn thành chỉ được lưu trữ tại `workout_execution` và đồng bộ sang `coaching`. Người dùng chưa có thước đo tổng hợp năng lực thể chất.

## User scenarios / use cases

### UC-ELO-01: Cập nhật ELO sau buổi tập hoàn thành
- **Actor**: Hệ thống (Kafka Consumer).
- **Preconditions**: Người dùng đã hoàn thành một buổi tập hợp lệ. Sự kiện `WorkoutSessionCompleted` được phát ra trên Kafka topic `workout_execution.events`.
- **Trigger**: Consumer nhận được CloudEvent.
- **Main Flow**:
  1. Consumer xác thực tính hợp lệ của sự kiện và kiểm tra Idempotency Key (`session_id`).
  2. Hệ thống nạp hồ sơ ELO hiện tại của người dùng kèm khóa dòng `SELECT ... FOR UPDATE`.
  3. Hệ thống tính toán $\Delta ELO$ dựa trên các tham số: tỷ lệ hoàn thành khối lượng ($V_{\text{actual}} / V_{\text{target}}$), điểm form trung bình (`average_form_score`), và hệ số $K$ của bậc hạng hiện tại.
  4. Hệ thống cập nhật điểm ELO mới vào cơ sở dữ liệu và ghi nhật ký biến động điểm (`elo_history`).
  5. Nếu điểm mới vượt qua ngưỡng bậc hạng tiếp theo, hệ thống kích hoạt thăng bậc và phát sự kiện `RankTierPromoted`.
  6. Nếu điểm mới tụt xuống dưới ngưỡng sàn của bậc hiện tại, hệ thống hạ bậc ngay lập tức và phát sự kiện `RankTierDemoted`.
  7. Lưu sự kiện vào Outbox trong cùng transaction.
- **Postconditions**: Điểm ELO của người dùng được cập nhật. Sự kiện được phát tới Kafka topic `gamification.events`.

### UC-ELO-02: Thưởng ELO từ Kỷ Luật Dinh Dưỡng Hàng Ngày
- **Actor**: Hệ thống (Kafka Consumer).
- **Preconditions**: Người dùng ghi nhật ký các bữa ăn trong ngày qua module `nutrition`.
- **Trigger**: Nhận sự kiện `MealLogged` hoàn tất ngày hoặc đạt đủ mốc mục tiêu dinh dưỡng ngày.
- **Main Flow**:
  1. Consumer nhận sự kiện dinh dưỡng và tổng hợp Calo cùng Protein đã nạp trong ngày của người dùng.
  2. Hệ thống đối chiếu với mục tiêu dinh dưỡng do AI Coach đề ra: độ lệch Calo nằm trong ngưỡng an toàn $\pm 10\%$ và lượng Protein đạt $\ge 90\%$ mục tiêu.
  3. Nếu đạt chuẩn và chưa nhận thưởng dinh dưỡng cho ngày này:
     - Khóa dòng `user_elo` và cộng điểm thưởng kỷ luật dinh dưỡng: $\Delta ELO_{\text{nutrition}} = +3$ ELO (hoặc $+5$ ELO nếu là chuỗi 3 ngày ăn chuẩn).
     - Ghi nhận vào bảng lịch sử `elo_history` với lý do `NUTRITION_ADHERENCE`.
     - Cập nhật `last_nutrition_reward_date = user_local_date` (Idempotent per `user_id + local_date`).
     - Phát sự kiện Outbox `EloScoreUpdated`.
  4. Nếu không đạt chuẩn hoặc chưa đủ dữ liệu: Không thực hiện trừ điểm ELO (ADR-0001).
- **Postconditions**: Người dùng được cộng điểm thưởng dinh dưỡng mà không có rủi ro bị trừ điểm khi ăn sai.

### UC-ELO-03: Suy giảm điểm ELO do bất hoạt (Inactivity ELO Decay)
- **Actor**: Hệ thống (Background Worker / Cron).
- **Preconditions**: Người dùng có `current_elo > 1000` (không áp dụng cho Bậc Đồng).
- **Trigger**: Quét định kỳ người dùng có `last_workout_at` cách thời điểm hiện tại $> 14$ ngày.
- **Main Flow**:
  1. Worker xác định danh sách người dùng bất hoạt $> 14$ ngày và chưa bị trừ decay trong vòng 7 ngày gần nhất.
  2. Mở Transaction và khóa dòng `SELECT ... FOR UPDATE` cho từng người dùng.
  3. Trừ 15 điểm ELO: $\text{new\_elo} = \max(\text{current\_elo} - 15, 1000)$.
  4. Cập nhật `last_decay_at = NOW()`.
  5. Ghi nhật ký vào `elo_history` với lý do `INACTIVITY_DECAY`.
  6. Nếu điểm mới rớt xuống bậc dưới, hạ bậc tức thì và phát sự kiện `RankTierDemoted`.
  7. Commit Transaction.
- **Postconditions**: Người dùng bị trừ điểm phạt bất hoạt và tự động hạ bậc nếu điểm tụt dưới ngưỡng sàn.

### UC-ELO-04: Truy vấn điểm ELO cá nhân và lịch sử biến động
- **Actor**: Gym User (Mobile/Web Client).
- **Preconditions**: Người dùng đã đăng nhập và gửi Bearer Token hợp lệ.
- **Trigger**: Người dùng mở màn hình Profile hoặc màn hình Gamification trên ứng dụng.
- **Main Flow**:
  1. Client gọi gRPC / REST Gateway endpoint `GetMyElo`.
  2. Hệ thống truy vấn bản ghi `gamification.user_elo` theo `user_id`. Nếu chưa có, lazy-create mặc định 1,000 ELO (Bronze).
  3. Hệ thống tính toán khoảng cách điểm tới bậc kế tiếp: `points_to_next_tier = next_threshold - current_elo`.
  4. Client gọi `GetEloHistory` để nhận danh sách biến động ELO phân trang (Pagination).
- **Postconditions**: Người dùng xem được thứ hạng, điểm ELO, khoảng cách thăng hạng và biểu đồ tiến trình.

## Functional requirements
- **FR-ELO-01**: Hệ thống phải tự động khởi tạo hồ sơ ELO với mức điểm cơ sở là 1,000 ELO khi người dùng đăng ký hoặc lần đầu hoàn thành buổi tập.
- **FR-ELO-02**: Hệ thống phải tính toán $\Delta ELO$ sau mỗi buổi tập hoàn thành hợp lệ và cộng/trừ trực tiếp vào hồ sơ người dùng.
- **FR-ELO-03**: Hệ thống phải phân định người dùng vào đúng 1 trong 5 bậc hạng tương ứng với điểm ELO.
- **FR-ELO-04**: Hệ thống phải tự động giáng bậc hạng (Demotion) ngay lập tức khi điểm ELO của người dùng tụt xuống dưới ngưỡng sàn của bậc hiện tại.
- **FR-ELO-05**: Hệ thống phải áp dụng suy giảm điểm ELO định kỳ nếu người dùng không phát sinh buổi tập nào trong vòng 14 ngày liên tiếp.
- **FR-ELO-06**: Hệ thống phải lưu trữ lịch sử toàn bộ các lần biến động ELO để phục vụ truy vấn đồ thị tiến trình.
- **FR-ELO-07**: Hệ thống phải cộng điểm thưởng kỷ luật dinh dưỡng hàng ngày (Daily Nutrition Adherence Bonus) từ module `nutrition` khi người dùng đạt mục tiêu calo/macro (tham chiếu [ADR-0001](./adr/ADR-0001-incorporate-nutrition-adherence-bonus-into-elo.md)).

## Business rules and invariants

### BR-ELO-01: Ngưỡng điểm Bậc Hạng & Trần Cứng (Rank Tier Cutoffs & Hard Cap)
Hệ thống áp dụng 5 bậc hạng cố định với sàn 1,000 và trần cứng 3,000 ELO (tham chiếu [ADR-0005](./adr/ADR-0005-establish-elo-range-and-hard-cap.md)):
- **Đồng (Bronze)**: 1,000 – 1,199 ELO (Sàn tối thiểu hệ thống).
- **Bạc (Silver)**: 1,200 – 1,499 ELO.
- **Vàng (Gold)**: 1,500 – 1,799 ELO.
- **Bạch Kim (Platinum)**: 1,800 – 2,199 ELO.
- **Kim Cương (Diamond)**: 2,200 – 3,000 ELO (Trần tối đa tuyệt đối).

### BR-ELO-02: Hệ số biến động $K$ suy giảm theo cấp bậc
Hệ số $K$ quyết định biên độ dao động điểm tối đa sau mỗi buổi tập nhằm tránh lạm phát điểm ở các bậc cao:
- Bậc Đồng & Bạc: $K = 32$.
- Bậc Vàng: $K = 24$.
- Bậc Bạch Kim: $K = 16$.
- Bậc Kim Cương: $K = 10$.

### BR-ELO-03: Công thức tính biến động điểm $\Delta ELO$
$$\Delta ELO = \text{round}\left( K \times \left( 0.5 \cdot \min\left(\frac{V_{\text{actual}}}{V_{\text{target}}}, 1.2\right) + 0.3 \cdot \frac{\text{FormScore}}{100} + 0.2 \cdot \text{PRBonus} - 0.5 \right) \right)$$
- Trong đó:
  - Nếu bài tập phi AI không có Form Score: gán mặc định $\text{FormScore} = 75$.
  - $\text{PRBonus} = 1.0$ nếu có sự kiện `NewPersonalRecordAchieved`, ngược lại bằng $0.0$.
  - Giới hạn biên độ trong 1 buổi tập: $-25 \le \Delta ELO \le +40$.

### BR-ELO-04: Ánh xạ Bậc Hạng trực tiếp và Giáng bậc tức thì (Direct Rank Mapping)
- Bậc hạng phản ánh thuần túy và trực tiếp theo mốc điểm ELO hiện tại (`Stateless Direct Mapping`).
- Khi điểm ELO bị giảm (do buổi tập kém hiệu quả hoặc do Inactivity Decay) xuống dưới ngưỡng sàn của bậc hiện tại, hệ thống hạ bậc ngay lập tức mà không áp dụng cơ chế đệm an toàn (tham chiếu [ADR-0003](./adr/ADR-0003-stateless-direct-rank-mapping-without-demotion-shield.md)).
- Hệ thống phát sự kiện `RankTierDemoted` ngay khi xảy ra hạ bậc.

### BR-ELO-05: Suy giảm điểm do bất hoạt (Inactivity Decay)
- Nếu khoảng cách từ buổi tập cuối cùng $> 14$ ngày: Mỗi 7 ngày bất hoạt tiếp theo, trừ 15 ELO.
- Điểm ELO sau khi trừ không được thấp hơn mức sàn 1,000 ELO (Bậc Đồng không bị áp dụng Decay).

### BR-ELO-06: Bất biến dữ liệu & Kiểm soát đồng thời (Data Invariants & Concurrency)
- Điểm ELO luôn bị kẹp chặt trong đoạn $[1000, 3000]$: $\text{new\_elo} = \min(\max(\text{old\_elo} + \Delta ELO, 1000), 3000)$.
- Mọi thao tác cập nhật điểm bắt buộc giữ khóa dòng bi quan `SELECT ... FOR UPDATE` trên `user_id` để loại bỏ hoàn toàn race condition khi có sự kiện cộng/trừ xảy ra cùng mili-giây (tham chiếu [ADR-0004](./adr/ADR-0004-use-pessimistic-row-locking-for-elo-updates.md)).
- Mỗi `session_id` chỉ được tính điểm ELO duy nhất 1 lần.

### BR-ELO-07: Quy tắc thưởng kỷ luật dinh dưỡng (Nutrition Adherence Bonus)
- **Chỉ thưởng dương (Positive Reinforcement Only)**: 
  - Đạt mục tiêu Calo ($\pm 10\%$) và Protein ($\ge 90\%$) trong ngày: $+3$ ELO.
  - Đạt chuỗi 3 ngày liên tiếp ăn chuẩn mục tiêu: $+5$ ELO/ngày.
- **Không áp dụng hình phạt**: Ăn vượt Calo (cheat meal) hoặc quên ghi nhật ký bữa ăn **tuyệt đối không bị trừ ELO** để bảo vệ tính toàn vẹn của dữ liệu dinh dưỡng (tham chiếu [ADR-0001](./adr/ADR-0001-incorporate-nutrition-adherence-bonus-into-elo.md)).
- **Trần giới hạn**: Tối đa $+5$ ELO/ngày từ dinh dưỡng.

## Validation and permissions
- Chỉ có sự kiện hợp lệ từ hệ thống nội bộ qua Kafka mới có quyền thay đổi điểm ELO qua buổi tập.
- API công khai chỉ cung cấp quyền đọc (Read-only) đối với điểm số và lịch sử.

## Error and boundary scenarios
- **Duplicate Event**: Nếu Kafka gửi lại sự kiện `WorkoutSessionCompleted` có cùng `session_id`, hệ thống phát hiện trong bảng `processed_events` và bỏ qua không xử lý tiếp.
- **Anomalous Workout Session**: Buổi tập có thời lượng $> 240$ phút hoặc số hiệp $< 1$ bị đánh dấu bất thường, không cộng điểm ELO.
- **Boundary Point**: Điểm số chạm đúng 1,200 ELO được tính là Bạc; chạm đúng 1,500 ELO được tính là Vàng.
- **Hard Cap Boundary**: Người dùng ở mức 2,990 ELO khi nhận $\Delta ELO = +30$ sẽ dừng ở mức 3,000 ELO.

## Acceptance criteria
- [ ] GIVEN người dùng ở mức 1,180 ELO WHEN hoàn thành buổi tập đạt $\Delta ELO = +30$ THEN điểm tăng lên 1,210 ELO và bậc hạng chuyển từ Đồng sang Bạc.
- [ ] GIVEN người dùng ở mức 1,210 ELO (Bạc) WHEN buổi tập tiếp theo bị trừ 25 ELO xuống 1,185 ELO THEN bậc hạng tự động hạ xuống Đồng và phát sự kiện `RankTierDemoted`.
- [ ] GIVEN sự kiện `WorkoutSessionCompleted` gửi lại 2 lần THEN điểm ELO chỉ được cộng 1 lần duy nhất.
- [ ] GIVEN người dùng Bậc Vàng không tập luyện trong 21 ngày THEN điểm ELO bị trừ đúng 15 điểm Decay.
- [ ] GIVEN người dùng ở mức 2,990 ELO hoàn thành buổi tập xuất sắc (+40 ELO) THEN điểm ELO chạm trần 3,000.
- [ ] GIVEN hai sự kiện trừ ELO và cộng ELO xảy ra cùng một mili-giây THEN giao dịch chạy tuần tự qua khóa `FOR UPDATE` và điểm số không bị mất mát (No Lost Update).

## Architecture Decision Records (ADRs)
- [ADR-0001: Tích Hợp Điểm Thưởng Kỷ Luật Dinh Dưỡng Vào Hệ Thống ELO](./adr/ADR-0001-incorporate-nutrition-adherence-bonus-into-elo.md)
- [ADR-0002: Hoãn Xác Lập Cơ Chế Tính Điểm Solo (PvP) Cho Đến Khi Xây Dựng Module Competition](./adr/ADR-0002-defer-solo-pvp-elo-scoring-mechanism.md)
- [ADR-0003: Ánh Xạ Bậc Hạng Trực Tiếp Không Trạng Thái (Loại Bỏ Demotion Shield)](./adr/ADR-0003-stateless-direct-rank-mapping-without-demotion-shield.md)
- [ADR-0004: Sử Dụng Khóa Dòng Bi Quan (SELECT FOR UPDATE) Cho Cập Nhật Điểm ELO](./adr/ADR-0004-use-pessimistic-row-locking-for-elo-updates.md)
- [ADR-0005: Thiết Lập Trần Cứng (Max ELO = 3,000) và Sàn Điểm ELO Cơ Sở](./adr/ADR-0005-establish-elo-range-and-hard-cap.md)

## Open questions and unresolved issues (Decisions needed)
1. **Khởi tạo ELO**: Điểm khởi đầu 1,000 ELO có áp dụng cho toàn bộ người dùng, hay có bài kiểm tra đầu vào (Onboarding Assessment) để phân cấp ban đầu?
2. **Chu kỳ chạy Inactivity Decay**: Nên thực thi bằng Scheduled Cron Job chạy ngầm hàng đêm hay tính toán lười (Lazy Evaluation) ngay tại thời điểm người dùng mở app hoặc quay lại tập?
3. **Bài tập Phi AI**: Trọng số Form Score mặc định cho bài tập không có camera hiện để 75/100, mức này đã phù hợp chưa?

## Explicit assumptions
- Hệ thống xem các buổi tập hoàn thành phát ra từ `workout_execution` là nguồn sự thật đáng tin cậy.
- Điểm ELO không bị can thiệp thủ công bởi quản trị viên trừ trường hợp xử lý gian lận được phê duyệt.
