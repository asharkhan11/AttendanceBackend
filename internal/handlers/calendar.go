package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/school/attendance-backend/internal/apperror"
	"github.com/school/attendance-backend/internal/auth"
	"github.com/school/attendance-backend/internal/dto"
	"github.com/school/attendance-backend/internal/httpx"
	"github.com/school/attendance-backend/internal/models"
	"github.com/school/attendance-backend/internal/services"
)

type CalendarHandler struct {
	svc *services.CalendarService
}

func NewCalendarHandler(svc *services.CalendarService) *CalendarHandler {
	return &CalendarHandler{svc: svc}
}

// GetWeeklySchedule godoc
// @Summary Get school's weekly working schedule
// @Description Returns the school working days configuration (0=Sunday, 1=Monday, etc.).
// @Tags Calendar
// @Accept json
// @Produce json
// @Security Bearer
// @Success 200 {object} httpx.Envelope{data=[]models.WeeklySchedule}
// @Failure 401 {object} httpx.Envelope{error=httpx.ErrBody}
// @Router /api/v1/school-days/weekly [get]
func (h *CalendarHandler) GetWeeklySchedule(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromContext(r.Context())
	if claims == nil {
		httpx.WriteError(w, apperror.Unauthorized("unauthorized"))
		return
	}

	schedules, err := h.svc.GetWeeklySchedule(r.Context(), claims.SchoolID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteOK(w, schedules)
}

// UpdateWeeklySchedule godoc
// @Summary Update school's weekly working schedule
// @Description Updates the working days weekly configuration.
// @Tags Calendar
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body models.UpdateWeeklyScheduleRequest true "Weekly schedule configuration"
// @Success 200 {object} httpx.Envelope{data=[]models.WeeklySchedule}
// @Failure 400 {object} httpx.Envelope{error=httpx.ErrBody}
// @Failure 401 {object} httpx.Envelope{error=httpx.ErrBody}
// @Failure 403 {object} httpx.Envelope{error=httpx.ErrBody}
// @Router /api/v1/school-days/weekly [put]
func (h *CalendarHandler) UpdateWeeklySchedule(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromContext(r.Context())
	if claims == nil {
		httpx.WriteError(w, apperror.Unauthorized("unauthorized"))
		return
	}

	var req models.UpdateWeeklyScheduleRequest
	if err := dto.Decode(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}

	schedules, err := h.svc.UpdateWeeklySchedule(r.Context(), claims.SchoolID, req)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteOK(w, schedules)
}

// GetDateHolidays godoc
// @Summary Get school holidays
// @Description Returns custom holiday/non-working dates for a specified range.
// @Tags Calendar
// @Accept json
// @Produce json
// @Security Bearer
// @Param from query string false "Start date (YYYY-MM-DD)"
// @Param to query string false "End date (YYYY-MM-DD)"
// @Success 200 {object} httpx.Envelope{data=[]models.DateHoliday}
// @Failure 401 {object} httpx.Envelope{error=httpx.ErrBody}
// @Router /api/v1/school-days/holidays [get]
func (h *CalendarHandler) GetDateHolidays(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromContext(r.Context())
	if claims == nil {
		httpx.WriteError(w, apperror.Unauthorized("unauthorized"))
		return
	}

	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")

	holidays, err := h.svc.GetDateHolidays(r.Context(), claims.SchoolID, from, to)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteOK(w, holidays)
}

// CreateDateHoliday godoc
// @Summary Add a new holiday/event date
// @Description Declares a specific date as a holiday or custom school day.
// @Tags Calendar
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body models.CreateHolidayRequest true "Holiday creation request"
// @Success 201 {object} httpx.Envelope{data=models.DateHoliday}
// @Failure 400 {object} httpx.Envelope{error=httpx.ErrBody}
// @Failure 401 {object} httpx.Envelope{error=httpx.ErrBody}
// @Failure 403 {object} httpx.Envelope{error=httpx.ErrBody}
// @Router /api/v1/school-days/holidays [post]
func (h *CalendarHandler) CreateDateHoliday(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromContext(r.Context())
	if claims == nil {
		httpx.WriteError(w, apperror.Unauthorized("unauthorized"))
		return
	}

	var req models.CreateHolidayRequest
	if err := dto.Decode(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}

	holiday, err := h.svc.CreateDateHoliday(r.Context(), claims.SchoolID, req)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteCreated(w, holiday)
}

// DeleteDateHoliday godoc
// @Summary Remove a holiday date
// @Description Deletes a declared holiday/custom date.
// @Tags Calendar
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path string true "Holiday ID (UUID)"
// @Success 204 "No Content"
// @Failure 400 {object} httpx.Envelope{error=httpx.ErrBody}
// @Failure 401 {object} httpx.Envelope{error=httpx.ErrBody}
// @Failure 403 {object} httpx.Envelope{error=httpx.ErrBody}
// @Router /api/v1/school-days/holidays/{id} [delete]
func (h *CalendarHandler) DeleteDateHoliday(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromContext(r.Context())
	if claims == nil {
		httpx.WriteError(w, apperror.Unauthorized("unauthorized"))
		return
	}

	idStr := chi.URLParam(r, "id")
	holidayID, err := uuid.Parse(idStr)
	if err != nil {
		httpx.WriteError(w, apperror.Validation("invalid holiday id"))
		return
	}

	if err := h.svc.DeleteDateHoliday(r.Context(), claims.SchoolID, holidayID); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteNoContent(w)
}
