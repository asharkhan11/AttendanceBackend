package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/school/attendance-backend/internal/models"
)

type CalendarRepo struct {
	db *DB
}

func NewCalendarRepo(db *DB) *CalendarRepo {
	return &CalendarRepo{db: db}
}

func (r *CalendarRepo) Db() *DB { return r.db }

var dayNames = map[int16]string{
	0: "Sunday",
	1: "Monday",
	2: "Tuesday",
	3: "Wednesday",
	4: "Thursday",
	5: "Friday",
	6: "Saturday",
}

func (r *CalendarRepo) GetWeeklySchedules(ctx context.Context, schoolID uuid.UUID) ([]models.WeeklySchedule, error) {
	q := r.db.Querier(ctx)
	rows, err := q.Query(ctx, `
		SELECT id, school_id, day_of_week, is_working_day, created_at, updated_at
		FROM school_weekly_schedules
		WHERE school_id = $1
		ORDER BY day_of_week ASC
	`, schoolID)
	if err != nil {
		return nil, fmt.Errorf("query weekly schedules: %w", err)
	}
	defer rows.Close()

	var list []models.WeeklySchedule
	for rows.Next() {
		var s models.WeeklySchedule
		if err := rows.Scan(&s.ID, &s.SchoolID, &s.DayOfWeek, &s.IsWorkingDay, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan weekly schedule: %w", err)
		}
		s.DayName = dayNames[s.DayOfWeek]
		list = append(list, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return list, nil
}

func (r *CalendarRepo) UpsertWeeklySchedule(ctx context.Context, schoolID uuid.UUID, dayOfWeek int16, isWorkingDay bool) error {
	q := r.db.Querier(ctx)
	_, err := q.Exec(ctx, `
		INSERT INTO school_weekly_schedules (school_id, day_of_week, is_working_day, updated_at)
		VALUES ($1, $2, $3, now())
		ON CONFLICT (school_id, day_of_week)
		DO UPDATE SET is_working_day = EXCLUDED.is_working_day, updated_at = now()
	`, schoolID, dayOfWeek, isWorkingDay)
	if err != nil {
		return fmt.Errorf("upsert weekly schedule: %w", err)
	}
	return nil
}

func (r *CalendarRepo) GetDateHolidays(ctx context.Context, schoolID uuid.UUID, from, to *time.Time) ([]models.DateHoliday, error) {
	q := r.db.Querier(ctx)
	query := `
		SELECT id, school_id, holiday_date, title, description, is_holiday, created_at, updated_at
		FROM school_date_holidays
		WHERE school_id = $1
	`
	args := []any{schoolID}

	if from != nil {
		args = append(args, *from)
		query += fmt.Sprintf(" AND holiday_date >= $%d", len(args))
	}
	if to != nil {
		args = append(args, *to)
		query += fmt.Sprintf(" AND holiday_date <= $%d", len(args))
	}

	query += " ORDER BY holiday_date ASC"

	rows, err := q.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query date holidays: %w", err)
	}
	defer rows.Close()

	var list []models.DateHoliday
	for rows.Next() {
		var h models.DateHoliday
		if err := rows.Scan(&h.ID, &h.SchoolID, &h.HolidayDate, &h.Title, &h.Description, &h.IsHoliday, &h.CreatedAt, &h.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan date holiday: %w", err)
		}
		list = append(list, h)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return list, nil
}

func (r *CalendarRepo) CreateDateHoliday(ctx context.Context, h models.DateHoliday) (*models.DateHoliday, error) {
	q := r.db.Querier(ctx)
	var created models.DateHoliday
	err := q.QueryRow(ctx, `
		INSERT INTO school_date_holidays (school_id, holiday_date, title, description, is_holiday, updated_at)
		VALUES ($1, $2, $3, $4, $5, now())
		ON CONFLICT (school_id, holiday_date)
		DO UPDATE SET title = EXCLUDED.title, description = EXCLUDED.description, is_holiday = EXCLUDED.is_holiday, updated_at = now()
		RETURNING id, school_id, holiday_date, title, description, is_holiday, created_at, updated_at
	`, h.SchoolID, h.HolidayDate, h.Title, h.Description, h.IsHoliday).Scan(
		&created.ID, &created.SchoolID, &created.HolidayDate, &created.Title, &created.Description, &created.IsHoliday, &created.CreatedAt, &created.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create date holiday: %w", err)
	}
	return &created, nil
}

func (r *CalendarRepo) DeleteDateHoliday(ctx context.Context, schoolID uuid.UUID, id uuid.UUID) error {
	q := r.db.Querier(ctx)
	ct, err := q.Exec(ctx, `
		DELETE FROM school_date_holidays
		WHERE school_id = $1 AND id = $2
	`, schoolID, id)
	if err != nil {
		return fmt.Errorf("delete date holiday: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return fmt.Errorf("holiday not found")
	}
	return nil
}
