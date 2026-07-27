package services

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/school/attendance-backend/internal/apperror"
	"github.com/school/attendance-backend/internal/models"
	"github.com/school/attendance-backend/internal/repository"
)

type CalendarService struct {
	repo *repository.CalendarRepo
}

func NewCalendarService(repo *repository.CalendarRepo) *CalendarService {
	return &CalendarService{repo: repo}
}

var defaultDayNames = map[int16]string{
	0: "Sunday",
	1: "Monday",
	2: "Tuesday",
	3: "Wednesday",
	4: "Thursday",
	5: "Friday",
	6: "Saturday",
}

func (s *CalendarService) GetWeeklySchedule(ctx context.Context, schoolID uuid.UUID) ([]models.WeeklySchedule, error) {
	existing, err := s.repo.GetWeeklySchedules(ctx, schoolID)
	if err != nil {
		return nil, fmt.Errorf("get weekly schedules: %w", err)
	}

	byDay := make(map[int16]models.WeeklySchedule)
	for _, item := range existing {
		byDay[item.DayOfWeek] = item
	}

	var result []models.WeeklySchedule
	for day := int16(0); day <= 6; day++ {
		if item, found := byDay[day]; found {
			result = append(result, item)
		} else {
			// Default: Sunday (0) is Holiday (false), Mon-Sat (1-6) are Working Days (true)
			isWorking := day != 0
			result = append(result, models.WeeklySchedule{
				SchoolID:     schoolID,
				DayOfWeek:    day,
				DayName:      defaultDayNames[day],
				IsWorkingDay: isWorking,
			})
		}
	}

	return result, nil
}

func (s *CalendarService) UpdateWeeklySchedule(ctx context.Context, schoolID uuid.UUID, req models.UpdateWeeklyScheduleRequest) ([]models.WeeklySchedule, error) {
	if len(req.Schedules) == 0 {
		return nil, apperror.Validation("schedules list cannot be empty")
	}

	for _, item := range req.Schedules {
		if item.DayOfWeek < 0 || item.DayOfWeek > 6 {
			return nil, apperror.Validation(fmt.Sprintf("invalid day_of_week: %d. Must be 0-6", item.DayOfWeek))
		}
		if err := s.repo.UpsertWeeklySchedule(ctx, schoolID, item.DayOfWeek, item.IsWorkingDay); err != nil {
			return nil, fmt.Errorf("upsert weekly schedule day %d: %w", item.DayOfWeek, err)
		}
	}

	return s.GetWeeklySchedule(ctx, schoolID)
}

func (s *CalendarService) GetDateHolidays(ctx context.Context, schoolID uuid.UUID, fromStr, toStr string) ([]models.DateHoliday, error) {
	var from, to *time.Time
	if fromStr != "" {
		t, err := time.Parse("2006-01-02", fromStr)
		if err != nil {
			return nil, apperror.Validation("from date must be format YYYY-MM-DD")
		}
		from = &t
	}
	if toStr != "" {
		t, err := time.Parse("2006-01-02", toStr)
		if err != nil {
			return nil, apperror.Validation("to date must be format YYYY-MM-DD")
		}
		to = &t
	}

	return s.repo.GetDateHolidays(ctx, schoolID, from, to)
}

func (s *CalendarService) CreateDateHoliday(ctx context.Context, schoolID uuid.UUID, req models.CreateHolidayRequest) (*models.DateHoliday, error) {
	if req.HolidayDate == "" {
		return nil, apperror.Validation("holiday_date is required (YYYY-MM-DD)")
	}
	if req.Title == "" {
		return nil, apperror.Validation("holiday title is required")
	}

	date, err := time.Parse("2006-01-02", req.HolidayDate)
	if err != nil {
		return nil, apperror.Validation("holiday_date must be format YYYY-MM-DD")
	}

	isHoliday := true
	if req.IsHoliday != nil {
		isHoliday = *req.IsHoliday
	}

	h := models.DateHoliday{
		SchoolID:    schoolID,
		HolidayDate: date,
		Title:       req.Title,
		Description: req.Description,
		IsHoliday:   isHoliday,
	}

	return s.repo.CreateDateHoliday(ctx, h)
}

func (s *CalendarService) DeleteDateHoliday(ctx context.Context, schoolID uuid.UUID, holidayID uuid.UUID) error {
	return s.repo.DeleteDateHoliday(ctx, schoolID, holidayID)
}
