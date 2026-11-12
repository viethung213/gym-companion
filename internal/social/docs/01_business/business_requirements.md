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
   - **Chỉ số thể lực đặc thù**: Kèm theo khối dữ liệu thể lực chi tiết: Tên buổi tập (`workout_title`), thời lượng (`duration_seconds`), tổng khối lượng nâng (`total_volume_kg`), số hiệp hoàn thành (`total_sets`).

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
  - Module Social duy trì một bảng bản sao thông tin đại diện: `social.user_snapshots` (`id`, `full_name`, `avatar_url`, `role`, `follower_count`, `following_count`, `updated_at`).
  - Dữ liệu này được tự động cập nhật bất đồng bộ thông qua Kafka khi:
    1. Người dùng đăng ký tài khoản thành công (`contracts.generic.auth.v1.userRegistered` trên topic `auth.events`).
    2. Người dùng thay đổi họ tên hoặc avatar ở hồ sơ cá nhân (`contracts.supporting.profile.v1.event.UserIdentityUpdated` trên topic `profile.events`).
  - Các chỉ số `follower_count` và `following_count` được cập nhật nguyên tử ngay khi phát sinh hành động Follow/Unfollow, giúp truy vấn Social Summary và Discover/Search Users đạt hiệu năng cao tức thì mà không cần COUNT trên bảng `social.follows` kích thước lớn.
  - Giúp việc hiển thị Newfeed luôn kèm theo Avatar và Họ tên mới nhất với tốc độ tức thì, không gây nghẽn mạng liên module.

---

### 2.6. Thống Kê Hoạt Động Xã Hội (Social Summary)
- Cho phép người dùng hoặc bạn bè xem nhanh trang tổng quan thể hình (Social Profile Summary) gồm:
  - Số lượng bài viết đã đăng (`posts_count`).
  - Số lượng người đang theo dõi (`followers_count`).
  - Số lượng người mình đang theo dõi (`following_count`).
  - Tổng số giờ/buổi tập luyện đã chia sẻ và tổng volume tạ đã hoàn thành.


---

### 2.7. Tìm Kiếm Bài Viết & Hoạt Động (Post & Activity Search)
- **Mục tiêu**: Cho phép người dùng tìm kiếm bài viết hoặc nhật ký tập luyện trong cộng đồng theo từ khóa nội dung và người dùng.
- **Quy tắc tìm kiếm theo từ khóa (`query`)**: Khớp theo cụm từ hoặc từ khóa xuất hiện trong trường `caption` của bài viết (hỗ trợ chuỗi con và tiếng Việt có dấu/không dấu qua Trigram index).
- **Bộ lọc kết hợp**:
  - *Lọc theo tác giả (`user_id`)*: Giới hạn phạm vi tìm kiếm trong các bài viết của một người dùng cụ thể. Bỏ trống để tìm kiếm trên toàn mạng xã hội.
  - *Lọc theo loại nội dung (`item_type`)*: Lọc riêng bài viết thông thường (`POST`) hoặc bài tập hoàn thành (`WORKOUT_ACTIVITY`). Bỏ trống hoặc đặt unspecified để lấy tất cả.
- **Quy tắc bảo mật & phân quyền hiển thị (Access Control)**:
  - Bài viết `PUBLIC`: Mọi người dùng hợp lệ đều có thể tìm thấy.
  - Bài viết `FOLLOWERS_ONLY`: Chỉ xuất hiện trong kết quả tìm kiếm nếu người tìm kiếm là tác giả hoặc đang theo dõi tác giả đó.
  - Bài viết `PRIVATE`: Chỉ xuất hiện khi người tìm kiếm chính là tác giả bài viết.
  - Kiểm tra quyền trực tiếp tại tầng truy vấn cơ sở dữ liệu để bảo vệ dữ liệu và bảo đảm phân trang chính xác.
- **Phân trang dạng Cursor**:
  - Hỗ trợ cuộn trang liên tục dựa trên cursor thời gian `created_at` giảm dần kết hợp định dạng ISO8601 UTC.

