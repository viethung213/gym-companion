# ĐẶC TẢ YÊU CẦU NGHIỆP VỤ CỐT LÕI (CORE REQUIREMENTS SPECIFICATION)
## DỰ ÁN: AI FITNESS SOCIAL PLATFORM (FITAI)
### TIÊU CHUẨN ĐẶC TẢ: BABOK® GUIDE V3.0 Compliant

---

## KIỂM SOÁT TÀI LIỆU (DOCUMENT CONTROL)

| Thông tin | Chi tiết |
|---|---|
| **Mã tài liệu** | FITAI-BRD-CORE-001 |
| **Phiên bản** | 1.6.0 |
| **Ngày hiệu lực** | 02/07/2026 (rev 2) |
| **Tác giả** | Senior Business Analyst |
| **Trạng thái** | Approved |
| **Phân loại bảo mật** | Internal Confidential |

### Lịch sử thay đổi tài liệu
| Phiên bản | Ngày | Mô tả thay đổi | Người thực hiện |
|---|---|---|---|
| 0.1 | 25/06/2026 | Khởi tạo cấu trúc theo chuẩn BABOK v3 | Senior BA |
| 1.0 | 27/06/2026 | Hoàn thiện đặc tả chi tiết 7 nhóm nghiệp vụ cốt lõi | Senior BA |
| 1.1 | 28/06/2026 | Loại bỏ Module Gamification, tính năng BPM Sync và bổ sung quy tắc loại bỏ nhóm cơ chấn thương | Antigravity AI |
| 1.2 | 29/06/2026 | Bổ sung mục tiêu nghiệp vụ điều chỉnh tư thế (form) cho người mới tập và tối ưu thực đơn dinh dưỡng | Antigravity AI |
| 1.3 | 01/07/2026 | Thay đổi cơ chế từ sinh giáo án 4 tuần tĩnh sang sinh lộ trình tổng quan & sinh giáo án chi tiết theo buổi tập | Antigravity AI |
| 1.4 | 02/07/2026 | Bổ sung FR-AC-07 (Warm-up/Cool-down), BR-AC-03 (xử lý bỏ lịch), Quy trình 3.4 (Review & Renew + Mid-cycle Adjustment 2 chiều), tối giản Onboarding, hỏi thiết bị/dị ứng theo ngữ cảnh, Module 7 (Admin) | Antigravity AI |
| 1.5 | 02/07/2026 | Viết lại Quy trình 3.4 theo cơ chế adaptive cho người mới có lịch bất định; bổ sung BR-WL-01/02 kiểm soát thời gian & volume buổi tập bất thường | Antigravity AI |
| 1.6 | 02/07/2026 | Tái cấu trúc Quy trình 3.4: tách logic phân nhánh CR và Signal B1-B4 ra thành BR-AC-04 – BR-AC-08; tổng quát hóa BR-WL-02 | Antigravity AI |

---

## 1. BỐI CẢNH & MỤC TIÊU NGHIỆP VỤ (BUSINESS CONTEXT & OBJECTIVES)

### 1.1 Khái quát giải pháp (Solution Scope)
Giải pháp cốt lõi cung cấp nền tảng số hỗ trợ tập luyện cá nhân hóa tự động dựa trên Trí tuệ nhân tạo (AI) và Thị giác máy tính (Computer Vision), kết hợp Dinh dưỡng cá nhân hóa nhằm giải quyết rào cản chi phí huấn luyện viên cá nhân (PT) và động lực tập luyện của người mới bắt đầu.

### 1.2 Mục tiêu nghiệp vụ (Business Objectives)
Theo khung phân tích BABOK, các yêu cầu cốt lõi phải liên kết trực tiếp với các mục tiêu kinh doanh sau:
* **OB-01 (Accessibility)**: Giảm chi phí tiếp cận hướng dẫn tập luyện chuyên nghiệp xuống 90% so với thuê PT truyền thống.
* **OB-02 (Safety)**: Giảm tỷ lệ chấn thương do tập sai kỹ thuật của người dùng thông qua phân tích góc khớp thời gian thực.
* **OB-03 (Retention)**: Duy trì tỷ lệ người dùng tiếp tục luyện tập sau 30 ngày đạt $\ge 40\%$ nhờ AI Coach đồng hành.
* **OB-04 (Form Standardization)**: Điều chỉnh chuẩn tư thế (form) tập luyện cho người mới tập thông qua hướng dẫn chi tiết và phản hồi sửa lỗi thời gian thực từ AI Camera.
* **OB-05 (Nutrition Optimization)**: Cung cấp thực đơn dinh dưỡng tối ưu và cá nhân hóa sâu theo thể trạng, ngân sách và mục tiêu luyện tập của từng người dùng.

---

## 2. PHÂN TÍCH TÁC NHÂN (STAKEHOLDER ANALYSIS)

