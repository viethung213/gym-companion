# 14. Kiến Trúc Dịch Vụ (Service Architecture) - FITAI

Tài liệu này xác định mô hình kiến trúc phân rã dịch vụ cho hệ thống **FITAI**, định hướng từ triển khai ban đầu đến lộ trình mở rộng quy mô.

---

## 14.1 Lựa Chọn Kiến Trúc: Đơn Khối Mô-Đun (Modular Monolith)

Hệ thống backend FITAI được thiết kế theo mô hình **Modular Monolith (Đơn khối dạng mô-đun)**. 

```text
┌────────────────────────────────────────────────────────┐
│                   FITAI BACKEND APP                    │
│                                                        │
│   ┌───────────────┐               ┌────────────────┐   │
│   │    profile    │ ──(In-Memory)─►    workout     │   │
│   │    Module     │               │     Module     │   │
│   └───────────────┘               └────────────────┘   │
│           │                               │            │
│           │                               │            │
│           ▼                               ▼            │
│   ┌───────────────┐               ┌────────────────┐   │
│   │   nutrition   │ ◄─(In-Memory)──  workout_log   │   │
│   │    Module     │               │     Module     │   │
│   └───────────────┘               └────────────────┘   │
│                                                        │
│ ────────────────────────────────────────────────────── │
│                SHARED KERNEL (Thư viện chung)          │
└────────────────────────────────────────────────────────┘
```

### Tại sao chọn Modular Monolith?
1. **Dễ triển khai & vận hành**: Chỉ cần bảo trì một repository và deploy một thực thể chạy duy nhất (Single Deployment Unit), giảm thiểu tối đa chi phí hạ tầng ban đầu.
2. **Giao tiếp hiệu năng cao**: Các module tương tác với nhau bằng các hàm gọi trực tiếp (In-memory calls) hoặc thông qua một Event Bus nội bộ của ứng dụng, tránh được độ trễ mạng (Network Latency) của microservices.
3. **Phân định ranh giới chặt chẽ**: Mỗi module sở hữu một cơ sở dữ liệu logic riêng (sử dụng Schema riêng trong PostgreSQL) và không được phép truy vấn chéo trực tiếp dữ liệu của module khác; mọi giao tiếp bắt buộc đi qua API Interface hoặc các sự kiện miền.
4. **Sẵn sàng chuyển đổi thành Microservices**: Nếu một module (ví dụ: `workout` cần mở rộng quy mô lớn do xử lý nhiều dữ liệu telemetry) bị quá tải, nó có thể dễ dàng tách ra thành một Service độc lập mà không cần viết lại toàn bộ mã nguồn.

---

## 14.2 Kiến Trúc Từng Module: Hexagonal Architecture (Ports & Adapters)

Mỗi module nghiệp vụ (Bounded Context) bên trong dự án được tổ chức theo kiến trúc **Hexagonal Architecture (Kiến trúc Lục giác)** hay **Ports & Adapters**, giúp độc lập hoàn toàn phần logic nghiệp vụ cốt lõi khỏi chi tiết công nghệ (Database, Framework, Web client, Dịch vụ bên thứ ba).

* **Hexagon Core (Lõi nghiệp vụ)**:
  * **Domain Layer**: Chứa Entities, Value Objects, Domain Services và **Ports (Interfaces)** xác định cách lõi giao tiếp ra bên ngoài (chưa có triển khai công nghệ cụ thể).
  * **Application Layer**: Chứa các Use Cases (CQRS Command/Query Handlers) điều phối luồng nghiệp vụ.
* **Outside the Hexagon (Bên ngoài lục giác)**:
  * **Infrastructure Layer**: Chứa các **Adapters (Bộ chuyển đổi)** cụ thể về mặt công nghệ (GORM Repositories kết nối DB, HTTP/gRPC handlers đón nhận request, các API Clients gọi AI model bên ngoài) để hiện thực hóa các Ports đã định nghĩa ở Core.

### Cấu Trúc Thư Mục Dự Án Toàn Diện (Project Directory Tree)

