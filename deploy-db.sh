#!/usr/bin/env bash

set -euo pipefail

# ============================================================
# Attendance Database - One Click Production DB Deployment
# Target: PostgreSQL Production Server (155.248.241.83)
# ============================================================

# -------------------------
# Configuration
# -------------------------

SERVER_USER="ubuntu"
SERVER_HOST="155.248.241.83"

SSH_KEY="/c/Users/Ashar/Desktop/OCI/keys/ssh-key-2026-06-29.key"

IMAGE_NAME="attendance-db-init"
IMAGE_TAG="latest"
IMAGE="${IMAGE_NAME}:${IMAGE_TAG}"

CONTAINER_NAME="attendance-postgres"
DB_NAME="attendance"
DB_USER="attendance_owner"

# Temporary files on local machine
IMAGE_FILE="attendance-db-init.tar.gz"

# Remote deployment directory on DB server
REMOTE_DIR="/home/ubuntu/attendance-db-production"

# Production compose and init files
COMPOSE_FILE="deployments/docker-compose.db-prod.yml"
INIT_SQL_FILE="scripts/init_db.sql"
DOCKERFILE_DB_INIT="deployments/Dockerfile.db-init"
ENV_FILE=".env"

# Local backup directory
BACKUP_DIR="$(pwd)/scripts/superuser-tools/backups"


# ============================================================
# Helper functions
# ============================================================

info() {
    echo
    echo "============================================================"
    echo "$1"
    echo "============================================================"
}

warn() {
    echo
    echo "WARNING: $1"
}

error() {
    echo
    echo "ERROR: $1"
    exit 1
}


# ============================================================
# Mode Selection & CLI Flags
# ============================================================

DEPLOY_MODE="prompt"

for arg in "$@"; do
    case "$arg" in
        --fresh|--reset|-f)
            DEPLOY_MODE="fresh"
            ;;
        --standard|--migrate|-s)
            DEPLOY_MODE="standard"
            ;;
        --help|-h)
            echo "Usage: ./deploy-db.sh [OPTIONS]"
            echo
            echo "Options:"
            echo "  --standard, -s    Standard deployment: Preserves data and applies pending migrations."
            echo "  --fresh, -f       Fresh start / Reset: Takes local backup to superuser-tools/backups,"
            echo "                    wipes database cleanly, runs init_db and applies all migrations from scratch."
            echo "  --help, -h        Show this help message."
            echo
            exit 0
            ;;
    esac
done


# ============================================================
# Validation
# ============================================================

info "Validating database deployment environment"

command -v docker >/dev/null 2>&1 || \
    error "Docker is not installed or not available in PATH."

command -v ssh >/dev/null 2>&1 || \
    error "SSH is not available in PATH."

command -v scp >/dev/null 2>&1 || \
    error "SCP is not available in PATH."

[ -f "$SSH_KEY" ] || \
    error "SSH key not found: $SSH_KEY"

[ -f "$COMPOSE_FILE" ] || \
    error "Production DB compose file not found: $COMPOSE_FILE"

[ -f "$INIT_SQL_FILE" ] || \
    error "Init SQL file not found: $INIT_SQL_FILE"

[ -f "$DOCKERFILE_DB_INIT" ] || \
    error "Dockerfile.db-init not found: $DOCKERFILE_DB_INIT"

[ -f "$ENV_FILE" ] || \
    error ".env file not found: $ENV_FILE"

mkdir -p "$BACKUP_DIR"

echo "Project directory  : $(pwd)"
echo "DB Image Tag       : $IMAGE"
echo "Database Server    : $SERVER_USER@$SERVER_HOST"
echo "DB Compose file    : $COMPOSE_FILE"
echo "SSH Key            : $SSH_KEY"
echo "Backup Directory   : $BACKUP_DIR"


# ============================================================
# Deployment Mode Selection
# ============================================================

