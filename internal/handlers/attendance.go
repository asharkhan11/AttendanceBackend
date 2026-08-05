package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/school/attendance-backend/internal/apperror"
	"github.com/school/attendance-backend/internal/auth"
	"github.com/school/attendance-backend/internal/dto"
	"github.com/school/attendance-backend/internal/httpx"
	"github.com/school/attendance-backend/internal/services"
)

// ─── Attendance ───────────────────────────────────────────────────────────────

type AttendanceHandler struct {
	svc *services.AttendanceService
}

func NewAttendanceHandler(svc *services.AttendanceService) *AttendanceHandler {
	return &AttendanceHandler{svc}
}

// CreateSession godoc
// @Summary Create a new attendance session
// @Description Creates a new attendance session for a class, date, and period. Idempotent: returns existing if duplicate.
// @Tags Attendance
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body dto.CreateSessionRequest true "Session creation payload"
// @Success 201 {object} httpx.Envelope{data=models.AttendanceSession} "Session created"
// @Success 200 {object} httpx.Envelope{data=models.AttendanceSession} "Session already exists (idempotent)"
// @Failure 400 {object} httpx.Envelope{error=httpx.ErrBody}
// @Failure 401 {object} httpx.Envelope{error=httpx.ErrBody}
// @Router /api/v1/attendance/sessions [post]
func (h *AttendanceHandler) CreateSession(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateSessionRequest
	if err := dto.Decode(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	session, created, err := h.svc.CreateSession(r.Context(), &req)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	if created {
		httpx.WriteCreated(w, session)
	} else {
		httpx.WriteOK(w, session) // idempotent: existing session returned as 200
	}
}

// ListSessions godoc
// @Summary List attendance sessions
// @Description Returns a paginated list of attendance sessions, optionally filtered by class or date range.
// @Tags Attendance
// @Accept json
// @Produce json
// @Security Bearer
// @Param class_id query string false "Filter by Class ID (UUID)"
// @Param from query string false "Start date (YYYY-MM-DD)"
// @Param to query string false "End date (YYYY-MM-DD)"
// @Param page query int false "Page number"
// @Param page_size query int false "Page size"
// @Success 200 {object} httpx.Envelope{data=[]models.AttendanceSession}
// @Failure 401 {object} httpx.Envelope{error=httpx.ErrBody}
// @Router /api/v1/attendance/sessions [get]
func (h *AttendanceHandler) ListSessions(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	var classID *uuid.UUID
	if s := q.Get("class_id"); s != "" {
		if id, err := uuid.Parse(s); err == nil {
			classID = &id
		}
	}
	var from, to *time.Time
	if s := q.Get("from"); s != "" {
		if t, err := time.Parse("2006-01-02", s); err == nil {
			from = &t
		}
	}
	if s := q.Get("to"); s != "" {
		if t, err := time.Parse("2006-01-02", s); err == nil {
			to = &t
		}
	}
	pg := httpx.ParsePagination(r)
	items, total, err := h.svc.ListSessions(r.Context(), classID, from, to, pg)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteList(w, http.StatusOK, items, httpx.Meta{Page: pg.Page, PageSize: pg.PageSize, Total: total})
}

// GetSession godoc
// @Summary Get session details with roster
// @Description Retrieves an attendance session along with its current roster of students and their marks.
// @Tags Attendance
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path string true "Session ID (UUID)"
// @Success 200 {object} httpx.Envelope{data=services.SessionWithRoster}
// @Failure 400 {object} httpx.Envelope{error=httpx.ErrBody}
// @Failure 401 {object} httpx.Envelope{error=httpx.ErrBody}
// @Failure 404 {object} httpx.Envelope{error=httpx.ErrBody}
// @Router /api/v1/attendance/sessions/{id} [get]
func (h *AttendanceHandler) GetSession(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "id")
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	result, err := h.svc.GetSession(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteOK(w, result)
}

// BulkMark godoc
// @Summary Bulk mark attendance
// @Description Records attendance status for multiple students in a session.
// @Tags Attendance
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path string true "Session ID (UUID)"
// @Param request body dto.BulkMarkRequest true "List of student marks"
// @Success 200 {object} httpx.Envelope{data=map[string]int} "marked count"
// @Failure 400 {object} httpx.Envelope{error=httpx.ErrBody}
// @Failure 401 {object} httpx.Envelope{error=httpx.ErrBody}
// @Failure 403 {object} httpx.Envelope{error=httpx.ErrBody}
// @Router /api/v1/attendance/sessions/{id}/mark [post]
func (h *AttendanceHandler) BulkMark(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "id")
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	var req dto.BulkMarkRequest
	if err := dto.Decode(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	if err := h.svc.BulkMark(r.Context(), id, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteOK(w, map[string]any{"marked": len(req.Marks)})
}

// SingleMark godoc
// @Summary Mark attendance for a single student
// @Description Updates or records attendance status for one specific student in a session.
// @Tags Attendance
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path string true "Session ID (UUID)"
// @Param student_id path string true "Student ID (UUID)"
// @Param request body dto.SingleMarkRequest true "Single student mark details"
// @Success 200 {object} httpx.Envelope{data=map[string]string} "success message"
// @Failure 400 {object} httpx.Envelope{error=httpx.ErrBody}
// @Failure 401 {object} httpx.Envelope{error=httpx.ErrBody}
// @Failure 403 {object} httpx.Envelope{error=httpx.ErrBody}
// @Router /api/v1/attendance/sessions/{id}/mark/{student_id} [patch]
func (h *AttendanceHandler) SingleMark(w http.ResponseWriter, r *http.Request) {
	sessionID, err := parseUUID(r, "id")
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	studentID, err := parseUUID(r, "student_id")
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	var req dto.SingleMarkRequest
	if err := dto.Decode(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	if err := h.svc.SingleMark(r.Context(), sessionID, studentID, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteOK(w, map[string]any{"message": "attendance updated"})
}

// CloseSession godoc
// @Summary Finalize/Close attendance session
// @Description Locks the attendance session to prevent further modification and triggers WhatsApp notifications to parents of absent students.
// @Tags Attendance
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path string true "Session ID (UUID)"
// @Success 204 "No Content"
// @Failure 400 {object} httpx.Envelope{error=httpx.ErrBody}
// @Failure 401 {object} httpx.Envelope{error=httpx.ErrBody}
// @Failure 403 {object} httpx.Envelope{error=httpx.ErrBody}
// @Router /api/v1/attendance/sessions/{id}/close [post]
func (h *AttendanceHandler) CloseSession(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "id")
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	if err := h.svc.CloseSession(r.Context(), id); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteNoContent(w)
}

// ReopenSession godoc
// @Summary Reopen a closed attendance session
// @Description Reopens a finalized session to allow corrections (Admin only).
// @Tags Attendance
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path string true "Session ID (UUID)"
// @Success 204 "No Content"
// @Failure 400 {object} httpx.Envelope{error=httpx.ErrBody}
// @Failure 401 {object} httpx.Envelope{error=httpx.ErrBody}
// @Failure 403 {object} httpx.Envelope{error=httpx.ErrBody}
// @Router /api/v1/attendance/sessions/{id}/reopen [post]
func (h *AttendanceHandler) ReopenSession(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "id")
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	if err := h.svc.ReopenSession(r.Context(), id); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteNoContent(w)
}

// ─── Analytics & Reports ──────────────────────────────────────────────────────

type AnalyticsHandler struct {
	svc *services.AnalyticsService
}

func NewAnalyticsHandler(svc *services.AnalyticsService) *AnalyticsHandler {
	return &AnalyticsHandler{svc}
}

// ClassSummary godoc
// @Summary Get class attendance summary
// @Description Returns overall attendance statistics for a specific class within a date range.
// @Tags Dashboard
// @Accept json
// @Produce json
// @Security Bearer
// @Param class_id path string true "Class ID (UUID)"
// @Param from query string true "Start date (YYYY-MM-DD)"
// @Param to query string true "End date (YYYY-MM-DD)"
// @Success 200 {object} httpx.Envelope{data=repository.ClassSummaryData}
// @Failure 400 {object} httpx.Envelope{error=httpx.ErrBody}
// @Failure 401 {object} httpx.Envelope{error=httpx.ErrBody}
// @Router /api/v1/analytics/class/{class_id}/summary [get]
func (h *AnalyticsHandler) ClassSummary(w http.ResponseWriter, r *http.Request) {
	classID, err := parseUUID(r, "class_id")
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	q := r.URL.Query()
	from, to := q.Get("from"), q.Get("to")
	if from == "" || to == "" {
		httpx.WriteError(w, apperror.Validation("from and to query params are required (YYYY-MM-DD)"))
		return
	}
	result, err := h.svc.ClassSummary(r.Context(), classID, from, to)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteOK(w, result)
}

// StudentSummary godoc
// @Summary Get student attendance summary
// @Description Returns subject-wise and daily attendance summary logs for a student within a date range.
// @Tags Dashboard
// @Accept json
// @Produce json
// @Security Bearer
// @Param student_id path string true "Student ID (UUID)"
// @Param from query string true "Start date (YYYY-MM-DD)"
// @Param to query string true "End date (YYYY-MM-DD)"
// @Success 200 {object} httpx.Envelope{data=services.StudentSummaryResponse}
// @Failure 400 {object} httpx.Envelope{error=httpx.ErrBody}
// @Failure 401 {object} httpx.Envelope{error=httpx.ErrBody}
// @Router /api/v1/analytics/student/{student_id}/summary [get]
func (h *AnalyticsHandler) StudentSummary(w http.ResponseWriter, r *http.Request) {
	studentID, err := parseUUID(r, "student_id")
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	q := r.URL.Query()
	from, to := q.Get("from"), q.Get("to")
	if from == "" || to == "" {
		httpx.WriteError(w, apperror.Validation("from and to query params are required"))
		return
	}
	result, err := h.svc.StudentSummary(r.Context(), studentID, from, to)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteOK(w, result)
}

// Defaulters godoc
// @Summary Get class attendance defaulters
// @Description Returns a list of students in a class whose attendance percentage is below a threshold.
// @Tags Dashboard
// @Accept json
// @Produce json
// @Security Bearer
// @Param class_id path string true "Class ID (UUID)"
// @Param from query string true "Start date (YYYY-MM-DD)"
// @Param to query string true "End date (YYYY-MM-DD)"
// @Param threshold query number false "Attendance threshold percentage (default: 75.0)"
// @Success 200 {object} httpx.Envelope{data=[]repository.DefaulterRow}
// @Failure 400 {object} httpx.Envelope{error=httpx.ErrBody}
// @Failure 401 {object} httpx.Envelope{error=httpx.ErrBody}
// @Router /api/v1/analytics/class/{class_id}/defaulters [get]
func (h *AnalyticsHandler) Defaulters(w http.ResponseWriter, r *http.Request) {
	classID, err := parseUUID(r, "class_id")
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	q := r.URL.Query()
	from, to := q.Get("from"), q.Get("to")
	if from == "" || to == "" {
		httpx.WriteError(w, apperror.Validation("from and to required"))
		return
	}
	threshold := 75.0
	if t := q.Get("threshold"); t != "" {
		if v, err := strconv.ParseFloat(t, 64); err == nil {
			threshold = v
		}
	}
	result, err := h.svc.Defaulters(r.Context(), classID, from, to, threshold)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteOK(w, result)
}

// ClassTrend godoc
// @Summary Get class attendance trend
// @Description Returns time-series attendance statistics (weekly/monthly) for a class.
// @Tags Dashboard
// @Accept json
// @Produce json
// @Security Bearer
// @Param class_id path string true "Class ID (UUID)"
// @Param from query string true "Start date (YYYY-MM-DD)"
// @Param to query string true "End date (YYYY-MM-DD)"
// @Param granularity query string false "Granularity: week, month (default: week)"
// @Success 200 {object} httpx.Envelope{data=[]repository.TrendRow}
// @Failure 400 {object} httpx.Envelope{error=httpx.ErrBody}
// @Failure 401 {object} httpx.Envelope{error=httpx.ErrBody}
// @Router /api/v1/analytics/class/{class_id}/trend [get]
func (h *AnalyticsHandler) ClassTrend(w http.ResponseWriter, r *http.Request) {
	classID, err := parseUUID(r, "class_id")
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	q := r.URL.Query()
	from, to := q.Get("from"), q.Get("to")
	granularity := q.Get("granularity")
	if granularity != "month" {
		granularity = "week"
	}
	result, err := h.svc.ClassTrend(r.Context(), classID, from, to, granularity)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteOK(w, result)
}

// ExportClass godoc
// @Summary Export class attendance report
// @Description Downloads class attendance report as PDF or CSV.
// @Tags Reports
// @Accept json
// @Produce octet-stream
// @Security Bearer
// @Param class_id path string true "Class ID (UUID)"
// @Param from query string true "Start date (YYYY-MM-DD)"
// @Param to query string true "End date (YYYY-MM-DD)"
// @Param format query string false "Format: csv, pdf (default: csv)"
// @Success 200 {file} file "Class report file"
// @Failure 400 {object} httpx.Envelope{error=httpx.ErrBody}
// @Failure 401 {object} httpx.Envelope{error=httpx.ErrBody}
// @Router /api/v1/reports/class/{class_id}/export [get]
func (h *AnalyticsHandler) ExportClass(w http.ResponseWriter, r *http.Request) {
	classID, err := parseUUID(r, "class_id")
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	q := r.URL.Query()
	from, to := q.Get("from"), q.Get("to")
	format := q.Get("format")

	switch format {
	case "pdf":
		data, err := h.svc.ExportClassPDF(r.Context(), classID, from, to)
		if err != nil {
			httpx.WriteError(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"class_%s_%s_%s.pdf\"", classID, from, to))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	default: // csv
		data, err := h.svc.ExportClassCSV(r.Context(), classID, from, to)
		if err != nil {
			httpx.WriteError(w, err)
			return
		}
		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"class_%s_%s_%s.csv\"", classID, from, to))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	}
}

