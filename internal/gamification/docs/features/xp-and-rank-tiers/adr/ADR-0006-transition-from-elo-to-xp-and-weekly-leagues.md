# ADR-0006: Chuyển Đổi Từ ELO Rating Sang Điểm Kinh Nghiệm (XP) & Giải Đấu Tuần (Weekly Leagues)

- **Feature**: xp-and-rank-tiers
- **Date**: 2026-10-10
- **Status**: Accepted
- **Deciders**: Maintainer (User), AI Assistant
- **Supersedes**: ADR-0002, ADR-0005

## Context and Problem Statement
Ban đầu, hệ thống sử dụng thang điểm **ELO Rating** (1,000 - 3,000) làm thước đo năng lực thể chất. Tuy nhiên, sau khi đánh giá thực tế vận hành và trải nghiệm gymer:
1. **Sai lệch bản chất (PvP vs PvE)**: ELO là thuật toán tính điểm đối kháng Zero-Sum cho cờ vua/PvP. Trong khi đó, tập gym là hành trình nỗ lực cá nhân (PvE). Việc áp dụng ELO dẫn đến hệ quả phi lý: Gymer tập buổi deload (giảm tải phục hồi) hoặc ốm mệt bị trừ điểm ELO, tạo ra tâm lý trừng phạt tiêu cực (negative reinforcement).
2. **Cơ chế Decay trừ điểm gây ức chế**: Gymer đi du lịch, chấn thương hoặc bận công tác quá 14 ngày bị trừ điểm, triệt tiêu động lực quay trở lại phòng gym.
3. **Quá phức tạp, khó giải thích**: Công thức K-factor scaling, kẹp trần sàn $[-25, +40]$ khiến người dùng không hiểu tại sao mình nhận được số điểm đó.

Cần một mô hình tính điểm mới học tập từ tiêu chuẩn vàng của ứng dụng tạo động lực thói quen hàng đầu thế giới: **Duolingo**.

## Decision Drivers
- **100% Khích lệ dương (Positive Reinforcement)**: Mọi giọt mồ hôi và hiệp tập đều mang lại điểm thưởng. Tuyệt đối không bao giờ trừ điểm khi tập nhẹ hoặc nghỉ ngơi.
- **Dễ hiểu, phổ quát (Universally Understood)**: Điểm kinh nghiệm (XP) là khái niệm quen thuộc với mọi đối tượng người dùng.
- **Tạo động lực ngắn hạn và dài hạn**:
  - Dài hạn: Tổng XP cả đời (Lifetime XP) $\rightarrow$ Thăng cấp Level 1..100.
  - Ngắn hạn: XP giải tuần (Weekly XP) $\rightarrow$ Đua Top bảng đấu tuần (Leagues: Đồng $\rightarrow$ Kim Cương).

## Decision Outcome
Quyết định: **Thay thế toàn bộ hệ thống ELO bằng mô hình Duolingo XP (Dual-Engine: Lifetime XP + Weekly Leagues)**:

1. **Trục 1 — Total / Lifetime XP & Levels**:
   - Khởi đầu từ 0 XP, chỉ có chiều tăng dần (không bao giờ giảm).
   - Tích lũy để thăng cấp Level 1 $\rightarrow$ 100.
2. **Trục 2 — Weekly XP & Giải Đấu Tuần (Leagues)**:
   - Theo dõi XP kiếm được trong tuần (Thứ Hai 00:00:00 $\rightarrow$ Chủ Nhật 23:59:59).
   - Phục vụ bảng xếp hạng giải đấu tuần (Bronze, Silver, Gold, Platinum, Diamond).
3. **Cơ chế Reset Tuần Lười (Lazy Reset on Activity - The 3 AM Test)**:
   - Thay vì chạy batch script update hàng triệu user lúc nửa đêm Chủ nhật gây nghẽn database, hệ thống sử dụng cờ `current_week_number`. Khi user phát sinh hoạt động đầu tiên ở tuần mới, hệ thống tự động reset `weekly_xp = 0` trước khi cộng điểm mới.

### Consequences
- **Positive:**
  - Loại bỏ hoàn toàn cảm giác tiêu cực khi tập nhẹ hoặc nghỉ ngơi.
  - Tăng mạnh tỷ lệ giữ chân (Retention) nhờ cơ chế đua top tuần.
  - Đơn giản hóa kiến trúc: Xóa bỏ Scheduled Worker trừ điểm bất hoạt ban đêm.
- **Negative / Trade-offs:**
  - Cần quản lý chu kỳ tuần cho trường `weekly_xp`.
- **Mitigation:**
  - Sử dụng cơ chế Lazy Reset on write, loại bỏ 100% rủi ro nghẽn DB.
