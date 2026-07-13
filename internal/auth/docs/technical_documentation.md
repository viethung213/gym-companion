# Tài Liệu Kỹ Thuật Module Auth (Identity & Authentication)

Dịch vụ Auth (Identity Service) được xây dựng dựa trên kiến trúc sạch, đề cao tính mở rộng, bảo mật và sự tách biệt rõ ràng giữa logic nghiệp vụ (Domain) và các yếu tố kỹ thuật bên ngoài (Infrastructure).

---

## 1. Kiến Trúc Hệ Thống (Hexagonal Architecture)

Dịch vụ tuân thủ mô hình **Hexagonal Architecture (Ports & Adapters)** để đảm bảo domain core không bị ảnh hưởng bởi framework, cơ sở dữ liệu hay các nhà cung cấp dịch vụ bên ngoài:

- **Domain Layer (`internal/auth/domain/`)**:
  - Chứa các Aggregate Root (`User`), các Entity (`JSONWebKey`, `Session`, và `OutboxEvent`).
  - Định nghĩa các cổng lưu trữ và tiện ích mật mã (**Ports**): `UserRepository`, `KeyRepository`, `SessionRepository`, `KeyGenerator` và `OutboxRepository`.
  - Hoàn toàn độc lập, không import bất kỳ thư viện ngoài hay framework mã hóa/cấu trúc dữ liệu nào (không GORM tags, không JSON/DB tags, không logic sinh khóa trực tiếp).
- **Application Layer (`internal/auth/application/`)**:
  - Đóng vai trò điều phối (orchestration), chứa các Command/Query Handler và các interface dịch vụ bên ngoài (`TokenService`, `OAuthService`, và `TransactionManager`).
- **Shared Layer (`internal/shared/database/`)**:
  - Chứa **Database Connection Registry** quản lý tập trung, khởi tạo một lần duy nhất (Singleton, thread-safe, lazy-load) tất cả các connection pools (`*sql.DB`) cho toàn bộ các module trong hệ thống Modular Monolith.
- **Infrastructure Layer (`internal/auth/infrastructure/`)**:
  - Chứa các Adapters cụ thể triển khai công nghệ:
    - **`crypto`**: Triển khai `KeyGenerator` bằng thuật toán sinh khóa RSA-2048 thật.
    - **`persistence`**: Giao tiếp database qua PostgreSQL (triển khai `UserRepository`, `KeyRepository`, `SessionRepository`, `OutboxRepository` và `TransactionManager` trên PostgreSQL).
    - **`jwt`**: Ký và xác thực JWT qua thư viện `golang-jwt` (`TokenService` implementation).
    - **`oauth`**: Gọi OAuth API qua HTTPS (`OAuthService` implementation).
    - **`transport/grpc`**: Cung cấp API giao tiếp qua gRPC.

---

## 2. Các Công Nghệ & Kỹ Thuật Cốt Lõi

### A. Mã Hóa Bất Đối Xứng & Xử Lý Khóa (RS256 - RSA 2048-bit)
- **Cơ chế ký JWT**: Access Token (JWT) được ký bằng thuật toán mã hóa bất đối xứng **RS256** (RSA với SHA-256).
- **Khóa ký**: Server sử dụng Private Key (được lưu an toàn trong database dưới dạng PEM mã hóa) để ký token.
- **Key ID (`kid`)**: Khi sinh Access Token, ID của cặp khóa (`kid` - UUID) được gán trực tiếp vào Header của JWT. Điều này cho phép bên nhận biết chính xác khóa nào được dùng để ký nhằm chọn đúng Public Key để xác thực.

### B. Chuẩn JWKS (JSON Web Key Set)
- **Mục tiêu**: Hỗ trợ API Gateway và các microservice khác tự động xác thực token (stateless verification) mà không cần gọi trực tiếp (gRPC call) tới Identity Service cho mỗi request.
- **Định dạng public key**: Dịch vụ expose endpoint `GET /api/v1/auth/jwks` trả về định dạng JWKS chuẩn hóa, phân tách RSA Public Key thành các thuộc tính:
  - `kty`: Loại khóa (luôn là `"RSA"`).
  - `use`: Mục đích sử dụng (luôn là `"sig"` - signature).
  - `alg`: Thuật toán ký (luôn là `"RS256"`).
  - `kid`: Key ID tương ứng.
  - `n`: Modulus của khóa RSA, được mã hóa **Base64URL không padding** (`base64.RawURLEncoding`).
  - `e`: Exponent của khóa RSA (thường là số `65537`), được chuyển đổi sang dạng mảng byte và mã hóa **Base64URL không padding** (trả về `"AQAB"`).