// ExportStudent godoc
// @Summary Export student attendance report
// @Description Downloads individual student attendance report as a PDF.
// @Tags Reports
// @Accept json
// @Produce application/pdf
// @Security Bearer
// @Param student_id path string true "Student ID (UUID)"
// @Param from query string true "Start date (YYYY-MM-DD)"
// @Param to query string true "End date (YYYY-MM-DD)"
// @Success 200 {file} file "Student report PDF file"
// @Failure 400 {object} httpx.Envelope{error=httpx.ErrBody}
// @Failure 401 {object} httpx.Envelope{error=httpx.ErrBody}
// @Router /api/v1/reports/student/{student_id}/export [get]
func (h *AnalyticsHandler) ExportStudent(w http.ResponseWriter, r *http.Request) {
	studentID, err := parseUUID(r, "student_id")
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	q := r.URL.Query()
	from, to := q.Get("from"), q.Get("to")
	data, err := h.svc.ExportStudentPDF(r.Context(), studentID, from, to)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"student_%s_%s_%s.pdf\"", studentID, from, to))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// SchoolOverview godoc
// @Summary Get school attendance overview dashboard
// @Description Returns school-wide statistics (classes, students, sessions, defaulters counts, and class comparisons).
// @Tags Dashboard
// @Accept json
// @Produce json
// @Security Bearer
// @Param academic_year_id query string false "Filter by Academic Year ID (UUID)"
// @Param class_id query string false "Filter by Class ID (UUID)"
// @Param from query string false "Start date (YYYY-MM-DD) (defaults to first day of current month)"
// @Param to query string false "End date (YYYY-MM-DD) (defaults to today)"
// @Param threshold query number false "Defaulter threshold percentage (default: 75.0)"
// @Success 200 {object} httpx.Envelope{data=repository.SchoolOverviewData}
// @Failure 401 {object} httpx.Envelope{error=httpx.ErrBody}
// @Router /api/v1/analytics/school/overview [get]
func (h *AnalyticsHandler) SchoolOverview(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromContext(r.Context())
	if claims == nil {
		httpx.WriteError(w, apperror.Unauthorized("unauthenticated"))
		return
	}
	q := r.URL.Query()
	var academicYearID *uuid.UUID
	if ay := q.Get("academic_year_id"); ay != "" {
		if id, err := uuid.Parse(ay); err == nil {
			academicYearID = &id
		}
	}
	var classID *uuid.UUID
	if cid := q.Get("class_id"); cid != "" {
		if id, err := uuid.Parse(cid); err == nil {
			classID = &id
		}
	}
	from, to := q.Get("from"), q.Get("to")
	if from == "" || to == "" {
		now := time.Now()
		from = fmt.Sprintf("%d-%02d-01", now.Year(), now.Month())
		to = now.Format("2006-01-02")
	}
	threshold := 75.0
	if t := q.Get("threshold"); t != "" {
		if v, err := strconv.ParseFloat(t, 64); err == nil {
			threshold = v
		}
	}
	data, err := h.svc.SchoolOverview(r.Context(), claims.SchoolID, classID, academicYearID, from, to, threshold)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteOK(w, data)
}

