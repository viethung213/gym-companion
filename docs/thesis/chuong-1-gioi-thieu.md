# CHƯƠNG 1. GIỚI THIỆU ĐỀ TÀI

## 1.1. Đặt vấn đề

Nhu cầu tập luyện thể dục tại nhà đã tăng mạnh trong những năm gần đây, đặc biệt sau giai đoạn dịch bệnh khiến nhiều người thay đổi thói quen vận động. Trong nhóm người bắt đầu tập luyện, phần lớn gặp cùng một tình huống: **muốn tập nhưng không có huấn luyện viên riêng** — vì chi phí thuê PT cao, lịch cố định khó theo, và ở nhiều địa phương gần như không có lựa chọn PT chất lượng. Kết quả là người mới phải tự học qua video trên mạng, tự xây giáo án, tự đoán mình đang tập đúng hay sai.

Từ thực tế đó, có thể quan sát năm vấn đề phổ biến ở người mới tập:

- **Ngại đến phòng gym**: nhiều người mới e ngại ánh nhìn ở phòng tập, không biết dùng máy nào, sợ tập sai giữa nơi đông người. Kết quả là họ chọn tập ở nhà, nhưng ở nhà lại thiếu người hướng dẫn.
- **Không nắm rõ lộ trình**: người mới thường không biết nên tập bài gì trước, tập bao lâu thì có kết quả, khi nào tăng tạ, khi nào nghỉ. Sự mơ hồ về lộ trình khiến việc tập trở nên rời rạc và mất phương hướng.
- **Mất động lực trong vài tuần đầu**: khảo sát của các ứng dụng fitness cho thấy phần lớn người dùng bỏ tập trong 2–4 tuần đầu. Nguyên nhân thường không phải do bài tập quá nặng, mà do *không thấy tiến bộ*, *không có ai đồng hành*, và giáo án cố định trở nên nhàm chán hoặc không còn phù hợp khi thể trạng thay đổi. Cảm giác "tập một tuần rồi mà chưa thay đổi gì" là lý do bỏ cuộc phổ biến nhất, trong khi thực tế các thay đổi ở giai đoạn này (sức mạnh tăng, tư thế chuẩn hơn) không hiện ra trực quan nếu không có ai chỉ cho họ thấy.
- **Sai tư thế lặp lại mà không được ai chỉ ra**: dẫn tới hiệu quả tập thấp và tăng nguy cơ chấn thương khớp gối, cột sống, vai. Người mới thường không đủ kinh nghiệm để tự nhận biết lỗi qua video quay lại.
- **Ít để ý dinh dưỡng**: người mới thường tập trung vào bài tập mà bỏ qua chuyện ăn uống, hoặc mặc định "muốn giảm cân thì ăn ít, muốn tăng cơ thì ăn nhiều" mà không biết lượng calo và tỷ lệ đạm/tinh bột/chất béo phù hợp với mình. Kết quả là dù tập chăm chỉ vẫn không đạt mục tiêu, và họ dễ đổ lỗi cho việc tập — càng dễ bỏ cuộc. Giáo án và chế độ dinh dưỡng gần như không bao giờ được thiết kế đồng bộ.

Trong khi đó, các công nghệ hỗ trợ đã trưởng thành đáng kể: các mô hình ước lượng tư thế người (Human Pose Estimation) có thể chạy trên camera điện thoại thông thường; các mô hình ngôn ngữ lớn và kiến trúc *agentic* cho phép xây dựng "huấn luyện viên ảo" có khả năng lập kế hoạch tập luyện cá nhân hóa, điều chỉnh giáo án theo tiến độ, và sinh lời giải thích tự nhiên cho mỗi quyết định. Đây là bối cảnh trực tiếp thúc đẩy đề tài này ra đời — hướng vào **nhóm người mới bắt đầu tập tại nhà**, những người cần nhất một "người đồng hành" có thể quan sát, nhắc nhở và giữ cho họ tiếp tục.

