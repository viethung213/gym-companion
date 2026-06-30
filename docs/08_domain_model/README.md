# 8. Hướng Dẫn Thiết Kế Mô Hình Miền (Domain Model Design Guidelines) - FITAI

Tài liệu này định nghĩa tiêu chuẩn lập trình và tổ chức thư mục cho **Tầng Miền (Domain Layer)** của hệ thống FITAI nhằm bảo vệ tính thuần khiết của mô hình nghiệp vụ cốt lõi, tránh bị phụ thuộc vào các công nghệ và hạ tầng bên ngoài.

---

## 8.1 Triết Lý Thiết Kế Tầng Miền (Domain Layer Purity)

Tầng Miền chứa đựng toàn bộ các quy tắc bất biến nghiệp vụ (Domain Invariants). Để đảm bảo tầng này không bị ảnh hưởng khi công nghệ thay đổi (ví dụ đổi Database hay Web Framework):
1. **Không phụ thuộc thư viện ngoài**: Các tệp tin trong tầng miền chỉ sử dụng thư viện chuẩn của ngôn ngữ (Go Standard Library), tuyệt đối không import ORM (như GORM), Web Framework (như Gin), hay các thư viện hạ tầng khác.
2. **Không có Tag dữ liệu**: Các struct thực thể không được chứa các tag định dạng cơ sở dữ liệu hoặc JSON (ví dụ `gorm:"primaryKey"` hay `json:"id"`). Việc ánh xạ dữ liệu sẽ do tầng hạ tầng tự đảm nhận qua các DTO (Data Transfer Objects).
3. **Kiểm soát tính bất biến (Domain Invariants)**: Các thực thể không cung cấp trực tiếp các thuộc tính ra ngoài để sửa đổi tùy ý. Mọi thay đổi trạng thái phải thông qua các phương thức nghiệp vụ rõ nghĩa (ví dụ: `Activate()`, `AddSet()`) để tự động kiểm tra tính hợp lệ trước khi thực hiện.

---

## 8.2 Cấu Trúc Thư Mục Tầng Miền (Domain Directory Layout)

Mỗi module nghiệp vụ (Bounded Context) nằm trong thư mục `/internal/<module_name>` phải tổ chức tầng miền theo cấu trúc chuẩn sau:

```text
/internal/<module_name>/domain/
├── aggregate/           # Định nghĩa các Aggregate Roots (Thực thể gốc điều phối giao dịch)
├── entity/              # Định nghĩa các Entities phụ thuộc khác (có danh tính riêng)
├── value_object/        # Định nghĩa các Đối tượng giá trị (Value Objects - bất biến, không danh tính)
├── event/               # Định nghĩa các Sự kiện miền nội bộ (Domain Events)
├── repository/          # Định nghĩa các Giao diện lưu trữ (Repository Interfaces - cổng Outbound Port)
└── service/             # Định nghĩa các Dịch vụ miền (Domain Services - phối hợp nhiều Aggregate)
```

### 1. Quy định cho Aggregate / Entity
* Mọi thực thể bắt buộc phải có thuộc tính định danh duy nhất (Identity).
* Aggregate Root là điểm tiếp xúc duy nhất từ bên ngoài. Các Entity phụ thuộc bên trong Aggregate Root không được phép truy xuất trực tiếp từ các use case mà phải thông qua Aggregate Root.

### 2. Quy định cho Value Object
* Value Object là đối tượng bất biến (Immutable), không có thuộc tính định danh.
* Khi muốn thay đổi giá trị của một Value Object bên trong Entity, ta phải thay thế hoàn toàn bằng một thực thể Value Object mới thay vì sửa thuộc tính của nó.

### 3. Quy định cho Repository Interfaces
* Tầng miền chỉ định nghĩa Interface mô tả các hành động cần thiết với cơ sở dữ liệu (ví dụ: `GetByID`, `Save`).
* Việc triển khai chi tiết giao tiếp với DB thực tế (PostgreSQL, MongoDB) sẽ được viết ở tầng Hạ tầng (`/infrastructure`).
