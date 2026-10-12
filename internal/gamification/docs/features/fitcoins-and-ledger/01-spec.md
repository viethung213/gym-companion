# Feature: Tiền Tệ Ảo FitCoins & Sổ Cái Bất Biến (FitCoins & Immutable Ledger)

## Summary
Tính năng **Tiền Tệ Ảo FitCoins & Sổ Cái Bất Biến (FitCoins & Immutable Ledger)** chịu trách nhiệm thiết lập và vận hành nền kinh tế ảo nội bộ cho hệ thống FITAI:
- **Đơn vị tiền tệ FitCoins**: Thước đo giá trị hữu hình vinh danh tính kỷ luật và nỗ lực thực chất của gymer (Earned-Only Currency).
- **Hệ thống Sổ Cái Bất Biến (Immutable Audit Ledger)**: Vận hành theo mô hình append-only, ghi vết $100\%$ giao dịch thu/chi, đảm bảo không thể sửa hoặc xóa dữ liệu cũ, triệt tiêu rủi ro chi tiêu kép (Double Spending) và số dư âm.
- **Tập trung vào Động Cơ Tích Lũy (Earning Engine)**: Phân tích và chuẩn hóa toàn bộ 7 trường hợp gymer có thể tích lũy coin (thăng cấp, hoàn thành buổi tập hợp lệ, phá kỷ lục cá nhân, dinh dưỡng, cột mốc chuỗi ngày, khởi tạo hồ sơ, mở khóa thành tựu) đi kèm **Trần khống chế chống gian lận (Daily Earning Caps)**.
- **Cổng Thưởng Nội Bộ Đồng Bộ (In-Process Rewarder Port)**: Cung cấp Application Port `WalletRewarder` cho phép các điều phối viên (`ActivityIngestionCoordinator`) và dịch vụ nội bộ gọi cộng tiền trực tiếp tức thì ($0\text{ ms}$ lag) trong cùng Database Transaction thay vì chỉ phụ thuộc vào Kafka Consumers.
- **Cổng Chi Tiêu Mở (Generic Spending Seam)**: Cung cấp API trừ coin tổng quát và an toàn (`SpendCoins`), đóng vai trò là "Ngân Hàng" phục vụ các tính năng tiêu dùng sau này (Shop, Voucher đối tác) mà không bị phụ thuộc cứng vào bảng giá hay kho hàng cụ thể.

---

## Problem and Desired Outcome

### Problem
1. **Thiếu phần thưởng hữu hình**: Điểm XP và Cấp độ chỉ mang tính biểu tượng tinh thần; người dùng cần một loại "tài sản ảo" tích lũy được để đổi lấy các đặc quyền và tiện ích thực tế trong ứng dụng.
2. **Rủi ro lạm phát & Gian lận kinh tế ảo**: Nếu không có cơ chế kiểm soát nguồn cung (Faucet) và trần thu nhập hàng ngày (Daily Caps), người dùng có thể gian lận (spam tạo buổi tập ảo 1 phút, log dinh dưỡng ảo) để cày coin vô tội vạ, làm phá sản nền kinh tế in-app.
3. **Lỗ hổng kỹ thuật của ví tiền truyền thống**:
   - Các hệ thống ví chỉ cập nhật đè số dư (`UPDATE wallet SET balance = balance - 100`) thường xuyên gặp lỗi **chi tiêu kép (Double Spending)** khi có 2 request gửi đồng thời, dẫn đến số dư âm hoặc mất dấu vết kiểm toán khi xảy ra khiếu nại.
4. **Độ trễ bất đồng bộ làm giảm trải nghiệm (Eventual Consistency Latency)**:
   - Nếu việc cộng coin CHỈ thực hiện qua việc nghe Kafka events, khi người dùng vừa mở khóa thành tựu hoặc lên cấp trên giao diện (popup chúc mừng), tiền trong ví chưa xuất hiện ngay vì còn nghẽn trong hàng đợi broker, gây cảm giác ứng dụng bị lỗi.

