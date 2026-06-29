# ĐẶC TẢ YÊU CẦU NGHIỆP VỤ CỐT LÕI (CORE REQUIREMENTS SPECIFICATION)
## DỰ ÁN: AI FITNESS SOCIAL PLATFORM (FITAI)
### TIÊU CHUẨN ĐẶC TẢ: BABOK® GUIDE V3.0 Compliant

---

## KIỂM SOÁT TÀI LIỆU (DOCUMENT CONTROL)

| Thông tin | Chi tiết |
|---|---|
| **Mã tài liệu** | FITAI-BRD-CORE-001 |
| **Phiên bản** | 1.2.0 |
| **Ngày hiệu lực** | 29/06/2026 |
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
[AI Coach] ─────► Xuất Kế hoạch tập 4 tuần đầu & Gợi ý thực đơn cá nhân hóa
```

### 3.2 Quy trình Luyện tập dưới AI Camera (Workout Execution)

```
[Người dùng] ──► Check-in buổi tập & Thiết lập Audio/Playlist nhạc mong muốn
     │
     ▼
[Người dùng] ──► Bật Camera & Điều chỉnh khoảng cách/Ánh sáng theo chỉ dẫn
     │
     ▼
[AI Camera] ────► Tracking skeleton (Khung xương) & Ước lượng cân nặng tạ thực tế
     │
     ▼
[AI Camera] ────► Đếm số rep, đo lường % hoàn thiện rep (ROM %)
     ├── (Lỗi tư thế phát hiện) ─► [Hệ thống] ──► Audio Ducking & Voice alert sửa lỗi
     └── (Rep hoàn thành đạt chuẩn) ─► [Hệ thống] ──► Cộng dồn rep và tính điểm Form
     │
     ▼
[Người dùng] ──► Xác nhận kết quả Set (AI tự động điền) → Nghỉ ngơi
     │
     ▼
[Người dùng] ──► Kết thúc buổi tập → Nhận Post-session Report
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
| **FR-AC-01** | Khởi tạo Lịch tập 4 tuần | AI Coach phân tích chỉ số cơ thể, mục tiêu và hạn chế chấn thương để tự động sinh giáo án 4 tuần đầu bao gồm bài tập, số set, rep và cân nặng gợi ý. | ACT-02 | M |
| **FR-AC-02** | Tự động điều chỉnh kế hoạch | AI Coach tự động phân tích dữ liệu hiệu năng sau mỗi 2 tuần để tinh chỉnh: tăng/giảm cường độ, thay thế bài tập hoặc chèn Deload Week (tuần giảm tải). | ACT-02 | M |
| **FR-AC-03** | Đề xuất bài tập thay thế | Khi người dùng báo chấn thương đột xuất hệ thống sẽ loại bỏ các bài tập tác động trực tiếp vào nhóm cơ bị chấn thương đó cho đến khi nhận được thông báo đã hồi phục từ người dùng. | ACT-02, ACT-01 | S |
| **FR-AC-04** | Đồng hành & Cổ vũ | AI Coach gửi tin nhắn động viên cá nhân hóa dựa trên dữ liệu thật (VD: khi người dùng vượt PR hoặc khi nghỉ dài ngày quay trở lại). | ACT-02 | S |
| **FR-AC-05** | Thiết lập phong cách Coach | Người dùng có quyền lựa chọn phong cách tương tác của Coach: Drill Sergeant (nghiêm khắc), Best Friend (thân thiện), hoặc Data Analyst (khoa học). | ACT-01 | C |

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

---

## 6. YÊU CẦU DỮ LIỆU NGHIỆP VỤ (DATA REQUIREMENTS)

Mô tả các luồng thông tin nghiệp vụ vào/ra (Inputs/Outputs) cốt lõi cần quản lý:

### 6.1 Dữ liệu Đầu vào chính (Inputs)
- **User Health Profile**: Họ tên, ngày sinh, chiều cao, cân nặng, tỷ lệ mỡ, danh sách vùng chấn thương, phân loại bệnh lý mãn tính, mục tiêu, trình độ, khung giờ tập cố định.
- **Real-time Video Stream**: Luồng dữ liệu video camera độ phân giải tối thiểu 720p, tốc độ 30fps.
- **RPE (Rate of Perceived Exertion)**: Đánh giá độ gắng sức chủ quan của người tập sau mỗi set (thang điểm từ 1-10).
- **Meal Logs**: Tên thực phẩm, khối lượng (gram) hoặc số khẩu phần ăn thực tế.

### 6.2 Dữ liệu Đầu ra chính (Outputs)
- **4-Week Workout Plan**: Giáo án gồm ngày tập, bài tập, set, reps tiêu chuẩn, cân nặng gợi ý, video hướng dẫn kỹ thuật.
- **Daily Meal Plan**: Thực đơn 3 bữa chính + 1 bữa phụ cá nhân hóa gồm 3 lựa chọn (tiết kiệm, phổ thông, thoải mái) kèm chi tiết macronutrients (Carbs, Protein, Fat, Calories).
- **Posture Correction Alert**: Nhãn cảnh báo lỗi (Visual Overlay) trên màn hình và tệp âm thanh hướng dẫn sửa tư thế.
- **Post-session Report**: Bảng thống kê tổng thể buổi tập bao gồm: tổng thời gian, tổng volume nâng (kg), điểm Form trung bình, lượng calo tiêu thụ ước tính, lỗi tư thế phổ biến và lời khuyên phục hồi từ AI.

---

## 7. GIẢ DỊNH & RÀNG BUỘC (ASSUMPTIONS & CONSTRAINTS)

- **Assumption-01 (Môi trường tập)**: Giả định người dùng tập luyện trong không gian đủ rộng (tối thiểu cách camera 1.5m - 2m) và có đủ ánh sáng để camera nhận diện chính xác khung xương.
- **Assumption-02 (Thiết bị phần cứng)**: Thiết bị di động của người dùng hỗ trợ tối thiểu hệ điều hành iOS 14 hoặc Android 8.0 với camera trước/sau hoạt động bình thường.
- **Constraint-01 (Y tế)**: AI Coach và AI Nutrition **không đưa ra lời khuyên hoặc chẩn đoán y khoa**. Mọi cảnh báo hoặc thực đơn chỉ mang tính chất hỗ trợ thể thao nâng cao sức khỏe thông thường.
- **Constraint-02 (Bảo mật)**: Luồng video trực tiếp từ camera của người dùng phải được xử lý on-device (Edge AI) để bảo vệ quyền riêng tư cá nhân tuyệt đối; chỉ gửi các thông số keypoint được trích xuất (tọa độ khớp dạng số) về máy chủ để phân tích dữ liệu lớn.

---

*Tài liệu Đặc tả Yêu cầu Nghiệp vụ Cốt lõi theo chuẩn BABOK v3.0 – Phê duyệt ngày 29/06/2026*
