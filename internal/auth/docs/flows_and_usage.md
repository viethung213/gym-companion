# Thiết Kế Luồng Nghiệp Vụ & Hướng Dẫn Sử Dụng Module Auth (gRPC-Gateway)

Tài liệu này hướng dẫn chi tiết các luồng chạy nghiệp vụ của Module Auth (Google/Facebook Login, Key Rotation, JWT Validation) và cách tương tác với các API thông qua **gRPC-Gateway**.

---

## 1. Biểu Đồ Luồng Nghiệp Vụ

### A. Luồng Đăng Nhập Google / Facebook (OAuth2 gRPC-Gateway Flow)
Dưới đây là sơ đồ tương tác thực tế giữa Frontend (FE), gRPC-Gateway (API Gateway), Backend gRPC (Auth Module) và OAuth Provider (Google/Facebook):

```mermaid
sequenceDiagram
    autonumber
    actor User as Người dùng
    participant FE as Frontend App
    participant GW as gRPC-Gateway
    participant BE as Backend gRPC
    participant Provider as Google/Facebook API

    User->>FE: Click "Login with Google/Facebook"
    FE->>GW: GET /api/v1/auth/oauth/login?provider=google
    GW->>BE: (gRPC) GetOAuthLoginURL()
    Note over BE: Sinh state ngẫu nhiên & chuẩn bị Login URL
    BE-->>GW: Trả về Login URL
    GW-->>FE: Phản hồi JSON { "login_url": "https://accounts.google.com/..." }
    
    Note over FE: FE tự lưu state vào SessionStorage để chống CSRF
    FE->>User: Chuyển hướng trình duyệt sang trang đăng nhập của Provider (login_url)
    User->>Provider: Nhập thông tin & Đồng ý ủy quyền
    Provider-->>User: Redirect về Frontend URL Callback với code & state
    
    Note over FE: Trình duyệt quay về: /oauth/callback?code=CODE&state=STATE<br/>FE đối chiếu state trong SessionStorage để xác minh CSRF
    
    FE->>GW: POST /api/v1/auth/login/oauth (Payload: provider, code, redirect_uri)
    GW->>BE: (gRPC) LoginWithOAuth()
    
    BE->>Provider: POST /token (Exchange code bằng client_secret qua Backchannel)
    Provider-->>BE: Trả về Access Token / ID Token
    BE->>Provider: GET /userinfo (Lấy profile email, id, name)
    Provider-->>BE: Trả về Profile thông tin người dùng
    
    Note over BE: Tìm/Tạo User trong DB<br/>Ký Access Token bằng active key<br/>Sinh Refresh Token & lưu Session
    
    BE-->>GW: Trả về Access Token, Refresh Token, UserID
    GW-->>FE: Trả về JSON { "access_token": "...", "refresh_token": "...", "user_id": "..." }
    Note over FE: Lưu token vào Storage và hoàn tất đăng nhập
```

---

### B. Luồng Xoay Khóa JWT & Cập Nhật JWKS
Quy trình xoay khóa diễn ra định kỳ (qua worker chạy nền) hoặc kích hoạt thủ công bởi quản trị viên qua gRPC:

```mermaid
sequenceDiagram
    autonumber
    participant Admin as Admin / Worker
    participant BE as Backend (Auth)
    participant DB as PostgreSQL DB
    participant GW as gRPC-Gateway
    participant Client as Các Microservices khác

    Admin->>BE: Trigger Rotate (RotateKeys API hoặc Cron Worker)
    Note over BE: Sinh cặp khóa RSA-2048 mới
    BE->>DB: INSERT khóa mới (status='active')
    BE->>DB: UPDATE khóa active cũ (status='inactive', expires_at = now + 7 days)
    BE->>DB: DELETE các khóa inactive đã hết hạn đệm (expires_at < now)
    BE-->>Admin: Phản hồi thành công
    Note over BE: Access Token mới phát hành sẽ ký bằng key active mới
    
    Note over Client: Định kỳ hoặc khi gặp kid lạ
    Client->>GW: GET /api/v1/auth/jwks
    GW->>BE: (gRPC) GetJWKS()
    BE->>DB: SELECT tất cả keys có status IN ('active', 'inactive')
    DB-->>BE: Trả về danh sách khóa
    BE-->>GW: Trả về các khóa định dạng JWKS
    GW-->>Client: Trả về JSON JWKS chứa danh sách public key
    Note over Client: Client dùng public key tương ứng với kid để verify token
```

---

## 2. Hướng Dẫn Tích Hợp & Cấu Hình

### A. Thiết Lập Biến Môi Trường (Environment Variables)
Để module Auth hoạt động đầy đủ, cần thiết lập các biến môi trường sau trong file `.env` ở thư mục gốc:

```env
# Database Connection
DATABASE_URL=postgres://postgres:postgres@localhost:5432/fitai?sslmode=disable

# Cổng dịch vụ
APP_PORT=8080
GRPC_PORT=9090

# Cấu hình Google OAuth
GOOGLE_CLIENT_ID=your-google-client-id.apps.googleusercontent.com
GOOGLE_CLIENT_SECRET=your-google-client-secret
GOOGLE_REDIRECT_URI=http://localhost:3000/oauth/callback

# Cấu hình Facebook OAuth
FACEBOOK_CLIENT_ID=your-facebook-client-id
FACEBOOK_CLIENT_SECRET=your-facebook-client-secret
FACEBOOK_REDIRECT_URI=http://localhost:3000/oauth/callback
```

---

### B. Sử Dụng API HTTP (Exposed via gRPC-Gateway)

#### 1. Lấy URL đăng nhập OAuth
- **Endpoint**: `GET /api/v1/auth/oauth/login`
- **Query Parameters**:
  - `provider`: `"google"` hoặc `"facebook"` (Bắt buộc).
- **Phản hồi (HTTP 200)**:
  ```json
  {
    "login_url": "https://accounts.google.com/o/oauth2/v2/auth?client_id=..."
  }
  ```
- **Hành vi**: Frontend sử dụng URL này để chuyển hướng trình duyệt của người dùng.

#### 2. Đổi mã code lấy Token (OAuth Login)
- **Endpoint**: `POST /api/v1/auth/login/oauth`
- **Payload (JSON)**:
  ```json
  {
    "provider": "google",
    "code": "4/0AdQt8...",
    "redirect_uri": "http://localhost:3000/oauth/callback"
  }
  ```
- **Phản hồi (HTTP 200)**:
  ```json
  {
    "access_token": "eyJhbGciOiJSUzI1NiIs...",
    "refresh_token": "a1b2c3d4...",
    "user_id": "9b1deb4d-3b7d-4bad-9bdd-2b0d7b3dcb6d"
  }
  ```

#### 3. Lấy Danh Sách Public Keys (JWKS JSON API)
- **Endpoint**: `GET /api/v1/auth/jwks`
- **Phản hồi (HTTP 200)**:
  ```json
  {
    "keys": [
      {
        "kty": "RSA",
        "use": "sig",
        "alg": "RS256",
        "kid": "4ee282ef-a5b6-455b-80df-4d56d2baeb1f",
        "n": "u1W...[Modulus Base64URL]...Qw",
        "e": "AQAB"
      }
    ]
  }
  ```

---

### C. Sử Dụng API gRPC (Auth Service Server)
Dành cho giao tiếp nội bộ giữa các microservices:

#### 1. Xác thực Access Token (`ValidateToken`)
- **RPC**: `ValidateToken (ValidateTokenRequest) returns (ValidateTokenResponse)`
- **Request**: `{ "token": "access-token-string" }`
- **Response**: `{ "is_valid": true, "user_id": "user-uuid", "roles": ["user"] }`

#### 2. Làm mới Access Token (`RefreshToken`)
- **RPC**: `RefreshToken (RefreshTokenRequest) returns (RefreshTokenResponse)`
- **Request**: `{ "refresh_token": "refresh-token-string" }`
- **Response**: `{ "access_token": "new-access-token", "refresh_token": "same-refresh-token" }`

#### 3. Xoay khóa thủ công (`RotateKeys`)
- **RPC**: `RotateKeys (RotateKeysRequest) returns (RotateKeysResponse)`
- **Hành vi**: Chỉ gọi bởi admin. Sinh ngay lập tức khóa ký mới, chuyển khóa cũ sang `inactive`.

---

## 3. Hướng Dẫn Chạy Thử Nghiệm Cục Bộ (Mock OAuth Flow)

Do việc chạy thử nghiệm OAuth2 thật đòi hỏi Client ID/Secret thực tế, bạn có thể kiểm thử luồng đăng nhập cục bộ bằng cách gửi mã `code` giả lập bắt đầu bằng tiền tố `"mock_"`.

### Các Bước Test:
1. Đảm bảo server đang chạy: `go run cmd/api/main.go`.
2. Dùng công cụ gọi API (như Postman hoặc `curl`) để gửi yêu cầu login giả lập:
   ```bash
   curl -X POST http://localhost:8080/api/v1/auth/login/oauth \
     -H "Content-Type: application/json" \
     -d '{"provider": "google", "code": "mock_johndoe123", "redirect_uri": "http://localhost:3000/oauth/callback"}'
   ```
3. Server sẽ trả về JSON chứa access token, refresh token và mock userID:
   ```json
   {
     "access_token": "eyJhbGciOiJSUzI1NiIs...",
     "refresh_token": "mock-refresh-token",
     "user_id": "mock_id_google_johndoe123"
   }
   ```
