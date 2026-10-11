# Feature: Tiền Tệ Ảo FitCoins & Sổ Cái Bất Biến (FitCoins & Shop Ledger)

## Summary
Tính năng **Tiền Tệ Ảo FitCoins & Sổ Cái Bất Biến (FitCoins & Shop Ledger)** thiết lập nền kinh tế ảo nội bộ cho hệ thống FITAI:
- Cung cấp đơn vị tiền tệ thể thao **FitCoins (Gems)** dùng để vinh danh gymer và trao quyền tự do quy đổi tiện ích.
- Vận hành ví tiền theo mô hình **Sổ cái kiểm toán bất biến (Immutable Audit Ledger - Append-Only)**: $100\%$ giao dịch thu/chi coin được lưu vết vĩnh viễn, chống chi tiêu kép (Double Spending), chống số dư âm.
- Cơ chế nhận coin (Earning): Thưởng tự động khi người dùng thăng cấp Cấp độ trọn đời (`UserLeveledUp`).
- Cơ chế tiêu coin (Shop / Spending): Mua vật phẩm bổ trợ tập luyện tại Cửa hàng (Đóng Băng Chuỗi - Streak Freeze, Tăng Tốc XP - Double XP Boost).

---

## Problem and Desired Outcome

### Problem
- Nếu chỉ có điểm kinh nghiệm (XP) và Cấp độ, người dùng dễ cảm thấy thành tích bị trừu tượng hóa, thiếu động lực ngắn hạn để duy trì tập luyện đều đặn.
- Trong ứng dụng thể hình, người dùng gặp rủi ro gãy chuỗi (Streak) do các tình huống đột xuất (ốm đau, công tác, lịch thi cử) nhưng không có công cụ dự phòng để bảo vệ thành quả.
- Các hệ thống ví tiền ảo nếu thiết kế cẩu thả (chỉ lưu 1 cột `balance` và cộng/trừ trực tiếp) thường đối mặt với các lỗi vận hành nghiêm trọng:
  - **Chi tiêu kép (Double Spending)**: Gửi 2 request mua hàng đồng thời dẫn đến trừ tiền sai hoặc mua được 2 món với số dư của 1 món.
  - **Số dư âm (Negative Balance)**: Tài khoản bị trừ tiền dưới 0 do thiếu ràng buộc ACID.
  - **Mất dấu vết kiểm toán (No Audit Trail)**: Không thể tra cứu nguyên nhân vì sao người dùng bị trừ hoặc cộng coin khi có khiếu nại.

### Desired Outcome
- **Tạo động lực linh hoạt**: Gymer có thêm mục tiêu tích lũy coin để tự mua các vật phẩm bảo vệ chuỗi ngày và gia tăng trải nghiệm.
- **Bảo mật & Toàn vẹn tuyệt đối**: 
  - Toàn bộ giao dịch thu/chi được ghi nhận append-only vào bảng `coin_ledger`, không bao giờ được phép sửa/xóa dòng cũ.
  - Ràng buộc cứng `CHECK (balance >= 0)` và cơ chế khóa dòng bi quan `SELECT ... FOR UPDATE` bảo đảm triệt tiêu hoàn toàn chi tiêu kép và số dư âm.
- **Tính lũy đẳng (Idempotency)**: Đảm bảo mạng chập chờn hay người dùng bấm 2 lần nút "Mua" không bao giờ gây trừ tiền 2 lần nhờ `idempotency_key`.

---

## Actors and Entry Points

- **Gym User (Mobile App / Web Client)**:
  - Xem số dư ví hiện tại và lịch sử giao dịch: RPC `GetWalletBalance`, `ListLedgerTransactions`.
  - Mua vật phẩm tại Cửa hàng (Shop): RPC `PurchaseShopItem`.
- **System Event Consumers (Kafka)**:
  - `UserLeveledUpConsumer`: Lắng nghe sự kiện `contracts.gamification.xp.v1.UserLeveledUp` $\rightarrow$ Tự động cộng FitCoins thưởng lên cấp vào ví người dùng.

---

## Scope

