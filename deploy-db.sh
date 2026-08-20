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

# Temporary files on local machine
IMAGE_FILE="attendance-db-init.tar.gz"

# Remote deployment directory on DB server
REMOTE_DIR="/home/ubuntu/attendance-db-production"

# Production compose and init files
COMPOSE_FILE="deployments/docker-compose.db-prod.yml"
INIT_SQL_FILE="scripts/init_db.sql"
DOCKERFILE_DB_INIT="deployments/Dockerfile.db-init"
ENV_FILE=".env"


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

echo "Project directory  : $(pwd)"
echo "DB Image Tag       : $IMAGE"
echo "Database Server    : $SERVER_USER@$SERVER_HOST"
echo "DB Compose file    : $COMPOSE_FILE"
echo "SSH Key            : $SSH_KEY"


# ============================================================
# Confirm production deployment
# ============================================================

echo
read -r -p "Deploy database updates, optimizations & migrations to PRODUCTION ($SERVER_HOST)? [y/N]: " CONFIRM

if [[ ! "$CONFIRM" =~ ^[Yy]$ ]]; then
    echo "Database deployment cancelled."
    exit 0
fi


# ============================================================
# 1. Build DB-Init / Migration Docker image
# ============================================================

info "[1/7] Building DB Migration image ($IMAGE)"

docker build \
    -f "$DOCKERFILE_DB_INIT" \
    -t "$IMAGE" \
    .

echo "DB Migration image built successfully."


# ============================================================
# 2. Package and compress image
# ============================================================

info "[2/7] Compressing image for transfer"

rm -f "$IMAGE_FILE"

echo "Exporting and compressing $IMAGE..."
docker save "$IMAGE" | gzip > "$IMAGE_FILE"

echo "Created transfer package:"
ls -lh "$IMAGE_FILE"


# ============================================================
# 3. Test SSH connection to Database Server
# ============================================================

info "[3/7] Testing SSH connection to Database Server ($SERVER_HOST)"

ssh \
    -i "$SSH_KEY" \
    -o BatchMode=yes \
    -o ConnectTimeout=10 \
    "$SERVER_USER@$SERVER_HOST" \
    "echo 'SSH connection to DB Server ($SERVER_HOST) successful.'"


# ============================================================
# 4. Prepare remote deployment directory structure
# ============================================================

info "[4/7] Preparing remote directory ($REMOTE_DIR)"

ssh \
    -i "$SSH_KEY" \
    "$SERVER_USER@$SERVER_HOST" \
    "mkdir -p '$REMOTE_DIR/deployments' '$REMOTE_DIR/scripts'"


# ============================================================
# 5. Upload configuration & image to DB server
# ============================================================

info "[5/7] Uploading database configuration and images"

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
# 6. Load image & Recreate PostgreSQL container
# ============================================================

info "[6/7] Loading image and starting optimized PostgreSQL container"

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
# 7. Wait for PostgreSQL readiness & Apply Migrations
# ============================================================

info "[7/7] Verifying PostgreSQL health and running pending migrations"

ssh \
    -i "$SSH_KEY" \
    "$SERVER_USER@$SERVER_HOST" \
    "set -e

    cd '$REMOTE_DIR'

    echo 'Waiting for PostgreSQL ($CONTAINER_NAME) to become ready...'
    MAX_ATTEMPTS=30
    ATTEMPT=1
    until docker exec '$CONTAINER_NAME' pg_isready -U attendance_owner -d attendance >/dev/null 2>&1 || [ \$ATTEMPT -eq \$MAX_ATTEMPTS ]; do
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

    echo
    echo 'Ensuring database roles and base grants...'
    docker exec -i '$CONTAINER_NAME' psql -U attendance_owner -d attendance < scripts/init_db.sql || true

    echo
    echo 'Running pending database migrations...'
    # Run migration binary against postgres container on docker network
    docker compose \
        --env-file .env \
        -f deployments/docker-compose.db-prod.yml \
        run --rm \
        -e MIGRATE_DATABASE_URL=postgres://attendance_owner:AttendanceOwner2026@postgres:5432/attendance?sslmode=disable \
        db-init /app/attendance-migrate up

    echo
    echo 'Checking migration version...'
    docker compose \
        --env-file .env \
        -f deployments/docker-compose.db-prod.yml \
        run --rm \
        -e MIGRATE_DATABASE_URL=postgres://attendance_owner:AttendanceOwner2026@postgres:5432/attendance?sslmode=disable \
        db-init /app/attendance-migrate version

    echo
    echo 'PostgreSQL status:'
    docker ps \
        --filter 'name=^/$CONTAINER_NAME$' \
        --format 'table {{.Names}}\t{{.Image}}\t{{.Status}}\t{{.Ports}}'
    "

# Clean up local image file
rm -f "$IMAGE_FILE"

info "DATABASE DEPLOYMENT & MIGRATION COMPLETED SUCCESSFULLY"
echo "Database Server : $SERVER_HOST"
echo "Container       : $CONTAINER_NAME"
echo "Postgres Status : Ready & Tuned for High Concurrency (200 conns, SSD I/O, Parallel Workers)"
echo