Đặc tả các tác nhân tương tác trực tiếp hoặc gián tiếp với hệ thống cốt lõi:

| Mã tác nhân | Tên tác nhân | Mô tả vai trò | Mức độ ảnh hưởng |
|---|---|---|---|
| **ACT-01** | End User (Người tập) | Người sử dụng ứng dụng để lên lịch, tập luyện dưới camera, theo dõi sức khỏe và dinh dưỡng. | High |
| **ACT-02** | AI Coach Engine | Tác nhân hệ thống: Phân tích dữ liệu người dùng, lên lịch, động viên và điều chỉnh giáo án. | High |
| **ACT-03** | AI Camera Engine | Tác nhân hệ thống: Xử lý luồng hình ảnh camera, đếm rep, ước lượng cân nặng, phát hiện lỗi tư thế. | High |
| **ACT-04** | AI Nutrition Engine | Tác nhân hệ thống: Tính toán calo cá nhân hóa và luân chuyển thực đơn không trùng lặp. | Medium |
| **ACT-05** | System Administrator | Quản trị viên hệ thống: Quản lý cấu hình bài tập, kiểm tra dữ liệu và bảo mật. | Low |

---

## 3. BẢN ĐỒ QUY TRÌNH NGHIỆP VỤ (BUSINESS PROCESS MODELING)

### 3.1 Quy trình Thiết lập Hồ sơ & Khởi tạo Lịch tập (Onboarding & Planning)

```
[Người dùng] ──► Nhập Thông tin Cơ bản & Chỉ số Cơ thể (Cân nặng, Chiều cao)
     │
     ▼
[Người dùng] ──► Chọn Mục tiêu (Tăng cơ/Giảm mỡ) & Khung giờ tập cố định
     │
     ▼
[Người dùng] ──► Khai báo Sức khỏe (Chấn thương cũ, Bệnh lý mãn tính)
     │
     ▼
[AI Coach] ─────► Tính toán User Fitness Score & Thiết lập lịch tuần
     │
     ▼
[AI Coach] ─────► Khởi tạo Lộ trình 4 tuần, Lịch tập tuần & Gợi ý thực đơn cá nhân hóa
```

### 3.2 Quy trình Luyện tập dưới AI Camera (Workout Execution)

```
[Người dùng] ──► Check-in buổi tập & Thiết lập Audio/Playlist nhạc mong muốn
     │
     ▼
[Người dùng] ──► Bật Camera & Điều chỉnh khoảng cách/Ánh sáng theo chỉ dẫn
     │
     ▼
[AI Camera### 3.4 Quy trình Đánh giá & Điều chỉnh Lộ trình (Adaptive Review Cycle)

> **Nguyên tắc thiết kế:** Người mới tập thường có lịch sinh hoạt không ổn định. Hệ thống KHÔNG dùng ngưỡng cứng
> theo tuần mà dùng tín hiệu hành vi tích lũy (định nghĩa tại BR-AC-04 – BR-AC-08) để phát hiện
> sự lệch hướng và đề xuất điều chỉnh — thay vì phán xét hay phạt người dùng.

```
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
TRIGGER A — KẾT THÚC CHU KỲ 4 TUẦN (chạy 1 lần/chu kỳ)
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

[Hệ thống] ──► Ngày kết thúc tuần 4 của lộ trình hiện tại
     │
     ▼
[AI Coach] ──► Tính toán Completion Rate (CR) = (buổi đã hoàn thành / buổi đã lên lịch) × 100%
     │         và tập hợp: tăng trưởng chỉ số thể lực, điểm Form trung bình, số buổi Anomalous
     │
     ▼
[AI Coach] ──► Áp dụng BR-AC-04: Phân nhánh quyết định và sinh
     │         phương án điều chỉnh lộ trình cho chu kỳ tiếp theo
     │
     ▼
[Người dùng] ──► Nhận Báo cáo Tổng kết 4 tuần & Xem trước lộ trình mới
     │             (Có thể từ chối đề xuất và chọn điều chỉnh thủ công)
     ▼
[AI Coach] ──► Xác nhận & Kích hoạt Lộ trình 4 tuần tiếp theo


━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
TRIGGER B — GIÁM SÁT GIỮA CHU KỲ (event-driven, không theo lịch cố định)
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

[Hệ thống] ──► Liên tục theo dõi 4 tín hiệu hành vi độc lập (BR-AC-05 – BR-AC-08).
     │         Mỗi tín hiệu kích hoạt ngay khi điều kiện xảy ra,
     │         không phụ thuộc vào nhàu hay quá trình kiểm tra theo lịch.
     │
     ├── B1 (Không hoạt động kéo dài) → Kiểm tra theo BR-AC-05
     ├── B2 (Lịch không tương thích)     → Kiểm tra theo BR-AC-06
     ├── B3 (Tăng cường độ đột biến)       → Kiểm tra theo BR-AC-07
     └── B4 (Tiến bộ đình trệ)             → Kiểm tra theo BR-AC-08
