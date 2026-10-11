# Danh Sách Nhiệm Vụ Kỹ Thuật (TDD Tasks): Tiền Tệ Ảo FitCoins & Sổ Cái Bất Biến

## Quy Ước Triển Khai
- Áp dụng nghiêm ngặt chu trình **TDD (RED $\rightarrow$ GREEN $\rightarrow$ REFACTOR)**.
- Mọi logic nghiệp vụ tại tầng Domain & Application phải đạt độ bao phủ kiểm thử (Test Coverage) $\ge 90\%$.
- Kiểm tra tính toàn vẹn và thực thi lệnh test sau mỗi task.

---

## Danh Sách Task Chi Tiết

### [T-FC-01] [CONTRACT] Hợp Đồng API Protobuf & Sinh Mã Go Stubs
- **Mục tiêu**: Định nghĩa API ConnectRPC `WalletService` theo chuẩn Contract-First.
- **Tệp nguồn**:
  - `contracts/gamification/wallet/v1/wallet.proto`
  - `proto/contracts/supporting/gamification/v1/event/wallet_events.proto`
- **Các bước thực hiện**:
  1. Khai báo dịch vụ `WalletService` với các RPCs: `GetWallet`, `ListTransactions`, `SpendCoins`.
  2. Khai báo các message: `GetWalletRequest`, `GetWalletResponse`, `SpendCoinsRequest`, `SpendCoinsResponse`, `LedgerEntry`.
  3. Khai báo các CloudEvents message: `CoinsEarned`, `CoinsSpent`.
  4. Chạy lệnh sinh mã: `buf generate`.
- **Tiêu chí hoàn thành (DoD)**:
  - `buf lint` không có lỗi.
  - File Go stubs sinh ra thành công tại `internal/gen/go/contracts/`.

---

### [T-FC-02] [MIGRATION] Migration Cơ Sở Dữ Liệu Ví Tiền & Sổ Cái Bất Biến
- **Mục tiêu**: Tạo các bảng `user_wallet`, `coin_ledger` với đầy đủ ràng buộc CHECK và khóa duy nhất.
- **Tệp nguồn**:
  - `internal/gamification/infrastructure/persistence/migrations/`
- **Các bước thực hiện**:
  1. Viết migration SQL tạo bảng `gamification.user_wallet` kèm `CONSTRAINT chk_user_wallet_balance_non_negative CHECK (balance >= 0)`.
  2. Viết migration SQL tạo bảng `gamification.coin_ledger` kèm:
     - `CONSTRAINT chk_coin_ledger_amount_nonzero CHECK (amount <> 0)`.
     - `CONSTRAINT chk_coin_ledger_balance_after_non_negative CHECK (balance_after >= 0)`.
     - `CONSTRAINT uq_coin_ledger_idempotency_key UNIQUE (idempotency_key)`.
  3. Thêm chỉ mục `idx_coin_ledger_user_created ON gamification.coin_ledger (user_id, created_at DESC)`.
- **Tiêu chí hoàn thành (DoD)**:
  - Chạy migration SQL thành công vào cơ sở dữ liệu.

---

### [T-FC-03] [DOMAIN] Định Nghĩa Value Objects & Entity CoinLedgerEntry
- **Mục tiêu**: Xây dựng các thành phần nền tảng của tầng Domain.
- **Tệp nguồn**:
  - `internal/gamification/domain/vo/coin_transaction_reason.go`
  - `internal/gamification/domain/entity/coin_ledger_entry.go`
- **Các bước thực hiện**:
  1. Khai báo các hằng số enum cho `CoinTransactionReason` (`EARN_LEVEL_UP`, `EARN_WORKOUT`, `EARN_PR`, `EARN_NUTRITION`, `EARN_STREAK_MILESTONE`, `EARN_ONBOARDING`, `SPEND_GENERIC`).
  2. Khai báo Entity `CoinLedgerEntry` với hàm khởi tạo `NewCoinLedgerEntry` kiểm tra tính hợp lệ (`amount != 0`, `balance_after >= 0`, `idempotency_key != ""`).
- **Tiêu chí hoàn thành (DoD)**:
  - Code compile sạch, không import bất kỳ package bên ngoài nào.

---

### [T-FC-04] [RED] Viết Unit Tests Cho UserWallet Aggregate
- **Mục tiêu**: Thiết lập các kịch bản kiểm thử tự động thất bại trước khi viết code logic ví (Chu trình RED).
- **Tệp nguồn**:
  - `internal/gamification/domain/aggregate/user_wallet_test.go`
