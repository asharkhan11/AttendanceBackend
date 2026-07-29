package services

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/school/attendance-backend/internal/apperror"
	"github.com/school/attendance-backend/internal/auth"
	"github.com/school/attendance-backend/internal/dto"
	"github.com/school/attendance-backend/internal/httpx"
	"github.com/school/attendance-backend/internal/models"
	"github.com/school/attendance-backend/internal/repository"
)

// SessionWithRoster bundles a session + its current marks per student.
type SessionWithRoster struct {
	Session *models.AttendanceSession `json:"session"`
	Roster  []RosterEntry             `json:"roster"`
}

type RosterEntry struct {
	StudentID  string  `json:"student_id"`
	FullName   string  `json:"full_name"`
	RollNumber string  `json:"roll_number"`
	Status     *string `json:"status"` // nil = unmarked
	Remarks    *string `json:"remarks"`
}

type AttendanceService struct {
	attendanceRepo     *repository.AttendanceRepo
	attendanceBulkRepo *repository.AttendanceBulkRepository
	studentRepo        *repository.StudentRepo
	classRepo          *repository.ClassRepo
	whatsappRepo       *repository.WhatsAppRepo
	whatsappClient     *WhatsAppClient
}

func NewAttendanceService(
	attendanceRepo *repository.AttendanceRepo,
	attendanceBulkRepo *repository.AttendanceBulkRepository,
	studentRepo *repository.StudentRepo,
	classRepo *repository.ClassRepo,
) *AttendanceService {
	return &AttendanceService{
		attendanceRepo:     attendanceRepo,
		attendanceBulkRepo: attendanceBulkRepo,
		studentRepo:        studentRepo,
		classRepo:          classRepo,
	}
}

func (s *AttendanceService) SetWhatsApp(whatsappRepo *repository.WhatsAppRepo, whatsappClient *WhatsAppClient) {
	s.whatsappRepo = whatsappRepo
	s.whatsappClient = whatsappClient
}

// CreateSession is idempotent — returns existing session if one already exists for the tuple.
func (s *AttendanceService) CreateSession(ctx context.Context, req *dto.CreateSessionRequest) (*models.AttendanceSession, bool, error) {
	claims := auth.ClaimsFromContext(ctx)
	userID, _ := uuid.Parse(claims.Subject)

	classID, err := uuid.Parse(req.ClassID)
	if err != nil {
		return nil, false, apperror.Validation("invalid class_id")
	}

	if claims.Role == models.RoleTeacher {
		myClass, err := s.classRepo.GetByTeacherID(ctx, claims.SchoolID, userID)
		if err != nil || myClass.Class.ID != classID {
			return nil, false, apperror.Forbidden("teachers can only mark attendance for their assigned class")
		}
	}

	date, err := time.Parse("2006-01-02", req.SessionDate)
	if err != nil {
		return nil, false, apperror.Validation("session_date must be YYYY-MM-DD")
	}
	period := int16(1)
	if req.PeriodNumber != nil {
		period = *req.PeriodNumber
	}

	session, created, err := s.attendanceRepo.GetOrCreateSession(ctx, claims.SchoolID, classID, userID, date, period)
	if err != nil {
		return nil, false, apperror.Internal(fmt.Errorf("get or create session: %w", err))
	}
	return session, created, nil
}

