package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/school/attendance-backend/internal/models"
)


type StudentRepo struct{ db *DB }

func NewStudentRepo(db *DB) *StudentRepo { return &StudentRepo{db: db} }
func (r *StudentRepo) q(ctx context.Context) pgx.Tx { return r.db.Querier(ctx) }

func (r *StudentRepo) GetByID(ctx context.Context, id uuid.UUID) (*models.Student, error) {
	row := r.q(ctx).QueryRow(ctx,
		`SELECT id,school_id,class_id,admission_number,roll_number,full_name,
		        gender,dob,guardian_name,guardian_phone,is_active,created_at,updated_at
		 FROM students WHERE id=$1`, id)
	return scanStudent(row)
}

func (r *StudentRepo) ListBySchool(ctx context.Context, schoolID uuid.UUID, academicYearID *uuid.UUID, classID *uuid.UUID, search string, limit, offset int) ([]*models.Student, int64, error) {
	where := "s.school_id = $1"
	args := []any{schoolID}

	if academicYearID != nil {
		args = append(args, *academicYearID)
		where += fmt.Sprintf(" AND c.academic_year_id = $%d", len(args))
	}
	if classID != nil {
		args = append(args, *classID)
		where += fmt.Sprintf(" AND s.class_id = $%d", len(args))
	}
	if search != "" {
		args = append(args, "%"+search+"%")
		where += fmt.Sprintf(" AND (s.full_name ILIKE $%d OR s.admission_number ILIKE $%d OR s.roll_number ILIKE $%d)", len(args), len(args), len(args))
	}

	countQ := fmt.Sprintf(`SELECT COUNT(*) FROM students s JOIN classes c ON s.class_id = c.id WHERE %s`, where)
	var total int64
	if err := r.q(ctx).QueryRow(ctx, countQ, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count students: %w", err)
	}

	q := fmt.Sprintf(`SELECT s.id,s.school_id,s.class_id,s.admission_number,s.roll_number,s.full_name,
	                         s.gender,s.dob,s.guardian_name,s.guardian_phone,s.is_active,s.created_at,s.updated_at
	                  FROM students s JOIN classes c ON s.class_id = c.id
	                  WHERE %s
	                  ORDER BY s.roll_number LIMIT $%d OFFSET $%d`, where, len(args)+1, len(args)+2)
	args = append(args, limit, offset)
	rows, err := r.q(ctx).Query(ctx, q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var out []*models.Student
	for rows.Next() {
		s, err := scanStudentRow(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, s)
	}
	return out, total, nil
}

func (r *StudentRepo) ListByClass(ctx context.Context, classID uuid.UUID, activeOnly bool, limit, offset int) ([]*models.Student, int64, error) {
	countQ := `SELECT COUNT(*) FROM students WHERE class_id=$1`
	if activeOnly {
		countQ += " AND is_active=true"
	}
	var total int64
	if err := r.q(ctx).QueryRow(ctx, countQ, classID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count students: %w", err)
	}

	q := `SELECT id,school_id,class_id,admission_number,roll_number,full_name,
		         gender,dob,guardian_name,guardian_phone,is_active,created_at,updated_at
		  FROM students WHERE class_id=$1`
	args := []any{classID}
	if activeOnly {
		q += " AND is_active=true"
	}
	q += fmt.Sprintf(" ORDER BY roll_number LIMIT $%d OFFSET $%d", len(args)+1, len(args)+2)
	args = append(args, limit, offset)
	rows, err := r.q(ctx).Query(ctx, q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := make([]*models.Student, 0)
	for rows.Next() {
		s, err := scanStudentRow(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("students rows err: %w", err)
	}
	return out, total, nil
}

func (r *StudentRepo) Create(ctx context.Context, s *models.Student) (*models.Student, error) {
	row := r.q(ctx).QueryRow(ctx,
		`INSERT INTO students(school_id,class_id,admission_number,roll_number,full_name,
		                      gender,dob,guardian_name,guardian_phone)
		 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)
		 RETURNING id,school_id,class_id,admission_number,roll_number,full_name,
		           gender,dob,guardian_name,guardian_phone,is_active,created_at,updated_at`,
		s.SchoolID, s.ClassID, s.AdmissionNumber, s.RollNumber, s.FullName,
		s.Gender, s.DOB, s.GuardianName, s.GuardianPhone)
	return scanStudent(row)
}

func (r *StudentRepo) Update(ctx context.Context, id uuid.UUID, classID uuid.UUID, admissionNumber, rollNumber, fullName string, gender *string, dob *time.Time, guardianName, guardianPhone *string) (*models.Student, error) {
	row := r.q(ctx).QueryRow(ctx,
		`UPDATE students SET class_id=$2, admission_number=$3, roll_number=$4, full_name=$5, gender=$6, dob=$7, guardian_name=$8, guardian_phone=$9, updated_at=now()
		 WHERE id=$1
		 RETURNING id,school_id,class_id,admission_number,roll_number,full_name,
		           gender,dob,guardian_name,guardian_phone,is_active,created_at,updated_at`,
		id, classID, admissionNumber, rollNumber, fullName, gender, dob, guardianName, guardianPhone)
	return scanStudent(row)
}

func (r *StudentRepo) Delete(ctx context.Context, id uuid.UUID) error {
	cmd, err := r.q(ctx).Exec(ctx, `DELETE FROM students WHERE id=$1`, id)
	if err != nil {
		return fmt.Errorf("delete student: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return fmt.Errorf("student not found")
	}
	return nil
}

func (r *StudentRepo) Deactivate(ctx context.Context, id uuid.UUID) error {
	_, err := r.q(ctx).Exec(ctx,
		`UPDATE students SET is_active=false,updated_at=now() WHERE id=$1`, id)
	return err
}

// GetAllByIDs returns students matching a list of IDs (used in bulk-mark validation).
func (r *StudentRepo) GetAllByClassAndIDs(ctx context.Context, classID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]bool, error) {
	rows, err := r.q(ctx).Query(ctx,
		`SELECT id FROM students WHERE class_id=$1 AND id=ANY($2) AND is_active=true`,
		classID, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	found := make(map[uuid.UUID]bool)
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		found[id] = true
	}
	return found, nil
}

type NextStudentInfo struct {
	NextAdmissionNumber     string `json:"next_admission_number"`
	LastRollNumber          string `json:"last_roll_number"`
	SuggestedNextRollNumber string `json:"suggested_next_roll_number"`
}

func (r *StudentRepo) GetNextStudentInfo(ctx context.Context, schoolID, classID uuid.UUID) (*NextStudentInfo, error) {
	// 1. Determine next admission number for school
	var lastAdm string
	err := r.q(ctx).QueryRow(ctx,
		`SELECT admission_number 
		 FROM students 
		 WHERE school_id = $1 AND admission_number ~ '\d+$'
		 ORDER BY (substring(admission_number from '\d+$'))::integer DESC 
		 LIMIT 1`, schoolID).Scan(&lastAdm)

	admPrefix := "ADM"
	admPad := 3
	nextAdm := fmt.Sprintf("%s%0*d", admPrefix, admPad, 1)

	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("get max admission number: %w", err)
	}

	if err == nil {
		prefix, num, pad := extractNumberSuffix(lastAdm)
		if num >= 0 {
			if pad > 0 {
				nextAdm = fmt.Sprintf("%s%0*d", prefix, pad, num+1)
			} else {
				nextAdm = fmt.Sprintf("%s%d", prefix, num+1)
			}
		}
	}

	// 2. Determine last roll number for class
	var lastRoll string
	var nextRoll string
	var rollStr string
	err = r.q(ctx).QueryRow(ctx,
		`SELECT roll_number FROM students WHERE class_id=$1 ORDER BY length(roll_number) DESC, roll_number DESC LIMIT 1`,
		classID).Scan(&rollStr)

	if err != nil {
		lastRoll = "None"
		nextRoll = "01"
	} else {
		lastRoll = rollStr
		prefix, num, pad := extractNumberSuffix(rollStr)
		if num >= 0 {
			if pad > 0 {
				nextRoll = fmt.Sprintf("%s%0*d", prefix, pad, num+1)
			} else {
				nextRoll = fmt.Sprintf("%s%d", prefix, num+1)
			}
		} else {
			nextRoll = rollStr + "-1"
		}
	}

	return &NextStudentInfo{
		NextAdmissionNumber:     nextAdm,
		LastRollNumber:          lastRoll,
		SuggestedNextRollNumber: nextRoll,
	}, nil
}

func extractNumberSuffix(s string) (prefix string, num int, pad int) {
	i := len(s) - 1
	for i >= 0 && s[i] >= '0' && s[i] <= '9' {
		i--
	}
	if i == len(s)-1 {
		return s, -1, 0
	}
	numStr := s[i+1:]
	var n int
	fmt.Sscanf(numStr, "%d", &n)
	return s[:i+1], n, len(numStr)
}

func scanStudent(row pgx.Row) (*models.Student, error) {
	s := &models.Student{}
	if err := row.Scan(&s.ID, &s.SchoolID, &s.ClassID, &s.AdmissionNumber, &s.RollNumber,
		&s.FullName, &s.Gender, &s.DOB, &s.GuardianName, &s.GuardianPhone,
		&s.IsActive, &s.CreatedAt, &s.UpdatedAt); err != nil {
		return nil, fmt.Errorf("scan student: %w", err)
	}
	return s, nil
}

func scanStudentRow(rows pgx.Rows) (*models.Student, error) {
	s := &models.Student{}
	if err := rows.Scan(&s.ID, &s.SchoolID, &s.ClassID, &s.AdmissionNumber, &s.RollNumber,
		&s.FullName, &s.Gender, &s.DOB, &s.GuardianName, &s.GuardianPhone,
		&s.IsActive, &s.CreatedAt, &s.UpdatedAt); err != nil {
		return nil, fmt.Errorf("scan student row: %w", err)
	}
	return s, nil
}
