# 1. Phân Tích Nghiệp Vụ (Business Analysis) - FITAI

Tài liệu này phân tích chi tiết bối cảnh, mục tiêu và mô hình hóa quy trình nghiệp vụ cho hệ thống **AI Fitness Social Platform (FITAI)** theo chuẩn **BABOK® Guide V3.0**.

---

## 1.1 Khái Quát Giải Pháp (Solution Scope)
Giải pháp cốt lõi cung cấp một nền tảng số hỗ trợ tập luyện cá nhân hóa tự động dựa trên **Trí tuệ nhân tạo (AI)** và **Thị giác máy tính (Computer Vision)**, kết hợp **Dinh dưỡng cá nhân hóa** nhằm giải quyết hai rào cản lớn nhất của người mới tập:
* **Chi phí cao** khi thuê Huấn luyện viên cá nhân (PT) truyền thống.
* **Thiếu động lực** và **nguy cơ chấn thương** do tập sai kỹ thuật.

---

## 1.2 Mục Tiêu Nghiệp Vụ (Business Objectives)
Các mục tiêu nghiệp vụ được liên kết trực tiếp với nhu cầu chiến lược của doanh nghiệp và người dùng:
* **OB-01 (Accessibility)**: Giảm chi phí tiếp cận hướng dẫn tập luyện chuyên nghiệp xuống 90% so với thuê PT truyền thống.
* **OB-02 (Safety)**: Giảm tỷ lệ chấn thương do tập sai kỹ thuật của người dùng thông qua phân tích góc khớp thời gian thực.
* **OB-03 (Retention)**: Duy trì tỷ lệ người dùng tiếp tục luyện tập sau 30 ngày đạt $\ge 40\%$ nhờ AI Coach đồng hành.
* **OB-04 (Form Standardization)**: Điều chỉnh chuẩn tư thế (form) tập luyện cho người mới tập thông qua hướng dẫn chi tiết và phản hồi sửa lỗi thời gian thực từ AI Camera.
* **OB-05 (Nutrition Optimization)**: Cung cấp thực đơn dinh dưỡng tối ưu và cá nhân hóa sâu theo thể trạng, ngân sách và mục tiêu luyện tập của từng người dùng.

---

## 1.3 Phân Tích Tác Nhân (Stakeholder Analysis)
Các tác nhân tương tác trực tiếp hoặc gián tiếp với hệ thống cốt lõi:

| Mã tác nhân | Tên tác nhân | Mô tả vai trò | Mức độ ảnh hưởng |
|---|---|---|---|
| **ACT-01** | End User (Người tập) | Người sử dụng ứng dụng để lên lịch, tập luyện dưới camera, theo dõi sức khỏe và dinh dưỡng. | High |
| **ACT-02** | AI Coach Engine | Tác nhân hệ thống: Phân tích dữ liệu người dùng, lên lịch, động viên và điều chỉnh giáo án. | High |
| **ACT-03** | AI Camera Engine | Tác nhân hệ thống: Xử lý luồng hình ảnh camera, đếm rep, ước lượng cân nặng, phát hiện lỗi tư thế. | High |
| **ACT-04** | AI Nutrition Engine | Tác nhân hệ thống: Tính toán calo cá nhân hóa và luân chuyển thực đơn không trùng lặp. | Medium |
| **ACT-05** | System Administrator | Quản trị viên hệ thống: Quản lý cấu hình bài tập, kiểm tra dữ liệu và bảo mật. | Low |

---

## 1.4 Quy Trình Nghiệp Vụ Cốt Lõi (Core Business Processes)

### Quy trình 1: Thiết lập Hồ sơ & Khởi tạo Lịch tập (Onboarding & Planning)
1. **Người dùng** nhập Thông tin Cơ bản & Chỉ số Cơ thể (Cân nặng, Chiều cao).
2. **Người dùng** chọn Mục tiêu (Tăng cơ/Giảm mỡ) & Khung giờ tập cố định.
3. **Người dùng** khai báo Sức khỏe (Chấn thương cũ, Bệnh lý mãn tính).
4. **AI Coach** tính toán *User Fitness Score* và thiết lập lịch tập tuần.
5. **AI Coach** xuất Kế hoạch tập 4 tuần đầu & gợi ý thực đơn dinh dưỡng cá nhân hóa.

### Quy trình 2: Luyện tập dưới AI Camera (Workout Execution)
1. **Người dùng** check-in buổi tập & chọn Playlist nhạc mong muốn.
2. **Người dùng** bật Camera & điều chỉnh khoảng cách/ánh sáng theo chỉ dẫn.
3. **AI Camera** tracking skeleton (khung xương) & ước lượng cân nặng tạ thực tế.
4. **AI Camera** đếm số rep, đo lường độ hoàn thiện của rep (ROM %).
   * **Nếu phát hiện lỗi tư thế**: Hệ thống kích hoạt *Audio Ducking* (giảm nhạc nền) và phát tín hiệu giọng nói cảnh báo sửa lỗi (Voice Alert) với độ trễ < 500ms.
   * **Nếu rep hoàn thành đạt chuẩn**: Cộng dồn rep và tính điểm kỹ thuật (Form Score).
5. **Người dùng** xác nhận kết quả Set (hệ thống tự động điền) và tiến hành nghỉ ngơi.
6. **Người dùng** kết thúc buổi tập và nhận Báo cáo tổng hợp sau buổi tập (*Post-session Report*).

---

## 1.5 Giả Định & Ràng Buộc (Assumptions & Constraints)
* **Assumption-01 (Môi trường tập)**: Không gian đủ rộng (cách camera từ 1.5m - 2m) và ánh sáng tốt để nhận diện chính xác khung xương.
* **Assumption-02 (Thiết bị phần cứng)**: Thiết bị di động chạy iOS >= 14 hoặc Android >= 8.0, có camera hoạt động tốt.
* **Constraint-01 (Y tế)**: Hệ thống **không đưa ra chẩn đoán y khoa**. Mọi thông tin chỉ mang tính hỗ trợ nâng cao thể chất.
* **Constraint-02 (Bảo mật)**: Luồng video trực tiếp phải được xử lý on-device (Edge AI) để bảo vệ quyền riêng tư tuyệt đối; chỉ gửi các tọa độ khớp dạng số về server để phân tích dữ liệu lớn.
