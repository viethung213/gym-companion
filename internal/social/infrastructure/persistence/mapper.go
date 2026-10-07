package persistence

import (
	"encoding/json"

	"github.com/viethung213/gym-companion/internal/social/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/social/domain/entity"
	"github.com/viethung213/gym-companion/internal/social/domain/vo"
)

type workoutDataJSON struct {
	SessionID       string  `json:"session_id,omitempty"`
	WorkoutTitle    string  `json:"workout_title,omitempty"`
	DurationSeconds int32   `json:"duration_seconds,omitempty"`
	TotalVolumeKg   float64 `json:"total_volume_kg,omitempty"`
	ExerciseCount   int32   `json:"exercise_count,omitempty"`
	TotalSets       int32   `json:"total_sets,omitempty"`
	PRCount         int32   `json:"pr_count,omitempty"`
}

func ToDomainFollow(m *FollowModel) (*aggregate.Follow, error) {
	return aggregate.NewFollow(m.ID, m.FollowerID, m.FollowingID, m.CreatedAt)
}

func ToPersistenceFollow(f *aggregate.Follow) *FollowModel {
	return &FollowModel{
		ID:          f.ID(),
		FollowerID:  f.FollowerID(),
		FollowingID: f.FollowingID(),
		CreatedAt:   f.CreatedAt(),
	}
}

func ToDomainFeedItem(m *FeedItemModel) (*aggregate.FeedItem, error) {
	var urls []string
	if len(m.MediaURLs) > 0 {
		_ = json.Unmarshal(m.MediaURLs, &urls)
	}

	it := vo.NewItemType(m.ItemType)

	visibility := vo.NewVisibility(m.Visibility)

	if it == vo.ItemTypeWorkoutActivity {
		var wj workoutDataJSON
		if len(m.Data) > 0 {
			_ = json.Unmarshal(m.Data, &wj)
		}

		metrics := vo.NewWorkoutMetrics(
			wj.SessionID,
			wj.WorkoutTitle,
			wj.DurationSeconds,
			wj.TotalVolumeKg,
			wj.ExerciseCount,
			wj.TotalSets,
			wj.PRCount,
		)

		return aggregate.NewWorkoutActivityItem(
			m.ID,
			m.UserID,
			m.Caption,
			urls,
			metrics,
			visibility,
			m.ReactionCount,
			m.CommentCount,
			m.CreatedAt,
			m.UpdatedAt,
		)
	}

	return aggregate.NewPostItem(
		m.ID,
		m.UserID,
		m.Caption,
		urls,
		visibility,
		m.ReactionCount,
		m.CommentCount,
		m.CreatedAt,
		m.UpdatedAt,
	)
}

func ToPersistenceFeedItem(item *aggregate.FeedItem) *FeedItemModel {
	urlsBytes, _ := json.Marshal(item.MediaURLs())

	var dataBytes []byte
	if item.ItemType() == vo.ItemTypeWorkoutActivity && !item.WorkoutData().IsZero() {
		wj := workoutDataJSON{
			SessionID:       item.WorkoutData().SessionID(),
			WorkoutTitle:    item.WorkoutData().WorkoutTitle(),
			DurationSeconds: item.WorkoutData().DurationSeconds(),
			TotalVolumeKg:   item.WorkoutData().TotalVolumeKg(),
			ExerciseCount:   item.WorkoutData().ExerciseCount(),
			TotalSets:       item.WorkoutData().TotalSets(),
			PRCount:         item.WorkoutData().PRCount(),
		}
		dataBytes, _ = json.Marshal(wj)
	} else {
		dataBytes = []byte("{}")
	}

	return &FeedItemModel{
		ID:            item.ID(),
		UserID:        item.UserID(),
		ItemType:      item.ItemType().String(),
		Caption:       item.Caption(),
		MediaURLs:     urlsBytes,
		Data:          dataBytes,
		Visibility:    item.Visibility().String(),
		ReactionCount: item.ReactionCount(),
		CommentCount:  item.CommentCount(),
		CreatedAt:     item.CreatedAt(),
		UpdatedAt:     item.UpdatedAt(),
	}
}

func ToDomainReaction(m *ReactionModel) (*entity.Reaction, error) {
	rt, err := vo.NewReactionType(string(m.ReactionType))
	if err != nil {
		return nil, err
	}
	return entity.NewReaction(m.ID, m.UserID, m.FeedItemID, rt, m.CreatedAt)
}

func ToPersistenceReaction(r *entity.Reaction) *ReactionModel {
	return &ReactionModel{
		ID:           r.ID(),
		UserID:       r.UserID(),
		FeedItemID:   r.FeedItemID(),
		ReactionType: r.ReactionType(),
		CreatedAt:    r.CreatedAt(),
	}
}

func ToDomainComment(m *CommentModel) (*entity.Comment, error) {
	return entity.NewComment(m.ID, m.UserID, m.FeedItemID, m.ParentID, m.Content, m.CreatedAt, m.UpdatedAt)
}

func ToPersistenceComment(c *entity.Comment) *CommentModel {
	return &CommentModel{
		ID:         c.ID(),
		UserID:     c.UserID(),
		FeedItemID: c.FeedItemID(),
		ParentID:   c.ParentID(),
		Content:    c.Content(),
		CreatedAt:  c.CreatedAt(),
		UpdatedAt:  c.UpdatedAt(),
	}
}

func ToDomainUserSnapshot(m *SocialUserModel) *entity.UserSnapshot {
	return entity.NewUserSnapshot(m.ID, m.FullName, m.AvatarURL, m.Role, m.UpdatedAt)
}

func ToPersistenceUserSnapshot(u *entity.UserSnapshot) *SocialUserModel {
	return &SocialUserModel{
		ID:        u.ID(),
		FullName:  u.FullName(),
		AvatarURL: u.AvatarURL(),
		Role:      u.Role(),
		UpdatedAt: u.UpdatedAt(),
	}
}
