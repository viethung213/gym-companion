# ADR-0001: Kiến Trúc Ví Số Dư & Sổ Cái Kiểm Toán Bất Biến (Immutable Audit Ledger)

- **Feature**: fitcoins-and-ledger
- **Date**: 2026-10-11
- **Status**: Accepted
- **Deciders**: Maintainer (User), AI Assistant

## Context and Problem Statement
Module Gamification cần quản lý đơn vị tiền tệ thể thao **FitCoins**. Tiền tệ ảo trong ứng dụng thường xuyên đối mặt với hai rủi ro lớn:
1. **Chi tiêu kép (Double Spending) và Số dư âm**: Xảy ra khi hai request (hoặc hai consumer) cùng đọc một số dư và thực hiện trừ tiền đồng thời.
2. **Mất dấu vết kiểm toán (No Audit Trail)**: Nếu hệ thống chỉ lưu một cột `balance` và cập nhật trực tiếp (`UPDATE user_wallet SET balance = balance - 50`), khi người dùng khiếu nại hoặc hệ thống phát sinh sai lệch, lập trình viên không có cách nào tái hiện lại lịch sử thu/chi để điều tra.

Cần một kiến trúc lưu trữ và kiểm soát tương tranh bảo đảm tính toàn vẹn tài chính $100\%$, dễ kiểm toán và đạt hiệu năng cao.

## Decision Drivers
- **Tính toàn vẹn tài chính (Financial Integrity)**: Số dư không bao giờ được phép âm (`balance >= 0`), không bao giờ bị chi tiêu kép.
- **Khả năng kiểm toán 100% (Full Auditability)**: Mọi đồng coin tăng hoặc giảm đều phải giải trình được: sinh ra từ đâu, ai tiêu, vào thời điểm nào, mã sự kiện nào.
- **Hiệu năng cao (Sub-5ms Latency)**: Thao tác đọc số dư và ghi nhận giao dịch phải diễn ra cực nhanh, không gây nghẽn database.
- **The 3 AM Test**: Không sử dụng cron job đối soát số dư nửa đêm. Tính nhất quán phải được bảo đảm tức thì trong từng transaction (Strong Consistency).

## Considered Options

### Option 1: Chỉ dùng một bảng duy nhất `coin_ledger` (Pure Event Sourcing / Dynamic Balance)
- **Cơ chế**: Không lưu bảng ví; mỗi khi cần số dư, thực hiện `SELECT SUM(amount) FROM coin_ledger WHERE user_id = $1`.
- **Hạn chế**:
  - Khi một người dùng tích lũy hàng nghìn giao dịch, câu lệnh `SUM(amount)` sẽ ngày càng chậm, tốn CPU và I/O.
  - Khó áp dụng ràng buộc cứng `CHECK (balance >= 0)` ở tầng database nếu không có cột `balance`.

### Option 2: Kiến trúc Kép: Ví Số Dư (`user_wallet`) + Sổ Cái Bất Biến (`coin_ledger`) với Khóa Bi Quan
- **Cơ chế**:
  1. Bảng `gamification.user_wallet`: Lưu số dư tức thời `balance`, có ràng buộc `CHECK (balance >= 0)`.
  2. Bảng `gamification.coin_ledger`: Lưu lịch sử append-only, ghi nhận `amount`, `balance_after`, `reason`, `idempotency_key`. Nghiêm cấm `UPDATE` hoặc `DELETE`.
  3. Mọi thao tác biến động số dư bắt buộc chạy trong transaction ACID:
     ```sql
     SELECT balance FROM gamification.user_wallet WHERE user_id = $1 FOR UPDATE;
     ```
     Cập nhật ví và chèn dòng vào sổ cái trong **cùng 1 transaction**.

## Decision Outcome
Chosen: **Option 2 (Kiến trúc Kép: Ví Số Dư + Sổ Cái Bất Biến với Khóa Bi Quan)**.

### Consequences
- **Positive:**
  - **Truy vấn số dư cực nhanh ($< 1\text{ ms}$)**: Chỉ cần đọc 1 dòng theo khóa chính `user_id` trên bảng `user_wallet`.
  - **Kiểm toán tuyệt đối ($100\%$ Audit Trail)**: Bảng `coin_ledger` lưu toàn bộ lịch sử biến động. Bất kỳ lúc nào cũng có thể đối soát:
    $$\text{balance} = \sum \text{amount}$$
  - **Triệt tiêu Double Spending**: Khóa dòng bi quan tuần tự hóa các yêu cầu chi tiêu đồng thời; ràng buộc `CHECK (balance >= 0)` ngăn chặn triệt để âm tiền ngay từ engine database.
- **Negative / Trade-offs:**
  - Mỗi thao tác thay đổi số dư phải ghi vào 2 bảng (`user_wallet` và `coin_ledger`) trong cùng transaction, tăng nhẹ I/O ghi nhưng hoàn toàn xứng đáng để bảo đảm an toàn dữ liệu tài chính.
