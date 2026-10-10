# Bounded Context: Gamification Engine — Gym Companion

Tài liệu định nghĩa nghiệp vụ và ranh giới hệ thống của module **Gamification** (`/internal/gamification/`) trong nền tảng FITAI.

Module này áp dụng triết lý Gamification thực chứng chuẩn **Duolingo** cho thể hình: **100% củng cố tích cực (Positive Reinforcement)**, xóa bỏ hoàn toàn cơ chế phạt/trừ điểm/tuột cấp, tạo động lực bền vững qua Cấp Độ Trọn Đời (Lifetime XP), Giải Đấu Tuần 30 Người (Weekly Leagues), Chuỗi Ngày (Streaks) và Đóng Băng Chuỗi (Streak Freeze).

```text
internal/gamification/docs/
├── README.md                          # Định nghĩa tổng quan module & các trụ cột nghiệp vụ chuẩn Duolingo
├── architecture.md                    # Thiết kế kiến trúc kỹ thuật Hexagonal, Database & Event Bus
└── features/                          # Đặc tả chi tiết từng tính năng (Feature Specifications)
    ├── xp-and-rank-tiers/             # 1. Hệ Thống XP & Cấp Độ Trọn Đời (Lifetime XP & Level 1-100) (Đang triển khai)
    │   ├── 01-spec.md                 # Đặc tả nghiệp vụ & Acceptance Criteria
    │   ├── adr/                       # Các bản ghi quyết định kiến trúc (ADR-0001 -> ADR-0004)
    │   ├── 02-design.md               # Thiết kế kỹ thuật Hexagonal & DDL
    │   ├── 03-plan.md                 # Chiến lược triển khai theo pha
    │   └── 04-tasks.md                # Danh sách đầu việc chi tiết theo chuẩn TDD (T-01 -> T-13)
    ├── leaderboards/                  # 2. Giải Đấu Tuần 30 Người (Weekly Leagues: Đồng -> Kim Cương) (Roadmap)
    ├── streaks-and-habits/            # 3. Chuỗi Ngày & Đóng Băng Chuỗi (Streaks & Streak Freeze) (Roadmap)
    ├── fitcoins-and-ledger/           # 4. Tiền Tệ Ảo FitCoins / Gems & Sổ Cái Bất Biến (Roadmap)
    └── badges-and-achievements/       # 5. Huy Hiệu Thành Tựu & Thử Thách Hàng Tháng (Roadmap)
```

---

## 1. Mục Tiêu & Trách Nhiệm Của Module (Core Purpose)

Module **Gamification** đóng vai trò là phân hệ bổ trợ (**Supporting Domain**), phụ trách toàn bộ hệ thống động lực, ghi nhận thói quen và vinh danh nỗ lực cho gymer dựa trên các mô thức thành công của Duolingo:
* **100% Củng cố tích cực (Zero Penalties)**: Không phạt trừ điểm, không decay khi nghỉ ngơi, không tuột Cấp độ (Level). Mọi giọt mồ hôi đều tích lũy giá trị vĩnh viễn. Tuần deload phục hồi hay ngày nghỉ ốm không bao giờ bị trừng phạt.
* **Tách bạch động lực dài hạn vs. ngắn hạn**:
  * **Dài hạn**: **Lifetime XP & Level 1–100** vinh danh tổng khối lượng công sức và thời gian gắn bó trọn đời.
  * **Ngắn hạn**: **Weekly XP & Giải đấu tuần 30 người (Weekly Leagues)** tạo sân chơi cạnh tranh công bằng hàng tuần, thăng/trụ/rớt bậc giải đấu linh hoạt.
* **Hình thành thói quen kỷ luật**: Duy trì Chuỗi Ngày (Streak) hàng ngày kết hợp cơ chế Đóng Băng Chuỗi (Streak Freeze) bảo vệ gymer vào ngày nghỉ giáo án hoặc sự cố bận đột xuất.
* **Tích lũy & Trao thưởng tự do**: Tiền tệ ảo FitCoins (Gems) nhận được khi lên cấp, hoàn thành chuỗi ngày, hoặc lọt Top 3 giải đấu tuần; dùng để mua Streak Freeze, nhân đôi XP, hoặc đổi voucher đối tác.

