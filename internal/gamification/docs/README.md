# Bounded Context: Gamification Engine — Gym Companion

Tài liệu định nghĩa nghiệp vụ và ranh giới hệ thống của module **Gamification** (`/internal/gamification/`) trong nền tảng FITAI.

Module này áp dụng triết lý Gamification thực chứng chuẩn **Duolingo** cho thể hình: **100% củng cố tích cực (Positive Reinforcement)**, xóa bỏ hoàn toàn cơ chế phạt/trừ điểm/tuột cấp, tạo động lực bền vững qua Hệ Thống XP & Cấp Độ Trọn Đời (XP & Levels), Bảng Xếp Hạng Toàn Hệ Thống (XP Leaderboard), Chuỗi Ngày (Streaks), Huy Hiệu (Badges) và Ví Ảo FitCoins & Sổ Cái Bất Biến (FitCoins & Immutable Ledger).

```text
internal/gamification/docs/
├── README.md                          # Định nghĩa tổng quan module & các trụ cột nghiệp vụ chuẩn Duolingo
├── architecture.md                    # Thiết kế kiến trúc kỹ thuật Hexagonal, Database & Event Bus
└── features/                          # Đặc tả chi tiết từng tính năng (Feature Specifications)
    ├── xp-and-levels/                 # 1. Hệ Thống XP & Cấp Độ Trọn Đời (XP & Level 1-100) (Đã triển khai)
    │   ├── 01-spec.md                 # Đặc tả nghiệp vụ & Acceptance Criteria
    │   ├── adr/                       # Các bản ghi quyết định kiến trúc (ADR-0001 -> ADR-0004)
    │   ├── 02-design.md               # Thiết kế kỹ thuật Hexagonal & DDL
    │   └── 04-tasks.md                # Danh sách đầu việc chi tiết theo chuẩn TDD (T-01 -> T-12)
    ├── leaderboards/                  # 2. Bảng Xếp Hạng Toàn Hệ Thống Theo Điểm XP (Đã triển khai)
    │   ├── 01-spec.md                 # Đặc tả nghiệp vụ & Acceptance Criteria
    │   ├── adr/                       # Các bản ghi quyết định kiến trúc (ADR-0001, ADR-0002)
    │   ├── 02-design.md               # Thiết kế kỹ thuật Hexagonal & DDL
    │   └── 04-tasks.md                # Danh sách đầu việc chi tiết theo chuẩn TDD (T-LB-01 -> T-LB-07)
    ├── streaks-and-habits/            # 3. Chuỗi Ngày Kỷ Luật (Streaks Engine) (Đã triển khai)
    │   ├── 01-spec.md                 # Đặc tả nghiệp vụ & Thói quen kép
    │   ├── adr/                       # Các bản ghi quyết định kiến trúc (ADR-0001, ADR-0002)
    │   ├── 02-design.md               # Thiết kế kỹ thuật Hexagonal & DDL
    │   └── 04-tasks.md                # Danh sách đầu việc chi tiết theo chuẩn TDD (T-ST-01 -> T-ST-06)
    └── fitcoins-and-ledger/           # 4. Ví Ảo FitCoins & Sổ Cái Bất Biến (Đã triển khai)
        ├── 01-spec.md                 # Đặc tả nghiệp vụ, Tokenomics & Daily Caps
        ├── adr/                       # Các bản ghi quyết định kiến trúc (ADR-0001 -> ADR-0004)
        ├── 02-design.md               # Thiết kế kỹ thuật Hexagonal, In-Process Port & DDL
        └── 04-tasks.md                # Danh sách đầu việc chi tiết theo chuẩn TDD (T-FC-01 -> T-FC-08)
```

---

## 1. Mục Tiêu & Trách Nhiệm Của Module (Core Purpose)

