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
