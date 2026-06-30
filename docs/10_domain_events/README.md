# 10. Sự Kiện Miền & Sự Kiện Tích Hợp (Domain Events & Integration Events) - FITAI

Tài liệu này định nghĩa chi tiết tiêu chuẩn thiết kế các **Domain Events** (Sự kiện miền) và **Integration Events** (Sự kiện tích hợp) để giao tiếp bất đồng bộ qua Message Broker (Kafka/RabbitMQ) giữa các Bounded Contexts của hệ thống FITAI.

---

## 10.1 Khái Niệm Phân Biệt
* **Domain Event**: Biểu diễn một sự kiện nghiệp vụ xảy ra trong một Aggregation. Thường được xử lý bởi các handlers trong cùng một tiến trình ứng dụng (Memory) hoặc trong cùng một giao dịch Database.
* **Integration Event**: Biểu diễn một thay đổi trạng thái mà các hệ thống/ngữ cảnh khác cần biết. Được tuần tự hóa (Serialize) thành Protobuf hoặc JSON và gửi qua Message Broker.

---

## 10.2 Chuẩn Thiết Kế Sự Kiện Hệ Thống (Event Design Standards)

Để đảm bảo tính đồng bộ, khả năng mở rộng và tính tương thích cao giữa các service, toàn bộ Bounded Contexts phải tuân thủ chuẩn thiết kế sự kiện tích hợp sau:

### 1. Cấu trúc Khung Sự Kiện Chuẩn (Standard Event Envelope)
Tất cả các sự kiện lưu chuyển qua Message Broker bắt buộc phải tuân theo chuẩn **CloudEvents (v1.0)** để thống nhất phần Metadata. Khung cấu hình chuẩn được mô tả như sau:

* **Trường Metadata bắt buộc**:
  * `specversion`: Phiên bản đặc tả CloudEvents (mặc định là `"1.0"`).
  * `id`: ID duy nhất của sự kiện (sử dụng định dạng ULID hoặc UUIDv4).
  * `source`: Đường dẫn định danh của service phát đi sự kiện (ví dụ: `"/core/workout"`).
  * `type`: Định danh loại sự kiện (theo quy tắc đặt tên bên dưới).
  * `time`: Thời điểm xảy ra sự kiện theo chuẩn ISO 8601 chuỗi UTC (ví dụ: `"2026-06-29T16:40:00Z"`).
  * `datacontenttype`: Định dạng của dữ liệu nghiệp vụ (thường là `"application/json"` hoặc `"application/x-protobuf"`).
  * `data`: Đối tượng chứa thông tin nghiệp vụ cụ thể của sự kiện (sử dụng định dạng **`camelCase`**).

### 2. Quy Tắc Đặt Tên Sự Kiện (Naming Conventions)
Tên loại sự kiện (`type` trong Envelope) phải tuân thủ định dạng sau:
$$\text{contracts} . \langle\text{domain\_type}\rangle . \langle\text{service\_name}\rangle . \langle\text{version}\rangle . \langle\text{event\_name}\rangle$$

Trong đó:
* `contracts`: Namespace gốc của dự án.
* `domain_type`: Phân loại miền con (`core`, `supporting`, `generic`).
* `service_name`: Tên của Bounded Context viết thường (ví dụ: `workout`, `profile`, `auth`, `notification`).
* `version`: Phiên bản của Schema (ví dụ: `v1`, `v2`).
* `event_name`: Tên hành động nghiệp vụ ở dạng quá khứ, viết theo chuẩn **`camelCase`** (ví dụ: `sessionCompleted`, `userRegistered`).

*Ví dụ hợp lệ*: 
* `contracts.core.workout.v1.sessionCompleted`
* `contracts.generic.auth.v1.userRegistered`

### 3. Quy Định Thiết Kế Payload Dữ Liệu (`data`)
Khi thiết kế phần dữ liệu nghiệp vụ cụ thể trong các tệp `.proto`:
* **Tính tối thiểu (Minimality)**: Chỉ đưa vào những dữ liệu thực sự cần thiết cho các service khác phản ứng. Tránh đưa toàn bộ thực thể (Aggregate) vào event để giảm kích thước payload.
* **Mã định danh (Identifiers)**: Luôn đi kèm các khóa chính như `userId`, `sessionId`, `mealId` (sử dụng **`camelCase`**).

---

## 10.3 Cấu Trúc Thư Mục Lưu Trữ Event Contract

Tất cả các Event Contracts định nghĩa bằng Protobuf phải được lưu trữ đúng theo phân vùng thư mục tại `/proto/contracts`:

```text
/proto
├── buf.yaml
├── buf.gen.yaml
└── contracts/
    └── <domain_type>/                  # core, supporting, generic
        └── <service_name>/             # workout, profile, auth, v.v.
            └── <version>/              # v1, v2, v.v.
                ├── event/              # Thư mục chứa định nghĩa Event
                │   └── <event_name>.proto
                ├── message/            # Thư mục chứa Request/Response payload
                │   └── <payload_name>.proto
                └── service/            # Thư mục chứa định nghĩa gRPC Service
                    └── <service_name>_service.proto
```

Quy tắc khai báo trong các tệp dưới từng thư mục con:
1. **Event**:
   - `package contracts.<domain_type>.<service_name>.<version>.event;`
   - `option go_package = "github.com/viethung213/gym-companion/internal/gen/go/contracts/<domain_type>/<service_name>/<version>/event;<service_name>v<version>event";`
2. **Message**:
   - `package contracts.<domain_type>.<service_name>.<version>.message;`
   - `option go_package = "github.com/viethung213/gym-companion/internal/gen/go/contracts/<domain_type>/<service_name>/<version>/message;<service_name>v<version>message";`
3. **Service**:
   - `package contracts.<domain_type>.<service_name>.<version>.service;`
   - `option go_package = "github.com/viethung213/gym-companion/internal/gen/go/contracts/<domain_type>/<service_name>/<version>/service;<service_name>v<version>service";`

---

## 10.4 Thiết Kế Outbox Pattern (Đảm Bảo Tính Nhất Quán Giao Dịch)
Để tránh tình trạng "Database lưu thành công nhưng Event bị mất do Broker lỗi" hoặc ngược lại, FITAI áp dụng thiết kế **Outbox Pattern**:
1. Trong cùng một giao dịch lưu trữ Aggregation vào cơ sở dữ liệu, một bản ghi Event sẽ được lưu vào bảng `outbox_events`.
2. Một tiến trình ngầm (Outbox Publisher/CDC) sẽ đọc bảng `outbox_events` theo chu kỳ và đẩy sang Kafka/RabbitMQ.
3. Khi đẩy thành công, đánh dấu bản ghi Event trong `outbox_events` là `processed = true`.
