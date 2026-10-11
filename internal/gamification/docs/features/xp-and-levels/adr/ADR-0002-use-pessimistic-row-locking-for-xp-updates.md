# ADR-0002: Sử Dụng Khóa Dòng Bi Quan (SELECT FOR UPDATE) Cho Cập Nhật Điểm XP

- **Feature**: xp-and-levels
- **Date**: 2026-10-09
- **Status**: Accepted
- **Deciders**: Maintainer (User), AI Assistant

## Context and Problem Statement
Khi người dùng hoàn thành buổi tập hoặc cập nhật chỉ số dinh dưỡng, các sự kiện từ Kafka hoặc HTTP API có thể được gửi đến đồng thời hoặc với khoảng cách thời gian rất ngắn (ví dụ: kết thúc buổi tập đúng lúc hệ thống dinh dưỡng tự động chốt ngày).

Nếu hai giao dịch cùng đọc bản ghi `user_xp`, cùng tính toán và ghi lại vào database, sự kiện đến sau có thể ghi đè làm mất điểm của sự kiện trước (**Lost Update Anomaly**).

Cần lựa chọn cơ chế kiểm soát tương tranh (Concurrency Control) để bảo vệ số dư điểm `xp` và tính toán chính xác cấp độ `level`.

## Decision Drivers
- **Tính toàn vẹn dữ liệu tuyệt đối (Zero Lost Updates)**: Không bao giờ được phép mất dù chỉ 1 điểm XP nỗ lực của gymer.
- **Tuân thủ Hexagonal Architecture & DDD**: Tầng Domain Go phải nắm được trạng thái chuyển đổi điểm số (`old_xp` $\rightarrow$ `new_xp`) để tính toán thăng cấp (`level`), phát sinh Domain Event (`UserLeveledUp`), và ghi nhật ký Outbox.
- **Triệt tiêu độ phức tạp (Zero Retry Loops)**: Không làm phức tạp mã nguồn Kafka Consumer bằng các vòng lặp thử lại (exponential backoff / retry) hoặc nguy cơ nghẽn Dead Letter Queue (DLQ).
- **Thực tế hành vi người dùng**: Tần suất phát sinh sự kiện ghi điểm trên cùng một `user_id` là thưa thớt (vài buổi tập/ngày), đụng độ cấp độ mili-giây giữa 2 sự kiện của cùng 1 user là cực hiếm ($< 0.01\%$).

## Considered Options

### Option 1: Cơ chế MVCC mặc định & Cột hệ thống `xmin` của PostgreSQL
- **Bản chất**: PostgreSQL sử dụng MVCC và gán `xmin` (Transaction ID tạo phiên bản dòng) cho mỗi bản ghi.
- **Hạn chế nghiêm trọng**:
  - Ở mức cô lập mặc định `READ COMMITTED`, PostgreSQL **không tự động ngăn chặn Lost Update** đối với chu trình "Đọc $\rightarrow$ Tính toán trong App $\rightarrow$ Ghi". Nếu Tx 1 và Tx 2 cùng đọc `xp = 100`, cả hai đều tính toán trong RAM và ghi đè giá trị của nhau.
  - Sử dụng `xmin` làm khóa lạc quan (`WHERE user_id = $1 AND xmin = $2`) bị PostgreSQL Core Team khuyến cáo không nên dùng cho logic nghiệp vụ: `xmin` là số 32-bit, sẽ bị reset khi gặp sự kiện **Vacuum Freeze / Transaction ID Wraparound** (khi hệ thống đạt 2 tỷ transactions) và dễ bị sai lệch do subtransactions hoặc vacuum ngầm.

### Option 2: Cập nhật trực tiếp nguyên tử trong SQL (`UPDATE ... SET xp = xp + :delta`)
- **Bản chất**: Đẩy toàn bộ phép cộng điểm xuống tầng database engine: `UPDATE gamification.user_xp SET xp = xp + $1 WHERE user_id = $2`.
- **Hạn chế**:
  - Không thể tính toán Cấp độ trong Go Domain: Cấp độ được xác định theo hàm phi tuyến $Level = \min(\lfloor \sqrt{xp / 50} \rfloor + 1, 100)$.
  - Nếu dùng atomic SQL, tầng Domain Go trong RAM sẽ không biết người dùng có vừa thăng cấp hay không để phát sinh Domain Event `UserLeveledUp`, không thể sinh bản ghi `xp_history` với `old_xp` và `new_xp` chính xác để kiểm toán, trừ khi phải viết Stored Procedure / Trigger phức tạp trong database (vi phạm nguyên tắc tách biệt Domain logic).

### Option 3: Khóa lạc quan (Optimistic Concurrency Control — OCC) với cột `version INT`
- **Bản chất**: Bổ sung cột `version INT` vào bảng `user_xp`, khi cập nhật kiểm tra `WHERE user_id = :id AND version = :read_version`.
- **Hạn chế**:
  - Khi có xung đột đồng thời, giao dịch đến sau cập nhật thất bại (`RowsAffected == 0`).
  - **Bắt buộc tầng Application / Kafka Consumer phải cài đặt Retry Loop**: Rollback transaction $\rightarrow$ Truy vấn lại DB $\rightarrow$ Tính toán lại từ đầu $\rightarrow$ Thử ghi lại.
  - Tốn tài nguyên CPU/IO, làm phức tạp code xử lý lỗi và có nguy cơ đẩy tin nhắn vào Dead Letter Queue (DLQ) khi hết số lần retry.

### Option 4: Khóa dòng bi quan (Pessimistic Row Locking — `SELECT ... FOR UPDATE`)
- **Bản chất**: Khóa dòng của người dùng trong transaction khi đọc aggregate:
  ```sql
  SELECT user_id, xp, level, updated_at
  FROM gamification.user_xp
  WHERE user_id = $1
  FOR UPDATE;
  ```
- **Ưu điểm**:
  - Khi Tx 1 đang giữ khóa, Tx 2 đến sau sẽ tự động xếp hàng chờ (block nhẹ $\sim 2\text{ ms}$) trong PostgreSQL engine.
  - Ngay khi Tx 1 commit, Tx 2 lập tức đọc được dữ liệu mới nhất đã được cập nhật, thực hiện tính toán và commit tuần tự.
  - **Zero Retries**: 100% giao dịch thành công ngay lần đầu, không cần viết bất kỳ dòng code retry nào.

## Decision Outcome
Chosen: **Option 4 (Pessimistic Row Locking - `SELECT ... FOR UPDATE`)**.

### Consequences
- **Positive:**
  - **Triệt tiêu 100% rủi ro Lost Update**: Đảm bảo tính nhất quán tuyệt đối của điểm số `xp` và tính toán `level`.
  - **Đơn giản hóa mã nguồn tối đa**: Loại bỏ hoàn toàn Retry Interceptor, Exponential Backoff và cấu hình Dead Letter Queue phức tạp cho nghiệp vụ tính điểm.
  - **Hiệu năng xuất sắc**: Khóa chỉ áp dụng trên **đúng 1 dòng** của chính `user_id` đang xử lý (Row-level lock qua Primary Key index), không khóa bảng, không ảnh hưởng đến bất kỳ người dùng nào khác. Thời gian giữ khóa cực ngắn ($< 3\text{ ms}$) trong transaction cục bộ.
- **Negative / Trade-offs:**
  - Bắt buộc lập trình viên phải giữ transaction ngắn và gọn: Không được phép thực hiện gọi HTTP/gRPC ra bên ngoài hoặc tác vụ I/O nặng khi đang mở transaction chứa `SELECT FOR UPDATE`.
