# CHƯƠNG 2. CƠ SỞ LÝ THUYẾT

Chương này trình bày các nền tảng lý thuyết và công nghệ được sử dụng trong đề tài FitAI, tổ chức theo ba phần lớn tương ứng với ba nhóm chức năng chính của hệ thống:

- **Phần A** — Nền tảng cho khối nhận diện thể chất và dinh dưỡng: AI/ML/DL, Computer Vision, Human Pose Estimation, phân tích chất lượng động tác, triển khai mô hình trên thiết bị biên, và khoa học dinh dưỡng.
- **Phần B** — Nền tảng cho khối huấn luyện viên ảo: LLM, Agentic AI, Tool use (Function Calling), RAG, và cơ chế suy luận.
- **Phần C** — Kiến trúc phần mềm và các công nghệ nền tảng của hệ thống.

Chi tiết kiến trúc cụ thể của FitAI, quy trình huấn luyện mô hình và đo lường thực nghiệm được đưa vào các chương sau.

---

# PHẦN A. NHẬN DIỆN, PHÂN TÍCH ĐỘNG TÁC VÀ DINH DƯỠNG

## 2.1. Trí tuệ nhân tạo, Học máy và Học sâu

**Trí tuệ nhân tạo (AI)** là ngành nghiên cứu các phương pháp giúp máy tính thực hiện những nhiệm vụ vốn đòi hỏi trí thông minh của con người. **Học máy (ML)** là nhánh của AI, xây dựng mô hình học quy luật từ dữ liệu thay vì lập trình quy tắc tường minh. **Học sâu (DL)** là nhánh của ML sử dụng mạng nơ-ron nhiều tầng để học biểu diễn phân tầng của dữ liệu. Quan hệ: $DL \subset ML \subset AI$.

Các bài toán ML thường thuộc ba nhóm: học có giám sát (dữ liệu có nhãn), học không giám sát (không nhãn), và học tăng cường (agent học qua tương tác và phần thưởng). Trong FitAI, khối phân tích tư thế dùng học sâu có giám sát (CNN), khối AI Coach dùng LLM (học có giám sát + tinh chỉnh RLHF), khối gợi ý dinh dưỡng dùng rule-based kết hợp ràng buộc tối ưu.

## 2.2. Thị giác máy tính và Ước lượng tư thế người

### 2.2.1. Thị giác máy tính

**Thị giác máy tính (Computer Vision, CV)** giúp máy tính "hiểu" nội dung ảnh và video. Các bài toán chính gồm phân loại ảnh, phát hiện đối tượng, phân đoạn, ước lượng tư thế và bám sát đối tượng. FitAI chủ yếu dùng bài toán ước lượng tư thế.

Kiến trúc chủ đạo của CV hiện đại là **Convolutional Neural Network (CNN)** — mạng nơ-ron dùng phép tích chập (convolution) để học các đặc trưng không gian địa phương từ ảnh, kết hợp với pooling để giảm chiều và tăng tính bất biến với dịch chuyển. Trong những năm gần đây, kiến trúc **Vision Transformer (ViT)** dùng cơ chế self-attention thay cho convolution cũng trở nên phổ biến.

### 2.2.2. Bài toán Ước lượng tư thế người

**Ước lượng tư thế người (Human Pose Estimation, HPE)** xác định tọa độ các khớp trên cơ thể người từ ảnh/video. Đầu ra là tập $K$ điểm $\{(x_k, y_k)\}$ (2D) hoặc $\{(x_k, y_k, z_k)\}$ (3D), kèm độ tin cậy $c_k$.

Chuẩn nhãn phổ biến là **COCO Keypoints** với 17 khớp chính: mũi, mắt trái/phải, tai trái/phải, vai, khuỷu, cổ tay, hông, đầu gối, mắt cá trái/phải. Trong FitAI, 17 điểm này đủ để tính các góc khớp và biên độ chuyển động cần cho phân tích động tác.

### 2.2.3. Ba hướng tiếp cận chính

- **Top-down (hai giai đoạn)**: phát hiện người trước, rồi chạy pose trên từng ảnh cắt. Đại diện: HRNet, ViTPose, RTMPose. Độ chính xác cao, chi phí tăng theo số người trong khung hình.
- **Bottom-up**: phát hiện toàn bộ keypoints trong một lần, sau đó gom nhóm theo cá nhân. Đại diện: OpenPose. Tốc độ ổn định khi nhiều người, độ chính xác thường thấp hơn top-down ở kịch bản một người.
- **One-stage**: phát hiện người và keypoints trong cùng một lần forward, không cần detector riêng. Đại diện: YOLO-Pose, YOLOv8-Pose, DEKR. Pipeline gọn, tốc độ cao, phù hợp thiết bị biên.

Một trục phân loại song song là **heatmap-based** (dự đoán bản đồ nhiệt của từng khớp, lấy argmax) so với **regression-based** (hồi quy trực tiếp tọa độ). Heatmap chính xác hơn khi độ phân giải đủ, còn regression tiết kiệm bộ nhớ và tự nhiên hơn cho one-stage.

### 2.2.4. Các mô hình tiêu biểu

