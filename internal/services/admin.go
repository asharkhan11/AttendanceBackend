package services

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/school/attendance-backend/internal/apperror"
	"github.com/school/attendance-backend/internal/auth"
	"github.com/school/attendance-backend/internal/dto"
	"github.com/school/attendance-backend/internal/httpx"
	"github.com/school/attendance-backend/internal/models"
	"github.com/school/attendance-backend/internal/repository"
)

// ─── Class Service ────────────────────────────────────────────────────────────

type ClassService struct {
	classRepo *repository.ClassRepo
	yearRepo  *repository.AcademicYearRepo
}

func NewClassService(cr *repository.ClassRepo, yr *repository.AcademicYearRepo) *ClassService {
	return &ClassService{cr, yr}
}

func (s *ClassService) List(ctx context.Context, academicYearID *uuid.UUID, pg httpx.Pagination) ([]*repository.ClassWithTeacher, int64, error) {
	claims := auth.ClaimsFromContext(ctx)
	return s.classRepo.List(ctx, claims.SchoolID, academicYearID, pg.PageSize, pg.Offset)
}

func (s *ClassService) GetMyClass(ctx context.Context) (*repository.ClassWithTeacher, error) {
	claims := auth.ClaimsFromContext(ctx)
	userID, _ := uuid.Parse(claims.Subject)
	cls, err := s.classRepo.GetByTeacherID(ctx, claims.SchoolID, userID)
	if err != nil {
		return nil, apperror.NotFound("no class assigned to teacher")
	}
	return cls, nil
}

func (s *ClassService) Create(ctx context.Context, req *dto.CreateClassRequest) (*repository.ClassWithTeacher, error) {
	claims := auth.ClaimsFromContext(ctx)
	var yearID uuid.UUID
	if req.AcademicYearID != "" {
		id, err := uuid.Parse(req.AcademicYearID)
		if err != nil {
			return nil, apperror.Validation("invalid academic_year_id")
		}
		yearID = id
	} else {
		currentYear, err := s.yearRepo.GetCurrent(ctx, claims.SchoolID)
		if err != nil {
			return nil, apperror.Validation("no active academic year found for school")
		}
		yearID = currentYear.ID
	}
	var teacherID *uuid.UUID
	if req.ClassTeacherID != nil && *req.ClassTeacherID != "" {
		id, err := uuid.Parse(*req.ClassTeacherID)
		if err != nil {
			return nil, apperror.Validation("invalid class_teacher_id")
		}
		teacherID = &id
	}
	cls, err := s.classRepo.Create(ctx, claims.SchoolID, yearID, req.GradeLevel, req.Section, teacherID)
	if err != nil {
		if isDuplicateError(err) {
			return nil, apperror.Conflict("class or class teacher assignment already exists")
		}
		return nil, apperror.Internal(err)
	}
	return cls, nil
}

func (s *ClassService) AssignTeacher(ctx context.Context, classID uuid.UUID, teacherID *uuid.UUID) (*repository.ClassWithTeacher, error) {
	claims := auth.ClaimsFromContext(ctx)
	cls, err := s.classRepo.AssignTeacher(ctx, claims.SchoolID, classID, teacherID)
	if err != nil {
		if isDuplicateError(err) {
			return nil, apperror.Conflict("this teacher is already assigned to another class")
		}
		return nil, apperror.Internal(err)
	}
	return cls, nil
}

func (s *ClassService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.classRepo.Delete(ctx, id)
}

// ─── Teacher Service ──────────────────────────────────────────────────────────

type TeacherService struct {
	userRepo *repository.UserRepo
	passSvc  *auth.PasswordService
}

func NewTeacherService(userRepo *repository.UserRepo, passSvc *auth.PasswordService) *TeacherService {
	return &TeacherService{userRepo, passSvc}
}

func (s *TeacherService) List(ctx context.Context, pg httpx.Pagination) ([]*models.User, int64, error) {
	claims := auth.ClaimsFromContext(ctx)
	return s.userRepo.ListTeachers(ctx, claims.SchoolID, pg.PageSize, pg.Offset)
}

func (s *TeacherService) Create(ctx context.Context, req *dto.CreateTeacherRequest) (*models.User, error) {
	claims := auth.ClaimsFromContext(ctx)
	hash, err := s.passSvc.HashPassword(req.Password)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	u, err := s.userRepo.Create(ctx, claims.SchoolID, models.RoleTeacher, req.FullName, req.Email, hash, req.Phone)
	if err != nil {
		if isDuplicateError(err) {
			return nil, apperror.Conflict("a teacher with this email already exists")
		}
		return nil, apperror.Internal(err)
	}
	return u, nil
}

