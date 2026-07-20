# Agent Instructions

## 1. Agent Behavior
- **Think Before Coding**: State assumptions, ask if unsure, and surface tradeoffs. Don't hide confusion. If a requirement is unclear, or if you must make any assumptions, you MUST stop and ask for explicit confirmation from the user before writing any code or continuing.
- **Simplicity First**: Write the minimum code required. No speculative features, abstractions, or "flexibility" not requested.
- **Surgical Edits**: Change only what is necessary. Match existing style. Cleanup only unused code created by your changes.
- **Verify Before Returning**: Confirm your output matches the original request. If tests exist, run them. If not, trace the logic mentally.

## 2. Knowledge & Rule Management
- **Knowledge Persistence**:
  - Propose Updates: If discovering undocumented project conventions or patterns, propose adding them to `AGENTS.md`.
  - Report Conflicts: If `AGENTS.md` contradicts the codebase, report it to the Developer.
  - CONSTRAINT: NEVER create or modify `AGENTS.md` without explicit Developer approval.
- **Proactive Refactoring**:
  - If you identify code smells, architectural coupling, or opportunities for design improvement, you must propose the refactoring to the Developer. Do not execute refactoring without explicit approval.

## 3. Project Conventions & Architecture
- **Contracts Folder as the SSOT**:
  - The `contracts/` folder is the Single Source of Truth (SSOT) for the entire application interface. All gRPC services, REST gateway mappings, and OpenAPI documentation are generated and derived directly from the schemas defined here. Implementing manual HTTP routing, REST controller mappings, or writing manual OpenAPI specs is prohibited.
  - API security requirements and authentication bypasses must be declared directly in the proto contract using gRPC-Gateway OpenAPI annotations (referencing `"BearerAuth"`). Manual bypass lists or hardcoded endpoint arrays in Go interceptors are prohibited.
- **Domain-Encapsulated Modular Monolith**:
  - Code under `internal/` must be structured into self-contained, high-cohesion business modules. Each module is responsible for encapsulating its own data schemas, data access layers, and business logic, minimizing cross-module coupling.
  - Dependencies must point inward: Domain must not depend on infrastructure. Application defines interfaces, not implementations.
- **Event-Driven Standards (CloudEvents)**:
  - If event-driven messaging is introduced, all event envelopes must adhere strictly to the **CloudEvents** specification. The `data` payload of these events must follow the camelCase JSON mapping standard, while envelope-level extension attributes must remain all-lowercase.
- **Pre-Commit Quality Assurance**:
  - Before committing any changes to version control, all code validation procedures—including styling/formatting verification, API contract/schema linting, and full test suite execution—must be executed and pass successfully to ensure codebase integrity and correctness.
- **Test-Driven & Verifiable**:
  - Every new feature must include:
    - Unit Tests covering the Domain and Application layers.
    - Integration Tests covering the Infrastructure layer.
  - All test suites must be fully automated, executable, and verifiable to confirm implementation correctness.
  - Always maintain or improve statement coverage (recommended targets: >80% for core business logic, >90% for critical middleware/helpers). Never regress test coverage when modifying or refactoring code.
- **Go Coding Rules**:
  - **Go Style Rules**: Tuân thủ nghiêm ngặt chuẩn lập trình Go theo skill **go-style-rules** (.agents/skills/go-style-rules/SKILL.md) khi viết, sửa đổi, hoặc review code Go.
  - Kiến trúc: **Hexagonal (Ports & Adapters)**. Mỗi Bounded Context = 1 module tại `/internal/<module_name>/`.
  - Cấu trúc thư mục chuẩn mỗi module: `/internal/<module>/` gồm `domain/` (aggregate, entity, value_object, event, repository, service), `application/` (commands, queries), `infrastructure/` (persistence, ai, transport).
  - Domain layer không import thư viện ngoài (không GORM, không Gin, không ORM tag). Domain structs không chứa JSON/DB tag; mapping do Infrastructure qua `ToDomain()` / `ToPersistence()`.
  - Mọi thay đổi trạng thái Aggregate phải qua method nghiệp vụ rõ tên (`Activate()`, `AddSet()`).
  - Interface (Port) định nghĩa ở nơi **sử dụng** (Domain/Application), không ở nơi triển khai (Infrastructure). Dependency Injection qua constructor.
  - Không bỏ qua lỗi: luôn `if err != nil`, wrap lỗi bằng `fmt.Errorf("context: %w", err)`. Goroutine ngầm bắt buộc dùng `context.Context`.
  - Không `AutoMigrate` GORM ở production; dùng migration SQL đánh số version. Các thao tác ghi phối hợp nhiều Aggregate chạy chung 1 transaction với `context.Context`.
- **API & Protobuf Convention**:
  - Phương pháp: **Contract-First** — `.proto` là nguồn sự thật duy nhất cho mọi API và Event. Quản lý bằng **Buf CLI**: lint + breaking change detection.
  - Sinh mã 1 lần tại `/proto`: Go stubs → `/internal/gen/go/contracts/`, OpenAPI → `/docs/swagger/contracts/`.
  - Tên event type: `contracts.<domain_type>.<service_name>.<version>.<eventName>` (camelCase). JSON field dùng **camelCase**. REST URL dùng danh từ số nhiều: `POST /api/v1/users/{userId}/profile`.
  - Event envelope bắt buộc: `specversion`, `id`, `source`, `type`, `time`, `datacontenttype`, `data`. Event payload tối thiểu.
- **Database & Persistence Convention**:
  - **Database Persistence**: PostgreSQL (ACID, raw coordinates JSONB, sessions, lockout cache, rate limit).
  - Schema isolation: mỗi module có PostgreSQL schema riêng. Cấm `JOIN` chéo schema.
  - **Outbox Pattern** bắt buộc: lưu event vào `outbox_events` trong cùng transaction, CDC/publisher đẩy sang Kafka sau (partition key = `userId`).
  - Rate limit: 100 req/phút Onboarding API, 10 req/phút CompleteSession API.
