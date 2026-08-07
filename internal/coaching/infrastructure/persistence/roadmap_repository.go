package persistence

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	"github.com/viethung213/gym-companion/internal/coaching/domain/roadmap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// RoadmapRepository is the GORM-backed adapter for port.RoadmapRepository.
type RoadmapRepository struct {
	db *gorm.DB
}

var _ port.RoadmapRepository = (*RoadmapRepository)(nil)

// NewRoadmapRepository constructs the repository.
func NewRoadmapRepository(db *gorm.DB) *RoadmapRepository {
	return &RoadmapRepository{db: db}
}

func (r *RoadmapRepository) getDB(ctx context.Context) *gorm.DB {
	if tx := GetTx(ctx); tx != nil {
		return tx
	}

	return r.db.WithContext(ctx)
}

// Save upserts the roadmap tree in a single call. Callers should already be
// inside a transaction (TxManager.WithTransaction) when saving alongside outbox.
func (r *RoadmapRepository) Save(ctx context.Context, rm *roadmap.Roadmap) error {
	if rm == nil {
		return errors.New("nil roadmap")
	}

	db := r.getDB(ctx)

	if err := db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "roadmap_id"}},

		DoUpdates: clause.AssignmentColumns([]string{"status", "start_date", "end_date", "updated_at"}),
	}).Create(toRoadmapRecordPtr(rm)).Error; err != nil {
		return fmt.Errorf("upsert roadmap: %w", err)
	}

	for _, w := range rm.Weeks() {
		wr := toWeekPlanRecord(w)

		if err := db.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "week_plan_id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"phase", "target_rpe", "start_date", "end_date", "muscle_split_type",
			}),
		}).Create(&wr).Error; err != nil {
			return fmt.Errorf("upsert week plan %s: %w", wr.WeekPlanID, err)
		}

		for _, d := range w.Days() {
			dr := toDayPlanRecord(d)

			if err := db.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "day_plan_id"}},
				DoUpdates: clause.AssignmentColumns([]string{"scheduled_date"}),
			}).Create(&dr).Error; err != nil {
				return fmt.Errorf("upsert day plan %s: %w", dr.DayPlanID, err)
			}

			for _, s := range d.Sessions() {
				sr, err := toSessionPlanRecord(s)

				if err != nil {
					return err
				}

				if err := db.Clauses(clause.OnConflict{
					Columns: []clause.Column{{Name: "session_plan_id"}},
					DoUpdates: clause.AssignmentColumns([]string{
						"slot_time", "status", "target_muscle_groups", "prescription",
						"reasoning", "generated_at", "completed_at", "session_scr", "session_delta_rpe",
					}),
				}).Create(&sr).Error; err != nil {
					return fmt.Errorf("upsert session plan %s: %w", sr.SessionPlanID, err)
				}
			}
		}
	}

	return nil
}

// FindByID loads a full roadmap tree.
func (r *RoadmapRepository) FindByID(ctx context.Context, roadmapID string) (*roadmap.Roadmap, error) {
	db := r.getDB(ctx)

	var rec roadmapRecord

	if err := db.Where("roadmap_id = ?", roadmapID).First(&rec).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, roadmap.ErrRoadmapNotFound
		}

		return nil, fmt.Errorf("find roadmap: %w", err)
	}

	return r.loadTree(ctx, &rec)
}

// FindActiveByUser returns the (up to 1) ACTIVE roadmap for a user.
func (r *RoadmapRepository) FindActiveByUser(ctx context.Context, userID string) (*roadmap.Roadmap, error) {
	db := r.getDB(ctx)

	var rec roadmapRecord

	if err := db.Where("user_id = ? AND status = ?", userID, string(roadmap.StatusActive)).First(&rec).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, roadmap.ErrRoadmapNotFound
		}

		return nil, fmt.Errorf("find active roadmap: %w", err)
	}

	return r.loadTree(ctx, &rec)
}

// ListByUser returns roadmaps for a user with optional status filter.
func (r *RoadmapRepository) ListByUser(ctx context.Context, userID string, status roadmap.Status, limit, offset int) ([]*roadmap.Roadmap, error) {
	db := r.getDB(ctx)

	q := db.Where("user_id = ?", userID)

	if status != "" {
		q = q.Where("status = ?", string(status))
	}

	if limit <= 0 {
		limit = 20
	}

	q = q.Order("created_at DESC").Limit(limit).Offset(offset)

	var recs []roadmapRecord

	if err := q.Find(&recs).Error; err != nil {
		return nil, fmt.Errorf("list roadmaps: %w", err)
	}

	out := make([]*roadmap.Roadmap, 0, len(recs))

	for i := range recs {
		rm, err := r.loadTree(ctx, &recs[i])

		if err != nil {
			return nil, err
		}

		out = append(out, rm)
	}

	return out, nil
}

