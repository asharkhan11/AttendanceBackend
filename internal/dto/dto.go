package dto

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/school/attendance-backend/internal/apperror"
)

var validate = validator.New()

// Decode decodes JSON body into v and validates it.
func Decode(r *http.Request, v any) error {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		return apperror.Validation(fmt.Sprintf("invalid JSON: %s", err.Error()))
	}
	if err := validate.Struct(v); err != nil {
		var details []apperror.FieldError
		for _, fe := range err.(validator.ValidationErrors) {
			details = append(details, apperror.FieldError{
				Field: fe.Field(),
				Issue: fe.Tag(),
			})
		}
		return apperror.Validation("validation failed", details...)
	}
	return nil
}

// ─── Auth ─────────────────────────────────────────────────────────────────────

type LoginRequest struct {
	SchoolCode string `json:"school_code" validate:"required"`
	Email      string `json:"email"       validate:"required,email"`
	Password   string `json:"password"    validate:"required,min=6"`
}

type LoginResponse struct {
	AccessToken  string   `json:"access_token"`
	RefreshToken string   `json:"refresh_token"`
	ExpiresIn    int      `json:"expires_in_seconds"`
	User         UserResp `json:"user"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

type LogoutRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

type UpdateProfileRequest struct {
	FullName string  `json:"full_name" validate:"required,min=2"`
	Phone    *string `json:"phone"`
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" validate:"required"`
	NewPassword     string `json:"new_password"     validate:"required,min=8"`
}

// ─── Users ────────────────────────────────────────────────────────────────────

type UserResp struct {
	ID       string `json:"id"`
	SchoolID string `json:"school_id"`
	Role     string `json:"role"`
	FullName string `json:"full_name"`
	Email    string `json:"email"`
	Phone    *string `json:"phone,omitempty"`
	IsActive bool   `json:"is_active"`
}

// ─── Academic Years ───────────────────────────────────────────────────────────

type CreateAcademicYearRequest struct {
	Name      string `json:"name"       validate:"required,max=50"`
	StartDate string `json:"start_date" validate:"required"`
	EndDate   string `json:"end_date"   validate:"required"`
	IsCurrent bool   `json:"is_current"`
}

type UpdateAcademicYearRequest struct {
	Name      string  `json:"name"       validate:"required,max=50"`
	StartDate *string `json:"start_date,omitempty"`
	EndDate   *string `json:"end_date,omitempty"`
}

type PromoteStudentsRequest struct {
	SourceClassID string   `json:"source_class_id" validate:"required,uuid"`
	TargetClassID string   `json:"target_class_id" validate:"required,uuid"`
	StudentIDs    []string `json:"student_ids"      validate:"required,min=1"`
}

type CopyClassesRequest struct {
	SourceAcademicYearID string `json:"source_academic_year_id" validate:"required,uuid"`
	TargetAcademicYearID string `json:"target_academic_year_id" validate:"required,uuid"`
}

type CopyTeachersRequest struct {
	SourceAcademicYearID string            `json:"source_academic_year_id" validate:"required,uuid"`
	TargetAcademicYearID string            `json:"target_academic_year_id" validate:"required,uuid"`
	ClassMappings        map[string]string `json:"class_mappings,omitempty"` // map source class_id -> target class_id
}

type CopyStudentsRequest struct {
	SourceAcademicYearID string `json:"source_academic_year_id" validate:"required,uuid"`
	TargetAcademicYearID string `json:"target_academic_year_id" validate:"required,uuid"`
}

type CopyAllRequest struct {
	SourceAcademicYearID string `json:"source_academic_year_id" validate:"required,uuid"`
	TargetAcademicYearID string `json:"target_academic_year_id" validate:"required,uuid"`
}

// ─── Classes ──────────────────────────────────────────────────────────────────

