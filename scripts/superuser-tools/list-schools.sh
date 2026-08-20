#!/usr/bin/env bash
# list-schools.sh — print an overview of schools on the platform.
# Supports interactive inspection and direct CLI arguments.
#
# Usage:
#   ./list-schools.sh
#   ./list-schools.sh <school_code>

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib.sh"

echo "============================================================"
echo "  Tenant Schools Overview"
echo "============================================================"

SCHOOL_CODE="${1:-}"

if [[ -z "$SCHOOL_CODE" ]]; then
  echo
  psql_exec "
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
"
  echo
  read -r -p "Enter school code to view details (or press Enter to finish): " SCHOOL_CODE
  SCHOOL_CODE="$(echo "$SCHOOL_CODE" | xargs)"
  if [[ -z "$SCHOOL_CODE" ]]; then
    exit 0
  fi
fi

# Detail view for specified school
SCHOOL_ID=$(psql_scalar "SELECT id FROM schools WHERE code='${SCHOOL_CODE}'")
if [[ -z "$SCHOOL_ID" ]]; then
  echo "ERROR: No school found with code '$SCHOOL_CODE'." >&2
  exit 1
fi

echo
echo "============================================================"
echo "  Details for School: $SCHOOL_CODE"
echo "============================================================"
psql_exec "SELECT id, name, code, is_active, is_whatsapp_enabled, whatsapp_monthly_limit, created_at FROM schools WHERE id='${SCHOOL_ID}'"

echo
echo "─── Admins ───"
psql_tenant_exec "${SCHOOL_ID}" "SELECT full_name, email, is_active, created_at::date FROM users WHERE school_id='${SCHOOL_ID}' AND role='admin' ORDER BY created_at"

echo
echo "─── Teachers ───"
psql_tenant_exec "${SCHOOL_ID}" "SELECT full_name, email, is_active, created_at::date FROM users WHERE school_id='${SCHOOL_ID}' AND role='teacher' ORDER BY full_name"

echo
echo "─── Classes ───"
psql_tenant_exec "${SCHOOL_ID}" "SELECT grade_level, section, (SELECT COUNT(*) FROM students st WHERE st.class_id = c.id AND st.is_active) AS active_students FROM classes c WHERE school_id='${SCHOOL_ID}' ORDER BY grade_level, section"

echo
echo "─── Recent Attendance Activity (Last 7 Days) ───"
psql_tenant_exec "${SCHOOL_ID}" "SELECT session_date, COUNT(*) AS sessions FROM attendance_sessions WHERE school_id='${SCHOOL_ID}' AND session_date >= CURRENT_DATE - INTERVAL '7 days' GROUP BY session_date ORDER BY session_date DESC"
echo