``` bỏ cuối cùng            │
│       (b) Đặt lại lịch tập cho tuần này (reschedule)            │
│       (c) Tạm dừng lộ trình chính thức (Pause — tối đa 4 tuần) │
│    → Nếu không có phản hồi sau 48 giờ: nhắc lại 1 lần duy nhất │
│    → KHÔNG tự động điều chỉnh lộ trình khi chưa có phản hồi    │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│  SIGNAL B2 — LỊCH KHÔNG TƯƠNG THÍCH (Schedule Mismatch)        │
│  Điều kiện: Người dùng bỏ cùng 1 ngày trong tuần ≥ 3 lần liên  │
│             tiếp (VD: luôn bỏ thứ Hai).                         │
│  Hành động:                                                      │
│    → AI Coach chủ động hỏi: "Bạn thường bận vào thứ Hai.        │
│      Bạn muốn dời buổi tập ngày đó sang ngày khác không?"       │
│    → Nếu đồng ý: tự động dịch chuyển slot trong lịch tuần       │
│    → Nếu từ chối: giữ nguyên, ghi nhận và không hỏi lại         │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│  SIGNAL B3 — TĂNG CƯỜNG ĐỘ ĐỘT BIẾN (Overtraining Signal)      │
│  Điều kiện (bất kỳ 1 trong 2):                                  │
│    (i)  Người dùng ghi nhận ≥ 2 buổi trong cùng 1 ngày; HOẶC   │
│    (ii) Trung bình RPE ≥ 8.5 trong ≥ 5 buổi liên tiếp.         │
│  Hành động:                                                      │
│    → Hiển thị cảnh báo Overtraining Risk với giải thích ngắn    │
│    → Bắt buộc chèn ít nhất 1 ngày nghỉ vào giáo án tiếp theo   │
│    → Gợi ý 1 buổi Active Recovery (giãn cơ, đi bộ nhẹ)         │
│    → Người dùng có thể bỏ qua cảnh báo nhưng hệ thống ghi nhận │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│  SIGNAL B4 — TIẾN BỘ ĐÌNH TRỆ (Plateau Signal)                 │
│  Điều kiện: Cả 1RM ước tính VÀ Form Score trung bình không tăng │
│             trong 3 tuần liên tiếp (với CR ≥ 70% để loại trừ   │
│             trường hợp tập không đủ).                            │
│  Hành động:                                                      │
│    → AI Coach thông báo "Bạn đang ở giai đoạn ổn định"          │
│    → Đề xuất 1 trong 3 phương án phá plateau:                   │
│       (a) Deload Week: giảm volume 40% trong 1 tuần             │
│       (b) Thay đổi biến thể bài tập (variation swap)            │
│       (c) Tăng số set thay vì tăng tạ (volume accumulation)     │
│    → Người dùng chọn và xác nhận. AI Coach áp dụng ngay         │
│       vào giáo án buổi tiếp theo.                                │
└─────────────────────────────────────────────────────────────────┘
```

---

## 4. YÊU CẦU CHỨC NĂNG CHI TIẾT (FUNCTIONAL REQUIREMENTS - FR)

Các yêu cầu chức năng được phân loại theo tiêu chuẩn BABOK v3.0, ký hiệu bằng mã: **FR-[Module]-[Số thứ tự]**. Độ ưu tiên phân loại theo MoSCoW: Must have (M), Should have (S), Could have (C), Won't have (W).

---

### Module 1: Quản lý Người dùng & Hồ sơ (User & Profile Management)

| Mã yêu cầu | Tên yêu cầu | Mô tả nghiệp vụ chi tiết | Tác nhân | Độ ưu tiên |
|---|---|---|---|---|
| **FR-UM-01** | Đăng ký & Đăng nhập | Hệ thống phải hỗ trợ đăng ký/đăng nhập thông qua Email, Số điện thoại (xác thực OTP) và liên kết tài khoản định danh mạng xã hội (Google, Apple, Facebook). | ACT-01 | M |
| **FR-UM-02** | Khai báo Hồ sơ Sức khỏe | Hệ thống cho phép người dùng nhập thông số cơ thể (tuổi, giới tính, chiều cao, cân nặng), mục tiêu luyện tập và tiền sử bệnh lý/chấn thương. | ACT-01 | M |
| **FR-UM-03** | Thiết lập Khung giờ Cố định | Người dùng bắt buộc phải chọn tối thiểu 1 khung giờ tập luyện cố định trong ngày tại luồng Onboarding để làm cơ sở nhắc nhở lịch tập. | ACT-01 | M |
| **FR-UM-04** | Nhắc nhở lịch tập tự động | Hệ thống tự động quét và gửi Push Notification trước khung giờ tập cố định 15 phút, nội dung thông báo được cá nhân hóa theo phong cách AI Coach đã thiết lập. | ACT-02 | S |

