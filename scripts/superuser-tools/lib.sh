#!/usr/bin/env bash
# scripts/superuser-tools/lib.sh — shared helpers sourced by every script in this folder.
# Not meant to be run directly.

set -euo pipefail

# Resolve superuser tools dir and optional repo root
SUPERUSER_TOOLS_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SUPERUSER_TOOLS_DIR/../.." 2>/dev/null && pwd || echo "$SUPERUSER_TOOLS_DIR")"

# Load .env if present (checking SUPERUSER_TOOLS_DIR first, then REPO_ROOT),
# but do NOT override variables already set in the calling shell's environment.
ENV_FILE=""
if [[ -f "$SUPERUSER_TOOLS_DIR/.env" ]]; then
  ENV_FILE="$SUPERUSER_TOOLS_DIR/.env"
elif [[ -f "$REPO_ROOT/.env" ]]; then
  ENV_FILE="$REPO_ROOT/.env"
fi

if [[ -n "$ENV_FILE" ]]; then
  while IFS='=' read -r key value || [[ -n "$key" ]]; do
    # skip comments/blank lines
    [[ "$key" =~ ^[[:space:]]*# ]] && continue
    [[ -z "$key" ]] && continue
    key="$(echo "$key" | xargs)"
    # strip carriage return if any
    value="${value%$'\r'}"
    if [[ -n "$key" && -z "${!key:-}" ]]; then
      export "$key=$value"
    fi
  done < "$ENV_FILE"
fi

# -----------------------------------------------------------------------------
# Remote SSH & Database Execution Engine Configuration
# -----------------------------------------------------------------------------

SSH_USER="${DB_SSH_USER:-${SERVER_USER:-ubuntu}}"
SSH_HOST="${DB_SSH_HOST:-${SERVER_HOST:-155.248.241.83}}"
CONTAINER_NAME="${DB_CONTAINER:-attendance-postgres}"
DB_USER="${POSTGRES_USER:-attendance_owner}"
DB_NAME="${POSTGRES_DB:-attendance}"

# Resolve SSH Key
resolve_ssh_key() {
  if [[ -n "${SSH_KEY:-}" && -f "$SSH_KEY" ]]; then
    return 0
  fi

  local candidates=(
    "/c/Users/Ashar/Desktop/OCI/keys/ssh-key-2026-06-29.key"
    "C:/Users/Ashar/Desktop/OCI/keys/ssh-key-2026-06-29.key"
    "$HOME/Desktop/OCI/keys/ssh-key-2026-06-29.key"
    "$HOME/.ssh/id_rsa"
    "$HOME/.ssh/id_ed25519"
  )

  for candidate in "${candidates[@]}"; do
    if [[ -f "$candidate" ]]; then
      SSH_KEY="$candidate"
      return 0
    fi
  done
  SSH_KEY=""
}

resolve_ssh_key

# Determine execution mode: 'ssh' (default for remote DB container) or 'local'
EXEC_MODE="${EXEC_MODE:-ssh}"
if [[ "$EXEC_MODE" == "ssh" && -z "$SSH_KEY" ]]; then
  # Fallback to local if no SSH key found and local psql might be available
  if command -v psql >/dev/null 2>&1; then
    EXEC_MODE="local"
  fi
fi

RAW_DB_URL="${MIGRATE_DATABASE_URL:-${DATABASE_URL:-postgres://attendance_owner:AttendanceOwner2026@localhost:5433/attendance?sslmode=disable}}"
DB_URL="${RAW_DB_URL//@postgres:5432\//@localhost:5433\/}"
DB_URL="${DB_URL//@postgres\//@localhost:5433\/}"

# -----------------------------------------------------------------------------
# Database Execution Helpers
# -----------------------------------------------------------------------------

# db_psql — executes SQL piped from stdin (or heredoc) against PostgreSQL
db_psql() {
  if [[ "$EXEC_MODE" == "ssh" ]]; then
    ssh \
      -i "$SSH_KEY" \
      -o StrictHostKeyChecking=no \
      -o ConnectTimeout=15 \
      "$SSH_USER@$SSH_HOST" \
      "docker exec -i '$CONTAINER_NAME' psql -U '$DB_USER' -d '$DB_NAME' -v ON_ERROR_STOP=1"
  else
    psql "$DB_URL" -v ON_ERROR_STOP=1
  fi
}

# psql_exec <sql> — run a SQL statement/query, formatted table output
psql_exec() {
  local sql="$1"
  if [[ "$EXEC_MODE" == "ssh" ]]; then
    echo "$sql" | ssh \
      -i "$SSH_KEY" \
      -o StrictHostKeyChecking=no \
      -o ConnectTimeout=15 \
      "$SSH_USER@$SSH_HOST" \
      "docker exec -i '$CONTAINER_NAME' psql -U '$DB_USER' -d '$DB_NAME' -v ON_ERROR_STOP=1"
  else
    psql "$DB_URL" -v ON_ERROR_STOP=1 -c "$sql"
  fi
}

# psql_tenant_exec <school_id> <sql> — run SQL with tenant context set
psql_tenant_exec() {
  local school_id="$1" sql="$2"
  psql_exec "SET app.current_school_id = '${school_id}'; ${sql}"
}

# psql_scalar <sql> — run a query, print only the first column of the first row
psql_scalar() {
  local sql="$1"
  if [[ "$EXEC_MODE" == "ssh" ]]; then
    echo "$sql" | ssh \
      -i "$SSH_KEY" \
      -o StrictHostKeyChecking=no \
      -o ConnectTimeout=15 \
      "$SSH_USER@$SSH_HOST" \
      "docker exec -i '$CONTAINER_NAME' psql -U '$DB_USER' -d '$DB_NAME' -At -v ON_ERROR_STOP=1" | tr -d '\r'
  else
    psql "$DB_URL" -v ON_ERROR_STOP=1 -At -c "$sql" | tr -d '\r'
  fi
}

# psql_tenant_scalar <school_id> <sql> — run tenant-scoped query, print scalar
psql_tenant_scalar() {
  local school_id="$1" sql="$2"
  psql_scalar "SET app.current_school_id = '${school_id}'; ${sql}" | grep -v '^SET$' | head -n 1
}

# psql_file <path> — run a .sql file
psql_file() {
  local file="$1"
  if [[ "$EXEC_MODE" == "ssh" ]]; then
    cat "$file" | db_psql
  else
    psql "$DB_URL" -v ON_ERROR_STOP=1 -f "$file"
  fi
}

# -----------------------------------------------------------------------------
# Password Tool (pwtool) Resolution
# -----------------------------------------------------------------------------

resolve_pwtool() {
  if [[ -n "${PWTOOL_BIN:-}" ]]; then
    if [[ -f "$PWTOOL_BIN" ]] || command -v "$PWTOOL_BIN" >/dev/null 2>&1; then
      return 0
    fi
  fi

  local candidates=(
    "$REPO_ROOT/bin/pwtool.exe"
    "$REPO_ROOT/bin/pwtool"
    "$SUPERUSER_TOOLS_DIR/pwtool.exe"
    "$SUPERUSER_TOOLS_DIR/pwtool"
    "$REPO_ROOT/bin/pwtool-linux"
  )

  for candidate in "${candidates[@]}"; do
    if [[ -f "$candidate" ]]; then
      PWTOOL_BIN="$candidate"
      return 0
    fi
  done

  if command -v pwtool.exe >/dev/null 2>&1; then
    PWTOOL_BIN="pwtool.exe"
  elif command -v pwtool >/dev/null 2>&1; then
    PWTOOL_BIN="pwtool"
  else
    PWTOOL_BIN=""
  fi
}

resolve_pwtool

ensure_pwtool() {
  resolve_pwtool
  local valid=false
  if [[ -n "$PWTOOL_BIN" ]]; then
    if [[ -f "$PWTOOL_BIN" ]] || command -v "$PWTOOL_BIN" >/dev/null 2>&1; then
      valid=true
    fi
  fi

  if [[ "$valid" == "false" ]]; then
    echo "ERROR: 'pwtool' binary not found." >&2
    echo "       Please build pwtool locally (run 'make build' or 'go build -o bin/pwtool.exe ./cmd/pwtool/')" >&2
    exit 1
  fi

  if [[ -f "$PWTOOL_BIN" && ! -x "$PWTOOL_BIN" ]]; then
    chmod +x "$PWTOOL_BIN" 2>/dev/null || true
  fi
}

# -----------------------------------------------------------------------------
# Interactive Prompt Helpers
# -----------------------------------------------------------------------------

# prompt_input <var_name> <prompt_message> [default_value]
prompt_input() {
  local var_name="$1"
  local prompt_msg="$2"
  local default_val="${3:-}"
  local val="${!var_name:-}"

  if [[ -n "$val" ]]; then
    return 0
  fi

  local display_prompt="$prompt_msg"
  if [[ -n "$default_val" ]]; then
    display_prompt="$display_prompt [$default_val]"
  fi
  display_prompt="$display_prompt: "

  read -r -p "$display_prompt" input_val
  input_val="$(echo "$input_val" | xargs)"

  if [[ -z "$input_val" && -n "$default_val" ]]; then
    input_val="$default_val"
  fi

  printf -v "$var_name" "%s" "$input_val"
}

# prompt_required <var_name> <prompt_message>
prompt_required() {
  local var_name="$1"
  local prompt_msg="$2"
  local val="${!var_name:-}"

  while [[ -z "$val" ]]; do
    read -r -p "$prompt_msg: " val
    val="$(echo "$val" | xargs)"
    if [[ -z "$val" ]]; then
      echo "  ⚠ This field is required. Please provide a value." >&2
    fi
  done

  printf -v "$var_name" "%s" "$val"
}

# confirm <prompt> — ask y/N, exit 1 if not confirmed
confirm() {
  local prompt="$1"
  read -r -p "$prompt [y/N] " reply
  case "$reply" in
    [yY][eE][sS]|[yY]) return 0 ;;
    *) echo "Aborted."; exit 1 ;;
  esac
}

# require_arg <name> <value> — fail with a clear message if value is empty
require_arg() {
  local name="$1" value="${2:-}"
  if [[ -z "$value" ]]; then
    echo "ERROR: $name is required." >&2
    exit 1
  fi
}

# is_valid_email <email>
is_valid_email() {
  [[ "$1" =~ ^[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}$ ]]
}

# is_valid_school_code <code> — lowercase, digits, hyphens only (3-50 chars)
is_valid_school_code() {
  [[ "$1" =~ ^[a-z0-9][a-z0-9-]{2,49}$ ]]
}
