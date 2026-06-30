# AI Fitness Social Platform (FITAI) - Gym Companion

[![BABOK v3.0 Compliant](https://img.shields.io/badge/BABOK-v3.0--Compliant-blue.svg)](docs/NGHIEP_VU_COT_LOI_BABOK.md)
[![License: MIT](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

**FITAI** (Gym Companion) là một nền tảng số hỗ trợ tập luyện cá nhân hóa tự động dựa trên **Trí tuệ nhân tạo (AI)** và **Thị giác máy tính (Computer Vision)**, kết hợp **Dinh dưỡng cá nhân hóa** nhằm tối ưu hóa việc tập luyện, giảm thiểu chấn thương và duy trì động lực cho người tập, đặc biệt là người mới bắt đầu.

Tài liệu nghiệp vụ cốt lõi của dự án tuân thủ nghiêm ngặt theo chuẩn **BABOK® Guide v3.0**.

---

## 🌟 Tính Năng Cốt Lõi (Core Modules)

Hệ thống được thiết kế xoay quanh 6 nhóm nghiệp vụ cốt lõi:

1. **Quản lý Hồ sơ Sức khỏe & Thiết lập Lịch trình (`User & Profile`)**:
   * Khai báo hồ sơ sức khỏe chi tiết ($\ge 80\%$ hoàn thiện để kích hoạt AI Coach).
   * Lên lịch tập luyện tự động và gửi thông báo nhắc nhở thông minh trước 15 phút.
2. **Huấn luyện viên ảo (`AI Coach Engine`)**:
   * Tự động khởi tạo giáo án tập luyện 4 tuần cá nhân hóa dựa trên chỉ số cơ thể, mục tiêu và hạn chế chấn thương.
   * Tự động điều chỉnh cường độ tập luyện định kỳ mỗi 2 tuần (Progressive Overload giới hạn $\le 10\%$ volume).
   * Đề xuất bài tập thay thế ngay lập tức khi phát hiện chấn thương của người dùng.
3. **Giám sát tư thế thời gian thực (`AI Camera Coach`)**:
   * Tracking khung xương 33 điểm khớp thông qua camera (Edge AI - xử lý trên thiết bị để đảm bảo quyền riêng tư).
   * Đo lường biên độ chuyển động (ROM) và phát hiện lỗi tư thế (võng lưng, gối chụm,...) với độ trễ $< 500$ms.
   * Cảnh báo âm thanh (Audio Ducking) và hình ảnh (Visual Overlay) thời gian thực.
4. **Ghi nhận tập luyện & Tương tác âm thanh (`Workout Logging & Smart Audio`)**:
   * Tự động đếm số lần lặp (Rep) hợp lệ ($\ge 70\%$ ROM tiêu chuẩn) và ước lượng cân nặng tạ thực tế chống gian lận.
   * Hệ thống âm thanh thông minh tự động giảm nhạc nền khi AI Coach phát giọng nói hướng dẫn tư thế.
5. **Dinh dưỡng thông minh (`AI Nutrition Engine`)**:
   * Tính toán TDEE và macronutrients hàng ngày riêng biệt theo thể trạng và mức độ vận động thực tế.
   * Thuật toán **Anti-Repetition**: Khóa và không lặp lại nguồn protein trong 7 ngày, tinh bột trong 5 ngày để chống gây ngán.
   * Đề xuất thực đơn đa dạng phân cấp theo 3 mức ngân sách (Tiết kiệm, Phổ thông, Thoải mái).
6. **Theo dõi tiến trình & Phân tích (`Progress Tracking & Analytics`)**:
   * Biểu đồ xu hướng biến động chỉ số cơ thể, sức mạnh (1RM) và điểm kỹ thuật (Form Score).

---

## 📊 Mục Tiêu Nghiệp Vụ (Business Objectives)

* **OB-01 (Accessibility)**: Giảm chi phí tiếp cận hướng dẫn tập luyện chuyên nghiệp xuống **90%** so với thuê PT truyền thống.
* **OB-02 (Safety)**: Giảm tỷ lệ chấn thương do tập sai kỹ thuật nhờ hệ thống phân tích góc khớp thời gian thực.
* **OB-03 (Retention)**: Đạt tỷ lệ duy trì người dùng tiếp tục luyện tập sau 30 ngày $\ge 40\%$.
* **OB-04 (Form Standardization)**: Điều chỉnh chuẩn tư thế tập luyện cho người mới bắt đầu.
* **OB-05 (Nutrition Optimization)**: Cung cấp thực đơn dinh dưỡng tối ưu và cá nhân hóa sâu theo thể trạng và mục tiêu.

## 🔧 Cài Đặt Git Hooks Cho Nhà Phát Triển (Developer Setup)

Để đảm bảo các commit và Pull Request của bạn tuân thủ đúng chuẩn định dạng (Gitmoji + Conventional Commits), vui lòng chạy script cài đặt Local Git Hook ngay sau khi clone dự án về máy:

```bash
bash scripts/install-hooks.sh
```

Lệnh này sẽ tự động đăng ký hook `commit-msg` vào thư mục `.git/hooks/` trên máy của bạn để tự động kiểm tra thông điệp mỗi khi bạn chạy lệnh `git commit`.

---

## 📂 Cấu Trúc Thư Mục Dự Án

```text
├── .agents/                    # Cấu hình AI Assistant & Custom Skills
│   └── skills/                 # Các kỹ năng tự động hóa (git-commit, pr-creator,...)
├── docs/                       # Tài liệu đặc tả nghiệp vụ & thiết kế hệ thống
│   └── NGHIEP_VU_COT_LOI_BABOK.md  # Tài liệu Đặc tả Yêu cầu Nghiệp vụ Cốt lõi (BABOK v3.0)
├── .gitignore                  # Cấu hình bỏ qua các file không cần thiết trong Git
└── README.md                   # Tài liệu giới thiệu tổng quan dự án
```

---

## 🛠️ Yêu Cầu Hệ Thống & Ràng Buộc

* **Môi trường tập**: Người dùng tập luyện trong không gian rộng (cách camera 1.5m - 2m) và đủ ánh sáng.
* **Thiết bị phần cứng**: Hỗ trợ tối thiểu iOS 14 hoặc Android 8.0 với camera trước/sau hoạt động bình thường.
* **Ràng buộc bảo mật**: Luồng video trực tiếp từ camera được xử lý hoàn toàn trên thiết bị (Edge AI); chỉ gửi các tọa độ khung xương dạng số về server để bảo mật quyền riêng tư của người dùng.
* **Ràng buộc y khoa**: Nền tảng không đưa ra chẩn đoán hoặc lời khuyên y khoa. Mọi khuyến nghị chỉ mang tính chất hỗ trợ nâng cao thể chất thể thao thông thường.