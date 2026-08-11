#!/usr/bin/env bash

set -euo pipefail

# ============================================================
# Attendance Backend - One Click Production Deployment
# ============================================================

# -------------------------
# Configuration
# -------------------------

SERVER_USER="ubuntu"
SERVER_HOST="130.210.50.187"

SSH_KEY="/c/Users/Ashar/Desktop/OCI/keys/ssh-key-2026-06-29.key"

IMAGE_NAME="attendance-api"
IMAGE_TAG="latest"
IMAGE="${IMAGE_NAME}:${IMAGE_TAG}"

CONTAINER_NAME="attendance-api"

# Temporary files on local machine
IMAGE_FILE="attendance-api.tar.gz"

# Remote deployment directory
REMOTE_DIR="/home/ubuntu/attendance-production"

# Production compose file in your project
COMPOSE_FILE="deployments/docker-compose.api-prod.yml"

# Production environment file
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

info "Validating deployment environment"

command -v docker >/dev/null 2>&1 || \
    error "Docker is not installed or not available in PATH."

command -v ssh >/dev/null 2>&1 || \
    error "SSH is not available in PATH."

command -v scp >/dev/null 2>&1 || \
    error "SCP is not available in PATH."

[ -f "$SSH_KEY" ] || \
    error "SSH key not found: $SSH_KEY"

[ -f "$COMPOSE_FILE" ] || \
    error "Production compose file not found: $COMPOSE_FILE"

[ -f "$ENV_FILE" ] || \
    error ".env file not found: $ENV_FILE"

echo "Project directory : $(pwd)"
echo "Docker image      : $IMAGE"
echo "Production server : $SERVER_USER@$SERVER_HOST"
echo "Compose file      : $COMPOSE_FILE"
echo "SSH key           : $SSH_KEY"


# ============================================================
# Confirm production deployment
# ============================================================

echo
read -r -p "Deploy this version to PRODUCTION? [y/N]: " CONFIRM

if [[ ! "$CONFIRM" =~ ^[Yy]$ ]]; then
    echo "Deployment cancelled."
    exit 0
fi


# ============================================================
# 1. Build Docker image
# ============================================================

info "[1/8] Building Docker image"

docker build \
    -f deployments/Dockerfile \
    -t "$IMAGE" \
    .

echo "Docker image built successfully."


# ============================================================
# 2. Remove old local transfer file
# ============================================================

info "[2/8] Preparing image"

rm -f "$IMAGE_FILE"

echo "Exporting and compressing Docker image..."

docker save "$IMAGE" | gzip > "$IMAGE_FILE"

echo "Created:"
ls -lh "$IMAGE_FILE"


# ============================================================
# 3. Test SSH connection
# ============================================================

info "[3/8] Testing production SSH connection"

ssh \
    -i "$SSH_KEY" \
    -o BatchMode=yes \
    -o ConnectTimeout=10 \
    "$SERVER_USER@$SERVER_HOST" \
    "echo 'SSH connection successful.'"


# ============================================================
# 4. Prepare remote deployment directory
# ============================================================

info "[4/8] Preparing production deployment directory"

ssh \
    -i "$SSH_KEY" \
    "$SERVER_USER@$SERVER_HOST" \
    "mkdir -p '$REMOTE_DIR/deployments'"


# ============================================================
# 5. Upload image + deployment configuration
# ============================================================

info "[5/8] Uploading files to production"

echo "Uploading Docker image..."

scp \
    -i "$SSH_KEY" \
    "$IMAGE_FILE" \
    "$SERVER_USER@$SERVER_HOST:$REMOTE_DIR/$IMAGE_FILE"

echo "Uploading production Compose file..."

scp \
    -i "$SSH_KEY" \
    "$COMPOSE_FILE" \
    "$SERVER_USER@$SERVER_HOST:$REMOTE_DIR/deployments/docker-compose.api-prod.yml"

echo "Uploading production .env..."