if [ "$DEPLOY_MODE" = "prompt" ]; then
    echo
    echo "Select Database Deployment Mode:"
    echo "  1) Standard Update : Keep existing data & apply pending migrations [Default]"
    echo "  2) Fresh Start     : Take backup to superuser-tools/backups, wipe DB & start fresh with empty tables"
    echo
    read -r -p "Enter choice [1 or 2, default 1]: " MODE_CHOICE
    case "${MODE_CHOICE:-1}" in
        2|fresh|reset)
            DEPLOY_MODE="fresh"
            ;;
        *)
            DEPLOY_MODE="standard"
            ;;
    esac
fi


# ============================================================
# Confirmations & Safety Checks
# ============================================================

if [ "$DEPLOY_MODE" = "fresh" ]; then
    echo
    echo "****************************************************************"
    echo "             DANGER: FRESH DATABASE RESET SELECTED              "
    echo "****************************************************************"
    echo " Target Server : $SERVER_HOST"
    echo " Database      : $DB_NAME"
    echo
    echo " Actions to be performed:"
    echo " 1. A FULL backup will be automatically saved to:"
    echo "    $BACKUP_DIR/"
    echo " 2. All existing tables and data in '$DB_NAME' will be WIPED."
    echo " 3. Role grants and fresh schema will be created from scratch."
    echo " 4. All migrations will run UP creating empty, up-to-date tables."
    echo "****************************************************************"
    echo
    read -r -p "To proceed with WIPING and restarting the database, type 'RESET': " CONFIRM_RESET
    if [ "$CONFIRM_RESET" != "RESET" ]; then
        echo "Fresh start cancelled. No changes were made."
        exit 0
    fi
else
    echo
    read -r -p "Deploy database updates & migrations to PRODUCTION ($SERVER_HOST)? [y/N]: " CONFIRM_STD
    if [[ ! "$CONFIRM_STD" =~ ^[Yy]$ ]]; then
        echo "Database deployment cancelled."
        exit 0
    fi
fi


# ============================================================
# 1. Automated Pre-Deployment Backup
# ============================================================

info "[1/8] Checking Database & Creating Backup"

TIMESTAMP=$(date +"%Y%m%d_%H%M%S")
BACKUP_FILE="$BACKUP_DIR/attendance_backup_${TIMESTAMP}.sql.gz"

echo "Testing connection and generating backup..."
if ssh -i "$SSH_KEY" -o BatchMode=yes -o ConnectTimeout=10 "$SERVER_USER@$SERVER_HOST" "docker ps --filter 'name=^/$CONTAINER_NAME$' --format '{{.Names}}'" | grep -q "$CONTAINER_NAME"; then
    echo "Dumping database from $SERVER_HOST ($CONTAINER_NAME)..."
    ssh \
        -i "$SSH_KEY" \
        -o StrictHostKeyChecking=no \
        -o ConnectTimeout=15 \
        "$SERVER_USER@$SERVER_HOST" \
        "docker exec '$CONTAINER_NAME' pg_dump -U '$DB_USER' -d '$DB_NAME' --no-owner --no-privileges" | gzip > "$BACKUP_FILE"

    if [ -f "$BACKUP_FILE" ] && [ -s "$BACKUP_FILE" ]; then
        BACKUP_SIZE=$(ls -lh "$BACKUP_FILE" | awk '{print $5}')
        echo "✓ Backup created successfully: $BACKUP_FILE ($BACKUP_SIZE)"
    else
        if [ "$DEPLOY_MODE" = "fresh" ]; then
            error "Failed to create non-empty backup file! Aborting fresh reset for safety."
        else
            warn "Backup could not be generated (database may be empty or newly initialized)."
            rm -f "$BACKUP_FILE"
        fi
    fi
else
    echo "Database container '$CONTAINER_NAME' is not currently running. Skipping live dump."
fi


# ============================================================
# 2. Build DB-Init / Migration Docker image
# ============================================================

info "[2/8] Building DB Migration image ($IMAGE)"

docker build \
    -f "$DOCKERFILE_DB_INIT" \
    -t "$IMAGE" \
    .

echo "DB Migration image built successfully."


# ============================================================
# 3. Package and compress image
# ============================================================

info "[3/8] Compressing image for transfer"

rm -f "$IMAGE_FILE"

echo "Exporting and compressing $IMAGE..."
docker save "$IMAGE" | gzip > "$IMAGE_FILE"

