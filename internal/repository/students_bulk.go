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

func (r *StudentBulkRepository) BulkCreate(ctx context.Context, students []*models.Student) error {
	tx := TxFromContext(ctx)
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

	studentIDs := make([]uuid.UUID, len(students))
	for i, s := range students {
		studentIDs[i] = s.ID
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO student_class_enrollments (school_id, student_id, class_id, academic_year_id, roll_number, status)
		SELECT s.school_id, s.id, s.class_id, c.academic_year_id, s.roll_number, 'active'
		FROM students s
		JOIN classes c ON s.class_id = c.id
		WHERE s.id = ANY($1)
		ON CONFLICT (student_id, academic_year_id) DO UPDATE SET class_id = EXCLUDED.class_id, roll_number = EXCLUDED.roll_number`,
		studentIDs)
	if err != nil {
		return fmt.Errorf("bulk create student enrollments: %w", err)
	}

	return nil
}
