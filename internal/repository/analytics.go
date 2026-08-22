package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type AnalyticsRepo struct{ db *DB }

func NewAnalyticsRepo(db *DB) *AnalyticsRepo { return &AnalyticsRepo{db: db} }
func (r *AnalyticsRepo) Db() *DB { return r.db }
func (r *AnalyticsRepo) q(ctx context.Context) DBTX { return r.db.Querier(ctx) }

// ClassSummaryData is the overall attendance metrics for a class.
type ClassSummaryData struct {
	ClassID              uuid.UUID `json:"class_id"`
	TotalStudents        int64     `json:"total_students"`
	TotalSessions        int64     `json:"total_sessions"`
	TotalRecords         int64     `json:"total_records"`
	Present              int64     `json:"present"`
	Absent               int64     `json:"absent"`
	Late                 int64     `json:"late"`
	Excused              int64     `json:"excused"`
	AttendancePercentage float64   `json:"attendance_percentage"`
}

// ClassSummary returns overall attendance stats for a class in a date range.
func (r *AnalyticsRepo) ClassSummary(ctx context.Context, classID uuid.UUID, from, to time.Time) (*ClassSummaryData, error) {
	q := `
		SELECT
			$1::uuid AS class_id,
			COALESCE((SELECT COUNT(*) FROM students st LEFT JOIN student_class_enrollments e ON e.student_id = st.id AND e.class_id = $1 WHERE (e.class_id = $1 OR (e.class_id IS NULL AND st.class_id = $1)) AND st.is_active = true), 0) AS total_students,
			COALESCE(COUNT(DISTINCT s.id), 0) AS total_sessions,
			COALESCE(COUNT(ar.id), 0) AS total_records,
			COALESCE(COUNT(ar.id) FILTER (WHERE ar.status = 'present'), 0) AS present,
			COALESCE(COUNT(ar.id) FILTER (WHERE ar.status = 'absent'), 0) AS absent,
			COALESCE(COUNT(ar.id) FILTER (WHERE ar.status = 'late'), 0) AS late,
			COALESCE(COUNT(ar.id) FILTER (WHERE ar.status = 'excused'), 0) AS excused,
			COALESCE(ROUND(100.0 * COUNT(ar.id) FILTER (WHERE ar.status = 'present') / NULLIF(COUNT(ar.id), 0), 2), 0) AS attendance_percentage
		FROM attendance_sessions s
		LEFT JOIN attendance_records ar ON ar.session_id = s.id
		WHERE s.class_id = $1
		  AND s.session_date BETWEEN $2 AND $3`

	row := r.q(ctx).QueryRow(ctx, q, classID, from, to)
	out := &ClassSummaryData{}
	if err := row.Scan(
		&out.ClassID,
		&out.TotalStudents,
		&out.TotalSessions,
		&out.TotalRecords,
		&out.Present,
		&out.Absent,
		&out.Late,
		&out.Excused,
		&out.AttendancePercentage,
	); err != nil {
		return nil, fmt.Errorf("class summary query: %w", err)
	}
	return out, nil
}

// StudentSummaryRow is one row of per-period summary for a student.
type StudentSummaryRow struct {
	PeriodNumber  int16     `json:"period_number"`
	PeriodName    string    `json:"period_name"`
	SubjectName   string    `json:"subject_name"`
	PresentCount  int64     `json:"present_count"`
	TotalCount    int64     `json:"total_count"`
	AttendancePct float64   `json:"attendance_pct"`
}

// StudentDayRow is one day's records for a student.
type StudentDayRow struct {
	SessionDate  time.Time `json:"session_date"`
	SubjectName  string    `json:"subject_name"`
	PeriodNumber int16     `json:"period_number"`
	Status       string    `json:"status"`
	Remarks      *string   `json:"remarks"`
}

