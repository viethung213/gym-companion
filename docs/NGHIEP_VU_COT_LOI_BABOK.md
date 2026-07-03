# ĐẶC TẢ YÊU CẦU NGHIỆP VỤ CỐT LÕI (CORE BRD) - FITAI
### TIÊU CHUẨN: BABOK® GUIDE V3.0 Compliant | Phiên bản: 1.7.0-OPT (Tối ưu Token)

---

## KIỂM SOÁT TÀI LIỆU
- **Mã tài liệu**: FITAI-BRD-CORE-001 | **Ngày hiệu lực**: 03/07/2026
- **Trạng thái**: Approved | **Phân loại**: Internal Confidential
- **Lịch sử đổi mới chính**:
  * v1.0 (27/06/2026): Hoàn thiện 7 module nghiệp vụ gốc.
  * v1.3 (01/07/2026): Chuyển sang cơ chế sinh lộ trình tổng quan & sinh giáo án chi tiết theo buổi.
  * v1.6 (02/07/2026): Thêm Warm-up/Cool-down, xử lý bỏ tập, Onboarding tối giản (hỏi thiết bị/dị ứng theo ngữ cảnh), Module Admin và Tái cấu trúc quy trình 3.4 thành các Quy tắc nghiệp vụ BR-AC-04 -> BR-AC-08 (CR & Signals B1-B4), tổng quát hóa BR-WL-02.
  * v1.7 (03/07/2026): Bổ sung luồng tập phi AI (timer/nhạc/hướng dẫn), cơ chế điểm XP cơ bản/thưởng, và tư vấn món ăn ngoài.

---

## 1. BỐI CẢNH & MỤC TIÊU NGHIỆP VỤ
- **Bối cảnh**: Nền tảng số hỗ trợ tập luyện + dinh dưỡng cá nhân hóa tự động bằng AI & Computer Vision, giúp giải quyết rào cản chi phí PT và duy trì động lực cho người mới tập.
- **Mục tiêu**:
  * **OB-01 (Chi phí)**: Giảm 90% chi phí hướng dẫn so với thuê PT truyền thống.
  * **OB-02 (An toàn)**: Giảm chấn thương nhờ phân tích góc khớp sửa lỗi tư thế thời gian thực.
  * **OB-03 (Duy trì)**: Tỷ lệ người dùng tiếp tục tập luyện sau 30 ngày đạt $\ge 40\%$.
  * **OB-04 (Tư thế)**: Chuẩn hóa tư thế cho người mới qua camera phản hồi tức thì.
  * **OB-05 (Dinh dưỡng)**: Cá nhân hóa sâu thực đơn theo thể trạng, ngân sách và mục tiêu.

---

## 2. PHÂN TÍCH TÁC NHÂN (STAKEHOLDERS)
- **ACT-01 (User)**: Người tập. Nhập chỉ số, check-in, tập dưới camera, theo dõi sức khỏe & ăn uống. (High)
- **ACT-02 (AI Coach)**: Hệ thống. Phân tích hiệu suất, sinh lộ trình/lịch tập, động viên, chỉnh giáo án. (High)
- **ACT-03 (AI Camera)**: Hệ thống. Xử lý video, tracking khớp (33 điểm), đếm rep, đo ROM, phát hiện lỗi tư thế. (High)
- **ACT-04 (AI Nutrition)**: Hệ thống. Tính calo/macro cá nhân hóa, gợi ý & luân chuyển thực đơn không trùng. (Medium)
- **ACT-05 (Admin)**: Quản trị viên. Quản lý thư viện bài tập, thực đơn, kiểm tra dữ liệu và bảo mật. (Low)

---

## 3. BẢN ĐỒ QUY TRÌNH NGHIỆP VỤ

### 3.1 Quy trình Khởi tạo (Onboarding & Planning)
1. User nhập thông tin cơ bản, chỉ số cơ thể, mục tiêu (Tăng cơ/Giảm mỡ) & khung giờ tập cố định.
2. User khai báo chấn thương cũ hoặc bệnh lý mãn tính.
3. AI Coach tính toán `User Fitness Score` & khởi tạo Lộ trình tổng quan 4 tuần, Lịch tập tuần và Gợi ý dinh dưỡng.

