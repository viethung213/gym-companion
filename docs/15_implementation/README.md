# 15. Triển Khai Mã Nguồn (Implementation) - FITAI

Tài liệu này cung cấp bản vẽ lập trình chi tiết (Coding Blueprint), giải thích cách áp dụng kiến trúc **Hexagonal (Ports & Adapters)** và thiết lập cấu trúc mã nguồn cho từng module nghiệp vụ trong hệ thống FITAI.

---

## 15.1 Sơ Đồ Thiết Kế Hexagonal Architecture

Mỗi module nghiệp vụ tự đóng gói độc lập và giao tiếp với bên ngoài thông qua hai loại Ports:
* **Driving Ports (Cổng vào - Inbound)**: Nhận yêu cầu từ bên ngoài (ví dụ: HTTP REST request, gRPC request, CLI command). Lớp điều phối (Application Use Case) chính là thành phần hiện thực hóa các cổng này.
* **Driven Ports (Cổng ra - Outbound)**: Lõi gọi ra bên ngoài để tương tác với cơ sở dữ liệu, dịch vụ AI, hệ thống gửi thông báo. Lõi chỉ định nghĩa **Interface (Cổng)** tại tầng `domain`, còn hạ tầng ngoài sẽ cung cấp **Adapter (Bộ chuyển đổi)** để thực thi.

```text
               OUTSIDE THE HEXAGON                     INSIDE THE HEXAGON
       ┌─────────────────────────────────┐
       │   DRIVING ADAPTERS (Cổng vào)   │
       │   - HTTP REST Handlers          │
       │   - gRPC Server Handlers        │
       │   - Event Consumers (Kafka)     │
       └────────────────┬────────────────┘
                        │
                        ▼ (Calls)
       ┌─────────────────────────────────┐
       │     APPLICATION USE CASES       │
       │   - CQRS Command Handlers       │ ◄─── Lõi điều phối nghiệp vụ
       │   - CQRS Query Handlers         │
       └────────────────┬────────────────┘
                        │
                        ▼ (Uses)
       ┌─────────────────────────────────┐
       │    DOMAIN CORE (Lõi miền)       │
       │   - Aggregate Roots & Entities  │
       │   - Value Objects               │
       │   - Domain Services (AI Math)   │
       │   - Driven Ports (Interfaces)   │
       └────────────────┬────────────────┘
                        │
                        ▼ (Implemented by)
       ┌─────────────────────────────────┐
       │   DRIVEN ADAPTERS (Cổng ra)     │
       │   - PostgreSQL GORM Repositories│
       │   - MongoDB Keypoint Repositories│
       │   - OpenAI Menu Clients         │
       │   - Python AI gRPC Clients      │
       └─────────────────────────────────┘
```

---

## 15.2 Chi Tiết Cấu Trúc Mã Nguồn Một Module

Dưới đây là thiết kế chi tiết cấu trúc thư mục của hai module Core tiêu biểu (`workout` và `nutrition`), thể hiện vị trí của Ports, Adapters, các thuật toán AI và phân tách Database:

### 1. Module Luyện Tập (`internal/workout`)