func (r *AnalyticsRepo) StudentSubjectSummary(ctx context.Context, studentID uuid.UUID, from, to time.Time) ([]*StudentSummaryRow, error) {
	rows, err := r.q(ctx).Query(ctx, `
		SELECT
			s.period_number,
			COUNT(*) FILTER (WHERE ar.status='present'),
			COUNT(*),
			COALESCE(ROUND(100.0 * COUNT(*) FILTER (WHERE ar.status='present') / NULLIF(COUNT(*),0), 2), 0)
		FROM attendance_records ar
		JOIN attendance_sessions s ON s.id=ar.session_id
		WHERE ar.student_id=$1
		  AND s.session_date BETWEEN $2 AND $3
		GROUP BY s.period_number
		ORDER BY s.period_number`, studentID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*StudentSummaryRow
	for rows.Next() {
		row := &StudentSummaryRow{}
		if err := rows.Scan(&row.PeriodNumber, &row.PresentCount, &row.TotalCount, &row.AttendancePct); err != nil {
			return nil, err
		}
		row.PeriodName = "Daily Attendance"
		row.SubjectName = "Daily Attendance"
		out = append(out, row)
	}
	return out, nil
}

func (r *AnalyticsRepo) StudentDayLog(ctx context.Context, studentID uuid.UUID, from, to time.Time) ([]*StudentDayRow, error) {
	rows, err := r.q(ctx).Query(ctx, `
		SELECT s.session_date, s.period_number, ar.status, ar.remarks
		FROM attendance_records ar
		JOIN attendance_sessions s ON s.id=ar.session_id
		WHERE ar.student_id=$1
		  AND s.session_date BETWEEN $2 AND $3
		ORDER BY s.session_date, s.period_number`, studentID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*StudentDayRow
	for rows.Next() {
		row := &StudentDayRow{}
		if err := rows.Scan(&row.SessionDate, &row.PeriodNumber, &row.Status, &row.Remarks); err != nil {
			return nil, err
		}
		row.SubjectName = "Daily Attendance"
		out = append(out, row)
	}
	return out, nil
}

// DefaulterRow is a student below the threshold.
type DefaulterRow struct {
	StudentID     uuid.UUID `json:"student_id"`
	FullName      string    `json:"full_name"`
	RollNumber    string    `json:"roll_number"`
	AttendancePct float64   `json:"attendance_pct"`
	PresentCount  int64     `json:"present_count"`
	TotalSessions int64     `json:"total_sessions"`
}

func (r *AnalyticsRepo) Defaulters(ctx context.Context, classID uuid.UUID, from, to time.Time, threshold float64) ([]*DefaulterRow, error) {
	rows, err := r.q(ctx).Query(ctx, `
		SELECT
			st.id,
			st.full_name,
			COALESCE(e.roll_number, st.roll_number) AS roll_number,
			COALESCE(ROUND(100.0 * COUNT(*) FILTER (WHERE ar.status='present') / NULLIF(COUNT(*),0), 2), 0) AS pct,
			COALESCE(COUNT(*) FILTER (WHERE ar.status='present'), 0) AS present_count,
			COALESCE(COUNT(*), 0) AS total_sessions
		FROM students st
		LEFT JOIN student_class_enrollments e ON e.student_id = st.id AND e.class_id = $1
		JOIN attendance_records ar ON ar.student_id=st.id
		JOIN attendance_sessions s ON s.id=ar.session_id AND s.class_id=$1
		WHERE (e.class_id=$1 OR (e.class_id IS NULL AND st.class_id=$1))
		  AND s.session_date BETWEEN $2 AND $3
		  AND st.is_active=true
		GROUP BY st.id, st.full_name, COALESCE(e.roll_number, st.roll_number)
		HAVING COALESCE(ROUND(100.0 * COUNT(*) FILTER (WHERE ar.status='present') / NULLIF(COUNT(*),0), 2), 0) < $4
		ORDER BY pct ASC`, classID, from, to, threshold)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*DefaulterRow
	for rows.Next() {
		row := &DefaulterRow{}
		if err := rows.Scan(&row.StudentID, &row.FullName, &row.RollNumber, &row.AttendancePct, &row.PresentCount, &row.TotalSessions); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, nil
}

// TrendRow is one time-bucket in the trend query.
type TrendRow struct {
	Bucket        string  `json:"bucket"`
	PresentCount  int64   `json:"present_count"`
	TotalCount    int64   `json:"total_count"`
	AttendancePct float64 `json:"attendance_pct"`
}

func (r *AnalyticsRepo) ClassTrend(ctx context.Context, classID uuid.UUID, from, to time.Time, granularity string) ([]*TrendRow, error) {
	var trunc string
	switch granularity {
	case "month":
		trunc = "month"
	default:
		trunc = "week"
	}
	rows, err := r.q(ctx).Query(ctx, fmt.Sprintf(`
		SELECT
			TO_CHAR(DATE_TRUNC('%s', s.session_date), 'YYYY-MM-DD') AS bucket,
			COUNT(*) FILTER (WHERE ar.status='present'),
			COUNT(*),
			COALESCE(ROUND(100.0 * COUNT(*) FILTER (WHERE ar.status='present') / NULLIF(COUNT(*),0), 2), 0)
		FROM attendance_records ar
		JOIN attendance_sessions s ON s.id=ar.session_id
		WHERE s.class_id=$1
		  AND s.session_date BETWEEN $2 AND $3
		GROUP BY DATE_TRUNC('%s', s.session_date)
		ORDER BY DATE_TRUNC('%s', s.session_date)`, trunc, trunc, trunc),
		classID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*TrendRow
	for rows.Next() {
		row := &TrendRow{}
		if err := rows.Scan(&row.Bucket, &row.PresentCount, &row.TotalCount, &row.AttendancePct); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, nil
}

func (r *AnalyticsRepo) SchoolTrend(ctx context.Context, schoolID uuid.UUID, from, to time.Time, granularity string) ([]*TrendRow, error) {
	var trunc string
	switch granularity {
	case "month":
		trunc = "month"
	default:
		trunc = "week"
	}
	rows, err := r.q(ctx).Query(ctx, fmt.Sprintf(`
		SELECT
			TO_CHAR(DATE_TRUNC('%s', s.session_date), 'YYYY-MM-DD') AS bucket,
			COUNT(*) FILTER (WHERE ar.status='present'),
			COUNT(*),
			COALESCE(ROUND(100.0 * COUNT(*) FILTER (WHERE ar.status='present') / NULLIF(COUNT(*),0), 2), 0)
		FROM attendance_records ar
		JOIN attendance_sessions s ON s.id=ar.session_id
		WHERE s.school_id=$1
		  AND s.session_date BETWEEN $2 AND $3
		GROUP BY DATE_TRUNC('%s', s.session_date)
		ORDER BY DATE_TRUNC('%s', s.session_date)`, trunc, trunc, trunc),
		schoolID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*TrendRow
	for rows.Next() {
		row := &TrendRow{}
		if err := rows.Scan(&row.Bucket, &row.PresentCount, &row.TotalCount, &row.AttendancePct); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, nil
}

// ClassRosterWithAttendance returns each active student with their overall attendance % in range.
type RosterAttendanceRow struct {
	StudentID     uuid.UUID `json:"student_id"`
	FullName      string    `json:"full_name"`
	RollNumber    string    `json:"roll_number"`
	AttendancePct float64   `json:"attendance_pct"`
	PresentCount  int64     `json:"present_count"`
	TotalCount    int64     `json:"total_count"`
}

func (r *AnalyticsRepo) ClassRosterWithAttendance(ctx context.Context, classID uuid.UUID, from, to time.Time) ([]*RosterAttendanceRow, error) {
	rows, err := r.q(ctx).Query(ctx, `
		SELECT
			st.id,
			st.full_name,
			COALESCE(e.roll_number, st.roll_number) AS roll_number,
			COALESCE(ROUND(100.0 * COUNT(ar.id) FILTER (WHERE ar.status='present') / NULLIF(COUNT(ar.id),0), 2), 0),
			COALESCE(COUNT(ar.id) FILTER (WHERE ar.status='present'), 0),
			COALESCE(COUNT(ar.id), 0)
		FROM students st
		LEFT JOIN student_class_enrollments e ON e.student_id = st.id AND e.class_id = $1
		LEFT JOIN attendance_records ar ON ar.student_id=st.id
		LEFT JOIN attendance_sessions s ON s.id=ar.session_id AND s.class_id=$1 AND s.session_date BETWEEN $2 AND $3
		WHERE (e.class_id=$1 OR (e.class_id IS NULL AND st.class_id=$1)) AND st.is_active=true
		GROUP BY st.id, st.full_name, COALESCE(e.roll_number, st.roll_number)
		ORDER BY COALESCE(e.roll_number, st.roll_number)`, classID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*RosterAttendanceRow
	for rows.Next() {
		row := &RosterAttendanceRow{}
		if err := rows.Scan(&row.StudentID, &row.FullName, &row.RollNumber,
			&row.AttendancePct, &row.PresentCount, &row.TotalCount); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, nil
}

// SchoolOverviewData contains global stats for the admin dashboard.
type SchoolOverviewData struct {
	TotalStudents        int64                 `json:"total_students"`
	TotalClasses         int64                 `json:"total_classes"`
	TotalSessions        int64                 `json:"total_sessions"`
	TotalDefaulters      int64                 `json:"total_defaulters"`
	OverallAttendancePct float64               `json:"overall_attendance_pct"`
	PresentRecords       int64                 `json:"present_records"`
	AbsentRecords        int64                 `json:"absent_records"`
	LateRecords          int64                 `json:"late_records"`
	ExcusedRecords       int64                 `json:"excused_records"`
	TotalRecords         int64                 `json:"total_records"`
	ClassComparisons     []*ClassComparisonRow `json:"class_comparisons"`
}

type ClassComparisonRow struct {
	ClassID              uuid.UUID `json:"class_id"`
	ClassName            string    `json:"class_name"`
	GradeLevel           int16     `json:"grade_level"`
	Section              string    `json:"section"`
	TotalStudents        int64     `json:"total_students"`
	AttendancePercentage float64   `json:"attendance_percentage"`
}

func (r *AnalyticsRepo) SchoolOverview(ctx context.Context, schoolID uuid.UUID, classID *uuid.UUID, academicYearID *uuid.UUID, from, to time.Time, threshold float64) (*SchoolOverviewData, error) {
	out := &SchoolOverviewData{ClassComparisons: []*ClassComparisonRow{}}

	// Total students
	qStud := `SELECT COUNT(*) FROM students WHERE school_id=$1 AND is_active=true`
	argsStud := []any{schoolID}
	if classID != nil {
		qStud += ` AND class_id=$2`
		argsStud = append(argsStud, *classID)
	}
	_ = r.q(ctx).QueryRow(ctx, qStud, argsStud...).Scan(&out.TotalStudents)

	// Total classes
	qClasses := `SELECT COUNT(*) FROM classes WHERE school_id=$1`
	argsC := []any{schoolID}
	if classID != nil {
		qClasses += ` AND id=$2`
		argsC = append(argsC, *classID)
	} else if academicYearID != nil {
		qClasses += ` AND academic_year_id=$2`
		argsC = append(argsC, *academicYearID)
	}
	_ = r.q(ctx).QueryRow(ctx, qClasses, argsC...).Scan(&out.TotalClasses)

	// Total sessions
	qSess := `SELECT COUNT(DISTINCT id) FROM attendance_sessions WHERE school_id=$1 AND session_date BETWEEN $2 AND $3`
	argsSess := []any{schoolID, from, to}
	if classID != nil {
		qSess += ` AND class_id=$4`
		argsSess = append(argsSess, *classID)
	}
	_ = r.q(ctx).QueryRow(ctx, qSess, argsSess...).Scan(&out.TotalSessions)

	// Attendance records breakdown and overall attendance %
	qPct := `SELECT
	         COALESCE(COUNT(ar.id), 0) AS total_records,
	         COALESCE(COUNT(ar.id) FILTER (WHERE ar.status='present'), 0) AS present_records,
	         COALESCE(COUNT(ar.id) FILTER (WHERE ar.status='absent'), 0) AS absent_records,
	         COALESCE(COUNT(ar.id) FILTER (WHERE ar.status='late'), 0) AS late_records,
	         COALESCE(COUNT(ar.id) FILTER (WHERE ar.status='excused'), 0) AS excused_records,
	         COALESCE(ROUND(100.0 * COUNT(ar.id) FILTER (WHERE ar.status='present') / NULLIF(COUNT(ar.id), 0), 2), 0) AS overall_pct
	         FROM attendance_records ar
	         JOIN attendance_sessions s ON s.id=ar.session_id
	         WHERE ar.school_id=$1 AND s.session_date BETWEEN $2 AND $3`
	argsPct := []any{schoolID, from, to}
	if classID != nil {
		qPct += ` AND s.class_id=$4`
		argsPct = append(argsPct, *classID)
	}
	_ = r.q(ctx).QueryRow(ctx, qPct, argsPct...).Scan(
		&out.TotalRecords,
		&out.PresentRecords,
		&out.AbsentRecords,
		&out.LateRecords,
		&out.ExcusedRecords,
		&out.OverallAttendancePct,
	)

	// Class comparisons (join sessions correctly with session_date check)
	qComp := `SELECT
		c.id,
		CONCAT('Grade ', c.grade_level, '-', c.section) AS class_name,
		c.grade_level,
		c.section,
		COUNT(DISTINCT st.id) AS total_students,
		COALESCE(ROUND(100.0 * COUNT(ar.id) FILTER (WHERE ar.status='present') / NULLIF(COUNT(ar.id), 0), 2), 0) AS attendance_pct
	FROM classes c
	LEFT JOIN students st ON st.class_id=c.id AND st.is_active=true
	LEFT JOIN attendance_sessions s ON s.class_id=c.id AND s.session_date BETWEEN $2 AND $3
	LEFT JOIN attendance_records ar ON ar.session_id=s.id AND ar.student_id=st.id
	WHERE c.school_id=$1`
	argsComp := []any{schoolID, from, to}
	if classID != nil {
		qComp += ` AND c.id=$4`
		argsComp = append(argsComp, *classID)
	} else if academicYearID != nil {
		qComp += ` AND c.academic_year_id=$4`
		argsComp = append(argsComp, *academicYearID)
	}
	qComp += ` GROUP BY c.id, c.grade_level, c.section ORDER BY c.grade_level, c.section`

	rows, err := r.q(ctx).Query(ctx, qComp, argsComp...)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			row := &ClassComparisonRow{}
			if err := rows.Scan(&row.ClassID, &row.ClassName, &row.GradeLevel, &row.Section, &row.TotalStudents, &row.AttendancePercentage); err == nil {
				out.ClassComparisons = append(out.ClassComparisons, row)
			}
		}
	}

	// Total defaulters across school / class
	qDef := `SELECT COUNT(*) FROM (
		SELECT st.id, COALESCE(ROUND(100.0 * COUNT(ar.id) FILTER (WHERE ar.status='present') / NULLIF(COUNT(ar.id),0), 2), 0) AS pct
		FROM students st
		JOIN attendance_records ar ON ar.student_id=st.id
		JOIN attendance_sessions s ON s.id=ar.session_id
		WHERE st.school_id=$1 AND s.session_date BETWEEN $2 AND $3 AND st.is_active=true`
	argsDef := []any{schoolID, from, to, threshold}
	if classID != nil {
		qDef += ` AND st.class_id=$5`
		argsDef = append(argsDef, *classID)
	}
	qDef += ` GROUP BY st.id
		HAVING COALESCE(ROUND(100.0 * COUNT(ar.id) FILTER (WHERE ar.status='present') / NULLIF(COUNT(ar.id),0), 2), 0) < $4
	) t`
	_ = r.q(ctx).QueryRow(ctx, qDef, argsDef...).Scan(&out.TotalDefaulters)

	return out, nil
}

// SchoolDefaulterRow is a detailed defaulter record with class and guardian info.
type SchoolDefaulterRow struct {
	StudentID     uuid.UUID `json:"student_id"`
	FullName      string    `json:"full_name"`
	AdmissionNo   string    `json:"admission_number"`
	RollNumber    string    `json:"roll_number"`
	ClassName     string    `json:"class_name"`
	GuardianName  string    `json:"guardian_name"`
	GuardianPhone string    `json:"guardian_phone"`
	TotalSessions int64     `json:"total_sessions"`
	PresentCount  int64     `json:"present_count"`
	AttendancePct float64   `json:"attendance_pct"`
}

func (r *AnalyticsRepo) SchoolDefaulters(ctx context.Context, schoolID uuid.UUID, classID *uuid.UUID, from, to time.Time, threshold float64) ([]*SchoolDefaulterRow, error) {
	q := `SELECT
		st.id,
		st.full_name,
		st.admission_number,
		st.roll_number,
		CONCAT('Grade ', c.grade_level, '-', c.section) AS class_name,
		COALESCE(st.guardian_name, ''),
		COALESCE(st.guardian_phone, ''),
		COUNT(ar.id) AS total_sessions,
		COUNT(ar.id) FILTER (WHERE ar.status='present') AS present_count,
		COALESCE(ROUND(100.0 * COUNT(ar.id) FILTER (WHERE ar.status='present') / NULLIF(COUNT(ar.id),0), 2), 0) AS pct
	FROM students st
	JOIN classes c ON c.id=st.class_id
	JOIN attendance_records ar ON ar.student_id=st.id
	JOIN attendance_sessions s ON s.id=ar.session_id
	WHERE st.school_id=$1 AND s.session_date BETWEEN $2 AND $3 AND st.is_active=true`

	args := []any{schoolID, from, to, threshold}
	if classID != nil {
		q += ` AND st.class_id=$5`
		args = append(args, *classID)
	}

	q += ` GROUP BY st.id, st.full_name, st.admission_number, st.roll_number, c.grade_level, c.section, st.guardian_name, st.guardian_phone
	      HAVING COALESCE(ROUND(100.0 * COUNT(ar.id) FILTER (WHERE ar.status='present') / NULLIF(COUNT(ar.id),0), 2), 0) < $4
	      ORDER BY pct ASC, st.full_name ASC`

	rows, err := r.q(ctx).Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*SchoolDefaulterRow
	for rows.Next() {
		row := &SchoolDefaulterRow{}
		if err := rows.Scan(&row.StudentID, &row.FullName, &row.AdmissionNo, &row.RollNumber, &row.ClassName, &row.GuardianName, &row.GuardianPhone, &row.TotalSessions, &row.PresentCount, &row.AttendancePct); err == nil {
			out = append(out, row)
		}
	}
	return out, nil
}

// MonthlyMatrixStudent represents one row in the monthly attendance sheet.
type MonthlyMatrixStudent struct {
	StudentID    uuid.UUID         `json:"student_id"`
	AdmissionNo  string            `json:"admission_number"`
	RollNumber   string            `json:"roll_number"`
	FullName     string            `json:"full_name"`
	DailyStatus  map[string]string `json:"daily_status"` // "YYYY-MM-DD" -> "P", "A", "L", "E"
	TotalPresent int               `json:"total_present"`
	TotalAbsent  int               `json:"total_absent"`
	TotalLate    int               `json:"total_late"`
	TotalExcused int               `json:"total_excused"`
}

type MonthlyMatrixData struct {
	ClassID   uuid.UUID               `json:"class_id"`
	ClassName string                  `json:"class_name"`
	Year      int                     `json:"year"`
	Month     int                     `json:"month"`
	DaysInMonth int                   `json:"days_in_month"`
	Students  []*MonthlyMatrixStudent `json:"students"`
}

func (r *AnalyticsRepo) MonthlyMatrix(ctx context.Context, classID uuid.UUID, year, month int) (*MonthlyMatrixData, error) {
	startDate := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	endDate := startDate.AddDate(0, 1, -1)

	var className string
	_ = r.q(ctx).QueryRow(ctx, `SELECT CONCAT('Grade ', grade_level, '-', section) FROM classes WHERE id=$1`, classID).Scan(&className)

	out := &MonthlyMatrixData{
		ClassID:     classID,
		ClassName:   className,
		Year:        year,
		Month:       month,
		DaysInMonth: endDate.Day(),
		Students:    []*MonthlyMatrixStudent{},
	}

	stRows, err := r.q(ctx).Query(ctx, `
		SELECT st.id, st.admission_number, COALESCE(e.roll_number, st.roll_number), st.full_name
		FROM students st
		LEFT JOIN student_class_enrollments e ON e.student_id = st.id AND e.class_id = $1
		WHERE (e.class_id = $1 OR (e.class_id IS NULL AND st.class_id = $1)) AND st.is_active=true
		ORDER BY COALESCE(e.roll_number, st.roll_number)`, classID)
	if err != nil {
		return nil, err
	}
	defer stRows.Close()

	stMap := make(map[uuid.UUID]*MonthlyMatrixStudent)
	var stList []*MonthlyMatrixStudent
	for stRows.Next() {
		st := &MonthlyMatrixStudent{DailyStatus: make(map[string]string)}
		if err := stRows.Scan(&st.StudentID, &st.AdmissionNo, &st.RollNumber, &st.FullName); err == nil {
			stMap[st.StudentID] = st
			stList = append(stList, st)
		}
	}
	out.Students = stList

	if len(stList) == 0 {
		return out, nil
	}

	startDateStr := fmt.Sprintf("%04d-%02d-01", year, month)
	endDateStr := fmt.Sprintf("%04d-%02d-%02d", year, month, endDate.Day())

	// 2. Fetch daily attendance records in month
	recRows, err := r.q(ctx).Query(ctx, `
		SELECT ar.student_id, TO_CHAR(s.session_date, 'YYYY-MM-DD'), ar.status
		FROM attendance_records ar
		JOIN attendance_sessions s ON s.id=ar.session_id
		WHERE s.class_id=$1 AND s.session_date >= $2::date AND s.session_date <= $3::date`, classID, startDateStr, endDateStr)
	if err != nil {
		return nil, err
	}
	defer recRows.Close()

	for recRows.Next() {
		var sid uuid.UUID
		var dateStr, status string
		if err := recRows.Scan(&sid, &dateStr, &status); err == nil {
			if st, ok := stMap[sid]; ok {
				code := "P"
				switch strings.ToLower(strings.TrimSpace(status)) {
				case "absent":
					code = "A"
					st.TotalAbsent++
				case "late":
					code = "L"
					st.TotalLate++
				case "excused":
					code = "E"
					st.TotalExcused++
				default:
					code = "P"
					st.TotalPresent++
				}
				st.DailyStatus[dateStr] = code
			}
		}
	}

	return out, nil
}

// AuditLogRow represents one record modification in audit log.
type AuditLogRow struct {
	ID          uuid.UUID `json:"id"`
	StudentName string    `json:"student_name"`
	RollNumber  string    `json:"roll_number"`
	ClassName   string    `json:"class_name"`
	SessionDate time.Time `json:"session_date"`
	OldStatus   *string   `json:"old_status"`
	NewStatus   string    `json:"new_status"`
	ChangedByName string  `json:"changed_by_name"`
	ChangedAt   time.Time `json:"changed_at"`
}

func (r *AnalyticsRepo) AuditLogs(ctx context.Context, schoolID uuid.UUID, from, to time.Time) ([]*AuditLogRow, error) {
	q := `SELECT
		l.id,
		st.full_name,
		st.roll_number,
		CONCAT('Grade ', c.grade_level, '-', c.section),
		s.session_date,
		l.old_status::text,
		l.new_status::text,
		u.full_name,
		l.changed_at
	FROM attendance_audit_log l
	JOIN attendance_records ar ON ar.id=l.attendance_record_id
	JOIN attendance_sessions s ON s.id=ar.session_id
	JOIN students st ON st.id=ar.student_id
	JOIN classes c ON c.id=st.class_id
	JOIN users u ON u.id=l.changed_by
	WHERE l.school_id=$1 AND l.changed_at BETWEEN $2 AND $3
	ORDER BY l.changed_at DESC`

	rows, err := r.q(ctx).Query(ctx, q, schoolID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*AuditLogRow
	for rows.Next() {
		row := &AuditLogRow{}
		if err := rows.Scan(&row.ID, &row.StudentName, &row.RollNumber, &row.ClassName, &row.SessionDate, &row.OldStatus, &row.NewStatus, &row.ChangedByName, &row.ChangedAt); err == nil {
			out = append(out, row)
		}
	}
	return out, nil
}