### Desired Outcome
1. **Minh bạch nguồn thu nhập (Observable Inflow)**: Quy định chính xác từng hành vi thể chất nào sinh ra coin, bao nhiêu coin, và giới hạn tối đa mỗi ngày là bao nhiêu.
2. **Kiểm toán bất biến & Ràng buộc toàn vẹn tuyệt đối**:
   - $100\%$ giao dịch thu/chi được ghi 1 dòng vào `gamification.coin_ledger`.
   - Ràng buộc cứng `CHECK (balance >= 0)` và khóa dòng bi quan `SELECT ... FOR UPDATE` đảm bảo không bao giờ âm tiền và chống chi tiêu kép.
3. **Giao tiếp lai linh hoạt (Hybrid Integration)**:
   - Hỗ trợ gọi trực tiếp qua Go Interface `WalletRewarder` trong cùng tiến trình và transaction khi hoàn thành bài tập / mở khóa thành tựu.
   - Hỗ trợ lắng nghe Kafka CloudEvents cho các sự kiện từ các module phân tán bên ngoài.
4. **Tách biệt ranh giới rõ ràng (Separation of Concerns)**:
   - Module FitCoins chỉ đóng vai trò là **Sổ Cái & Ngân Hàng Trung Ương**.
   - Module không quản lý kho hàng, không hardcode danh mục vật phẩm hay bảng giá Shop; chỉ cung cấp cổng trừ coin chuẩn hóa cho các tính năng khác gọi vào.

---

## Actors and Entry Points

- **Gym User (Mobile App / Web Client)**:
  - Xem số dư ví hiện tại: RPC `GetWalletBalance`.
  - Xem lịch sử biến động sổ cái (phân trang): RPC `ListLedgerTransactions`.
- **In-Process Callers (Application Ports)**:
  - `ActivityIngestionCoordinator`: Gọi trực tiếp `WalletRewarder.AwardCoins(txCtx, ...)` để cộng coin buổi tập / PR trong cùng transaction với XP và Streak.
  - `AchievementService` / Use Cases mở khóa thành tựu: Gọi trực tiếp `WalletRewarder.AwardCoins(...)` khi người dùng bứt phá cột mốc.
- **Inbound Event Consumers (Kafka CloudEvents)**:
  - Lắng nghe các sự kiện nỗ lực từ các Bounded Context bên ngoài:
    - `DailyNutritionAchieved` (từ `nutrition`).
    - `ProfileOnboardingCompleted` (từ `profile`).
- **Internal Cross-Module Services**:
  - Các module dịch vụ khác (Shop, Voucher Engine) gọi vào Application Port `WalletService.SpendCoins(...)` để trừ coin khi người dùng thực hiện giao dịch đổi quà.

---

## Scope

### In Scope
- Quản lý số dư ví người dùng (`gamification.user_wallet`).
- Sổ cái kiểm toán bất biến append-only (`gamification.coin_ledger`).
- Đặc tả và chuẩn hóa **toàn bộ 7 trường hợp tích lũy FitCoins** kèm cơ chế trần khống chế (Daily Caps).
- Thiết kế **In-Process Application Port (`WalletRewarder`)** cộng coin tức thì trong cùng tiến trình/transaction.
- Cung cấp cơ chế trừ coin an toàn (`SpendCoins`) hỗ trợ `idempotency_key` và kiểm soát số dư tối thiểu.
- API ConnectRPC Protobuf: `GetWalletBalance`, `ListLedgerTransactions`.

### Out of Scope & Explicitly Deferred
- **Bảng giá cứng & Kho hàng vật phẩm Cửa Hàng [DEFERRED]**: Không hardcode giá vật phẩm hay XP Boost vào module này. Danh mục hàng hóa sẽ do phân hệ Shop/Inventory quản lý độc lập.
- **Nạp tiền thật mua FitCoins (IAP / Fiat Gateway) [DEFERRED]**: FitCoins là tiền tệ tích lũy thuần túy qua vận động (Proof of Workout), chưa hỗ trợ nạp tiền thật qua Apple Pay/Google Play/Stripe.
- **Chuyển coin P2P giữa người dùng [DEFERRED]**: Nghiêm cấm chuyển tiền giữa các tài khoản để ngăn chặn cày clone và gian lận.

---

## Phân Tích Toàn Diện Các Trường Hợp Tích Lũy FitCoins (Earning Cases)