- **OpenPose**: bottom-up kinh điển với Part Affinity Fields; có tính lịch sử, hiện đã bị vượt qua về hiệu năng.
- **HRNet**: giữ đặc trưng ở nhiều độ phân giải song song, độ chính xác cao nhưng nặng.
- **ViTPose**: dùng Vision Transformer làm backbone, đạt SOTA trên COCO nhưng chi phí lớn.
- **MediaPipe Pose**: pipeline của Google gồm detector và pose model chạy tuần tự, 33 keypoints, tốc độ cao trên di động.
- **MoveNet**: mô hình pose one-stage của Google cho single-person, tối ưu mobile/web.
- **RTMPose (MMPose)**: mô hình top-down real-time của OpenMMLab, dùng biểu diễn SimCC (Simple Coordinate Classification), cân bằng tốt tốc độ – độ chính xác.
- **YOLOv8-Pose**: mở rộng từ họ YOLO, one-stage, xuất ONNX gốc, cộng đồng lớn.

### 2.2.5. So sánh và lựa chọn cho FitAI

| Mô hình | Cách tiếp cận | Tốc độ trên di động | Độ chính xác | Xuất ONNX | Cộng đồng |
|---|---|---|---|---|---|
| MediaPipe Pose | Pipeline detector + pose | Rất cao | Trung bình | Hạn chế | Lớn |
| MoveNet | One-stage | Rất cao | Trung bình | Có | Trung bình |
| RTMPose | Top-down | Cao | Cao | Có | Đang tăng |
| YOLOv8-Pose | One-stage | Cao | Trung bình–cao | Có | Rất lớn |
| HRNet | Top-down | Thấp | Rất cao | Có | Lớn |
| ViTPose | Top-down | Rất thấp | Rất cao | Có | Trung bình |

FitAI chọn **YOLOv8-Pose** làm mô hình chính và **RTMPose** làm phương án so sánh. Lý do chọn YOLOv8-Pose:

1. **One-stage**: một mô hình duy nhất trên client, pipeline đơn giản hơn top-down (phải chạy detector rồi mới chạy pose).
2. **Kích thước nhỏ** (biến thể *nano* ~ 6.5 MB), hướng tới đạt phản hồi thời gian thực dưới 500 ms trên smartphone tầm trung; con số thực đo sẽ được trình bày ở Chương 5.
3. **Xuất ONNX gốc**: Ultralytics cung cấp API xuất ONNX trực tiếp, tương thích ONNX Runtime trên iOS/Android/Web.
4. **Cộng đồng và tài liệu phong phú**, phù hợp phạm vi khóa luận.

RTMPose được giữ như phương án so sánh vì (i) top-down cho độ chính xác thường cao hơn ở kịch bản single-person, và (ii) có SimCC head hiệu quả cho di động.

### 2.2.6. Độ đo đánh giá

**Object Keypoint Similarity (OKS)** là độ đo chuẩn của COCO:

$$
OKS = \frac{\sum_{i} \exp\left(-\dfrac{d_i^2}{2 s^2 k_i^2}\right) \delta(v_i > 0)}{\sum_{i} \delta(v_i > 0)}
$$

trong đó $d_i$ là khoảng cách Euclid giữa keypoint dự đoán và ground truth thứ $i$, $s$ là scale của người, $k_i$ là hằng số per-keypoint. Từ OKS tính **Average Precision (AP)** tại các ngưỡng $\{0.5, 0.55, ..., 0.95\}$. Ngoài ra, **Percentage of Correct Keypoints (PCK)** đơn giản hơn: một keypoint đúng nếu khoảng cách tới ground truth dưới một ngưỡng cho trước.

---

## 2.3. Phân tích chất lượng động tác

### 2.3.1. Chuỗi xử lý

Từ 17 keypoints trên mỗi frame, hệ thống xử lý theo bốn bước: (1) làm mượt tín hiệu để giảm nhiễu keypoint theo thời gian, (2) tính đặc trưng biomechanics (góc khớp, ROM, tỷ lệ đoạn cơ thể), (3) đếm rep bằng máy trạng thái, (4) chấm form và sinh phản hồi.

Ở bước làm mượt, hai kỹ thuật phổ biến là **Exponential Moving Average (EMA)** — đơn giản, độ trễ thấp — và **One-Euro Filter** — thích ứng, giảm nhiễu tốt khi động tác chậm và tăng đáp ứng khi động tác nhanh. FitAI dự kiến dùng EMA làm mặc định vì đủ nhẹ cho thiết bị biên.

> **Hình 2.1** – Sơ đồ pipeline phân tích động tác *(placeholder)*

### 2.3.2. Biomechanics tối thiểu

Đa số bài tập cơ bản (squat, deadlift, push-up) được quan sát ở **mặt phẳng sagittal** (camera đặt bên hông).

**Góc khớp** được tính từ ba điểm liên tiếp $A$, $B$, $C$ (với $B$ là đỉnh):

$$
\theta = \arccos\left(\frac{\vec{BA} \cdot \vec{BC}}{\|\vec{BA}\| \cdot \|\vec{BC}\|}\right)
$$

Ví dụ, góc gối khi squat được tính từ ba điểm hông – gối – mắt cá; theo quy ước tham khảo trong biomechanics thể thao, squat đúng thường đạt góc gối tối thiểu 90° ở đáy động tác (parallel squat).

**Biên độ chuyển động (Range of Motion, ROM)** là chênh lệch góc khớp giữa hai vị trí biên trong một rep. Một rep được coi là hợp lệ khi ROM đạt ít nhất một tỷ lệ định trước so với biên độ tiêu chuẩn của bài; ngưỡng cụ thể sẽ được trình bày ở Chương 3.

### 2.3.3. Đếm rep bằng máy trạng thái