### Trách nhiệm:
* Quản lý điểm kinh nghiệm Lifetime XP, Weekly XP và cấp độ Level (`user_xp`, `xp_history`).
* Vận hành giải đấu tuần theo bảng đấu ngẫu nhiên 30 người (Weekly Cohort Leagues).
* Quản lý chuỗi ngày tập liên tục (Streak) và trạng thái Đóng Băng Chuỗi (Streak Freeze).
* Quản lý số dư FitCoins và lịch sử sổ cái kiểm toán bất biến (`coin_ledger`).
* Đánh giá và mở khóa Huy hiệu thành tựu (Badges) cùng Thử thách độc quyền hàng tháng (Monthly Challenges).

### Không thuộc trách nhiệm:
* Không theo dõi chuyển động camera hay đếm rep (thuộc `workout_execution`).
* Không quản lý phòng đấu real-time qua WebSocket hay ghép trận đối kháng trực tiếp (thuộc `competition`).
* Không quản lý đồ thị bạn bè follow/following (thuộc `social`).

---

## 2. Các Trụ Cột Nghiệp Vụ Chính (High-Level)

### 2.1. [Hệ Thống XP & Cấp Độ Trọn Đời (Lifetime XP & Level Progression)](./features/xp-and-rank-tiers/01-spec.md)
* **Thước đo nỗ lực tuyệt đối**: Sử dụng **Điểm Kinh Nghiệm (XP)** làm đơn vị tính toán duy nhất. Tập luyện là có XP, không bao giờ bị trừ điểm.
* **Công thức XP minh bạch theo buổi tập**:
  $$\text{XP} = (\text{Base: 50} + \text{Volume: 10–30} + \text{Form: 10–20} + \text{PR: 25}) \times \text{Streak Multiplier (1.0x – 1.2x)}$$
* **Thưởng dinh dưỡng**: Nhận thêm $+30$ XP khi hoàn thành mục tiêu Calo/Protein trong ngày (không phạt khi quên log hay ăn cheat meal).
* **Cấp độ trọn đời (Level 1–100)**: Xác định theo hàm căn bậc hai $\text{Level} = \min(\lfloor \sqrt{X / 50} \rfloor + 1, 100)$ (với $X$ là `total_xp`). Cấp độ chỉ tăng không giảm, vĩnh viễn vinh danh đẳng cấp của gymer.
* Chi tiết đặc tả: xem [features/xp-and-rank-tiers/01-spec.md](./features/xp-and-rank-tiers/01-spec.md).

---

### 2.2. [Giải Đấu Tuần 30 Người (Weekly Leagues)](./features/leaderboards/01-spec.md)
* **Cơ chế Bảng đấu 30 người (30-Person Cohorts)**: Giống Duolingo, người dùng không phải cạnh tranh với toàn bộ triệu người trên hệ thống. Khi hoàn thành bài tập đầu tiên trong tuần, hệ thống xếp người dùng vào một nhóm 30 người cùng trình độ và mức độ năng động.
* **5 Bậc Giải Đấu (League Tiers)**: Đồng (Bronze) $\rightarrow$ Bạc (Silver) $\rightarrow$ Vàng (Gold) $\rightarrow$ Bạch Kim (Platinum) $\rightarrow$ Kim Cương (Diamond).
* **Chu kỳ & Thăng/Rớt Hạng**:
  * Diễn ra từ 00:00 Thứ Hai đến 23:59 Chủ Nhật (giờ địa phương). Thước đo xếp hạng là **Weekly XP**.
  * **Vùng Thăng Hạng (Promotion Zone)**: Top 7 (hoặc Top 20%) thăng lên bậc giải đấu cao hơn vào tuần sau.
  * **Vùng An Toàn (Safe Zone)**: Vị trí 8 đến 24 trụ hạng.
  * **Vùng Rớt Hạng (Demotion Zone)**: Bottom 5 (hoặc Bottom 20%) bị hạ 1 bậc giải đấu (trừ Bậc Đồng không thể rớt).
* **Thưởng Top 3**: Top 1, Top 2, Top 3 nhận cúp vinh danh và lượng FitCoins lớn khi chốt tuần.
* **Lazy Week Reset**: Tuần thi đấu được làm mới lười biếng theo từng user khi có hoạt động mới, loại bỏ nguy cơ nghẽn DB và khóa bảng diện rộng lúc 23:59 Chủ Nhật.
* Chi tiết đặc tả: xem [features/leaderboards/01-spec.md](./features/leaderboards/01-spec.md).

---