Hệ thống thiết lập 7 kịch bản tích lũy coin (Inflow Cases), bảo đảm mọi đồng coin sinh ra đều gắn liền với nỗ lực thể chất có thật:

```mermaid
flowchart TD
    subgraph EarningSources["Nguồn Tích Lũy FitCoins (Inflow Sources)"]
        C1["Case 1: Thăng Cấp Độ<br/>(Level Up Bonus)"]
        C2["Case 2: Hoàn Thành Buổi Tập Hợp Lệ<br/>(Proof of Workout)"]
        C3["Case 3: Phá Kỷ Lục Cá Nhân<br/>(Personal Record - PR)"]
        C4["Case 4: Kỷ Luật Dinh Dưỡng<br/>(Daily Nutrition Goal)"]
        C5["Case 5: Cột Mốc Chuỗi Ngày<br/>(Streak Milestones)"]
        C6["Case 6: Hoàn Tất Onboarding<br/>(First-time Profile Setup)"]
        C7["Case 7: Mở Khóa Thành Tựu<br/>(Achievement / Badge Bonus)"]
    end

    subgraph LedgerGuard["Hàng Rào Kiểm Soát & Chống Gian Lận"]
        CAP["Daily Caps & Idempotency Key"]
        CHECK["Validation & Proof of Workout Check"]
    end

    subgraph LedgerStorage["Hệ Thống Sổ Cái (Core Ledger)"]
        WALLET["user_wallet<br/>(Current Balance)"]
        LEDGER["coin_ledger<br/>(Append-Only Audit Log)"]
    end

    C1 & C2 & C3 & C4 & C5 & C6 & C7 --> CAP --> CHECK --> WALLET & LEDGER
```

---

### Case 1: Thưởng Thăng Cấp Độ Trọn Đời (Level Up Bonus)
- **Bản chất**: Thưởng khi gymer tích lũy đủ XP để nâng Level từ cấp cũ lên cấp mới.
- **Điều kiện**: Gọi qua `WalletRewarder` hoặc nhận sự kiện `UserLeveledUp`.
- **Công thức tính thưởng**:
  - **Cấp thông thường**: Thưởng cố định **$+10$ FitCoins**.
  - **Cột mốc tròn chục (10, 20, 30, 40, 50...)**: Thưởng lớn **$+50$ FitCoins**.
  - **Cột mốc huyền thoại (Level 100)**: Thưởng đặc biệt **$+200$ FitCoins**.
- **Cơ chế chống gian lận**: Do hàm XP căn bậc hai có độ dốc tăng dần, người dùng không thể spam lên level liên tục trong thời gian ngắn.
- **Idempotency Key**: `earn:level_up:<user_id>:<new_level>`.

---

### Case 2: Thưởng Hoàn Thành Buổi Tập Hợp Lệ (Proof of Workout Reward)
- **Bản chất**: Thưởng cho từng buổi tập luyện thực tế mỗi ngày.
- **Tiêu chuẩn Buổi tập hợp lệ (Proof of Workout)**:
  - Thời lượng buổi tập $\ge 15$ phút.
  - Tổng số hiệp tập (sets) $\ge 3$.
  - Không bị cờ bất thường (`is_anomalous_session == false`).
- **Mức thưởng**: **$+5$ FitCoins / buổi tập hợp lệ**.
- **Trần khống chế gian lận (Daily Cap - The 3 AM Test)**:
  - **Tối đa 1 lần nhận thưởng tập luyện / ngày** theo múi giờ địa phương (`user_local_date`).
  - *Ý nghĩa*: Dù gymer tập 2–3 buổi trong ngày, chỉ buổi tập đầu tiên đạt chuẩn được nhận $+5$ coin (các buổi sau vẫn nhận XP bình thường nhưng không nhận coin). Ngăn chặn $100\%$ hành vi bấm hoàn thành 20 bài tập ảo để cày trộm coin.
- **Idempotency Key**: `earn:workout:<workout_session_id>`.

---