Module **Gamification** đóng vai trò là phân hệ bổ trợ (**Supporting Domain**), phụ trách toàn bộ hệ thống động lực, ghi nhận thói quen và vinh danh nỗ lực cho gymer dựa trên các mô thức thành công của Duolingo:
* **100% Củng cố tích cực (Zero Penalties)**: Không phạt trừ điểm, không decay khi nghỉ ngơi, không tuột Cấp độ (Level). Mọi giọt mồ hôi đều tích lũy giá trị vĩnh viễn. Tuần deload phục hồi hay ngày nghỉ ốm không bao giờ bị trừng phạt.
* **Tích lũy dài hạn & Vinh danh minh bạch**:
  * **Cấp độ trọn đời**: **XP & Level 1–100** vinh danh tổng khối lượng công sức và thời gian gắn bó trọn đời.
  * **Bảng xếp hạng toàn hệ thống**: **XP Leaderboard** hiển thị Top 50 gymer bền bỉ nhất nền tảng và vị trí cá nhân theo thời gian thực.
  * **Tài sản nỗ lực hữu hình**: **FitCoins & Immutable Ledger** ghi nhận phần thưởng xu ảo từ nỗ lực vận động thực chất trên sổ cái kiểm toán bất biến.
* **Hình thành thói quen kỷ luật**: Duy trì Chuỗi Ngày (Streak) hàng ngày kết hợp thói quen kép (Workout & Nutrition).

### Trách nhiệm:
* Lắng nghe sự kiện đăng ký tài khoản từ Auth để khởi tạo tức thì hồ sơ Game hóa cơ sở (`user_xp = 0, level = 1`, `user_streak = 0`, `user_wallet = 0`) và snapshot thông tin hiển thị.
* Quản lý điểm kinh nghiệm XP và cấp độ Level (`user_xp`, `xp_history`).
* Cung cấp dữ liệu Bảng xếp hạng toàn hệ thống theo điểm XP (Top 50) và tra cứu thứ hạng cá nhân (`my_rank`).
* Quản lý chuỗi ngày tập liên tục (Streak) và lịch sử hoạt động (`user_streak`, `streak_history`).
* Quản lý danh hiệu, huy hiệu và thành tựu cá nhân (`user_badges`).
* Quản lý số dư ví ảo FitCoins (`user_wallet`) và sổ cái giao dịch bất biến (`coin_ledger`) kèm kiểm soát trần nhận thưởng hàng ngày (Daily Caps).

### Không thuộc trách nhiệm:
* Không xử lý cổng thanh toán tiền thật (IAP / Fiat Gateway) hay chuyển tiền P2P giữa người dùng.
* Không theo dõi chuyển động camera hay đếm rep (thuộc `workout_execution`).
* Không quản lý phòng đấu real-time qua WebSocket hay ghép trận đối kháng trực tiếp (thuộc `competition`).
* Không quản lý đồ thị bạn bè follow/following (thuộc `social`).

---

## 2. Các Trụ Cột Nghiệp Vụ Chính (High-Level)

### 2.1. [Hệ Thống XP & Cấp Độ Trọn Đời (XP & Level Progression)](./features/xp-and-levels/01-spec.md)
* **Thước đo nỗ lực tuyệt đối**: Sử dụng **Điểm Kinh Nghiệm (XP)** làm đơn vị tính toán duy nhất. Tập luyện là có XP, không bao giờ bị trừ điểm.
* **Công thức XP minh bạch theo buổi tập**:
  $$\text{XP} = (\text{Base: 50} + \text{Volume: 10–30} + \text{Form: 10–20} + \text{PR: 25}) \times \text{Streak Multiplier (1.0x – 1.2x)}$$
* **Thưởng dinh dưỡng**: Nhận thêm $+30$ XP khi hoàn thành mục tiêu Calo/Protein trong ngày (không phạt khi quên log hay ăn cheat meal).
* **Cấp độ trọn đời (Level 1–100)**: Xác định theo hàm căn bậc hai:
  $$\text{Level}(X) = \min\left(\lfloor \sqrt{X / 50} \rfloor + 1, 100\right) \quad (\text{với } X \text{ là } \texttt{xp})$$
  Cấp độ chỉ tăng không giảm, vĩnh viễn vinh danh đẳng cấp của gymer.