type CreateClassRequest struct {
	AcademicYearID string  `json:"academic_year_id" validate:"omitempty,uuid"`
	GradeLevel     int16   `json:"grade_level"      validate:"required,min=1,max=12"`
	Section        string  `json:"section"          validate:"required,max=10"`
	ClassTeacherID *string `json:"class_teacher_id" validate:"omitempty,uuid"`
}

type UpdateClassRequest struct {
	GradeLevel int16  `json:"grade_level" validate:"required,min=1,max=12"`
	Section    string `json:"section"     validate:"required,max=10"`
}

type AssignClassTeacherRequest struct {
	ClassTeacherID *string `json:"class_teacher_id" validate:"omitempty,uuid"`
}

type ClassResp struct {
	ID                string  `json:"id"`
	SchoolID          string  `json:"school_id"`
	AcademicYearID    string  `json:"academic_year_id"`
	GradeLevel        int16   `json:"grade_level"`
	Section           string  `json:"section"`
	ClassTeacherID    *string `json:"class_teacher_id,omitempty"`
	ClassTeacherName  *string `json:"class_teacher_name,omitempty"`
	ClassTeacherEmail *string `json:"class_teacher_email,omitempty"`
	CreatedAt         string  `json:"created_at"`
}

// ─── Teachers ─────────────────────────────────────────────────────────────────

type CreateTeacherRequest struct {
	FullName string  `json:"full_name" validate:"required,max=255"`
	Email    string  `json:"email"     validate:"required,email"`
	Phone    *string `json:"phone"`
	Password string  `json:"password"  validate:"required,min=8"`
}

type UpdateTeacherRequest struct {
	FullName string  `json:"full_name" validate:"required,max=255"`
	Phone    *string `json:"phone"`
}

type ResetPasswordRequest struct {
	NewPassword string `json:"new_password" validate:"required,min=8"`
}

// ─── Students ─────────────────────────────────────────────────────────────────

type CreateStudentRequest struct {
	ClassID         string  `json:"class_id"          validate:"required,uuid"`
	AdmissionNumber string  `json:"admission_number"  validate:"max=50"`
	RollNumber      string  `json:"roll_number"       validate:"required,max=20"`
	FullName        string  `json:"full_name"         validate:"required,max=255"`
	Gender          *string `json:"gender"`
	DOB             *string `json:"dob"`
	GuardianName    *string `json:"guardian_name"`
	GuardianPhone   *string `json:"guardian_phone"`
}

type UpdateStudentRequest struct {
	ClassID         string  `json:"class_id"          validate:"required,uuid"`
	AdmissionNumber string  `json:"admission_number"  validate:"required,max=50"`
	RollNumber      string  `json:"roll_number"       validate:"required,max=20"`
	FullName        string  `json:"full_name"         validate:"required,max=255"`
	Gender          *string `json:"gender"`
	DOB             *string `json:"dob"`
	GuardianName    *string `json:"guardian_name"`
	GuardianPhone   *string `json:"guardian_phone"`
}

// ─── Attendance ───────────────────────────────────────────────────────────────

type CreateSessionRequest struct {
	ClassID      string `json:"class_id"     validate:"required,uuid"`
	SessionDate  string `json:"session_date" validate:"required"`
	PeriodNumber *int16 `json:"period_number"`
}

type MarkEntry struct {
	StudentID string  `json:"student_id" validate:"required,uuid"`
	Status    string  `json:"status"     validate:"required,oneof=present absent late excused"`
	Remarks   *string `json:"remarks"`
}

type BulkMarkRequest struct {
	Marks []MarkEntry `json:"marks" validate:"required,min=1,dive"`
}

type SingleMarkRequest struct {
	Status  string  `json:"status"  validate:"required,oneof=present absent late excused"`
	Remarks *string `json:"remarks"`
}

// ─── Analytics ────────────────────────────────────────────────────────────────

type DateRangeQuery struct {
	From string `json:"from"` // YYYY-MM-DD
	To   string `json:"to"`
}