---

### Module 2: AI Coach Cá nhân (Huấn luyện viên ảo)

| Mã yêu cầu | Tên yêu cầu | Mô tả nghiệp vụ chi tiết | Tác nhân | Độ ưu tiên |
|---|---|---|---|---|
| **FR-AC-01** | Khởi tạo Lộ trình & Lịch tập | AI Coach phân tích chỉ số cơ thể, mục tiêu và hạn chế chấn thương để tự động sinh Lộ trình 4 tuần (các cột mốc định hướng) và Lịch tập tuần (phân bổ nhóm cơ cho từng ngày). Không sinh chi tiết bài tập ở bước này. | ACT-02 | M |
| **FR-AC-02** | Tự động điều chỉnh kế hoạch | AI Coach tự động phân tích dữ liệu hiệu năng sau mỗi buổi tập để tinh chỉnh: tăng/giảm cường độ, thay thế bài tập hoặc chèn Deload Week (tuần giảm tải). | ACT-02 | M |
| **FR-AC-03** | Đề xuất bài tập thay thế | Khi người dùng báo chấn thương đột xuất hệ thống sẽ loại bỏ các bài tập tác động trực tiếp vào nhóm cơ bị chấn thương đó cho đến khi nhận được thông báo đã hồi phục từ người dùng. | ACT-02, ACT-01 | S |
| **FR-AC-04** | Đồng hành & Cổ vũ | AI Coach gửi tin nhắn động viên cá nhân hóa dựa trên dữ liệu thật (VD: khi người dùng vượt PR hoặc khi nghỉ dài ngày quay trở lại). | ACT-02 | S |
| **FR-AC-05** | Thiết lập phong cách Coach | Người dùng có quyền lựa chọn phong cách tương tác của Coach: Drill Sergeant (nghiêm khắc), Best Friend (thân thiện), hoặc Data Analyst (khoa học). | ACT-01 | C |
| **FR-AC-06** | Sinh giáo án chi tiết theo buổi | AI Coach tự động sinh danh sách bài tập, số set, rep và cân nặng gợi ý cho buổi tập cụ thể ngay trước khi bắt đầu (hoặc chạy ngầm sinh trước bản nháp), dựa trên Lộ trình 4 tuần, Lịch tập tuần và dữ liệu lịch sử/sức khỏe mới nhất của người dùng. Trước khi xác nhận giáo án, AI Coach hỏi người dùng 1-2 câu ngắn theo ngữ cảnh nếu chưa có thông tin: (a) môi trường & dụng cụ sẵn có cho buổi hôm nay, (b) thực phẩm cần tránh nếu giáo án kèm gợi ý dinh dưỡng. Thông tin này được lưu vào hồ sơ để không hỏi lại. | ACT-02, ACT-01 | M |
| **FR-AC-07** | Warm-up & Cool-down tự động | AI Coach tự động chèn khối khởi động (5-10 phút) vào đầu và khối hạ nhiệt/giãn cơ (5 phút) vào cuối mỗi giáo án chi tiết, cá nhân hóa theo nhóm cơ chính sẽ được kích hoạt trong buổi tập đó. Warm-up và Cool-down không được tính vào tổng volume nâng (kg) của buổi tập nhưng được tính vào tổng thời gian và calo tiêu thụ. | ACT-02 | M |

---

### Module 3: AI Camera Coach (Phân tích tư thế thời gian thực)

| Mã yêu cầu | Tên yêu cầu | Mô tả nghiệp vụ chi tiết | Tác nhân | Độ ưu tiên |
|---|---|---|---|---|
| **FR-CC-01** | Tracking Khung xương | Phân tích luồng video camera trước/sau để xác định 33 điểm khớp chính trên cơ thể người tập. | ACT-03 | M |
| **FR-CC-02** | Đo lường góc ROM % | AI Camera tính toán biên độ chuyển động (ROM) của các khớp và đưa ra tỷ lệ % hoàn thiện của mỗi lần lặp (rep). | ACT-03 | M |
| **FR-CC-03** | Phát hiện lỗi tư thế | Tự động so sánh tọa độ các khớp với mô hình chuyển động chuẩn để phát hiện lỗi kỹ thuật (VD: võng lưng, gối chụm...). | ACT-03 | M |
| **FR-CC-04** | Cảnh báo thời gian thực | Hệ thống hiển thị visual overlay và phát tín hiệu giọng nói hướng dẫn sửa lỗi tức thì với độ trễ tối đa < 500ms. | ACT-03 | M |
| **FR-CC-05** | Chấm điểm kỹ thuật | Tính điểm kỹ thuật (Form Score từ 0-100) cho từng rep dựa trên sự kết hợp giữa ROM %, căn chỉnh khớp và tốc độ thực hiện. | ACT-03 | S |

