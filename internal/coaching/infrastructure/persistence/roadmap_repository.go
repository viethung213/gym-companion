package persistence

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/lib/pq"
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
// activeRoadmapConstraint is the unique index allowing one ACTIVE roadmap per user.
const activeRoadmapConstraint = "ux_roadmaps_user_active"

// plannedRoadmapConstraint is the unique index allowing one PLANNED roadmap per user.
const plannedRoadmapConstraint = "ux_roadmaps_user_planned"

func (r *RoadmapRepository) Save(ctx context.Context, rm *roadmap.Roadmap) error {
	if rm == nil {
		return errors.New("nil roadmap")
	}

	db := r.getDB(ctx)

	// Optimistic lock: the row is written only if it still holds the version
	// this copy was read at, so a writer working from a stale copy (one that
	// waited on the model, say) cannot erase what others saved meanwhile.
	rec := toRoadmapRecordPtr(rm)
	rec.Version = rm.Version() + 1
	res := db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "roadmap_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"status", "start_date", "end_date", "updated_at", "timezone", "version"}),
		Where: clause.Where{Exprs: []clause.Expression{
			clause.Expr{SQL: "coaching.roadmaps.version = ?", Vars: []any{rm.Version()}},
		}},
	}).Create(rec)
	if err := res.Error; err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Constraint == activeRoadmapConstraint {
			// Another node initiated this user's roadmap first.
			return fmt.Errorf("%w: %s", roadmap.ErrActiveRoadmapExists, rm.UserID())
		}
		if errors.As(err, &pqErr) && pqErr.Constraint == plannedRoadmapConstraint {
			return fmt.Errorf("%w: %s", roadmap.ErrPlannedRoadmapExists, rm.UserID())
		}
		return fmt.Errorf("upsert roadmap: %w", err)
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("%w: %s at version %d", roadmap.ErrRoadmapChanged, rm.ID(), rm.Version())
	}

	// Delete what a reschedule dropped before upserting its replacements: a new
	// day may take the date (uq_day_week_date) of a day it emptied.
	if err := deleteRemovedChildren(db, rm); err != nil {
		return err
	}

	for _, w := range rm.Weeks() {
		wr := toWeekPlanRecord(w)

		if err := db.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "week_plan_id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"phase", "target_rpe", "start_date", "end_date", "muscle_split_type", "reviewed_at",
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
						"slot_time", "estimated_duration_minutes", "status", "session_kind", "target_muscle_groups", "prescription",
						"reasoning", "generated_at", "completed_at", "session_scr", "session_delta_rpe",
					}),
				}).Create(&sr).Error; err != nil {
					return fmt.Errorf("upsert session plan %s: %w", sr.SessionPlanID, err)
				}
			}
		}
	}

	rm.MarkSaved(rec.Version)

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

