# ADR 02: Thiết Kế Quy Trình Lập Lịch Tập Luyện (Workout Planning)

* **Trạng thái**: Approved
* **Tác giả**: Antigravity & Developer

---

## 1. Bối cảnh
AI Coach cần khởi tạo lộ trình 4 tuần, sinh lịch tuần và giáo án chi tiết từng ngày (JIT) cho người dùng. Quy trình này phải tuân thủ nghiêm ngặt các giới hạn an toàn thể thao như Progressive Overload $\le 10\%$ volume và né chấn thương. Chúng ta cần thiết lập một kiến trúc đảm bảo:
- **An toàn số học**: Không xảy ra sai lệch/ảo giác khi tính toán mức tạ, set, rep và volume.
- **Tiết kiệm chi phí**: Giảm thiểu tối đa token tiêu thụ đầu vào/đầu ra của Agent.
- **Tính độc lập của Client**: Client App (Mobile) không cần biết đến sự tồn tại của Agent, chỉ tương tác qua API REST/gRPC tiêu chuẩn với Backend Go.

## 2. Các phương án lựa chọn (Options Considered)
* **Phương án 1 (Agent tự quyết định toàn bộ)**: Agent tự sinh danh sách bài tập, số set, số rep và mức tạ thông qua prompt suy luận. (Dễ ảo giác số học, tính sai volume gây chấn thương, và tốn token rất lớn).
* **Phương án 2 (Phân rã trách nhiệm - Agent chọn bài, Backend tính volume)**:
  - **Agent (Chạy ngầm ở Backend)**: Chỉ chịu trách nhiệm quyết định tập bài gì (What to train) để né tránh vùng chấn thương hoặc thay thế bài tập linh hoạt.
  - **Backend Go**: Lưu trữ các giáo án mẫu (Workout Templates), tự động tính set, rep, tạ dựa trên 1RM của user, thực thi kiểm định `OverloadValidator` ($\le 10\%$ volume) và lưu DB để trả trực tiếp kết quả cho Client.

## 3. Quyết định (Decision)
Chọn **Phương án 2** để tối ưu hóa hiệu năng, bảo đảm an toàn y tế thể thao tuyệt đối bằng code Go, đồng thời ẩn đi sự hiện diện của Agent đối với Client App.

## 4. Hệ quả
* Luồng dữ liệu giữa Client và Backend là luồng JSON có cấu trúc chuẩn. Agent chỉ là một dịch vụ xử lý ngầm (Background Service) được Backend gọi khi cần quyết định bài tập.
* Agent không cần xử lý các con số toán học (set, rep, tạ, volume), giúp prompt cực kỳ ngắn gọn và tiết kiệm token.
* Thuật toán kiểm duyệt Progressive Overload và phân bổ tải lượng được viết và kiểm thử độc lập bằng Unit Test trên Backend Go.
