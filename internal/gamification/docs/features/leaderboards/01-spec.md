# Feature: Bảng Xếp Hạng Toàn Hệ Thống Theo Điểm XP (XP Leaderboard)

## Summary
Tính năng **Bảng Xếp Hạng Toàn Hệ Thống Theo Điểm XP (XP Leaderboard)** cung cấp cơ chế vinh danh và so sánh thành tích thể chất giữa các gymer trên toàn nền tảng FITAI:
- Thước đo xếp hạng duy nhất: **Điểm Kinh Nghiệm Trọn Đời (`xp`)**.
- Cấu trúc tối giản, không chia phòng đấu (No Cohorts), không chu kỳ tuần (No Weekly Reset/Settlement), không có cơ chế rớt hạng.
- Cung cấp danh sách **Top Bảng Vàng (Top 50)** và **Vị Trí Hiện Tại Của Bản Thân (My Standing)** theo thời gian thực.
- Vận hành trực tiếp trên dữ liệu `gamification.user_xp`, đạt hiệu năng cao và loại bỏ hoàn toàn các tác vụ cron ngầm phức tạp.

---

## Problem and desired outcome

### Problem
- Gymer cần một thước đo trực quan để biết được nỗ lực tích lũy lâu dài của mình đang đứng ở vị trí nào trong cộng đồng người tập thể hình FITAI.
- Các mô hình chia phòng đấu ngẫu nhiên hoặc chia bảng tuần có thể làm phân mảnh trải nghiệm khi người dùng chỉ muốn xem ai là người kiên trì và tích lũy nhiều điểm nhất toàn hệ thống.
- Các cơ chế chốt giải đấu định kỳ (Weekly Settlement) đòi hỏi nhiều tác vụ nền phức tạp, dễ gây nghẽn database hoặc lỗi vận hành.

### Desired outcome
- **Minh bạch & Đơn giản tuyệt đối**: Ai nỗ lực tập luyện nhiều hơn, tích lũy `xp` cao hơn sẽ đứng ở vị trí cao hơn.
- **Thời gian thực (Real-time)**: Thứ hạng tự động cập nhật ngay khi người dùng hoàn thành buổi tập mà không cần chờ chu kỳ tuần hay cron job.
- **Zero Background Jobs**: Không cần bất kỳ batch job quét chốt hay worker phân phòng nào (100% an tâm theo chuẩn "Bài toán 3 AM").
- **Trải nghiệm mượt mà**: Giao diện hiển thị Top 50 dẫn đầu kèm vị trí chính xác của bản thân, phản hồi với độ trễ cực thấp ($< 20\text{ ms}$).

---

## Actors and entry points
- **Gym User (Mobile App / Web Client)**:
  - Gọi API ConnectRPC `GetGlobalLeaderboard` để xem danh sách Top 50 và thứ hạng của chính mình.
- **Tầng dữ liệu nội bộ**:
  - Tận dụng trực tiếp bảng `gamification.user_xp` (được cập nhật từ tính năng `xp-and-levels`), không cần tạo bảng trung gian hay sao chép dữ liệu.

---

## Scope

### In scope
- Truy vấn danh sách Top 50 gymer có `xp` cao nhất trên toàn nền tảng.
- Truy vấn vị trí thứ hạng của người dùng hiện tại (`my_rank`, `my_xp`, `my_level`).
- Quy tắc phân định hòa điểm (Tie-breaking): Bằng điểm thì ưu tiên người đạt mốc điểm trước (`updated_at ASC`).
- Tối ưu hóa truy vấn cơ sở dữ liệu bằng B-tree index trên `gamification.user_xp (xp DESC, updated_at ASC)`.
- Định nghĩa API hợp đồng Protobuf: `GetGlobalLeaderboardRequest` và `GetGlobalLeaderboardResponse`.

### Out of scope & Explicitly Deferred
- **Phòng đấu nhóm (Cohorts/Rooms) [DEFERRED]**: Tạm thời loại bỏ theo yêu cầu của Maintainer để đơn giản hóa hệ thống.
- **Chu kỳ xếp hạng theo tuần (Weekly Leagues & Demotion) [DEFERRED]**: Bỏ qua chu kỳ tuần; toàn bộ bảng xếp hạng tính theo `xp` trọn đời.
- **Thưởng FitCoins [DEFERRED]**: Không tích hợp trả thưởng coin cho bảng xếp hạng; giao dịch coin do module `fitcoins-and-ledger` phụ trách.
- **Bảng xếp hạng bạn bè (Friend Leaderboard) [DEFERRED]**: Lọc theo đồ thị bạn bè thuộc phân hệ `social`.