```text
.
├── cmd/
│   └── api/                    # Điểm khởi chạy ứng dụng (Main Entrypoint)
├── docs/
│   ├── swagger/                # Tài liệu Swagger OpenAPI generated
│   │   └── contracts/
│   │       ├── core/
│   │       ├── supporting/
│   │       └── generic/
│   └── ...                     # Các tài liệu phân tích thiết kế khác
├── internal/
│   ├── gen/                    # Thư mục chứa stubs tự động sinh ra
│   │   └── go/
│   │       └── contracts/      # Go gRPC & HTTP Gateway stubs từ proto
│   │           ├── core/
│   │           ├── supporting/
│   │           └── generic/
│   ├── shared/                 # Shared Kernel (Thư viện dùng chung)
│   │   ├── database/           # Bộ Helper kết nối DB vật lý (Postgres, Mongo, Redis)
│   │   └── eventbus/           # Bộ điều phối sự kiện in-memory hoặc Kafka wrapper
│   │
│   ├── workout/                # Module AI Workout Coaching & Execution (Core)
│   │   ├── domain/             # Lõi: Thực thể, Value Objects, Ports (Interfaces)
│   │   ├── application/        # Lõi: CQRS Use Cases, Command/Query Handlers
│   │   ├── infrastructure/     # Ngoài: DB Repositories, API handlers, AI Clients (Adapters)
│   │   └── docs/               # Lưu trữ tài liệu phân tích nghiệp vụ & kỹ thuật của module
│   ├── nutrition/              # Module AI Nutrition Engine (Core)
│   │   ├── domain/
│   │   ├── application/
│   │   ├── infrastructure/
│   │   └── docs/
│   ├── profile/                # Module User Profile & Onboarding (Supporting)
│   │   ├── domain/
│   │   ├── application/
│   │   ├── infrastructure/
│   │   └── docs/
│   ├── workout_log/            # Module Workout Logging & Progress (Supporting)
│   │   ├── domain/
│   │   ├── application/
│   │   ├── infrastructure/
│   │   └── docs/
│   ├── auth/                   # Module Identity & Auth (Generic)
│   │   ├── domain/
│   │   ├── application/
│   │   ├── infrastructure/
│   │   └── docs/
│   ├── notification/           # Module Notification Service (Generic)
│   │   ├── domain/
│   │   ├── application/
│   │   ├── infrastructure/
│   │   └── docs/
│   └── audio/                  # Module Audio Integration (Generic)
│       ├── domain/
│       ├── application/
│       ├── infrastructure/
│       └── docs/
└── proto/                      # Định nghĩa các API contract và cấu hình Buf
    ├── buf.yaml                # Cấu hình Buf module v2
    ├── buf.gen.yaml            # Cấu hình plugins sinh mã stubs (Go & Swagger)
    └── contracts/              # Thư mục chứa các tệp Protocol Buffers gốc
        ├── core/
        │   ├── workout/v1/
        │   └── nutrition/v1/
        ├── supporting/
        │   ├── profile/v1/
        │   └── workout_log/v1/
        └── generic/
            ├── auth/v1/
            ├── notification/v1/
            └── audio/v1/
```

---

## 14.3 Giao Tiếp Giữa Các Module (Inter-Module Communication)

Hệ thống kết hợp cả hai mô hình truyền thông để tối ưu hóa hiệu năng và độ độc lập:

### 1. Đồng bộ (Synchronous via Internal gRPC)
Khi một module cần truy vấn thông tin tức thời hoặc yêu cầu thực thi hành động khẩn cấp từ một module khác, nó sẽ gọi thông qua các **gRPC Client Interface** được sinh ra bởi `buf`. 

Trong Modular Monolith, các cuộc gọi này được định tuyến trực tiếp trong bộ nhớ (In-process gRPC calls) qua cơ chế in-process connection để triệt tiêu độ trễ mạng. Dưới đây là các luồng gRPC nội bộ được định nghĩa trong hệ thống:

