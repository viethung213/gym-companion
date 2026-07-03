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
1. **Người dùng** nhập Thông tin Cơ bản & Chỉ số Cơ thể tối giản (Tuổi, Giới tính, Chiều cao, Cân nặng).
2. **Người dùng** chọn Mục tiêu (Tăng cơ/Giảm mỡ) & Khung giờ tập cố định.
3. **Người dùng** khai báo Sức khỏe ban đầu (Chấn thương cũ, Bệnh lý mãn tính).
   * *Lưu ý*: Các thông tin chi tiết về dụng cụ tập luyện (`equipment_list`) và dị ứng thực phẩm (`food_restrictions`) không bắt buộc khai báo tại bước này mà sẽ được chatbot tự động hỏi dần theo ngữ cảnh khi sử dụng tính năng tập luyện và gợi ý dinh dưỡng.
4. **AI Coach** tính toán *User Fitness Score* ban đầu.
5. **AI Coach** khởi tạo **Lộ trình tổng quan 4 tuần** (khung định hướng phân bổ) & **Lịch tập tuần** (phân bổ nhóm cơ tập/nghỉ). Hệ thống **không** sinh chi tiết bài tập/set/rep/tạ ở bước này.
6. **AI Nutrition** gợi ý thực đơn dinh dưỡng cơ bản.

### Quy trình 2: Luyện tập (Workout Execution)
1. **Người dùng** thực hiện check-in buổi tập & chọn Playlist nhạc mong muốn.
2. Hệ thống kiểm tra hình thức tập luyện và rẽ nhánh:
   * **Nhánh tập dưới AI Camera (Nhánh AI)**:
     - Người dùng bật Camera và điều chỉnh khoảng cách (1.5m - 2m) / ánh sáng.
     - **AI Camera** tracking skeleton (33 điểm khớp), ước lượng cân nặng tạ thực tế.
     - **AI Camera** đếm số rep đạt tiêu chuẩn (ROM% $\ge 70\%$).
     - Nếu phát hiện lỗi tư thế: Kích hoạt *Audio Ducking* (giảm nhạc nền) và phát tín hiệu giọng nói cảnh báo sửa lỗi (Voice Alert) với độ trễ < 500ms.
     - Nếu rep đạt chuẩn: Cộng rep và tính điểm kỹ thuật (Form Score).
     - Người dùng xác nhận kết quả Set (hệ thống tự điền tự động) và nghỉ ngơi.
   * **Nhánh tập tự ghi nhận (Nhánh phi AI)**:
     - Giao diện hiển thị trình bấm giờ (timer) đếm ngược theo Set hoặc thời gian nghỉ.
     - Phát nhạc nền và hiển thị video/hướng dẫn bài tập trực quan để người dùng thực hiện theo.
     - Người dùng tự tập luyện và ghi nhận thủ công kết quả Set (số rep, mức tạ thực tế).
     - Kết quả tập phi AI không có điểm Form Score (báo N/A/Trống) nhưng vẫn ghi nhận số set, rep, tạ để tính toán Tải lượng tập luyện (Training Load) và Overload.
3. Người dùng kết thúc buổi tập và nhận Báo cáo tổng hợp sau buổi tập (*Post-session Report*) bao gồm tổng volume, calo tiêu thụ, và (nếu tập dưới camera) điểm Form trung bình kèm lỗi phổ biến.

### Quy trình 3: Sinh giáo án chi tiết theo buổi (Just-In-Time Workout Generation)
1. **Trigger**: Đến ngày tập theo lịch hoặc khi người dùng mở ứng dụng.
2. **AI Coach** đặt câu hỏi ngắn hoặc kiểm tra trạng thái sức khỏe hiện tại (chấn thương mới, độ hồi phục) và các thông tin thiết bị khả dụng (nếu chưa có).
3. **AI Coach** phân tích dữ liệu hiệu năng tập luyện gần nhất (RPE và Form Score của buổi tập trước).
4. **AI Coach** tự động sinh giáo án chi tiết hôm nay (danh sách bài tập, số set, rep mục tiêu, mức tạ gợi ý).
5. **AI Coach** tự động chèn các bài tập Khởi động (Warm-up - 5-10 phút) và Giãn cơ (Cool-down - 5 phút) phù hợp với nhóm cơ tập của giáo án hôm nay.
6. Người dùng nhận giáo án chi tiết và bắt đầu thực hiện (chuyển sang Quy trình 2).

