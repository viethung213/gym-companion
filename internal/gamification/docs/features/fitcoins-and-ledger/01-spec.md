# Feature: Tiền Tệ Ảo FitCoins & Sổ Cái Bất Biến (FitCoins & Immutable Ledger)

## Summary
Tính năng **Tiền Tệ Ảo FitCoins & Sổ Cái Bất Biến (FitCoins & Immutable Ledger)** thiết lập và vận hành nền kinh tế ảo nội bộ cho hệ thống FITAI:
- **Đơn vị tiền tệ FitCoins**: Thước đo giá trị hữu hình vinh danh tính kỷ luật và nỗ lực thực chất của gymer (Earned-Only Currency).
- **Hệ thống Sổ Cái Bất Biến (Immutable Audit Ledger)**: Vận hành theo mô hình append-only (`gamification.coin_ledger`), ghi vết $100\%$ giao dịch thu/chi/điều chỉnh, đảm bảo không thể sửa hoặc xóa dữ liệu lịch sử, triệt tiêu rủi ro chi tiêu kép (Double Spending) và số dư âm.
- **Động Cơ Tích Lũy & Trần Chống Gian Lận (Daily Earning Caps)**: Chuẩn hóa các nguồn thưởng từ hoạt động thể chất (hoàn thành buổi tập, phá kỷ lục cá nhân PR, hoàn thành mục tiêu dinh dưỡng, chạm cột mốc chuỗi ngày, mở khóa huy hiệu) đi kèm kiểm tra Daily Cap $O(1)$ trực tiếp trên dòng `gamification.user_wallet`.
- **Cổng Thưởng Nội Bộ Đồng Bộ (In-Process Rewarder Port)**: Cung cấp Application Port `WalletRewarder` cho phép `ActivityIngestionCoordinator`, `ProcessNutritionActivityHandler`, và `ProcessPersonalRecordHandler` gọi cộng xu trực tiếp tức thì ($0\text{ ms}$ lag) trong cùng Database Transaction.
- **Cổng Chi Tiêu Mở & Quản Trị (Spend & Admin Adjust)**: Cung cấp API trừ xu an toàn (`SpendCoins`) và API điều chỉnh số dư dành cho Admin (`AdminAdjustCoins` bảo vệ bởi RBAC `middleware.RequireAdmin`).

---

## Actors and Entry Points

- **Gym User (Mobile App / Web Client)**:
  - Xem số dư ví hiện tại: RPC `GetWallet` (`GET /api/v1/gamification/wallet/me`).
  - Xem lịch sử biến động sổ cái (Cursor Pagination): RPC `ListTransactions` (`GET /api/v1/gamification/wallet/transactions`).
  - Chi tiêu xu đổi vật phẩm/đặc quyền: RPC `SpendCoins` (`POST /api/v1/gamification/wallet/spend`).
- **System Administrator**:
  - Điều chỉnh số dư thủ công (cộng bù hoặc thu hồi có ghi chú kiểm toán): RPC `AdminAdjustCoins` (`POST /api/v1/gamification/admin/wallet/adjust`).
- **In-Process Callers (Application Port `WalletRewarder`)**:
  - `ActivityIngestionCoordinator`: Gọi trực tiếp `WalletRewarder.HandleEvents(txCtx, ...)` (ủy quyền xuống `AwardCoinsBatch`) ở Bước 4 trong cùng transaction cho cả 3 luồng (`IngestWorkoutSession`, `IngestNutritionActivity`, `IngestPersonalRecord`).

---

## Ma Trận Thu Nhập & Trần Kiểm Soát (Tokenomics Summary)

| Nguồn tích lũy (`Reason`) | Mức thưởng (FitCoins) | Trần khống chế (Daily Cap / Limit) | Khóa lũy đẳng (`IdempotencyKey`) |
| :--- | :---: | :--- | :--- |
| **1. Hoàn thành buổi tập (`EARN_WORKOUT`)** | **$+10$** | **Tối đa 1 lần / ngày** (`last_workout_reward_date`) | `WORKOUT:{user_id}:{activity_date}` |
| **2. Phá kỷ lục cá nhân (`EARN_PR`)** | **$+20$** | **Tối đa 1 lần / ngày** (`last_pr_reward_date`) | `PR:{user_id}:{activity_date}` |
| **3. Kỷ luật dinh dưỡng (`EARN_NUTRITION`)** | **$+3$** | **Tối đa 1 lần / ngày** (`last_nutrition_reward_date`) | `NUTRITION:{user_id}:{activity_date}` |
| **4. Mở khóa huy hiệu (`EARN_BADGE`)** | **$+15$** | 1 lần duy nhất cho mỗi huy hiệu trọn đời | `BADGE:{user_id}:{badge_code}` |
| **5. Cột mốc Streak (`EARN_STREAK_MILESTONE`)** | **$+30 / +75 / +150 / +250 / +500 / +1500$** (mốc 7/14/30/50/100/365 ngày) | Khi chuỗi ngày chạm cột mốc | `STREAK_MILESTONE:{user_id}:{days}:{activity_date}` |

