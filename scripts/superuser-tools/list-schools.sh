#!/usr/bin/env bash
# list-schools.sh — print an overview of every school on this backend:
# code, name, active status, and counts of admins/teachers/classes/students.
#
# Usage:
#   ./list-schools.sh              # all schools
#   ./list-schools.sh <school_code> # detail view for one school

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib.sh"

SCHOOL_CODE="${1:-}"

if [[ -z "$SCHOOL_CODE" ]]; then
  psql "$DB_URL" -v ON_ERROR_STOP=1 <<'SQL'
SELECT
  s.code,
  s.name,
  s.is_active,
  (SELECT COUNT(*) FROM users u WHERE u.school_id = s.id AND u.role = 'admin')   AS admins,
  (SELECT COUNT(*) FROM users u WHERE u.school_id = s.id AND u.role = 'teacher') AS teachers,
  (SELECT COUNT(*) FROM classes c WHERE c.school_id = s.id)                     AS classes,
  (SELECT COUNT(*) FROM students st WHERE st.school_id = s.id AND st.is_active) AS active_students,
  s.created_at::date AS created
FROM schools s
ORDER BY s.created_at DESC;
SQL
  exit 0
fi

# Detail view for one school
SCHOOL_ID=$(psql_scalar "SELECT id FROM schools WHERE code='${SCHOOL_CODE}'")
if [[ -z "$SCHOOL_ID" ]]; then
  echo "ERROR: no school found with code '$SCHOOL_CODE'." >&2
  exit 1
fi

echo "=== School: $SCHOOL_CODE ==="
psql_exec "SELECT id, name, code, is_active, created_at FROM schools WHERE id='${SCHOOL_ID}'"

echo ""
echo "=== Admins ==="
psql_tenant_exec "${SCHOOL_ID}" "SELECT full_name, email, is_active, created_at::date FROM users WHERE school_id='${SCHOOL_ID}' AND role='admin' ORDER BY created_at"

echo ""
echo "=== Teachers ==="
psql_tenant_exec "${SCHOOL_ID}" "SELECT full_name, email, is_active, created_at::date FROM users WHERE school_id='${SCHOOL_ID}' AND role='teacher' ORDER BY full_name"

echo ""
echo "=== Classes ==="
psql_tenant_exec "${SCHOOL_ID}" "SELECT grade_level, section, (SELECT COUNT(*) FROM students st WHERE st.class_id = c.id AND st.is_active) AS students FROM classes c WHERE school_id='${SCHOOL_ID}' ORDER BY grade_level, section"

echo ""
echo "=== Recent attendance activity (last 7 days) ==="
psql_tenant_exec "${SCHOOL_ID}" "SELECT session_date, COUNT(*) AS sessions FROM attendance_sessions WHERE school_id='${SCHOOL_ID}' AND session_date >= CURRENT_DATE - INTERVAL '7 days' GROUP BY session_date ORDER BY session_date DESC"