### C. Cơ Chế Xoay Khóa Ký Trơn Tru (Smooth Key Rotation & Grace Period)
Để thay đổi khóa ký định kỳ (nhằm giảm thiểu rủi ro khi khóa bị lộ) mà không làm gián đoạn trải nghiệm người dùng (bị logout đột ngột):
- **Trạng thái khóa**: Khóa ký có 3 trạng thái:
  1. `active`: Khóa hiện tại đang được dùng để ký các token mới phát hành. Chỉ có duy nhất 1 khóa `active` tại một thời điểm.
  2. `inactive`: Khóa cũ đã bị xoay. Khóa này không dùng để ký mới nữa nhưng **vẫn nằm trong JWKS** để phục vụ xác thực các token cũ chưa hết hạn.
  3. `retired`: Khóa đã hoàn toàn hết hạn và bị xóa khỏi hệ thống.
- **Quy tắc thời gian đệm (Grace Period)**: 
  $$\text{Grace Period} \ge \text{Access Token TTL}$$
  Ví dụ: Access Token có TTL là 15 phút, thời gian đệm Grace Period của khóa inactive là 24 giờ. Khi xoay khóa, khóa cũ đổi sang `inactive` và được giữ lại trong JWKS 24h. Bất kỳ token nào được ký trước thời điểm xoay 1 giây (ví dụ còn hạn 15 phút) vẫn xác thực thành công trong suốt vòng đời của nó. Sau 15 phút, client tự động dùng Refresh Token để lấy Access Token mới (sẽ được ký bằng khóa `active` mới).

### D. Bảo Mật OAuth2 & Chống Tấn Công CSRF
- **State Validation**: Khi khởi tạo luồng login OAuth, Frontend/Backend sử dụng tham số `state` ngẫu nhiên để verify tính toàn vẹn của callback, chống CSRF.
- **Backchannel Exchange**: Backend trao đổi mã Authorization Code lấy Token trực tiếp từ máy chủ Google/Facebook thông qua kết nối HTTPS bảo mật (Backchannel), sử dụng Client Secret được bảo mật tuyệt đối ở phía Server.

### E. Quản Lý Concurrency & Lifecycle
- **Automated Key Rotation Worker**: Chạy nền dưới dạng goroutine để kiểm tra thời hạn khóa mỗi giờ và xoay key.
- **Lifecycle Control**: Worker tuân thủ nghiêm ngặt chuẩn Go, nhận tín hiệu tắt máy thông qua `context.Context` để giải phóng tài nguyên (Ticker) và thoát goroutine sạch sẽ, tránh hiện tượng rò rỉ goroutine (Free Goroutines).

---

## 3. Kiến Trúc Sự Kiện & Quản Lý Transaction

### A. Outbox Pattern & CloudEvents 1.0
Dịch vụ áp dụng mô hình **Transactional Outbox Pattern** để đảm bảo tính nhất quán cuối cùng (eventual consistency) khi giao tiếp hướng sự kiện:
- Khi người dùng mới được tạo qua đăng nhập OAuth lần đầu, thông tin người dùng được lưu vào bảng `auth.users`.
- Trong cùng một transaction SQL, một sự kiện `UserRegistered` (CloudEvent envelope) được chèn vào bảng `auth.outbox`.
- Database cam kết transaction (Commit). Nhờ đó, chắc chắn việc tạo người dùng và sinh sự kiện là một hành động nguyên tử (Atomic).
- Sự kiện được bọc trong một envelope tuân thủ chuẩn **CloudEvents 1.0** (gồm `specversion`, `id`, `source`, `type`, `time`, `datacontenttype`, `data`).
- Theo quy định, các field trong payload `data` phải ở dạng **camelCase** (sử dụng `"google.golang.org/protobuf/encoding/protojson"` để serialize).

### B. Transaction Propagation qua Context
- Để các repository khác nhau (như `UserRepository` và `OutboxRepository`) có thể tham gia vào cùng một transaction SQL mà không vi phạm sự độc lập kiến trúc của tầng Domain:
  - Interface `TransactionManager` (Application Port) định nghĩa cơ chế chạy transaction.
  - `SQLTransactionManager` (Infrastructure Adapter) triển khai việc bắt đầu transaction, gán đối tượng `*sql.Tx` vào `context.Context` qua hàm `WithTx(ctx, tx)` và gọi callback.
  - Các Repository lấy `*sql.Tx` từ context bằng `GetTx(ctx)`. Nếu có, chúng thực thi lệnh SQL trên transaction đó; nếu không, chúng tự chạy độc lập trên db pool.
  - Điều này giải quyết hoàn hảo quy định: *"Các thao tác ghi phối hợp nhiều Aggregate chạy chung 1 transaction với context.Context."*