### 3.2 Quy trình Luyện tập (Workout Execution)
1. User check-in & cấu hình playlist âm nhạc.
2. Đối với bài tập có hỗ trợ AI Camera (Nhánh AI):
   - User bật camera trước/sau, căn chỉnh khoảng cách (1.5m - 2m) và ánh sáng.
   - AI Camera tracking khung xương, ước lượng tạ thực tế, đếm rep, tính % hoàn thiện chuyển động (ROM %).
   - Nếu sai tư thế: Audio Ducking (giảm nhạc nền) + Phát giọng nói sửa lỗi thời gian thực (độ trễ <500ms).
   - Nếu đúng tư thế: Cộng rep, tính Form Score.
   - User xác nhận kết quả Set (AI điền tự động).
3. Đối với bài tập phi AI (Nhánh tự ghi nhận):
   - Giao diện hiển thị trình bấm giờ (timer) đếm ngược theo set hoặc thời gian nghỉ, kết hợp phát nhạc nền.
   - Hiển thị video/hướng dẫn bài tập để người dùng tập theo.
   - User tự thực hiện và xác nhận kết quả set thủ công.
4. Nghỉ ngơi → Lặp lại cho đến khi hết giáo án → Nhận Post-session Report sau khi kết thúc buổi tập.

### 3.3 Quy trình Sinh giáo án theo buổi (Just-In-Time Workout Generation)
1. Trigger: Đến ngày tập / User mở app.
2. AI Coach hỏi/nhận trạng thái sức khỏe (chấn thương mới, độ phục hồi) & phân tích dữ liệu RPE/Form buổi trước.
3. AI Coach tự động sinh giáo án chi tiết hôm nay (bài tập, set, rep, tạ gợi ý).
4. User nhận giáo án và chuẩn bị thực hiện (chuyển sang Quy trình 3.2).

### 3.4 Quy trình Đánh giá & Điều chỉnh Lộ trình (Adaptive Review Cycle)
1. **Trigger A (Cuối chu kỳ 4 tuần)**: AI Coach tính Completion Rate (CR) để tự động nâng/hạ hoặc cấu hình lại lộ trình 4 tuần kế tiếp theo quy tắc **BR-AC-04**.
2. **Trigger B (Giữa chu kỳ - Event-driven)**: Hệ thống liên tục quét 4 tín hiệu hành vi độc lập (Không hoạt động, Lịch không tương thích, Tập quá tải, Tiến bộ đình trệ) để đề xuất điều chỉnh nhanh giáo án theo các quy tắc **BR-AC-05** -> **BR-AC-08**.

---

## 4. YÊU CẦU CHỨC NĂNG CHI TIẾT (FR)

### Module 1: Quản lý Người dùng & Hồ sơ
| Mã | Nghiệp vụ chi tiết | MoSCoW |
|---|---|---|
| **FR-UM-01** | **Đăng ký/Đăng nhập**: Qua Email, SĐT (xác thực OTP) và liên kết MXH (Google, Apple, Facebook). | M |
| **FR-UM-02** | **Hồ sơ sức khỏe**: Khai báo tuổi, giới tính, chiều cao, cân nặng, mục tiêu, chấn thương/bệnh lý. | M |
| **FR-UM-03** | **Khung giờ cố định**: Bắt buộc chọn tối thiểu 1 khung giờ tập cố định trong ngày để nhắc lịch. | M |
| **FR-UM-04** | **Nhắc lịch tự động**: Push Notification trước 15 phút theo phong cách AI Coach đã chọn. | S |

### Module 2: AI Coach cá nhân
| Mã | Nghiệp vụ chi tiết | MoSCoW |
|---|---|---|
| **FR-AC-01** | **Khởi tạo kế hoạch**: Sinh Lộ trình 4 tuần (mốc định hướng) & Lịch tập tuần (phân bổ cơ). Không sinh chi tiết bài ở bước này. | M |
| **FR-AC-02** | **Tự động điều chỉnh**: Phân tích hiệu suất tập để tăng/giảm tạ, thay bài tập hoặc chèn Deload Week. | M |
| **FR-AC-03** | **Bài tập thay thế**: Loại bỏ bài tác động vào vùng chấn thương đột xuất cho đến khi báo phục hồi. | S |
| **FR-AC-04** | **Đồng hành**: Gửi tin nhắn động viên cá nhân hóa dựa trên dữ liệu thực tế (PR, quay lại sau nghỉ dài). | S |
| **FR-AC-05** | **Phong cách Coach**: Cho chọn Drill Sergeant (nghiêm khắc), Best Friend (thân thiện), Data Analyst (khoa học). | C |
| **FR-AC-06** | **Sinh giáo án theo buổi**: Sinh bài tập, set, rep, tạ gợi ý trước buổi tập. AI Coach hỏi 1-2 câu ngắn về thiết bị & dị ứng thực phẩm theo ngữ cảnh nếu chưa có thông tin. | M |
| **FR-AC-07** | **Warm-up/Cool-down**: Tự chèn khởi động (5-10') và giãn cơ (5') theo nhóm cơ sẽ tập của giáo án. | M |

