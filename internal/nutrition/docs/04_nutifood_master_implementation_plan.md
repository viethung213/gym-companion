# Master Implementation Plan: Module AI Nutrition Engine / NutiFood (`internal/nutrition`)

Tài liệu kế hoạch kỹ thuật tổng thể dành cho Bounded Context **Nutrition / NutiFood** (`internal/nutrition`) chuẩn kiến trúc **Domain-Driven Design (DDD)**, **Hexagonal Architecture (Ports & Adapters)**, và các quy ước lập trình Go trong `AGENTS.md`.

Tài liệu được cập nhật chính xác theo nguyên lý **Server-Heavy Rule Engine + AI Creative Chef** (đã thiết kế tại các file **06, 07, 08**):
- **Server Go Backend**: Gánh **90% khối lượng công việc** — tính toán TDEE, phân bổ calo, tính chính xác từng gram nguyên liệu per 100g, lọc dị ứng, check `LockoutRegistry` (7/5/3 ngày) và ghép ma trận tổ hợp ($180,000+$ món).
- **External AI (Gemini/OpenAI)**: Đóng vai trò là **"Đầu bếp chế biến & Sáng tạo" (Creative Chef)** — chỉ làm nhiệm vụ đặt tên món ăn hấp dẫn, gợi ý phong cách nêm nếm gia vị và thời gian nấu từ các khối nguyên liệu chuẩn do Server tính sẵn.

---

## 1. User Review & Scope Alignment

> [!IMPORTANT]
> **Khẳng định kiến trúc cốt lõi**:
> 1. **Phạm vi tính năng**: Triển khai đầy đủ 3 Aggregates (`NutritionPlan`, `MealHistory`, `FoodItem`), `TDEECalculator`, `CombinatorialMatrix` (Server Rule Engine), `RecipeCacheRepository` ($0$ AI Cost cho lượt tái sử dụng), cơ chế chống lặp món Lockout (BR-NU-02), đề xuất sản phẩm NutiFood (BR-NU-03), tái hiệu chỉnh động khi tập luyện (BR-NU-04) / tủ lạnh (BR-NU-06), và Outbox CDC phát sự kiện lên Kafka.
> 2. **Cơ sở dữ liệu**: Sử dụng PostgreSQL schema riêng `nutrition` với các bảng `nutrition_plans`, `meal_histories`, `meal_logs`, `lockout_registries`, `food_items` (lưu các trường `calories_per_100g`, `protein_per_100g`, `carbs_per_100g`, `fat_per_100g`), `recipes` (Recipe Cache), bảng `outbox` & `outbox_log` (đã khởi tạo tại `01-init-schemas.sql`).
> 3. **Phân định ranh giới Server vs AI**: 
>    - **Server**: Quyết định lượng gram, món ăn, chỉ tiêu Calo/Macros, check lockout.
>    - **External AI**: Chỉ đóng vai trò Đầu bếp chế biến — đặt tên món, tinh chỉnh lượng gia vị/thời gian nấu khi tăng định lượng hoặc yêu cầu chế biến mới lạ.

---

## 2. Proposed Changes & File Structure

### Database & Migrations

#### [NEW] [07-create-nutrition-tables.sql](file:///e:/LEAN/TTTN/internal/shared/database/migrations/07-create-nutrition-tables.sql)
- Tạo các bảng nghiệp vụ trong PostgreSQL schema `nutrition` (schema `nutrition`, `outbox` và `outbox_log` đã được tạo từ `01-init-schemas.sql`):
  - `nutrition.nutrition_plans`: Lưu kế hoạch calo/macros theo ngày, target calories, phân bổ protein/carb/fat, danh sách bữa ăn gợi ý dưới dạng JSONB.
  - `nutrition.meal_histories`: Quản lý lịch sử ăn uống của user.
  - `nutrition.meal_logs`: Các món ăn thực tế đã ghi nhận (`meal_log_id`, `meal_name`, `meal_type`, `calories`, `protein`, `carbs`, `fat`, `logged_at`).
  - `nutrition.lockout_registries`: Lưu vết khóa nguyên liệu (Protein khóa 7 ngày, Carb khóa 5 ngày, Dish Theme khóa 3 ngày - BR-NU-02).
  - `nutrition.food_items`: Danh mục thực phẩm chuẩn per 100g (`calories_per_100g`, `protein_per_100g`, `carbs_per_100g`, `fat_per_100g`), nhãn dị ứng/chay/Halal, nhãn sản phẩm NutiFood, trạng thái vòng đời (`Draft`, `PendingApproval`, `Active`).
  - `nutrition.recipes`: Thư viện Recipe Cache tự động tích lũy từ External AI để tái sử dụng $0$ AI Cost.
