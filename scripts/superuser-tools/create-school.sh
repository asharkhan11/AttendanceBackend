#!/usr/bin/env bash
# create-school.sh — provision a brand-new school and its initial admin user.
#
# Usage:
#   ./create-school.sh <school_code> <school_name> <admin_email> <admin_full_name> [admin_password] [admin_phone]
#
# Example:
#   ./create-school.sh oakridge-hs "Oakridge High School" admin@oakridge.edu "Oakridge Admin" "SecretPass123" "+1-555-0199"

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib.sh"

SCHOOL_CODE="${1:-}"
SCHOOL_NAME="${2:-}"
ADMIN_EMAIL="${3:-}"
ADMIN_NAME="${4:-}"
ADMIN_PASSWORD="${5:-}"
ADMIN_PHONE="${6:-}"

require_arg "school_code" "$SCHOOL_CODE"
require_arg "school_name" "$SCHOOL_NAME"
require_arg "admin_email" "$ADMIN_EMAIL"
require_arg "admin_full_name" "$ADMIN_NAME"

if ! is_valid_school_code "$SCHOOL_CODE"; then
  echo "ERROR: school_code must be lowercase letters/digits/hyphens, 3-50 chars (e.g. 'oakridge-hs')." >&2
  exit 1
fi
if ! is_valid_email "$ADMIN_EMAIL"; then
  echo "ERROR: '$ADMIN_EMAIL' doesn't look like a valid email." >&2
  exit 1
fi

# Reject if school_code already exists
EXISTING=$(psql_scalar "SELECT COUNT(*) FROM schools WHERE code='${SCHOOL_CODE}'")
if [[ "$EXISTING" != "0" ]]; then
  echo "ERROR: school_code '$SCHOOL_CODE' already exists." >&2
  exit 1
fi

ensure_pwtool
if [[ -z "$ADMIN_PASSWORD" ]]; then
  ADMIN_PASSWORD=$("$PWTOOL_BIN" genpass)
fi
ADMIN_HASH=$("$PWTOOL_BIN" hash "$ADMIN_PASSWORD")

PHONE_SQL="NULL"
if [[ -n "$ADMIN_PHONE" ]]; then
  PHONE_SQL="'${ADMIN_PHONE//\'/\'\'}'"
fi

YEAR_LABEL=$(date +%Y)-$(( $(date +%Y) + 1 ))
START_DATE=$(date +%Y-%m-%d)
END_DATE=$(date -d "+9 months" +%Y-%m-%d 2>/dev/null || date -v+9m +%Y-%m-%d)

echo "Creating school '$SCHOOL_NAME' ($SCHOOL_CODE)..."

psql "$DB_URL" -v ON_ERROR_STOP=1 <<SQL
BEGIN;

INSERT INTO schools (id, name, code)
VALUES (gen_random_uuid(), '${SCHOOL_NAME//\'/\'\'}', '${SCHOOL_CODE}')
RETURNING id AS school_id \gset

SET app.current_school_id = :'school_id';

INSERT INTO academic_years (id, school_id, name, start_date, end_date, is_current)
VALUES (gen_random_uuid(), :'school_id', '${YEAR_LABEL}', '${START_DATE}', '${END_DATE}', true);

INSERT INTO users (id, school_id, role, full_name, email, password_hash, phone)
VALUES (gen_random_uuid(), :'school_id', 'admin', '${ADMIN_NAME//\'/\'\'}', '${ADMIN_EMAIL}', '${ADMIN_HASH}', ${PHONE_SQL});

COMMIT;
SQL

echo ""
echo "✓ School environment & admin created successfully."
echo ""
echo "─────────────────────────────────────────────"
echo "  School code:    $SCHOOL_CODE"
echo "  Admin email:    $ADMIN_EMAIL"
echo "  Admin name:     $ADMIN_NAME"
echo "  Admin password: $ADMIN_PASSWORD"
if [[ -n "$ADMIN_PHONE" ]]; then
  echo "  Admin phone:    $ADMIN_PHONE"
fi
echo "─────────────────────────────────────────────"
echo ""
echo "⚠ Give the school_code + admin credentials to the school's admin."
