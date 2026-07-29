package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/school/attendance-backend/internal/models"
)

type StudentBulkRepository struct {
	db *DB
}

func NewStudentBulkRepository(db *DB) *StudentBulkRepository {
	return &StudentBulkRepository{db: db}
}

func (r *StudentBulkRepository) q(ctx context.Context) pgx.Tx {
	return r.db.Querier(ctx)
}

func (r *StudentBulkRepository) BulkCreate(ctx context.Context, students []*models.Student) error {
	tx := r.q(ctx)
	if tx == nil {
		return fmt.Errorf("no active transaction in context")
	}

	rows := make([][]any, len(students))
	for i, s := range students {
		if s.ID == uuid.Nil {
			s.ID = uuid.New()
		}
		rows[i] = []any{
			s.ID,
			s.SchoolID,
			s.ClassID,
			s.AdmissionNumber,
			s.RollNumber,
			s.FullName,
			s.Gender,
			s.DOB,
			s.GuardianName,
			s.GuardianPhone,
		}
	}

	_, err := tx.CopyFrom(
		ctx,
		pgx.Identifier{"students"},
		[]string{
			"id",
			"school_id",
			"class_id",
			"admission_number",
			"roll_number",
			"full_name",
			"gender",
			"dob",
			"guardian_name",
			"guardian_phone",
		},
		pgx.CopyFromRows(rows),
	)
	if err != nil {
		return fmt.Errorf("bulk create students: %w", err)
	}
	return nil
}