---

### Module 4: Quản lý Buổi tập (Workout Logging & Audio)

| Mã yêu cầu | Tên yêu cầu | Mô tả nghiệp vụ chi tiết | Tác nhân | Độ ưu tiên |
|---|---|---|---|---|
| **FR-WL-01** | Ghi chép buổi tập tự động | AI tự động điền số rep đã đếm được và % hoàn thiện. AI ước lượng cân nặng tạ thực tế thông qua việc nhận dạng đĩa tạ và tốc độ nâng của người dùng để tránh nhập khống dữ liệu. | ACT-03 | M |
| **FR-WL-02** | Ghi chép thủ công | Cho phép người dùng chỉnh sửa số rep và cân nặng thực tế nếu AI ước lượng hoặc đếm chưa chính xác trước khi bấm lưu set. | ACT-01 | M |
| **FR-WL-03** | Tương tác âm thanh thông minh | Hệ thống cho phép người dùng chọn nhạc nền buổi tập (EDM/Lo-fi). Hệ thống tự giảm âm lượng nhạc nền (Audio Ducking) khi AI phát âm thanh sửa lỗi kỹ thuật. | ACT-01 | S |
| **FR-WL-04** | Ghi nhận Kỷ lục Cá nhân (PR) | Tự động tính toán 1RM ước tính (Epley Formula) sau mỗi buổi tập và vinh danh bằng ăn mừng nếu đạt PR mới. | ACT-02 | S |

---

### Module 5: Dinh dưỡng AI (Anti-Repetition & Budget)

| Mã yêu cầu | Tên yêu cầu | Mô tả nghiệp vụ chi tiết | Tác nhân | Độ ưu tiên |
|---|---|---|---|---|
| **FR-NU-01** | Tính toán kcal cá nhân hóa | Tính toán TDEE và macronutrients hàng ngày riêng biệt cho từng cá nhân, dựa theo công thức Mifflin-St Jeor và khối lượng vận động thực tế. | ACT-04 | M |
| **FR-NU-02** | Đa lựa chọn theo ngân sách | Mỗi bữa ăn (Sáng, Trưa, Tối) gợi ý tối thiểu 3 lựa chọn món ăn đạt chuẩn macro, chia theo mức giá: Tiết kiệm, Phổ thông, và Thoải mái. | ACT-04 | S |
| **FR-NU-03** | Thuật toán Anti-Repetition | Loại trừ các nguồn protein chính đã dùng trong 7 ngày, tinh bột trong 5 ngày và chủ đề món trong 3 ngày để chống lặp lại món ăn gây ngán. | ACT-04 | M |
| **FR-NU-04** | Nhật ký dinh dưỡng | Người dùng ghi lại bữa ăn thực tế bằng cách tìm kiếm món ăn hoặc quét mã vạch sản phẩm. | ACT-01 | S |

---

### Module 6: Theo dõi Tiến trình (Progress Tracking)

| Mã yêu cầu | Tên yêu cầu | Mô tả nghiệp vụ chi tiết | Tác nhân | Độ ưu tiên |
|---|---|---|---|---|
| **FR-PT-01** | Ghi nhận chỉ số cơ thể | Cho phép người dùng cập nhật cân nặng, tỷ lệ mỡ cơ thể (Body Fat %), số đo các vòng cơ bắp và lưu trữ ảnh tiến trình. | ACT-01 | M |
| **FR-PT-02** | Phân tích xu hướng (Analytics) | Vẽ biểu đồ xu hướng biến động cân nặng (moving average), xu hướng tăng sức mạnh (1RM) và biểu đồ điểm kỹ thuật trung bình. | ACT-02 | S |
| **FR-PT-03** | AI phân tích chuyên sâu | Định kỳ gửi báo cáo phân tích chuyên sâu về tiến độ đạt mục tiêu của người dùng kèm lời khuyên tối ưu. | ACT-02 | S |

---



## 5. QUY TẮC NGHIỆP VỤ (BUSINESS RULES - BR)

Theo chuẩn BABOK, quy tắc nghiệp vụ định nghĩa các chính sách, công thức và ràng buộc mang tính bắt buộc:

| Mã quy tắc | Module | Nội dung quy tắc nghiệp vụ |
|---|---|---|
| **BR-UM-01** | Hồ sơ | Hồ sơ sức khỏe của người dùng phải đạt tỷ lệ hoàn thiện **$\ge 80\%$** trước khi hệ thống cho phép kích hoạt AI Coach và tạo kế hoạch luyện tập đầu tiên. |
| **BR-AC-01** | Tập luyện | AI Coach không được phép lên lịch tập vượt quá **6 buổi/tuần** và bắt buộc phải có tối thiểu **1 ngày nghỉ hoàn toàn** trong tuần để đảm bảo cơ bắp phục hồi. |
| **BR-AC-02** | Tiến độ | Tốc độ tăng tiến khối lượng tập luyện (Progressive Overload) do AI đề xuất không được vượt quá **10% tổng volume** của tuần trước đó nhằm tránh quá tải. |
| **BR-CC-01** | AI Camera | Một rep chỉ được tính là hợp lệ (Valid Rep) để đếm số khi và chỉ khi biên độ chuyển động của khớp (ROM) đạt tối thiểu **$\ge 70\%$** so với biên độ tiêu chuẩn. |
| **BR-CC-02** | Chống gian lận | Nếu tỷ lệ số khung hình video phát hiện khung xương hợp lệ < 50% trong buổi tập dưới camera, hệ thống sẽ đánh dấu buổi tập là "Không đạt chuẩn xác thực" và không ghi nhận kết quả buổi tập tự động. |
| **BR-NU-01** | Dinh dưỡng | AI Nutrition tuyệt đối không gợi ý chế độ ăn uống có tổng năng lượng dưới **1,200 kcal/ngày** cho bất kỳ đối tượng nào nhằm đảm bảo an toàn sinh học cơ bản. |
| **BR-NU-02** | Dinh dưỡng | Nguồn protein chính đã sử dụng trong bữa ăn được lưu trong Meal History sẽ bị khóa và **không xuất hiện lại trong thực đơn gợi ý trong vòng 7 ngày tiếp theo**. |
| **BR-AC-03** | Tập luyện | Logic xử lý bỏ lịch tập và điều chỉnh giữa chu kỳ được định nghĩa theo cơ chế Signal B1–B4 tại Quy trình 3.4. Giáo án của các buổi bị bỏ được đánh dấu "Bỏ qua" và **không tự động bù** vào ngày tiếp theo nếu chưa có xác nhận của người dùng. |
| **BR-AC-04** | Lộ trình | **Điều chỉnh lộ trình theo Completion Rate (CR):** Sau mỗi chu kỳ 4 tuần, AI Coach phân nhánh quyết định dựa trên CR = (số buổi đã hoàn thành / số buổi đã lên lịch) × 100%: (1) **CR < 40%** — Lộ trình chưa phù hợp cuộc sống: AI Coach hỏi lý do bỏ tập, chờ phản hồi rồi mới đề xuất giảm số buổi/tuần và rút ngắn thời lượng. (2) **40% ≤ CR < 70%** — Hoàn thành một phần: giữ nguyên số buổi, giảm tải lượng 10-15%, thêm buổi Express Workout 30 phút; tự động sinh lộ trình mới không cần hỏi. (3) **70% ≤ CR < 90%** — Hoàn thành tốt: giữ cấu trúc, tăng Progressive Overload ≤ 10% theo BR-AC-02; tự động sinh lộ trình mới. (4) **CR ≥ 90%** — Xuất sắc: tăng cường độ hoặc thêm 1 buổi/tuần (không vượt BR-AC-01); gắn badge "Hoàn thành xuất sắc". Người dùng luôn có quyền từ chối và điều chỉnh thủ công trước khi xác nhận. |
| **BR-AC-05** | Lộ trình | **Signal B1 — Không hoạt động kéo dài:** Nếu không có buổi tập nào được ghi nhận trong **7 ngày liên tiếp** và không có chấn thương đã khai báo, AI Coach gửi 1 tin check-in với giọng điệu theo phong cách đã chọn (không phải thông báo nhắc lịch thông thường) và đề xuất 3 lựa chọn: (a) Tiếp tục từ buổi bỏ gần nhất, (b) Rắp lại lịch tuần này, (c) Tạm dừng lộ trình (Pause tối đa 4 tuần). Nếu không có phản hồi sau 48 giờ: nhắc lại đúng 1 lần. Hệ thống tuyệt đối **không tự động điều chỉnh lộ trình** khi chưa có phản hồi. |
| **BR-AC-06** | Lịch tập | **Signal B2 — Lịch không tương thích:** Nếu người dùng bỏ đúng **cùng 1 ngày trong tuần ≥ 3 lần liên tiếp**, AI Coach chủ động hỏi có muốn dời slot lịch đó sang ngày khác không. Nếu đồng ý: tự động dịch chuyển slot trong lịch tuần và ghi vào hồ sơ. Nếu từ chối: giữ nguyên và không hỏi lại về ngày đó. |
| **BR-AC-07** | Tập luyện | **Signal B3 — Tăng cường độ đột biến:** Kích hoạt khi (i) người dùng ghi nhận ≥ 2 buổi tập trong cùng 1 ngày; HOẶC (ii) trung bình RPE ≥ 8.5 trong ≥ 5 buổi liên tiếp. Hành động: hiển thị cảnh báo Overtraining Risk kèm giải thích ngắn, bắt buộc chèn ít nhất 1 ngày nghỉ vào giáo án kế tiếp, gợi ý 1 buổi Active Recovery (giãn cơ/vận động nhẹ). Người dùng có thể bỏ qua cảnh báo nhưng hệ thống ghi nhận vào hồ sơ để đưa vào báo cáo cuối chu kỳ. |
| **BR-AC-08** | Lộ trình | **Signal B4 — Tiến bộ đình trệ (Plateau):** Kích hoạt khi cả chỉ số thể lực ước tính VÀ điểm Form trung bình không tăng trong **3 tuần liên tiếp** (chỉ tính các tuần có CR ≥ 70% để loại trừ trường hợp tập không đủ). Hành động: AI Coach thông báo đang ở giai đoạn ổn định và đề xuất 1 trong 3 phương án phá plateau do người dùng chọn: (a) Deload Week (giảm tải lượng 40% trong 1 tuần), (b) Variation Swap (thay biến thể bài tập tương đương), (c) Volume Accumulation (tăng số set thay vì tăng cường độ). Áp dụng vào giáo án buổi tiếp theo sau khi có xác nhận. |
| **BR-WL-01** | Buổi tập | **Giới hạn thời gian buổi tập liên tục:** Khi thời gian kể từ Check-in vượt quá **90 phút** đối với người dùng có `experience_level = Chưa từng tập`, hoặc **180 phút** đối với tất cả người dùng khác, hệ thống hiển thị cảnh báo mềm "Bạn đã tập [X] phút — hãy xem xét kết thúc buổi tập". Nếu thời gian tiếp tục vượt **240 phút (4 tiếng)** mà không có tương tác nào từ người dùng, hệ thống tự động kết thúc và lưu buổi tập với nhãn **"Buổi tập bất thường" (Anomalous Session)**. Kết quả vẫn được lưu đầy đủ nhưng: (a) AI Coach bắt buộc xếp giáo án Recovery (phục hồi nhẹ) cho buổi kế tiếp; (b) dữ liệu buổi này bị loại khỏi tính toán Progressive Overload (BR-AC-02) của tuần đó. |
| **BR-WL-02** | Buổi tập | **Phát hiện tải lượng luyện tập bất thường:** Nếu tổng tải lượng luyện tập (Training Load) của một buổi tập — bao gồm tất cả loại hình vận động: bài sức mạnh, bài cardio, bài thể dục với trọng lượng cơ thể — vượt quá **250% trung bình** của 5 buổi gần nhất có cùng nhóm cơ/mục tiêu tương ứng, hệ thống yêu cầu người dùng xác nhận trước khi lưu. Nếu xác nhận, buổi tập được lưu với nhãn "Tải lượng cao bất thường" và AI Coach bắt buộc chèn ít nhất **1 ngày nghỉ hoàn toàn** trước buổi tập tiếp theo tác động vào cùng nhóm cơ/mục tiêu đó.

