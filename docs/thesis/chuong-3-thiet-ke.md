# CHƯƠNG 3. PHÂN TÍCH VÀ THIẾT KẾ HỆ THỐNG

Chương này trình bày cách FitAI được thiết kế để giải quyết bài toán đã nêu ở Chương 1, không nhắc lại lý thuyết đã có ở Chương 2. Nội dung được tổ chức theo **hành trình người dùng** — từ khi đăng ký, nhận lộ trình, tập luyện, tới sau tập và dinh dưỡng — thay vì cắt theo thành phần kỹ thuật đơn thuần. Cách bố cục này giúp người đọc nhìn thấy giá trị sản phẩm và mỗi thành phần kỹ thuật xuất hiện đúng nơi nó phục vụ.

Ngôn ngữ nghiệp vụ (Ubiquitous Language) trong chương này bám sát tài liệu DDD Tactical của dự án; các mã yêu cầu (`FR-XX-YY`), mã quy tắc nghiệp vụ (`BR-XX-YY`), và mã use case (`UC-XX.Y`) được dùng thống nhất để tham chiếu chéo.

---

## 3.1. Phân tích yêu cầu

### 3.1.1. Yêu cầu chức năng

Yêu cầu chức năng được nhóm theo bảy nhóm module. Chi tiết use case của mỗi nhóm sẽ được trình bày trong các mục Journey (3.4–3.8).

**Quản lý người dùng (UM)** — thuộc `User Profile Context`
- FR-UM-01 Đăng ký, đăng nhập qua Email/SĐT (OTP) và OAuth (Google, Apple, Facebook).
- FR-UM-02 Khai báo hồ sơ sức khỏe: tuổi, giới tính, chiều cao, cân nặng, mục tiêu (tăng cơ / giảm mỡ), chấn thương, bệnh lý.
- FR-UM-03 Chọn khung giờ tập cố định (có thể để trống, cập nhật sau).
- FR-UM-04 Nhắc lịch tự động qua push notification.

**AI Coach (AC)** — thuộc `Coaching & Planning Context`
- FR-AC-01 Sinh `WorkoutRoadmap` 4 tuần và `WeeklySchedule` đầu tiên khi hồ sơ hoàn thiện ≥ 80% (BR-UM-01).
- FR-AC-02 Sinh `DailyWorkoutPlan` (Just-In-Time) trong ngày tập, kèm check-in ngắn về sức khỏe và dồn/bù buổi bỏ tập.
- FR-AC-03 Sinh `WeeklySchedule` các tuần tiếp theo với `OverloadValidator` (BR-AC-02).
- FR-AC-04 Đánh giá `CompletionRate` cuối chu kỳ 4 tuần và điều chỉnh lộ trình (BR-AC-04).
- FR-AC-05 Phát hiện tín hiệu B1–B4 giữa chu kỳ (BR-AC-05 → BR-AC-08).
- FR-AC-06 Sinh lời giải thích (adjustment explanation, post-session report) theo `CoachPersonality`.

**Camera & Phân tích tư thế (CC)** — thuộc `Workout Execution & Motion Context`
- FR-CC-01 Nhận diện 17 điểm khớp (COCO standard) qua camera phía client.
- FR-CC-02 Tính góc khớp và ROM% thời gian thực.
- FR-CC-03 Đếm rep hợp lệ khi ROM ≥ 70% (BR-CC-01).
- FR-CC-04 Phát hiện lỗi tư thế và phản hồi âm thanh có phân mức (Severity 1/2) dưới 150 ms từ khi lỗi được nhận diện.
- FR-CC-05 Chấm `FormScore` cho mỗi rep và cả buổi tập.

**Ghi nhận buổi tập (WL)** — thuộc `Workout Execution & Motion Context`
- FR-WL-01 Ghi `WorkoutSetLog` tự động cho bài có AI Camera.
- FR-WL-02 Ghi thủ công cho bài phi AI (timer + hướng dẫn), FormScore = N/A (BR-WL-03).
- FR-WL-03 Áp `TrainingLoadGuard` khi kết thúc buổi (BR-WL-01, BR-WL-02).
- FR-WL-04 Sinh `SessionSummary` và cập nhật `PersonalRecord` (Epley Formula, cờ `Unverified` nếu bài phi AI).

**Dinh dưỡng (NU)** — thuộc `Nutrition Context`
- FR-NU-01 Tính `CalorieAllocation` theo Mifflin-St Jeor (BR-NU-01: tối thiểu 1200 kcal/ngày).
- FR-NU-02 Sinh `DailyMealOption` (3 bữa chính + 1 bữa phụ) theo `BudgetTier` (BR-NU-03).
- FR-NU-03 Áp `LockoutRegistry` — khóa protein 7 ngày, carb 5 ngày, chủ đề món 3 ngày (BR-NU-02).
- FR-NU-04 Ghi `MealLog` (tìm kiếm hoặc quét mã vạch), cập nhật `LockoutRegistry`.

**Theo dõi tiến độ (PT)** — thuộc `User Profile Context` + `Workout Execution & Motion Context`
- FR-PT-01 Ghi nhận `BodyMetricsHistory` (cân nặng, %mỡ, số đo, ảnh tiến trình).
- FR-PT-02 Biểu đồ xu hướng volume, 1RM, FormScore trung bình.

**Danh mục (CAT)** — thuộc `Exercise Context` và `Nutrition Context`
- FR-CAT-01 Vòng đời `Exercise`: `Draft` → `PendingApproval` → `Active` → `Archived` (BR-CAT-01).
- FR-CAT-02 Vòng đời `FoodItem` tương tự (BR-NU-04).

### 3.1.2. Yêu cầu phi chức năng

| Nhóm | Yêu cầu | Vì sao |
|---|---|---|
| **Độ trễ (Latency)** | Phản hồi âm thanh sửa tư thế < 150 ms từ khi nhận diện lỗi | Nếu chậm hơn, rep đã kết thúc, mất tính hữu ích |
| **Real-time** | Pose estimation ≥ 24–30 fps trên smartphone tầm trung | Ngưỡng tối thiểu để đếm rep chính xác |
| **Quyền riêng tư** | Video và tọa độ khớp thô không rời khỏi thiết bị | Cam kết privacy-first; giảm bề mặt tấn công; chỉ upload logs định danh khóa và summary |
| **Sẵn sàng** | Client vẫn dùng được (timer + log tay) khi Coaching Service lỗi | User tự tập bình thường, không phụ thuộc AI |
| **Bảo mật** | JWT ngắn hạn (RS256) + JWKS rotation, refresh token lưu PostgreSQL với session-per-device | Bảo vệ dữ liệu sức khỏe, hỗ trợ đăng xuất thiết bị cụ thể |
| **Khả năng mở rộng** | Modular monolith, mỗi BC là một schema PostgreSQL độc lập; sẵn sàng tách microservices khi cần | Nguyên mẫu bắt đầu nhỏ, không đóng cửa cho tương lai |
| **Khả năng bảo trì** | Hexagonal Architecture; contract-first API (Protobuf); Ubiquitous Language nhất quán | Một sinh viên bảo trì; giảm sai lỗi khi thêm tính năng |
| **Đáp ứng đa thiết bị** | Web (React) và Android; nội dung UI đồng nhất | User tập tại nhà dùng smartphone + laptop |
| **Dễ dùng** | Onboarding tối giản, các trường không bắt buộc thu thập dần qua `ChatbotContext` | Người mới dễ nản nếu form dài |
| **Khả năng mở rộng tính năng** | Thư viện `Exercise` và `FoodItem` có vòng đời phê duyệt, không đụng vào code lõi khi thêm mới | Nguyên mẫu sẽ mở rộng dần |

---

## 3.2. Kiến trúc tổng thể

### 3.2.1. Bốn tầng kiến trúc

```
   ┌──────────────────────────────────────────────────────────────┐
   │ Layer 1 — Clients                                            │
   │   Mobile App · Web Client · AI Edge Camera (on-device model) │
   └───────────────────────────┬──────────────────────────────────┘
                               │
                               ▼
   ┌──────────────────────────────────────────────────────────────┐
   │ Layer 2 — Gateway                                            │
   │   HTTP API Gateway (auth middleware, routing, rate limit)    │
   └───────────────────────────┬──────────────────────────────────┘
                               │
                               ▼
   ┌──────────────────────────────────────────────────────────────┐
   │ Layer 3 — Backend Services (Golang Modular Monolith)         │
   │                                                              │
   │   ┌────────────────────────────────────────────────────┐     │
   │   │ Core BC                                            │     │
   │   │   Coaching & Planning · Workout Execution & Motion │     │
   │   │   Nutrition                                        │     │
   │   ├────────────────────────────────────────────────────┤     │
   │   │ Supporting BC                                      │     │
   │   │   User Profile · Exercise                          │     │
   │   ├────────────────────────────────────────────────────┤     │
   │   │ Generic Services                                   │     │
   │   │   Auth · Notification · Audio                      │     │
   │   └────────────────────────────────────────────────────┘     │
   └───────────────────────────┬──────────────────────────────────┘
                               │
                               ▼
   ┌──────────────────────────────────────────────────────────────┐
   │ Layer 4 — Infrastructure                                     │
   │   PostgreSQL (schema-per-module) · Blob Storage · CDN ·      │
   │   OAuth Providers · Push Notification                        │
   └──────────────────────────────────────────────────────────────┘
```

### 3.2.2. Bốn nguyên tắc thiết kế

