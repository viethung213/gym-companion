# ADR-0003: Cổng Chi Tiêu Mở (Generic Spending Seam) & Tách Rời Phân Hệ Cửa Hàng

- **Feature**: fitcoins-and-ledger
- **Date**: 2026-10-11
- **Status**: Accepted
- **Deciders**: Maintainer (User), AI Assistant

## Context and Problem Statement
Người dùng tích lũy FitCoins với mục đích quy đổi thành các tiện ích trong tương lai (mua khiên Đóng Băng Chuỗi - Streak Freeze, tăng tốc XP, đổi voucher đối tác).

Câu hỏi đặt ra là: **Module `fitcoins-and-ledger` có nên quản lý luôn danh mục sản phẩm Cửa Hàng (Shop Catalog), bảng giá cứng và kho đồ vật phẩm (Inventory) của người dùng hay không?**

## Decision Drivers
- **Nguyên tắc Đơn Trách Nhiệm (Single Responsibility Principle - SRP)**: Module FitCoins đóng vai trò là "Ngân Hàng / Sổ Cái", chịu trách nhiệm bảo toàn số dư và kiểm toán thu/chi, không phải là "Cửa Hàng Bán Lẻ".
- **Tránh Khớp Nối Sớm (Avoid Premature Coupling)**: Tính năng Chuỗi Ngày (`streaks-and-habits`) hiện tại chưa được triển khai. Nếu hardcode bảng giá hay bảng tồn kho khiên Streak Freeze vào module FitCoins sẽ tạo ra sự phụ thuộc chéo khó bảo trì.
- **Tính Linh Hoạt & Khả Năng Mở Rộng**: Sau này hệ thống có thể bổ sung thêm nhiều loại vật phẩm mới hoặc điều chỉnh giá bán mà không cần phải can thiệp hay sửa đổi cấu trúc bảng của Sổ Cái tài chính.

## Considered Options

### Option 1: Quản lý Cửa Hàng & Kho Đồ Trực Tiếp Trong Module FitCoins
- **Cơ chế**: Tạo các bảng `gamification.shop_items`, `gamification.user_inventory`, hardcode giá Streak Freeze (100 coin), Double XP (150 coin) ngay trong DDL của module ví.
- **Hạn chế**:
  - Vi phạm nguyên tắc phân ranh giới domain: Ví tiền lại đi quản lý số lượng khiên streak.
  - Phụ thuộc vào các tính năng chưa ra mắt. Khi cần khuyến mãi giảm giá hoặc thay đổi logic vật phẩm, phải sửa code của hệ thống sổ cái tài chính.

### Option 2: Cổng Chi Tiêu Mở (Generic Spending Seam) Tách Biệt
- **Cơ chế**:
  1. Module `fitcoins-and-ledger` chỉ cung cấp một phương thức trừ tiền tổng quát:
     ```go
     SpendCoins(ctx context.Context, cmd SpendCoinsCommand) (*SpendCoinsResult, error)
     ```
     với các tham số nghiệp vụ: `UserID`, `Amount`, `Reason` (mã định danh nguyên nhân chi tiêu), `IdempotencyKey`.
  2. Module chỉ đảm bảo:
     - Số dư đủ để trừ (`balance >= amount`).
     - Khóa dòng bi quan chống Lost Update & Double Spending.
     - Ghi nhận vết kiểm toán append-only vào `coin_ledger`.
  3. Bất kỳ phân hệ nào sau này (Shop, Streak, Voucher) khi cần trừ coin chỉ việc gọi vào cổng này.

## Decision Outcome
Chosen: **Option 2 (Cổng Chi Tiêu Mở - Generic Spending Seam Tách Biệt)**.

### Consequences
- **Positive:**
  - **Kiến trúc sạch (Clean Hexagonal Boundaries)**: Module FitCoins hoàn toàn tinh gọn, hoạt động độc lập như một Micro-Ledger Engine thuần túy.
  - **Khả năng tái sử dụng tối đa**: Mọi tính năng trong tương lai cần tiêu coin đều có thể tích hợp qua một Application Port duy nhất mà không làm phình to module ví.
  - **Dễ dàng kiểm thử (Lean TDD)**: Unit test của FitCoins chỉ cần tập trung vào logic tài chính (cộng, trừ, chặn âm tiền, chống race condition) mà không phải mock các bảng hàng hóa phức tạp.
- **Negative / Trade-offs:**
  - Khi triển khai tính năng Cửa Hàng (Shop) trong tương lai, sẽ cần một use-case điều phối để gọi `SpendCoins` trước khi cấp vật phẩm vào kho đồ người dùng.