### 2.3. [Chuỗi Ngày & Đóng Băng Chuỗi (Streaks & Streak Freeze)](./features/streaks-and-habits/01-spec.md)
* **Động cơ giữ chân cốt lõi (Habit Engine)**: Đếm số ngày liên tục người dùng hoàn thành ít nhất 1 buổi tập hoặc đạt mục tiêu dinh dưỡng.
* **Bảo toàn ngày nghỉ giáo án (Scheduled Rest Day)**: Không ngắt chuỗi vào các ngày nghỉ phục hồi theo lịch của AI Coach.
* **Đóng Băng Chuỗi (Streak Freeze)**:
  * Khi người dùng lỡ một ngày không tập luyện ngoài kế hoạch (ốm, bận công việc), 1 Streak Freeze được tự động tiêu hao để bảo vệ chuỗi không bị đưa về 0.
  * Người dùng có thể dự trữ tối đa 2 Streak Freeze (mua tại Cửa hàng FitCoins hoặc nhận thưởng từ các mốc thành tựu).
* **Cột mốc Chuỗi (Streak Milestones)**: Thưởng FitCoins và kích hoạt hiệu ứng nhân đôi XP khi cán mốc 7, 14, 30, 50, 100, 365 ngày.
* Chi tiết đặc tả: xem [features/streaks-and-habits/01-spec.md](./features/streaks-and-habits/01-spec.md).

---

### 2.4. [Tiền Tệ Ảo FitCoins / Gems & Cửa Hàng (FitCoins & Shop Ledger)](./features/fitcoins-and-ledger/01-spec.md)
* **Cơ chế thu nhập (Earning)**: Thưởng FitCoins khi lên Cấp độ mới, đạt cột mốc Streak, hoặc hoàn thành tuần trong Top 3 Giải đấu.
* **Cửa Hàng Vật Phẩm (Shop)**:
  * Trang bị **Đóng Băng Chuỗi (Streak Freeze)** để phòng ngừa rủi ro bận đột xuất.
  * Mua **Tăng Tốc XP (Double XP Boost 15 phút)** trước buổi tập để tăng tốc thăng hạng giải đấu tuần.
  * Đổi lấy các voucher giảm giá đồ tập, whey protein từ các thương hiệu đối tác.
* **Kiểm soát & Sổ cái bất biến (`coin_ledger`)**: Ghi nhận toàn bộ giao dịch nạp/tiêu coin theo mô hình append-only, chống chi tiêu kép (Double Spending) và kiểm toán $100\%$ giao dịch.
* Chi tiết đặc tả: xem [features/fitcoins-and-ledger/01-spec.md](./features/fitcoins-and-ledger/01-spec.md).

---

### 2.5. [Huy Hiệu & Thử Thách Hàng Tháng (Badges & Monthly Challenges)](./features/badges-and-achievements/01-spec.md)
* **Thử Thách Hàng Tháng (Monthly Badge Challenge)**:
  * Mỗi tháng dương lịch có 1 sự kiện thử thách với Huy hiệu độc quyền (Collector Badge - ví dụ: "Chiến Binh Tháng 10").
  * Điều kiện mở khóa: Đạt 1,000 XP hoặc hoàn thành 15 buổi tập hợp lệ trong tháng.
* **Huy Hiệu Thành Tựu Đa Cấp (Tiered Badges - 1 đến 5 sao)**:
  * Huy hiệu nâng cấp dần theo các cột mốc dài hạn: Tổng khối lượng tạ tích lũy (Iron Lifter), Chuỗi ngày bền bỉ (Consistency Master), Chuẩn form kỹ thuật AI Camera (Technique Virtuoso).
* **Nhiệm Vụ Hàng Ngày (Daily Quests)**: 3 nhiệm vụ ngắn mỗi ngày (hoàn thành 1 bài tập, ghi nhật ký ăn uống, đạt 50 XP) tặng thêm FitCoins và XP.
* Chi tiết đặc tả: xem [features/badges-and-achievements/01-spec.md](./features/badges-and-achievements/01-spec.md).

---

## 3. Kiến Trúc Kỹ Thuật

Xem chi tiết tại: **[architecture.md](./architecture.md)**
* **Mô hình Hexagonal (Ports & Adapters)**: Bóc tách 4 tầng Domain, Application, Infrastructure, Transport.
* **PostgreSQL Schema cô lập**: Schema riêng `gamification.*`, không JOIN chéo module khác.
* **Kafka Event Bus & Outbox**: Giao tiếp bất đồng bộ qua CloudEvents 1.0 JSON và Outbox Pattern.
* **Tính lũy đẳng (Idempotency)**: Enforce qua khóa duy nhất nghiệp vụ (`uq_xp_history_workout_session` trên bảng `xp_history`), loại bỏ hoàn toàn nguy cơ cộng điểm trùng lặp.

