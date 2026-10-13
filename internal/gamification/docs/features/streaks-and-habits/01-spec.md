# Đặc Tả Nghiệp Vụ: Chuỗi Ngày Kỷ Luật (Streaks Engine)

## 1. Summary & Giá Trị Nghiệp Vụ

Tính năng **Chuỗi Ngày Kỷ Luật (Streaks Engine)** là động cơ duy trì thói quen cốt lõi của Gym Companion:
* **Tôn vinh sự kỷ luật thực chất**: Đếm số ngày liên tục gymer có nỗ lực thực chất (tập luyện hoặc dinh dưỡng).
* **Thói quen kép (Workout & Nutrition)**: Ngày tập nâng tạ giữ chuỗi, ngày nghỉ cơ bắp thì log ăn uống đạt chuẩn Calo/Protein để giữ chuỗi, giúp gymer duy trì kỷ luật thể hình mà không lo tập quá tải (Overtraining).
* **Đánh giá lười không cron (Lazy Evaluation $O(1)$)**: Hệ thống tự động xác định tính liên tục của chuỗi ngay khi có sự kiện hoạt động mới, không cần chạy cron job quét database nửa đêm.
* **Vinh danh mốc bền bỉ**: Tự động phát sự kiện `StreakMilestoneReached` khi chạm các mốc chuỗi lớn (7, 14, 30, 50, 100, 365 ngày).

---

## 2. Problem & Desired Outcome

### Vấn Đề (Problem)
1. Thể hình đòi hỏi tính nhất quán kéo dài hàng tháng, hàng năm; gymer rất dễ bỏ cuộc nếu thiếu cơ chế theo dõi thói quen trực quan hàng ngày.
2. Tuy nhiên, cơ thể người tập thể hình cần 48–72h nghỉ ngơi để cơ bắp phục hồi và phát triển. Nếu bắt buộc ngày nào cũng phải nâng tạ mới giữ được chuỗi thì sẽ dẫn đến tập luyện quá tải (Overtraining) hoặc chấn thương.
3. Nếu dùng background cron job quét lúc nửa đêm để reset chuỗi sẽ gây nghẽn I/O cơ sở dữ liệu (The 3 AM Test) và xung đột múi giờ giữa các gymer toàn cầu.

### Kết Quả Kỳ Vọng (Desired Outcome)
* **Kỷ luật thực chất**: Chỉ ghi nhận chuỗi khi gymer có hành động đổ mồ hôi hoặc kỷ luật ăn uống. Tuyệt đối không tính khi chỉ mở app lướt xem.
* **Thói quen toàn diện (Workout & Nutrition)**: Ngày tập thì nâng tạ, ngày nghỉ thì log ăn uống đúng chuẩn Calo/Protein để giữ chuỗi liên tục.
* **Tự động phát hiện đứt chuỗi theo sự kiện**: Khi người dùng lỡ một ngày (không tập và không log ăn), chuỗi tự động kết thúc và bắt đầu lại từ 1 ở lần hoạt động kế tiếp mà vẫn bảo toàn kỷ lục dài nhất trọn đời (`longest_streak`).

---

## 3. Actors & Entry Points

* **Gym User**:
  * Tra cứu trạng thái chuỗi: số ngày hiện tại (`current_streak`), kỷ lục dài nhất (`longest_streak`), ngày hoạt động gần nhất (`last_active_date`), và trạng thái hoàn thành hôm nay (`is_active_today`).
* **Kafka Consumers (Sự kiện kích hoạt chuỗi)**:
  * Sự kiện buổi tập: `WorkoutSessionCompleted` (`workout_execution.events`).
  * Sự kiện dinh dưỡng: `DailyNutritionGoalAchieved` (`nutrition.events`).
* **Transactional Outbox Worker**: Xuất bản các sự kiện: `StreakAdvanced`, `StreakReset`, `StreakMilestoneReached` sang Kafka topic `gamification.events`.

---

## 4. Phạm Vi Hệ Thống (Scope)

