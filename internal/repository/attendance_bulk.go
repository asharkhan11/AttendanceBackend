package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/school/attendance-backend/internal/models"
)

type AttendanceMark struct {
	StudentID uuid.UUID
	Status    models.AttendanceStatus
	Remarks   *string
}

type AttendanceBulkRepository struct {
	db *DB
}

func NewAttendanceBulkRepository(db *DB) *AttendanceBulkRepository {
	return &AttendanceBulkRepository{db: db}
}

func (r *AttendanceBulkRepository) BulkUpsertRecords(ctx context.Context, schoolID, sessionID uuid.UUID, marks []AttendanceMark, markedBy uuid.UUID) error {
	tx := TxFromContext(ctx)
	if tx == nil {
		return fmt.Errorf("no active transaction in context")
	}

	batch := &pgx.Batch{}
	for _, m := range marks {
		batch.Queue(
			`INSERT INTO attendance_records(school_id,session_id,student_id,status,remarks,marked_by)
			 VALUES($1,$2,$3,$4,$5,$6)
			 ON CONFLICT (session_id,student_id) DO UPDATE
			 SET status=EXCLUDED.status, remarks=EXCLUDED.remarks, updated_at=now()`,
			schoolID, sessionID, m.StudentID, m.Status, m.Remarks, markedBy,
		)
	}

	br := tx.SendBatch(ctx, batch)
	defer br.Close()

	for i := 0; i < len(marks); i++ {
		if _, err := br.Exec(); err != nil {
			return fmt.Errorf("execute batch item %d: %w", i, err)
		}
	}
	return nil
}