* Chi tiết đặc tả: xem [features/xp-and-levels/01-spec.md](./features/xp-and-levels/01-spec.md).

---

### 2.2. [Bảng Xếp Hạng Toàn Hệ Thống Theo Điểm XP (XP Leaderboard)](./features/leaderboards/01-spec.md)
* **Thước đo duy nhất**: Toàn bộ thứ hạng được tính toán dựa trên **Điểm Kinh Nghiệm Trọn Đời (`xp`)**.
* **Tối giản & Trực quan**: Loại bỏ sự phức tạp của việc chia phòng đấu hay reset tuần; ai tích lũy nỗ lực nhiều hơn sẽ đứng cao hơn.
* **Bảng vàng danh dự**:
  * Hiển thị danh sách **Top 50** gymer có điểm tích lũy cao nhất toàn nền tảng.
  * Hiển thị chính xác **Vị trí cá nhân (My Standing)** và khoảng cách điểm số, bất kể người dùng nằm trong hay ngoài Top 50.
* **Hiệu năng & Thời gian thực (The 3 AM Test)**:
  * Không sử dụng cron job ngầm hay worker chốt giải đấu; thứ hạng cập nhật tức thì theo thời gian thực mỗi khi có buổi tập mới.
  * Tối ưu hóa bằng index B-tree `(xp DESC, updated_at ASC, user_id ASC)` trên bảng `gamification.user_xp`, đảm bảo thời gian truy vấn $< 5\text{ ms}$.
* Chi tiết đặc tả: xem [features/leaderboards/01-spec.md](./features/leaderboards/01-spec.md).

---

### 2.3. [Chuỗi Ngày Kỷ Luật (Streaks Engine)](./features/streaks-and-habits/01-spec.md)
* **Động cơ giữ chân cốt lõi (Habit Engine)**: Đếm số ngày liên tục người dùng hoàn thành ít nhất 1 buổi tập hoặc đạt mục tiêu dinh dưỡng.
* **Kỷ luật thực chất (Workout & Nutrition)**: Ngày tập thì nâng tạ, ngày nghỉ thì log ăn uống đúng chuẩn Calo/Protein để giữ chuỗi liên tục.
* **Cột mốc Chuỗi (Streak Milestones)**: Vinh danh khi cán mốc 7, 14, 30, 50, 100, 365 ngày liên tục qua domain event `StreakMilestoneReached`.
* Chi tiết đặc tả: xem [features/streaks-and-habits/01-spec.md](./features/streaks-and-habits/01-spec.md).

---

### 2.4. Huy Hiệu & Thành Tựu (Badges & Achievements)
* **Vinh danh thành tựu (Milestones & Trophies)**: Mở khóa các huy hiệu biểu tượng khi gymer vượt qua các giới hạn bản thân (Buổi tập đầu tiên, Mốc chuỗi 7/14/30/50/100/365 ngày, Đạt Cấp độ 10/25/50/100, Phá kỷ lục PR).
* **Mở khóa vĩnh viễn (Immutable Unlocks)**: Huy hiệu một khi đã đạt được sẽ được lưu trữ vĩnh viễn trong `user_badges` (`badge_code`, `unlocked_at`, `metadata`).
* **Sự kiện phát hành**: Phát tán `BadgeUnlocked` qua Kafka Outbox khi có huy hiệu mới được mở khóa.

---