- Seed dữ liệu ban đầu duy nhất cho bảng `nutrition.food_items` (danh mục thực phẩm chuẩn & sản phẩm NutiFood như NutiFood Varna, NutiFood LeanMax).

---

### Domain Layer (`internal/nutrition/domain`)

Không import thư viện ngoài (không GORM, không Gin/REST, không ORM tags). Định nghĩa đầy đủ Aggregate Roots, Entities, Value Objects, Domain Events và Interfaces (Ports).

#### [NEW] [calorie_allocation.go](file:///e:/LEAN/TTTN/internal/nutrition/domain/calorie_allocation.go)
- **Value Object**: `CalorieAllocation` (Target calories, Protein grams, Carb grams, Fat grams).
- **Invariant**: Target calories >= 1200 kcal/ngày (BR-NU-01).

#### [NEW] [lockout_registry.go](file:///e:/LEAN/TTTN/internal/nutrition/domain/lockout_registry.go)
- **Value Object**: `LockoutRegistry` quản lý danh sách nguyên liệu/loại món bị khóa kèm thời hạn mở khóa.
- **BR-NU-02**: Tự động khóa Protein chính 7 ngày, Carb chính 5 ngày, Chủ đề món 3 ngày sau khi log bữa ăn.
- **BR-NU-02.1**: Cấm tuyệt đối nguyên liệu bị khóa nếu không có input tủ lạnh.
- **BR-NU-02.2**: Nếu nguyên liệu tủ lạnh trùng Lockout -> Cho dùng nhưng yêu cầu AI đổi phương pháp chế biến mới lạ.

#### [NEW] [food_nutrient.go](file:///e:/LEAN/TTTN/internal/nutrition/domain/food_nutrient.go)
- **Value Object**: `FoodNutrient` chứa calo, macro per 100g (`CaloriesPer100g`, `ProteinPer100g`, `CarbsPer100g`, `FatPer100g`), allergen tags, partner tags (NutiFood).

#### [NEW] [nutrition_plan.go](file:///e:/LEAN/TTTN/internal/nutrition/domain/nutrition_plan.go)
- **Aggregate Root**: `NutritionPlan`
- Chứa `PlanID`, `UserID`, `Date`, `CalorieAllocation`, `DailyMealOptions` (Sáng, Trưa, Tối, Phụ).
- **BR-NU-05 (Chuyển đổi Thực đơn Kế hoạch -> Lịch sử Ăn uống & Nhắc nhở theo bữa)**:
  - **BR-NU-05.1**: Sinh ít nhất 3 Lựa chọn (`MealOption`) cho mỗi bữa ăn.
  - **BR-NU-05.2**: Nhắc nhở đẩy sau mỗi bữa ăn (8:30, 13:00, 19:30, 21:00).
  - **BR-NU-05.3**: Quick-log 1-tap 1/3 options | Custom log có calo | Custom log KHÔNG calo -> AI tự động ước tính & cache DB.

#### [NEW] [meal_history.go](file:///e:/LEAN/TTTN/internal/nutrition/domain/meal_history.go)
- **Aggregate Root**: `MealHistory`
- Chứa `HistoryID`, `UserID`, danh sách `MealLog` entities và `LockoutRegistry`.
- Business Method: `LogMeal(...)`, `LogMealFromPlan(...)`, `LogCustomMealWithAIEstimation(...)`, `ApplyLockout(...)`.