func (s *AttendanceService) GetSession(ctx context.Context, sessionID uuid.UUID) (*SessionWithRoster, error) {
	claims := auth.ClaimsFromContext(ctx)
	userID, _ := uuid.Parse(claims.Subject)

	session, err := s.attendanceRepo.GetSessionByID(ctx, sessionID)
	if err != nil {
		return nil, apperror.NotFound("session")
	}

	if claims.Role == models.RoleTeacher {
		myClass, err := s.classRepo.GetByTeacherID(ctx, claims.SchoolID, userID)
		if err != nil || myClass.Class.ID != session.ClassID {
			return nil, apperror.Forbidden("teachers can only view attendance for their assigned class")
		}
	}

	// Get student roster
	students, _, err := s.studentRepo.ListByClass(ctx, session.ClassID, true, 1000, 0)
	if err != nil {
		return nil, apperror.Internal(err)
	}

	// Get existing records
	records, err := s.attendanceRepo.ListRecordsBySession(ctx, sessionID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	recMap := make(map[uuid.UUID]*models.AttendanceRecord)
	for _, rec := range records {
		recMap[rec.StudentID] = rec
	}

	// Build roster entries
	roster := make([]RosterEntry, 0, len(students))
	for _, st := range students {
		entry := RosterEntry{
			StudentID:  st.ID.String(),
			FullName:   st.FullName,
			RollNumber: st.RollNumber,
		}
		if rec, ok := recMap[st.ID]; ok {
			s := string(rec.Status)
			entry.Status = &s
			entry.Remarks = rec.Remarks
		}
		roster = append(roster, entry)
	}

	return &SessionWithRoster{Session: session, Roster: roster}, nil
}

func (s *AttendanceService) ListSessions(ctx context.Context, classID *uuid.UUID, from, to *time.Time, pg httpx.Pagination) ([]*models.AttendanceSession, int64, error) {
	claims := auth.ClaimsFromContext(ctx)
	return s.attendanceRepo.ListSessions(ctx, claims.SchoolID, classID, from, to, pg.PageSize, pg.Offset)
}

// BulkMark marks attendance for multiple students in one transaction-safe upsert.
func (s *AttendanceService) BulkMark(ctx context.Context, sessionID uuid.UUID, req *dto.BulkMarkRequest) error {
	claims := auth.ClaimsFromContext(ctx)
	userID, _ := uuid.Parse(claims.Subject)

	session, err := s.attendanceRepo.GetSessionByID(ctx, sessionID)
	if err != nil {
		return apperror.NotFound("session")
	}

	if claims.Role == models.RoleTeacher {
		myClass, err := s.classRepo.GetByTeacherID(ctx, claims.SchoolID, userID)
		if err != nil || myClass.Class.ID != session.ClassID {
			return apperror.Forbidden("teachers can only mark attendance for their assigned class")
		}
	}

	// Business rule 6: closed sessions reject marking
	if session.Status == models.SessionClosed {
		return apperror.Conflict("session is closed; admin must reopen before marking")
	}

	// Business rule 9 (concurrency): validate all student IDs belong to this class before writing any
	ids := make([]uuid.UUID, 0, len(req.Marks))
	for _, m := range req.Marks {
		id, err := uuid.Parse(m.StudentID)
		if err != nil {
			return apperror.Validation(fmt.Sprintf("invalid student_id: %s", m.StudentID))
		}
		ids = append(ids, id)
	}
	found, err := s.studentRepo.GetAllByClassAndIDs(ctx, session.ClassID, ids)
	if err != nil {
		return apperror.Internal(err)
	}
	var invalidIDs []string
	for _, id := range ids {
		if !found[id] {
			invalidIDs = append(invalidIDs, id.String())
		}
	}
	if len(invalidIDs) > 0 {
		return apperror.Validation(fmt.Sprintf("student IDs not in this class: %v", invalidIDs))
	}

	// All valid — upsert records in bulk using pgx.Batch
	marks := make([]repository.AttendanceMark, len(req.Marks))
	for i, m := range req.Marks {
		sid := ids[i]
		marks[i] = repository.AttendanceMark{
			StudentID: sid,
			Status:    models.AttendanceStatus(m.Status),
			Remarks:   m.Remarks,
		}
	}
	if err := s.attendanceBulkRepo.BulkUpsertRecords(ctx, claims.SchoolID, sessionID, marks, userID); err != nil {
		return apperror.Internal(fmt.Errorf("bulk upsert records: %w", err))
	}

	s.ProcessWhatsAppNotificationsForSession(ctx, sessionID, claims.SchoolID)
	return nil
}

// SingleMark marks or updates a single student's attendance record.
func (s *AttendanceService) SingleMark(ctx context.Context, sessionID, studentID uuid.UUID, req *dto.SingleMarkRequest) error {
	claims := auth.ClaimsFromContext(ctx)
	userID, _ := uuid.Parse(claims.Subject)

	session, err := s.attendanceRepo.GetSessionByID(ctx, sessionID)
	if err != nil {
		return apperror.NotFound("session")
	}

	if claims.Role == models.RoleTeacher {
		myClass, err := s.classRepo.GetByTeacherID(ctx, claims.SchoolID, userID)
		if err != nil || myClass.Class.ID != session.ClassID {
			return apperror.Forbidden("teachers can only mark attendance for their assigned class")
		}
	}

	if session.Status == models.SessionClosed {
		return apperror.Conflict("session is closed")
	}

	status := models.AttendanceStatus(req.Status)
	if err := s.attendanceRepo.UpdateRecordWithAudit(ctx, claims.SchoolID, sessionID, studentID, status, req.Remarks, userID); err != nil {
		return apperror.Internal(err)
	}
	return nil
}

func (s *AttendanceService) CloseSession(ctx context.Context, sessionID uuid.UUID) error {
	session, err := s.attendanceRepo.GetSessionByID(ctx, sessionID)
	if err != nil {
		return apperror.NotFound("session")
	}

	claims := auth.ClaimsFromContext(ctx)
	if claims.Role == models.RoleTeacher {
		userID, _ := uuid.Parse(claims.Subject)
		myClass, err := s.classRepo.GetByTeacherID(ctx, claims.SchoolID, userID)
		if err != nil || myClass.Class.ID != session.ClassID {
			return apperror.Forbidden("teachers can only close sessions for their assigned class")
		}
	}

	if err := s.attendanceRepo.CloseSession(ctx, sessionID); err != nil {
		return err
	}
	s.ProcessWhatsAppNotificationsForSession(ctx, sessionID, claims.SchoolID)
	return nil
}

func (s *AttendanceService) ReopenSession(ctx context.Context, sessionID uuid.UUID) error {
	session, err := s.attendanceRepo.GetSessionByID(ctx, sessionID)
	if err != nil {
		return apperror.NotFound("session")
	}

	claims := auth.ClaimsFromContext(ctx)
	if claims.Role == models.RoleTeacher {
		userID, _ := uuid.Parse(claims.Subject)
		myClass, err := s.classRepo.GetByTeacherID(ctx, claims.SchoolID, userID)
		if err != nil || myClass.Class.ID != session.ClassID {
			return apperror.Forbidden("teachers can only reopen sessions for their assigned class")
		}
	}

	return s.attendanceRepo.ReopenSession(ctx, sessionID)
}

func (s *AttendanceService) ProcessWhatsAppNotificationsForSession(ctx context.Context, sessionID, schoolID uuid.UUID) {
	log.Printf("[WhatsApp debug] ProcessWhatsAppNotificationsForSession called. sessionID: %s, schoolID: %s, repo: %v, client: %v", sessionID, schoolID, s.whatsappRepo != nil, s.whatsappClient != nil)
	if s.whatsappRepo == nil || s.whatsappClient == nil {
		return
	}

	_, err := s.whatsappRepo.FinalizeSession(ctx, sessionID)
	if err != nil {
		log.Printf("error finalizing session %s for whatsapp: %v", sessionID, err)
		return
	}

	// Trigger notifications asynchronously
	go func(sid, schID uuid.UUID) {
		time.Sleep(500 * time.Millisecond)
		bgCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		txCtx, tx, err := s.whatsappRepo.Db().WithTenantTx(bgCtx, schID.String())
		if err != nil {
			log.Printf("error starting tenant tx for whatsapp notifications: %v", err)
			return
		}
		defer func() { _ = tx.Rollback(txCtx) }()

		log.Printf("[WhatsApp debug] Goroutine started. Querying absent students for session: %s", sid)
		absentStudents, err := s.whatsappRepo.GetAbsentStudentsForSession(txCtx, sid)
		log.Printf("[WhatsApp debug] GetAbsentStudentsForSession returned count: %d, err: %v", len(absentStudents), err)
		if err != nil {
			log.Printf("error fetching absent students for session %s: %v", sid, err)
			return
		}
		if len(absentStudents) == 0 {
			return
		}

		schoolID := absentStudents[0].SchoolID
		enabled, monthlyLimit, err := s.whatsappRepo.GetSchoolWhatsAppConfig(txCtx, schoolID)
		if err != nil {
			log.Printf("error fetching school whatsapp config for %s: %v", schoolID, err)
			return
		}

		now := time.Now()
		sentCount, err := s.whatsappRepo.GetMonthlySentCount(txCtx, schoolID, now.Year(), now.Month())
		if err != nil {
			log.Printf("error fetching monthly sent count for %s: %v", schoolID, err)
			return
		}

		for _, student := range absentStudents {
			logEntry := &models.WhatsAppLog{
				SchoolID:       student.SchoolID,
				SessionID:      &student.SessionID,
				StudentID:      &student.StudentID,
				RecipientPhone: student.GuardianPhone,
			}

			if !enabled {
				logEntry.Status = "skipped_disabled"
				errMsg := "WhatsApp messaging disabled for school"
				logEntry.ErrorMessage = &errMsg
				_ = s.whatsappRepo.LogWhatsAppMessage(txCtx, logEntry)
				continue
			}

			if student.GuardianPhone == "" {
				logEntry.Status = "missing_phone"
				errMsg := "Student guardian phone number is missing"
				logEntry.ErrorMessage = &errMsg
				_ = s.whatsappRepo.LogWhatsAppMessage(txCtx, logEntry)
				continue
			}

			if sentCount >= monthlyLimit {
				logEntry.Status = "skipped_rate_limit"
				errMsg := fmt.Sprintf("Monthly rate limit of %d messages reached (sent: %d)", monthlyLimit, sentCount)
				logEntry.ErrorMessage = &errMsg
				_ = s.whatsappRepo.LogWhatsAppMessage(txCtx, logEntry)
				continue
			}

			dateStr := student.SessionDate.Format("02 Jan 2006")
			msgID, sendErr := s.whatsappClient.SendAbsentNotification(txCtx, student.GuardianPhone, student.StudentName, dateStr, student.SchoolName)
			if sendErr != nil {
				logEntry.Status = "failed"
				errMsg := sendErr.Error()
				logEntry.ErrorMessage = &errMsg
			} else {
				logEntry.Status = "sent"
				logEntry.MetaMessageID = &msgID
				sentCount++
			}

			_ = s.whatsappRepo.LogWhatsAppMessage(txCtx, logEntry)
		}

		if err := tx.Commit(txCtx); err != nil {
			log.Printf("error committing transaction for whatsapp notifications: %v", err)
		}
	}(sessionID, schoolID)
}