echo "Created transfer package:"
ls -lh "$IMAGE_FILE"


# ============================================================
# 4. Prepare remote deployment directory structure
# ============================================================

info "[4/8] Preparing remote directory ($REMOTE_DIR)"

ssh \
    -i "$SSH_KEY" \
    "$SERVER_USER@$SERVER_HOST" \
    "mkdir -p '$REMOTE_DIR/deployments' '$REMOTE_DIR/scripts'"


# ============================================================
# 5. Upload configuration & image to DB server
# ============================================================

info "[5/8] Uploading database configuration and images"

echo "Uploading $IMAGE_FILE..."
scp \
    -i "$SSH_KEY" \
    "$IMAGE_FILE" \
    "$SERVER_USER@$SERVER_HOST:$REMOTE_DIR/$IMAGE_FILE"

echo "Uploading $COMPOSE_FILE..."
scp \
    -i "$SSH_KEY" \
    "$COMPOSE_FILE" \
    "$SERVER_USER@$SERVER_HOST:$REMOTE_DIR/deployments/docker-compose.db-prod.yml"

echo "Uploading $INIT_SQL_FILE..."
scp \
    -i "$SSH_KEY" \
    "$INIT_SQL_FILE" \
    "$SERVER_USER@$SERVER_HOST:$REMOTE_DIR/scripts/init_db.sql"

echo "Uploading $ENV_FILE..."
scp \
    -i "$SSH_KEY" \
    "$ENV_FILE" \
    "$SERVER_USER@$SERVER_HOST:$REMOTE_DIR/.env"

echo "Upload completed."


# ============================================================
# 6. Load image & Ensure PostgreSQL container is running
# ============================================================

info "[6/8] Loading image and starting PostgreSQL container"

ssh \
    -i "$SSH_KEY" \
    "$SERVER_USER@$SERVER_HOST" \
    "set -e

    cd '$REMOTE_DIR'

    echo 'Loading new attendance-db-init image...'
    gunzip -c '$IMAGE_FILE' | docker load

    echo
    echo 'Starting high-performance PostgreSQL container...'
    docker compose \
        --env-file .env \
        -f deployments/docker-compose.db-prod.yml \
        up -d \
        --no-build \
        postgres

    echo
    echo 'Postgres container started.'
    "


# ============================================================
# 7. Wait for PostgreSQL readiness
# ============================================================

info "[7/8] Verifying PostgreSQL health"

ssh \
    -i "$SSH_KEY" \
    "$SERVER_USER@$SERVER_HOST" \
    "set -e

    echo 'Waiting for PostgreSQL ($CONTAINER_NAME) to become ready...'
    MAX_ATTEMPTS=30
    ATTEMPT=1
    until docker exec '$CONTAINER_NAME' pg_isready -U '$DB_USER' -d '$DB_NAME' >/dev/null 2>&1 || [ \$ATTEMPT -eq \$MAX_ATTEMPTS ]; do
        echo \"Waiting for database ready state... (\$ATTEMPT/\$MAX_ATTEMPTS)\"
        sleep 2
        ATTEMPT=\$((ATTEMPT + 1))
    done

    if [ \$ATTEMPT -eq \$MAX_ATTEMPTS ]; then
        echo 'ERROR: PostgreSQL failed to become ready in time.'
        docker logs --tail 50 '$CONTAINER_NAME'
        exit 1
    fi

    echo 'PostgreSQL is healthy!'
    "


# ============================================================
# 8. Apply Reset (if fresh mode) & Run Migrations
# ============================================================