---

## 4. Bảng Thuật Ngữ Nghiệp Vụ (Glossary)

| Thuật ngữ (English) | Thuật ngữ (Tiếng Việt) | Định nghĩa & Ý nghĩa nghiệp vụ chuẩn Duolingo |
| :--- | :--- | :--- |
| **Total XP (`total_xp`)** | Kinh Nghiệm Trọn Đời | Thước đo tích lũy nỗ lực vĩnh viễn, 100% củng cố tích cực, không bao giờ bị trừ hay suy giảm theo thời gian. |
| **Weekly XP (`weekly_xp`)** | Kinh Nghiệm Tuần | Điểm kinh nghiệm tích lũy trong tuần thi đấu hiện tại (00:00 T2 $\rightarrow$ 23:59 CN), là thước đo duy nhất để xếp hạng Giải Đấu Tuần. |
| **Level (`level`)** | Cấp Độ Trọn Đời | Cấp độ từ 1 đến 100, xác định động theo căn bậc hai của Total XP: $\lfloor \sqrt{X / 50} \rfloor + 1$ (với $X$ là `total_xp`). Cấp độ chỉ tăng không giảm. |
| **Weekly League** | Giải Đấu Tuần | Giải đấu hàng tuần nhóm 30 người (Cohort of 30) với 5 bậc: Đồng (Bronze), Bạc (Silver), Vàng (Gold), Bạch Kim (Platinum), Kim Cương (Diamond). |
| **Promotion Zone** | Vùng Thăng Hạng | Top 7 người đứng đầu bảng đấu 30 người (Top ~20%) sẽ thăng lên bậc giải đấu cao hơn vào tuần tiếp theo. |
| **Demotion Zone** | Vùng Rớt Hạng | Bottom 5 người cuối bảng đấu 30 người (Bottom ~20%) sẽ bị hạ 1 bậc giải đấu vào tuần tiếp theo (bậc Đồng không bị rớt). |
| **Safe Zone** | Vùng An Toàn | Các vị trí giữa bảng đấu (hạng 8 đến 24), tiếp tục duy trì bậc giải đấu hiện tại vào tuần sau. |
| **Streak** | Chuỗi Ngày Tập Luyện | Số ngày liên tiếp người dùng hoàn thành buổi tập hoặc đạt mục tiêu dinh dưỡng theo múi giờ địa phương (`user_local_date`). |
| **Streak Freeze** | Đóng Băng Chuỗi | Vật phẩm tự động tiêu hao để bảo vệ chuỗi ngày không bị về 0 khi người dùng lỡ một ngày tập/log dinh dưỡng ngoài kế hoạch (tối đa dự trữ 2 khiên). |
| **FitCoins (Gems)** | Tiền Tệ Thể Thao Ảo | Đơn vị tiền tệ thưởng khi lên cấp, hoàn thành chuỗi streak, hoặc lọt Top 3 giải đấu; dùng tại Cửa hàng để mua Streak Freeze, XP Boost hoặc voucher. |
| **Coin Ledger (`coin_ledger`)** | Sổ Cái Kiểm Toán Bất Biến | Bảng lưu trữ append-only ghi nhận $100\%$ các giao dịch thu/chi coin kèm số dư lũy kế (`running_balance`). Nghiêm cấm sửa hoặc xóa bản ghi cũ. |
| **Monthly Challenge** | Thử Thách Hàng Tháng | Sự kiện kéo dài 1 tháng dương lịch, hoàn thành mục tiêu XP hoặc số buổi tập để mở khóa Huy hiệu sưu tầm độc quyền của tháng. |
| **Daily Quests** | Nhiệm Vụ Hàng Ngày | 3 nhiệm vụ ngắn làm mới mỗi ngày (tập luyện, dinh dưỡng, đạt mốc XP) tặng thêm FitCoins và XP. |
| **Lazy Week Reset** | Làm Mới Tuần Lười Biếng | Cơ chế reset `weekly_xp = 0` được thực hiện trực tiếp trong transaction của hoạt động đầu tiên của user trong tuần mới, tránh cron job batch lock toàn DB lúc nửa đêm. |
| **Proof of Workout** | Tiêu Chuẩn Buổi Tập Hợp Lệ | Điều kiện tối thiểu để buổi tập được xét thưởng XP và FitCoins: thời lượng $\ge 15$ phút, số hiệp $\ge 3$, và không bị cờ `Anomalous Session`. |
