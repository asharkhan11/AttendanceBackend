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
	// Auto-archive any currently active session for this school when creating a new session
	_, err := r.q(ctx).Exec(ctx,
		`UPDATE academic_years SET is_current = false, is_archived = true WHERE school_id = $1 AND is_current = true`,
		schoolID)
	if err != nil {
		return nil, fmt.Errorf("auto-archive existing active year: %w", err)
	}

	var ay models.AcademicYear
	// New session is always active and not archived
	err = r.q(ctx).QueryRow(ctx,
		`INSERT INTO academic_years (school_id, name, start_date, end_date, is_current, is_archived)
		 VALUES ($1, $2, $3, $4, true, false)
		 RETURNING id, school_id, name, start_date, end_date, is_current, is_archived, created_at`,
		schoolID, name, start, end).
		Scan(&ay.ID, &ay.SchoolID, &ay.Name, &ay.StartDate, &ay.EndDate, &ay.IsCurrent, &ay.IsArchived, &ay.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create academic year: %w", err)
	}
	return &ay, nil
}

func (r *AcademicYearRepo) Update(ctx context.Context, schoolID, yearID uuid.UUID, name string, start, end *time.Time) (*models.AcademicYear, error) {
	var query string
	var args []interface{}
	if start != nil && end != nil {
		query = `UPDATE academic_years 
		         SET name = $1, start_date = $2, end_date = $3 
		         WHERE id = $4 AND school_id = $5
		         RETURNING id, school_id, name, start_date, end_date, is_current, is_archived, created_at`
		args = []interface{}{name, *start, *end, yearID, schoolID}
	} else {
		query = `UPDATE academic_years 
		         SET name = $1 
		         WHERE id = $2 AND school_id = $3
		         RETURNING id, school_id, name, start_date, end_date, is_current, is_archived, created_at`
		args = []interface{}{name, yearID, schoolID}
	}

	var ay models.AcademicYear
	err := r.q(ctx).QueryRow(ctx, query, args...).
		Scan(&ay.ID, &ay.SchoolID, &ay.Name, &ay.StartDate, &ay.EndDate, &ay.IsCurrent, &ay.IsArchived, &ay.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("update academic year: %w", err)
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

func (r *AcademicYearRepo) GetByID(ctx context.Context, schoolID, yearID uuid.UUID) (*models.AcademicYear, error) {
	var ay models.AcademicYear
	err := r.q(ctx).QueryRow(ctx,
		`SELECT id, school_id, name, start_date, end_date, is_current, is_archived, created_at
		 FROM academic_years
		 WHERE school_id = $1 AND id = $2`, schoolID, yearID).
		Scan(&ay.ID, &ay.SchoolID, &ay.Name, &ay.StartDate, &ay.EndDate, &ay.IsCurrent, &ay.IsArchived, &ay.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get academic year by id: %w", err)
	}
	return &ay, nil
}

func (r *AcademicYearRepo) List(ctx context.Context, schoolID uuid.UUID) ([]*models.AcademicYear, error) {
	rows, err := r.q(ctx).Query(ctx,
		`SELECT id, school_id, name, start_date, end_date, is_current, is_archived, created_at
		 FROM academic_years
		 WHERE school_id = $1
		 ORDER BY is_current DESC, start_date DESC`, schoolID)
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
	// Verify session is not archived
	var isArchived bool
	err := r.q(ctx).QueryRow(ctx, `SELECT is_archived FROM academic_years WHERE id = $1 AND school_id = $2`, yearID, schoolID).Scan(&isArchived)
	if err != nil {
		return fmt.Errorf("academic year not found: %w", err)
	}
	if isArchived {
		return errors.New("cannot set an archived academic session as active (archival is permanent)")
	}

	_, err = r.q(ctx).Exec(ctx, `UPDATE academic_years SET is_current=false WHERE school_id=$1`, schoolID)
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
	if !archived {
		return errors.New("archival is permanent and irreversible; an archived session cannot be unarchived")
	}
	// Archiving locks the session into read-only mode and clears is_current
	cmd, err := r.q(ctx).Exec(ctx,
		`UPDATE academic_years SET is_archived = true, is_current = false WHERE id = $1 AND school_id = $2`,
		yearID, schoolID)
	if err != nil {
		return fmt.Errorf("archive academic year: %w", err)
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
		`UPDATE students SET class_id=$1, academic_year_id=$2, updated_at=now()
		 WHERE id = ANY($3)`,
		targetClassID, targetYearID, studentIDs)
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

func (r *AcademicYearRepo) CopyStudents(ctx context.Context, schoolID, sourceYearID, targetYearID uuid.UUID) (int, error) {
	// Create brand new student records isolated in target academic year
	cmd, err := r.q(ctx).Exec(ctx,
		`INSERT INTO students (
			school_id, academic_year_id, class_id, admission_number, roll_number, full_name,
			gender, dob, guardian_name, guardian_phone, is_active
		 )
		 SELECT 
			s.school_id, 
			c_target.academic_year_id, 
			c_target.id, 
			s.admission_number, 
			s.roll_number, 
			s.full_name,
			s.gender, 
			s.dob, 
			s.guardian_name, 
			s.guardian_phone, 
			s.is_active
		 FROM students s
		 JOIN classes c_source ON s.class_id = c_source.id
		 JOIN classes c_target ON c_target.school_id = s.school_id 
							  AND c_target.academic_year_id = $2 
							  AND c_target.grade_level = c_source.grade_level 
							  AND c_target.section = c_source.section
		 WHERE s.school_id = $1 
		   AND (s.academic_year_id = $3 OR c_source.academic_year_id = $3)
		 ON CONFLICT (academic_year_id, admission_number) DO NOTHING`,
		schoolID, targetYearID, sourceYearID)
	if err != nil {
		return 0, fmt.Errorf("copy students: %w", err)
	}

	// Insert enrollments for target session
	_, _ = r.q(ctx).Exec(ctx,
		`INSERT INTO student_class_enrollments (school_id, student_id, class_id, academic_year_id, roll_number, status)
		 SELECT s.school_id, s.id, s.class_id, s.academic_year_id, s.roll_number, 'active'
		 FROM students s
		 WHERE s.school_id = $1 AND s.academic_year_id = $2
		 ON CONFLICT (student_id, academic_year_id) DO NOTHING`,
		schoolID, targetYearID)

	return int(cmd.RowsAffected()), nil
}

type CopyAllResult struct {
	ClassesCopied  int `json:"classes_copied"`
	TeachersCopied int `json:"teachers_copied"`
	StudentsCopied int `json:"students_copied"`
}

func (r *AcademicYearRepo) CopyAll(ctx context.Context, schoolID, sourceYearID, targetYearID uuid.UUID) (*CopyAllResult, error) {
	clsCount, err := r.CopyClasses(ctx, schoolID, sourceYearID, targetYearID)
	if err != nil {
		return nil, err
	}
	tchCount, err := r.CopyTeachers(ctx, schoolID, sourceYearID, targetYearID, nil)
	if err != nil {
		return nil, err
	}
	stuCount, err := r.CopyStudents(ctx, schoolID, sourceYearID, targetYearID)
	if err != nil {
		return nil, err
	}
	return &CopyAllResult{
		ClassesCopied:  clsCount,
		TeachersCopied: tchCount,
		StudentsCopied: stuCount,
	}, nil
}

type MoveHigherResult struct {
	GraduatedCount int64 `json:"graduated_count"`
	PromotedCount  int64 `json:"promoted_count"`
}

func (r *AcademicYearRepo) MoveAllStudentsHigher(ctx context.Context, schoolID, academicYearID uuid.UUID) (*MoveHigherResult, error) {
	// 1. Fetch all distinct grade levels in this academic year
	rows, err := r.q(ctx).Query(ctx,
		`SELECT DISTINCT grade_level FROM classes WHERE school_id = $1 AND academic_year_id = $2 ORDER BY grade_level DESC`,
		schoolID, academicYearID)
	if err != nil {
		return nil, fmt.Errorf("fetch grade levels: %w", err)
	}
	defer rows.Close()

	var grades []int16
	for rows.Next() {
		var g int16
		if err := rows.Scan(&g); err == nil {
			grades = append(grades, g)
		}
	}
	if len(grades) == 0 {
		return &MoveHigherResult{GraduatedCount: 0, PromotedCount: 0}, nil
	}

	maxGrade := grades[0] // Since ordered DESC

	// 2. Students in highest grade level graduate and are removed from the active session
	delCmd, err := r.q(ctx).Exec(ctx,
		`DELETE FROM students 
		 WHERE school_id = $1 AND academic_year_id = $2 
		   AND class_id IN (SELECT id FROM classes WHERE school_id = $1 AND academic_year_id = $2 AND grade_level = $3)`,
		schoolID, academicYearID, maxGrade)
	if err != nil {
		return nil, fmt.Errorf("graduate max grade students: %w", err)
	}
	graduatedCount := delCmd.RowsAffected()

	// 3. Move remaining grades up by 1 (from maxGrade-1 down to minGrade)
	var totalPromoted int64
	for i := 1; i < len(grades); i++ {
		currentGrade := grades[i]
		nextGrade := currentGrade + 1

		// Move students to matching grade+1 and section, or fallback to any grade+1 class
		cmd, err := r.q(ctx).Exec(ctx,
			`UPDATE students s
			 SET class_id = COALESCE(
				(SELECT id FROM classes c_target 
				 WHERE c_target.school_id = $1 AND c_target.academic_year_id = $2 AND c_target.grade_level = $4 
				   AND c_target.section = (SELECT section FROM classes c_src WHERE c_src.id = s.class_id)
				 LIMIT 1),
				(SELECT id FROM classes c_target 
				 WHERE c_target.school_id = $1 AND c_target.academic_year_id = $2 AND c_target.grade_level = $4 
				 LIMIT 1)
			 ),
			 updated_at = now()
			 WHERE s.school_id = $1 AND s.academic_year_id = $2 
			   AND s.class_id IN (SELECT id FROM classes WHERE school_id = $1 AND academic_year_id = $2 AND grade_level = $3)`,
			schoolID, academicYearID, currentGrade, nextGrade)
		if err != nil {
			return nil, fmt.Errorf("move grade %d to %d: %w", currentGrade, nextGrade, err)
		}
		totalPromoted += cmd.RowsAffected()

		// Update enrollments
		_, _ = r.q(ctx).Exec(ctx,
			`UPDATE student_class_enrollments e
			 SET class_id = s.class_id, updated_at = now()
			 FROM students s
			 WHERE e.student_id = s.id AND e.academic_year_id = $2 AND s.academic_year_id = $2`,
			schoolID, academicYearID)
	}

	return &MoveHigherResult{
		GraduatedCount: graduatedCount,
		PromotedCount:  totalPromoted,
	}, nil
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
