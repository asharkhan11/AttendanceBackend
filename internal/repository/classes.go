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

type AcademicYearRepo struct{ db *DB }

func NewAcademicYearRepo(db *DB) *AcademicYearRepo { return &AcademicYearRepo{db: db} }
func (r *AcademicYearRepo) Db() *DB { return r.db }
func (r *AcademicYearRepo) q(ctx context.Context) DBTX   { return r.db.Querier(ctx) }

func (r *AcademicYearRepo) Create(ctx context.Context, schoolID uuid.UUID, name string, start, end time.Time, isCurrent bool) (*models.AcademicYear, error) {
	var ay models.AcademicYear
	err := r.q(ctx).QueryRow(ctx,
		`INSERT INTO academic_years (school_id, name, start_date, end_date, is_current)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, school_id, name, start_date, end_date, is_current, created_at`,
		schoolID, name, start, end, isCurrent).
		Scan(&ay.ID, &ay.SchoolID, &ay.Name, &ay.StartDate, &ay.EndDate, &ay.IsCurrent, &ay.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create academic year: %w", err)
	}
	return &ay, nil
}

func (r *AcademicYearRepo) GetCurrent(ctx context.Context, schoolID uuid.UUID) (*models.AcademicYear, error) {
	var ay models.AcademicYear
	err := r.q(ctx).QueryRow(ctx,
		`SELECT id, school_id, name, start_date, end_date, is_current, is_archived, created_at
		 FROM academic_years
		 WHERE school_id = $1 AND is_current = true
		 ORDER BY created_at DESC LIMIT 1`, schoolID).
		Scan(&ay.ID, &ay.SchoolID, &ay.Name, &ay.StartDate, &ay.EndDate, &ay.IsCurrent, &ay.IsArchived, &ay.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get current academic year: %w", err)
	}
	return &ay, nil
}

func (r *AcademicYearRepo) List(ctx context.Context, schoolID uuid.UUID) ([]*models.AcademicYear, error) {
	rows, err := r.q(ctx).Query(ctx,
		`SELECT id, school_id, name, start_date, end_date, is_current, is_archived, created_at
		 FROM academic_years
		 WHERE school_id = $1
		 ORDER BY start_date DESC`, schoolID)
	if err != nil {
		return nil, fmt.Errorf("list academic years: %w", err)
	}
	defer rows.Close()

	var list []*models.AcademicYear
	for rows.Next() {
		var ay models.AcademicYear
		if err := rows.Scan(&ay.ID, &ay.SchoolID, &ay.Name, &ay.StartDate, &ay.EndDate, &ay.IsCurrent, &ay.IsArchived, &ay.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, &ay)
	}
	return list, nil
}

