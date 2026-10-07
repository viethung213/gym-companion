# Đặc Tả Yêu Cầu Nghiệp Vụ — Module Social (Business Requirements)

Tài liệu này định nghĩa toàn bộ mục tiêu, phạm vi và quy tắc nghiệp vụ của Bounded Context **Social & Activity Feed** trong hệ thống Gym Companion.

---

## 1. Giới Thiệu & Mục Tiêu Nghiệp Vụ

Module **Social** đóng vai trò là mạng xã hội thể hình thu nhỏ, kết nối các gymer, người tập luyện và huấn luyện viên. Mục tiêu là:
1. **Tạo động lực tập luyện (Gamification & Social Motivation)**: Khuyến khích người dùng chia sẻ thành tích, kỷ lục cá nhân (PR), thời gian và cường độ tập luyện để nhận được sự cổ vũ từ bạn bè.
2. **Xây dựng cộng đồng (Community Graph)**: Thiết lập mạng lưới theo dõi (Follower / Following) giữa những người có cùng sở thích và giáo án tập luyện.
3. **Tương tác tích cực (Positive Interactions)**: Cho phép thả cảm xúc thể hình (Like, Fire 🔥, Muscle 💪, Clap 👏) và trao đổi, thảo luận thông qua bình luận đa cấp.

---

## 2. Các Phân Hệ Nghiệp Vụ Chính

### 2.1. Đồ Thị Quan Hệ (Social Graph)
- **Theo dõi (Follow)**:
  - Một người dùng ($A$) có thể theo dõi người dùng ($B$) bất kỳ nếu tài khoản $B$ tồn tại và hợp lệ.
  - Người dùng không được phép tự follow chính mình.
  - Hành động follow mang tính lũy đẳng (Idempotent): Nếu đã follow rồi thì không phát sinh bản ghi trùng.
- **Hủy theo dõi (Unfollow)**:
  - Cho phép người dùng $A$ ngừng theo dõi $B$ bất kỳ lúc nào.
- **Danh sách Followers / Following**:
  - Hỗ trợ xem danh sách người đang theo dõi mình (Followers) và danh sách người mình đang theo dõi (Following).
  - Phân trang dạng Cursor để đảm bảo hiển thị mượt mà khi danh sách mở rộng lên hàng nghìn người.

---

### 2.2. Bảng Tin Thống Nhất (Single-Table Activity Feed)
Thay vì chia tách bảng tin thành nhiều nguồn dữ liệu rời rạc, hệ thống hợp nhất thành mô hình bảng tin duy nhất với 2 loại nội dung:
1. **Bài viết cá nhân (Post - `POST`)**:
   - Do người dùng tự soạn thảo nội dung (caption), có thể đính kèm nhiều hình ảnh/video (`media_urls`).
2. **Hoạt động tập luyện chia sẻ (Workout Activity - `WORKOUT_ACTIVITY`)**:
   - Được nạp tự động thông qua sự kiện khi người dùng hoàn thành buổi tập tại phòng gym và bấm chia sẻ.
   - **Nội dung đính kèm**: Người dùng có thể viết lời tựa/cảm nghĩ (`caption`) và đính kèm hình ảnh/video check-in tại phòng gym (`media_urls`).
   - **Chỉ số thể lực đặc thù**: Kèm theo khối dữ liệu thể lực chi tiết: Tên buổi tập (`workout_title`), thời lượng (`duration_seconds`), tổng khối lượng nâng (`total_volume_kg`), số hiệp hoàn thành (`total_sets`), số kỷ lục cá nhân mới phá được (`pr_count`).

---

### 2.3. Quy Tắc Quyền Riêng Tư (Privacy-First Workout Sharing)
- **Không tự động đăng (No Auto-Share)**:
  - Hệ thống tôn trọng quyền riêng tư tối đa. Khi người dùng kết thúc buổi tập ở module `workout_execution`, dữ liệu buổi tập **không tự ý xuất hiện** trên mạng xã hội.
  - Chỉ khi người dùng chủ động bấm nút **"Chia sẻ lên cộng đồng"**, sự kiện `workoutSessionShared` mới được kích hoạt và nạp vào bảng tin.
