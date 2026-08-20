#!/usr/bin/env bash
# update-admin.sh — update an admin's information (name, email, phone, password).
#
# Usage:
#   ./update-admin.sh <school_code> <admin_email> [--name "New Name"] [--email "new@email.com"] [--password "newpass"] [--phone "+15550199"]
#
# Example:
#   ./update-admin.sh oakridge-hs admin@oakridge.edu --name "Principal Reyes" --password "NewPass123!"

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib.sh"

SCHOOL_CODE="${1:-}"
ADMIN_EMAIL="${2:-}"
shift 2 || true

require_arg "school_code" "$SCHOOL_CODE"
require_arg "admin_email" "$ADMIN_EMAIL"

SCHOOL_ID=$(psql_scalar "SELECT id FROM schools WHERE code='${SCHOOL_CODE}'")
if [[ -z "$SCHOOL_ID" ]]; then
  echo "ERROR: no school found with code '$SCHOOL_CODE'." >&2
  exit 1
fi

USER_ID=$(psql_tenant_scalar "${SCHOOL_ID}" "SELECT id FROM users WHERE school_id='${SCHOOL_ID}' AND email='${ADMIN_EMAIL}' AND role='admin'")
if [[ -z "$USER_ID" ]]; then
  echo "ERROR: no admin found with email '$ADMIN_EMAIL' in school '$SCHOOL_CODE'." >&2
  exit 1
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

if [[ -z "$NEW_NAME" && -z "$NEW_EMAIL" && -z "$NEW_PASSWORD" && -z "$NEW_PHONE" ]]; then
  echo "ERROR: Provide at least one flag to update (--name, --email, --password, --phone)." >&2
  exit 1
fi

SET_CLAUSES=()

if [[ -n "$NEW_NAME" ]]; then
  SET_CLAUSES+=("full_name = '${NEW_NAME//\'/\'\'}'")
fi

if [[ -n "$NEW_EMAIL" ]]; then
  if ! is_valid_email "$NEW_EMAIL"; then
    echo "ERROR: '$NEW_EMAIL' doesn't look like a valid email." >&2
    exit 1
  fi
  EXISTING=$(psql_tenant_scalar "${SCHOOL_ID}" "SELECT COUNT(*) FROM users WHERE school_id='${SCHOOL_ID}' AND email='${NEW_EMAIL}' AND id != '${USER_ID}'")
  if [[ "$EXISTING" != "0" ]]; then
    echo "ERROR: email '$NEW_EMAIL' is already taken in this school." >&2
    exit 1
  fi
  SET_CLAUSES+=("email = '${NEW_EMAIL//\'/\'\'}'")
fi

REVOKE_TOKENS=false
if [[ -n "$NEW_PASSWORD" ]]; then
  ensure_pwtool
  HASH=$("$PWTOOL_BIN" hash "$NEW_PASSWORD")
  SET_CLAUSES+=("password_hash = '${HASH}'")
  REVOKE_TOKENS=true
fi

if [[ -n "$NEW_PHONE" ]]; then
  SET_CLAUSES+=("phone = '${NEW_PHONE//\'/\'\'}'")
fi

SET_CLAUSES+=("updated_at = now()")

# Join clauses with comma
IFS="," SET_SQL="${SET_CLAUSES[*]}"

psql "$DB_URL" -v ON_ERROR_STOP=1 <<SQL
BEGIN;
SET app.current_school_id = '${SCHOOL_ID}';
UPDATE users SET ${SET_SQL} WHERE id = '${USER_ID}';
SQL

if [[ "$REVOKE_TOKENS" == "true" ]]; then
  psql "$DB_URL" -v ON_ERROR_STOP=1 <<SQL
SET app.current_school_id = '${SCHOOL_ID}';
UPDATE refresh_tokens SET revoked_at = now() WHERE user_id = '${USER_ID}' AND revoked_at IS NULL;
SQL
fi

psql "$DB_URL" -v ON_ERROR_STOP=1 <<SQL
COMMIT;
SQL

echo ""
echo "✓ Admin user updated successfully for school '$SCHOOL_CODE'."
if [[ "$REVOKE_TOKENS" == "true" ]]; then
  echo "  Active refresh tokens revoked (forced re-login)."
fi
