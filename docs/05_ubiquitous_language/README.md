# 5. Ngôn Ngữ Chung (Ubiquitous Language) - FITAI

Tài liệu này định nghĩa bảng thuật ngữ chung thống nhất giữa các chuyên gia nghiệp vụ, đội ngũ phát triển và mô hình mã nguồn trong dự án **FITAI**, được phân chia theo từng ngữ cảnh nghiệp vụ.

---

## 5.1 User Profile & Health Context (Ngữ cảnh Hồ sơ & Sức khỏe)

* **User Health Profile (Hồ sơ Sức khỏe Người dùng)**: Tập hợp các chỉ số sinh học hiện tại (tuổi, giới tính, chiều cao, cân nặng, tỷ lệ mỡ) cùng với mục tiêu rèn luyện và tiền sử bệnh lý của người dùng.
* **BodyMetricsHistory (Lịch sử Chỉ số Hình thể)**: Tập hợp các bản ghi lịch sử cập nhật cân nặng, tỷ lệ mỡ theo dòng thời gian tập luyện để vẽ biểu đồ tiến trình và làm căn cứ điều chỉnh cho AI Coach/AI Nutrition.
* **MetricsLogEntry (Bản ghi Chỉ số)**: Một điểm dữ liệu đơn lẻ trong lịch sử (ngày ghi nhận, cân nặng, body fat %, ảnh chụp tiến trình).
* **Injury Tracker (Bộ Theo dõi Chấn thương)**: Danh sách chứa các vùng cơ/khớp đang bị chấn thương của người dùng nhằm cung cấp thông tin loại trừ bài tập cho hệ thống.
* **Profile Completion Rate (Tỷ lệ Hoàn thiện Hồ sơ)**: Chỉ số phần trăm đo lường mức độ hoàn tất thông tin sức khỏe bắt buộc. Yêu cầu đạt tối thiểu **$\ge 80\%$** để kích hoạt các tính năng của AI Coach.

---

## 5.2 AI Coaching & Planning Context (Ngữ cảnh Huấn luyện & Lên kế hoạch)

* **Coach Personality (Phong cách Huấn luyện viên)**: Thiết lập tính cách tương tác của AI (Nghiêm khắc, Thân thiện, Khoa học), quyết định giọng văn và từ ngữ trong thông báo/tin nhắn cổ vũ.
* **4-Week Workout Plan (Kế hoạch Tập 4 Tuần)**: Giáo án tập luyện do AI Coach tự động tạo ra cho chu kỳ 4 tuần, chứa lịch các buổi tập, bài tập, số Set, Rep và khối lượng tạ khuyến nghị.
* **Progressive Overload (Tăng tiến Khối lượng)**: Phương pháp tăng dần áp lực (tổng khối lượng tạ - volume) lên cơ bắp qua từng tuần. Giới hạn tăng tối đa **$\le 10\%$** volume so với tuần trước.
* **Deload Week (Tuần Giảm tải)**: Tuần tập luyện giảm cường độ và thể tích (thường giảm 30-50% volume) để cơ thể phục hồi hoàn toàn sau chu kỳ áp lực cao.
* **Injury Avoidance Rule (Quy tắc Tránh Chấn thương)**: Ràng buộc loại bỏ các bài tập gây lực nén lên nhóm cơ/khớp khớp với danh sách chấn thương hiện tại của người dùng.

---

## 5.3 Workout Execution Context (Ngữ cảnh Thực thi Buổi tập)

* **Skeleton Tracking (Theo dõi Khung xương)**: Quá trình xử lý hình ảnh thời gian thực để trích xuất tọa độ 33 điểm khớp trên cơ thể từ luồng video.
* **ROM % (Biên độ Chuyển động)**: Tỷ lệ phần trăm thể hiện biên độ di chuyển của khớp so với góc chuẩn của bài tập.
* **Valid Rep (Lần lặp Hợp lệ)**: Lần thực hiện động tác đáp ứng điều kiện biên độ chuyển động đạt **ROM% $\ge 70\%$**.
* **Audio Ducking (Giảm Âm lượng Nền)**: Hành vi tự động giảm nhỏ âm lượng của nhạc nền đang phát (EDM/Lofi) khi hệ thống phát âm thanh hướng dẫn sửa tư thế của AI Camera.
* **Voice Alert (Cảnh báo Giọng nói)**: Hướng dẫn sửa sai phát qua loa/tai nghe với độ trễ dưới 500ms khi phát hiện lỗi kỹ thuật.
* **Form Score (Điểm Kỹ thuật)**: Điểm số từ 0 đến 100 đánh giá độ chuẩn xác của động tác dựa trên ROM%, sự căn chỉnh khớp và tốc độ thực hiện.
* **Anti-Cheat Validation (Xác thực Chống Gian lận)**: Quy tắc kiểm tra tính hợp lệ của buổi tập; nếu tỷ lệ khung hình nhận diện được khung xương < 50%, buổi tập sẽ không được lưu tự động.
* **Estimated Weight (Cân nặng Ước lượng)**: Khối lượng tạ thực tế được AI Camera nhận dạng thông qua kích thước đĩa tạ và vận tốc nâng.
* **RPE (Rate of Perceived Exertion - Chỉ số gắng sức)**: Đánh giá cảm nhận chủ quan của người tập về độ nặng từ 1 (rất nhẹ) đến 10 (gắng sức tối đa) sau mỗi Set.

---

## 5.4 AI Nutrition Context (Ngữ cảnh Dinh dưỡng AI)

* **TDEE (Total Daily Energy Expenditure - Tổng tiêu hao năng lượng hàng ngày)**: Tổng lượng calo cơ thể tiêu thụ trong một ngày, tính theo công thức Mifflin-St Jeor cộng với hệ số vận động.
* **Anti-Repetition Lockout (Khóa Chống Lặp món)**: Thuật toán hạn chế sự lặp lại của thực đơn (khóa nguồn protein chính trong 7 ngày, tinh bột trong 5 ngày và chủ đề món ăn trong 3 ngày).
* **Budget Tier (Mức Ngân sách)**: Lọc các gợi ý món ăn theo 3 nhóm giá trị kinh tế: Tiết kiệm (Budget), Phổ thông (Standard), và Thoải mái (Comfortable) nhưng vẫn đảm bảo macro dinh dưỡng.