// SchoolTrend godoc
// @Summary Get school attendance trend
// @Description Returns school-wide time-series attendance statistics (weekly/monthly).
// @Tags Dashboard
// @Accept json
// @Produce json
// @Security Bearer
// @Param from query string false "Start date (YYYY-MM-DD) (defaults to first day of current month)"
// @Param to query string false "End date (YYYY-MM-DD) (defaults to today)"
// @Param granularity query string false "Granularity: week, month (default: week)"
// @Success 200 {object} httpx.Envelope{data=[]repository.TrendRow}
// @Failure 401 {object} httpx.Envelope{error=httpx.ErrBody}
// @Router /api/v1/analytics/school/trend [get]
func (h *AnalyticsHandler) SchoolTrend(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromContext(r.Context())
	if claims == nil {
		httpx.WriteError(w, apperror.Unauthorized("unauthenticated"))
		return
	}
	q := r.URL.Query()
	from, to := q.Get("from"), q.Get("to")
	if from == "" || to == "" {
		now := time.Now()
		from = fmt.Sprintf("%d-%02d-01", now.Year(), now.Month())
		to = now.Format("2006-01-02")
	}
	granularity := q.Get("granularity")
	if granularity == "" {
		granularity = "week"
	}
	data, err := h.svc.SchoolTrend(r.Context(), claims.SchoolID, from, to, granularity)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteOK(w, data)
}

