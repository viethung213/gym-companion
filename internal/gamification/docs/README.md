# Bounded Context: Gamification Engine — Gym Companion

Tài liệu định nghĩa nghiệp vụ và ranh giới hệ thống của module **Gamification** (`/internal/gamification/`) trong nền tảng FITAI.

```text
internal/gamification/docs/
├── README.md                          # Định nghĩa tổng quan module & các trụ cột nghiệp vụ (High-level)
├── architecture.md                    # Thiết kế kiến trúc kỹ thuật Hexagonal, Database & Event Bus
└── features/                          # Đặc tả chi tiết từng tính năng (Feature Specifications)
    ├── elo-and-rank-tiers/            # 1. Hệ Thống Xếp Hạng ELO & Bậc Hạng (Đang triển khai)
    │   ├── 01-spec.md                 # Đặc tả nghiệp vụ & Acceptance Criteria
    │   ├── adr/                       # Các bản ghi quyết định kiến trúc (ADR-0001 -> ADR-0005)
    │   ├── 02-design.md               # Thiết kế kỹ thuật Hexagonal & DDL
    │   ├── 03-plan.md                 # Chiến lược triển khai theo pha
    │   └── 04-tasks.md                # Danh sách đầu việc chi tiết theo chuẩn TDD (T-01 -> T-13)
    ├── fitcoins-and-ledger/           # 2. Tiền Tệ Thể Thao FitCoins & Sổ Cái Bất Biến (Roadmap)
    ├── badges-and-achievements/       # 3. Bộ Huy Hiệu Thành Tựu (Roadmap)
    ├── streaks-and-habits/            # 4. Chuỗi Ngày Tập Thích Ứng & Ngày Nghỉ (Roadmap)
    └── leaderboards/                  # 5. Động Cơ Bảng Xếp Hạng (Roadmap)
```

---

## 1. Mục Tiêu & Trách Nhiệm Của Module (Core Purpose)

Module **Gamification** đóng vai trò là phân hệ bổ trợ (**Supporting Domain**), phụ trách toàn bộ hệ thống động lực, ghi nhận thành tựu và xếp hạng thể chất cho gymer:
* **Tạo động lực ngắn hạn**: Khắc phục tỷ lệ bỏ cuộc của người mới tập gym bằng phản hồi tích cực và ghi nhận thành tựu sau mỗi buổi tập.
* **Xếp hạng thể chất trực quan**: Sử dụng **Điểm ELO (`elo_rating`)** làm thước đo năng lực duy nhất, loại bỏ sự phức tạp của các bảng đấu chia nhóm hay cron job chốt tuần.
* **Tích lũy & Trao thưởng**: Hệ thống tiền tệ ảo FitCoins và huy hiệu Badges để vinh danh sự kiên trì.

### Trách nhiệm:
* Quản lý số dư FitCoins và lịch sử sổ cái kiểm toán bất biến (`coin_ledger`).
* Tính toán biến động điểm ELO và xác định Bậc Hạng (Rank Tier) tức thì.
* Đánh giá và mở khóa Huy hiệu thành tựu (Badges).
* Cung cấp dữ liệu Bảng xếp hạng ELO (Toàn cầu & Bạn bè).

### Không thuộc trách nhiệm:
* Không theo dõi chuyển động camera hay đếm rep (thuộc `workout_execution`).
* Không quản lý phòng đấu real-time qua WebSocket hay ghép trận PvP (thuộc `competition`).
* Không quản lý đồ thị bạn bè follow/following (thuộc `social`).

---

## 2. Các Trụ Cột Nghiệp Vụ Chính (High-Level)

