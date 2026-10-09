# ADR-0003: Ánh Xạ Bậc Hạng Trực Tiếp Không Trạng Thái (Loại Bỏ Demotion Shield)

- **Feature**: elo-and-rank-tiers
- **Date**: 2026-10-09
- **Status**: Accepted
- **Deciders**: Maintainer (User), AI Assistant

## Context and Problem Statement
Trong bản thảo đặc tả ban đầu của hệ thống ELO, một cơ chế "Đệm an toàn chống rớt hạng" (Demotion Shield - cấp 3 lượt bảo vệ sau khi thăng hạng) đã được đề xuất nhằm giảm bớt cảm giác thất vọng cho người dùng khi vừa thăng bậc đã bị rớt lại ngay.

Tuy nhiên, cơ chế này làm phát sinh thêm trạng thái quản lý (`demotion_shield_count`), các câu lệnh cập nhật có điều kiện phức tạp trong database, và làm mờ nhạt tính nhất quán giữa điểm số ELO với Bậc Hạng hiển thị (người dùng có điểm dưới sàn nhưng vẫn hiển thị ở bậc cao hơn).

Maintainer đã đánh giá và chỉ đạo: **Xóa bỏ cơ chế này vì phức tạp không cần thiết**.

## Decision Drivers
- **Practical Simplicity (Nguyên tắc Thực Dụng Đơn Giản)**: Bậc Hạng phải là một hàm ánh xạ thuần túy (Pure Function) không trạng thái: $\text{RankTier} = f(\text{elo\_rating})$.
- **Loại bỏ trạng thái thừa (State Minimization)**: Không cần thêm cột `demotion_shield_count` hay logic đếm lượt trong PostgreSQL.
- **Tính minh bạch tuyệt đối**: Điểm số thế nào thì bậc hạng thế đó, người dùng và hệ thống luôn có góc nhìn đồng nhất 100%.

## Considered Options
- **Option 1 (Demotion Shield có trạng thái)**: Duy trì bộ đếm 3 lượt bảo vệ trong bảng `user_elo`. Khi rớt điểm dưới sàn, trừ dần số khiên trước khi hạ bậc.
- **Option 2 (Stateless Direct Mapping - Ánh xạ trực tiếp)**: Bậc hạng được tính toán và cập nhật thẳng theo mốc điểm ELO hiện tại. Nếu điểm ELO giảm xuống dưới ngưỡng sàn của bậc hiện tại, hệ thống hạ bậc ngay lập tức (`RankTierDemoted`).

## Decision Outcome
Chosen: **Option 2 (Stateless Direct Mapping)** theo quyết định của Maintainer.

### Consequences
- **Positive:**
  - **Mô hình dữ liệu sạch**: Bảng `gamification.user_elo` không cần lưu trữ hay quản lý trạng thái khiên rớt hạng.
  - **Logic nghiệp vụ đơn giản**: Hàm xác định bậc hạng chỉ là một hàm so sánh ngưỡng cố định (Switch-case / Lookup table).
  - **Dễ kiểm thử và bảo trì**: Không có rủi ro race condition hoặc lỗi logic liên quan đến việc tiêu hao khiên khi xử lý song song các sự kiện.
- **Trade-offs / Negatives:**
  - Người dùng vừa chạm ngưỡng thăng hạng nếu buổi tập tiếp theo có kết quả kém sẽ rớt hạng ngay.
- **Mitigation:**
  - Hệ số biến động $K$ ở các bậc cao đã được giảm dần ($K=24, 16, 10$) để điểm số dao động mượt mà hơn, tránh biến động giật cục.
