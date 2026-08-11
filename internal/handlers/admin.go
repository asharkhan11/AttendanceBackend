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

// List godoc
// @Summary List all classes
// @Description Returns a paginated list of classes, optionally filtered by academic year.
// @Tags Classes
// @Accept json
// @Produce json
// @Security Bearer
// @Param academic_year_id query string false "Filter by Academic Year ID (UUID)"
// @Param page query int false "Page number (default: 1)"
// @Param page_size query int false "Page size (default: 20)"
// @Success 200 {object} httpx.Envelope{data=[]repository.ClassWithTeacher}
// @Failure 401 {object} httpx.Envelope{error=httpx.ErrBody}
// @Router /api/v1/classes [get]
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

// Create godoc
// @Summary Create a new class
// @Description Creates a new class inside the current or specified academic year.
// @Tags Classes
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body dto.CreateClassRequest true "Class creation payload"
// @Success 201 {object} httpx.Envelope{data=repository.ClassWithTeacher}
// @Failure 400 {object} httpx.Envelope{error=httpx.ErrBody}
// @Failure 401 {object} httpx.Envelope{error=httpx.ErrBody}
// @Failure 403 {object} httpx.Envelope{error=httpx.ErrBody}
// @Failure 409 {object} httpx.Envelope{error=httpx.ErrBody}
// @Router /api/v1/classes [post]
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

// GetMyClass godoc
// @Summary Get class details for the logged-in teacher
// @Description Returns the class details of which the logged-in teacher is a class teacher.
// @Tags Classes
// @Accept json
// @Produce json
// @Security Bearer
// @Success 200 {object} httpx.Envelope{data=repository.ClassWithTeacher}
// @Failure 401 {object} httpx.Envelope{error=httpx.ErrBody}
// @Failure 404 {object} httpx.Envelope{error=httpx.ErrBody}
// @Router /api/v1/classes/my-class [get]
func (h *ClassHandler) GetMyClass(w http.ResponseWriter, r *http.Request) {
	cls, err := h.svc.GetMyClass(r.Context())
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteOK(w, cls)
}

// AssignTeacher godoc
// @Summary Assign a class teacher to a class
// @Description Assigns or unassigns a teacher to act as class teacher for a specific class.
// @Tags Classes
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path string true "Class ID (UUID)"
// @Param request body dto.AssignClassTeacherRequest true "Teacher assignment payload"
// @Success 200 {object} httpx.Envelope{data=repository.ClassWithTeacher}
// @Failure 400 {object} httpx.Envelope{error=httpx.ErrBody}
// @Failure 401 {object} httpx.Envelope{error=httpx.ErrBody}
// @Failure 403 {object} httpx.Envelope{error=httpx.ErrBody}
// @Failure 409 {object} httpx.Envelope{error=httpx.ErrBody}
// @Router /api/v1/classes/{id}/teacher [put]
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

// Delete godoc
// @Summary Delete a class
// @Description Deletes a specific class.
// @Tags Classes
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path string true "Class ID (UUID)"
// @Success 204 "No Content"
// @Failure 400 {object} httpx.Envelope{error=httpx.ErrBody}
// @Failure 401 {object} httpx.Envelope{error=httpx.ErrBody}
// @Failure 403 {object} httpx.Envelope{error=httpx.ErrBody}
// @Router /api/v1/classes/{id} [delete]
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

// ListStudents godoc
// @Summary List students in a class
// @Description Returns a paginated list of students enrolled in a specific class.
// @Tags Classes
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path string true "Class ID (UUID)"
// @Param page query int false "Page number"
// @Param page_size query int false "Page size"
// @Success 200 {object} httpx.Envelope{data=[]models.Student}
// @Failure 400 {object} httpx.Envelope{error=httpx.ErrBody}
// @Failure 401 {object} httpx.Envelope{error=httpx.ErrBody}
// @Router /api/v1/classes/{id}/students [get]
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

// List godoc
// @Summary List all teachers
// @Description Returns a paginated list of all teachers in the school.
// @Tags Teachers
// @Accept json
// @Produce json
// @Security Bearer
// @Param page query int false "Page number"
// @Param page_size query int false "Page size"
// @Success 200 {object} httpx.Envelope{data=[]models.User}
// @Failure 401 {object} httpx.Envelope{error=httpx.ErrBody}
// @Failure 403 {object} httpx.Envelope{error=httpx.ErrBody}
// @Router /api/v1/teachers [get]
func (h *TeacherHandler) List(w http.ResponseWriter, r *http.Request) {
	pg := httpx.ParsePagination(r)
	items, total, err := h.svc.List(r.Context(), pg)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteList(w, http.StatusOK, items, httpx.Meta{Page: pg.Page, PageSize: pg.PageSize, Total: total})
}