---

## 6. YÊU CẦU DỮ LIỆU NGHIỆP VỤ (DATA REQUIREMENTS)

Mô tả các luồng thông tin nghiệp vụ vào/ra (Inputs/Outputs) cốt lõi cần quản lý:

### 6.1 Dữ liệu Đầu vào chính (Inputs)
- **User Health Profile**: Họ tên, ngày sinh, chiều cao, cân nặng, tỷ lệ mỡ, danh sách vùng chấn thương, phân loại bệnh lý mãn tính, mục tiêu, trình độ kinh nghiệm (`experience_level`), khung giờ tập cố định. Các trường `equipment_list` (dụng cụ/môi trường tập) và `food_restrictions` (thực phẩm cần tránh/dị ứng) được thu thập dần thông qua hội thoại ngữ cảnh của AI Coach (FR-AC-06) thay vì bắt buộc trong Onboarding, nhằm giảm tải ma sát cho người mới.
- **Real-time Video Stream**: Luồng dữ liệu video camera độ phân giải tối thiểu 720p, tốc độ 30fps.
- **RPE (Rate of Perceived Exertion)**: Đánh giá độ gắng sức chủ quan của người tập sau mỗi set (thang điểm từ 1-10).
- **Meal Logs**: Tên thực phẩm, khối lượng (gram) hoặc số khẩu phần ăn thực tế.