// SchoolDefaulters godoc
// @Summary Get school attendance defaulters
// @Description Returns school-wide list of students below the attendance threshold.
// @Tags Dashboard
// @Accept json
// @Produce json
// @Security Bearer
// @Param class_id query string false "Filter by Class ID (UUID)"
// @Param from query string false "Start date (YYYY-MM-DD) (defaults to first day of current month)"
// @Param to query string false "End date (YYYY-MM-DD) (defaults to today)"
// @Param threshold query number false "Defaulter threshold percentage (default: 75.0)"
// @Success 200 {object} httpx.Envelope{data=[]repository.SchoolDefaulterRow}
// @Failure 401 {object} httpx.Envelope{error=httpx.ErrBody}
// @Router /api/v1/analytics/school/defaulters [get]
func (h *AnalyticsHandler) SchoolDefaulters(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromContext(r.Context())
	if claims == nil {
		httpx.WriteError(w, apperror.Unauthorized("unauthenticated"))
		return
	}
	q := r.URL.Query()
	var classID *uuid.UUID
	if cid := q.Get("class_id"); cid != "" {
		if id, err := uuid.Parse(cid); err == nil {
			classID = &id
		}
	}
	from, to := q.Get("from"), q.Get("to")
	if from == "" || to == "" {
		now := time.Now()
		from = fmt.Sprintf("%d-%02d-01", now.Year(), now.Month())
		to = now.Format("2006-01-02")
	}
	threshold := 75.0
	if t := q.Get("threshold"); t != "" {
		if v, err := strconv.ParseFloat(t, 64); err == nil {
			threshold = v
		}
	}
	data, err := h.svc.SchoolDefaulters(r.Context(), claims.SchoolID, classID, from, to, threshold)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteOK(w, data)
}

