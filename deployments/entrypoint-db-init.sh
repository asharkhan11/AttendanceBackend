#!/bin/sh
set -e

DB_URL="${MIGRATE_DATABASE_URL:-$DATABASE_URL}"

if [ -z "$DB_URL" ]; then
  echo "ERROR: Neither MIGRATE_DATABASE_URL nor DATABASE_URL environment variable is set."
  exit 1
fi

echo "==> Waiting for PostgreSQL database to be ready..."

MAX_RETRIES=30
RETRY_COUNT=0

until pg_isready -d "$DB_URL" > /dev/null 2>&1 || [ $RETRY_COUNT -eq $MAX_RETRIES ]; do
  echo "PostgreSQL is not ready yet. Retrying in 2 seconds ($RETRY_COUNT/$MAX_RETRIES)..."
  sleep 2
  RETRY_COUNT=$((RETRY_COUNT + 1))
done

if [ $RETRY_COUNT -eq $MAX_RETRIES ]; then
  echo "ERROR: PostgreSQL was not ready after $MAX_RETRIES attempts. Exiting."
  exit 1
fi

echo "==> PostgreSQL is ready!"

echo "==> Running database migrations..."
/app/attendance-migrate up

if [ "${RUN_SEED:-false}" = "true" ] || [ "${RUN_SEED:-0}" = "1" ]; then
  echo "==> Seeding demo school data for testing..."
  /app/attendance-seed || echo "Warning: Seed skipped or already populated."
else
  echo "==> Production mode: Skipping mock data seed (set RUN_SEED=true to seed)."
fi

echo "==> Database initialization and migrations completed successfully!"
