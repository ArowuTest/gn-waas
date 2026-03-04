#!/bin/bash
# GN-WAAS Database Initialisation Script
# Runs migrations and seeds in order
#
# Environment variables:
#   POSTGRES_USER          — superuser (gnwaas_user), set by Docker postgres image
#   POSTGRES_DB            — database name (gnwaas)
#   POSTGRES_PASSWORD      — superuser password
#   GNWAAS_ADMIN_PASSWORD  — password for gnwaas_admin (BYPASSRLS) role
#                            INFRA-01 fix: sourced from env, not hardcoded
#   GNWAAS_APP_PASSWORD    — password for gnwaas_app (application role, RLS-enforced)
#                            SEC-01 fix: application services connect as gnwaas_app, not superuser
#   APP_ENV                — "production" skips demo seed data

set -e

echo "=========================================="
echo "GN-WAAS Database Initialisation"
echo "=========================================="

# Resolve passwords — use env vars with safe development fallbacks.
# INFRA-01 fix: no hardcoded production passwords.
ADMIN_PASSWORD="${GNWAAS_ADMIN_PASSWORD:-gnwaas_admin_dev_2026}"
APP_PASSWORD="${GNWAAS_APP_PASSWORD:-gnwaas_app_dev_2026}"

# Create Keycloak database
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-EOSQL
    CREATE DATABASE keycloak OWNER $POSTGRES_USER;
EOSQL

echo "✓ Keycloak database created"

# Run migrations in order
# CF-L01 fix: use `sort -V` (version sort) instead of relying on glob expansion.
# Plain glob expansion is alphabetical and can mis-order files like 010a_ vs 010_.
# sort -V handles numeric components correctly: 001 < 002 < ... < 009 < 010 < 011.
echo "Running migrations..."
for f in $(ls -1 /docker-entrypoint-initdb.d/migrations/*.sql | sort -V); do
    echo "  → Applying: $(basename $f)"
    psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" -f "$f"
done

echo "✓ Migrations complete"

# ─── Post-migration role password setup ──────────────────────────────────────
# Migration 012 creates gnwaas_app and gnwaas_admin with placeholder passwords.
# Here we set the real passwords from environment variables so that:
#   • gnwaas_admin — used by seed-runner (BYPASSRLS, for seeding only)
#   • gnwaas_app   — used by all backend services (RLS enforced)
#
# SEC-01 fix: gnwaas_app password is set from GNWAAS_APP_PASSWORD env var.
# INFRA-01 fix: gnwaas_admin password is set from GNWAAS_ADMIN_PASSWORD env var.
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-EOSQL
    -- Set gnwaas_admin password from env (BYPASSRLS superuser for seeding)
    DO \$\$
    BEGIN
        IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'gnwaas_admin') THEN
            CREATE ROLE gnwaas_admin LOGIN BYPASSRLS SUPERUSER;
        END IF;
    END \$\$;
    ALTER ROLE gnwaas_admin PASSWORD '$ADMIN_PASSWORD';

    -- Set gnwaas_app password from env (application role, RLS enforced)
    DO \$\$
    BEGIN
        IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'gnwaas_app') THEN
            CREATE ROLE gnwaas_app LOGIN;
        END IF;
    END \$\$;
    ALTER ROLE gnwaas_app PASSWORD '$APP_PASSWORD';
EOSQL

echo "✓ Role passwords configured from environment variables"

# Run seeds in order as gnwaas_admin (BYPASSRLS) so RLS does not block inserts
# In production (APP_ENV=production), skip demo timeseries data (006_demo_timeseries.sql)
echo "Running seeds..."
for f in $(ls -1 /docker-entrypoint-initdb.d/seeds/*.sql | sort -V); do
    basename_f=$(basename "$f")
    if [ "${APP_ENV:-development}" = "production" ] && echo "$basename_f" | grep -q "demo"; then
        echo "  ⏭  Skipping demo seed in production: $basename_f"
        continue
    fi
    echo "  → Seeding: $basename_f"
    PGPASSWORD="$ADMIN_PASSWORD" psql -v ON_ERROR_STOP=1 --username "gnwaas_admin" --dbname "$POSTGRES_DB" -h localhost -f "$f"
done

echo "✓ Seeds complete"
echo "=========================================="
echo "GN-WAAS Database Ready"
echo "=========================================="