func (r *AcademicYearRepo) SetCurrent(ctx context.Context, schoolID, yearID uuid.UUID) error {
	_, err := r.q(ctx).Exec(ctx, `UPDATE academic_years SET is_current=false WHERE school_id=$1`, schoolID)
	if err != nil {
		return fmt.Errorf("unset current academic year: %w", err)
	}
	cmd, err := r.q(ctx).Exec(ctx, `UPDATE academic_years SET is_current=true WHERE id=$1 AND school_id=$2`, yearID, schoolID)
	if err != nil {
		return fmt.Errorf("set current academic year: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return fmt.Errorf("academic year not found")
	}
	return nil
}

func (r *AcademicYearRepo) SetArchived(ctx context.Context, schoolID, yearID uuid.UUID, archived bool) error {
	cmd, err := r.q(ctx).Exec(ctx, `UPDATE academic_years SET is_archived=$1 WHERE id=$2 AND school_id=$3`, archived, yearID, schoolID)
	if err != nil {
		return fmt.Errorf("set archived academic year: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return fmt.Errorf("academic year not found")
	}
	return nil
}

func (r *AcademicYearRepo) PromoteStudents(ctx context.Context, sourceClassID, targetClassID uuid.UUID, studentIDs []uuid.UUID) (int64, error) {
	var targetYearID uuid.UUID
	err := r.q(ctx).QueryRow(ctx, `SELECT academic_year_id FROM classes WHERE id=$1`, targetClassID).Scan(&targetYearID)
	if err != nil {
		return 0, fmt.Errorf("target class not found: %w", err)
	}

	if _, err := r.q(ctx).Exec(ctx,
		`UPDATE student_class_enrollments
		 SET status='promoted', updated_at=now()
		 WHERE class_id=$1 AND student_id=ANY($2)`,
		sourceClassID, studentIDs); err != nil {
		return 0, fmt.Errorf("update enrollment status to promoted: %w", err)
	}

	_, err = r.q(ctx).Exec(ctx,
		`INSERT INTO student_class_enrollments (school_id, student_id, class_id, academic_year_id, roll_number, status)
		 SELECT s.school_id, s.id, $1, $2, s.roll_number, 'active'
		 FROM students s
		 WHERE s.id = ANY($3)
		 ON CONFLICT (student_id, academic_year_id)
		 DO UPDATE SET class_id=EXCLUDED.class_id, roll_number=EXCLUDED.roll_number, status='active', updated_at=now()`,
		targetClassID, targetYearID, studentIDs)
	if err != nil {
		return 0, fmt.Errorf("upsert student enrollments: %w", err)
	}

	cmd, err := r.q(ctx).Exec(ctx,
		`UPDATE students SET class_id=$1, updated_at=now()
		 WHERE id = ANY($2)`,
		targetClassID, studentIDs)
	if err != nil {
		return 0, fmt.Errorf("promote students: %w", err)
	}
	return cmd.RowsAffected(), nil
}

func (r *AcademicYearRepo) CopyTeachers(ctx context.Context, schoolID, sourceYearID, targetYearID uuid.UUID, classMappings map[string]string) (int, error) {
	cmd, err := r.q(ctx).Exec(ctx,
		`UPDATE classes c_target
		 SET class_teacher_id = c_source.class_teacher_id
		 FROM classes c_source
		 WHERE c_source.school_id = $1 AND c_source.academic_year_id = $3
		   AND c_target.school_id = $1 AND c_target.academic_year_id = $2
		   AND c_target.grade_level = c_source.grade_level
		   AND c_target.section = c_source.section
		   AND c_source.class_teacher_id IS NOT NULL`,
		schoolID, targetYearID, sourceYearID)
	if err != nil {
		return 0, fmt.Errorf("copy teachers: %w", err)
	}
	return int(cmd.RowsAffected()), nil
}

func (r *AcademicYearRepo) CopyClasses(ctx context.Context, schoolID, sourceYearID, targetYearID uuid.UUID) (int, error) {
	cmd, err := r.q(ctx).Exec(ctx,
		`INSERT INTO classes (school_id, academic_year_id, grade_level, section)
		 SELECT school_id, $2, grade_level, section
		 FROM classes
		 WHERE school_id = $1 AND academic_year_id = $3
		 ON CONFLICT (school_id, academic_year_id, grade_level, section) DO NOTHING`,
		schoolID, targetYearID, sourceYearID)
	if err != nil {
		return 0, fmt.Errorf("copy classes: %w", err)
	}
	return int(cmd.RowsAffected()), nil
}

type ClassWithTeacher struct {
	models.Class
	ClassTeacherName  *string `json:"class_teacher_name"`
	ClassTeacherEmail *string `json:"class_teacher_email"`
}


type ClassRepo struct{ db *DB }

func NewClassRepo(db *DB) *ClassRepo { return &ClassRepo{db: db} }
func (r *ClassRepo) Db() *DB { return r.db }
func (r *ClassRepo) q(ctx context.Context) DBTX   { return r.db.Querier(ctx) }

func (r *ClassRepo) Create(ctx context.Context, schoolID, academicYearID uuid.UUID, gradeLevel int16, section string, classTeacherID *uuid.UUID) (*ClassWithTeacher, error) {
	var c models.Class
	err := r.q(ctx).QueryRow(ctx,
		`INSERT INTO classes (school_id, academic_year_id, grade_level, section, class_teacher_id)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, school_id, academic_year_id, grade_level, section, class_teacher_id, created_at`,
		schoolID, academicYearID, gradeLevel, section, classTeacherID).
		Scan(&c.ID, &c.SchoolID, &c.AcademicYearID, &c.GradeLevel, &c.Section, &c.ClassTeacherID, &c.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create class: %w", err)
	}
	return r.GetByID(ctx, c.ID)
}

func (r *ClassRepo) GetByID(ctx context.Context, id uuid.UUID) (*ClassWithTeacher, error) {
	q := `SELECT c.id, c.school_id, c.academic_year_id, c.grade_level, c.section, c.class_teacher_id, c.created_at, u.full_name, u.email
		  FROM classes c
		  LEFT JOIN users u ON c.class_teacher_id = u.id
		  WHERE c.id = $1`
	var c models.Class
	var tName, tEmail *string
	err := r.q(ctx).QueryRow(ctx, q, id).
		Scan(&c.ID, &c.SchoolID, &c.AcademicYearID, &c.GradeLevel, &c.Section, &c.ClassTeacherID, &c.CreatedAt, &tName, &tEmail)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("class not found")
		}
		return nil, fmt.Errorf("get class by id: %w", err)
	}
	return &ClassWithTeacher{Class: c, ClassTeacherName: tName, ClassTeacherEmail: tEmail}, nil
}