```text
internal/workout/
├── domain/                         # Lõi Nghiệp Vụ Thuần Khiết (Không chứa thư viện ngoài)
│   ├── models/                     # Entities & Value Objects
│   │   ├── workout_session.go      # Thực thể gốc (Aggregate Root)
│   │   └── pose_keypoint.go        # Đối tượng giá trị (33 tọa độ x, y, z)
│   ├── services/
│   │   └── pose_geometry.go        # Thuật toán hình học góc xương (AI Math - Pure Go)
│   └── ports/                      # Driven Ports (Interfaces)
│       ├── workout_repo.go         # Interface lưu trữ chỉ số buổi tập
│       ├── keypoint_repo.go        # Interface lưu trữ chuỗi thời gian tọa độ 3D
│       └── pose_classifier.go      # Interface nhận dạng lỗi tư thế qua mô hình AI
│
├── application/                    # Lớp điều phối Use Cases (CQRS)
│   ├── commands/
│   │   ├── start_session.go        # Lệnh bắt đầu buổi tập
│   │   └── log_set.go              # Lệnh ghi nhận Set tập (Gồm logic gọi Port pose_classifier)
│   └── queries/
│       └── get_active_session.go   # Lấy thông tin buổi tập hiện tại
│
├── infrastructure/                 # Tầng hạ tầng chứa các Adapters thực tế
│   ├── persistence/                # Driven Adapters (Database)
│   │   ├── postgres_repo.go        # Triển khai lưu trữ Postgres GORM (Inject schema 'workout')
│   │   └── mongo_keypoint_repo.go  # Triển khai lưu trữ MongoDB (Lưu chuỗi tọa độ 3D)
│   ├── ai/                         # Driven Adapters (AI Models/Services)
│   │   └── python_grpc_client.go   # Adapter gọi gRPC sang Python AI Service để chạy model ML
│   └── transport/                  # Driving Adapters (Giao thức mạng vào)
│       ├── grpc_handler.go         # Tiếp nhận request gRPC stubs
│       └── http_handler.go         # Đăng ký HTTP REST endpoints cho gRPC-Gateway
│
└── docs/                           # Tài liệu phân tích nghiệp vụ & kỹ thuật áp dụng riêng của module
    └── README.md
```

### 2. Module Dinh Dưỡng (`internal/nutrition`)

```text
internal/nutrition/
├── domain/
│   ├── models/
│   │   └── nutrition_plan.go       # Thực thể kế hoạch dinh dưỡng
│   ├── services/
│   │   └── anti_repetition.go      # Giải thuật chống lặp món (Rule-based AI - Pure Go)
│   └── ports/
│       ├── nutrition_repo.go       # Cổng ra DB
│       └── menu_generator.go       # Interface định nghĩa bộ tạo thực đơn thông minh
│
├── application/
│   └── commands/
│       └── generate_today_menu.go  # Use Case phối hợp gọi port menu_generator
│
├── infrastructure/
│   ├── persistence/
│   │   └── postgres_repo.go        # Triển khai lưu trữ Postgres GORM (Inject schema 'nutrition')
│   ├── ai/
│   │   └── openai_adapter.go       # Adapter gọi OpenAI API để sinh thực đơn (LLM)
│   └── transport/
│       ├── grpc_handler.go
│       └── http_handler.go
│
└── docs/                           # Tài liệu phân tích nghiệp vụ & kỹ thuật áp dụng riêng của module
    └── README.md
```

---

## 15.3 Quy Tắc Viết Code Cốt Lõi (Uber Go Style Compliant)

Để mã nguồn có tính nhất quán cao, dễ bảo trì và tuân thủ các quy tắc trong [uber-go-style](file:///e:/LEAN/TTTN/.agents/skills/uber-go-style/SKILL.md):

1. **Dependency Injection (Constructor Injection)**:
   * Không sử dụng biến toàn cục (Global Variables) hoặc khởi tạo trực tiếp instance phụ thuộc bằng từ khóa `new()` trong Use Case. 
   * Mọi phụ thuộc (Database connections, AI Clients, Event Publisher) phải được khai báo dạng Port (Interface) ở Domain/Application và được truyền qua Hàm dựng ở `main.go`.
2. **Error Handling**:
   * Không bao giờ bỏ qua lỗi trả về. Luôn luôn kiểm tra `if err != nil`.
   * Sử dụng cơ chế bọc lỗi `fmt.Errorf("context message: %w", err)` để giữ nguyên vết ngăn xếp lỗi (stack trace) khi chuyển lên các tầng phía trên.
3. **Quản lý Goroutines & Concurrency**:
   * Khi khởi chạy các goroutines chạy ngầm (ví dụ: phát sự kiện Event Bus, hoặc nén dữ liệu tọa độ tải lên MongoDB), bắt buộc phải truyền và quản lý thời gian sống qua `context.Context` để tránh rò rỉ bộ nhớ (Goroutine Leak).
4. **Interface Segregation**:
   * Định nghĩa Interface (Port) ở nơi chúng được sử dụng (Consumer - tức là tầng Domain/Application của module đó), không định nghĩa ở nơi chúng được triển khai (Provider - tầng Infrastructure).