#### [NEW] [food_item.go](file:///e:/LEAN/TTTN/internal/nutrition/domain/food_item.go)
- **Aggregate Root**: `FoodItem`
- State Lifecycle: `Draft` -> `SubmitForApproval()` -> `Approve()` (`Active`) / `Reject()`.

#### [NEW] [tdee_calculator.go](file:///e:/LEAN/TTTN/internal/nutrition/domain/tdee_calculator.go)
- **Domain Service**: Tính TDEE theo Mifflin-St Jeor và tự động điều chỉnh Calo/Macro linh hoạt.
- **BR-NU-04**: Tái hiệu chỉnh thực đơn động khi có sự kiện tập luyện `WorkoutSessionCompleted`.

#### [NEW] [combinatorial_matrix.go](file:///e:/LEAN/TTTN/internal/nutrition/domain/combinatorial_matrix.go)
- **Domain Service (Rule-Based Matrix Engine)**: Thuật toán ma trận tổ hợp nguyên liệu ($N_1 \text{ Protein} \times N_2 \text{ Carb} \times N_3 \text{ Veggie} \times N_4 \text{ Style} = 180,000+$ món) + Tự động căn chỉnh gram nguyên liệu chuẩn theo công thức:
  $$\Delta \text{Gram} = \frac{\Delta \text{Calo}}{\text{CaloriesPer100g}} \times 100$$

#### [NEW] [menu_generator.go](file:///e:/LEAN/TTTN/internal/nutrition/domain/menu_generator.go)
- **Domain Service Orchestrator**: 
  1. Sử dụng `CombinatorialMatrix` trên Server để sinh tổ hợp nguyên liệu chuẩn Calo/Macros.
  2. Tra cứu `RecipeCacheRepository`: Nếu đã có công thức -> Lấy DB ($0$ AI Cost).
  3. Nếu chưa có -> Gọi `AIService` làm **Đầu bếp chế biến** (đặt tên món & hướng dẫn chế biến từ nguyên liệu chuẩn của Server).
  4. **Quy tắc Bổ Sung Nguyên Liệu Phụ & Tự Động Đồng Bộ CSDL (Auto DB Catalog Sync)**:
     - Prompt cho External AI cho phép AI **tự do bổ sung thêm nguyên liệu/gia vị phụ trợ bên ngoài** (như tỏi, ớt, dầu oliu, nấm, sốt...) để món ăn chế biến tròn vị.
     - Khi AI trả về công thức có chứa nguyên liệu phụ mới chưa có trong CSDL `nutrition.food_items`, Server **tự động chèn nguyên liệu mới này vào CSDL `nutrition.food_items`** (với thông số dinh dưỡng per 100g do AI ước tính) để tái sử dụng cho tất cả người dùng khác lần sau.
     - Lưu toàn bộ công thức vào `nutrition.recipes` (Recipe Cache DB).

#### [NEW] [ai_service.go](file:///e:/LEAN/TTTN/internal/nutrition/domain/ai_service.go)
- **Port / Interface**: `AIService` hợp đồng gọi External AI làm **Creative Chef (Đầu bếp chế biến & nêm nếm)** và **Nutrient Estimator**, tích hợp Prompt Rule cho phép bổ sung nguyên liệu phụ trợ và trích xuất nguyên liệu mới để đồng bộ CSDL.

#### [NEW] [events.go](file:///e:/LEAN/TTTN/internal/nutrition/domain/events.go)
- Domain Events: `NutritionPlanGenerated`, `NutritionPlanRecalibrated`, `MealLogged`, `LockoutApplied`, `FoodItemCreated`, `FoodItemApproved`.

#### [NEW] [repository.go](file:///e:/LEAN/TTTN/internal/nutrition/domain/repository.go)
- Interfaces: `NutritionPlanRepository`, `MealHistoryRepository`, `FoodItemRepository`, `RecipeCacheRepository`.

---

### Application Layer (`internal/nutrition/application`)