FitAI dùng **máy trạng thái** trên góc khớp chủ đạo: định nghĩa hai trạng thái "top" và "bottom" theo ngưỡng góc, chuyển "top → bottom → top" được tính là một rep. Để tránh đếm dao động (bouncing) do nhiễu keypoint, hai ngưỡng chuyển trạng thái được đặt lệch nhau — kỹ thuật **hysteresis**: ví dụ chỉ chuyển sang "bottom" khi góc gối < 100° và chỉ chuyển ngược lên "top" khi góc gối > 160°. Cách này đơn giản, dễ giải thích, phù hợp với các bài chu kỳ rõ ràng khi đã biết bài đang tập.

### 2.3.4. Chấm điểm Form

Form Score được tổ hợp từ nhiều luật, mỗi luật đánh giá một khía cạnh (ROM đủ, không lệch gối, lưng không cong, cân bằng trái–phải…):

$$
S_{\text{form}} = \sum_{r \in R} w_r \cdot f_r(\text{features}), \quad S_{\text{form}} \in [0, 100]
$$

trong đó $w_r$ là trọng số luật, $f_r \in [0, 1]$ là mức độ đạt luật, thường được tính bằng hàm mượt (chẳng hạn sigmoid) quanh một ngưỡng chuẩn. Ví dụ với luật "ROM gối đủ ≥ 90°", $f_r$ đạt 1.0 khi ROM ≥ 90° và giảm dần về 0 khi ROM ≤ 60°. Trọng số $w_r$ được xác định theo mức độ quan trọng của lỗi đối với an toàn và hiệu quả (lỗi lệch gối được ưu tiên cao hơn lỗi tốc độ).

### 2.3.5. Vì sao rule-based thay vì học sâu end-to-end

Một hướng thay thế là huấn luyện mạng nơ-ron nhận keypoints và trực tiếp xuất Form Score. Với đề tài này, rule-based được chọn vì: (1) không cần dữ liệu chuyên gia lớn, (2) chỉ ra được lỗi cụ thể để sửa — trái với "hộp đen" của end-to-end, và (3) dễ mở rộng khi thêm bài tập mới, chỉ cần thêm luật.

---

## 2.4. Triển khai mô hình trên thiết bị biên

### 2.4.1. ONNX và ONNX Runtime

**Open Neural Network Exchange (ONNX)** là định dạng mở, chuẩn hóa biểu diễn mô hình machine learning. ONNX cho phép mô hình được huấn luyện trong một framework (PyTorch, TensorFlow) rồi xuất và chạy trong một môi trường khác mà không cần training framework nguyên gốc — một dạng "portable IR" (intermediate representation) cho model.

**ONNX Runtime** là engine inference đa nền tảng, hỗ trợ CPU, GPU và các bộ tăng tốc chuyên dụng thông qua khái niệm **Execution Provider**: CoreML trên iOS, NNAPI trên Android, DirectML/CUDA/TensorRT trên desktop, WebAssembly/WebGL trên trình duyệt. Runtime tự tối ưu graph (constant folding, operator fusion) và chọn execution provider phù hợp với phần cứng có sẵn.

Trong FitAI, YOLOv8-Pose được xuất từ PyTorch sang ONNX rồi chạy bằng ONNX Runtime trên thiết bị người dùng. Điều này tách biệt training và inference: có thể cập nhật mô hình mà không đổi mã client.

### 2.4.2. Edge AI

**Edge AI** chạy mô hình AI trực tiếp trên thiết bị đầu cuối thay vì trên đám mây. FitAI chọn edge cho pose estimation vì:

- **Độ trễ**: yêu cầu phản hồi sửa lỗi tư thế dưới 500 ms rất khó đạt qua mạng — roundtrip client–server qua 4G/5G thông thường đã tốn cỡ 100–300 ms chỉ riêng phần mạng (ước tính theo dữ liệu tốc độ mạng di động phổ biến), chưa kể tải video và inference phía server. Chạy trên thiết bị giữ độ trễ chỉ bằng thời gian inference (dự kiến ~15–30 ms/frame với YOLOv8n-pose, sẽ đo cụ thể ở Chương 5).
- **Quyền riêng tư**: video không rời khỏi thiết bị; chỉ tọa độ khớp và số liệu tổng hợp được đồng bộ lên server. Băng thông cần cho tọa độ khớp rất nhỏ: 17 điểm × 3 giá trị × 30 fps × 4 byte ≈ 6 KB/s.
- **Chi phí và độ tin cậy**: không phụ thuộc mạng, giảm tải máy chủ, có thể hoạt động offline.

Đánh đổi: mô hình phải nhỏ đủ để chạy trên thiết bị tầm trung.

### 2.4.3. Tối ưu inference

Ba kỹ thuật phổ biến:

- **Lượng tử hóa (quantization)**: giảm độ chính xác biểu diễn trọng số từ FP32 xuống FP16 hoặc INT8. FP16 thường giảm ~50% kích thước và tăng tốc 1.5–2× trên GPU/NPU với ảnh hưởng không đáng kể tới AP.
- **Cắt tỉa (pruning)**: loại bỏ trọng số/kênh ít quan trọng; cần fine-tune sau khi cắt.
- **Chưng cất tri thức (knowledge distillation)**: huấn luyện mô hình nhỏ (student) học từ đầu ra của mô hình lớn (teacher).

Trong khuôn khổ khóa luận, FitAI dự kiến chỉ áp dụng lượng tử hóa FP16 (dễ triển khai, ít rủi ro về độ chính xác). Các kỹ thuật còn lại được đưa vào hướng phát triển tương lai.