### 6.2 Dữ liệu Đầu ra chính (Outputs)
- **4-Week Roadmap & Weekly Schedule**: Lộ trình 4 tuần với các cột mốc mục tiêu và Lịch tập tuần phân bổ nhóm cơ cần tập cho từng ngày (không bao gồm chi tiết bài tập).
- **Session Workout Details**: Giáo án chi tiết cho từng buổi tập cụ thể bao gồm: danh sách bài tập, số set, rep tiêu chuẩn, cân nặng gợi ý, video hướng dẫn kỹ thuật cho từng bài.
- **Daily Meal Plan**: Thực đơn 3 bữa chính + 1 bữa phụ cá nhân hóa gồm 3 lựa chọn (tiết kiệm, phổ thông, thoải mái) kèm chi tiết macronutrients (Carbs, Protein, Fat, Calories).
- **Posture Correction Alert**: Nhãn cảnh báo lỗi (Visual Overlay) trên màn hình và tệp âm thanh hướng dẫn sửa tư thế.
- **Post-session Report**: Bảng thống kê tổng thể buổi tập bao gồm: tổng thời gian, tổng volume nâng (kg), điểm Form trung bình, lượng calo tiêu thụ ước tính, lỗi tư thế phổ biến và lời khuyên phục hồi từ AI.

---

## 7. QUẢN TRỊ NỘI DUNG & HỆ THỐNG (CONTENT & SYSTEM MANAGEMENT)

*Module dành cho tác nhân ACT-05 (System Administrator). Đảm bảo chất lượng dữ liệu đầu vào cho các Engine AI.*

| Mã yêu cầu | Tên yêu cầu | Mô tả nghiệp vụ chi tiết | Tác nhân | Độ ưu tiên |
|---|---|---|---|---|
| **FR-SM-01** | Quản lý Thư viện Bài tập | Admin có thể thêm, sửa, xóa bài tập trong thư viện hệ thống. Mỗi bài tập bao gồm: tên bài, nhóm cơ kích hoạt chính/phụ, video hướng dẫn kỹ thuật chuẩn, mô hình tọa độ khớp chuẩn (dùng cho AI Camera), danh sách dụng cụ yêu cầu, mức độ khó (1-5) và danh sách bài tập thay thế tương đương. Bài tập chỉ được kích hoạt trên hệ thống sau khi Admin đánh dấu `Đã kiểm duyệt`. | ACT-05 | M |
| **FR-SM-02** | Quản lý Thư viện Thực phẩm & Thực đơn | Admin quản lý danh mục thực phẩm (tên, kcal/100g, macronutrients, các nhãn phân loại: chay, Halal, dị ứng phổ biến) và các mẫu thực đơn gốc. Dữ liệu thực phẩm là nguồn tham chiếu bắt buộc cho AI Nutrition Engine khi sinh gợi ý thực đơn. | ACT-05 | M |
| **FR-SM-03** | Dashboard Giám sát Hệ thống AI | Cung cấp bảng điều khiển thời gian thực cho Admin theo dõi: tỷ lệ nhận diện khung xương thành công của AI Camera, tỷ lệ sinh giáo án thất bại, độ trễ phản hồi cảnh báo tư thế (so với ngưỡng < 500ms trong FR-CC-04), và số lượng buổi tập bị đánh dấu "Không đạt chuẩn xác thực". | ACT-05 | S |

---

## 8. GIẢ ĐỊNH & RÀNG BUỘC (ASSUMPTIONS & CONSTRAINTS)

- **Assumption-01 (Môi trường tập)**: Giả định người dùng tập luyện trong không gian đủ rộng (tối thiểu cách camera 1.5m - 2m) và có đủ ánh sáng để camera nhận diện chính xác khung xương.
- **Assumption-02 (Thiết bị phần cứng)**: Thiết bị di động của người dùng hỗ trợ tối thiểu hệ điều hành iOS 14 hoặc Android 8.0 với camera trước/sau hoạt động bình thường.
- **Assumption-03 (Thu thập thông tin dần)**: Các thông tin không bắt buộc trong Onboarding (dụng cụ tập, thực phẩm cần tránh) được thu thập dần thông qua hội thoại ngữ cảnh. Hệ thống phải luôn có phương án dự phòng (bài tập không cần dụng cụ, thực đơn phổ thông) khi chưa có đủ thông tin.
- **Constraint-01 (Y tế)**: AI Coach và AI Nutrition **không đưa ra lời khuyên hoặc chẩn đoán y khoa**. Mọi cảnh báo hoặc thực đơn chỉ mang tính chất hỗ trợ thể thao nâng cao sức khỏe thông thường.
- **Constraint-02 (Bảo mật)**: Luồng video trực tiếp từ camera của người dùng phải được xử lý on-device (Edge AI) để bảo vệ quyền riêng tư cá nhân tuyệt đối; chỉ gửi các thông số keypoint được trích xuất (tọa độ khớp dạng số) về máy chủ để phân tích dữ liệu lớn.

---

*Tài liệu Đặc tả Yêu cầu Nghiệp vụ Cốt lõi theo chuẩn BABOK v3.0 – Cập nhật lần cuối ngày 02/07/2026*
