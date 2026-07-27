package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── Auth Tests ───────────────────────────────────────────────────────────────

func TestLogin_ValidCredentials(t *testing.T) {
	e := newTestEnv(t)
	token := e.login(t, "demo-school", "admin@demo-school.edu", "Admin@1234")
	assert.NotEmpty(t, token, "expected non-empty access token")
}

func TestLogin_WrongPassword(t *testing.T) {
	e := newTestEnv(t)
	resp := e.post(t, "/api/v1/auth/login", map[string]string{
		"school_code": "demo-school",
		"email":       "admin@demo-school.edu",
		"password":    "wrongpassword",
	}, "")
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	var env envelope
	decode(t, resp, &env)
	require.NotNil(t, env.Error)
	assert.Equal(t, "UNAUTHORIZED", env.Error.Code)
}

func TestLogin_WrongSchoolCode(t *testing.T) {
	e := newTestEnv(t)
	resp := e.post(t, "/api/v1/auth/login", map[string]string{
		"school_code": "nonexistent-school",
		"email":       "admin@demo-school.edu",
		"password":    "Admin@1234",
	}, "")
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestLogin_MissingFields(t *testing.T) {
	e := newTestEnv(t)
	resp := e.post(t, "/api/v1/auth/login", map[string]string{
		"school_code": "demo-school",
	}, "")
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	var env envelope
	decode(t, resp, &env)
	assert.Equal(t, "VALIDATION_ERROR", env.Error.Code)
}

func TestInvalidToken_Returns401(t *testing.T) {
	e := newTestEnv(t)
	resp := e.get(t, "/api/v1/me", "not.a.valid.jwt")
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestMe_ReturnsCurrentUser(t *testing.T) {
	e := newTestEnv(t)
	token := e.login(t, "demo-school", "admin@demo-school.edu", "Admin@1234")
	resp := e.get(t, "/api/v1/me", token)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var env envelope
	decode(t, resp, &env)
	var user map[string]any
	require.NoError(t, json.Unmarshal(env.Data, &user))
	assert.Equal(t, "admin", user["role"])
	assert.Equal(t, "School Admin", user["full_name"])
}

func TestRefreshToken_Works(t *testing.T) {
	e := newTestEnv(t)
	resp := e.post(t, "/api/v1/auth/login", map[string]string{
		"school_code": "demo-school",
		"email":       "admin@demo-school.edu",
		"password":    "Admin@1234",
	}, "")
	var loginEnv envelope
	decode(t, resp, &loginEnv)
	var loginData struct {
		RefreshToken string `json:"refresh_token"`
	}
	require.NoError(t, json.Unmarshal(loginEnv.Data, &loginData))
	require.NotEmpty(t, loginData.RefreshToken)

	resp2 := e.post(t, "/api/v1/auth/refresh", map[string]string{
		"refresh_token": loginData.RefreshToken,
	}, "")
	assert.Equal(t, http.StatusOK, resp2.StatusCode)
	var refreshEnv envelope
	decode(t, resp2, &refreshEnv)
	var refreshData struct {
		AccessToken string `json:"access_token"`
	}
	require.NoError(t, json.Unmarshal(refreshEnv.Data, &refreshData))
	assert.NotEmpty(t, refreshData.AccessToken)
}

// ─── CRITICAL: Cross-Tenant Isolation Tests ───────────────────────────────────
// This is the most important test in the system.
// School A's token must NEVER see School B's data under any circumstance.

// ─── CRITICAL: Cross-Tenant Isolation Tests ───────────────────────────────────
// This is the most important test in the system.
// School A's token must NEVER see School B's data under any circumstance.

func TestCrossTenantIsolation_ClassesAreIsolated(t *testing.T) {
	e := newTestEnv(t)
	tokenA := e.login(t, "demo-school", "admin@demo-school.edu", "Admin@1234")
	tokenB := e.login(t, "school-b", "admin@school-b.edu", "Admin@1234")

	respA := e.get(t, "/api/v1/classes", tokenA)
	require.Equal(t, http.StatusOK, respA.StatusCode)

	respB := e.get(t, "/api/v1/classes", tokenB)
	require.Equal(t, http.StatusOK, respB.StatusCode)
	var envB envelope
	decode(t, respB, &envB)
	var metaB map[string]any
	require.NoError(t, json.Unmarshal(envB.Meta, &metaB))
	// School B has no classes
	assert.Equal(t, float64(0), metaB["total"], "School B should see 0 classes")
}

// ─── RBAC Tests ───────────────────────────────────────────────────────────────

func TestRBAC_TeacherCannotAccessAdminRoutes(t *testing.T) {
	e := newTestEnv(t)
	teacherToken := e.login(t, "demo-school", "priya@demo-school.edu", "Admin@1234")

	routes := []struct {
		method string
		path   string
	}{
		{"GET", "/api/v1/teachers"},
		{"POST", "/api/v1/subjects"},
		{"GET", "/api/v1/students"},
	}

	for _, rt := range routes {
		t.Run(fmt.Sprintf("%s %s", rt.method, rt.path), func(t *testing.T) {
			var resp *http.Response
			if rt.method == "GET" {
				resp = e.get(t, rt.path, teacherToken)
			} else {
				resp = e.post(t, rt.path, map[string]string{}, teacherToken)
			}
			assert.Equal(t, http.StatusForbidden, resp.StatusCode,
				"teacher should get 403 on admin-only route %s %s", rt.method, rt.path)
		})
	}
}

func TestRBAC_AdminCanAccessAllRoutes(t *testing.T) {
	e := newTestEnv(t)
	adminToken := e.login(t, "demo-school", "admin@demo-school.edu", "Admin@1234")
	resp := e.get(t, "/api/v1/teachers", adminToken)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestRBAC_UnauthenticatedCannotAccess(t *testing.T) {
	e := newTestEnv(t)
	routes := []string{"/api/v1/me", "/api/v1/subjects", "/api/v1/classes"}
	for _, path := range routes {
		resp := e.get(t, path, "") // no token
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode,
			"unauthenticated request to %s should return 401", path)
	}
}

// ─── Attendance Business Rules Tests ─────────────────────────────────────────

func TestAttendance_SessionIsIdempotent(t *testing.T) {
	e := newTestEnv(t)
	teacherToken := e.login(t, "demo-school", "priya@demo-school.edu", "Admin@1234")

	classID := "a0000000-0000-0000-0000-000000000040"
	date := time.Now().Format("2006-01-02")
	// Use a high period number to minimize collision risk across runs
	period := int(time.Now().UnixNano()%88) + 10 // 10-97

	body := map[string]any{
		"class_id":      classID,
		"session_date":  date,
		"period_number": period,
	}

	// First call → 201 (created) or 200 (already exists from a same-day re-run)
	resp1 := e.post(t, "/api/v1/attendance/sessions", body, teacherToken)
	assert.True(t,
		resp1.StatusCode == http.StatusCreated || resp1.StatusCode == http.StatusOK,
		"first call should return 201 or 200, got %d", resp1.StatusCode)
	var env1 envelope
	decode(t, resp1, &env1)
	var sess1 map[string]any
	require.NoError(t, json.Unmarshal(env1.Data, &sess1))
	id1 := sess1["id"].(string)

	// Second call with same tuple → always 200 (existing session returned)
	resp2 := e.post(t, "/api/v1/attendance/sessions", body, teacherToken)
	assert.Equal(t, http.StatusOK, resp2.StatusCode,
		"second call must return 200 (idempotent)")
	var env2 envelope
	decode(t, resp2, &env2)
	var sess2 map[string]any
	require.NoError(t, json.Unmarshal(env2.Data, &sess2))
	id2 := sess2["id"].(string)

	assert.Equal(t, id1, id2, "idempotent session create must return the same session ID")
}

func TestAttendance_BulkMarkAndAuditLog(t *testing.T) {
	e := newTestEnv(t)
	teacherToken := e.login(t, "demo-school", "priya@demo-school.edu", "Admin@1234")

	// Create session
	date := time.Now().AddDate(0, 0, -1).Format("2006-01-02") // yesterday to avoid date conflicts
	sessResp := e.post(t, "/api/v1/attendance/sessions", map[string]any{
		"class_id":      "a0000000-0000-0000-0000-000000000040",
		"session_date":  date,
		"period_number": 8,
	}, teacherToken)
	require.True(t, sessResp.StatusCode == http.StatusCreated || sessResp.StatusCode == http.StatusOK)
	var sessEnv envelope
	decode(t, sessResp, &sessEnv)
	var sessData map[string]any
	require.NoError(t, json.Unmarshal(sessEnv.Data, &sessData))
	sessionID := sessData["id"].(string)

	// Get roster to find real student IDs
	rosterResp := e.get(t, "/api/v1/attendance/sessions/"+sessionID, teacherToken)
	require.Equal(t, http.StatusOK, rosterResp.StatusCode)
	var rosterEnv envelope
	decode(t, rosterResp, &rosterEnv)
	var rosterData map[string]any
	require.NoError(t, json.Unmarshal(rosterEnv.Data, &rosterData))
	roster := rosterData["roster"].([]any)
	require.GreaterOrEqual(t, len(roster), 2)
	stu1ID := roster[0].(map[string]any)["student_id"].(string)
	stu2ID := roster[1].(map[string]any)["student_id"].(string)

	// Bulk mark
	markResp := e.post(t, "/api/v1/attendance/sessions/"+sessionID+"/mark", map[string]any{
		"marks": []map[string]any{
			{"student_id": stu1ID, "status": "present"},
			{"student_id": stu2ID, "status": "absent"},
		},
	}, teacherToken)
	require.Equal(t, http.StatusOK, markResp.StatusCode)

	// Update stu2 status absent→late → should write audit log
	updateResp := e.patch(t,
		fmt.Sprintf("/api/v1/attendance/sessions/%s/mark/%s", sessionID, stu2ID),
		map[string]string{"status": "late"},
		teacherToken,
	)
	require.Equal(t, http.StatusOK, updateResp.StatusCode)

	// Verify audit log in DB (must set tenant config since attendance_audit_log has RLS)
	ctx := context.Background()
	tx, err := e.pool.Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, "SELECT set_config('app.current_school_id','a0000000-0000-0000-0000-000000000001',true)")
	require.NoError(t, err)
	var auditCount int
	err = tx.QueryRow(ctx,
		`SELECT COUNT(*) FROM attendance_audit_log WHERE attendance_record_id IN (
			SELECT id FROM attendance_records WHERE session_id=$1 AND student_id=$2
		)`, sessionID, stu2ID,
	).Scan(&auditCount)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, auditCount, 1, "audit log must have at least one entry for the status change")
}

