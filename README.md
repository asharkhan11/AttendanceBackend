# Attendance Management Backend

Production-grade, multi-tenant REST API for school/college attendance management. Built with Go + PostgreSQL. One deployment serves any number of schools; data between schools is fully isolated via Postgres Row-Level Security.

---

## Features

- **Multi-tenancy** — pooled DB with per-row `school_id` + Postgres RLS. A bug in application filtering cannot leak another school's data.
- **Roles** — `admin` (school management, reporting) and `teacher` (attendance marking for own subjects). "Class teacher" is a teacher with an additional `class_teacher_assignments` row.
- **Attendance** — manual roll-call marking, single or bulk, subject-wise per class. Idempotent session creation. Immutable audit log on every status change.
- **Analytics** — class summary, student summary, defaulters list, time-series trend — all as JSON for the frontend to graph.
- **Reports** — CSV and PDF export of class and student attendance.
- **Auth** — argon2id password hashing, JWT access tokens (15 min), rotatable refresh tokens (30 days), revocable on logout and password reset.

---

## Quick Start (local dev)

### Prerequisites

- Go 1.22+
- PostgreSQL 16+ (running locally or via Docker)

### 1. Clone and configure

```bash
git clone <repo-url>
cd attendance-backend
cp .env.example .env
# Edit .env — at minimum set JWT_SECRET and DATABASE_URL
```

### 2. Set up the database

```bash
# Create the database and roles (run once as postgres superuser)
psql -U postgres -c "CREATE DATABASE attendance;"
psql -U postgres -c "CREATE USER attendance_owner WITH PASSWORD 'your-owner-pass' CREATEDB;"
psql -U postgres -c "GRANT ALL ON DATABASE attendance TO attendance_owner;"
psql -U postgres -d attendance -c "GRANT ALL ON SCHEMA public TO attendance_owner;"

# Run migrations (applies schema + RLS + attendance_app role)
MIGRATE_DATABASE_URL="postgres://attendance_owner:your-owner-pass@localhost/attendance?sslmode=disable" \
  make migrate-up

# Seed demo data
make seed
```

### 3. Run

```bash
make run
# Server starts on :8080
curl http://localhost:8080/healthz   # → {"status":"ok"}
curl http://localhost:8080/readyz    # → {"status":"ok"}
```

### 4. Test login

```bash
curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"school_code":"demo-school","email":"admin@demo-school.edu","password":"Admin@1234"}'
```

---

## Docker Compose (full stack)

```bash
# Start Postgres + API together
docker-compose -f deployments/docker-compose.yml up --build

# Stop
docker-compose -f deployments/docker-compose.yml down
```

The compose file mounts the migration SQL into Postgres's init directory, so the schema and seed data are applied automatically on first boot.

---

## Running Tests

```bash
make test
# or directly:
TEST_DATABASE_URL="postgres://attendance_app:app_secure_pass_123@localhost/attendance?sslmode=disable" \
  go test -v -count=1 -timeout 120s ./internal/tests/...
```

Tests run against a real Postgres instance (not mocks). The cross-tenant isolation tests are the most important — they verify RLS is actually blocking data leakage.

---

## Environment Variables

| Variable | Example | Required | Notes |
|---|---|---|---|
| `PORT` | `8080` | No | Defaults to `8080` |
| `ENV` | `development` | No | `production` enables JSON logs |
| `DATABASE_URL` | `postgres://attendance_app:pass@localhost/attendance?sslmode=disable` | **Yes** | Must be non-superuser (attendance_app) so RLS applies |
| `MIGRATE_DATABASE_URL` | `postgres://attendance_owner:pass@localhost/attendance?sslmode=disable` | For migrations only | Owner/superuser connection for schema changes |
| `JWT_SECRET` | `<32+ random bytes>` | **Yes** | Generate: `openssl rand -base64 32` |
| `JWT_ACCESS_TTL` | `15m` | No | Go duration string |
| `JWT_REFRESH_TTL` | `720h` | No | 30 days |
| `ARGON2_MEMORY_KB` | `65536` | No | 64MB — don't lower in production |
| `ARGON2_ITERATIONS` | `3` | No | |
| `ARGON2_PARALLELISM` | `2` | No | |
| `CORS_ALLOWED_ORIGINS` | `https://school-a.example.com,https://school-b.example.com` | No | One per school frontend |
| `RATE_LIMIT_LOGIN_PER_MIN` | `10` | No | Per-IP on login/refresh routes |

---

## Database Role Architecture

Two Postgres roles are used deliberately:

| Role | Purpose | RLS applies? |
|---|---|---|
| `attendance_owner` | Runs migrations, owns the schema | Yes (FORCE RLS), but only used for migrations — never for the API |
| `attendance_app` | Used by the running API | Yes — all tenant tables enforce RLS |

`FORCE ROW LEVEL SECURITY` is applied to every tenant table, which means even the table owner cannot bypass RLS without explicitly disabling it. This ensures that even a misconfigured connection string (pointing to the owner role instead of app role) cannot leak cross-tenant data at runtime.

The only RLS bypass in the codebase is `get_user_school_id(UUID)` — a narrow `SECURITY DEFINER` function used exclusively during token refresh to resolve a user's school_id before setting the tenant context. See `DECISIONS.md` for rationale.

---

## Adding a New School

School provisioning in v1 is a manual, operator-driven step (see HLD.md §10):