func (s *TeacherService) Update(ctx context.Context, id uuid.UUID, req *dto.UpdateTeacherRequest) (*models.User, error) {
	u, err := s.userRepo.Update(ctx, id, req.FullName, req.Phone)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return u, nil
}

func (s *TeacherService) Deactivate(ctx context.Context, id uuid.UUID) error {
	return s.userRepo.Deactivate(ctx, id)
}

func (s *TeacherService) Activate(ctx context.Context, id uuid.UUID) error {
	return s.userRepo.Activate(ctx, id)
}

func (s *TeacherService) Delete(ctx context.Context, id uuid.UUID) error {
	claims := auth.ClaimsFromContext(ctx)
	return s.userRepo.Db().RunInTx(ctx, claims.SchoolID.String(), func(ctx context.Context) error {
		return s.userRepo.Delete(ctx, id)
	})
}

func (s *TeacherService) ResetPassword(ctx context.Context, id uuid.UUID, req *dto.ResetPasswordRequest) error {
	hash, err := s.passSvc.HashPassword(req.NewPassword)
	if err != nil {
		return apperror.Internal(err)
	}
	claims := auth.ClaimsFromContext(ctx)
	err = s.userRepo.Db().RunInTx(ctx, claims.SchoolID.String(), func(ctx context.Context) error {
		if err := s.userRepo.UpdatePasswordHash(ctx, id, hash); err != nil {
			return err
		}
		return s.userRepo.RevokeAllRefreshTokens(ctx, id)
	})
	if err != nil {
		return apperror.Internal(err)
	}
	return nil
}

// ─── Student Service ──────────────────────────────────────────────────────────

type StudentService struct {
	studentRepo     *repository.StudentRepo
	studentBulkRepo *repository.StudentBulkRepository
}

func NewStudentService(repo *repository.StudentRepo, bulkRepo *repository.StudentBulkRepository) *StudentService {
	return &StudentService{studentRepo: repo, studentBulkRepo: bulkRepo}
}

type AcademicYearService struct {
	yearRepo *repository.AcademicYearRepo
}

func NewAcademicYearService(yr *repository.AcademicYearRepo) *AcademicYearService {
	return &AcademicYearService{yr}
}

func (s *AcademicYearService) List(ctx context.Context) ([]*models.AcademicYear, error) {
	claims := auth.ClaimsFromContext(ctx)
	return s.yearRepo.List(ctx, claims.SchoolID)
}

func (s *AcademicYearService) Create(ctx context.Context, req *dto.CreateAcademicYearRequest) (*models.AcademicYear, error) {
	claims := auth.ClaimsFromContext(ctx)
	start, err := time.Parse("2006-01-02", req.StartDate)
	if err != nil {
		return nil, apperror.Validation("start_date must be YYYY-MM-DD")
	}
	end, err := time.Parse("2006-01-02", req.EndDate)
	if err != nil {
		return nil, apperror.Validation("end_date must be YYYY-MM-DD")
	}
	var ay *models.AcademicYear
	err = s.yearRepo.Db().RunInTx(ctx, claims.SchoolID.String(), func(ctx context.Context) error {
		var err error
		ay, err = s.yearRepo.Create(ctx, claims.SchoolID, req.Name, start, end, req.IsCurrent)
		if err != nil {
			return err
		}
		if req.IsCurrent {
			if err := s.yearRepo.SetCurrent(ctx, claims.SchoolID, ay.ID); err != nil {
				return err
			}
			ay.IsCurrent = true
		}
		return nil
	})
	if err != nil {
		if isDuplicateError(err) {
			return nil, apperror.Conflict("academic year name already exists")
		}
		return nil, apperror.Internal(err)
	}
	return ay, nil
}

func (s *AcademicYearService) SetCurrent(ctx context.Context, id uuid.UUID) error {
	claims := auth.ClaimsFromContext(ctx)
	return s.yearRepo.Db().RunInTx(ctx, claims.SchoolID.String(), func(ctx context.Context) error {
		return s.yearRepo.SetCurrent(ctx, claims.SchoolID, id)
	})
}

func (s *AcademicYearService) SetArchived(ctx context.Context, id uuid.UUID, archived bool) error {
	claims := auth.ClaimsFromContext(ctx)
	return s.yearRepo.SetArchived(ctx, claims.SchoolID, id, archived)
}

