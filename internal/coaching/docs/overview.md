# Coaching & Planning Overview

## Mục tiêu

Context `coaching` chuyển mục tiêu tập luyện của người dùng thành một kế hoạch có thể thực hiện:

```text
Chiến lược chu kỳ
        ↓
Phân bổ kế hoạch theo tuần
        ↓
Giáo án thực hiện trong ngày
```

Module này trả lời ba câu hỏi:

1. Trong chu kỳ bốn tuần, người dùng đang hướng tới mục tiêu gì?
2. Trong tuần này, người dùng tập ngày nào và tập nhóm cơ nào?
3. Trong ngày tập hôm nay, người dùng thực hiện bài tập, set và rep nào?

## Ranh giới context

`coaching` chỉ sở hữu quyết định lập kế hoạch. Các context khác vẫn giữ trách nhiệm riêng:

| Context | Trách nhiệm |
| --- | --- |
| Coaching & Planning | Tạo roadmap, phân bổ lịch tuần và sinh daily workout plan. |
| Exercise Catalog | Cung cấp danh mục bài tập đang hoạt động và metadata của bài tập. |
| Profile | Cung cấp thông tin sức khỏe, mục tiêu, thiết bị và lịch rảnh của người dùng. |
| Workout Execution | Ghi nhận kết quả thực hiện của buổi tập và dữ liệu hiệu suất. |

Coaching không truy vấn trực tiếp database của các context khác. Các dữ liệu bên ngoài được lấy qua application port.

## Aggregates

| Aggregate | Trách nhiệm |
| --- | --- |
| `WorkoutRoadmap` | Quản lý mục tiêu, thời gian và trạng thái của chu kỳ bốn tuần. Lưu input và phiên bản thuật toán đã dùng để tạo chu kỳ. Không quản lý lịch ngày hoặc bài tập cụ thể. |
| `WeeklySchedule` | Phân bổ ngày tập/nghỉ, nhóm cơ và khối lượng tập dự kiến cho một tuần. Là nguồn sự thật cho trạng thái slot tập: tập, nghỉ, bỏ hoặc dời. |
| `DailyWorkoutPlan` | Quản lý prescription sinh Just-In-Time cho một slot tập: bài tập, set, rep, nghỉ, warm-up và cool-down. Không thay đổi mục tiêu chu kỳ hoặc lịch tuần. |

## Domain services

| Service | Trách nhiệm |
| --- | --- |
| `SchedulePlanner` | Tạo lịch tập/nghỉ và muscle split; bảo đảm tối đa sáu ngày tập và tối thiểu một ngày nghỉ theo BR-AC-01. |
| `PrescriptionPlanner` | Tạo prescription deterministic từ slot tập, thiết bị hiện có và trình độ người dùng. |
| `PlannedVolumeValidator` | Bảo đảm khối lượng tập dự kiến của tuần mới không vượt quá 110% tuần trước. |

Trong MVP, khối lượng tập là volume dự kiến từ set và rep. Đây chưa phải volume thực tế người dùng đã hoàn thành.

## Quy tắc ownership

- Roadmap sở hữu mục tiêu và lifecycle của chu kỳ.
- Weekly Schedule sở hữu lịch và trạng thái của từng slot tập.
- Daily Workout Plan sở hữu prescription của một slot tập.
- Application service phối hợp cập nhật nhiều aggregate trong cùng transaction khi sinh hoặc thay Daily Plan.

## Giới hạn MVP

- Chưa tích hợp AI Coach; planner chạy theo rule deterministic.
- Chưa gợi ý mức tạ vì chưa có dữ liệu 1RM và hiệu suất đáng tin cậy.
- Chưa dùng volume thực tế để điều chỉnh progressive overload.
- Chưa triển khai Adaptive Review và adaptive recommendation.
- Exercise Catalog hiện chưa có metadata an toàn cho chấn thương; nếu có injury chưa được catalog hỗ trợ, Coaching không tự thay bài.

## Hướng mở rộng

- Thay inline planning snapshot bằng `ProfileReader` khi Profile context hoàn chỉnh.
- Dùng `PerformanceReader` để tính volume thực tế và mức tạ gợi ý.
- Thêm metadata chống chỉ định, movement pattern và substitution group vào Exercise Catalog.
- Bổ sung Adaptive Review khi đã có dữ liệu Workout Execution.
