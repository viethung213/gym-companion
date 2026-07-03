# 13. Thiết Kế Hạ Tầng Kỹ Thuật (Infrastructure Design) - FITAI

Tài liệu này đặc tả thiết kế hệ thống hạ tầng kỹ thuật (Infrastructure Design), cách thức triển khai và tích hợp công nghệ để đáp ứng các ràng buộc về **hiệu năng thời gian thực** và **bảo mật quyền riêng tư**.

---

## 13.1 Sơ Đồ Kiến Trúc Vật Lý & Dòng Dữ Liệu

```
┌──────────────────────────────────────────────────────────────┐
│                    THIẾT BỊ DI ĐỘNG (CLIENT)                 │
│                                                              │
│  ┌───────────────────────┐       ┌────────────────────────┐  │
│  │  Camera Video Stream  │ ────► │ MediaPipe Skeleton AI  │  │
│  └───────────────────────┘       │ (Xử lý Edge AI thiết bị)│  │
│                                  └───────────┬────────────┘  │
│                                              │ (Tọa độ khớp) │
│                                              ▼               │
│  ┌───────────────────────┐       ┌────────────────────────┐  │
│  │   Audio Player Engine │ ◄──── │  Local Pose Evaluator  │  │
│  │ (Nhạc nền & Ducking)  │       │  (Tính ROM%, Đếm Rep)  │  │
│  └───────────────────────┘       └───────────┬────────────┘  │
└──────────────────────────────────────────────┼───────────────┘
                                               │ (Set & Session Logs)
                                               ▼
┌──────────────────────────────────────────────────────────────┐
│                   HỆ THỐNG MÁY CHỦ (SERVER)                  │
│                                                              │
│  ┌───────────────────────┐       ┌────────────────────────┐  │
│  │    API Gateway &      │ ────► │ PostgreSQL Database    │  │
│  │    Load Balancer      │       │ (Lưu trữ quan hệ)      │  │
│  └───────────┬───────────┘       └────────────────────────┘  │
│              │                                               │
│              ▼                                               │
│  ┌───────────────────────┐       ┌────────────────────────┐  │
│  │  Message Broker       │ ────► │  AI Nutrition Worker   │  │
│  │  (Kafka / RabbitMQ)   │       │  (Tính TDEE, gợi ý)    │  │
│  └───────────────────────┘       └────────────────────────┘  │
└──────────────────────────────────────────────────────────────┘
```

---

## 13.2 Các Giải Pháp Hạ Tầng Đặc Thù

### 1. Xử lý Thị giác Máy tính On-Device (Edge AI & Hybrid Data Flow)
* **Mục tiêu**: Đáp ứng ràng buộc an toàn bảo mật `Constraint-02` (không gửi dữ liệu video nhạy cảm của người dùng lên server) và tối ưu hóa băng thông truyền tải.
* **Giải pháp chạy Edge AI**:
  * **Trên thiết bị di động (Mobile Client)**: Tích hợp bộ thư viện **MediaPipe Pose** (iOS / Android) chạy trực tiếp trên luồng CPU/GPU của thiết bị.
  * **Trên nền tảng Web (Web Client)**: Sử dụng **MediaPipe Pose for Web** (biên dịch bằng WebAssembly - WASM) tận dụng công nghệ WebGL/WebGPU của trình duyệt để gia tốc phần cứng trực tiếp.
  * Cả hai nền tảng đều đọc luồng video từ camera cục bộ để trích xuất 33 điểm neo khớp xương (Joint Keypoints) dạng tọa độ 3D $(x, y, z)$ tại bộ nhớ thiết bị.
* **Quy trình truyền dữ liệu hỗn hợp (Hybrid Approach)**:
  * **Đo đếm và cảnh báo lỗi tức thời**: Được thực hiện hoàn toàn ở phía Client nhờ bộ đánh giá tư thế cục bộ (Local Pose Evaluator) nhằm triệt tiêu độ trễ mạng, đảm bảo phát âm thanh sửa tư thế (Audio Ducking) ngay lập tức (< 500ms).
  * **Đẩy dữ liệu không đồng bộ (Asynchronous Post-Set Upload)**: Khi người tập bấm hoàn thành Set tập, Client thực hiện nén chuỗi dữ liệu tọa độ 3D của cả Set tập (khoảng ~150KB - 200KB dữ liệu chuỗi thời gian) và đẩy ngầm lên Server thông qua kết nối **gRPC**:
    * **PostgreSQL**: Lưu kết quả tổng hợp của Set (số Rep nâng được, cân nặng thực tế, điểm Form trung bình) để phục vụ cho thuật toán Progressive Overload ở AI Coaching Context.
    * **MongoDB**: Lưu toàn bộ chuỗi tọa độ 3D $(x, y, z)$ của 33 điểm theo thời gian phục vụ cho việc vẽ biểu đồ quỹ đạo chuyển động 3D cho người dùng xem lại và dùng làm tập dữ liệu để huấn luyện lại AI sau này.