// FindSessionByID loads the parent roadmap of a session by session id.
func (r *RoadmapRepository) FindSessionByID(ctx context.Context, sessionPlanID string) (*roadmap.Roadmap, error) {
	db := r.getDB(ctx)

	var srec sessionPlanRecord

	if err := db.Where("session_plan_id = ?", sessionPlanID).First(&srec).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, roadmap.ErrSessionNotFound
		}

		return nil, fmt.Errorf("find session: %w", err)
	}

	return r.FindByID(ctx, srec.RoadmapID)
}

// FindPendingSessionsByDate queries session plans scheduled for targetDate that are PENDING.
func (r *RoadmapRepository) FindPendingSessionsByDate(ctx context.Context, targetDate time.Time) ([]*roadmap.SessionPlanInfo, error) {
	db := r.getDB(ctx)

	var sRecs []sessionPlanRecord
	dateStr := targetDate.Format("2006-01-02")

	if err := db.Where("DATE(scheduled_date) = ? AND status = ?", dateStr, string(roadmap.SessionPlanStatusPending)).Find(&sRecs).Error; err != nil {
		return nil, fmt.Errorf("find pending sessions by date: %w", err)
	}

	out := make([]*roadmap.SessionPlanInfo, 0, len(sRecs))
	for i := range sRecs {
		sp, err := fromSessionPlanRecord(&sRecs[i])
		if err != nil {
			continue
		}
		info := sp.Info()
		out = append(out, &info)
	}

	return out, nil
}

// loadTree loads all descendants of the given roadmap record.
func (r *RoadmapRepository) loadTree(ctx context.Context, rec *roadmapRecord) (*roadmap.Roadmap, error) {
	if rec == nil {
		return nil, errors.New("nil roadmapRecord")
	}

	db := r.getDB(ctx)

	var wRecs []weekPlanRecord

	if err := db.Where("roadmap_id = ?", rec.RoadmapID).Order("week_number ASC").Find(&wRecs).Error; err != nil {
		return nil, fmt.Errorf("load weeks: %w", err)
	}

	var dRecs []dayPlanRecord

	if err := db.Where("roadmap_id = ?", rec.RoadmapID).Order("scheduled_date ASC").Find(&dRecs).Error; err != nil {
		return nil, fmt.Errorf("load days: %w", err)
	}

	var sRecs []sessionPlanRecord

	if err := db.Where("roadmap_id = ?", rec.RoadmapID).Order("scheduled_date ASC").Find(&sRecs).Error; err != nil {
		return nil, fmt.Errorf("load sessions: %w", err)
	}

	sessionsByDay := map[string][]*roadmap.SessionPlan{}

	for i := range sRecs {
		s := &sRecs[i]

		sp, err := fromSessionPlanRecord(s)

		if err != nil {
			return nil, err
		}

		sessionsByDay[s.DayPlanID] = append(sessionsByDay[s.DayPlanID], sp)
	}

	daysByWeek := map[string][]*roadmap.DayPlan{}

	for i := range dRecs {
		d := &dRecs[i]

		dp, err := fromDayPlanRecord(d, sessionsByDay[d.DayPlanID])

		if err != nil {
			return nil, err
		}

		daysByWeek[d.WeekPlanID] = append(daysByWeek[d.WeekPlanID], dp)
	}

	weeks := make([]*roadmap.WeekPlan, 0, len(wRecs))

	for i := range wRecs {
		w := &wRecs[i]

		wp, err := fromWeekPlanRecord(w, daysByWeek[w.WeekPlanID])

		if err != nil {
			return nil, err
		}

		weeks = append(weeks, wp)
	}

	return fromRoadmapRecord(rec, weeks)
}

// toRoadmapRecordPtr is a small helper to avoid taking &foo of a function return.
func toRoadmapRecordPtr(rm *roadmap.Roadmap) *roadmapRecord {
	if rm == nil {
		return nil
	}

	rec := toRoadmapRecord(rm)

	return &rec
}
