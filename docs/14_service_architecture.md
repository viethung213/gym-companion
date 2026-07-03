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

### Tại sao chọn Modular Monolith? (So sánh & Biện luận)

FITAI lựa chọn kiến trúc **Modular Monolith** làm mô hình kiến trúc chủ đạo cho giai đoạn hiện tại thay vì Microservices hay Monolith truyền thống vì các lý do chiến lược sau:

1. **So sánh với Monolith truyền thống (Spaghetti Monolith)**:
   - *Vấn đề của Monolith truyền thống*: Dễ dẫn đến mã nguồn bị trộn lẫn (tight coupling), các bảng cơ sở dữ liệu liên kết chéo vô tổ chức qua các câu lệnh `JOIN SQL`, khiến việc sửa đổi ở một module (ví dụ: Dinh dưỡng) có thể gây đổ vỡ không lường trước ở module khác (ví dụ: Tập luyện).
   - *Giải pháp của Modular Monolith*: FITAI phân tách ranh giới các module cực kỳ nghiêm ngặt. Mỗi module có cơ sở dữ liệu logic riêng (PostgreSQL Schema isolation) và hoàn toàn **cấm truy vấn chéo DB**. Các lập trình viên chỉ có thể tương tác thông qua các API Port hoặc Event rõ ràng, loại bỏ hoàn toàn mã nguồn spaghetti nhưng vẫn giữ sự đơn giản khi chạy trên một server vật lý.

2. **So sánh với Microservices**:
   - *Vấn đề của Microservices*: Đòi hỏi chi phí vận hành (DevOps) cực kỳ lớn (Docker, Kubernetes, Service Mesh, Distributed Tracing, API Gateways) và đội ngũ nhân sự lớn để quản lý. Đồng thời, giao tiếp giữa các dịch vụ qua mạng (HTTP/gRPC qua Network) sinh ra độ trễ mạng lớn và nguy cơ lỗi kết nối cao, yêu cầu cài đặt cơ chế Circuit Breaker phức tạp.
   - *Giải pháp của Modular Monolith*: Toàn bộ backend được gói gọn trong một đơn vị deploy duy nhất (Single Deployment Unit). Việc giao tiếp giữa các module (ví dụ: `coaching` gọi sang `workout_log`) được định tuyến trực tiếp trong bộ nhớ (In-process gRPC/In-memory calls) qua cơ chế in-process connection, giúp **triệt tiêu hoàn toàn độ trễ mạng** và nâng hiệu suất lên tối đa mà không tốn chi phí hạ tầng.

3. **Lộ trình tiến hóa linh hoạt (Sẵn sàng chuyển đổi thành Microservices)**:
   - Các module được đóng gói hoàn toàn độc lập theo chuẩn Hexagonal (Ports & Adapters). 
   - Nếu trong tương lai, một module cụ thể (ví dụ: module `workout` xử lý dữ liệu tracking skeleton telemetry rất nặng) cần mở rộng quy mô (horizontal scaling) độc lập, ta có thể dễ dàng tách riêng module đó ra thành một microservice độc lập chạy trên hạ tầng riêng mà **không cần phải tái cấu trúc hay viết lại phần core logic nghiệp vụ**.

---

## 14.2 Kiến Trúc Từng Module: Hexagonal Architecture (Ports & Adapters)

Mỗi module nghiệp vụ (Bounded Context) bên trong dự án được tổ chức theo kiến trúc **Hexagonal Architecture (Kiến trúc Lục giác)** hay **Ports & Adapters**, giúp độc lập hoàn toàn phần logic nghiệp vụ cốt lõi khỏi chi tiết công nghệ (Database, Framework, Web client, Dịch vụ bên thứ ba).

* **Hexagon Core (Lõi nghiệp vụ)**:
  * **Domain Layer**: Chứa Entities, Value Objects, Domain Services và **Ports (Interfaces)** xác định cách lõi giao tiếp ra bên ngoài (chưa có triển khai công nghệ cụ thể).
  * **Application Layer**: Chứa các Use Cases (CQRS Command/Query Handlers) điều phối luồng nghiệp vụ.