1. **Edge-first cho AI real-time**: video và tọa độ khớp thô không rời client. Backend chỉ nhận `SessionSummary` và batch error logs (đã trừu tượng thành mã lỗi + góc khớp, không phải ảnh).
2. **Contract-first**: mọi API được định nghĩa trong `proto/contracts/` (Protobuf), sinh mã cho Go server và client TypeScript, sinh Swagger cho tài liệu. Contract là nguồn sự thật, không phải code hoặc DB.
3. **Modular Monolith + Hexagonal per module**: một binary duy nhất khi triển khai, nhưng mã nguồn tách theo `internal/<module>/{domain, application, infrastructure}` theo Ports & Adapters. Sẵn sàng bóc tách thành microservices khi cần.
4. **Deterministic first, AI second**: mọi phép tính có công thức (Mifflin-St Jeor, Epley, ROM, TrainingLoadGuard, CompletionRate, LockoutRegistry) do rule engine Go xử lý. AI (LLM) chỉ tham gia ở phần khó số hóa: chọn/sắp xếp bài, trích xuất NLP, sinh lời giải thích.

### 3.2.3. Hexagonal per module

```
                    ┌──────────────────────────┐
                    │      Application         │
                    │  (Command/Query Handlers,│
                    │   Use Cases, Ports)      │
                    └─────┬───────────────┬────┘
                          │               │
                driver ports    driven ports
                (in)              (out)
                          │               │
   ┌──────────────────────▼───┐   ┌───────▼───────────────────┐
   │  Domain (pure Go)        │   │  Infrastructure           │
   │  - Aggregates            │   │  - Postgres repositories  │
   │  - Value Objects         │   │  - HTTP/gRPC handlers     │
   │  - Domain Services       │   │  - Event bus adapter      │
   │  - Domain Events         │   │  - External API clients   │
   └──────────────────────────┘   └───────────────────────────┘
```

- **Domain**: thuần Go, không import bất kỳ thư viện ngoài. Chứa Aggregate Roots, Value Objects, Domain Services (`AdaptiveCoachEngine`, `OverloadValidator`, `TrainingLoadGuard`), Domain Events.
- **Application**: nhận Command/Query, gọi Domain, publish event. Ports được định nghĩa ở đây dưới dạng interface (ví dụ `WorkoutRoadmapRepository`, `EventPublisher`, `LLMPort`).
- **Infrastructure**: cài đặt ports — PostgreSQL adapter, gRPC handler, EventBus adapter, Gemini adapter.

Lợi ích: có thể test Application + Domain hoàn toàn không cần DB thật; thay đổi backing store (Postgres → khác) hoặc LLM (Gemini → khác) chỉ đụng vào adapter, không đụng logic nghiệp vụ.

### 3.2.4. Trade-off của kiến trúc tổng thể

| Quyết định | Chọn | Lý do |
|---|---|---|
| Edge vs Cloud AI cho pose | Edge | Latency < 150 ms không đạt qua mạng; privacy-first; hoạt động offline |
| Modular Monolith vs Microservices | Modular Monolith | Phạm vi khóa luận 1 sinh viên; Hexagonal đảm bảo ranh giới; có thể tách sau |
| Hexagonal vs Layered đơn thuần | Hexagonal | Ports & Adapters cho phép test Application layer không cần Infra; đổi adapter dễ |
| Deterministic + AI vs LLM thuần | Kết hợp | Đảm bảo an toàn và tốc độ; chi phí LLM thấp; có fallback template khi LLM lỗi |

---

## 3.3. Bounded Context Map và ánh xạ vật lý

### 3.3.1. Năm Bounded Context nghiệp vụ

| # | Context | Câu hỏi nghiệp vụ | Phân loại | Thư mục |
|---|---|---|---|---|
| 1 | User Profile | *"Tôi là ai? Thể trạng ra sao?"* | Supporting | `internal/profile/` |
| 2 | Coaching & Planning | *"Tôi nên tập gì? Khi nào điều chỉnh?"* | Core | `internal/coaching/` |
| 3 | Workout Execution & Motion | *"Tôi tập thế nào? Tư thế đúng/sai?"* | Core | `internal/workout_execution/` |
| 4 | Nutrition | *"Tôi ăn gì hôm nay?"* | Core | `internal/nutrition/` |
| 5 | Exercise | *"Danh mục bài tập chuẩn gồm những gì?"* | Supporting | (module thư viện, có thể nằm ngang cùng Workout Execution) |

Ba module Generic (không phải BC nghiệp vụ) phục vụ toàn hệ thống:

- **Auth** (`internal/auth/`) — xác thực, JWT, JWKS rotation, OAuth.
- **Notification** (`internal/notification/`) — push, email, in-app.
- **Audio** (`internal/audio/`) — quản lý và phân phối kho thoại (voice cache) theo `CoachPersonality`.

### 3.3.2. Context Map — quan hệ giữa các BC

```
   User Profile ── BiologicalMetrics ─▶ Coaching & Planning
                └─ BiologicalMetrics ─▶ Nutrition

   Exercise ──── Exercise (ID ref) ───▶ Coaching & Planning
             └── Exercise (ID ref) ───▶ Workout Execution & Motion

   Coaching & Planning ── WorkoutPrescription ─▶ Workout Execution & Motion

   Workout Execution ─── WorkoutSessionCompleted ─▶ Coaching & Planning
                     └─ BodyMetricUpdated       ─▶ User Profile
                     └─ NewPersonalRecordAchieved ─▶ (Notification)

   Nutrition ── NutritionPlanGenerated ────────▶ (Notification)
```

Quan hệ chủ yếu là **customer/supplier** một chiều qua Domain Event, hạn chế phụ thuộc trực tiếp. Coaching là "khách hàng" của Workout Execution qua event `WorkoutSessionCompleted`; User Profile là "khách hàng" nhận `BodyMetricUpdated`.

Chi tiết Aggregate, Value Object và Domain Event của từng BC được trình bày trong các mục Journey 3.4–3.8 dưới đây, và tổng hợp lại trong mục 3.9.

---

## 3.4. Journey 1 — Đăng ký và Onboarding

**Use case bao trùm**: UC-01.1, UC-01.2, UC-01.3.

### 3.4.1. Câu chuyện người dùng

Người mới mở app lần đầu. Họ muốn bắt đầu càng nhanh càng tốt. FitAI phải cân bằng: (i) thu thập đủ dữ liệu để sinh lộ trình an toàn, (ii) không bắt user nhập một biểu mẫu dài lê thê. Do đó FitAI chia làm hai giai đoạn:

1. **Đăng ký nhanh** (UC-01.1): chỉ cần Email/SĐT + OTP, hoặc OAuth. Sau bước này user đã có tài khoản `Incomplete` và có thể vào app.
2. **Hoàn thiện hồ sơ** (UC-01.2): các trường bắt buộc (chỉ số sinh học + mục tiêu) được nhắc dần cho tới khi `ProfileCompletionRate ≥ 80%`. Khi đạt ngưỡng, `ActiveCoachEnabled = true` và event `UserProfileCompleted` được phát → kích hoạt Journey 2.

Chấn thương và bệnh lý (UC-01.3) không bắt buộc lúc đăng ký, có thể khai báo bất cứ khi nào; điều này giữ luồng đăng ký ngắn nhưng vẫn cho user cập nhật thông tin an toàn quan trọng.

### 3.4.2. Aggregate và Value Object

Aggregate Root `User` trong `User Profile Context`:

- **Entities**: `Injury` (vùng cơ, ngày báo, trạng thái `Active | Recovered`).
- **Value Objects**:
  - `BiologicalMetrics` — tuổi, giới tính, chiều cao, cân nặng, %mỡ.
  - `TrainingScheduleSlot` — khung giờ tập cố định.
  - `ChatbotContext` — thiết bị có sẵn, dị ứng thức ăn (thu thập dần qua chatbot).
  - `CoachPersonality` — `DrillSergeant | BestFriend | DataAnalyst`.
- **Invariants**: `ActiveCoachEnabled = true` chỉ khi hồ sơ ≥ 80% (BR-UM-01).
- **Events**: `UserProfileCompleted`, `InjuryReported`, `InjuryRecovered`.

### 3.4.3. Auth — xác thực và phiên

Auth là module Generic dùng chung. Cấu trúc phiên tuân theo các quyết định thực tế trong `internal/auth/`:

- **Access token JWT** ký RS256, TTL ngắn (khoảng 15 phút).
- **JWKS rotation** ba trạng thái (`active | inactive | retired`) — có Grace Period cho token đã phát bằng key cũ.
- **Refresh token** lưu trong PostgreSQL bảng `auth.sessions` (không phải Redis) để hỗ trợ **session-per-device**: user có thể đăng xuất một thiết bị cụ thể.
- **OAuth** cho Google, Apple, Facebook theo chuẩn OAuth 2.0 + PKCE.
- **Outbox Pattern**: các event `UserRegistered`, `UserLoggedIn` được đẩy vào bảng outbox trong cùng transaction với thay đổi state, worker publish sang Event Bus dưới dạng CloudEvents 1.0.

### 3.4.4. Luồng chính