func (s *AcademicYearService) PromoteStudents(ctx context.Context, req *dto.PromoteStudentsRequest) (int64, error) {
	srcID, err := uuid.Parse(req.SourceClassID)
	if err != nil {
		return 0, apperror.Validation("invalid source_class_id")
	}
	tgtID, err := uuid.Parse(req.TargetClassID)
	if err != nil {
		return 0, apperror.Validation("invalid target_class_id")
	}
	var studentUUIDs []uuid.UUID
	for _, sid := range req.StudentIDs {
		uid, err := uuid.Parse(sid)
		if err == nil {
			studentUUIDs = append(studentUUIDs, uid)
		}
	}
	if len(studentUUIDs) == 0 {
		return 0, apperror.Validation("no valid student_ids provided")
	}
	return s.yearRepo.PromoteStudents(ctx, srcID, tgtID, studentUUIDs)
}

func (s *AcademicYearService) CopyClasses(ctx context.Context, req *dto.CopyClassesRequest) (int, error) {
	claims := auth.ClaimsFromContext(ctx)
	srcYearID, err := uuid.Parse(req.SourceAcademicYearID)
	if err != nil {
		return 0, apperror.Validation("invalid source_academic_year_id")
	}
	tgtYearID, err := uuid.Parse(req.TargetAcademicYearID)
	if err != nil {
		return 0, apperror.Validation("invalid target_academic_year_id")
	}
	return s.yearRepo.CopyClasses(ctx, claims.SchoolID, srcYearID, tgtYearID)
}

func (s *AcademicYearService) CopyTeachers(ctx context.Context, req *dto.CopyTeachersRequest) (int, error) {
	claims := auth.ClaimsFromContext(ctx)
	srcYearID, err := uuid.Parse(req.SourceAcademicYearID)
	if err != nil {
		return 0, apperror.Validation("invalid source_academic_year_id")
	}
	tgtYearID, err := uuid.Parse(req.TargetAcademicYearID)
	if err != nil {
		return 0, apperror.Validation("invalid target_academic_year_id")
	}
	return s.yearRepo.CopyTeachers(ctx, claims.SchoolID, srcYearID, tgtYearID, req.ClassMappings)
}

func (s *StudentService) List(ctx context.Context, academicYearID *uuid.UUID, classID *uuid.UUID, search string, pg httpx.Pagination) ([]*models.Student, int64, error) {
	claims := auth.ClaimsFromContext(ctx)
	return s.studentRepo.ListBySchool(ctx, claims.SchoolID, academicYearID, classID, search, pg.PageSize, pg.Offset)
}

func (s *StudentService) ListByClass(ctx context.Context, classID uuid.UUID, pg httpx.Pagination) ([]*models.Student, int64, error) {
	return s.studentRepo.ListByClass(ctx, classID, true, pg.PageSize, pg.Offset)
}

func (s *StudentService) GetNextStudentInfo(ctx context.Context, classID uuid.UUID) (*repository.NextStudentInfo, error) {
	claims := auth.ClaimsFromContext(ctx)
	return s.studentRepo.GetNextStudentInfo(ctx, claims.SchoolID, classID)
}

func (s *StudentService) Create(ctx context.Context, req *dto.CreateStudentRequest) (*models.Student, error) {
	claims := auth.ClaimsFromContext(ctx)
	classID, err := uuid.Parse(req.ClassID)
	if err != nil {
		return nil, apperror.Validation("invalid class_id")
	}
	var dob *time.Time
	if req.DOB != nil && *req.DOB != "" {
		t, err := time.Parse("2006-01-02", *req.DOB)
		if err != nil {
			return nil, apperror.Validation("dob must be YYYY-MM-DD")
		}
		dob = &t
	}
	admNum := req.AdmissionNumber
	if admNum == "" {
		info, err := s.studentRepo.GetNextStudentInfo(ctx, claims.SchoolID, classID)
		if err == nil && info.NextAdmissionNumber != "" {
			admNum = info.NextAdmissionNumber
		}
	}
	st := &models.Student{
		SchoolID:        claims.SchoolID,
		ClassID:         classID,
		AdmissionNumber: admNum,
		RollNumber:      req.RollNumber,
		FullName:        req.FullName,
		Gender:          req.Gender,
		DOB:             dob,
		GuardianName:    req.GuardianName,
		GuardianPhone:   req.GuardianPhone,
	}
	out, err := s.studentRepo.Create(ctx, st)
	if err != nil {
		if isDuplicateError(err) {
			errStr := err.Error()
			if strings.Contains(errStr, "admission_number") {
				return nil, apperror.Conflict("Admission number already exists in this school")
			}
			if strings.Contains(errStr, "roll_number") {
				return nil, apperror.Conflict("Roll number already exists in this class")
			}
			return nil, apperror.Conflict("A student with this admission number or roll number already exists")
		}
		return nil, apperror.Internal(err)
	}
	return out, nil
}

