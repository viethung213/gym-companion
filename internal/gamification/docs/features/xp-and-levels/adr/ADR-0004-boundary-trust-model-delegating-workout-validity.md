# ADR-0004: Boundary Trust Model — Delegating Workout Duration & Validity to Workout Execution

## Status
Accepted

## Context
Ban đầu, quy tắc nghiệp vụ `BR-XP-02` (Proof of Workout) quy định module Gamification phải thẩm định cả 2 điều kiện trước khi trao điểm XP:
1. `DurationSeconds >= 900` (Tối thiểu 15 phút tập luyện).
2. `TotalSets >= 3` (Tối thiểu 3 hiệp hoàn thành).

Tuy nhiên, trong kiến trúc Bounded Context & Modular Monolith:
- Module `workout_execution` là chủ quản sở hữu toàn bộ logic vòng đời buổi tập (`WorkoutSessionStarted`, `WorkoutSessionAborted`, `WorkoutSessionCompleted`).
- Khi một sự kiện `WorkoutSessionCompleted` được phát ra trên Kafka topic `workout_execution.events`, điều đó có nghĩa là phiên tập đó đã trải qua toàn bộ quy trình kiểm chứng, hợp lệ hóa và hoàn tất nghiệp vụ tại nguồn.
- Nếu module Gamification lại can thiệp sửa hợp đồng `workout_session_completed.proto` hoặc yêu cầu kiểm tra lại thời lượng `duration_seconds`, Gamification đang vi phạm ranh giới Bounded Context (Domain Boundary Bleed / Over-validation), tạo ra sự phụ thuộc chặt chẽ và trùng lặp trách nhiệm không cần thiết giữa hai module.

## Decision
1. **Tuyệt đối không can thiệp sửa đổi Protobuf Contract của Workout Execution (`workout_session_completed.proto`)**:
   Hợp đồng giữa các miền nghiệp vụ phải duy trì tính ổn định (Contract Immutability). Gamification không áp đặt trường dữ liệu đặc thù của mình lên miền lõi quản lý phiên tập.
2. **Thiết lập mô hình Boundary Trust (Niềm tin ranh giới nghiệp vụ)**:
   - Gamification tin tưởng tuyệt đối vào sự kiện `WorkoutSessionCompleted` do `workout_execution` phát hành: một khi sự kiện hoàn thành đã được công bố, phiên tập đó mặc nhiên được công nhận là hợp lệ về mặt thời gian và tư cách buổi tập.
   - Gamification chỉ thẩm định các thông số có sẵn trong hợp đồng để phân phối tỷ lệ thưởng (ví dụ: `total_volume` để tính Volume XP, `average_form_score` để tính Form XP, `total_sets >= 3` để lọc tối thiểu) mà không tái kiểm tra thời lượng buổi tập (`duration_seconds`).

## Consequences
### Tích cực
- **Zero Coupling:** Bảo vệ nguyên vẹn hợp đồng `.proto` của miền `workout_execution`, không gây ảnh hưởng hay làm vỡ compatibility với các consumer khác (Analytics, Coaching, Social).
- **Phân tách trách nhiệm sạch sẽ (High Cohesion, Low Coupling):** Trách nhiệm kiểm tra tính hợp lệ của buổi tập (gian lận, thời lượng tối thiểu, ngắt quãng) thuộc về `workout_execution`. Trách nhiệm của Gamification là chuyển hóa thành quả đã được công nhận thành phần thưởng (XP, Streak, Huy hiệu).
- **Đơn giản hóa mã nguồn:** Loại bỏ logic hardcode hay các nhánh điều kiện giả định thời lượng trong consumer của Gamification.

### Tiêu cực / Trade-offs
- Nếu `workout_execution` hoàn thành một buổi tập ngắn dưới 15 phút mà không hủy (`abort`), Gamification vẫn sẽ trao điểm thưởng cơ bản ($50\text{ XP}$) nếu `total_sets >= 3`. Điều này là sự đánh đổi có chủ đích để giữ vững tính toàn vẹn ranh giới kiến trúc.