#### Commands:
- [NEW] [generate_daily_plan.go](file:///e:/LEAN/TTTN/internal/nutrition/application/command/generate_daily_plan.go): UC-05.1 Base TDEE plan generation.
- [NEW] [recalibrate_plan_with_pantry.go](file:///e:/LEAN/TTTN/internal/nutrition/application/command/recalibrate_plan_with_pantry.go): **BR-NU-06** API tái hiệu chỉnh bữa ăn theo nguyên liệu tủ lạnh.
- [NEW] [log_meal.go](file:///e:/LEAN/TTTN/internal/nutrition/application/command/log_meal.go): UC-05.2 & **BR-NU-05** Log meal (1-tap quick log / custom log với AI auto estimation).
- [NEW] [create_food_item.go](file:///e:/LEAN/TTTN/internal/nutrition/application/command/create_food_item.go)
- [NEW] [approve_food_item.go](file:///e:/LEAN/TTTN/internal/nutrition/application/command/approve_food_item.go)

#### Queries:
- [NEW] [get_today_menu.go](file:///e:/LEAN/TTTN/internal/nutrition/application/query/get_today_menu.go)
- [NEW] [get_nutrition_history.go](file:///e:/LEAN/TTTN/internal/nutrition/application/query/get_nutrition_history.go)
- [NEW] [get_nutrition_summary.go](file:///e:/LEAN/TTTN/internal/nutrition/application/query/get_nutrition_summary.go)

---

### Infrastructure Layer (`internal/nutrition/infrastructure`)

#### AI Integration (Google ADK v2 & FallbackLLM) & Persistence:
- [NEW] [agent.go](file:///e:/LEAN/TTTN/internal/nutrition/infrastructure/ai/adk/agent.go): `ADKNutritionAgent` triển khai `repository.AIService`.
- [NEW] [fallback_llm.go](file:///e:/LEAN/TTTN/internal/nutrition/infrastructure/ai/adk/fallback_llm.go): `FallbackLLM` bọc `model.LLM`, tự động failover sang model dự phòng khi hết quota/429 (`GEMINI_MODEL` & `GEMINI_FALLBACK_MODELS`).
- [NEW] [build_agents.go](file:///e:/LEAN/TTTN/internal/nutrition/infrastructure/ai/adk/build_agents.go): Khởi tạo LLM nodes & workflow agents với file prompt nhúng tĩnh (`//go:embed prompts/*.txt`).
- [NEW] [tools.go](file:///e:/LEAN/TTTN/internal/nutrition/infrastructure/ai/adk/tools.go): ADK Function Tools (`fetch_food_catalog`, `calculate_macro_gram`, `suggest_nutifood_supplement`).
- [NEW] [retry.go](file:///e:/LEAN/TTTN/internal/nutrition/infrastructure/ai/adk/retry.go): Retry mechanism & plan validation.
- [NEW] [gorm_models.go](file:///e:/LEAN/TTTN/internal/nutrition/infrastructure/persistence/gorm_models.go): Mapping `ToDomain()` / `ToPersistence()`.
- [NEW] [postgres_repository.go](file:///e:/LEAN/TTTN/internal/nutrition/infrastructure/persistence/postgres_repository.go)
- [NEW] [outbox_repository.go](file:///e:/LEAN/TTTN/internal/nutrition/infrastructure/persistence/outbox_repository.go)
- [NEW] [grpc_handler.go](file:///e:/LEAN/TTTN/internal/nutrition/infrastructure/transport/grpc_handler.go): gRPC Adapter `NutritionServiceServer`.

#### Workers:
- [NEW] [outbox_worker.go](file:///e:/LEAN/TTTN/internal/nutrition/infrastructure/worker/outbox_worker.go): CDC Outbox Worker sang Kafka.
- [NEW] [daily_menu_cron_worker.go](file:///e:/LEAN/TTTN/internal/nutrition/infrastructure/worker/daily_menu_cron_worker.go): Cron Job 5:00 AM (`0 5 * * *`) pre-generate Base TDEE plan.

---

## 3. Workflows & Activity Diagrams (7 Sơ Đồ Mermaid Chuẩn AI Creative Chef)

### Flow 1: Cron Job 5:00 AM Tự Động Sinh Thực Đơn Đầu Ngày (`DailyMenuCronWorker` & `GetTodayMenu`)

```mermaid
sequenceDiagram
    autonumber
    participant Cron as Cron Scheduler (5:00 AM)
    participant Worker as DailyMenuCronWorker
    participant ProfileClient as Profile Module (BiologicalMetrics)
    participant ServerEngine as Server Rule Engine (CombinatorialMatrix)
    participant Cache as Recipe Cache DB (nutrition.recipes)
    participant CreativeAI as External AI (Creative Chef)
    participant Repo as NutritionPlanRepository
    actor User
    participant Client as Mobile App
    participant gRPC as Nutrition gRPC Server

    rect rgb(240, 248, 255)
        Note over Cron,Repo: Tự động chạy lúc 5:00 sáng hàng ngày (0 5 * * *)
        Cron->>Worker: Trigger 5:00 AM Daily Job
        Worker->>Repo: GetActiveUsersWithoutTodayPlan(today)
        Repo-->>Worker: List UserIDs cần sinh thực đơn
        loop Mỗi Active User
            Worker->>ProfileClient: GetBiologicalMetrics & FoodRestrictions(userID)
            ProfileClient-->>Worker: Cân nặng, Chiều cao, Dị ứng, Mục tiêu
            Worker->>ServerEngine: CalculateBaseTDEE & GenerateIngredientMatrix(metrics, lockouts)
            Note over ServerEngine: 1. Server tính Base TDEE (>=1200 kcal - BR-NU-01)<br/>2. Server ghép ma trận P + C + V + S<br/>3. Server tính chính xác gram từng nguyên liệu
            ServerEngine->>Cache: LookupRecipeByMatrixKey(ingredientHash)
            alt HIT CACHE (Đã có công thức chế biến)
                Cache-->>ServerEngine: Return Recipe Name & Cooking Steps ($0 AI Cost)
            else MISS CACHE (Chưa có công thức)
                ServerEngine->>CreativeAI: GenerateRecipeDetails(ingredientGrams, cookingStyle)
                Note over CreativeAI: AI đóng vai Đầu bếp chế biến:<br/>- Đặt tên món ăn hấp dẫn<br/>- Nêm nếm gia vị & các bước nấu 
                CreativeAI-->>ServerEngine: Recipe Name & Cooking Steps
                ServerEngine->>Cache: SaveNewRecipeToDB(ingredientHash, recipe)
            end
            ServerEngine-->>Worker: Complete Daily Meals (3 Options Per Meal)
            Worker->>Repo: Save(NutritionPlan) [In Transaction]
        end
    end

    rect rgb(255, 250, 240)
        Note over User,gRPC: Người dùng mở app trong ngày (Ví dụ: 7:00 AM)
        User->>Client: Mở tab Dinh Dưỡng Hôm Nay
        Client->>gRPC: GetTodayMenu(user_id)
        gRPC->>Repo: FindByUserIDAndDate(userID, today)
        Repo-->>gRPC: Return Pre-generated NutritionPlan (Phản hồi siêu tốc <20ms!)
        gRPC-->>Client: Trả về thực đơn đầy đủ (3 Options mỗi bữa + Sản phẩm NutiFood)
    end
```

---

### Flow 2: Log Bữa Ăn Theo Nhắc Nhở & AI Tự Động Tính Calo (`LogMeal` & `BR-NU-05`)

```mermaid
sequenceDiagram
    autonumber
    actor User
    participant Client as Mobile App / Push Notification
    participant gRPC as Nutrition gRPC Server
    participant Handler as LogMeal Command Handler
    participant PlanRepo as NutritionPlanRepository
    participant CreativeAI as External AI (Nutrient Estimator)
    participant Cache as Nutrient Cache DB (nutrition.food_items)
    participant HistoryRepo as MealHistoryRepository
    participant Domain as MealHistory Aggregate & LockoutRegistry

    Note over User,Client: Nhắc nhở sau mỗi bữa ăn (Post-Meal Push Notification: 8:30, 13:00, 19:30, 21:00)

    alt Nhánh A: Chọn 1 trong 3 Món Gợi Ý từ Thực Đơn (1-Tap Quick Log)
        User->>Client: Chọn 1 trong 3 món gợi ý (Option A/B/C) -> Bấm "Đã ăn"
        Client->>gRPC: LogMeal(user_id, planned_meal_id, meal_type)
        gRPC->>Handler: Execute(LogMealFromPlanCommand)
        Handler->>PlanRepo: GetMealOption(planned_meal_id)
        PlanRepo-->>Handler: MealOption (Tên món, Calo, Macros từ Kế hoạch)
    else Nhánh B: Ăn Món Khác + Nhập Tên + Có Nhập Calo/Macros
        User->>Client: Nhập tên món + Số lượng + Nhập số Calo/Macros
        Client->>gRPC: LogMeal(user_id, meal_name, calories, macros)
        gRPC->>Handler: Execute(LogMealCommand)
    else Nhánh C: Ăn Món Khác + Nhập Tên + KHÔNG Nhập Calo (AI Tự Động Ước Tính)
        User->>Client: Nhập tên món + Số lượng (VD: "1 tô bún bò huế") -> Để trống Calo
        Client->>gRPC: LogMeal(user_id, meal_name, portion, calories: 0)
        gRPC->>Handler: Execute(LogMealWithAIEstimationCommand)
        Handler->>Cache: LookupNutrientCache(meal_name, portion)
        alt HIT NUTRIENT CACHE
            Cache-->>Handler: Return Calories/Macros ($0 AI / <5ms)
        else MISS NUTRIENT CACHE
            Handler->>CreativeAI: EstimateNutrient(meal_name, portion)
            CreativeAI-->>Handler: Estimated Calories, Protein, Carb, Fat
            Handler->>Cache: SaveNutrientCache(meal_name, portion, nutrients)
        end
    end

    Handler->>HistoryRepo: GetOrCreate(userID)
    HistoryRepo-->>Handler: MealHistory Aggregate
    Handler->>Domain: LogMeal(mealLog)
    Handler->>Domain: ApplyLockout(proteinSource: 7d, carbSource: 5d, category: 3d) (BR-NU-02)
    Handler->>HistoryRepo: Save(MealHistory) [Transaction]
    gRPC-->>Client: Trả về kết quả thành công, cập nhật Vòng Calo & Đánh dấu đã ăn bữa này
```

---

### Flow 3: Tổng Quan Calo/Macro & Lịch Sử (`GetSummary` & `GetHistory`)

```mermaid
sequenceDiagram
    autonumber
    actor User
    participant Client as Mobile App
    participant gRPC as Nutrition gRPC Server
    participant QueryHandler as NutritionQueryHandler
    participant PlanRepo as NutritionPlanRepository
    participant HistoryRepo as MealHistoryRepository

    par Lấy Tổng Quan Hôm Nay
        Client->>gRPC: GetNutritionSummary(user_id)
        gRPC->>QueryHandler: GetSummary(userID, today)
        QueryHandler->>PlanRepo: GetTargetAllocation(userID, today)
        QueryHandler->>HistoryRepo: GetTodayConsumedCaloriesAndMacros(userID, today)
        QueryHandler-->>gRPC: Target Calories/Macros vs Consumed Calories/Macros
        gRPC-->>Client: GetNutritionSummaryResponse (Progress Ring UI)
    and Lấy Lịch Sử Ghi Bữa Ăn
        Client->>gRPC: GetNutritionHistory(user_id, start_date, end_date)
        gRPC->>QueryHandler: GetHistory(userID, startDate, endDate)
        QueryHandler->>HistoryRepo: FindMealLogsByDateRange(userID, startDate, endDate)
        QueryHandler-->>gRPC: Danh sách MealLogItems
        gRPC-->>Client: GetNutritionHistoryResponse
    end
```

---

### Flow 4: Vòng Đời Phê Duyệt Thực Phẩm (`FoodItem` Lifecycle)

```mermaid
stateDiagram-v2
    [*] --> Draft: Partner / Admin tạo mới món ăn (CreateFoodItem)
    Draft --> PendingApproval: SubmitForApproval() (Chờ Admin duyệt)
    PendingApproval --> Active: ApproveFoodItem() (Admin chấp nhận -> Sẵn sàng hiển thị trong Search)
    PendingApproval --> Rejected: RejectFoodItem() (Admin từ chối kèm lý do)
    Rejected --> Draft: Sửa đổi thông tin dinh dưỡng
    Active --> Archived: ArchiveFoodItem() (Ngưng kinh doanh / lỗi)
```

---

### Flow 5: Outbox CDC & Phát Sự Kiện Sang Kafka

```mermaid
sequenceDiagram
    autonumber
    participant DB as Postgres (nutrition.outbox)
    participant Worker as Outbox Polling Worker
    participant Kafka as Aiven Kafka Broker
    participant Consumer as Analytics / Notification Modules

    loop Polling Mới Bút Toán Mới (Mỗi 1s)
        Worker->>DB: SELECT * FROM nutrition.outbox WHERE published = false ORDER BY created_at LIMIT 50 FOR UPDATE SKIP LOCKED
        DB-->>Worker: List Outbox Events (MealLogged, LockoutApplied)
        alt Có Events
            Worker->>Kafka: Publish Event to Topic "contracts.core.nutrition.v1.mealLogged" (Partition Key = UserID)
            Kafka-->>Worker: ACK Success
            Worker->>DB: UPDATE nutrition.outbox SET published = true, published_at = NOW(), status = 'PUBLISHED'
        end
    end
    Kafka->>Consumer: Consume MealLogged Event
```

---

### Flow 6: Tái Hiệu Chỉnh Định Lượng Khi Tập Luyện Chiều (`WorkoutSessionCompleted` Event - BR-NU-04)

```mermaid
sequenceDiagram
    autonumber
    participant Kafka as Aiven Kafka Broker
    participant Consumer as Nutrition Workout Event Consumer
    participant PlanRepo as NutritionPlanRepository
    participant HistoryRepo as MealHistoryRepository
    participant ServerEngine as Server Rule Engine (DeltaScaling)
    participant CreativeAI as External AI (Creative Chef)
    participant Outbox as OutboxRepository (DB)

    Kafka->>Consumer: Event "WorkoutSessionCompleted" (userID, burnedCalories: 450 kcal)
    Consumer->>PlanRepo: FindByUserIDAndDate(userID, today)
    alt Đã có NutritionPlan hôm nay
        Consumer->>HistoryRepo: GetTodayLoggedMeals(userID, today)
        HistoryRepo-->>Consumer: List Bữa Ăn Đã Log (VD: Bữa Sáng, Bữa Trưa)
        
        Consumer->>ServerEngine: CalculateDeltaSurplus(existingAllocation, extraBurnedCalories: 450)
        Note over ServerEngine: 1. Server giữ nguyên bữa đã ăn/log<br/>2. Server tự động tính số gram cần thêm dựa trên DB per 100g:<br/>DeltaGram = (450 kcal / CaloriesPer100g) * 100
        
        alt Phương án 1: Server chèn Bữa Phụ Phục Hồi (0$ AI Cost)
            ServerEngine-->>Consumer: Insert Post-Workout Recovery Snack (Shake NutiFood / Trứng chuối)
        else Phương án 2: Tăng định lượng trực tiếp món chính Bữa Tối
            ServerEngine->>CreativeAI: AdjustRecipeCookingTimeAndSpices(dishName, oldGram: 130g, newGram: 220g)
            Note over CreativeAI: AI đóng vai Đầu bếp chế biến:<br/>Chỉ tinh chỉnh tỷ lệ gia vị sốt & thời gian áp chảo (VD: 5p -> 8p)
            CreativeAI-->>ServerEngine: Adjusted Cooking Steps & Spice Ratios
        end
        
        Consumer->>PlanRepo: UpdateRemainingMeals(NutritionPlan) [In Transaction]
        Consumer->>Outbox: Save(NutritionPlanRecalibrated Event) [In Transaction]
    end
```

---

### Flow 7: Tái Hiệu Chỉnh Thực Đơn Khi Nhập Nguyên Liệu Sẵn Có (`RecalibratePlanWithPantry` API - BR-NU-06)

```mermaid
sequenceDiagram
    autonumber
    actor User
    participant Client as Mobile App
    participant gRPC as Nutrition gRPC Server
    participant Handler as RecalibratePlanWithPantry Command
    participant HistoryRepo as MealHistoryRepository
    participant ServerEngine as Server Rule Engine (CombinatorialMatrix)
    participant CreativeAI as External AI (Creative Chef)
    participant PlanRepo as NutritionPlanRepository

    User->>Client: Nhập nguyên liệu sẵn có trong tủ lạnh (VD: trứng, ức gà...) -> Bấm "Cập nhật thực đơn"
    Client->>gRPC: RecalibratePlanWithPantry(user_id, available_ingredients[])
    gRPC->>Handler: Execute(RecalibratePlanWithPantryCommand)
    Handler->>PlanRepo: FindByUserIDAndDate(userID, today)
    Handler->>HistoryRepo: GetTodayLoggedMeals(userID, today) & GetActiveLockouts(userID)

    Handler->>ServerEngine: FilterAndMatchPantryIngredients(availableIngredients, lockouts, unconsumedMeals)
    Note over ServerEngine: 1. Server giữ nguyên bữa đã ăn/log<br/>2. Kiểm tra available_ingredients với LockoutRegistry:<br/>- Nếu trùng khóa -> Gán flag RequiresNovelCookingStyle (BR-NU-02.2)<br/>- Server ghép ma trận P + C + V + S cho các bữa chưa ăn
    
    ServerEngine->>CreativeAI: GenerateNovelRecipes(ingredientGrams, lockedMatchList)
    Note over CreativeAI: AI đóng vai Đầu bếp chế biến:<br/>Tạo hướng dẫn chế biến mới lạ cho nguyên liệu tủ lạnh trùng lockout
    CreativeAI-->>ServerEngine: Recipe Names & Cooking Steps

    ServerEngine-->>Handler: Updated Remaining Meals (3 Options Per Meal)
    Handler->>PlanRepo: UpdateRemainingMeals(NutritionPlan) [In Transaction]
    Handler-->>gRPC: RecalibratePlanWithPantryResponse (Success, Updated Plan)
    gRPC-->>Client: Trả về thực đơn các bữa còn lại đã được làm mới theo tủ lạnh!
```

---

## 4. Verification Plan

### Automated Unit Tests
1. **Domain Tests**:
   - `go test -v ./internal/nutrition/domain/...`
   - Test `CalorieAllocation`: Kiểm tra Invariant `target >= 1200 kcal` (BR-NU-01).
   - Test `CombinatorialMatrix`: Kiểm tra ghép tổ hợp ma trận $180k+$ món và tính gram chuẩn theo công thức $\Delta \text{Gram} = \frac{\Delta \text{Calo}}{\text{CaloriesPer100g}} \times 100$.
   - Test `LockoutRegistry`: Kiểm tra nguyên tắc khóa 7 ngày đạm, 5 ngày tinh bột, 3 ngày chủ đề món (BR-NU-02, BR-NU-02.1, BR-NU-02.2).
   - Test `MenuGenerator`: Tra cứu Recipe Cache HIT/MISS và đính kèm sản phẩm NutiFood (BR-NU-03).
2. **Application CQRS Tests**:
   - `go test -v ./internal/nutrition/application/...`
   - Mock repositories để kiểm tra logic `GenerateDailyPlan`, `RecalibratePlanWithPantry`, `LogMeal`.

### Automated Integration Tests
1. **Repository & Outbox Integration Tests**:
   - `go test -v ./internal/nutrition/tests/integration/...`
   - Chạy với Postgres container / test DB để kiểm tra transaction lưu Aggregate và Outbox event nguyên khối.

### Code Quality & Contract Validation
1. **Go Code Style Rules**: `golangci-lint run ./internal/nutrition/...`
2. **Buf Proto Schema**: `buf lint`
