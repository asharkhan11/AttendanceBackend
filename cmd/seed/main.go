package main

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var (
	schoolID     = uuid.MustParse("a0000000-0000-0000-0000-000000000001")
	pastYearID   = uuid.MustParse("a0000000-0000-0000-0000-000000000009")
	currYearID   = uuid.MustParse("a0000000-0000-0000-0000-000000000010")
	adminID      = uuid.MustParse("a0000000-0000-0000-0000-000000000020")
	passwordHash = "$argon2id$v=19$m=65536,t=3,p=2$c29tZXNhbHRzb21lc2FsdA$GZOZnVsuIl859JTA3EJ6HQbji+o9QcsazcWmDssypW4" // Admin@1234 / Teacher@1234
)

var firstNamesM = []string{"Aarav", "Rohan", "Kiran", "Arjun", "Aditya", "Vivek", "Siddharth", "Rahul", "Varun", "Yash", "Dev", "Ishaan", "Kabir", "Reyansh", "Vihaan", "Ayaan", "Dhruv", "Rudra", "Shriram", "Tejas"}
var firstNamesF = []string{"Diya", "Ananya", "Sneha", "Pooja", "Riya", "Kavya", "Isha", "Tara", "Meera", "Sanya", "Anushka", "Nisha", "Tanvi", "Shreya", "Aditi", "Priya", "Divya", "Neha", "Roshni", "Bhavna"}
var lastNames = []string{"Patel", "Sharma", "Reddy", "Singh", "Joshi", "Nair", "Verma", "Gupta", "Kumar", "Iyer", "Rao", "Deshmukh", "Chaudhary", "Bhat", "Mehta", "Agarwal", "Kulkarni", "Pillai", "Das", "Shah"}

