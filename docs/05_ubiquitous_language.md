# 5. Ngôn Ngữ Chung (Ubiquitous Language) - FITAI

Tài liệu này định nghĩa bảng thuật ngữ chung thống nhất giữa các chuyên gia nghiệp vụ, đội ngũ phát triển và mô hình mã nguồn trong dự án **FITAI**, được phân chia theo từng ngữ cảnh nghiệp vụ.

---

## 5.1 User Profile & Health Context (Ngữ cảnh Hồ sơ & Sức khỏe)

* **Minimal Onboarding (Onboarding Tối Giản)**: Quy trình đăng ký ban đầu thu gọn, chỉ thu thập các chỉ số sinh học bắt buộc (tuổi, giới tính, chiều cao, cân nặng, mục tiêu, chấn thương cũ) để giảm ma sát trải nghiệm, các thông tin khác sẽ được thu thập dần sau đó.
* **User Health Profile (Hồ sơ Sức khỏe Người dùng)**: Tập hợp các chỉ số sinh học hiện tại cùng với mục tiêu rèn luyện và tiền sử bệnh lý của người dùng.
* **BodyMetricsHistory (Lịch sử Chỉ số Hình thể)**: Tập hợp các bản ghi lịch sử cập nhật cân nặng, tỷ lệ mỡ theo dòng thời gian tập luyện để vẽ biểu đồ tiến trình và làm căn cứ điều chỉnh cho AI Coach/AI Nutrition.
* **MetricsLogEntry (Bản ghi Chỉ số)**: Một điểm dữ liệu đơn lẻ trong lịch sử (ngày ghi nhận, cân nặng, body fat %, ảnh chụp tiến trình).
* **Injury Tracker (Bộ Theo dõi Chấn thương)**: Danh sách chứa các vùng cơ/khớp đang bị chấn thương của người dùng nhằm cung cấp thông tin loại trừ bài tập cho hệ thống.
* **Profile Completion Rate (Tỷ lệ Hoàn thiện Hồ sơ)**: Chỉ số phần trăm đo lường mức độ hoàn tất thông tin sức khỏe bắt buộc. Yêu cầu đạt tối thiểu **$\ge 80\%$** để kích hoạt các tính năng của AI Coach.

---

## 5.2 AI Coaching & Planning Context (Ngữ cảnh Huấn luyện & Lên kế hoạch)

* **Coach Personality (Phong cách Huấn luyện viên)**: Thiết lập tính cách tương tác của AI ( Drill Sergeant, Best Friend, Data Analyst), quyết định giọng văn và từ ngữ trong thông báo/tin nhắn cổ vũ.
* **Workout Roadmap (Lộ Trình Tổng Quan 4 Tuần)**: Khung kế hoạch tập luyện định hướng cho chu kỳ 4 tuần tiếp theo, chỉ xác định số buổi tập, ngày nghỉ và phân bổ nhóm cơ tập luyện (Weekly Split) cho từng ngày, không bao gồm chi tiết bài tập.
* **Just-In-Time (JIT) Workout Generation (Sinh Giáo Án JIT)**: Cơ chế tự động sinh chi tiết bài tập, set, rep, tạ gợi ý và khởi động/giãn cơ hàng ngày ngay tại thời điểm người dùng bắt đầu buổi tập/mở app, dựa trên phân tích sức khỏe, chấn thương và hiệu năng thực tế của buổi tập trước.
* **Completion Rate - CR (Tỷ lệ Hoàn thành)**: Tỷ số phần trăm giữa số buổi tập thực tế đã hoàn thành và số buổi tập dự kiến trong chu kỳ 4 tuần, làm căn cứ để Trigger A điều chỉnh lộ trình tiếp theo.
* **Progressive Overload (Tăng tiến Khối lượng)**: Phương pháp tăng dần áp lực (tổng khối lượng tạ - volume) lên cơ bắp qua từng tuần. Giới hạn tăng tối đa **$\le 10\%$** volume so với tuần trước.
* **Deload Week (Tuần Giảm tải)**: Tuần tập luyện giảm cường độ và thể tích (thường giảm 40% volume) để cơ thể phục hồi hoàn toàn sau chu kỳ áp lực cao.
* **Injury Avoidance Rule (Quy tắc Tránh Chấn thương)**: Ràng buộc loại bỏ các bài tập gây lực nén lên nhóm cơ/khớp khớp với danh sách chấn thương hiện tại của người dùng.