* **Outside the Hexagon (Bên ngoài lục giác)**:
  * **Infrastructure Layer**: Chứa các **Adapters (Bộ chuyển đổi)** cụ thể về mặt công nghệ (GORM Repositories kết nối DB, HTTP/gRPC handlers đón nhận request, các API Clients gọi AI model bên ngoài) để hiện thực hóa các Ports đã định nghĩa ở Core.

### Biện luận lựa chọn Hexagonal Architecture so với các kiến trúc khác

| Tiêu chí | Hexagonal Architecture (Ports & Adapters) | Layered Architecture (N-Tier) | Clean / Onion Architecture | Transaction Script / Active Record |
| :--- | :--- | :--- | :--- | :--- |
| **Tính thuần khiết của Lõi nghiệp vụ (Domain Purity)** | **Cực kỳ cao**. Lõi không chứa mã nguồn hạ tầng, không phụ thuộc thư viện ngoài. | **Thấp**. Tầng nghiệp vụ thường phụ thuộc trực tiếp vào tầng Data Access hoặc ORM Models. | **Rất cao**. Tách biệt nghiệp vụ thông qua các vòng tròn đồng tâm. | **Cực kỳ thấp**. Nghiệp vụ bị trộn lẫn trực tiếp trong các tệp DB Model hoặc controller. |
| **Khả năng viết Unit Test (Testability)** | **Cực kỳ dễ dàng**. Có thể Mock 100% các cổng Outbound Ports (Driven Ports) bằng Go interfaces. | **Khó**. Thường yêu cầu kết nối Database thật hoặc giả lập database rất phức tạp do dính chặt mã nguồn. | **Dễ**. Sử dụng cơ chế đảo chiều phụ thuộc tương tự như Hexagonal. | **Cực kỳ khó**. Hầu như không thể viết unit test độc lập mà phải chạy Integration test với DB. |
| **Độ phức tạp cấu trúc (Complexity & Overhead)** | **Vừa phải**. Cấu trúc rõ ràng thành 2 vùng chính: Trong lục giác (Nghiệp vụ) và Ngoài lục giác (Hạ tầng). | **Thấp**. Phân lớp đơn giản (UI -> BLL -> DAL) nhưng khó mở rộng. | **Rất cao**. Phân chia quá chi tiết thành nhiều vòng tròn (Entities, Use Cases, Gateways, Presenters) dễ gây over-engineering. | **Rất thấp**. Viết nhanh trong giai đoạn đầu nhưng nhanh chóng trở thành bãi rác (spaghetti) khi quy mô tăng. |
| **Khả năng thay thế công nghệ (Adaptability)** | **Cực kỳ linh hoạt**. Thay thế Database, Web Framework hoặc AI Client chỉ cần viết Adapter mới, không đổi code Core. | **Kém**. Đổi Database đòi hỏi phải viết lại hoặc sửa đổi trực tiếp mã nguồn tầng nghiệp vụ. | **Linh hoạt**. Tương đương với Hexagonal Architecture. | **Hầu như không thể**. Nếu thay đổi Database, toàn bộ hệ thống phải được viết lại. |

