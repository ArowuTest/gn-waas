# Phase 2-7 — Backend Services Documentation

## Architecture Overview

GN-WAAS uses a **microservices architecture** with 6 Go services communicating via HTTP.
All services follow the same layered structure:

```
cmd/main.go              → Entry point, graceful shutdown
internal/config/         → Viper-based configuration
internal/domain/         → Entities (pure structs, no DB logic)
internal/repository/
  interfaces/            → Repository contracts (interfaces)
  postgres/              → PostgreSQL implementations
internal/service/        → Business logic
internal/handler/        → HTTP handlers (Fiber)
internal/app/            → Dependency wiring
```

## Services

### 1. Tariff Engine (Port 3001)
**Purpose:** Calculate shadow bills using PURC 2026 tiered tariffs + 20% VAT

**Key Endpoints:**
- `POST /api/v1/tariff/calculate` — Calculate shadow bill for a single account
- `POST /api/v1/tariff/batch` — Batch calculate for multiple accounts
- `GET /api/v1/tariff/rates` — Get active tariff rates
- `GET /api/v1/tariff/variance/:district_id` — Get variance summary

**Tariff Logic (PURC 2026):**
```
Residential:
  Tier 1: 0–5 m³    → ₵6.1225/m³
  Tier 2: >5 m³     → ₵10.8320/m³
  Fixed charge:      ₵5.00/month

Commercial:
  All consumption   → ₵18.45/m³
  Fixed charge:     ₵25.00/month

Industrial:
  All consumption   → ₵22.80/m³
  Fixed charge:     ₵50.00/month

VAT: 20% on all categories (GRA 2026 rate)
```

### 2. Sentinel Service (Port 3002)
**Purpose:** Fraud detection engine — runs 5 parallel checks per district

**Detection Checks:**
1. **Shadow Bill Reconciliation** — Compares GWL bill vs shadow bill; flags >15% variance
2. **Phantom Meter Detection** — Detects accounts with impossible consumption patterns
3. **Ghost Account Detection** — Flags accounts outside the pipe network (GPS validation)
4. **District Balance Check** — Production vs billed volume (IWA/AWWA NRW analysis)
5. **Category Mismatch** — Residential accounts with commercial-level consumption (>100 m³/month)

**Orchestrator Pattern:**
All 5 checks run in parallel using goroutines. Results are deduplicated using a
`detection_hash` (SHA-256 of account_id + anomaly_type + period) before persistence.

**Key Endpoints:**
- `POST /api/v1/sentinel/scan/:district_id` — Trigger full scan
- `GET /api/v1/sentinel/anomalies` — List anomalies with filters
- `GET /api/v1/sentinel/summary/:district_id` — District anomaly summary
- `PATCH /api/v1/sentinel/anomalies/:id/resolve` — Resolve anomaly
- `PATCH /api/v1/sentinel/anomalies/:id/false-positive` — Mark false positive

### 3. GRA Bridge (Port 3003)
**Purpose:** Integration with Ghana Revenue Authority VSDC API for VAT invoice signing

**Flow:**
1. Audit event completed → GRA Bridge called
2. Build invoice from audit evidence
3. Submit to GRA VSDC API (sandbox or production)
4. Receive QR code receipt
5. Lock audit event (immutable after GRA signing)

**Sandbox Mode:** Enabled via `GRA_SANDBOX_MODE=true` env var.
Returns mock QR codes until GRA grants production API access.

### 4. API Gateway (Port 3000)
**Purpose:** Single entry point for all frontend requests; handles auth + routing

**RBAC Matrix:**

| Endpoint | SYSTEM_ADMIN | DISTRICT_MANAGER | AUDIT_SUPERVISOR | FIELD_OFFICER | GRA_LIAISON | FINANCE_ANALYST |
|----------|:---:|:---:|:---:|:---:|:---:|:---:|
| GET /districts | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| POST /audits | ✓ | ✓ | ✓ | — | — | — |
| PATCH /audits/:id/assign | ✓ | ✓ | ✓ | — | — | — |
| GET /field-jobs/my-jobs | — | — | — | ✓ | — | — |
| POST /field-jobs/:id/sos | — | — | — | ✓ | — | — |
| GET /config/:category | ✓ | — | — | — | — | — |
| PATCH /config/:key | ✓ | — | — | — | — | — |

### 5. CDC Ingestor (Port 3006)
**Purpose:** Change Data Capture from GWL's read-only database replica

**Schema Mapping:** Configured via `config/gwl_schema_map.yaml`
- Maps GWL column names → GN-WAAS field names
- Normalises GWL category codes → GN-WAAS enums
- Handles type conversions (dates, floats, strings)

**Status:** Skeleton ready. Requires GWL database credentials to activate.
Set `GWL_DB_HOST` in environment to enable live sync.

### 6. OCR Service (Port 3005)
**Purpose:** Meter photo processing using Tesseract OCR

**Processing Pipeline:**
1. Receive meter photo (JPEG/PNG, max 10MB)
2. Compute SHA-256 hash (immutability proof)
3. Write to temp file
4. Run Tesseract with `--psm 7` (single line) + digit whitelist
5. Parse numeric reading + confidence score
6. Return result with photo hash

**GPS Fence Validation:**
Uses Haversine formula to verify field officer is within 50m of meter location.
Prevents remote photo submission fraud.

## Shared Packages (`pkg/shared/`)

| Package | Purpose |
|---------|---------|
| `database/pool.go` | pgxpool with custom type registration, WithTransaction helper |
| `database/errors.go` | PostgreSQL error wrapping (unique violation, FK, not found) |
| `pagination/pagination.go` | Generic paginated result type |
| `http/response/response.go` | Standard API response envelope |
| `middleware/auth.go` | Keycloak JWT validation + RBAC RequireRoles |
| `middleware/logger.go` | Request logging, CORS, security headers, panic recovery |

## Running Services Locally

```bash
# Start all infrastructure
docker-compose up -d postgres redis keycloak minio

# Run individual service
cd services/tariff-engine && go run ./cmd/main.go

# Run all services
docker-compose up
```
