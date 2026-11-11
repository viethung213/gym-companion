# Kiến Trúc Giao Tiếp API Gateway: Chuẩn Hóa 100% ConnectRPC & gRPC-Web

## 1. Quyết Định Kiến Trúc (Architectural Decision Record - ADR)

### Bối cảnh:
Trong các file `.proto` của hệ thống, các RPC method được khai báo kèm annotation `google.api.http`, ví dụ:
```protobuf
rpc SearchExercises(SearchExercisesRequest) returns (SearchExercisesResponse) {
  option (google.api.http) = {
    get: "/api/v1/exercises"
  };
}
```
Tuy nhiên, toàn bộ backend microservices của hệ thống Gym Companion được triển khai dựa trên **ConnectRPC** (`connectrpc.com/connect/v2`).

### Quyết định:
Hệ thống chính thức chuẩn hóa theo **Hướng 1 (Pure ConnectRPC / gRPC-Web)**:
1. **Giao thức duy nhất**: Mọi luồng giao tiếp giữa Client (Web/Mobile) qua Gateway vào Backend Microservices đều sử dụng giao thức **ConnectRPC** qua các đường dẫn canonical chuẩn:
   ```
   POST /contracts.<domain>.<service>.<version>.service.<ServiceName>/<MethodName>
   ```
   *(Ví dụ: `/contracts.supporting.exercise.v1.service.ExerciseService/SearchExercises`)*

2. **Vai trò của `google.api.http` trong `.proto`**:
   - Được giữ lại **duy nhất với mục đích xuất tài liệu OpenAPI / Swagger** (`/docs/swagger/contracts/`) để phục vụ việc tra cứu đặc tả schema và giao diện trực quan cho đội ngũ phát triển.
   - **Không** sử dụng làm đường dẫn định tuyến thực tế trên Gateway.

3. **Làm sạch cấu hình Gateway (`gateway_routes.json`)**:
   - Loại bỏ toàn bộ các prefix thừa dạng `/api/v1/...`.
   - Các prefix định tuyến chỉ chứa namespace package chuẩn ConnectRPC (ví dụ `/contracts.core.coaching.`, `/contracts.supporting.exercise.`).

---

## 2. Vì Sao ConnectRPC Vượt Trội Hơn Mô Hình REST Transcoding?

| Tiêu chí | Mô hình REST Transcoding (grpc-gateway truyền thống) | Mô hình Pure ConnectRPC (Kiến trúc Gym Companion) |
| :--- | :--- | :--- |
| **Bản chất giao thức** | Cần 1 proxy tầng giữa dịch từ HTTP/JSON sang gRPC Protobuf binary | **Native HTTP/1.1 & HTTP/2**: ConnectRPC hỗ trợ cả JSON payload và binary Protobuf out-of-the-box |
| **Độ trễ & Overhead** | Tốn CPU giải mã và mã hóa lại 2 lần (Client $\rightarrow$ JSON $\rightarrow$ Binary $\rightarrow$ Backend) | **Zero-Transcoding Overhead**: Gateway hoạt động như 1 L7 Reverse Proxy siêu tốc, forward trực tiếp stream h2c |
| **Streaming & AI LLM** | Rất phức tạp khi làm Server-Sent Events (SSE) hoặc gRPC bidirectional streaming | **Hỗ trợ Streaming tự nhiên**: Server streaming cho AI Coach và Pose Detection mượt mà (`FlushInterval: -1`) |
| **Độ phức tạp code** | Cần sinh thêm mã `*.pb.gw.go` và đăng ký multiplexer phức tạp | Mã nguồn cực kỳ gọn gàng, giảm thiểu 50% code boilerplate sinh tự động |

---

## 3. Định Tuyến Tại Gateway

Cấu hình định tuyến tại `configs/gateway_routes.json`:

```json
{
  "allowed_hosts": [
    "api.gymcompanion.com",
    "localhost",
    "127.0.0.1"
  ],
  "services": [
    {
      "id": "ai-notification-service",
      "prefixes": [
        "/contracts.core.coaching.",
        "/contracts.core.nutrition.",
        "/contracts.generic.notification."
      ],
      "upstreams": [
        "http://localhost:8081"
      ]
    },
    {
      "id": "core-fitness-service",
      "prefixes": [
        "/contracts.generic.auth.",
        "/contracts.core.workout_execution.",
        "/contracts.core.solo_match.",
        "/contracts.supporting.exercise.",
        "/contracts.supporting.gamification.",
        "/contracts.supporting.profile.",
        "/contracts.supporting.social.",
        "/contracts.generic.audio."
      ],
      "upstreams": [
        "http://localhost:8080"
      ]
    }
  ]
}
```
Mọi request từ Frontend Connect Client sẽ gửi trực tiếp header `Connect-Protocol-Version: 1` và `Content-Type: application/json` hoặc `application/proto`, Gateway nhận diện prefix và chuyển tiếp tới service tương ứng mà không cần bất kỳ bộ chuyển đổi nào.

