# Coaching Infrastructure Adapters

Thư mục này chứa các Adapter triển khai các Port Reader của Module Coaching (`UserProfileReader`, `WorkoutSessionReader`, `ExerciseCatalogReader`).

Theo thiết kế kiến trúc hệ thống và nguyên tắc **Schema Isolation**, toàn bộ truy vấn chéo dữ liệu sang các Bounded Context khác (`profile`, `workout_execution`, `exercise`) được thực hiện thông qua **ConnectRPC Client qua API Gateway**, thay vì truy cập trực tiếp vào database của context khác.

---

## Các Triển khai (Implementations)

### ConnectRPC API Client Readers (Production Ready)

Giao tiếp với `service_core` thông qua **API Gateway** bằng giao thức **ConnectRPC (HTTP/2 cleartext - h2c)** và xác thực nội bộ qua header `X-Internal-Token`.

- **`ConnectUserProfileReader`** (`connect_user_profile_reader.go`):
  - Triển khai `port.UserProfileReader`.
  - Sử dụng client `profilev1serviceconnect.ProfileServiceClient` gọi RPC `GetProfile(userID)`.
  - Đọc và ánh xạ các thông tin: độ tuổi, cân nặng (`weight_kg`), chiều cao (`height_cm`), mục tiêu chính (`primary_goal`), trang thiết bị (`available_equipment`), nhóm cơ ưu tiên (`preferred_muscle_groups`), khung giờ tập luyện rảnh (`available_slots`), và danh sách các chấn thương đang hoạt động (`active_injuries`).

- **`ConnectWorkoutSessionReader`** (`connect_workout_session_reader.go`):
  - Triển khai `port.WorkoutSessionReader`.
  - Sử dụng client `workoutexecutionv1serviceconnect.AdminWorkoutExecutionServiceClient`:
    - `GetRecentSessions`: Gọi RPC `AdminGetWorkoutHistory` để lấy danh sách các buổi tập gần đây của người dùng, lọc theo mốc thời gian `since` và tính điểm form trung bình.
    - `GetSetLogs`: Gọi RPC `AdminGetPersonalRecords` để tra cứu kỷ lục 1RM gần nhất cho một bài tập cụ thể.
  - Khi người dùng mới chưa có lịch sử buổi tập hoặc chưa có kỷ lục PR, reader trả về slice rỗng để AI Agent nhận biết và khởi tạo mức tạ bắt đầu an toàn.

- **`ConnectExerciseCatalogReader`** (`connect_exercise_catalog_reader.go`):
  - Triển khai `port.ExerciseCatalogReader`.
  - Sử dụng client `exercisev1serviceconnect.ExerciseServiceClient`:
    - `SearchByFilter`: Gọi RPC `SearchExercises` để tìm kiếm và lọc bài tập active theo nhóm cơ, thiết bị, độ khó, vùng chấn thương cần tránh, và từ khóa. Tự động nhận diện thiết bị (`IsBodyweight`, `IsMachineOrCable`).
    - `GetByID`: Gọi RPC `GetExercise(id)` để lấy thông tin chi tiết của bài tập. Trả về đúng `port.ErrExerciseNotFound` khi bài tập không tồn tại.

- **`newDefaultHTTPClient`** (`http_client.go`):
  - Khởi tạo `*http.Client` chuẩn HTTP/2 Cleartext (h2c) với connection pooling, idle timeout và request timeout (10s).

---

## Mục lục Kiểm thử (Test Index)

File test: [`connect_readers_test.go`](file:///e:/LEAN/DATN/gym-companion/internal/coaching/infrastructure/adapters/connect_readers_test.go)

| Test Function / Suite | Mục đích kiểm thử | Kịch bản chi tiết | Kết quả mong đợi |
| :--- | :--- | :--- | :--- |
| `TestConnectUserProfileReader_Success` | Đọc thành công profile qua RPC | Mock `GetProfile` trả về dữ liệu cân nặng, chiều cao, slot nhiều định dạng JSON/string, chấn thương | Ánh xạ chính xác sang `port.Profile`, active injury được lọc đúng |
| `TestConnectUserProfileReader_NotFound_Error` | Xử lý lỗi khi user chưa có profile | Server trả về mã lỗi `connect.CodeNotFound` | Báo lỗi không tìm thấy profile |
| `TestConnectUserProfileReader_EmptyUserID` | Kiểm tra validation đầu vào | Truyền `userID = ""` | Báo lỗi `user id cannot be empty` ngay lập tức |
| `TestConnectExerciseCatalogReader_SearchByFilter` | Tìm kiếm bài tập với bộ lọc | Gửi filter theo nhóm cơ `chest`, mock trả về 3 bài tập | Trả về danh sách bài tập kèm gắn nhãn `IsBodyweight`, `IsMachineOrCable` chính xác |
| `TestConnectExerciseCatalogReader_GetByID_SuccessAndNotFound` | Lấy bài tập theo ID và xử lý NotFound | Gọi bài tồn tại và bài không tồn tại / ID rỗng | Bài tồn tại trả về đúng struct, bài không tồn tại trả về `port.ErrExerciseNotFound` |
| `TestConnectWorkoutSessionReader_GetRecentSessions` | Lấy lịch sử buổi tập gần nhất | Mock trả về 2 session, lọc với mốc thời gian `since` | Chỉ trả về session thỏa mãn `since`, tính đúng `AverageFormScore` |
| `TestConnectWorkoutSessionReader_GetSetLogs` | Lấy log set / PR cho bài tập | Mock trả về PR cho `bench-press` và rỗng cho `squat` | Tìm thấy PR -> trả về 1RM, không có PR -> trả về slice rỗng |
| `TestConnectReaders_ConstructorsAndErrors` | Kiểm tra khởi tạo default HTTP client & xử lý lỗi mạng | Khởi tạo với `httpClient = nil` và gửi request tới địa chỉ không tồn tại | Khởi tạo thành công client h2c và bọc lỗi mạng chuẩn Go |