### Trong Phạm Vi (In Scope)
* Theo dõi chuỗi ngày liên tiếp theo múi giờ địa phương của người dùng (`user_timezone`).
* Ghi nhận hoàn thành ngày từ 2 nguồn: Buổi tập hợp lệ HOẶC Dinh dưỡng đạt chuẩn Calo/Protein.
* Đánh giá lười (Lazy Evaluation $O(1)$): Tự động phát hiện ngày kế tiếp để tăng chuỗi (`current_streak += 1`) hoặc đứt chuỗi để reset về 1 khi có khoảng trống ngày nghỉ.
* Ghi nhận cột mốc chuỗi: 7, 14, 30, 50, 100, 365 ngày liên tục qua domain event `StreakMilestoneReached`.
* Lưu vết lịch sử hoạt động chuỗi (`streak_history`) để hiển thị bản đồ nhiệt (Activity Heatmap) 365 ngày.

### Ngoài Phạm Vi (Non-Goals)
* Không xây dựng Cửa Hàng (Shop), không quản lý vật phẩm khiên đóng băng trong giai đoạn này.
* Không dùng background cron job quét nửa đêm (tránh nghẽn DB lúc 3 AM).
* Mở app lướt xem không được tính là duy trì chuỗi.

---

## 5. Yêu Cầu Chức Năng (Functional Requirements)

* **FR-ST-01 [Ghi Nhận Chuỗi Qua Buổi Tập]**: Khi nhận sự kiện `WorkoutSessionCompleted` hợp lệ ($\ge 15$ phút, $\ge 3$ set, không anomalous), hệ thống kiểm tra và cập nhật chuỗi cho ngày địa phương tương ứng.
* **FR-ST-02 [Ghi Nhận Chuỗi Qua Dinh Dưỡng]**: Khi nhận sự kiện `DailyNutritionGoalAchieved` đạt chuẩn Calo (±10%) và Protein (≥90%), hệ thống kiểm tra và cập nhật chuỗi nếu ngày hôm đó chưa có hoạt động tập luyện.
* **FR-ST-03 [Xử Lý Đứt Chuỗi Tự Động (Lazy Reset)]**: Khi phát hiện người dùng lỡ ít nhất 1 ngày không hoạt động ($\Delta = \text{DateDiff}($ `local_date`, `last_active_date` $) \ge 2$), chuỗi hiện tại bị đứt. Khi người dùng phát sinh hoạt động tiếp theo, `current_streak` bắt đầu lại từ `1` và phát sự kiện `StreakReset`.
* **FR-ST-04 [Vinh Danh Cột Mốc Bền Bỉ]**: Khi `current_streak` chạm các mốc 7, 14, 30, 50, 100, 365 ngày, tự động phát sự kiện `StreakMilestoneReached` sang Outbox.

---

## 6. Quy Tắc Nghiệp Vụ & Bất Biến (Business Rules & Invariants)

* **BR-ST-01 [Nỗ Lực Thực Chất - Không Tính Mở App]**:
  * Chỉ có buổi tập hợp lệ hoặc nhật ký dinh dưỡng đạt chuẩn mới được đánh dấu là ngày hoạt động (`active_day`).
  * Hành vi mở app hoặc gọi API tra cứu `GetStreak` là tác vụ đọc thuần túy, tuyệt đối KHÔNG làm tăng số ngày chuỗi.
* **BR-ST-02 [Bất Biến Múi Giờ Địa Phương (Timezone Invariant)]**:
  * Ngày hoạt động được tính theo `user_local_date = Format(event.time, user_timezone)`.
  * Tập luyện lúc 23:30 tại múi giờ `Asia/Ho_Chi_Minh` (UTC+7) phải tính cho ngày hôm đó, không bị lệch sang ngày hôm sau của giờ UTC.
* **BR-ST-03 [Bảo Toàn Kỷ Lục Dài Nhất]**:
  $$\text{LongestStreak} = \max(\text{LongestStreak}, \text{CurrentStreak})$$
  * Khi chuỗi bị reset về 1, `longest_streak` giữ nguyên giá trị cao nhất trọn đời của người dùng.
* **BR-ST-04 [Cột Mốc Chuỗi Kỷ Luật (Streak Milestones)]**:
  * Các mốc ghi nhận: 7, 14, 30, 50, 100, 365 ngày.
  * Khi chạm mốc, phát sự kiện `StreakMilestoneReached` (chứa `user_id`, `milestone_days`) để các hệ thống thông báo (`notification`) hoặc mạng xã hội (`social`) xử lý vinh danh.
  * Mỗi mốc chỉ được kích hoạt 1 lần cho mỗi chuỗi liên tục (không phát lặp lại trong cùng 1 chuỗi).

---

## 7. Các Ca Sử Dụng Chính (Use Cases)

