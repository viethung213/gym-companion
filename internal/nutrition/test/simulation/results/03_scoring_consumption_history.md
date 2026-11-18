# BẢNG GHI LỊCH SỬ TIÊU THỤ & HỆ SỐ SUY GIẢM (MÔ HÌNH MỚI - MEAL_LOGS)

> Mô hình mới không dùng bảng lockout mà đọc trực tiếp từ `meal_logs` và tính suy giảm hàm mũ:
> $$R(i) = \sum_{k=1}^n s(i, i_k) e^{-\Delta t_k / \tau}$$ với $\tau = 2.0$ ngày.

| STT | Món ăn | Danh sách nguyên liệu | Ngày tiêu thụ | Phạt lặp lại sau 1 ngày (e^-1/2) | Phạt lặp lại sau 3 ngày (e^-3/2) | Phạt lặp lại sau 5 ngày (e^-5/2) |
| :---: | :--- | :--- | :---: | :---: | :---: | :---: |
| 1 | Ức gà không da tươi | Ức gà không da tươi | 2026-11-02 | 0.607 (-39%) | 0.223 (-78%) | 0.082 (-92%) |
| 2 | Yến mạch cán vỡ Quick Oats | Yến mạch cán vỡ Quick Oats | 2026-11-02 | 0.607 (-39%) | 0.223 (-78%) | 0.082 (-92%) |
| 3 | Thịt heo thăn nạc | Thịt heo thăn nạc | 2026-11-02 | 0.607 (-39%) | 0.223 (-78%) | 0.082 (-92%) |
| 4 | Bánh mì nguyên cám Whole Wheat | Bánh mì nguyên cám Whole Wheat | 2026-11-02 | 0.607 (-39%) | 0.223 (-78%) | 0.082 (-92%) |
| 5 | Cá ngừ đại dương tươi | Cá ngừ đại dương tươi | 2026-11-03 | 0.607 (-39%) | 0.223 (-78%) | 0.082 (-92%) |
| 6 | Yến mạch nguyên hạt Rolled Oats | Yến mạch nguyên hạt Rolled Oats | 2026-11-03 | 0.607 (-39%) | 0.223 (-78%) | 0.082 (-92%) |
| 7 | Đùi gà tháo da | Đùi gà tháo da | 2026-11-03 | 0.607 (-39%) | 0.223 (-78%) | 0.082 (-92%) |
| 8 | Hạt kiều mạch (Buckwheat) | Hạt kiều mạch (Buckwheat) | 2026-11-03 | 0.607 (-39%) | 0.223 (-78%) | 0.082 (-92%) |
| 9 | Cá ngừ ngâm nước khoáng (Hộp) | Cá ngừ ngâm nước khoáng (Hộp) | 2026-11-04 | 0.607 (-39%) | 0.223 (-78%) | 0.082 (-92%) |
| 10 | Đậu Gà luộc (Chickpeas) | Đậu Gà luộc (Chickpeas) | 2026-11-04 | 0.607 (-39%) | 0.223 (-78%) | 0.082 (-92%) |
| 11 | Gà tây phi lê | Gà tây phi lê | 2026-11-04 | 0.607 (-39%) | 0.223 (-78%) | 0.082 (-92%) |
| 12 | Đậu đỏ luộc | Đậu đỏ luộc | 2026-11-04 | 0.607 (-39%) | 0.223 (-78%) | 0.082 (-92%) |
| 13 | Seitan (Mì căn đạm lúa mì) | Seitan (Mì căn đạm lúa mì) | 2026-11-05 | 0.607 (-39%) | 0.223 (-78%) | 0.082 (-92%) |
| 14 | Yến mạch cán vỡ Quick Oats | Yến mạch cán vỡ Quick Oats | 2026-11-05 | 0.607 (-39%) | 0.223 (-78%) | 0.082 (-92%) |
| 15 | Cánh gà nướng | Cánh gà nướng | 2026-11-05 | 0.607 (-39%) | 0.223 (-78%) | 0.082 (-92%) |
| 16 | Đậu Lăng đỏ (Lentils) | Đậu Lăng đỏ (Lentils) | 2026-11-05 | 0.607 (-39%) | 0.223 (-78%) | 0.082 (-92%) |
| 17 | Thịt vịt bỏ da | Thịt vịt bỏ da | 2026-11-06 | 0.607 (-39%) | 0.223 (-78%) | 0.082 (-92%) |
| 18 | Bánh mì sandwich trắng | Bánh mì sandwich trắng | 2026-11-06 | 0.607 (-39%) | 0.223 (-78%) | 0.082 (-92%) |
| 19 | Thịt gà xay nạc | Thịt gà xay nạc | 2026-11-06 | 0.607 (-39%) | 0.223 (-78%) | 0.082 (-92%) |
| 20 | Bánh mì nguyên cám Whole Wheat | Bánh mì nguyên cám Whole Wheat | 2026-11-06 | 0.607 (-39%) | 0.223 (-78%) | 0.082 (-92%) |
| 21 | Cá bass (Cá chẽm) | Cá bass (Cá chẽm) | 2026-11-07 | 0.607 (-39%) | 0.223 (-78%) | 0.082 (-92%) |
| 22 | Yến mạch nguyên hạt Rolled Oats | Yến mạch nguyên hạt Rolled Oats | 2026-11-07 | 0.607 (-39%) | 0.223 (-78%) | 0.082 (-92%) |
| 23 | Thịt heo thăn nạc | Thịt heo thăn nạc | 2026-11-07 | 0.607 (-39%) | 0.223 (-78%) | 0.082 (-92%) |
| 24 | Đậu đen luộc | Đậu đen luộc | 2026-11-07 | 0.607 (-39%) | 0.223 (-78%) | 0.082 (-92%) |
| 25 | Cá ngừ đại dương tươi | Cá ngừ đại dương tươi | 2026-11-08 | 0.607 (-39%) | 0.223 (-78%) | 0.082 (-92%) |
| 26 | Yến mạch cán vỡ Quick Oats | Yến mạch cán vỡ Quick Oats | 2026-11-08 | 0.607 (-39%) | 0.223 (-78%) | 0.082 (-92%) |
| 27 | Đùi gà tháo da | Đùi gà tháo da | 2026-11-08 | 0.607 (-39%) | 0.223 (-78%) | 0.082 (-92%) |
| 28 | Hạt kiều mạch (Buckwheat) | Hạt kiều mạch (Buckwheat) | 2026-11-08 | 0.607 (-39%) | 0.223 (-78%) | 0.082 (-92%) |
