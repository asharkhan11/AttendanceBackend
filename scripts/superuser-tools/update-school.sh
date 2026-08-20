#!/usr/bin/env bash
# update-school.sh — update school information (name, code, active status).
#
# Usage:
#   ./update-school.sh <current_school_code> [--name "New School Name"] [--code "new-school-code"] [--status "active|inactive"]
#
# Example:
#   ./update-school.sh oakridge-hs --name "Oakridge International School" --status "inactive"

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib.sh"

SCHOOL_CODE="${1:-}"
shift || true

require_arg "school_code" "$SCHOOL_CODE"

SCHOOL_ID=$(psql_scalar "SELECT id FROM schools WHERE code='${SCHOOL_CODE}'")
if [[ -z "$SCHOOL_ID" ]]; then
  echo "ERROR: no school found with code '$SCHOOL_CODE'." >&2
  exit 1
fi

NEW_NAME=""
NEW_CODE=""
NEW_STATUS=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --name)
      NEW_NAME="$2"
      shift 2
      ;;
    --code)
      NEW_CODE="$2"
      shift 2
      ;;
    --status)
      NEW_STATUS="$2"
      shift 2
      ;;
    *)
      echo "ERROR: Unknown option '$1'" >&2
      exit 1
      ;;
  esac
done

if [[ -z "$NEW_NAME" && -z "$NEW_CODE" && -z "$NEW_STATUS" ]]; then
  echo "ERROR: Provide at least one flag to update (--name, --code, --status)." >&2
  exit 1
fi

SET_CLAUSES=()

if [[ -n "$NEW_NAME" ]]; then
  SET_CLAUSES+=("name = '${NEW_NAME//\'/\'\'}'")
fi

if [[ -n "$NEW_CODE" ]]; then
  if ! is_valid_school_code "$NEW_CODE"; then
    echo "ERROR: new school code must be lowercase letters/digits/hyphens, 3-50 chars." >&2
    exit 1
  fi
  EXISTING=$(psql_scalar "SELECT COUNT(*) FROM schools WHERE code='${NEW_CODE}' AND id != '${SCHOOL_ID}'")
  if [[ "$EXISTING" != "0" ]]; then
    echo "ERROR: school_code '$NEW_CODE' already exists." >&2
    exit 1
  fi
  SET_CLAUSES+=("code = '${NEW_CODE}'")
fi

if [[ -n "$NEW_STATUS" ]]; then
  if [[ "$NEW_STATUS" == "active" ]]; then
    SET_CLAUSES+=("is_active = true")
  elif [[ "$NEW_STATUS" == "inactive" ]]; then
    SET_CLAUSES+=("is_active = false")
  else
    echo "ERROR: --status must be 'active' or 'inactive'." >&2
    exit 1
  fi
fi

SET_CLAUSES+=("updated_at = now()")

IFS="," SET_SQL="${SET_CLAUSES[*]}"

psql_exec "UPDATE schools SET ${SET_SQL} WHERE id='${SCHOOL_ID}'" > /dev/null

echo ""
echo "✓ School '$SCHOOL_CODE' updated successfully."