### Case 3: Thưởng Phá Kỷ Lục Cá Nhân (Personal Record - PR Bonus)
- **Bản chất**: Khuyến khích người dùng bứt phá giới hạn thể lực bản thân (nâng mức tạ nặng hơn hoặc nhiều rep hơn ở các bài tập lớn: Bench Press, Squat, Deadlift...).
- **Điều kiện**: Buổi tập hợp lệ có đánh dấu cờ `is_pr == true` từ phân hệ `workout_execution`.
- **Mức thưởng**: **$+5$ FitCoins / ngày có kỷ lục mới**.
- **Trần khống chế gian lận (Daily Cap)**:
  - **Tối đa 1 lần thưởng PR / ngày** (tối đa $+5$ coin/ngày), bất kể trong buổi tập đó người dùng phá PR ở bao nhiêu bài tập.
- **Idempotency Key**: `earn:pr:<user_id>:<user_local_date>`.

---

### Case 4: Thưởng Kỷ Luật Dinh Dưỡng Hàng Ngày (Nutrition Adherence Reward)
- **Bản chất**: Thể hình đòi hỏi cả tập luyện lẫn ăn uống. Thưởng cho gymer khi hoàn thành mục tiêu năng lượng và đạm trong ngày.
- **Điều kiện**: Nhận sự kiện `DailyNutritionGoalAchieved` xác nhận lượng Calo và Protein đạt mục tiêu trong dung sai cho phép ($\pm 10\%$).
- **Mức thưởng**: **$+3$ FitCoins / ngày**.
- **Trần khống chế gian lận (Daily Cap)**:
  - **Tối đa 1 lần / ngày** theo ngày địa phương `user_local_date`.
- **Idempotency Key**: `earn:nutrition:<user_id>:<user_local_date>`.

---

### Case 5: Thưởng Cán Cột Mốc Chuỗi Ngày Bền Bỉ (Streak Milestones Reward)
- **Bản chất**: Tôn vinh tính kiên trì dài hạn (chỉ thưởng coin ở các cột mốc lớn, ngày thường không thưởng coin để tránh lạm phát).
- **Mức thưởng bậc thang**:
  - Cán mốc **7 ngày** (1 tuần): Thưởng **$+15$ FitCoins**.
  - Cán mốc **14 ngày** (2 tuần): Thưởng **$+30$ FitCoins**.
  - Cán mốc **30 ngày** (1 tháng): Thưởng **$+50$ FitCoins**.
  - Cán mốc **50 ngày**: Thưởng **$+75$ FitCoins**.
  - Cán mốc **100 ngày**: Thưởng **$+150$ FitCoins**.
  - Cán mốc **365 ngày** (1 năm): Thưởng **$+500$ FitCoins**.
- **Điều kiện**: Gọi trực tiếp qua In-Process Port `WalletRewarder` hoặc nhận sự kiện `StreakMilestoneReached`. Mỗi cột mốc chỉ nhận thưởng **duy nhất 1 lần trọn đời**.
- **Idempotency Key**: `earn:streak:<user_id>:<milestone_days>`.

---

### Case 6: Thưởng Hoàn Tất Khởi Tạo Hồ Sơ (First-Time Onboarding Bonus)
- **Bản chất**: Tặng "vốn khởi nghiệp" ban đầu khi người dùng mới hoàn thành thiết lập hồ sơ thể trạng (chiều cao, cân nặng, mục tiêu).
- **Mức thưởng**: **$+20$ FitCoins**.
- **Điều kiện**: Chỉ nhận **duy nhất 1 lần trong suốt vòng đời tài khoản**.
- **Idempotency Key**: `earn:onboarding:<user_id>`.

---

### Case 7: Thưởng Mở Khóa Thành Tựu (Achievement Unlocked Bonus)
- **Bản chất**: Thưởng khi gymer chinh phục một thử thách hoặc đạt danh hiệu thể hình đặc biệt (ví dụ: *"Chiến Binh 5 Giờ Sáng"*, *"Gia Nhập CLB 100kg Bench Press"*, *"Tập Luyện 30 Ngày Liên Tục"*).
- **Mức thưởng theo cấp bậc (Achievement Tier)**:
  - **Đồng (Bronze)**: Thưởng **$+10$ FitCoins**.
  - **Bạc (Silver)**: Thưởng **$+25$ FitCoins**.
  - **Vàng (Gold)**: Thưởng **$+50$ FitCoins**.
  - **Bạch Kim (Platinum)**: Thưởng **$+100$ FitCoins**.
  - **Kim Cương (Diamond / Legend)**: Thưởng **$+250$ FitCoins**.
