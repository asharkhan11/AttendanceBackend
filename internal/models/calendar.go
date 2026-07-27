package models

import (
	"time"

	"github.com/google/uuid"
)

type WeeklySchedule struct {
	ID           uuid.UUID `json:"id"`
	SchoolID     uuid.UUID `json:"school_id"`
	DayOfWeek    int16     `json:"day_of_week"` // 0=Sunday, 1=Monday, ..., 6=Saturday
	DayName      string    `json:"day_name"`
	IsWorkingDay bool      `json:"is_working_day"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type DateHoliday struct {
	ID          uuid.UUID `json:"id"`
	SchoolID    uuid.UUID `json:"school_id"`
	HolidayDate time.Time `json:"holiday_date"`
	Title       string    `json:"title"`
	Description *string   `json:"description,omitempty"`
	IsHoliday   bool      `json:"is_holiday"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type UpdateWeeklyScheduleRequest struct {
	Schedules []WeeklyScheduleItem `json:"schedules"`
}

type WeeklyScheduleItem struct {
	DayOfWeek    int16 `json:"day_of_week"`
	IsWorkingDay bool  `json:"is_working_day"`
}

type CreateHolidayRequest struct {
	HolidayDate string  `json:"holiday_date"` // YYYY-MM-DD
	Title       string  `json:"title"`
	Description *string `json:"description,omitempty"`
	IsHoliday   *bool   `json:"is_holiday,omitempty"`
}