## 1.2. Khảo sát các giải pháp hiện có

Trên Google Play, App Store và các website chính thức, các ứng dụng hỗ trợ tập luyện hiện nay có thể được chia thành bốn nhóm chính:

**Nhóm 1 — Thư viện bài tập có video hướng dẫn.** Đại diện: *Nike Training Club*, các ứng dụng "Home Workout" phổ biến. Người dùng xem video và làm theo. Ưu điểm là kho nội dung phong phú, dễ tiếp cận. Hạn chế: phản hồi hoàn toàn một chiều, không biết người tập làm đúng hay sai.

**Nhóm 2 — Sinh giáo án cá nhân hóa dựa trên quy tắc.** Đại diện: *Freeletics*, *Fitbod*. Ứng dụng chọn bài và khối lượng dựa trên mục tiêu, thiết bị và lịch sử tập. Ưu điểm là giáo án phù hợp với từng người. Hạn chế: không quan sát người tập, "cá nhân hóa" dừng ở mức lựa chọn theo mẫu.

**Nhóm 3 — Phân tích tư thế qua camera.** Đại diện: *Kaia Health*, *Onyx*. Sử dụng camera để đánh giá động tác trên một tập bài giới hạn. Ưu điểm: có phản hồi về chất lượng động tác. Hạn chế: danh mục bài hẹp, phản hồi thường chỉ ở dạng chấm điểm hoặc chú thích ngắn, và không tích hợp với gợi ý dinh dưỡng.

**Nhóm 4 — Theo dõi dinh dưỡng.** Đại diện: *MyFitnessPal*, *Lifesum*. Ghi nhật ký ăn uống, tính calo. Ưu điểm: cơ sở dữ liệu thực phẩm lớn. Hạn chế: hoạt động độc lập, không liên kết với dữ liệu tập luyện.

## 1.3. Khoảng trống nhận diện được

Từ khảo sát trên, có thể tổng hợp các khoảng trống có căn cứ:

1. **Chức năng bị chia mảnh**: phân tích tư thế, đề xuất giáo án và gợi ý dinh dưỡng nằm ở các ứng dụng khác nhau, buộc người dùng nhập liệu chéo.
2. **Phản hồi chưa mang tính đối thoại**: đa số dừng ở chấm điểm hoặc video một chiều; người dùng không thể *hỏi lại* "tôi sai ở đâu", "sao động tác này khó".
3. **Cá nhân hóa còn dạng thống kê**: chọn bài theo template thay vì lập luận theo hoàn cảnh của người dùng.
4. **Hỗ trợ tiếng Việt và ngữ cảnh Việt Nam còn hạn chế**: đa số ứng dụng ưu tiên tiếng Anh, cơ sở dữ liệu thực phẩm thiên về khẩu vị phương Tây.
5. **Rào cản thiết bị và phí**: một số giải pháp phụ thuộc thiết bị đeo hoặc yêu cầu subscription cho các tính năng nâng cao.

Những khoảng trống này chính là phạm vi mà đề tài lựa chọn để nghiên cứu và hiện thực hóa — không nhằm mục tiêu vượt qua các sản phẩm thương mại, mà để **kiểm chứng khả năng ghép nối các công nghệ AI hiện đại thành một hệ thống thống nhất**.

## 1.4. Động cơ đề tài

Đề tài được định vị là một **nguyên mẫu nghiên cứu (research prototype)** trong khuôn khổ khóa luận đại học. Động cơ chính bao gồm:

- **Về học thuật**: kiểm chứng tính khả thi của việc tích hợp ba nhánh công nghệ — Computer Vision (phân tích tư thế), Agentic AI (huấn luyện viên ảo lập kế hoạch và giải thích quyết định), và Recommender System (gợi ý dinh dưỡng) — trong một pipeline thống nhất phục vụ hỗ trợ tập luyện.
- **Về giáo dục**: cung cấp một case-study khép kín để tham khảo về cách kết nối các thành phần AI hiện đại thành sản phẩm có ý nghĩa thực tiễn.
- **Về xã hội (giá trị tiềm năng)**: hạ rào cản tiếp cận huấn luyện chất lượng cho người tập tại nhà, đặc biệt trong bối cảnh Việt Nam.

