package repository

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/school/attendance-backend/internal/models"
)

type WhatsAppRepo struct{ db *DB }

func NewWhatsAppRepo(db *DB) *WhatsAppRepo { return &WhatsAppRepo{db: db} }

func (r *WhatsAppRepo) Db() *DB { return r.db }

type AbsentStudentNotificationInfo struct {
	SchoolID      uuid.UUID
	SessionID     uuid.UUID
	StudentID     uuid.UUID
	StudentName   string
	GuardianPhone string
	SessionDate   time.Time
	SchoolName    string
}

func (r *WhatsAppRepo) FinalizeSession(ctx context.Context, sessionID uuid.UUID) (bool, error) {
	tx := r.db.Querier(ctx)
	if tx != nil {
		tag, err := tx.Exec(ctx,
			`UPDATE attendance_sessions 
			 SET is_finalized = true, finalized_at = now(), updated_at = now() 
			 WHERE id = $1 AND is_finalized = false`, sessionID)
		if err != nil {
			return false, fmt.Errorf("finalize session: %w", err)
		}
		return tag.RowsAffected() > 0, nil
	}

	tag, err := r.db.Pool().Exec(ctx,
		`UPDATE attendance_sessions 
		 SET is_finalized = true, finalized_at = now(), updated_at = now() 
		 WHERE id = $1 AND is_finalized = false`, sessionID)
	if err != nil {
		return false, fmt.Errorf("finalize session: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

func (r *WhatsAppRepo) GetAbsentStudentsForSession(ctx context.Context, sessionID uuid.UUID) ([]AbsentStudentNotificationInfo, error) {
	query := `SELECT s.school_id, s.id AS session_id, st.id AS student_id, st.full_name, COALESCE(st.guardian_phone, ''), s.session_date, sch.name AS school_name
		 FROM attendance_records r
		 JOIN attendance_sessions s ON r.session_id = s.id
		 JOIN students st ON r.student_id = st.id
		 JOIN schools sch ON s.school_id = sch.id
		 WHERE r.session_id = $1 AND r.status = 'absent'`

	tx := r.db.Querier(ctx)
	log.Printf("[WhatsApp debug] GetAbsentStudentsForSession: tx is nil? %v", tx == nil)
	if tx != nil {
		var dbSchoolID string
		errScan := tx.QueryRow(ctx, "SELECT current_setting('app.current_school_id', true)").Scan(&dbSchoolID)
		log.Printf("[WhatsApp debug] current_setting('app.current_school_id') in tx: %q, scan err: %v", dbSchoolID, errScan)
	}
	var rows pgxRows
	var err error
	if tx != nil {
		rows, err = tx.Query(ctx, query, sessionID)
	} else {
		rows, err = r.db.Pool().Query(ctx, query, sessionID)
	}
	if err != nil {
		return nil, fmt.Errorf("query absent students for session: %w", err)
	}
	defer rows.Close()

	var result []AbsentStudentNotificationInfo
	for rows.Next() {
		var info AbsentStudentNotificationInfo
		if err := rows.Scan(&info.SchoolID, &info.SessionID, &info.StudentID, &info.StudentName, &info.GuardianPhone, &info.SessionDate, &info.SchoolName); err != nil {
			return nil, fmt.Errorf("scan absent student: %w", err)
		}
		result = append(result, info)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows err: %w", err)
	}
	return result, nil
}

type pgxRows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
	Close()
}

func (r *WhatsAppRepo) GetSchoolWhatsAppConfig(ctx context.Context, schoolID uuid.UUID) (enabled bool, limit int, err error) {
	query := `SELECT is_whatsapp_enabled, whatsapp_monthly_limit FROM schools WHERE id = $1`
	tx := r.db.Querier(ctx)
	if tx != nil {
		err = tx.QueryRow(ctx, query, schoolID).Scan(&enabled, &limit)
	} else {
		err = r.db.Pool().QueryRow(ctx, query, schoolID).Scan(&enabled, &limit)
	}
	if err != nil {
		return false, 0, fmt.Errorf("get school whatsapp config: %w", err)
	}
	return enabled, limit, nil
}

func (r *WhatsAppRepo) GetMonthlySentCount(ctx context.Context, schoolID uuid.UUID, year int, month time.Month) (int, error) {
	startOfMonth := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
	startOfNextMonth := startOfMonth.AddDate(0, 1, 0)

	query := `SELECT COUNT(*) FROM whatsapp_logs 
		 WHERE school_id = $1 AND status = 'sent' AND created_at >= $2 AND created_at < $3`
	var count int
	tx := r.db.Querier(ctx)
	if tx != nil {
		err := tx.QueryRow(ctx, query, schoolID, startOfMonth, startOfNextMonth).Scan(&count)
		if err != nil {
			return 0, fmt.Errorf("count monthly sent whatsapp messages: %w", err)
		}
	} else {
		err := r.db.Pool().QueryRow(ctx, query, schoolID, startOfMonth, startOfNextMonth).Scan(&count)
		if err != nil {
			return 0, fmt.Errorf("count monthly sent whatsapp messages: %w", err)
		}
	}
	return count, nil
}

func (r *WhatsAppRepo) LogWhatsAppMessage(ctx context.Context, l *models.WhatsAppLog) error {
	tx := r.db.Querier(ctx)
	if tx != nil {
		_, err := tx.Exec(ctx,
			`INSERT INTO whatsapp_logs(school_id, session_id, student_id, recipient_phone, status, meta_message_id, error_message, created_at)
			 VALUES($1, $2, $3, $4, $5, $6, $7, now())`,
			l.SchoolID, l.SessionID, l.StudentID, l.RecipientPhone, l.Status, l.MetaMessageID, l.ErrorMessage)
		return err
	}

	tenantCtx, tenantTx, err := r.db.WithTenantTx(ctx, l.SchoolID.String())
	if err != nil {
		return fmt.Errorf("begin tenant tx for log whatsapp message: %w", err)
	}
	defer tenantTx.Rollback(tenantCtx)

	_, err = tenantTx.Exec(tenantCtx,
		`INSERT INTO whatsapp_logs(school_id, session_id, student_id, recipient_phone, status, meta_message_id, error_message, created_at)
		 VALUES($1, $2, $3, $4, $5, $6, $7, now())`,
		l.SchoolID, l.SessionID, l.StudentID, l.RecipientPhone, l.Status, l.MetaMessageID, l.ErrorMessage)
	if err != nil {
		return fmt.Errorf("log whatsapp message: %w", err)
	}
	return tenantTx.Commit(tenantCtx)
}

func (r *WhatsAppRepo) UpdateWhatsAppLogStatusByMetaID(ctx context.Context, metaMsgID string, status string, errMsg *string) error {
	tx := r.db.Querier(ctx)
	if tx != nil {
		_, err := tx.Exec(ctx,
			`UPDATE whatsapp_logs 
			 SET status = $1, error_message = $2 
			 WHERE meta_message_id = $3`,
			status, errMsg, metaMsgID)
		return err
	}

	_, err := r.db.Pool().Exec(ctx,
		`UPDATE whatsapp_logs 
		 SET status = $1, error_message = $2 
		 WHERE meta_message_id = $3`,
		status, errMsg, metaMsgID)
	if err != nil {
		return fmt.Errorf("update whatsapp log status by meta message id: %w", err)
	}
	return nil
}