// Create godoc
// @Summary Create a new teacher user
// @Description Registers a new teacher in the school database.
// @Tags Teachers
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body dto.CreateTeacherRequest true "Teacher creation payload"
// @Success 201 {object} httpx.Envelope{data=models.User}
// @Failure 400 {object} httpx.Envelope{error=httpx.ErrBody}
// @Failure 401 {object} httpx.Envelope{error=httpx.ErrBody}
// @Failure 403 {object} httpx.Envelope{error=httpx.ErrBody}
// @Failure 409 {object} httpx.Envelope{error=httpx.ErrBody}
// @Router /api/v1/teachers [post]
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

// Update godoc
// @Summary Update teacher details
// @Description Updates the profile details (name, phone) of a specific teacher.
// @Tags Teachers
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path string true "Teacher User ID (UUID)"
// @Param request body dto.UpdateTeacherRequest true "Teacher update payload"
// @Success 200 {object} httpx.Envelope{data=models.User}
// @Failure 400 {object} httpx.Envelope{error=httpx.ErrBody}
// @Failure 401 {object} httpx.Envelope{error=httpx.ErrBody}
// @Failure 403 {object} httpx.Envelope{error=httpx.ErrBody}
// @Router /api/v1/teachers/{id} [put]
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

// Deactivate godoc
// @Summary Deactivate a teacher account
// @Description Deactivates a teacher account, preventing them from logging in.
// @Tags Teachers
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path string true "Teacher User ID (UUID)"
// @Success 204 "No Content"
// @Failure 400 {object} httpx.Envelope{error=httpx.ErrBody}
// @Failure 401 {object} httpx.Envelope{error=httpx.ErrBody}
// @Failure 403 {object} httpx.Envelope{error=httpx.ErrBody}
// @Router /api/v1/teachers/{id}/deactivate [patch]
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

// Activate godoc
// @Summary Activate a teacher account
// @Description Activates a previously deactivated teacher account.
// @Tags Teachers
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path string true "Teacher User ID (UUID)"
// @Success 204 "No Content"
// @Failure 400 {object} httpx.Envelope{error=httpx.ErrBody}
// @Failure 401 {object} httpx.Envelope{error=httpx.ErrBody}
// @Failure 403 {object} httpx.Envelope{error=httpx.ErrBody}
// @Router /api/v1/teachers/{id}/activate [patch]
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

// Delete godoc
// @Summary Delete a teacher user
// @Description Hard-deletes a teacher user account.
// @Tags Teachers
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path string true "Teacher User ID (UUID)"
// @Success 204 "No Content"
// @Failure 400 {object} httpx.Envelope{error=httpx.ErrBody}
// @Failure 401 {object} httpx.Envelope{error=httpx.ErrBody}
// @Failure 403 {object} httpx.Envelope{error=httpx.ErrBody}
// @Router /api/v1/teachers/{id} [delete]
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

// ResetPassword godoc
// @Summary Reset a teacher's password
// @Description Forces a password reset for a specific teacher's account.
// @Tags Teachers
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path string true "Teacher User ID (UUID)"
// @Param request body dto.ResetPasswordRequest true "Password details"
// @Success 204 "No Content"
// @Failure 400 {object} httpx.Envelope{error=httpx.ErrBody}
// @Failure 401 {object} httpx.Envelope{error=httpx.ErrBody}
// @Failure 403 {object} httpx.Envelope{error=httpx.ErrBody}
// @Router /api/v1/teachers/{id}/reset-password [post]
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

// BulkImport godoc
// @Summary Bulk import teachers from CSV
// @Description Imports multiple teachers via a multipart CSV file upload.
// @Tags Teachers
// @Accept multipart/form-data
// @Produce json
// @Security Bearer
// @Param file formData file true "CSV file containing teachers list"
// @Success 201 {object} httpx.Envelope{data=map[string]any} "imported count and skip details"
// @Failure 400 {object} httpx.Envelope{error=httpx.ErrBody}
// @Failure 401 {object} httpx.Envelope{error=httpx.ErrBody}
// @Failure 403 {object} httpx.Envelope{error=httpx.ErrBody}
// @Router /api/v1/teachers/bulk-import [post]
func (h *TeacherHandler) BulkImport(w http.ResponseWriter, r *http.Request) {
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
	if count == 0 && len(errs) > 0 {
		httpx.WriteError(w, apperror.Validation("bulk import failed", apperror.FieldError{Field: "csv", Issue: strings.Join(errs, "; ")}))
		return
	}
	httpx.WriteCreated(w, map[string]any{"imported": count, "errors": errs})
}

// ─── Students ─────────────────────────────────────────────────────────────────

type StudentHandler struct{ svc *services.StudentService }

func NewStudentHandler(svc *services.StudentService) *StudentHandler { return &StudentHandler{svc} }