scp \
    -i "$SSH_KEY" \
    "$ENV_FILE" \
    "$SERVER_USER@$SERVER_HOST:$REMOTE_DIR/.env"

echo "Upload completed."


# ============================================================
# 6. Load image on production server
# ============================================================

info "[6/8] Loading Docker image on production"

ssh \
    -i "$SSH_KEY" \
    "$SERVER_USER@$SERVER_HOST" \
    "set -e

    echo 'Current attendance-api image:'
    docker images '$IMAGE_NAME'

    echo
    echo 'Loading new image...'

    gunzip -c '$REMOTE_DIR/$IMAGE_FILE' | docker load

    echo
    echo 'New attendance-api image:'
    docker images '$IMAGE_NAME'
    "


# ============================================================
# 7. Recreate API container & Run Migrations
# ============================================================

info "[7/8] Recreating attendance-api and running migrations"

ssh \
    -i "$SSH_KEY" \
    "$SERVER_USER@$SERVER_HOST" \
    "set -e

    cd '$REMOTE_DIR'

    echo 'Starting updated attendance-api...'

    docker compose \
        --env-file .env \
        -f deployments/docker-compose.api-prod.yml \
        up -d \
        --no-build \
        --force-recreate \
        api

    echo
    echo 'Running pending database migrations...'
    docker exec '$CONTAINER_NAME' /app/attendance-migrate up

    echo
    echo 'Container status:'

    docker ps \
        --filter 'name=^/$CONTAINER_NAME$' \
        --format 'table {{.Names}}\t{{.Image}}\t{{.Status}}\t{{.Ports}}'
    "


# ============================================================
# 8. Wait for health check
# ============================================================

info "[8/8] Waiting for API health check"

MAX_ATTEMPTS=30
ATTEMPT=1

while [ "$ATTEMPT" -le "$MAX_ATTEMPTS" ]; do

    STATUS=$(ssh \
        -i "$SSH_KEY" \
        "$SERVER_USER@$SERVER_HOST" \
        "docker inspect --format='{{if .State.Health}}{{.State.Health.Status}}{{else}}no-healthcheck{{end}}' '$CONTAINER_NAME'" \
        2>/dev/null || echo "missing")

    echo "Health status: $STATUS"

    if [ "$STATUS" = "healthy" ]; then
        echo
        echo "============================================================"
        echo "DEPLOYMENT SUCCESSFUL"
        echo "============================================================"
        echo
        echo "Container : $CONTAINER_NAME"
        echo "Image     : $IMAGE"
        echo "Status    : healthy"
        echo

        echo "Recent logs:"
        ssh \
            -i "$SSH_KEY" \
            "$SERVER_USER@$SERVER_HOST" \
            "docker logs --tail 30 '$CONTAINER_NAME'"

        # Remove local transfer file
        rm -f "$IMAGE_FILE"

        exit 0
    fi

    if [ "$STATUS" = "unhealthy" ] || [ "$STATUS" = "missing" ]; then
        echo
        echo "Deployment failed."

        echo
        echo "Container status:"
        ssh \
            -i "$SSH_KEY" \
            "$SERVER_USER@$SERVER_HOST" \
            "docker ps -a --filter 'name=^/$CONTAINER_NAME$'"

        echo
        echo "Container logs:"
        ssh \
            -i "$SSH_KEY" \
            "$SERVER_USER@$SERVER_HOST" \
            "docker logs --tail 100 '$CONTAINER_NAME'"

        exit 1
    fi

    sleep 2
    ATTEMPT=$((ATTEMPT + 1))
done


echo
echo "============================================================"
echo "HEALTH CHECK TIMEOUT"
echo "============================================================"

echo "The container did not become healthy within 60 seconds."

ssh \
    -i "$SSH_KEY" \
    "$SERVER_USER@$SERVER_HOST" \
    "docker ps -a --filter 'name=^/$CONTAINER_NAME$' && echo && docker logs --tail 100 '$CONTAINER_NAME'"

exit 1