### Module 3: AI Camera Coach (Phân tích tư thế)
| Mã | Nghiệp vụ chi tiết | MoSCoW |
|---|---|---|
| **FR-CC-01** | **Tracking khung xương**: Phân tích video xác định 33 điểm khớp chính trên cơ thể. | M |
| **FR-CC-02** | **Đo lường góc ROM**: Tính toán biên độ chuyển động (ROM) khớp & tỷ lệ hoàn thiện rep. | M |
| **FR-CC-03** | **Phát hiện lỗi**: So so sánh tọa độ khớp với mô hình chuẩn để phát hiện lỗi kỹ thuật (võng lưng, gối chụm...). | M |
| **FR-CC-04** | **Cảnh báo real-time**: Overlay hình ảnh & âm thanh hướng dẫn sửa lỗi tức thì với độ trễ <500ms. | M |
| **FR-CC-05** | **Chấm điểm Form**: Tính điểm Form Score (0-100) cho mỗi rep dựa trên ROM, căn chỉnh khớp, tốc độ. | S |

### Module 4: Quản lý Buổi tập (Workout Logging)
| Mã | Nghiệp vụ chi tiết | MoSCoW |
|---|---|---|
| **FR-WL-01** | **Ghi chép tự động**: Điền rep, % hoàn thiện và ước lượng tạ thực tế (qua kích thước đĩa tạ & tốc độ nâng). | M |
| **FR-WL-02** | **Ghi chép thủ công**: Cho phép sửa kết quả set. Hỗ trợ luồng tập phi AI tích hợp trình bấm giờ (timer), âm nhạc và hướng dẫn trực quan. | M |
| **FR-WL-03** | **Tương tác âm thanh**: Audio Ducking tự giảm nhạc nền khi AI phát giọng nói cảnh báo tư thế. | S |
| **FR-WL-04** | **Ghi nhận PR**: Tính 1RM ước tính (Epley Formula) sau buổi và vinh danh ăn mừng nếu đạt PR mới. | S |

### Module 5: Dinh dưỡng AI
| Mã | Nghiệp vụ chi tiết | MoSCoW |
|---|---|---|
| **FR-NU-01** | **Tính kcal cá nhân**: Tính TDEE/macro hàng ngày theo công thức Mifflin-St Jeor & mức vận động thực tế. | M |
| **FR-NU-02** | **Đa lựa chọn**: Gợi ý bữa ăn linh hoạt (tự chuẩn bị, ăn ngoài) chia theo 3 mức giá; ưu tiên đề xuất sản phẩm/gói ăn sẵn có hoặc đối tác. | S |
| **FR-NU-03** | **Anti-Repetition**: Không lặp lại nguồn protein chính trong 7 ngày, tinh bột 5 ngày, chủ đề món 3 ngày. | M |
| **FR-NU-04** | **Nhật ký ăn uống**: Log bữa ăn thực tế bằng cách tìm kiếm món ăn hoặc quét mã vạch sản phẩm. | S |

### Module 6: Theo dõi Tiến trình
| Mã | Nghiệp vụ chi tiết | MoSCoW |
|---|---|---|
| **FR-PT-01** | **Ghi nhận chỉ số**: Cập nhật cân nặng, % mỡ cơ thể, số đo các vòng cơ bắp & lưu ảnh tiến trình. | M |
| **FR-PT-02** | **Phân tích xu hướng**: Vẽ biểu đồ xu hướng biến động cân nặng, sức mạnh (1RM) và điểm Form trung bình. | S |
| **FR-PT-03** | **AI phân tích sâu**: Gửi báo cáo định kỳ đánh giá tiến trình kèm lời khuyên tối ưu hóa. | S |

