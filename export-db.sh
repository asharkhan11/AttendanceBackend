#!/usr/bin/env bash

set -euo pipefail

# ============================================================
# Attendance Database - Production Export / Backup Script
# Target Server: 155.248.241.83
# Output: Windows PC Downloads Folder
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

TIMESTAMP="$(date +"%Y%m%d_%H%M%S")"
BACKUP_FILENAME="attendance_prod_backup_${TIMESTAMP}.dump"
REMOTE_TEMP_FILE="/tmp/${BACKUP_FILENAME}"

# -------------------------
# Resolve Local Downloads Directory (Windows / Git Bash / WSL)
# -------------------------

if [ -n "${USERPROFILE:-}" ]; then
    if command -v cygpath >/dev/null 2>&1; then
        DOWNLOADS_DIR="$(cygpath -u "$USERPROFILE/Downloads")"
    else
        # Replace backslashes and drive letter C:\ -> /c/
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
    mkdir -p "$DOWNLOADS_DIR"
fi

LOCAL_OUTPUT_PATH="${DOWNLOADS_DIR}/${BACKUP_FILENAME}"


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
# Validation
# ============================================================

info "Validating Database Export Environment"

command -v ssh >/dev/null 2>&1 || error "SSH is not available in PATH."
command -v scp >/dev/null 2>&1 || error "SCP is not available in PATH."

[ -f "$SSH_KEY" ] || error "SSH key not found: $SSH_KEY"

[ -d "$DOWNLOADS_DIR" ] || mkdir -p "$DOWNLOADS_DIR"

echo "Target DB Server    : $SERVER_USER@$SERVER_HOST"
echo "Container Name      : $CONTAINER_NAME"
echo "Database Name       : $DB_NAME"
echo "Local Download Dir  : $DOWNLOADS_DIR"
echo "Target Backup File  : $BACKUP_FILENAME"


# ============================================================
# 1. Test SSH Connection
# ============================================================

info "[1/4] Testing SSH Connection to DB Server ($SERVER_HOST)"

ssh \
    -i "$SSH_KEY" \
    -o BatchMode=yes \
    -o ConnectTimeout=10 \
    "$SERVER_USER@$SERVER_HOST" \
    "echo 'SSH connection successful.'"


# ============================================================
# 2. Trigger Full pg_dump on Remote PostgreSQL Container
# ============================================================

info "[2/4] Generating Complete Database Dump on Production Server"

ssh \
    -i "$SSH_KEY" \
    "$SERVER_USER@$SERVER_HOST" \
    "set -e

    echo 'Checking if $CONTAINER_NAME container is running...'
    docker ps --filter 'name=^/$CONTAINER_NAME$' --format '{{.Names}}: {{.Status}}'

    echo
    echo 'Running pg_dump for database \"$DB_NAME\"...'
    docker exec '$CONTAINER_NAME' pg_dump \
        -U '$DB_USER' \
        -d '$DB_NAME' \
        -F c \
        -b \
        -v \
        -f '$REMOTE_TEMP_FILE'

    echo 'Dump generated on remote server:'
    ls -lh '$REMOTE_TEMP_FILE'
    "


# ============================================================
# 3. Download Backup to Local PC Downloads Folder
# ============================================================

info "[3/4] Downloading Dump to Local Downloads Directory"

echo "Transferring: $REMOTE_TEMP_FILE -> $LOCAL_OUTPUT_PATH"

scp \
    -i "$SSH_KEY" \
    "$SERVER_USER@$SERVER_HOST:$REMOTE_TEMP_FILE" \
    "$LOCAL_OUTPUT_PATH"

if [ ! -f "$LOCAL_OUTPUT_PATH" ] || [ ! -s "$LOCAL_OUTPUT_PATH" ]; then
    error "Download failed or backup file is empty!"
fi


# ============================================================
# 4. Cleanup Remote Temporary Dump File
# ============================================================

info "[4/4] Cleaning Up Remote Temporary Files"

ssh \
    -i "$SSH_KEY" \
    "$SERVER_USER@$SERVER_HOST" \
    "rm -f '$REMOTE_TEMP_FILE'"

echo "Remote temporary file removed."


# ============================================================
# Summary
# ============================================================

info "DATABASE EXPORT SUCCESSFUL!"

echo "Backup File Saved Locally To:"
echo "-> $LOCAL_OUTPUT_PATH"
echo
echo "File Size: $(ls -lh "$LOCAL_OUTPUT_PATH" | awk '{print $5}')"
echo
echo "To restore this backup onto production in the future, run:"
echo "   ./import-db.sh \"$LOCAL_OUTPUT_PATH\""
echo