---

## 2.5. Khoa học dinh dưỡng và Hệ thống gợi ý

### 2.5.1. Kiến thức dinh dưỡng cần thiết

**Basal Metabolic Rate (BMR)** — công thức **Mifflin-St Jeor**, phổ biến nhất hiện nay:

$$
BMR_{\text{nam}} = 10W + 6.25H - 5A + 5
$$

$$
BMR_{\text{nữ}} = 10W + 6.25H - 5A - 161
$$

với $W$ là cân nặng (kg), $H$ chiều cao (cm), $A$ tuổi (năm).

**Total Daily Energy Expenditure (TDEE)** = BMR × hệ số vận động (1.2 ít vận động → 1.9 rất nặng). Muốn giảm cân ăn dưới TDEE, muốn tăng cơ ăn trên TDEE.

**Macronutrients**: protein (4 kcal/g), tinh bột (4 kcal/g), chất béo (9 kcal/g). Khuyến nghị của Hội Dinh dưỡng Thể thao Quốc tế (ISSN): protein 1.6–2.2 g/kg thể trọng cho người tập tăng cơ, chất béo 20–35% tổng calo, tinh bột phần còn lại.

### 2.5.2. Hệ thống gợi ý

**Hệ thống gợi ý (Recommender System)** dự đoán mức độ ưa thích của người dùng với các mục chưa tương tác. Bốn cách tiếp cận phổ biến:

- **Content-based**: dựa trên đặc trưng mục và hồ sơ người dùng.
- **Collaborative Filtering (CF)**: dựa trên hành vi người dùng tương tự (user–user hoặc item–item CF); phổ biến trong e-commerce, Netflix.
- **Hybrid**: kết hợp cả hai.
- **Constraint-based / Knowledge-based**: dựa trên tri thức miền và ràng buộc rõ ràng thay vì học từ tương tác quá khứ.

### 2.5.3. Cách tiếp cận cho FitAI

Bài toán gợi ý thực đơn của FitAI là **tổ hợp có ràng buộc**: mỗi ngày chọn bộ bữa ăn thỏa mãn ngân sách calo, tỷ lệ macro, dị ứng, loại thực đơn (chay/halal), và ràng buộc **anti-repetition** — không lặp lại nguồn protein chính, tinh bột và chủ đề món trong một cửa sổ thời gian nhất định để tránh nhàm chán và cân bằng vi chất.

FitAI dùng **rule-based / constraint-based** thay vì Collaborative Filtering vì: (1) cold start nghiêm trọng — nguyên mẫu chưa có dữ liệu tương tác đủ cho CF; (2) mục tiêu dinh dưỡng có ngưỡng calo/macro cụ thể — ràng buộc rõ ràng hoạt động tốt hơn học từ tín hiệu ngầm; (3) tính giải thích — LLM có thể sinh lý do trực tiếp từ luật đã áp, khó làm với CF ẩn.

---

# PHẦN B. AGENTIC AI VÀ HUẤN LUYỆN VIÊN ẢO

## 2.6. Mô hình ngôn ngữ lớn

### 2.6.1. Kiến trúc Transformer

**LLM** là mô hình neural hàng tỷ tham số, huấn luyện trên corpus văn bản khổng lồ để dự đoán token kế tiếp. Kiến trúc chủ đạo là **Transformer** với cơ chế **self-attention** cho phép mỗi token đối chiếu với mọi token khác trong ngữ cảnh:

$$
\text{Attention}(Q, K, V) = \text{softmax}\!\left(\frac{QK^\top}{\sqrt{d_k}}\right) V
$$

trong đó $Q$, $K$, $V$ là các ma trận query, key, value được sinh từ embedding của chuỗi đầu vào, $d_k$ là chiều của key. Điểm mấu chốt là chuỗi được xử lý song song (không tuần tự như RNN), giúp huấn luyện được ở quy mô lớn trên GPU/TPU.

### 2.6.2. Quy trình huấn luyện

Một LLM hiện đại trải qua ba giai đoạn:

1. **Pre-training**: học dự đoán token tiếp theo trên corpus khổng lồ (hàng nghìn tỷ token), hàm mất mát là cross-entropy $\mathcal{L} = -\sum_{t} \log P(x_t \mid x_{<t})$. Kết quả là một *foundation model* có tri thức rộng nhưng chưa "biết nghe lời".
2. **Instruction tuning**: tinh chỉnh có giám sát trên các cặp (prompt, response) chất lượng cao để mô hình học tuân theo yêu cầu.
3. **RLHF (Reinforcement Learning from Human Feedback)**: dùng học tăng cường với reward model được huấn luyện từ đánh giá của con người để căn chỉnh mô hình với sở thích người dùng.

Kết quả là các mô hình như GPT-4, Claude, Llama-3, Gemini có khả năng đối thoại tự nhiên và tuân theo yêu cầu.

### 2.6.3. Context window và giới hạn

**Context window** là số token tối đa LLM có thể xử lý trong một lượt (thường 8K–1M token tùy mô hình). Mọi thứ vượt quá cửa sổ này phải bị cắt hoặc tóm tắt. Đây là ràng buộc kỹ thuật quan trọng khi thiết kế prompt: không thể nhồi toàn bộ lịch sử người dùng vào mỗi lượt gọi.

Ngoài ra, LLM có hai giới hạn cố hữu: (i) tri thức bị đóng băng tại thời điểm training (knowledge cutoff), và (ii) không biết dữ liệu riêng của hệ thống. Cả hai được khắc phục bằng RAG (mục 2.9).

