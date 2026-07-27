package handlers

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/school/attendance-backend/internal/apperror"
	"github.com/school/attendance-backend/internal/dto"
	"github.com/school/attendance-backend/internal/httpx"
	"github.com/school/attendance-backend/internal/services"
)

// ─── Classes ──────────────────────────────────────────────────────────────────

type ClassHandler struct {
	svc    *services.ClassService
	stuSvc *services.StudentService
}

func NewClassHandler(svc *services.ClassService, stuSvc *services.StudentService) *ClassHandler {
	return &ClassHandler{svc, stuSvc}
}

func (h *ClassHandler) List(w http.ResponseWriter, r *http.Request) {
	pg := httpx.ParsePagination(r)
	var yearID *uuid.UUID
	if yStr := r.URL.Query().Get("academic_year_id"); yStr != "" {
		if id, err := uuid.Parse(yStr); err == nil {
			yearID = &id
		}
	}
	items, total, err := h.svc.List(r.Context(), yearID, pg)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteList(w, http.StatusOK, items, httpx.Meta{Page: pg.Page, PageSize: pg.PageSize, Total: total})
}

func (h *ClassHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateClassRequest
	if err := dto.Decode(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	cls, err := h.svc.Create(r.Context(), &req)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteCreated(w, cls)
}

func (h *ClassHandler) GetMyClass(w http.ResponseWriter, r *http.Request) {
	cls, err := h.svc.GetMyClass(r.Context())
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteOK(w, cls)
}

func (h *ClassHandler) AssignTeacher(w http.ResponseWriter, r *http.Request) {
	classID, err := parseUUID(r, "id")
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	var req dto.AssignClassTeacherRequest
	if err := dto.Decode(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	var teacherID *uuid.UUID
	if req.ClassTeacherID != nil && *req.ClassTeacherID != "" {
		id, err := uuid.Parse(*req.ClassTeacherID)
		if err != nil {
			httpx.WriteError(w, apperror.Validation("invalid class_teacher_id"))
			return
		}
		teacherID = &id
	}
	cls, err := h.svc.AssignTeacher(r.Context(), classID, teacherID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteOK(w, cls)
}

func (h *ClassHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "id")
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	if err := h.svc.Delete(r.Context(), id); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteNoContent(w)
}

func (h *ClassHandler) ListStudents(w http.ResponseWriter, r *http.Request) {
	classID, err := parseUUID(r, "id")
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	pg := httpx.ParsePagination(r)
	items, total, err := h.stuSvc.ListByClass(r.Context(), classID, pg)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteList(w, http.StatusOK, items, httpx.Meta{Page: pg.Page, PageSize: pg.PageSize, Total: total})
}

// ─── Teachers ─────────────────────────────────────────────────────────────────

type TeacherHandler struct{ svc *services.TeacherService }

func NewTeacherHandler(svc *services.TeacherService) *TeacherHandler { return &TeacherHandler{svc} }

func (h *TeacherHandler) List(w http.ResponseWriter, r *http.Request) {
	pg := httpx.ParsePagination(r)
	items, total, err := h.svc.List(r.Context(), pg)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteList(w, http.StatusOK, items, httpx.Meta{Page: pg.Page, PageSize: pg.PageSize, Total: total})
}

func (h *TeacherHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateTeacherRequest
	if err := dto.Decode(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	u, err := h.svc.Create(r.Context(), &req)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteCreated(w, u)
}

func (h *TeacherHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "id")
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	var req dto.UpdateTeacherRequest
	if err := dto.Decode(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	u, err := h.svc.Update(r.Context(), id, &req)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteOK(w, u)
}

func (h *TeacherHandler) Deactivate(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "id")
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	if err := h.svc.Deactivate(r.Context(), id); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteNoContent(w)
}

func (h *TeacherHandler) Activate(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "id")
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	if err := h.svc.Activate(r.Context(), id); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteNoContent(w)
}

func (h *TeacherHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "id")
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	if err := h.svc.Delete(r.Context(), id); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteNoContent(w)
}