### 2.5. [Tiền Tệ Ảo FitCoins & Sổ Cái Bất Biến (FitCoins & Immutable Ledger)](./features/fitcoins-and-ledger/01-spec.md)
* **Tiền tệ tích lũy từ nỗ lực (Earned-Only Currency)**: Thưởng xu ảo khi hoàn thành buổi tập ($+10$/ngày), phá kỷ lục PR ($+20$/ngày), đạt mục tiêu dinh dưỡng ($+3$/ngày), mở khóa huy hiệu ($+15$/huy hiệu), và chạm cột mốc chuỗi ngày ($+30 \rightarrow +1500$).
* **Sổ cái kiểm toán bất biến (`gamification.coin_ledger`)**: Append-only 100%, bảo toàn phương trình kế toán $\text{BalanceAfter} = \text{BalanceBefore} + \text{Amount}$ dưới khóa bi quan `SELECT ... FOR UPDATE`.
* **Kiểm soát Daily Cap $O(1)$**: Lưu trữ ngày nhận thưởng gần nhất (`last_workout_reward_date`, `last_pr_reward_date`, `last_nutrition_reward_date`) trực tiếp trên `gamification.user_wallet` kết hợp Partial Unique Index `uq_coin_ledger_daily_cap`.
* Chi tiết đặc tả: xem [features/fitcoins-and-ledger/01-spec.md](./features/fitcoins-and-ledger/01-spec.md).

---

## 3. Kiến Trúc Kỹ Thuật

Xem chi tiết tại: **[architecture.md](./architecture.md)**
* **Mô hình Hexagonal (Ports & Adapters)**: Bóc tách 4 tầng Domain, Application, Infrastructure, Transport.
* **PostgreSQL Schema cô lập**: Schema riêng `gamification.*`, không JOIN chéo module khác.
* **Kafka Event Bus & Outbox**: Giao tiếp bất đồng bộ qua CloudEvents 1.0 JSON và Outbox Pattern.
* **Tính lũy đẳng (Idempotency)**: Enforce qua khóa duy nhất nghiệp vụ (`uq_xp_history_workout_session`, `uq_xp_history_nutrition_daily`, `uq_coin_ledger_user_idempotency`, `uq_coin_ledger_daily_cap`), loại bỏ hoàn toàn nguy cơ cộng/trừ trùng lặp.

---

## 4. Bảng Thuật Ngữ Nghiệp Vụ (Glossary)

| Thuật ngữ (English) | Thuật ngữ (Tiếng Việt) | Định nghĩa & Ý nghĩa nghiệp vụ chuẩn Duolingo |
| :--- | :--- | :--- |
| **XP (`xp`)** | Điểm Kinh Nghiệm | Thước đo tích lũy nỗ lực vĩnh viễn, 100% củng cố tích cực, không bao giờ bị trừ hay suy giảm theo thời gian. |
| **Level (`level`)** | Cấp Độ Trọn Đời | Cấp độ từ 1 đến 100, xác định động theo căn bậc hai của XP: $\lfloor \sqrt{X / 50} \rfloor + 1$ (với $X$ là `xp`). Cấp độ chỉ tăng không giảm. |
| **Leaderboard** | Bảng Xếp Hạng Toàn Hệ Thống | Bảng vinh danh Top 50 gymer có số điểm `xp` cao nhất nền tảng và vị trí cá nhân theo thời gian thực. |
| **Streak** | Chuỗi Ngày Kỷ Luật | Số ngày liên tiếp người dùng hoàn thành buổi tập hoặc đạt mục tiêu dinh dưỡng theo múi giờ địa phương (`user_local_date`). |
| **FitCoins** | Tiền Tệ Ảo FitCoins | Đơn vị tiền tệ nội bộ tích lũy từ nỗ lực tập luyện/dinh dưỡng, dùng để đổi đặc quyền trong ứng dụng và ghi vết trên sổ cái bất biến (`coin_ledger`). |
| **Proof of Workout** | Tiêu Chuẩn Buổi Tập Hợp Lệ | Điều kiện tối thiểu để buổi tập được xét thưởng XP/FitCoins và duy trì Streak: thời lượng $\ge 15$ phút, số hiệp $\ge 3$, và không bị cờ `Anomalous Session`. |