### 2.1. [Hệ Thống Xếp Hạng ELO (Rank Tiers)](./features/elo-and-rank-tiers/01-spec.md)
* **Thước đo duy nhất**: Sử dụng điểm **ELO (`elo_rating`)** để phản ánh tổng hòa thể lực, tính kỷ luật và chất lượng kỹ thuật của người tập.
* **Thăng bậc tức thì**: Bậc hạng (Đồng, Bạc, Vàng, Bạch Kim, Kim Cương) được xác định tự động theo mốc điểm ELO; người dùng thăng hạng ngay khi đạt điểm mà không cần chờ đợi chu kỳ tuần.
* **Cơ chế biến động**: 
  * Tăng điểm qua quá trình tập luyện hàng ngày (hoàn thành giáo án, chất lượng động tác chuẩn xác, kỷ lục cá nhân).
  * Điều chỉnh điểm qua các trận đấu đối kháng (PvP) khi tích hợp với phân hệ đấu trường sau này.
* Chi tiết đặc tả: xem [features/elo-and-rank-tiers/01-spec.md](./features/elo-and-rank-tiers/01-spec.md).

---

### 2.2. [Bộ Huy Hiệu Thành Tựu (Badges)](./features/badges-and-achievements/01-spec.md)
Tự động kích hoạt qua các sự kiện tập luyện và hoạt động trên app, chia làm 3 nhóm cột mốc:
* **Khám phá tính năng (Onboarding)**: Ghi nhận lần đầu trải nghiệm các tính năng cốt lõi (AI Camera, nhật ký dinh dưỡng, chia sẻ mạng xã hội).
* **Bền bỉ & Thâm niên (Dedication & Streak)**: Tôn vinh chuỗi ngày tập liên tục và thời gian gắn bó lâu dài.
* **Cột mốc số hiệp tạ (Set Milestones)**: Ghi nhận tổng khối lượng và số lượng hiệp tập đã tích lũy.
* Chi tiết đặc tả: xem [features/badges-and-achievements/01-spec.md](./features/badges-and-achievements/01-spec.md).

---

### 2.3. [Tiền Tệ Thể Thao (FitCoins & Ledger)](./features/fitcoins-and-ledger/01-spec.md)
* **Tích lũy**: Thưởng coin khi hoàn thành buổi tập hợp lệ, duy trì streak và mở khóa huy hiệu mới.
* **Kiểm soát & Chống gian lận**: Áp dụng hạn mức trần hàng ngày và tiêu chuẩn buổi tập tối thiểu nhằm ngăn chặn hành vi farm coin ảo.
* **Tiêu dùng & Sổ cái**: Dùng coin để đổi các ưu đãi từ đối tác thương hiệu (Brand); bảo vệ bằng sổ cái bất biến và khóa chống chi tiêu kép.
* Chi tiết đặc tả: xem [features/fitcoins-and-ledger/01-spec.md](./features/fitcoins-and-ledger/01-spec.md).

---

### 2.4. [Chuỗi Ngày Tập Thích Ứng (Adaptive Streaks)](./features/streaks-and-habits/01-spec.md)
* **Bảo toàn ngày nghỉ (Rest Day Preservation)**: Không phạt mất chuỗi khi người dùng nghỉ theo đúng giáo án phục hồi của AI Coach.
* **Khiên phục hồi (Rest Shield)**: Cấp 2 khiên nghỉ tự do mỗi tháng để chống gãy chuỗi đột xuất.
* Chi tiết đặc tả: xem [features/streaks-and-habits/01-spec.md](./features/streaks-and-habits/01-spec.md).

---

### 2.5. [Bảng Xếp Hạng (Leaderboard Engine)](./features/leaderboards/01-spec.md)
* Hỗ trợ xem xếp hạng **Toàn cầu** và **Bạn bè** trực tiếp theo thứ tự điểm ELO.
* Tối giản hóa truy vấn: Hệ thống chỉ sắp xếp theo điểm số, giao diện tự động ánh xạ thứ tự hiển thị mà không cần tác vụ tính toán ngầm định kỳ.
* Chi tiết đặc tả: xem [features/leaderboards/01-spec.md](./features/leaderboards/01-spec.md).

---

## 3. Kiến Trúc Kỹ Thuật

