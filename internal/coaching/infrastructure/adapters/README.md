# Coaching Infrastructure Adapters

Thư mục này chứa các Adapter triển khai các Port Reader của Module Coaching (`UserProfileReader`, `WorkoutSessionReader`, `ExerciseCatalogReader`).

## Các Triển khai (Implementations)

### 1. PostgreSQL Readers (Production Ready)
- **`PostgresUserProfileReader`** (`postgres_user_profile_reader.go`):
  - Truy vấn dữ liệu thực tế từ schema `profile`.
  - Bảng `profile.users`: Đọc độ tuổi, kinh nghiệm, mục tiêu chính, trang thiết bị có sẵn, nhóm cơ ưu tiên, và khung giờ tập luyện rảnh.
  - Bảng `profile.body_metrics`: Đọc thông số cân nặng (`weight_kg`) và chiều cao (`height_cm`) mới nhất.
  - Bảng `profile.injuries`: Đọc danh sách các chấn thương đang hoạt động (chưa hồi phục).
  - *Fallback*: Khi không tìm thấy profile user trong DB (user mới chưa hoàn tất onboarding), reader trả về dữ liệu cơ bản an toàn để ứng dụng không bị rán dừng.

- **`PostgresWorkoutSessionReader`** (`postgres_workout_session_reader.go`):
  - Truy vấn dữ liệu thực tế từ schema `workout_execution`.
  - Bảng `workout_execution.workout_sessions`: Đọc lịch sử các buổi tập gần đây (`COMPLETED`/`ABORTED`), tính RPE trung bình và form score.
  - Bảng `workout_execution.workout_set_logs`: Đọc chi tiết các set tập gần đây cho một bài tập cụ thể.
  - Bảng `workout_execution.personal_records`: Đọc kỷ lục cá nhân (PR) khi user chưa có set log gần đây.
  - *Fallback*: Khi người dùng mới hoàn toàn chưa từng tập bài đó (không có set log lẫn PR), reader trả về danh sách rỗng `[]port.SetLog{}` để AI Agent tự nhận biết và kê tạ khởi tạo an toàn (thay vì giả định mức 80kg).

- **`PostgresExerciseCatalogReader`** (`postgres_exercise_catalog_reader.go`):
  - Truy vấn dữ liệu thực tế từ schema `exercise`.
  - Bảng `exercise.exercises`: Tìm kiếm và lọc bài tập theo nhóm cơ (`target_muscle_id`, `body_part_id`), thiết bị, độ khó, và từ khóa.
  - Tự động nhận biết loại thiết bị (`IsBodyweight`, `IsMachineOrCable`).
  - *Fallback*: Khi cơ sở dữ liệu chưa được seed dữ liệu bài tập, reader fallback về danh mục mặc định để đảm bảo môi trường phát triển luôn hoạt động.

### 2. Mock Readers (Testing & Offline Dev)
- `MockUserProfileReader`, `MockWorkoutSessionReader`, `MockExerciseCatalogReader` (`readers.go`): Dùng cho môi trường Unit Test độc lập hoặc phát triển offline không kết nối DB.

---

## Mục lục Kiểm thử (Test Index)

File test: [`postgres_readers_test.go`](file:///E:/LEARN/a/gym-companion/internal/coaching/infrastructure/adapters/postgres_readers_test.go)

| Tên Test Function / Case | Mục đích kiểm thử | Kịch bản chi tiết | Kết quả mong đợi |
| :--- | :--- | :--- | :--- |
| `TestPostgresUserProfileReader/returns_fallback_for_non-existent_user_profile` | Kiểm tra fallback cho user mới | User ID chưa tồn tại trong `profile.users` | Trả về profile mặc định an toàn, không báo lỗi đứt gãy |
| `TestPostgresUserProfileReader/reads_real_user_profile_and_body_metrics_from_DB` | Kiểm tra đọc dữ liệu thực từ DB | Chèn record user 52.5kg, goal strength, chấn thương vai | Đọc chính xác cân nặng 52.5kg, goal strength, slots & active injuries |
| `TestPostgresWorkoutSessionReader/returns_empty_slice_for_user_with_no_set_logs_or_PRs` | Kiểm tra an toàn cho user chưa có PR | Query set log cho bài bench-press của user mới | Trả về slice rỗng `[]SetLog{}` để Agent nhận biết không có PR |
| `TestPostgresWorkoutSessionReader/reads_set_logs_and_personal_records_from_DB` | Kiểm tra đọc lịch sử buổi tập & set log | Chèn 1 session completed & 1 set log tạ 120kg squat | Trả về đúng session ID và set log tạ 120kg squat |
| `TestPostgresWorkoutSessionReader/falls_back_to_personal_records_table_when_no_set_logs_exist` | Kiểm tra fallback đọc từ bảng `personal_records` | Không có set log trong `workout_set_logs`, chỉ có record trong `personal_records` | Đọc thành công tạ và reps từ bảng PR |
| `TestPostgresExerciseCatalogReader/searches_and_filters_exercises_in_catalog_DB` | Kiểm tra tìm kiếm & lọc bài tập từ catalog DB | Chèn 2 bài tập (Barbell Incline Press & Lat Pulldown), lọc theo chest | Tìm thấy đúng bài Barbell Incline Press với `IsMachineOrCable=false` |
| `TestPostgresExerciseCatalogReader/returns_mock_catalog_fallback_when_database_table_is_unseeded` | Kiểm tra fallback catalog khi DB rỗng | DB chưa seed bài tập | Fallback trả về bài tập từ catalog mặc định |
