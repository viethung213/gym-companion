package vo

// CoinTransactionReason represents the strongly-typed business reason for a FitCoins balance change.
type CoinTransactionReason string

const (
	// ReasonEarnWorkout is the daily FitCoins reward (+10) for completing a valid workout session.
	ReasonEarnWorkout CoinTransactionReason = "EARN_WORKOUT"
	// ReasonEarnPR is the daily FitCoins reward (+20) for breaking a Personal Record.
	ReasonEarnPR CoinTransactionReason = "EARN_PR"
	// ReasonEarnStreakMilestone is the FitCoins reward (+30/+75/+150/...) for reaching a streak milestone.
	ReasonEarnStreakMilestone CoinTransactionReason = "EARN_STREAK_MILESTONE"
	// ReasonEarnBadge is the FitCoins reward (+15) for unlocking an achievement badge.
	ReasonEarnBadge CoinTransactionReason = "EARN_BADGE"
	// ReasonEarnNutrition is the daily FitCoins reward (+3) for completing daily nutrition targets.
	ReasonEarnNutrition CoinTransactionReason = "EARN_NUTRITION"

	// ReasonSpendRedeem deducts FitCoins when redeeming a reward or voucher.
	ReasonSpendRedeem CoinTransactionReason = "SPEND_REDEEM"
	// ReasonSpendStreakFreeze deducts FitCoins when purchasing a Streak Freeze item.
	ReasonSpendStreakFreeze CoinTransactionReason = "SPEND_STREAK_FREEZE"
	// ReasonSpendCosmetic deducts FitCoins when unlocking a profile cosmetic item.
	ReasonSpendCosmetic CoinTransactionReason = "SPEND_COSMETIC"

	// ReasonAdminAdjust represents a manual balance adjustment (positive or negative) by an administrator.
	ReasonAdminAdjust CoinTransactionReason = "ADMIN_ADJUST"
)

// Reward amount constants for standard gamification activities.
const (
	WorkoutCoinReward        int64 = 10
	PersonalRecordCoinReward int64 = 20
	BadgeUnlockedCoinReward  int64 = 15
	NutritionDailyCoinReward int64 = 3
	StreakMilestone7Reward   int64 = 30
	StreakMilestone14Reward  int64 = 75
	StreakMilestone30Reward  int64 = 150
	StreakMilestone50Reward  int64 = 250
	StreakMilestone100Reward int64 = 500
	StreakMilestone365Reward int64 = 1500
)

// IsEarn returns true if the reason is a valid positive earning category.
func (r CoinTransactionReason) IsEarn() bool {
	switch r {
	case ReasonEarnWorkout, ReasonEarnPR, ReasonEarnStreakMilestone, ReasonEarnBadge, ReasonEarnNutrition:
		return true
	default:
		return false
	}
}

// IsSpend returns true if the reason is a valid user spending category.
func (r CoinTransactionReason) IsSpend() bool {
	switch r {
	case ReasonSpendRedeem, ReasonSpendStreakFreeze, ReasonSpendCosmetic:
		return true
	default:
		return false
	}
}

// IsAdmin returns true if the reason is an administrative adjustment.
func (r CoinTransactionReason) IsAdmin() bool {
	return r == ReasonAdminAdjust
}

// HasDailyCap returns true if the earning reason is limited to at most 1 reward per calendar day.
func (r CoinTransactionReason) HasDailyCap() bool {
	switch r {
	case ReasonEarnWorkout, ReasonEarnPR, ReasonEarnNutrition:
		return true
	default:
		return false
	}
}

// StreakMilestoneCoinReward returns the FitCoins reward for a streak milestone day count, or 0 if not a milestone.
func StreakMilestoneCoinReward(days int32) int64 {
	switch days {
	case 7:
		return StreakMilestone7Reward
	case 14:
		return StreakMilestone14Reward
	case 30:
		return StreakMilestone30Reward
	case 50:
		return StreakMilestone50Reward
	case 100:
		return StreakMilestone100Reward
	case 365:
		return StreakMilestone365Reward
	default:
		return 0
	}
}