### 2. Cơ chế Giảm Âm lượng Thông minh (Audio Ducking) tại Client
* **Mục tiêu**: Phát cảnh báo bằng giọng nói (Voice Alert) sửa tư thế rõ ràng cho người tập mà không bị chèn bởi tiếng nhạc nền (`EDM` hoặc `Lofi`).
* **Giải pháp thực thi ở Client**:
  * Khi `Local Pose Evaluator` phát hiện sự kiện `InvalidRepDetected` (ví dụ: võng lưng quá mức khi squat), nó kích hoạt cơ chế điều khiển âm lượng tương ứng trực tiếp trên hệ thống Client (không gọi về Server):
    * **Đối với Web (Browser)**: Sử dụng **Web Audio API** để quản lý luồng nhạc nền thông qua một `GainNode` (Bộ điều khiển âm lượng). Khi có cảnh báo, gọi hàm `gainNode.gain.linearRampToValueAtTime(0.2, audioCtx.currentTime + 0.1)` để giảm mượt mà âm lượng nhạc xuống 20% trong vòng 100ms. Sau khi giọng nói sửa lỗi phát xong (bắt sự kiện `onended`), gọi hàm `gainNode.gain.linearRampToValueAtTime(1.0, audioCtx.currentTime + 0.3)` để phục hồi âm lượng nhạc nền về 100%.
    * **Đối với Android**: Sử dụng `AudioFocusRequest` để hạ mức âm lượng nguồn nhạc (Stream Music) xuống 80% (Ducking).
    * **Đối với iOS**: Sử dụng cấu hình `AVAudioSession.sharedInstance().setCategory(.playAndRecord, options: [.duckOthers])`.
  * Phát file audio cảnh báo lỗi (VD: *"Hãy giữ lưng thẳng"*).
  * Khi file âm thanh cảnh báo phát xong, phục hồi âm lượng nhạc nền về mức 100%.

### 3. Tích hợp Hàng đợi Thông điệp (Message Broker)
* **Giải pháp**: Sử dụng **Apache Kafka** làm xương sống truyền thông điệp sự kiện tích hợp giữa các Bounded Context.
* **Cấu hình Topics chính**:
  * `fitai.auth.events`: Các sự kiện đăng ký tài khoản người dùng mới.
  * `fitai.profile.events`: Chứa thông tin hoàn thiện hồ sơ sinh học, cập nhật chỉ số cơ thể hiện tại và báo chấn thương mới.
  * `fitai.workout.events`: Nhận kết quả các buổi tập hoàn thành từ Client (cả AI và Non-AI), lưu lượng calo tiêu thụ và volume để chuyển cho Dinh dưỡng & Coaching.
  * `fitai.coaching.events`: Các sự kiện đề xuất thay đổi kế hoạch tập do Plateau, Overtraining, lịch tập không tương thích.
* **Partition Key**: Sử dụng `userId` làm khóa phân vùng để đảm bảo thứ tự các sự kiện của cùng một người dùng luôn được xử lý tuần tự (In-Order Processing).

### 4. Cache & Rate Limiting với Redis
* **Lưu trữ phiên**: Redis lưu các token xác thực và các giá trị khóa thực đơn chống lặp món tạm thời của từng tài khoản người dùng để kiểm tra tức thì khi Client yêu cầu thực đơn gợi ý ngày.
* **Bộ đệm Dinh dưỡng & Đối tác**: Redis lưu cache danh sách thực phẩm, thông tin dinh dưỡng, danh mục quán ăn đối tác ăn ngoài (Dining-out partners) và thực đơn liên kết sẵn có để tăng tốc sinh gợi ý bữa ăn mà không cần truy vấn PostgreSQL nhiều lần.
* **Rate Limiter**: Giới hạn tần suất gọi API (ví dụ 100 requests/phút cho các API Onboarding, 10 requests/phút cho các API kết thúc buổi tập) để chống tấn công DDoS.

### 5. Giao tiếp REST HTTP và REST-to-gRPC Gateway (Generated by Buf)
* **Mục tiêu**: Chuẩn hóa giao tiếp Client-Server qua REST HTTP (JSON) để đơn giản hóa lập trình phía Client, đồng thời tối ưu hóa hiệu năng và thống nhất kiểu dữ liệu qua gRPC ở phía Backend.
* **Giải pháp**:
  * **Phía Client (Web & Mobile)**: Tương tác với Server hoàn toàn thông qua giao thức **REST HTTP (JSON)**. Toàn bộ các hành động từ đăng nhập, khai báo sức khỏe, cho đến gửi không đồng bộ tệp tọa độ khớp xương nén sau mỗi Set tập đều chạy qua HTTP POST/GET thông thường.
  * **Phía Server**: Triển khai **gRPC-Gateway** (sinh tự động từ file `.proto` thông qua `buf`) làm reverse proxy tại cổng vào. Trình biên dịch này sẽ đón nhận các cuộc gọi REST HTTP (JSON) từ Client, tự động phân tích cú pháp chuyển hóa thành các cuộc gọi **gRPC** nhị phân tương ứng và chuyển tiếp tới gRPC Server nội bộ của Backend để xử lý.
  * **Tài liệu hóa**: Tích hợp **Swagger UI** tự động tạo ra từ đặc tả OpenAPI của `buf` để hỗ trợ đội ngũ phát triển Client tích hợp và kiểm thử API REST một cách dễ dàng.