## 2.7. Từ chatbot tới Agent

### 2.7.1. Khái niệm Agent

**Chatbot cổ điển** phản hồi theo mẫu hoặc theo một cặp prompt–response đơn: input → output, không có bộ nhớ hoặc hành động tiếp theo.

**Agent** trong AI là một hệ thống có khả năng *quan sát (perceive)* môi trường, *suy luận (reason)* về trạng thái và mục tiêu, *lập kế hoạch (plan)*, và *hành động (act)* trên môi trường thông qua các công cụ. Vòng lặp agent tiếp tục cho đến khi mục tiêu được hoàn thành hoặc dừng theo tiêu chí.

**Agentic AI** đề cập tới việc dùng LLM làm bộ não của agent: LLM nhận ngữ cảnh, quyết định hành động tiếp theo, hệ thống thực thi hành động và trả kết quả về, LLM tiếp tục suy luận. Sự khác biệt cốt lõi so với chatbot: agent chủ động thực hiện chuỗi hành động để đạt mục tiêu, không chỉ trả lời.

### 2.7.2. ReAct pattern

Mẫu thiết kế phổ biến nhất là **ReAct** — xen kẽ giữa *Reasoning* (viết ra suy nghĩ) và *Acting* (gọi tool). Ví dụ:

```
Thought: Người dùng nói bị đau gối. Cần biết bài nào không tác động tới gối.
Action: search_exercise_library(exclude_joints=["knee"])
Observation: [danh sách 12 bài tập tay và core]
Thought: Trong lịch tuần này có 2 buổi cardio dùng chạy bộ. Nên thay.
Action: update_weekly_schedule(user_id=123, replace={"cardio_run": "cycling"})
Observation: Đã cập nhật.
Final answer: Đã thay chạy bộ bằng đạp xe cho hai buổi cardio...
```

Việc viết ra "Thought" giúp mô hình duy trì mạch logic, còn "Action" cho phép nó vượt qua giới hạn tri thức tĩnh.

## 2.8. Tool use và Function Calling

**Tool use** cho phép LLM gọi hàm ngoài (API, DB, tính toán) trong quá trình suy luận. Cơ chế phổ biến hiện nay là **function calling**: developer định nghĩa các function kèm JSON schema mô tả tham số; khi LLM cần dùng, nó sinh ra một khối JSON có cấu trúc; runtime parse, thực thi, và trả kết quả về LLM để tiếp tục.

Ví dụ trong FitAI, khi bắt đầu buổi tập, AI Coach hỏi người dùng ngắn về chấn thương mới hoặc dụng cụ hiện có. Sau khi user trả lời, Agent trích xuất và gọi:

```json
{
  "tool": "UpdateWorkoutContext",
  "arguments": {
    "avoid_joints": ["wrist"],
    "recovered_joints": ["knee"],
    "override_equipments": ["dumbbell", "bodyweight"]
  }
}
```

Backend Go nhận payload, cập nhật trạng thái chấn thương/thiết bị trong cơ sở dữ liệu và trả về danh sách bài tập hợp lệ đã được lọc. Điểm quan trọng: LLM không tự thực hiện thao tác DB, chỉ *đề xuất* hành động dưới dạng có cấu trúc; hệ thống deterministic mới thực thi. Điều này giữ được tính an toàn và khả năng kiểm thử.

## 2.9. Retrieval-Augmented Generation (RAG)

**RAG** gắn LLM với một cơ sở tri thức truy vấn được, giải quyết cả hai giới hạn nêu ở 2.6.3 (knowledge cutoff, dữ liệu riêng).

Pipeline:

1. **Ingest & chunk**: chia tài liệu thành các đoạn nhỏ.
2. **Embed**: mã hóa mỗi đoạn thành vector đặc trưng qua mô hình embedding (ví dụ `text-embedding-3-small`, `bge-small`).
3. **Index**: lưu các vector trong cơ sở dữ liệu vector (pgvector, Qdrant, Milvus…).
4. **Retrieve**: khi có câu hỏi, mã hóa câu hỏi thành vector, tìm top-k đoạn có độ tương đồng cosine cao nhất:

$$
\text{sim}(u, v) = \frac{u \cdot v}{\|u\| \cdot \|v\|}
$$

5. **Generate**: nối các đoạn truy vấn vào prompt để LLM sinh câu trả lời có căn cứ.

Chất lượng RAG phụ thuộc vào chiến lược **chunking**: đoạn quá dài làm loãng ngữ nghĩa, đoạn quá ngắn mất ngữ cảnh. Với dữ liệu FitAI (mô tả bài tập, hồ sơ, log buổi tập), cách chia theo đơn vị nghiệp vụ (một bài, một buổi, một ngày) thường hiệu quả hơn chia theo số ký tự cố định.

Trong FitAI, RAG cho phép AI Coach lấy ngữ cảnh hồ sơ và lịch sử tập của người dùng khi sinh giáo án và khi viết lời giải thích, mà không phải đưa toàn bộ lịch sử vào prompt.

## 2.10. Suy luận trong LLM

### 2.10.1. Chain-of-Thought

**Chain-of-Thought (CoT) prompting** là kỹ thuật yêu cầu LLM viết ra các bước suy luận trung gian trước khi đưa ra câu trả lời cuối. Các nghiên cứu cho thấy CoT cải thiện đáng kể độ chính xác trên các bài toán cần suy luận nhiều bước (toán, logic, planning), đặc biệt với mô hình lớn.

