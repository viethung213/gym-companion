# 16. Chiến Lược Kiểm Thử (Testing Strategy) - FITAI

Tài liệu này quy định các chính sách kiểm thử bắt buộc, cấp độ kiểm thử và các tiêu chuẩn về độ bao phủ kiểm thử (Test Coverage) áp dụng trên toàn bộ mã nguồn của hệ thống **FITAI**.

---

## 16.1 Các Cấp Độ Kiểm Thử Bắt Buộc (Testing Levels)

Hệ thống FITAI yêu cầu kiểm thử toàn diện trên cả 4 cấp độ để đảm bảo tính ổn định và tính mở rộng của mô hình Modular Monolith:

```text
                  ┌──────────────────────┐
                  │      E2E Tests       │ ◄── Luồng người dùng hoàn chỉnh (Onboarding -> Workout)
                  └──────────┬───────────┘
                  ┌──────────▼───────────┐
                  │  Integration Tests   │ ◄── Tích hợp cơ sở dữ liệu (Postgres, Mongo) & Message Broker
                  └──────────┬───────────┘
                  ┌──────────▼───────────┐
                  │    Contract Tests    │ ◄── Kiểm thử tương thích OpenAPI/gRPC Gateway (Buf)
                  └──────────┬───────────┘
                  ┌──────────▼───────────┐
                  │      Unit Tests      │ ◄── Kiểm thử Invariants của Domain & Use Cases Application
                  └──────────────────────┘
```

1. **Unit Tests (Kiểm thử đơn vị)**:
   * **Phạm vi**: Tập trung vào tầng **Domain** (Aggregate Roots, Entities, Domain Services) và tầng **Application** (CQRS Use Cases).
   * **Ràng buộc**: Tuyệt đối không kết nối mạng, I/O hoặc cơ sở dữ liệu vật lý. Sử dụng Mocking để giả lập hành vi của các Outbound Ports (Repository, AI client).
2. **Contract Tests (Kiểm thử hợp đồng API)**:
   * **Phạm vi**: Đảm bảo các cấu trúc dữ liệu JSON/Protobuf truyền tải thực tế tương thích hoàn toàn với các schema định nghĩa trong `/proto/contracts`.
   * **Ràng buộc**: Chạy kiểm tra tính tương thích ngược (Breaking Change Detection) thông qua Buf CLI và kiểm tra định dạng dữ liệu (Casing, Headers) ở cổng gRPC-Gateway.
3. **Integration Tests (Kiểm thử tích hợp)**:
   * **Phạm vi**: Kiểm tra chi tiết hoạt động của các Infrastructure Adapters.
   * **Ràng buộc**: Kết nối trực tiếp vào cơ sở dữ liệu Docker Testcontainers (PostgreSQL, MongoDB, Redis) để kiểm thử các câu truy vấn thực tế, kiểm tra việc đẩy/nhận tin nhắn từ Message Broker (Kafka/RabbitMQ).
4. **End-to-End (E2E) Tests (Kiểm thử đầu-cuối)**:
   * **Phạm vi**: Giả lập toàn bộ hành trình trải nghiệm của người dùng chạy xuyên suốt qua nhiều Module.
   * **Ràng buộc**: Chạy trong môi trường tích hợp cục bộ, giả lập từ bước đăng ký tài khoản (Auth) -> khai báo chấn thương (Profile) -> lên kế hoạch tập luyện (Coaching) -> gửi kết quả buổi tập (Workout) -> xuất báo cáo dinh dưỡng (Nutrition).

---

## 16.2 Chính Sách Kiểm Thử Bắt Buộc (Mandatory Testing Policies)

Toàn bộ đội ngũ phát triển phải tuân thủ nghiêm ngặt các quy tắc kiểm thử sau:

1. **Quy tắc Tính năng Hoàn thiện (Feature Completeness Rule)**:
   * Mọi tính năng mới hoặc thay đổi logic nghiệp vụ khi đưa vào hệ thống **bắt buộc** phải đi kèm với Unit Tests (cho Domain & Application) và Integration Tests (cho Infrastructure).
   * Pull Request (PR) không có test sẽ bị tự động từ chối.
2. **Quy tắc Tuyệt đối Không Giảm Độ Bao Phủ (No Coverage Regression Rule)**:
   * Không cho phép sáp nhập bất kỳ mã nguồn nào làm giảm phần trăm tổng độ bao phủ (Test Coverage) của dự án.
3. **Chỉ số Độ bao phủ Mục tiêu (Target Test Coverage)**:
   * **Core Logic (`>80%`)**: Áp dụng cho các thành phần nghiệp vụ cốt lõi nằm ở tầng Lõi Lục giác (Domain Services, Entities, Application Use Cases).
   * **Critical Middleware & Helpers (`>90%`)**: Áp dụng cho các thư viện tiện ích, cơ chế mã hóa, hoặc phần mềm trung gian quan trọng (ví dụ: Auth JWT Middleware, Rate Limiter, các hàm tính toán thời gian chạy nền).

---

## 16.3 Quy Trình Thực Thi Tự Động (CI/CD Pipeline)

Hệ thống CI/CD sẽ tự động chạy bộ kiểm thử mỗi khi có sự kiện đẩy code (Push) hoặc tạo Pull Request:
* Chạy linting kiểm tra định dạng code.
* Chạy `buf lint` và `buf breaking` kiểm tra tính tương thích API Contract.
* Khởi động các database test cục bộ để chạy Integration Tests.
* Xuất báo cáo độ bao phủ: `go test -coverprofile=coverage.out ./...` và kiểm tra xem có đạt chỉ tiêu `>80%` và không bị giảm coverage hay không.