```
   User               API Gateway         Auth Module         Profile Module
   ────               ───────────         ───────────         ──────────────
   
   Nhập Email/OTP ──▶ POST /auth/register
                      ────────────────▶  Create User
                                         Send OTP
                      ◀────────────────  OTP sent
   
   Nhập OTP     ──▶  POST /auth/verify
                      ────────────────▶  Verify OTP
                                         Issue access + refresh
                                         Publish UserRegistered
                      ◀────────────────  Tokens
   
   Điền form
   hồ sơ       ──▶   PUT /profile
                      ─────────────────────────────────────▶  Update User
                                                              Compute rate
                                                              If ≥80%:
                                                                Publish 
                                                                UserProfileCompleted
   
                                         ◀── Event bus ──   UserProfileCompleted
                                         (Coaching listens → Journey 2 starts)
```

---

## 3.5. Journey 2 — Nhận lộ trình (Coaching & Planning khởi tạo)

**Use case bao trùm**: UC-02.1, UC-02.3.

### 3.5.1. Câu chuyện người dùng

Sau khi hoàn thiện hồ sơ, user muốn thấy ngay một *bức tranh dài hạn*: "tôi sẽ tập gì trong 4 tuần tới, tuần này tập ngày nào, nhóm cơ nào". FitAI phản hồi bằng cách sinh **hai tầng kế hoạch**:

- **Lộ trình 4 tuần (`WorkoutRoadmap`)**: mốc định hướng, chia thành `RoadmapPhase` với target Progressive Overload.
- **Lịch tuần (`WeeklySchedule`)**: phân bổ `MuscleSplit` theo ngày, tuân thủ BR-AC-01 (≥ 1 ngày nghỉ, ≤ 6 buổi/tuần).

Chưa sinh giáo án chi tiết (đó là JIT ở Journey 3). Cách này giúp user thấy được cam kết dài hạn nhưng vẫn cho AI Coach linh hoạt điều chỉnh sát ngày.

### 3.5.2. Aggregate

Trong `Coaching & Planning Context`:

- **`WorkoutRoadmap`** — Aggregate Root cho lộ trình 4 tuần.
  - VO: `RoadmapPhase`, `CompletionRate`.
  - Lifecycle: `Active → Paused → Resumed → Completed`.
  - Events: `RoadmapInitiated`, `RoadmapAdjusted`, `RoadmapPaused`, `RoadmapResumed`.
- **`WeeklySchedule`** — Aggregate Root, tham chiếu `WorkoutRoadmap` bằng ID (không nhúng).
  - VO: `MuscleSplit`, `DailyPlanIds`.
  - Invariants: BR-AC-01 (≥ 1 ngày nghỉ, ≤ 6 buổi), BR-AC-03 (buổi bỏ không tự dồn bù).

Domain Services:

- **`OverloadValidator`** — kiểm tra volume tuần mới ≤ 110% volume thực tế tuần trước (BR-AC-02). Nếu vượt, tự giảm và retry.
- **`AdaptiveCoachEngine`** — dùng cho Journey 4 (Adaptive Review), không dùng ở khởi tạo.

### 3.5.3. Luồng khởi tạo lộ trình (UC-02.1)

Trigger: nhận event `UserProfileCompleted`.

```
   AdaptiveCoachEngine input:
   - BiologicalMetrics (age, W, H, sex, %fat)
   - Goal (TangCo / GiamMo)
   - Injury[] active
       │
       ▼
   Compute FitnessScore
       │
       ▼
   Xác định RoadmapPhase khởi điểm
       │
       ▼
   Sinh WorkoutRoadmap (4 tuần, CR=0)
       │
       ▼
   Sinh WeeklySchedule tuần 1 (MuscleSplit)
       │
       ▼
   OverloadValidator kiểm tra volume tuần 1
       │
       ▼ (pass)
   Save + Publish RoadmapInitiated, WeeklyScheduleGenerated
```

### 3.5.4. Sinh lịch tuần tiếp theo (UC-02.3)

Cuối mỗi tuần, `CoachingService.GenerateWeeklySchedule()` đọc tổng volume thực tế tuần vừa rồi, sinh `WeeklySchedule` mới, gọi `OverloadValidator` để đảm bảo tăng không quá 10%. Nếu bị từ chối, tự động giảm tải và thử lại.

### 3.5.5. Ranh giới Agent

Ở Journey 2, Agent **không tham gia** vào bước tính toán hay validate. Agent chỉ được gọi khi cần **sắp xếp thứ tự bài tập** theo nguyên tắc thể thao (compound trước isolation) trong bước sinh giáo án buổi (JIT — Journey 3). Việc khởi tạo `WorkoutRoadmap` và `WeeklySchedule` là deterministic hoàn toàn.

---

## 3.6. Journey 3 — Check-in đầu ngày và Sinh giáo án buổi (JIT)

**Use case bao trùm**: UC-02.2.

### 3.6.1. Câu chuyện người dùng

Đến ngày tập, user mở app. Trước khi bắt đầu, họ có thể:
- vừa bị đau vai
- đang ở nhà nghỉ, không có ổ tạ
- mới ăn no, muốn tập nhẹ hơn hôm nay

Nếu FitAI sinh giáo án trước từ đêm hôm trước mà không hỏi lại, người dùng có nguy cơ tập sai với trạng thái thực. Vì thế FitAI thiết kế luồng **Just-In-Time**: hỏi ngắn ngay khi user mở app, cập nhật ngữ cảnh, rồi sinh (hoặc điều chỉnh) giáo án tương ứng.

### 3.6.2. Kết hợp Pre-cache và Warm-up Rendering

Vấn đề: nếu chỉ sinh khi user mở app, độ trễ có thể 3–5 giây (LLM chọn bài + Backend tính tạ). ADR-01 giải quyết bằng cách:

- **Pre-cache**: cuối buổi tập trước, Backend đã sinh sẵn `DailyWorkoutPlan (draft_cached)` cho buổi tiếp theo dựa trên trạng thái *cuối tuần trước*.
- **Warm-up Rendering**: khi user mở app, hệ thống hiển thị ngay bài Warm-up từ cache (độ trễ 0 ms). Trong lúc user khởi động, Backend kiểm tra check-in, sinh lại giáo án chính nếu có thay đổi ngữ cảnh, và **stream progressive** từng bài chính về client bằng NDJSON qua SSE — thay thế skeleton từng ô bài tập.

### 3.6.3. Tool duy nhất và gọi song song

ADR-01 gộp ba thao tác `UpdateInjuryStatus`, `UpdateUserEquipments`, `SearchExercises` thành **một Tool duy nhất `UpdateWorkoutContext`** để giảm số vòng function call, cho phép gọi song song với `SearchExercises`. Payload:

```json
{
  "avoid_joints": ["wrist"],
  "recovered_joints": ["knee"],
  "override_equipments": ["dumbbell", "bodyweight"]
}
```

Backend hợp nhất với DB theo công thức:

$$\text{Khớp khóa cuối} = (\text{Chấn thương active}) \setminus (\text{Recovered do Agent}) \cup (\text{Avoid do Agent})$$

Sau đó chạy SQL lọc **30–40 bài tập** an toàn (Two-Stage Selection) và trả về cho Agent.

### 3.6.4. Ranh giới Agent vs Backend trong Journey 3

Đây là điểm nhấn kiến trúc quan trọng của dự án. Tài liệu `ai_agent_and_backend_go_responsibility_separation.md` quy định rõ:

| Nhiệm vụ | Backend Go | AI Agent |
|---|---|---|
| Lọc bài an toàn theo chấn thương/thiết bị | ✔ SQL query | — |
| Chọn tập con bài + sắp xếp thứ tự (compound → isolation) | — | ✔ Reasoning |
| Áp số tạ, set, rep, warm-up/cool-down | ✔ Epley + BR-AC-02 | — |
| Trích xuất chấn thương/thiết bị từ câu trả lời user | — | ✔ NLP |
| Merge ngữ cảnh với DB | ✔ Deterministic | — |
| Sinh lời giải thích lý do chọn bài | — | ✔ NL generation |
| Fast-track / Down-track (BR-AC-02) | ✔ Số học | — |
| Bảo vệ khớp sau phục hồi 3 buổi (BR-AC-09) | ✔ Rule engine | — |

Agent **không tự bịa bài tập** — bắt buộc chọn từ tập ứng viên do Backend cung cấp. Nếu Agent trả về `exercise_id` không có trong DB, Backend hoặc loại bỏ bài đó hoặc kích hoạt **Static Rule Fallback**.

### 3.6.5. Fallback Static Rule Template

Khi gọi Gemini bị timeout (> 3 s) hoặc trả JSON sai cấu trúc:

- Backend tự lấy **Workout Template** phù hợp với thiết bị và chấn thương từ DB.
- Áp số tạ/set/rep bằng rule engine.
- Hiển thị lời nhắn *"AI Coach hiện đang bận, đây là giáo án tiêu chuẩn của bạn hôm nay"*.

Cách này đảm bảo user không bao giờ bị treo, ngay cả khi dịch vụ LLM ngoài gặp sự cố.

### 3.6.6. Bảo mật dữ liệu khi gọi LLM

- Payload gửi Vertex AI: chỉ `anonymous_session_id`, chỉ số cơ thể làm tròn, không có định danh cá nhân.
- Cấu hình **Zero Data Retention** phía provider (GDPR/HIPAA-compliant).

---

## 3.7. Journey 4 — Buổi tập (Workout Execution & Motion)

**Use case bao trùm**: UC-03.1, UC-03.2, UC-03.3, UC-03.4, UC-03.5.

Đây là journey **quan trọng nhất về mặt kỹ thuật** — nơi Edge AI, Rule Engine và trải nghiệm real-time gặp nhau. Dự án dành riêng một tài liệu 7 tầng cho phần này (`ai_camera_coach_design_and_roadmap.md`); mục 3.7 tổng hợp các quyết định thiết kế cốt lõi.