### In Scope
- Quản lý số dư ví người dùng (`gamification.user_wallet`).
- Sổ cái kiểm toán bất biến ghi nhận $100\%$ giao dịch thu/chi (`gamification.coin_ledger`).
- Nghiệp vụ cộng coin thưởng khi Lên Cấp (`REASON_LEVEL_UP`).
- Nghiệp vụ trừ coin khi Mua Vật Phẩm Cửa Hàng (`PURCHASE_STREAK_FREEZE`, `PURCHASE_XP_BOOST`).
- Cơ chế kiểm soát giới hạn kho vật phẩm (ví dụ: Streak Freeze tối đa sở hữu 2 khiên).
- Đảm bảo tính lũy đẳng (Idempotency Key) cho mọi giao dịch ghi điểm/trừ điểm.
- API ConnectRPC Protobuf: `GetWalletBalance`, `ListLedgerTransactions`, `PurchaseShopItem`.

### Out of Scope & Explicitly Deferred
- **Nạp tiền thật mua FitCoins (In-App Purchases / Fiat Payment) [DEFERRED]**: FitCoins hiện tại chỉ kiếm được qua nỗ lực tập luyện (Proof of Workout), chưa mở bán bằng tiền thật.
- **Chuyển tiền giữa người dùng (P2P Coin Transfer) [DEFERRED]**: Không hỗ trợ chuyển coin giữa các tài khoản để ngăn ngừa rửa tiền, cày clone và gian lận.
- **Đổi Voucher đối tác thương hiệu (Partner Brand Vouchers) [DEFERRED]**: Sẽ mở rộng trong giai đoạn sau khi kết nối với cổng quà tặng thương hiệu.

---

## Current vs Desired Behavior

- **Current Behavior**: Khi người dùng nhận XP và lên cấp độ mới, sự kiện `UserLeveledUp` được bắn ra nhưng chưa có phân hệ ví để trao thưởng hay ghi nhận tiền tệ.
- **Desired Behavior**: 
  - Khi lên cấp, người dùng được cộng tự động $+20$ FitCoins vào ví, lưu vết rõ ràng trong sổ cái.
  - Người dùng có thể vào Cửa Hàng mua Streak Freeze (100 FitCoins) để tích trữ khiên bảo vệ chuỗi ngày tập luyện.

---

## User Scenarios / Use Cases

### UC-FC-01: Nhận Thưởng FitCoins Tự Động Khi Thăng Cấp (Level Up Earning)
- **Actor**: `UserLeveledUpConsumer` (Kafka Background Consumer).
- **Preconditions**: Người dùng vừa hoàn thành bài tập hoặc dinh dưỡng, nâng điểm XP khiến Level tăng từ $L_{old}$ lên $L_{new}$.
- **Trigger**: Nhận CloudEvent `contracts.gamification.xp.v1.UserLeveledUp`.
- **Main Flow**:
  1. Consumer trích xuất `user_id`, `new_level`, `event_id`.
  2. Tạo `idempotency_key = "level_up:" + event_id`.
  3. Bắt đầu transaction cơ sở dữ liệu:
     - Khóa dòng ví người dùng `SELECT balance FROM gamification.user_wallet WHERE user_id = :id FOR UPDATE`. (Nếu chưa có ví, tự động khởi tạo với balance = 0).
     - Kiểm tra trong `coin_ledger` xem `idempotency_key` đã tồn tại chưa: nếu đã có thì bỏ qua (Idempotent success).
     - Xác định số coin thưởng (Mặc định: $+20$ FitCoins).
     - Cập nhật số dư ví: `new_balance = balance + 20`.
     - Ghi 1 dòng vào `coin_ledger` với `amount = +20`, `balance_after = new_balance`, `reason = REASON_LEVEL_UP`, `idempotency_key`.
  4. Commit transaction.
- **Postconditions**: Người dùng nhận được coin thưởng, số dư ví tăng thêm 20, có bản ghi kiểm toán bất biến.

---

