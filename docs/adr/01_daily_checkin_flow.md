# ADR 01: Quy Trình Hỏi Check-in Hàng Ngày Của AI Coach

* **Trạng thái**: Approved
* **Tác giả**: Antigravity & Developer

---

## 1. Bối cảnh
Cần quyết định cách đặt câu hỏi check-in (hỏi về mệt mỏi, chấn thương, dồn/bù lịch) khi người dùng mở ứng dụng đầu ngày nhằm tối ưu chi phí vận hành và trải nghiệm người dùng.

## 2. Các phương án lựa chọn (Options Considered)
* **Phương án 1 (Agent hỏi qua Chat tự do)**: Người dùng và Agent trò chuyện qua lại từ đầu để thu thập thông tin check-in. (Trải nghiệm tự nhiên nhưng trễ cao, tốn token).
* **Phương án 2 (Kết hợp UI Form & Agent tự quyết định đặt câu hỏi bổ sung)**: 
  - Các câu hỏi cơ bản (mức độ phục hồi, chấn thương mới) dùng Form UI tĩnh để bấm chọn nhanh ($0$ token).
  - Khi phát hiện sự kiện bất thường (nghỉ lâu ngày, quá tải), Agent tự phân tích ngữ cảnh để quyết định đặt thêm câu hỏi tương tác riêng.

## 3. Quyết định (Decision)
Chọn **Phương án 2** để tối ưu hóa chi phí token và tốc độ tải trang cho các ngày tập bình thường, trong khi vẫn cho phép Agent tự quyết định đặt câu hỏi thông minh khi cần.

## 4. Hệ quả
* Backend Go cung cấp API trả về danh sách câu hỏi dạng schema động (gồm câu hỏi tĩnh từ UI và câu hỏi động do Agent tự quyết định).
* Ứng dụng di động cần hỗ trợ render linh hoạt cả form tĩnh lẫn câu hỏi động từ Agent.