if [ "$DEPLOY_MODE" = "fresh" ]; then
    info "[8/8] Wiping Database and Applying All Migrations Fresh"

    ssh \
        -i "$SSH_KEY" \
        "$SERVER_USER@$SERVER_HOST" \
        "set -e

        cd '$REMOTE_DIR'

        echo 'Terminating active database connections...'
        docker exec '$CONTAINER_NAME' psql -U '$DB_USER' -d postgres -c \"
            SELECT pg_terminate_backend(pid) 
            FROM pg_stat_activity 
            WHERE datname = '$DB_NAME' AND pid <> pg_backend_pid();
        \" >/dev/null 2>&1 || true

        echo 'Dropping and recreating public schema (clean wipe)...'
        docker exec '$CONTAINER_NAME' psql -U '$DB_USER' -d '$DB_NAME' -c \"
            DROP SCHEMA IF EXISTS public CASCADE;
            CREATE SCHEMA public;
            GRANT ALL ON SCHEMA public TO $DB_USER;
            GRANT ALL ON SCHEMA public TO public;
        \"

        echo
        echo 'Ensuring database roles and base grants...'
        docker exec -i '$CONTAINER_NAME' psql -U '$DB_USER' -d '$DB_NAME' < scripts/init_db.sql || true

        echo
        echo 'Running all database migrations from scratch (version 1 -> latest)...'
        docker compose \
            --env-file .env \
            -f deployments/docker-compose.db-prod.yml \
            run --rm \
            -e MIGRATE_DATABASE_URL=postgres://$DB_USER:AttendanceOwner2026@postgres:5432/$DB_NAME?sslmode=disable \
            db-init /app/attendance-migrate up

        echo
        echo 'Re-applying application permissions on all fresh tables...'
        docker exec '$CONTAINER_NAME' psql -U '$DB_USER' -d '$DB_NAME' -c \"
            GRANT CONNECT ON DATABASE $DB_NAME TO attendance_app;
            GRANT USAGE ON SCHEMA public TO attendance_app;
            GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO attendance_app;
            GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO attendance_app;
            ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO attendance_app;
            ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT USAGE, SELECT ON SEQUENCES TO attendance_app;
        \"

        echo
        echo 'Migration Version:'
        docker compose \
            --env-file .env \
            -f deployments/docker-compose.db-prod.yml \
            run --rm \
            -e MIGRATE_DATABASE_URL=postgres://$DB_USER:AttendanceOwner2026@postgres:5432/$DB_NAME?sslmode=disable \
            db-init /app/attendance-migrate version

        echo
        echo 'Fresh Database Tables (all empty):'
        docker exec '$CONTAINER_NAME' psql -U '$DB_USER' -d '$DB_NAME' -c \"
            SELECT table_name, 
                   (xpath('/row/cnt/text()', xml_count))[1]::text::int as row_count
            FROM (
              SELECT table_name, 
                     query_to_xml(format('SELECT COUNT(*) as cnt FROM %I', table_name), false, true, '') as xml_count
              FROM information_schema.tables
              WHERE table_schema = 'public' AND table_type = 'BASE TABLE'
            ) t
            ORDER BY table_name;
        \"
        "
else
    info "[8/8] Running Pending Migrations (Preserving Existing Data)"

    ssh \
        -i "$SSH_KEY" \
        "$SERVER_USER@$SERVER_HOST" \
        "set -e

        cd '$REMOTE_DIR'

        echo 'Ensuring database roles and base grants...'
        docker exec -i '$CONTAINER_NAME' psql -U '$DB_USER' -d '$DB_NAME' < scripts/init_db.sql || true

        echo
        echo 'Running pending database migrations...'
        docker compose \
            --env-file .env \
            -f deployments/docker-compose.db-prod.yml \
            run --rm \
            -e MIGRATE_DATABASE_URL=postgres://$DB_USER:AttendanceOwner2026@postgres:5432/$DB_NAME?sslmode=disable \
            db-init /app/attendance-migrate up

        echo
        echo 'Migration Version:'
        docker compose \
            --env-file .env \
            -f deployments/docker-compose.db-prod.yml \
            run --rm \
            -e MIGRATE_DATABASE_URL=postgres://$DB_USER:AttendanceOwner2026@postgres:5432/$DB_NAME?sslmode=disable \
            db-init /app/attendance-migrate version
        "
fi

# Clean up local image file
rm -f "$IMAGE_FILE"

info "DATABASE OPERATION COMPLETED SUCCESSFULLY"
echo "Database Server : $SERVER_HOST"
echo "Container       : $CONTAINER_NAME"
echo "Mode Executed   : $DEPLOY_MODE"
if [ -f "$BACKUP_FILE" ]; then
    echo "Backup Saved At : $BACKUP_FILE"
fi
echo