---

## Business Rules and Invariants

### BR-FC-01: Bất Biến Số Dư Không Âm & Chống Tràn Số
- Tại mọi thời điểm: $0 \le \text{balance} \le \text{math.MaxInt64}$.
- Được bảo vệ 2 lớp:
  1. Logic Domain trong `UserWallet` (`ErrInsufficientFunds`, `ErrBalanceOverflow`).
  2. Ràng buộc PostgreSQL: `CONSTRAINT chk_user_wallet_balance_non_negative CHECK (balance >= 0)` và `CONSTRAINT chk_coin_ledger_balance_after_non_negative CHECK (balance_after >= 0)`.

### BR-FC-02: Nguyên Tắc Sổ Cái Append-Only (Immutable Ledger Rule)
- Bảng `gamification.coin_ledger` chỉ cho phép `INSERT`, nghiêm cấm `UPDATE` hoặc `DELETE`.
- Phương trình cân bằng sổ cái luôn được bảo toàn:
  $$\text{BalanceAfter} = \text{BalanceBefore} + \text{Amount}$$

### BR-FC-03: Chống Chi Tiêu Kép & TOCTOU (Anti-Double-Spending Rule)
- Mọi thao tác thay đổi số dư ví bắt buộc phải:
  1. Chiếm khóa dòng bi quan: `SELECT * FROM gamification.user_wallet WHERE user_id = $1 FOR UPDATE`.
  2. Kiểm tra `FindLedgerEntryByIdempotencyKey(txCtx, userID, key)` **bên trong** phạm vi khóa dòng trước khi biến đổi trạng thái.

### BR-FC-04: Tuân Thủ Trần Thu Nhập Ngày $O(1)$ (Daily Cap Enforcement)
- Kiểm tra `last_workout_reward_date`, `last_pr_reward_date`, `last_nutrition_reward_date` ngay trên aggregate `UserWallet` đang giữ khóa `FOR UPDATE` (chi phí $O(1)$) kết hợp chốt chặn tầng DB qua Partial Unique Index `uq_coin_ledger_daily_cap`.

---

## Acceptance Criteria

- [x] **AC-FC-01**: GIVEN người dùng hoàn thành buổi tập hợp lệ đầu tiên trong ngày WHEN sự kiện `WorkoutSessionCompleted` được xử lý THEN ví được cộng đúng $+10$ FitCoins và ledger ghi nhận bản ghi với `amount = +10`, `reason = EARN_WORKOUT`.
- [x] **AC-FC-02**: GIVEN người dùng hoàn thành buổi tập thứ 2 trong cùng một ngày WHEN sự kiện được xử lý THEN ví không cộng thêm xu buổi tập (`Skipped = true`), nhưng vẫn xử lý XP bình thường.
- [x] **AC-FC-03**: GIVEN người dùng mở khóa huy hiệu mới hoặc chạm mốc Streak 7/14/30/50/100/365 ngày WHEN sự kiện được xử lý THEN ví được cộng tương ứng $+15$ FitCoins mỗi huy hiệu và $+30 \dots +1500$ FitCoins cho cột mốc Streak.
- [x] **AC-FC-04**: GIVEN ví có 50 FitCoins WHEN có yêu cầu `SpendCoins` trừ 60 FitCoins THEN hệ thống từ chối với lỗi `CodeFailedPrecondition` (`ErrInsufficientFunds`) và số dư giữ nguyên 50.
- [x] **AC-FC-05**: GIVEN cùng một yêu cầu `SpendCoins` hoặc `AdminAdjustCoins` có `idempotency_key` được gửi lại lần 2 THEN hệ thống trả về đúng `balance_after` và `transaction_id` ban đầu mà không trừ/cộng tiền lần 2.
- [x] **AC-FC-06**: GIVEN người dùng không có quyền Admin gọi `AdminAdjustCoins` THEN hệ thống từ chối ngay với mã lỗi `CodePermissionDenied`.