---

## 5.3 Workout Execution Context (Ngữ cảnh Thực thi Buổi tập)

* **Skeleton Tracking (Theo dõi Khung xương)**: Quá trình xử lý hình ảnh thời gian thực để trích xuất tọa độ 33 điểm khớp trên cơ thể từ luồng video tại Client (Edge AI).
* **ROM % (Biên độ Chuyển động)**: Tỷ lệ phần trăm thể hiện biên độ di chuyển của khớp so với góc chuẩn của bài tập.
* **Valid Rep (Lần lặp Hợp lệ)**: Lần thực hiện động tác đáp ứng điều kiện biên độ chuyển động đạt **ROM% $\ge 70\%$**.
* **Audio Ducking (Giảm Âm lượng Nền)**: Hành vi tự động giảm nhỏ âm lượng của nhạc nền đang phát (EDM/Lofi) xuống 20% khi hệ thống phát âm thanh hướng dẫn sửa tư thế của AI Camera (thực thi trên client).
* **Voice Alert (Cảnh báo Giọng nói)**: Hướng dẫn sửa sai phát qua loa/tai nghe với độ trễ dưới 500ms khi phát hiện lỗi kỹ thuật.
* **Form Score (Điểm Kỹ thuật)**: Điểm số từ 0 đến 100 đánh giá độ chuẩn xác của động tác dựa trên ROM%, sự căn chỉnh khớp và tốc độ thực hiện.
* **Anti-Cheat Validation (Xác thực Chống Gian lận)**: Quy tắc kiểm tra tính hợp lệ của buổi tập; nếu tỷ lệ khung hình nhận diện được khung xương < 50%, buổi tập sẽ không được lưu tự động (chỉ áp dụng khi dùng AI Camera).
* **Estimated Weight (Cân nặng Ước lượng)**: Khối lượng tạ thực tế được AI Camera nhận dạng thông qua kích thước đĩa tạ và vận tốc nâng.
* **Non-AI Workout (Buổi Tập Phi AI)**: Chế độ tập luyện tự ghi chép kết quả set tập thủ công, kết hợp trình bấm giờ (timer), nhạc nền và video demo mà không cần bật camera tracking. Điểm Form Score của buổi tập này sẽ để trống (N/A).
* **Anomalous Session (Buổi Tập Bất Thường)**: Buổi tập vượt quá 240 phút không có tương tác, bị hệ thống tự động đóng, gắn nhãn Anomalous và loại bỏ khỏi việc tính toán Overload tuần sau.
* **Abnormal Training Load (Tải Lượng Bất Thường)**: Trạng thái volume buổi tập tăng vượt quá 250% so với trung bình 5 buổi gần nhất của cùng nhóm cơ, kích hoạt cảnh báo an toàn.
* **RPE (Rate of Perceived Exertion - Chỉ số gắng sức)**: Đánh giá cảm nhận chủ quan của người tập về độ nặng từ 1 (rất nhẹ) đến 10 (gắng sức tối đa) sau mỗi Set.

---

## 5.4 AI Nutrition Context (Ngữ cảnh Dinh dưỡng AI)

* **TDEE (Total Daily Energy Expenditure - Tổng tiêu hao năng lượng hàng ngày)**: Tổng lượng calo cơ thể tiêu thụ trong một ngày, tính theo công thức Mifflin-St Jeor cộng với hệ số vận động.
* **Anti-Repetition Lockout (Khóa Chống Lặp món)**: Thuật toán hạn chế sự lặp lại của thực đơn (khóa nguồn protein chính trong 7 ngày, tinh bột trong 5 ngày và chủ đề món ăn trong 3 ngày).
* **Budget Tier (Mức Ngân sách)**: Lọc các gợi ý món ăn theo 3 nhóm giá trị kinh tế: Tiết kiệm (Budget), Phổ thông (Standard), và Thoải mái (Comfortable) nhưng vẫn đảm bảo macro dinh dưỡng.
* **Dining-Out Meal Suggestion (Tư Vấn Món Ăn Ngoài)**: Tính năng gợi ý món ăn phù hợp với calo/macro tại các nhà hàng, quán ăn ngoài tiệm, ưu tiên liên kết với các đối tác cung cấp suất ăn tiện lợi.
