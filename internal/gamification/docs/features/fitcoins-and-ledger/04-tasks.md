# Danh Sách Nhiệm Vụ Kỹ Thuật (TDD Tasks): Tiền Tệ Ảo FitCoins & Sổ Cái Bất Biến

## Danh Sách Task Chi Tiết

### [x] [T-FC-01] [CONTRACT] Hợp Đồng API Protobuf & Sinh Mã Go Stubs
- **Tệp nguồn**:
  - `proto/contracts/supporting/gamification/v1/service/gamification_service.proto`
  - `proto/contracts/supporting/gamification/v1/message/gamification_messages.proto`
  - `proto/contracts/supporting/gamification/v1/event/coins_earned.proto`
  - `proto/contracts/supporting/gamification/v1/event/coins_spent.proto`
- **Trạng thái**: Hoàn thành (`buf lint` & `buf generate` pass).

---

### [x] [T-FC-02] [MIGRATION] Migration Cơ Sở Dữ Liệu Ví Tiền & Sổ Cái Bất Biến
- **Tệp nguồn**:
  - `internal/shared/database/migrations/13-create-gamification-tables.sql`
- **Trạng thái**: Hoàn thành (`gamification.user_wallet`, `gamification.coin_ledger`, `uq_coin_ledger_user_idempotency`, `uq_coin_ledger_daily_cap`, `idx_coin_ledger_user_created`).

---

### [x] [T-FC-03] [DOMAIN] Định Nghĩa Value Objects, Events & Entity CoinLedgerEntry
- **Tệp nguồn**:
  - `internal/gamification/domain/vo/coin_transaction_reason.go`
  - `internal/gamification/domain/entity/coin_ledger_entry.go`
  - `internal/gamification/domain/event/events.go`
  - `internal/gamification/domain/repository/user_wallet_repository.go`
- **Trạng thái**: Hoàn thành.

---

### [x] [T-FC-04 & T-FC-05] [DOMAIN] Triển Khai UserWallet Aggregate & Unit Tests
- **Tệp nguồn**:
  - `internal/gamification/domain/aggregate/user_wallet.go`
  - `internal/gamification/domain/aggregate/user_wallet_test.go`
- **Trạng thái**: Hoàn thành (Bảo vệ bất biến số dư không âm, chống tràn số `int64`, kiểm tra Daily Cap $O(1)$).

---

### [x] [T-FC-06] [INFRA] Hiện Thực Hóa GormUserWalletRepository & Outbox Mapping
- **Tệp nguồn**:
  - `internal/gamification/infrastructure/persistence/models.go`
  - `internal/gamification/infrastructure/persistence/user_wallet_repository.go`
  - `internal/gamification/infrastructure/event/outbox_writer.go`
- **Trạng thái**: Hoàn thành (Fast-path `SELECT ... FOR UPDATE` + fallback seed, Keyset Cursor Pagination `(created_at DESC, id DESC)`).

---

### [x] [T-FC-07] [APP] Triển Khai Tầng Application, WalletRewarder Port & Coordinator Integration
- **Tệp nguồn**:
  - `internal/gamification/application/port/wallet_rewarder.go`
  - `internal/gamification/application/command/award_coins.go`
  - `internal/gamification/application/command/spend_coins.go`
  - `internal/gamification/application/command/admin_adjust_coins.go`
  - `internal/gamification/application/query/get_wallet.go`
  - `internal/gamification/application/query/list_transactions.go`
  - `internal/gamification/application/service/activity_ingestion_coordinator.go`
  - `internal/gamification/application/command/init_user_gamification.go`
  - `internal/gamification/application/command/process_nutrition_activity.go`
  - `internal/gamification/application/command/process_personal_record.go`
- **Trạng thái**: Hoàn thành.

---

### [x] [T-FC-08] [TRANSPORT] Triển Khai ConnectRPC Server, Admin RBAC & Module Bootstrap
- **Tệp nguồn**:
  - `internal/gamification/transport/grpc/handler.go`
  - `internal/gamification/module.go`
- **Trạng thái**: Hoàn thành.
