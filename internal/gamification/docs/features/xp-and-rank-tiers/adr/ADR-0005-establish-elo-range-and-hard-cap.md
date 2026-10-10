# ADR-0005: Thiết Lập Trần Cứng (Max ELO = 3,000) và Sàn Điểm ELO Cơ Sở

- **Feature**: xp-and-rank-tiers
- **Date**: 2026-10-09
- **Status**: Superseded by ADR-0006
- **Deciders**: Maintainer (User), AI Assistant

## Context and Problem Statement
Trong các hệ thống tính điểm theo cơ chế tích lũy nỗ lực (PvE), nếu không có giới hạn trên (Upper Bound / Ceiling), điểm số của những người dùng lâu năm sẽ tăng tiến vô hạn (ví dụ: lên 10,000+ ELO sau 2 năm). Điều này gây ra:
1. **Lạm phát điểm số phi mã (Point Inflation)**.
2. **Kéo giãn khoảng cách bất hợp lý trên Bảng xếp hạng**: Người mới bắt đầu (1,000 ELO) nhìn vào Top bảng xếp hạng sẽ thấy khoảng cách không thể san lấp, dẫn đến triệt tiêu động lực phấn đấu.

Cần xác định thang điểm tối thiểu và tối đa chuẩn hóa cho toàn bộ hệ sinh thái FITAI.

## Decision Drivers
- Giữ thang đo năng lực thể chất trong khoảng nhận thức trực quan và có ý nghĩa đối với người dùng.
- Ngăn chặn lạm phát điểm trong dài hạn.
- Đảm bảo tính toán số học an toàn và đồng bộ với phân cấp 5 Bậc Hạng (Bronze $\rightarrow$ Diamond).

## Considered Options
- **Option 1 (Không giới hạn trần - Uncapped)**: Điểm ELO có thể tăng vô tận tùy theo số buổi tập tích lũy.
- **Option 2 (Thiết lập Trần Cứng - Hard Cap 3,000 ELO)**: Cố định dải điểm ELO từ **1,000 (Sàn)** đến **3,000 (Trần cứng)** kết hợp cơ chế hãm hệ số $K$ ở Bậc Kim Cương.

## Decision Outcome
Chosen: **Option 2 (Thiết lập Trần Cứng - Hard Cap 3,000 ELO)**.
- **Điểm sàn cơ sở (Min ELO)**: **1,000 ELO** (Điểm khởi tạo ban đầu khi người dùng gia nhập nền tảng; Inactivity Decay không được trừ dưới mốc này).
- **Điểm trần tối đa (Max ELO)**: **3,000 ELO** (Đỉnh cao thể chất tuyệt đối của Bậc Kim Cương).
- Bất kể kết quả buổi tập hoặc điểm thưởng dinh dưỡng, điểm mới luôn bị kẹp trong phạm vi:
  $$\text{current\_elo} = \min(\max(\text{current\_elo} + \Delta ELO, 1000), 3000)$$

### Consequences
- **Positive:**
  - Bảng xếp hạng luôn ổn định trong dải điểm chuẩn mực $[1000, 3000]$.
  - Loại bỏ hoàn toàn rủi ro lạm phát điểm ELO theo thời gian.
- **Trade-offs / Negatives:**
  - Khi người dùng đã chạm mốc 3,000 ELO, các buổi tập tiếp theo không tăng thêm ELO (nhưng vẫn nhận FitCoins và ghi nhận khối lượng Volume vào Badges).
- **Mitigation:**
  - Ở mốc 3,000 ELO, động lực duy trì của gymer chuyển sang bảo vệ thứ hạng Top 1 trước nguy cơ Inactivity Decay và tích lũy FitCoins đổi quà.