### 3.7.1. Aggregate

Trong `Workout Execution & Motion Context`:

- **`WorkoutSession`** — Aggregate Root cho một buổi tập.
  - Entities: `WorkoutSetLog` (rep, tạ, FormScore trung bình, RPE).
  - VO: `SessionSummary`, `RepLog` (skeleton thô + ROM + lỗi từng rep, chỉ nhánh AI), `SessionTimer`.
  - Lifecycle: `Scheduled → InProgress → Completed | Aborted (Anomalous)`.
  - Invariants: BR-CC-01, BR-CC-02, BR-WL-01, BR-WL-03.
  - Events: `WorkoutSessionStarted`, `WorkoutSessionCompleted`, `WorkoutSessionAborted`, `BodyMetricUpdated`.
- **`WorkoutPerformance`** — Aggregate Root lưu kỷ lục 1RM (Epley).
  - Entities: `PersonalRecord` per exercise.
  - Events: `NewPersonalRecordAchieved`.
- **`MotionSpecification`** — Aggregate Root cấu hình AI cho từng bài tập.
  - VO: `PoseTemplate` (17 điểm), `CalibrationConfig` (khoảng cách 1.5–2.5 m, góc camera), `RepCountingRules` (ROM ≥ 70%), `FormScoringRules`.

Domain Service:

- **`TrainingLoadGuard`** — kiểm tra volume buổi hiện tại > 250% trung bình 5 buổi gần nhất cùng nhóm cơ (BR-WL-02); nếu vượt, yêu cầu user xác nhận và chèn ngày nghỉ.

### 3.7.2. Kiến trúc AI Camera 7 tầng (client-side)

```
   ┌─────────────────────────────────────────────────────────┐
   │ Client (Web: onnxruntime-web · Android: ORT Mobile)     │
   │                                                         │
   │  Tầng 1 — Pose Detection                                │
   │    Model .onnx (RTMPose/YOLOv8-Pose hoặc MMPose bundle) │
   │    Input: video 30 fps, 640×480 / 1280×720              │
   │    Output: 17 keypoints (COCO) + confidence             │
   │             │                                           │
   │             ▼                                           │
   │  Tầng 2 — Feature Extraction & Local Rule Engine        │
   │    - Tính góc khớp (dot product / arccos)               │
   │    - Bộ lọc jitter (One-Euro Filter)                    │
   │    - State Machine đếm rep + hysteresis                 │
   │    - Sliding window để chống nhiễu                      │
   │             │                                           │
   │             ▼                                           │
   │  Tầng 3 — Severity Classifier                           │
   │    Model .onnx phân loại lỗi (RandomForest/SVM/MLP)     │
   │    Huấn luyện bằng scikit-learn, xuất qua skl2onnx      │
   │    Input: vector góc khớp + velocity                    │
   │    Output: 0 (không lỗi) / 1 (nhẹ) / 2 (nặng)           │
   │             │                                           │
   │             ▼                                           │
   │  Tầng 4 — Dialogue Engine & Voice Cache                 │
   │    - Priority Queue (chọn lỗi nghiêm trọng nhất)        │
   │    - Cooldown Timer (Severity 1: 1 lần/buổi;            │
   │                       Severity 2: 3 s)                  │
   │    - Local Voice Cache theo CoachPersonality            │
   │    - Audio Ducking (giảm 70% nhạc nền khi phát cảnh báo)│
   │             │                                           │
   │             ▼                                           │
   │  Tầng 5 — Local Log Buffer + Async Batch Sync           │
   │    - Lưu mọi lỗi (kể cả bị Priority Queue drop)         │
   │    - POST /logs mỗi 10 s: batch upload                  │
   │    - POST /summary khi kết thúc: chỉ số tổng            │
   └─────────────────────────────────────────────────────────┘
                                │
                                ▼
   ┌─────────────────────────────────────────────────────────┐
   │ Backend Go                                              │
   │                                                         │
   │  Tầng 6 — Data Preparation & Speech Repository          │
   │    - Cung cấp MotionSpecification khi bắt đầu bài       │
   │    - Cấu hình Rules JSON + Dialogue Map                 │
   │    - URL file .onnx (Pose + Severity) trên CDN          │
   │    - Ngân hàng câu thoại theo CoachPersonality          │
   │                                                         │
   │  Tầng 7 — Data Scraping & Model Training Pipeline       │
   │    Python + yt-dlp + Streamlit (human-in-the-loop)      │
   │    - Cào video mẫu, trích 17 keypoints                  │
   │    - Gán nhãn tự động + duyệt tay                       │
   │    - Huấn luyện RandomForest → xuất .onnx               │
   └─────────────────────────────────────────────────────────┘
```

### 3.7.3. Năm use case của Journey 4

**UC-03.1 StartWorkoutSession** — user check-in, chọn playlist, hệ thống tạo `WorkoutSession (InProgress)`, khởi động `SessionTimer`, phát `WorkoutSessionStarted`. Nếu giáo án có Warm-up, chuyển sang UC-03.2 hoặc UC-03.3 kèm nút Skip.

**UC-03.2 LogSet — nhánh AI Camera** — bật camera, calibrate khoảng cách (1.5–2 m), tải `PoseTemplate` + `RepCountingRules`, tracking 17 điểm theo thời gian thực. Mỗi rep: ROM ≥ 70% thì `repCount++` và tính `FormScore`. Lỗi phát hiện → Audio Ducking + voice cảnh báo. User xác nhận kết quả → tạo `WorkoutSetLog`. Xử lý cả anti-cheat (BR-CC-02): frame skeleton hợp lệ < 50% toàn buổi → gắn cờ `AntiCheat`.

**UC-03.3 LogSet — nhánh Phi AI** — timer + hướng dẫn on-demand, user nhập tay số rep và tạ. `FormScore = N/A` (BR-WL-03). Vẫn có Audio Ducking khi bật thuyết minh.

**UC-03.4 CompleteWorkoutSession** — user bấm kết thúc. Nếu giáo án có Cool-down → chạy UC-03.2/03.3 với Skip. Gọi `TrainingLoadGuard`: nếu volume vượt 250% trung bình 5 buổi gần nhất → yêu cầu user xác nhận, chèn ngày nghỉ. Tính `SessionSummary`, chuyển sang `Completed`, phát `WorkoutSessionCompleted`. Xử lý các trường hợp bất thường:
- Quá 240' không tương tác → tự đóng, gắn `AnomalousSession`, loại khỏi tính Overload (BR-WL-01).
- Quá 90'/180' → cảnh báo, cho user chọn tiếp tục hay kết thúc.
- User cập nhật cân → phát `BodyMetricUpdated` sang User Profile.

**UC-03.5 RecordPersonalRecord** — xử lý bất đồng bộ sau `WorkoutSessionCompleted`. Tính 1RM Epley cho từng bài, so với `PersonalRecord` hiện tại. Nếu vượt → cập nhật + phát `NewPersonalRecordAchieved` → notification vinh danh PR. `AnomalousSession` không tính. Bài phi AI vẫn tính nhưng gắn `Unverified`.

### 3.7.4. Đồng bộ dữ liệu client → server

Đây là điểm phân tách rõ ràng theo tài liệu AI Camera:

| Endpoint | Nội dung | Tần suất |
|---|---|---|
| `POST /workouts/sessions/{id}/logs` | Batch của `error_logs` + `rep_logs` chi tiết | Mỗi 10 s trong khi tập + phần còn sót khi kết thúc |
| `POST /workouts/sessions/{id}/summary` | `SessionSummary` (tổng reps, sets, FormScore) | Một lần khi kết thúc |

Không gửi lại toàn bộ logs khi kết thúc để tiết kiệm băng thông. Nếu client offline lúc tập, buffer trong RAM và gửi khi có mạng.

### 3.7.5. Trade-off tại Journey 4

- **Client-side full inference** thay vì gửi tọa độ lên server: chọn client vì privacy + latency; đánh đổi kích thước model và pin.
- **RandomForest ONNX** thay vì deep learning end-to-end: chọn RF vì (i) train được từ vector góc khớp (feature engineering rõ), (ii) dễ giải thích lỗi, (iii) model nhẹ, chạy tốt trên browser/mobile.
- **Priority Queue + Cooldown** thay vì phát mọi cảnh báo: tránh loạn user; chấp nhận có thể miss vài lỗi Severity 1.
- **Batch sync + summary tách endpoint** thay vì stream real-time: đơn giản, ít stateful, tiết kiệm băng thông.

### 3.7.6. Tổng quan luồng tập end-to-end

Gom năm use case ở 3.7.3 và các use case phụ đề xuất bổ sung, luồng tập của một người dùng trải qua các bước sau (ký hiệu ✓ = đã có trong docs, + = đề xuất bổ sung):