---

## Current vs Desired Behavior
- **Current Behavior**: Hệ thống lưu trữ `xp` và `level` trong `gamification.user_xp`, nhưng chưa có endpoint tra cứu bảng xếp hạng công khai.
- **Desired Behavior**: Người dùng có thể mở tab "Xếp Hạng" trên ứng dụng bất kỳ lúc nào để xem Top 50 người dẫn đầu toàn cầu và thứ hạng chính xác của bản thân trong cộng đồng.

---

## User Scenarios / Use Cases

### UC-LB-01: Xem Bảng Xếp Hạng Toàn Hệ Thống và Thứ Hạng Bản Thân
- **Actor**: Gym User (Đã xác thực Bearer Token).
- **Preconditions**: Người dùng đã có bản ghi trong `gamification.user_xp` (hoặc khởi tạo mặc định 0 XP).
- **Trigger**: Người dùng mở màn hình "Bảng Xếp Hạng" trên ứng dụng.
- **Main Flow**:
  1. Client gửi request `GetGlobalLeaderboard(limit = 50)` kèm Token xác thực của người dùng.
  2. Hệ thống truy vấn Top 50 bản ghi từ `gamification.user_xp` sắp xếp theo `xp DESC, updated_at ASC`.
  3. Hệ thống xác định thông tin của người dùng gọi API:
     - Nếu người dùng nằm trong Top 50: Lấy trực tiếp vị trí index $+ 1$ làm `my_rank`.
     - Nếu người dùng nằm ngoài Top 50: Thực thi câu lệnh đếm số người có điểm cao hơn:
       $$\text{rank} = \text{COUNT}(\text{users có } \texttt{xp} > \texttt{my\_xp}) + 1$$
  4. Hệ thống đóng gói danh sách Top 50 (gồm `rank`, `user_id`, `xp`, `level`) kèm khối `my_standing` (gồm `rank`, `user_id`, `xp`, `level`).
  5. Trả về response `200 OK`.
- **Postconditions**: Người dùng nắm bắt được bảng xếp hạng toàn cầu và vị trí phấn đấu của bản thân.

---

## Functional Requirements
- **FR-LB-01**: Cung cấp API `GetGlobalLeaderboard` trả về danh sách Top N gymer có `xp` cao nhất hệ thống (mặc định $N = 50$, tối đa $100$).
- **FR-LB-02**: Trả về thông tin vị trí thứ hạng chính xác của người dùng gọi request (`my_rank`), bất kể người dùng nằm trong hay ngoài Top N.
- **FR-LB-03**: Thứ hạng của mỗi thành viên trong danh sách là số thứ tự liên tục từ $1$ đến $N$.
- **FR-LB-04**: Thứ hạng cập nhật thời gian thực (Real-time): Ngay khi một người dùng nhận thêm XP từ buổi tập hoặc dinh dưỡng, vị trí trên bảng xếp hạng phản ánh tức thì.
- **FR-LB-05**: Khi hai hoặc nhiều người dùng có cùng số `xp`, hệ thống ưu tiên người đạt mốc điểm đó sớm hơn (`updated_at ASC`).

---

## Business Rules and Invariants

### BR-LB-01: Thước Đo Xếp Hạng Duy Nhất (Single Metric)
- Thước đo duy nhất để phân định thứ hạng là **Điểm Kinh Nghiệm Trọn Đời (`xp`)**.
- Tuyệt đối không trừ điểm, không suy giảm điểm theo thời gian.

### BR-LB-02: Quy Tắc Phân Định Hòa Điểm (Tie-Breaking Rule)
- Nếu User A và User B có cùng số `xp`:
  - Người có `updated_at` nhỏ hơn (đạt mốc điểm đó sớm hơn) sẽ có thứ hạng cao hơn.
  - Nếu cả `xp` và `updated_at` trùng nhau: So sánh theo `user_id ASC` (đảm bảo thứ tự xác định $100\%$, không bị nhảy vị trí ngẫu nhiên giữa các lần truy vấn).

### BR-LB-03: Quy Mô Danh Sách Mặc Định
- Danh sách Bảng Vàng trả về tối đa **50 người dùng** dẫn đầu (`limit = 50`).
- Nếu toàn hệ thống có ít hơn 50 người dùng: Trả về toàn bộ danh sách hiện có theo đúng thứ tự.

---