func (h *TeacherHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "id")
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	var req dto.ResetPasswordRequest
	if err := dto.Decode(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	if err := h.svc.ResetPassword(r.Context(), id, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteNoContent(w)
}

// ─── Students ─────────────────────────────────────────────────────────────────

type StudentHandler struct{ svc *services.StudentService }

func NewStudentHandler(svc *services.StudentService) *StudentHandler { return &StudentHandler{svc} }

func (h *StudentHandler) List(w http.ResponseWriter, r *http.Request) {
	classIDStr := r.URL.Query().Get("class_id")
	yearIDStr := r.URL.Query().Get("academic_year_id")
	search := r.URL.Query().Get("search")

	var classID *uuid.UUID
	if classIDStr != "" {
		id, err := uuid.Parse(classIDStr)
		if err == nil {
			classID = &id
		}
	}
	var yearID *uuid.UUID
	if yearIDStr != "" {
		id, err := uuid.Parse(yearIDStr)
		if err == nil {
			yearID = &id
		}
	}

	pg := httpx.ParsePagination(r)
	items, total, err := h.svc.List(r.Context(), yearID, classID, search, pg)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteList(w, http.StatusOK, items, httpx.Meta{Page: pg.Page, PageSize: pg.PageSize, Total: total})
}

// ─── Academic Years Handler ──────────────────────────────────────────────────

type AcademicYearHandler struct {
	svc *services.AcademicYearService
}

func NewAcademicYearHandler(svc *services.AcademicYearService) *AcademicYearHandler {
	return &AcademicYearHandler{svc: svc}
}

func (h *AcademicYearHandler) List(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.List(r.Context())
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteOK(w, items)
}

func (h *AcademicYearHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateAcademicYearRequest
	if err := dto.Decode(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	ay, err := h.svc.Create(r.Context(), &req)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteCreated(w, ay)
}

func (h *AcademicYearHandler) SetCurrent(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "id")
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	if err := h.svc.SetCurrent(r.Context(), id); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteNoContent(w)
}

func (h *AcademicYearHandler) Archive(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "id")
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	if err := h.svc.SetArchived(r.Context(), id, true); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteNoContent(w)
}

func (h *AcademicYearHandler) Unarchive(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "id")
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	if err := h.svc.SetArchived(r.Context(), id, false); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteNoContent(w)
}

func (h *AcademicYearHandler) PromoteStudents(w http.ResponseWriter, r *http.Request) {
	var req dto.PromoteStudentsRequest
	if err := dto.Decode(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	count, err := h.svc.PromoteStudents(r.Context(), &req)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteOK(w, map[string]any{"promoted": count})
}

func (h *AcademicYearHandler) CopyClasses(w http.ResponseWriter, r *http.Request) {
	var req dto.CopyClassesRequest
	if err := dto.Decode(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	count, err := h.svc.CopyClasses(r.Context(), &req)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteOK(w, map[string]any{"copied": count})
}

func (h *AcademicYearHandler) CopyTeachers(w http.ResponseWriter, r *http.Request) {
	var req dto.CopyTeachersRequest
	if err := dto.Decode(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	count, err := h.svc.CopyTeachers(r.Context(), &req)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteOK(w, map[string]any{"copied": count})
}

func (h *StudentHandler) NextStudentInfo(w http.ResponseWriter, r *http.Request) {
	classIDStr := r.URL.Query().Get("class_id")
	if classIDStr == "" {
		httpx.WriteError(w, apperror.Validation("class_id query param is required"))
		return
	}
	classID, err := uuid.Parse(classIDStr)
	if err != nil {
		httpx.WriteError(w, apperror.Validation("invalid class_id"))
		return
	}
	info, err := h.svc.GetNextStudentInfo(r.Context(), classID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteOK(w, info)
}

func (h *StudentHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateStudentRequest
	if err := dto.Decode(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	st, err := h.svc.Create(r.Context(), &req)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteCreated(w, st)
}

func (h *StudentHandler) BulkImport(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		httpx.WriteError(w, apperror.Validation("invalid multipart form"))
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		httpx.WriteError(w, apperror.Validation("file field required"))
		return
	}
	defer file.Close()
	count, errs, err := h.svc.BulkImportCSV(r.Context(), file)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteCreated(w, map[string]any{"imported": count, "errors": errs})
}

func (h *StudentHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "id")
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	var req dto.UpdateStudentRequest
	if err := dto.Decode(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	st, err := h.svc.Update(r.Context(), id, &req)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteOK(w, st)
}

func (h *StudentHandler) Deactivate(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "id")
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	if err := h.svc.Deactivate(r.Context(), id); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteNoContent(w)
}

func (h *StudentHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "id")
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	if err := h.svc.Delete(r.Context(), id); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteNoContent(w)
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func parseUUID(r *http.Request, param string) (uuid.UUID, error) {
	s := chi.URLParam(r, param)
	s = strings.TrimSpace(s)
	id, err := uuid.Parse(s)
	if err != nil {
		return uuid.Nil, apperror.Validation("invalid " + param + " in URL")
	}
	return id, nil
}