func TestAttendance_ClosedSessionRejectsMark(t *testing.T) {
	e := newTestEnv(t)
	teacherToken := e.login(t, "demo-school", "priya@demo-school.edu", "Admin@1234")

	date := time.Now().AddDate(0, 0, -2).Format("2006-01-02")
	sessResp := e.post(t, "/api/v1/attendance/sessions", map[string]any{
		"class_id":      "a0000000-0000-0000-0000-000000000040",
		"session_date":  date,
		"period_number": 7,
	}, teacherToken)
	var sessEnv envelope
	decode(t, sessResp, &sessEnv)
	var sessData map[string]any
	json.Unmarshal(sessEnv.Data, &sessData)
	sessionID := sessData["id"].(string)

	// Close session
	closeResp := e.post(t, "/api/v1/attendance/sessions/"+sessionID+"/close", nil, teacherToken)
	assert.Equal(t, http.StatusNoContent, closeResp.StatusCode)

	// Try to mark on closed session → must be 409
	markResp := e.post(t, "/api/v1/attendance/sessions/"+sessionID+"/mark", map[string]any{
		"marks": []map[string]any{
			{"student_id": "a0000000-0000-0000-0000-000000000001", "status": "present"},
		},
	}, teacherToken)
	assert.Equal(t, http.StatusConflict, markResp.StatusCode,
		"marking on a closed session must return 409 Conflict")
}