```
   ┌────────────────────────────────────────────────────┐
   │ Trước buổi                                         │
   │  ✓ Check-in Q&A (UC-02.2)                          │
   │  ✓ Nhận DailyWorkoutPlan (pre-cache + NDJSON)      │
   │  + Calibrate camera lần đầu mỗi bài (UC-03.6)      │
   └───────────────────────┬────────────────────────────┘
                           ▼
   ┌────────────────────────────────────────────────────┐
   │ Bắt đầu                                            │
   │  ✓ Start Workout Session (UC-03.1)                 │
   │    - Tạo WorkoutSession (InProgress)               │
   │    - Chọn playlist                                 │
   │  + Warm-up: Execute hoặc Skip (UC-03.7)            │
   └───────────────────────┬────────────────────────────┘
                           ▼
   ┌────────────────────────────────────────────────────┐
   │ Vòng lặp cho từng bài × từng set                   │
   │                                                    │
   │  ┌────────────────────────────────────────────┐    │
   │  │  Bài có AI Camera?                         │    │
   │  │    ✓ Có  → UC-03.2 (LogSet AI)             │    │
   │  │    ✓ Không → UC-03.3 (LogSet phi AI)       │    │
   │  └───────────────────┬────────────────────────┘    │
   │                      │                             │
   │  Trong lúc tập:                                    │
   │    + Xem/nghe hướng dẫn on-demand (UC-03.13)       │
   │    + Đổi bài giữa buổi (UC-03.11)                  │
   │    + Bỏ set (đề xuất UC-03.14)                     │
   │    + Điều chỉnh thời gian nghỉ (UC-03.12)          │
   │    + Chuyển AI → phi AI (thiếu sáng, UC-03.10)     │
   │                                                    │
   │  Sự kiện gián đoạn:                                │
   │    + Pause & Resume (UC-03.9)                      │
   │    ✓ Camera mất kết nối → alt flow UC-03.2 A2      │
   │    ✓ Anti-cheat (frame < 50%) — BR-CC-02           │
   │                                                    │
   │  Sau mỗi rep:                                      │
   │    ✓ ROM ≥ 70% → repCount++, FormScore             │
   │    ✓ Lỗi → Priority Queue + Cooldown + Voice       │
   │                                                    │
   │  Sau mỗi set:                                      │
   │    ✓ User xác nhận WorkoutSetLog                   │
   │    ✓ Timer đếm ngược nghỉ                          │
   └───────────────────────┬────────────────────────────┘
                           ▼
   ┌────────────────────────────────────────────────────┐
   │ Kết thúc                                           │
   │  + Cool-down: Execute hoặc Skip (UC-03.8)          │
   │  ✓ Complete Workout Session (UC-03.4)              │
   │    - TrainingLoadGuard (BR-WL-02)                  │
   │    - Xử lý AnomalousSession nếu >240′ (BR-WL-01)   │
   │    - Optional: cập nhật cân → BodyMetricUpdated    │
   │    - SessionSummary                                │
   │    - Publish WorkoutSessionCompleted               │
   └───────────────────────┬────────────────────────────┘
                           ▼
   ┌────────────────────────────────────────────────────┐
   │ Sau buổi (bất đồng bộ)                             │
   │  ✓ Record Personal Record (UC-03.5) — Epley + PR   │
   │  ✓ AdaptiveCoachEngine kiểm tra Signal B3          │
   │  ✓ Sinh trước DailyWorkoutPlan buổi tiếp (draft)   │
   │  ✓ Post-session Report (notification + màn)        │
   │  → Chuyển sang Journey 7 (theo dõi)                │
   └────────────────────────────────────────────────────┘
```

Điểm then chốt: **rule engine và Domain Service Go xử lý mọi phép tính** (ROM, FormScore, TrainingLoadGuard, Epley, AntiCheat); **Agent LLM chỉ tham gia ở trước buổi** (sinh giáo án + explanation) và **sau buổi** (Post-session Report). Trong lúc tập, mọi phản hồi âm thanh và đếm rep diễn ra hoàn toàn tại client, không có network dependency.

---

## 3.8. Journey 5 — Sau tập (Adaptive Review Cycle)

**Use case bao trùm**: UC-04.1 → UC-04.5.

### 3.8.1. Câu chuyện người dùng

Sau buổi tập không phải chỉ có "xem báo cáo và về". Hệ thống phải quan sát dài hạn — user có đi tập đều không, có bỏ ngày nào lặp lại không, có dấu hiệu quá tải không, có bị plateau không. Nếu không phát hiện và can thiệp sớm, user sẽ nản và bỏ (đúng bối cảnh đã nêu ở Chương 1).

FitAI dùng `AdaptiveCoachEngine` (Domain Service trong Coaching Context) để giám sát và kích hoạt điều chỉnh theo 5 tín hiệu:

### 3.8.2. Bảng năm tín hiệu

| Mã | Tín hiệu | Trigger | Hành động |
|---|---|---|---|
| **UC-04.1** | Cuối chu kỳ 4 tuần | Roadmap kết thúc | Tính CR, áp BR-AC-04: <40% → giảm số buổi; 40–70% → giảm tải 10–15% + Express 30′; 70–90% → tăng ≤10% Overload; ≥90% → thêm buổi + badge |
| **UC-04.2 (B1)** | Không hoạt động | ≥ 7 ngày không có session | Check-in theo CoachPersonality, 3 phương án: tiếp tục / đặt lại / Pause (tối đa 4 tuần) |
| **UC-04.3 (B2)** | Lịch không tương thích | Bỏ cùng 1 ngày trong tuần ≥ 3 lần liên tiếp | Đề xuất dời slot; đồng ý → cập nhật; từ chối → giữ, không hỏi lại |
| **UC-04.4 (B3)** | Overtraining | ≥ 2 buổi/ngày hoặc RPE trung bình ≥ 8.5 liên tục ≥ 5 buổi | Cảnh báo, chèn 1 ngày nghỉ bắt buộc |
| **UC-04.5 (B4)** | Plateau | 1RM + FormScore không tăng 3 tuần liên tiếp (CR ≥ 70%) | 3 phương án: Deload 40% / đổi biến thể / tăng set |

### 3.8.3. Cơ chế thực thi

`AdaptiveCoachEngine` chạy như một background job (cron hoặc reactive theo event):

- Khi nhận `WorkoutSessionCompleted` → kiểm tra B3 (overtraining).
- Cron daily → kiểm tra B1 (7 ngày), B2 (pattern bỏ ngày).
- Cron weekly → kiểm tra B4 (plateau).
- Cron cuối chu kỳ → chạy UC-04.1.

Mọi phát hiện và quyết định deterministic được ghi vào `AgentAction` log để audit. LLM tham gia ở bước **sinh lời nhắn theo CoachPersonality**, không tham gia phát hiện.

### 3.8.4. Trade-off

- **Adaptive theo event + cron** thay vì stream real-time: giảm complexity; đủ real-time cho use case ngày/tuần.
- **AI chỉ sinh câu văn** thay vì AI phát hiện tín hiệu: các tín hiệu có công thức số học rõ, deterministic đủ; LLM chỉ cần personality.

---

## 3.9. Journey 6 — Dinh dưỡng

**Use case bao trùm**: UC-05.1, UC-05.2.

### 3.9.1. Câu chuyện người dùng

Người mới thường tập trung tập mà quên ăn (đã nêu ở Chương 1). FitAI xử lý bằng cách sinh **thực đơn ngày** đồng bộ với ngân sách calo/macro và lịch tập, và chống lặp món để tránh nhàm.

### 3.9.2. Aggregate

Trong `Nutrition Context`:

- **`NutritionPlan`** — Aggregate Root cho kế hoạch ngày.
  - VO: `DailyMealOption` (Sáng/Trưa/Tối/Phụ, tự nấu hoặc ăn ngoài), `CalorieAllocation` (target + P/C/F), `BudgetTier` (`TietKiem | PhoThong | ThoaiMai`).
  - Invariant: `CalorieAllocation.target ≥ 1200 kcal` (BR-NU-01).
- **`MealHistory`** — Aggregate Root theo dõi lịch sử ăn.
  - Entities: `MealLog`.
  - VO: `LockoutRegistry` (nguyên liệu đang khóa + ngày mở khóa).
  - Invariant: thêm `MealLog` → tự cập nhật `LockoutRegistry` (Protein 7 ngày, Carb 5 ngày, chủ đề món 3 ngày — BR-NU-02).
- **`FoodItem`** — Aggregate Root cho thư viện thực phẩm.
  - VO: `FoodNutrient` (calo, macro, nhãn chay/Halal, dị ứng).
  - Lifecycle: `Draft → PendingApproval → Active` (BR-NU-04).

### 3.9.3. Pipeline gợi ý thực đơn (UC-05.1)

```
   BiologicalMetrics + Goal + WorkoutSession hôm nay
                    │
                    ▼
   TDEE (Mifflin-St Jeor) × ActivityFactor
                    │
                    ▼
   CalorieAllocation (target + P/C/F)
   (điều chỉnh ±10% theo cường độ tập hôm nay)
                    │
                    ▼
   Filter FoodItem theo:
     - ChatbotContext.food_restrictions (dị ứng)
     - Chay/Halal
     - Lockout hiện tại (LockoutRegistry)
                    │
                    ▼
   Constraint Solver (heuristic + local search):
     Tổ hợp bữa thỏa calo ± ε, macro, số bữa
                    │
                    ▼
   Chọn top theo BudgetTier user đã set
                    │
                    ▼
   Response Assembler:
     Đính kèm chuỗi luật đã áp (calo, macro,
     không trùng, ngân sách) để UI hiển thị lý do
                    │
                    ▼
   Save NutritionPlan + Publish NutritionPlanGenerated
```

**Đặc biệt**: Journey 6 **không dùng LLM** ở bước sinh gợi ý. Toàn bộ là rule engine + constraint solver deterministic. Điều này khác với Journey 3 (Coaching) — nơi có Agent — và là quyết định có chủ ý:

- Dinh dưỡng có ngưỡng calo/macro rõ, ràng buộc solve tốt hơn LLM.
- LLM không đảm bảo tổng calo ± ε; rule engine đảm bảo.
- Rẻ hơn nhiều so với gọi LLM mỗi ngày cho mỗi user.

Lý do gợi ý được hiển thị dạng nhãn có cấu trúc (ví dụ *"Đủ protein sau tập chân"*, *"Không lặp với hôm qua"*, *"Trong ngân sách phổ thông"*) — UI dịch thành câu ngắn, không cần LLM.

### 3.9.4. Ghi log bữa ăn (UC-05.2)

User tìm món hoặc quét mã vạch → tìm `FoodItem` → xác nhận khẩu phần → tạo `MealLog`. System đọc `FoodNutrient` để xác định nguồn protein, carb, chủ đề món chính → cập nhật `LockoutRegistry`. Phát `MealLogged` + `LockoutApplied`.

### 3.9.5. Xử lý bất thường

- Tất cả protein bị khóa → tự giải phóng nguyên liệu unlock sớm nhất.
- Cân nặng cũ > 7 ngày → dùng giá trị cuối + cảnh báo.
- Sản phẩm không có trong DB → fallback nhập tay.

---

## 3.10. Journey 7 — Theo dõi tiến độ

**Use case đề xuất bổ sung** (chưa có file UC riêng trong docs): UC-06.1 → UC-06.5.

### 3.10.1. Câu chuyện người dùng

Sau vài tuần tập luyện đều đặn, người dùng bắt đầu cần *nhìn thấy tiến bộ* để duy trì động lực. Nếu FitAI chỉ đưa ra bài tập mà không kể lại "bạn đã đi được bao xa", user sẽ có cảm giác "không thay đổi gì" — đúng vấn đề đã nêu ở Chương 1. Vì thế Journey 7 tập trung vào **hiển thị tiến độ** dưới dạng dễ hiểu, không phải chỉ số kỹ thuật khô khan.

Đây là journey **đọc** (query-heavy): người dùng chỉ xem và ghi nhận chỉ số cơ thể, không có phép tính phức tạp hay ràng buộc real-time. Vì vậy Journey 7 đơn giản về mặt kiến trúc, tận dụng dữ liệu đã sinh ra từ các Journey 4 (buổi tập), Journey 5 (adaptive review) và Journey 1 (profile).

### 3.10.2. Tổng quan luồng theo dõi

```
   ┌─────────────────────────────────────────────────────────┐
   │ Nhập chỉ số                                             │
   │  + UC-06.1 LogBodyMetric                                │
   │    - Cân nặng, %mỡ, số đo (vòng eo, ngực, tay…)         │
   │    - Ảnh tiến trình                                     │
   │    - Ghi vào BodyMetricsHistory (Profile Context)       │
   │    - Publish UserMetricsUpdated                         │
   └────────────────────┬────────────────────────────────────┘
                        │
                        ▼
   ┌─────────────────────────────────────────────────────────┐
   │ Xem tổng quan                                           │
   │  + UC-06.2 ViewProgressDashboard                        │
   │    Nguồn dữ liệu:                                       │
   │      - BodyMetricsHistory (cân, %mỡ, số đo)             │
   │      - WorkoutSession (volume, số buổi)                 │
   │      - WorkoutPerformance (1RM theo bài)                │
   │      - WorkoutSetLog (FormScore trung bình)             │
   │    Biểu đồ:                                             │
   │      - Xu hướng cân nặng                                │
   │      - Volume tuần                                      │
   │      - Chart 1RM cho các bài chính                      │
   │      - FormScore trung bình                             │
   │      - Streak (chuỗi ngày tập liên tiếp)                │
   └────────────────────┬────────────────────────────────────┘
                        │
                        ▼
   ┌─────────────────────────────────────────────────────────┐
   │ Đi sâu                                                  │
   │  + UC-06.3 ViewSessionHistory                           │
   │    - Danh sách WorkoutSession theo tuần/tháng           │
   │    - Filter theo nhóm cơ, bài, ngày                     │
   │    - Drill-down: xem WorkoutSetLog + PoseError chi tiết │
   │                                                         │
   │  + UC-06.4 ViewPersonalRecordCollection                 │
   │    - Danh sách PR đạt được từ WorkoutPerformance        │
   │    - Phân biệt Verified vs Unverified (bài phi AI)      │
   │                                                         │
   │  + UC-06.5 ViewWeeklySummaryReport                      │
   │    - Báo cáo cuối tuần: CR tuần, volume, số PR          │
   │    - So sánh tuần trước → xu hướng cải thiện/tụt        │
   │    - Sinh trực tiếp từ AdaptiveCoachEngine output       │
   └─────────────────────────────────────────────────────────┘
```

### 3.10.3. Nguồn dữ liệu và Aggregate

Journey 7 **không có Aggregate mới** — chỉ đọc từ những Aggregate đã tồn tại:

| Nguồn | Bounded Context | Dùng cho |
|---|---|---|
| `BodyMetricsHistory` + `MetricsLogEntry` | User Profile | UC-06.1 nhập chỉ số; UC-06.2 chart cân/%mỡ |
| `WorkoutSession` + `WorkoutSetLog` | Workout Execution | UC-06.2 volume; UC-06.3 lịch sử buổi |
| `WorkoutPerformance` + `PersonalRecord` | Workout Execution | UC-06.2 chart 1RM; UC-06.4 danh sách PR |
| `WorkoutRoadmap` + `CompletionRate` | Coaching | UC-06.5 CR tuần/chu kỳ |

UC-06.1 (`LogBodyMetric`) là use case duy nhất có **ghi**; phát `UserMetricsUpdated` để đồng bộ với cache biểu đồ.

### 3.10.4. Cross-cutting với các Journey khác

- **Journey 4 (Workout)**: mỗi `WorkoutSessionCompleted` là một điểm dữ liệu mới cho Journey 7.
- **Journey 5 (Adaptive Review)**: `WeeklySummaryReport` (UC-06.5) chia sẻ dữ liệu với `AdaptiveCoachEngine` — cả hai đều đọc `WorkoutSession` để tính CR tuần và phát hiện xu hướng.
- **Journey 1 (Onboarding)**: các trường trong `BiologicalMetrics` được cập nhật qua UC-06.1 sẽ trigger `AdaptiveCoachEngine` nếu cân nặng thay đổi đáng kể (>5% trong 4 tuần) — có thể ảnh hưởng tới TDEE và giáo án.

### 3.10.5. Thiết kế UI (tổng quan)

- **Home tab**: widget streak + PR gần nhất + biểu đồ cân nặng 30 ngày.
- **Progress tab** (bottom nav): full dashboard với các tab con — *Body*, *Strength*, *Sessions*, *PRs*.
- **Weekly Report**: hiển thị dạng notification "Báo cáo tuần" mỗi tối Chủ nhật; click mở màn báo cáo chi tiết.
- **Body Metric Input**: quick action từ Home tab hoặc Progress tab; hỗ trợ chụp ảnh tiến trình từ camera trước.

### 3.10.6. Trade-off

- **Không tách materialized view** trong PostgreSQL: dùng truy vấn trực tiếp với index thích hợp; đủ nhanh cho quy mô nguyên mẫu. Khi scale → chuyển sang materialized view hoặc bảng aggregate cập nhật khi có `WorkoutSessionCompleted`.
- **Chart rendering ở client** (Recharts/Chart.js): server chỉ trả JSON số liệu thô. Giảm coupling giữa API và presentation.
- **Không có Analytics Context riêng**: chấp nhận Journey 7 đọc chéo nhiều BC. Nếu về sau phát sinh nhu cầu báo cáo BI phức tạp (cohort, funnel), có thể tách một BC `Progress & Analytics` riêng — nhưng chưa cần ở nguyên mẫu.

---

## 3.11. Dữ liệu, Contract và Communication xuyên suốt

### 3.11.1. Sơ đồ dữ liệu tổng thể

Sơ đồ dưới đây dùng đúng tên aggregate và value object trong tài liệu DDD:

```
   ┌────────┐     1  1  ┌───────────────────┐
   │  User  │───────────│ BiologicalMetrics │(VO)
   │ (Agg)  │     1  n  │                   │
   │        │─────┐     └───────────────────┘
   └────┬───┘     │
        │  1  n   │  ┌─────────┐
        │         └──│ Injury  │
        │            │(entity) │
        │            └─────────┘
        │  1  1
        │
        ▼
   ┌────────────────────┐
   │ BodyMetricsHistory │  1  n  ┌───────────────┐
   │      (Agg)         │────────│MetricsLogEntry│
   └────────────────────┘        └───────────────┘

   ┌────────────────┐  1  n  ┌────────────────┐  1  n  ┌──────────────────┐
   │ WorkoutRoadmap │────────│ WeeklySchedule │────────│ DailyWorkoutPlan │
   │ (Agg, 4-tuần)  │        │    (Agg)       │        │      (Agg)       │
   └────────────────┘        └────────────────┘        └────────┬─────────┘
                                                                │ WorkoutPrescription (VO)
                                                                │
                                                                ▼
   ┌────────────────┐  1  n  ┌────────────────┐          ┌────────────────┐
   │ WorkoutSession │────────│ WorkoutSetLog  │  ref     │ MotionSpec     │
   │   (Agg)        │        │  (entity)      │──────────│ (Agg, per bài) │
   └────────┬───────┘        └────────────────┘          └────────────────┘
            │  publishes                                          │
            │  WorkoutSessionCompleted                            │  VO
            ▼                                                     │
   ┌────────────────┐  1  n  ┌────────────────┐            ┌─────▼──────┐
   │WorkoutPerformance│──────│ PersonalRecord │            │PoseTemplate│
   │    (Agg)         │       │  (entity)      │            │Rules/Rules │
   └────────────────┘        └────────────────┘            └────────────┘

   ┌────────────────┐  1  n  ┌────────────────┐
   │ NutritionPlan  │────────│ DailyMealOption│
   │   (Agg)        │        │      (VO)      │
   └────────────────┘        └────────────────┘

   ┌────────────────┐  1  n  ┌──────────┐  ref  ┌────────────┐
   │ MealHistory    │────────│ MealLog  │───────│ FoodItem   │
   │   (Agg)        │        │ (entity) │       │   (Agg)    │
   └────────┬───────┘        └──────────┘       └────────────┘
            │  1  1
            ▼
   ┌────────────────┐
   │ LockoutRegistry│  (VO)
   └────────────────┘

   ┌────────────────┐
   │   Exercise     │  (Agg, thư viện dùng chung)
   └────────────────┘
```