- **Phương thức tiếp nhận**: Gọi qua In-Process Port `WalletRewarder.AwardCoins(...)` hoặc qua Consumer lắng nghe `AchievementUnlocked`.
- **Ràng buộc chống gian lận**:
  - Mỗi thành tựu chỉ nhận thưởng **DUY NHẤT 1 LẦN TRỌN ĐỜI** cho mỗi tài khoản.
  - Bắt buộc kiểm tra tính lũy đẳng: Nếu thành tựu đã nhận coin $\rightarrow$ Bỏ qua và không ghi ledger lần 2.
- **Idempotency Key**: `earn:achievement:<user_id>:<achievement_id>`.

---

### Tổng Kết Ma Trận Thu Nhập & Trần Kiểm Soát (Tokenomics Summary)

| Nguồn tích lũy | Tần suất phát sinh | Mức thưởng (FitCoins) | Trần khống chế (Daily Cap / Limit) |
| :--- | :--- | :---: | :--- |
| **1. Thăng cấp độ** | Khi tích lũy đủ XP | **$+10$** (Cột mốc: **$+50$** – **$+200$**) | Tự nhiên theo đường cong XP căn bậc hai |
| **2. Buổi tập hợp lệ** | Hàng ngày | **$+5$** | **Tối đa 1 lần / ngày** (5 coin/ngày) |
| **3. Kỷ lục cá nhân (PR)** | Khi phá kỷ lục | **$+5$** | **Tối đa 1 lần / ngày** (5 coin/ngày) |
| **4. Dinh dưỡng kỷ luật** | Hàng ngày | **$+3$** | **Tối đa 1 lần / ngày** (3 coin/ngày) |
| **5. Cột mốc Streak** | Khi cán mốc 7, 14, 30, 50, 100, 365 ngày | **$+15$** $\rightarrow$ **$+500$** | 1 lần cho mỗi cột mốc trọn đời |
| **6. Khởi tạo hồ sơ** | 1 lần duy nhất | **$+20$** | 1 lần duy nhất trọn đời |
| **7. Mở khóa thành tựu** | Khi đạt danh hiệu/thử thách | **$+10$** $\rightarrow$ **$+250$** (theo Tier) | 1 lần duy nhất cho mỗi thành tựu trọn đời |

---

## Khung Cơ Chế Chi Tiêu Mở (Generic Spending Framework)

Thay vì hardcode danh mục vật phẩm, module FitCoins cung cấp cơ chế trừ coin an toàn:

### Cổng trừ coin nghiệp vụ (`SpendCoins`)
```go
type SpendRequest struct {
    UserID         uuid.UUID
    Amount         int64          // Số coin cần trừ (> 0)
    Reason         string         // Mã nghiệp vụ: SPEND_DOUBLE_XP, SPEND_VOUCHER...
    IdempotencyKey string         // Khóa lũy đẳng chống trừ trùng
    Metadata       map[string]any // Dữ liệu bổ trợ
}
```

---

## User Scenarios / Use Cases

### UC-FC-01: Tích Lũy Coin Tự Động Từ Hoạt Động (Earning Inflow)
- **Actor**: Inbound Kafka Consumers.
- **Trigger**: Hệ thống phát sinh một trong các sự kiện: `WorkoutCompleted`, `UserLeveledUp`, `DailyNutritionAchieved`, `StreakMilestoneReached`, `AchievementUnlocked`.
- **Main Flow**:
  1. Consumer xác thực tính hợp lệ của sự kiện và tạo `idempotency_key` tương ứng.
  2. Bắt đầu Database Transaction:
     - Khóa dòng ví: `SELECT balance FROM gamification.user_wallet WHERE user_id = :id FOR UPDATE`.
     - Kiểm tra `coin_ledger` theo `idempotency_key`. Nếu đã tồn tại $\rightarrow$ Trả về thành công ngay (Idempotent).
     - Kiểm tra trần Daily Cap trong ngày (nếu là sự kiện workout, pr, nutrition).
     - Cập nhật số dư: `new_balance = balance + amount`.
     - Chèn bản ghi append-only vào `coin_ledger` với `amount`, `balance_after = new_balance`, `reason`, `idempotency_key`.
  3. Commit transaction.