// List godoc
// @Summary List all students
// @Description Returns a paginated list of students, optionally filtered by academic year, class, or search query.
// @Tags Students
// @Accept json
// @Produce json
// @Security Bearer
// @Param academic_year_id query string false "Filter by Academic Year ID (UUID)"
// @Param class_id query string false "Filter by Class ID (UUID)"
// @Param search query string false "Search by student name or roll number"
// @Param page query int false "Page number"
// @Param page_size query int false "Page size"
// @Success 200 {object} httpx.Envelope{data=[]models.Student}
// @Failure 401 {object} httpx.Envelope{error=httpx.ErrBody}
// @Failure 403 {object} httpx.Envelope{error=httpx.ErrBody}
// @Router /api/v1/students [get]
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

// List godoc
// @Summary List all academic years
// @Description Returns all academic years configuration for the school.
// @Tags Academic Years
// @Accept json
// @Produce json
// @Security Bearer
// @Success 200 {object} httpx.Envelope{data=[]models.AcademicYear}
// @Failure 401 {object} httpx.Envelope{error=httpx.ErrBody}
// @Failure 403 {object} httpx.Envelope{error=httpx.ErrBody}
// @Router /api/v1/academic-years [get]
func (h *AcademicYearHandler) List(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.List(r.Context())
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteOK(w, items)
}

// Create godoc
// @Summary Create a new academic year
// @Description Registers a new academic session/year for the school.
// @Tags Academic Years
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body dto.CreateAcademicYearRequest true "Academic year creation payload"
// @Success 201 {object} httpx.Envelope{data=models.AcademicYear}
// @Failure 400 {object} httpx.Envelope{error=httpx.ErrBody}
// @Failure 401 {object} httpx.Envelope{error=httpx.ErrBody}
// @Failure 403 {object} httpx.Envelope{error=httpx.ErrBody}
// @Failure 409 {object} httpx.Envelope{error=httpx.ErrBody}
// @Router /api/v1/academic-years [post]
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

// SetCurrent godoc
// @Summary Set current active academic year
// @Description Activates a specific academic year session as the current active session.
// @Tags Academic Years
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path string true "Academic Year ID (UUID)"
// @Success 204 "No Content"
// @Failure 400 {object} httpx.Envelope{error=httpx.ErrBody}
// @Failure 401 {object} httpx.Envelope{error=httpx.ErrBody}
// @Failure 403 {object} httpx.Envelope{error=httpx.ErrBody}
// @Router /api/v1/academic-years/{id}/set-current [patch]
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

// Archive godoc
// @Summary Archive an academic year
// @Description Archives a past academic year to prevent modifications.
// @Tags Academic Years
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path string true "Academic Year ID (UUID)"
// @Success 204 "No Content"
// @Failure 400 {object} httpx.Envelope{error=httpx.ErrBody}
// @Failure 401 {object} httpx.Envelope{error=httpx.ErrBody}
// @Failure 403 {object} httpx.Envelope{error=httpx.ErrBody}
// @Router /api/v1/academic-years/{id}/archive [patch]
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

// Unarchive godoc
// @Summary Unarchive an academic year
// @Description Restores an archived academic year back to available state.
// @Tags Academic Years
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path string true "Academic Year ID (UUID)"
// @Success 204 "No Content"
// @Failure 400 {object} httpx.Envelope{error=httpx.ErrBody}
// @Failure 401 {object} httpx.Envelope{error=httpx.ErrBody}
// @Failure 403 {object} httpx.Envelope{error=httpx.ErrBody}
// @Router /api/v1/academic-years/{id}/unarchive [patch]
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

// PromoteStudents godoc
// @Summary Promote students to next class
// @Description Bulk promotes a list of students from a source class to a target class.
// @Tags Academic Years
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body dto.PromoteStudentsRequest true "Students promotion payload"
// @Success 200 {object} httpx.Envelope{data=map[string]int} "promoted count"
// @Failure 400 {object} httpx.Envelope{error=httpx.ErrBody}
// @Failure 401 {object} httpx.Envelope{error=httpx.ErrBody}
// @Failure 403 {object} httpx.Envelope{error=httpx.ErrBody}
// @Router /api/v1/academic-years/promote-students [post]
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

// CopyClasses godoc
// @Summary Copy classes schema to target academic year
// @Description Duplicates classes setup from a source academic year into a target academic year.
// @Tags Academic Years
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body dto.CopyClassesRequest true "Copy classes request"
// @Success 200 {object} httpx.Envelope{data=map[string]int} "copied count"
// @Failure 400 {object} httpx.Envelope{error=httpx.ErrBody}
// @Failure 401 {object} httpx.Envelope{error=httpx.ErrBody}
// @Failure 403 {object} httpx.Envelope{error=httpx.ErrBody}
// @Router /api/v1/academic-years/copy-classes [post]
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