### Quy trình 4: Đánh giá & Điều chỉnh Lộ trình thích ứng (Adaptive Review Cycle)
1. **Trigger A (Cuối chu kỳ 4 tuần)**:
   - AI Coach tính toán Tỷ lệ hoàn thành lịch tập (Completion Rate - CR) của cả chu kỳ.
   - Áp dụng quy tắc nâng/hạ hoặc cấu hình lại lộ trình 4 tuần kế tiếp theo quy tắc **BR-AC-04** (CR < 40%, 40-70%, 70-90%, >=90%).
2. **Trigger B (Giữa chu kỳ - Hướng sự kiện)**:
   - Hệ thống liên tục giám sát hiệu suất và các tín hiệu hành vi độc lập của người dùng để đề xuất điều chỉnh nhanh giáo án:
     - *Tín hiệu B1 (Không tập 7 ngày)*: Check-in và đề xuất tiếp tục từ buổi bỏ qua, đặt lại lịch tuần hoặc tạm dừng (Pause).
     - *Tín hiệu B2 (Lịch không tương thích)*: Bỏ tập cùng ngày $\ge 3$ lần liên tiếp -> đề xuất đổi ngày tập sang slot khác.
     - *Tín hiệu B3 (Quá tải - Overtraining)*: Tập $\ge 2$ buổi/ngày hoặc RPE trung bình $\ge 8.5$ liên tục $\ge 5$ buổi -> Cảnh báo quá tải, chèn ngày nghỉ bắt buộc trong tuần tới.
     - *Tín hiệu B4 (Tiến bộ đình trệ - Plateau)*: Sức mạnh và Form trung bình không tăng trong 3 tuần liên tiếp (khi CR $\ge 70\%$) -> đề xuất Deload Week (giảm 40% tải lượng), đổi biến thể bài tập hoặc tăng set.

---

## 1.5 Giả Định & Ràng Buộc (Assumptions & Constraints)
* **Assumption-01 (Môi trường tập)**: Không gian đủ rộng (cách camera từ 1.5m - 2m) và ánh sáng tốt để nhận diện chính xác khung xương.
* **Assumption-02 (Thiết bị phần cứng)**: Thiết bị di động chạy iOS >= 14 hoặc Android >= 8.0, có camera hoạt động tốt.
* **Assumption-03 (Thu thập thông tin dần)**: Các thông tin phụ trợ (thiết bị, dị ứng) được hỏi dần qua hội thoại. Hệ thống luôn có phương án dự phòng (bài không dụng cụ, thực đơn phổ thông) khi thiếu dữ liệu.
* **Constraint-01 (Y tế)**: Hệ thống **không đưa ra chẩn đoán hoặc lời khuyên y khoa**. Mọi thông tin chỉ mang tính hỗ trợ nâng cao thể chất.
* **Constraint-02 (Bảo mật)**: Luồng video trực tiếp phải được xử lý on-device (Edge AI) để bảo vệ quyền riêng tư tuyệt đối; chỉ gửi các tọa độ khớp dạng số về server để phân tích dữ liệu lớn.
* **Constraint-03 (Giới hạn thời gian tập - BR-WL-01)**: Cảnh báo kết thúc buổi tập sau 90 phút (người mới) / 180 phút (người cũ). Tự động đóng sau 240 phút không tương tác (nhãn `Anomalous Session` - loại bỏ khỏi tính Overload).
* **Constraint-04 (Tải lượng bất thường - BR-WL-02)**: Tải lượng buổi tập vượt 250% trung bình 5 buổi gần nhất có cùng nhóm cơ/mục tiêu -> Yêu cầu người dùng xác nhận và bắt buộc chèn ít nhất 1 ngày nghỉ hoàn toàn cho nhóm cơ đó.
