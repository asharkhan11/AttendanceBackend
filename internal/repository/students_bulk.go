package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
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
	if len(students) == 0 {
		return nil
	}

	ids := make([]uuid.UUID, len(students))
	schoolIDs := make([]uuid.UUID, len(students))
	yearIDs := make([]uuid.UUID, len(students))
	classIDs := make([]uuid.UUID, len(students))
	admissionNums := make([]string, len(students))
	rollNums := make([]string, len(students))
	fullNames := make([]string, len(students))
	genders := make([]*string, len(students))
	dobs := make([]*time.Time, len(students))
	guardianNames := make([]*string, len(students))
	guardianPhones := make([]*string, len(students))

	for i, s := range students {
		if s.ID == uuid.Nil {
			s.ID = uuid.New()
		}
		ids[i] = s.ID
		schoolIDs[i] = s.SchoolID
		yearIDs[i] = s.AcademicYearID
		classIDs[i] = s.ClassID
		admissionNums[i] = s.AdmissionNumber
		rollNums[i] = s.RollNumber
		fullNames[i] = s.FullName
		genders[i] = s.Gender
		dobs[i] = s.DOB
		guardianNames[i] = s.GuardianName
		guardianPhones[i] = s.GuardianPhone
	}

	query := `
		INSERT INTO students (id, school_id, academic_year_id, class_id, admission_number, roll_number, full_name, gender, dob, guardian_name, guardian_phone)
		SELECT 
			u.id, 
			u.school_id, 
			COALESCE(NULLIF(u.academic_year_id, '00000000-0000-0000-0000-000000000000'::uuid), c.academic_year_id), 
			u.class_id, 
			u.admission_number, 
			u.roll_number, 
			u.full_name, 
			u.gender, 
			u.dob, 
			u.guardian_name, 
			u.guardian_phone
		FROM UNNEST(
			$1::uuid[],
			$2::uuid[],
			$3::uuid[],
			$4::uuid[],
			$5::text[],
			$6::text[],
			$7::text[],
			$8::text[],
			$9::date[],
			$10::text[],
			$11::text[]
		) AS u(id, school_id, academic_year_id, class_id, admission_number, roll_number, full_name, gender, dob, guardian_name, guardian_phone)
		JOIN classes c ON u.class_id = c.id
	`
	_, err := tx.Exec(ctx, query, ids, schoolIDs, yearIDs, classIDs, admissionNums, rollNums, fullNames, genders, dobs, guardianNames, guardianPhones)
	if err != nil {
		return fmt.Errorf("bulk create students: %w", err)
	}

	studentIDs := ids

	_, err = tx.Exec(ctx, `
		INSERT INTO student_class_enrollments (school_id, student_id, class_id, academic_year_id, roll_number, status)
		SELECT s.school_id, s.id, s.class_id, s.academic_year_id, s.roll_number, 'active'
		FROM students s
		WHERE s.id = ANY($1)
		ON CONFLICT (student_id, academic_year_id) DO UPDATE SET class_id = EXCLUDED.class_id, roll_number = EXCLUDED.roll_number`,
		studentIDs)
	if err != nil {
		return fmt.Errorf("bulk create student enrollments: %w", err)
	}

	return nil
}