Biến thể **Self-Consistency**: sinh nhiều lời giải với nhiệt độ cao rồi chọn đáp án đa số. Trade-off: chi phí tính toán tăng tuyến tính với số lời giải.

### 2.10.2. Reasoning Models

Từ 2024, nhóm mô hình "reasoning" như OpenAI o1, DeepSeek-R1, Claude với extended thinking dành riêng một pha *"suy nghĩ"* trước khi trả lời, được huấn luyện có mục đích để cải thiện lập luận. Chi phí độ trễ tăng đáng kể (vài giây tới vài chục giây/lượt) nhưng chất lượng suy luận trên bài phức tạp cải thiện rõ.

Trong FitAI, các quyết định định kỳ (sinh lộ trình 4 tuần, review cuối chu kỳ, sinh giáo án buổi trước ngày tập) có thể dùng reasoning model vì được thực hiện ở nền (pre-caching), không chặn user. Bước sinh giáo án ngay khi check-in đầu buổi cần đáp ứng nhanh (mục tiêu vài giây), ưu tiên mô hình đối thoại thông thường có tốc độ cao.

## 2.11. Vai trò AI Coach trong FitAI

Điểm quan trọng cần nhấn mạnh: AI Coach trong FitAI **không phải là chatbot đối thoại tự do**. Trách nhiệm của Agent được giới hạn ở ba nhóm nhiệm vụ, mọi thứ còn lại do Backend deterministic xử lý:

1. **Lập kế hoạch giáo án**: sau khi Backend Go lọc sẵn 30–40 bài tập an toàn theo chấn thương/dụng cụ của user, Agent chọn và sắp xếp thứ tự bài tập theo nguyên tắc khoa học thể thao (compound trước, isolation sau, luân phiên kiểu chuyển động). Agent **không tính** số tạ, set, rep — phần đó do rule engine (công thức Epley và các quy tắc tăng tiến) xử lý.
2. **Trích xuất ngữ cảnh từ ngôn ngữ tự nhiên**: ở check-in đầu buổi, Agent hỏi ngắn user về chấn thương mới, khớp phục hồi, dụng cụ thay đổi; nhận câu trả lời và trích xuất thành cấu trúc để gọi tool `UpdateWorkoutContext`.
3. **Sinh lời giải thích**: viết lời giải thích tự nhiên cho mỗi quyết định (vì sao chọn bài này, vì sao tăng/giảm tạ, báo cáo sau buổi tập) theo phong cách coach mà user đã chọn.

Do context window có giới hạn, thiết kế prompt cần chọn lọc: system prompt cố định định hình vai trò và phong cách, phần ngữ cảnh động chỉ gồm thông tin liên quan trực tiếp (hồ sơ user, danh sách bài tập ứng viên, lịch sử buổi tập gần nhất — được cung cấp qua tool + RAG).

Cách bố trí này (Agent = reasoning + NLP + explanation, Backend = mọi phép tính deterministic) giúp giữ chi phí LLM ở mức thấp, tăng độ tin cậy và có phương án fallback bằng template khi Agent lỗi.

---

# PHẦN C. KIẾN TRÚC PHẦN MỀM VÀ CÔNG NGHỆ NỀN TẢNG

## 2.12. Kiến trúc Client–Server và phân tách real-time / batch

Kiến trúc client–server cổ điển đủ phục vụ FitAI, nhưng đặc thù real-time của khối phân tích tư thế đòi hỏi **phân tách rõ hai đường xử lý**:

- **Real-time path (phía client)**: nhận frame camera → pose estimation → rule-based analysis → audio feedback. Toàn bộ nằm trên thiết bị người dùng, độ trễ dưới 500 ms.
- **Batch / analytics path (phía server)**: nhận kết quả tổng hợp buổi tập → lưu trữ → AI Coach phân tích xu hướng → sinh giáo án buổi kế tiếp → gợi ý dinh dưỡng ngày mai. Không yêu cầu độ trễ ngặt nghèo.

Phân tách này giúp đường xử lý real-time và đường phân tích được tối ưu độc lập, thay vì phải cân bằng chung trên cùng một tầng. Server có thể chậm hơn, dùng LLM mạnh hơn, giữ lịch sử dài hơn — client vẫn hoạt động được mà không phụ thuộc.

## 2.13. Modular Monolith và Domain-Driven Design

### 2.13.1. Ba lựa chọn kiến trúc

- **Monolith cổ điển**: toàn bộ ứng dụng đóng gói thành một triển khai duy nhất, không có ranh giới module rõ. Đơn giản khi nhỏ, khó bảo trì khi lớn.
- **Microservices**: tách ứng dụng thành nhiều dịch vụ độc lập triển khai riêng. Mở rộng linh hoạt nhưng đội chi phí vận hành (mạng, quan sát, phối hợp giao dịch phân tán).
- **Modular Monolith**: triển khai vẫn là một ứng dụng, nhưng mã nguồn tách thành các module có ranh giới rõ, giao tiếp qua interface tường minh, không import chéo trực tiếp.

### 2.13.2. Bounded Context

**Domain-Driven Design (DDD)** định nghĩa **bounded context** là một ranh giới rõ ràng trong đó một mô hình miền (domain model) và ngôn ngữ chung (ubiquitous language) là nhất quán. Vượt qua ranh giới, cùng một khái niệm có thể mang ý nghĩa khác — ví dụ "User" trong Auth có thể chỉ có id + credentials, còn "User" trong Profile có thêm chỉ số cơ thể và mục tiêu.

