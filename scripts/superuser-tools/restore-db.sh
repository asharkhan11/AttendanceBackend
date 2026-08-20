#!/usr/bin/env bash
# restore-db.sh — restore the database from a .sql.gz backup produced by backup-db.sh.
# DESTRUCTIVE: this will overwrite existing data. Confirms before proceeding.
#
# Usage:
#   ./restore-db.sh <path-to-backup.sql.gz>

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib.sh"

BACKUP_FILE="${1:-}"
require_arg "backup_file" "$BACKUP_FILE"

if [[ ! -f "$BACKUP_FILE" ]]; then
  echo "ERROR: file not found: $BACKUP_FILE" >&2
  exit 1
fi

echo "This will restore data from:"
echo "  $BACKUP_FILE"
echo "into the database at:"
echo "  $DB_URL"
echo ""
echo "Note: backup-db.sh produces a FULL dump (schema + data). The target"
echo "database should be EMPTY (freshly created, no migrations run) — this"
echo "restore will recreate the schema itself. Do not restore into a"
echo "database that already has the schema applied via 'make migrate-up'."
echo ""
confirm "This OVERWRITES existing data. Continue?"

echo "Restoring..."
# Strip CREATE EXTENSION / COMMENT ON EXTENSION statements: these require
# superuser privileges that attendance_owner intentionally does not have.
# Instead, ensure pgcrypto exists first via a superuser-run command before
# calling this script if restoring into a brand-new database — see README.
gunzip -c "$BACKUP_FILE" \
  | grep -v -E '^(CREATE EXTENSION|COMMENT ON EXTENSION)' \
  | psql "$DB_URL" -v ON_ERROR_STOP=1

echo "✓ Restore complete."