func (s *StudentService) Update(ctx context.Context, id uuid.UUID, req *dto.UpdateStudentRequest) (*models.Student, error) {
	classID, err := uuid.Parse(req.ClassID)
	if err != nil {
		return nil, apperror.Validation("invalid class_id")
	}
	var dob *time.Time
	if req.DOB != nil && *req.DOB != "" {
		t, err := time.Parse("2006-01-02", *req.DOB)
		if err != nil {
			return nil, apperror.Validation("dob must be YYYY-MM-DD")
		}
		dob = &t
	}
	st, err := s.studentRepo.Update(ctx, id, classID, req.AdmissionNumber, req.RollNumber, req.FullName, req.Gender, dob, req.GuardianName, req.GuardianPhone)
	if err != nil {
		if isDuplicateError(err) {
			errStr := err.Error()
			if strings.Contains(errStr, "admission_number") {
				return nil, apperror.Conflict("Admission number already exists in this school")
			}
			if strings.Contains(errStr, "roll_number") {
				return nil, apperror.Conflict("Roll number already exists in this class")
			}
			return nil, apperror.Conflict("A student with this admission number or roll number already exists")
		}
		return nil, apperror.Internal(err)
	}
	return st, nil
}

func (s *StudentService) Delete(ctx context.Context, id uuid.UUID) error {
	err := s.studentRepo.Delete(ctx, id)
	if err != nil {
		if isNotFound(err) {
			return apperror.NotFound("student")
		}
		return apperror.Internal(err)
	}
	return nil
}

func (s *StudentService) Deactivate(ctx context.Context, id uuid.UUID) error {
	return s.studentRepo.Deactivate(ctx, id)
}

// BulkImportCSV reads a CSV reader and imports students. Returns count and any row errors.
func (s *StudentService) BulkImportCSV(ctx context.Context, r io.Reader) (int, []string, error) {
	claims := auth.ClaimsFromContext(ctx)
	reader := csv.NewReader(r)
	records, err := reader.ReadAll()
	if err != nil {
		return 0, nil, apperror.Validation("invalid CSV: " + err.Error())
	}
	if len(records) < 2 {
		return 0, nil, apperror.Validation("CSV must have a header row and at least one data row")
	}

	var errs []string
	var students []*models.Student

	for i, row := range records[1:] {
		lineNo := i + 2
		if len(row) < 5 {
			errs = append(errs, fmt.Sprintf("line %d: not enough columns", lineNo))
			continue
		}
		classID, err := uuid.Parse(strings.TrimSpace(row[0]))
		if err != nil {
			errs = append(errs, fmt.Sprintf("line %d: invalid class_id", lineNo))
			continue
		}
		st := &models.Student{
			SchoolID:        claims.SchoolID,
			ClassID:         classID,
			AdmissionNumber: strings.TrimSpace(row[1]),
			RollNumber:      strings.TrimSpace(row[2]),
			FullName:        strings.TrimSpace(row[3]),
		}
		if len(row) > 4 && row[4] != "" {
			g := strings.TrimSpace(row[4])
			st.Gender = &g
		}
		students = append(students, st)
	}

	if len(errs) > 0 {
		return 0, errs, nil
	}

	err = s.studentRepo.Db().RunInTx(ctx, claims.SchoolID.String(), func(ctx context.Context) error {
		return s.studentBulkRepo.BulkCreate(ctx, students)
	})
	if err != nil {
		return 0, nil, apperror.Internal(err)
	}

	return len(students), nil, nil
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func isDuplicateError(err error) bool {
	return strings.Contains(err.Error(), "duplicate key") ||
		strings.Contains(err.Error(), "unique constraint") ||
		strings.Contains(err.Error(), "UNIQUE")
}

func isNotFound(err error) bool {
	return strings.Contains(err.Error(), "no rows") ||
		strings.Contains(err.Error(), "not found")
}

// used by TeacherService to extract teacher count for page meta
func intStr(i int) string { return strconv.Itoa(i) }
var _ = intStr // suppress unused warning