// MonthlyMatrix godoc
// @Summary Get monthly attendance matrix for a class
// @Description Returns daily attendance code grid for all active students in a class for a specific month.
// @Tags Dashboard
// @Accept json
// @Produce json
// @Security Bearer
// @Param class_id path string true "Class ID (UUID)"
// @Param year query int false "Year (default: current year)"
// @Param month query int false "Month (1-12) (default: current month)"
// @Success 200 {object} httpx.Envelope{data=repository.MonthlyMatrixData}
// @Failure 400 {object} httpx.Envelope{error=httpx.ErrBody}
// @Failure 401 {object} httpx.Envelope{error=httpx.ErrBody}
// @Router /api/v1/analytics/class/{class_id}/monthly-matrix [get]
func (h *AnalyticsHandler) MonthlyMatrix(w http.ResponseWriter, r *http.Request) {
	classID, err := parseUUID(r, "class_id")
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	q := r.URL.Query()
	year, _ := strconv.Atoi(q.Get("year"))
	month, _ := strconv.Atoi(q.Get("month"))
	if year == 0 || month == 0 {
		now := time.Now()
		year = now.Year()
		month = int(now.Month())
	}
	data, err := h.svc.MonthlyMatrix(r.Context(), classID, year, month)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteOK(w, data)
}

