# ARCHITECTURE DECISION RECORD (ADR)
## ADR-01: Single Workout Context Update Tool and Warm-up Rendering with JIT Streaming

*   **Status**: Accepted
*   **Date**: 2026-07-13
*   **Authors**: Lead Architect & AI Coach Team

---

### 1. Problem (Vấn đề)
Trong thiết kế ban đầu của cấu phần AI Coach (FITAI), AI Agent cần thực hiện nhiều hành động nối tiếp khi người dùng check-in đầu buổi tập (như báo chấn thương mới, khớp vừa phục hồi hoặc dụng cụ thay đổi). 
Việc phân rã API thành các Tool nhỏ lẻ tuân thủ CQRS (`UpdateInjuryStatus`, `UpdateUserEquipments`, `SearchExercises`) tạo ra rào cản lớn về mặt hiệu năng:
*   Mỗi cuộc gọi Tool (Function Calling) khứ hồi từ LLM lên Backend tốn trung bình 1.5 - 2.5 giây.
*   Việc gọi nối tiếp (chaining) 3 Tool liên tiếp làm tăng tổng thời gian chờ của người dùng lên **6 - 8 giây** trước khi buổi tập có thể bắt đầu, gây ức chế và làm gián đoạn nhịp độ tập luyện của user.

---

### 2. Options (Các lựa chọn)

#### Option 1: Giữ nguyên thiết kế CQRS truyền thống (Nhiều Tool Mutation nhỏ lẻ)
*   *Mô tả*: AI Agent gọi lần lượt `UpdateInjuryStatus` -> đợi Backend lưu DB -> gọi `UpdateUserEquipments` -> đợi Backend lưu DB -> gọi `SearchExercises` để lấy bài tập.
*   *Pros (Ưu điểm)*: Ranh giới API cực kỳ rõ ràng, tách biệt trách nhiệm tuyệt đối.
*   *Cons (Nhược điểm)*: Latency khứ hồi quá cao (6 - 8 giây), trải nghiệm người dùng tệ.

#### Option 2: Gom các Tool Mutation và gọi song song (Hợp nhất Context Update)
*   *Mô tả*: Gom chấn thương mới (`avoid_joints`), khớp đã khỏi (`recovered_joints`) và thiết bị ghi đè đột xuất (`override_equipments`) vào đúng 1 Tool Command duy nhất `UpdateWorkoutContext`. Cấu hình cho phép Agent gọi song song Tool này với Tool Query `SearchExercises` để rút ngắn thời gian.
*   *Pros (Ưu điểm)*: Giảm số vòng gọi Tool của Agent từ 3 xuống 1-2 vòng. Giữ vững ranh giới CQRS (Write tách biệt Read), code Backend Go gọn gàng và dễ bảo trì.
*   *Cons (Nhược điểm)*: Đòi hỏi Agent phải trích xuất NLP chính xác nhiều thực thể cùng lúc để gửi trong 1 payload duy nhất.

#### Option 3: Kết hợp Mutation và Query vào chung 1 API (Hybrid Search)
*   *Mô tả*: Cho phép API `SearchExercises` nhận luôn các tham số chấn thương và thiết bị để vừa cập nhật DB, vừa trả về danh sách bài tập đã lọc trong cùng 1 request duy nhất.
*   *Pros (Ưu điểm)*: Tốc độ phản hồi nhanh nhất (chỉ tốn đúng 1 cuộc gọi Tool duy nhất).
*   *Cons (Nhược điểm)*: Vi phạm nghiêm trọng CQRS (API Query có tác dụng phụ làm thay đổi DB), dễ gây lỗi mâu thuẫn trạng thái nếu Agent gọi lại nhiều lần trong cùng 1 session.

---

### 3. Decision (Quyết định)
Chúng ta quyết định chọn **Option 2 (Gom các Tool Mutation thành `UpdateWorkoutContext` và hỗ trợ gọi song song)**.

Đồng thời, kết hợp với cơ chế **Warm-up Rendering & NDJSON Streaming**:
*   Khi có thay đổi ngữ cảnh, Backend Go chạy ngầm sinh lại cache mới.
*   Nếu thay đổi phút chót làm lệch cache: Client hiển thị ngay bài Warm-up cũ không bị load lại (0ms) để user bắt đầu tập. Trong lúc user khởi động, Backend mở kết nối SSE và stream progressive từng bài tập chính mới (dạng NDJSON) do Agent sinh về Client.

---

### 4. Consequences (Hệ quả)

#### Pros (Ưu điểm):
*   **UX Perceived Latency = 0ms**: Người dùng được vào tập ngay bài Warm-up cũ, hoàn toàn không nhận ra thời gian chờ hệ thống stream giáo án chính mới ở background.
*   **Tối giản và An toàn**: Đưa việc check cache về 1 phép so sánh đơn giản (sự tồn tại của bản ghi), bỏ qua băm `context_hash` phức tạp.
*   **Giao tiếp Sạch sẽ**: Giữ được ranh giới CQRS ở mức chấp nhận được, code API rõ ràng.

#### Cons (Nhược điểm):
*   Payload JSON của Tool `UpdateWorkoutContext` phình to hơn do chứa nhiều tham số của cả chấn thương và thiết bị.
