# Multi-Server Docker Deployment Guide

This guide details how to build, push, and run the split multi-container architecture for **Attendance Backend**:
- **Database Server Instance**: `155.248.241.83` (PostgreSQL 16 + Migration & Seeding runner)
- **Go API Server Instance**: `130.210.50.187` (Go REST API Backend)

---

## 1. Local Machine: Build & Push Images to Docker Hub

Run the following commands on your local development machine or CI/CD runner:

### Step 1: Login to Docker Hub
```bash
docker login
```

### Step 2: Build and Push Docker Images
Replace `YOUR_DOCKERHUB_USERNAME` with your Docker Hub username:

```bash
# Build both images
make docker-build-all

# Tag and push images to Docker Hub
make docker-push DOCKER_USER=YOUR_DOCKERHUB_USERNAME
```

This builds and pushes two container images:
1. `YOUR_DOCKERHUB_USERNAME/attendance-api:latest`
2. `YOUR_DOCKERHUB_USERNAME/attendance-db-init:latest`

---

## 2. Server 1 Setup: Database Instance (`155.248.241.83`)

### Step 1: Transfer Deployment Files
Copy `deployments/docker-compose.db.yml` and `scripts/init_db.sql` to your DB server:
```bash
scp deployments/docker-compose.db.yml user@155.248.241.83:/home/user/docker-compose.yml
scp scripts/init_db.sql user@155.248.241.83:/home/user/scripts/init_db.sql
```

### Step 2: Configure Environment Variables
SSH into `155.248.241.83` and create `.env`:
```bash
ssh user@155.248.241.83
cat << 'EOF' > .env
POSTGRES_DB=attendance
POSTGRES_USER=attendance_owner
POSTGRES_PASSWORD=AttendanceOwner2026

DOCKER_IMAGE_DB_INIT=YOUR_DOCKERHUB_USERNAME/attendance-db-init:latest
EOF
```

### Step 3: Run Database & Auto-Seeding Containers
```bash
docker compose pull
docker compose up -d
```

> [!NOTE]
> The `attendance-db-init` container will wait for PostgreSQL to start, automatically apply all migrations (`cmd/migrate up`), and seed 1 demo school (`Demo High School`) with complete test data, admin, teachers, classes, students, and attendance records.

### Step 4: Firewall Configuration
Ensure TCP port `5432` is open on `155.248.241.83` firewall (ufw / cloud security group) allowing incoming connections from `130.210.50.187`.
```bash
# Ubuntu UFW example:
sudo ufw allow from 130.210.50.187 to any port 5432 proto tcp
```

---

## 3. Server 2 Setup: Go API Instance (`130.210.50.187`)

### Step 1: Transfer Compose File
Copy `deployments/docker-compose.api.yml` to your Go API server:
```bash
scp deployments/docker-compose.api.yml user@130.210.50.187:/home/user/docker-compose.yml
```

### Step 2: Configure Environment Variables
SSH into `130.210.50.187` and create `.env`:
```bash
ssh user@130.210.50.187
cat << 'EOF' > .env
PORT=8080
ENV=production

# Connect to remote PostgreSQL server on 155.248.241.83
DATABASE_URL=postgres://attendance_app:AttendanceApp2026@155.248.241.83:5432/attendance?sslmode=disable
MIGRATE_DATABASE_URL=postgres://attendance_owner:AttendanceOwner2026@155.248.241.83:5432/attendance?sslmode=disable

JWT_SECRET=your_secure_random_production_jwt_secret_key_here
JWT_ACCESS_TTL=15m
JWT_REFRESH_TTL=720h

ARGON2_MEMORY_KB=65536
ARGON2_ITERATIONS=3
ARGON2_PARALLELISM=2

CORS_ALLOWED_ORIGINS=*
RATE_LIMIT_LOGIN_PER_MIN=10

DB_MAX_CONNS=20
DB_MIN_CONNS=4
DB_MAX_CONN_IDLE_TIME=30m
DB_MAX_CONN_LIFETIME=1h

# Meta WhatsApp Cloud API Configuration (Disabled by default; set ENABLE_WHATSAPP=true to enable)
ENABLE_WHATSAPP=false
# WHATSAPP_API_TOKEN=your_whatsapp_token
# WHATSAPP_PHONE_NUMBER_ID=your_phone_number_id
# WHATSAPP_APP_ID=your_app_id
# WHATSAPP_APP_SECRET=your_app_secret
# WHATSAPP_WABA_ID=your_waba_id
# WHATSAPP_WEBHOOK_VERIFY_TOKEN=your_verify_token
# WHATSAPP_TEMPLATE_NAME=absent_student_notification
# WHATSAPP_TEMPLATE_LANG=en
# DEFAULT_COUNTRY_CODE=91

DOCKER_IMAGE_API=YOUR_DOCKERHUB_USERNAME/attendance-api:latest
EOF
```

### Step 3: Run Go API Service
```bash
docker compose pull
docker compose up -d
```

### Step 4: Verify API Service Health
```bash
curl http://130.210.50.187:8080/healthz
```

---

## 4. Default Seeded Credentials for Testing

The automated seeding script populates **Demo High School** (`demo-school`) with the following test credentials:

- **School Admin**:
  - **Email**: `admin@demo-school.edu`
  - **Password**: `Admin@1234`

- **Teachers**:
  - **Email**: `priya@demo-school.edu`
  - **Password**: `Admin@1234` (or `Teacher@1234`)
