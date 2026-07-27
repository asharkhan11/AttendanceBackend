# DECISIONS.md
## Ambiguity resolutions made during autonomous build

Per AGENT_BUILD_GUIDE.md §7, all implementation decisions not pinned down by HLD/LLD are recorded here.

---

**2026-07-03 — JSON field naming**
Chose `snake_case` for all JSON fields to match PostgreSQL column names. This makes the mapping from DB to response transparent and avoids a translation layer. Example: `full_name`, `school_id`, `created_at`.

**2026-07-03 — Timestamp format**
All timestamps serialised as RFC3339 (Go's `time.Time` JSON default). Date-only fields (session_date, dob, start_date, end_date) serialised as ISO 8601 `YYYY-MM-DD` strings in requests; stored as Postgres `DATE` columns.

**2026-07-03 — PDF library**
`github.com/jung-kurt/gofpdf` was specified in the build guide but requires `golang.org/x/image` which is blocked in this network. Replaced with `github.com/go-pdf/fpdf` — its actively maintained fork with the same API surface and no blocked dependencies. Decision recorded per §7.

**2026-07-03 — golang-migrate CLI**
The build guide calls for a `make migrate-up` target. Because the `migrate` binary cannot be downloaded (blocked network) and `go install go.uber.org/...` was also blocked, implemented migration as `cmd/migrate/main.go` (a small Go program). The Makefile target `make migrate-up` runs `go run ./cmd/migrate/ up`. This is functionally identical to the CLI and requires no external binary.

**2026-07-03 — `FORCE ROW LEVEL SECURITY`**
AGENT_BUILD_GUIDE.md §6 gives two options for ensuring RLS applies to the owner role. Chose `ALTER TABLE <t> FORCE ROW LEVEL SECURITY` (applied to every tenant table) over creating a separate migration role. This is simpler to operate and impossible to forget. Documented in README.

**2026-07-03 — Login RLS bypass**
`users` table has FORCE RLS, but login needs to look up a user before a tenant transaction exists. Resolved with a two-step approach: (1) open a short-lived transaction, (2) set `app.current_school_id` using the school_id resolved from `schools` (no RLS), (3) query `users`. This adds one extra round-trip only at login, acceptable cost for correctness.

**2026-07-03 — Refresh token school_id lookup**
Token refresh needs the user's school_id to set RLS config before querying the `users` table. Created a `SECURITY DEFINER` PostgreSQL function `get_user_school_id(UUID)` owned by the postgres superuser that returns `school_id` bypassing RLS. The function is narrowly scoped (read-only, returns one column) and is the only RLS bypass in the system.

**2026-07-03 — "Class teacher" analytics scoping**
LLD §3.4 states class teachers get class-scoped analytics for "their own class". Implemented: the `AnalyticsService.checkClassAccess` method verifies `class_teacher_assignments` for teacher role, bypasses for admin. This is checked in the service layer, not the router, because the router cannot know the class_id before the request body/path is parsed.

**2026-07-03 — Pagination defaults**
Default `page=1`, `page_size=20`, max `page_size=100`. Consistent across all list endpoints. Any invalid (non-numeric) value silently falls back to the default rather than returning a 400, consistent with common API conventions and reducing friction for frontend developers.

**2026-07-03 — DELETE subject with active assignments**
No explicit soft-delete on subjects. If a subject has `class_subjects`, `teacher_subject_assignments`, or `attendance_sessions` rows, Postgres will return a foreign key constraint error (500 → INTERNAL_ERROR in the response). A future improvement would catch this and return a descriptive 409. Not done in v1 to keep scope tight.

**2026-07-03 — Student bulk import error handling**
On CSV bulk import, rows with errors are skipped and counted; valid rows are still imported. The response includes `{"imported": N, "errors": [...]}`. This is more useful than an all-or-nothing transaction when importing hundreds of students, where one bad row would reject the whole batch. A strict-mode flag could be added later.

**2026-07-03 — Logout without tenant transaction**
`POST /auth/logout` revokes a refresh token. The `refresh_tokens` table has no RLS (intentional — it is not school-scoped; tokens are looked up purely by hash). So logout can call `pool.Exec` directly without a tenant transaction. This is the only write endpoint that doesn't go through TenantContext middleware.

**2026-07-03 — Test isolation strategy**
Integration tests run against the shared dev database (not isolated containers) for speed. Tests that create data use unique codes (timestamp-suffixed) to avoid collisions across runs. Seed data (demo-school, school-b) is pre-loaded and relied upon as stable test fixtures. Cleanup of created test data is handled in `t.Cleanup()` where possible.
