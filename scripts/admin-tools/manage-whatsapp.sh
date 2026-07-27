#!/usr/bin/env bash
# manage-whatsapp.sh — enable/disable WhatsApp messaging and set monthly rate limits per school.
#
# Usage:
#   ./manage-whatsapp.sh <school_code> [--enable] [--disable] [--limit <number>] [--status]
#
# Examples:
#   ./manage-whatsapp.sh demo-school --enable
#   ./manage-whatsapp.sh demo-school --disable
#   ./manage-whatsapp.sh demo-school --limit 1500
#   ./manage-whatsapp.sh demo-school --enable --limit 2000
#   ./manage-whatsapp.sh demo-school --status

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib.sh"

SCHOOL_CODE="${1:-}"
shift || true

require_arg "school_code" "$SCHOOL_CODE"

SCHOOL_ID=$(psql_scalar "SELECT id FROM schools WHERE code='${SCHOOL_CODE}'")
if [[ -z "$SCHOOL_ID" ]]; then
  echo "ERROR: no school found with code '$SCHOOL_CODE'." >&2
  exit 1
fi

ACTION_ENABLE=false
ACTION_DISABLE=false
NEW_LIMIT=""
SHOW_STATUS=false

while [[ $# -gt 0 ]]; do
  case "$1" in
    --enable)
      ACTION_ENABLE=true
      shift
      ;;
    --disable)
      ACTION_DISABLE=true
      shift
      ;;
    --limit)
      NEW_LIMIT="$2"
      shift 2
      ;;
    --status)
      SHOW_STATUS=true
      shift
      ;;
    *)
      echo "ERROR: Unknown option '$1'" >&2
      echo "Usage: ./manage-whatsapp.sh <school_code> [--enable] [--disable] [--limit <number>] [--status]" >&2
      exit 1
      ;;
  esac
done

if [[ "$ACTION_ENABLE" == true && "$ACTION_DISABLE" == true ]]; then
  echo "ERROR: Cannot specify both --enable and --disable." >&2
  exit 1
fi

SET_CLAUSES=()

if [[ "$ACTION_ENABLE" == true ]]; then
  SET_CLAUSES+=("is_whatsapp_enabled = true")
elif [[ "$ACTION_DISABLE" == true ]]; then
  SET_CLAUSES+=("is_whatsapp_enabled = false")
fi

if [[ -n "$NEW_LIMIT" ]]; then
  if ! [[ "$NEW_LIMIT" =~ ^[0-9]+$ ]] || [[ "$NEW_LIMIT" -lt 0 ]]; then
    echo "ERROR: --limit must be a non-negative integer." >&2
    exit 1
  fi
  SET_CLAUSES+=("whatsapp_monthly_limit = ${NEW_LIMIT}")
fi

if [[ ${#SET_CLAUSES[@]} -gt 0 ]]; then
  SET_CLAUSES+=("updated_at = now()")
  IFS="," SET_SQL="${SET_CLAUSES[*]}"
  psql_exec "UPDATE schools SET ${SET_SQL} WHERE id='${SCHOOL_ID}'" > /dev/null
  echo "✓ WhatsApp settings updated for school '$SCHOOL_CODE'."
fi

# Query current status
INFO=$(psql_exec "
  SELECT 
    name, 
    code, 
    is_whatsapp_enabled AS enabled, 
    whatsapp_monthly_limit AS monthly_limit,
    (SELECT COUNT(*) FROM whatsapp_logs 
     WHERE school_id = '${SCHOOL_ID}' 
       AND status = 'sent' 
       AND created_at >= date_trunc('month', now()) 
       AND created_at < date_trunc('month', now()) + interval '1 month') AS current_month_sent
  FROM schools 
  WHERE id = '${SCHOOL_ID}'
")

echo ""
echo "=== School WhatsApp Configuration ==="
echo "$INFO"
