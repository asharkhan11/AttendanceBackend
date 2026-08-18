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

// BulkImportCSV reads a CSV reader and imports teacher accounts. Returns count and any row errors.
func (s *TeacherService) BulkImportCSV(ctx context.Context, r io.Reader) (int, []string, error) {
	claims := auth.ClaimsFromContext(ctx)
	reader := csv.NewReader(r)
	records, err := reader.ReadAll()
	if err != nil {
		return 0, nil, apperror.Validation("invalid CSV format: " + err.Error())
	}
	if len(records) < 2 {
		return 0, nil, apperror.Validation("CSV must have a header row and at least one data row")
	}

	headerMap := make(map[string]int)
	for idx, col := range records[0] {
		cleaned := strings.ToLower(strings.TrimSpace(col))
		cleaned = strings.TrimPrefix(cleaned, "\ufeff")
		headerMap[cleaned] = idx
	}

	var errs []string
	importedCount := 0

	for i, row := range records[1:] {
		lineNo := i + 2
		if len(row) == 0 || (len(row) == 1 && strings.TrimSpace(row[0]) == "") {
			continue // skip empty lines
		}

		getCol := func(name string, defaultIdx int) string {
			if idx, ok := headerMap[name]; ok && idx < len(row) {
				return strings.TrimSpace(row[idx])
			}
			if defaultIdx >= 0 && defaultIdx < len(row) {
				return strings.TrimSpace(row[defaultIdx])
			}
			return ""
		}

		fullName := getCol("full_name", 0)
		email := getCol("email", 1)
		phoneStr := getCol("phone", 2)
		password := getCol("password", 3)

		if fullName == "" {
			errs = append(errs, fmt.Sprintf("line %d: full_name is required", lineNo))
			continue
		}
		if email == "" {
			errs = append(errs, fmt.Sprintf("line %d: email is required", lineNo))
			continue
		}
		if password == "" {
			errs = append(errs, fmt.Sprintf("line %d: password is required", lineNo))
			continue
		}

		hash, err := s.passSvc.HashPassword(password)
		if err != nil {
			errs = append(errs, fmt.Sprintf("line %d: failed to hash password", lineNo))
			continue
		}

		var phone *string
		if phoneStr != "" {
			phone = &phoneStr
		}

		_, err = s.userRepo.Create(ctx, claims.SchoolID, models.RoleTeacher, fullName, email, hash, phone)
		if err != nil {
			if isDuplicateError(err) {
				errs = append(errs, fmt.Sprintf("line %d: teacher with email %q already exists", lineNo, email))
				continue
			}
			errs = append(errs, fmt.Sprintf("line %d: %s", lineNo, err.Error()))
			continue
		}
		importedCount++
	}

	return importedCount, errs, nil
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
		ay, err = s.yearRepo.Create(ctx, claims.SchoolID, req.Name, start, end, true)
		return err
	})
	if err != nil {
		if isDuplicateError(err) {
			return nil, apperror.Conflict("academic year name already exists")
		}
		return nil, apperror.Internal(err)
	}
	return ay, nil
}

func (s *AcademicYearService) Update(ctx context.Context, id uuid.UUID, req *dto.UpdateAcademicYearRequest) (*models.AcademicYear, error) {
	claims := auth.ClaimsFromContext(ctx)
	if req.Name == "" {
		return nil, apperror.Validation("name cannot be empty")
	}

	var start *time.Time
	var end *time.Time
	if req.StartDate != nil && *req.StartDate != "" {
		sTime, err := time.Parse("2006-01-02", *req.StartDate)
		if err != nil {
			return nil, apperror.Validation("start_date must be YYYY-MM-DD")
		}
		start = &sTime
	}
	if req.EndDate != nil && *req.EndDate != "" {
		eTime, err := time.Parse("2006-01-02", *req.EndDate)
		if err != nil {
			return nil, apperror.Validation("end_date must be YYYY-MM-DD")
		}
		end = &eTime
	}

	ay, err := s.yearRepo.Update(ctx, claims.SchoolID, id, req.Name, start, end)
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
	err := s.yearRepo.Db().RunInTx(ctx, claims.SchoolID.String(), func(ctx context.Context) error {
		return s.yearRepo.SetCurrent(ctx, claims.SchoolID, id)
	})
	if err != nil {
		return apperror.Validation(err.Error())
	}
	return nil
}

func (s *AcademicYearService) SetArchived(ctx context.Context, id uuid.UUID, archived bool) error {
	claims := auth.ClaimsFromContext(ctx)
	if !archived {
		return apperror.Validation("archived sessions cannot be unarchived (archival is permanent)")
	}
	return s.yearRepo.SetArchived(ctx, claims.SchoolID, id, true)
}