### UC-FC-02: Mua Vật Phẩm Tại Cửa Hàng (Purchase Shop Item)
- **Actor**: Gym User (Đã xác thực Bearer Token).
- **Preconditions**: Người dùng có đủ số dư FitCoins và chưa đạt giới hạn trần của vật phẩm muốn mua.
- **Trigger**: Người dùng chọn mua vật phẩm (ví dụ: "Streak Freeze" giá 100 coin) trên ứng dụng và nhấn Xác nhận.
- **Main Flow**:
  1. Client gửi request `PurchaseShopItem(item_type = "ITEM_STREAK_FREEZE", idempotency_key = "uuid-client")`.
  2. Hệ thống kiểm tra tính hợp lệ của vật phẩm:
     - Giá vật phẩm: $100$ FitCoins.
     - Kiểm tra kho đồ người dùng: nếu số lượng Streak Freeze hiện tại $\ge 2$, trả về lỗi `ERR_MAX_INVENTORY_REACHED` (tối đa dự trữ 2 khiên).
  3. Bắt đầu transaction cơ sở dữ liệu:
     - Khóa dòng ví: `SELECT balance FROM gamification.user_wallet WHERE user_id = :id FOR UPDATE`.
     - Kiểm tra số dư: Nếu `balance < 100`, rollback và trả về lỗi `ERR_INSUFFICIENT_FUNDS`.
     - Trừ số dư ví: `new_balance = balance - 100`.
     - Ghi bản ghi vào `coin_ledger`: `amount = -100`, `balance_after = new_balance`, `reason = REASON_PURCHASE_STREAK_FREEZE`, `idempotency_key`.
     - Cộng 1 Streak Freeze vào kho đồ người dùng (`user_inventory` hoặc `streak_status`).
  4. Commit transaction.
  5. Trả về response `200 OK` kèm `new_balance` và thông tin vật phẩm đã nhận.
- **Alternate Flow (Không Đủ Tiền)**:
  - Nếu `balance < item_cost`: Hệ thống không thay đổi dữ liệu, trả về mã lỗi `FAILED_PRECONDITION: Số dư FitCoins không đủ để thực hiện giao dịch`.

---

### UC-FC-03: Xem Số Dư Ví Và Lịch Sử Giao Dịch
- **Actor**: Gym User (Đã xác thực Bearer Token).
- **Trigger**: Người dùng mở màn hình "Ví FitCoins & Cửa Hàng".
- **Main Flow**:
  1. Client gửi request `GetWalletBalance()`.
  2. Hệ thống đọc bản ghi `user_wallet` của người dùng, trả về `balance`.
  3. Client gửi request `ListLedgerTransactions(page_size = 20, cursor = ...)`.
  4. Hệ thống đọc từ `coin_ledger WHERE user_id = :id ORDER BY created_at DESC LIMIT 20`.
  5. Trả về danh sách chi tiết các dòng thu/chi (thời gian, số coin, lý do, số dư sau giao dịch).

---

## Functional Requirements

- **FR-FC-01**: Cung cấp ví lưu trữ số dư FitCoins hiện tại (`balance`) cho mỗi người dùng, khởi tạo mặc định bằng $0$ khi tài khoản mới tạo.
- **FR-FC-02**: Mọi biến động tăng hoặc giảm coin bắt buộc phải ghi một bản ghi bất biến tương ứng vào `coin_ledger`.
- **FR-FC-03**: Hệ thống nghiêm cấm mọi hành vi cập nhật (`UPDATE`) hoặc xóa (`DELETE`) dữ liệu trên bảng `coin_ledger` (Append-Only Enforcement).
- **FR-FC-04**: Hệ thống phải bảo đảm số dư ví không bao giờ âm (`balance >= 0`). Nếu giao dịch trừ coin khiến số dư $< 0$, hệ thống bắt buộc phải từ chối và rollback toàn bộ transaction.
- **FR-FC-05**: Hỗ trợ cơ chế Idempotency Key cho toàn bộ các thao tác cộng và trừ coin để chống duplicate giao dịch.
- **FR-FC-06**: Tự động cộng coin thưởng khi nhận sự kiện thăng cấp `UserLeveledUp`.
- **FR-FC-07**: Cho phép người dùng mua các vật phẩm cố định trong Cửa Hàng:
  - `ITEM_STREAK_FREEZE`: Giá 100 FitCoins (Giới hạn tối đa 2 khiên trong kho).
  - `ITEM_DOUBLE_XP_BOOST`: Giá 150 FitCoins (Tác dụng nhân đôi XP trong 15 phút tập luyện).