### Module 7: Quản trị Hệ thống (Admin)
| Mã | Nghiệp vụ chi tiết | MoSCoW |
|---|---|---|
| **FR-SM-01** | **Thư viện bài tập**: CRUD bài tập (nhóm cơ, video hướng dẫn, tọa độ khớp chuẩn, dụng cụ, bài thay thế). Cần Admin duyệt mới kích hoạt. | M |
| **FR-SM-02** | **Thư viện dinh dưỡng**: CRUD thực phẩm (kcal, macro, phân loại chay/Halal, nhãn dị ứng). | M |
| **FR-SM-03** | **Dashboard giám sát**: Theo dõi tỉ lệ tracking thành công, độ trễ cảnh báo (<500ms), tỉ lệ lỗi hệ thống. | S |

---

## 5. QUY TẮC NGHIỆP VỤ (BUSINESS RULES - BR)

| Mã | Module | Nội dung quy tắc nghiệp vụ |
|---|---|---|
| **BR-UM-01** | Hồ sơ | Hồ sơ sức khỏe phải hoàn thiện **$\ge 80\%$** trước khi kích hoạt AI Coach và sinh lộ trình tập đầu tiên. |
| **BR-AC-01** | Tập luyện | Lịch tập tối đa **6 buổi/tuần**; bắt buộc có ít nhất **1 ngày nghỉ hoàn toàn** trong tuần để phục hồi cơ bắp. |
| **BR-AC-02** | Tiến độ | Tăng tiến Progressive Overload do AI đề xuất không vượt quá **10% tổng volume** của tuần trước đó. |
| **BR-CC-01** | AI Camera | Rep hợp lệ để đếm số khi biên độ chuyển động (ROM) khớp đạt ít nhất **$\ge 70\%$** so với biên độ tiêu chuẩn. |
| **BR-CC-02** | Chống gian lận | Tỷ lệ frame nhận diện khớp hợp lệ < 50% trong buổi tập dưới camera → Đánh dấu "Không đạt chuẩn xác thực" (Chỉ áp dụng khi sử dụng AI Camera, trừ bài nằm sàn/phòng tối được chuyển sang ghi nhận thủ công). |
| **BR-NU-01** | Dinh dưỡng | AI Nutrition tuyệt đối không gợi ý thực đơn tổng năng lượng dưới **1,200 kcal/ngày** cho bất kỳ đối tượng nào. |
| **BR-NU-02** | Dinh dưỡng | Nguồn protein chính đã ăn trong Meal History sẽ bị khóa không gợi ý lại trong vòng **7 ngày tiếp theo**. |
| **BR-AC-03** | Tập luyện | Giáo án các buổi bỏ tập đánh dấu là "Bỏ qua", **không tự động dồn/bù** vào ngày tiếp theo nếu chưa có xác nhận từ người dùng. |
| **BR-AC-04** | Lộ trình | **Quy tắc điều chỉnh CR cuối chu kỳ (Trigger A)**:<br>- **CR < 40%**: Hỏi lý do bỏ tập, chờ phản hồi mới đề xuất giảm số buổi/tuần và rút ngắn thời lượng giáo án.<br>- **40% <= CR < 70%**: Giữ nguyên số buổi, giảm tải lượng 10-15%, chèn xen kẽ buổi Express 30 phút. Tự động sinh lộ trình mới.<br>- **70% <= CR < 90%**: Giữ nguyên cấu trúc, tăng Progressive Overload <= 10% theo BR-AC-02. Tự sinh lộ trình.<br>- **CR >= 90%**: Đề xuất tăng cường độ hoặc thêm 1 buổi/tuần (không vượt BR-AC-01), gắn badge "Xuất sắc". |
| **BR-AC-05** | Lộ trình | **Signal B1 (Không hoạt động 7 ngày liên tiếp)**: AI Coach gửi tin nhắn check-in theo phong cách đã chọn, đề xuất 3 phương án: (a) tập tiếp từ buổi bỏ gần nhất, (b) đặt lại lịch tuần này, (c) tạm dừng lộ trình (Pause tối đa 4 tuần). Không tự chỉnh lịch nếu user chưa phản hồi. |
| **BR-AC-06** | Lịch tập | **Signal B2 (Lịch không tương thích)**: User bỏ tập cùng 1 ngày trong tuần $\ge 3$ lần liên tiếp → AI đề xuất dời slot ngày đó sang ngày khác. Nếu đồng ý thì cập nhật lịch tuần, nếu từ chối thì giữ nguyên và không hỏi lại. |
| **BR-AC-07** | Tập luyện | **Signal B3 (Tập quá tải - Overtraining)**: Kích hoạt khi user tập $\ge 2$ buổi/ngày hoặc RPE trung bình $\ge 8.5$ liên tục $\ge 5$ buổi → Cảnh báo quá tải, bắt buộc chèn 1 ngày nghỉ trong lịch kế tiếp, gợi ý Active Recovery. |
| **BR-AC-08** | Lộ trình | **Signal B4 (Tiến bộ đình trệ - Plateau)**: Sức mạnh (1RM) và Form trung bình không tăng trong 3 tuần liên tiếp (chỉ tính tuần có CR $\ge 70\%$) → AI Coach gợi ý chọn: (a) Deload Week (giảm 40% tải lượng 1 tuần), (b) Đổi biến thể bài tập tương đương, (c) Tăng set giữ tạ. |
| **BR-WL-01** | Buổi tập | **Giới hạn thời gian**: Cảnh báo kết thúc sau 90 phút (người mới) hoặc 180 phút (người cũ). Đạt 240 phút không tương tác → Tự động đóng buổi tập, lưu nhãn `Anomalous Session`, loại dữ liệu khỏi tính Overload, buổi sau bắt buộc Recovery. |
| **BR-WL-02** | Buổi tập | **Phát hiện tải lượng luyện tập (Training Load) bất thường**: Tải lượng buổi tập > 250% trung bình 5 buổi gần nhất có cùng nhóm cơ/mục tiêu → Yêu cầu xác nhận trước khi lưu; bắt buộc chèn $\ge 1$ ngày nghỉ hoàn toàn cho nhóm cơ đó. |
| **BR-WL-03** | Buổi tập | **Ghi nhận bài tập phi AI**: Đảm bảo tính liên tục của dữ liệu hiệu suất tổng thể. Các bài tập phi AI không ghi nhận điểm Form Score (báo N/A/Trống), chỉ ghi nhận số set, rep/thời gian thực tế và mức tạ (do người dùng tự nhập) để làm cơ sở tính Tải lượng tập luyện (Training Load) và Overload. |
| **BR-NU-03** | Dinh dưỡng | **Tư vấn Dinh dưỡng**: AI Coach hỗ trợ tư vấn chi tiết định lượng cho đồ ăn tự chuẩn bị hoặc quán ngoài tiệm, nhưng luôn kèm đề xuất sản phẩm đối tác tiện lợi tương đương nếu có. |