| Luồng giao tiếp | Module gọi (Client) | Module nhận (Server) | Service & Method gRPC | Mục đích nghiệp vụ |
| :--- | :--- | :--- | :--- | :--- |
| **Xác thực Token** | `workout`, `nutrition`, `profile` (Middleware) | `auth` | `AuthService.ValidateToken` | Middleware xác thực JWT, lấy `userId` và roles trước khi vào Use Case. |
| **Lấy Hồ sơ Sức khỏe** | `workout`, `nutrition` | `profile` | `ProfileService.GetHealthProfile` | Lấy chỉ số cơ thể, mục tiêu và chấn thương để AI tính calo hoặc chọn bài tập phù hợp. |
| **Đánh giá hiệu suất** | `coaching` | `workout_log` | `WorkoutLogService.GetWorkoutHistory` | Lấy lịch sử 1RM và volume nâng tạ 2 tuần qua để chạy thuật toán tăng tải Progressive Overload. |
| **Giảm âm lượng nhạc** | `workout` | `audio` | `AudioService.RequestAudioDucking` | Ra lệnh hạ nhạc nền hệ thống xuống 20% khi có lỗi tư thế nguy hiểm để phát Voice Alert sửa lỗi. |
| **Gửi thông báo đẩy** | `workout`, `coaching` | `notification` | `NotificationService.SendPushNotification` | Yêu cầu gửi ngay lập tức thông báo chúc mừng PR hoặc cảnh báo bảo mật đến thiết bị người dùng. |

### 2. Bất đồng bộ (Asynchronous via Event-Driven)
Khi một hành động nghiệp vụ hoàn thành và các module khác cần biết để phản ứng tự do, hệ thống phát đi các **Integration Events** thông qua một Event Bus dùng chung (Kafka/RabbitMQ hoặc In-memory Event Bus).
* Ví dụ:
  * `UserRegistered` (Auth) ──► `Profile` tự động khởi tạo hồ sơ trống.
  * `ProfileCompleted` (Profile) ──► `Coaching` tự động kích hoạt AI Coach và gen giáo án.
  * `WorkoutSessionCompleted` (Workout) ──► `Nutrition` tính lại calo hạn mức; `Notification` đẩy tin chúc mừng thành tích.

---

## 14.4 Nguyên Tắc Phân Tách và Kết Nối Database

Hệ thống tuân thủ nghiêm ngặt tính độc lập dữ liệu giữa các module:

1. **Cô lập về mặt Logic (Logical Database/Schema Isolation)**:
   * Chạy trên cùng 1 server vật lý để giảm chi phí, nhưng phân chia thành các Database logical hoặc các **Schema độc lập** cho từng module (ví dụ: `workout_schema`, `profile_schema`, `auth_schema`).
   * Cấm hoàn toàn việc viết truy vấn `JOIN` SQL hoặc gọi trực tiếp bảng dữ liệu của module khác. Giao tiếp chéo chỉ thực hiện qua API.
2. **Khởi tạo và Phân phối kết nối (Dependency Injection)**:
   * Thư mục `/internal/shared/database/` chỉ chứa hàm dựng kết nối (connection factory) dùng chung như kết nối Postgres, Mongo, Redis.
   * Khi khởi chạy ứng dụng, file khởi động `cmd/api/main.go` sẽ tạo ra các kết nối Logical riêng biệt cho từng Module (cài đặt schema riêng biệt).
   * Các kết nối này sẽ được Inject vào tầng `infrastructure/persistence` của từng module tương ứng trong quá trình khởi tạo ứng dụng.

---

## 14.5 Tích Hợp các Thành Phần Trí Tuệ Nhân Tạo (AI Components)

Các thành phần AI trong FITAI được phân định vị trí rõ ràng theo kiến trúc Hexagonal:

1. **Thuật toán thuần túy (Rule-based, Math, Geometrics)**:
   * Các logic tính toán không cần gọi API bên thứ ba hay file model cồng kềnh (ví dụ: tính toán góc khớp xương từ tọa độ điểm neo, thuật toán sắp xếp thực đơn chống lặp món ăn).
   * **Vị trí**: Đặt tại tầng **`domain/services/`** dưới dạng hàm Go thuần túy (Pure Go functions), độc lập công nghệ và cực kỳ dễ kiểm thử (Unit Test).
2. **Mô hình học máy và Gọi API dịch vụ AI (Inference Engines, LLMs, external ML API clients)**:
   * Các thành phần phụ thuộc công nghệ hoặc I/O (gọi OpenAI API để sinh mô tả thực đơn, gọi Python AI service qua gRPC để phân tích video).
   * **Vị trí**:
     * **Cổng (Ports)**: Khai báo Interface tại tầng **`domain/ports/`** (ví dụ: `type PoseClassifier interface`).
     * **Bộ chuyển đổi (Adapters)**: Hiện thực hóa interface tại tầng **`infrastructure/ai/`** (ví dụ: `python_model_adapter.go`, `openai_menu_adapter.go`).