// ExportMonthlyMatrix godoc
// @Summary Export monthly attendance matrix to CSV
// @Description Downloads class monthly attendance matrix code grid as a CSV file.
// @Tags Reports
// @Accept json
// @Produce text/csv
// @Security Bearer
// @Param class_id path string true "Class ID (UUID)"
// @Param year query int false "Year (default: current year)"
// @Param month query int false "Month (1-12) (default: current month)"
// @Success 200 {file} file "Monthly matrix CSV file"
// @Failure 400 {object} httpx.Envelope{error=httpx.ErrBody}
// @Failure 401 {object} httpx.Envelope{error=httpx.ErrBody}
// @Router /api/v1/reports/class/{class_id}/monthly-matrix/export [get]
func (h *AnalyticsHandler) ExportMonthlyMatrix(w http.ResponseWriter, r *http.Request) {
	classID, err := parseUUID(r, "class_id")
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	q := r.URL.Query()
	year, _ := strconv.Atoi(q.Get("year"))
	month, _ := strconv.Atoi(q.Get("month"))
	if year == 0 || month == 0 {
		now := time.Now()
		year = now.Year()
		month = int(now.Month())
	}
	data, err := h.svc.ExportMonthlyMatrixCSV(r.Context(), classID, year, month)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"monthly_matrix_%d_%02d.csv\"", year, month))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// ExportDefaulters godoc
// @Summary Export school defaulters report to CSV
// @Description Downloads list of attendance defaulters below a threshold as a CSV file.
// @Tags Reports
// @Accept json
// @Produce text/csv
// @Security Bearer
// @Param class_id query string false "Filter by Class ID (UUID)"
// @Param from query string false "Start date (YYYY-MM-DD) (defaults to first day of current month)"
// @Param to query string false "End date (YYYY-MM-DD) (defaults to today)"
// @Param threshold query number false "Defaulter threshold percentage (default: 75.0)"
// @Success 200 {file} file "Defaulters CSV file"
// @Failure 401 {object} httpx.Envelope{error=httpx.ErrBody}
// @Router /api/v1/reports/defaulters/export [get]
func (h *AnalyticsHandler) ExportDefaulters(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromContext(r.Context())
	if claims == nil {
		httpx.WriteError(w, apperror.Unauthorized("unauthenticated"))
		return
	}
	q := r.URL.Query()
	var classID *uuid.UUID
	if cid := q.Get("class_id"); cid != "" {
		if id, err := uuid.Parse(cid); err == nil {
			classID = &id
		}
	}
	from, to := q.Get("from"), q.Get("to")
	if from == "" || to == "" {
		now := time.Now()
		from = fmt.Sprintf("%d-%02d-01", now.Year(), now.Month())
		to = now.Format("2006-01-02")
	}
	threshold := 75.0
	if t := q.Get("threshold"); t != "" {
		if v, err := strconv.ParseFloat(t, 64); err == nil {
			threshold = v
		}
	}
	data, err := h.svc.ExportDefaultersCSV(r.Context(), claims.SchoolID, classID, from, to, threshold)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment; filename=\"defaulters_report.csv\"")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// AuditLogs godoc
