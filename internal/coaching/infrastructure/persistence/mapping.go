package persistence

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/domain/roadmap"
)

// ---------- Roadmap ----------

func toRoadmapRecord(r *roadmap.Roadmap) roadmapRecord {
	info := r.Info()
	return roadmapRecord{
		RoadmapID: info.RoadmapID,
		UserID:    info.UserID,
		Status:    string(info.Status),
		StartDate: info.StartDate,
		EndDate:   info.EndDate,
		CreatedAt: info.CreatedAt,
		UpdatedAt: info.UpdatedAt,
	}
}

func fromRoadmapRecord(rec *roadmapRecord, weeks []*roadmap.WeekPlan) (*roadmap.Roadmap, error) {
	if rec == nil {
		return nil, fmt.Errorf("nil roadmapRecord")
	}
	return roadmap.RehydrateRoadmap(&roadmap.RoadmapInfo{
		RoadmapID: rec.RoadmapID,
		UserID:    rec.UserID,
		Status:    roadmap.RoadmapStatus(rec.Status),
		StartDate: rec.StartDate,
		EndDate:   rec.EndDate,
		CreatedAt: rec.CreatedAt,
		UpdatedAt: rec.UpdatedAt,
	}, weeks)
}

// ---------- WeekPlan ----------

func toWeekPlanRecord(w *roadmap.WeekPlan) weekPlanRecord {
	info := w.Info()
	return weekPlanRecord{
		WeekPlanID:      info.WeekPlanID,
		RoadmapID:       info.RoadmapID,
		UserID:          info.UserID,
		WeekNumber:      int16(info.WeekNumber),
		Phase:           string(info.Phase),
		TargetRPE:       info.TargetRPE,
		StartDate:       info.StartDate,
		EndDate:         info.EndDate,
		MuscleSplitType: info.MuscleSplitType,
	}
}

func fromWeekPlanRecord(rec *weekPlanRecord, days []*roadmap.DayPlan) (*roadmap.WeekPlan, error) {
	if rec == nil {
		return nil, fmt.Errorf("nil weekPlanRecord")
	}
	return roadmap.RehydrateWeekPlan(&roadmap.WeekPlanInfo{
		WeekPlanID:      rec.WeekPlanID,
		RoadmapID:       rec.RoadmapID,
		UserID:          rec.UserID,
		WeekNumber:      int32(rec.WeekNumber),
		Phase:           roadmap.Phase(rec.Phase),
		TargetRPE:       rec.TargetRPE,
		StartDate:       rec.StartDate,
		EndDate:         rec.EndDate,
		MuscleSplitType: rec.MuscleSplitType,
	}, days)
}

// ---------- DayPlan ----------

func toDayPlanRecord(d *roadmap.DayPlan) dayPlanRecord {
	info := d.Info()
	return dayPlanRecord{
		DayPlanID:     info.DayPlanID,
		WeekPlanID:    info.WeekPlanID,
		RoadmapID:     info.RoadmapID,
		UserID:        info.UserID,
		ScheduledDate: info.ScheduledDate,
		IsRestDay:     info.IsRestDay,
	}
}

func fromDayPlanRecord(rec *dayPlanRecord, sessions []*roadmap.SessionPlan) (*roadmap.DayPlan, error) {
	if rec == nil {
		return nil, fmt.Errorf("nil dayPlanRecord")
	}
	return roadmap.RehydrateDayPlan(&roadmap.DayPlanInfo{
		DayPlanID:     rec.DayPlanID,
		WeekPlanID:    rec.WeekPlanID,
		RoadmapID:     rec.RoadmapID,
		UserID:        rec.UserID,
		ScheduledDate: rec.ScheduledDate,
		IsRestDay:     rec.IsRestDay,
	}, sessions)
}

// ---------- SessionPlan ----------

func toSessionPlanRecord(s *roadmap.SessionPlan) (sessionPlanRecord, error) {
	info := s.Info()
	prescBytes, err := json.Marshal(info.Prescription)
	if err != nil {
		return sessionPlanRecord{}, fmt.Errorf("marshal prescription: %w", err)
	}
	musclesBytes, err := json.Marshal(info.TargetMuscleGroups)
	if err != nil {
		return sessionPlanRecord{}, fmt.Errorf("marshal muscle groups: %w", err)
	}
	rec := sessionPlanRecord{
		SessionPlanID:      info.SessionPlanID,
		DayPlanID:          info.DayPlanID,
		WeekPlanID:         info.WeekPlanID,
		RoadmapID:          info.RoadmapID,
		UserID:             info.UserID,
		ScheduledDate:      info.ScheduledDate,
		SlotTime:           info.SlotTime,
		Status:             string(info.Status),
		TargetMuscleGroups: musclesBytes,
		Prescription:       prescBytes,
		Reasoning:          info.Reasoning,
		GeneratedAt:        info.GeneratedAt,
	}
	if info.CompletedAt != nil {
		rec.CompletedAt = sql.NullTime{Time: *info.CompletedAt, Valid: true}
	}
	if info.SessionSCR != nil {
		rec.SessionSCR = sql.NullFloat64{Float64: float64(*info.SessionSCR), Valid: true}
	}
	if info.SessionDeltaRPE != nil {
		rec.SessionDeltaRPE = sql.NullFloat64{Float64: float64(*info.SessionDeltaRPE), Valid: true}
	}
	return rec, nil
}

func fromSessionPlanRecord(rec *sessionPlanRecord) (*roadmap.SessionPlan, error) {
	if rec == nil {
		return nil, fmt.Errorf("nil sessionPlanRecord")
	}
	var presc roadmap.WorkoutPrescription
	if len(rec.Prescription) > 0 {
		if err := json.Unmarshal(rec.Prescription, &presc); err != nil {
			return nil, fmt.Errorf("unmarshal prescription: %w", err)
		}
	}
	var muscles []string
	if len(rec.TargetMuscleGroups) > 0 {
		if err := json.Unmarshal(rec.TargetMuscleGroups, &muscles); err != nil {
			return nil, fmt.Errorf("unmarshal muscle groups: %w", err)
		}
	}
	info := roadmap.SessionPlanInfo{
		SessionPlanID:      rec.SessionPlanID,
		DayPlanID:          rec.DayPlanID,
		WeekPlanID:         rec.WeekPlanID,
		RoadmapID:          rec.RoadmapID,
		UserID:             rec.UserID,
		ScheduledDate:      rec.ScheduledDate,
		SlotTime:           rec.SlotTime,
		Status:             roadmap.SessionPlanStatus(rec.Status),
		TargetMuscleGroups: muscles,
		Prescription:       presc,
		Reasoning:          rec.Reasoning,
		GeneratedAt:        rec.GeneratedAt,
	}
	if rec.CompletedAt.Valid {
		t := rec.CompletedAt.Time
		info.CompletedAt = &t
	}
	if rec.SessionSCR.Valid {
		v := float32(rec.SessionSCR.Float64)
		info.SessionSCR = &v
	}
	if rec.SessionDeltaRPE.Valid {
		v := float32(rec.SessionDeltaRPE.Float64)
		info.SessionDeltaRPE = &v
	}
	return roadmap.RehydrateSessionPlan(&info)
}

// Unused import guard.
var _ = time.Time{}
