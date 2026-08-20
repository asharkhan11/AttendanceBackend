#!/usr/bin/env bash
# backup-db.sh — dump the production database directly to a timestamped local .sql.gz file.
# Supports interactive prompting and CLI argument for output folder.
#
# Usage:
#   ./backup-db.sh [output_dir]

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib.sh"

echo "============================================================"
echo "  Backup Database to Local Archive"
echo "============================================================"

DEFAULT_DIR="$SUPERUSER_TOOLS_DIR/backups"
OUT_DIR="${1:-}"

if [[ -z "$OUT_DIR" ]]; then
  echo
  prompt_input OUT_DIR "Enter output directory for backup" "$DEFAULT_DIR"
fi

mkdir -p "$OUT_DIR"

TIMESTAMP=$(date +%Y%m%d_%H%M%S)
OUT_FILE="$OUT_DIR/attendance_backup_${TIMESTAMP}.sql.gz"

echo
echo "Backing up database from $SSH_HOST ($CONTAINER_NAME)..."
echo "Output: $OUT_FILE"

if [[ "$EXEC_MODE" == "ssh" ]]; then
  ssh \
    -i "$SSH_KEY" \
    -o StrictHostKeyChecking=no \
    -o ConnectTimeout=15 \
    "$SSH_USER@$SSH_HOST" \
    "docker exec '$CONTAINER_NAME' pg_dump -U '$DB_USER' -d '$DB_NAME' --no-owner --no-privileges" | gzip > "$OUT_FILE"
else
  pg_dump "$DB_URL" --no-owner --no-privileges | gzip > "$OUT_FILE"
fi

if [[ -f "$OUT_FILE" && -s "$OUT_FILE" ]]; then
  SIZE=$(du -h "$OUT_FILE" 2>/dev/null | cut -f1 || ls -lh "$OUT_FILE" | awk '{print $5}')
  echo
  echo "✓ Backup completed successfully: $OUT_FILE ($SIZE)"
else
  echo
  echo "ERROR: Backup file was not created or is empty." >&2
  exit 1
fi
echo