func (s *AcademicYearService) PromoteStudents(ctx context.Context, req *dto.PromoteStudentsRequest) (int64, error) {
	claims := auth.ClaimsFromContext(ctx)
	srcID, err := uuid.Parse(req.SourceClassID)
	if err != nil {
		return 0, apperror.Validation("invalid source_class_id")
	}
	tgtID, err := uuid.Parse(req.TargetClassID)
	if err != nil {
		return 0, apperror.Validation("invalid target_class_id")
	}

	// Verify target class is not in an archived session
	var isArchived bool
	if err := s.yearRepo.Db().Querier(ctx).QueryRow(ctx,
		`SELECT ay.is_archived FROM classes c JOIN academic_years ay ON c.academic_year_id = ay.id WHERE c.id = $1`, tgtID).Scan(&isArchived); err == nil && isArchived {
		return 0, apperror.Validation("cannot move students into an archived academic session")
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
	var promoted int64
	err = s.yearRepo.Db().RunInTx(ctx, claims.SchoolID.String(), func(ctx context.Context) error {
		var err error
		promoted, err = s.yearRepo.PromoteStudents(ctx, srcID, tgtID, studentUUIDs)
		return err
	})
	if err != nil {
		return 0, apperror.Internal(err)
	}
	return promoted, nil
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
	// Verify target is not archived
	target, err := s.yearRepo.GetByID(ctx, claims.SchoolID, tgtYearID)
	if err != nil {
		return 0, apperror.NotFound("target academic year not found")
	}
	if target.IsArchived {
		return 0, apperror.Validation("cannot copy into an archived academic session")
	}

	var count int
	err = s.yearRepo.Db().RunInTx(ctx, claims.SchoolID.String(), func(ctx context.Context) error {
		var err error
		count, err = s.yearRepo.CopyClasses(ctx, claims.SchoolID, srcYearID, tgtYearID)
		return err
	})
	if err != nil {
		return 0, apperror.Internal(err)
	}
	return count, nil
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
	target, err := s.yearRepo.GetByID(ctx, claims.SchoolID, tgtYearID)
	if err != nil {
		return 0, apperror.NotFound("target academic year not found")
	}
	if target.IsArchived {
		return 0, apperror.Validation("cannot copy into an archived academic session")
	}

	var count int
	err = s.yearRepo.Db().RunInTx(ctx, claims.SchoolID.String(), func(ctx context.Context) error {
		var err error
		count, err = s.yearRepo.CopyTeachers(ctx, claims.SchoolID, srcYearID, tgtYearID, req.ClassMappings)
		return err
	})
	if err != nil {
		return 0, apperror.Internal(err)
	}
	return count, nil
}

func (s *AcademicYearService) CopyStudents(ctx context.Context, req *dto.CopyStudentsRequest) (int, error) {
	claims := auth.ClaimsFromContext(ctx)
	srcYearID, err := uuid.Parse(req.SourceAcademicYearID)
	if err != nil {
		return 0, apperror.Validation("invalid source_academic_year_id")
	}
	tgtYearID, err := uuid.Parse(req.TargetAcademicYearID)
	if err != nil {
		return 0, apperror.Validation("invalid target_academic_year_id")
	}
	target, err := s.yearRepo.GetByID(ctx, claims.SchoolID, tgtYearID)
	if err != nil {
		return 0, apperror.NotFound("target academic year not found")
	}
	if target.IsArchived {
		return 0, apperror.Validation("cannot copy into an archived academic session")
	}

	var count int
	err = s.yearRepo.Db().RunInTx(ctx, claims.SchoolID.String(), func(ctx context.Context) error {
		var err error
		count, err = s.yearRepo.CopyStudents(ctx, claims.SchoolID, srcYearID, tgtYearID)
		return err
	})
	if err != nil {
		return 0, apperror.Internal(err)
	}
	return count, nil
}

func (s *AcademicYearService) CopyAll(ctx context.Context, req *dto.CopyAllRequest) (*repository.CopyAllResult, error) {
	claims := auth.ClaimsFromContext(ctx)
	srcYearID, err := uuid.Parse(req.SourceAcademicYearID)
	if err != nil {
		return nil, apperror.Validation("invalid source_academic_year_id")
	}
	tgtYearID, err := uuid.Parse(req.TargetAcademicYearID)
	if err != nil {
		return nil, apperror.Validation("invalid target_academic_year_id")
	}
	target, err := s.yearRepo.GetByID(ctx, claims.SchoolID, tgtYearID)
	if err != nil {
		return nil, apperror.NotFound("target academic year not found")
	}
	if target.IsArchived {
		return nil, apperror.Validation("cannot copy into an archived academic session")
	}

	var res *repository.CopyAllResult
	err = s.yearRepo.Db().RunInTx(ctx, claims.SchoolID.String(), func(ctx context.Context) error {
		var err error
		res, err = s.yearRepo.CopyAll(ctx, claims.SchoolID, srcYearID, tgtYearID)
		return err
	})
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return res, nil
}

func (s *AcademicYearService) MoveAllStudentsHigher(ctx context.Context, yearID uuid.UUID) (*repository.MoveHigherResult, error) {
	claims := auth.ClaimsFromContext(ctx)
	target, err := s.yearRepo.GetByID(ctx, claims.SchoolID, yearID)
	if err != nil {
		return nil, apperror.NotFound("academic year not found")
	}
	if target.IsArchived {
		return nil, apperror.Validation("cannot move students in an archived academic session")
	}

	var res *repository.MoveHigherResult
	err = s.yearRepo.Db().RunInTx(ctx, claims.SchoolID.String(), func(ctx context.Context) error {
		var err error
		res, err = s.yearRepo.MoveAllStudentsHigher(ctx, claims.SchoolID, yearID)
		return err
	})
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return res, nil
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
func (s *StudentService) BulkImportCSV(ctx context.Context, r io.Reader, classIDParam string) (int, []string, error) {
	claims := auth.ClaimsFromContext(ctx)
	reader := csv.NewReader(r)
	records, err := reader.ReadAll()
	if err != nil {
		return 0, nil, apperror.Validation("invalid CSV format: " + err.Error())
	}
	if len(records) < 2 {
		return 0, nil, apperror.Validation("CSV must have a header row and at least one data row")
	}

	var defaultClassID *uuid.UUID
	if classIDParam != "" {
		id, err := uuid.Parse(classIDParam)
		if err == nil {
			defaultClassID = &id
		}
	}

	// Map header names to column indices
	headerMap := make(map[string]int)
	for idx, col := range records[0] {
		cleaned := strings.ToLower(strings.TrimSpace(col))
		cleaned = strings.TrimPrefix(cleaned, "\ufeff")
		headerMap[cleaned] = idx
	}

	var errs []string
	var students []*models.Student

	for i, row := range records[1:] {
		lineNo := i + 2
		if len(row) == 0 || (len(row) == 1 && strings.TrimSpace(row[0]) == "") {
			continue // skip empty lines
		}

		getCol := func(name string, defaultIdx int) string {
			if idx, ok := headerMap[name]; ok && idx < len(row) {
				return strings.TrimSpace(row[idx])
			}
			if defaultIdx >= 0 && defaultIdx < len(row) {
				return strings.TrimSpace(row[defaultIdx])
			}
			return ""
		}

		// Determine Class ID
		var studentClassID uuid.UUID
		classIDVal := getCol("class_id", -1)
		if classIDVal != "" {
			parsedID, err := uuid.Parse(classIDVal)
			if err != nil {
				errs = append(errs, fmt.Sprintf("line %d: invalid class_id %q", lineNo, classIDVal))
				continue
			}
			studentClassID = parsedID
		} else if defaultClassID != nil {
			studentClassID = *defaultClassID
		} else {
			errs = append(errs, fmt.Sprintf("line %d: class_id is missing or invalid", lineNo))
			continue
		}

		// Positional fallbacks match template order:
		// 0: class_id, 1: admission_number, 2: roll_number, 3: full_name, 4: gender, 5: dob, 6: guardian_name, 7: guardian_phone
		admNum := getCol("admission_number", 1)
		rollNum := getCol("roll_number", 2)
		fullName := getCol("full_name", 3)
		genderStr := getCol("gender", 4)
		dobStr := getCol("dob", 5)
		guardianNameStr := getCol("guardian_name", 6)
		guardianPhoneStr := getCol("guardian_phone", 7)

		if fullName == "" {
			errs = append(errs, fmt.Sprintf("line %d: full_name is required", lineNo))
			continue
		}
		if admNum == "" {
			errs = append(errs, fmt.Sprintf("line %d: admission_number is required", lineNo))
			continue
		}

		st := &models.Student{
			SchoolID:        claims.SchoolID,
			ClassID:         studentClassID,
			AdmissionNumber: admNum,
			RollNumber:      rollNum,
			FullName:        fullName,
		}

		if genderStr != "" {
			g := genderStr
			st.Gender = &g
		}
		if guardianNameStr != "" {
			gn := guardianNameStr
			st.GuardianName = &gn
		}
		if guardianPhoneStr != "" {
			gp := guardianPhoneStr
			st.GuardianPhone = &gp
		}
		if dobStr != "" {
			t, err := time.Parse("2006-01-02", dobStr)
			if err == nil {
				st.DOB = &t
			}
		}

		students = append(students, st)
	}

	if len(students) == 0 {
		if len(errs) > 0 {
			return 0, errs, nil
		}
		return 0, nil, apperror.Validation("no valid student records found in CSV")
	}

	err = s.studentRepo.Db().RunInTx(ctx, claims.SchoolID.String(), func(ctx context.Context) error {
		return s.studentBulkRepo.BulkCreate(ctx, students)
	})
	if err != nil {
		return 0, nil, apperror.Internal(err)
	}

	return len(students), errs, nil
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
