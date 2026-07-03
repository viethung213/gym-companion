# 11. Hướng Dẫn Thiết Kế Tầng Lưu Trữ (Persistence Layer Design Guidelines) - FITAI

Tài liệu này định nghĩa tiêu chuẩn thiết kế, phân tách và tổ chức thư mục cho **Tầng Lưu Trữ (Persistence / Database Layer)** thuộc Tầng Hạ Tầng (Infrastructure Layer) trong hệ thống FITAI.

---

## 11.1 Kiến Trúc Đa Cơ Sở Dữ Liệu (Polyglot Persistence)

Để tối ưu hóa hiệu năng và mục đích sử dụng dữ liệu, FITAI định hướng áp dụng các loại cơ sở dữ liệu sau:
1. **PostgreSQL (Cơ sở dữ liệu quan hệ)**:
   * Sử dụng để lưu trữ các dữ liệu nghiệp vụ quan trọng, yêu cầu tính toàn vẹn cao và các giao dịch ACID chặt chẽ (ví dụ: Thông tin người dùng, lịch trình, nhật ký tập luyện và ăn uống).
2. **MongoDB (Cơ sở dữ liệu tài liệu)**:
   * Sử dụng để lưu trữ các dữ liệu thô bán cấu trúc có kích thước lớn (ví dụ: Tọa độ khớp xương skeleton thô từ thiết bị truyền về để phục vụ phân tích hậu kỳ).
3. **Redis (Cơ sở dữ liệu bộ nhớ đệm)**:
   * Sử dụng để lưu trữ các thông tin tạm thời cần truy xuất cực nhanh (ví dụ: Session xác thực, cache khóa nguyên liệu, trạng thái kết nối socket).

---

## 11.2 Nguyên Tắc Phân Tách Giữa Thực Thể Miền Và Tầng Lưu Trữ

Để bảo vệ tính thuần khiết của Tầng Miền (Domain Layer), hệ thống áp dụng nguyên tắc **Data Mapper Pattern**:
* **Không lưu trực tiếp Aggregate Root**: Thực thể miền (`Domain Entity`) không bao giờ được lưu trực tiếp vào database.
* **Sử dụng Data Model**: Mỗi thực thể trong DB sẽ được đại diện bởi một `Data Model` tương ứng (ví dụ: struct ORM của GORM hoặc struct BSON của MongoDB).
* **Cơ chế Ánh xạ (Mapping)**: 
  * Tầng lưu trữ định nghĩa các hàm mapper để chuyển đổi qua lại:
    * `ToDomain(dbModel) -> DomainEntity` (khi đọc dữ liệu từ DB lên).
    * `ToPersistence(domainEntity) -> DBModel` (khi ghi dữ liệu từ Domain xuống DB).

---

## 11.3 Cấu Trúc Thư Mục Tầng Lưu Trữ (Persistence Directory Layout)

Mã nguồn triển khai chi tiết giao tiếp database được tổ chức bên trong thư mục `/internal/<module_name>/infrastructure/persistence/`:

```text
/internal/<module_name>/infrastructure/persistence/
├── postgres/            # Triển khai lưu trữ PostgreSQL
│   ├── model/           # Các struct ánh xạ bảng DB (ví dụ: GORM models)
│   ├── repository/      # Thực thi các Repository Interfaces được định nghĩa ở Tầng Miền
│   └── migration/       # Các tệp kịch bản SQL Migration của riêng module (ví dụ: .up.sql, .down.sql)
├── mongodb/             # Triển khai lưu trữ MongoDB (nếu có)
└── redis/               # Triển khai lưu trữ và caching Redis (nếu có)
```

### Quy định quản lý Schema (Database Migrations):
* Tuyệt đối không sử dụng tính năng tự động tạo bảng (như `AutoMigrate` của GORM) trong môi trường production.
* Toàn bộ thay đổi cơ sở dữ liệu phải được quản lý thông qua các tệp **Migration SQL** có đánh số phiên bản đặt tại thư mục `migration/` của từng module và được thực thi thông qua công cụ chạy migration tập trung.

### Quản lý Giao Dịch (Transactions):
* Các thao tác ghi phối hợp nhiều Aggregate phải đảm bảo chạy chung một transaction.
* Tầng lưu trữ phải hỗ trợ cơ chế nhận transaction context truyền xuống từ tầng ứng dụng (ví dụ: thông qua `context.Context` mang session của GORM hoặc `*sql.Tx`).
