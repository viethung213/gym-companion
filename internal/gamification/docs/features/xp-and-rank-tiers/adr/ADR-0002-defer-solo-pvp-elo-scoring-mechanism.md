# ADR-0002: Hoãn Xác Lập Cơ Chế Điểm Thưởng Solo (PvP) Cho Đến Khi Xây Dựng Module Competition

- **Feature**: xp-and-rank-tiers
- **Date**: 2026-10-09
- **Status**: Accepted
- **Deciders**: Maintainer (User), AI Assistant

## Context and Problem Statement
Trong kế hoạch dài hạn, hệ thống FITAI sẽ có tính năng Solo đối kháng 1v1 (PvP) giữa các gymer. Cần làm rõ: Điểm XP của module Gamification trong giai đoạn hiện tại có chốt công thức thưởng điểm trận Solo hay không?

## Decision Drivers
- Tôn trọng nguyên tắc Simple First & Surgical: Không thiết kế hoặc suy đoán trước hành vi của một module chưa tồn tại.
- Tránh phụ thuộc giả định (Speculative Coupling) giữa Gamification và Competition.
- Rõ ràng ranh giới phạm vi triển khai của giai đoạn hiện tại.

## Considered Options
- **Option 1 (Chốt trước cơ chế Solo XP)**: Đặc tả sẵn công thức tính điểm thắng/thua Solo ngay trong tài liệu thiết kế Gamification.
- **Option 2 (Hoãn chốt và ghi nhận ranh giới mở - Defer & Explicit Boundary)**: Tạm thời không đưa cơ chế điểm Solo vào hệ thống XP của giai đoạn này vì chưa có module Solo/Competition. Ghi rõ trong tài liệu đặc tả rằng Solo PvP là phân hệ mở rộng tương lai, sẽ được chốt chi tiết khi module Solo được phát triển.

## Decision Outcome
Chosen: **Option 2 (Hoãn chốt và ghi nhận ranh giới mở)** theo chỉ đạo trực tiếp từ Maintainer.
- Giai đoạn hiện tại: Điểm `total_xp` và `weekly_xp` chỉ tính toán dựa trên hoạt động tập luyện cá nhân (PvE từ `workout_execution`) và điểm thưởng kỷ luật dinh dưỡng (từ `nutrition`).
- Module Solo/Competition khi được triển khai sau này sẽ đóng vai trò là một nguồn sự kiện bổ sung (Inbound Event Source) thưởng XP.

### Consequences
- **Positive:**
  - Giữ cho mã nguồn và tài liệu của Gamification giai đoạn này tinh gọn, tập trung hoàn thiện trải nghiệm cốt lõi của người tập đơn.
  - Tránh nợ kỹ thuật (Tech Debt) do phải sửa đổi hợp đồng dữ liệu khi module Solo thay đổi luật chơi trong tương lai.
- **Trade-offs / Negatives:**
  - Chưa có bảng quy chuẩn hoàn chỉnh cho chế độ PvP.
- **Mitigation:**
  - Ghi chú rõ ràng trạng thái `DEFERRED` trong tài liệu `01-spec.md` để các kỹ sư phát triển module Solo sau này dễ dàng đối chiếu.