Cần nhấn mạnh: đề tài **không cạnh tranh trực tiếp** với các sản phẩm thương mại vốn có đội ngũ R&D và dữ liệu quy mô lớn. Giá trị nằm ở tính **tích hợp** và tính **khả thi được kiểm chứng**.

## 1.5. Đề xuất hệ thống FitAI

**FitAI** là hệ thống hỗ trợ tập luyện thông minh, ở mức khái niệm gồm ba khối chức năng:

- **Khối phân tích tư thế**: sử dụng camera điện thoại/laptop thông thường để nhận diện và đánh giá chất lượng động tác trên một tập bài tập giới hạn.
- **Khối huấn luyện viên ảo (Agentic AI)**: tiếp nhận đầu ra của khối phân tích tư thế và phản hồi của người dùng để giải thích lỗi, điều chỉnh giáo án và trả lời câu hỏi bằng ngôn ngữ tự nhiên.
- **Khối gợi ý dinh dưỡng**: đề xuất khẩu phần và nhóm thực phẩm phù hợp với mục tiêu tập luyện, dựa trên cơ sở dữ liệu thực phẩm mở.

**Nhóm người dùng mục tiêu**: người tập tại nhà, không có huấn luyện viên, sở hữu thiết bị phổ thông (smartphone hoặc laptop có camera), muốn được hướng dẫn tư thế và có gợi ý ăn uống đồng bộ với giáo án.

**Điểm khác biệt chính** so với các giải pháp hiện có:

- Tích hợp ba khối chức năng trong cùng một hệ thống, thay vì để người dùng tự ghép từ nhiều ứng dụng.
- Phản hồi và giải thích quyết định bằng ngôn ngữ tự nhiên, thay vì chỉ chấm điểm.
- Điều chỉnh giáo án thích ứng theo hoàn cảnh thực tế của người dùng (chấn thương phát sinh, dụng cụ hiện có, mục tiêu thay đổi, tiến độ thực tế) — thay vì áp một giáo án cố định.
- Không yêu cầu thiết bị đeo chuyên dụng.

Các chi tiết về mô hình, thuật toán và kiến trúc sẽ được trình bày ở các chương sau.

## 1.6. Mục tiêu nghiên cứu

### 1.6.1. Mục tiêu tổng quát

Xây dựng một nguyên mẫu hệ thống hỗ trợ tập luyện có khả năng: (i) phân tích tư thế người dùng qua camera thông thường, (ii) lập và điều chỉnh giáo án tập luyện cá nhân hóa nhờ Agentic AI, (iii) gợi ý chế độ dinh dưỡng phù hợp với mục tiêu tập luyện — qua đó kiểm chứng tính khả thi của việc tích hợp các công nghệ này trong một hệ thống thống nhất.

### 1.6.2. Mục tiêu cụ thể

1. Khảo sát hiện trạng ứng dụng AI và Computer Vision trong lĩnh vực hỗ trợ tập luyện.
2. Xác định tập bài tập đại diện để phân tích tư thế trong phạm vi nguyên mẫu.
3. Thiết kế kiến trúc hệ thống tích hợp ba khối: phân tích tư thế, agent huấn luyện, gợi ý dinh dưỡng.
4. Hiện thực hóa nguyên mẫu ở mức đủ để trình diễn và đánh giá thực nghiệm.
5. Đánh giá nguyên mẫu trên các tiêu chí: độ chính xác phân tích tư thế trên tập bài đã chọn, tính hữu ích của phản hồi từ agent, mức độ phù hợp của gợi ý dinh dưỡng, và trải nghiệm tổng thể của người dùng thử.

## 1.7. Phạm vi nghiên cứu

**Đề tài bao gồm**:

- Phân tích tư thế cho một tập bài tập giới hạn (dự kiến 8–12 bài phổ biến như squat, push-up, plank, lunge, glute bridge…, sẽ được chốt ở Chương 3).
- Sử dụng camera đơn (monocular) trên điện thoại hoặc laptop, không dùng cảm biến đeo hoặc camera chiều sâu.
- Agent huấn luyện đảm nhiệm lập kế hoạch tập luyện (lộ trình dài hạn, lịch tuần, giáo án buổi), điều chỉnh giáo án khi ngữ cảnh thay đổi và sinh lời giải thích tự nhiên cho các quyết định.
- Gợi ý dinh dưỡng ở mức chọn nhóm thực phẩm và khẩu phần theo mục tiêu tập luyện, dựa trên cơ sở dữ liệu thực phẩm mở.
- Xử lý video trên thiết bị người dùng (edge) để bảo vệ riêng tư; chỉ đồng bộ tọa độ khớp và số liệu tổng hợp lên server.

**Đề tài không bao gồm**:

- Chẩn đoán y khoa hoặc kê đơn dinh dưỡng cá nhân hóa theo bệnh lý.
- Phân tích tư thế cho các môn thể thao chuyên sâu (võ thuật, thể hình thi đấu, phục hồi chức năng).
- Tích hợp thiết bị đeo, cảm biến nhịp tim, EMG.
- Triển khai ở quy mô sản phẩm thương mại (CI/CD toàn diện, phát hành lên store, marketing).

**Giới hạn**:

- Dữ liệu đánh giá thu thập từ nhóm người dùng thử nghiệm nhỏ, không đại diện dân số chung.
- Độ chính xác phân tích tư thế phụ thuộc điều kiện ánh sáng, góc camera và chất lượng thiết bị.
- Gợi ý dinh dưỡng mang tính tham khảo, không thay thế tư vấn của chuyên gia y tế.

## 1.8. Đóng góp của đề tài

**Về mặt học thuật**:

- Khảo sát có hệ thống hiện trạng ứng dụng AI, Computer Vision và Agentic AI trong lĩnh vực hỗ trợ tập luyện.
- Đề xuất và kiểm chứng thực nghiệm mô hình tích hợp ba khối chức năng: phân tích tư thế, agent huấn luyện, gợi ý dinh dưỡng, trên cùng một pipeline.
- Ghi nhận các quan sát về khả năng, giới hạn và các bài toán mở cho hướng nghiên cứu tiếp theo.

**Về mặt kỹ thuật**:

- Xây dựng nguyên mẫu hoạt động được, có thể trình diễn.
- Tổng hợp một tập tiêu chí đánh giá cho hệ thống lai kết hợp Computer Vision, Agent và Recommender trong bối cảnh fitness.
- Chuẩn bị nền tảng mã nguồn có khả năng mở rộng cho các đề tài tiếp theo.

Đề tài không khẳng định vượt trội về độ chính xác so với các sản phẩm thương mại; giá trị đóng góp nằm ở **tính tích hợp** và **tính khả thi được kiểm chứng** trong khuôn khổ một nguyên mẫu nghiên cứu.

## 1.9. Bố cục khóa luận

Ngoài Chương 1 đã giới thiệu, khóa luận gồm các chương tiếp theo:

- **Chương 2 — Cơ sở lý thuyết**: trình bày nền tảng về Computer Vision, Human Pose Estimation, Agentic AI và Recommender System liên quan đến đề tài.
- **Chương 3 — Phân tích và thiết kế hệ thống**: xác định yêu cầu chức năng, phi chức năng và kiến trúc tổng thể của FitAI.
- **Chương 4 — Hiện thực hóa hệ thống**: chi tiết cài đặt ba khối chức năng chính.
- **Chương 5 — Thực nghiệm và đánh giá**: kịch bản thử nghiệm, kết quả và phân tích.
- **Chương 6 — Kết luận và hướng phát triển**: tổng kết đóng góp, hạn chế và các hướng mở rộng.
