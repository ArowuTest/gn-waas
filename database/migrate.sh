#!/bin/bash
# ============================================================
# GN-WAAS Database Migration & Seed Runner
# ============================================================
# Used by:
#   - Render: database/Dockerfile.migrate (cron job / manual trigger)
#   - Local:  docker-compose run --rm seed-runner
#
# Required env vars:
#   DATABASE_URL        — full postgres connection string (Render provides this)
#   OR individual:
#   PGHOST, PGPORT, PGDATABASE, PGUSER, PGPASSWORD
#
# Optional:
#   GNWAAS_APP_PASSWORD — password for gnwaas_app role (created if not exists)
#                         If unset, gnwaas_app is aliased to the Render DB user
#   SKIP_SEEDS          — set to "true" to skip seed data (migrations only)
# ============================================================

set -e

echo "============================================"
echo "  GN-WAAS Database Migration Runner"
echo "  $(date -u '+%Y-%m-%d %H:%M:%S UTC')"
echo "============================================"

# ── Parse DATABASE_URL if provided (Render format) ────────────────────────
if [ -n "$DATABASE_URL" ]; then
  # Extract components from postgres://user:pass@host:port/dbname?sslmode=require
  export PGUSER=$(echo "$DATABASE_URL" | sed -n 's|.*://\([^:]*\):.*|\1|p')
  export PGPASSWORD=$(echo "$DATABASE_URL" | sed -n 's|.*://[^:]*:\([^@]*\)@.*|\1|p')
  export PGHOST=$(echo "$DATABASE_URL" | sed -n 's|.*@\([^:]*\):.*|\1|p')
  export PGPORT=$(echo "$DATABASE_URL" | sed -n 's|.*:\([0-9]*\)/.*|\1|p')
  export PGDATABASE=$(echo "$DATABASE_URL" | sed -n 's|.*/\([^?]*\).*|\1|p')
  export PGSSLMODE=require
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

# ── Set up gnwaas_app role ─────────────────────────────────────────────────
# On Render managed Postgres, the connection user (gnwaas_user) is the owner.
# We create gnwaas_app as a separate role for RLS policies.
# If GNWAAS_APP_PASSWORD is not set, we grant the Render user the gnwaas_app
# role so it can satisfy the "TO gnwaas_app" RLS policy clauses.
echo ""
echo "Setting up application roles..."

APP_PASS="${GNWAAS_APP_PASSWORD:-}"

psql -v ON_ERROR_STOP=0 << SQL
-- Enable required extensions (non-fatal if unavailable on managed Postgres)
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Create gnwaas_app role for RLS policies
DO \$\$
BEGIN
  IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'gnwaas_app') THEN
    IF '${APP_PASS}' != '' THEN
      CREATE ROLE gnwaas_app WITH LOGIN PASSWORD '${APP_PASS}';
    ELSE
      -- No password provided: create as nologin role (Render user will be granted it)
      CREATE ROLE gnwaas_app NOLOGIN;
    END IF;
    RAISE NOTICE 'Created gnwaas_app role';
  ELSE
    RAISE NOTICE 'gnwaas_app role already exists';
  END IF;
END \$\$;

-- Grant the Render DB user (gnwaas_user) the gnwaas_app role so RLS policies
-- that say "TO gnwaas_app" apply to connections made by gnwaas_user.
DO \$\$
BEGIN
  IF EXISTS (SELECT FROM pg_roles WHERE rolname = '${PGUSER}') AND
     EXISTS (SELECT FROM pg_roles WHERE rolname = 'gnwaas_app') THEN
    EXECUTE 'GRANT gnwaas_app TO ' || quote_ident('${PGUSER}');
    RAISE NOTICE 'Granted gnwaas_app to ${PGUSER}';
  END IF;
END \$\$;

-- Grant table permissions to gnwaas_app
GRANT CONNECT ON DATABASE "${PGDATABASE}" TO gnwaas_app;
GRANT USAGE ON SCHEMA public TO gnwaas_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO gnwaas_app;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO gnwaas_app;

-- Ensure future tables are also accessible
ALTER DEFAULT PRIVILEGES IN SCHEMA public
  GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO gnwaas_app;
ALTER DEFAULT PRIVILEGES IN SCHEMA public
  GRANT USAGE, SELECT ON SEQUENCES TO gnwaas_app;
SQL

echo "✅ Roles configured"

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
    head -3 /tmp/migration_err
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