- **Postconditions**: Ví người dùng được cộng coin chính xác, số dư cập nhật và có dòng kiểm toán bất biến.

---

### UC-FC-02: Thực Hiện Giao Dịch Chi Tiêu Coin (Spending Outflow)
- **Actor**: Tính năng nghiệp vụ nội bộ (Shop / Perks).
- **Trigger**: Người dùng xác nhận đổi một tiện ích hoặc vật phẩm.
- **Main Flow**:
  1. Dịch vụ gọi `WalletService.SpendCoins(ctx, req)`.
  2. Bắt đầu Database Transaction:
     - Khóa dòng ví: `SELECT balance FROM gamification.user_wallet WHERE user_id = :id FOR UPDATE`.
     - Kiểm tra `idempotency_key`: Nếu đã từng xử lý thành công $\rightarrow$ Trả về kết quả trước đó.
     - Kiểm tra số dư: Nếu `balance < req.Amount` $\rightarrow$ Rollback transaction, trả về lỗi `ERR_INSUFFICIENT_FUNDS`.
     - Cập nhật số dư: `new_balance = balance - req.Amount`.
     - Chèn bản ghi vào `coin_ledger`: `amount = -req.Amount`, `balance_after = new_balance`, `reason`, `idempotency_key`.
  3. Commit transaction.
  4. Trả về thành công kèm `balance_after`.
- **Alternate Flow (Không đủ số dư)**:
  - Nếu `balance < req.Amount`: Báo lỗi `FAILED_PRECONDITION: Số dư FitCoins không đủ`, không ghi ledger, không thay đổi số dư ví.

---

### UC-FC-03: Tra Cứu Số Dư & Lịch Sử Giao Dịch Sổ Cái
- **Actor**: Gym User (Đã xác thực).
- **Main Flow**:
  1. Người dùng gọi API `GetWalletBalance` $\rightarrow$ Trả về `balance` hiện tại.
  2. Người dùng gọi API `ListLedgerTransactions(limit = 20, cursor = ...)` $\rightarrow$ Trả về danh sách các dòng lịch sử thu/chi từ `coin_ledger` kèm phân trang.

---

### UC-FC-04: Thưởng Coin Trực Tiếp Qua In-Process Port (WalletRewarder)
- **Actor**: `ActivityIngestionCoordinator`, `AchievementService`, hoặc các use cases nội bộ khác.
- **Trigger**: Hoàn tất buổi tập hoặc đạt thành tựu trong cùng tiến trình ứng dụng.
- **Main Flow**:
  1. Caller khởi tạo `AwardCoinsCommand` chứa `UserID`, `Amount`, `Reason`, `IdempotencyKey`.
  2. Caller gọi `walletRewarder.AwardCoins(ctx, cmd)`.
  3. Nếu `ctx` đã bọc transaction (từ `txManager.WithTransaction`): Hàm sử dụng transaction hiện tại; nếu chưa có, hàm tự tạo transaction độc lập.
  4. Thực thi khóa bi quan `SELECT ... FOR UPDATE` trên `user_wallet`, kiểm tra idempotency, cập nhật số dư và ghi 1 dòng vào `coin_ledger`.
  5. Trả về `AwardCoinsResult` chứa số dư mới tức thì cho Caller mà không phát sinh độ trễ hàng đợi.

---

## Business Rules and Invariants

### BR-FC-01: Bất Biến Số Dư Không Âm (Non-Negative Balance Invariant)
- Tại mọi thời điểm:
  $$\text{balance} \ge 0$$
- Được bảo vệ 2 lớp:
  1. Logic Domain kiểm tra: `if balance < amount { return ErrInsufficientBalance }`.
  2. Ràng buộc PostgreSQL: `CONSTRAINT chk_wallet_balance_non_negative CHECK (balance >= 0)`.

### BR-FC-02: Nguyên Tắc Sổ Cái Append-Only (Immutable Ledger Rule)
- Bảng `gamification.coin_ledger` nghiêm cấm cập nhật (`UPDATE`) hoặc xóa (`DELETE`).
- Phương trình cân bằng sổ cái luôn được bảo toàn:
  $$\text{BalanceAfter} = \text{BalanceBefore} + \text{Amount}$$

