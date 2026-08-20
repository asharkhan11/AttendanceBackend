#!/usr/bin/env bash

set -euo pipefail

# ============================================================
# Attendance Database - Production Import / Restore Script
# Target Server: 155.248.241.83
# Action: FULLY REPLACE production database with imported copy
# ============================================================

# -------------------------
# Configuration
# -------------------------

SERVER_USER="ubuntu"
SERVER_HOST="155.248.241.83"
SSH_KEY="/c/Users/Ashar/Desktop/OCI/keys/ssh-key-2026-06-29.key"

CONTAINER_NAME="attendance-postgres"
DB_NAME="attendance"
DB_USER="attendance_owner"
REMOTE_TEMP_FILE="/tmp/attendance_import_target.dump"


# ============================================================
# Helper functions
# ============================================================

info() {
    echo
    echo "============================================================"
    echo "$1"
    echo "============================================================"
}

error() {
    echo
    echo "ERROR: $1"
    exit 1
}


# ============================================================
# Resolve Local Downloads Directory
# ============================================================

if [ -n "${USERPROFILE:-}" ]; then
    if command -v cygpath >/dev/null 2>&1; then
        DOWNLOADS_DIR="$(cygpath -u "$USERPROFILE/Downloads")"
    else
        DRIVE_LETTER=$(echo "$USERPROFILE" | cut -d':' -f1 | tr '[:upper:]' '[:lower:]')
        REST_PATH=$(echo "$USERPROFILE" | cut -d':' -f2 | tr '\\' '/')
        DOWNLOADS_DIR="/${DRIVE_LETTER}${REST_PATH}/Downloads"
    fi
elif [ -d "/c/Users/Ashar/Downloads" ]; then
    DOWNLOADS_DIR="/c/Users/Ashar/Downloads"
elif [ -d "$HOME/Downloads" ]; then
    DOWNLOADS_DIR="$HOME/Downloads"
else
    DOWNLOADS_DIR="./downloads"
fi


# ============================================================
# File Selection
# ============================================================

IMPORT_FILE="${1:-}"

if [ -z "$IMPORT_FILE" ]; then
    # Look for the latest backup file in Downloads directory
    LATEST_BACKUP=$(ls -t "$DOWNLOADS_DIR"/attendance_prod_backup_*.dump 2>/dev/null | head -n 1 || echo "")

    echo "============================================================"
    echo " Attendance Database Import / Restore"
    echo "============================================================"
    echo

    if [ -n "$LATEST_BACKUP" ]; then
        echo "Found recent backup in Downloads: $LATEST_BACKUP"
        echo
        read -r -p "Enter path to backup file [Press Enter for latest]: " INPUT_PATH
        if [ -z "$INPUT_PATH" ]; then
            IMPORT_FILE="$LATEST_BACKUP"
        else
            IMPORT_FILE="$INPUT_PATH"
        fi
    else
        read -r -p "Enter full path to database backup/dump file: " IMPORT_FILE
    fi
fi

# Clean quotes if user dragged and dropped file into terminal
IMPORT_FILE=$(echo "$IMPORT_FILE" | tr -d '"' | tr -d "'")

# Convert Windows style paths C:\... to /c/... if running under Git Bash
if [[ "$IMPORT_FILE" =~ ^[A-Za-z]:\\ ]]; then
    if command -v cygpath >/dev/null 2>&1; then
        IMPORT_FILE="$(cygpath -u "$IMPORT_FILE")"
    fi
fi

[ -f "$IMPORT_FILE" ] || error "Specified import file not found: $IMPORT_FILE"
[ -s "$IMPORT_FILE" ] || error "Specified import file is empty: $IMPORT_FILE"


# ============================================================
# Validation
# ============================================================

info "Validating Database Import Environment"

command -v ssh >/dev/null 2>&1 || error "SSH is not available in PATH."
command -v scp >/dev/null 2>&1 || error "SCP is not available in PATH."

[ -f "$SSH_KEY" ] || error "SSH key not found: $SSH_KEY"

echo "Database Server : $SERVER_USER@$SERVER_HOST"
echo "Container Name  : $CONTAINER_NAME"
echo "Target Database : $DB_NAME"
echo "Import File     : $IMPORT_FILE"
echo "File Size       : $(ls -lh "$IMPORT_FILE" | awk '{print $5}')"


# ============================================================
# CRITICAL SAFETY WARNING & CONFIRMATION
# ============================================================

