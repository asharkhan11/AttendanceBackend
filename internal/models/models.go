package models

import (
	"time"

	"github.com/google/uuid"
)

// ─── Enums ───────────────────────────────────────────────────────────────────

type UserRole string

const (
	RoleAdmin   UserRole = "admin"
	RoleTeacher UserRole = "teacher"
)

type AttendanceStatus string

const (
	StatusPresent AttendanceStatus = "present"
	StatusAbsent  AttendanceStatus = "absent"
	StatusLate    AttendanceStatus = "late"
	StatusExcused AttendanceStatus = "excused"
)

type SessionStatus string

const (
	SessionOpen   SessionStatus = "open"
	SessionClosed SessionStatus = "closed"
)

// ─── Domain Models ────────────────────────────────────────────────────────────

type School struct {
	ID                   uuid.UUID `json:"id"`
	Name                 string    `json:"name"`
	Code                 string    `json:"code"`
	IsActive             bool      `json:"is_active"`
	IsWhatsAppEnabled    bool      `json:"is_whatsapp_enabled"`
	WhatsAppMonthlyLimit int       `json:"whatsapp_monthly_limit"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

type AcademicYear struct {
	ID         uuid.UUID `json:"id"`
	SchoolID   uuid.UUID `json:"school_id"`
	Name       string    `json:"name"`
	StartDate  time.Time `json:"start_date"`
	EndDate    time.Time `json:"end_date"`
	IsCurrent  bool      `json:"is_current"`
	IsArchived bool      `json:"is_archived"`
	CreatedAt  time.Time `json:"created_at"`
}

type User struct {
	ID           uuid.UUID `json:"id"`
	SchoolID     uuid.UUID `json:"school_id"`
	Role         UserRole  `json:"role"`
	FullName     string    `json:"full_name"`
	Email        string    `json:"email"`
	Phone        *string   `json:"phone,omitempty"`
	IsActive     bool      `json:"is_active"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type Class struct {
	ID             uuid.UUID  `json:"id"`
	SchoolID       uuid.UUID  `json:"school_id"`
	AcademicYearID uuid.UUID  `json:"academic_year_id"`
	GradeLevel     int16      `json:"grade_level"`
	Section        string     `json:"section"`
	ClassTeacherID *uuid.UUID `json:"class_teacher_id,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

type Student struct {
	ID              uuid.UUID  `json:"id"`
	SchoolID        uuid.UUID  `json:"school_id"`
	AcademicYearID  uuid.UUID  `json:"academic_year_id"`
	ClassID         uuid.UUID  `json:"class_id"`
	AdmissionNumber string     `json:"admission_number"`
	RollNumber      string     `json:"roll_number"`
	FullName        string     `json:"full_name"`
	Gender          *string    `json:"gender,omitempty"`
	DOB             *time.Time `json:"dob,omitempty"`
	GuardianName    *string    `json:"guardian_name,omitempty"`
	GuardianPhone   *string    `json:"guardian_phone,omitempty"`
	IsActive        bool       `json:"is_active"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type AttendanceSession struct {
	ID           uuid.UUID     `json:"id"`
	SchoolID     uuid.UUID     `json:"school_id"`
	ClassID      uuid.UUID     `json:"class_id"`
	TeacherID    uuid.UUID     `json:"teacher_id"`
	SessionDate  time.Time     `json:"session_date"`
	PeriodNumber int16         `json:"period_number"`
	Status       SessionStatus `json:"status"`
	IsFinalized  bool          `json:"is_finalized"`
	FinalizedAt  *time.Time    `json:"finalized_at,omitempty"`
	CreatedAt    time.Time     `json:"created_at"`
	UpdatedAt    time.Time     `json:"updated_at"`
}

type AttendanceRecord struct {
	ID        uuid.UUID        `json:"id"`
	SchoolID  uuid.UUID        `json:"school_id"`
	SessionID uuid.UUID        `json:"session_id"`
	StudentID uuid.UUID        `json:"student_id"`
	Status    AttendanceStatus `json:"status"`
	Remarks   *string          `json:"remarks,omitempty"`
	MarkedBy  uuid.UUID        `json:"marked_by"`
	MarkedAt  time.Time        `json:"marked_at"`
	UpdatedAt time.Time        `json:"updated_at"`
}

type AttendanceAuditLog struct {
	ID                 uuid.UUID        `json:"id"`
	SchoolID           uuid.UUID        `json:"school_id"`
	AttendanceRecordID uuid.UUID        `json:"attendance_record_id"`
	OldStatus          *AttendanceStatus `json:"old_status,omitempty"`
	NewStatus          AttendanceStatus `json:"new_status"`
	ChangedBy          uuid.UUID        `json:"changed_by"`
	ChangedAt          time.Time        `json:"changed_at"`
}

type WhatsAppLog struct {
	ID             uuid.UUID  `json:"id"`
	SchoolID       uuid.UUID  `json:"school_id"`
	SessionID      *uuid.UUID `json:"session_id,omitempty"`
	StudentID      *uuid.UUID `json:"student_id,omitempty"`
	RecipientPhone string     `json:"recipient_phone"`
	Status         string     `json:"status"`
	MetaMessageID  *string    `json:"meta_message_id,omitempty"`
	ErrorMessage   *string    `json:"error_message,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

// ─── Context key ─────────────────────────────────────────────────────────────

type contextKey string

const (
	CtxClaims    contextKey = "claims"
	CtxTx        contextKey = "tx"
	CtxRequestID contextKey = "request_id"
)

