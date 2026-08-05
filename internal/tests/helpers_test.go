// Package tests contains integration tests that run against a real PostgreSQL instance.
// Tests use a shared local Postgres (not testcontainers, to avoid Docker-in-Docker issues).
// Run with: go test -v ./internal/tests/... -timeout 120s
package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/school/attendance-backend/internal/auth"
	"github.com/school/attendance-backend/internal/config"
	"github.com/school/attendance-backend/internal/repository"
	"github.com/school/attendance-backend/internal/server"
)

// testEnv holds shared infrastructure for a test run.
type testEnv struct {
	srv   *httptest.Server
	pool  *pgxpool.Pool
	cfg   *config.Config
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()

	// Use env var or fall back to local dev DB (superuser for setup)
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://attendance_app:app_secure_pass_123@localhost/attendance?sslmode=disable"
	}

	// Minimal config for tests
	os.Setenv("DATABASE_URL", dbURL)
	os.Setenv("JWT_SECRET", "test-secret-key-at-least-32-chars-long")
	os.Setenv("JWT_ACCESS_TTL", "15m")
	os.Setenv("JWT_REFRESH_TTL", "720h")
	os.Setenv("ARGON2_MEMORY_KB", "65536")
	os.Setenv("ARGON2_ITERATIONS", "3")
	os.Setenv("ARGON2_PARALLELISM", "2")
	os.Setenv("CORS_ALLOWED_ORIGINS", "http://localhost:3000")
	os.Setenv("RATE_LIMIT_LOGIN_PER_MIN", "1000")
	os.Setenv("PORT", "0")
	os.Setenv("ENV", "development")
	os.Setenv("ENABLE_WHATSAPP", "true")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := repository.Connect(ctx, dbURL, repository.PoolConfig{
		MaxConns:        cfg.DBMaxConns,
		MinConns:        cfg.DBMinConns,
		MaxConnIdleTime: cfg.DBMaxConnIdleTime,
		MaxConnLifetime: cfg.DBMaxConnLifetime,
	})
	if err != nil {
		t.Skipf("skipping integration tests: cannot connect to DB: %v", err)
	}

	router := server.NewRouter(cfg, pool)
	srv := httptest.NewServer(router)
	t.Cleanup(func() {
		srv.Close()
		pool.Close()
	})

	return &testEnv{srv: srv, pool: pool, cfg: cfg}
}

// ─── HTTP helpers ─────────────────────────────────────────────────────────────

func (e *testEnv) post(t *testing.T, path string, body any, token string) *http.Response {
	t.Helper()
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPost, e.srv.URL+path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	return resp
}

func (e *testEnv) get(t *testing.T, path, token string) *http.Response {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, e.srv.URL+path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	return resp
}

func (e *testEnv) patch(t *testing.T, path string, body any, token string) *http.Response {
	t.Helper()
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPatch, e.srv.URL+path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PATCH %s: %v", path, err)
	}
	return resp
}

func decode(t *testing.T, resp *http.Response, v any) {
	t.Helper()
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		t.Fatalf("decode response: %v", err)
	}
}

type envelope struct {
	Data  json.RawMessage `json:"data"`
	Meta  json.RawMessage `json:"meta"`
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// login performs POST /api/v1/auth/login and returns the access token.
func (e *testEnv) login(t *testing.T, schoolCode, email, password string) string {
	t.Helper()
	resp := e.post(t, "/api/v1/auth/login", map[string]string{
		"school_code": schoolCode,
		"email":       email,
		"password":    password,
	}, "")
	var env envelope
	decode(t, resp, &env)
	if env.Error != nil {
		t.Fatalf("login failed: %s — %s", env.Error.Code, env.Error.Message)
	}
	var loginData struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(env.Data, &loginData); err != nil {
		t.Fatalf("unmarshal login data: %v", err)
	}
	return loginData.AccessToken
}

// seedTestSchool inserts a minimal test school and returns its code + admin credentials.
// Uses the superuser DB connection to bypass RLS.
func seedTestSchool(t *testing.T, pool *pgxpool.Pool, suffix string) (schoolCode, adminEmail, adminPass string) {
	t.Helper()
	ctx := context.Background()

	passSvc := auth.NewPasswordService(65536, 3, 2)
	password := "TestAdmin@123"
	hash, err := passSvc.HashPassword(password)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	schoolCode = "test-school-" + suffix
	adminEmail = fmt.Sprintf("admin@%s.test", suffix)

	// Use superuser-level connection (pool connects as attendance_app which has RLS,
	// so we need to temporarily set the school_id config OR use raw SQL via the owner conn)
	// Since tests use attendance_app, we use set_config trick.
	// First create the school (no RLS on schools table)
	var schoolID string
	err = pool.QueryRow(ctx,
		`INSERT INTO schools(name, code) VALUES($1,$2) ON CONFLICT(code) DO UPDATE SET name=EXCLUDED.name RETURNING id`,
		"Test School "+suffix, schoolCode,
	).Scan(&schoolID)
	if err != nil {
		t.Fatalf("insert school: %v", err)
	}

	// Use a tx with school_id set to insert other records
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, "SELECT set_config('app.current_school_id',$1,true)", schoolID); err != nil {
		t.Fatalf("set config: %v", err)
	}

	// Academic year
	var yearID string
	err = tx.QueryRow(ctx,
		`INSERT INTO academic_years(school_id,name,start_date,end_date,is_current)
		 VALUES($1,'2026-2027','2026-06-01','2027-03-31',true)
		 ON CONFLICT(school_id,name) DO UPDATE SET name=EXCLUDED.name RETURNING id`, schoolID,
	).Scan(&yearID)
	if err != nil {
		t.Fatalf("insert year: %v", err)
	}

	// Admin user
	_, err = tx.Exec(ctx,
		`INSERT INTO users(school_id,role,full_name,email,password_hash)
		 VALUES($1,'admin','Test Admin',$2,$3)
		 ON CONFLICT(school_id,email) DO UPDATE SET password_hash=EXCLUDED.password_hash`,
		schoolID, adminEmail, hash,
	)
	if err != nil {
		t.Fatalf("insert admin: %v", err)
	}

	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit seed: %v", err)
	}

	t.Cleanup(func() {
		// Clean up test school (cascade deletes all child rows)
		_, _ = pool.Exec(ctx, `DELETE FROM schools WHERE code=$1`, schoolCode)
	})

	return schoolCode, adminEmail, password
}