### BR-FC-03: Chống Chi Tiêu Kép (Anti-Double-Spending Rule)
- Mọi thao tác thay đổi số dư ví bắt buộc phải sử dụng khóa dòng bi quan:
  ```sql
  SELECT balance FROM gamification.user_wallet WHERE user_id = $1 FOR UPDATE;
  ```
- Đảm bảo tuần tự hóa tuyệt đối, loại bỏ Lost Update khi nhiều request diễn ra đồng thời.

### BR-FC-04: Tuân Thủ Trần Thu Nhập Ngày (Daily Cap Enforcement)
- Hệ thống khống chế chặt chẽ giới hạn thu nhập theo ngày dương lịch địa phương (`user_local_date`):
  - Tối đa 1 lần thưởng Buổi tập hợp lệ / ngày ($+5$ coin).
  - Tối đa 1 lần thưởng Phá kỷ lục cá nhân / ngày ($+5$ coin).
  - Tối đa 1 lần thưởng Kỷ luật dinh dưỡng / ngày ($+3$ coin).

---

## Error and Boundary Scenarios

- **Số Dư Không Đủ**: Trả về `ERR_INSUFFICIENT_FUNDS`, không trừ tiền, không ghi ledger.
- **Trùng Lặp Idempotency Key**: Trả về kết quả giao dịch cũ, không thực hiện cộng/trừ lần 2.
- **Tập Luyện Nhiều Buổi Trong Ngày**: Buổi thứ nhất nhận $+5$ coin; các buổi tiếp theo trong cùng ngày nhận $0$ coin (vẫn nhận XP bình thường).
- **Người Dùng Mới Chưa Có Ví**: Khởi tạo tự động ví với `balance = 0` trong lần giao dịch đầu tiên.

---

## Acceptance Criteria

- [ ] **AC-FC-01**: GIVEN người dùng hoàn thành buổi tập hợp lệ đầu tiên trong ngày WHEN sự kiện `WorkoutCompleted` được xử lý THEN ví được cộng đúng $+5$ FitCoins và ledger ghi nhận bản ghi với `amount = +5`.
- [ ] **AC-FC-02**: GIVEN người dùng hoàn thành buổi tập thứ 2 trong cùng một ngày WHEN sự kiện `WorkoutCompleted` được xử lý THEN ví không được cộng thêm coin (Daily Cap = 1 lần/ngày).
- [ ] **AC-FC-03**: GIVEN người dùng lên Level 10 (mốc tròn chục) WHEN gọi thưởng coin THEN ví được cộng đúng $+50$ FitCoins.
- [ ] **AC-FC-04**: GIVEN ví có 50 FitCoins WHEN có yêu cầu trừ 60 FitCoins THEN hệ thống từ chối với lỗi `ERR_INSUFFICIENT_FUNDS` và số dư giữ nguyên 50.
- [ ] **AC-FC-05**: GIVEN ví có 50 FitCoins WHEN có 2 yêu cầu trừ 40 FitCoins gửi đến đồng thời THEN đúng 1 yêu cầu thành công, yêu cầu còn lại bị từ chối do không đủ số dư (chống chi tiêu kép).
- [ ] **AC-FC-06**: GIVEN cùng một sự kiện nạp coin có `idempotency_key` bị gửi lại 2 lần THEN ví chỉ được cộng tiền đúng 1 lần duy nhất.
- [ ] **AC-FC-07**: GIVEN người dùng mở khóa thành tựu Vàng WHEN gọi `WalletRewarder.AwardCoins` THEN ví được cộng đúng $+50$ FitCoins và ledger ghi nhận với `reason = EARN_ACHIEVEMENT`. Nếu gọi lại cùng `achievement_id` thì trả về kết quả cũ, không cộng lặp.

---

## Non-Functional Requirements

- **The 3 AM Test**: Zero cron jobs định kỳ. Toàn bộ tính toán diễn ra theo thời gian thực (Real-time).
- **ACID Transaction**: Cập nhật `user_wallet` và chèn bản ghi `coin_ledger` bắt buộc nằm trong cùng 1 Database Transaction.
- **Hiệu Năng**: Truy vấn số dư ví $< 2\text{ ms}$; ghi nhận giao dịch $< 5\text{ ms}$.
