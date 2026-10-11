# ADR-0004: Kết Hợp Cổng Thưởng Nội Bộ Đồng Bộ (In-Process Rewarder Port) Với Lắng Nghe Sự Kiện Bất Đồng Bộ

- **Feature**: fitcoins-and-ledger
- **Date**: 2026-10-12
- **Status**: Accepted
- **Deciders**: Maintainer (User), AI Assistant

## Context and Problem Statement
Trong phân hệ FitCoins, tiền ảo được thưởng khi người dùng thực hiện các hành động kỷ luật: lên cấp độ (`UserLeveledUp`), hoàn thành bài tập (`WorkoutCompleted`), bứt phá kỷ lục (`PersonalRecord`), đạt mốc chuỗi (`StreakMilestone`), hoặc mở khóa danh hiệu (`AchievementUnlocked`).

Có hai cách tiếp cận để nạp coin vào ví:
1. **Chỉ dùng Event-Driven (Kafka CloudEvents Consumers)**: Mọi nguồn phát sinh nạp coin đều đẩy ra Kafka broker, sau đó Consumer của FitCoins kéo về để xử lý.
2. **Cung cấp In-Process Port (Interface Go nội bộ)**: Cho phép các điều phối viên (`ActivityIngestionCoordinator`) hoặc service trong cùng tiến trình gọi trực tiếp để cộng coin trong cùng transaction.

Cần một quyết định kiến trúc cân bằng giữa tính phân tách ranh giới module (Decoupling) và trải nghiệm người dùng thực tế (UX Real-time Consistency).

## Decision Drivers
- **Trải nghiệm người dùng tức thì (Zero-Latency Feedback)**: Khi người dùng hoàn thành buổi tập hoặc mở khóa thành tựu trên màn hình, số dư trong ví phải cập nhật ngay lập tức. Tránh trường hợp màn hình hiện popup chúc mừng nhận $+50$ FitCoins nhưng mở ví ra số dư vẫn là số cũ do sự kiện còn đang xếp hàng trong Kafka broker.
- **Tính nguyên tử ACID & Chống Deadlock**: Trong cùng module Gamification, `UserXp`, `UserStreak`, và `UserWallet` đều nằm trong schema `gamification.*`. `ActivityIngestionCoordinator` cần khóa theo thứ tự cố định và thực thi trong **1 Database Transaction duy nhất** để triệt tiêu deadlock (tuân thủ `architecture.md#5`).
- **Thực tiễn Modular Monolith (Practical Simplicity - Rule 2)**: Các module chạy chung một tiến trình Go, không nên lạm dụng việc bắn message qua broker mạng chỉ để giao tiếp giữa các thành phần nội bộ.

## Considered Options

### Option 1: Chỉ Lắng Nghe Kafka CloudEvents (Pure Asynchronous Event-Driven)
- **Hạn chế**:
  - Gây ra độ trễ Eventual Consistency không đáng có cho các tác vụ cần phản hồi tức thì.
  - Vòng lặp message nội bộ (Internal Kafka Ping-Pong): `UserXp` lên cấp $\rightarrow$ Outbox $\rightarrow$ Kafka Broker $\rightarrow$ FitCoins Consumer $\rightarrow$ Cập nhật ví. Gây lãng phí I/O mạng, CPU và độ trễ.
  - Không thể chạy chung Database Transaction với `ActivityIngestionCoordinator`.

### Option 2: Gọi Loopback qua gRPC / ConnectRPC Nội Bộ
- **Hạn chế**:
  - Tốn tài nguyên serialize/deserialize Protobuf và loopback network port trên cùng 1 server. Không tận dụng được giao tiếp bộ nhớ của Go.

### Option 3: Mô Hình Lai (Hybrid): In-Process Application Port Kết Hợp Async Kafka Consumers
- **Cơ chế**:
  1. Tầng Application của FitCoins định nghĩa Input Port `WalletRewarder`:
     ```go
     type WalletRewarder interface {
         AwardCoins(ctx context.Context, cmd AwardCoinsCommand) (*AwardCoinsResult, error)
     }
     ```
  2. **Luồng nội bộ cùng tiến trình**:
     - `ActivityIngestionCoordinator` hoặc use cases mở khóa thành tựu gọi trực tiếp `walletRewarder.AwardCoins(txCtx, cmd)` ngay trong cùng Database Transaction. Số dư ví cập nhật tức thì ($0\text{ ms}$ lag).
  3. **Luồng liên module bên ngoài**:
     - Kafka Consumers vẫn lắng nghe các sự kiện từ các Bounded Context bên ngoài (ví dụ `DailyNutritionAchieved` từ module `nutrition`).
     - Khi nhận sự kiện, Consumer chỉ việc ủy quyền (delegate) sang chính `WalletRewarder.AwardCoins(...)`.

## Decision Outcome
Chosen: **Option 3 (Mô Hình Lai: In-Process Application Port Kết Hợp Async Kafka Consumers)**.

### Consequences
- **Positive:**
  - **Trải nghiệm người dùng hoàn hảo**: Tiền vào ví ngay lập tức khi hoàn thành bài tập hoặc đạt thành tựu.
  - **Bảo toàn nguyên tắc chống Deadlock**: Hoàn toàn tích hợp với `ActivityIngestionCoordinator` và thứ tự khóa `user_xp -> user_streak -> user_wallet` trong 1 transaction ACID.
  - **Thống nhất logic (Single Entry Point)**: Dù gọi từ Go code hay gọi từ Kafka Consumer, cả hai đều đi qua chung 1 logic nghiệp vụ của `UserWallet` Aggregate và ghi vào `coin_ledger`.
- **Negative / Trade-offs:**
  - Cần cẩn trọng về ranh giới transaction: Nếu một caller giữ transaction quá lâu, việc giữ khóa `SELECT ... FOR UPDATE` trên ví có thể làm chậm các giao dịch khác của cùng user đó (tuy nhiên thao tác ví chỉ mất $< 2\text{ ms}$).