### 3.11.2. Domain Events chính

| Event | Publisher | Subscribers |
|---|---|---|
| `UserProfileCompleted` | Profile | Coaching (trigger UC-02.1) |
| `InjuryReported` / `InjuryRecovered` | Profile | Coaching (loại bài) |
| `RoadmapInitiated` / `RoadmapAdjusted` / `RoadmapPaused` | Coaching | Notification |
| `WeeklyScheduleGenerated` / `ScheduleDayRescheduled` | Coaching | Notification |
| `DailyWorkoutPlanGenerated` | Coaching | Notification (nhắc lịch) |
| `WorkoutSessionStarted` | Workout Execution | — |
| `WorkoutSessionCompleted` | Workout Execution | Coaching (fast/down-track, Adaptive), Profile (nếu có `BodyMetricUpdated`) |
| `WorkoutSessionAborted` | Workout Execution | Coaching (loại khỏi Overload) |
| `BodyMetricUpdated` | Workout Execution | Profile (`BodyMetricsHistory`) |
| `NewPersonalRecordAchieved` | Workout Execution | Notification (vinh danh) |
| `NutritionPlanGenerated` | Nutrition | Notification |
| `MealLogged` / `LockoutApplied` | Nutrition | — |
| `ExerciseApproved` / `FoodItemApproved` | Exercise / Nutrition | — |

Event được publish bằng **Outbox Pattern**: transactional với thay đổi state, worker đẩy sang Event Bus dạng CloudEvents 1.0. Nguyên mẫu dùng in-memory event bus (`internal/shared/eventbus/`); có thể thay bằng Kafka khi cần scale.

### 3.11.3. API và Contract

- **Contract-first Protobuf** trong `proto/contracts/{core,supporting,generic}/<module>/v1/`.
- **Buf** để lint và breaking check.
- **gRPC + gRPC-Gateway** sinh cả gRPC binary lẫn HTTP/JSON endpoint từ cùng file `.proto` — client web dùng REST/JSON, mobile/native có thể dùng gRPC.
- **Swagger OpenAPI** sinh tự động cho tài liệu.

Không dùng WebSocket. Streaming (giáo án NDJSON, chat check-in) dùng **SSE**. Push notification dùng FCM/APNs, không cần persistent connection.

### 3.11.4. Schema-per-module

Mỗi bounded context có schema PostgreSQL riêng (`profile.*`, `coaching.*`, `workout.*`, `nutrition.*`, `exercise.*`, `auth.*`). Module tự quản lý migration của schema mình qua `Registry.GetPool(module)` + `search_path=<schema>`. Cách này chuẩn bị sẵn cho việc tách microservices (đổi thành database riêng).

---

## 3.12. Bảo mật (cross-cutting)

Chi tiết trong module Auth (`internal/auth/`), tổng hợp lại đây:

| Mối quan tâm | Biện pháp |
|---|---|
| Xác thực | JWT access token RS256 TTL 15′; refresh token trong PostgreSQL `auth.sessions` (session-per-device) |
| Key management | **JWKS Rotation** ba trạng thái (`active | inactive | retired`) với Grace Period |
| Đăng ký | OTP 6 chữ số, TTL 5′, khóa 15′ sau 3 lần sai |
| OAuth | Google/Apple/Facebook qua OAuth 2.0 + PKCE cho client public |
| Ủy quyền | RBAC (user, admin), enforce ở middleware; endpoint admin cách ly |
| Camera / Video | Không rời client. Chỉ tọa độ đã trừu tượng thành mã lỗi và summary lên server |
| LLM privacy | `anonymous_session_id`, làm tròn chỉ số, Zero Data Retention Vertex AI |
| Transport | HTTPS/TLS 1.3 bắt buộc, HSTS, CSP header |
| Storage | Password `bcrypt`/`argon2id`; ảnh tiến trình mã hóa at-rest khi triển khai cloud |
| Rate limit | Sliding window trong Redis cho auth, OTP, tool call agent |
| Input validation | Ở tầng handler dùng Protobuf validation (`buf.validate`) — reject sớm |
| Audit | `AgentAction` log mọi tool call của Agent; `auth.audit_log` cho hoạt động security |
| Secret management | Env var + `.env` (không commit); production dùng secret manager của cloud |

---

## 3.13. Frontend và Backend structure

### 3.13.1. Backend — Hexagonal per module

Cấu trúc thư mục thực tế:

```
   cmd/api/                             # main entrypoint
   internal/
     shared/
       database/                        # postgres pool registry (schema-per-module)
       eventbus/                        # in-memory event bus (Outbox → publish)
     gen/go/contracts/                  # protobuf-generated stubs
     
     coaching/                          # Core BC
       domain/
         workout_roadmap.go             # Aggregate
         adaptive_coach_engine.go       # Domain Service
         overload_validator.go
         events.go
         ports.go                       # WorkoutRoadmapRepository, LLMPort, ...
       application/
         initiate_roadmap_handler.go    # Command handler
         generate_daily_plan_handler.go
         evaluate_end_of_cycle_handler.go
         ...
       infrastructure/
         postgres/                      # adapter cài đặt WorkoutRoadmapRepository
         grpc/                          # gRPC handler
         llm/                           # Gemini adapter
       docs/
     
     workout_execution/                 # Core BC (similar Hexagonal layout)
     nutrition/                         # Core BC
     profile/                           # Supporting BC
     auth/                              # Generic
     notification/                      # Generic
     audio/                             # Generic
   
   proto/contracts/
     core/{coaching,workout_execution,nutrition}/v1/
     supporting/profile/v1/
     generic/{auth,notification,audio}/v1/
```

Application layer định nghĩa Command/Query Handlers, gọi Domain qua Ports. Infrastructure layer cài đặt Ports (Postgres, gRPC, LLM). Domain thuần Go, testable không cần bất kỳ dependency ngoài.

Wiring ở `cmd/api/main.go` — nơi duy nhất biết cách nối handler với service với repository.

### 3.13.2. Frontend — Screen flow

```
   Login/Register ──▶ Onboarding (đủ 80% → Home) ──▶ Home
                                                       │
                                                       ├─▶ Workout
                                                       │     ├─ Check-in Q&A (Agent hỏi)
                                                       │     ├─ Warm-up (render ngay)
                                                       │     ├─ Bài chính (stream NDJSON
                                                       │     │   thay skeleton)
                                                       │     ├─ Camera Flow (calibrate,
                                                       │     │   pose overlay, audio)
                                                       │     ├─ Non-AI (timer + log tay)
                                                       │     ├─ Cool-down
                                                       │     └─ Post-session Report
                                                       │         (notification + màn)
                                                       │
                                                       ├─▶ Nutrition (today + log meal)
                                                       │
                                                       ├─▶ Progress (chart, photos, PR)
                                                       │
                                                       └─▶ Settings (profile, coach style,
                                                            equipment, restrictions)
```

Bottom tab bar 3 mục: **Home**, **Nutrition**, **Progress**. Không có tab Chat — Agent chỉ xuất hiện ở check-in đầu buổi (một cửa sổ Q&A ngắn) và ở notification/màn hình sau tập (một chiều).

Workout mở dạng full-screen modal để tối đa không gian camera. State management: **React Query** cho server state, **Zustand** cho client state (session hiện tại, buffer pose). Pose inference chạy trong Web Worker để không block main thread.

---

## 3.14. Trade-offs tổng hợp