// CopyTeachers godoc
// @Summary Copy class teacher assignments to target year
// @Description Copies teacher-to-class mapping from a source academic year to a target academic year.
// @Tags Academic Years
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body dto.CopyTeachersRequest true "Copy teachers request"
// @Success 200 {object} httpx.Envelope{data=map[string]int} "copied count"
// @Failure 400 {object} httpx.Envelope{error=httpx.ErrBody}
// @Failure 401 {object} httpx.Envelope{error=httpx.ErrBody}
// @Failure 403 {object} httpx.Envelope{error=httpx.ErrBody}
// @Router /api/v1/academic-years/copy-teachers [post]
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

// NextStudentInfo godoc
// @Summary Get suggestion for next student's roll and admission number
// @Description Calculates the next sequential roll number and admission number for a class.
// @Tags Students
// @Accept json
// @Produce json
// @Security Bearer
// @Param class_id query string true "Class ID (UUID)"
// @Success 200 {object} httpx.Envelope{data=repository.NextStudentInfo}
// @Failure 400 {object} httpx.Envelope{error=httpx.ErrBody}
// @Failure 401 {object} httpx.Envelope{error=httpx.ErrBody}
// @Router /api/v1/students/next-info [get]
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

// Create godoc
// @Summary Create a new student
// @Description Registers a new student in a specific class.
// @Tags Students
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body dto.CreateStudentRequest true "Student creation payload"
// @Success 201 {object} httpx.Envelope{data=models.Student}
// @Failure 400 {object} httpx.Envelope{error=httpx.ErrBody}
// @Failure 401 {object} httpx.Envelope{error=httpx.ErrBody}
// @Failure 403 {object} httpx.Envelope{error=httpx.ErrBody}
// @Failure 409 {object} httpx.Envelope{error=httpx.ErrBody}
// @Router /api/v1/students [post]
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

// BulkImport godoc
// @Summary Bulk import students from CSV
// @Description Imports multiple students via a multipart CSV file upload.
// @Tags Students
// @Accept multipart/form-data
// @Produce json
// @Security Bearer
// @Param file formData file true "CSV file containing students list"
// @Success 201 {object} httpx.Envelope{data=map[string]any} "imported count and skip details"
// @Failure 400 {object} httpx.Envelope{error=httpx.ErrBody}
// @Failure 401 {object} httpx.Envelope{error=httpx.ErrBody}
// @Failure 403 {object} httpx.Envelope{error=httpx.ErrBody}
// @Router /api/v1/students/bulk-import [post]
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

	classIDStr := r.FormValue("class_id")
	if classIDStr == "" {
		classIDStr = r.URL.Query().Get("class_id")
	}

	count, errs, err := h.svc.BulkImportCSV(r.Context(), file, classIDStr)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	if count == 0 && len(errs) > 0 {
		httpx.WriteError(w, apperror.Validation("bulk import failed", apperror.FieldError{Field: "csv", Issue: strings.Join(errs, "; ")}))
		return
	}
	httpx.WriteCreated(w, map[string]any{"imported": count, "errors": errs})
}

// Update godoc
// @Summary Update student details
// @Description Updates the registration profile of a specific student.
// @Tags Students
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path string true "Student ID (UUID)"
// @Param request body dto.UpdateStudentRequest true "Student update payload"
// @Success 200 {object} httpx.Envelope{data=models.Student}
// @Failure 400 {object} httpx.Envelope{error=httpx.ErrBody}
// @Failure 401 {object} httpx.Envelope{error=httpx.ErrBody}
// @Failure 403 {object} httpx.Envelope{error=httpx.ErrBody}
// @Router /api/v1/students/{id} [put]
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

// Deactivate godoc
// @Summary Deactivate a student profile
// @Description Soft deactivates a student to prevent them from showing up on active rosters.
// @Tags Students
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path string true "Student ID (UUID)"
// @Success 204 "No Content"
// @Failure 400 {object} httpx.Envelope{error=httpx.ErrBody}
// @Failure 401 {object} httpx.Envelope{error=httpx.ErrBody}
// @Failure 403 {object} httpx.Envelope{error=httpx.ErrBody}
// @Router /api/v1/students/{id}/deactivate [patch]
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

// Delete godoc
// @Summary Delete a student record
// @Description Hard-deletes a student from the database.
// @Tags Students
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path string true "Student ID (UUID)"
// @Success 204 "No Content"
// @Failure 400 {object} httpx.Envelope{error=httpx.ErrBody}
// @Failure 401 {object} httpx.Envelope{error=httpx.ErrBody}
// @Failure 403 {object} httpx.Envelope{error=httpx.ErrBody}
// @Router /api/v1/students/{id} [delete]
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
