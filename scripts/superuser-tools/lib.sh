#!/usr/bin/env bash
# scripts/admin-tools/lib.sh — shared helpers sourced by every script in this folder.
# Not meant to be run directly.

set -euo pipefail

# Resolve admin tools dir and optional repo root
ADMIN_TOOLS_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$ADMIN_TOOLS_DIR/../.." 2>/dev/null && pwd || echo "$ADMIN_TOOLS_DIR")"

# Load .env if present (checking ADMIN_TOOLS_DIR first, then REPO_ROOT),
# but do NOT override variables already set in the calling shell's environment.
ENV_FILE=""
if [[ -f "$ADMIN_TOOLS_DIR/.env" ]]; then
  ENV_FILE="$ADMIN_TOOLS_DIR/.env"
elif [[ -f "$REPO_ROOT/.env" ]]; then
  ENV_FILE="$REPO_ROOT/.env"
fi

if [[ -n "$ENV_FILE" ]]; then
  while IFS='=' read -r key value || [[ -n "$key" ]]; do
    # skip comments/blank lines
    [[ "$key" =~ ^[[:space:]]*# ]] && continue
    [[ -z "$key" ]] && continue
    key="$(echo "$key" | xargs)"
    if [[ -n "$key" && -z "${!key:-}" ]]; then
      export "$key=$value"
    fi
  done < "$ENV_FILE"
fi

# MIGRATE_DATABASE_URL (owner role) is used for admin scripts since they need
# cross-tenant visibility by design (e.g. listing all schools, provisioning
# new ones). Falls back to DATABASE_URL if not set.
#
# NOTE: In PostgreSQL RLS with `FORCE ROW LEVEL SECURITY`, table owners are also
# subject to RLS unless the role has `BYPASSRLS` or the session sets 
# `app.current_school_id`. Admin tools helper functions handle school-scoping 
# or run with BYPASSRLS privileges.
# Resolve connection URL for local operator scripts running on host machine:
# If MIGRATE_DATABASE_URL/DATABASE_URL point to internal docker host 'postgres:5432',
# substitute with host-exposed address 'localhost:5433'.
RAW_DB_URL="${MIGRATE_DATABASE_URL:-${DATABASE_URL:-postgres://attendance_owner:AttendanceOwner2026@localhost:5433/attendance?sslmode=disable}}"
DB_URL="${RAW_DB_URL//@postgres:5432\//@localhost:5433\/}"
DB_URL="${DB_URL//@postgres\//@localhost:5433\/}"

if [[ -z "$DB_URL" ]]; then
  echo "ERROR: Set DATABASE_URL or MIGRATE_DATABASE_URL (in .env or environment)." >&2
  echo "       Use an owner-role connection string (with BYPASSRLS or superuser permissions), e.g.:" >&2
  echo "       postgres://attendance_owner:PASSWORD@localhost:5433/attendance?sslmode=disable" >&2
  exit 1
fi

# Resolve location of pwtool binary (supports Linux/Ubuntu without extension and Windows .exe)
resolve_pwtool() {
  if [[ -n "${PWTOOL_BIN:-}" ]]; then
    if [[ -f "$PWTOOL_BIN" ]] || command -v "$PWTOOL_BIN" >/dev/null 2>&1; then
      return 0
    fi
  fi

  # Prefer .exe on Windows environments, extension-less binary on Linux (Ubuntu) / Unix
  if [[ "${OSTYPE:-}" == "msys" || "${OSTYPE:-}" == "cygwin" || "${OSTYPE:-}" == "win32" ]]; then
    local candidates=(
      "$ADMIN_TOOLS_DIR/pwtool.exe"
      "$ADMIN_TOOLS_DIR/pwtool"
      "$REPO_ROOT/bin/pwtool.exe"
      "$REPO_ROOT/bin/pwtool"
    )
  else
    local candidates=(
      "$ADMIN_TOOLS_DIR/pwtool"
      "$ADMIN_TOOLS_DIR/pwtool.exe"
      "$REPO_ROOT/bin/pwtool"
      "$REPO_ROOT/bin/pwtool.exe"
    )
  fi

  for candidate in "${candidates[@]}"; do
    if [[ -f "$candidate" ]]; then
      PWTOOL_BIN="$candidate"
      return 0
    fi
  done

  if command -v pwtool >/dev/null 2>&1; then
    PWTOOL_BIN="pwtool"
  elif command -v pwtool.exe >/dev/null 2>&1; then
    PWTOOL_BIN="pwtool.exe"
  else
    PWTOOL_BIN=""
  fi
}

resolve_pwtool

# Verify pwtool binary exists and is executable (does NOT attempt to build from source)
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
    echo "       Please place the 'pwtool' (or 'pwtool.exe') binary in '$ADMIN_TOOLS_DIR' or make it available in PATH." >&2
    exit 1
  fi
  # On Linux/Ubuntu, ensure executable permission is set on the binary
  if [[ -f "$PWTOOL_BIN" && ! -x "$PWTOOL_BIN" ]]; then
    chmod +x "$PWTOOL_BIN" 2>/dev/null || true
  fi
}

# psql_exec <sql>   — run a SQL statement/query, table output
psql_exec() {
  psql "$DB_URL" -v ON_ERROR_STOP=1 -c "$1"
}

# psql_tenant_exec <school_id> <sql> — run SQL statement/query with tenant context set
psql_tenant_exec() {
  local school_id="$1" sql="$2"
  psql "$DB_URL" -v ON_ERROR_STOP=1 -c "SET app.current_school_id = '${school_id}'; ${sql}"
}

# psql_file <path>  — run a .sql file
psql_file() {
  psql "$DB_URL" -v ON_ERROR_STOP=1 -f "$1"
}

# psql_scalar <sql> — run a query, print only the first column of the first row
psql_scalar() {
  psql "$DB_URL" -v ON_ERROR_STOP=1 -At -c "$1"
}

# psql_tenant_scalar <school_id> <sql> — run query with tenant context, print first column of first row
psql_tenant_scalar() {
  local school_id="$1" sql="$2"
  psql "$DB_URL" -v ON_ERROR_STOP=1 -At -c "SET app.current_school_id = '${school_id}'; ${sql}"
}

# confirm <prompt>  — ask y/N, exit 1 if not confirmed
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

# is_valid_school_code <code> — lowercase, digits, hyphens only
is_valid_school_code() {
  [[ "$1" =~ ^[a-z0-9][a-z0-9-]{2,49}$ ]]
}
