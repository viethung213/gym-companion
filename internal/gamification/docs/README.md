# Bounded Context: Gamification Engine — Gym Companion

Tài liệu định nghĩa nghiệp vụ và ranh giới hệ thống của module **Gamification** (`/internal/gamification/`) trong nền tảng FITAI.

Module này áp dụng triết lý Gamification thực chứng chuẩn **Duolingo** cho thể hình: **100% củng cố tích cực (Positive Reinforcement)**, xóa bỏ hoàn toàn cơ chế phạt/trừ điểm/tuột cấp, tạo động lực bền vững qua Hệ Thống XP & Cấp Độ Trọn Đời (XP & Levels), Bảng Xếp Hạng Toàn Hệ Thống (XP Leaderboard), Chuỗi Ngày (Streaks) và Đóng Băng Chuỗi (Streak Freeze).

```text
internal/gamification/docs/
├── README.md                          # Định nghĩa tổng quan module & các trụ cột nghiệp vụ chuẩn Duolingo
├── architecture.md                    # Thiết kế kiến trúc kỹ thuật Hexagonal, Database & Event Bus
└── features/                          # Đặc tả chi tiết từng tính năng (Feature Specifications)
    ├── xp-and-levels/                 # 1. Hệ Thống XP & Cấp Độ Trọn Đời (XP & Level 1-100) (Đang triển khai)
    │   ├── 01-spec.md                 # Đặc tả nghiệp vụ & Acceptance Criteria
    │   ├── adr/                       # Các bản ghi quyết định kiến trúc (ADR-0001 -> ADR-0003)
    │   ├── 02-design.md               # Thiết kế kỹ thuật Hexagonal & DDL
    │   ├── 03-plan.md                 # Chiến lược triển khai theo pha
    │   └── 04-tasks.md                # Danh sách đầu việc chi tiết theo chuẩn TDD (T-01 -> T-13)
    ├── leaderboards/                  # 2. Bảng Xếp Hạng Toàn Hệ Thống Theo Điểm XP (Roadmap)
    ├── streaks-and-habits/            # 3. Chuỗi Ngày & Đóng Băng Chuỗi (Streaks & Streak Freeze) (Roadmap)
    └── fitcoins-and-ledger/           # 4. Tiền Tệ Ảo FitCoins / Gems & Sổ Cái Bất Biến (Roadmap)
```

---

## 1. Mục Tiêu & Trách Nhiệm Của Module (Core Purpose)

Module **Gamification** đóng vai trò là phân hệ bổ trợ (**Supporting Domain**), phụ trách toàn bộ hệ thống động lực, ghi nhận thói quen và vinh danh nỗ lực cho gymer dựa trên các mô thức thành công của Duolingo:
* **100% Củng cố tích cực (Zero Penalties)**: Không phạt trừ điểm, không decay khi nghỉ ngơi, không tuột Cấp độ (Level). Mọi giọt mồ hôi đều tích lũy giá trị vĩnh viễn. Tuần deload phục hồi hay ngày nghỉ ốm không bao giờ bị trừng phạt.
* **Tích lũy dài hạn & Vinh danh minh bạch**:
  * **Cấp độ trọn đời**: **XP & Level 1–100** vinh danh tổng khối lượng công sức và thời gian gắn bó trọn đời.
  * **Bảng xếp hạng toàn hệ thống**: **XP Leaderboard** hiển thị Top 50 gymer bền bỉ nhất nền tảng và vị trí cá nhân theo thời gian thực.
* **Hình thành thói quen kỷ luật**: Duy trì Chuỗi Ngày (Streak) hàng ngày kết hợp cơ chế Đóng Băng Chuỗi (Streak Freeze) bảo vệ gymer vào ngày nghỉ giáo án hoặc sự cố bận đột xuất.
* **Tích lũy & Trao thưởng tự do**: Tiền tệ ảo FitCoins (Gems) nhận được khi lên cấp hoặc hoàn thành chuỗi ngày; dùng để mua Streak Freeze, nhân đôi XP, hoặc đổi voucher đối tác.

### Trách nhiệm:
* Quản lý điểm kinh nghiệm XP và cấp độ Level (`user_xp`, `xp_history`).
* Cung cấp dữ liệu Bảng xếp hạng toàn hệ thống theo điểm XP (Top 50) và tra cứu thứ hạng cá nhân (`my_rank`).
* Quản lý chuỗi ngày tập liên tục (Streak) và trạng thái Đóng Băng Chuỗi (Streak Freeze).
* Quản lý số dư FitCoins và lịch sử sổ cái kiểm toán bất biến (`coin_ledger`).

### Không thuộc trách nhiệm:
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