Xem chi tiết tại: **[architecture.md](./architecture.md)**
* **Mô hình Hexagonal (Ports & Adapters)**: Bóc tách 4 tầng Domain, Application, Infrastructure, Transport.
* **PostgreSQL Schema cô lập**: Schema riêng `gamification.*`, không JOIN chéo module khác.
* **Kafka Event Bus & Outbox**: Giao tiếp bất đồng bộ qua CloudEvents 1.0 JSON và Outbox Pattern.

---

## 4. Bảng Thuật Ngữ Nghiệp Vụ (Glossary)

| Thuật ngữ (English) | Thuật ngữ (Tiếng Việt) | Định nghĩa & Ý nghĩa nghiệp vụ |
| :--- | :--- | :--- |
| **ELO Rating (`elo_rating`)** | Điểm ELO thể chất | Thước đo năng lực chuẩn hóa duy nhất phản ánh tổng hòa thể lực, kỹ thuật và tính kỷ luật của gymer. Khởi đầu ở mức 1,000 điểm. |
| **Rank Tier** | Bậc Hạng | Phân cấp trình độ gồm 5 bậc cố định: Đồng (Bronze), Bạc (Silver), Vàng (Gold), Bạch Kim (Platinum), Kim Cương (Diamond). |
| **Inactivity Decay** | Suy Giảm Bất Hoạt | Cơ chế tự động trừ 15 ELO mỗi 7 ngày nếu người dùng không tập luyện quá 14 ngày liên tiếp (chỉ áp dụng từ Bạc trở lên, không trừ dưới sàn 1,000). |
| **Nutrition Compliance Bonus** | Thưởng Kỷ Luật Dinh Dưỡng | Điểm ELO thưởng dương ($+3 \rightarrow +5$ ELO/ngày) khi đạt mục tiêu Calo và Protein; tuyệt đối không phạt trừ ELO khi ăn sai/quên ghi log. |
| **FitCoins** | Tiền tệ thể thao ảo | Đơn vị tiền tệ thưởng cho sự nỗ lực tập luyện, dùng để đổi các ưu đãi/voucher từ đối tác thương hiệu (Brand) hoặc vật phẩm cá nhân hóa. |
| **Coin Ledger (`coin_ledger`)** | Sổ Cái Kiểm Toán Bất Biến | Bảng lưu trữ append-only ghi nhận $100\%$ các giao dịch thu/chi coin kèm số dư lũy kế (`running_balance`). Nghiêm cấm sửa hoặc xóa bản ghi cũ. |
| **Daily Earning Cap** | Trần Thu Nhập Hàng Ngày | Hạn mức trần tối đa 100 FitCoins/ngày từ hoạt động tập luyện nhằm ngăn chặn hành vi farm coin ảo và gian lận bào voucher đối tác. |
| **Proof of Workout** | Tiêu Chuẩn Buổi Tập Đạt Chuẩn | Điều kiện tối thiểu để buổi tập được xét thưởng ELO và coin: thời lượng $\ge 15$ phút, số hiệp $\ge 3$, và không bị cờ `Anomalous Session`. |
| **Adaptive Streak** | Chuỗi Ngày Tập Thích Ứng | Số ngày tập luyện liên tục được tính theo giờ địa phương (`user_local_date`), tự động bảo lưu không bị ngắt quãng vào Ngày Nghỉ theo giáo án (`SCHEDULED_REST_DAY`). |
| **Rest Shield / Grace Shield** | Khiên Phục Hồi | Khiên tự động cứu chuỗi (tối đa 2 khiên/tháng) khi người dùng nghỉ đột xuất không theo kế hoạch; tự động làm mới vào ngày 1 hàng tháng. |
| **Badge** | Huy Hiệu Thành Tựu | Phần thưởng danh dự mở khóa một lần duy nhất khi người dùng chinh phục các cột mốc Onboarding, Bền bỉ, hoặc Khối lượng tạ (Volume). |
| **Percentile Standing** | Thứ Hạng Phân Vị | Cách biểu diễn vị trí tương đối dạng phần trăm (ví dụ: Top 10% Bậc Bạc) cho người dùng ngoài Top 1,000 nhằm loại bỏ truy vấn quét toàn bảng gây sập DB. |