```bash
# 1. Insert the school and its first admin user via psql (as attendance_owner)
psql "$MIGRATE_DATABASE_URL" << 'SQL'
INSERT INTO schools(name, code) VALUES('New School Name', 'new-school-code');

-- Insert the academic year
INSERT INTO academic_years(school_id, name, start_date, end_date, is_current)
SELECT id, '2026-2027', '2026-06-01', '2027-03-31', true
FROM schools WHERE code = 'new-school-code';

-- Insert the first admin (password must be generated by your app's hash util or a helper script)
-- See scripts/seed.sql for the hash format
SQL

# 2. Communicate the school_code and initial password to the school admin out-of-band.
# 3. The admin then sets up subjects, classes, teachers, and students via the API.
```

---

## API Reference

Base path: `POST /api/v1`. All authenticated endpoints require `Authorization: Bearer <access_token>`.

See `api/openapi.yaml` for the full OpenAPI 3 spec (generated by `swaggo`).

### Auth
| Method | Path | Role | Description |
|---|---|---|---|
| POST | `/auth/login` | public | `{school_code, email, password}` → tokens |
| POST | `/auth/refresh` | public | `{refresh_token}` → new token pair |
| POST | `/auth/logout` | auth | Revoke refresh token |
| GET | `/me` | auth | Current user profile |
| GET | `/me/subject-assignments` | teacher | My class+subject assignments |
| GET | `/me/class-teacher-assignments` | teacher | Classes I'm class teacher of |

### Admin — Subjects / Classes / Teachers / Students / Assignments
Full CRUD on all five resources. See `LLD.md` §4 for the complete table.

### Attendance
| Method | Path | Role | Description |
|---|---|---|---|
| POST | `/attendance/sessions` | teacher/admin | Open session (idempotent) |
| GET | `/attendance/sessions/{id}` | teacher/admin | Session + roster with current marks |
| POST | `/attendance/sessions/{id}/mark` | teacher/admin | Bulk mark (upsert) |
| PATCH | `/attendance/sessions/{id}/mark/{student_id}` | teacher/admin | Single mark/update |
| POST | `/attendance/sessions/{id}/close` | teacher/admin | Lock session |
| POST | `/attendance/sessions/{id}/reopen` | admin | Unlock closed session |

### Analytics & Reports
| Path | Role | Description |
|---|---|---|
| `/analytics/class/{id}/summary?from=&to=` | admin / class-teacher | Attendance % per subject |
| `/analytics/class/{id}/defaulters?threshold=75&from=&to=` | admin / class-teacher | Students below threshold |
| `/analytics/class/{id}/trend?granularity=week&from=&to=` | admin / class-teacher | Time-series for charting |
| `/analytics/student/{id}/summary?from=&to=` | admin / class-teacher | Per-subject + daily log |
| `/reports/class/{id}/export?format=csv&from=&to=` | admin | Download CSV |
| `/reports/class/{id}/export?format=pdf&from=&to=` | admin | Download PDF |
| `/reports/student/{id}/export?format=pdf&from=&to=` | admin | Download PDF |

---

## Project Structure

```
attendance-backend/
├── cmd/
│   ├── api/main.go          # Binary entrypoint
│   └── migrate/main.go      # Migration runner
├── internal/
│   ├── config/              # Env var loading
│   ├── apperror/            # Typed errors + HTTP status mapping
│   ├── httpx/               # Response envelope, pagination helpers
│   ├── auth/                # JWT, argon2id, refresh tokens
│   ├── middleware/           # RequestID, Logger, Recoverer, CORS, RateLimit, JWTAuth, TenantContext, RBAC
│   ├── models/              # Domain types + context keys
│   ├── repository/          # Raw SQL queries (one file per resource)
│   ├── services/            # Business logic + ownership checks
│   ├── handlers/            # HTTP handlers (decode → service → encode)
│   ├── server/              # Chi router assembly
│   ├── db/migrations/       # golang-migrate .up.sql / .down.sql
│   └── tests/               # Integration tests against real DB
├── scripts/
│   ├── seed.sql             # Demo data (2 schools for isolation testing)
│   └── init_db.sql          # Docker init: creates attendance_app role
├── deployments/
│   ├── Dockerfile           # Multi-stage build
│   └── docker-compose.yml   # Postgres + API together
├── .env.example
├── Makefile
├── sqlc.yaml
├── DECISIONS.md             # All implementation decisions logged here
└── README.md
```

---

## Regenerating sqlc Code

If you add queries to `internal/db/queries/*.sql`:

```bash
make sqlc-generate
# requires: go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
```

Note: the current implementation uses hand-written repository functions rather than sqlc-generated code (sqlc binary was unavailable in the build environment). The `sqlc.yaml` file is present for future use. See `DECISIONS.md`.

---

## Seed Credentials (local dev only)

| School | school_code | Email | Password | Role |
|---|---|---|---|---|
| Demo High School | `demo-school` | `admin@demo-school.edu` | `Admin@1234` | admin |
| Demo High School | `demo-school` | `priya@demo-school.edu` | `Admin@1234` | teacher (CT of 10A, teaches Math in 10A+10B) |
| Demo High School | `demo-school` | `rahul@demo-school.edu` | `Admin@1234` | teacher (CT of 10B, teaches Science in 10A) |
| Second School | `school-b` | `admin@school-b.edu` | `Admin@1234` | admin |

**Change all passwords before using in any non-local environment.**
