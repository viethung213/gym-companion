# Kế Hoạch Nâng Cấp & Hiện Trạng Custom API Gateway Chuẩn Enterprise

Tài liệu này tổng hợp hiện trạng thực thi, đánh giá theo **7 tiêu chuẩn vàng của API Gateway cấp doanh nghiệp (Enterprise Grade)** và định hướng các hạng mục mở rộng tiếp theo cho **Custom Go API Gateway** của dự án Gym-Companion.

---

## 1. Bảng Đánh Giá Hiện Trạng 7 Tiêu Chuẩn Doanh Nghiệp

| Tiêu chuẩn | Thành phần đã hoàn thành `[x]` | Thành phần chưa hoàn thành / Backlog `[ ]` | Trạng thái |
| :--- | :--- | :--- | :---: |
| **1. Core Proxy** | - HTTP/1.1 & HTTP/2 Cleartext (`h2c`) cho gRPC & ConnectRPC.<br>- Longest Prefix Trie Router.<br>- Reverse Proxy Pool (`Keep-Alive`, MaxIdleConns=100).<br>- Zero-Buffering Streaming (`FlushInterval: -1`) cho AI LLM.<br>- **Graceful Shutdown** (`SIGINT`/`SIGTERM`, 10s drain window).<br>- Cấu hình Server Timeouts (`ReadHeaderTimeout`, `IdleTimeout`). | - Dynamic Circuit Breaker theo ngưỡng lỗi tỷ lệ % (hiện tại dựa vào xử lý 502/503 trực tiếp). | **95%**<br>🟢 Sẵn sàng Production |
| **2. Config Engine** | - Đọc JSON định tuyến (`configs/gateway_routes.json` & `gateway_routes.docker.json`).<br>- In-memory File Watcher tự động Hot-Reload không downtime (`sync.RWMutex`).<br>- Fallback Config an toàn (giữ nguyên config cũ nếu file JSON bị lỗi cú pháp). | - Hỗ trợ nạp config từ remote K/V store (Consul / Etcd) - *Không bắt buộc cho đồ án*. | **100%**<br>🟢 Hoàn thành |
| **3. Reliability & Routing** | - Load Balancing Round-Robin đa luồng an toàn (`sync/atomic`).<br>- Dynamic Upstream Add/Remove trong RAM.<br>- Cơ chế bảo vệ Zero-Upstream trả về 503 JSON chuẩn hóa.<br>- Dedicated Admin Health Check (`GET /healthz` trên Control Plane). | - **Active Health Check Ping**: Goroutine chạy ngầm định kỳ ping `/health` từng upstream để tự động gạch tên node chết và khôi phục khi sống lại. | **80%**<br>🟡 Đang chạy tốt |
| **4. Security (Zero-Trust)** | - **Stateless 100% (Zero-DB)**: Tự động khám phá Auth Service upstream để lấy JWKS.<br>- Token Translation: Public JWT (ES256/RS256) $\rightarrow$ Short-lived HMAC-SHA256 (`X-Internal-Token`, TTL=1m).<br>- Protobuf SSOT Auth Matcher qua ConnectRPC Descriptor Registry.<br>- **Host & Authority Filter**: Chặn DNS Rebinding, chặn truy cập trực tiếp qua IP lạ, whitelist allowed hosts.<br>- **Enforce TLS Option**: Từ chối request HTTP không an toàn khi bật flag.<br>- **Global IP Rate Limiter**: Giới hạn tần suất request theo IP client với thuật toán Token Bucket. | - WAF rules nâng cao (chặn SQLi, XSS payload tại Gateway) - *Hiện tại Backend validator đã đảm nhận*. | **95%**<br>🟢 Rất vững chắc |
| **5. Observability** | - Phân tách biệt lập 2 Port: **Data Plane (8000)** & **Control Plane (8001)**.<br>- Bảo vệ Control Plane bằng `X-Admin-Key` (Timing attack safe).<br>- Distributed Tracing: Tự động cấp/kế thừa `X-Request-ID`.<br>- **Ring Buffer Trace Store**: Lưu vết 1.000+ request gần nhất trong RAM, hỗ trợ lọc theo path, client IP, mã lỗi, duration.<br>- Endpoint `/metrics` chuẩn Prometheus Format.<br>- Admin REST APIs: `/admin/overview`, `/admin/upstreams`, `/admin/traces`, `/admin/config`. | - **Web Dashboard UI (`/dashboard`)**: Trang giao diện đồ họa trực quan (HTML nhúng) để quan sát thời gian thực phục vụ báo cáo bảo vệ đồ án. | **85%**<br>🟢 Đã có API nền tảng |
| **6. Production Operations & CI/CD** | - Unit Tests & Integration E2E Tests chuẩn Go đạt **PASS 100%**.<br>- Multi-stage Dockerfile độc lập ([infra/gateway/Dockerfile](file:///e:/LEAN/DATN/gym-companion/infra/gateway/Dockerfile)), kích thước siêu nhẹ (~9.78 MB).<br>- Tách biệt hoàn toàn khỏi mã nguồn Backend Monolith.<br>- Target `tester` cho phép chạy toàn bộ test độc lập ngay trong Docker.<br>- GitHub Actions Workflow ([.github/workflows/publish-gateway-image.yml](file:///e:/LEAN/DATN/gym-companion/.github/workflows/publish-gateway-image.yml)) tự động test, build và publish lên **GHCR** (`ghcr.io`). | - Kịch bản kiểm thử tải thực tế (Load test benchmark với `k6` / `vegeta`). | **90%**<br>🟢 Chuẩn DevOps |
| **7. Advanced Service Discovery** | - Kiến trúc Control Plane sẵn sàng nhận chỉ thị nạp/xóa route động trong runtime. | - **Self-Registration Service Discovery**: API cho phép các service replica khi khởi động tự gọi đăng ký IP/Port và gửi Heartbeat định kỳ 5s (`/admin/discovery/register`). | **50%**<br>⚪ Đang cấu hình tĩnh |

---

## 2. Chi Tiết Các Hạng Mục Đã Thực Hiện

### [x] 1. Tách biệt kiến trúc độc lập (Independent Gateway)
- **Vị trí**: [internal/gateway/](file:///e:/LEAN/DATN/gym-companion/internal/gateway/), [cmd/gateway/](file:///e:/LEAN/DATN/gym-companion/cmd/gateway/), [infra/gateway/](file:///e:/LEAN/DATN/gym-companion/infra/gateway/).
- **Đặc điểm**:
  - Không import database driver, không kết nối GORM/PostgreSQL.
  - Tự tải JWKS từ Auth Service qua HTTP/2 h2c và cache in-memory.
  - Image Docker chỉ copy đúng 4 package tối thiểu: `cmd/gateway`, `internal/gateway`, `internal/gen/go/contracts`, `internal/shared/internaltoken`.

### [x] 2. Phân tách Data Plane (:8000) & Control Plane (:8001)
- **Data Plane (Cổng 8000)**: Chỉ phục vụ routing, token translation, streaming proxy. Không để lộ endpoint quản trị.
- **Control Plane (Cổng 8001)**: Độc lập hoàn toàn, yêu cầu header `X-Admin-Key`:
  - `GET /healthz`: Kiểm tra trạng thái sống của Gateway.
  - `GET /admin/overview`: Thống kê RPS, tổng request, mã lỗi 2xx/4xx/5xx, số upstreams.
  - `GET /admin/upstreams`: Danh sách upstreams chi tiết kèm tải điều phối.
  - `GET /admin/traces`: Xem lại hành trình chi tiết của các request gần nhất.
  - `GET /metrics`: Cung cấp số liệu cho Prometheus / Grafana.

### [x] 3. Bảo vệ an ninh & Chống lạm dụng
- **Token Translation**: Chuyển đổi JWT người dùng sang `X-Internal-Token` ký bằng HMAC-SHA256 để bảo vệ Backend Monolith theo chuẩn Zero-Trust.
- **Host Filter**: Chặn truy cập từ các domain lạ hoặc IP trực tiếp chống tấn công DNS Rebinding.
- **IP Rate Limiter**: Giới hạn số request/phút trên từng IP client để chống brute-force và DDoS.

### [x] 4. Chuyển hóa E2E Tests & Dockerization
- Xóa bỏ script CLI tạm bợ `cmd/test_gateway_e2e`.
- Toàn bộ 6 bài kiểm thử E2E được chuyển thành file test Go chính quy: [internal/gateway/tests/e2e/gateway_e2e_test.go](file:///e:/LEAN/DATN/gym-companion/internal/gateway/tests/e2e/gateway_e2e_test.go).
- Tích hợp stage `tester` trong Dockerfile: có thể kiểm thử toàn diện trên bất kỳ môi trường nào bằng lệnh:
  ```bash
  docker build --target tester -t gateway-test -f infra/gateway/Dockerfile . && docker run --rm gateway-test
  ```

---

## 3. Các Hạng Mục Chưa Thực Hiện (Backlog & Lộ Trình Nâng Cấp)

### [ ] Hạng mục 1: Web Dashboard UI Trực Quan (`GET /dashboard`)
- **Mục tiêu**: Phục vụ buổi báo cáo Đồ Án Tốt Nghiệp có màn hình dashboard trực quan, sinh động.
- **Mô tả**:
  - Hiện tại Control Plane đã có sẵn các API dữ liệu: `/admin/overview`, `/admin/upstreams`, `/admin/traces`.
  - Cần bổ sung file HTML/CSS/JS (Dark Mode) nhúng trực tiếp bằng `embed.FS` trong Go binary.
  - Hiển thị biểu đồ realtime: RPS, thời gian phản hồi (latency), danh sách upstream đang gánh tải, các vết request lỗi (5xx).

### [ ] Hạng mục 2: Active Health Checker Ping Định Kỳ
- **Mục tiêu**: Tự động phát hiện khi một service con bị sập để không chuyển request vào đó.
- **Mô tả**:
  - Goroutine chạy nền định kỳ (mỗi 5s - 10s) gửi `GET /healthz` đến từng URL upstream trong bộ cân bằng tải.
  - Nếu fail liên tiếp 3 lần: Đánh dấu `Unhealthy` và tạm thời ngắt khỏi Round-Robin pool.
  - Khi node phản hồi 200 OK trở lại: Tự động tái kích hoạt vào danh sách phân phối.

### [ ] Hạng mục 3: Dynamic Self-Registration & Heartbeat (Service Discovery)
- **Mục tiêu**: Hỗ trợ mở rộng cụm microservice không cần sửa file JSON.
- **Mô tả**:
  - Thêm endpoint `POST /admin/discovery/register` nhận `{ "service_id": "...", "upstream_url": "..." }`.
  - Thêm cơ chế TTL eviction: Nếu replica không gửi heartbeat sau 15 giây, Gateway tự xóa node khỏi RAM.

### [ ] Hạng mục 4: Kịch bản kiểm thử tải (Benchmark Load Testing)
- **Mục tiêu**: Đo đạc độ trễ và khả năng chịu tải của Gateway trước khi bảo vệ đồ án.
- **Mô tả**:
  - Viết file kịch bản `k6` hoặc `vegeta` bắn tải 5.000 - 10.000 req/s vào Data Plane.
  - Đo lường overhead độ trễ do Gateway thêm vào (mục tiêu: < 2ms latency overhead).

---

## 4. Cấu Trúc Thư Mục Thực Tế Hiện Tại

```text
configs/
├── gateway_routes.docker.json           # Dùng cho chạy Docker Network / Test (upstream: http://app:8080)
└── gateway_routes.json                  # Dùng cho môi trường Production / Host (upstream: http://localhost:8080)

infra/gateway/
├── Dockerfile                           # Multi-stage: builder -> tester -> prod (Alpine <10MB)
└── docker-compose.yml                   # Cấu hình container Gateway chạy độc lập

internal/gateway/
├── admin/                               # Control Plane (:8001) - Overview, Upstreams, Traces, Prometheus
├── balancer/                            # Round-Robin Load Balancer an toàn đa luồng
├── config/                              # Đọc cấu hình JSON & Hot-reload Watcher
├── docs/                                # Tài liệu Gateway (ARCHITECTURE.md, PLAN.md)
├── middleware/                          # JWT Validator, Host Filter, Rate Limiter, Tracing, TLS
├── proxy/                               # Reverse Proxy H2C & Streaming
├── telemetry/                           # Metrics Collector & Ring Buffer Trace Store
├── tests/e2e/                           # 6 bài kiểm thử tích hợp E2E chuẩn Go
├── gateway.go                           # Data Plane (:8000) Orchestrator
└── ...

.github/workflows/
└── publish-gateway-image.yml            # CI/CD: Tự động chạy test -> build -> publish GHCR
```

---

## 5. Tiến Độ Thực Thi (Checklist Tổng Kết)

- [x] **Milestone 1**: Core Data Plane Proxy với HTTP/2 Cleartext (`h2c`) và streaming SSE/LLM.
- [x] **Milestone 2**: In-Memory Route Watcher tự động Hot-Reload không downtime kèm fallback an toàn.
- [x] **Milestone 3**: Cơ chế bảo mật Zero-Trust Token Translation và Protobuf SSOT Auth Matcher.
- [x] **Milestone 4**: Bổ sung Host/Authority Filtering và Global IP Rate Limiter bảo vệ cửa ngõ.
- [x] **Milestone 5**: Tách biệt Control Plane (:8001), tích hợp Ring Buffer Trace Store và Prometheus Metrics.
- [x] **Milestone 6**: Tích hợp Graceful Shutdown bắt tín hiệu OS an toàn.
- [x] **Milestone 7**: Tách Dockerfile độc lập, tối ưu build context (<20kB), thu nhỏ image (<10MB).
- [x] **Milestone 8**: Chuyển hóa toàn bộ kịch bản E2E vào `internal/gateway/tests/e2e/`, xóa sạch mã rác.
- [x] **Milestone 9**: Thiết lập CI/CD GitHub Actions đẩy image lên GHCR (`ghcr.io`).
- [ ] **Milestone 10 (Backlog)**: Xây dựng giao diện đồ họa Web Dashboard (`/dashboard`) nhúng trực tiếp.
- [ ] **Milestone 11 (Backlog)**: Bổ sung Active Health Check Ping chạy ngầm cho upstreams.
- [ ] **Milestone 12 (Backlog)**: Triển khai kịch bản kiểm thử tải Benchmark (`k6`).