---

## Business Rules and Invariants

### BR-FC-01: Ràng Buộc Bất Biến Về Số Dư (Balance Non-Negative Invariant)
- Tại mọi thời điểm:
  $$\text{balance} \ge 0$$
- Được bảo vệ bằng 2 tầng:
  1. Kiểm tra logic tại tầng Domain: `if wallet.Balance < amount { return ErrInsufficientBalance }`.
  2. Ràng buộc cơ sở dữ liệu PostgreSQL: `CONSTRAINT chk_wallet_balance_non_negative CHECK (balance >= 0)`.

### BR-FC-02: Nguyên Tắc Sổ Cái Append-Only (Immutable Ledger Rule)
- Mọi giao dịch thay đổi số dư đều phải thỏa mãn phương trình cân bằng sổ cái:
  $$\text{balance\_after} = \text{balance\_before} + \text{amount}$$
- Nghiêm cấm mọi thao tác ghi đè số dư mà không có dòng kiểm toán tương ứng.

### BR-FC-03: Quy Tắc Chống Chi Tiêu Kép (Anti-Double-Spending Rule)
- Mọi thao tác trừ coin bắt buộc phải sử dụng khóa dòng bi quan:
  ```sql
  SELECT balance FROM gamification.user_wallet WHERE user_id = $1 FOR UPDATE;
  ```
- Tuần tự hóa các request chi tiêu đồng thời, ngăn chặn việc 2 request cùng đọc số dư cũ rồi mua hàng vượt quá khả năng chi trả.

### BR-FC-04: Giới Hạn Tích Trữ Vật Phẩm (Inventory Caps)
- Số khiên Streak Freeze dự trữ tối đa của một người dùng là **2 khiên**.
- Nếu người dùng đã sở hữu $\ge 2$ Streak Freeze mà vẫn bấm mua: Giao dịch bị từ chối, coin không bị trừ.

### BR-FC-05: Bảng Giá & Mức Thưởng Mặc Định
| Hành động / Vật phẩm | Loại biến động | Số lượng FitCoins | Ghi chú |
| :--- | :---: | :---: | :--- |
| **Thăng Cấp Độ (Level Up)** | Thu nhập (`+`) | **$+20$** | Thưởng mỗi khi Level tăng lên 1 bậc |
| **Mua Đóng Băng Chuỗi (Streak Freeze)** | Chi tiêu (`-`) | **$-100$** | Tối đa tích trữ 2 khiên |
| **Mua Nhân Đôi XP (Double XP Boost)** | Chi tiêu (`-`) | **$-150$** | Hiệu lực trong 15 phút tập luyện |

---

## Error, Alternate, and Boundary Scenarios

- **Số Dư Không Đủ (Insufficient Balance)**:
  - Khi user có 50 coin nhưng mua vật phẩm 100 coin $\rightarrow$ Báo lỗi `FAILED_PRECONDITION: ERR_INSUFFICIENT_FUNDS`, không ghi ledger, không trừ coin.
- **Trùng Lặp Idempotency Key (Duplicate Request)**:
  - Khi client gửi cùng `idempotency_key` 2 lần do mạng lag $\rightarrow$ Giao dịch thứ hai trả về kết quả thành công của giao dịch trước mà không trừ tiền lần hai.
- **Đạt Trần Kho Vật Phẩm (Max Inventory Cap)**:
  - Khi user đã có 2 Streak Freeze $\rightarrow$ Báo lỗi `FAILED_PRECONDITION: ERR_MAX_INVENTORY_REACHED`.
- **Người Dùng Mới Chưa Có Ví**:
  - Tự động khởi tạo ví với `balance = 0` trong lần giao dịch hoặc truy vấn đầu tiên mà không gây lỗi `NotFound`.

---

## Acceptance Criteria

