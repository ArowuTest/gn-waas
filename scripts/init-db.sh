#!/bin/bash
# GN-WAAS Database Initialisation Script
# Runs migrations and seeds in order

set -e

echo "=========================================="
echo "GN-WAAS Database Initialisation"
echo "=========================================="

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

# Ensure gnwaas_admin role exists for seeding (bypass RLS)
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-EOSQL
    DO \$\$
    BEGIN
        IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'gnwaas_admin') THEN
            CREATE ROLE gnwaas_admin LOGIN PASSWORD 'gnwaas_admin_dev_2026' BYPASSRLS SUPERUSER;
        END IF;
    END \$\$;
EOSQL

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
    PGPASSWORD=gnwaas_admin_dev_2026 psql -v ON_ERROR_STOP=1 --username "gnwaas_admin" --dbname "$POSTGRES_DB" -h localhost -f "$f"
done

echo "✓ Seeds complete"
echo "=========================================="
echo "GN-WAAS Database Ready"
echo "=========================================="