### C. Quản Lý Kết Nối Database Tập Trung (Database Connection Registry)
- Để cô lập hoàn toàn giữa các module (schema isolation) nhưng vẫn đảm bảo tính quản lý tập trung và hiệu quả tài nguyên:
  - File [db.go](file:///e:/LEAN/TTTN/internal/shared/database/db.go) ở tầng `internal/shared/database` đóng gói một Singleton **`Registry`** an toàn đa luồng (`sync.RWMutex`).
  - Mỗi module sẽ gọi `Registry.GetPool("module_name")` (ví dụ `GetPool("auth")`).
  - Hệ thống sử dụng cơ chế **Lazy Loading** (Khởi tạo theo nhu cầu). Registry sẽ đọc biến môi trường dành riêng cho module đó (ví dụ: `AUTH_DATABASE_URL`), nếu không có sẽ tự động fallback về `DATABASE_URL` chung kèm tham số `search_path=auth` để tự động điều hướng schema.
  - Cơ chế này cho phép các Connection Pools (`*sql.DB`) của các module được khởi tạo một lần duy nhất (Singleton) trong suốt vòng đời của ứng dụng API, đồng thời giúp việc dọn dẹp (Close tất cả các pool khi tắt ứng dụng) thông qua hàm `dbRegistry.CloseAll()` diễn ra an toàn.

---

## 4. Cấu Trúc Bảng Database (PostgreSQL)

Hệ thống lưu trữ dữ liệu xác thực độc lập trên PostgreSQL với các bảng nằm trong schema `auth`:

### Bảng `auth.users`
Lưu trữ thông tin định danh cơ bản của người dùng, liên kết trực tiếp với các ID mạng xã hội:
- `id` (VARCHAR(255), PRIMARY KEY): UUID định danh người dùng.
- `email` (VARCHAR(255), UNIQUE, NOT NULL): Email tài khoản.
- `password_hash` (VARCHAR(255), NULL): Dành cho khả năng mở rộng trong tương lai.
- `google_id` (VARCHAR(255), UNIQUE, NULL): ID tài khoản Google của người dùng.
- `facebook_id` (VARCHAR(255), UNIQUE, NULL): ID tài khoản Facebook của người dùng.
- `full_name` (VARCHAR(255), NULL): Họ tên đầy đủ.
- `phone` (VARCHAR(255), NULL): Số điện thoại.
- `role` (VARCHAR(50), NOT NULL): Quyền hạn người dùng (`user`, `admin`).
- `created_at` (TIMESTAMP, NOT NULL): Thời điểm tạo tài khoản.
- `updated_at` (TIMESTAMP, NOT NULL): Thời điểm cập nhật cuối cùng.

### Bảng `auth.jwk_keys`
Lưu trữ lịch sử và các khóa ký JWT phục vụ JWKS:
- `id` (VARCHAR(255), PRIMARY KEY): Key ID (`kid` - UUID).
- `private_key_pem` (TEXT, NOT NULL): Private key định dạng PEM.
- `public_key_pem` (TEXT, NOT NULL): Public key định dạng PEM.
- `algorithm` (VARCHAR(50), NOT NULL): Thuật toán mã hóa (`RS256`).
- `status` (VARCHAR(50), NOT NULL): Trạng thái khóa (`active`, `inactive`, `retired`).
- `created_at` (TIMESTAMP, NOT NULL): Thời điểm sinh khóa.
- `expires_at` (TIMESTAMP, NOT NULL): Thời điểm hết hạn hiệu lực của khóa.

### Bảng `auth.sessions`
Lưu trữ các Refresh Token đang hoạt động:
- `token` (VARCHAR(255), PRIMARY KEY): Refresh Token ngẫu nhiên (32-bytes hex).
- `user_id` (VARCHAR(255), NOT NULL, REFERENCES auth.users(id)): FK liên kết với bảng `auth.users`.
- `created_at` (TIMESTAMP, NOT NULL): Thời điểm cấp phát.
- `expires_at` (TIMESTAMP, NOT NULL): Thời điểm hết hạn của phiên làm việc.

### Bảng `auth.outbox`
Bảng trung gian lưu trữ các sự kiện cần gửi (Outbox Pattern):
- `id` (UUID, PRIMARY KEY): ID của bản ghi outbox.
- `event_id` (UUID, UNIQUE, NOT NULL): ID của sự kiện (Event ID trong CloudEvent).
- `event_type` (VARCHAR(255), NOT NULL): Tên của sự kiện (`contracts.generic.auth.v1.userRegistered`).
- `payload` (JSONB, NOT NULL): Toàn bộ CloudEvent envelope bao gồm cả payload dữ liệu camelCase.
- `partition_key` (VARCHAR(255), NOT NULL): Giá trị dùng để định tuyến partition (ví dụ: `userID`).
- `created_at` (TIMESTAMP WITH TIME ZONE, NOT NULL): Thời gian tạo bản ghi.

---

## 5. Cơ Chế Mock Để Kiểm Thử
Để đảm bảo các bài viết kiểm thử tự động (Unit Test / Integration Test) có thể chạy ổn định mà không bị phụ thuộc vào mạng internet hoặc API thật của Google/Facebook:
- Trong [provider.go](file:///e:/LEAN/TTTN/internal/auth/infrastructure/oauth/provider.go), nếu tham số `code` gửi lên Endpoint callback bắt đầu bằng tiền tố `"mock_"`, OAuthProvider sẽ tự động bỏ qua việc gửi request HTTPS ra bên ngoài.
- Hệ thống sẽ trả về một Mock Profile có `email` và `id` được trích xuất từ chính chuỗi `code` đó (ví dụ: code `"mock_testuser"` sẽ ánh xạ thành email `"mock_google_testuser@example.com"`).