- **Các kịch bản kiểm thử**:
  1. `TestUserWallet_EarnFromLevelUp`: Thưởng cấp thường $+10$, mốc tròn chục $+50$, mốc 100 $+200$.
  2. `TestUserWallet_EarnFromWorkout_DailyCap`: Buổi tập thứ 1 trong ngày được $+5$; buổi thứ 2 cùng ngày trả về `ErrDailyCapExceeded`; sang ngày hôm sau được $+5$.
  3. `TestUserWallet_EarnFromPR_DailyCap`: Phá PR lần 1 trong ngày được $+5$; lần 2 cùng ngày trả về `ErrDailyCapExceeded`.
  4. `TestUserWallet_EarnFromNutrition_DailyCap`: Đạt dinh dưỡng lần 1 trong ngày được $+3$; lần 2 cùng ngày trả về `ErrDailyCapExceeded`.
  5. `TestUserWallet_EarnFromStreakMilestone`: Đúng mức thưởng cho các mốc 7, 30, 100, 365 ngày.
  6. `TestUserWallet_Spend_Success`: Số dư $100$, chi tiêu $60 \rightarrow$ số dư còn $40$.
  7. `TestUserWallet_Spend_InsufficientBalance`: Số dư $40$, chi tiêu $50 \rightarrow$ báo lỗi `ErrInsufficientBalance`, số dư không đổi.
  8. `TestUserWallet_Spend_NegativeAmount`: Chi tiêu số âm $\rightarrow$ báo lỗi `ErrNegativeSpendAmount`.
- **Tiêu chí hoàn thành (DoD)**:
  - Chạy `go test ./internal/gamification/domain/aggregate/...` báo failing test như dự kiến.

---

### [T-FC-05] [GREEN] Triển Khai Logic UserWallet Aggregate
- **Mục tiêu**: Viết code tối giản vừa đủ để toàn bộ test trong `T-FC-04` chuyển sang màu xanh (Chu trình GREEN).
- **Tệp nguồn**:
  - `internal/gamification/domain/aggregate/user_wallet.go`
- **Các bước thực hiện**:
  1. Hiện thực hóa các method của `UserWallet`: `EarnFromLevelUp`, `EarnFromWorkout`, `EarnFromPR`, `EarnFromNutrition`, `EarnFromStreakMilestone`, `EarnFromOnboarding`, `Spend`.
  2. Xử lý so sánh ngày địa phương `isSameDate` để khống chế Daily Caps.
- **Tiêu chí hoàn thành (DoD)**:
  - Toàn bộ unit tests trong `user_wallet_test.go` pass $100\%$.
  - Test coverage đạt $\ge 95\%$.

---

### [T-FC-06] [APP] Triển Khai Tầng Application & Command Handlers
- **Mục tiêu**: Xây dựng Use Cases nạp và trừ coin có bảo vệ Idempotency và ghi nhận sự kiện Outbox trong cùng 1 transaction.
- **Tệp nguồn**:
  - `internal/gamification/application/port/wallet_repository.go`
  - `internal/gamification/application/port/outbox_repository.go`
  - `internal/gamification/application/command/earn_coins.go`
  - `internal/gamification/application/command/spend_coins.go`
  - `internal/gamification/application/query/get_wallet_balance.go`
  - `internal/gamification/application/query/list_ledger_transactions.go`
- **Các kịch bản kiểm thử**:
  - `spend_coins_test.go`: Test trùng lặp `idempotency_key` trả về kết quả cũ mà không trừ tiền lần 2; test rollback khi lưu lỗi; test ghi bản ghi `CoinsSpent` vào Outbox.
  - `earn_coins_test.go`: Test ghi bản ghi `CoinsEarned` vào Outbox khi cộng coin thành công.
- **Tiêu chí hoàn thành (DoD)**:
  - Unit tests cho Command Handlers pass $100\%$.

---

### [T-FC-07] [INFRA] Hiện Thực Hóa PostgresWalletRepository & Integration Tests
- **Mục tiêu**: Triển khai tầng lưu trữ với khóa dòng bi quan `SELECT ... FOR UPDATE` và kiểm thử tương tranh trên PostgreSQL thật.
- **Tệp nguồn**:
  - `internal/gamification/infrastructure/persistence/wallet_repository.go`
  - `internal/gamification/infrastructure/persistence/wallet_repository_test.go`
- **Các kịch bản kiểm thử tương tranh**:
  1. Kiểm tra 2 goroutines cùng trừ tiền trên tài khoản 100 coin: Đúng 1 bên thành công, bên kia bị từ chối; số dư cuối cùng là 40 coin (Zero Double Spending).
  2. Kiểm tra ràng buộc DB: `CHECK (balance >= 0)`.
- **Tiêu chí hoàn thành (DoD)**:
  - Toàn bộ integration test chạy thành công trên PostgreSQL.

---

### [T-FC-08] [TRANSPORT] Triển Khai ConnectRPC Server & Kafka Consumers
- **Mục tiêu**: Đấu nối API và các sự kiện hệ thống.
- **Tệp nguồn**:
  - `internal/gamification/transport/grpc/wallet_handler.go`
  - `internal/gamification/transport/consumer/level_up_consumer.go`
  - `internal/gamification/transport/consumer/workout_consumer.go`
  - `cmd/server/wire.go`
- **Các bước thực hiện**:
  1. Hiện thực `WalletGrpcHandler` ánh xạ từ Protobuf sang Use Cases.
  2. Viết Kafka Consumers lắng nghe `UserLeveledUp` và `WorkoutCompleted` gọi sang `EarnCoinsCommandHandler`.
  3. Hoàn tất Dependency Injection tại `wire.go`.
- **Tiêu chí hoàn thành (DoD)**:
  - Gọi API ConnectRPC qua `buf curl` trả về đúng số dư và lịch sử giao dịch.
