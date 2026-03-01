# Phase 1 — Database Layer Documentation

## Overview
The GN-WAAS database is built on **PostgreSQL 16 + TimescaleDB** with PostGIS for geospatial queries.
All migrations are idempotent and ordered numerically.

## Migration Files

| File | Purpose |
|------|---------|
| `001_extensions_and_types.sql` | Enable PostGIS, TimescaleDB, pgcrypto, uuid-ossp; define all ENUM types |
| `002_core_tables.sql` | system_config, tariff_rates, vat_config, districts, water_accounts, users, audit_thresholds, supply_schedules |
| `003_billing_and_shadow_ledger.sql` | gwl_billing_records, shadow_bills, water_balance_records, production_records |
| `004_sentinel_and_audit.sql` | anomaly_flags, audit_events, field_jobs, gra_compliance_logs, audit_trail |
| `005_data_confidence_and_reporting.sql` | data_confidence_scores, recovery_records, notifications, system_health_metrics, cdc_sync_log; views |

## Key Design Decisions

### TimescaleDB Hypertables
`gwl_billing_records`, `production_records`, and `system_health_metrics` are converted to
TimescaleDB hypertables partitioned by time. This enables:
- Automatic time-based partitioning
- Efficient time-range queries (billing period analysis)
- Compression of historical data

### Immutable Audit Trail
`audit_trail` uses append-only semantics. The `is_locked` flag on `audit_events` prevents
modification after GRA signing. This satisfies GRA's requirement for tamper-proof records.

### ENUM Types
All status/category fields use PostgreSQL ENUM types for:
- Type safety at the database level
- Efficient storage (4 bytes vs VARCHAR)
- Self-documenting schema

### PostGIS
`water_accounts` stores GPS coordinates as `GEOMETRY(POINT, 4326)` for:
- Spatial queries (accounts within a district boundary)
- Ghost account detection (accounts outside pipe network)
- Field officer GPS fence validation

## Seed Data

| File | Contents |
|------|---------|
| `001_system_config.sql` | 25+ configurable thresholds (sentinel, GRA, business model, CDC) |
| `002_tariff_rates.sql` | PURC 2026 tiered rates + VAT config |
| `003_districts.sql` | 25 Ghana districts (Accra West + Tema flagged as pilot) |
| `004_users.sql` | 22 users across all 7 RBAC roles |
| `005_sample_accounts.sql` | Sample accounts with known fraud scenarios for testing |

## Running Migrations

```bash
# Local development
./scripts/init-db.sh

# Docker Compose
docker-compose up -d postgres
docker-compose run --rm api-gateway /scripts/init-db.sh

# Production (Kubernetes)
kubectl apply -f infrastructure/k8s/db-migration-job.yaml
```
