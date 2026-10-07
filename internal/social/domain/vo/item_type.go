package vo

type ItemType string

const (
	ItemTypePost            ItemType = "POST"
	ItemTypeWorkoutActivity ItemType = "WORKOUT_ACTIVITY"
)

func NewItemType(s string) ItemType {
	if s == string(ItemTypeWorkoutActivity) {
		return ItemTypeWorkoutActivity
	}
	return ItemTypePost
}

func (i ItemType) String() string {
	return string(i)
}