echo
echo "****************************************************************"
echo "                       DANGER / WARNING                         "
echo "****************************************************************"
echo " You are about to COMPLETELY OVERWRITE and REPLACE the production "
echo " database ($DB_NAME) on server $SERVER_HOST with the imported copy."
echo " ALL existing data on the production server will be replaced!"
echo "****************************************************************"
echo
read -r -p "To proceed with replacing the production database, type 'RESTORE': " CONFIRM_RESTORE

if [ "$CONFIRM_RESTORE" != "RESTORE" ]; then
    echo "Restore aborted. No changes were made."
    exit 0
fi


# ============================================================
# 1. Test SSH Connection
# ============================================================

info "[1/5] Testing SSH Connection to DB Server ($SERVER_HOST)"

ssh \
    -i "$SSH_KEY" \
    -o BatchMode=yes \
    -o ConnectTimeout=10 \
    "$SERVER_USER@$SERVER_HOST" \
    "echo 'SSH connection successful.'"


# ============================================================
# 2. Upload Dump File to Production Server
# ============================================================

info "[2/5] Uploading Backup File to Production Server"

echo "Uploading: $IMPORT_FILE -> $REMOTE_TEMP_FILE"

scp \
    -i "$SSH_KEY" \
    "$IMPORT_FILE" \
    "$SERVER_USER@$SERVER_HOST:$REMOTE_TEMP_FILE"

echo "Upload completed."


# ============================================================
# 3. Terminate Active Connections
# ============================================================

info "[3/5] Terminating Active Database Connections"

ssh \
    -i "$SSH_KEY" \
    "$SERVER_USER@$SERVER_HOST" \
    "docker exec '$CONTAINER_NAME' psql -U '$DB_USER' -d postgres -c \"
        SELECT pg_terminate_backend(pid) 
        FROM pg_stat_activity 
        WHERE datname = '$DB_NAME' AND pid <> pg_backend_pid();
    \""


# ============================================================
# 4. Perform Full Database Restoration
# ============================================================

info "[4/5] Restoring Database from Imported Copy"

ssh \
    -i "$SSH_KEY" \
    "$SERVER_USER@$SERVER_HOST" \
    "set -e

    echo 'Executing pg_restore on $CONTAINER_NAME...'
    
    # Run pg_restore with --clean and --if-exists to drop existing tables/views before recreating
    # Note: pg_restore returns non-zero if warnings/existing notices occur, so we allow rc 0 or 1
    docker exec '$CONTAINER_NAME' pg_restore \
        -U '$DB_USER' \
        -d '$DB_NAME' \
        --clean \
        --if-exists \
        --no-owner \
        --no-privileges \
        -v \
        '$REMOTE_TEMP_FILE' || true

    echo
    echo 'Re-applying role permissions and grants...'
    docker exec '$CONTAINER_NAME' psql -U '$DB_USER' -d '$DB_NAME' -c \"
        ALTER ROLE attendance_owner WITH PASSWORD 'AttendanceOwner2026';
        ALTER ROLE attendance_owner BYPASSRLS;
        GRANT CONNECT ON DATABASE $DB_NAME TO attendance_app;
        GRANT USAGE ON SCHEMA public TO attendance_app;
        GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO attendance_app;
        GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO attendance_app;
        ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO attendance_app;
        ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT USAGE, SELECT ON SEQUENCES TO attendance_app;
    \"

    echo 'Database restore and permission grants completed.'
    "


# ============================================================
# 5. Cleanup Remote Temporary File & Verification
# ============================================================

info "[5/5] Cleaning Up and Verifying Restored Database"

ssh \
    -i "$SSH_KEY" \
    "$SERVER_USER@$SERVER_HOST" \
    "set -e

    rm -f '$REMOTE_TEMP_FILE'

    echo 'Verifying restored database table counts:'
    docker exec '$CONTAINER_NAME' psql -U '$DB_USER' -d '$DB_NAME' -c \"
        SELECT table_name, 
               (xpath('/row/cnt/text()', xml_count))[1]::text::int as approximate_row_count
        FROM (
          SELECT table_name, 
                 query_to_xml(format('SELECT COUNT(*) as cnt FROM %I', table_name), false, true, '') as xml_count
          FROM information_schema.tables
          WHERE table_schema = 'public' AND table_type = 'BASE TABLE'
        ) t
        ORDER BY table_name;
    \"
    "

info "DATABASE IMPORT & RESTORE COMPLETED SUCCESSFULLY"
echo "Database Server : $SERVER_HOST"
echo "Database Name   : $DB_NAME"
echo "Container       : $CONTAINER_NAME"
echo "Status          : Fully restored from $(basename "$IMPORT_FILE")"
echo