| Quyết định | Chọn | Ưu | Nhược | Lý do |
|---|---|---|---|---|
| Kiến trúc tổng thể | Modular Monolith + Hexagonal | Đơn giản triển khai, ranh giới rõ, dễ tách | Cùng scale | Phạm vi 1 sinh viên; Hexagonal đảm bảo migrate được sang microservices |
| Pose inference | Client-side (ONNX Runtime Web/Mobile) | < 150 ms, privacy, offline | Model bị giới hạn kích thước | Yêu cầu real-time không đạt qua mạng |
| Model pose | MMPose 17 keypoints + `.onnx` (Pose model + RandomForest Severity) | Vector góc rõ ràng, RF nhẹ, dễ giải thích lỗi | Kém linh hoạt hơn deep learning end-to-end | Ưu tiên tính giải thích + kích thước nhỏ + tài liệu dự án đã chọn hướng này |
| Rule engine + LLM | Kết hợp | Nhanh, deterministic, dễ test, chi phí LLM thấp | Cần code luật cho từng bài | Đếm rep/chấm form/TDEE/1RM đều có công thức; LLM chỉ tham gia chỗ khó số hóa |
| Vai trò Agent | Giới hạn 3 nhiệm vụ (chọn+sắp xếp bài, NLP extract, explanation) | Chi phí thấp, có fallback | Không đáp ứng Q&A mở | Người mới cần định hướng rõ, FAQ tĩnh đủ; tránh over-promising chatbot |
| Gọi LLM | Function Calling native (Gemini SDK) thay MCP | Ít lớp trung gian, độ trễ thấp | Khóa Gemini | Đội 1 người; adapter port cho phép đổi sau |
| Pre-cache + Warm-up | Sinh giáo án đêm trước + render Warm-up 0 ms | Perceived latency 0 | Thêm background job | ADR-01 lựa chọn để giải quyết 6–8s tool chain |
| Tool `UpdateWorkoutContext` gộp | 1 tool cho cả injury + recovered + equipment | Ít vòng LLM roundtrip | Payload to hơn | ADR-01 chọn Option 2 (giữ CQRS mức chấp nhận được) |
| Two-Stage Selection | Backend lọc 30–40 bài → Agent chọn | Agent không bịa bài | Cần index tốt cho SQL filter | Đảm bảo an toàn tuyệt đối, khớp yêu cầu |
| Fallback | Static Rule Template khi Gemini > 3 s | User không bao giờ bị treo | Giáo án ít cá nhân hóa hơn | Robustness > perfection |
| Communication | gRPC + gRPC-Gateway | Contract chặt, sinh cả gRPC + REST/JSON | Học curve | Tương thích cả web (JSON) và mobile (binary) |
| Streaming | SSE + NDJSON | Đơn giản, HTTP-friendly, progressive rendering | Một chiều | Đủ cho stream giáo án; không cần bidirectional |
| DB | PostgreSQL + schema-per-module + pgvector | Nhất quán, ít component | pgvector chậm hơn vector DB chuyên | Quy mô nguyên mẫu; sẵn sàng tách sau |
| JSONB | Cho payload bán cấu trúc (WorkoutPrescription, RulesConfig) | Linh hoạt, không migration mỗi field | Truy vấn phức tạp chậm hơn | Cấu hình thay đổi thường xuyên; index GIN khi cần |
| Auth | JWT + JWKS rotation + refresh token PG (session-per-device) | Hỗ trợ đăng xuất thiết bị, key rotation an toàn | Phức tạp hơn cookie session | Chuẩn bị cho mobile client, nhiều thiết bị |
| Event Bus | In-memory + Outbox + CloudEvents 1.0 | Đơn giản, transactional | Không scale cross-process | Modular Monolith 1 process; sẵn envelope chuẩn nếu đổi Kafka |
| Nutrition AI | Không dùng LLM, rule + constraint solver thuần | Deterministic, rẻ, chính xác về macro | Không có "giọng coach" | Ràng buộc rõ ràng hoạt động tốt hơn LLM; UI dịch nhãn thành câu ngắn |
| State FE | React Query + Zustand | Ít boilerplate, đủ dùng | Ít devtool bằng Redux | Phạm vi nguyên mẫu |
| Pose inference thread | Web Worker | Không block main thread | Message passing overhead | 30 fps inference bắt buộc phải off-main-thread |

---

## 3.15. Tự kiểm tra thiết kế

- **Logic**: mỗi FR ở 3.1 có ít nhất một UC bao trùm ở Journey 1–6, và mỗi Aggregate/Domain Service ở 3.10 có nhà rõ. Không FR mồ côi.
- **Bám sát docs**: ubiquitous language (WorkoutRoadmap, DailyWorkoutPlan, WorkoutSession, MotionSpecification, LockoutRegistry, MealHistory, PersonalRecord…), business rules (BR-*), và ADR (01 daily check-in flow, 02 workout planning, 03 coaching service contract) đều được cite.
- **Cân bằng**: Journey 4 (Workout Execution) là journey dài nhất — đúng với vai trò Core BC quan trọng nhất, tránh lệch về Agent như bản trước.
- **Khả năng mở rộng**: Hexagonal cho phép đổi adapter (Postgres → khác, Gemini → khác) không đụng logic; schema-per-module cho phép tách microservices; Outbox + CloudEvents chuẩn bị sẵn cho Kafka.
- **Over-engineering**: không dùng microservices, không dùng Kafka ngay, không tách vector DB, không thiết kế cho triệu người dùng — đúng tinh thần nguyên mẫu.
- **Điểm rủi ro cần lưu ý ở Chương 4**:
  - Độ chính xác Severity model cho các bài mới (phụ thuộc dữ liệu scrape).
  - Kiểm soát chi phí LLM khi user tạo nhiều lần check-in trong ngày.
  - Đồng bộ khi client offline lâu — buffer log có thể lớn.
  - Latency thực đo của pose inference trên smartphone tầm trung (chưa đo được ở Chương 3).

---

## 3.16. Danh sách sơ đồ cần vẽ (chuẩn bị cho báo cáo cuối)

ASCII inline trong chương này sẽ được thay bằng sơ đồ chuyên nghiệp (draw.io / PlantUML / Mermaid) trong bản báo cáo cuối:

1. **Use Case Diagram tổng thể** — 5 nhóm UC, actor User + AI Coach + AI Camera + AI Nutrition + Admin.
2. **Component Diagram / C4 Level 2** — 4 layer (Client / Gateway / Backend BC / Infra).
3. **Deployment Diagram** — client (browser/mobile), server (Go container), PostgreSQL, Vertex AI, CDN, FCM/APNs.
4. **Hexagonal Architecture per module** — domain / application / infrastructure với ports & adapters.
5. **Bounded Context Map** — 5 BC + 3 Generic, quan hệ theo Domain Event.
6. **ERD chi tiết** — theo aggregate roots đã liệt kê ở 3.10, kèm kiểu dữ liệu và khóa.
7. **State Machine của `WorkoutSession`** — Scheduled → InProgress → Completed | Aborted.
8. **State Machine của `WorkoutRoadmap`** — Active → Paused → Resumed → Completed.
9. **State Machine của `Exercise` / `FoodItem`** — Draft → PendingApproval → Active → Archived.
10. **Sequence — UC-01.2 CompleteHealthProfile** — user → API → Profile → publish `UserProfileCompleted`.
11. **Sequence — UC-02.1 InitiateWorkoutRoadmap** — event trigger → AdaptiveCoachEngine + OverloadValidator → Save.
12. **Sequence — UC-02.2 GenerateDailyWorkoutPlan (JIT + Warm-up Rendering)** — check-in Q&A → Gemini `UpdateWorkoutContext` → SQL filter → NDJSON stream.
13. **Sequence — UC-03.2 LogSet AI Camera** — client-side loop: MMPose → Severity ONNX → Priority Queue → Voice Cache.
14. **Sequence — UC-03.4 CompleteWorkoutSession** — TrainingLoadGuard → SessionSummary → publish `WorkoutSessionCompleted` → PR async.
15. **Sequence — UC-04.4 DetectSignalB3 Overtraining** — cron / event → AdaptiveCoachEngine → insert rest day.
16. **Sequence — UC-05.1 GenerateDailyNutritionPlan** — TDEE → LockoutRegistry filter → Constraint Solver → response.
17. **Data Flow Diagram AI Camera 7 tầng** — thay cho 3.7.2.
18. **Screen flow + wireframe** cho các màn quan trọng (Onboarding, Home, Workout Session, Post-session Report, Nutrition Today).

---

## 3.17. Tổng kết chương

Chương 3 đã trình bày FitAI theo hành trình người dùng — từ đăng ký, hoàn thiện hồ sơ, nhận lộ trình 4 tuần, check-in đầu ngày, buổi tập với AI Camera, giám sát thích ứng sau tập, tới gợi ý dinh dưỡng — với các quyết định thiết kế được đặt trong ngữ cảnh nghiệp vụ thay vì trình bày tách rời.

Kiến trúc FitAI được xây dựng trên bốn nguyên tắc: Edge-first cho AI real-time, Contract-first qua Protobuf, Modular Monolith với Hexagonal per module, và Deterministic first – AI second. Năm Bounded Context nghiệp vụ (User Profile, Coaching & Planning, Workout Execution & Motion, Nutrition, Exercise) cùng ba module Generic (Auth, Notification, Audio) được phân định rõ ràng theo tài liệu DDD Tactical.

Điểm nhấn kỹ thuật của dự án nằm ở Journey 4 — hệ thống AI Camera 7 tầng chạy hoàn toàn trên client, với MMPose cho pose estimation và RandomForest ONNX cho Severity Classifier, kết hợp Priority Queue + Cooldown Timer + Local Voice Cache để đạt phản hồi âm thanh dưới 150 ms và bảo vệ quyền riêng tư tuyệt đối. Ở phía Coaching, Agent Gemini được giới hạn ở ba nhiệm vụ (chọn+sắp xếp bài, trích xuất NLP từ check-in, sinh lời giải thích), phần còn lại do rule engine Go xử lý deterministic; bên Nutrition, không dùng LLM.

Chương 4 tiếp theo sẽ trình bày chi tiết hiện thực hóa: pipeline huấn luyện Severity model bằng scikit-learn + skl2onnx, cài đặt module Coaching với tích hợp Vertex AI + fallback Static Rule Template, backend Go theo Hexagonal, và triển khai bằng Docker Compose.