---

## 6. YÊU CẦU DỮ LIỆU NGHIỆP VỤ (DATA)
- **Đầu vào (Inputs)**:
  * Profile: Chỉ số cơ thể, mục tiêu, chấn thương/bệnh lý, `experience_level`, khung giờ cố định. (`equipment_list` & `food_restrictions` thu thập dần qua chatbot).
  * Video: Luồng video $\ge 720\text{p}$, $30\text{fps}$.
  * RPE: Đánh giá gắng sức (1-10) sau set/buổi.
  * Nhật ký ăn uống (Meal Logs).
- **Đầu ra (Outputs)**:
  * Lộ trình 4 tuần & Lịch tập tuần (phân bổ cơ).
  * Giáo án buổi: Bài tập, set, rep, tạ gợi ý, video demo.
  * Thực đơn ngày: 3 bữa chính + 1 bữa phụ (3 mức giá, chi tiết macro/calo).
  * Cảnh báo sửa tư thế (Visual Overlay + Audio Alert).
  * Báo cáo buổi tập: Tổng time, volume, calo, Form trung bình, lỗi phổ biến, lời khuyên phục hồi.

---

## 7. GIẢ ĐỊNH & RÀNG BUỘC (CONSTRAINTS)
- **Assumption-01**: Khoảng cách tập cách camera 1.5m - 2m, đủ sáng.
- **Assumption-02**: Thiết bị tối thiểu iOS 14 / Android 8.0, camera hoạt động bình thường.
- **Assumption-03 (Thu thập thông tin dần)**: Thông tin không bắt buộc trong Onboarding được hỏi dần qua hội thoại ngữ cảnh. Hệ thống luôn có phương án dự phòng (bài không dụng cụ, thực đơn phổ thông) khi thiếu dữ liệu.
- **Constraint-01 (Y tế)**: Không đưa ra lời khuyên hoặc chẩn đoán y khoa.
- **Constraint-02 (Bảo mật)**: Xử lý video on-device (Edge AI); chỉ gửi tọa độ khớp dạng số về server.

---
*Tài liệu Đặc tả Yêu cầu Nghiệp vụ Cốt lõi theo chuẩn BABOK v3.0 – Cập nhật lần cuối ngày 02/07/2026*
