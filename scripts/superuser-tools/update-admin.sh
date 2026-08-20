#!/usr/bin/env bash
# update-admin.sh — update an admin account (name, email, phone, password).
# Supports both interactive prompts (one-by-one) and CLI flags.
#
# Interactive Usage:
#   ./update-admin.sh
#
# CLI Usage:
#   ./update-admin.sh <school_code> <admin_email> [--name "New Name"] [--email "new@email.com"] [--password "newpass"] [--phone "+15550199"]

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib.sh"

echo "============================================================"
echo "  Update School Administrator Account"
echo "============================================================"

SCHOOL_CODE="${1:-}"
ADMIN_EMAIL="${2:-}"

if [[ -n "$SCHOOL_CODE" && "$SCHOOL_CODE" != --* ]]; then
  shift || true
else
  SCHOOL_CODE=""
fi

if [[ -n "$ADMIN_EMAIL" && "$ADMIN_EMAIL" != --* ]]; then
  shift || true
else
  ADMIN_EMAIL=""
fi

NEW_NAME=""
NEW_EMAIL=""
NEW_PASSWORD=""
NEW_PHONE=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --name)
      NEW_NAME="$2"
      shift 2
      ;;
    --email)
      NEW_EMAIL="$2"
      shift 2
      ;;
    --password)
      NEW_PASSWORD="$2"
      shift 2
      ;;
    --phone)
      NEW_PHONE="$2"
      shift 2
      ;;
    *)
      echo "ERROR: Unknown option '$1'" >&2
      exit 1
      ;;
  esac
done

if [[ -z "$SCHOOL_CODE" ]]; then
  echo
  prompt_required SCHOOL_CODE "Enter School Code"
fi

SCHOOL_ID=$(psql_scalar "SELECT id FROM schools WHERE code='${SCHOOL_CODE}'")
if [[ -z "$SCHOOL_ID" ]]; then
  echo "ERROR: No school found with code '$SCHOOL_CODE'." >&2
  exit 1
fi

if [[ -z "$ADMIN_EMAIL" ]]; then
  echo
  echo "Admins in '$SCHOOL_CODE':"
  psql_tenant_exec "${SCHOOL_ID}" "SELECT full_name, email, is_active FROM users WHERE school_id='${SCHOOL_ID}' AND role='admin'"
  echo
  prompt_required ADMIN_EMAIL "Enter Admin Email to update"
fi

USER_ID=$(psql_tenant_scalar "${SCHOOL_ID}" "SELECT id FROM users WHERE school_id='${SCHOOL_ID}' AND email='${ADMIN_EMAIL}' AND role='admin'")
if [[ -z "$USER_ID" ]]; then
  echo "ERROR: No admin found with email '$ADMIN_EMAIL' in school '$SCHOOL_CODE'." >&2
  exit 1
fi

CURRENT_NAME=$(psql_tenant_scalar "${SCHOOL_ID}" "SELECT full_name FROM users WHERE id='${USER_ID}'")
CURRENT_PHONE=$(psql_tenant_scalar "${SCHOOL_ID}" "SELECT COALESCE(phone, '(none)') FROM users WHERE id='${USER_ID}'")

echo
echo "Current Admin Details:"
echo "  Name:  $CURRENT_NAME"
echo "  Email: $ADMIN_EMAIL"
echo "  Phone: $CURRENT_PHONE"
echo

# If flags were not provided, prompt interactively
if [[ -z "$NEW_NAME" && -z "$NEW_EMAIL" && -z "$NEW_PASSWORD" && -z "$NEW_PHONE" ]]; then
  prompt_input NEW_NAME "Enter New Full Name (press Enter to keep current)" ""
  prompt_input NEW_EMAIL "Enter New Email Address (press Enter to keep current)" ""
  prompt_input NEW_PASSWORD "Enter New Password (press Enter to keep current)" ""
  prompt_input NEW_PHONE "Enter New Phone Number (press Enter to keep current)" ""
fi

if [[ -z "$NEW_NAME" && -z "$NEW_EMAIL" && -z "$NEW_PASSWORD" && -z "$NEW_PHONE" ]]; then
  echo "No changes requested. Exiting."
  exit 0
fi

SET_CLAUSES=()

if [[ -n "$NEW_NAME" ]]; then
  SET_CLAUSES+=("full_name = '${NEW_NAME//\'/\'\'}'")
fi

if [[ -n "$NEW_EMAIL" ]]; then
  if ! is_valid_email "$NEW_EMAIL"; then
    echo "ERROR: '$NEW_EMAIL' is not a valid email format." >&2
    exit 1
  fi
  EXISTING=$(psql_tenant_scalar "${SCHOOL_ID}" "SELECT COUNT(*) FROM users WHERE school_id='${SCHOOL_ID}' AND email='${NEW_EMAIL}' AND id != '${USER_ID}'")
  if [[ "$EXISTING" != "0" ]]; then
    echo "ERROR: Email '$NEW_EMAIL' is already taken in this school." >&2
    exit 1
  fi
  SET_CLAUSES+=("email = '${NEW_EMAIL//\'/\'\'}'")
fi

REVOKE_TOKENS=false
if [[ -n "$NEW_PASSWORD" ]]; then
  ensure_pwtool
  HASH=$("$PWTOOL_BIN" hash "$NEW_PASSWORD" | tr -d '\r')
  SET_CLAUSES+=("password_hash = '${HASH}'")
  REVOKE_TOKENS=true
fi

if [[ -n "$NEW_PHONE" ]]; then
  SET_CLAUSES+=("phone = '${NEW_PHONE//\'/\'\'}'")
fi

SET_CLAUSES+=("updated_at = now()")

IFS="," SET_SQL="${SET_CLAUSES[*]}"

REVOKE_SQL=""
if [[ "$REVOKE_TOKENS" == "true" ]]; then
  REVOKE_SQL="UPDATE refresh_tokens SET revoked_at = now() WHERE user_id = '${USER_ID}' AND revoked_at IS NULL;"
fi

db_psql <<SQL
BEGIN;
SET app.current_school_id = '${SCHOOL_ID}';
UPDATE users SET ${SET_SQL} WHERE id = '${USER_ID}';
${REVOKE_SQL}
COMMIT;
SQL

echo
echo "✓ Admin user updated successfully for school '$SCHOOL_CODE'."
if [[ "$REVOKE_TOKENS" == "true" ]]; then
  echo "  Active sessions revoked (forced re-login with new password)."
fi
echo
