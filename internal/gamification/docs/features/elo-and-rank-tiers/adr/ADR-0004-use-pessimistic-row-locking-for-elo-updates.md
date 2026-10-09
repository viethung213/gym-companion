# ADR-0004: Sử Dụng Khóa Dòng Bi Quan (SELECT FOR UPDATE) Cho Cập Nhật Điểm ELO

- **Feature**: elo-and-rank-tiers
- **Date**: 2026-10-09
- **Status**: Accepted
- **Deciders**: Maintainer (User), AI Assistant

## Context and Problem Statement
Trong các hệ thống phân tán, các sự kiện cập nhật điểm ELO có thể xuất hiện đồng thời trên cùng một tài khoản người dùng (ví dụ: Worker chạy trừ điểm bất hoạt Inactivity Decay chạm trán sự kiện hoàn thành buổi tập WorkoutSessionCompleted lúc nửa đêm). Nếu không có cơ chế kiểm soát đồng thời, hệ thống sẽ gặp lỗi mất cập nhật (Lost Update).

Cần quyết định giữa hai giải pháp kiểm soát đồng thời:
1. **Khóa Lạc Quan (Optimistic Locking)**: Dùng cột `version` và câu lệnh `UPDATE ... WHERE version = :oldVersion`.
2. **Khóa Bi Quan (Pessimistic Locking)**: Dùng `SELECT ... FROM gamification.user_elo WHERE user_id = :id FOR UPDATE` trong transaction.

## Decision Drivers
- **Practical Simplicity (Đơn giản thực tế)**: Không làm phức tạp hóa mã nguồn ứng dụng Go khi không có nhu cầu thực sự.
- **Đặc thù hành vi người dùng (User Behavior Reality)**: Tần suất phát sinh sự kiện ghi điểm ELO trên cùng một `user_id` là rất thưa thớt (1-2 buổi tập/ngày, tỷ lệ đụng độ mili-giây $< 0.01\%$).
- **Chi phí vận hành và rủi ro lỗi**: Khóa lạc quan đòi hỏi viết logic Retry Loop (đọc lại, tính lại $K$-factor, thử lại với exponential backoff, xử lý Dead-Letter Queue khi vượt quá số lần retry).

## Considered Options
- **Option 1 (Optimistic Locking với `version`)**: Đọc không khóa, khi cập nhật kiểm tra `version`. Nếu `RowsAffected == 0`, ứng dụng Go phải chạy vòng lặp Retry Loop để thử lại.
- **Option 2 (Pessimistic Row Locking với `FOR UPDATE`)**: Mở transaction, khóa đúng 1 dòng của `user_id` bằng `FOR UPDATE`. PostgreSQL tự động xếp hàng luồng đến sau trong 2-5ms mà không cần code retry trên Go.

## Decision Outcome
Chosen: **Option 2 (Pessimistic Row Locking với `FOR UPDATE`)** theo thống nhất giữa Maintainer và AI Assistant.
- Tần suất xung đột ghi trên cùng một gymer là cực kỳ hiếm.
- Giao dịch cập nhật ELO chỉ gồm các truy vấn SQL nội bộ (tuyệt đối không gọi HTTP/gRPC ra ngoài trong transaction), thời gian giữ lock chỉ mất 1-3ms.
- PostgreSQL giải quyết việc xếp hàng ở mức engine database, mã nguồn Go giữ được sự trong sáng và tối giản, không cần cơ chế retry phức tạp.

### Consequences
- **Positive:**
  - Triệt tiêu hoàn toàn rủi ro Lost Update mà không cần viết thêm mã nguồn Retry Loop trong Go.
  - Khóa ở cấp độ dòng (Row-level Lock) trên `user_id` nên hoàn toàn không ảnh hưởng đến hàng trăm nghìn người dùng khác đang tập luyện đồng thời.
- **Trade-offs / Negatives:**
  - Giữ kết nối database trong suốt thời gian transaction (khoảng 1-3ms).
- **Mitigation:**
  - Ràng buộc thiết kế nghiêm ngặt: Tuyệt đối cấm thực hiện I/O mạng hoặc gọi dịch vụ ngoài trong transaction đang giữ lock `FOR UPDATE`.
