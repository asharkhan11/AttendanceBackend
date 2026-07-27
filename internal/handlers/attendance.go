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

func (h *AnalyticsHandler) StudentTemplate(w http.ResponseWriter, r *http.Request) {
	data := h.svc.GetStudentTemplateCSV()
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment; filename=\"students_template.csv\"")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (h *AnalyticsHandler) TeacherTemplate(w http.ResponseWriter, r *http.Request) {
	data := h.svc.GetTeacherTemplateCSV()
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment; filename=\"teachers_template.csv\"")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

