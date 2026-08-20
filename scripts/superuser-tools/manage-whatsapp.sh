#!/usr/bin/env bash
# manage-whatsapp.sh — enable/disable WhatsApp messaging and set monthly limits per school.
# Supports both interactive prompts and CLI flags.
#
# Interactive Usage:
#   ./manage-whatsapp.sh
#
# CLI Usage:
#   ./manage-whatsapp.sh <school_code> [--enable] [--disable] [--limit <number>] [--status]

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib.sh"

echo "============================================================"
echo "  Manage School WhatsApp Configuration"
echo "============================================================"

SCHOOL_CODE="${1:-}"
if [[ -n "$SCHOOL_CODE" && "$SCHOOL_CODE" != --* ]]; then
  shift || true
else
  SCHOOL_CODE=""
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

# If no action flags were passed, offer an interactive menu
if [[ "$ACTION_ENABLE" == false && "$ACTION_DISABLE" == false && -z "$NEW_LIMIT" && "$SHOW_STATUS" == false ]]; then
  echo
  echo "Current WhatsApp Settings for '$SCHOOL_CODE':"
  psql_exec "
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
  "
  echo
  echo "Select an action:"
  echo "  1) Enable WhatsApp messaging"
  echo "  2) Disable WhatsApp messaging"
  echo "  3) Update monthly message limit"
  echo "  4) Enable WhatsApp AND set monthly message limit"
  echo "  5) Done / Exit"
  read -r -p "Enter choice [1-5]: " CHOICE

  case "$CHOICE" in
    1)
      ACTION_ENABLE=true
      ;;
    2)
      ACTION_DISABLE=true
      ;;
    3)
      prompt_required NEW_LIMIT "Enter new monthly message limit (e.g., 2000)"
      ;;
    4)
      ACTION_ENABLE=true
      prompt_required NEW_LIMIT "Enter new monthly message limit (e.g., 2000)"
      ;;
    5|*)
      echo "Exiting."
      exit 0
      ;;
  esac
fi

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
  echo
  echo "✓ WhatsApp settings updated for school '$SCHOOL_CODE'."
fi

# Display updated status
echo
echo "=== Updated WhatsApp Status ==="
psql_exec "
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
"
echo