func TestAttendance_BulkMarkRejectsInvalidStudents(t *testing.T) {
	e := newTestEnv(t)
	teacherToken := e.login(t, "demo-school", "priya@demo-school.edu", "Admin@1234")

	date := time.Now().AddDate(0, 0, -3).Format("2006-01-02")
	sessResp := e.post(t, "/api/v1/attendance/sessions", map[string]any{
		"class_id":      "a0000000-0000-0000-0000-000000000040",
		"session_date":  date,
		"period_number": 6,
	}, teacherToken)
	var sessEnv envelope
	decode(t, sessResp, &sessEnv)
	var sessData map[string]any
	json.Unmarshal(sessEnv.Data, &sessData)
	sessionID := sessData["id"].(string)

	// Try bulk mark with a student UUID that doesn't belong to this class
	fakeStudentID := "ffffffff-ffff-ffff-ffff-ffffffffffff"
	markResp := e.post(t, "/api/v1/attendance/sessions/"+sessionID+"/mark", map[string]any{
		"marks": []map[string]any{
			{"student_id": fakeStudentID, "status": "present"},
		},
	}, teacherToken)
	assert.Equal(t, http.StatusBadRequest, markResp.StatusCode,
		"bulk mark with invalid student must return 400")
}

// ─── Health Check Tests ───────────────────────────────────────────────────────

func TestHealthz(t *testing.T) {
	e := newTestEnv(t)
	resp, err := http.Get(e.srv.URL + "/healthz")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestReadyz(t *testing.T) {
	e := newTestEnv(t)
	resp, err := http.Get(e.srv.URL + "/readyz")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}
