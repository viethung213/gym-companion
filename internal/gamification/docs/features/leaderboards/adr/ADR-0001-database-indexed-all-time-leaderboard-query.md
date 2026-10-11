# ADR-0001: Sử Dụng Trực Tiếp Index B-Tree PostgreSQL Cho Bảng Xếp Hạng Thay Vì Redis ZSET

- **Feature**: leaderboards
- **Date**: 2026-10-11
- **Status**: Accepted
- **Deciders**: Maintainer (User), AI Assistant

## Context and Problem Statement
Tính năng **Bảng Xếp Hạng Toàn Hệ Thống (XP Leaderboard)** cần phục vụ truy vấn Top 50 gymer dẫn đầu và tính toán thứ hạng của người dùng hiện tại (`my_rank`).

Một giải pháp phổ biến trong các hệ thống Gaming là đưa toàn bộ `(user_id, xp)` vào **Redis Sorted Set (ZSET)** (`ZREVRANGE`, `ZREVRANK`). Cần đánh giá xem nên sử dụng Redis ZSET hay truy vấn trực tiếp trên cơ sở dữ liệu quan hệ PostgreSQL với B-tree index.

## Decision Drivers
- **Chi phí vận hành & Sự tinh gọn (Practical Simplicity)**: FITAI hiện tại là một Modular Monolith lưu trữ chính trên PostgreSQL. Tránh việc đưa thêm cụm Redis cluster chỉ để phục vụ một bảng xếp hạng Top 50.
- **Tính toàn vẹn & Nguồn sự thật duy nhất (Single Source of Truth)**: Dữ liệu `user_xp` nằm trong PostgreSQL. Dùng Redis đòi hỏi cơ chế đồng bộ (Dual Write / Outbox Consumer sync sang Redis), dễ xảy ra hiện tượng lệch điểm số (Cache Desync).
- **The 3 AM Test**: Loại bỏ rủi ro mất dữ liệu cache khi Redis khởi động lại hoặc nghẽn mạng giữa App và Redis node.

## Considered Options

### Option 1: Sử dụng Redis Sorted Set (ZSET)
- **Cơ chế**: Mỗi khi có event `XpEarned`, worker cập nhật vào Redis ZSET: `ZADD global_leaderboard <xp> <user_id>`. Truy vấn dùng `ZREVRANGE 0 49 WITHSCORES` ($O(\log(N) + M)$).
- **Hạn chế**:
  - Tốn thêm chi phí hạ tầng và vận hành Redis cluster.
  - Phải quản lý đồng bộ dữ liệu: nếu cập nhật PostgreSQL thành công nhưng Redis lỗi, bảng xếp hạng sẽ bị sai lệch.
  - Không hỗ trợ quy tắc hòa điểm phụ (`updated_at ASC`) tự nhiên như SQL; phải áp dụng các thủ thuật mã hóa timestamp vào float score của Redis rất dễ lỗi sai số float64.

### Option 2: Truy vấn trực tiếp trên PostgreSQL với B-tree Composite Index
- **Cơ chế**: Tận dụng trực tiếp bảng `gamification.user_xp` với index hỗn hợp:
  ```sql
  CREATE INDEX idx_user_xp_leaderboard 
  ON gamification.user_xp (xp DESC, updated_at ASC);
  ```
- **Hiệu năng thực tế**:
  - Truy vấn Top 50: `SELECT user_id, xp, level FROM gamification.user_xp ORDER BY xp DESC, updated_at ASC LIMIT 50;` $\rightarrow$ Engine chỉ đọc đúng 50 con trỏ đầu tiên trên cây B-tree (**Index Only Scan / Index Scan**), thực thi trong $< 2\text{ ms}$.
  - Truy vấn thứ hạng cá nhân (`my_rank`): `SELECT COUNT(*) + 1 FROM gamification.user_xp WHERE xp > :my_xp;` $\rightarrow$ Quét nhánh B-tree với độ trễ P99 $< 15\text{–}20\text{ ms}$.

## Decision Outcome
Chosen: **Option 2 (Truy vấn trực tiếp trên PostgreSQL với B-tree Composite Index cho giai đoạn MVP)**.

> [!IMPORTANT]
> **Giới Hạn Phạm Vi Giai Đoạn MVP (< 50,000 users)**:
> Giải pháp dùng `COUNT(*)` để tính `my_rank` trực tiếp trên PostgreSQL chỉ được áp dụng và bảo đảm hiệu năng tối ưu trong giai đoạn MVP (dưới 50,000 người dùng). Khi vượt ngưỡng này, chi phí quét MVCC của PostgreSQL sẽ làm gia tăng độ trễ p99 nếu không có chiến lược phân tầng.

### Chiến Lược Phân Tầng Theo Quy Mô Hệ Thống (Tiered Scaling Strategy)

Để hệ thống sẵn sàng mở rộng mà không phá vỡ giao diện Ports & Adapters của Application Layer (`LeaderboardRepository`):

| Tầng Quy Mô | Số Lượng Users | Cơ Chế Phục Vụ Top 50 | Cơ Chế Phục Vụ `my_rank` (Ngoài Top 50) | Độ Trễ Mục Tiêu |
| :--- | :--- | :--- | :--- | :--- |
| **Giai đoạn 1 (MVP)** | $< 50,000$ | PostgreSQL B-tree Index Scan (`LIMIT 50`) | PostgreSQL `COUNT(*) + 1` qua B-tree index | $< 20\text{ ms}$ (P99) |
| **Giai đoạn 2 (Growth)** | $50,000\text{–}500,000$ | PostgreSQL B-tree Index Scan (`LIMIT 50`) | **Materialized View** (`leaderboard_ranks_mv`) refresh định kỳ mỗi 5–15 phút | $< 5\text{ ms}$ (P99) |
| **Giai đoạn 3 (Scale)** | $> 500,000$ | **Redis Sorted Set (ZSET)** (`ZREVRANGE 0 49`) | **Redis Sorted Set** (`ZREVRANK` $O(\log N)$) đồng bộ qua Outbox Worker | $< 2\text{ ms}$ (P99) |

### Consequences
- **Positive:**
  - **Không tốn thêm hạ tầng trong giai đoạn đầu**: Hoạt động $100\%$ trên cụm PostgreSQL sẵn có, zero chi phí vận hành cụm Redis.
  - **100% Nhất quán cho MVP (Strong Consistency)**: Cập nhật XP trong transaction là bảng xếp hạng Top 50 phản ánh tức thì, không có độ trễ đồng bộ (Zero sync lag).
  - **Hỗ trợ hòa điểm tự nhiên**: Cột `updated_at ASC` được index trực tiếp, đảm bảo thứ tự ưu tiên tuyệt đối cho người đạt mốc điểm trước.
  - **Dễ dàng nâng cấp hạ tầng**: Tầng Application hoàn toàn tách biệt thông qua `LeaderboardRepository`, việc chuyển từ PostgreSQL sang Materialized View hoặc Redis chỉ đòi hỏi thay đổi implementation ở Driven Adapter (`infrastructure/persistence/`) mà không đổi code nghiệp vụ.
- **Negative / Trade-offs:**
  - `COUNT(*)` bắt đầu suy giảm hiệu năng khi chạm mốc $50,000$ users, bắt buộc đội ngũ kỹ thuật phải kích hoạt Giai đoạn 2 (Materialized View) trước khi chạm ngưỡng nghẽn I/O.