- **Cấp độ hiển thị (Visibility Level)**:
  - `PUBLIC`: Toàn bộ cộng đồng có thể xem được.
  - `FOLLOWERS_ONLY`: Chỉ những người đang theo dõi tác giả mới nhìn thấy bài viết trên bảng tin.
  - `PRIVATE`: Chỉ tác giả xem được trên trang cá nhân của mình.

---

### 2.4. Hệ Thống Tương Tác Xã Hội (Reactions & Comments)

#### A. Cảm xúc (Reactions)
- Người dùng có thể biểu đạt cảm xúc trên bất kỳ bài viết/hoạt động nào với 4 loại cảm xúc thể hình:
  - `LIKE`: Thích.
  - `FIRE`: Năng lượng, bùng cháy 🔥.
  - `MUSCLE`: Sức mạnh, cơ bắp 💪.
  - `CLAP`: Cổ vũ, tán thưởng 👏.
- **Cơ chế Toggle & Chuyển đổi**:
  - Nếu bấm lại cùng loại reaction đang có: Hủy reaction.
  - Nếu bấm loại reaction khác: Cập nhật sang loại mới.
- **Bộ đếm phi chuẩn hóa (Denormalized Counter)**:
  - `reaction_count` trên bài viết được tự động tăng/giảm đồng thời để tối ưu truy vấn xem bảng tin với chi phí $O(1)$ mà không cần chạy `COUNT(*)` đắt đỏ.
- **Xem danh sách người thả cảm xúc**:
  - Hỗ trợ API xem danh sách người đã thả reaction kèm thông tin họ tên, avatar và loại biểu cảm đã chọn.

#### B. Bình luận (Comments)
- Người dùng có thể để lại lời bình luận văn bản trên bài viết.
- **Bình luận đa cấp (Threaded / Replies)**:
  - Hỗ trợ phản hồi trực tiếp một bình luận khác thông qua thuộc tính `parent_id`.
- **Xóa bình luận**:
  - Tác giả bình luận hoặc tác giả bài viết có quyền xóa bình luận. Khi xóa, tự động trừ `comment_count` của bài viết.

---

### 2.5. Bản Sao Danh Tính Người Dùng (User Snapshot)
- Do tuân thủ nguyên tắc **Schema Isolation** (mỗi module có cơ sở dữ liệu riêng, cấm JOIN chéo sang module Profile hay Auth):
  - Module Social duy trì một bảng bản sao thông tin đại diện: `social.user_snapshots` (`id`, `full_name`, `avatar_url`, `updated_at`).
  - Dữ liệu này được tự động cập nhật bất đồng bộ thông qua Kafka khi:
    1. Người dùng đăng ký tài khoản thành công (`contracts.generic.auth.v1.userRegistered` trên topic `auth.events`).
    2. Người dùng thay đổi họ tên hoặc avatar ở hồ sơ cá nhân (`contracts.supporting.profile.v1.event.UserIdentityUpdated` trên topic `profile.events`).
  - Giúp việc hiển thị Newfeed luôn kèm theo Avatar và Họ tên mới nhất với tốc độ tức thì, không gây nghẽn mạng liên module.

---

### 2.6. Thống Kê Hoạt Động Xã Hội (Social Summary)
- Cho phép người dùng hoặc bạn bè xem nhanh trang tổng quan thể hình (Social Profile Summary) gồm:
  - Số lượng bài viết đã đăng (`posts_count`).
  - Số lượng người đang theo dõi (`followers_count`).
  - Số lượng người mình đang theo dõi (`following_count`).
  - Tổng số giờ/buổi tập luyện đã chia sẻ và tổng volume tạ đã hoàn thành.
