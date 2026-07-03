# 9. Hướng Dẫn Thiết Kế Tầng Ứng Dụng (Application Layer Design Guidelines) - FITAI

Tài liệu này định nghĩa tiêu chuẩn lập trình, phân tách trách nhiệm và tổ chức thư mục cho **Tầng Ứng Dụng (Application / Use Case Layer)** trong kiến trúc hệ thống FITAI.

---

## 9.1 Vai Trò Của Tầng Ứng Dụng (The Orchestrator)

Tầng ứng dụng đóng vai trò là "nhà điều phối" (Orchestrator), kết nối các yêu cầu từ bên ngoài (API Controllers, WebSockets) với tầng Miền (Domain Layer) và tầng Hạ tầng (Infrastructure Layer).
Nhiệm vụ chính của tầng ứng dụng bao gồm:
1. Nhận dữ liệu đầu vào (DTOs) từ tầng giao tiếp (REST/gRPC controllers).
2. Tải các Aggregate Roots cần thiết từ cơ sở dữ liệu qua các Repository Interfaces.
3. Điều phối và kích hoạt các phương thức nghiệp vụ của Aggregate Root.
4. Quản lý giao dịch (Database Transaction) để đảm bảo tính toàn vẹn dữ liệu (commit/rollback).
5. Lưu lại trạng thái mới của Aggregate thông qua Repository.
6. Kích hoạt và phát đi các sự kiện tích hợp (Integration Events) ra bên ngoài qua Message Broker.

*Lưu ý quan trọng*: **Tầng ứng dụng không chứa bất kỳ quy tắc nghiệp vụ nào**. Nếu xuất hiện các câu lệnh điều kiện rẽ nhánh nghiệp vụ (ví dụ: `if formScore < 70`), mã nguồn đó bắt buộc phải chuyển vào Tầng Miền.

---

## 9.2 Hướng Thiết Kế Sử Dụng CQRS (Command Query Responsibility Segregation)

Để tối ưu hóa luồng xử lý và khả năng bảo trì, FITAI khuyến khích áp dụng mô hình **CQRS** phân tách giữa thao tác ghi (Command) và thao tác đọc (Query):

* **Command (Thao tác Ghi)**: 
  * Biểu diễn các hành động làm thay đổi trạng thái hệ thống (tạo mới, cập nhật, xóa).
  * Mỗi Command chỉ tương ứng với một Command Handler cụ thể và chạy trong một Database Transaction.
* **Query (Thao tác Đọc)**:
  * Biểu diễn các yêu cầu lấy thông tin dữ liệu (chỉ đọc).
  * Không làm thay đổi bất kỳ trạng thái nào của hệ thống.
  * Có thể truy vấn trực tiếp thông qua các Read Model hoặc cơ chế tối ưu riêng mà không cần đi qua Aggregate Root của tầng miền để tăng hiệu năng.

---

## 9.3 Cấu Trúc Thư Mục Tầng Ứng Dụng (Application Directory Layout)

Mỗi module nghiệp vụ tại `/internal/<module_name>` tổ chức tầng ứng dụng theo cấu trúc chuẩn sau:

```text
/internal/<module_name>/application/
├── command/             # Định nghĩa các Command và Command Handlers tương ứng
├── query/               # Định nghĩa các Query và Query Handlers tương ứng
├── dto/                 # Định nghĩa các tệp chuyển đổi dữ liệu (Request/Response DTOs)
└── port/                # Các cổng giao tiếp phụ trợ (ví dụ: EmailPort, PaymentPort, v.v.)
```

### Quy định lập trình:
* **DTO (Data Transfer Object)**: 
  * Định nghĩa cấu trúc dữ liệu thô nhận từ API controller và cấu trúc dữ liệu trả về cho client.
  * Sử dụng các thư viện validation để kiểm tra cú pháp đầu vào (ví dụ: định dạng email, độ dài chuỗi) trước khi truyền sang Use Case.
* **Handlers (Command/Query Handlers)**:
  * Nhận Command/Query Object và thực thi logic điều phối.
  * Không trực tiếp tương tác với các thư viện DB cụ thể mà chỉ gọi qua Interface Port.