func main() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339})

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" || dbURL == "postgres://postgres:postgres@localhost:5432/attendance?sslmode=disable" {
		for _, file := range []string{".env.local", ".env"} {
			if b, err := os.ReadFile(file); err == nil {
				for _, line := range strings.Split(string(b), "\n") {
					line = strings.TrimSpace(line)
					if strings.HasPrefix(line, "DATABASE_URL=") {
						val := strings.TrimPrefix(line, "DATABASE_URL=")
						val = strings.Trim(val, `"' `)
						val = strings.Replace(val, "@postgres:", "@localhost:", 1)
						dbURL = val
						break
					}
				}
			}
			if dbURL != "" && !strings.Contains(dbURL, "@postgres:") {
				break
			}
		}
	}
	if dbURL == "" {
		dbURL = "postgres://attendance_app:AttendanceApp2026@localhost:5432/attendance?sslmode=disable"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 600*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to database")
	}
	defer pool.Close()

	_, err = pool.Exec(ctx, fmt.Sprintf("SET app.current_school_id = '%s'", schoolID.String()))
	if err != nil {
		log.Fatal().Err(err).Msg("failed to set app.current_school_id for RLS")
	}

	log.Info().Msg("Starting database seed with multi-session mock data...")

	_, _ = pool.Exec(ctx, `ALTER TABLE attendance_sessions DROP COLUMN IF EXISTS subject_id CASCADE;`)
	_, _ = pool.Exec(ctx, `ALTER TABLE academic_years ADD COLUMN IF NOT EXISTS is_archived BOOLEAN NOT NULL DEFAULT false;`)
	_, _ = pool.Exec(ctx, `ALTER TABLE classes ADD COLUMN IF NOT EXISTS class_teacher_id UUID REFERENCES users(id) ON DELETE SET NULL;`)

	// Wipe existing demo data
	wipeSQL := `
	DO $$
	DECLARE
		v_school_id UUID := 'a0000000-0000-0000-0000-000000000001';
	BEGIN
		DELETE FROM attendance_audit_log  WHERE school_id = v_school_id;
		DELETE FROM attendance_records    WHERE school_id = v_school_id;
		DELETE FROM attendance_sessions   WHERE school_id = v_school_id;
		DELETE FROM students              WHERE school_id = v_school_id;
		IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'class_teacher_assignments') THEN
			EXECUTE 'DELETE FROM class_teacher_assignments WHERE school_id = ''' || v_school_id || '''';
		END IF;
		DELETE FROM classes               WHERE school_id = v_school_id;
		DELETE FROM academic_years        WHERE school_id = v_school_id;
		DELETE FROM refresh_tokens        WHERE user_id IN (SELECT id FROM users WHERE school_id = v_school_id);
		DELETE FROM users                 WHERE school_id = v_school_id;
		DELETE FROM schools               WHERE id = v_school_id;
	END $$;`

	if _, err := pool.Exec(ctx, wipeSQL); err != nil {
		log.Fatal().Err(err).Msg("failed to wipe existing seed data")
	}
	log.Info().Msg("Wiped existing seed data.")

	// Insert School
	_, err = pool.Exec(ctx, `INSERT INTO schools (id, name, code, is_active) VALUES ($1, $2, $3, true)`, schoolID, "Demo High School", "demo-school")
	if err != nil {
		log.Fatal().Err(err).Msg("failed to insert school")
	}

	// Insert Academic Years (2025-2026 archived and 2026-2027 current)
	var hasArchivedCol bool
	_ = pool.QueryRow(ctx, `SELECT EXISTS (
		SELECT 1 FROM information_schema.columns 
		WHERE table_name='academic_years' AND column_name='is_archived'
	)`).Scan(&hasArchivedCol)

	if hasArchivedCol {
		_, err = pool.Exec(ctx, `INSERT INTO academic_years (id, school_id, name, start_date, end_date, is_current, is_archived) VALUES
			($1, $2, '2025-2026', '2025-06-01', '2026-03-31', false, true),
			($3, $2, '2026-2027', '2026-06-01', '2027-03-31', true, false)`,
			pastYearID, schoolID, currYearID)
	} else {
		_, err = pool.Exec(ctx, `INSERT INTO academic_years (id, school_id, name, start_date, end_date, is_current) VALUES
			($1, $2, '2025-2026', '2025-06-01', '2026-03-31', false),
			($3, $2, '2026-2027', '2026-06-01', '2027-03-31', true)`,
			pastYearID, schoolID, currYearID)
	}
	if err != nil {
		log.Fatal().Err(err).Msg("failed to insert academic years")
	}

	// Insert Admin
	_, err = pool.Exec(ctx, `INSERT INTO users (id, school_id, role, full_name, email, password_hash, is_active) VALUES
		($1, $2, 'admin', 'School Admin', 'admin@demo-school.edu', $3, true)`, adminID, schoolID, passwordHash)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to insert admin user")
	}

	// Insert 10 Teachers
	teachers := []struct {
		ID    uuid.UUID
		Name  string
		Email string
		Phone string
	}{
		{uuid.MustParse("a0000000-0000-0000-0000-000000000021"), "Priya Sharma", "priya@demo-school.edu", "+91-9876543210"},
		{uuid.MustParse("a0000000-0000-0000-0000-000000000022"), "Rahul Mehta", "rahul@demo-school.edu", "+91-9876543211"},
		{uuid.MustParse("a0000000-0000-0000-0000-000000000023"), "Amit Verma", "amit@demo-school.edu", "+91-9876543212"},
		{uuid.MustParse("a0000000-0000-0000-0000-000000000024"), "Sunita Rao", "sunita@demo-school.edu", "+91-9876543213"},
		{uuid.MustParse("a0000000-0000-0000-0000-000000000025"), "Vikram Singh", "vikram@demo-school.edu", "+91-9876543214"},
		{uuid.MustParse("a0000000-0000-0000-0000-000000000026"), "Neha Gupta", "neha@demo-school.edu", "+91-9876543215"},
		{uuid.MustParse("a0000000-0000-0000-0000-000000000027"), "Rajesh Kulkarni", "rajesh@demo-school.edu", "+91-9876543216"},
		{uuid.MustParse("a0000000-0000-0000-0000-000000000028"), "Kavita Nair", "kavita@demo-school.edu", "+91-9876543217"},
		{uuid.MustParse("a0000000-0000-0000-0000-000000000029"), "Sanjay Bhat", "sanjay@demo-school.edu", "+91-9876543218"},
		{uuid.MustParse("a0000000-0000-0000-0000-00000000002a"), "Meenakshi Iyer", "meenakshi@demo-school.edu", "+91-9876543219"},
	}

	for _, t := range teachers {
		_, err = pool.Exec(ctx, `INSERT INTO users (id, school_id, role, full_name, email, phone, password_hash, is_active) VALUES ($1, $2, 'teacher', $3, $4, $5, $6, true)`,
			t.ID, schoolID, t.Name, t.Email, t.Phone, passwordHash)
		if err != nil {
			log.Fatal().Err(err).Msgf("failed to insert teacher %s", t.Email)
		}
	}
	log.Info().Msg("Inserted 10 teacher users.")

	// Insert Classes for BOTH 2025-2026 (pastYearID) and 2026-2027 (currYearID)
	type ClassConfig struct {
		ID        uuid.UUID
		YearID    uuid.UUID
		YearName  string
		Grade     int16
		Section   string
		TeacherID uuid.UUID
	}

	classes := []ClassConfig{
		// 2025-2026 Session (Archived) — past year classes
		{uuid.MustParse("a0000000-0000-0000-0000-000000000030"), pastYearID, "2025-2026", 9, "A", uuid.Nil},
		{uuid.MustParse("a0000000-0000-0000-0000-000000000031"), pastYearID, "2025-2026", 9, "B", uuid.Nil},
		{uuid.MustParse("a0000000-0000-0000-0000-000000000032"), pastYearID, "2025-2026", 10, "A", uuid.Nil},
		{uuid.MustParse("a0000000-0000-0000-0000-000000000033"), pastYearID, "2025-2026", 10, "B", uuid.Nil},
		{uuid.MustParse("a0000000-0000-0000-0000-000000000034"), pastYearID, "2025-2026", 11, "A", uuid.Nil},
		{uuid.MustParse("a0000000-0000-0000-0000-000000000035"), pastYearID, "2025-2026", 11, "B", uuid.Nil},
		{uuid.MustParse("a0000000-0000-0000-0000-000000000036"), pastYearID, "2025-2026", 12, "A", uuid.Nil},
		{uuid.MustParse("a0000000-0000-0000-0000-000000000037"), pastYearID, "2025-2026", 12, "B", uuid.Nil},

		// 2026-2027 Session (Current)
		{uuid.MustParse("a0000000-0000-0000-0000-000000000040"), currYearID, "2026-2027", 9, "A", teachers[0].ID},
		{uuid.MustParse("a0000000-0000-0000-0000-000000000041"), currYearID, "2026-2027", 9, "B", teachers[1].ID},
		{uuid.MustParse("a0000000-0000-0000-0000-000000000042"), currYearID, "2026-2027", 10, "A", teachers[2].ID},
		{uuid.MustParse("a0000000-0000-0000-0000-000000000043"), currYearID, "2026-2027", 10, "B", teachers[3].ID},
		{uuid.MustParse("a0000000-0000-0000-0000-000000000044"), currYearID, "2026-2027", 11, "A", teachers[4].ID},
		{uuid.MustParse("a0000000-0000-0000-0000-000000000045"), currYearID, "2026-2027", 11, "B", teachers[5].ID},
		{uuid.MustParse("a0000000-0000-0000-0000-000000000046"), currYearID, "2026-2027", 12, "A", teachers[6].ID},
		{uuid.MustParse("a0000000-0000-0000-0000-000000000047"), currYearID, "2026-2027", 12, "B", teachers[7].ID},
	}

	var hasCTA bool
	_ = pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'class_teacher_assignments')`).Scan(&hasCTA)

	var hasCTCol bool
	_ = pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'classes' AND column_name = 'class_teacher_id')`).Scan(&hasCTCol)

	for _, c := range classes {
		if hasCTCol && c.TeacherID != uuid.Nil {
			_, err = pool.Exec(ctx, `INSERT INTO classes (id, school_id, academic_year_id, grade_level, section, class_teacher_id) VALUES ($1, $2, $3, $4, $5, $6)`,
				c.ID, schoolID, c.YearID, c.Grade, c.Section, c.TeacherID)
		} else {
			_, err = pool.Exec(ctx, `INSERT INTO classes (id, school_id, academic_year_id, grade_level, section) VALUES ($1, $2, $3, $4, $5)`,
				c.ID, schoolID, c.YearID, c.Grade, c.Section)
		}
		if err != nil {
			log.Fatal().Err(err).Msgf("failed to insert class %d-%s for year %s", c.Grade, c.Section, c.YearName)
		}
		if hasCTA && c.TeacherID != uuid.Nil {
			_, err = pool.Exec(ctx, `INSERT INTO class_teacher_assignments (school_id, teacher_id, class_id) VALUES ($1, $2, $3)`,
				schoolID, c.TeacherID, c.ID)
			if err != nil {
				log.Warn().Err(err).Msgf("could not insert into class_teacher_assignments")
			}
		}
	}
	log.Info().Msg("Inserted 16 classes (8 per academic year) with assigned class teachers.")

	// Insert Students (30 per class = 480 students total across 16 classes)
	type StudentSeed struct {
		ID            uuid.UUID
		ClassID       uuid.UUID
		AdmissionNo   string
		RollNo        string
		FullName      string
		Gender        string
		DOB           time.Time
		GuardianName  string
		GuardianPhone string
		TargetRate    float64
	}

	var allStudents []StudentSeed
	admissionSeq := 1

	for _, c := range classes {
		for r := 1; r <= 30; r++ {
			var gender string
			var firstName string
			if r%2 == 1 {
				gender = "M"
				firstName = firstNamesM[(r-1)%len(firstNamesM)]
			} else {
				gender = "F"
				firstName = firstNamesF[(r-1)%len(firstNamesF)]
			}
			lastName := lastNames[(admissionSeq+r)%len(lastNames)]
			fullName := fmt.Sprintf("%s %s", firstName, lastName)

			yearPrefix := "2025"
			if c.YearName == "2026-2027" {
				yearPrefix = "2026"
			}
			admNo := fmt.Sprintf("ADM%s%03d", yearPrefix, admissionSeq)
			rollNo := fmt.Sprintf("%02d", r)
			dob := time.Date(2010-(int(c.Grade)-9), time.Month((r%12)+1), (r%28)+1, 0, 0, 0, 0, time.UTC)
			guardianName := fmt.Sprintf("%s %s", lastNames[(r*3)%len(lastNames)], lastName)
			guardianPhone := fmt.Sprintf("+91-98100%05d", admissionSeq)

			var targetRate float64
			if r <= 20 {
				targetRate = 0.91 + float64(r%7)*0.01
			} else if r <= 25 {
				targetRate = 0.79 + float64(r%6)*0.015
			} else {
				targetRate = 0.55 + float64(r%5)*0.035 // Defaulters (<75%)
			}

			st := StudentSeed{
				ID:            uuid.New(),
				ClassID:       c.ID,
				AdmissionNo:   admNo,
				RollNo:        rollNo,
				FullName:      fullName,
				Gender:        gender,
				DOB:           dob,
				GuardianName:  guardianName,
				GuardianPhone: guardianPhone,
				TargetRate:    targetRate,
			}
			allStudents = append(allStudents, st)
			admissionSeq++

			_, err = pool.Exec(ctx, `INSERT INTO students (id, school_id, class_id, admission_number, roll_number, full_name, gender, dob, guardian_name, guardian_phone, is_active) VALUES
				($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, true)`,
				st.ID, schoolID, st.ClassID, st.AdmissionNo, st.RollNo, st.FullName, st.Gender, st.DOB, st.GuardianName, st.GuardianPhone)
			if err != nil {
				log.Fatal().Err(err).Msgf("failed to insert student %s", st.FullName)
			}
		}
	}
	log.Info().Msgf("Inserted %d students across all 16 classes.", len(allStudents))

	// Pre-group students by class
	studentsByClass := make(map[uuid.UUID][]StudentSeed)
	for _, st := range allStudents {
		studentsByClass[st.ClassID] = append(studentsByClass[st.ClassID], st)
	}

	var sessionCount int

	rGen := rand.New(rand.NewSource(42))

	// Seed sessions for 2025-2026 (Past Year: Oct 2025 - Mar 2026)
	pastStart := time.Date(2025, 10, 1, 0, 0, 0, 0, time.UTC)
	pastEnd := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)

	// Seed sessions for 2026-2027 (Current Year: June 2026 - July 2026)
	currStart := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	currEnd := time.Date(2026, 7, 25, 0, 0, 0, 0, time.UTC)

	// Check if subject_id column exists in attendance_sessions
	var hasSubjectIDCol bool
	_ = pool.QueryRow(ctx, `SELECT EXISTS (
		SELECT 1 FROM information_schema.columns 
		WHERE table_name='attendance_sessions' AND column_name='subject_id'
	)`).Scan(&hasSubjectIDCol)

	var dummySubjectID uuid.UUID
	if hasSubjectIDCol {
		dummySubjectID = uuid.MustParse("a0000000-0000-0000-0000-000000000030")
		_, _ = pool.Exec(ctx, `INSERT INTO subjects (id, school_id, name, code) VALUES ($1, $2, 'General', 'GEN') ON CONFLICT DO NOTHING`, dummySubjectID, schoolID)
	}

	var recordRows [][]interface{}
	var auditRows [][]interface{}

	for _, c := range classes {
		var sDate, eDate time.Time
		if c.YearName == "2025-2026" {
			sDate = pastStart
			eDate = pastEnd
		} else {
			sDate = currStart
			eDate = currEnd
		}

		classStudents := studentsByClass[c.ID]

		for cur := sDate; !cur.After(eDate); cur = cur.AddDate(0, 0, 1) {
			if cur.Weekday() == time.Sunday || (c.YearName == "2025-2026" && cur.Weekday() == time.Saturday) {
				continue
			}

			sessionID := uuid.New()
			period := int16(1)
			nowTs := cur.Add(9 * time.Hour)

			teacherForSession := c.TeacherID
			if teacherForSession == uuid.Nil {
				teacherForSession = teachers[0].ID
			}

			sessStatus := "closed"
			if cur.Format("2006-01-02") == time.Now().Format("2006-01-02") {
				sessStatus = "open"
			}

			if hasSubjectIDCol {
				_, err = pool.Exec(ctx, `INSERT INTO attendance_sessions (id, school_id, class_id, subject_id, teacher_id, session_date, period_number, status, created_at, updated_at)
					VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
					sessionID, schoolID, c.ID, dummySubjectID, teacherForSession, cur, period, sessStatus, nowTs, nowTs)
			} else {
				_, err = pool.Exec(ctx, `INSERT INTO attendance_sessions (id, school_id, class_id, teacher_id, session_date, period_number, status, created_at, updated_at)
					VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
					sessionID, schoolID, c.ID, teacherForSession, cur, period, sessStatus, nowTs, nowTs)
			}
			if err != nil {
				log.Fatal().Err(err).Msgf("failed to insert session for class %s on %s", c.ID, cur.Format("2006-01-02"))
			}
			sessionCount++

			for _, st := range classStudents {
				recID := uuid.New()
				val := rGen.Float64()

				var status string
				var remarks *string

				if val < st.TargetRate {
					status = "present"
				} else if val < st.TargetRate+((1.0-st.TargetRate)*0.25) {
					status = "late"
					r := "Late by 10 mins"
					remarks = &r
				} else if val < st.TargetRate+((1.0-st.TargetRate)*0.40) {
					status = "excused"
					r := "Medical Leave"
					remarks = &r
				} else {
					status = "absent"
				}

				markedAt := cur.Add(8*time.Hour + time.Duration(rGen.Intn(30))*time.Minute)

				recordRows = append(recordRows, []interface{}{
					recID, schoolID, sessionID, st.ID, status, remarks, teacherForSession, markedAt, markedAt,
				})

				if rGen.Intn(60) == 0 && status == "present" {
					oldStat := "absent"
					auditRows = append(auditRows, []interface{}{
						uuid.New(), schoolID, recID, oldStat, status, adminID, markedAt.Add(2 * time.Hour),
					})
				}
			}
		}
	}

	log.Info().Msgf("Batch inserting %d Attendance Records...", len(recordRows))
	chunkSize := 500
	for i := 0; i < len(recordRows); i += chunkSize {
		end := i + chunkSize
		if end > len(recordRows) {
			end = len(recordRows)
		}
		chunk := recordRows[i:end]

		// Build multi-value INSERT query: INSERT INTO attendance_records (...) VALUES ($1,$2,...), ($10,$11,...)
		var sb strings.Builder
		sb.WriteString("INSERT INTO attendance_records (id, school_id, session_id, student_id, status, remarks, marked_by, marked_at, updated_at) VALUES ")

		args := make([]interface{}, 0, len(chunk)*9)
		for j, row := range chunk {
			if j > 0 {
				sb.WriteString(",")
			}
			base := j * 9
			sb.WriteString(fmt.Sprintf("($%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d)",
				base+1, base+2, base+3, base+4, base+5, base+6, base+7, base+8, base+9))
			args = append(args, row...)
		}

		if _, err := pool.Exec(ctx, sb.String(), args...); err != nil {
			log.Fatal().Err(err).Msgf("failed to batch insert attendance_records chunk %d-%d", i, end)
		}
	}

	if len(auditRows) > 0 {
		log.Info().Msgf("Batch inserting %d Audit Log entries...", len(auditRows))
		for i := 0; i < len(auditRows); i += chunkSize {
			end := i + chunkSize
			if end > len(auditRows) {
				end = len(auditRows)
			}
			chunk := auditRows[i:end]

			var sb strings.Builder
			sb.WriteString("INSERT INTO attendance_audit_log (id, school_id, attendance_record_id, old_status, new_status, changed_by, changed_at) VALUES ")

			args := make([]interface{}, 0, len(chunk)*7)
			for j, row := range chunk {
				if j > 0 {
					sb.WriteString(",")
				}
				base := j * 7
				sb.WriteString(fmt.Sprintf("($%d,$%d,$%d,$%d,$%d,$%d,$%d)",
					base+1, base+2, base+3, base+4, base+5, base+6, base+7))
				args = append(args, row...)
			}

			if _, err := pool.Exec(ctx, sb.String(), args...); err != nil {
				log.Warn().Err(err).Msgf("failed to batch insert audit log chunk %d-%d", i, end)
			}
		}
	}

	log.Info().Msgf("Successfully seeded DB with %d Sessions, %d Records, and %d Audit Log entries across both academic sessions!", sessionCount, len(recordRows), len(auditRows))
	log.Info().Msg("Seeding completed successfully! Login Credentials: Admin = admin@demo-school.edu / Admin@1234, Teacher = priya@demo-school.edu / Teacher@1234")
}
