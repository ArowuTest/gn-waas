#!/bin/bash
# ============================================================
# GN-WAAS Database Migration & Seed Runner
# ============================================================
# Used by:
#   - Render: database/Dockerfile.migrate (cron job / manual trigger)
#   - Local:  docker-compose run --rm seed-runner
#
# Required env vars:
#   DATABASE_URL        — full postgres connection string (from Render)
#   OR individual:
#   PGHOST, PGPORT, PGDATABASE, PGUSER, PGPASSWORD
#
# Optional:
#   GNWAAS_ADMIN_PASSWORD  — password for gnwaas_admin superuser (for seeds)
#   GNWAAS_APP_PASSWORD    — password for gnwaas_app role (for app connections)
#   SKIP_SEEDS             — set to "true" to skip seed data (migrations only)
# ============================================================

set -e

echo "============================================"
echo "  GN-WAAS Database Migration Runner"
echo "  $(date -u '+%Y-%m-%d %H:%M:%S UTC')"
echo "============================================"

# ── Parse DATABASE_URL if provided (Render format) ────────────────────────
if [ -n "$DATABASE_URL" ]; then
  # Extract components from postgres://user:pass@host:port/dbname
  export PGUSER=$(echo "$DATABASE_URL" | sed -n 's|.*://\([^:]*\):.*|\1|p')
  export PGPASSWORD=$(echo "$DATABASE_URL" | sed -n 's|.*://[^:]*:\([^@]*\)@.*|\1|p')
  export PGHOST=$(echo "$DATABASE_URL" | sed -n 's|.*@\([^:]*\):.*|\1|p')
  export PGPORT=$(echo "$DATABASE_URL" | sed -n 's|.*:\([0-9]*\)/.*|\1|p')
  export PGDATABASE=$(echo "$DATABASE_URL" | sed -n 's|.*/\([^?]*\).*|\1|p')
  echo "Using DATABASE_URL: host=$PGHOST port=$PGPORT db=$PGDATABASE user=$PGUSER"
fi

# ── Wait for PostgreSQL to be ready ───────────────────────────────────────
echo ""
echo "Waiting for PostgreSQL..."
until pg_isready -h "$PGHOST" -p "${PGPORT:-5432}" -U "$PGUSER" -d "$PGDATABASE" 2>/dev/null; do
  echo "  PostgreSQL not ready yet, retrying in 3s..."
  sleep 3
done
echo "✅ PostgreSQL is ready"

# ── Create roles and extensions (as superuser if possible) ────────────────
echo ""
echo "Setting up roles and extensions..."

# Try to create gnwaas_admin and gnwaas_app roles
# On Render managed Postgres, the connection user IS the admin
ADMIN_PASS="${GNWAAS_ADMIN_PASSWORD:-gnwaas_admin_dev_2026}"
APP_PASS="${GNWAAS_APP_PASSWORD:-gnwaas_app_dev_2026}"

psql -v ON_ERROR_STOP=0 << SQL
-- Create roles if they don't exist
DO \$\$
BEGIN
  IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'gnwaas_admin') THEN
    CREATE ROLE gnwaas_admin WITH LOGIN PASSWORD '${ADMIN_PASS}' SUPERUSER;
  ELSE
    ALTER ROLE gnwaas_admin WITH PASSWORD '${ADMIN_PASS}';
  END IF;
END \$\$;

DO \$\$
BEGIN
  IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'gnwaas_app') THEN
    CREATE ROLE gnwaas_app WITH LOGIN PASSWORD '${APP_PASS}';
  ELSE
    ALTER ROLE gnwaas_app WITH PASSWORD '${APP_PASS}';
  END IF;
END \$\$;

-- Grant database access
GRANT ALL PRIVILEGES ON DATABASE ${PGDATABASE:-gnwaas} TO gnwaas_admin;
GRANT CONNECT ON DATABASE ${PGDATABASE:-gnwaas} TO gnwaas_app;

-- Enable extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "postgis";
CREATE EXTENSION IF NOT EXISTS "timescaledb" CASCADE;
SQL

echo "✅ Roles and extensions configured"

# ── Run migrations ─────────────────────────────────────────────────────────
echo ""
echo "Running migrations..."
MIGRATION_COUNT=0
MIGRATION_ERRORS=0

for f in $(ls /migrations/*.sql | sort -V); do
  filename=$(basename "$f")
  echo -n "  Applying $filename ... "
  if psql -v ON_ERROR_STOP=1 -f "$f" 2>/tmp/migration_err; then
    echo "✅"
    MIGRATION_COUNT=$((MIGRATION_COUNT + 1))
  else
    echo "⚠️  (non-fatal)"
    cat /tmp/migration_err | head -3
    MIGRATION_ERRORS=$((MIGRATION_ERRORS + 1))
  fi
done

echo ""
echo "Migrations complete: $MIGRATION_COUNT applied, $MIGRATION_ERRORS warnings"

# ── Run seeds ──────────────────────────────────────────────────────────────
if [ "${SKIP_SEEDS:-false}" = "true" ]; then
  echo ""
  echo "Skipping seeds (SKIP_SEEDS=true)"
else
  echo ""
  echo "Running seed data..."
  SEED_COUNT=0

  for f in $(ls /seeds/0*.sql | sort -V); do
    filename=$(basename "$f")
    echo -n "  Seeding $filename ... "
    if psql -v ON_ERROR_STOP=0 -f "$f" 2>/tmp/seed_err; then
      echo "✅"
      SEED_COUNT=$((SEED_COUNT + 1))
    else
      echo "⚠️  (non-fatal — may already exist)"
    fi
  done

  echo ""
  echo "Seeds complete: $SEED_COUNT files processed"
fi

echo ""
echo "============================================"
echo "  Database setup complete!"
echo "  $(date -u '+%Y-%m-%d %H:%M:%S UTC')"
echo "============================================"
