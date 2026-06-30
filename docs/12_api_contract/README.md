# 12. Thiết Kế Hợp Đồng API (API Contract) - FITAI

Tài liệu này định nghĩa tiêu chuẩn thiết kế hợp đồng API (REST & gRPC) cho toàn bộ hệ thống FITAI, sử dụng phương pháp tiếp cận **Contract-First** và quản lý tập trung bằng công cụ **Buf**.

---

## 12.1 Triết lý Thiết kế: Contract-First với Protocol Buffers & Buf

Hệ thống FITAI áp dụng phương pháp thiết kế **Contract-First** sử dụng **Protocol Buffers (Protobuf)** làm Nguồn sự thật duy nhất (Single Source of Truth) cho tất cả các giao diện giao tiếp:
1. **Định nghĩa Hợp đồng**: Mọi API endpoints (REST & gRPC) và sự kiện (Events) được định nghĩa trong các file `.proto` đặt trong thư mục `/proto/contracts` ở gốc dự án.
2. **Quản lý bằng Buf CLI**: Sử dụng công cụ `buf` để lint, kiểm tra tính tương thích ngược (breaking change detection) và quản lý phiên bản.
3. **Sinh mã tự động (Auto-Generation)**:
   * **Mã nguồn Go**: Sinh ra các structs và gRPC Server/Client Code cho Backend.
   * **OpenAPI Specs / Swagger**: Tự động tạo file `*.swagger.json` từ Protobuf thông qua plugin `protoc-gen-openapiv2` để phục vụ làm tài liệu API cho Frontend/Mobile Client.
   * **gRPC-Gateway**: Sinh mã chuyển đổi tự động từ HTTP REST API sang gRPC nội bộ.

---

## 12.2 Tiêu Chuẩn Thiết Kế API (API Design Standards)

Để các API có tính nhất quán cao, việc định nghĩa trong các file `.proto` phải tuân thủ các quy tắc sau:

### 1. Định dạng URL Resource-Oriented
* Đường dẫn HTTP REST (định nghĩa qua annotation `google.api.http` trong file `.proto`) phải sử dụng danh từ số nhiều và định dạng resource-oriented.
* Các biến đường dẫn sử dụng định dạng **`camelCase`**.
* *Ví dụ*:
  * Khai báo hồ sơ: `POST /api/v1/users/{userId}/profile`
  * Nhật ký set tập: `POST /api/v1/workouts/sessions/{sessionId}/sets`

### 2. Định dạng JSON Payload
* Tất cả các trường dữ liệu JSON thô gửi lên hoặc nhận lời giải đáp đều sử dụng định dạng **`camelCase`** (ví dụ: `userId`, `accessToken`).
* Điều này được cấu hình tự động khi sinh mã thông qua plugin `protoc-gen-openapiv2` với tùy chọn `json_names_for_fields=true`.

---

## 12.3 Cấu Trúc Thư Mục API Contract `/proto`

Thư mục chứa các hợp đồng API được tổ chức như sau:

```text
/proto
├── buf.yaml                      # Cấu hình Buf CLI v2 (linter, dependencies)
├── buf.gen.yaml                  # Quy tắc sinh mã (Go, gRPC-Gateway, OpenAPI)
└── contracts/                    # Thư mục chứa các API Contracts định nghĩa bằng Proto
    ├── core/                     # Miền cốt lõi (Core Domain)
    │   ├── workout/v1/           # Bounded Context: AI Workout Analysis & Coaching
    │   └── nutrition/v1/         # Bounded Context: AI Nutrition Engine
    ├── supporting/               # Miền hỗ trợ (Supporting Domain)
    │   ├── profile/v1/           # Bounded Context: User Profile & Onboarding
    │   └── workout_log/v1/       # Bounded Context: Workout Logging & Tracking
    └── generic/                  # Miền chung (Generic Domain)
        ├── auth/v1/              # Bounded Context: Identity & Authentication
        ├── notification/v1/      # Bounded Context: Notification Service
        └── audio/v1/             # Bounded Context: Audio Integration Service
```

Mỗi thư mục con của Bounded Context sẽ chứa:
* `<context_name>_service.proto`: Định nghĩa các gRPC service và REST gateway mapping.
* `<context_name>_events.proto`: Định nghĩa các cấu trúc sự kiện trao đổi.

---

## 12.4 Quy Trình Sinh Mã Tập Trung (Centralized Code-Gen Workflow)

Vì backend của FITAI được thiết kế theo mô hình **Modular Monolith** chạy trong cùng một repository (monorepo), hệ thống áp dụng quy trình sinh mã tập trung một lần duy nhất (Generate Once) thay vì sinh mã riêng biệt cho từng service:

1. **Biên dịch một lần (Unified Compilation)**:
   * Chúng ta chạy lệnh `buf generate` một lần tại gốc thư mục `/proto` để biên dịch toàn bộ các tệp `.proto` (của cả Core, Supporting và Generic domains) cùng một lúc.
2. **Sử dụng chung (Central Import)**:
   * Tất cả các Module nghiệp vụ trong hệ thống (như workout, profile, auth) sẽ trực tiếp import các structs Go và gRPC stubs từ thư mục dùng chung này.
   * Việc này đảm bảo tính đồng nhất kiểu dữ liệu (Type Consistency) khi các module truyền nhận sự kiện in-memory qua Event Bus nội bộ hoặc gọi gRPC trực tiếp trong bộ nhớ (In-process gRPC calls) mà không gặp lỗi type mismatch.
3. **Quy định vị trí đầu ra (Output Directories)**:
   * Mã nguồn Go stubs tự động sinh ra sẽ được lưu tập trung tại: `/internal/gen/go/contracts/`.
   * Đặc tả Swagger OpenAPI tự động sinh ra sẽ được lưu tập trung tại: `/docs/swagger/contracts/`.
