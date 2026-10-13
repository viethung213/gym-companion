package vo

// BadgeDefinition represents a system achievement definition.
type BadgeDefinition struct {
	Code        string
	Name        string
	Description string
	Category    string // "STREAK", "LEVEL", "WORKOUT", "PR", "NUTRITION"
	IconURL     string
}

// MasterBadgeCatalog is the Single Source of Truth for system achievement badges.
var MasterBadgeCatalog = []BadgeDefinition{
	{
		Code:        "FIRST_WORKOUT",
		Name:        "Khởi Đầu Mới",
		Description: "Hoàn thành buổi tập gym đầu tiên trong đời",
		Category:    "WORKOUT",
		IconURL:     "/assets/badges/first_workout.svg",
	},
	{
		Code:        "STREAK_7",
		Name:        "Chiến Binh 7 Ngày",
		Description: "Duy trì chuỗi kỷ luật 7 ngày liên tiếp",
		Category:    "STREAK",
		IconURL:     "/assets/badges/streak_7.svg",
	},
	{
		Code:        "STREAK_14",
		Name:        "Kiên Trì 2 Tuần",
		Description: "Duy trì chuỗi kỷ luật 14 ngày liên tiếp",
		Category:    "STREAK",
		IconURL:     "/assets/badges/streak_14.svg",
	},
	{
		Code:        "STREAK_30",
		Name:        "Kỷ Luật Sắt",
		Description: "Duy trì chuỗi kỷ luật 30 ngày liên tiếp",
		Category:    "STREAK",
		IconURL:     "/assets/badges/streak_30.svg",
	},
	{
		Code:        "STREAK_50",
		Name:        "Thói Quen Bền Vững",
		Description: "Duy trì chuỗi kỷ luật 50 ngày liên tiếp",
		Category:    "STREAK",
		IconURL:     "/assets/badges/streak_50.svg",
	},
	{
		Code:        "STREAK_100",
		Name:        "Huyền Thoại 100 Ngày",
		Description: "Duy trì chuỗi kỷ luật 100 ngày liên tiếp",
		Category:    "STREAK",
		IconURL:     "/assets/badges/streak_100.svg",
	},
	{
		Code:        "STREAK_365",
		Name:        "Chiến Thần Kỷ Luật",
		Description: "Duy trì chuỗi kỷ luật 365 ngày trọn vẹn",
		Category:    "STREAK",
		IconURL:     "/assets/badges/streak_365.svg",
	},
	{
		Code:        "LEVEL_10",
		Name:        "Tân Binh Đột Phá",
		Description: "Đạt Cấp Độ 10 trọn đời",
		Category:    "LEVEL",
		IconURL:     "/assets/badges/level_10.svg",
	},
	{
		Code:        "LEVEL_25",
		Name:        "Vận Động Viên Bền Bỉ",
		Description: "Đạt Cấp Độ 25 trọn đời",
		Category:    "LEVEL",
		IconURL:     "/assets/badges/level_25.svg",
	},
	{
		Code:        "LEVEL_50",
		Name:        "Bậc Thầy Thể Hình",
		Description: "Đạt Cấp Độ 50 trọn đời",
		Category:    "LEVEL",
		IconURL:     "/assets/badges/level_50.svg",
	},
	{
		Code:        "LEVEL_100",
		Name:        "Đỉnh Cao Thể Lực",
		Description: "Đạt Cấp Độ Tối Đa 100 trọn đời",
		Category:    "LEVEL",
		IconURL:     "/assets/badges/level_100.svg",
	},
	{
		Code:        "PR_BREAKER",
		Name:        "Phá Vỡ Giới Hạn",
		Description: "Thiết lập kỷ lục cá nhân (PR) mới trong buổi tập",
		Category:    "PR",
		IconURL:     "/assets/badges/pr_breaker.svg",
	},
}

// GetBadgeDefinition returns badge metadata for a given badge code.
func GetBadgeDefinition(code string) (BadgeDefinition, bool) {
	for _, b := range MasterBadgeCatalog {
		if b.Code == code {
			return b, true
		}
	}
	return BadgeDefinition{
		Code:        code,
		Name:        code,
		Description: "",
		Category:    "GENERAL",
		IconURL:     "",
	}, false
}