// ListByUser returns a user's roadmaps whose status is one of statuses (all
// when empty), newest first.
func (r *RoadmapRepository) ListByUser(ctx context.Context, userID string, statuses []roadmap.Status, limit, offset int) ([]*roadmap.Roadmap, error) {
	db := r.getDB(ctx)

	q := db.Where("user_id = ?", userID)

	if len(statuses) > 0 {
		names := make([]string, len(statuses))
		for i, s := range statuses {
			names[i] = string(s)
		}
		q = q.Where("status IN ?", names)
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

	// Same-day sessions need a fixed order: callers walk them as the schedule.
	if err := db.Where("roadmap_id = ?", rec.RoadmapID).Order("scheduled_date ASC, slot_time ASC, session_plan_id ASC").Find(&sRecs).Error; err != nil {
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

// deleteRemovedChildren deletes the sessions and days the aggregate dropped.
// Only PENDING sessions are ever dropped; the status guard keeps a session
// finished concurrently from being lost.
func deleteRemovedChildren(db *gorm.DB, rm *roadmap.Roadmap) error {
	sessionIDs, dayIDs := rm.RemovedChildren()
	if len(sessionIDs) > 0 {
		if err := db.Where("roadmap_id = ? AND status = ? AND session_plan_id IN ?",
			rm.ID(), string(roadmap.SessionPlanStatusPending), sessionIDs).
			Delete(&sessionPlanRecord{}).Error; err != nil {
			return fmt.Errorf("delete rescheduled sessions: %w", err)
		}
	}
	if len(dayIDs) > 0 {
		if err := db.Where("roadmap_id = ? AND day_plan_id IN ?", rm.ID(), dayIDs).
			Where("NOT EXISTS (SELECT 1 FROM coaching.session_plans s WHERE s.day_plan_id = coaching.day_plans.day_plan_id)").
			Delete(&dayPlanRecord{}).Error; err != nil {
			return fmt.Errorf("delete emptied day plans: %w", err)
		}
	}
	return nil
}

// FindPendingSessionsStarting returns PENDING sessions whose start, as wall
// clock in their roadmap's zone, falls within [from, to]. Candidates are
// narrowed by scheduled_date a day either side, since no zone is more than a
// day from UTC.
func (r *RoadmapRepository) FindPendingSessionsStarting(ctx context.Context, from, to time.Time) ([]port.ScheduledSession, error) {
	db := r.getDB(ctx)

	var sRecs []sessionPlanRecord
	err := db.Where("status = ? AND scheduled_date BETWEEN ? AND ? AND roadmap_id IN (SELECT roadmap_id FROM coaching.roadmaps WHERE status = ?)",
		string(roadmap.SessionPlanStatusPending),
		roadmap.CalendarDate(from, time.UTC).AddDate(0, 0, -1),
		roadmap.CalendarDate(to, time.UTC).AddDate(0, 0, 1),
		string(roadmap.StatusActive)).
		Find(&sRecs).Error
	if err != nil {
		return nil, fmt.Errorf("find pending sessions starting: %w", err)
	}
	if len(sRecs) == 0 {
		return nil, nil
	}

	roadmapIDs := make([]string, 0, len(sRecs))
	for i := range sRecs {
		roadmapIDs = append(roadmapIDs, sRecs[i].RoadmapID)
	}
	var zones []struct {
		RoadmapID string
		Timezone  string
	}
	if err := db.Table("coaching.roadmaps").Select("roadmap_id, timezone").
		Where("roadmap_id IN ?", roadmapIDs).Scan(&zones).Error; err != nil {
		return nil, fmt.Errorf("load roadmap zones: %w", err)
	}
	zoneOf := make(map[string]*time.Location, len(zones))
	for _, z := range zones {
		zoneOf[z.RoadmapID] = roadmap.LoadLocation(z.Timezone)
	}

	out := make([]port.ScheduledSession, 0, len(sRecs))
	for i := range sRecs {
		sp, err := fromSessionPlanRecord(&sRecs[i])
		if err != nil {
			slog.WarnContext(ctx, "Skipping unreadable session plan",
				slog.String("session_plan_id", sRecs[i].SessionPlanID), slog.String("error", err.Error()))
			continue
		}
		loc := zoneOf[sRecs[i].RoadmapID]
		if loc == nil {
			loc = time.UTC
		}
		at := sp.StartsAt(loc)
		if at.Before(from) || at.After(to) {
			continue
		}
		out = append(out, port.ScheduledSession{Info: sp.Info(), StartsAt: at, Location: loc})
	}
	return out, nil
}

var _ port.WeekReviewQueue = (*RoadmapRepository)(nil)

// RoadmapIDsWithOpenWeekEndingBy lists ACTIVE roadmaps with an unreviewed week
// whose end_date is on or before date.
func (r *RoadmapRepository) RoadmapIDsWithOpenWeekEndingBy(ctx context.Context, date time.Time) ([]string, error) {
	var ids []string
	err := r.getDB(ctx).Table("coaching.week_plans AS w").
		Distinct("w.roadmap_id").
		Joins("JOIN coaching.roadmaps AS r ON r.roadmap_id = w.roadmap_id").
		Where("r.status = ? AND w.reviewed_at IS NULL AND w.end_date <= ?",
			string(roadmap.StatusActive), roadmap.CalendarDate(date, time.UTC)).
		Pluck("w.roadmap_id", &ids).Error
	if err != nil {
		return nil, fmt.Errorf("find roadmaps with an open week ending by %s: %w", date.Format(time.DateOnly), err)
	}
	return ids, nil
}

var _ port.RoadmapRotation = (*RoadmapRepository)(nil)

// FindPlannedByUser returns the user's PLANNED roadmap.
func (r *RoadmapRepository) FindPlannedByUser(ctx context.Context, userID string) (*roadmap.Roadmap, error) {
	var rec roadmapRecord
	err := r.getDB(ctx).Where("user_id = ? AND status = ?", userID, string(roadmap.StatusPlanned)).First(&rec).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, roadmap.ErrRoadmapNotFound
		}
		return nil, fmt.Errorf("find planned roadmap: %w", err)
	}
	return r.loadTree(ctx, &rec)
}

// DeletePlannedRoadmap removes a PLANNED roadmap; weeks, days and sessions go
// with it (ON DELETE CASCADE). The status condition makes it a no-op on a
// roadmap a roll-over activated in between, so a live plan is never lost.
func (r *RoadmapRepository) DeletePlannedRoadmap(ctx context.Context, roadmapID string) error {
	err := r.getDB(ctx).Where("roadmap_id = ? AND status = ?", roadmapID, string(roadmap.StatusPlanned)).
		Delete(&roadmapRecord{}).Error
	if err != nil {
		return fmt.Errorf("delete planned roadmap %s: %w", roadmapID, err)
	}
	return nil
}

// ActiveRoadmapsEndingBy lists ACTIVE roadmaps whose end_date is on or before
// date.
func (r *RoadmapRepository) ActiveRoadmapsEndingBy(ctx context.Context, date time.Time) ([]port.RoadmapRef, error) {
	var refs []port.RoadmapRef
	err := r.getDB(ctx).Table("coaching.roadmaps").Select("roadmap_id, user_id").
		Where("status = ? AND end_date <= ?", string(roadmap.StatusActive), roadmap.CalendarDate(date, time.UTC)).
		Scan(&refs).Error
	if err != nil {
		return nil, fmt.Errorf("find active roadmaps ending by %s: %w", date.Format(time.DateOnly), err)
	}
	return refs, nil
}
