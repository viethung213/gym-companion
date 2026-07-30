# Quy chuẩn Sự kiện & Outbox (Event-Driven & CloudEvents Standards)

## 1. Nguyên tắc Cốt lõi

1. **Contract-First**: File `.proto` trong thư mục `proto/contracts/` là Nguồn sự thật duy nhất (SSOT) cho mọi cấu trúc dữ liệu sự kiện.
2. **CloudEvents 1.0**: Tất cả sự kiện phát ra bắt buộc bọc trong CloudEvent Envelope 1.0 tiêu chuẩn.
3. **Outbox Consistency**: Lưu sự kiện vào bảng `outbox_events` trong cùng SQL Transaction với thao tác thay đổi dữ liệu Aggregate.

---

## 2. Quy định Cấu trúc CloudEvent Envelope

CloudEvent Envelope phải chứa đủ 7 thuộc tính (tất cả thuộc tính tầng Envelope viết chữ thường):

- `specversion`: Cố định là `"1.0"`.
- `id`: Chuỗi UUID v4 duy nhất cho mỗi sự kiện.
- `source`: Định danh dịch vụ phát sự kiện dạng `services/<service-name>`.
- `type`: Chuỗi định danh loại sự kiện theo quy định tại Mục 3.
- `time`: Mốc thời gian UTC định dạng RFC3339 / ISO-8601.
- `datacontenttype`: Cố định là `"application/json"`.
- `data`: Raw JSON byte được sinh từ `protojson.Marshal()` của Protobuf Struct.

---

## 3. Quy định Đặt tên Event Type (`type`)

- **Định dạng**: `contracts.<domain_type>.<service_name>.<version>.<eventName>`
- **`domain_type`**: Nhận một trong các giá trị `generic`, `supporting`, `core`.
- **`service_name`**: Tên phân hệ (dùng tên thư mục tương ứng trong `internal/`).
- **`version`**: Phiên bản hợp đồng API (ví dụ: `v1`).
- **`eventName`**: Viết dạng **camelCase** (chữ cái đầu viết thường).
- **Cấm**: Không chèn phân đoạn `.event.` vào giữa namespace.

---

## 4. Quy định File Protobuf Contract (`.proto`)

- **Đường dẫn**: `proto/contracts/<domain_type>/<service_name>/<version>/event/<event_name>.proto`
- **Tên Message**: Viết PascalCase trực tiếp tên sự kiện. Cấm thêm hậu tố `EventPayload`.
- **Mốc thời gian**: Bắt buộc dùng `google.protobuf.Timestamp`. Cấm dùng `string` hoặc `int64`.

---

## 5. Quy định Serialization Payload (`data`)

- **Quy trình bắt buộc**: Domain Event $\rightarrow$ Map sang Proto Struct $\rightarrow$ Serialize bằng `protojson.Marshal()` $\rightarrow$ Bọc vào trường `data` của Envelope.
- **Cấm**: Không dùng `json.Marshal()` trực tiếp từ Go Domain Struct để tránh sai khác kiểu dữ liệu và key name.

---

## 6. Quy định Phân vùng Kafka

- **Partition Key**: Bắt buộc gán `PartitionKey = userId` khi ghi record vào `outbox_events` để bảo đảm tuyệt đối thứ tự tuần tự của các sự kiện theo người dùng trên cùng một Kafka Partition (Event Ordering).