Trong FitAI, các bounded context chính (Auth, Profile, Workout, Nutrition, Coaching, Notification…) được cài đặt như các module Go riêng biệt, giao tiếp qua interface được sinh từ Protocol Buffers. Danh sách và ranh giới cụ thể sẽ được trình bày ở Chương 3.

### 2.13.3. Vì sao chọn Modular Monolith

FitAI chọn modular monolith vì: (i) đơn giản triển khai (một binary/container); (ii) ranh giới module rõ nhờ DDD, hỗ trợ khả năng tiến hóa; (iii) khi thực sự cần scale ngang một module, có thể tách thành service riêng mà không phải viết lại toàn bộ; (iv) phù hợp phạm vi khóa luận một sinh viên, không có nhu cầu vận hành phức tạp của microservices.

## 2.14. Contract-first Design: Protocol Buffers và ConnectRPC/gRPC

### 2.14.1. Protocol Buffers

**Protocol Buffers (Protobuf)** là ngôn ngữ mô tả cấu trúc dữ liệu và service, độc lập ngôn ngữ. File `.proto` định nghĩa message và service; công cụ `protoc` sinh mã nguồn cho nhiều ngôn ngữ (Go, TypeScript, Python…). Nhờ vậy, client và server luôn cùng hiểu một hợp đồng (contract) mà không cần đồng bộ tay.

FitAI dùng Protobuf làm nguồn sự thật cho toàn bộ API — mọi endpoint, message, enum được định nghĩa trong thư mục `proto/`.

### 2.14.2. gRPC và ConnectRPC

**gRPC** là framework RPC hiệu năng cao dựa trên Protobuf và HTTP/2. Ưu điểm: mã hóa nhị phân nhỏ gọn, hỗ trợ streaming bốn chiều (unary, server-stream, client-stream, bidirectional), sinh code tự động. Nhược điểm với web: HTTP/2 và định dạng binary không tương thích trực tiếp với trình duyệt, phải qua proxy `grpc-web`.

**ConnectRPC** là giao thức RPC do Buf.build đề xuất, tương thích ngược với gRPC nhưng chạy được trực tiếp trên HTTP/1.1 và JSON, dễ debug bằng curl/browser. Server ConnectRPC vẫn nhận được cả yêu cầu ConnectRPC lẫn gRPC gốc, giúp một backend phục vụ cả web client (dùng Connect) và mobile/native (dùng gRPC).

FitAI dùng **ConnectRPC** cho backend Go, cho phép frontend TypeScript gọi trực tiếp qua HTTP/JSON trong lúc phát triển, đồng thời giữ khả năng chuyển sang gRPC nhị phân khi cần tối ưu hiệu năng.

## 2.15. Backend: Ngôn ngữ Go

**Go** là ngôn ngữ được Google phát triển, phù hợp cho các dịch vụ backend hiệu năng cao. Các đặc trưng quan trọng đối với FitAI:

- **Concurrency qua goroutine và channel**: mô hình concurrency dựa trên CSP (Communicating Sequential Processes), viết code xử lý nhiều kết nối song song đơn giản mà không cần callback hell hay thread pool phức tạp.
- **Binary tĩnh, không runtime**: compile ra một file thực thi duy nhất, không cần cài JVM/CLR trên server; giảm kích thước Docker image và thời gian khởi động.
- **Compile nhanh**: giúp vòng lặp phát triển nhanh, quan trọng khi thay đổi contract Protobuf phải regenerate và build.
- **Chuẩn hóa cao**: gofmt, module system tích hợp, ít framework "chiến" nhau; tài liệu chính thức đầy đủ.
- **Hệ sinh thái ML/AI đang tăng**: ONNX Runtime, Ollama và nhiều thư viện AI có Go binding trưởng thành.

## 2.16. Frontend: React và TypeScript

### 2.16.1. React

**React** là thư viện UI theo mô hình component: giao diện được xây dựng từ các thành phần độc lập, mỗi component nhận props và trả về mô tả DOM. React dùng **Virtual DOM** — cây biểu diễn trung gian — để tính diff và cập nhật DOM thật một cách tối thiểu. Điều này giúp UI phản ứng nhanh với state thay đổi mà không cần thao tác DOM tay.

Cơ chế **Hooks** (useState, useEffect…) cho phép quản lý state và side-effect trong function component, đơn giản hóa code so với class component. Với FitAI, các màn hình workout, chat với coach, dashboard đều xây từ component tái sử dụng.

### 2.16.2. TypeScript

**TypeScript** là ngôn ngữ mở rộng JavaScript với hệ thống kiểu tĩnh. Ưu điểm chính: bắt lỗi tại compile-time thay vì runtime, tự động hoàn thiện code trong IDE, refactor an toàn.

Đặc biệt quan trọng với FitAI: các type sinh tự động từ Protobuf (qua ConnectRPC) mang thẳng contract backend sang frontend, đảm bảo mỗi thay đổi API được phát hiện ngay lúc compile client, không phải chờ tới lúc chạy.

## 2.17. Lưu trữ: PostgreSQL và Redis

### 2.17.1. PostgreSQL

**PostgreSQL** là hệ quản trị cơ sở dữ liệu quan hệ mã nguồn mở, tuân thủ ACID nghiêm ngặt. Các đặc trưng phù hợp FitAI:

