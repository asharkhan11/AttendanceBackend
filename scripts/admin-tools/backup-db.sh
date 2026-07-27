#!/usr/bin/env bash
# backup-db.sh — dump the full database to a timestamped .sql.gz file.
#
# Usage:
#   ./backup-db.sh [output_dir]     # defaults to ./backups

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib.sh"

OUT_DIR="${1:-$REPO_ROOT/backups}"
mkdir -p "$OUT_DIR"

TIMESTAMP=$(date +%Y%m%d_%H%M%S)
OUT_FILE="$OUT_DIR/attendance_backup_${TIMESTAMP}.sql.gz"

echo "Backing up database to $OUT_FILE ..."
pg_dump "$DB_URL" --no-owner --no-privileges | gzip > "$OUT_FILE"

SIZE=$(du -h "$OUT_FILE" | cut -f1)
echo "✓ Backup complete: $OUT_FILE ($SIZE)"