- [ ] **AC-FC-01**: GIVEN người dùng có số dư 120 FitCoins WHEN gửi request mua Streak Freeze (100 FitCoins) THEN số dư ví giảm còn 20 FitCoins, và 1 bản ghi `coin_ledger` xuất hiện với `amount = -100`, `balance_after = 20`.
- [ ] **AC-FC-02**: GIVEN người dùng có số dư 80 FitCoins WHEN gửi request mua Streak Freeze (100 FitCoins) THEN request bị từ chối với lỗi `ERR_INSUFFICIENT_FUNDS`, số dư ví giữ nguyên 80.
- [ ] **AC-FC-03**: GIVEN người dùng đã có 2 Streak Freeze trong kho WHEN gửi request mua thêm 1 Streak Freeze THEN request bị từ chối với lỗi `ERR_MAX_INVENTORY_REACHED`, số dư ví không bị trừ.
- [ ] **AC-FC-04**: GIVEN hai request mua hàng đồng thời trên cùng tài khoản có số dư 100 coin WHEN cả hai cùng yêu cầu trừ 100 coin THEN đúng 1 request thành công, request còn lại bị từ chối do không đủ số dư (chống chi tiêu kép).
- [ ] **AC-FC-05**: GIVEN một sự kiện `UserLeveledUp` được bắn ra WHEN consumer xử lý sự kiện THEN ví người dùng được cộng chính xác $+20$ FitCoins kèm bản ghi ledger với lý do `REASON_LEVEL_UP`.
- [ ] **AC-FC-06**: GIVEN cùng một sự kiện `UserLeveledUp` bị gửi lại lần thứ hai (Kafka retry) WHEN consumer xử lý THEN hệ thống nhận diện `idempotency_key` trùng lặp và không cộng coin lần hai.

---

## Non-Functional Requirements

- **The 3 AM Test**:
  - Không sử dụng các tác vụ cron job chốt số dư định kỳ. Sổ cái hoạt động hoàn toàn theo thời gian thực (Real-time Transactional).
  - Khóa dòng bi quan trên ví chỉ kéo dài $< 3\text{ ms}$ trong transaction cục bộ, không gây nghẽn toàn cục.
- **Toàn Vẹn Dữ Liệu (ACID Guarantee)**:
  - Thao tác cập nhật `user_wallet` và chèn dòng vào `coin_ledger` bắt buộc phải nằm chung trong **cùng 1 Database Transaction**.
- **Hiệu Năng Truy Vấn (Query Performance)**:
  - Truy vấn số dư ví: Đọc theo khóa chính `user_id` $\rightarrow$ Độ trễ $< 1\text{ ms}$.
  - Truy vấn lịch sử giao dịch: Tối ưu qua index `idx_coin_ledger_user_created ON gamification.coin_ledger (user_id, created_at DESC)` $\rightarrow$ Độ trễ $< 5\text{ ms}$.

---

## Open Questions & Decisions Needed

> [!NOTE]
> 1. **Cơ cấu giá vật phẩm Shop**:
>    Mức giá đề xuất hiện tại:
>    - Streak Freeze: $100$ FitCoins (Tương đương 5 lần thăng cấp).
>    - Double XP Boost (15m): $150$ FitCoins (Tương đương 7.5 lần thăng cấp).
>    *Câu hỏi*: Mức giá này đã hợp lý về mặt cân bằng kinh tế in-game chưa?
>
> 2. **Công thức thưởng FitCoins khi lên cấp**:
>    - Phương án A (Cố định - Khuyến nghị): Luôn thưởng $+20$ FitCoins cho mỗi lần tăng 1 Level (đơn giản, dễ nhớ, dễ tính toán).
>    - Phương án B (Tăng dần theo mốc): Level 1–10 (+10 coin), Level 11–20 (+20 coin), Level 50+ (+50 coin).

---

## Explicit Assumptions
- Giả định rằng FitCoins là đơn vị tiền tệ chỉ có thể tích lũy qua nỗ lực tập luyện (in-game earned currency), không quy đổi ngược lại thành tiền thật.
- Giả định rằng khi người dùng mua Streak Freeze, số lượng khiên sẽ được lưu trữ hoặc cập nhật trực tiếp vào trạng thái chuỗi ngày của người dùng.