## Error, Alternate, and Boundary Scenarios
- **Người Dùng Mới Chưa Có Hoạt Động (0 XP)**:
  - Nếu người dùng mới đăng ký (hoặc chưa phát sinh buổi tập nào), hệ thống khởi tạo mặc định hoặc coi là $0$ XP, thứ hạng hiển thị ở vị trí cuối cùng của hệ thống.
- **Hệ Thống Rỗng (Chưa Có User Nào Có XP)**:
  - Trả về danh sách rỗng `items: []`, `my_rank: 1` mà không gây lỗi panic/crash.
- **Người Dùng Nằm Ngoài Top 50**:
  - Vẫn trả về đúng danh sách Top 50 người dẫn đầu, đồng thời trường `my_standing.rank` thể hiện đúng vị trí thực tế của họ (ví dụ: hạng 1,250).

---

## Acceptance Criteria
- [ ] **AC-LB-01**: GIVEN hệ thống có 100 người dùng có XP khác nhau WHEN gọi `GetGlobalLeaderboard(limit = 50)` THEN trả về đúng 50 người có `xp` cao nhất, sắp xếp giảm dần từ hạng 1 đến 50.
- [ ] **AC-LB-02**: GIVEN User A có 1,000 XP lúc 10:00 và User B có 1,000 XP lúc 11:00 WHEN truy vấn bảng xếp hạng THEN User A đứng trước User B.
- [ ] **AC-LB-03**: GIVEN User C đang đứng thứ 75 toàn hệ thống WHEN User C gọi API THEN danh sách trả về Top 50, và `my_standing.rank = 75`.
- [ ] **AC-LB-04**: GIVEN User D đang đứng thứ 3 toàn hệ thống WHEN User D gọi API THEN `my_standing.rank = 3` và User D xuất hiện ở phần tử thứ 3 trong danh sách Top 50.
- [ ] **AC-LB-05**: GIVEN user vừa hoàn thành bài tập nhận 100 XP giúp điểm tăng từ 950 lên 1,050 XP WHEN gọi lại API ngay sau đó THEN thứ hạng được cập nhật phản ánh đúng điểm số mới tức thì (Real-time).

---

## Non-Functional Requirements
- **The 3 AM Test**: Triệt tiêu $100\%$ rủi ro sập hệ thống nửa đêm:
  - Không có scheduled batch job / cron job định kỳ.
  - Không có worker nền xử lý chốt phòng đấu.
  - Không có rủi ro table lock hay deadlock.
- **Hiệu Năng Truy Vấn (Query Latency)**:
  - Truy vấn Top 50: Thực thi bằng index scan `idx_user_xp_leaderboard ON gamification.user_xp (xp DESC, updated_at ASC)` $\rightarrow$ Độ trễ $< 5\text{ ms}$.
  - Truy vấn `my_rank` cho user ngoài Top 50: Sử dụng `COUNT(*) WHERE xp > :my_xp` tận dụng B-tree index $\rightarrow$ Độ trễ P99 $< 20\text{ ms}$.
- **Schema Isolation**:
  - Toàn bộ truy vấn nằm trọn trong bảng `gamification.user_xp`. Không JOIN chéo sang schema `profile` hay bất kỳ module nào khác.

---

## Open Questions & Decisions Needed

> [!NOTE]
> **Hiển thị Tên & Ảnh đại diện (Display Name & Avatar) trên Leaderboard**:
> Do nguyên tắc Modular Monolith cấm JOIN chéo sang schema `profile`, API của Gamification sẽ trả về `user_id`. Để Client hiển thị được Tên và Avatar của Top 50:
> - **Phương án A (BFF / API Gateway Aggregation - Khuyến nghị)**: Tầng API Gateway hoặc Mobile Client sẽ gửi batch `user_ids` sang module `profile` để nạp tên và avatar tương ứng.
> - **Phương án B (Event-Driven Profile Cache)**: Module Gamification lắng nghe sự kiện `UserProfileUpdated` từ Kafka và lưu bản sao `display_name`, `avatar_url` trực tiếp trong bảng `user_xp`.
>
> *(Khuyến nghị Phương án A để giữ module Gamification tinh gọn, thuần túy và không bị dư thừa dữ liệu).*

---

## Explicit Assumptions
- Giả định rằng điểm số `xp` được quản lý và bảo vệ tính toàn vẹn (chống Lost Update bằng khóa dòng bi quan) bởi tính năng `xp-and-levels`.
- Giả định rằng danh sách hiển thị mặc định trên ứng dụng là Top 50 người dùng dẫn đầu.
