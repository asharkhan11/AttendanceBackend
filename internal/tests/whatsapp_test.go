package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/school/attendance-backend/internal/auth"
	"github.com/school/attendance-backend/internal/models"
	"github.com/school/attendance-backend/internal/repository"
	"github.com/school/attendance-backend/internal/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCleanPhoneNumber(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"+91 98765-43210", "919876543210"},
		{"9876543210", "9876543210"},
		{"+1 (555) 000-1234", "15550001234"},
		{" +44 20 7946 0958 ", "442079460958"},
	}

	for _, tt := range tests {
		got := services.CleanPhoneNumber(tt.input)
		assert.Equal(t, tt.expected, got, "input: %s", tt.input)
	}
}

func TestWhatsAppIntegration_FirstTimeFinalizeAndRateLimit(t *testing.T) {
	e := newTestEnv(t)
	ctx := context.Background()

	// Ensure migration 000008 schema columns and tables exist on test DB
	ensureWhatsAppSchema(t, e.pool)

	// 1. Seed a clean test school, academic year, admin & teacher
	suffix := fmt.Sprintf("wa-%d", time.Now().UnixNano())
	schoolCode, adminEmail, adminPass := seedTestSchool(t, e.pool, suffix)
	adminToken := e.login(t, schoolCode, adminEmail, adminPass)

	var schoolID string
	err := e.pool.QueryRow(ctx, "SELECT id FROM schools WHERE code = $1", schoolCode).Scan(&schoolID)
	require.NoError(t, err)

	// Create teacher user
	passSvc := auth.NewPasswordService(65536, 3, 2)
	teacherPass := "Teacher@1234"
	teacherHash, _ := passSvc.HashPassword(teacherPass)
	teacherEmail := fmt.Sprintf("teacher@%s.test", suffix)

	var teacherID string
	tx, err := e.pool.Begin(ctx)
	require.NoError(t, err)
	_, _ = tx.Exec(ctx, "SELECT set_config('app.current_school_id',$1,true)", schoolID)
	err = tx.QueryRow(ctx,
		`INSERT INTO users(school_id, role, full_name, email, password_hash)
		 VALUES($1, 'teacher', 'WA Teacher', $2, $3) RETURNING id`,
		schoolID, teacherEmail, teacherHash).Scan(&teacherID)
	require.NoError(t, err)

	// Get academic year
	var yearID string
	err = tx.QueryRow(ctx, "SELECT id FROM academic_years WHERE school_id = $1", schoolID).Scan(&yearID)
	require.NoError(t, err)

	// Create class
	var classID string
	err = tx.QueryRow(ctx,
		`INSERT INTO classes(school_id, academic_year_id, grade_level, section)
		 VALUES($1, $2, 10, 'A') RETURNING id`, schoolID, yearID).Scan(&classID)
	require.NoError(t, err)

	// Assign class teacher
	_, err = tx.Exec(ctx,
		`INSERT INTO class_teacher_assignments(school_id, teacher_id, class_id)
		 VALUES($1, $2, $3)`, schoolID, teacherID, classID)
	require.NoError(t, err)

	// Insert 2 students with guardian phones
	var stu1ID, stu2ID string
	err = tx.QueryRow(ctx,
		`INSERT INTO students(school_id, class_id, admission_number, roll_number, full_name, guardian_name, guardian_phone)
		 VALUES($1, $2, 'ADM001', '1', 'Alice Student', 'Guardian 1', '+91 9876543210') RETURNING id`,
		schoolID, classID).Scan(&stu1ID)
	require.NoError(t, err)

	err = tx.QueryRow(ctx,
		`INSERT INTO students(school_id, class_id, admission_number, roll_number, full_name, guardian_name, guardian_phone)
		 VALUES($1, $2, 'ADM002', '2', 'Bob Student', 'Guardian 2', '+91 9876543211') RETURNING id`,
		schoolID, classID).Scan(&stu2ID)
	require.NoError(t, err)

	require.NoError(t, tx.Commit(ctx))

	// Login as teacher
	teacherToken := e.login(t, schoolCode, teacherEmail, teacherPass)
	assert.NotEmpty(t, teacherToken)

	// 2. Enable WhatsApp for this test school with a monthly limit of 1
	_, err = e.pool.Exec(ctx, fmt.Sprintf("UPDATE schools SET is_whatsapp_enabled = true, whatsapp_monthly_limit = 1 WHERE id = '%s'", schoolID))
	require.NoError(t, err)

	// 3. Create session for class
	sessionDate := time.Now().Format("2006-01-02")
	sessResp := e.post(t, "/api/v1/attendance/sessions", map[string]any{
		"class_id":     classID,
		"session_date": sessionDate,
	}, teacherToken)
	require.Equal(t, http.StatusCreated, sessResp.StatusCode)

	var sessEnv envelope
	decode(t, sessResp, &sessEnv)
	var session models.AttendanceSession
	require.NoError(t, json.Unmarshal(sessEnv.Data, &session))
	sessionID := session.ID

	// 4. Mark both students absent for the FIRST time
	markResp := e.post(t, "/api/v1/attendance/sessions/"+sessionID.String()+"/mark", map[string]any{
		"marks": []map[string]string{
			{"student_id": stu1ID, "status": "absent"},
			{"student_id": stu2ID, "status": "absent"},
		},
	}, teacherToken)
	require.Equal(t, http.StatusOK, markResp.StatusCode)

	// Wait for async background worker
	time.Sleep(250 * time.Millisecond)

	// Verify session is marked as finalized
	parsedSchoolID, _ := uuid.Parse(schoolID)
	dbSession, err := repository.NewAttendanceRepo(repository.NewDB(e.pool)).GetSessionByID(context.Background(), sessionID)
	require.NoError(t, err)
	assert.True(t, dbSession.IsFinalized)

	// Check WhatsApp logs: 1 sent, 1 rate limited
	rows, err := e.pool.Query(ctx,
		fmt.Sprintf("SELECT status FROM whatsapp_logs WHERE school_id = '%s' AND session_id = '%s' ORDER BY created_at ASC", parsedSchoolID, sessionID))
	require.NoError(t, err)
	var statuses []string
	for rows.Next() {
		var st string
		require.NoError(t, rows.Scan(&st))
		statuses = append(statuses, st)
	}
	rows.Close()

	require.Len(t, statuses, 2)
	assert.Equal(t, "sent", statuses[0])
	assert.Equal(t, "skipped_rate_limit", statuses[1])

	// 5. Submitting attendance AGAIN (re-marking) MUST NOT trigger sending again
	markResp2 := e.post(t, "/api/v1/attendance/sessions/"+sessionID.String()+"/mark", map[string]any{
		"marks": []map[string]string{
			{"student_id": stu1ID, "status": "absent"},
			{"student_id": stu2ID, "status": "absent"},
		},
	}, teacherToken)
	require.Equal(t, http.StatusOK, markResp2.StatusCode)

	time.Sleep(150 * time.Millisecond)

	// Count logs for this session — must STILL be 2, because session was already finalized!
	var countAfterSecondMark int
	err = e.pool.QueryRow(ctx,
		fmt.Sprintf("SELECT COUNT(*) FROM whatsapp_logs WHERE session_id = '%s'", sessionID)).Scan(&countAfterSecondMark)
	require.NoError(t, err)
	assert.Equal(t, 2, countAfterSecondMark, "no new WhatsApp logs should be generated on subsequent marks")
	_ = adminToken
}
