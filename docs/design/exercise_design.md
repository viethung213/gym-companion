# Thiết Kế Module Exercise Catalog (Ý Định Thiết Kế)

Tài liệu này tóm tắt ý định thiết kế (Design Intent) cho Bounded Context **Exercise Catalog** dựa trên các hợp đồng API gRPC đã thống nhất.

---

## 1. Mục Tiêu Nghiệp Vụ (Intent & Scope)
- Cung cấp thư viện bài tập chuẩn hóa làm nền tảng cho việc tạo giáo án và cho Agentic AI Coach đề xuất bài tập cá nhân hóa.
- Quản lý các danh mục phân loại giải phẫu học cơ thể và dụng cụ tập luyện.
- Hỗ trợ công cụ tìm kiếm thông minh: Lọc chấn thương an toàn, khớp nhanh bài Warm-up/Cooldown và phân trang.

---

## 2. Mô Hình Trạng Thái & Vòng Đời (Lifecycle)
Bài tập được kiểm soát trạng thái chặt chẽ qua Enum `ExerciseStatus`:
- **`DRAFT`**: Bản nháp đang soạn thảo, chưa công khai. Cho phép xóa cứng.
- **`PENDING_APPROVAL`**: Đang chờ kiểm duyệt để đảm bảo an toàn động tác.
- **`ACTIVE`**: Đã phê duyệt, đưa vào thư viện sử dụng công khai.

---

## 3. Thiết Kế Hợp Đồng API (Protobuf APIs)

Tầng API được thiết kế tinh giản tối đa dựa trên 2 file proto:

### A. API Metadata Tập Trung (`GetCatalogMetadata`)
- **Mục tiêu**: Gom toàn bộ lookup tables tĩnh vào 1 API duy nhất.
- **Dữ liệu trả về**: Danh sách `body_parts`, `equipments`, `muscles`, và `tags`.
- **Lợi ích**: Frontend chỉ cần gọi 1 network request duy nhất để tải toàn bộ bộ lọc UI khi mở trang tìm kiếm.

### B. API Tìm Kiếm Thông Minh (`SearchExercises`)
- **Tham số lọc**: `body_part_id`, `equipment_id`, `target_muscle_id`, `secondary_muscle_ids`, `tag_ids`, `keyword`, `difficulty`.
- **An toàn chấn thương**: Tham số `avoid_injury_areas` tự động loại bỏ các bài tập nguy hiểm ở mức DB (ví dụ: chấn thương gối né đùi, chấn thương vai né đẩy vai).
- **Phân trang**: `limit` và `offset` điều khiển cuộn vô hạn.

### C. Bộ API CRUD Admin
- Quyền Admin được bảo vệ tại các endpoint `/api/v1/admin/exercises`:
  - `CreateExercise`: Tạo nháp bài tập mới.
  - `UpdateExercise`: Cập nhật thông tin hoặc đổi trạng thái phê duyệt.
  - `DeleteExercise`: Xóa cứng bản ghi khỏi database (tạm thời).