---

### 2.8. Đa Dạng Hóa Bảng Tin & Bể Trending (Activity Feed Diversity & Trending Pool)
- **Vấn đề giải quyết**: Khắc phục hiện tượng Cold Start (người dùng mới chưa theo dõi ai hoặc theo dõi ít người) và hiện tượng bảng tin đơn điệu, lặp đi lặp lại nội dung đã xem.
- **Tỷ lệ pha trộn 80/20 (Blending Ratio)**:
  - Bảng tin ưu tiên 80% dung lượng trang (`limit`) cho bài viết từ mạng lưới người đang theo dõi (`Following`).
  - Dành 20% dung lượng trang còn lại để lồng ghép các bài viết nổi bật từ **Bể Trending** của cộng đồng.
  - Trường hợp danh sách bạn bè không đủ 80%, toàn bộ vị trí còn trống sẽ tự động được lấp đầy bằng bài viết từ Bể Trending.
- **Bể Trending 500 bài trong 48 giờ (Trending Pool)**:
  - Bể chứa tối đa 500 bài viết công khai (`PUBLIC`) có tương tác cao nhất trong vòng 48 giờ qua.
  - Điểm nóng được tính theo công thức: $\text{Score} = (\text{reactions} \times 2) + (\text{comments} \times 5)$.
- **Khử trùng lặp nội dung bằng Bloom Filter (Deduplication)**:
  - Sử dụng cấu trúc dữ liệu xác suất **Redis Bloom Filter** để ghi nhớ những bài viết người dùng đã đọc trong 7 ngày qua.
  - Mỗi khi người dùng xem một trang feed, danh sách ID bài viết được nạp vào Bloom Filter của người dùng (`BF.MADD`).
  - Trong các lượt tải trang kế tiếp, những bài đã tồn tại trong Bloom Filter sẽ bị loại trừ (`BF.MEXISTS`), ngăn chặn việc người dùng nhìn thấy bài viết cũ nhiều lần.
  - Key Bloom Filter tự động hết hạn (TTL 7 ngày) để tránh rò rỉ bộ nhớ.
- **Neo con trỏ thời gian (Cursor Stability)**:
  - `nextCursor` của trang feed luôn bám sát theo mốc thời gian của bài viết Following cuối cùng được hiển thị, ngăn chặn triệt để lỗi trôi thời gian do bài viết Trending gây ra.
- **Van xả an toàn (Safety Valve Emergency Recycle)**:
  - Khi người dùng đã xem hết 100% nội dung (cả bạn bè lẫn Trending đều bị Bloom Filter chặn), hệ thống kích hoạt van xả an toàn: tự động lấy các bài viết công khai mới nhất mà không áp bộ lọc Bloom.
  - Đảm bảo người dùng không bao giờ rơi vào trạng thái "màn hình trắng trơn" (Empty Screen) khi lướt mạng xã hội.

---

### 2.9. Dữ Liệu Mẫu Cho Bot Mạng Xã Hội (Bot Social Seeding)
- **Mục tiêu**: Tự động sinh dữ liệu mạng xã hội phong phú cho các tài khoản Bot hệ thống nhằm phục vụ demo, báo cáo và nạp đầy Bể Trending 500 bài.
- **Quy tắc thực thi**:
  - Thực thi ngầm (Background goroutine) bất đồng bộ lúc khởi động server, không làm chậm quá trình khởi động hoặc chặn kết nối người dùng.
  - Thực thi hoàn toàn thông qua tầng API chuẩn (`SocialAPI`) với danh tính được gắn vào Context, tuyệt đối không chèn trực tiếp bằng SQL thô vào DB.
  - Đảm bảo tính lũy đẳng (Idempotent): Tự động kiểm tra số lượng bài viết hiện có của các bot, nếu đã có $\ge 200$ bài thì tự động bỏ qua để tránh trùng lặp dữ liệu khi restart server.