### UC-ST-01: Duy Trì Chuỗi Hàng Ngày Qua Buổi Tập
* **Actor**: Kafka Consumer (`WorkoutSessionCompleted`).
* **Luồng chính**:
  1. Consumer đọc payload buổi tập kèm `user_id`, `completed_at`, `duration`, `sets`.
  2. Xác định ngày địa phương `local_date` theo `user_timezone`.
  3. Nạp `user_streak` có khóa dòng `FOR UPDATE`.
  4. Nếu `local_date == last_active_date`: Đã tính hôm nay $\rightarrow$ Bỏ qua (Idempotent 200 OK).
  5. Nếu `local_date == last_active_date + 1`: Ngày kế tiếp liên tục $\rightarrow$ `current_streak += 1`, `last_active_date = local_date`.
  6. Kiểm tra thưởng cột mốc nếu đạt mốc $\rightarrow$ Bắn `StreakMilestoneReached` vào Outbox.
  7. Commit transaction.

### UC-ST-02: Duy Trì Chuỗi Vào Ngày Nghỉ Cơ Qua Dinh Dưỡng
* **Actor**: Kafka Consumer (`DailyNutritionGoalAchieved`).
* **Luồng chính**:
  1. Consumer nhận sự kiện ăn uống đạt chuẩn Calo/Protein.
  2. Xác định `local_date`.
  3. Nạp `user_streak`. Nếu hôm nay chưa có buổi tập nào (`last_active_date < local_date`):
     - Xử lý tăng chuỗi tương tự như UC-ST-01.
     - Ghi nhận `streak_history` với loại `NUTRITION`.

### UC-ST-03: Tự Động Reset Chuỗi Khi Lỡ Ngày (Lazy Reset)
* **Actor**: Hệ thống (khi nhận event hoạt động của ngày tiếp theo).
* **Điều kiện**: $\Delta = \text{DateDiff}($ `local_date`, `last_active_date` $) \ge 2$ (có ít nhất 1 ngày nghỉ không có hoạt động).
* **Luồng chính**:
  1. Chuỗi bị đứt do gián đoạn: `current_streak = 1`, `last_active_date = local_date`.
  2. Cập nhật `longest_streak = max(longest_streak, 1)`.
  3. Ghi nhận `streak_history` cho ngày hôm nay.
  4. Phát CloudEvent `StreakReset` vào Outbox.
  5. Phát CloudEvent `StreakAdvanced` (với chuỗi mới = 1).
  6. Commit transaction.

### UC-ST-04: Tra Cứu Trạng Thái Chuỗi
* **Actor**: Gym User (qua ConnectRPC `GetStreak`).
* **Luồng chính**:
  1. Đọc bản ghi `user_streak` theo `user_id`.
  2. Xác định `is_active_today = (last_active_date == current_local_date)`.
  3. Trả về `{ current_streak, longest_streak, last_active_date, is_active_today }`.
  4. Không thay đổi bất kỳ trạng thái nào trong DB.

---

## 8. Tiêu Chí Nghiệm Thu (Acceptance Criteria)

* **AC-ST-01 [Tăng Chuỗi Tuần Tự]**: Tập luyện 3 ngày liên tiếp (`D`, `D+1`, `D+2`) $\rightarrow$ Chuỗi tăng chính xác $1 \rightarrow 2 \rightarrow 3$.
* **AC-ST-02 [Nhiều Hoạt Động Cùng Ngày]**: Trong cùng ngày `D`, tập 2 buổi và log 3 bữa ăn $\rightarrow$ Chuỗi chỉ tăng đúng 1 lần cho ngày `D`.
* **AC-ST-03 [Mở App Không Tăng Chuỗi]**: User mở app 10 lần trong ngày nhưng không tập và không log ăn $\rightarrow$ `current_streak` giữ nguyên, không được tăng.
* **AC-ST-04 [Đứt Chuỗi Khi Nghỉ Ngày]**: Tập ngày `D`, nghỉ ngày `D+1`, quay lại tập ngày `D+2` ($\Delta = 2$) $\rightarrow$ Chuỗi bị reset về 1, `longest_streak` vẫn bảo toàn kỷ lục cũ.
* **AC-ST-05 [Nghỉ Nhiều Ngày Liên Tiếp]**: Tập ngày `D`, nghỉ cả tuần, quay lại tập ngày `D+8` ($\Delta = 8$) $\rightarrow$ Chuỗi bắt đầu lại từ 1, phát sự kiện `StreakReset`.
