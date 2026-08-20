#!/usr/bin/env bash
# update-school.sh — update school information (name, code, active status).
# Supports both interactive prompts (one-by-one) and CLI flags.
#
# Interactive Usage:
#   ./update-school.sh
#
# CLI Usage:
#   ./update-school.sh <current_school_code> [--name "New School Name"] [--code "new-school-code"] [--status "active|inactive"]

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib.sh"

echo "============================================================"
echo "  Update School Information"
echo "============================================================"

SCHOOL_CODE="${1:-}"
if [[ -n "$SCHOOL_CODE" && "$SCHOOL_CODE" != --* ]]; then
  shift || true
else
  SCHOOL_CODE=""
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

if [[ -z "$SCHOOL_CODE" ]]; then
  echo
  prompt_required SCHOOL_CODE "Enter School Code to update"
fi

SCHOOL_ID=$(psql_scalar "SELECT id FROM schools WHERE code='${SCHOOL_CODE}'")
if [[ -z "$SCHOOL_ID" ]]; then
  echo "ERROR: No school found with code '$SCHOOL_CODE'." >&2
  exit 1
fi

CURRENT_NAME=$(psql_scalar "SELECT name FROM schools WHERE id='${SCHOOL_ID}'")
CURRENT_STATUS=$(psql_scalar "SELECT CASE WHEN is_active THEN 'active' ELSE 'inactive' END FROM schools WHERE id='${SCHOOL_ID}'")

echo
echo "Current Settings for '$SCHOOL_CODE':"
echo "  Name:   $CURRENT_NAME"
echo "  Code:   $SCHOOL_CODE"
echo "  Status: $CURRENT_STATUS"
echo

# If flags were not provided, prompt interactively
if [[ -z "$NEW_NAME" && -z "$NEW_CODE" && -z "$NEW_STATUS" ]]; then
  prompt_input NEW_NAME "Enter New School Name (press Enter to keep current)" ""
  prompt_input NEW_CODE "Enter New School Code (press Enter to keep current)" ""
  prompt_input NEW_STATUS "Enter New Status [active/inactive] (press Enter to keep current)" ""
fi

if [[ -z "$NEW_NAME" && -z "$NEW_CODE" && -z "$NEW_STATUS" ]]; then
  echo "No changes requested. Exiting."
  exit 0
fi

SET_CLAUSES=()

if [[ -n "$NEW_NAME" ]]; then
  SET_CLAUSES+=("name = '${NEW_NAME//\'/\'\'}'")
fi

if [[ -n "$NEW_CODE" ]]; then
  if ! is_valid_school_code "$NEW_CODE"; then
    echo "ERROR: New school code must be lowercase letters/digits/hyphens, 3-50 chars." >&2
    exit 1
  fi
  EXISTING=$(psql_scalar "SELECT COUNT(*) FROM schools WHERE code='${NEW_CODE}' AND id != '${SCHOOL_ID}'")
  if [[ "$EXISTING" != "0" ]]; then
    echo "ERROR: School code '$NEW_CODE' is already taken." >&2
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
    echo "ERROR: Status must be 'active' or 'inactive'." >&2
    exit 1
  fi
fi

SET_CLAUSES+=("updated_at = now()")

IFS="," SET_SQL="${SET_CLAUSES[*]}"

psql_exec "UPDATE schools SET ${SET_SQL} WHERE id='${SCHOOL_ID}'" > /dev/null

echo
echo "✓ School '$SCHOOL_CODE' updated successfully."
echo