#### Tại sao FITAI chọn Hexagonal thay vì các kiến trúc còn lại?
1. **Khắc phục điểm yếu của Layered Architecture**: Trong Layered, nghiệp vụ bị phụ thuộc vào Database. Ở FITAI, các nghiệp vụ toán học/hình học (pose geometry) và sắp xếp dinh dưỡng là "tài sản" lớn nhất của dự án. Chúng ta không thể để mã nguồn nghiệp vụ này bị dính chặt hay thay đổi mỗi khi nâng cấp ORM hoặc thay đổi hệ quản trị cơ sở dữ liệu. Hexagonal đảo ngược sự phụ thuộc (Dependency Inversion), buộc Database phải phụ thuộc vào Nghiệp vụ thông qua các Ports.
2. **Tránh sự cồng kềnh của Clean/Onion Architecture**: Clean Architecture rất tốt nhưng thường sinh ra quá nhiều tệp tin trung gian và các cấu trúc phức tạp (như Presenters, ViewModels, Interactors) cho từng tính năng nhỏ. Hexagonal đơn giản hóa ranh giới chỉ thành **Ports & Adapters**, giúp đội ngũ lập trình viết code nhanh hơn trong mô hình Modular Monolith mà vẫn đảm bảo tính cô lập hoàn toàn.
3. **Phù hợp với lộ trình tách thành Microservices**: Khi cần tách một module (ví dụ `workout` tracking) ra thành một microservice độc lập, với Hexagonal ta chỉ cần thay thế các Adapter ở lớp ngoài (đổi Postgres Adapter thành gRPC/API Client Adapter) mà không cần chạm vào lõi Domain của các module khác. điều này không thể làm được nếu dùng Transaction Script.

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
| **Xác thực Token** | Mọi Module (Middleware) | `auth` | `AuthService.ValidateToken` | Middleware xác thực JWT, lấy `userId` và roles trước khi vào Use Case. |
| **Lấy Hồ sơ Sức khỏe** | `coaching`, `nutrition` | `profile` | `ProfileService.GetHealthProfile` | Lấy chỉ số cơ thể, chấn thương, thiết bị, dị ứng thực phẩm để AI lập kế hoạch/dinh dưỡng phù hợp. |
| **Cập nhật Thông tin Ngữ cảnh** | `coaching`, `nutrition` | `profile` | `ProfileService.UpdateContextInfo` | Chatbot cập nhật thiết bị tập (`equipment_list`) và dị ứng thực phẩm (`food_restrictions`) thu thập qua hội thoại. |
| **Lấy Giáo án JIT Hôm Nay** | `workout` | `coaching` | `CoachingService.GetTodayWorkoutSession` | Lấy giáo án bài tập, set, rep, tạ gợi ý và warm-up/cool-down sinh JIT cho ngày hiện tại. |
| **Lấy Lịch sử Luyện tập** | `coaching` | `workout_log` | `WorkoutLogService.GetWorkoutHistory` | Lấy RPE, Form score và volume của các buổi tập trước để sinh giáo án JIT và phân tích Trigger A/B. |
| **Lấy Danh Sách Bài Hát** | `workout` | `audio` | `AudioService.GetPlaylistConfig` | Lấy cấu hình danh sách bài hát và nhạc nền (EDM/Lofi). Lưu ý: Audio Ducking thực thi on-device. |
| **Gửi thông báo đẩy** | `workout`, `coaching`, `profile`, `auth` | `notification` | `NotificationService.SendPushNotification` | Yêu cầu gửi ngay lập tức thông báo nhắc lịch tập, Plateau, Overtraining, PR hoặc cảnh báo bảo mật. |

### 2. Bất đồng bộ (Asynchronous via Event-Driven)
Khi một hành động nghiệp vụ hoàn thành và các module khác cần biết để phản ứng tự do, hệ thống phát đi các **Integration Events** thông qua một Event Bus dùng chung (Kafka/RabbitMQ hoặc In-memory Event Bus).
* Ví dụ:
  * `UserRegistered` (Auth) ──► `Profile` tự động khởi tạo hồ sơ trống.
  * `ProfileCompleted` (Profile) ──► `Coaching` tự động kích hoạt AI Coach và khởi tạo roadmap; `Nutrition` tự động tính calo TDEE.
  * `WorkoutSessionCompleted` (Workout) ──► `Nutrition` cộng calo tiêu thụ vào hạn mức nạp hàng ngày; `Coaching` lưu volume lịch sử và chạy phân tích Trigger B; `Notification` đẩy tin chúc mừng thành tích và kỷ lục mới (PR).

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