- **JSONB**: kiểu dữ liệu JSON được lưu ở dạng nhị phân, hỗ trợ index (GIN), toán tử và truy vấn theo path. Phù hợp cho các phần bán cấu trúc như payload giáo án, log buổi tập, cấu hình cá nhân — không cần tách bảng cho mọi trường hợp.
- **Extension phong phú**: `pgvector` cho tìm kiếm vector (dùng cho RAG), `pg_trgm` cho tìm kiếm mờ, `TimescaleDB` cho time-series (nếu cần metric).
- **Full-text search** tích hợp, giảm nhu cầu Elasticsearch cho các use case đơn giản.
- **Transaction mạnh**: giao dịch phân tán trong cùng database, giữ được tính nhất quán khi cập nhật nhiều bounded context.

### 2.17.2. Redis

**Redis** là kho dữ liệu key-value in-memory, truy vấn dưới mili-giây. Vai trò trong FitAI:

- **Session store**: lưu trạng thái xác thực của client giữa các request.
- **Cache**: caching profile người dùng, thư viện bài tập, hạn chế truy vấn PostgreSQL lặp lại.
- **Rate limiting**: dùng counter với TTL để hạn chế số lần gọi API.
- **Pub/Sub**: gửi thông điệp thời gian thực nhẹ giữa các module (ví dụ trigger notification khi có PR mới).

Redis không phải hệ quản trị cơ sở dữ liệu chính; toàn bộ dữ liệu bền vững vẫn nằm ở PostgreSQL.

## 2.18. Đóng gói và triển khai: Docker

**Docker** đóng gói ứng dụng cùng phụ thuộc thành **container** — đơn vị chạy độc lập, cách ly bằng namespace/cgroup của Linux kernel. Ưu điểm với FitAI:

- **Tính tái lập**: cùng image chạy giống nhau trên máy phát triển, CI và production; loại bỏ hiện tượng "chạy trên máy tôi thì OK".
- **Multi-stage build**: một Dockerfile có thể có nhiều stage (build → runtime), stage cuối chỉ chứa binary Go tĩnh, kích thước image chỉ vài chục MB.
- **Docker Compose**: khai báo nhiều container (backend, PostgreSQL, Redis, frontend dev server) trong một file YAML, khởi động cả stack bằng một lệnh; phù hợp môi trường phát triển cục bộ.

Chi tiết Dockerfile và Compose file cụ thể của FitAI sẽ được trình bày ở Chương 4.

## 2.19. Tổng kết công nghệ

| Công nghệ | Tầng | Vai trò trong FitAI |
|---|---|---|
| YOLOv8-Pose | AI/Model | Ước lượng tư thế 17 keypoints, chạy trên client |
| ONNX Runtime | AI/Inference | Engine inference đa nền tảng, chạy YOLOv8-Pose ở client |
| LLM (Gemini qua Vertex AI) | AI/Reasoning | Bộ não của AI Coach; function calling native |
| pgvector | AI/Storage | Vector index cho RAG |
| React + TypeScript | Frontend | UI web, tương tác người dùng, hiển thị pose overlay |
| Go | Backend | Ngôn ngữ backend, xử lý request và điều phối module |
| ConnectRPC + Protobuf | Communication | Contract-first API giữa client và server |
| PostgreSQL | Storage | CSDL chính (dữ liệu quan hệ + JSONB + vector) |
| Redis | Storage | Cache, session, pub/sub |
| Docker + Compose | Deployment | Đóng gói và điều phối môi trường |

Ba tiêu chí chung khi chọn: (i) mã nguồn mở và miễn phí, (ii) đủ trưởng thành để ổn định trong thời gian làm khóa luận, (iii) có cộng đồng và tài liệu tốt.

---

## 2.20. Tổng kết chương

Chương 2 đã trình bày các nền tảng cần thiết cho FitAI, chia theo ba phần:

- **Phần A** đặt cơ sở cho khối nhận diện thể chất và dinh dưỡng: AI/ML/DL, Computer Vision, HPE với lựa chọn YOLOv8-Pose, phân tích chất lượng động tác dựa trên biomechanics và rule-based, triển khai edge qua ONNX Runtime, và khoa học dinh dưỡng cùng hệ thống gợi ý constraint-based.
- **Phần B** đặt cơ sở cho AI Coach: kiến trúc LLM/Transformer, mẫu Agent với ReAct, tool use qua function calling, RAG cho tri thức riêng, và các kỹ thuật suy luận (CoT, reasoning model). Vai trò của Agent trong FitAI được giới hạn ở lập kế hoạch, trích xuất ngữ cảnh và sinh lời giải thích, không phải chatbot đối thoại tự do.
- **Phần C** đặt cơ sở kiến trúc phần mềm: client–server phân tách real-time/batch, modular monolith kèm DDD, contract-first với Protobuf và ConnectRPC, cùng các công nghệ nền tảng (Go, React, TypeScript, PostgreSQL, Redis, Docker).

Chương 3 sẽ dựa trên các nền tảng này để trình bày yêu cầu chi tiết và kiến trúc tổng thể của FitAI.

---

## TÀI LIỆU THAM KHẢO

[1] S. Russell and P. Norvig, *Artificial Intelligence: A Modern Approach*, 4th ed. Pearson, 2021.

[2] A. Vaswani et al., "Attention Is All You Need," in *Proc. NeurIPS*, 2017.

[3] Ultralytics, "YOLOv8 Documentation — Pose Estimation," https://docs.ultralytics.com/tasks/pose/ (truy cập 2026).