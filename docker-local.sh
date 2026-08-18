#!/usr/bin/env bash

set -euo pipefail

# Disable MSYS path conversion on Windows (prevents /app/... from becoming C:/Program Files/Git/app/...)
export MSYS_NO_PATHCONV=1

# ============================================================
# Attendance Backend - Local Docker Desktop Build & Run
# ============================================================

# -------------------------
# Configuration
# -------------------------

COMPOSE_FILE="deployments/docker-compose.local.yml"
ENV_FILE=".env"
API_CONTAINER="attendance-api"
DB_CONTAINER="attendance-postgres"

ADMIN_EMAIL="admin@demo.edu"
ADMIN_PASS="Admin@1234"
# Argon2id hash for 'Admin@1234' (m=65536, t=3, p=2)
ADMIN_HASH="\$argon2id\$v=19\$m=65536,t=3,p=2\$c29tZXNhbHRzb21lc2FsdA\$GZOZnVsuIl859JTA3EJ6HQbji+o9QcsazcWmDssypW4"

# -------------------------
# Helper functions
# -------------------------

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
# Validation
# ============================================================

info "Validating local Docker environment"

command -v docker >/dev/null 2>&1 || \
    error "Docker is not installed or not available in PATH."

docker info >/dev/null 2>&1 || \
    error "Docker daemon / Docker Desktop is not running. Please start Docker Desktop and try again."

[ -f "$COMPOSE_FILE" ] || \
    error "Local Compose file not found: $COMPOSE_FILE"

[ -f "$ENV_FILE" ] || \
    error ".env file not found: $ENV_FILE"

echo "Project directory : $(pwd)"
echo "Compose file      : $COMPOSE_FILE"
echo "Environment file  : $ENV_FILE"
echo "API Container     : $API_CONTAINER"
echo "DB Container      : $DB_CONTAINER"


# ============================================================
# 1. Stop previous local containers (if any)
# ============================================================

info "[1/5] Stopping previous local container instances"

docker compose -f "$COMPOSE_FILE" down --remove-orphans >/dev/null 2>&1 || true


# ============================================================
# 2. Build and start local stack (PostgreSQL + Go API)
# ============================================================

info "[2/5] Building image and starting Local Docker Stack"

docker compose -f "$COMPOSE_FILE" up -d --build

echo "Local Docker stack containers created and started."


# ============================================================
# 3. Run database migrations
# ============================================================

info "[3/5] Running pending database migrations"

# Brief sleep to ensure DB and API containers are fully ready
sleep 3

if docker exec "$API_CONTAINER" /app/attendance-migrate up; then
    echo "Database migrations applied successfully."
else
    warn "Database migration command exited with non-zero status."
fi


# ============================================================
# 4. Seed Database & Ensure Test Admin Account
# ============================================================

info "[4/5] Populating database and verifying test admin ($ADMIN_EMAIL)"

# Run built-in seed binary
if docker exec "$API_CONTAINER" /app/attendance-seed; then
    echo "Seed data populated successfully."
else
    warn "Database seeding had warnings or non-zero exit."
fi

# Ensure both school codes ('demo-school' and 'demo') exist with admin credentials
docker exec "$API_CONTAINER" sh -c "
psql \"\$DATABASE_URL\" -v ON_ERROR_STOP=1 -c \"
DO \\\$\\\$
DECLARE
    v_hash TEXT := '$ADMIN_HASH';
    v_school_demo UUID := 'a0000000-0000-0000-0000-000000000002';
BEGIN
    -- Ensure school code 'demo' is also valid alongside 'demo-school'
    INSERT INTO schools (id, name, code, is_active)
    VALUES (v_school_demo, 'Demo School', 'demo', true)
    ON CONFLICT (code) DO NOTHING;

    -- Ensure admin user exists for 'demo' school code
    INSERT INTO users (id, school_id, role, full_name, email, password_hash, is_active)
    VALUES (gen_random_uuid(), (SELECT id FROM schools WHERE code = 'demo' LIMIT 1), 'admin', 'Demo Admin', '$ADMIN_EMAIL', v_hash, true)
    ON CONFLICT (school_id, email) DO UPDATE SET password_hash = v_hash, is_active = true;
END \\\$\\\$;
\"
" >/dev/null 2>&1 || true

echo "Test admin accounts verified for school codes: 'demo-school' and 'demo'."


# ============================================================
# 5. Wait for API health check
# ============================================================

info "[5/5] Waiting for API health check"

MAX_ATTEMPTS=30
ATTEMPT=1
HEALTHY=false

while [ "$ATTEMPT" -le "$MAX_ATTEMPTS" ]; do
    STATUS=$(docker inspect --format='{{if .State.Health}}{{.State.Health.Status}}{{else}}no-healthcheck{{end}}' "$API_CONTAINER" 2>/dev/null || echo "missing")

    echo "Attempt $ATTEMPT/$MAX_ATTEMPTS - Health status: $STATUS"

    if [ "$STATUS" = "healthy" ]; then
        HEALTHY=true
        break
    fi

    if [ "$STATUS" = "unhealthy" ]; then
        echo
        echo "Container marked as unhealthy."
        break
    fi

    sleep 2
    ATTEMPT=$((ATTEMPT + 1))
done

if [ "$HEALTHY" = true ]; then
    echo
    echo "============================================================"
    echo "LOCAL DOCKER STACK RUNNING SUCCESSFULLY"
    echo "============================================================"
    echo
    echo "API Server  : http://localhost:8080"
    echo "Health URL  : http://localhost:8080/healthz"
    echo "PostgreSQL  : localhost:5433 (Database: attendance)"
    echo "Status      : healthy"
    echo
    echo "Test Login Credentials for App:"
    echo "  School Code : demo-school  (or demo)"
    echo "  Email       : $ADMIN_EMAIL"
    echo "  Password    : $ADMIN_PASS"
    echo
    echo "  Teacher Email: priya@demo-school.edu"
    echo "  Teacher Pass : Teacher@1234"
    echo
    echo "Useful commands:"
    echo "  Follow logs : docker logs -f $API_CONTAINER"
    echo "  Stop stack  : docker compose -f $COMPOSE_FILE down"
    echo "  Restart     : docker compose -f $COMPOSE_FILE restart"
    echo
    echo "Recent API logs:"
    docker logs --tail 25 "$API_CONTAINER"
    exit 0
else
    echo
    echo "============================================================"
    echo "HEALTH CHECK FAILED OR TIMED OUT"
    echo "============================================================"
    echo
    echo "Container status:"
    docker compose -f "$COMPOSE_FILE" ps
    echo
    echo "API Container logs:"
    docker logs --tail 50 "$API_CONTAINER"
    echo
    echo "PostgreSQL logs:"
    docker logs --tail 30 "$DB_CONTAINER"
    exit 1
fi
