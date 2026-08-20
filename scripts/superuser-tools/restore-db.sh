#!/usr/bin/env bash
# restore-db.sh — restore database from a local .sql.gz backup.
# DESTRUCTIVE: Confirms before proceeding.
#
# Usage:
#   ./restore-db.sh [path-to-backup.sql.gz]

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib.sh"

echo "============================================================"
echo "  Restore Database from Backup Archive"
echo "============================================================"

BACKUP_FILE="${1:-}"

if [[ -z "$BACKUP_FILE" ]]; then
  echo
  prompt_required BACKUP_FILE "Enter path to backup file (.sql.gz)"
fi

if [[ ! -f "$BACKUP_FILE" ]]; then
  echo "ERROR: File not found: $BACKUP_FILE" >&2
  exit 1
fi

echo
echo "This will restore data from:"
echo "  $BACKUP_FILE"
echo "into the target database at:"
if [[ "$EXEC_MODE" == "ssh" ]]; then
  echo "  $SSH_USER@$SSH_HOST (container: $CONTAINER_NAME, db: $DB_NAME)"
else
  echo "  $DB_URL"
fi
echo
echo "⚠ WARNING: Restoring will overwrite existing data in the database."
confirm "Are you sure you want to proceed with restore?"

echo
echo "Restoring database..."

if [[ "$EXEC_MODE" == "ssh" ]]; then
  gunzip -c "$BACKUP_FILE" \
    | grep -v -E '^(CREATE EXTENSION|COMMENT ON EXTENSION)' \
    | ssh \
        -i "$SSH_KEY" \
        -o StrictHostKeyChecking=no \
        -o ConnectTimeout=15 \
        "$SSH_USER@$SSH_HOST" \
        "docker exec -i '$CONTAINER_NAME' psql -U '$DB_USER' -d '$DB_NAME' -v ON_ERROR_STOP=1"
else
  gunzip -c "$BACKUP_FILE" \
    | grep -v -E '^(CREATE EXTENSION|COMMENT ON EXTENSION)' \
    | psql "$DB_URL" -v ON_ERROR_STOP=1
fi

echo
echo "✓ Database restore completed successfully."
echo
