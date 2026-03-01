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
echo "Running migrations..."
for f in /docker-entrypoint-initdb.d/migrations/*.sql; do
    echo "  → Applying: $(basename $f)"
    psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" -f "$f"
done

echo "✓ Migrations complete"

# Run seeds in order
echo "Running seeds..."
for f in /docker-entrypoint-initdb.d/seeds/*.sql; do
    echo "  → Seeding: $(basename $f)"
    psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" -f "$f"
done

echo "✓ Seeds complete"
echo "=========================================="
echo "GN-WAAS Database Ready"
echo "=========================================="