### 2.2. Bảng Xếp Hạng Toàn Hệ Thống Theo Điểm XP (XP Leaderboard)
* **Thước đo duy nhất**: Toàn bộ thứ hạng được tính toán dựa trên **Điểm Kinh Nghiệm Trọn Đời (`xp`)**.
* **Tối giản & Trực quan**: Loại bỏ sự phức tạp của việc chia phòng đấu hay reset tuần; ai tích lũy nỗ lực nhiều hơn sẽ đứng cao hơn.
* **Bảng vàng danh dự**:
  * Hiển thị danh sách **Top 50** gymer có điểm tích lũy cao nhất toàn nền tảng.
  * Hiển thị chính xác **Vị trí cá nhân (My Standing)** và khoảng cách điểm số, bất kể người dùng nằm trong hay ngoài Top 50.
* **Hiệu năng & Thời gian thực (The 3 AM Test)**:
  * Không sử dụng cron job ngầm hay worker chốt giải đấu; thứ hạng cập nhật tức thì theo thời gian thực mỗi khi có buổi tập mới.
  * Tối ưu hóa bằng index B-tree `(xp DESC, updated_at ASC)` trên bảng `gamification.user_xp`, đảm bảo thời gian truy vấn $< 5\text{ ms}$.
* Chi tiết đặc tả: Phân tách thành lớp tính năng độc lập (Stack PR) kế tiếp.

---

### 2.3. Chuỗi Ngày Kỷ Luật (Streaks Engine)
* **Động cơ giữ chân cốt lõi (Habit Engine)**: Đếm số ngày liên tục người dùng hoàn thành ít nhất 1 buổi tập hoặc đạt mục tiêu dinh dưỡng.
* **Kỷ luật thực chất (Workout & Nutrition)**: Ngày tập thì nâng tạ, ngày nghỉ thì log ăn uống đúng chuẩn Calo/Protein để giữ chuỗi liên tục.
* **Cột mốc Chuỗi (Streak Milestones)**: Thưởng FitCoins khi cán mốc 7, 14, 30, 50, 100, 365 ngày liên tục.
* Chi tiết đặc tả: Phân tách thành lớp tính năng độc lập (Stack PR) kế tiếp.

---

### 2.4. [Tiền Tệ Ảo FitCoins / Gems & Cửa Hàng (FitCoins & Shop Ledger)](./features/fitcoins-and-ledger/01-spec.md)
* **Cơ chế thu nhập (Earning)**: Thưởng FitCoins khi lên Cấp độ mới, đạt cột mốc Streak, hoặc đạt thứ hạng cao trên Bảng xếp hạng.
* **Cửa Hàng Vật Phẩm (Shop Roadmap)**:
  * Mua **Tăng Tốc XP (Double XP Boost 15 phút)** trước buổi tập để tăng tốc thăng cấp và leo hạng.
  * Đổi lấy các voucher giảm giá đồ tập, whey protein từ các thương hiệu đối tác.
* **Kiểm soát & Sổ cái bất biến (`coin_ledger`)**: Ghi nhận toàn bộ giao dịch nạp/tiêu coin theo mô hình append-only, chống chi tiêu kép (Double Spending) và kiểm toán $100\%$ giao dịch.
* Chi tiết đặc tả: xem [features/fitcoins-and-ledger/01-spec.md](./features/fitcoins-and-ledger/01-spec.md).

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
| **XP (`xp`)** | Điểm Kinh Nghiệm | Thước đo tích lũy nỗ lực vĩnh viễn, 100% củng cố tích cực, không bao giờ bị trừ hay suy giảm theo thời gian. |
| **Level (`level`)** | Cấp Độ Trọn Đời | Cấp độ từ 1 đến 100, xác định động theo căn bậc hai của XP: $\lfloor \sqrt{X / 50} \rfloor + 1$ (với $X$ là `xp`). Cấp độ chỉ tăng không giảm. |
| **Leaderboard** | Bảng Xếp Hạng Toàn Hệ Thống | Bảng vinh danh Top 50 gymer có số điểm `xp` cao nhất nền tảng và vị trí cá nhân theo thời gian thực. |
| **Streak** | Chuỗi Ngày Tập Luyện | Số ngày liên tiếp người dùng hoàn thành buổi tập hoặc đạt mục tiêu dinh dưỡng theo múi giờ địa phương (`user_local_date`). |
| **FitCoins (Gems)** | Tiền Tệ Thể Thao Ảo | Đơn vị tiền tệ thưởng khi lên cấp, hoàn thành chuỗi streak; dùng tại Cửa hàng để mua XP Boost hoặc voucher. |
| **Coin Ledger (`coin_ledger`)** | Sổ Cái Kiểm Toán Bất Biến | Bảng lưu trữ append-only ghi nhận $100\%$ các giao dịch thu/chi coin kèm số dư lũy kế (`running_balance`). Nghiêm cấm sửa hoặc xóa bản ghi cũ. |
| **Proof of Workout** | Tiêu Chuẩn Buổi Tập Hợp Lệ | Điều kiện tối thiểu để buổi tập được xét thưởng XP và FitCoins: thời lượng $\ge 15$ phút, số hiệp $\ge 3$, và không bị cờ `Anomalous Session`. |