// @Summary Get attendance change audit logs
// @Description Returns audit records tracking all attendance state modifications.
// @Tags Dashboard
// @Accept json
// @Produce json
// @Security Bearer
// @Param from query string false "Start date (YYYY-MM-DD) (defaults to first day of current month)"
// @Param to query string false "End date (YYYY-MM-DD) (defaults to today)"
// @Success 200 {object} httpx.Envelope{data=[]repository.AuditLogRow}
// @Failure 401 {object} httpx.Envelope{error=httpx.ErrBody}
// @Router /api/v1/analytics/school/audit-logs [get]
func (h *AnalyticsHandler) AuditLogs(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromContext(r.Context())
	if claims == nil {
		httpx.WriteError(w, apperror.Unauthorized("unauthenticated"))
		return
	}
	q := r.URL.Query()
	from, to := q.Get("from"), q.Get("to")
	if from == "" || to == "" {
		now := time.Now()
		from = fmt.Sprintf("%d-%02d-01", now.Year(), now.Month())
		to = now.Format("2006-01-02")
	}
	logs, err := h.svc.AuditLogs(r.Context(), claims.SchoolID, from, to)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteOK(w, logs)
}

// ExportAuditLogs godoc
// @Summary Export attendance audit logs to CSV
// @Description Downloads change logs history sheet as a CSV file.
// @Tags Reports
// @Accept json
// @Produce text/csv
// @Security Bearer
// @Param from query string false "Start date (YYYY-MM-DD) (defaults to first day of current month)"
// @Param to query string false "End date (YYYY-MM-DD) (defaults to today)"
// @Success 200 {file} file "Audit logs CSV file"
// @Failure 401 {object} httpx.Envelope{error=httpx.ErrBody}
// @Router /api/v1/reports/audit-logs/export [get]
func (h *AnalyticsHandler) ExportAuditLogs(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromContext(r.Context())
	if claims == nil {
		httpx.WriteError(w, apperror.Unauthorized("unauthenticated"))
		return
	}
	q := r.URL.Query()
	from, to := q.Get("from"), q.Get("to")
	if from == "" || to == "" {
		now := time.Now()
		from = fmt.Sprintf("%d-%02d-01", now.Year(), now.Month())
		to = now.Format("2006-01-02")
	}
	data, err := h.svc.ExportAuditLogCSV(r.Context(), claims.SchoolID, from, to)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment; filename=\"audit_logs.csv\"")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// StudentTemplate godoc
// @Summary Download CSV import template for students
// @Description Returns a starter template file for bulk student import.
// @Tags Reports
// @Accept json
// @Produce text/csv
// @Security Bearer
// @Success 200 {file} file "Students template CSV file"
// @Router /api/v1/reports/templates/students [get]
func (h *AnalyticsHandler) StudentTemplate(w http.ResponseWriter, r *http.Request) {
	data := h.svc.GetStudentTemplateCSV()
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment; filename=\"students_template.csv\"")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// TeacherTemplate godoc
// @Summary Download CSV import template for teachers
// @Description Returns a starter template file for bulk teacher import.
// @Tags Reports
// @Accept json
// @Produce text/csv
// @Security Bearer
// @Success 200 {file} file "Teachers template CSV file"
// @Router /api/v1/reports/templates/teachers [get]
func (h *AnalyticsHandler) TeacherTemplate(w http.ResponseWriter, r *http.Request) {
	data := h.svc.GetTeacherTemplateCSV()
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment; filename=\"teachers_template.csv\"")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