func (r *ClassRepo) GetByTeacherID(ctx context.Context, schoolID, teacherID uuid.UUID) (*ClassWithTeacher, error) {
	q := `SELECT c.id, c.school_id, c.academic_year_id, c.grade_level, c.section, c.class_teacher_id, c.created_at, u.full_name, u.email
		  FROM classes c
		  JOIN academic_years ay ON c.academic_year_id = ay.id
		  LEFT JOIN users u ON c.class_teacher_id = u.id
		  WHERE c.school_id = $1 AND c.class_teacher_id = $2 AND ay.is_current = true`
	var c models.Class
	var tName, tEmail *string
	err := r.q(ctx).QueryRow(ctx, q, schoolID, teacherID).
		Scan(&c.ID, &c.SchoolID, &c.AcademicYearID, &c.GradeLevel, &c.Section, &c.ClassTeacherID, &c.CreatedAt, &tName, &tEmail)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("no class assigned to teacher")
		}
		return nil, fmt.Errorf("get class by teacher id: %w", err)
	}
	return &ClassWithTeacher{Class: c, ClassTeacherName: tName, ClassTeacherEmail: tEmail}, nil
}

func (r *ClassRepo) List(ctx context.Context, schoolID uuid.UUID, academicYearID *uuid.UUID, limit, offset int) ([]*ClassWithTeacher, int64, error) {
	q := `SELECT c.id, c.school_id, c.academic_year_id, c.grade_level, c.section, c.class_teacher_id, c.created_at, u.full_name, u.email
		  FROM classes c
		  LEFT JOIN users u ON c.class_teacher_id = u.id
		  WHERE c.school_id = $1`
	countQ := `SELECT COUNT(*) FROM classes WHERE school_id = $1`
	args := []any{schoolID}

	if academicYearID != nil {
		q += fmt.Sprintf(" AND c.academic_year_id = $%d", len(args)+1)
		countQ += fmt.Sprintf(" AND academic_year_id = $%d", len(args)+1)
		args = append(args, *academicYearID)
	}

	q += fmt.Sprintf(" ORDER BY c.grade_level ASC, c.section ASC LIMIT $%d OFFSET $%d", len(args)+1, len(args)+2)
	queryArgs := append(args, limit, offset)

	var total int64
	if err := r.q(ctx).QueryRow(ctx, countQ, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count classes: %w", err)
	}

	rows, err := r.q(ctx).Query(ctx, q, queryArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list classes: %w", err)
	}
	defer rows.Close()

	classes := make([]*ClassWithTeacher, 0)
	for rows.Next() {
		var c models.Class
		var tName, tEmail *string
		if err := rows.Scan(&c.ID, &c.SchoolID, &c.AcademicYearID, &c.GradeLevel, &c.Section, &c.ClassTeacherID, &c.CreatedAt, &tName, &tEmail); err != nil {
			return nil, 0, fmt.Errorf("scan class: %w", err)
		}
		classes = append(classes, &ClassWithTeacher{Class: c, ClassTeacherName: tName, ClassTeacherEmail: tEmail})
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("classes rows err: %w", err)
	}

	return classes, total, nil
}

func (r *ClassRepo) AssignTeacher(ctx context.Context, schoolID, classID uuid.UUID, teacherID *uuid.UUID) (*ClassWithTeacher, error) {
	_, err := r.q(ctx).Exec(ctx,
		`UPDATE classes SET class_teacher_id = $1 WHERE id = $2 AND school_id = $3`,
		teacherID, classID, schoolID)
	if err != nil {
		return nil, fmt.Errorf("assign teacher: %w", err)
	}
	return r.GetByID(ctx, classID)
}

func (r *ClassRepo) Delete(ctx context.Context, id uuid.UUID) error {
	cmd, err := r.q(ctx).Exec(ctx, `DELETE FROM classes WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete class: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return fmt.Errorf("class not found")
	}
	return nil
}
