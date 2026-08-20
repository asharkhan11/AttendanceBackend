#!/usr/bin/env bash
# create-school.sh — provision a brand-new school and its initial admin user.
# Supports both interactive prompts (one-by-one) and CLI arguments.
#
# Interactive Usage:
#   ./create-school.sh
#
# CLI Usage:
#   ./create-school.sh <school_code> <school_name> <admin_email> <admin_full_name> [admin_password] [admin_phone]

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib.sh"

echo "============================================================"
echo "  Provision New Tenant School & Admin Account"
echo "============================================================"

SCHOOL_CODE="${1:-}"
SCHOOL_NAME="${2:-}"
ADMIN_EMAIL="${3:-}"
ADMIN_NAME="${4:-}"
ADMIN_PASSWORD="${5:-}"
ADMIN_PHONE="${6:-}"

# Interactive prompts if arguments are not supplied
if [[ -z "$SCHOOL_NAME" ]]; then
  echo
  prompt_required SCHOOL_NAME "Enter School Name (e.g., Green Valley High School)"
fi

if [[ -z "$SCHOOL_CODE" ]]; then
  # Auto-generate a suggested slug from school name
  SUGGESTED_CODE=$(echo "$SCHOOL_NAME" | tr '[:upper:]' '[:lower:]' | sed -E 's/[^a-z0-9]+/-/g' | sed -E 's/^-+|-+$//g' | cut -c1-30)
  prompt_input SCHOOL_CODE "Enter School Code (lowercase, digits, hyphens)" "$SUGGESTED_CODE"
fi

if [[ -z "$ADMIN_NAME" ]]; then
  prompt_required ADMIN_NAME "Enter Admin Full Name (e.g., Sarah Connor)"
fi

if [[ -z "$ADMIN_EMAIL" ]]; then
  prompt_required ADMIN_EMAIL "Enter Admin Email Address (e.g., admin@greenvalley.edu)"
fi

if [[ -z "$ADMIN_PASSWORD" ]]; then
  prompt_input ADMIN_PASSWORD "Enter Admin Password (press Enter to auto-generate a secure random password)" ""
fi

if [[ -z "$ADMIN_PHONE" ]]; then
  prompt_input ADMIN_PHONE "Enter Admin Phone Number (optional, e.g., +1-555-0199)" ""
fi

# Validation
if ! is_valid_school_code "$SCHOOL_CODE"; then
  echo "ERROR: school_code must be lowercase letters/digits/hyphens, 3-50 chars (e.g. 'green-valley-hs')." >&2
  exit 1
fi

if ! is_valid_email "$ADMIN_EMAIL"; then
  echo "ERROR: '$ADMIN_EMAIL' is not a valid email format." >&2
  exit 1
fi

# Check for existing school code
EXISTING=$(psql_scalar "SELECT COUNT(*) FROM schools WHERE code='${SCHOOL_CODE}'")
if [[ "$EXISTING" != "0" ]]; then
  echo "ERROR: school_code '$SCHOOL_CODE' already exists." >&2
  exit 1
fi

# Password Hashing locally using pwtool
ensure_pwtool
if [[ -z "$ADMIN_PASSWORD" ]]; then
  ADMIN_PASSWORD=$("$PWTOOL_BIN" genpass | tr -d '\r')
fi
ADMIN_HASH=$("$PWTOOL_BIN" hash "$ADMIN_PASSWORD" | tr -d '\r')

PHONE_SQL="NULL"
if [[ -n "$ADMIN_PHONE" ]]; then
  PHONE_SQL="'${ADMIN_PHONE//\'/\'\'}'"
fi

YEAR_LABEL="$(date +%Y)-$(( $(date +%Y) + 1 ))"
START_DATE="$(date +%Y-%m-%d)"
END_DATE="$(date -d "+9 months" +%Y-%m-%d 2>/dev/null || date -v+9m +%Y-%m-%d 2>/dev/null || echo "$(date +%Y)-12-31")"

echo
echo "Creating school '$SCHOOL_NAME' ($SCHOOL_CODE)..."

db_psql <<SQL
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

echo
echo "✓ School environment & admin created successfully."
echo "────────────────────────────────────────────────────────────"
echo "  School Name:    $SCHOOL_NAME"
echo "  School Code:    $SCHOOL_CODE"
echo "  Admin Email:    $ADMIN_EMAIL"
echo "  Admin Name:     $ADMIN_NAME"
echo "  Admin Password: $ADMIN_PASSWORD"
if [[ -n "$ADMIN_PHONE" ]]; then
  echo "  Admin Phone:    $ADMIN_PHONE"
fi
echo "────────────────────────────────────────────────────────────"
echo "⚠ Please share the school code and admin credentials with the school administrator."
echo
